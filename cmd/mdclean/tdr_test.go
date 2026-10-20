package main

import (
	"strings"
	"testing"
)

// TestCollapseBlankLines comprueba que las líneas en blanco consecutivas se
// reducen a una sola.
func TestCollapseBlankLines(t *testing.T) {
	in := []string{"a", "", "", "b", "", "", ""}
	want := []string{"a", "", "b", ""}

	got := collapseBlankLines(in)

	if len(got) != len(want) {
		t.Fatalf("se esperaban %d líneas, hay %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("línea %d = %q, se esperaba %q", i, got[i], want[i])
		}
	}
}

// TestMergeEmphasisLines comprueba que las líneas consecutivas de solo-negrita
// se fusionan en una.
func TestMergeEmphasisLines(t *testing.T) {
	in := []string{"**uno**", "**dos**", "texto", "*x*"}

	got := mergeEmphasisLines(in, boldOnlyLine, "**")

	want := []string{"**uno dos**", "texto", "*x*"}
	if len(got) != len(want) {
		t.Fatalf("se esperaban %d líneas, hay %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("línea %d = %q, se esperaba %q", i, got[i], want[i])
		}
	}
}

// TestTDRPass4BoldToHeadings comprueba la conversión de negrita en
// encabezados y la regla de "TÉRMINOS", que solo aplica la primera vez.
func TestTDRPass4BoldToHeadings(t *testing.T) {
	in := []string{
		"**INDICE**",
		"**CONTENIDO**",
		"**TÉRMINOS DE REFERENCIA**",
		"**METODOLOGÍA**",
		"**TÉRMINOS**",
		"**OBJETIVO GENERAL**",
		"**texto normal**",
		"no es negrita",
	}

	got := tdrPass4BoldToHeadings(in)

	want := []string{
		"## INDICE",
		"### CONTENIDO",
		"# TÉRMINOS DE REFERENCIA",
		"## METODOLOGÍA",
		"## TÉRMINOS",
		"**OBJETIVO GENERAL**",
		"**texto normal**",
		"no es negrita",
	}
	if len(got) != len(want) {
		t.Fatalf("se esperaban %d líneas, hay %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("línea %d = %q, se esperaba %q", i, got[i], want[i])
		}
	}
}

// TestTDRPass4bNormalizeTOC comprueba la normalización de la tabla de
// contenidos: fusión de la entrada con su número y del número de página.
func TestTDRPass4bNormalizeTOC(t *testing.T) {
	in := []string{
		"### CONTENIDO",
		"1.",
		"ANTECEDENTES",
		"OBJETIVO",
		".",
		"5",
		"## OTRA SECCIÓN",
	}

	got := tdrPass4bNormalizeTOC(in)

	want := []string{
		"### CONTENIDO",
		"1. ANTECEDENTES",
		"OBJETIVO",
		". 5",
		"## OTRA SECCIÓN",
	}
	if len(got) != len(want) {
		t.Fatalf("se esperaban %d líneas, hay %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("línea %d = %q, se esperaba %q", i, got[i], want[i])
		}
	}
}

// TestFixQuotedItalicSpans comprueba la reparación de una cita en cursiva
// partida entre dos líneas.
func TestFixQuotedItalicSpans(t *testing.T) {
	in := []string{`"*texto`, `continuación*"`, "fin"}

	got := fixQuotedItalicSpans(in)

	want := []string{`"*texto continuación*"`, "fin"}
	if len(got) != len(want) {
		t.Fatalf("se esperaban %d líneas, hay %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("línea %d = %q, se esperaba %q", i, got[i], want[i])
		}
	}
}

// TestTDRPass6UnwrapParagraphs comprueba que la desenvoltura une las líneas
// de un párrafo y respeta la estructura.
func TestTDRPass6UnwrapParagraphs(t *testing.T) {
	in := []string{"linea uno", "linea dos", "", "párrafo", "| tabla |"}

	got := tdrPass6UnwrapParagraphs(in)

	want := []string{"linea uno linea dos", "", "párrafo", "| tabla |"}
	if len(got) != len(want) {
		t.Fatalf("se esperaban %d líneas, hay %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("línea %d = %q, se esperaba %q", i, got[i], want[i])
		}
	}
}

// TestIsRepeatedTDRHeaderLineAndImage comprueba los datos del corpus: las
// cabeceras institucionales y las imágenes de cabecera repetidas.
func TestIsRepeatedTDRHeaderLineAndImage(t *testing.T) {
	if !isRepeatedTDRHeaderLine("GAD MUNICIPAL DE MACHALA") {
		t.Error("la cabecera institucional debería detectarse")
	}
	if !isRepeatedTDRHeaderLine("gad  municipal  de  machala") {
		t.Error("la cabecera debería detectarse ignorando mayúsculas y espacios")
	}
	if isRepeatedTDRHeaderLine("TEXTO NORMAL") {
		t.Error("una línea normal no es cabecera institucional")
	}

	if !isRepeatedTDRHeaderImage("![](media/rId40.png)") {
		t.Error("la imagen de cabecera debería detectarse")
	}
	if isRepeatedTDRHeaderImage("![](media/rId99.png)") {
		t.Error("una imagen que no es de cabecera no debe detectarse")
	}
}

// TestProcessTDR_Characterization fija el resultado del modo TDR sobre un
// documento de ejemplo. Es la red de seguridad del refactor: cualquier cambio
// en la salida tiene que ser intencionado y actualizar esta prueba.
func TestProcessTDR_Characterization(t *testing.T) {
	in := []string{
		"**GAD MUNICIPAL DE MACHALA**",
		"**DIRECCIÓN DE TECNOLOGÍA DE LA INFORMACIÓN**",
		"",
		"**TÉRMINOS DE REFERENCIA**",
		"",
		"**1. OBJETIVO**",
		"Este documento define el alcance",
		"del trabajo contratado.",
		"",
		"### CONTENIDO",
		"1.",
		"ANTECEDENTES",
		"",
		"2. METODOLOGÍA",
		"",
		"## PLAN DE TRABAJO",
		"",
		"*texto en cursiva*",
		"",
		"| Columna | Valor |",
		"| --- | --- |",
		"| Fila | 1 |",
	}

	want := []string{
		"## GAD MUNICIPAL DE MACHALA",
		"",
		"# TÉRMINOS DE REFERENCIA",
		"",
		"**1. OBJETIVO**",
		"Este documento define el alcance del trabajo contratado.",
		"",
		"### CONTENIDO",
		"1. ANTECEDENTES",
		"",
		"2. METODOLOGÍA",
		"",
		"## PLAN DE TRABAJO",
		"",
		"*texto en cursiva*",
		"",
		"| Columna | Valor |",
		"| --- | --- |",
		"| Fila | 1 |",
	}

	got := processTDR(in)

	if len(got) != len(want) {
		t.Fatalf("se esperaban %d líneas, hay %d:\n%s", len(want), len(got), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("línea %d = %q, se esperaba %q", i, got[i], want[i])
		}
	}
}
