package converter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test basico que verifica que ConvertFile crea un .md con contenido
func TestConvertFile_CreatesMarkdownWithContent(t *testing.T) {
	// Ruta del archivo de prueba (usar el dossier `word/` del repo)
	input := filepath.Join("..", "..", "word", "INSTRUMENTO DE DETERMINACIÓN DEL PRESUPUESTO PUNTOS WIFI (01-12-25).docx")
	// En muchos entornos de test la ruta relativa al módulo funciona también
	if _, err := os.Stat("word/INSTRUMENTO DE DETERMINACIÓN DEL PRESUPUESTO PUNTOS WIFI (01-12-25).docx"); err == nil {
		input = "word/INSTRUMENTO DE DETERMINACIÓN DEL PRESUPUESTO PUNTOS WIFI (01-12-25).docx"
	}

	if _, err := os.Stat(input); os.IsNotExist(err) {
		t.Skipf("archivo de prueba no encontrado, se omite: %s", input)
	}

	t.Logf("Archivo de entrada: %s", input)

	tmp := t.TempDir()
	out := filepath.Join(tmp, "test_output.md")

	conv := NewConverter()
	if err := conv.ConvertFile(input, out); err != nil {
		t.Fatalf("ConvertFile falló: %v", err)
	}
	t.Logf("Archivo generado: %s", out)

	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("archivo de salida no creado: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("archivo de salida vacío")
	}
	t.Logf("Tamaño de salida: %d bytes", info.Size())

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("error leyendo archivo de salida: %v", err)
	}
	if len(data) < 20 {
		t.Fatalf("contenido inesperadamente corto: %d bytes", len(data))
	}

	// Mostrar previsualización breve del contenido para debugging
	preview := string(data)
	if len(preview) > 200 {
		preview = preview[:200]
	}
	t.Logf("Preview (primeros %d chars):\n%s", len(preview), preview)

	// Buscar una cadena conocida del documento de prueba para mayor seguridad
	if !strings.Contains(string(data), "GOBIERNO AUTONOMO") {
		t.Fatalf("el contenido convertido no contiene la cadena esperada")
	}
}
