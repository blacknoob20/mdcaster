# SPEC — `internal/converter/integrity.go` (Verificación de integridad)

**Módulo:** `converter/integrity`
**Código:** `internal/converter/integrity.go`
**Depende de:** `internal/converter/processor.go` (`TextExtractor`)

---

## Responsabilidad

Medir qué porcentaje del contenido del original se conserva en el Markdown generado. **No parsea
el documento**: reutiliza el texto que el processor ya extrajo al convertir.

La semántica de la métrica (token exacto, qué se cuenta, qué significa cada valor) está en
[`quality/integridad.md`](../quality/integridad.md). Aquí se documenta la API.

## API

| Función | Líneas | Contrato |
|---|---|---|
| `(*Converter).CheckIntegrity(source, markdown string) float64` | 23-35 | Verifica el original contra su Markdown. |
| `CheckIntegrityAgainst(originalContent, markdown string) float64` | 42-81 | Verifica un texto original ya extraído. |
| `normalizeContent(content string) string` | 89-97 | Normaliza el contenido para comparar. |

### `Converter.CheckIntegrity` — `internal/converter/integrity.go:23-35`

```go
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
```

Es el método que usa la CLI.

| Situación | Resultado |
|---|---|
| El processor implementa `TextExtractor` y tiene texto | La cobertura de ese texto |
| El processor implementa `TextExtractor` y **no** tiene texto (documento vacío) | `100`: no hay nada que verificar |
| El processor **no** implementa `TextExtractor` (un PDF) | `0`: el porcentaje no significa nada (ver [P4]) |
| La extensión no tiene processor | `0` |

**Precondición:** solo es válido si se llama inmediatamente después de convertir con la **misma**
`Converter`, porque el texto pertenece al estado del processor.

**No hay una segunda vía.** Antes, si el processor no tenía texto, la librería releía el original
desde disco con un parser propio (`extractDocxContent`). Esa vía duplicaba el parseo del DOCX y se
ha eliminado: el texto del processor es la única fuente. La garantía de que no pierde nada la
comprueba `TestDocx_ExtractedTextMatchesIndependentRead` (`internal/converter/docx/reader_test.go`),
que lee el original por una vía independiente y compara las dos cifras.

### `CheckIntegrityAgainst` — `internal/converter/integrity.go:42-81`

```
1. os.ReadFile(markdownFile)     → error → 0
2. normalizeContent(original)     → si queda vacío → 100 (no verificable)
3. strings.Fields(original)       → tokens del original
4. Se indexan los tokens del Markdown en un conjunto
5. foundCount = tokens del original que están en el conjunto
6. percent = foundCount / len(originalWords) * 100
```

La coincidencia es por **token exacto** (D10), no por subcadena. Los tokens del Markdown se
indexan una sola vez: el coste es lineal en el número de palabras, no el producto de ambos.

### `normalizeContent` — `internal/converter/integrity.go:83-97`

```
1. strings.ToLower(content)
2. Eliminar todo lo que no sea [a-z0-9\s]      ← regex: [^a-z0-9\s]
3. strings.Join(strings.Fields(content), " ")   ← colapsar espacios
```

La regex se compila **una sola vez**, a nivel de paquete (`nonAlphanumericAre`): la función se
llama dos veces por verificación.

## Gotchas

1. El PDF da `0`, no "no medido": la CLI lo muestra con ❌ aunque la conversión sea correcta
   (ver [P4]).
2. Un documento vacío da `100`, no `0`: no hay contenido que perder.
3. `normalizeContent` **borra** los acentos y la `ñ` en vez de normalizarlos: `técnica` → `tcnica`
   (ver [P6](../../../ARCHITECTURE.md#pendientes)).
4. La métrica no mide estructura, solo texto: el contenido de una tabla cuenta igual se haya
   convertido en tabla o en texto plano.
