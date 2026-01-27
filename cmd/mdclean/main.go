package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	flagInplace = flag.Bool("inplace", false, "Rewrite the input file in place")
	flagTDR     = flag.Bool("tdr", false, "Clean/format TDR-style Markdown exported from DOCX/PDF")
)

var twoPlusSpaces = regexp.MustCompile(`\s{2,}`)
var boldOnlyLine = regexp.MustCompile(`^\*\*(.+?)\*\*\s*$`)
var italicOnlyLine = regexp.MustCompile(`^\*(.+?)\*\s*$`)
var pageNumberLine = regexp.MustCompile(`^Pág\.?\s*\d+\s*$`)
var digitsOnlyLine = regexp.MustCompile(`^\d{1,3}$`)
var dotLeaderRun = regexp.MustCompile(`\s*\.{5,}\s*`)

func isStructuralLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	if strings.HasPrefix(trimmed, "![](") {
		return true
	}

	// Headings / section markers common in this corpus.
	// Keep conservative: don't join across these.
	structural := []*regexp.Regexp{
		regexp.MustCompile(`^NOTA GENERAL:$`),
		regexp.MustCompile(`^(TITULO|Capítulo|Sección)\b`),
		regexp.MustCompile(`^Art\.`),
		regexp.MustCompile(`^(Concordancias|Jurisprudencia):$`),
		regexp.MustCompile(`^(FUNCION EJECUTIVA|ASAMBLEA NACIONAL|PRESIDENCIA DE LA REPUBLICA)\b`),
		regexp.MustCompile(`^EL PLENO$`),
		regexp.MustCompile(`^CONSIDERANDO:$`),
	}
	for _, re := range structural {
		if re.MatchString(trimmed) {
			return true
		}
	}

	// Bullets / enumerations: keep line breaks.
	if regexp.MustCompile(`^[-*]\s+`).MatchString(trimmed) {
		return true
	}
	if regexp.MustCompile(`^[a-zA-Z]\)\s+`).MatchString(trimmed) {
		return true
	}
	if regexp.MustCompile(`^\d+\.-\s+`).MatchString(trimmed) {
		return true
	}

	// Page headers often centered; keep them as-is.
	if strings.Contains(trimmed, "LEXIS FINDER") {
		return true
	}
	return false
}

func isTDRStructuralLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if boldOnlyLine.MatchString(trimmed) || italicOnlyLine.MatchString(trimmed) {
		return true
	}
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	if strings.HasPrefix(trimmed, "![](") {
		return true
	}
	if regexp.MustCompile(`^[-*]\s+`).MatchString(trimmed) {
		return true
	}
	// Preserve explicit section labels like "1." "2." in indices.
	if regexp.MustCompile(`^\d+\.$`).MatchString(trimmed) {
		return true
	}
	return false
}

func isRepeatedTDRHeaderImage(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "![](") {
		return false
	}
	// Typical repeated page header images in these exports.
	for _, id := range []string{"media/rId40.png", "media/rId43.png", "media/rId46.png", "media/rId49.jpg", "media/rId52.png", "media/rId55.png"} {
		if strings.Contains(trimmed, "[]("+id+")") {
			return true
		}
	}
	return false
}

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

func looksWrappedLine(line string) bool {
	if len(line) >= 72 {
		return true
	}
	if strings.Contains(line, "\t") {
		return true
	}
	return twoPlusSpaces.MatchString(line)
}

func normalizeLine(line string) string {
	// Convert NBSP to normal spaces.
	line = strings.ReplaceAll(line, "\u00a0", " ")
	// Strip trailing markdown hard break markers like "\\" (common in exports).
	line = strings.TrimRightFunc(line, unicode.IsSpace)
	line = strings.TrimRight(line, "\\")
	line = strings.TrimRightFunc(line, unicode.IsSpace)
	return line
}

func collapseSpaces(s string) string {
	return strings.TrimSpace(twoPlusSpaces.ReplaceAllString(s, " "))
}

func isRepeatedTDRHeaderLine(line string) bool {
	// Normalize to compare across NBSP and spacing.
	n := strings.ToUpper(collapseSpaces(line))
	switch n {
	case "GAD MUNICIPAL DE MACHALA",
		"DIRECCIÓN DE TECNOLOGÍA DE LA INFORMACIÓN",
		"COORDINACIÓN DE DESARROLLO DE SISTEMAS",
		"COORDINACIÓN DE DESARROLLO DE SISTEMAS INFORMÁTICOS":
		return true
	default:
		return false
	}
}

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

