package converter

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Espacios de nombres OOXML usados por el parser.
const (
	nsWord        = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsWordStrict  = "http://purl.oclc.org/ooxml/wordprocessingml/main"
	nsDrawingML   = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsWordDrawing = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsVML         = "urn:schemas-microsoft-com:vml"
	nsRel         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// isWordNamespace indica si un espacio de nombres corresponde a
// WordprocessingML.
//
// Un prefijo sin resolver (Go deja el prefijo en Name.Space cuando el
// namespace no está declarado) no contiene "://": se asume Word, porque es el
// caso habitual en DOCX reales. Los namespaces de otros formatos (DrawingML,
// VML) sí son URIs y quedan fuera.
func isWordNamespace(space string) bool {
	switch space {
	case nsWord, nsWordStrict, "":
		return true
	}
	return !strings.Contains(space, "://")
}

// inlineSpan es un fragmento de texto con el formato que tenía en el
// documento original.
type inlineSpan struct {
	text      string
	bold      bool
	italic    bool
	strike    bool
	underline bool
	// raw indica que el texto se emite tal cual, sin marcas de énfasis.
	// Se usa para las imágenes, que ya son Markdown.
	raw bool
}

// docxParagraph es un párrafo del cuerpo del documento.
type docxParagraph struct {
	// style es el nombre del estilo de párrafo (w:pStyle).
	style string
	// numID identifica la lista a la que pertenece el párrafo. Vacío si no
	// es un elemento de lista.
	numID string
	// ilvl es el nivel dentro de la lista.
	ilvl int
	// text es el párrafo en Markdown, ya recortado y limpio.
	text string
	// spans guarda los fragmentos originales: hacen falta para renderizar el
	// párrafo como HTML (listas con etiquetas no decimales).
	spans []inlineSpan
	// plainStart es la longitud del texto plano cuando empezó el párrafo.
	plainStart int
}

// vMergeKind describe la participación de una celda en una unión vertical.
type vMergeKind int

const (
	vMergeNone vMergeKind = iota
	// vMergeRestart: la celda abre la unión y contiene el texto.
	vMergeRestart
	// vMergeContinue: la celda continúa la unión de arriba; no lleva texto.
	vMergeContinue
)

// docxCell es una celda de tabla.
type docxCell struct {
	// gridSpan es el número de columnas que ocupa la celda (w:gridSpan).
	gridSpan int
	// vMerge describe la unión vertical de la celda.
	vMerge vMergeKind
	// lines es el texto de cada párrafo de la celda.
	lines []string
}

// docxRow es una fila de tabla.
type docxRow struct {
	cells []*docxCell
}

// docxTable es una tabla.
type docxTable struct {
	rows []*docxRow
	// plainStart es la longitud del texto plano cuando empezó la tabla.
	plainStart int
}

// docxBlockKind distingue los tipos de bloque del cuerpo del documento.
type docxBlockKind int

const (
	blockParagraph docxBlockKind = iota
	blockTable
)

// docxBlock es un bloque del cuerpo: un párrafo o una tabla.
type docxBlock struct {
	kind  docxBlockKind
	para  *docxParagraph
	table *docxTable
	// plainStart es la longitud del texto plano cuando empezó el bloque.
	plainStart int
}

// docxDocument es el resultado de parsear word/document.xml.
type docxDocument struct {
	// blocks son los bloques del cuerpo, en orden.
	blocks []docxBlock
	// plain es el texto plano del documento, para la verificación de
	// integridad. Es un slice para poder recortar el contenido descartado.
	plain []byte
	// footnotes son los identificadores de las notas al pie referenciadas en
	// el cuerpo, en orden de aparición.
	footnotes []int
	// images son las imágenes referenciadas en el cuerpo, en orden de
	// aparición y sin repetir.
	images []docxImageRef
}

// docxImageRef es una imagen referenciada desde el cuerpo del documento.
type docxImageRef struct {
	// relID es el identificador de la relación en document.xml.rels.
	relID string
	// alt es el texto alternativo, si lo hay.
	alt string
	// path es la ruta del fichero extraído, relativa al Markdown. Vacía si
	// la imagen no se pudo extraer.
	path string
}

// docxParser recorre word/document.xml con un decoder de tokens y construye
// el modelo de bloques del documento.
//
// El decoder por tokens (y no Unmarshal) permite distinguir el cierre de cada
// elemento, que es lo que da la estructura de tablas y celdas.
type docxParser struct {
	dec *xml.Decoder
	doc *docxDocument

	// Formato en línea vigente (se reinicia en cada run).
	bold      bool
	italic    bool
	strike    bool
	underline bool
	// textStack apila el texto de cada <w:t> abierto.
	textStack []string

	// Párrafo en curso.
	para     *docxParagraph
	inPPr    bool
	inNumPr  bool
	inRun    bool
	inRunRPr bool

	// Pila de estructura de tablas.
	tables []*docxTable
	rows   []*docxRow
	cells  []*docxCell

	// pendingAlt guarda el texto alternativo del <wp:docPr>, que en el XML
	// precede al <a:blip> de la misma imagen.
	pendingAlt string

	// skipBeforeHeading descarta el contenido anterior al primer encabezado.
	skipBeforeHeading bool
}

// parseDocxDocument parsea el cuerpo del documento.
//
// Con skipBeforeHeading, el contenido anterior al primer encabezado se
// descarta: no se emiten ni bloques ni texto, de modo que la verificación de
// integridad mide el contenido que se conserva. Si el documento no tiene
// ningún encabezado, se conserva todo: descartar un documento entero sería
// perder la información.
func parseDocxDocument(content []byte, skipBeforeHeading bool) (*docxDocument, error) {
	p := &docxParser{
		dec:               xml.NewDecoder(bytes.NewReader(content)),
		doc:               &docxDocument{},
		skipBeforeHeading: skipBeforeHeading,
	}

	if err := p.parse(); err != nil {
		return nil, err
	}

	if skipBeforeHeading {
		p.doc.skipBeforeFirstHeading()
	}

	return p.doc, nil
}

// skipBeforeFirstHeading descarta los bloques anteriores al primer
// encabezado, y el texto plano que les corresponde.
func (d *docxDocument) skipBeforeFirstHeading() {
	for i, block := range d.blocks {
		if block.kind != blockParagraph || block.para == nil {
			continue
		}
		if paragraphPrefix(block.para.style) == "" {
			continue
		}

		if i == 0 {
			return
		}

		d.blocks = d.blocks[i:]
		// El texto plano de un bloque empieza en su plainStart: desde ahí
		// se conserva, y lo anterior se descarta.
		if block.plainStart <= len(d.plain) {
			d.plain = d.plain[block.plainStart:]
		}
		return
	}
}

// parse recorre todos los tokens del XML.
func (p *docxParser) parse() error {
	for {
		token, err := p.dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch t := token.(type) {
		case xml.StartElement:
			p.startElement(t)
		case xml.CharData:
			p.charData(t)
		case xml.EndElement:
			p.endElement(t)
		}
	}
}

// startElement atiende la apertura de un elemento.
func (p *docxParser) startElement(t xml.StartElement) {
	local := t.Name.Local
	space := t.Name.Space

	switch local {
	case "p":
		if isWordNamespace(space) {
			p.startParagraph()
		}
	case "tbl":
		if isWordNamespace(space) {
			p.tables = append(p.tables, &docxTable{plainStart: len(p.doc.plain)})
		}
	case "tr":
		if isWordNamespace(space) {
			p.rows = append(p.rows, &docxRow{})
		}
	case "tc":
		if isWordNamespace(space) {
			p.cells = append(p.cells, &docxCell{gridSpan: 1})
		}
	case "pPr":
		if isWordNamespace(space) {
			p.inPPr = true
		}
	case "numPr":
		if isWordNamespace(space) && p.inPPr {
			p.inNumPr = true
		}
	case "pStyle":
		if isWordNamespace(space) && p.inPPr && p.para != nil {
			p.para.style = attrValue(t, "val")
		}
	case "ilvl":
		if isWordNamespace(space) && p.inNumPr && p.para != nil {
			p.para.ilvl = atoiDefault(attrValue(t, "val"), 0)
		}
	case "numId":
		if isWordNamespace(space) && p.inNumPr && p.para != nil {
			p.para.numID = attrValue(t, "val")
		}
	case "r":
		if isWordNamespace(space) {
			p.inRun = true
		}
	case "rPr":
		// Solo cuentan las propiedades del run: las de <w:pPr><w:rPr> son
		// las de la marca de párrafo y no affects al texto.
		if isWordNamespace(space) && p.inRun {
			p.inRunRPr = true
		}
	case "b", "i", "strike", "u":
		if isWordNamespace(space) && p.inRunRPr {
			switch local {
			case "b":
				p.bold = attrBool(t, "val", true)
			case "i":
				p.italic = attrBool(t, "val", true)
			case "strike":
				p.strike = attrBool(t, "val", true)
			case "u":
				p.underline = attrUnderline(t)
			}
		}
	case "t":
		if isWordNamespace(space) {
			p.textStack = append(p.textStack, "")
		}
	case "tab":
		if isWordNamespace(space) {
			p.addSpan("  ")
		}
	case "br":
		if isWordNamespace(space) {
			p.addSpan("\n")
		}
	case "gridSpan":
		if isWordNamespace(space) {
			if cell := p.currentCell(); cell != nil {
				cell.gridSpan = atoiDefault(attrValue(t, "val"), 1)
			}
		}
	case "vMerge":
		if isWordNamespace(space) {
			if cell := p.currentCell(); cell != nil {
				if attrValue(t, "val") == "restart" {
					cell.vMerge = vMergeRestart
				} else {
					cell.vMerge = vMergeContinue
				}
			}
		}
	case "footnoteReference":
		if isWordNamespace(space) {
			if id, err := strconv.Atoi(attrValue(t, "id")); err == nil {
				p.doc.footnotes = append(p.doc.footnotes, id)
				p.addSpan(fmt.Sprintf("[^%d]", id))
			}
		}
	case "docPr":
		if space == nsWordDrawing {
			p.pendingAlt = firstNonEmpty(attrValue(t, "descr"), attrValue(t, "name"))
		}
	case "blip":
		if space == nsDrawingML {
			p.addImage(attrValueNS(t, nsRel, "embed"))
		}
	case "imagedata":
		if space == nsVML {
			p.addImage(attrValueNS(t, nsRel, "id"))
		}
	}
}

// endElement atiende el cierre de un elemento.
func (p *docxParser) endElement(t xml.EndElement) {
	local := t.Name.Local
	space := t.Name.Space

	switch local {
	case "p":
		if isWordNamespace(space) {
			p.endParagraph()
		}
	case "tbl":
		if isWordNamespace(space) {
			p.endTable()
		}
	case "tr":
		if isWordNamespace(space) {
			p.endRow()
		}
	case "tc":
		if isWordNamespace(space) {
			p.endCell()
		}
	case "pPr":
		if isWordNamespace(space) {
			p.inPPr = false
			p.inNumPr = false
		}
	case "numPr":
		if isWordNamespace(space) {
			p.inNumPr = false
		}
	case "r":
		if isWordNamespace(space) {
			p.inRun = false
			p.inRunRPr = false
			p.bold, p.italic, p.strike, p.underline = false, false, false, false
		}
	case "rPr":
		if isWordNamespace(space) {
			p.inRunRPr = false
		}
	case "t":
		if isWordNamespace(space) {
			p.endText()
		}
	}
}

// charData acumula el texto del elemento abierto más reciente.
func (p *docxParser) charData(t xml.CharData) {
	if len(p.textStack) == 0 {
		return
	}
	p.textStack[len(p.textStack)-1] += string(t)
}

// startParagraph abre un párrafo.
func (p *docxParser) startParagraph() {
	p.para = &docxParagraph{plainStart: len(p.doc.plain)}
	p.bold, p.italic, p.strike, p.underline = false, false, false, false
}

// endParagraph cierra el párrafo y decide dónde va su texto: a una celda de
// tabla si está dentro de una, o al cuerpo del documento.
func (p *docxParser) endParagraph() {
	para := p.para
	p.para = nil
	p.bold, p.italic, p.strike, p.underline = false, false, false, false

	if para == nil {
		return
	}

	text := cleanFormatting(strings.TrimSpace(renderSpansMarkdown(para.spans)))
	if text == "" && para.numID == "" {
		// Párrafo vacío: se descarta. Si estaba numerado se conserva, para
		// que el contador de la lista siga como lo lleva Word.
		return
	}
	para.text = text

	if cell := p.currentCell(); cell != nil {
		if text == "" {
			return
		}
		cell.lines = append(cell.lines, text)
		return
	}

	p.doc.blocks = append(p.doc.blocks, docxBlock{
		kind:       blockParagraph,
		para:       para,
		plainStart: para.plainStart,
	})
}

// endText cierra un <w:t>: vuelca el texto al párrafo y al texto plano.
func (p *docxParser) endText() {
	if len(p.textStack) == 0 {
		return
	}

	text := p.textStack[len(p.textStack)-1]
	p.textStack = p.textStack[:len(p.textStack)-1]

	if text == "" {
		return
	}

	p.doc.plain = append(p.doc.plain, text...)
	p.doc.plain = append(p.doc.plain, ' ')
	p.addSpan(text)
}

// addSpan añade un fragmento al párrafo en curso, fusionándolo con el
// anterior si el formato es el mismo.
func (p *docxParser) addSpan(text string) {
	if text == "" || p.para == nil {
		return
	}

	if n := len(p.para.spans); n > 0 {
		last := &p.para.spans[n-1]
		if !last.raw && last.bold == p.bold && last.italic == p.italic &&
			last.strike == p.strike && last.underline == p.underline {
			last.text += text
			return
		}
	}

	p.para.spans = append(p.para.spans, inlineSpan{
		text:      text,
		bold:      p.bold,
		italic:    p.italic,
		strike:    p.strike,
		underline: p.underline,
	})
}

// addImage registra una imagen referenciada desde el cuerpo.
func (p *docxParser) addImage(relID string) {
	if relID == "" || p.para == nil {
		return
	}

	for _, ref := range p.doc.images {
		if ref.relID == relID {
			return
		}
	}

	ref := docxImageRef{relID: relID, alt: p.pendingAlt}
	p.doc.images = append(p.doc.images, ref)
	p.pendingAlt = ""

	// La imagen se inserta en el punto del párrafo en que aparece. La ruta
	// se resuelve más tarde, cuando se_extraen los ficheros.
	p.para.spans = append(p.para.spans, inlineSpan{
		text: "\x00image:" + relID,
		raw:  true,
	})
}

// endTable cierra una tabla. Si estaba dentro de una celda, se aplana a
// texto; si no, es un bloque del cuerpo.
func (p *docxParser) endTable() {
	if len(p.tables) == 0 {
		return
	}

	tbl := p.tables[len(p.tables)-1]
	p.tables = p.tables[:len(p.tables)-1]

	if cell := p.currentCell(); cell != nil {
		cell.lines = append(cell.lines, flattenTable(tbl)...)
		return
	}

	p.doc.blocks = append(p.doc.blocks, docxBlock{
		kind:       blockTable,
		table:      tbl,
		plainStart: tbl.plainStart,
	})
}

// endRow cierra una fila de la tabla en curso.
func (p *docxParser) endRow() {
	if len(p.rows) == 0 {
		return
	}

	row := p.rows[len(p.rows)-1]
	p.rows = p.rows[:len(p.rows)-1]

	if len(p.tables) == 0 {
		return
	}

	tbl := p.tables[len(p.tables)-1]
	tbl.rows = append(tbl.rows, row)
}

// endCell cierra una celda de la fila en curso.
func (p *docxParser) endCell() {
	if len(p.cells) == 0 {
		return
	}

	cell := p.cells[len(p.cells)-1]
	p.cells = p.cells[:len(p.cells)-1]

	if len(p.rows) == 0 {
		return
	}

	row := p.rows[len(p.rows)-1]
	row.cells = append(row.cells, cell)
}

// currentCell devuelve la celda abierta, si la hay.
func (p *docxParser) currentCell() *docxCell {
	if len(p.cells) == 0 {
		return nil
	}
	return p.cells[len(p.cells)-1]
}

// flattenTable convierte una tabla anidada en líneas de texto.
func flattenTable(tbl *docxTable) []string {
	var lines []string
	for _, row := range tbl.rows {
		cells := make([]string, 0, len(row.cells))
		for _, cell := range row.cells {
			cells = append(cells, strings.Join(cell.lines, " "))
		}
		lines = append(lines, strings.Join(cells, " | "))
	}
	return lines
}

// renderSpansMarkdown convierte los fragmentos de un párrafo a Markdown.
//
// Los espacios que rodean al texto del fragmento quedan fuera de las marcas
// de énfasis: "**texto **" no es énfasis válido en Markdown, "**texto** " sí.
func renderSpansMarkdown(spans []inlineSpan) string {
	var sb strings.Builder
	for _, s := range spans {
		text := s.text
		if text == "" {
			continue
		}
		if s.raw {
			sb.WriteString(text)
			continue
		}

		lead := text[:len(text)-len(strings.TrimLeft(text, " \t\n"))]
		core := strings.TrimSpace(text)
		if core == "" {
			sb.WriteString(text)
			continue
		}
		trail := text[len(lead)+len(core):]

		if s.bold {
			core = "**" + core + "**"
		}
		if s.italic {
			core = "*" + core + "*"
		}
		if s.strike {
			core = "~~" + core + "~~"
		}
		if s.underline {
			// El subrayado no existe en Markdown: se emite como HTML en
			// línea, por fuera del énfasis, para que las marcas de Markdown
			// de dentro sigan interpretándose.
			core = "<u>" + core + "</u>"
		}

		sb.WriteString(lead)
		sb.WriteString(core)
		sb.WriteString(trail)
	}
	return sb.String()
}

// renderSpansHTML convierte los fragmentos de un párrafo a HTML en línea. Se
// usa dentro de bloques <ol>, donde Markdown no se procesa.
func renderSpansHTML(spans []inlineSpan) string {
	var sb strings.Builder
	for _, s := range spans {
		text := s.text
		if text == "" {
			continue
		}
		if s.raw {
			sb.WriteString(text)
			continue
		}
		if s.bold {
			text = "<strong>" + text + "</strong>"
		}
		if s.italic {
			text = "<em>" + text + "</em>"
		}
		if s.strike {
			text = "<del>" + text + "</del>"
		}
		if s.underline {
			text = "<u>" + text + "</u>"
		}
		sb.WriteString(text)
	}
	return sb.String()
}

// attrValue devuelve el valor del primer atributo con ese nombre local.
func attrValue(t xml.StartElement, name string) string {
	for _, attr := range t.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

// attrValueNS devuelve el valor del atributo con ese nombre local en el
// espacio de nombres indicado.
func attrValueNS(t xml.StartElement, space, name string) string {
	for _, attr := range t.Attr {
		if attr.Name.Local == name && attr.Name.Space == space {
			return attr.Value
		}
	}
	return ""
}

// attrBool interpreta el atributo de activación de una propiedad de run.
// Sin atributo, la propiedad está activada; "0", "false" y "off" la
// desactivan.
func attrBool(t xml.StartElement, name string, def bool) bool {
	value := attrValue(t, name)
	if value == "" {
		return def
	}
	switch strings.ToLower(value) {
	case "0", "false", "off":
		return false
	}
	return true
}

// attrUnderline interpreta el valor de <w:u>. El subrayado no tiene
// equivalente en Markdown: se emite como HTML en línea.
func attrUnderline(t xml.StartElement) bool {
	value := strings.ToLower(strings.TrimSpace(attrValue(t, "val")))
	switch value {
	case "":
		// Sin valor: subrayado simple.
		return true
	case "none", "0", "false", "off":
		return false
	}
	return true
}

// atoiDefault convierte a entero con un valor por defecto.
func atoiDefault(value string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return def
	}
	return n
}

// firstNonEmpty devuelve el primer argumento no vacío.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
