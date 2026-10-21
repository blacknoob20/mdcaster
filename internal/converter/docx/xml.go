package docx

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// Espacios de nombres OOXML usados por el parser.
const (
	nsWord        = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsWordStrict  = "http://purl.oclc.org/ooxml/wordprocessingml/main"
	nsDrawingML   = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsWordDrawing = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsVML         = "urn:schemas-microsoft-com:vml"
	nsRel         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// isWordNamespace indica si un espacio de nombres corresponde a
// WordprocessingML.
//
// Un prefijo sin resolver (Go deja el prefijo en Name.Space cuando el
// namespace no está declarado) no contiene "://": se asume Word, porque es el
// caso habitual en DOCX reales. Los namespaces de otros formatos (DrawingML,
// VML) sí son URIs y quedan fuera.
func isWordNamespace(space string) bool {
	switch space {
	case nsWord, nsWordStrict, "":
		return true
	}
	return !strings.Contains(space, "://")
}

// attrValue devuelve el valor del primer atributo con ese nombre local.
func attrValue(t xml.StartElement, name string) string {
	for _, attr := range t.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

// attrValueNS devuelve el valor del atributo con ese nombre local en el
// espacio de nombres indicado.
func attrValueNS(t xml.StartElement, space, name string) string {
	for _, attr := range t.Attr {
		if attr.Name.Local == name && attr.Name.Space == space {
			return attr.Value
		}
	}
	return ""
}

// attrBool interpreta el atributo de activación de una propiedad de run.
// Sin atributo, la propiedad está activada; "0", "false" y "off" la
// desactivan.
func attrBool(t xml.StartElement, name string, def bool) bool {
	value := attrValue(t, name)
	if value == "" {
		return def
	}
	switch strings.ToLower(value) {
	case "0", "false", "off":
		return false
	}
	return true
}

// attrUnderline interpreta el valor de <w:u>. El subrayado no tiene
// equivalente en Markdown: se emite como HTML en línea.
func attrUnderline(t xml.StartElement) bool {
	value := strings.ToLower(strings.TrimSpace(attrValue(t, "val")))
	switch value {
	case "":
		// Sin valor: subrayado simple.
		return true
	case "none", "0", "false", "off":
		return false
	}
	return true
}

// atoiDefault convierte a entero con un valor por defecto.
func atoiDefault(value string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return def
	}
	return n
}

// firstNonEmpty devuelve el primer argumento no vacío.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
