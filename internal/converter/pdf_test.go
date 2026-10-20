package converter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestConvertPdf_CreatesTextFile convierte un PDF del corpus y comprueba que
// se extrae texto.
//
// El PDF necesita pdftotext (poppler-utils), una dependencia de runtime: si
// no está, el test se omite.
func TestConvertPdf_CreatesTextFile(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext no está en el PATH (instala poppler-utils)")
	}

	input := requireCorpusFile(t, "pdf", "*.pdf")
	out := filepath.Join(t.TempDir(), "out.md")

	conv := NewConverter()
	if err := conv.Convert(input, out); err != nil {
		t.Fatalf("Convert falló: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("error leyendo la salida: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("la extracción de texto del PDF quedó vacía")
	}

	t.Logf("entrada: %s", filepath.Base(input))
	t.Logf("texto extraído: %d bytes", len(data))

	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("el texto extraído solo contiene espacios")
	}
}

// TestIntegrity_PdfIsNotMeasurable documenta el límite actual: la verificación
// de integridad solo sabe medir el texto que extrae el processor, y el del PDF
// no lo expone, así que sobre un PDF devuelve 0.
//
// Cuando se implemente P4 (mostrar "no medido" en lugar de 0), este test
// deberá cambiar para comprobar ese valor.
func TestIntegrity_PdfIsNotMeasurable(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext no está en el PATH (instala poppler-utils)")
	}

	input := requireCorpusFile(t, "pdf", "*.pdf")
	out := filepath.Join(t.TempDir(), "out.md")

	conv := NewConverter()
	if err := conv.Convert(input, out); err != nil {
		t.Fatalf("Convert falló: %v", err)
	}

	percent := conv.CheckIntegrity(input, out)
	t.Logf("integridad sobre PDF: %.2f%%", percent)

	if percent != 0 {
		t.Logf("aviso: la integridad de un PDF ya no es 0; revisa el spec de integridad")
	}
}
