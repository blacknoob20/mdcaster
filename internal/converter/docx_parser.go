package converter

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

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
//
// Delega en un método por familia de elementos: las propiedades del párrafo,
// el formato en línea, el texto, la estructura de tablas, las notas al pie y
// las imágenes.
func (p *docxParser) startElement(t xml.StartElement) {
	// Las imágenes viven en otros espacios de nombres: se atienden antes
	// del filtro de WordprocessingML.
	switch t.Name.Local {
	case "docPr", "blip", "imagedata":
		p.startImage(t)
		return
	}

	if !isWordNamespace(t.Name.Space) {
		return
	}

	switch t.Name.Local {
	case "p", "pPr", "numPr", "pStyle", "ilvl", "numId":
		p.startParagraphProps(t)
	case "r", "rPr", "b", "i", "strike", "u":
		p.startRunProps(t)
	case "t", "tab", "br":
		p.startText(t)
	case "tbl", "tr", "tc", "gridSpan", "vMerge":
		p.startTable(t)
	case "footnoteReference":
		p.startFootnote(t)
	}
}

// startParagraphProps atiende las propiedades del párrafo: su estilo y su
// pertenencia a una lista.
func (p *docxParser) startParagraphProps(t xml.StartElement) {
	switch t.Name.Local {
	case "p":
		p.startParagraph()
	case "pPr":
		p.inPPr = true
	case "numPr":
		if p.inPPr {
			p.inNumPr = true
		}
	case "pStyle":
		if p.inPPr && p.para != nil {
			p.para.style = attrValue(t, "val")
		}
	case "ilvl":
		if p.inNumPr && p.para != nil {
			p.para.ilvl = atoiDefault(attrValue(t, "val"), 0)
		}
	case "numId":
		if p.inNumPr && p.para != nil {
			p.para.numID = attrValue(t, "val")
		}
	}
}

// startRunProps atiende el formato en línea: negrita, cursiva, tachado y
// subrayado.
func (p *docxParser) startRunProps(t xml.StartElement) {
	switch t.Name.Local {
	case "r":
		p.inRun = true
	case "rPr":
		// Solo cuentan las propiedades del run: las de <w:pPr><w:rPr> son
		// las de la marca de párrafo y no afectan al texto.
		if p.inRun {
			p.inRunRPr = true
		}
	case "b":
		if p.inRunRPr {
			p.bold = attrBool(t, "val", true)
		}
	case "i":
		if p.inRunRPr {
			p.italic = attrBool(t, "val", true)
		}
	case "strike":
		if p.inRunRPr {
			p.strike = attrBool(t, "val", true)
		}
	case "u":
		if p.inRunRPr {
			p.underline = attrUnderline(t)
		}
	}
}

// startText atiende los elementos que aportan texto al párrafo en curso.
func (p *docxParser) startText(t xml.StartElement) {
	switch t.Name.Local {
	case "t":
		p.textStack = append(p.textStack, "")
	case "tab":
		p.addSpan("  ")
	case "br":
		p.addSpan("\n")
	}
}

// startTable atiende la estructura de tablas: apertura de tabla, fila y
// celda, y las propiedades de la celda.
func (p *docxParser) startTable(t xml.StartElement) {
	switch t.Name.Local {
	case "tbl":
		p.tables = append(p.tables, &docxTable{plainStart: len(p.doc.plain)})
	case "tr":
		p.rows = append(p.rows, &docxRow{})
	case "tc":
		p.cells = append(p.cells, &docxCell{gridSpan: 1})
	case "gridSpan":
		if cell := p.currentCell(); cell != nil {
			cell.gridSpan = atoiDefault(attrValue(t, "val"), 1)
		}
	case "vMerge":
		if cell := p.currentCell(); cell != nil {
			if attrValue(t, "val") == "restart" {
				cell.vMerge = vMergeRestart
			} else {
				cell.vMerge = vMergeContinue
			}
		}
	}
}

// startFootnote atiende la referencia a una nota al pie: la registra y
// escribe su marca en el párrafo.
func (p *docxParser) startFootnote(t xml.StartElement) {
	id, err := strconv.Atoi(attrValue(t, "id"))
	if err != nil {
		return
	}

	p.doc.footnotes = append(p.doc.footnotes, id)
	p.addSpan(fmt.Sprintf("[^%d]", id))
}

// startImage atiende los elementos que referencian una imagen.
//
// No son de WordprocessingML: cada uno va en su espacio de nombres.
func (p *docxParser) startImage(t xml.StartElement) {
	switch t.Name.Space {
	case nsWordDrawing:
		if t.Name.Local == "docPr" {
			p.pendingAlt = firstNonEmpty(attrValue(t, "descr"), attrValue(t, "name"))
		}
	case nsDrawingML:
		if t.Name.Local == "blip" {
			p.addImage(attrValueNS(t, nsRel, "embed"))
		}
	case nsVML:
		if t.Name.Local == "imagedata" {
			p.addImage(attrValueNS(t, nsRel, "id"))
		}
	}
}

// endElement atiende el cierre de un elemento.
func (p *docxParser) endElement(t xml.EndElement) {
	if !isWordNamespace(t.Name.Space) {
		return
	}

	switch t.Name.Local {
	case "p":
		p.endParagraph()
	case "tbl":
		p.endTable()
	case "tr":
		p.endRow()
	case "tc":
		p.endCell()
	case "pPr":
		p.inPPr = false
		p.inNumPr = false
	case "numPr":
		p.inNumPr = false
	case "r":
		p.inRun = false
		p.inRunRPr = false
		p.bold, p.italic, p.strike, p.underline = false, false, false, false
	case "rPr":
		p.inRunRPr = false
	case "t":
		p.endText()
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
	// se resuelve más tarde, cuando se extraen los ficheros.
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
