# SPEC — `cmd/mdcaster` (CLI de conversión)

**Módulo:** `cli/mdcaster`
**Código:** `cmd/mdcaster/main.go`, `cmd/mdcaster/help.go`
**Depende de:** `mdcaster/internal/converter`, `github.com/fatih/color`

---

## Responsabilidad

Orquestar la conversión de ficheros o carpetas a Markdown con **procesamiento concurrente**,
mostrar progreso por fichero y un resumen final con métricas de rendimiento e integridad.

## Flags

`cmd/mdcaster/main.go:29-36`

| Flag | Corto | Tipo | Default | Descripción |
|---|---|---|---|---|
| `--input` | `-i` | string | `""` | Ruta al archivo o carpeta de entrada |
| `--output` | `-o` | string | `"converted"` | Carpeta de destino |
| `--workers` | `-w` | int | `0` | Número de workers (`0` = auto) |
| `--skip-before-heading` | — | bool | `false` | Empezar la conversión en el primer encabezado |
| `--version` | — | bool | `false` | Mostrar versión |
| `--help` | — | bool | `false` | Mostrar ayuda |

### Versión — `internal/version/version.go`

El número que imprime `--version` sale de `internal/version.Version`, la **única fuente de
verdad** del SemVer del proyecto. Ningún otro fichero lo copia: esta spec, el README y el mensaje
del tag lo describen de forma genérica, para que no envejezcan en cada release.

La publica el agente al cerrar una release. La política está en `ARCHITECTURE.md` (D21) y el
procedimiento en `AGENTS.md`.

### Resolución de flags — `cmd/mdcaster/main.go:55-69`

Precedencia **normativa**:

| Flag | Regla | Referencia |
|---|---|---|
| `-i` / `--input` | Gana el **flag largo**; el corto se usa solo si el largo está vacío | 67-70 |
| `-o` / `--output` | Gana el **flag corto** si tiene valor; el default `"converted"` del flag largo cuenta como valor, así que `-o X` siempre prevalece | 72-75 |
| `-w` / `--workers` | Gana el **flag largo** si es `> 0`; si no, el corto | 77-80 |

> La asimetría entre `-i` y `-o` es deliberada en el código, no un error de la librería: `--output`
> tiene un default no vacío, por lo que la comprobación `*outputShort != ""` siempre puede
> sobreescribirlo. Está documentado aquí para que nadie lo "corrija" por error.

### Argumento posicional

Si `-i`/`--input` están vacíos, se usa `flag.Arg(0)` (línea 86). Si no hay ningún argumento
posicional → `ERR_MDC_INPUT_MISSING`, se imprime el usage y se sale con **código 1**.

## Flujo de `main` — `cmd/mdcaster/main.go:27-165`

```
1. flag.Parse()
2. Imprimir el banner SIEMPRE (línea 52) — incluso con --version y --help
3. --version  → imprimir "Versión: " + version.Version → exit 0
4. --help     → printHelp()                   → exit 0
5. Resolver flags (largo > corto)
6. Resolver input: flag largo → flag corto → flag.Arg(0)
7. Si input == "" → ERR_MDC_INPUT_MISSING → usage → exit 1
8. os.Stat(input)
   └─ no existe → ERR_MDC_PATH_NOT_FOUND → exit 1
9. Determinar ficheros a procesar
   ├─ input es directorio → os.ReadDir (SOLO primer nivel), se filtran por IsSupportedFile
   │     └─ 0 ficheros soportados → warning + lista de extensiones → exit 0
   └─ input es fichero → comprobar IsSupportedFile
         └─ no soportado → ERR_MDC_UNSUPPORTED_TYPE → exit 1
10. Resolver ruta de salida (absoluta) + subdirectorio timestamp
11. os.MkdirAll(salida, 0755)  → error → exit 1
12. Mostrar: carpeta de salida, nº de ficheros, nº de workers
13. processFilesWithConcurrency(..., skipBeforeHeading)
14. printResults()
```

### `--skip-before-heading`

Con el flag activo, la conversión de los DOCX **empieza en el primer encabezado**: se descartan
la portada, el índice y las tablas de versión que lo anteceden.

1. El descarte lo aplica el processor (`docx.Processor`), no la CLI: el texto que se extrae para la
   verificación de integridad se recorta también, de modo que la mide **sobre el contenido que se
   conserva** y no baja por un descarte intencionado.
2. Si el documento **no tiene ningún encabezado**, se conserva todo: descartar un documento
   entero sería perder la información.
3. El PDF ignora la opción: no tiene encabezados.
4. La opción se propaga por la interfaz opcional `SkipBeforeHeadingSetter`
   (`internal/converter/processor.go`), así que los formatos que no la soportan no se ven afectados.

### Banner

