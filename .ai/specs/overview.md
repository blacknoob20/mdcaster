# SPEC — mdcaster (Markdown Caster)

**Versión:** 1.1-spec
**Fecha:** 2026-10-20
**Estado:** vigente

> Las **decisiones de arquitectura** (el "por qué") están en
> [`ARCHITECTURE.md`](../../ARCHITECTURE.md). Este documento especifica el
> comportamiento: el "qué".

---

## Descripción general

`mdcaster` es un proyecto **Go 1.21** (módulo `mdcaster`) que convierte documentos a Markdown y
limpia el resultado. Consta de dos binarios y una librería interna:

| Binario | Ruta | Propósito |
|---|---|---|
| `mdcaster` | `cmd/mdcaster/` | Convierte `.docx` y `.pdf` a `.md` con concurrencia y reporte de integridad. |
| `mdclean` | `cmd/mdclean/` | Limpia y normaliza un `.md` ya convertido (modo genérico y modo TDR). |
| — | `internal/converter/` | Librería: selección de procesador por extensión, conversión y verificación de integridad. Un subpaquete por formato: `docx/` y `pdf/`. |

El objetivo de negocio es **conservar el contenido del documento original en el Markdown generado**,
y poder demostrarlo con un porcentaje de integridad por archivo.

## Arquitectura por capas

| Capa | Ruta | Responsabilidad |
|---|---|---|
| **CLI conversión** | `cmd/mdcaster/` | Flags, validación de entrada, escaneo, pool de workers, reporte con color |
| **CLI limpieza** | `cmd/mdclean/` | Normalización de Markdown (genérica o TDR), escritura a stdout o in-place. Ficheros: `main.go` (CLI), `generic.go` (modo genérico), `tdr.go` (modo TDR), `corpus.go` (datos del corpus), `markdown.go` (primitivas compartidas) |
| **Dispatcher** | `internal/converter/converter.go` | API interna (`Converter`), conversión por archivo, `GetOutputPath` |
| **Integridad** | `internal/converter/integrity.go` | Porcentaje de contenido conservado: reutiliza el texto que el processor ya extrajo |
| **Factory** | `internal/converter/processor.go` | Interfaz `FileProcessor`, registro y resolución de procesadores por extensión |
| **Módulo DOCX** | `internal/converter/docx/` | Conversión DOCX → Markdown: parser, modelo, render, numeración, notas, imágenes y metadatos |
| **Módulo PDF** | `internal/converter/pdf/` | Conversión PDF → texto plano con `pdftotext` |

Dependencias:

| Dependencia | Tipo | Uso | Obligatoria |
|---|---|---|---|
| — | — | **Ninguna dependencia de Go.** `go.mod` no tiene requisitos | — |
| `pdftotext` (poppler-utils) | Binario externo | Extracción de texto de `.pdf` | Runtime, solo para PDF |

