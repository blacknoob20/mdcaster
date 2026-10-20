package converter

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cabecera y pie de un word/document.xml mínimo, con los namespaces que
// usa el processor.
const testDocHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><w:body>`

const testDocFooter = `</w:body></w:document>`

// buildTestDocx crea un DOCX en un directorio temporal con las partes dadas.
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

// convertDocx convierte un DOCX y devuelve el Markdown generado.
func convertDocx(t *testing.T, parts map[string]string) string {
	t.Helper()

	input := buildTestDocx(t, parts)
	output := filepath.Join(t.TempDir(), "salida.md")

	conv := NewConverter()
	if err := conv.Convert(input, output); err != nil {
		t.Fatalf("la conversión falló: %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("no se pudo leer el Markdown: %v", err)
	}

	return string(data)
}

// docWithBody crea las partes de un DOCX con el cuerpo indicado.
func docWithBody(body string) map[string]string {
	return map[string]string{
		"word/document.xml": testDocHeader + body + testDocFooter,
	}
}

// TestDocx_TableBecomesMarkdownTable comprueba que una tabla se convierte en
// una tabla Markdown, con la primera fila como cabecera.
func TestDocx_TableBecomesMarkdownTable(t *testing.T) {
	body := `<w:p><w:r><w:t>Antes de la tabla</w:t></w:r></w:p>
<w:tbl>
  <w:tblGrid><w:gridCol w:w="2000"/><w:gridCol w:w="3000"/></w:tblGrid>
  <w:tr><w:tc><w:p><w:r><w:t>Nombre</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Valor</w:t></w:r></w:p></w:tc></w:tr>
  <w:tr><w:tc><w:p><w:r><w:t>Fila</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>1</w:t></w:r></w:p></w:tc></w:tr>
</w:tbl>
<w:p><w:r><w:t>Después de la tabla</w:t></w:r></w:p>`

	md := convertDocx(t, docWithBody(body))

	want := []string{
		"| Nombre | Valor |",
		"| --- | --- |",
		"| Fila | 1 |",
	}
	for _, line := range want {
		if !strings.Contains(md, line) {
			t.Errorf("falta la línea %q en:\n%s", line, md)
		}
	}

	if !strings.Contains(md, "Antes de la tabla") || !strings.Contains(md, "Después de la tabla") {
		t.Errorf("el texto alrededor de la tabla se perdió:\n%s", md)
	}

	// El separador de tabla solo puede aparecer una vez.
	if n := strings.Count(md, "| --- | --- |"); n != 1 {
		t.Errorf("se esperaba un separador de tabla, hay %d:\n%s", n, md)
	}
}

// TestDocx_TableGridSpanAndVerticalMerge comprueba el tratamiento de las
// celdas combinadas: el texto se conserva una vez y las posiciones
// combinadas quedan vacías.
func TestDocx_TableGridSpanAndVerticalMerge(t *testing.T) {
	body := `<w:tbl>
  <w:tblGrid><w:gridCol w:w="2000"/><w:gridCol w:w="2000"/><w:gridCol w:w="2000"/></w:tblGrid>
  <w:tr>
    <w:tc><w:tcPr><w:gridSpan w:val="2"/></w:tcPr><w:p><w:r><w:t>Cabecera combinada</w:t></w:r></w:p></w:tc>
    <w:tc><w:p><w:r><w:t>Otra</w:t></w:r></w:p></w:tc>
  </w:tr>
  <w:tr>
    <w:tc><w:tcPr><w:vMerge w:val="restart"/></w:tcPr><w:p><w:r><w:t>Vertical</w:t></w:r></w:p></w:tc>
    <w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc>
    <w:tc><w:p><w:r><w:t>C</w:t></w:r></w:p></w:tc>
  </w:tr>
  <w:tr>
    <w:tc><w:tcPr><w:vMerge/></w:tcPr><w:p><w:r><w:t>no debe aparecer</w:t></w:r></w:p></w:tc>
    <w:tc><w:p><w:r><w:t>D</w:t></w:r></w:p></w:tc>
    <w:tc><w:p><w:r><w:t>E</w:t></w:r></w:p></w:tc>
  </w:tr>
