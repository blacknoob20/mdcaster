package converter

import (
	"strings"
)

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
