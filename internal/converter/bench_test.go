package converter

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BenchmarkConvertDocx mide la conversión de un DOCX del corpus.
func BenchmarkConvertDocx(b *testing.B) {
	input := requireCorpusFile(b, "word", "*.docx")
	out := filepath.Join(b.TempDir(), "out.md")

	conv := NewConverter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := conv.Convert(input, out); err != nil {
			b.Fatalf("Convert falló: %v", err)
		}
	}
}

// BenchmarkCheckIntegrityDocx mide solo la verificación de integridad, que es
// la parte más cara del pipeline actual.
//
// Mide la vía que usa la CLI: el texto que el processor ya extrajo al
// convertir, sin releer el original.
func BenchmarkCheckIntegrityDocx(b *testing.B) {
	input := requireCorpusFile(b, "word", "*.docx")
	out := filepath.Join(b.TempDir(), "out.md")

	conv := NewConverter()
	if err := conv.Convert(input, out); err != nil {
		b.Fatalf("Convert falló: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conv.CheckIntegrity(input, out)
	}
}

// BenchmarkConvertPdf mide la conversión de un PDF del corpus. Requiere
// pdftotext en el PATH.
func BenchmarkConvertPdf(b *testing.B) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		b.Skip("pdftotext no está en el PATH")
	}

	input := requireCorpusFile(b, "pdf", "*.pdf")
	out := filepath.Join(b.TempDir(), "out.md")

	conv := NewConverter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := conv.Convert(input, out); err != nil {
			b.Fatalf("Convert falló: %v", err)
		}
	}
}

// BenchmarkConvertBatch convierte un lote de ficheros del corpus, que es el
// escenario real de uso con la CLI.
func BenchmarkConvertBatch(b *testing.B) {
	files := corpusPaths(b, "word", "*.docx")
	if len(files) == 0 {
		b.Skip("sin corpus DOCX en word/")
	}

	dir := b.TempDir()
	out := filepath.Join(dir, "out.md")

	conv := NewConverter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, file := range files {
			if err := conv.Convert(file, out); err != nil {
				b.Fatalf("Convert falló: %v", err)
			}
		}
	}
}

// TestCorpusAvailable documenta si el corpus está presente. No falla: el
// corpus es local y no se versiona.
func TestCorpusAvailable(t *testing.T) {
	docx := corpusPaths(t, "word", "*.docx")
	pdf := corpusPaths(t, "pdf", "*.pdf")

	t.Logf("corpus DOCX: %d fichero(s)", len(docx))
	t.Logf("corpus PDF: %d fichero(s)", len(pdf))

	for _, f := range docx {
		info, err := os.Stat(f)
		if err == nil {
			t.Logf("  %s (%d bytes)", filepath.Base(f), info.Size())
		}
	}
}
