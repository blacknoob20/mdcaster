package docx

import (
	"encoding/xml"
	"regexp"
	"strings"
)

// coreProperties es la estructura de docProps/core.xml.
type coreProperties struct {
	XMLName  xml.Name `xml:"coreProperties"`
	Creator  string   `xml:"creator"`
	Created  string   `xml:"created"`
	Modified string   `xml:"modified"`
}

// docxCoreProps son los metadatos del documento usados en el frontmatter.
type docxCoreProps struct {
	// Author es el autor del documento (dc:creator).
	Author string
	// Date es la fecha de creación (dcterms:created).
	Date string
	// Updated es la fecha de la última modificación (dcterms:modified).
	Updated string
}

// isoDateAre detecta una fecha ISO (AAAA-MM-DD) al principio de una cadena.
var isoDateAre = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// parseDocxCoreProps lee docProps/core.xml. Si el contenido está vacío o no
// se puede leer, devuelve metadatos vacíos: no hay frontmatter.
func parseDocxCoreProps(content []byte) docxCoreProps {
	var props docxCoreProps

	if len(content) == 0 {
		return props
	}

	var doc coreProperties
	if err := xml.Unmarshal(content, &doc); err != nil {
		return props
	}

	props.Author = strings.TrimSpace(doc.Creator)
	props.Date = formatCoreDate(doc.Created)
	props.Updated = formatCoreDate(doc.Modified)

	return props
}

// formatCoreDate reduce una marca de tiempo W3CDTF a la fecha.
func formatCoreDate(value string) string {
	value = strings.TrimSpace(value)
	if match := isoDateAre.FindString(value); match != "" {
		return match
	}
	return ""
}

// isEmpty indica si no hay metadatos que emitir.
func (p docxCoreProps) isEmpty() bool {
	return p.Author == "" && p.Date == "" && p.Updated == ""
}

// renderFrontmatter genera el bloque YAML del frontmatter. Devuelve la
// cadena vacía si no hay metadatos.
func (p docxCoreProps) renderFrontmatter() string {
	if p.isEmpty() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	if p.Author != "" {
		sb.WriteString("author: " + yamlString(p.Author) + "\n")
	}
	if p.Date != "" {
		sb.WriteString("date: " + yamlString(p.Date) + "\n")
	}
	if p.Updated != "" {
		sb.WriteString("updated: " + yamlString(p.Updated) + "\n")
	}
	sb.WriteString("---\n")

	return sb.String()
}

// yamlString entrecomilla un valor para YAML y escapa lo que hace falta.
func yamlString(value string) string {
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", " ")

	return "\"" + escaped + "\""
}
