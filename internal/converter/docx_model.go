package converter

import (
	"strings"
)

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

// paragraphPrefix devuelve el prefijo de encabezado que corresponde al
// estilo del párrafo, o la cadena vacía si no es un título.
//
// La detección es por subcadena del nombre del estilo, no por igualdad: los
// nombres dependen del idioma de Word ("Heading1", "Título1", "Ttulo1").
//
// Vive en el modelo y no en el render porque lo usan los dos: el parser, para
// saber si un párrafo es un encabezado, y el renderer, para emitirlo.
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