func processTDR(lines []string) []string {
	// Pass 1: normalize, merge split image lines, remove page numbers, dedupe repeated headers,
	// normalize bullets and dot leaders.
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

	// Pass 2: collapse excessive blank lines.
	var collapsed []string
	prevBlank := false
	for _, line := range normalized {
		if strings.TrimSpace(line) == "" {
			if prevBlank {
				continue
			}
			prevBlank = true
			collapsed = append(collapsed, "")
			continue
		}
		prevBlank = false
		collapsed = append(collapsed, line)
	}

	// Pass 3: merge consecutive bold-only lines (common in title pages) into a single line.
	var mergedBold []string
	for i := 0; i < len(collapsed); i++ {
		trimmed := strings.TrimSpace(collapsed[i])
		m := boldOnlyLine.FindStringSubmatch(trimmed)
		if m == nil {
			mergedBold = append(mergedBold, collapsed[i])
			continue
		}
		parts := []string{collapseSpaces(m[1])}
		for i+1 < len(collapsed) {
			nextTrim := strings.TrimSpace(collapsed[i+1])
			nm := boldOnlyLine.FindStringSubmatch(nextTrim)
			if nm == nil {
				break
			}
			parts = append(parts, collapseSpaces(nm[1]))
			i++
		}
		mergedBold = append(mergedBold, "**"+strings.Join(parts, " ")+"**")
	}

	// Pass 4: convert bold-only lines to headings where appropriate.
	var headed []string
	hasMainTitle := false
	for _, line := range mergedBold {
		trimmed := strings.TrimSpace(line)
		m := boldOnlyLine.FindStringSubmatch(trimmed)
		if m == nil {
			headed = append(headed, line)
			continue
		}
		content := collapseSpaces(m[1])
		// Some exports produce nested emphasis like **"***text***"**; normalize it.
		content = strings.ReplaceAll(content, "*", "")
		upper := strings.ToUpper(content)
		if upper == "INDICE" {
			headed = append(headed, "## INDICE")
			continue
		}
		if upper == "CONTENIDO" {
			headed = append(headed, "### CONTENIDO")
			continue
		}
		if !hasMainTitle && strings.Contains(upper, "TÉRMINOS") {
			headed = append(headed, "# "+content)
			hasMainTitle = true
			continue
		}
		// Common section headers in this document.
		if regexp.MustCompile(`^(GOBIERNO|GAD|MUNICIPAL|DIRECCIÓN|COORDINACIÓN|SECCIÓN|OBJETO|NOMBRE|CONTRATACIÓN|TÉRMINOS|ANTECEDENTES|JUSTIFICACION|OBJETIVOS|ALCANCE|METODOLOGÍA|INFORMACIÓN|PRODUCTOS|PLAZO|PERSONAL|FORMA|CONDICIONES|ANEXO)`).MatchString(upper) {
			headed = append(headed, "## "+content)
			continue
		}
		// Otherwise keep as bold but without the export artifacts.
		headed = append(headed, "**"+content+"**")
	}

	// Pass 4b: normalize table of contents formatting.
	headed = normalizeTDRTOC(headed)

	// Pass 5: merge consecutive italic-only lines into one italic paragraph.
	var italics []string
	for i := 0; i < len(headed); i++ {
		trimmed := strings.TrimSpace(headed[i])
		m := italicOnlyLine.FindStringSubmatch(trimmed)
		if m == nil {
			italics = append(italics, headed[i])
			continue
		}
		parts := []string{collapseSpaces(m[1])}
		for i+1 < len(headed) {
			nextTrim := strings.TrimSpace(headed[i+1])
			nm := italicOnlyLine.FindStringSubmatch(nextTrim)
			if nm == nil {
				break
			}
			parts = append(parts, collapseSpaces(nm[1]))
			i++
		}
		italics = append(italics, "*"+strings.Join(parts, " ")+"*")
	}

	// Pass 5b: fix quoted-italic spans broken across lines.
	italics = fixQuotedItalicSpans(italics)

	// Pass 6: unwrap paragraphs (reduce line count) but keep structure lines intact.
	out := make([]string, 0, len(italics))
	for i := 0; i < len(italics); i++ {
		cur := italics[i]
		for i+1 < len(italics) && !isTDRStructuralLine(cur) && !isTDRStructuralLine(italics[i+1]) {
			trimRight := strings.TrimRightFunc(cur, unicode.IsSpace)
			if trimRight == "" {
				break
			}
			r, _ := utf8.DecodeLastRuneInString(trimRight)
			if r == '.' || r == '!' || r == '?' || r == ':' {
				break
			}
			cur = cur + joinerBetweenTDR(cur, italics[i+1]) + strings.TrimLeftFunc(italics[i+1], unicode.IsSpace)
			i++
		}
		out = append(out, cur)
	}

	// Pass 7: final tidy (spacing, headings, emphasis artifacts).
	return postProcessTDRLines(out)
}

