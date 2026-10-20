// Primitivas compartidas por los dos modos de limpieza: qué líneas son
// estructura de Markdown, cómo se detectan los espacios en los bordes y cómo
// se extrae una palabra del principio o del final de una línea.
package main

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// reBullet detecta las viñetas de Markdown. Una viñeta abre una lista, así que
// nunca se une con la línea siguiente.
var reBullet = regexp.MustCompile(`^[-*]\s+`)

// isMarkdownStructureLine indica si la línea, ya recortada, es estructura de
// Markdown: vacía, encabezado, imagen, fila de tabla o viñeta.
//
// La comparten los dos modos: es estructura que no se puede unir con la línea
// siguiente.
func isMarkdownStructureLine(trimmed string) bool {
	switch {
	case trimmed == "":
		return true
	case strings.HasPrefix(trimmed, "#"):
		return true
	case strings.HasPrefix(trimmed, "![]("):
		return true
	case strings.HasPrefix(trimmed, "|"):
		return true
	case reBullet.MatchString(trimmed):
		return true
	}
	return false
}

// startsWithUnicodeSpace indica si s empieza por un espacio Unicode.
func startsWithUnicodeSpace(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r != utf8.RuneError && unicode.IsSpace(r)
}

// endsWithUnicodeSpace indica si s termina en un espacio Unicode.
func endsWithUnicodeSpace(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r != utf8.RuneError && unicode.IsSpace(r)
}

// firstWordToken devuelve la primera palabra alfabética de s.
func firstWordToken(s string) string {
	return wordToken(s, false)
}

// lastWordToken devuelve la última palabra alfabética de s.
func lastWordToken(s string) string {
	return wordToken(s, true)
}

// wordToken devuelve la palabra alfabética contigua más cercana al borde de s:
// desde el principio (fromEnd=false) o desde el final (fromEnd=true).
//
// Se salta espacios, puntuación y dígitos hasta la primera letra y después
// recoge las letras contiguas. Un rune inválido (utf8.RuneError) corta el
// escaneo, como en un documento mal codificado.
//
// El escaneo va en las dos direcciones con dos bucles distintos y no con uno
// parametrizado: wordToken se llama por cada par de líneas que se une, y la
// llamada extra por run se nota en el benchmark del modo genérico.
func wordToken(s string, fromEnd bool) string {
	var token []rune

	if fromEnd {
		// Fase 1: la letra más cercana al final.
		for len(s) > 0 {
			r, size := utf8.DecodeLastRuneInString(s)
			if r == utf8.RuneError {
				break
			}
			s = s[:len(s)-size]
			if unicode.IsLetter(r) {
				token = append(token, r)
				break
			}
		}
		// Fase 2: las letras contiguas hacia la izquierda.
		for len(s) > 0 {
			r, size := utf8.DecodeLastRuneInString(s)
			if r == utf8.RuneError || !unicode.IsLetter(r) {
				break
			}
			token = append(token, r)
			s = s[:len(s)-size]
		}
		// El token se ha ido rellenando de derecha a izquierda.
		for i, j := 0, len(token)-1; i < j; i, j = i+1, j-1 {
			token[i], token[j] = token[j], token[i]
		}
		return string(token)
	}

	// Fase 1: la letra más cercana al principio.
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError {
			break
		}
		s = s[size:]
		if unicode.IsLetter(r) {
			token = append(token, r)
			break
		}
	}
	// Fase 2: las letras contiguas hacia la derecha.
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError || !unicode.IsLetter(r) {
			break
		}
		token = append(token, r)
		s = s[size:]
	}
	return string(token)
}
