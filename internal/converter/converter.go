package converter

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// CheckIntegrity verifica qué porcentaje del contenido original se conserva en
// el Markdown.
//
// Si el procesador del original ya extrajo el texto al convertir, se reutiliza
// y no hace falta parsear el documento por segunda vez. En caso contrario,
// recurre a leer el original desde disco.
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

// CheckIntegrity verifica qué porcentaje del contenido original se encuentra en el markdown
func CheckIntegrity(sourceFile, markdownFile string) float64 {
	// Leer el contenido original del DOCX
	originalContent, err := extractDocxContent(sourceFile)
	if err != nil {
		return 0
	}

	return CheckIntegrityAgainst(originalContent, markdownFile)
}

// CheckIntegrityAgainst verifica la integridad usando un texto original ya
// extraído. Permite reutilizar el texto que el procesador leyó al convertir y
// evitar un segundo parseo del documento.
func CheckIntegrityAgainst(originalContent, markdownFile string) float64 {
	// Leer el contenido del markdown generado
	markdownContent, err := os.ReadFile(markdownFile)
	if err != nil {
		return 0
	}

	// Normalizar espacios para comparación
	original := normalizeContent(originalContent)

	// Si no hay contenido original, no se puede verificar
	if len(original) == 0 {
		return 100
	}

	// Contar cuántas palabras del original se encuentran en el markdown.
	//
	// D10: la coincidencia es por token exacto, no por subcadena. Los tokens
	// del Markdown se indexan una sola vez, de modo que el coste es lineal
	// en el número de palabras y no el producto de ambos.
	originalWords := strings.Fields(original)
	if len(originalWords) == 0 {
		return 100
	}

	markdownTokens := make(map[string]struct{}, len(originalWords))
	for _, token := range strings.Fields(normalizeContent(string(markdownContent))) {
		markdownTokens[token] = struct{}{}
	}

	foundCount := 0
	for _, word := range originalWords {
		if _, found := markdownTokens[word]; found {
			foundCount++
		}
	}

	percent := float64(foundCount) / float64(len(originalWords)) * 100
	return percent
}

// extractDocxContent extrae todo el contenido de texto de un archivo DOCX
//
// Incluye el texto de las notas al pie referenciadas en el cuerpo, para que
// esta vía y la reutilizada por el processor den la misma cifra de
// integridad.
func extractDocxContent(docxPath string) (string, error) {
	reader, err := zip.OpenReader(docxPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var documentContent []byte
	var footnotesContent []byte

	// Buscar el documento principal y las notas al pie
	for _, file := range reader.File {
		switch file.Name {
		case "word/document.xml":
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			documentContent, _ = io.ReadAll(rc)
			rc.Close()
		case "word/footnotes.xml":
			rc, err := file.Open()
			if err != nil {
				continue
			}
			footnotesContent, _ = io.ReadAll(rc)
			rc.Close()
		}
	}

	if documentContent == nil {
		return "", fmt.Errorf("no se encontró document.xml")
	}

	// Extraer solo el texto del XML
	text, footnoteIDs := extractTextFromXML(string(documentContent))

	// Sumar el texto de las notas al pie referenciadas
	footnotes := parseDocxFootnotes(footnotesContent)
	seen := make(map[int]bool, len(footnoteIDs))
	for _, id := range footnoteIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if note, ok := footnotes[id]; ok && note != "" {
			text += note + " "
		}
	}

	return text, nil
}

// extractTextFromXML extrae el texto de los elementos <w:t> del XML y los
// identificadores de las notas al pie referenciadas
func extractTextFromXML(xmlContent string) (string, []int) {
	var text strings.Builder
	var footnoteIDs []int
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
			switch {
			case t.Name.Local == "t":
				// Leer el siguiente token que será el texto
				token, err := decoder.Token()
				if err == nil {
					if charData, ok := token.(xml.CharData); ok {
						text.Write(charData)
						text.WriteString(" ")
					}
				}
			case t.Name.Local == "footnoteReference":
				if id, err := strconv.Atoi(strings.TrimSpace(attrValue(t, "id"))); err == nil {
					footnoteIDs = append(footnoteIDs, id)
				}
			}
		}
	}

	return text.String(), footnoteIDs
}

// nonAlphanumericAre es la expresión regular de normalización del contenido.
// Se compila una sola vez: normalizeContent se llama por cada verificación
// de integridad.
var nonAlphanumericAre = regexp.MustCompile(`[^a-z0-9\s]`)

// normalizeContent normaliza el contenido para comparación
func normalizeContent(content string) string {
	// Convertir a minúsculas
	content = strings.ToLower(content)
	// Eliminar caracteres especiales pero mantener espacios
	content = nonAlphanumericAre.ReplaceAllString(content, "")
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
