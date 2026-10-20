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

// Partes del paquete OOXML que se leen.
const (
	partDocument  = "word/document.xml"
	partRels      = "word/_rels/document.xml.rels"
	partNumbering = "word/numbering.xml"
	partFootnotes = "word/footnotes.xml"
	partCoreProps = "docProps/core.xml"
)

// DocxProcessor procesa archivos DOCX
type DocxProcessor struct {
	// relationships son las relaciones del documento, indexadas por Id.
	relationships map[string]docxRelationship
	// Texto plano del último documento procesado. Permite verificar la
	// integridad del contenido sin volver a parsear el DOCX.
	lastText string
	// skipBeforeHeading descarta el contenido anterior al primer encabezado.
	skipBeforeHeading bool
}

// docxRelationship es una relación del documento (document.xml.rels).
type docxRelationship struct {
	ID         string
	Type       string
	Target     string
	TargetMode string
}

// NewDocxProcessor crea una nueva instancia del procesador DOCX
func NewDocxProcessor() *DocxProcessor {
	return &DocxProcessor{
		relationships: make(map[string]docxRelationship),
	}
}

// Process convierte un DOCX a Markdown
func (dp *DocxProcessor) Process(inputPath, outputPath string) error {
	// Reiniciar el estado para cada archivo
	dp.relationships = make(map[string]docxRelationship)
	dp.lastText = ""

	// Abrir el archivo DOCX (que es un ZIP)
	reader, err := zip.OpenReader(inputPath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo DOCX: %w", err)
	}
	defer reader.Close()

	parts := make(map[string][]byte, 5)
	for _, file := range reader.File {
		switch file.Name {
		case partDocument, partRels, partNumbering, partFootnotes, partCoreProps:
			data, err := dp.readZipFile(file)
			if err != nil {
				return fmt.Errorf("error leyendo %s: %w", file.Name, err)
			}
			parts[file.Name] = data
		}
	}

	if parts[partDocument] == nil {
		return fmt.Errorf("no se encontró document.xml en el archivo DOCX")
	}

	// Parsear las relaciones si existen
	if rels := parts[partRels]; rels != nil {
		dp.parseRelationships(rels)
	}

	// Numeración, notas al pie y metadatos: todos opcionales
	numbering := parseDocxNumbering(parts[partNumbering])
	footnotes := parseDocxFootnotes(parts[partFootnotes])
	core := parseDocxCoreProps(parts[partCoreProps])

	// Parsear el documento
	doc, err := parseDocxDocument(parts[partDocument], dp.skipBeforeHeading)
	if err != nil {
		return fmt.Errorf("error parseando documento: %w", err)
	}

	// Extraer las imágenes referenciadas por el cuerpo
	images, err := extractDocxImages(&reader.Reader, doc.images, dp.relationships, outputPath)
	if err != nil {
		return err
	}
	resolveDocxImages(doc, images)

	// Renderizar el Markdown
	renderer := newDocxRenderer(doc, numbering)
	markdown := cleanMarkdown(renderer.render(doc))

	// Frontmatter, sólo si hay metadatos
	var sb strings.Builder
	if frontmatter := core.renderFrontmatter(); frontmatter != "" {
		sb.WriteString(frontmatter)
		sb.WriteString("\n")
	}
	sb.WriteString(markdown)

	// Definiciones de las notas al pie
	if defs := renderFootnotes(doc.footnotes, footnotes); defs != "" {
		sb.WriteString("\n")
		sb.WriteString(defs)
	}

	// El texto plano incluye el de las notas al pie: es contenido del
	// documento y cuenta para la integridad. Cada nota cuenta una sola vez,
	// aunque se referencie varias veces.
	plain := string(doc.plain)
	seenNotes := make(map[int]bool, len(doc.footnotes))
	for _, id := range doc.footnotes {
		if seenNotes[id] {
			continue
		}
		seenNotes[id] = true
		if text, ok := footnotes[id]; ok && text != "" {
			plain += text + " "
		}
	}
	dp.lastText = plain

	// Escribir el archivo Markdown
	if err := os.WriteFile(outputPath, []byte(strings.TrimSpace(sb.String())+"\n"), 0644); err != nil {
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
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr"`
}

// parseRelationships parsea el archivo de relaciones
func (dp *DocxProcessor) parseRelationships(content []byte) {
	var rels Relationships
	if err := xml.Unmarshal(content, &rels); err != nil {
		return
	}

	for _, rel := range rels.Relationships {
		dp.relationships[rel.ID] = docxRelationship{
			ID:         rel.ID,
			Type:       rel.Type,
			Target:     rel.Target,
			TargetMode: rel.TargetMode,
		}
	}
}

// SetSkipBeforeHeading activa o desactiva el descarte del contenido anterior
// al primer encabezado.
func (dp *DocxProcessor) SetSkipBeforeHeading(enabled bool) {
	dp.skipBeforeHeading = enabled
}

// ExtractedText devuelve el texto plano del último documento procesado.
func (dp *DocxProcessor) ExtractedText() string {
	return dp.lastText
}

// emptyBoldAre detecta marcas de negrita vacías. Se compila una sola vez:
// cleanFormatting se llama por cada párrafo del documento.
var emptyBoldAre = regexp.MustCompile(`\*\*\s*\*\*`)

// cleanFormatting limpia el formato duplicado
func cleanFormatting(content string) string {
	// Limpiar ** vacíos
	content = emptyBoldAre.ReplaceAllString(content, "")

	return content
}

// cleanMarkdown limpia el Markdown resultante
func cleanMarkdown(content string) string {
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
