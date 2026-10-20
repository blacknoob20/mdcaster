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

// DocxProcessor procesa archivos DOCX
type DocxProcessor struct {
	relationships map[string]string
	// Texto plano del último documento procesado. Permite verificar la
	// integridad del contenido sin volver a parsear el DOCX.
	lastText string
}

// NewDocxProcessor crea una nueva instancia del procesador DOCX
func NewDocxProcessor() *DocxProcessor {
	return &DocxProcessor{
		relationships: make(map[string]string),
	}
}

// Process convierte un DOCX a Markdown
func (dp *DocxProcessor) Process(inputPath, outputPath string) error {
	// Reiniciar el estado para cada archivo
	dp.relationships = make(map[string]string)
	dp.lastText = ""

	// Abrir el archivo DOCX (que es un ZIP)
	reader, err := zip.OpenReader(inputPath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo DOCX: %w", err)
	}
	defer reader.Close()

	var documentContent []byte
	var relsContent []byte

	// Buscar el documento principal y las relaciones
	for _, file := range reader.File {
		switch file.Name {
		case "word/document.xml":
			documentContent, err = dp.readZipFile(file)
			if err != nil {
				return fmt.Errorf("error leyendo document.xml: %w", err)
			}
		case "word/_rels/document.xml.rels":
			relsContent, err = dp.readZipFile(file)
			if err != nil {
				return fmt.Errorf("error leyendo relationships: %w", err)
			}
		}
	}

	if documentContent == nil {
		return fmt.Errorf("no se encontró document.xml en el archivo DOCX")
	}

	// Parsear las relaciones si existen
	if relsContent != nil {
		dp.parseRelationships(relsContent)
	}

	// Parsear el documento
	markdown, plainText, err := dp.parseDocument(documentContent)
	if err != nil {
		return fmt.Errorf("error parseando documento: %w", err)
	}
	dp.lastText = plainText

	// Escribir el archivo Markdown
	if err := os.WriteFile(outputPath, []byte(markdown), 0644); err != nil {
		return fmt.Errorf("error escribiendo archivo Markdown: %w", err)
	}

	return nil
}

// readZipFile lee el contenido de un archivo dentro del ZIP
func (dp *DocxProcessor) readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// Relationships almacena las relaciones del documento
type Relationships struct {
	XMLName       xml.Name       `xml:"Relationships"`
	Relationships []Relationship `xml:"Relationship"`
}

// Relationship una relación individual
type Relationship struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

// parseRelationships parsea el archivo de relaciones
func (dp *DocxProcessor) parseRelationships(content []byte) {
	var rels Relationships
	if err := xml.Unmarshal(content, &rels); err != nil {
		return
	}

	for _, rel := range rels.Relationships {
		if strings.Contains(rel.Type, "hyperlink") {
			dp.relationships[rel.ID] = rel.Target
		}
	}
}

// parseDocument convierte el XML del documento a Markdown extrayendo TODOS los párrafos
// ignorando completamente las estructuras de tabla de Word
//
// Devuelve el Markdown y el texto plano del documento. El texto plano se usa
// para verificar la integridad del contenido sin parsear el DOCX otra vez.
func (dp *DocxProcessor) parseDocument(content []byte) (string, string, error) {
	var markdown strings.Builder
	var plainText strings.Builder

	// Usar un decoder para manejar namespaces
	decoder := xml.NewDecoder(strings.NewReader(string(content)))

	var currentParagraph strings.Builder
	var inParagraph bool
	var textStack []string
	var isBold, isItalic, isStrike bool
	var paragraphStyle string
	var isListItem bool
	var listLevel int

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", err
		}

		switch t := token.(type) {
		case xml.StartElement:
			localName := t.Name.Local

			switch localName {
			case "p":
				inParagraph = true
				currentParagraph.Reset()
				paragraphStyle = ""
				isListItem = false
				listLevel = 0
			case "pStyle":
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" {
						paragraphStyle = attr.Value
					}
				}
			case "numPr":
				isListItem = true
			case "ilvl":
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" {
						fmt.Sscanf(attr.Value, "%d", &listLevel)
					}
				}
			case "b":
				isBold = true
			case "i":
				isItalic = true
			case "strike":
				isStrike = true
			case "t":
				textStack = append(textStack, "")
			case "tab":
				if inParagraph {
					currentParagraph.WriteString("  ")
				}
			case "br":
				if inParagraph {
					currentParagraph.WriteString("\n")
				}
			}

		case xml.CharData:
			text := string(t)
			if len(textStack) > 0 {
				textStack[len(textStack)-1] += text
			}

		case xml.EndElement:
			localName := t.Name.Local

			switch localName {
			case "t":
				if len(textStack) > 0 {
					text := textStack[len(textStack)-1]
					textStack = textStack[:len(textStack)-1]

					// Texto plano, sin marcas de formato, para la
					// verificación de integridad.
					plainText.WriteString(text)
					plainText.WriteString(" ")

					// Aplicar formato
					if text != "" {
						if isBold {
							text = "**" + text + "**"
						}
						if isItalic {
							text = "*" + text + "*"
						}
						if isStrike {
							text = "~~" + text + "~~"
						}

						if inParagraph {
							currentParagraph.WriteString(text)
						}
					}
				}
			case "r":
				// Reset formato al final del run
				isBold = false
				isItalic = false
				isStrike = false
			case "p":
				inParagraph = false
				content := strings.TrimSpace(currentParagraph.String())

				// Limpiar formato duplicado
				content = cleanFormatting(content)

				if content != "" {
					// Aplicar estilo de párrafo
					line := dp.applyParagraphStyle(content, paragraphStyle, isListItem, listLevel)
					markdown.WriteString(line)
					markdown.WriteString("\n")
					if !isListItem {
						markdown.WriteString("\n")
					}
				}
			}
		}
	}

	result := dp.cleanMarkdown(markdown.String())
	return result, plainText.String(), nil
}

