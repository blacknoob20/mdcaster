package converter

import (
	"fmt"
	"regexp"
	"strings"
)

// listKind es la forma en que se renderiza un elemento de lista.
type listKind int

const (
	// kindBullet: viñeta Markdown ("- ").
	kindBullet listKind = iota
	// kindDecimal: lista ordenada Markdown ("1. ").
	kindDecimal
	// kindHTML: lista HTML (<ol type="a">), para etiquetas que Markdown
	// no puede representar.
	kindHTML
	// kindLiteral: etiqueta literal ("2.1."), sin estructura de lista.
	kindLiteral
)

// listMode es el modo de render de una lista completa.
type listMode int

const (
	modeMarkdown listMode = iota
	modeHTML
	modeLiteral
)

// listEntry es un elemento de lista ya resuelto.
type listEntry struct {
	// numID identifica la lista a la que pertenece el elemento.
	numID string
	// level es el nivel del elemento dentro de la lista.
	level int
	// kind es la forma de render del elemento.
	kind listKind
	// marker es el marcador Markdown ("- ", "1. ").
	marker string
	// label es la etiqueta literal ("2.1.").
	label string
	// labelText es la etiqueta resuelta tal cual la mostraría Word
	// ("2.1.", "1.", "a)").
	labelText string
	// htmlType es el atributo type del <ol>: a, A, i, I.
	htmlType string
	// counter es el valor numérico del contador del nivel.
	counter int
	// content es el texto del elemento en Markdown.
	content string
	// html es el texto del elemento en HTML en línea.
	html string
}

// mode devuelve el modo de render del elemento.
func (e *listEntry) mode() listMode {
	switch e.kind {
	case kindHTML:
		return modeHTML
	case kindLiteral:
		return modeLiteral
	default:
		return modeMarkdown
	}
}

// orderedMarkerAre detecta las etiquetas que CommonMark reconoce como
// marcador de lista ordenada: dígitos seguidos de "." o ")".
var orderedMarkerAre = regexp.MustCompile(`^\d{1,9}[.)]$`)

// makeEntry resuelve el marcador de un elemento de lista.
func (r *docxRenderer) makeEntry(para *docxParagraph) *listEntry {
	entry := &listEntry{
		numID:   para.numID,
		level:   para.ilvl,
		content: para.text,
		html:    strings.TrimSpace(renderSpansHTML(para.spans)),
	}

	label, ok := r.numbering.resolveLabel(para.numID, para.ilvl, r.state)
	if !ok {
		// Sin definición de numeración: viñeta, como antes.
		entry.kind = kindBullet
		entry.marker = "- "
		return entry
	}

	entry.labelText = label.text

	switch {
	case label.levelFmt == "bullet":
		entry.kind = kindBullet
		entry.marker = "- "
	case label.composite:
		entry.kind = kindLiteral
		entry.label = label.text
	case label.levelFmt == "decimal" || label.levelFmt == "decimalZero":
		if orderedMarkerAre.MatchString(label.text) {
			entry.kind = kindDecimal
			entry.marker = label.text + " "
		} else {
			entry.kind = kindLiteral
			entry.label = label.text
		}
	case label.levelFmt == "lowerLetter" || label.levelFmt == "upperLetter" ||
		label.levelFmt == "lowerRoman" || label.levelFmt == "upperRoman":
		entry.kind = kindHTML
		entry.htmlType = htmlListType(label.levelFmt)
		entry.counter = label.counter
	default:
		entry.kind = kindLiteral
		entry.label = label.text
	}

	return entry
}

// writeList escribe un grupo de elementos de lista consecutivos, separando
// los tramos de distinto modo.
func (r *docxRenderer) writeList(entries []*listEntry) {
	start := 0
	for i := 1; i <= len(entries); i++ {
		if i < len(entries) && entries[i].mode() == entries[start].mode() {
			continue
		}

		switch entries[start].mode() {
		case modeHTML:
			r.writeBlock(r.renderHTMLList(entries[start:i]))
		case modeLiteral:
			r.writeBlock(r.renderLiteralList(entries[start:i]))
		default:
			r.writeBlock(r.renderMarkdownList(entries[start:i]))
		}

		start = i
	}
}

