# SPEC — Conversión DOCX → Markdown

**Módulo:** `converters/docx`
**Paquete:** `internal/converter/docx` (package `docx`)
**Código:** `processor.go`, `parser.go`, `model.go`, `spans.go`, `xml.go`, `render.go`, `lists.go`,
`tables.go`, `numbering.go`, `numbering_state.go`, `numbering_labels.go`, `footnotes.go`,
`frontmatter.go`, `images.go`
**Implementa:** `converter.FileProcessor` y `converter.TextExtractor`
**Dependencias:** solo biblioteca estándar (`archive/zip`, `encoding/xml`, `regexp`, `strconv`)

---

## Responsabilidad

Convertir un `.docx` (paquete OOXML = ZIP con XML) a Markdown preservando la estructura del
documento: párrafos, estilos de título, énfasis, listas con su numeración real, tablas, notas al
pie, imágenes y metadatos.

## API pública

El paquete expone **dos** identificadores:

| Identificador | Contrato |
|---|---|
| `NewProcessor() *Processor` | Crea el procesador del formato. |
| `Processor` | Implementa `Process`, `ExtractedText` y `SetSkipBeforeHeading`. |

El paquete **no importa** `internal/converter`: implementa las interfaces del dispatcher de forma
implícita, sin nombrarlas. La dependencia va en un solo sentido: la raíz registra el procesador.

## Estructura de los ficheros

El módulo se reparte por responsabilidad, dentro del paquete `docx`:

| Fichero | Responsabilidad |
|---|---|
| `processor.go` | El pipeline `Process`: abre el ZIP, lee las partes, parsea, renderiza y escribe |
| `parser.go` | El parser: recorre los tokens de `word/document.xml` y construye el modelo |
| `model.go` | El modelo de bloques: párrafos, celdas, tablas, imágenes y `paragraphPrefix` |
| `spans.go` | El render en línea: fragmentos a Markdown y a HTML, y `cleanFormatting` |
| `xml.go` | El vocabulario XML: espacios de nombres y lectura de atributos |
| `render.go` | El render del modelo a Markdown: recorre los bloques y escribe el resultado |
| `lists.go` | El render de las listas: marcador, sangría y los tres modos (Markdown, HTML y literal) |
| `tables.go` | El render de las tablas: filas, celdas y el escapado de `|` |
| `numbering.go` | La lectura de `word/numbering.xml`: los tipos y las definiciones de nivel |
| `numbering_state.go` | Los contadores de las listas: su avance y sus reinicios |
| `numbering_labels.go` | La etiqueta resuelta de un elemento de lista y su formato (decimal, letra, romano) |
| `footnotes.go` | Las notas al pie: lectura de `word/footnotes.xml` y sus definiciones |
| `frontmatter.go` | Los metadatos de `docProps/core.xml` y el frontmatter |
| `images.go` | La extracción de las imágenes y su enlace en el Markdown |

Reglas de reparto:

1. `xml.go` no depende de nada del módulo: es el vocabulario que comparten el parser, las
   notas al pie y la numeración.
2. `model.go` no depende del parser: el parser construye el modelo, no al revés.
3. `paragraphPrefix` vive en el modelo porque lo usan el parser (para saber si un párrafo es un
   encabezado) y el render (para emitirlo).
4. `cleanFormatting` vive junto al render en línea porque limpia las marcas que este produce.
5. La numeración se parte en tres: las definiciones (`numbering.go`), los contadores
   (`numbering_state.go`) y las etiquetas (`numbering_labels.go`). El render de las
   listas vive aparte, en `lists.go`, porque es un problema de Markdown, no de OOXML.
6. Los tipos internos conservan el prefijo `docx` (`docxParagraph`, `docxTable`): el prefijo es
   redundante dentro del paquete, pero renombrar veinte tipos no mejora el código y ensucia el
   diff. La API pública, en cambio, es idiomática: `docx.NewProcessor()`.

## Entrada / salida