func normalizeTDRTOC(lines []string) []string {
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
			if regexp.MustCompile(`^\d+\.$`).MatchString(trimmed) {
				j := i + 1
				for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
					j++
				}
				if j < len(lines) {
					next := strings.TrimSpace(lines[j])
					merged := trimmed + " " + next
					merged = dotLeaderRun.ReplaceAllString(merged, " … ")
					merged = collapseSpaces(merged)
					out = append(out, merged)
					i = j
					continue
				}
			}

			// If a TOC entry ends with dots and next line is a bare number, treat it as a page number.
			if strings.HasSuffix(strings.TrimRightFunc(trimmed, unicode.IsSpace), ".") {
				j := i + 1
				for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
					j++
				}
				if j < len(lines) {
					nextTrim := strings.TrimSpace(lines[j])
					if digitsOnlyLine.MatchString(nextTrim) {
						merged := trimmed + " " + nextTrim
						merged = dotLeaderRun.ReplaceAllString(merged, " … ")
						merged = collapseSpaces(merged)
						out = append(out, merged)
						i = j
						continue
					}
				}
			}

			line = dotLeaderRun.ReplaceAllString(line, " … ")
			line = collapseSpaces(line)
			out = append(out, line)
			continue
		}

		out = append(out, line)
	}
	return out
}

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
		for ; j < len(lines); j++ {
			buf = buf + " " + strings.TrimSpace(lines[j])
			if strings.Contains(lines[j], "*\"") {
				break
			}
		}
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

func postProcessTDRLines(lines []string) []string {
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
	final := make([]string, 0, len(out))
	prevBlank := false
	for _, line := range out {
		if strings.TrimSpace(line) == "" {
			if prevBlank {
				continue
			}
			prevBlank = true
			final = append(final, "")
			continue
		}
		prevBlank = false
		final = append(final, line)
	}
	return final
}

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
	switch left {
	case "de", "del", "la", "el", "y", "o", "a", "al", "en", "por", "para", "con", "sin",
		"un", "una", "unos", "unas", "los", "las", "lo", "que", "se", "su", "sus", "mi", "mis",
		"tu", "tus", "ya", "no":
		return " "
	}
	switch right {
	case "de", "del", "la", "el", "y", "o", "a", "al", "en", "por", "para", "con", "sin",
		"un", "una", "unos", "unas", "los", "las", "lo", "que", "se", "su", "sus", "mi", "mis",
		"tu", "tus":
		return " "
	}

	// Default: assume it was a word split.
	_ = left
	_ = right
	return ""
}

func firstWordToken(s string) string {
	// Walk from start to find a contiguous run of letters.
	var token []rune
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
		if unicode.IsSpace(r) {
			continue
		}
		// Skip punctuation/digits at start.
	}
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError {
			break
		}
		if !unicode.IsLetter(r) {
			break
		}
		token = append(token, r)
		s = s[size:]
	}
	return string(token)
}

func lastWordToken(s string) string {
	// Walk from end to find a contiguous run of letters.
	var token []rune
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
		if unicode.IsSpace(r) {
			continue
		}
		// Skip punctuation/digits at end.
	}
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r == utf8.RuneError {
			break
		}
		if !unicode.IsLetter(r) {
			break
		}
		token = append(token, r)
		s = s[:len(s)-size]
	}
	// token is reversed.
	for i, j := 0, len(token)-1; i < j; i, j = i+1, j-1 {
		token[i], token[j] = token[j], token[i]
	}
	return string(token)
}

func startsWithUnicodeSpace(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r != utf8.RuneError && unicode.IsSpace(r)
}

func endsWithUnicodeSpace(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r != utf8.RuneError && unicode.IsSpace(r)
}

func readAllLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	// Increase max token size for long lines.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func process(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		cur := lines[i]
		for i+1 < len(lines) && shouldJoin(cur, lines[i+1]) {
			cur = cur + joinerBetween(cur, lines[i+1]) + lines[i+1]
			i++
		}
		out = append(out, cur)
	}
	return out
}

func writeLines(w io.Writer, lines []string) error {
	bw := bufio.NewWriter(w)
	for i, line := range lines {
		if _, err := bw.WriteString(line); err != nil {
			return err
		}
		// Keep trailing newline at end of file.
		if i < len(lines)-1 {
			if err := bw.WriteByte('\n'); err != nil {
				return err
			}
		}
	}
	if len(lines) > 0 {
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Uso: %s [--inplace] <archivo.md>\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	path := flag.Arg(0)
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	lines, err := readAllLines(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var clean []string
	if *flagTDR {
		clean = processTDR(lines)
	} else {
		clean = process(lines)
	}

	if *flagInplace {
		// In shared folders (e.g., vboxsf), atomic rename over an existing file may fail.
		// Use a direct truncate+write to keep behavior consistent.
		outFile, err := os.Create(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := writeLines(outFile, clean); err != nil {
			_ = outFile.Close()
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := outFile.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := writeLines(os.Stdout, clean); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
