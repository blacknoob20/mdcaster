package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileProcessor define la interfaz para procesar diferentes tipos de archivo
type FileProcessor interface {
	Process(inputPath, outputPath string) error
}

// TextExtractor es una interfaz opcional. Los procesadores que, al convertir,
// han extraído ya el texto del original la implementan. Permite verificar la
// integridad del contenido sin tener que parsear el documento por segunda vez.
//
// Un procesador que no la implemente sigue siendo válido: la verificación
// recurrirá a leer el original desde disco.
type TextExtractor interface {
	// ExtractedText devuelve el texto plano del último documento procesado.
	ExtractedText() string
}

// ProcessorFactory crea procesadores según el tipo de archivo
type ProcessorFactory struct {
	processors map[string]FileProcessor
}

// NewProcessorFactory crea una nueva fábrica de procesadores
func NewProcessorFactory() *ProcessorFactory {
	pf := &ProcessorFactory{
		processors: make(map[string]FileProcessor),
	}

	// Registrar procesadores disponibles
	pf.processors[".docx"] = NewDocxProcessor()
	pf.processors[".pdf"] = NewPdfProcessor()

	return pf
}

// GetProcessor obtiene el procesador adecuado según la extensión del archivo
func (pf *ProcessorFactory) GetProcessor(path string) (FileProcessor, error) {
	ext := strings.ToLower(filepath.Ext(path))

	processor, exists := pf.processors[ext]
	if !exists {
		return nil, fmt.Errorf("no hay procesador disponible para archivos con extensión '%s'", ext)
	}

	return processor, nil
}

// RegisterProcessor registra un nuevo procesador para una extensión
func (pf *ProcessorFactory) RegisterProcessor(ext string, processor FileProcessor) {
	pf.processors[ext] = processor
}

// SupportedExtensions retorna las extensiones soportadas
func (pf *ProcessorFactory) SupportedExtensions() []string {
	var exts []string
	for ext := range pf.processors {
		exts = append(exts, ext)
	}
	return exts
}

// IsSupportedFile verifica si un archivo tiene una extensión soportada
func (pf *ProcessorFactory) IsSupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, exists := pf.processors[ext]
	return exists
}

// Convert convierte un archivo usando el procesador adecuado
func (pf *ProcessorFactory) Convert(path, outputPath string) error {
	// Validar que el archivo de entrada existe
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("archivo de entrada no encontrado: %w", err)
	}

	// Obtener el procesador según el tipo de archivo
	processor, err := pf.GetProcessor(path)
	if err != nil {
		return err
	}

	// Crear directorio de salida si no existe
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creando directorio de salida: %w", err)
	}

	// Procesar el archivo
	return processor.Process(path, outputPath)
}
