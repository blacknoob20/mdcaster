# SPEC — `internal/converter/processor.go` (Factory Pattern)

**Módulo:** `converter/factory`
**Código:** `internal/converter/processor.go`
**Depende de:** nada (es la capa base)

---

## Responsabilidad

Definir el **contrato** de conversión por formato y el **registro** de formatos soportados.
Es la única pieza que decide qué procesador corresponde a un fichero. Ningún otro fichero debe
mapear extensiones a implementaciones.

## Por qué un factory

> Decisión: [D1](../../../ARCHITECTURE.md#d1--factory-pattern-para-los-formatos-de-entrada)

El proyecto nació como un conversor DOCX→Markdown. La pregunta era cómo añadir formatos sin
reescribir el código existente cada vez.

Con el factory, añadir un formato son dos cosas: un fichero nuevo con su procesador y una línea de
registro. `converter.go` y `cmd/mdcaster` **no cambian**, porque no conocen los formatos: la CLI
filtra con `IsSupportedFile` y muestra la lista con `SupportedExtensions`, y ambas salen del
registro.

El coste de esa indirección es una interfaz que no se puede cambiar sin romper todos los
procesadores. Es un coste aceptable: la firma `Process(inputPath, outputPath string) error` es
suficiente para cualquier formato de entrada.

## Contrato: `FileProcessor`

```go
// internal/converter/processor.go:11-13
type FileProcessor interface {
    Process(inputPath, outputPath string) error
}
```

Reglas del contrato:

1. La firma es **exacta**. Añadir o cambiar parámetros rompe todos los procesadores existentes.
2. `Process` **debe** dejar el Markdown escrito en `outputPath` con permisos `0644`.
3. `Process` **no** es responsable de crear el directorio de salida: lo hace `ProcessorFactory.Convert`.
4. `Process` devuelve `nil` en éxito; en error devuelve un error envuelto con `%w` y prefijo en español.
5. `Process` no debe imprimir nada por stdout/stderr: la presentación es de la CLI.
6. `Process` debe ser seguro para uso concurrente desde varias goroutines **solo si** cada instancia
   tiene su propio estado (ver `DocxProcessor.relationships`).

### Interfaz opcional: `TextExtractor`

```go
// internal/converter/processor.go
type TextExtractor interface {
    // ExtractedText devuelve el texto plano del último documento procesado.
    ExtractedText() string
}
```

No forma parte de `FileProcessor`. Es **opcional**: un procesador que no la implemente sigue siendo
válido, y la verificación de integridad recurre a leer el original desde disco.

La implementa `DocxProcessor`, que guarda el texto que ya extrajo al convertir. Así la CLI puede
verificar la integridad sin parsear el DOCX por segunda vez. Ver
[../quality/integridad.md](../quality/integridad.md).

Regla: un procesador que implemente `TextExtractor` debe devolver el texto del **último**
documento procesado, y debe invalidarlo al empezar a procesar otro.

### Interfaz opcional: `SkipBeforeHeadingSetter`

```go
// internal/converter/processor.go
type SkipBeforeHeadingSetter interface {
    // SetSkipBeforeHeading activa o desactiva el descarte del contenido
    // anterior al primer encabezado.
    SetSkipBeforeHeading(enabled bool)
}
```

No forma parte de `FileProcessor`. Es **opcional**: un procesador que no la implemente sigue siendo
válido, y simplemente ignora la opción.

La implementa `DocxProcessor`. La CLI la activa con `--skip-before-heading`, que descarta la
portada, el índice y las tablas de versión de los DOCX. El PDF no la implementa: no tiene
encabezados.

La factory la propaga con `SetSkipBeforeHeading`, que recorre los procesadores registrados y se
laplica solo a los que la implementan.

## `ProcessorFactory`

```go
// internal/converter/processor.go:27-29
type ProcessorFactory struct {
    processors map[string]FileProcessor
}
```

| Método | Línea | Contrato |
|---|---|---|
| `NewProcessorFactory() *ProcessorFactory` | 21-31 | Crea la factory con losprocessors registrados: `.docx` → `DocxProcessor`, `.pdf` → `PdfProcessor`. |
| `GetProcessor(path string) (FileProcessor, error)` | 34-43 | Resuelve por extensión **en minúsculas**. Error `ERR_NO_PROCESSOR` si no hay coincide. |
| `RegisterProcessor(ext string, processor FileProcessor)` | 46-48 | Registra un procesador. **No normaliza `ext`** (ver gotcha). |
| `SupportedExtensions() []string` | 51-57 | Devuelve las extensiones registradas. **Orden no determinista** (iteración de map). |
| `IsSupportedFile(path string) bool` | 60-64 | `true` si la extensión del path está registrada. |
| `Convert(path, outputPath string) error` | 67-86 | Pipeline completo de conversión de un fichero. |

### `GetProcessor`

1. Obtiene la extensión con `strings.ToLower(filepath.Ext(path))` — la comparación **no** distingue mayúsculas.
2. Busca la clave exacta en el map. Una clave registrada en mayúsculas nunca se resuelve.
3. Si no existe, devuelve `nil` y `ERR_NO_PROCESSOR` con la extensión en el mensaje.

### `Convert` — pipeline normativo

```
1. os.Stat(path)
   └─ error → ERR_INPUT_NOT_FOUND ("archivo de entrada no encontrado: %w")
2. GetProcessor(path)
   └─ error → ERR_NO_PROCESSOR
3. os.MkdirAll(filepath.Dir(outputPath), 0755)
   └─ error → ERR_OUTPUT_DIR_CREATE
4. processor.Process(path, outputPath)
   └─ el error del procesador se propaga sin envolver
```

Reglas:

1. El orden de los pasos es **normativo**: la validación de existencia precede a la resolución del
   procesador, y la creación del directorio precede a `Process`.
2. `Convert` no comprueba la extensión por su cuenta: delega en `GetProcessor`.
3. `Convert` no crea el `.md`; solo el directorio que lo contiene.

## Formatos registrados

| Extensión | Procesador | Fichero | Spec |
|---|---|---|---|
| `.docx` | `DocxProcessor` | `internal/converter/docx_processor.go` | [converters/docx.md](../converters/docx.md) |
| `.pdf` | `PdfProcessor` | `internal/converter/pdf_processor.go` | [converters/pdf.md](../converters/pdf.md) |

La lista de formatos soportados es **part del contrato funcional**: la CLI la imprime al usuario
cuando no encuentra ficheros (`cmd/mdcaster/main.go:118, 135`). Añadir un formato la cambia.

## Cómo añadir un formato nuevo

```go
// 1. internal/converter/{formato}_processor.go
package converter

type {Formato}Processor struct{}

func New{Formato}Processor() *{Formato}Processor {
    return &{Formato}Processor{}
}

func (fp *{Formato}Processor) Process(inputPath, outputPath string) error {
    // ... escribir el .md en outputPath con 0644
    return nil
}

// 2. internal/converter/processor.go — NewProcessorFactory
pf.processors[".{ext}"] = New{Formato}Processor()
```

3. Test en `internal/converter/`.
4. Actualizar `.ai/specs/overview.md` (extensiones soportadas) y crear
   `.ai/specs/converters/{formato}.md`.
5. Si el formato no puede medirse con `CheckIntegrity`, documentarlo en
   `.ai/specs/quality/integridad.md` y añadir el caso al spec.
6. Si el formato puede medirse, implementar `TextExtractor` para que la verificación de integridad
   no tenga que releer el original.

`converter.go` **no** se toca: el dispatcher no conoce los formatos.

## Gotchas

1. `RegisterProcessor` no normaliza la extensión a minúsculas. Registrar `.DOCX` produce una entrada
   que `GetProcessor` nunca resolverá. Normalizar dentro del método, o documentar el gotcha.
2. `SupportedExtensions()` no tiene orden estable. Cualquier UI que lo muestre debe ordenarlo si
   quiere salida determinista.
3. `GetProcessor` devuelve `(nil, error)`. Llamar a `Process` sobre el resultado sin comprobar el
   error provoca `nil pointer dereference`.
4. La factory no es segura para escritura concurrente sobre el map. `cmd/mdcaster` lo evita creando
   **una `Converter` por worker** (`cmd/mdcaster/main.go:185`), no comparte la instancia. El motivo
   principal no es el map, sino que `DocxProcessor` guarda estado entre llamadas: compartir un
   procesador entre goroutines daría carreras. Ver
   [D2](../../../ARCHITECTURE.md#d2--una-converter-por-worker).
