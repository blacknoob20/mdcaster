package main

import "testing"

// TestWordToken comprueba la extracción de la primera y la última palabra
// alfabética de una línea, que decide si dos líneas se unen con espacio o
// pegadas.
func TestWordToken(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		fromEnd  bool
		expected string
	}{
		{"primera de una frase", "Hola mundo", false, "Hola"},
		{"última de una frase", "Hola mundo", true, "mundo"},
		{"primera tras dígitos", "123 abc", false, "abc"},
		{"última tras dígitos", "abc 123", true, "abc"},
		{"primera tras puntuación", "...xyz", false, "xyz"},
		{"última tras puntuación", "xyz...", true, "xyz"},
		{"palabra con acentos", "áéí óú", false, "áéí"},
		{"línea vacía", "", false, ""},
		{"sin letras", "123 456", true, ""},
		{"byte inválido corta el escaneo", "abc\xffdef", false, "abc"},
		{"byte inválido corta por el final", "abc\xffdef", true, "def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wordToken(tt.in, tt.fromEnd)
			if got != tt.expected {
				t.Errorf("wordToken(%q, fromEnd=%v) = %q, se esperaba %q", tt.in, tt.fromEnd, got, tt.expected)
			}
		})
	}
}

// TestFirstAndLastWordToken comprueba que los dos envoltorios usan el mismo
// escaneo que wordToken.
func TestFirstAndLastWordToken(t *testing.T) {
	if got := firstWordToken("Hola mundo"); got != "Hola" {
		t.Errorf("firstWordToken = %q, se esperaba %q", got, "Hola")
	}
	if got := lastWordToken("Hola mundo"); got != "mundo" {
		t.Errorf("lastWordToken = %q, se esperaba %q", got, "mundo")
	}
}

// TestIsMarkdownStructureLine comprueba las líneas de estructura de Markdown
// que los dos modos comparten.
func TestIsMarkdownStructureLine(t *testing.T) {
	estructura := []string{
		"",
		"# Título",
		"## Sección",
		"![](media/rId40.png)",
		"| Columna | Valor |",
		"- viñeta",
		"* viñeta",
	}
	for _, line := range estructura {
		if !isMarkdownStructureLine(line) {
			t.Errorf("%q debería ser estructura", line)
		}
	}

	texto := []string{"Texto normal", "Una frase con espacios", "1. Numeral"}
	for _, line := range texto {
		if isMarkdownStructureLine(line) {
			t.Errorf("%q no debería ser estructura", line)
		}
	}
}
