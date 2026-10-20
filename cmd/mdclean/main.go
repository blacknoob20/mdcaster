// Comando mdclean: limpia y normaliza un fichero Markdown ya convertido.
//
// La lógica de cada modo vive en su propio fichero:
//
//	generic.go  modo genérico: une líneas partidas por un extractor de texto
//	tdr.go      modo TDR: normaliza el Markdown del corpus institucional
//	corpus.go   datos del corpus institucional (cabeceras, imágenes, secciones)
//	markdown.go primitivas compartidas por los dos modos
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	flagInplace = flag.Bool("inplace", false, "Rewrite the input file in place")
	flagTDR     = flag.Bool("tdr", false, "Clean/format TDR-style Markdown exported from DOCX/PDF")
)

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

	// El frontmatter YAML se aparta: son metadatos, no texto que se pueda
	// unir ni normalizar.
	frontmatter, body := splitFrontmatter(lines)

	var clean []string
	if *flagTDR {
		clean = processTDR(body)
	} else {
		clean = process(body)
	}

	clean = append(frontmatter, clean...)

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

// readAllLines lee el contenido completo de r en líneas.
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

// writeLines escribe las líneas separadas por "\n" y añade siempre un salto
// de línea final.
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

// splitFrontmatter separa el bloque de frontmatter YAML del principio del
// documento.
//
// El bloque se devuelve tal cual, sin normalizar: son metadatos, no texto.
// Si el documento no empieza por un delimitador "---", no hay frontmatter.
func splitFrontmatter(lines []string) (frontmatter []string, body []string) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, lines
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[:i+1], lines[i+1:]
		}
	}

	// Frontmatter sin cerrar: no se toca nada.
	return nil, lines
}
