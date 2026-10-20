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

// docxRenderer convierte el modelo de bloques en Markdown.
type docxRenderer struct {
	numbering *docxNumbering
	state     *numState
	// cols guarda, por lista, la columna de contenido de cada nivel: de ahí
	// sale la sangría de los elementos anidados.
	cols map[string][]int

	sb        strings.Builder
	needBlank bool
}

// newDocxRenderer crea el renderizador.
func newDocxRenderer(doc *docxDocument, numbering *docxNumbering) *docxRenderer {
	return &docxRenderer{
		numbering: numbering,
		state:     newNumState(),
		cols:      make(map[string][]int),
	}
}

// render convierte el documento en Markdown.
func (r *docxRenderer) render(doc *docxDocument) string {
	var pending []*listEntry

	flush := func() {
		if len(pending) == 0 {
			return
		}
		r.writeList(pending)
		pending = nil
	}

	for _, block := range doc.blocks {
		switch block.kind {
		case blockTable:
			flush()
			r.writeTable(block.table)
		case blockParagraph:
			para := block.para
			if para == nil {
				continue
			}
			// Un párrafo con estilo de título gana al marcador de lista,
			// pero conserva la etiqueta si estaba numerado.
			if prefix := paragraphPrefix(para.style); prefix != "" {
				if strings.TrimSpace(para.text) == "" {
					continue
				}
				flush()
				text := para.text
				if para.numID != "" {
					if entry := r.makeEntry(para); entry.labelText != "" && entry.kind != kindBullet {
						text = entry.labelText + " " + text
					}
				}
				r.writeBlock([]string{prefix + text})
				continue
			}
			if para.numID != "" {
				// Un elemento de lista vacío no se emite, pero sí consume su
				// número: es lo que hace Word con los párrafos numerados sin
				// texto.
				entry := r.makeEntry(para)
				if strings.TrimSpace(para.text) != "" {
					pending = append(pending, entry)
				}
				continue
			}
			if strings.TrimSpace(para.text) == "" {
				continue
			}
			flush()
			r.writeBlock([]string{para.text})
		}
	}

	flush()

	return strings.TrimSpace(r.sb.String())
}

// writeBlock escribe un bloque de líneas, separado del anterior por una
// línea en blanco.
func (r *docxRenderer) writeBlock(lines []string) {
	if len(lines) == 0 {
		return
	}

	if r.needBlank {
		r.sb.WriteString("\n")
	}

	for _, line := range lines {
		r.sb.WriteString(line)
		r.sb.WriteString("\n")
	}

	r.needBlank = true
}

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

// writeTable escribe una tabla en Markdown.
func (r *docxRenderer) writeTable(tbl *docxTable) {
	if tbl == nil {
		return
	}

	rows := buildTableRows(tbl)
	if len(rows) == 0 {
		return
	}

	columns := 0
	for _, row := range rows {
		if len(row) > columns {
			columns = len(row)
		}
	}
	if columns == 0 {
		return
	}

	lines := make([]string, 0, len(rows)+1)
	separator := make([]string, columns)
	for i := range separator {
		separator[i] = "---"
	}

	for i, row := range rows {
		cells := make([]string, columns)
		copy(cells, row)

		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
		if i == 0 {
			lines = append(lines, "| "+strings.Join(separator, " | ")+" |")
		}
	}

	r.writeBlock(lines)
}

// buildTableRows convierte la tabla en filas de celdas de texto.
//
// Las celdas que continúan una unión vertical no llevan texto: el texto
// está en la celda que abre la unión. Las celdas que ocupan varias columnas
// (gridSpan) dejan vacías las columnas restantes.
func buildTableRows(tbl *docxTable) [][]string {
	rows := make([][]string, 0, len(tbl.rows))

	for _, row := range tbl.rows {
		cells := make([]string, 0, len(row.cells))
		for _, cell := range row.cells {
			cells = append(cells, cellContent(cell))
			for i := 1; i < cell.gridSpan; i++ {
				cells = append(cells, "")
			}
		}
		rows = append(rows, cells)
	}

	return rows
}

// cellContent devuelve el texto de una celda, listo para una tabla
// Markdown.
func cellContent(cell *docxCell) string {
	if cell.vMerge == vMergeContinue {
		return ""
	}

	text := strings.Join(cell.lines, "<br>")

	return escapeTableCell(text)
}

// escapeTableCell escapa los caracteres que rompen una tabla Markdown.
func escapeTableCell(text string) string {
	return strings.ReplaceAll(text, "|", "\\|")
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

// paragraphPrefix devuelve el prefijo de encabezado que corresponde al
// estilo del párrafo, o la cadena vacía si no es un título.
//
// La detección es por subcadena del nombre del estilo, no por igualdad: los
// nombres dependen del idioma de Word ("Heading1", "Título1", "Ttulo1").
func paragraphPrefix(style string) string {
	styleLower := strings.ToLower(style)

	switch {
	case strings.Contains(styleLower, "heading1") || strings.Contains(styleLower, "título1") || strings.Contains(styleLower, "ttulo1"):
		return "# "
	case strings.Contains(styleLower, "heading2") || strings.Contains(styleLower, "título2") || strings.Contains(styleLower, "ttulo2"):
		return "## "
	case strings.Contains(styleLower, "heading3") || strings.Contains(styleLower, "título3") || strings.Contains(styleLower, "ttulo3"):
		return "### "
	case strings.Contains(styleLower, "heading4") || strings.Contains(styleLower, "título4") || strings.Contains(styleLower, "ttulo4"):
		return "#### "
	case strings.Contains(styleLower, "heading5") || strings.Contains(styleLower, "título5") || strings.Contains(styleLower, "ttulo5"):
		return "##### "
	case strings.Contains(styleLower, "heading6") || strings.Contains(styleLower, "título6") || strings.Contains(styleLower, "ttulo6"):
		return "###### "
	case strings.Contains(styleLower, "title") || styleLower == "título" || styleLower == "ttulo":
		return "# "
	case strings.Contains(styleLower, "subtitle"):
		return "## "
	}

	return ""
}
