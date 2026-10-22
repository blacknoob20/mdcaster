package integration

import (
	"path/filepath"
	"testing"

	"mdcaster/internal/converter"
	"mdcaster/internal/converter/docx"
	"mdcaster/tests/fixtures"
)

// TestDocx_SkipBeforeHeadingKeepsIntegrity comprueba que el descarte también
// se aplica al texto de la integridad: la mide sobre lo que se conserva.
//
// Es un test de integración: necesita la métrica, que vive en el paquete raíz.
func TestDocx_SkipBeforeHeadingKeepsIntegrity(t *testing.T) {
	body := `<w:p><w:r><w:t>PORTADA</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Título</w:t></w:r></w:p>
<w:p><w:r><w:t>Cuerpo</w:t></w:r></w:p>`

	parts := fixtures.DocWithBody(body)
	input, err := fixtures.BuildDocx(t.TempDir(), parts)
	if err != nil {
		t.Fatalf("no se pudo crear el DOCX de prueba: %v", err)
	}
	output := filepath.Join(t.TempDir(), "salida.md")

	conv := converter.NewConverter()
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

	var _ converter.TextExtractor = processor
	var _ converter.SkipBeforeHeadingSetter = processor

	if processor.ExtractedText() != "" {
		t.Errorf("un processor recién creado ya tiene texto: %q", processor.ExtractedText())
	}
}
