# SPEC — `internal/converter/docx_processor.go` (DOCX → Markdown)

**Módulo:** `converters/docx`
**Código:** `internal/converter/docx_processor.go`
**Implementa:** `converter.FileProcessor`
**Dependencias:** solo biblioteca estándar (`archive/zip`, `encoding/xml`)

---

## Responsabilidad

Convertir un `.docx` (paquete OOXML = ZIP con XML) a Markdown preservando la estructura de
párrafos, los estilos de título, el énfasis y las listas.

## Entrada / salida

| Aspecto | Contrato |
|---|---|
| Entrada | Ruta a un `.docx` válido (ZIP con `word/document.xml`) |
| Salida | Fichero Markdown en `outputPath`, permisos `0644` |
| Extensión registrada | `.docx` (minúsculas) |

## Estructura del paquete OOXML leída

| Entrada del ZIP | Uso | Obligatoria |
|---|---|---|
| `word/document.xml` | Cuerpo del documento | **Sí** → si falta: `ERR_DOCX_NO_DOCUMENT_XML` |
| `word/_rels/document.xml.rels` | Relaciones (hipervínculos) | No (opcional) |

Cualquier otra entrada del ZIP se ignora.

## Flujo de `Process` — `internal/converter/docx_processor.go:30-84`

```
1. Reset de dp.relationships y dp.lastText  ← aisla el estado entre ficheros
2. zip.OpenReader(inputPath)                → ERR_DOCX_OPEN
3. Lectura de document.xml y document.xml.rels
4. Si no hay document.xml → ERR_DOCX_NO_DOCUMENT_XML
5. parseRelationships(rels)                 ← opcional, solo hipervínculos
6. parseDocument(documentContent)           → ERR_DOCX_PARSE
7. dp.lastText = texto plano extraído
8. os.WriteFile(outputPath, markdown, 0644) → ERR_WRITE_MARKDOWN
```

El reset del paso 1 es obligatorio: `DocxProcessor` guarda estado en el struct y la misma
instancia puede procesar varios ficheros (la CLI crea un `Converter` por worker, pero un worker
procesa varios ficheros con la misma instancia).

### `parseDocument` devuelve dos valores

```go
func (dp *DocxProcessor) parseDocument(content []byte) (string, string, error)
```

Devuelve el **Markdown** y el **texto plano** del documento. El texto plano se acumula en el
mismo recorrido del XML (cada `<w:t>` aporta su texto y un espacio) y se guarda en `dp.lastText`,
para que la verificación de integridad no tenga que parsear el DOCX por segunda vez.

El texto plano se expone con `ExtractedText()`, que implementa la interfaz opcional
[`TextExtractor`](../converter/factory.md). Ver [../quality/integridad.md](../quality/integridad.md).

## Reglas de parseo del documento — `parseDocument` (líneas 128-259)

Recorre el XML con `xml.Decoder` y usa un **decoder de tokens**, no `Unmarshal`, para poder
detectar el cierre de cada elemento.

### Elementos StartElement

| Elemento | Efecto | Referencia |
|---|---|---|
| `p` | Inicia párrafo: resetea texto, estilo, flag de lista y nivel | 157-162 |
| `pStyle` | Guarda el valor del atributo `val` como `paragraphStyle` | 163-168 |
| `numPr` | Marca el párrafo como elemento de lista | 169-170 |
| `ilvl` | Lee el atributo `val` como nivel de lista (int) | 171-176 |
| `b` | Activa negrita | 177-178 |
| `i` | Activa cursiva | 179-180 |
| `strike` | Activa tachado | 181-182 |
| `t` | Apila un buffer de texto | 183-184 |
| `tab` | Escribe **2 espacios** en el párrafo | 185-188 |
| `br` | Escribe `\n` en el párrafo | 189-192 |

### Elementos EndElement

| Elemento | Efecto | Referencia |
|---|---|---|
| `t` | Saca el buffer, acumula el texto plano, aplica énfasis y lo escribe en el párrafo | 205-231 |
| `r` | **Resetea** negrita, cursiva y tachado (fin de run) | 232-236 |
| `p` | Cierra el párrafo: trim, limpieza, estilo y escritura | 237-252 |

### CharData

Se acumula en el buffer de texto del tope de la pila (`textStack`). Si la pila está vacía
(texto fuera de un `<w:t>`), el contenido se descarta.

### Formato aplicado al cerrar `<w:t>`

Orden de aplicación (líneas 217-225):

1. Negrita → `**texto**`
2. Cursiva → `*texto*`
3. Tachado → `~~texto~~`

El texto vacío se descarta: no se emiten marcas de énfasis sin contenido.

## Cierre de párrafo — `internal/converter/docx_processor.go:237-252`

```
1. content = TrimSpace(currentParagraph)
2. content = cleanFormatting(content)
3. Si content == "" → el párrafo NO se escribe (párrafo vacío se descarta)
4. line = applyParagraphStyle(content, paragraphStyle, isListItem, listLevel)
5. Se escribe line + "\n"
6. Si NO es lista → se escribe un "\n" adicional (línea en blanco entre párrafos)
```

Consecuencia: dos párrafos de texto normal quedan separados por **una línea en blanco**; los
elementos de lista quedan **contiguos**.

## `applyParagraphStyle` — `internal/converter/docx_processor.go:267-297`

