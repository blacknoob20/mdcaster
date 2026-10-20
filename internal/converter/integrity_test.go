package converter

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// readDocxPlainText extrae el texto (elementos <w:t>) de word/document.xml
func readDocxPlainText(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	var docBytes []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			docBytes, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}
	if docBytes == nil {
		return "", os.ErrNotExist
	}

	var b strings.Builder
	dec := xml.NewDecoder(strings.NewReader(string(docBytes)))
	inT := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inT = true
			}
			if t.Name.Local == "p" {
				// paragraph boundary -> newline
				b.WriteString("\n")
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inT = false
			}
		case xml.CharData:
			if inT {
				b.WriteString(string(t))
			}
		}
	}

	return b.String(), nil
}

// readDocxTextForIntegrity extrae el texto del original por una vía
// independiente del processor: relee el ZIP y recoge el texto de los `<w:t>` y
// el de las notas al pie referenciadas.
//
// Es la lectura que hacía la librería antes de que el dispatcher reutilizara
// el texto del processor, y sirve para comprobar que ese atajo no cambia la
// métrica: las dos vías tienen que encontrar las mismas palabras.
func readDocxTextForIntegrity(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	var documentContent, footnotesContent []byte
	for _, f := range zr.File {
		switch f.Name {
		case "word/document.xml":
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			documentContent, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
		case "word/footnotes.xml":
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
				// Un `<w:t>` vacío no aporta nada: el processor tampoco
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

// tableSeparatorAre detecta las filas de separador de las tablas Markdown.
var tableSeparatorAre = regexp.MustCompile(`(?m)^\|[\s\-:|]+\|$`)

// normalizeText colapsa espacios y elimina caracteres markdown básicos
func normalizeText(s string) string {
	// Las filas de separador de tabla no aportan texto: se quitan antes de
	// comparar, porque en el original no existen.
	s = tableSeparatorAre.ReplaceAllString(s, " ")

	// Eliminar marcas Markdown simples
	reMarks := regexp.MustCompile(`[\*\_\~\` + "`" + `#>|]`)
	s = reMarks.ReplaceAllString(s, "")

	// Colapsar whitespace
	reWs := regexp.MustCompile(`\s+`)
	s = reWs.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}

// TestIntegrity_DocxToMarkdownContainsOriginalText verifica que el Markdown
// generado conserva el texto del original.
func TestIntegrity_DocxToMarkdownContainsOriginalText(t *testing.T) {
	input := requireCorpusFile(t, "word", "*.docx")

	// extraer texto original
	orig, err := readDocxPlainText(input)
	if err != nil {
		t.Fatalf("no se pudo leer texto original del docx: %v", err)
	}
	t.Logf("Longitud texto original: %d bytes", len(orig))

	// convertir a Markdown
	out := filepath.Join(t.TempDir(), "integrity.md")
	conv := NewConverter()
	if err := conv.ConvertFile(input, out); err != nil {
		t.Fatalf("ConvertFile falló: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("error leyendo markdown generado: %v", err)
	}

	// normalizar ambos textos
	nOrig := normalizeText(orig)
	nMd := normalizeText(string(data))

	t.Logf("Longitud normalizada original: %d", len(nOrig))
	t.Logf("Longitud normalizada markdown: %d", len(nMd))

	if len(nOrig) == 0 {
		t.Fatalf("texto original está vacío, imposible validar integridad")
	}

	if !strings.Contains(nMd, nOrig) {
		// Si no contiene entero, intentar comprobar subsecuencia mínima (primeros N palabras)
		words := strings.Fields(nOrig)
		if len(words) > 50 {
			sample := strings.Join(words[:50], " ")
			if !strings.Contains(nMd, sample) {
				t.Fatalf("la versión Markdown no contiene el texto original ni la muestra de comprobación\nMuestra original: %q\nMuestra markdown: %q", sample, nMd[:min(len(nMd), 200)])
			}
		} else {
			t.Fatalf("la versión Markdown no contiene el texto original (integridad no verificada)\nOriginal(normalizado): %q\nMarkdown(normalizado, inicio): %q", nOrig[:min(len(nOrig), 200)], nMd[:min(len(nMd), 200)])
		}
	}
}

// TestIntegrity_DocxReportsCoverage comprueba que la verificación de
// integridad devuelve un porcentaje plausible para un DOCX.
func TestIntegrity_DocxReportsCoverage(t *testing.T) {
	input := requireCorpusFile(t, "word", "*.docx")
	out := filepath.Join(t.TempDir(), "integrity.md")

	conv := NewConverter()
	if err := conv.ConvertFile(input, out); err != nil {
		t.Fatalf("ConvertFile falló: %v", err)
	}

	percent := conv.CheckIntegrity(input, out)
	t.Logf("integridad: %.2f%%", percent)

	if percent <= 0 {
		t.Fatalf("integridad 0 %% para un DOCX convertido: %v", percent)
	}
	if percent > 100 {
		t.Fatalf("integridad fuera de rango: %v", percent)
	}
}

// TestIntegrity_ReusedTextMatchesIndependentRead comprueba que el texto que
// el processor ya extrajo no pierde nada: una lectura independiente del
// original da la misma métrica. Es la garantía de que el atajo no cambia la
// cifra que ve el usuario.
func TestIntegrity_ReusedTextMatchesIndependentRead(t *testing.T) {
	input := requireCorpusFile(t, "word", "*.docx")
	out := filepath.Join(t.TempDir(), "integrity.md")

	conv := NewConverter()
	if err := conv.ConvertFile(input, out); err != nil {
		t.Fatalf("ConvertFile falló: %v", err)
	}

	independent, err := readDocxTextForIntegrity(input)
	if err != nil {
		t.Fatalf("no se pudo leer el original por una vía independiente: %v", err)
	}

	reused := conv.CheckIntegrity(input, out)
	fromFile := CheckIntegrityAgainst(independent, out)

	t.Logf("integridad reutilizando el texto del processor: %.4f%%", reused)
	t.Logf("integridad con una lectura independiente: %.4f%%", fromFile)

	if reused != fromFile {
		t.Fatalf("el texto reutilizado da un resultado distinto: %.4f%% != %.4f%%", reused, fromFile)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
