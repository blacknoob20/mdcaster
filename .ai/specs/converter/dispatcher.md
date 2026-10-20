# SPEC — `internal/converter/converter.go` (Dispatcher y API pública)

**Módulo:** `converter/dispatcher`
**Código:** `internal/converter/converter.go`
**Depende de:** `internal/converter/processor.go` (Factory)

---

## Responsabilidad

Exponer la **API pública** de conversión, orquestar la selección de procesador y calcular la
integridad del contenido. No contiene lógica de formato: delega en los procesadores.

> **Alcance de uso.** El paquete vive en `internal/`, así que **no es importable desde fuera del
> módulo `mdcaster`**. Es una API interna, no una librería pública. Si alguna vez se quiere
> publicar, hay que mover el paquete fuera de `internal/` y definir su contrato de estabilidad.

## `ConversionResult` — `internal/converter/converter.go:15-22`

```go
type ConversionResult struct {
    SourceFile       string
    OutputFile       string
    Success          bool
    Error            error
    Duration         float64
    IntegrityPercent float64
}
```

| Campo | Invariante |
|---|---|
| `SourceFile` | Ruta de la entrada |
| `OutputFile` | Ruta del `.md` generado |
| `Success` | `true` ⟺ `Error == nil` |
| `Error` | `nil` si `Success`; si no, el error original del procesador |
| `Duration` | Segundos. **Lo rellena el llamador**, no la librería |
| `IntegrityPercent` | `0..100`. **Lo rellena el llamador** vía `CheckIntegrity` |

`ConvertDirectory` y `ConvertDirectoryRecursive` **no** rellenan `Duration` ni `IntegrityPercent`:
dejan `0`. Solo `cmd/mdcaster` los rellena, porque hace su propio bucle de conversión.

## `Converter` — `internal/converter/converter.go:25-34`

```go
type Converter struct {
    factory *ProcessorFactory
}
```

`NewConverter()` crea la factory con los procesadores registrados. **Cada `Converter` tiene su
propia factory**: no hay estado compartido. Por eso `cmd/mdcaster` crea un `Converter` por worker.

## API interna

| Método | Línea | Contrato |
|---|---|---|
| `NewConverter() *Converter` | 30-34 | Crea el dispatcher con la factory por defecto. |
| `Convert(input, output string) error` | 37-39 | Convierte un fichero. Delegación total en `factory.Convert`. |
| `ConvertFile(input, output string) error` | 42-44 | **Alias** de `Convert` (compatibilidad hacia atrás). |
| `SetSkipBeforeHeading(enabled bool)` | 46-57 | Hace que la conversión empiece en el primer encabezado. |
| `Converter.CheckIntegrity(source, markdown string) float64` | 52-62 | Reutiliza el texto que el procesador ya extrajo, si puede. |
| `CheckIntegrity(source, markdown string) float64` | 65-73 | Verifica la integridad leyendo el original desde disco. |
| `CheckIntegrityAgainst(originalContent, markdown string) float64` | 78-117 | Verifica la integridad con el texto original ya extraído. |
| `ConvertPDFFile(input, output string) error` | 199-212 | **Alias** de `Convert` con una diferencia: no comprueba que el fichero exista. |
| `ConvertDirectory(inputDir, outputDir string) ([]ConversionResult, error)` | 216-273 | Convierte los ficheros soportados del **primer nivel** de un directorio. |
| `ConvertDirectoryRecursive(inputDir, outputDir string) ([]ConversionResult, error)` | 277-343 | Igual, pero **recursivo** y preservando la estructura de carpetas. |
| `GetOutputPath(inputPath, outputDir string) string` | 346-350 | Nombre base sin extensión + `.md`, dentro de `outputDir`. |
| `ByteCountSI(b int64) string` | 353-364 | Formatea un tamaño en unidades SI (base 1000). **No se usa en ninguna CLI.** |

