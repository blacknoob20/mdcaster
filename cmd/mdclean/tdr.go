// Modo TDR: normaliza el Markdown exportado del corpus institucional
// (DOCX/PDF). Son nueve etapas sobre el array de líneas; el orden es
// normativo, cada etapa consume el resultado de la anterior.
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
var (
	boldOnlyLine   = regexp.MustCompile(`^\*\*(.+?)\*\*\s*$`)
	italicOnlyLine = regexp.MustCompile(`^\*(.+?)\*\s*$`)
	pageNumberLine = regexp.MustCompile(`^Pág\.?\s*\d+\s*$`)
	digitsOnlyLine = regexp.MustCompile(`^\d{1,3}$`)
	dotLeaderRun   = regexp.MustCompile(`\s*\.{5,}\s*`)
	indexNumber    = regexp.MustCompile(`^\d+\.$`)
)

// processTDR normaliza el Markdown exportado del corpus institucional.
func processTDR(lines []string) []string {
	lines = tdrPass1Normalize(lines)                       // Pasada 1: normalización de cada línea
	lines = collapseBlankLines(lines)                      // Pasada 2: líneas en blanco
	lines = mergeEmphasisLines(lines, boldOnlyLine, "**")  // Pasada 3: negrita
	lines = tdrPass4BoldToHeadings(lines)                  // Pasada 4: negrita → encabezados
	lines = tdrPass4bNormalizeTOC(lines)                   // Pasada 4b: índice
	lines = mergeEmphasisLines(lines, italicOnlyLine, "*") // Pasada 5: cursiva
	lines = fixQuotedItalicSpans(lines)                    // Pasada 5b: cursivas partidas
	lines = tdrPass6UnwrapParagraphs(lines)                // Pasada 6: desenvoltura
	return tdrPass7PostProcess(lines)                      // Pasada 7: post-proceso
}

// tdrPass1Normalize normaliza cada línea: une las imágenes partidas, elimina
// los números de página y las cabeceras repetidas, y limpia los artefactos de
// exportación.
func tdrPass1Normalize(lines []string) []string {
	var normalized []string
	seenHeader := false
	seenHeaderImages := false
	for i := 0; i < len(lines); {
		line, next := combineSplitImageLine(lines, i)
		i = next
		line = normalizeLine(line)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			normalized = append(normalized, "")
			continue
		}
		if digitsOnlyLine.MatchString(trimmed) {
			// Common page-number artifact in these exports.
			continue
		}
		// Drop repeated page header images (after keeping the first set once).
		if isRepeatedTDRHeaderImage(trimmed) {
			if seenHeaderImages {
				continue
			}
			seenHeaderImages = true
		}

		if pageNumberLine.MatchString(trimmed) {
			continue
		}

		// Drop repeated institutional header lines after the first occurrence.
		plain := trimmed
		if m := boldOnlyLine.FindStringSubmatch(trimmed); m != nil {
			plain = m[1]
		}
		if isRepeatedTDRHeaderLine(plain) {
			if seenHeader {
				continue
			}
			seenHeader = true
		}

		// Replace "\\." sequences used to draw dotted leaders in indices.
		line = strings.ReplaceAll(line, "\\.", ".")
		// Remove any remaining literal backslashes used as line break artifacts.
		line = strings.ReplaceAll(line, "\\", "")

		// Normalize bullet glyphs.
		line = strings.TrimLeftFunc(line, unicode.IsSpace)
		line = strings.ReplaceAll(line, "➢", "-")
		line = strings.ReplaceAll(line, "•", "-")
		line = strings.ReplaceAll(line, "–", "-")
		line = strings.ReplaceAll(line, "—", "-")

		// Collapse excessive whitespace for readability (but keep images intact).
		if !strings.HasPrefix(strings.TrimSpace(line), "![](") {
			line = collapseSpaces(line)
		}
		normalized = append(normalized, line)
	}
	return normalized
}

