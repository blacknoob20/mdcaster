package converter

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"mdcaster/internal/converter/docx"
)

// Cabecera y pie de un word/document.xml mínimo, con los namespaces que
// usa el processor.
const testDocHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><w:body>`

const testDocFooter = `</w:body></w:document>`

// buildTestDocx crea un DOCX en un directorio temporal con las partes dadas.
//
// El módulo DOCX tiene sus propios tests, que construyen el DOCX igual. Este
// ayudante vive en el paquete raíz porque los tests de aquí necesitan
// comprobar el camino completo: fachada → factory → processor → integridad.
func buildTestDocx(t *testing.T, parts map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "prueba.docx")

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("no se pudo crear el DOCX de prueba: %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, content := range parts {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("no se pudo crear la entrada %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("no se pudo escribir la entrada %s: %v", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("no se pudo cerrar el ZIP: %v", err)
	}

	return path
}

// docWithBody crea las partes de un DOCX con el cuerpo indicado.
func docWithBody(body string) map[string]string {
	return map[string]string{
		"word/document.xml": testDocHeader + body + testDocFooter,
	}
}

// TestDocx_SkipBeforeHeadingKeepsIntegrity comprueba que el descarte también
// se aplica al texto de la integridad: la mide sobre lo que se conserva.
//
// Es un test de integración: necesita la métrica, que vive en el paquete raíz.
func TestDocx_SkipBeforeHeadingKeepsIntegrity(t *testing.T) {
	body := `<w:p><w:r><w:t>PORTADA</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Título</w:t></w:r></w:p>
<w:p><w:r><w:t>Cuerpo</w:t></w:r></w:p>`

	parts := docWithBody(body)
	input := buildTestDocx(t, parts)
	output := filepath.Join(t.TempDir(), "salida.md")

	conv := NewConverter()
	conv.SetSkipBeforeHeading(true)
	if err := conv.Convert(input, output); err != nil {
		t.Fatalf("la conversión falló: %v", err)
	}

	integrity := conv.CheckIntegrity(input, output)
	if integrity < 99 {
		t.Errorf("la integridad tras el descarte es %.2f %%: el texto descartado sigue contando", integrity)
	}
}

// TestDocxProcessorExposesText comprueba que el processor de DOCX sigue
// implementando las interfaces opcionales.
//
// Es una guarda de compilación: si el processor dejara de exponer su texto,
// la verificación de integridad de los DOCX caería a 0 sin avisar, porque el
// dispatcher solo reutiliza el texto si el processor lo implementa.
func TestDocxProcessorExposesText(t *testing.T) {
	processor := docx.NewProcessor()

	var _ TextExtractor = processor
	var _ SkipBeforeHeadingSetter = processor

	if processor.ExtractedText() != "" {
		t.Errorf("un processor recién creado ya tiene texto: %q", processor.ExtractedText())
	}
}
