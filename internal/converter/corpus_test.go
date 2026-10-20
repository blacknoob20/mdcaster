package converter

import (
	"os"
	"path/filepath"
	"testing"
)

// corpusPaths busca ficheros del corpus local en las rutas habituales: la raíz
// del módulo y la ruta relativa al paquete de test.
//
// El corpus (word/, pdf/) está en .gitignore: es material local que no se
// versiona. Los tests se omiten cuando no está disponible.
func corpusPaths(tb testing.TB, dir, pattern string) []string {
	tb.Helper()

	bases := []string{
		filepath.Join("..", "..", dir),
		dir,
	}

	for _, base := range bases {
		matches, err := filepath.Glob(filepath.Join(base, pattern))
		if err != nil {
			continue
		}

		var found []string
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() {
				continue
			}
			found = append(found, match)
		}

		if len(found) > 0 {
			return found
		}
	}

	return nil
}

// requireCorpusFile devuelve el primer fichero del corpus o omite el test.
func requireCorpusFile(tb testing.TB, dir, pattern string) string {
	tb.Helper()

	files := corpusPaths(tb, dir, pattern)
	if len(files) == 0 {
		tb.Skipf("sin corpus en %s/ (%s). El corpus es local y no se versiona.", dir, pattern)
	}

	return files[0]
}