// collapseBlankLines reduce las líneas en blanco consecutivas a una sola.
func collapseBlankLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	prevBlank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if prevBlank {
				continue
			}
			prevBlank = true
			out = append(out, "")
			continue
		}
		prevBlank = false
		out = append(out, line)
	}
	return out
}

// mergeEmphasisLines une las líneas consecutivas que son solo énfasis
// (negrita o cursiva) en una sola línea con el mismo marcador.
func mergeEmphasisLines(lines []string, re *regexp.Regexp, marker string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		m := re.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if m == nil {
			out = append(out, lines[i])
			continue
		}
		parts := []string{collapseSpaces(m[1])}
		for i+1 < len(lines) {
			nm := re.FindStringSubmatch(strings.TrimSpace(lines[i+1]))
			if nm == nil {
				break
			}
			parts = append(parts, collapseSpaces(nm[1]))
			i++
		}
		out = append(out, marker+strings.Join(parts, " ")+marker)
	}
	return out
}

// tdrPass4BoldToHeadings convierte las líneas solo-negrita en encabezados
// según su contenido.
func tdrPass4BoldToHeadings(lines []string) []string {
	out := make([]string, 0, len(lines))
	hasMainTitle := false
	for _, line := range lines {
		m := boldOnlyLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			out = append(out, line)
			continue
		}
		content := collapseSpaces(m[1])
		// Some exports produce nested emphasis like **"***text***"**; normalize it.
		content = strings.ReplaceAll(content, "*", "")
		upper := strings.ToUpper(content)
		switch {
		case upper == "INDICE":
			out = append(out, "## INDICE")
		case upper == "CONTENIDO":
			out = append(out, "### CONTENIDO")
		case !hasMainTitle && strings.Contains(upper, "TÉRMINOS"):
			out = append(out, "# "+content)
			hasMainTitle = true
		case reSeccionTDR.MatchString(upper):
			out = append(out, "## "+content)
		default:
			// Otherwise keep as bold but without the export artifacts.
			out = append(out, "**"+content+"**")
		}
	}
	return out
}

// tdrPass4bNormalizeTOC normaliza el formato de la tabla de contenidos.
func tdrPass4bNormalizeTOC(lines []string) []string {
	inTOC := false
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "### CONTENIDO" {
			inTOC = true
			out = append(out, trimmed)
			continue
		}
		if inTOC {
			// TOC ends at the next major section heading.
			if strings.HasPrefix(trimmed, "## ") && trimmed != "## INDICE" {
				inTOC = false
				out = append(out, line)
				continue
			}

			// Merge lines like "1." + "ANTECEDENTES ... 2".
			if indexNumber.MatchString(trimmed) {
				if j := nextNonBlankIndex(lines, i+1); j < len(lines) {
					out = append(out, mergeTOCEntry(trimmed, strings.TrimSpace(lines[j])))
					i = j
					continue
				}
			}

			// If a TOC entry ends with dots and next line is a bare number, treat it as a page number.
			if strings.HasSuffix(strings.TrimRightFunc(trimmed, unicode.IsSpace), ".") {
				if j := nextNonBlankIndex(lines, i+1); j < len(lines) {
					nextTrim := strings.TrimSpace(lines[j])
					if digitsOnlyLine.MatchString(nextTrim) {
						out = append(out, mergeTOCEntry(trimmed, nextTrim))
						i = j
						continue
					}
				}
			}

			out = append(out, collapseSpaces(dotLeaderRun.ReplaceAllString(line, " … ")))
			continue
		}

		out = append(out, line)
	}
	return out
}

// nextNonBlankIndex devuelve el índice de la siguiente línea no vacía a
// partir de i, o la longitud de la lista si no hay ninguna.
func nextNonBlankIndex(lines []string, i int) int {
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return len(lines)
}

// mergeTOCEntry fusiona dos entradas del índice y normaliza los líderes de
// puntos.
func mergeTOCEntry(cur, next string) string {
	merged := cur + " " + next
	merged = dotLeaderRun.ReplaceAllString(merged, " … ")
	return collapseSpaces(merged)
}

