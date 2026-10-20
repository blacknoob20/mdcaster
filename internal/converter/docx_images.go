package converter

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// imagePlaceholder marca, dentro de un párrafo, el punto en que hay una
// imagen. Se sustituye por el enlace Markdown cuando se ha extraído el
// fichero.
const imagePlaceholder = "\x00image:"

// extractDocxImages extrae las imágenes referenciadas por el cuerpo del
// documento a una carpeta contigua al Markdown, y devuelve el mapa
// relID → imagen con su ruta definitiva.
//
// Las imágenes cuyo relationship no existe, apunta fuera del paquete o no se
// puede leer se omiten: es preferible un enlace que no exista a un enlace
// roto.
func extractDocxImages(reader *zip.Reader, refs []docxImageRef, rels map[string]docxRelationship, outputPath string) (map[string]docxImageRef, error) {
	if len(refs) == 0 {
		return nil, nil
	}

	assetsDir := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "_assets"
	assetsName := filepath.Base(assetsDir)

	result := make(map[string]docxImageRef, len(refs))
	used := make(map[string]bool, len(refs))
	created := false

	for _, ref := range refs {
		if _, done := result[ref.relID]; done {
			continue
		}

		rel, ok := rels[ref.relID]
		if !ok || rel.TargetMode == "External" {
			continue
		}

		name, data, err := readDocxMedia(reader, rel.Target)
		if err != nil {
			continue
		}

		unique := uniqueAssetName(name, used)
		used[unique] = true

		if !created {
			if err := os.MkdirAll(assetsDir, 0755); err != nil {
				return nil, fmt.Errorf("error creando la carpeta de imágenes: %w", err)
			}
			created = true
		}

		if err := os.WriteFile(filepath.Join(assetsDir, unique), data, 0644); err != nil {
			return nil, fmt.Errorf("error escribiendo la imagen %s: %w", unique, err)
		}

		out := ref
		out.path = assetsName + "/" + unique
		result[ref.relID] = out
	}

	return result, nil
}

// readDocxMedia lee un fichero de media del paquete OOXML.
//
// El target de la relación es relativo a la carpeta de la parte (word/), y
// puede salir de ella con "../": se resuelve y se descarta si el resultado
// se sale del paquete.
func readDocxMedia(reader *zip.Reader, target string) (string, []byte, error) {
	cleaned := path.Clean(path.Join("word", target))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", nil, fmt.Errorf("ruta de imagen fuera del paquete: %s", target)
	}

	for _, file := range reader.File {
		if file.Name != cleaned {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return "", nil, err
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return "", nil, err
		}

		return sanitizeAssetName(path.Base(cleaned)), data, nil
	}

	return "", nil, fmt.Errorf("no se encontró la imagen %s", target)
}

// sanitizeAssetName deja un nombre de fichero seguro para el sistema de
// ficheros.
func sanitizeAssetName(name string) string {
	var sb strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteRune('_')
		}
	}

	cleaned := sb.String()
	if cleaned == "" || strings.HasPrefix(cleaned, ".") {
		return "imagen"
	}

	return cleaned
}

// uniqueAssetName evita que dos imágenes con el mismo nombre se pisen.
func uniqueAssetName(name string, used map[string]bool) string {
	if !used[name] {
		return name
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if !used[candidate] {
			return candidate
		}
	}
}

// resolveDocxImages sustituye los marcadores de imagen de los párrafos por el
// enlace Markdown definitivo.
func resolveDocxImages(doc *docxDocument, images map[string]docxImageRef) {
	for _, block := range doc.blocks {
		if block.kind != blockParagraph || block.para == nil {
			continue
		}
		resolveParagraphImages(block.para, images)
	}
}

// resolveParagraphImages sustituye los marcadores de imagen de un párrafo.
func resolveParagraphImages(para *docxParagraph, images map[string]docxImageRef) {
	if len(para.spans) == 0 {
		return
	}

	changed := false
	for i := range para.spans {
		span := &para.spans[i]
		if !span.raw || !strings.HasPrefix(span.text, imagePlaceholder) {
			continue
		}

		relID := strings.TrimPrefix(span.text, imagePlaceholder)
		img, ok := images[relID]
		if !ok {
			span.text = ""
			changed = true
			continue
		}

		span.text = fmt.Sprintf("![%s](%s)", img.alt, img.path)
		changed = true
	}

	if changed {
		para.text = cleanFormatting(strings.TrimSpace(renderSpansMarkdown(para.spans)))
	}
}