| Aspecto | Contrato |
|---|---|
| Entrada | Ruta a un `.docx` válido (ZIP con `word/document.xml`) |
| Salida | Fichero Markdown en `outputPath`, permisos `0644` |
| Salida secundaria | Carpeta `<nombre>_assets/` junto al Markdown, solo si el documento tiene imágenes |
| Extensión registrada | `.docx` (minúsculas) |

## Estructura del paquete OOXML leída

| Entrada del ZIP | Uso | Obligatoria |
|---|---|---|
| `word/document.xml` | Cuerpo del documento | **Sí** → si falta: `ERR_DOCX_NO_DOCUMENT_XML` |
| `word/_rels/document.xml.rels` | Relaciones (hipervínculos, imágenes) | No (opcional) |
| `word/numbering.xml` | Definiciones de numeración de listas | No (opcional) |
| `word/footnotes.xml` | Notas al pie | No (opcional) |
| `docProps/core.xml` | Metadatos del documento (frontmatter) | No (opcional) |

Cualquier otra entrada del ZIP se ignora, salvo las de `word/media/` que se extraen cuando el
cuerpo las referencia.

## Flujo de `Process`

```
1. Reset de dp.relationships y dp.lastText   ← aisla el estado entre ficheros
2. zip.OpenReader(inputPath)                 → ERR_DOCX_OPEN
3. Lectura de las cinco partes del paquete
4. Si no hay document.xml → ERR_DOCX_NO_DOCUMENT_XML
5. parseRelationships(rels)                  ← hipervínculos e imágenes
6. parseDocxNumbering(numbering)            ← opcional
7. parseDocxFootnotes(footnotes)            ← opcional
8. parseDocxCoreProps(core)                  ← opcional
9. parseDocxDocument(document)               → ERR_DOCX_PARSE
10. extractDocxImages(...)                   ← extrae a <nombre>_assets/
11. resolveDocxImages(...)                   ← sustituye los marcadores por enlaces
12. renderDocument(...)                      → Markdown
13. Se antepone el frontmatter y se anexan las definiciones de notas al pie
14. dp.lastText = texto plano (cuerpo + notas al pie)
15. os.WriteFile(outputPath, markdown, 0644) → ERR_WRITE_MARKDOWN
```

El reset del paso 1 es obligatorio: `docx.Processor` guarda estado en el struct y la misma
instancia puede procesar varios ficheros (la CLI crea un `Converter` por worker, pero un worker
procesa varios ficheros con la misma instancia).

## Modelo de documento

El parseo produce un modelo de bloques, no texto plano:

| Tipo | Contenido |
|---|---|
| `docxBlock` | Bloque del cuerpo: párrafo o tabla, en orden |
| `docxParagraph` | Estilo (`w:pStyle`), lista (`numId`, `ilvl`), texto y fragmentos con formato |
| `docxTable` / `docxRow` / `docxCell` | Tabla, filas y celdas |
| `docxCell` | `gridSpan`, `vMerge` y el texto de sus párrafos |
| `inlineSpan` | Fragmento de texto con negrita, cursiva, tachado y marca de "texto literal" |

El texto plano para la verificación de integridad se acumula en el mismo recorrido: cada
`<w:t>` aporta su texto y un espacio.

## Reglas de parseo del documento

El recorrido usa `xml.Decoder` con **decoder de tokens**, no `Unmarshal`, para poder distinguir
el cierre de cada elemento.

### Elementos StartElement