// fixQuotedItalicSpans repara las citas en cursiva partidas entre líneas.
func fixQuotedItalicSpans(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		start := strings.Index(line, "\"*")
		if start == -1 {
			out = append(out, line)
			continue
		}
		end := strings.Index(line[start+2:], "*\"")
		if end != -1 {
			out = append(out, line)
			continue
		}

		buf := strings.TrimRightFunc(line, unicode.IsSpace)
		j := i + 1
		// El fragmento se acumula en un builder para no copiar la cadena en
		// cada línea (coste O(n²) en fragmentos largos).
		var b strings.Builder
		b.WriteString(buf)
		for ; j < len(lines); j++ {
			b.WriteString(" ")
			b.WriteString(strings.TrimSpace(lines[j]))
			if strings.Contains(lines[j], "*\"") {
				break
			}
		}
		buf = b.String()
		// If we never found the closing marker, just emit the joined text.
		if !strings.Contains(buf[start+2:], "*\"") {
			out = append(out, collapseSpaces(buf))
			i = j
			continue
		}

		// Clean inner stray '*' so the whole span becomes a single italic phrase.
		closeAt := strings.Index(buf[start+2:], "*\"") + (start + 2)
		before := buf[:start+2]
		middle := buf[start+2 : closeAt]
		after := buf[closeAt:]
		middle = strings.ReplaceAll(middle, "*", "")
		middle = collapseSpaces(middle)
		out = append(out, before+middle+after)
		i = j
	}
	return out
}

// tdrPass6UnwrapParagraphs desenvuelve los párrafos: une las líneas
// consecutivas que no son estructura.
func tdrPass6UnwrapParagraphs(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		// El párrafo se acumula en un builder para no copiar la cadena en
		// cada unión (coste O(n²) en párrafos largos).
		var b strings.Builder
		b.WriteString(lines[i])
		cur := lines[i]

		for i+1 < len(lines) && !isTDRStructuralLine(cur) && !isTDRStructuralLine(lines[i+1]) {
			trimRight := strings.TrimRightFunc(cur, unicode.IsSpace)
			if trimRight == "" {
				break
			}
			r, _ := utf8.DecodeLastRuneInString(trimRight)
			if r == '.' || r == '!' || r == '?' || r == ':' {
				break
			}
			b.WriteString(joinerBetweenTDR(cur, lines[i+1]))
			b.WriteString(strings.TrimLeftFunc(lines[i+1], unicode.IsSpace))
			i++
			cur = b.String()
		}
		out = append(out, cur)
	}
	return out
}

// tdrPass7PostProcess es el remate: corrige énfasis anidado, aprieta los
// encabezados y colapsa los espacios sobrantes.
func tdrPass7PostProcess(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, "")
			continue
		}
		// Normalize weird nested emphasis artifacts inside bold wrappers.
		line = strings.ReplaceAll(line, `**"***`, `**"`)
		line = strings.ReplaceAll(line, `***"**`, `"**`)

		// Tighten headings.
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			line = collapseSpacedLetterRunsInHeading(line)
		}

		if !strings.HasPrefix(strings.TrimSpace(line), "![](") {
			line = collapseSpaces(line)
		}
		out = append(out, line)
	}

	// Collapse blank runs again in case the post-processing introduced empties.
	return collapseBlankLines(out)
}

// isTDRStructuralLine indica si la línea es estructura en el modo TDR.
func isTDRStructuralLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if isMarkdownStructureLine(trimmed) {
		return true
	}
	if boldOnlyLine.MatchString(trimmed) || italicOnlyLine.MatchString(trimmed) {
		return true
	}
	// Preserve explicit section labels like "1." "2." in indices.
	return indexNumber.MatchString(trimmed)
}

// isRepeatedTDRHeaderImage indica si la línea es una de las imágenes de
// cabecera que se repiten en cada página.
func isRepeatedTDRHeaderImage(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "![](") {
		return false
	}
	// Typical repeated page header images in these exports.
	for _, id := range tdrHeaderImages {
		if strings.Contains(trimmed, "[]("+id+")") {
			return true
		}
	}
	return false
}

