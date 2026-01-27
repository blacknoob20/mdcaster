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
)

var twoPlusSpaces = regexp.MustCompile(`\s{2,}`)

func isStructuralLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
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

	clean := process(lines)

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
