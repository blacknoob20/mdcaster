package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mdcaster/internal/converter"
)

// TestConvertFile_CreatesMarkdownWithContent convierte cada DOCX del corpus y
// comprueba que el Markdown resultante existe y tiene contenido.
func TestConvertFile_CreatesMarkdownWithContent(t *testing.T) {
	files := corpusPaths(t, "word", "*.docx")
	if len(files) == 0 {
		t.Skip("sin corpus DOCX en word/ (el corpus es local y no se versiona)")
	}

	for _, input := range files {
		t.Run(filepath.Base(input), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out.md")

			conv := converter.NewConverter()
			if err := conv.ConvertFile(input, out); err != nil {
				t.Fatalf("ConvertFile falló: %v", err)
			}

			info, err := os.Stat(out)
			if err != nil {
				t.Fatalf("archivo de salida no creado: %v", err)
			}
			if info.Size() == 0 {
				t.Fatal("archivo de salida vacío")
			}

			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("error leyendo archivo de salida: %v", err)
			}
			if len(data) < 20 {
				t.Fatalf("contenido inesperadamente corto: %d bytes", len(data))
			}

			t.Logf("entrada: %s (%d bytes)", filepath.Base(input), info.Size())
			t.Logf("salida: %d bytes", len(data))

			preview := string(data)
			if len(preview) > 200 {
				preview = preview[:200]
			}
			t.Logf("preview:\n%s", preview)

			// El Markdown debe contener texto, no solo marcas de formato.
			if strings.TrimSpace(stripMarkdown(string(data))) == "" {
				t.Fatal("la salida no contiene texto legible")
			}
		})
	}
}

// TestConvertFile_UnsupportedExtension comprueba que un formato no registrado
// produce un error claro en lugar de un fallo silencioso.
func TestConvertFile_UnsupportedExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archivo.rtf")
	if err := os.WriteFile(path, []byte("contenido"), 0644); err != nil {
		t.Fatalf("no se pudo crear el fichero de prueba: %v", err)
	}

	conv := converter.NewConverter()
	err := conv.ConvertFile(path, filepath.Join(t.TempDir(), "out.md"))
	if err == nil {
		t.Fatal("se esperaba un error para una extensión no soportada")
	}
	if !strings.Contains(err.Error(), "no hay procesador disponible") {
		t.Fatalf("mensaje de error inesperado: %v", err)
	}
}

// TestConvertFile_MissingInput comprueba que la entrada inexistente se
// rechaza antes de escribir nada.
func TestConvertFile_MissingInput(t *testing.T) {
	conv := converter.NewConverter()
	err := conv.ConvertFile(filepath.Join(t.TempDir(), "no-existe.docx"), filepath.Join(t.TempDir(), "out.md"))
	if err == nil {
		t.Fatal("se esperaba un error para una entrada inexistente")
	}
	if !strings.Contains(err.Error(), "archivo de entrada no encontrado") {
		t.Fatalf("mensaje de error inesperado: %v", err)
	}
}

// stripMarkdown quita las marcas de formato más habituales para comprobar que
// queda texto legible debajo.
func stripMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"#", "", "*", "", "~", "", "`", "", "-", "", " ", " ",
	)
	return replacer.Replace(s)
}