El banner (`cmd/mdcaster/main.go:18-25`) se imprime **siempre**, antes de evaluar `--version` y
`--help`. Es parte del contrato de UX: la salida de `mdcaster` empieza siempre por el recuadro.

## Resolución de la ruta de salida — `cmd/mdcaster/main.go:130-141`

```
1. Si la salida es RELATIVA:
   ├─ input es directorio → salida = input/salida
   └─ input es fichero   → salida = dir(input)/salida
2. salida = salida/<YYYYMMDD-HHMMSS>
```

Reglas:

1. La subcarpeta con timestamp es **obligatoria**: los `.md` nunca se escriben en la raíz del destino.
2. El formato del timestamp es `20060102-150405` (hora local).
3. La carpeta de destino por defecto `"converted"` es relativa, así que con un directorio de entrada
   `./documentos` el resultado va a `./documentos/converted/<timestamp>/`.
4. La carpeta de salida se crea con permisos `0755`.

## Selección de ficheros

| Caso | Regla | Referencia |
|---|---|---|
| Directorio | Se listan **solo las entradas del primer nivel**; los subdirectorios se saltan | 111-125 |
| Directorio | Se filtran por `factory.IsSupportedFile(path)`; el resto se ignora en silencio | 122-124 |
| Directorio sin ficheros soportados | Warning + `Extensiones soportadas: [...]` + **exit 0** | 127-131 |
| Fichero único | Debe ser de un tipo soportado; si no → error + **exit 1** | 133-137 |

## Concurrencia — `processFilesWithConcurrency` (`cmd/mdcaster/main.go:168-230`)

```
1. numWorkers <= 0 → runtime.NumCPU()
2. Canal jobs con buffer = len(files)     ← todos los trabajos encolados de golpe
3. numWorkers goroutines:
   ├─ cada worker crea SU PROPIO converter.NewConverter()
   ├─ conv.SetSkipBeforeHeading(skipBeforeHeading)   ← la opción, por worker
   ├─ por cada job:
   │    ├─ outputPath = converter.GetOutputPath(inputFile, outputDir)
   │    ├─ err = conv.ConvertFile(inputFile, outputPath)
   │    ├─ duration = tiempo de la conversión
   │    ├─ si err == nil → integrityPercent = conv.CheckIntegrity(inputFile, outputPath)
   │    ├─ append al slice de resultados (protegido por mutex)
   │    └─ printProgress(current, total, inputFile, ok, integrityPercent)
4. wg.Wait() → devolver resultados
```

Reglas:

1. **Cada worker tiene su propia `Converter`**, y por tanto su propia `ProcessorFactory`. Esto evita
   compartir el map de procesadores entre goroutines. No "optimizar" compartiéndolo.
2. La **integridad solo se calcula si la conversión tuvo éxito** (línea 207-209). En error queda 0.
3. El orden de `results` **no** es determinista: depende del orden en que los workers terminan.
4. `processed` se incrementa con `atomic.AddInt64` (línea 225) — el contador de progreso es
   thread-safe.
5. El nombre de salida es el nombre base de la entrada con extensión `.md`
   (`converter.GetOutputPath`), sin subdirectorios: el escaneo no es recursivo.

## Progreso por fichero — `printProgress` (`cmd/mdcaster/main.go:233-245`)

```
✅ [n/total] <símbolo> Convertido: <nombre> <pct>%
❌ [n/total] Error: <nombre>
```

Reglas:

1. El nombre se trunca a 40 caracteres: si es más largo, se corta a 37 y se añade `...`.
2. Se usa `\r` al inicio de la línea para sobrescribir en consola.
3. El formato del porcentaje es `%.1f%%` (un decimal).

## Símbolo de integridad — `getIntegritySymbol` (`cmd/mdcaster/main.go:248-259`)

| Símbolo | Rango | Significado |
|---|---|---|
| 🟢 | `>= 95` | Integridad óptima |
| 🟡 | `>= 80` | Integridad buena |
| 🟠 | `>= 50` | Integridad aceptable |
| ❌ | `< 50` | Integridad deficiente |

## Resumen final — `printResults` (`cmd/mdcaster/main.go:262-325`)

Bloques, en este orden:

1. Separador + título `📊 RESUMEN DE CONVERSIÓN`.
2. Errores de cada fichero fallido: `❌ <nombre>: <error>`.
3. `✅ Archivos convertidos exitosamente: N` y, si hay, `❌ Archivos con errores: M`.
4. `📋 INTEGRIDAD DE CONTENIDO:` con **Promedio**, **Mínima** y **Máxima** (solo si `successCount > 0`).
5. `⏱️  Tiempo total de ejecución` (formato `formatDuration`).
6. `📈 Tiempo promedio por archivo` y `🚀 Velocidad` (archivos/segundo) — solo si `successCount > 0`.
7. `⚡ Speedup por concurrencia: X.XXx más rápido` — solo si `successCount > 1`.

