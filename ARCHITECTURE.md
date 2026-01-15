# Arquitectura del Convertidor DOCX-to-MD con Factory Pattern

## Resumen

Se ha refactorizado la aplicación para implementar el **Factory Pattern**, permitiendo fácil extensión con nuevos procesadores de archivo sin modificar el código existente.

## Estructura de Archivos

```
internal/converter/
├── processor.go          # Factory pattern e interfaz FileProcessor
├── converter.go          # Dispatcher principal
├── docx_processor.go     # Procesador específico para DOCX
└── pdf_processor.go      # Procesador específico para PDF
```

## Componentes

### 1. `processor.go` - Factory Pattern
Define la interfaz `FileProcessor` y la fábrica `ProcessorFactory`:

```go
type FileProcessor interface {
    Process(inputPath, outputPath string) error
}

type ProcessorFactory struct {
    processors map[string]FileProcessor
}
```

**Responsabilidades:**
- Registrar procesadores para diferentes extensiones
- Obtener el procesador correcto según la extensión del archivo
- Coordinar el flujo de conversión

**Métodos principales:**
- `NewProcessorFactory()` - Crea la fábrica con procesadores registrados
- `GetProcessor(path string)` - Obtiene el procesador para una extensión
- `RegisterProcessor(ext string, processor FileProcessor)` - Registra un nuevo procesador
- `Convert(path, outputPath string)` - Convierte un archivo

### 2. `converter.go` - Dispatcher
Proporciona la interfaz pública y utiliza la factory internamente:

```go
type Converter struct {
    factory *ProcessorFactory
}
```

**Métodos:**
- `NewConverter()` - Crea un nuevo convertidor
- `Convert(inputPath, outputPath string)` - Convierte un archivo individual
- `ConvertFile(inputPath, outputPath string)` - Alias para compatibilidad
- `ConvertPDFFile(inputPath, outputPath string)` - Alias para compatibilidad
- `ConvertDirectory(inputDir, outputDir string)` - Convierte todos los archivos soportados en un directorio
- `ConvertDirectoryRecursive(inputDir, outputDir string)` - Convierte recursivamente manteniendo estructura de directorios

### 3. `docx_processor.go` - Procesador DOCX
Implementa la interfaz `FileProcessor` para archivos DOCX:

```go
type DocxProcessor struct {
    relationships map[string]string
}
```

**Funcionalidades:**
- Lee archivos DOCX como ZIP
- Parsea XML del documento Word
- Extrae relaciones (hiperlinks)
- Convierte a Markdown con preservación de formato:
  - Títulos (H1-H6)
  - Negritas, itálicas, tachados
  - Listas ordenadas
  - Espacios y tabulaciones

### 4. `pdf_processor.go` - Procesador PDF
Implementa la interfaz `FileProcessor` para archivos PDF:

```go
type PdfProcessor struct{}
```

**Funcionalidades:**
- Extrae texto de PDFs usando `pdftotext` (poppler-utils)
- Preserva layout del documento original
- Limpia espacios en blanco innecesarios

## Cómo Agregar Nuevos Procesadores

Para agregar soporte para un nuevo formato (ej: RTF, ODT, etc.):

### 1. Crear un nuevo archivo `{formato}_processor.go`:

```go
package converter

type RtfProcessor struct{}

func NewRtfProcessor() *RtfProcessor {
    return &RtfProcessor{}
}

// Implementar la interfaz FileProcessor
func (rp *RtfProcessor) Process(inputPath, outputPath string) error {
    // Lógica de conversión RTF -> Markdown
    return nil
}
```

### 2. Registrar en `processor.go`:

```go
func NewProcessorFactory() *ProcessorFactory {
    pf := &ProcessorFactory{
        processors: make(map[string]FileProcessor),
    }

    // Agregar esta línea:
    pf.processors[".rtf"] = NewRtfProcessor()

    return pf
}
```

### 3. ¡Listo! No hay cambios en `converter.go`

## Uso Programático

```go
package main

import "docx-to-md/internal/converter"

func main() {
    conv := converter.NewConverter()

    // Convertir archivo individual
    // (automáticamente selecciona procesador según extensión)
    err := conv.Convert("documento.docx", "output/documento.md")
    err := conv.Convert("documento.pdf", "output/documento.md")

    // Convertir todos los archivos en un directorio
    results, err := conv.ConvertDirectory("input/", "output/")
    for _, result := range results {
        if result.Success {
            println("✅", result.SourceFile, "→", result.OutputFile)
        } else {
            println("❌", result.SourceFile, "-", result.Error)
        }
    }

    // Convertir recursivamente manteniendo estructura de directorios
    results, err := conv.ConvertDirectoryRecursive("input/", "output/")
}
```

## Flujo de Selección Automática

### Para archivo individual:
```
archivo.docx → Extensión: .docx → Factory busca .docx → Usa DocxProcessor
archivo.pdf  → Extensión: .pdf  → Factory busca .pdf  → Usa PdfProcessor
archivo.rtf  → Extensión: .rtf  → Factory busca .rtf  → Error (no soportado aún)
```

### Para directorio:
```
ConvertDirectory(input/, output/)
│
├─ README.txt        → No soportado, se ignora
├─ documento.docx    → DocxProcessor → output/documento.md
├─ informe.pdf       → PdfProcessor  → output/informe.md
├─ presentacion.docx → DocxProcessor → output/presentacion.md
└─ datos.xlsx        → No soportado, se ignora

Resultado: 3 archivos procesados, 2 ignorados
```

## Ventajas del Factory Pattern

✅ **Escalabilidad**: Agregar nuevos formatos sin tocar `converter.go`
✅ **Mantenibilidad**: Cada procesador en su propio archivo
✅ **Testabilidad**: Fácil de mockear procesadores para pruebas
✅ **Flexibilidad**: Registrar procesadores dinámicamente
✅ **Extensibilidad**: Sin cambios en la API existente
✅ **Procesamiento Batch**: Procesa carpetas completas automáticamente
✅ **Auto-Detección**: Selecciona procesador según extensión del archivo
✅ **Tolerante**: Ignora archivos no soportados en procesamiento de carpetas

- **Para DOCX**: Ninguno (librería estándar de Go)
- **Para PDF**: `pdftotext` (incluido en `poppler-utils`)
  - Ubuntu/Debian: `sudo apt install poppler-utils`
  - macOS: `brew install poppler`

## Próximas Mejoras Posibles

- [ ] Soporte para ODT (LibreOffice)
- [ ] Soporte para RTF
- [ ] Cacheo de procesadores compilados
- [ ] Procesamiento paralelo de múltiples archivos
- [ ] Mejorar extracción de metadatos de PDF