| Elemento | Efecto |
|---|---|
| `p` | Inicia párrafo: resetea estilo, lista y formato en línea |
| `pPr` | Marca el inicio de las propiedades del párrafo |
| `pStyle` | Guarda el valor de `val` como estilo del párrafo |
| `numPr` | Marca el párrafo como elemento de lista (solo dentro de `pPr`) |
| `ilvl` | Nivel de lista (int) |
| `numId` | Identificador de la lista |
| `r` | Inicia un run |
| `rPr` | Propiedades del run (solo cuentan dentro de `r`) |
| `b`, `i`, `strike`, `u` | Activan negrita, cursiva, tachado y subrayado |
| `t` | Apila un buffer de texto |
| `tab` | Escribe **2 espacios** |
| `br` | Escribe `\n` |
| `tbl`, `tr`, `tc` | Estructura de tabla |
| `gridSpan` | Número de columnas que ocupa la celda |
| `vMerge` | Unión vertical: `restart` la abre, sin valor la continúa |
| `footnoteReference` | Referencia a una nota al pie; emite `[^id]` |
| `blip` (DrawingML), `imagedata` (VML) | Referencia a una imagen por `r:embed` / `r:id` |
| `docPr` | Texto alternativo de la imagen que viene a continuación |

### Espacios de nombres

Solo se interpretan los elementos de WordprocessingML. El namespace se comprueba por URI
(`…/wordprocessingml/2006/main` y su variante estricta); un prefijo sin resolver se asume Word.
Los elementos de otros formatos (DrawingML, VML) no se confunden con los de Word: `a:t` no es
`w:t`.

### Cierre de párrafo

```
1. text = render de los fragmentos en Markdown
2. text = TrimSpace(text)
3. text = cleanFormatting(text)
4. Si text == "" y el párrafo NO estaba numerado → el párrafo NO se escribe
5. Si text == "" y el párrafo estaba numerado → se conserva: consume su número, como en Word
6. Si el párrafo está dentro de una celda → el texto pasa a la celda
7. Si no → se escribe el bloque de párrafo
```

### Formato en línea

Cada `<w:t>` cerrado produce un fragmento con el formato vigente. Los fragmentos consecutivos con
el mismo formato se fusionan, de modo que dos runs en negrita seguidos dan `**texto**` y no
`**a****b**`.

El énfasis se aplica sobre el texto sin los espacios que lo rodean: `**texto **` se emite como
`**texto** `, porque `**texto **` no es énfasis válido en Markdown.

| Formato | Marcado |
|---|---|
| Negrita | `**texto**` |
| Cursiva | `*texto*` |
| Tachado | `~~texto~~` |
| Subrayado | `<u>texto</u>` (HTML en línea: Markdown no lo tiene) |

El subrayado se emite **por fuera** del énfasis, para que las marcas de Markdown de dentro sigan
interpretándose: un texto en negrita y subrayado sale `<u>**texto**</u>`.

Un subrayado con `w:val="none"` (o `"0"`, `"false"`, `"off"`) no se emite.

Las propiedades de la **marca de párrafo** (`w:pPr/w:rPr`) no se aplican al texto: describen el
párrafo, no sus runs. Una negrita desactivada (`<w:b w:val="0"/>`) no activa la negrita.

### Estilos de párrafo

`paragraphPrefix` devuelve el prefijo del párrafo, o la cadena vacía si no es un título. La
detección es por **subcadena** del nombre del estilo, no por igualdad:

| Estilo (minúsculas) | Prefijo |
|---|---|
| `heading1` \| `título1` \| `ttulo1` | `# ` |
| `heading2` … `heading6` (y variantes) | `## ` … `###### ` |
| `title` \| `título` \| `ttulo` | `# ` |
| `subtitle` | `## ` |

Un párrafo con estilo de título **y** numerado se renderiza como título, pero **conserva la
etiqueta** de lista: `## 2.1. Justificación Técnica`.

## Tablas

Las tablas se convierten a tablas Markdown. Reglas:

1. La **primera fila** es la cabecera. Se emite la fila de cabeceras y la fila de separadores
   (`| --- | --- |`).
2. Todas las filas se normalizan al número de columnas de la tabla: las filas cortas se rellenan
   con celdas vacías.
3. `w:gridSpan`: la celda ocupa varias columnas. El texto va en la primera y las demás quedan
   vacías.
4. `w:vMerge`: la celda que abre la unión (`restart`) conserva el texto; las que la continúan
   quedan vacías, aunque traigan texto.