</w:tbl>`

	md := convertDocx(t, docWithBody(body))

	if strings.Contains(md, "no debe aparecer") {
		t.Errorf("el texto de una celda que continúa la unión vertical se emissions:\n%s", md)
	}

	if n := strings.Count(md, "Vertical"); n != 1 {
		t.Errorf("el texto de la unión vertical debería aparecer una vez, aparece %d:\n%s", n, md)
	}

	if !strings.Contains(md, "| Cabecera combinada |  | Otra |") {
		t.Errorf("la celda combinada no dejó la columna vacía:\n%s", md)
	}
}

// TestDocx_NumberedListUsesRealFormat comprueba que una lista numerada usa
// el contador real, con reinicio por lista.
func TestDocx_NumberedListUsesRealFormat(t *testing.T) {
	parts := docWithBody(`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Primero</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Segundo</w:t></w:r></w:p>
<w:p><w:r><w:t>Texto normal</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Tercero</w:t></w:r></w:p>`)

	parts["word/numbering.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
</w:numbering>`

	md := convertDocx(t, parts)

	for _, line := range []string{"1. Primero", "2. Segundo", "3. Tercero"} {
		if !strings.Contains(md, line) {
			t.Errorf("falta la línea %q en:\n%s", line, md)
		}
	}

	// El contador no se reinicia al cambiar de párrafo.
	if strings.Contains(md, "1. Tercero") {
		t.Errorf("el contador de la lista se reinició:\n%s", md)
	}
}

