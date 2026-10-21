package pdf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestProcess_MissingFile comprueba que un fichero que no existe produce un
// error, en vez de un Markdown vacío.
func TestProcess_MissingFile(t *testing.T) {
	processor := NewProcessor()

	input := filepath.Join(t.TempDir(), "no-existe.pdf")
	output := filepath.Join(t.TempDir(), "salida.md")

	err := processor.Process(input, output)
	if err == nil {
		t.Fatal("se esperaba un error para un fichero inexistente")
	}
}

// TestProcess_NotAPdf comprueba que un fichero que no es un PDF produce un
// error que nombra la dependencia externa, para que el usuario sepa qué
// instalar.
func TestProcess_NotAPdf(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext no está en el PATH (instala poppler-utils)")
	}

	input := filepath.Join(t.TempDir(), "falso.pdf")
	if err := os.WriteFile(input, []byte("esto no es un PDF"), 0644); err != nil {
		t.Fatalf("no se pudo crear el fichero de prueba: %v", err)
	}

	processor := NewProcessor()
	err := processor.Process(input, filepath.Join(t.TempDir(), "salida.md"))
	if err == nil {
		t.Fatal("se esperaba un error para un fichero que no es un PDF")
	}
	if !strings.Contains(err.Error(), "pdftotext") {
		t.Errorf("el error no nombra la dependencia externa: %v", err)
	}
}
