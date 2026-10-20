package main

import (
	"strings"
	"testing"
)

// TestSplitFrontmatter_KeepsBlockIntact comprueba que el frontmatter YAML se
// aparta del cuerpo y se devuelve tal cual.
func TestSplitFrontmatter_KeepsBlockIntact(t *testing.T) {
	lines := []string{
		"---",
		`author: "Nombre"`,
		`date: "2026-01-02"`,
		"---",
		"",
		"Título",
	}

	frontmatter, body := splitFrontmatter(lines)

	if len(frontmatter) != 4 {
		t.Fatalf("se esperaba un frontmatter de 4 líneas, hay %d: %v", len(frontmatter), frontmatter)
	}
	if frontmatter[1] != `author: "Nombre"` {
		t.Errorf("el frontmatter se alteró: %q", frontmatter[1])
	}
	if len(body) != 2 || body[0] != "" || body[1] != "Título" {
		t.Errorf("el cuerpo no se separó bien: %v", body)
	}
}

// TestSplitFrontmatter_WithoutFrontmatter comprueba que un documento sin
// frontmatter se devuelve entero.
func TestSplitFrontmatter_WithoutFrontmatter(t *testing.T) {
	lines := []string{"Título", "Cuerpo"}

	frontmatter, body := splitFrontmatter(lines)

	if frontmatter != nil {
		t.Errorf("se detectó un frontmatter que no existe: %v", frontmatter)
	}
	if len(body) != 2 {
		t.Errorf("el cuerpo cambió: %v", body)
	}
}

// TestIsStructuralLine_TableRowsAreStructural comprueba que las filas de
// tabla Markdown no se unen con las líneas de alrededor.
func TestIsStructuralLine_TableRowsAreStructural(t *testing.T) {
	row := "| **Área** | *Detalle* |"

	if !isStructuralLine(row) {
		t.Error("una fila de tabla no es una línea estructural en el modo genérico")
	}
	if !isTDRStructuralLine(row) {
		t.Error("una fila de tabla no es una línea estructural en el modo TDR")
	}
}

// TestProcessTDR_KeepsTablesIntact comprueba que el modo TDR no destruye una
// tabla Markdown.
func TestProcessTDR_KeepsTablesIntact(t *testing.T) {
	lines := []string{
		"| Nombre | Valor |",
		"| --- | --- |",
		"| Fila | 1 |",
		"",
		"Texto normal que sigue",
	}

	out := processTDR(lines)

	joined := strings.Join(out, "\n")
	for _, want := range []string{"| Nombre | Valor |", "| --- | --- |", "| Fila | 1 |"} {
		if !strings.Contains(joined, want) {
			t.Errorf("la tabla se rompió, falta %q:\n%s", want, joined)
		}
	}
}
