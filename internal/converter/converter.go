package converter

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ConversionResult representa el resultado de una conversión
type ConversionResult struct {
	SourceFile       string
	OutputFile       string
	Success          bool
	Error            error
	Duration         float64
	IntegrityPercent float64 // Porcentaje de contenido que se encontró en el markdown
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

// CheckIntegrity verifica qué porcentaje del contenido original se encuentra en el markdown
func CheckIntegrity(sourceFile, markdownFile string) float64 {
	// Leer el contenido original del DOCX
	originalContent, err := extractDocxContent(sourceFile)
	if err != nil {
		return 0
	}

	// Leer el contenido del markdown generado
	markdownContent, err := os.ReadFile(markdownFile)
	if err != nil {
		return 0
	}

	// Normalizar espacios para comparación
	original := normalizeContent(originalContent)
	markdown := normalizeContent(string(markdownContent))

	// Si no hay contenido original, no se puede verificar
	if len(original) == 0 {
		return 100
	}

	// Contar cuántas palabras del original se encuentran en el markdown
	originalWords := strings.Fields(original)
	foundCount := 0

	for _, word := range originalWords {
		if strings.Contains(markdown, word) {
			foundCount++
		}
	}

	if len(originalWords) == 0 {
		return 100
	}

	percent := float64(foundCount) / float64(len(originalWords)) * 100
	return percent
}

// extractDocxContent extrae todo el contenido de texto de un archivo DOCX
func extractDocxContent(docxPath string) (string, error) {
	reader, err := zip.OpenReader(docxPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var documentContent []byte

	// Buscar el documento principal
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			documentContent, _ = io.ReadAll(rc)
			rc.Close()
			break
		}
	}

	if documentContent == nil {
		return "", fmt.Errorf("no se encontró document.xml")
	}

	// Extraer solo el texto del XML
	return extractTextFromXML(string(documentContent)), nil
}

// extractTextFromXML extrae el texto de los elementos <w:t> del XML
func extractTextFromXML(xmlContent string) string {
	var text strings.Builder
	decoder := xml.NewDecoder(strings.NewReader(xmlContent))

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				// Leer el siguiente token que será el texto
				token, err := decoder.Token()
				if err == nil {
					if charData, ok := token.(xml.CharData); ok {
						text.Write(charData)
						text.WriteString(" ")
					}
				}
			}
		}
	}

	return text.String()
}

// normalizeContent normaliza el contenido para comparación
func normalizeContent(content string) string {
	// Convertir a minúsculas
	content = strings.ToLower(content)
	// Eliminar caracteres especiales pero mantener espacios
	re := regexp.MustCompile(`[^a-z0-9\s]`)
	content = re.ReplaceAllString(content, "")
	// Normalizar espacios en blanco
	content = strings.Join(strings.Fields(content), " ")
	return content
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
