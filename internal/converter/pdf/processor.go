package pdf

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Processor procesa archivos PDF
type Processor struct{}

// NewProcessor crea una nueva instancia del procesador PDF
func NewProcessor() *Processor {
	return &Processor{}
}

// Process convierte un PDF a Markdown (texto plano)
func (pp *Processor) Process(inputPath, outputPath string) error {
	text, err := pp.extractText(inputPath)
	if err != nil {
		return err
	}

	// Escribir el archivo Markdown
	if err := os.WriteFile(outputPath, []byte(text), 0644); err != nil {
		return fmt.Errorf("error escribiendo archivo Markdown: %w", err)
	}

	return nil
}

// extractText extrae el texto del PDF usando pdftotext
func (pp *Processor) extractText(inputPath string) (string, error) {
	// Intentar usar `pdftotext` si está disponible
	cmd := exec.Command("pdftotext", "-layout", inputPath, "-")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error extrayendo texto del PDF con pdftotext: %w. Instala 'poppler-utils' o asegura que 'pdftotext' esté en PATH", err)
	}

	return strings.TrimSpace(string(out)), nil
}
