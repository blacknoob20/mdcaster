package converter

import (
	"os"
	"regexp"
	"strings"
)

// CheckIntegrity verifica qué porcentaje del contenido original se conserva en
// el Markdown.
//
// Reutiliza el texto que el procesador ya extrajo al convertir, de modo que no
// hace falta parsear el documento por segunda vez.
//
// Si el formato no sabe medir su contenido (un PDF), el resultado es 0: el
// porcentaje no significa nada. Ver [P4] en ARCHITECTURE.md.
//
// Precondición: solo es válido si se llama inmediatamente después de convertir
// con la **misma** `Converter`, porque el texto pertenece al estado del
// procesador.
//
// [P4]: ../../../ARCHITECTURE.md#p4--el-pdf-se-muestra-como-0--en-vez-de-no-medido
func (c *Converter) CheckIntegrity(sourceFile, markdownFile string) float64 {
	processor, err := c.factory.GetProcessor(sourceFile)
	if err != nil {
		return 0
	}

	extractor, ok := processor.(TextExtractor)
	if !ok {
		return 0
	}

	return CheckIntegrityAgainst(extractor.ExtractedText(), markdownFile)
}

// CheckIntegrityAgainst verifica la integridad usando un texto original ya
// extraído. Permite reutilizar el texto que el procesador leyó al convertir y
// evitar un segundo parseo del documento.
//
// Un original vacío no es verificable: el resultado es 100.
func CheckIntegrityAgainst(originalContent, markdownFile string) float64 {
	// Leer el contenido del markdown generado
	markdownContent, err := os.ReadFile(markdownFile)
	if err != nil {
		return 0
	}

	// Normalizar espacios para comparación
	original := normalizeContent(originalContent)

	// Si no hay contenido original, no se puede verificar
	if len(original) == 0 {
		return 100
	}

	// Contar cuántas palabras del original se encuentran en el markdown.
	//
	// D10: la coincidencia es por token exacto, no por subcadena. Los tokens
	// del Markdown se indexan una sola vez, de modo que el coste es lineal
	// en el número de palabras y no el producto de ambos.
	originalWords := strings.Fields(original)
	if len(originalWords) == 0 {
		return 100
	}

	markdownTokens := make(map[string]struct{}, len(originalWords))
	for _, token := range strings.Fields(normalizeContent(string(markdownContent))) {
		markdownTokens[token] = struct{}{}
	}

	foundCount := 0
	for _, word := range originalWords {
		if _, found := markdownTokens[word]; found {
			foundCount++
		}
	}

	percent := float64(foundCount) / float64(len(originalWords)) * 100
	return percent
}

// nonAlphanumericAre es la expresión regular de normalización del contenido.
// Se compila una sola vez: normalizeContent se llama por cada verificación
// de integridad.
var nonAlphanumericAre = regexp.MustCompile(`[^a-z0-9\s]`)

// normalizeContent normaliza el contenido para comparación
func normalizeContent(content string) string {
	// Convertir a minúsculas
	content = strings.ToLower(content)
	// Eliminar caracteres especiales pero mantener espacios
	content = nonAlphanumericAre.ReplaceAllString(content, "")
	// Normalizar espacios en blanco
	content = strings.Join(strings.Fields(content), " ")
	return content
}