// isRepeatedTDRHeaderLine indica si la línea es una cabecera institucional
// que se repite en cada página.
func isRepeatedTDRHeaderLine(line string) bool {
	// Normalize to compare across NBSP and spacing.
	n := strings.ToUpper(collapseSpaces(line))
	for _, header := range tdrHeaderLines {
		if n == header {
			return true
		}
	}
	return false
}

// combineSplitImageLine fusiona los atributos de una imagen partida entre dos
// líneas y devuelve la línea combinada con el índice de la siguiente.
func combineSplitImageLine(lines []string, i int) (string, int) {
	line := normalizeLine(lines[i])
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "![](") {
		return line, i + 1
	}
	// If attributes are split across two lines, merge them.
	if strings.Contains(line, "{width=") && !strings.Contains(line, "}") && i+1 < len(lines) {
		next := normalizeLine(lines[i+1])
		if strings.Contains(next, "height=") && strings.Contains(next, "}") {
			merged := strings.TrimRightFunc(line, unicode.IsSpace) + " " + strings.TrimSpace(next)
			merged = normalizeLine(merged)
			return merged, i + 2
		}
	}
	return line, i + 1
}

// joinerBetweenTDR decide el separador entre dos líneas del modo TDR.
func joinerBetweenTDR(a, b string) string {
	if endsWithUnicodeSpace(a) || startsWithUnicodeSpace(b) {
		return ""
	}
	atrim := strings.TrimRightFunc(a, unicode.IsSpace)
	if atrim != "" {
		r, _ := utf8.DecodeLastRuneInString(atrim)
		// Hyphenation or explicit separators should remain glued.
		if r == '-' || r == '/' {
			return ""
		}
	}
	btrim := strings.TrimLeftFunc(b, unicode.IsSpace)
	if btrim != "" {
		r, _ := utf8.DecodeRuneInString(btrim)
		switch r {
		case '.', ',', ';', ':', ')', ']', '}', '?', '!', '%':
			return ""
		}
	}
	// Default: these exports usually break between words.
	return " "
}

// normalizeLine limpia los artefactos de una línea: NBSP, marcador de salto
// de línea al final y espacios sobrantes.
func normalizeLine(line string) string {
	// Convert NBSP to normal spaces.
	line = strings.ReplaceAll(line, "\u00a0", " ")
	// Strip trailing markdown hard break markers like "\\" (common in exports).
	line = strings.TrimRightFunc(line, unicode.IsSpace)
	line = strings.TrimRight(line, "\\")
	line = strings.TrimRightFunc(line, unicode.IsSpace)
	return line
}

// collapseSpaces reduce los espacios múltiples a uno y recorta los bordes.
func collapseSpaces(s string) string {
	return strings.TrimSpace(twoPlusSpaces.ReplaceAllString(s, " "))
}

// collapseSpacedLetterRunsInHeading junta las letras sueltas mayúsculas de un
// encabezado: "D E L  N O M B R E" → "DEL NOMBRE".
func collapseSpacedLetterRunsInHeading(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return line
	}
	// Split on spaces and merge runs like "D E L" -> "DEL".
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return line
	}
	var out []string
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if len([]rune(p)) == 1 {
			r, _ := utf8.DecodeRuneInString(p)
			if r != utf8.RuneError && unicode.IsUpper(r) {
				j := i
				var run []string
				for j < len(parts) {
					pj := parts[j]
					if len([]rune(pj)) != 1 {
						break
					}
					rj, _ := utf8.DecodeRuneInString(pj)
					if rj == utf8.RuneError || !unicode.IsUpper(rj) {
						break
					}
					run = append(run, pj)
					j++
				}
				if len(run) >= 2 && len(run) <= 4 {
					out = append(out, strings.Join(run, ""))
					i = j - 1
					continue
				}
			}
		}
		out = append(out, p)
	}
	return strings.Join(out, " ")
}
