package main

import (
	"strings"
	"testing"
)

// TestJoinerBetween_ShortWords comprueba que las palabras cortas del español
// no se pegan a la palabra vecina.
func TestJoinerBetween_ShortWords(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected string
	}{
		{"preposición detrás", "establecido", "en la ley", " "},
		{"conjunción detrás", "mes", "y año", " "},
		{"artículo detrás", "ley", "del estado", " "},
		{"palabra larga detrás", "ley", "organización", ""},
		{"siguiente empieza en mayúscula", "texto", "Nuevo", " "},
		{"termina en puntuación", "texto,", "siguiente", " "},
		{"palabra partida", "palabra", "siguiente", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinerBetween(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("joinerBetween(%q, %q) = %q, se esperaba %q", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

// TestJoinerBetween_KeepsAsymmetry comprueba que "ya" y "no" solo se tratan
// como palabra corta cuando están al final de la línea izquierda.
func TestJoinerBetween_KeepsAsymmetry(t *testing.T) {
	if got := joinerBetween("ya", "casa"); got != " " {
		t.Errorf("joinerBetween(%q, %q) = %q, se esperaba %q", "ya", "casa", got, " ")
	}
	if got := joinerBetween("casa", "ya"); got != "" {
		t.Errorf("joinerBetween(%q, %q) = %q, se esperaba %q", "casa", "ya", got, "")
	}
}

// TestShouldJoin comprueba las condiciones para unir dos líneas.
func TestShouldJoin(t *testing.T) {
	larga := strings.Repeat("palabra ", 20)

	if !shouldJoin(larga, "siguiente") {
		t.Error("una línea larga y su continuación deberían unirse")
	}
	if shouldJoin("corta", "corta") {
		t.Error("dos líneas cortas no envueltas no deberían unirse")
	}
	if shouldJoin("# Título", "texto") {
		t.Error("no se debe unir a través de un encabezado")
	}
	if shouldJoin("Frase terminada.", "siguiente") {
		t.Error("no se debe unir tras un punto final")
	}
	if shouldJoin("", "texto") {
		t.Error("no se debe unir una línea vacía")
	}
}

// TestProcess_JoinsWrappedParagraph comprueba que el modo genérico rearma un
// párrafo partido y respeta la estructura.
func TestProcess_JoinsWrappedParagraph(t *testing.T) {
	lines := []string{
		"## Sección",
		"",
		strings.Repeat("palabra ", 20) + "continuación",
		"| Columna | Valor |",
	}

	out := process(lines)

	if len(out) != len(lines) {
		t.Fatalf("se esperaban %d líneas, hay %d: %v", len(lines), len(out), out)
	}
	if !strings.Contains(out[2], "palabra continuación") {
		t.Errorf("el párrafo no se rearmó: %q", out[2])
	}
	if out[3] != "| Columna | Valor |" {
		t.Errorf("la fila de tabla cambió: %q", out[3])
	}
}

// TestIsStructuralLine_CorpusMarkers comprueba los marcadores del corpus
// institucional, que viven en corpus.go.
func TestIsStructuralLine_CorpusMarkers(t *testing.T) {
	markers := []string{
		"NOTA GENERAL:",
		"EL PLENO",
		"CONSIDERANDO:",
		"Concordancias:",
		"Art. 1",
		"Capítulo I",
		"Sección II",
		"TITULO",
		"FUNCION EJECUTIVA",
		"ASAMBLEA NACIONAL",
		"PRESIDENCIA DE LA REPUBLICA",
		"LEXIS FINDER",
		"a) primero",
		"1.- primero",
	}
	for _, marker := range markers {
		if !isStructuralLine(marker) {
			t.Errorf("%q debería ser línea estructural", marker)
		}
	}
}