// TestDocx_ListStartOverride comprueba que w:startOverride fija el valor
// inicial del contador.
func TestDocx_ListStartOverride(t *testing.T) {
	parts := docWithBody(`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Uno</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Dos</w:t></w:r></w:p>`)

	parts["word/numbering.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="5"/></w:lvlOverride></w:num>
</w:numbering>`

	md := convertDocx(t, parts)

	if !strings.Contains(md, "5. Uno") || !strings.Contains(md, "6. Dos") {
		t.Errorf("el startOverride no se aplicó:\n%s", md)
	}
}

// TestDocx_BulletList comprueba que las viñetas siguen siendo viñetas.
func TestDocx_BulletList(t *testing.T) {
	parts := docWithBody(`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Viñeta</w:t></w:r></w:p>`)

	parts["word/numbering.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
</w:numbering>`

	md := convertDocx(t, parts)

	if !strings.Contains(md, "- Viñeta") {
		t.Errorf("la viñeta no se emitió:\n%s", md)
	}
}

// TestDocx_AlphabeticListUsesHTML comprueba que una lista alfabética, que
// Markdown no puede representar, se emite como <ol type="a">.
func TestDocx_AlphabeticListUsesHTML(t *testing.T) {
	parts := docWithBody(`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Primero</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Segundo</w:t></w:r></w:p>`)

	parts["word/numbering.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%1)"/></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
</w:numbering>`

	md := convertDocx(t, parts)

	if !strings.Contains(md, `<ol type="a" start="1">`) {
		t.Errorf("la lista alfabética no se emitió como <ol>:\n%s", md)
	}
	if !strings.Contains(md, "<li>Primero") || !strings.Contains(md, "<li>Segundo") {
		t.Errorf("faltan elementos en la lista HTML:\n%s", md)
	}
}

// TestDocx_CompositeLabelIsLiteral comprueba que una etiqueta compuesta
// ("2.1.") se emite como texto, porque no existe en Markdown.
func TestDocx_CompositeLabelIsLiteral(t *testing.T) {
	parts := docWithBody(`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Uno</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Sub</w:t></w:r></w:p>`)

	parts["word/numbering.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>
    <w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1.%2."/></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
</w:numbering>`

	md := convertDocx(t, parts)

	if !strings.Contains(md, "1. Uno") {
		t.Errorf("el elemento de primer nivel no es una lista Markdown:\n%s", md)
	}
	if !strings.Contains(md, "1.1. Sub") {
		t.Errorf("la etiqueta compuesta no se conservó:\n%s", md)
	}
}

// TestDocx_NestedListIndentation comprueba que los elementos anidados se
// sangran y que el nivel profundo reinicia cuando el superficial avanza.
func TestDocx_NestedListIndentation(t *testing.T) {
	parts := docWithBody(`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Uno</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Sub A</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Dos</w:t></w:r></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Sub B</w:t></w:r></w:p>`)

	parts["word/numbering.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>
    <w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%2."/></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
</w:numbering>`

	md := convertDocx(t, parts)

	if !strings.Contains(md, "1. Uno") || !strings.Contains(md, "2. Dos") {
		t.Errorf("los elementos de primer nivel no se numeraron:\n%s", md)
	}
	if !strings.Contains(md, "   1. Sub A") {
		t.Errorf("el primer subelemento no está sangrado:\n%s", md)
	}
	if !strings.Contains(md, "   1. Sub B") {
		t.Errorf("el subelemento no reinició su contador:\n%s", md)
	}
}

// TestDocx_Frontmatter comprueba que los metadatos del documento se emiten
// como frontmatter YAML.
func TestDocx_Frontmatter(t *testing.T) {
	parts := docWithBody(`<w:p><w:r><w:t>Cuerpo</w:t></w:r></w:p>`)
	parts["docProps/core.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/"><dc:creator>Nombre Autor</dc:creator><dcterms:created xsi:type="dcterms:W3CDTF">2026-01-02T03:04:05Z</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">2026-02-03T04:05:06Z</dcterms:modified></cp:coreProperties>`

	md := convertDocx(t, parts)

	for _, line := range []string{
		"---",
		`author: "Nombre Autor"`,
		`date: "2026-01-02"`,
		`updated: "2026-02-03"`,
	} {
		if !strings.Contains(md, line) {
			t.Errorf("falta %q en el frontmatter:\n%s", line, md)
		}
	}

	if !strings.HasPrefix(md, "---\n") {
		t.Errorf("el frontmatter no está al principio:\n%s", md)
	}
}

// TestDocx_NoFrontmatterWithoutMetadata comprueba que sin metadatos no se
// emite frontmatter.
func TestDocx_NoFrontmatterWithoutMetadata(t *testing.T) {
	md := convertDocx(t, docWithBody(`<w:p><w:r><w:t>Cuerpo</w:t></w:r></w:p>`))

	if strings.HasPrefix(md, "---") {
		t.Errorf("se emitió frontmatter sin metadatos:\n%s", md)
	}
}

// TestDocx_Footnotes comprueba que las notas al pie se referencian en el
// cuerpo y se definen al final.
func TestDocx_Footnotes(t *testing.T) {
	parts := docWithBody(`<w:p><w:r><w:t>Texto con nota</w:t></w:r><w:r><w:footnoteReference w:id="2"/></w:r></w:p>`)

	parts["word/footnotes.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote>
  <w:footnote w:id="2"><w:p><w:r><w:t>El texto de la nota</w:t></w:r></w:p></w:footnote>
</w:footnotes>`

	md := convertDocx(t, parts)

	if !strings.Contains(md, "[^2]") {
		t.Errorf("la referencia a la nota no se emitió:\n%s", md)
	}
	if !strings.Contains(md, "[^2]: El texto de la nota") {
		t.Errorf("la definición de la nota no se emitió:\n%s", md)
	}
	if strings.Contains(md, "separator") {
		t.Errorf("se emitió una nota estructural de Word:\n%s", md)
	}
}

// TestDocx_ImageIsExtractedAndLinked comprueba que una imagen del cuerpo se
// extrae a disco y se enlaza en el Markdown.
func TestDocx_ImageIsExtractedAndLinked(t *testing.T) {
	parts := docWithBody(`<w:p><w:r><w:drawing><wp:inline><wp:docPr id="1" name="Picture 1" descr="Un logo"/><a:blip r:embed="rId7"/></wp:inline></w:drawing></w:r></w:p>`)

	parts["word/_rels/document.xml.rels"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId7" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/imagen1.png"/></Relationships>`

	parts["word/media/imagen1.png"] = "contenido binario de la imagen"

	input := buildTestDocx(t, parts)
	output := filepath.Join(t.TempDir(), "salida.md")

	conv := NewConverter()
	if err := conv.Convert(input, output); err != nil {
		t.Fatalf("la conversión falló: %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("no se pudo leer el Markdown: %v", err)
	}

	md := string(data)
	if !strings.Contains(md, "![Un logo](salida_assets/imagen1.png)") {
		t.Errorf("la imagen no se enlazó:\n%s", md)
	}

	asset := filepath.Join(filepath.Dir(output), "salida_assets", "imagen1.png")
	content, err := os.ReadFile(asset)
	if err != nil {
		t.Fatalf("la imagen no se extrajo: %v", err)
	}
	if string(content) != "contenido binario de la imagen" {
		t.Errorf("el contenido de la imagen no coincide: %q", content)
	}
}

// TestDocx_MissingImageIsNotBrokenLink comprueba que una imagen sin
// relación no genera un enlace roto.
func TestDocx_MissingImageIsNotBrokenLink(t *testing.T) {
	parts := docWithBody(`<w:p><w:r><w:drawing><wp:inline><wp:docPr id="1" name="Picture 1"/><a:blip r:embed="rId99"/></wp:inline></w:drawing></w:r></w:p>`)

	md := convertDocx(t, parts)

	if strings.Contains(md, "rId99") || strings.Contains(md, "](") {
		t.Errorf("se emitió un enlace de imagen roto:\n%s", md)
	}
}

// TestDocx_BoldOnlyFromRuns comprueba que la marca de párrafo (w:pPr/w:rPr)
// no aplica énfasis al texto, y que una negrita desactivada se respeta.
func TestDocx_BoldOnlyFromRuns(t *testing.T) {
	body := `<w:p><w:pPr><w:rPr><w:b/></w:rPr></w:pPr><w:r><w:t>Sin negrita</w:t></w:r></w:p>
<w:p><w:r><w:rPr><w:b w:val="0"/></w:rPr><w:t>También sin negrita</w:t></w:r></w:p>
<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>Con negrita</w:t></w:r></w:p>`

	md := convertDocx(t, docWithBody(body))

	if strings.Contains(md, "**Sin negrita**") {
		t.Errorf("la marca de párrafo aplicó negrita:\n%s", md)
	}
	if strings.Contains(md, "**También sin negrita**") {
		t.Errorf("una negrita desactivada se aplicó igual:\n%s", md)
	}
	if !strings.Contains(md, "**Con negrita**") {
		t.Errorf("la negrita real no se aplicó:\n%s", md)
	}
}

// TestDocx_EmptyParagraphDoesNotBreakListNumbering comprueba que un párrafo
// numerado vacío consume su número, como en Word.
func TestDocx_EmptyParagraphDoesNotBreakListNumbering(t *testing.T) {
	parts := docWithBody(`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr></w:p>
<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Visible</w:t></w:r></w:p>`)

	parts["word/numbering.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
</w:numbering>`

	md := convertDocx(t, parts)

	if !strings.Contains(md, "3. Visible") {
		t.Errorf("los párrafos vacíos no consumieron su número:\n%s", md)
	}
}

// TestDocx_HeadingKeepsNumberLabel comprueba que un párrafo numerado con
// estilo de título conserva la etiqueta.
func TestDocx_HeadingKeepsNumberLabel(t *testing.T) {
	parts := docWithBody(`<w:p><w:pPr><w:pStyle w:val="Heading2"/><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Sección</w:t></w:r></w:p>`)

	parts["word/numbering.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
</w:numbering>`

	md := convertDocx(t, parts)

	if !strings.Contains(md, "## 1. Sección") {
		t.Errorf("el encabezado no conservó la etiqueta:\n%s", md)
	}
}

// TestDocx_WithoutNumberingFallsBackToBullets comprueba que un DOCX sin
// numbering.xml degrada a viñetas, sin fallar.
func TestDocx_WithoutNumberingFallsBackToBullets(t *testing.T) {
	md := convertDocx(t, docWithBody(`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Elemento</w:t></w:r></w:p>`))

	if !strings.Contains(md, "- Elemento") {
		t.Errorf("sin numbering.xml el elemento no es una viñeta:\n%s", md)
	}
}

// TestDocx_UnderlineIsEmittedAsHTML comprueba que el subrayado se conserva
// como HTML en línea, porque Markdown no lo tiene.
func TestDocx_UnderlineIsEmittedAsHTML(t *testing.T) {
	body := `<w:p><w:r><w:rPr><w:u w:val="single"/></w:rPr><w:t>Subrayado</w:t></w:r></w:p>
<w:p><w:r><w:rPr><w:u/></w:rPr><w:t>Sin valor</w:t></w:r></w:p>
<w:p><w:r><w:rPr><w:u w:val="none"/></w:rPr><w:t>Sin subrayado</w:t></w:r></w:p>
<w:p><w:r><w:rPr><w:b/><w:u/></w:rPr><w:t>Negrita y subrayado</w:t></w:r></w:p>`

	md := convertDocx(t, docWithBody(body))

	if !strings.Contains(md, "<u>Subrayado</u>") {
		t.Errorf("el subrayado no se emitió:\n%s", md)
	}
	if !strings.Contains(md, "<u>Sin valor</u>") {
		t.Errorf("un subrayado sin valor no se emitió:\n%s", md)
	}
	if strings.Contains(md, "<u>Sin subrayado</u>") {
		t.Errorf("un subrayado con val=\"none\" se emitió:\n%s", md)
	}
	// El HTML va por fuera del énfasis, para que las marcas de Markdown de
	// dentro sigan interpretándose.
	if !strings.Contains(md, "<u>**Negrita y subrayado**</u>") {
		t.Errorf("el subrayado no envuelve al énfasis:\n%s", md)
	}
}

// TestDocx_SkipBeforeHeading comprueba que el descarte empieza en el primer
// encabezado y descarta la portada.
func TestDocx_SkipBeforeHeading(t *testing.T) {
	body := `<w:p><w:r><w:t>PORTADA INSTITUCIONAL</w:t></w:r></w:p>
<w:p><w:r><w:t>Tabla de versión</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Título real</w:t></w:r></w:p>
<w:p><w:r><w:t>Cuerpo del documento</w:t></w:r></w:p>`

	parts := docWithBody(body)
	input := buildTestDocx(t, parts)
	output := filepath.Join(t.TempDir(), "salida.md")

	conv := NewConverter()
	conv.SetSkipBeforeHeading(true)
	if err := conv.Convert(input, output); err != nil {
		t.Fatalf("la conversión falló: %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("no se pudo leer el Markdown: %v", err)
	}

	md := string(data)
	if strings.Contains(md, "PORTADA INSTITUCIONAL") {
		t.Errorf("la portada no se descartó:\n%s", md)
	}
	if strings.Contains(md, "Tabla de versión") {
		t.Errorf("la tabla de versión no se descartó:\n%s", md)
	}
	if !strings.Contains(md, "# Título real") || !strings.Contains(md, "Cuerpo del documento") {
		t.Errorf("el contenido del encabezado se perdió:\n%s", md)
	}
}

// TestDocx_SkipBeforeHeadingKeepsIntegrity comprueba que el descarte también
// se aplica al texto de la integridad: la mide sobre lo que se conserva.
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

// TestDocx_SkipBeforeHeadingWithoutHeading comprueba que un documento sin
// encabezados no se vacía.
func TestDocx_SkipBeforeHeadingWithoutHeading(t *testing.T) {
	body := `<w:p><w:r><w:t>Solo texto</w:t></w:r></w:p>`

	parts := docWithBody(body)
	input := buildTestDocx(t, parts)
	output := filepath.Join(t.TempDir(), "salida.md")

	conv := NewConverter()
	conv.SetSkipBeforeHeading(true)
	if err := conv.Convert(input, output); err != nil {
		t.Fatalf("la conversión falló: %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("no se pudo leer el Markdown: %v", err)
	}

	if !strings.Contains(string(data), "Solo texto") {
		t.Errorf("un documento sin encabezados se vació:\n%s", data)
	}
}