### `Converter.CheckIntegrity` — `internal/converter/converter.go:52-62`

```go
func (c *Converter) CheckIntegrity(sourceFile, markdownFile string) float64 {
    if processor, err := c.factory.GetProcessor(sourceFile); err == nil {
        if extractor, ok := processor.(TextExtractor); ok {
            if text := extractor.ExtractedText(); text != "" {
                return CheckIntegrityAgainst(text, markdownFile)
            }
        }
    }
    return CheckIntegrity(sourceFile, markdownFile)
}
```

Es el método que usa la CLI. Si el procesador del original ya extrajo el texto al convertir, lo
reutiliza y **no parsea el DOCX por segunda vez**. Si no, recurre a la función de paquete.

Precondición: solo es válido si se llama inmediatamente después de convertir con la **misma**
`Converter`, porque el texto pertenece al estado del procesador.

## `SetSkipBeforeHeading` — `internal/converter/converter.go`

```go
func (c *Converter) SetSkipBeforeHeading(enabled bool) {
    c.factory.SetSkipBeforeHeading(enabled)
}
```

 Hace que la conversión de los DOCX empiece en el primer encabezado: se descartan la portada, el
índice y las tablas de versión que lo anteceden.

1. El descarte lo aplica el processor, no el dispatcher: el texto que se extrae para la
   verificación de integridad se recorta también, de modo que la mide sobre el contenido que se
   conserva.
2. Si el documento no tiene ningún encabezado, se conserva todo.
3. Los formatos que no lo soportan (el PDF) lo ignoran: la propagación va por la interfaz
   opcional `SkipBeforeHeadingSetter`.

## `GetOutputPath` — `internal/converter/converter.go:346-350`

```go
baseName := filepath.Base(inputPath)
nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
return filepath.Join(outputDir, nameWithoutExt+".md")
```

Reglas:

1. Usa **solo el nombre base**: se pierde la estructura de directorios.
2. La extensión se sustituye por `.md`; si el original ya era `.md`, el resultado es el mismo nombre.
3. `filepath.Join` normaliza separadores y limpia la ruta.

## `ConvertDirectory` — `internal/converter/converter.go:216-273`

```
1. os.Stat(inputDir)                  → no existe → ERR_INPUT_DIR_NOT_FOUND
2. os.MkdirAll(outputDir, 0755)        → error → ERR_OUTPUT_DIR_CREATE
3. os.ReadDir(inputDir)                → error → ERR_READ_DIR
4. Por cada entrada:
   ├─ si es directorio → se salta (NO recursivo)
   ├─ outputPath = Join(outputDir, GetOutputPath(inputPath, "")[len(outputDir)+1:])
   ├─ os.MkdirAll(dir(outputPath))     → error → resultado con Success=false
   ├─ factory.GetProcessor(inputPath)  → error → se salta el fichero (continue, sin resultado)
   └─ processor.Process(...)           → resultado con Success = (err == nil)
```

### ⚠️ Defecto vigente: la ruta de salida se calcula con un índice cruzado

`internal/converter/converter.go:242`:

```go
outputPath := filepath.Join(outputDir, GetOutputPath(inputPath, "")[len(outputDir)+1:])
```

`GetOutputPath(inputPath, "")` devuelve el **nombre base + `.md`** (por ejemplo `"a.md"`, 4
caracteres), pero el corte usa `len(outputDir)+1`, una longitud calculada sobre **otra** cadena.

| Condición | Resultado |
|---|---|
| `len(outputDir)+1 > len(nombreBase+".md")` | **panic**: slice bounds out of range |
| `len(outputDir)+1 == len(nombreBase+".md")` | ruta relativa vacía → `outputPath == outputDir` → se intenta escribir un `.md` sobre un directorio → error de escritura |
| `len(outputDir)+1 < len(nombreBase+".md")` | ruta relativa truncada (pierde los primeros caracteres) |

