package main

import (
	"strings"
	"testing"
)

// syntheticMarkdown genera un Markdown con líneas envueltas, similar a la
// salida de un extractor de texto, para medir el coste de la limpieza.
func syntheticMarkdown(paragraphs, linesPerParagraph int) []string {
	lines := make([]string, 0, paragraphs*linesPerParagraph)

	for p := 0; p < paragraphs; p++ {
		lines = append(lines, "## Sección "+string(rune('A'+p%26))+string(rune('a'+p%26)))
		lines = append(lines, "")
		for l := 0; l < linesPerParagraph; l++ {
			words := make([]string, 0, 12)
			for w := 0; w < 12; w++ {
				words = append(words, "palabra"+string(rune('a'+w)))
			}
			lines = append(lines, strings.Join(words, " "))
		}
		lines = append(lines, "")
	}

	return lines
}

// BenchmarkProcess mide el modo genérico de limpieza.
func BenchmarkProcess(b *testing.B) {
	lines := syntheticMarkdown(500, 20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = process(lines)
	}
}

// BenchmarkProcessTDR mide el modo TDR, que es el más caro por las pasadas
// sucesivas y las expresiones regulares.
func BenchmarkProcessTDR(b *testing.B) {
	lines := syntheticMarkdown(500, 20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processTDR(lines)
	}
}

// BenchmarkIsStructuralLine mide el predicado que se llama por cada par de
// líneas. Es el punto donde compilar las expresiones regulares dentro de la
// función se nota.
func BenchmarkIsStructuralLine(b *testing.B) {
	lines := syntheticMarkdown(200, 20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, line := range lines {
			_ = isStructuralLine(line)
		}
	}
}
