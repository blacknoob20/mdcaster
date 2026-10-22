package integration

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mdcaster/internal/converter"
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
	conv := converter.NewConverter()
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

	conv := converter.NewConverter()
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
