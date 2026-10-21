# SPEC — Verificación de integridad de contenido

**Módulo:** `quality/integridad`
**Código:** `internal/converter/converter.go` y `cmd/mdcaster/main.go`
**Depende de:** `internal/converter` (lectura del `.docx` original)

> **Decisión relacionada:** [D10](../../../ARCHITECTURE.md#d10--la-integridad-se-mide-por-tokens).
> Este spec describe el comportamiento **actual**.

---

## Objetivo

Medir **qué porcentaje del texto del documento original aparece en el Markdown generado**, para
detectar conversiones que perdieron contenido.

El resultado se muestra por fichero (símbolo + porcentaje) y se agrega en el resumen final.

## Interfaz pública

```go
// internal/converter/integrity.go
func CheckIntegrityAgainst(originalContent, markdownFile string) float64

// método de Converter
func (c *Converter) CheckIntegrity(sourceFile, markdownFile string) float64
```

| Función | Cuándo usarla |
|---|---|
| `CheckIntegrityAgainst` | Cuando ya se tiene el texto del original. No lo vuelve a leer. |
| `Converter.CheckIntegrity` | La que usa la CLI. Reutiliza el texto que el processor ya extrajo. |

| Aspecto | Contrato |
|---|---|
| Entrada | Ruta del original y ruta del `.md` generado |
| Salida | Porcentaje `0..100` (float64) |
| Errores | **No se devuelven.** Cualquier fallo → `0` |

## Algoritmo — `CheckIntegrityAgainst` (`converter.go`)

```
1. markdownContent = os.ReadFile(markdownFile)
   └─ error → devolver 0
2. original = normalizeContent(originalContent)
3. Si len(original) == 0 → devolver 100
4. originalWords = strings.Fields(original)
   Si len(originalWords) == 0 → devolver 100
5. markdownTokens = conjunto de strings.Fields(normalizeContent(markdownContent))
6. foundCount = nº de palabras de originalWords que están en markdownTokens
7. devolver foundCount / len(originalWords) * 100
```

Reglas:

1. La coincidencia es por **token exacto**, no por subcadena. D10.
2. Se cuenta cada **aparición** en el original: si el original tiene "la" diez veces y el Markdown
   la tiene una, se cuentan las diez.
3. Los tokens del Markdown se indexan en un conjunto: el coste es lineal en el número de palabras,
   no el producto de palabras por longitud del Markdown.
4. El resultado es `0` ante cualquier error. No hay forma de distinguir "0 % real" de "no se pudo
   medir".
5. `100` se devuelve cuando no hay original con el que comparar (caso degenerado).

### Por qué token exacto y no subcadena

La versión anterior buscaba cada palabra dentro del Markdown entero con `strings.Contains`. Con
palabras cortas ("la", "de", "a") la coincidencia por subcadena se cumplía casi siempre, porque
esas cadenas aparecen dentro de otras palabras. El resultado era **100 % en todos los documentos**,
sin excepción: la métrica no distinguía nada.

Medido sobre el corpus real:

| Documento | Subcadena (antes) | Token (ahora) |
|---|---|---|
| FORMATO SUBASTA INVERSA SERVICIOS-V7 17092025 | 100,00 % | 97,68 % |
| FORMATO_ESPECIFICACIONES TECNICAS_TDR AGOSTO 2026 | 100,00 % | 99,72 % |
| FORMATO_INFORME DE NECESIDAD - AGOSTO_2026 | 100,00 % | 99,89 % |
| Media de 4 documentos | 100,00 % | 99,32 % |

**Las cifras bajan.** Es una medida más estricta y por tanto más útil: un 100 % ya no significa
"nada se perdió" sino "no se detectó nada perdido".

## Origen del texto original — el processor

El texto con el que se compara el Markdown **lo extrae el processor al convertir**. No hay una
segunda lectura del original.

`docx.Processor` guarda el texto plano que extrajo y lo expone mediante la interfaz opcional
`TextExtractor`:

```go
// internal/converter/processor.go
type TextExtractor interface {
    ExtractedText() string
}
```

`Converter.CheckIntegrity` la usa: si el processor del original implementa la interfaz, reutiliza
su texto y **no parsea el DOCX por segunda vez**. Si no la implementa (un PDF), el resultado es `0`.

El texto que guarda el processor incluye el de las **notas al pie referenciadas** en el cuerpo,
cada una una sola vez, y viene recortado si se activó `--skip-before-heading`.

Antes había una segunda vía: la librería releía el original desde disco con un parser propio
(`extractDocxContent`). Se ha eliminado por duplicar el parseo del DOCX. La garantía de que el
texto del processor no pierde nada la comprueba `TestIntegrity_ReusedTextMatchesIndependentRead`,
que lee el original por una vía independiente y compara las dos cifras.

## Normalización — `normalizeContent` (`integrity.go`)

```
1. strings.ToLower(content)
2. Eliminar todo lo que no sea [a-z0-9\s]      ← regex: [^a-z0-9\s]
3. strings.Join(strings.Fields(content), " ")   ← colapsar espacios
```

Consecuencias **normativas** del paso 2:

| Original | Normalizado | Nota |
|---|---|---|
| `TÉCNICA` | `tcnica` | los acentos y la `ñ` se **borran**, no se transliteran |
| `NIÑO` | `nio` | |
| `datos-2024` | `datos2024` | los guiones desaparecen |

El mismo criterio se aplica al original y al Markdown, así que la comparación **sigue siendo
coherente**. Pero la medida se calcula sobre tokens degradados. Ver [P6](../../../ARCHITECTURE.md#pendientes).

## Escala de interpretación — `cmd/mdcaster/main.go`

| Símbolo | Rango | Interpretación |
|---|---|---|
| 🟢 | `>= 95` | Integridad óptima — todo el contenido se conservó |
| 🟡 | `80..94` | Integridad buena — casi todo el contenido se conservó |
| 🟠 | `50..79` | Integridad aceptable — parte del contenido se conservó |
| 🔴 | `< 50` | Integridad deficiente — se perdió mucho contenido |

## Uso en la CLI — `cmd/mdcaster/main.go`

```
Si la conversión tuvo éxito:
   integrityPercent = conv.CheckIntegrity(inputFile, outputPath)
Si no:
   integrityPercent = 0
```

La integridad **solo** se mide cuando la conversión fue exitosa. En caso de error, el símbolo es 🔴
por el valor 0.

## Agregación en el resumen — `cmd/mdcaster/main.go`

| Métrica | Cálculo |
|---|---|
| Promedio | `suma(integridad de los ficheros exitosos) / successCount` |
| Mínima | menor valor entre los exitosos |
| Máxima | mayor valor entre los exitosos |

`minIntegrity` se inicializa a `100` y `maxIntegrity` a `0`; solo se muestran si `successCount > 0`.

## Limitación vigente: **solo DOCX**

La verificación mide el texto que expone el processor, y el del PDF no lo expone: `pdf.Processor`
no implementa `TextExtractor`, así que el resultado es `0`.

**Consecuencia visible:** al convertir un PDF, `mdcaster` muestra **🔴 0.0 %** aunque la conversión
haya sido perfectamente correcta. El porcentaje de un PDF **no significa nada** hoy.

Regla para la CLI: un 🔴 en un PDF **no es** un fallo de conversión. No debe usarse para
diagnosticar problemas con PDF. Ver [P4](../../../ARCHITECTURE.md#pendientes).

## Limitación vigente: la métrica no mide estructura

La extracción compara **texto**, no estructura. El texto de una tabla se conserva tanto si el
Markdown tiene tabla como si no: el porcentaje puede ser alto sin que la estructura se haya
mantenido. La métrica no mide fidelidad estructural.

Con la conversión de tablas, el frontmatter y las notas al pie, el texto del Markdown es una
superset del original en esos puntos, y el porcentaje se mantiene entre el 97 % y el 100 % en el
corpus institucional.

## Qué mide la métrica ahora

| Elemento | ¿Cuenta para la integridad? |
|---|---|
| Párrafos y tablas | Sí |
| Frontmatter (`author`, `date`, `updated`) | No: son metadatos, no texto del documento |
| Notas al pie | Sí: su texto se suma al original y aparece en el Markdown |
| Etiquetas de lista (`1.`, `a)`, `2.1.`) | No: los marcadores no son texto del documento |
| Imágenes | No: no tienen texto |
| Contenido descartado con `--skip-before-heading` | No: el descarte se aplica también al texto que se extrae, para que la métrica mida lo que se conserva |

## Reglas para cambios futuros

1. Añadir soporte para otros formatos exige un extractor por formato con la misma firma de error
   → `0`.
2. Si se cambia la normalización, hay que aplicar **el mismo** criterio a original y Markdown.
3. Cambiar la escala de umbrales (95/80/50) obliga a actualizar este spec, el README y el texto de
   `printHelp` (`cmd/mdcaster/help.go`), que hoy muestran "🟡 Amarillo (80%)" y
   "🟠 Naranja (50%)" sin el límite superior.
4. Cambiar la forma de contar (subcadena, token, llevenshtein) es un cambio de **métrica**, no de
   implementación: exige una entrada nueva en `ARCHITECTURE.md` y medir el antes y el después.

## Relación con los tests

| Test | Qué comprueba |
|---|---|
| `TestIntegrity_DocxToMarkdownContainsOriginalText` | El Markdown contiene el texto original normalizado, o al menos sus 50 primeras palabras. Es **más estricta** que la función en runtime: el test verifica "el Markdown contiene el original", la función calcula "qué fracción de tokens aparece". |
| `TestIntegrity_DocxReportsCoverage` | El porcentaje está en (0, 100] para un DOCX convertido. |
| `TestIntegrity_ReusedTextMatchesIndependentRead` | El texto que el processor ya extrajo da la misma cifra que una lectura independiente del original. |
| `TestIntegrity_PdfIsNotMeasurable` | Documenta el límite actual: un PDF da 0. Cuando se resuelva P4, este test debe cambiar. |

## Gotchas

1. `CheckIntegrity` no devuelve error. Un `0` puede significar "0 % real" o "fallo de medición".
2. La métrica cuenta apariciones en el original, no tokens distintos: un original con diez "la" y
   un Markdown con un solo "la" puntúa 10/10 por esa palabra.
3. La normalización borra acentos y `ñ`.
4. Los PDF siempre dan 0 %.
5. La métrica no mide estructura, solo texto.
6. `Converter.CheckIntegrity` depende del estado del procesador: solo es válido si se llama
   inmediatamente después de convertir con la **misma** `Converter`. La CLI cumple esa condición
   porque cada worker procesa los ficheros de uno en uno.
