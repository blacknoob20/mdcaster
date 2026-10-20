package converter

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

// docxFootnote es una nota al pie de word/footnotes.xml.
type docxFootnote struct {
	// id es el identificador de la nota (w:id).
	id int
	// text es el texto plano de la nota.
	text string
}

// parseDocxFootnotes lee word/footnotes.xml y devuelve las notas al pie por
// identificador.
//
// Se ignoran las notas estructurales de Word (separator y
// continuationSeparator): no son contenido.
func parseDocxFootnotes(content []byte) map[int]string {
	footnotes := make(map[int]string)

	if len(content) == 0 {
		return footnotes
	}

	decoder := xml.NewDecoder(bytes.NewReader(content))
	var current *docxFootnote

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
			case t.Name.Local == "footnote" && isWordNamespace(t.Name.Space):
				if kind := attrValue(t, "type"); kind != "" {
					current = nil
					continue
				}
				id, err := strconv.Atoi(strings.TrimSpace(attrValue(t, "id")))
				if err != nil || id <= 0 {
					current = nil
					continue
				}
				current = &docxFootnote{id: id}
			case t.Name.Local == "t" && isWordNamespace(t.Name.Space) && current != nil:
				text, err := decoder.Token()
				if err == nil {
					if data, ok := text.(xml.CharData); ok {
						current.text += string(data)
						current.text += " "
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "footnote" && isWordNamespace(t.Name.Space) && current != nil {
				footnotes[current.id] = strings.TrimSpace(current.text)
				current = nil
			}
		}
	}

	return footnotes
}

// renderFootnotes genera el bloque de definiciones de notas al pie.
//
// Solo se emiten las notas referenciadas en el cuerpo, y en el orden en que
// se referencian.
func renderFootnotes(ids []int, footnotes map[int]string) string {
	if len(ids) == 0 || len(footnotes) == 0 {
		return ""
	}

	var sb strings.Builder
	seen := make(map[int]bool, len(ids))

	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true

		text, ok := footnotes[id]
		if !ok || text == "" {
			continue
		}

		sb.WriteString("[^")
		sb.WriteString(strconv.Itoa(id))
		sb.WriteString("]: ")
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	return sb.String()
}
