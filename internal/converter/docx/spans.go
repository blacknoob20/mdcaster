package docx

import (
	"regexp"
	"strings"
)

// renderSpansMarkdown convierte los fragmentos de un párrafo a Markdown.
//
// Los espacios que rodean al texto del fragmento quedan fuera de las marcas
// de énfasis: "**texto **" no es énfasis válido en Markdown, "**texto** " sí.
func renderSpansMarkdown(spans []inlineSpan) string {
	var sb strings.Builder
	for _, s := range spans {
		text := s.text
		if text == "" {
			continue
		}
		if s.raw {
			sb.WriteString(text)
			continue
		}

		lead := text[:len(text)-len(strings.TrimLeft(text, " \t\n"))]
		core := strings.TrimSpace(text)
		if core == "" {
			sb.WriteString(text)
			continue
		}
		trail := text[len(lead)+len(core):]

		if s.bold {
			core = "**" + core + "**"
		}
		if s.italic {
			core = "*" + core + "*"
		}
		if s.strike {
			core = "~~" + core + "~~"
		}
		if s.underline {
			// El subrayado no existe en Markdown: se emite como HTML en
			// línea, por fuera del énfasis, para que las marcas de Markdown
			// de dentro sigan interpretándose.
			core = "<u>" + core + "</u>"
		}

		sb.WriteString(lead)
		sb.WriteString(core)
		sb.WriteString(trail)
	}
	return sb.String()
}

// renderSpansHTML convierte los fragmentos de un párrafo a HTML en línea. Se
// usa dentro de bloques <ol>, donde Markdown no se procesa.
func renderSpansHTML(spans []inlineSpan) string {
	var sb strings.Builder
	for _, s := range spans {
		text := s.text
		if text == "" {
			continue
		}
		if s.raw {
			sb.WriteString(text)
			continue
		}
		if s.bold {
			text = "<strong>" + text + "</strong>"
		}
		if s.italic {
			text = "<em>" + text + "</em>"
		}
		if s.strike {
			text = "<del>" + text + "</del>"
		}
		if s.underline {
			text = "<u>" + text + "</u>"
		}
		sb.WriteString(text)
	}
	return sb.String()
}

// emptyBoldAre detecta marcas de negrita vacías. Se compila una sola vez:
// cleanFormatting se llama por cada párrafo del documento.
var emptyBoldAre = regexp.MustCompile(`\*\*\s*\*\*`)

// cleanFormatting limpia el formato duplicado
func cleanFormatting(content string) string {
	// Limpiar ** vacíos
	content = emptyBoldAre.ReplaceAllString(content, "")

	return content
}
