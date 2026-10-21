package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// readDocxTextIndependently extrae el texto del original por una vía
// independiente del parser: relee el ZIP y recoge el texto de los `<w:t>` y el
// de las notas al pie referenciadas.
//
// Es independiente a propósito: recorre el XML por su cuenta, en vez de usar
// el parser del módulo. Si compartiera su código, el test no probaría nada.
func readDocxTextIndependently(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	var documentContent, footnotesContent []byte
	for _, f := range zr.File {
		switch f.Name {
		case partDocument:
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			documentContent, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
		case partFootnotes:
			rc, err := f.Open()
			if err != nil {
				continue
			}
			footnotesContent, _ = io.ReadAll(rc)
			rc.Close()
		}
	}

	if documentContent == nil {
		return "", os.ErrNotExist
	}

	var sb strings.Builder
	var text strings.Builder
	var footnoteIDs []int
	inText := false

	decoder := xml.NewDecoder(bytes.NewReader(documentContent))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Local == "t" && isWordNamespace(t.Name.Space):
				inText = true
				text.Reset()
			case t.Name.Local == "footnoteReference" && isWordNamespace(t.Name.Space):
				if id, err := strconv.Atoi(attrValue(t, "id")); err == nil {
					footnoteIDs = append(footnoteIDs, id)
				}
			}
		case xml.EndElement:
			if t.Name.Local == "t" && isWordNamespace(t.Name.Space) {
				inText = false
				// Un `<w:t>` vacío no aporta nada: el parser tampoco
				// añade un espacio por él.
				if text.Len() > 0 {
					sb.WriteString(text.String())
					sb.WriteString(" ")
				}
				text.Reset()
			}
		case xml.CharData:
			if inText {
				text.WriteString(string(t))
			}
		}
	}

	// Las notas al pie referenciadas cuentan una sola vez, aunque se
	// referencien varias veces, y van detrás del texto del cuerpo.
	footnotes := parseDocxFootnotes(footnotesContent)
	seen := make(map[int]bool, len(footnoteIDs))
	for _, id := range footnoteIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if note, ok := footnotes[id]; ok && note != "" {
			sb.WriteString(note)
			sb.WriteString(" ")
		}
	}

	return sb.String(), nil
}

// docxTokens parte un texto en tokens comparables: minúsculas, solo
// alfanuméricos, separados por espacios.
func docxTokens(text string) []string {
	lower := strings.ToLower(text)

	var sb strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		default:
			sb.WriteRune(' ')
		}
	}

	return strings.Fields(sb.String())
}

// TestDocx_ExtractedTextMatchesIndependentRead comprueba que el texto que el
// processor expone no pierde nada: una lectura independiente del original da
// los mismos tokens.
//
// Es la garantía de que la verificación de integridad, que reutiliza ese
// texto, mide el documento entero.
func TestDocx_ExtractedTextMatchesIndependentRead(t *testing.T) {
	parts := docWithBody(`<w:p><w:r><w:t>Texto del cuerpo</w:t></w:r></w:p>
<w:p><w:r><w:t>Otro párrafo</w:t></w:r></w:p>`)

	input := buildTestDocx(t, parts)
	output := filepath.Join(t.TempDir(), "salida.md")

	processor := NewProcessor()
	if err := processor.Process(input, output); err != nil {
		t.Fatalf("la conversión falló: %v", err)
	}

	independent, err := readDocxTextIndependently(input)
	if err != nil {
		t.Fatalf("no se pudo leer el original por una vía independiente: %v", err)
	}

	got := docxTokens(processor.ExtractedText())
	want := docxTokens(independent)

	if len(got) != len(want) {
		t.Fatalf("el processor expone %d tokens y la lectura independiente %d:\nprocessor: %v\nindependiente: %v",
			len(got), len(want), got, want)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("el token %d difiere: %q frente a %q", i, got[i], want[i])
		}
	}
}
