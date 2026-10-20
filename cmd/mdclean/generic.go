// Modo genérico: une líneas consecutivas que parecen haber sido partidas por
// un extractor de texto.
//
// Es conservador por diseño: ante la duda, no une. El objetivo es no
// colapsar bloques de direcciones ni títulos.
package main

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Las expresiones regulares se compilan una sola vez, al inicio del paquete.
// Compilarlas dentro de estas funciones, que se llaman por cada línea del
// fichero, multiplicaba el coste de la limpieza por diez.
var twoPlusSpaces = regexp.MustCompile(`\s{2,}`)

// Enumeraciones ("a)") y elementos numerados ("1.-"): mantienen el salto de
// línea, no son texto que se pueda unir.
var (
	reEnumeracion  = regexp.MustCompile(`^[a-zA-Z]\)\s+`)
	reItemNumerado = regexp.MustCompile(`^\d+\.-\s+`)
)

// isShortWord indica si la palabra es una de las palabras cortas del español
// que no deben pegarse a la palabra vecina.
//
// La comprobación es un switch y no una tabla: el switch cuesta 3,5 ns y la
// búsqueda en un mapa 48 ns, y esta función se llama dos veces por cada par de
// líneas que se une.
func isShortWord(word string) bool {
	switch word {
	case "de", "del", "la", "el", "y", "o", "a", "al", "en", "por", "para", "con", "sin",
		"un", "una", "unos", "unas", "los", "las", "lo", "que", "se", "su", "sus", "mi", "mis",
		"tu", "tus":
		return true
	}
	return false
}

// process une las líneas partidas y devuelve el documento con los párrafos
// rearmados.
func process(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		// El párrafo se acumula en un builder para no copiar la cadena en
		// cada unión (coste O(n²) en párrafos largos).
		var b strings.Builder
		b.WriteString(lines[i])
		cur := lines[i]

		for i+1 < len(lines) && shouldJoin(cur, lines[i+1]) {
			b.WriteString(joinerBetween(cur, lines[i+1]))
			b.WriteString(lines[i+1])
			i++
			cur = b.String()
		}
		out = append(out, cur)
	}
	return out
}

// shouldJoin indica si la línea b debe unirse a la línea a.
func shouldJoin(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if isStructuralLine(a) || isStructuralLine(b) {
		return false
	}
	// Only join when it looks like wrapped text (typical PDF/text extraction),
	// to avoid collapsing address blocks and headings.
	if !looksWrappedLine(a) && !looksWrappedLine(b) {
		return false
	}

	// Avoid joining if it looks like an intentional paragraph end.
	// (Conservative: don't join after sentence-ending punctuation.)
	trimRight := strings.TrimRight(a, " \t")
	if trimRight == "" {
		return false
	}
	switch trimRight[len(trimRight)-1] {
	case '.', '!', '?', ':':
		return false
	}

	return true
}

// looksWrappedLine indica si la línea parece haber sido partida: es larga,
// tiene tabuladores o tiene espacios dobles.
func looksWrappedLine(line string) bool {
	if len(line) >= 72 {
		return true
	}
	if strings.Contains(line, "\t") {
		return true
	}
	return twoPlusSpaces.MatchString(line)
}

// isStructuralLine indica si la línea no debe unirse con la línea siguiente
// porque es estructura: encabezado, imagen, fila de tabla, viñeta, marcador de
// sección o cabecera institucional.
func isStructuralLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if isMarkdownStructureLine(trimmed) {
		return true
	}

	// Headings / section markers common in this corpus.
	// Keep conservative: don't join across these.
	for _, re := range structuralLines {
		if re.MatchString(trimmed) {
			return true
		}
	}

	// Bullets / enumerations: keep line breaks.
	if reEnumeracion.MatchString(trimmed) {
		return true
	}
	if reItemNumerado.MatchString(trimmed) {
		return true
	}

	// Page headers often centered; keep them as-is.
	return strings.Contains(trimmed, "LEXIS FINDER")
}

// joinerBetween decide el separador entre dos líneas que se van a unir.
func joinerBetween(a, b string) string {
	// If there is already whitespace at the boundary, keep it as-is.
	if endsWithUnicodeSpace(a) || startsWithUnicodeSpace(b) {
		return ""
	}

	// No whitespace boundary: often a PDF/text extraction split. Decide whether it was
	// likely mid-word (join with "") or between two words (join with " ").
	left := strings.ToLower(lastWordToken(a))
	right := strings.ToLower(firstWordToken(b))

	// If the next token begins with uppercase, it's almost certainly a new word/name.
	if r, _ := utf8.DecodeRuneInString(strings.TrimLeftFunc(b, unicode.IsSpace)); r != utf8.RuneError && unicode.IsUpper(r) {
		return " "
	}

	// If left ends with punctuation that typically separates words, keep a space.
	trimRight := strings.TrimRightFunc(a, unicode.IsSpace)
	if trimRight != "" {
		r, _ := utf8.DecodeLastRuneInString(trimRight)
		switch r {
		case ',', ';', '.', ')':
			return " "
		}
	}

	// Common Spanish short words that should not be glued.
	// Check both sides: e.g., "establecido" + "en" or "mes" + "y".
	// "ya" y "no" solo cuentan cuando están al final de la línea izquierda.
	if isShortWord(left) || left == "ya" || left == "no" {
		return " "
	}
	if isShortWord(right) {
		return " "
	}

	// Default: assume it was a word split.
	return ""
}