// renderMarkdownList renderiza una lista con marcadores Markdown.
//
// La sangría de un nivel es la columna de contenido de su elemento padre, de
// modo que la jerarquía se respeta en Markdown. La columna se guarda por
// lista: un nivel puede aparecer en un tramo distinto del que tiene su
// padre y la sangría debe conservarse.
func (r *docxRenderer) renderMarkdownList(entries []*listEntry) []string {
	lines := make([]string, 0, len(entries))

	for _, entry := range entries {
		contentCols := r.cols[entry.numID]
		if contentCols == nil {
			contentCols = make([]int, 0, 8)
		}

		indent := 0
		if entry.level > 0 {
			if entry.level-1 < len(contentCols) {
				indent = contentCols[entry.level-1]
			} else {
				// El nivel padre no aparece en el cuerpo: se asume la
				// sangría habitual de dos espacios por nivel.
				indent = entry.level * 2
			}
		}

		marker := entry.marker
		if marker == "" {
			marker = "- "
		}

		lines = append(lines, strings.Repeat(" ", indent)+marker+entry.content)

		for len(contentCols) <= entry.level {
			contentCols = append(contentCols, 0)
		}
		contentCols[entry.level] = indent + len(marker)
		r.cols[entry.numID] = contentCols
	}

	return lines
}

// renderLiteralList renderiza una lista con etiquetas literales, que Markdown
// no puede representar como lista.
//
// La sangría es de tres espacios por nivel, como en las plantillas
// institucionales: la etiqueta compuesta ("2.1.") ya lleva la jerarquía.
func (r *docxRenderer) renderLiteralList(entries []*listEntry) []string {
	lines := make([]string, 0, len(entries))

	for _, entry := range entries {
		lines = append(lines, strings.Repeat("   ", entry.level)+entry.label+" "+entry.content)
	}

	return lines
}

// htmlNode es un nodo de la jerarquía de una lista HTML.
type htmlNode struct {
	entry    *listEntry
	children []*htmlNode
}

// renderHTMLList renderiza una lista con etiquetas HTML (<ol type="a">).
func (r *docxRenderer) renderHTMLList(entries []*listEntry) []string {
	var roots []*htmlNode
	var stack []*htmlNode

	for _, entry := range entries {
		node := &htmlNode{entry: entry}

		for len(stack) > 0 && stack[len(stack)-1].entry.level >= entry.level {
			stack = stack[:len(stack)-1]
		}

		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, node)
		} else {
			roots = append(roots, node)
		}

		stack = append(stack, node)
	}

	var sb strings.Builder
	r.renderHTMLNodes(roots, &sb)

	return strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
}

// renderHTMLNodes escribe recursivamente los nodos de una lista HTML.
func (r *docxRenderer) renderHTMLNodes(nodes []*htmlNode, sb *strings.Builder) {
	for _, node := range nodes {
		entry := node.entry

		if entry.kind == kindBullet {
			sb.WriteString("<ul>\n")
			sb.WriteString("<li>" + entry.html + "\n")
			r.renderHTMLNodes(node.children, sb)
			sb.WriteString("</li>\n")
			sb.WriteString("</ul>\n")
			continue
		}

		sb.WriteString(fmt.Sprintf("<ol type=\"%s\" start=\"%d\">\n", entry.htmlType, entry.counter))
		sb.WriteString("<li>" + entry.html + "\n")
		r.renderHTMLNodes(node.children, sb)
		sb.WriteString("</li>\n")
		sb.WriteString("</ol>\n")
	}
}

// htmlListType traduce el formato de numeración al atributo type de <ol>.
func htmlListType(format string) string {
	switch format {
	case "upperLetter":
		return "A"
	case "lowerRoman":
		return "i"
	case "upperRoman":
		return "I"
	default:
		return "a"
	}
}
