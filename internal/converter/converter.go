package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConversionResult representa el resultado de una conversión
type ConversionResult struct {
	SourceFile string
	OutputFile string
	Success    bool
	Error      error
	Duration   float64
}

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

// ConvertPDFFile es un alias para mantener compatibilidad hacia atrás
func (c *Converter) ConvertPDFFile(inputPath, outputPath string) error {
	processor, err := c.factory.GetProcessor(inputPath)
	if err != nil {
		return err
	}

	// Crear directorio de salida si no existe
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creando directorio de salida: %w", err)
	}

	return processor.Process(inputPath, outputPath)
}

// ConvertDirectory convierte todos los archivos soportados en un directorio
// Automáticamente selecciona el procesador según la extensión de cada archivo
func (c *Converter) ConvertDirectory(inputDir, outputDir string) ([]ConversionResult, error) {
	var results []ConversionResult

	// Validar que el directorio de entrada existe
	if _, err := os.Stat(inputDir); err != nil {
		return nil, fmt.Errorf("directorio de entrada no encontrado: %w", err)
	}

	// Crear directorio de salida si no existe
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio de salida: %w", err)
	}

	// Leer archivos del directorio
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, fmt.Errorf("error leyendo directorio: %w", err)
	}

	// Procesar cada archivo
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Ignorar subdirectorios por ahora
		}

		inputPath := filepath.Join(inputDir, entry.Name())
		outputPath := filepath.Join(outputDir, GetOutputPath(inputPath, "")[len(outputDir)+1:])

		// Crear subdirectorio de salida si es necesario
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			results = append(results, ConversionResult{
				SourceFile: inputPath,
				OutputFile: outputPath,
				Success:    false,
				Error:      fmt.Errorf("error creando directorio: %w", err),
			})
			continue
		}

		// Intentar obtener el procesador para este archivo
		processor, err := c.factory.GetProcessor(inputPath)
		if err != nil {
			// Si no hay procesador, ignorar el archivo
			continue
		}

		// Procesar el archivo
		err = processor.Process(inputPath, outputPath)
		results = append(results, ConversionResult{
			SourceFile: inputPath,
			OutputFile: outputPath,
			Success:    err == nil,
			Error:      err,
		})
	}

	return results, nil
}

// ConvertDirectoryRecursive convierte todos los archivos soportados en un directorio recursivamente
// Mantiene la estructura de directorios en la salida
func (c *Converter) ConvertDirectoryRecursive(inputDir, outputDir string) ([]ConversionResult, error) {
	var results []ConversionResult

	// Validar que el directorio de entrada existe
	if _, err := os.Stat(inputDir); err != nil {
		return nil, fmt.Errorf("directorio de entrada no encontrado: %w", err)
	}

	// Crear directorio de salida si no existe
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio de salida: %w", err)
	}

	// Caminar recursivamente por los directorios
	err := filepath.Walk(inputDir, func(inputPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil // Continuar a los archivos
		}

		// Calcular la ruta relativa
		relPath, err := filepath.Rel(inputDir, inputPath)
		if err != nil {
			return err
		}

		// Generar ruta de salida manteniendo la estructura
		outputPath := filepath.Join(outputDir, strings.TrimSuffix(relPath, filepath.Ext(relPath))+".md")

		// Crear subdirectorio de salida si es necesario
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			results = append(results, ConversionResult{
				SourceFile: inputPath,
				OutputFile: outputPath,
				Success:    false,
				Error:      fmt.Errorf("error creando directorio: %w", err),
			})
			return nil
		}

		// Intentar obtener el procesador para este archivo
		processor, err := c.factory.GetProcessor(inputPath)
		if err != nil {
			// Si no hay procesador, ignorar el archivo
			return nil
		}

		// Procesar el archivo
		err = processor.Process(inputPath, outputPath)
		results = append(results, ConversionResult{
			SourceFile: inputPath,
			OutputFile: outputPath,
			Success:    err == nil,
			Error:      err,
		})

		return nil
	})

	if err != nil {
		return results, fmt.Errorf("error al procesar directorio: %w", err)
	}

	return results, nil
}

func GetOutputPath(inputPath, outputDir string) string {
	baseName := filepath.Base(inputPath)
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	return filepath.Join(outputDir, nameWithoutExt+".md")
}

// ByteCountSI formatea el tamaño de archivo
func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