5. El texto de una celda son sus párrafos separados por `<br>`.
6. Se escapan las barras verticales del texto: `|` → `\|`.
7. Una tabla **anidada** dentro de una celda se aplana a texto: sus filas se unen con ` | ` y se
   añaden como líneas de la celda.
8. Los párrafos de una celda **no** llevan marcador de lista: la celda muestra su texto.

## Listas

La numeración real se resuelve leyendo `word/numbering.xml`.

### Resolución de la etiqueta

```
a        = w:num[@numId=nid]/w:abstractNumId
lvl      = abstractNum[a]/w:lvl[@w:ilvl=ilvl]
fmt,text = lvl/w:numFmt, lvl/w:lvlText
start    = w:num[@numId=nid]/w:lvlOverride[@w:ilvl=ilvl]/w:startOverride
           ?? lvl/w:start ?? 1

contadores por (numId, ilvl):
  primera vez → start ; después → +1
  al avanzar (numId, ilvl) → se reinician los niveles más profundos, según w:lvlRestart:
     ausente  → se reinicia cuando cualquier nivel más superficial avanza
     0        → nunca se reinicia
     N        → se reinicia cuando avanza el nivel N-1

etiqueta = text sustituyendo cada %k por el contador del nivel k-1, formateado con el
           numFmt de ese nivel (decimal · lowerLetter · upperLetter · lowerRoman · upperRoman)
           w:isLgl presente → ese nivel se formatea como decimal
```

Los contadores **no** se reinician al cambiar de `numId`: Word mantiene una numeración continua
por instancia de lista.

### Formas de render

| Caso | Render |
|---|---|
| `numFmt` = `bullet` | Viñeta Markdown: `- ` |
| `numFmt` decimal y la etiqueta es `N.` o `N)` | Lista ordenada Markdown: `N. ` |
| `numFmt` `lowerLetter` / `upperLetter` / `lowerRoman` / `upperRoman` | Bloque HTML `<ol type="a" start="N">` |
| Etiqueta compuesta (`%1.%2.`) | Línea literal: `2.1. texto` |
| Sin `numbering.xml` o sin definición del `numId` | Viñeta Markdown: `- ` |

Markdown no admite marcadores de lista ordenada que no sean dígitos: las etiquetas alfabéticas y
romanas se emiten como HTML, y las compuestas como texto literal, que es la única forma de que la
etiqueta llegue intacta.

En un bloque `<ol>` el Markdown no se procesa, así que el contenido del `<li>` se emite con HTML
en línea (`<strong>`, `<em>`, `<del>`) en lugar de `**`, `*` y `~~`.

### Sangría

La sangría de un elemento es la **columna de contenido de su elemento padre**: el número de
caracteres del marcador del padre más su sangría. Así `1. ` deja la columna en 3 y un nivel
anidado se sangra 3 espacios; una viñeta `- ` deja la columna en 2 y su nivel anidado se sangra 2.

La columna se guarda **por lista**: un elemento anidado puede aparecer en un tramo distinto del
que tiene su padre y la sangría debe conservarse. Si el nivel padre no aparece en el cuerpo, se
usa la sangría habitual de dos espacios por nivel.

## Notas al pie

`word/footnotes.xml` se lee si existe. Las notas estructurales de Word (`separator`,
`continuationSeparator`) se ignoran.

- En el cuerpo, `<w:footnoteReference w:id="N"/>` se emite como `[^N]`.
- Al final del Markdown se emiten las definiciones: `[^N]: texto`.
- Solo se definen las notas referenciadas en el cuerpo, y en el orden de referencia.
- El texto de las notas al pie se suma al texto plano de la integridad.

## Imágenes

Una imagen del cuerpo es un `<w:drawing>` (o un `<w:pict>` de VML) que contiene un
`<a:blip r:embed="rIdN"/>` (o un `<v:imagedata r:id="rIdN"/>`).