// ExtractedText devuelve el texto plano del último documento procesado.
func (dp *DocxProcessor) ExtractedText() string {
	return dp.lastText
}

// applyParagraphStyle aplica el estilo del párrafo
func (dp *DocxProcessor) applyParagraphStyle(content, style string, isList bool, listLevel int) string {
	styleLower := strings.ToLower(style)

	// Manejar encabezados
	switch {
	case strings.Contains(styleLower, "heading1") || strings.Contains(styleLower, "título1") || strings.Contains(styleLower, "ttulo1"):
		return "# " + content
	case strings.Contains(styleLower, "heading2") || strings.Contains(styleLower, "título2") || strings.Contains(styleLower, "ttulo2"):
		return "## " + content
	case strings.Contains(styleLower, "heading3") || strings.Contains(styleLower, "título3") || strings.Contains(styleLower, "ttulo3"):
		return "### " + content
	case strings.Contains(styleLower, "heading4") || strings.Contains(styleLower, "título4") || strings.Contains(styleLower, "ttulo4"):
		return "#### " + content
	case strings.Contains(styleLower, "heading5") || strings.Contains(styleLower, "título5") || strings.Contains(styleLower, "ttulo5"):
		return "##### " + content
	case strings.Contains(styleLower, "heading6") || strings.Contains(styleLower, "título6") || strings.Contains(styleLower, "ttulo6"):
		return "###### " + content
	case strings.Contains(styleLower, "title") || styleLower == "título" || styleLower == "ttulo":
		return "# " + content
	case strings.Contains(styleLower, "subtitle"):
		return "## " + content
	}

	// Manejar listas
	if isList {
		indent := strings.Repeat("  ", listLevel)
		return indent + "- " + content
	}

	return content
}

// emptyBoldAre detecta marcas de negrita vacías. Se compila una sola vez:
// cleanFormatting se llama por cada párrafo del documento.
var emptyBoldAre = regexp.MustCompile(`\*\*\s*\*\*`)

// cleanFormatting limpia el formato duplicado
func cleanFormatting(content string) string {
	// Limpiar formato duplicado como ****texto****
	content = strings.ReplaceAll(content, "****", "**")

	// Limpiar ** vacíos
	content = emptyBoldAre.ReplaceAllString(content, "")

	return content
}

// cleanMarkdown limpia el Markdown resultante
func (dp *DocxProcessor) cleanMarkdown(content string) string {
	// Eliminar múltiples líneas en blanco consecutivas
	re := regexp.MustCompile(`\n{3,}`)
	content = re.ReplaceAllString(content, "\n\n")

	// Eliminar espacios al final de las líneas
	lines := strings.Split(content, "\n")
	var cleaned []string
	for _, line := range lines {
		cleaned = append(cleaned, strings.TrimRight(line, " \t"))
	}

	content = strings.Join(cleaned, "\n")

	// Limpiar formato problemático
	content = strings.ReplaceAll(content, "****", "**")
	content = strings.ReplaceAll(content, "**\n**", "\n")

	return strings.TrimSpace(content)
}

// IsDocxFile verifica si un archivo es DOCX
func IsDocxFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".docx"
}

// GetDocxFiles obtiene todos los archivos DOCX de un directorio
func GetDocxFiles(dir string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("error leyendo directorio: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && IsDocxFile(entry.Name()) {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	return files, nil
}