`pdftotext` es una **excepción documentada**: la biblioteca estándar de Go no incluye un parser de
PDF. Ver [decisión D4](../../ARCHITECTURE.md#d4--pdf-con-pdftotext-excepción-documentada).

## Flujo de datos principal

```
Entrada (archivo o carpeta)
  │
  └─ cmd/mdcaster/main.go
       ├─ flag.Parse                          → resolución de flags (largo > corto; posicional como fallback)
       ├─ os.Stat(ruta)                       → valida existencia (exit 1 si no existe)
       ├─ factory.IsSupportedFile()            → filtra por extensión (solo primer nivel del directorio)
       ├─ resolución de ruta de salida        → relativa al input + subdirectorio timestamp
       └─ processFilesWithConcurrency()        → N workers (runtime.NumCPU() por defecto)
            ├─ por cada worker: converter.NewConverter()   ← instancia propia, no compartida
            │    └─ conv.ConvertFile(input, output)         → alias de Convert()
            │         └─ factory.Convert(input, output)
            │              ├─ os.Stat(input)                → "archivo de entrada no encontrado"
            │              ├─ factory.GetProcessor(ext)      → docx.Processor | pdf.Processor
            │              ├─ os.MkdirAll(dir de salida)
            │              └─ processor.Process(input, output)
            └─ conv.CheckIntegrity(input, output)            → % (reutiliza el texto ya extraído)
```

```
Entrada (.md)
  │
  └─ cmd/mdclean/main.go
       ├─ flag.Parse                          → --inplace, --tdr
       ├─ exige exactamente 1 argumento        → usage + exit 2 si no
       ├─ readAllLines(f)                     → scanner con buffer de 4 MiB
       ├─ processTDR(lines)  si --tdr         → 7 pasadas de normalización TDR
       │  process(lines)     si no            → unión de líneas envueltas
       └─ writeLines(stdout | fichero)        → siempre con salto de línea final
```

## Reglas de negocio globales

| Regla | Valor | Referencia |
|---|---|---|
| Extensiones soportadas | `.docx`, `.pdf` | `internal/converter/processor.go:52-53` |
| Resolución de extensión | minúsculas vía `strings.ToLower(filepath.Ext(path))` | `internal/converter/processor.go:60` |
| Salida del conversor | mismo nombre base + `.md` | `internal/converter/converter.go:42-50` |
| Permisos de directorio creado | `0755` | `internal/converter/processor.go:116` |
| Permisos del `.md` escrito | `0644` | `internal/converter/docx/processor.go:135`, `pdf/processor.go:26` |
| Workers por defecto | `runtime.NumCPU()` | `cmd/mdcaster/main.go:153-155` |
| Subcarpeta de salida | siempre `YYYYMMDD-HHMMSS` | `cmd/mdcaster/main.go:140-141` |
| Escaneo de directorios | **no recursivo** (solo primer nivel) | `cmd/mdcaster/main.go:100-114` |
| Rutas de salida relativas | resueltas contra la carpeta de entrada (dir) o contra el dir del fichero | `cmd/mdcaster/main.go:131-137` |
| Integridad vacía | `100` (no verificable) | `internal/converter/integrity.go:53-55, 63-65` |
| Integridad con error | `0` | `internal/converter/integrity.go:24-32, 44-47` |
| Coincidencia de integridad | por **token exacto**, no por subcadena | `internal/converter/integrity.go:67-78` |
| Umbral 🟢 | `>= 95` | `cmd/mdcaster/main.go:250` |
| Umbral 🟡 | `>= 80` | `cmd/mdcaster/main.go:252` |
| Umbral 🟠 | `>= 50` | `cmd/mdcaster/main.go:254` |
| Umbral 🔴 | `< 50` | `cmd/mdcaster/main.go:256` |
| Línea de texto DOCX | 2 espacios por `w:tab`, `\n` por `w:br` | `internal/converter/docx/parser.go` |
| Negrita / cursiva / tachado | `**x**`, `*x*`, `~~x~~` | `internal/converter/docx/spans.go` |
| Subrayado | `<u>x</u>` (HTML en línea) | `internal/converter/docx/spans.go` |
| Descarte previo | `--skip-before-heading` empieza en el primer encabezado | `internal/converter/docx/model.go` |
| Heading por estilo | `Heading1..6` / `Título1..6` → `#`..`######` | `internal/converter/docx/model.go` |
| Título / Subtítulo | `#` / `##` | `internal/converter/docx/model.go` |
| Listas | numeración real de `word/numbering.xml`; viñeta `- `, decimal `N. ` | `internal/converter/docx/numbering.go` |
| Etiquetas no decimales | `<ol type="a" start="N">` (HTML) | `internal/converter/docx/lists.go` |
| Etiquetas compuestas (`2.1.`) | línea literal | `internal/converter/docx/lists.go` |
| Sangría de listas | columna de contenido del marcador del padre | `internal/converter/docx/lists.go` |
| Tablas DOCX | tabla Markdown: primera fila como cabecera | `internal/converter/docx/tables.go` |
| Celdas combinadas | `gridSpan` y `vMerge` dejan las posiciones vacías | `internal/converter/docx/tables.go` |
| Frontmatter | `docProps/core.xml` → `author`, `date`, `updated`; solo si hay metadatos | `internal/converter/docx/frontmatter.go` |
| Notas al pie | `[^id]` en el cuerpo y `[^id]: texto` al final | `internal/converter/docx/footnotes.go` |
| Imágenes | extraídas a `<nombre>_assets/` y enlazadas | `internal/converter/docx/images.go` |
| Línea en blanco tras párrafo | 1 si es lista, 2 si no | `internal/converter/docx/render.go` |
| Comandos PDF | `pdftotext -layout <in> -` | `internal/converter/pdf/processor.go:36` |
| Salida PDF | texto plano sin estructura Markdown | `internal/converter/pdf/processor.go:42` |
| Expresiones regulares | compiladas **una vez**, al inicio del paquete | `cmd/mdclean/generic.go:18`, `cmd/mdclean/tdr.go:16-24`, `cmd/mdclean/corpus.go:32-45` |
| Acumulación de párrafos | con `strings.Builder`, no con `+` | `cmd/mdclean/generic.go:45-64`, `cmd/mdclean/tdr.go:300-328` |
| `mdclean` sin `--tdr` | une líneas "envueltas" | `cmd/mdclean/generic.go:45-64` |
| `mdclean` con `--tdr` | 9 etapas de normalización (7 pasadas y 2 sub-pasos) | `cmd/mdclean/tdr.go:26-39` |
| Escritura `mdclean` | con salto de línea final garantizado | `cmd/mdclean/main.go:128-152` |
| Color de la salida | códigos ANSI de la stdlib; se desactiva sin terminal, con `NO_COLOR` o `TERM=dumb` | `cmd/mdcaster/ansi.go:24-51` |
| Versión del proyecto | SemVer en `internal/version.Version`: **fuente única de verdad** para los dos binarios | `internal/version/version.go:15` |

## Taxonomía de errores

| Código | Origen | Mensaje / comportamiento |
|---|---|---|
| `ERR_INPUT_NOT_FOUND` | `internal/converter/processor.go:105` | `archivo de entrada no encontrado: %w` |
| `ERR_NO_PROCESSOR` | `internal/converter/processor.go:64` | `no hay procesador disponible para archivos con extensión '%s'` |
| `ERR_OUTPUT_DIR_CREATE` | `internal/converter/processor.go:117` | `error creando directorio de salida: %w` |
| `ERR_DOCX_OPEN` | `internal/converter/docx/processor.go` | `error abriendo archivo DOCX: %w` |
| `ERR_DOCX_NO_DOCUMENT_XML` | `internal/converter/docx/processor.go` | `no se encontró document.xml en el archivo DOCX` |
| `ERR_DOCX_READ_XML` | `internal/converter/docx/processor.go` | `error leyendo word/document.xml: %w` |
| `ERR_DOCX_READ_RELS` | `internal/converter/docx/processor.go` | `error leyendo word/_rels/document.xml.rels: %w` |
| `ERR_DOCX_PARSE` | `internal/converter/docx/processor.go` | `error parseando documento: %w` |
| `ERR_DOCX_ASSETS_DIR` | `internal/converter/docx/images.go` | `error creando la carpeta de imágenes: %w` |
| `ERR_DOCX_ASSET_WRITE` | `internal/converter/docx/images.go` | `error escribiendo la imagen %s: %w` |
| `ERR_WRITE_MARKDOWN` | `internal/converter/docx/processor.go`, `pdf/processor.go` | `error escribiendo archivo Markdown: %w` |
| `ERR_PDFTOTEXT` | `internal/converter/pdf/processor.go:39` | `error extrayendo texto del PDF con pdftotext: %w. Instala 'poppler-utils' o asegura que 'pdftotext' esté en PATH` |
| `ERR_MDC_INPUT_MISSING` | `cmd/mdcaster/main.go:77` | `❌ Error: Debe especificar un archivo o carpeta de entrada` → **exit 1** |
| `ERR_MDC_PATH_NOT_FOUND` | `cmd/mdcaster/main.go:87` | `❌ Error: La ruta '%s' no existe` → **exit 1** |
| `ERR_MDC_UNSUPPORTED_TYPE` | `cmd/mdcaster/main.go:123` | `❌ Error: El archivo '%s' no es de un tipo soportado` → **exit 1** |
| `ERR_MDC_NO_SUPPORTED_FILES` | `cmd/mdcaster/main.go:117` | `⚠️  No se encontraron archivos soportados en la carpeta` → **exit 0** |
| `ERR_MDC_USAGE` | `cmd/mdclean/main.go:43-46` | usage en stderr → **exit 2** |
| `ERR_MDCLEAN_OPEN` / `ERR_MDCLEAN_READ` / `ERR_MDCLEAN_WRITE` | `cmd/mdclean/main.go:49-53, 56-60, 78-92, 95-98` | mensaje en stderr → **exit 1** |

Códigos de salida:

| Binario | 0 | 1 | 2 |
|---|---|---|---|
| `mdcaster` | éxito, `--help`, `--version`, carpeta sin ficheros soportados | cualquier error de entrada o conversión | — |
| `mdclean` | éxito, `--version` | error de apertura/lectura/escritura | número de argumentos incorrecto |

## Modelo de datos

### `ConversionResult` — `cmd/mdcaster/result.go:6-13`

Vive en el comando, no en la librería: es la CLI la que mide la duración y la integridad de cada
fichero, y las rellena aquí.

| Campo | Tipo | Significado |
|---|---|---|
| `SourceFile` | `string` | Ruta del fichero de entrada |
| `OutputFile` | `string` | Ruta del `.md` generado |
| `Success` | `bool` | `err == nil` |
| `Error` | `error` | Error de la conversión (nil si éxito) |
| `Duration` | `float64` | Segundos de la conversión |
| `IntegrityPercent` | `float64` | % de contenido conservado (0 en PDF) |

### `FileProcessor` — `internal/converter/processor.go:14-16`

```go
type FileProcessor interface {
    Process(inputPath, outputPath string) error
}
```

Firma **exacta**: añadir parámetros cambia el contrato de todos los procesadores.

### `TextExtractor` — `internal/converter/processor.go:24-27`

```go
type TextExtractor interface {
    ExtractedText() string
}
```

Interfaz **opcional**. La implementa `docx.Processor` para que la verificación de integridad no
tenga que parsear el DOCX por segunda vez. Un procesador que no la implemente sigue siendo válido.

## Índice de módulos

- [converter/factory.md](converter/factory.md) — Interfaz `FileProcessor`, `TextExtractor` y `ProcessorFactory`
- [converter/dispatcher.md](converter/dispatcher.md) — `Converter` (API interna), conversión por archivo, `GetOutputPath`
- [converter/integrity.md](converter/integrity.md) — Verificación de integridad: reutiliza el texto del processor
- [converters/docx.md](converters/docx.md) — Conversión DOCX → Markdown
- [converters/pdf.md](converters/pdf.md) — Conversión PDF → texto plano
- [cli/mdcaster.md](cli/mdcaster.md) — CLI de conversión: flags, concurrencia y reporte
- [cli/mdclean.md](cli/mdclean.md) — CLI de limpieza de Markdown (genérico y TDR)
- [quality/integridad.md](quality/integridad.md) — Verificación de integridad de contenido

## Dependencias y entorno

| Elemento | Requisito | Si falta |
|---|---|---|
| Go | 1.21+ | No se puede compilar ni testear |
| `pdftotext` | poppler-utils | Solo falla la conversión de `.pdf`; el resto de formatos funciona |

Instalación de `pdftotext`:

```bash
sudo apt install poppler-utils   # Debian/Ubuntu
brew install poppler             # macOS
```

## Problemas conocidos (derivados del código vigente)

Estos puntos **no** son comportamientos deseados: son defectos o limitaciones del código actual.
No se corrigen sin acuerdo explícito (política de precedencia SPEC vs. código). Cada uno tiene una
entrada en [`ARCHITECTURE.md`](../../ARCHITECTURE.md#pendientes).

| # | Ubicación | Descripción | Impacto |
|---|---|---|---|
| 1 | `internal/converter/integrity.go:29-32` | La verificación solo mide lo que el processor expone, y el del PDF no implementa `TextExtractor`: sobre un PDF devuelve `0`. | Medio: los PDF salen con 🔴 (0 %) aunque la conversión sea correcta. |
| 2 | `internal/converter/integrity.go:89-97` | `normalizeContent` filtra con `[^a-z0-9\s]`, que **borra** los caracteres acentuados y la `ñ` en vez de normalizarlos: `técnica` → `tcnica`. | Medio: la integridad se calcula sobre tokens degradados. Consistente entre original y Markdown, por lo que no rompe la comparación, pero penaliza la precisión. |
| 3 | `internal/converter/processor.go:71` | `RegisterProcessor` no normaliza la extensión a minúsculas. | Bajo: registrar `.DOCX` crearía una entrada inalcanzable. |
| 4 | `internal/converter/processor.go:76-82` | `SupportedExtensions()` itera un map → **orden no determinista**. | Bajo: la lista de extensiones que muestra la CLI cambia entre ejecuciones. |
| 5 | `internal/converter/docx/processor.go` | Las relaciones (hipervínculos) se parsean y guardan, pero **nunca se emiten** al Markdown. | Bajo: los enlaces del DOCX se pierden. |
| 6 | `internal/converter/docx/tables.go` | Los párrafos numerados **dentro de una celda de tabla** no emiten marcador: la celda muestra su texto. | Bajo: las etiquetas de listas dentro de tablas se pierden, también en `docx2md`. |
| 7 | `cmd/mdcaster/main.go:318-321` | "Speedup por concurrencia" se calcula como `suma(tiempos) / tiempoTotal`, es decir **eficiencia de paralelismo**, no speedup. Con 1 worker da ~1.0x. | Bajo: etiqueta engañosa en el reporte. |
| 8 | `cmd/mdcaster/main.go:100-114` | El escaneo de carpetas no es recursivo: los ficheros en subcarpetas se ignoran en silencio. | Medio: los ficheros en subcarpetas no se convierten. |
| 9 | `internal/converter/docx/lists.go` | Las etiquetas de lista no decimales se emiten como HTML (`<ol type="a">`), donde Markdown no se procesa. | Bajo: el contenido del `<li>` se emite con HTML en línea, no con `**` y `*`. |
| 10 | `internal/converter/docx/tables.go` | Las celdas combinadas (`gridSpan`, `vMerge`) se aplanan a celdas vacías: Markdown no tiene celdas combinadas. | Bajo: la estructura de la tabla se simplifica, el texto no se pierde. |

## Índice de referencias

- Código: `cmd/`, `internal/`
- Specs: `.ai/specs/`
- Decisiones de arquitectura: `ARCHITECTURE.md`
- Documentación de usuario: `README.md`
- Reglas para agentes: `AGENTS.md`
