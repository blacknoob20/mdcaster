// Package fixtures construye los documentos de prueba que usan las suites de
// mdcaster.
//
// Vive fuera de los paquetes de producción porque solo lo necesitan los
// tests: los DOCX sintéticos se construyen a mano, con un ZIP y un
// word/document.xml mínimo.
package fixtures

import (
	"archive/zip"
	"os"
	"path/filepath"
)

// Header y Footer son las partes fijas de un word/document.xml mínimo, con los
// namespaces que usa el processor.
const (
	Header = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><w:body>`

	Footer = `</w:body></w:document>`
)

// DocWithBody devuelve las partes de un DOCX con el cuerpo indicado.
func DocWithBody(body string) map[string]string {
	return map[string]string{
		"word/document.xml": Header + body + Footer,
	}
}

// BuildDocx escribe un DOCX con las partes dadas en el directorio dir y
// devuelve la ruta del fichero.
//
// No usa `testing`: el helper vive en un paquete normal, no en un `_test.go`,
// para que lo compartan varias suites.
func BuildDocx(dir string, parts map[string]string) (string, error) {
	path := filepath.Join(dir, "prueba.docx")

	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, content := range parts {
		entry, err := writer.Create(name)
		if err != nil {
			writer.Close()
			return "", err
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			writer.Close()
			return "", err
		}
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	return path, nil
}
