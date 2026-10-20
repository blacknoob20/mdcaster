package converter

import (
	"path/filepath"
	"strings"
)

// Converter es un dispatcher que utiliza la factory para procesar diferentes tipos de archivos
type Converter struct {
	factory *ProcessorFactory
}

// NewConverter crea una nueva instancia del convertidor
func NewConverter() *Converter {
	return &Converter{
		factory: NewProcessorFactory(),
	}
}

// Convert convierte un archivo a Markdown usando el procesador adecuado
func (c *Converter) Convert(inputPath, outputPath string) error {
	return c.factory.Convert(inputPath, outputPath)
}

// ConvertFile es un alias para mantener compatibilidad hacia atrás
func (c *Converter) ConvertFile(inputPath, outputPath string) error {
	return c.Convert(inputPath, outputPath)
}

// SetSkipBeforeHeading hace que la conversión de los DOCX empiece en el
// primer encabezado: se descartan la portada, el índice y las tablas de
// versión que lo anteceden.
//
// El descarte también se aplica al texto que se extrae para la verificación
// de integridad: la mide sobre el contenido que se conserva.
//
// Los formatos que no lo soportan (el PDF) lo ignoran.
func (c *Converter) SetSkipBeforeHeading(enabled bool) {
	c.factory.SetSkipBeforeHeading(enabled)
}

// GetOutputPath devuelve la ruta del Markdown que corresponde a un fichero:
// el nombre base con la extensión cambiada por `.md`, dentro de `outputDir`.
//
// Usa solo el nombre base: se pierde la estructura de directorios.
func GetOutputPath(inputPath, outputDir string) string {
	baseName := filepath.Base(inputPath)
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	return filepath.Join(outputDir, nameWithoutExt+".md")
}