Reglas:

1. `minIntegrity` se inicializa a `100` y `maxIntegrity` a `0` (líneas 278-279). Si todos los
   ficheros dan 0 %, se reportan mínima 0.00 % y máxima 0.00 %.
2. El "speedup" se calcula como `suma(tiempos de conversión) / tiempo total` (línea 331). Eso es la
   **eficiencia del paralelismo**, no un speedup: con 1 worker vale ~1.0x. Etiqueta engañosa
   documentada en `.ai/specs/overview.md`.
3. `formatDuration` (`cmd/mdcaster/help.go:55-63`): `< 1s` → `%.2fms`; `< 1min` → `%.2fs`; resto →
   `%dm %.2fs`.

## Color

`cmd/mdcaster/ansi.go:81-87` define cinco estilos globales reutilizados en todo el binario:

| Variable | Color | Uso |
|---|---|---|
| `success` | verde bold | ✅ conversiones correctas |
| `info` | cyan | mensajes informativos |
| `warning` | yellow | avisos |
| `orange` | naranja (ANSI 256, 208) | indicador de integridad aceptable |
| `errorMsg` | rojo bold | errores |
| `highlight` | magenta bold | banner, títulos y métricas destacadas |

El color sale de **códigos ANSI con la biblioteca estándar**, sin dependencias. Ver
[decisión D11](../../../ARCHITECTURE.md#d11--cero-dependencias-externas).

Reglas:

1. Los códigos son los mismos que usaba la librería de color anterior, así que la salida visible
   no cambia.
2. El color se decide **una vez** al arrancar (`cmd/mdcaster/ansi.go:24`):
   - se desactiva si `NO_COLOR` está definida;
   - se desactiva si `TERM=dumb`;
   - se desactiva si la salida no es un terminal (`os.Stdout.Stat()` y `ModeCharDevice`).
3. En Windows, `enableVirtualTerminal` (`cmd/mdcaster/ansi_windows.go`) activa el modo VT de la
   consola para que se interpreten los códigos ANSI. En el resto de plataformas es una función
   vacía (`cmd/mdcaster/ansi_other.go`).
4. `style` expone `Print`, `Printf` y `Println` con la misma firma que usaba la librería anterior,
   de modo que las llamadas de la CLI no cambian.

## `printHelp` — `cmd/mdcaster/help.go:9-45`

Secciones, en orden: descripción, `USO` (vía `printUsage`), `OPCIONES`, `INDICADOR DE INTEGRIDAD`
con los 4 símbolos, `EJEMPLOS` con 4 casos.

Las cuatro líneas del indicador se imprimen con los estilos de color semánticos ya definidos
en `cmd/mdcaster/ansi.go`: `success` (verde) para óptima, `warning` (amarillo) para buena,
`orange` (naranja, ANSI 256) para aceptable y `errorMsg` (rojo) para deficiente. El color se
respeta según `colorEnabled`
(`NO_COLOR`, `TERM=dumb`, salida no-terminal).

`printUsage` (`cmd/mdcaster/help.go:48-52`) imprime:

```
USO:
  mdcaster -i <archivo|carpeta> [-o <carpeta_salida>] [-w <workers>]
  mdcaster <archivo|carpeta>
```

## Códigos de salida

| Código | Cuándo |
|---|---|
| `0` | Conversión completada, `--help`, `--version`, o carpeta sin ficheros soportados |
| `1` | Entrada ausente, ruta inexistente, tipo no soportado, error al crear la salida |

## Gotchas

1. El banner se imprime antes de cualquier otra cosa. Cualquier script que parsee la salida debe
   tener en cuenta el banner, el progreso con `\r` y los emojis.
2. El escaneo de directorios **no es recursivo**: los ficheros en subcarpetas se ignoran. La
   librería ya no ofrece ninguna variante recursiva.
3. `-o` siempre gana sobre `--output` (ver "Resolución de flags").
4. El porcentaje de un PDF siempre es 0 % (❌): el processor del PDF no expone su texto, así que
   la verificación no tiene con qué comparar.
5. El orden de los ficheros en el resumen no es el de entrada, sino el de finalización de los workers.
6. `mdcaster` **no** tiene flag de recursividad. Para procesar subcarpetas hay que invocarlo por
   directorio o usar `mdclean` después.
7. `GetOutputPath` solo usa el **nombre base**: dos ficheros homónimos en directorios distintos
   colisionarían en la salida (no es posible hoy por el escaneo no recursivo, pero sí lo sería si se
   activara la recursividad).
8. `--skip-before-heading` **no** vacía un documento sin encabezados: si no hay ninguno, se
   conserva todo.
9. `--skip-before-heading` no cambia la integridad: el descarte se aplica también al texto que se
   extrae para la medición.