Se evalúa en este orden (el primero que coincide gana):

| # | Condición (sobre `strings.ToLower(style)`) | Salida |
|---|---|---|
| 1 | contiene `heading1` \| `título1` \| `ttulo1` | `# ` + contenido |
| 2 | … `heading2` \| `título2` \| `ttulo2` | `## ` + contenido |
| 3 | … `heading3` \| `título3` \| `ttulo3` | `### ` + contenido |
| 4 | … `heading4` \| `título4` \| `ttulo4` | `#### ` + contenido |
| 5 | … `heading5` \| `título5` \| `ttulo5` | `##### ` + contenido |
| 6 | … `heading6` \| `título6` \| `ttulo6` | `###### ` + contenido |
| 7 | contiene `title` \| `título` \| `ttulo` | `# ` + contenido |
| 8 | contiene `subtitle` | `## ` + contenido |
| 9 | `isList == true` | `"  " * nivel` + `"- "` + contenido |
| 10 | resto | contenido sin prefijo |

Reglas:

1. La detección es por **subcadena** del nombre del estilo, no por igualdad. `Heading1` se detecta
   antes que `title` porque las reglas de heading se evalúan primero.
2. Los alias `título` / `ttulo` cubren estilos con y sin tilde (el nombre del estilo depende del
   idioma de Word).
3. El nivel de lista viene de `w:ilvl`; nivel 0 → sin sangrado, nivel 1 → 2 espacios, etc.
4. Un párrafo con estilo de título **y** `numPr` se renderiza como título: las reglas de heading
   se evalúan antes que las de lista.

## Post-proceso del Markdown

### `cleanFormatting` — `internal/converter/docx_processor.go:304-312`

1. `****` → `**` (colapsa énfasis duplicado).
2. Elimina las marcas `**` vacías (`** **`).

### `cleanMarkdown` — `internal/converter/docx_processor.go:315-334`

1. Colapsa 3+ saltos de línea consecutivos → 2.
2. Elimina espacios y tabulaciones al final de cada línea.
3. `****` → `**` (segunda pasada, por si el formato se reintrodujo).
4. `**\n**` → `\n` (elimina énfasis vacío partido en dos líneas).
5. `TrimSpace` del resultado completo.

El resultado final **no** empieza ni termina con líneas en blanco.

## Tablas

El parser **no** interpreta la estructura de tablas de Word: no hay lógica de `<w:tbl>`, `<w:tr>`
ni `<w:tc>`. El texto de las celdas sí se captura (vive en `<w:t>`) y se emite como párrafos
planos, **sin** estructura de tabla Markdown.

Esto es intencional: el README declara "sin tablas Markdown complejas, solo texto fácil de leer".

## Hipervínculos

`parseRelationships` (líneas 110-121) guarda **solo** las relaciones cuyo `Type` contiene
`hyperlink`, indexadas por `Id` → `Target`.

**Limitación vigente:** ese mapa no se usa en ninguna fase posterior. Los hipervínculos del DOCX
**se pierden** en el Markdown generado. Guardar el mapa no equivale a emitir los enlaces.

## Utilidades exported

| Función | Línea | Contrato |
|---|---|---|
| `IsDocxFile(path string) bool` | 337-341 | `true` si la extensión (minúsculas) es `.docx`. |
| `GetDocxFiles(dir string) ([]string, error)` | 343-357 | Lista los `.docx` del **primer nivel** de `dir`. No recursivo. Error `ERR_READ_DIR` si no se puede leer. |

Ambas son auxiliares: la CLI usa `factory.IsSupportedFile` y no estas funciones.

## Gotchas

1. `<w:b w:val="0"/>` activa igualmente la negrita: el flag se pone a `true` al abrir el elemento
   sin mirar el atributo `val`. Documentos con negrita desactivada explícitamente producen énfasis
   espurio.
2. El reset de `dp.relationships` está al inicio de `Process`. Si se mueve, el estado de un
   fichero se filtra al siguiente.
3. El decoder se detiene en el primer error de XML (`return "", "", err` en la línea 149) → el documento
   entero falla. No hay conversión parcial.
4. `parseRelationships` **silencia** el error de `xml.Unmarshal` (línea 112): un `.rels` corrupto no
   rompe la conversión, simplemente no hay hipervínculos.
5. El texto fuera de `<w:t>` (por ejemplo en atributos) se descarta.
6. Los estilos de título se detectan por subcadena: un estilo llamado `MiHeading3Personal` produce
   un `###`.

## Relación con la verificación de integridad

El texto que extrae este processor es la base del cálculo de integridad. Hay **dos** caminos:

1. **Con reutilización** (el que usa la CLI): `DocxProcessor` guarda el texto plano en
   `dp.lastText` y lo expone con `ExtractedText()`. `Converter.CheckIntegrity` lo reutiliza y el
   DOCX no se parsea dos veces.
2. **Sin reutilización** (el que usa `converter.CheckIntegrity` a secas): la función
   `extractDocxContent` (`internal/converter/converter.go:120-148`) relee el ZIP y extrae el texto
   por su cuenta.

Ambos dan el mismo resultado. Lo garantiza `TestIntegrity_ReusedTextMatchesFilePath`, que compara
las dos cifras y falla si difieren.

Ver [../quality/integridad.md](../quality/integridad.md).