1. El `rIdN` se resuelve contra `word/_rels/document.xml.rels`.
2. La imagen se extrae del paquete a `<nombre_del_md>_assets/`, junto al Markdown.
3. En el Markdown se emite `![texto alternativo](<nombre>_assets/<fichero>)`.
4. El texto alternativo sale de `wp:docPr/@descr` o, si no hay, de `wp:docPr/@name`.
5. Si la relación no existe, apunta fuera del paquete o el fichero no está, la imagen **se omite**:
   es preferible no emitir nada a emitir un enlace roto.
6. Los nombres de fichero se sanean (solo letras, dígitos, `.`, `-` y `_`) y se desduplican.

## Frontmatter

`docProps/core.xml` se lee si existe:

| Origen | Clave |
|---|---|
| `dc:creator` | `author` |
| `dcterms:created` | `date` |
| `dcterms:modified` | `updated` |

- Las fechas se reducen a `AAAA-MM-DD`.
- Los valores se entrecomillan y se escapan.
- **Solo se emite frontmatter si hay al menos un metadato.**
- `source` no se emite: no forma parte del contrato.

```yaml
---
author: "Nombre Autor"
date: "2026-01-02"
updated: "2026-02-03"
---
```

## Descartar el contenido anterior al primer encabezado

`docx.Processor` puede descartar la portada, el índice y las tablas de versión que anteceden al
primer encabezado. La opción la activa la CLI con `--skip-before-heading` y llega al processor por
la interfaz opcional `SkipBeforeHeadingSetter`.

1. El descarte se hace **por bloques**: se busca el primer párrafo con estilo de título y se
   descartan los bloques anteriores.
2. El texto plano se recorta en el mismo punto, de modo que la verificación de integridad mide el
   contenido que se conserva y no baja por un descarte intencionado.
3. Si el documento **no tiene ningún encabezado**, se conserva todo: vaciar el documento sería
   perder la información.
4. El frontmatter no se ve afectado: son metadatos, no contenido del cuerpo.

## Hipervínculos

`parseRelationships` guarda **todas** las relaciones, indexadas por `Id`, con su tipo, su destino
y su `TargetMode`. Los hipervínculos se resuelven para localizar las imágenes.

**Limitación vigente:** los hipervínculos del cuerpo **no se emiten** en el Markdown. Ver
[P3](../../../ARCHITECTURE.md#pendientes).

## Gotchas

1. El decoder se detiene en el primer error de XML → el documento entero falla. No hay conversión
   parcial.
2. `parseRelationships` **silencia** el error de `xml.Unmarshal`: un `.rels` corrupto no rompe la
   conversión.
3. El texto fuera de `<w:t>` (por ejemplo en atributos) se descarta.
4. Los estilos de título se detectan por subcadena: un estilo llamado `MiHeading3Personal` produce
   un `###`.
5. En `numbering.xml` los valores van en el atributo `w:val` (`w:start`, `w:numFmt`, `w:lvlText`,
   `w:lvlRestart`, `w:isLgl`, `w:startOverride`), no en el texto del elemento. Un `w:num` sin
   `w:abstractNumId` correctamente leído deja la lista sin definición.
6. Un párrafo numerado vacío no se escribe, pero consume su número: es lo que hace Word, y de lo
   contrario las etiquetas siguientes se descuadran.
7. La sangría de las listas se guarda por `numId`, no por tramo de lista.
8. En `numbering.xml` los valores van en el atributo `w:val`, y el texto plano del documento se
   recorta con el mismo punto de corte que los bloques.

## Relación con la verificación de integridad

El texto que extrae este processor es la base del cálculo de integridad, y es la **única** fuente:
`docx.Processor` guarda el texto plano en `dp.lastText` y lo expone con `ExtractedText()`, y
`Converter.CheckIntegrity` lo reutiliza. El DOCX no se parsea dos veces.

El texto incluye el de las notas al pie referenciadas, una vez cada una. La garantía de que no
pierde nada la comprueba `TestDocx_ExtractedTextMatchesIndependentRead`
(`internal/converter/docx/reader_test.go`), que lee el original por una vía independiente y
compara las dos cifras.

Ver [../quality/integridad.md](../quality/integridad.md).