Consecuencia práctica: `Converter.ConvertDirectory` **no es utilizable** con rutas de salida
normales (cualquier ruta absoluta es más larga que un nombre de fichero típico).

**Impacto acotado:** `cmd/mdcaster` **no** usa este método; hace su propio escaneo con
`GetOutputPath(inputFile, outputDir)` (`cmd/mdcaster/main.go:189`), que sí es correcto. La CLI no
se ve afectada.

**No corregir sin acuerdo** (política de precedencia SPEC vs. código): documentar el defecto y
escalarlo. La corrección natural es
`outputPath := filepath.Join(outputDir, strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))+".md")`.

## `ConvertDirectoryRecursive` — `internal/converter/converter.go:277-344`

```
1. os.Stat(inputDir)                  → no existe → ERR_INPUT_DIR_NOT_FOUND
2. os.MkdirAll(outputDir, 0755)        → error → ERR_OUTPUT_DIR_CREATE
3. filepath.Walk(inputDir, func):
   ├─ si el error de walk no es nil → se aborta con ese error
   ├─ si es directorio → se salta
   ├─ relPath = filepath.Rel(inputDir, inputPath)
   ├─ outputPath = Join(outputDir, TrimSuffix(relPath, Ext(relPath)) + ".md")
   ├─ os.MkdirAll(dir(outputPath))     → error → resultado con Success=false, se continúa
   ├─ factory.GetProcessor(inputPath)  → error → se salta el fichero
   └─ processor.Process(...)           → resultado con Success = (err == nil)
4. Si el walk falló → (results, ERR_WALK_DIR)
```

Diferencias con `ConvertDirectory`:

| Aspecto | `ConvertDirectory` | `ConvertDirectoryRecursive` |
|---|---|---|
| Recursión | No | **Sí** |
| Estructura de carpetas en la salida | No (solo nombre base) | **Sí** (relPath) |
| Cálculo de la ruta de salida | Defectuoso (ver arriba) | Correcto |
| Ficheros no soportados | Se saltan en silencio | Se saltan en silencio |
| `Duration` / `IntegrityPercent` | `0` | `0` |

**Regla de diseño:** en ambos métodos, un fichero de extensión no soportada **no** genera un
`ConversionResult`: simplemente se omite. Solo los ficheros soportados aparecen en el resultado.

## `ConvertPDFFile` — `internal/converter/converter.go:199-212`

Alias histórico de `Convert` con una diferencia relevante:

| Método | Comprueba existencia de la entrada |
|---|---|
| `Convert` / `ConvertFile` | **Sí** (`os.Stat` en `factory.Convert`) |
| `ConvertPDFFile` | **No** (llama a `GetProcessor` y `Process` directamente) |

Además, `ConvertPDFFile` **no** restringe la extensión: pese al nombre, usa el procesador que
corresponda a la extensión real del fichero. Mantenerlo por compatibilidad; no usarlo en código
nuevo.

## `ByteCountSI` — `internal/converter/converter.go:353-364`

Formatea un tamaño en unidades SI de base **1000**: `B`, `kB`, `MB`, `GB`, `TB`, `PB`, `EB`.

**No se usa en ninguna CLI**: es código muerto. El sufijo `kB` es poco habitual (lo estándar sería
`KB`). Si se va a usar, renombrar la función y corregir el sufijo.

## Gotchas

1. `ConvertDirectory` puede hacer **panic** (ver arriba). No usarlo.
2. `ConvertDirectory` y `ConvertDirectoryRecursive` no rellenan `Duration` ni `IntegrityPercent`.
3. Los ficheros no soportados no aparecen en el slice de resultados: el nº de resultados puede ser
   menor que el nº de ficheros del directorio.
4. `ConvertPDFFile` no valida la existencia del fichero ni la extensión.
5. `GetOutputPath` descarta la estructura de directorios: solo es válido para el escaneo plano.
6. `ByteCountSI` está muerto y su sufijo `kB` es incorrecto por convención.
