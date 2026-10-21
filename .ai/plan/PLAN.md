# Plan de trabajo — mdcaster

> Plan local del proyecto. No es una spec: las specs viven en `.ai/specs/`.
> Este fichero registra el alcance, las decisiones tomadas y el orden de ejecución.

**Fecha:** 2026-10-20
**Estado:** en curso

## Resultado

| Fase | Estado | Resultado |
|---|---|---|
| 0 — Línea base | completada | 7 benchmarks de referencia, con el número de alocaciones como métrica fiable |
| 1 — Sin cambio de salida | completada | `mdclean` genérico 207× más rápido, `isStructuralLine` 681× y con 0 alocaciones, DOCX con 23 % menos alocaciones, `fatih/color` sustituido por ANSI, `go.mod` sin dependencias, LICENSE y `.gitignore` actualizados |
| 2 — D10 | completada | Integridad por tokens: la métrica pasa de 100 % en todos los documentos a 99,32 % de media, y el coste de la verificación baja de O(palabras × longitud) a lineal |
| 3 — Documentación | completada | README solo para uso, ARCHITECTURE con D1–D11 y P1–P6, 8 specs actualizados, AGENTS.md reducido a 41 líneas |
| 4 — Fidelidad DOCX | en curso | Tablas, numeración real, frontmatter, notas al pie e imágenes (D12, D13) |
| 5 — Cierre de huecos | completada | Subrayado como HTML en línea (D14) y descarte previo (D15) |
| 6 — Partición por responsabilidad | completada | `mdclean` en cinco ficheros (D16) y el dispatcher en dos, con el código muerto eliminado (D17) |
| 7 — Partición del parser DOCX | completada | El parser del DOCX en cuatro ficheros y `startElement` descompuesta por familia de elementos (D18) |
| 8 — Partición del render DOCX | completada | El render del DOCX en tres ficheros y la numeración en tres (D19) |

Verificación final de la fase 0-3: `gofmt` limpio, `go vet` limpio, `go test ./...` en verde,
compila para linux, darwin y windows (amd64), 52 enlaces markdown y 111 referencias de código
comprobadas.

## Objetivo

Dejar mdcaster coherente, rápido y documentado bajo la metodología SPEC-First:
specs como fuente de verdad, README para uso, ARCHITECTURE para decisiones, cero
dependencias externas y conversión lo más rápida posible.

Desde la fase 4, el objetivo es **cerrar la brecha de fidelidad con `docx2md-cli`**: mismo
contenido, misma estructura y la numeración real que muestra Word.

## Decisiones tomadas

| ID | Decisión | Estado |
|---|---|---|
| D1 | Factory Pattern: un procesador por formato tras la interfaz `FileProcessor` | Aceptada |
| D2 | Una `Converter` por worker. `DocxProcessor` guarda estado, así que compartirlo daría carreras | Aceptada |
| D3 | DOCX solo con biblioteca estándar (`archive/zip`, `encoding/xml`) | Aceptada |
| D4 | PDF mediante `pdftotext -layout`. **Excepción documentada**: la stdlib de Go no tiene parser de PDF | Aceptada como excepción |
| D5 | Integridad medida como cobertura de palabras del original | Reemplazada por D10 |
| D6 | `mdclean` es un binario separado y agnóstico al formato de origen | Aceptada |
| D7 | `--inplace` trunca y reescribe, sin rename atómico, por carpetas compartidas vboxsf | Aceptada |
| D8 | Cada ejecución escribe en una subcarpeta con timestamp | Aceptada |
| D9 | El rendimiento es un requisito de primera clase: medir antes y después de cada cambio | Aceptada |
| D10 | La integridad se mide por tokens exactos, no por subcadena | Aceptada, reemplaza a D5 |
| D11 | Cero dependencias externas. El color sale con códigos ANSI de la stdlib | Aceptada |
| D12 | El documento se parsea como un modelo de bloques y el render es una fase aparte | Aceptada |
| D13 | La numeración de listas se resuelve contra `word/numbering.xml`, como en Word | Aceptada |
| D14 | El subrayado se emite como `<u>texto</u>`, HTML en línea | Aceptada |
| D15 | El descarte antes del primer encabezado es una opción del processor, no de la CLI | Aceptada |
| D16 | `mdclean` se parte en cinco ficheros por responsabilidad y se deduplica | Aceptada |
| D17 | El dispatcher se parte en dos ficheros por responsabilidad y se elimina el código muerto | Aceptada |
| D18 | El parser del DOCX se parte en cuatro ficheros por responsabilidad | Aceptada |
| D19 | El render del DOCX se parte en tres ficheros y la numeración en tres | Aceptada |

### Pendientes

| ID | Pendiente |
|---|---|
| P1 | `Converter.ConvertDirectory` puede hacer `panic`. Corregir o eliminar (la CLI no lo usa) | Resuelto por D17 |
| P2 | `mdcaster` escanea sin recursión, pero la librería sí tiene versión recursiva | Resuelto por D17 |
| P3 | Los hipervínculos DOCX se parsean y no se emiten. Emitirlos o quitar el parseo |
| P4 | Un PDF aparece como 0 % (🔴). Mostrar "no medido" en lugar de 0 |
| P6 | La normalización borra acentos y `ñ` en vez de transliterarlos |
| P7 | Las etiquetas de lista dentro de una celda de tabla se pierden |
| P8 | El subrayado (`w:u`) no se conserva | Resuelto por D14 |

## Fases

### Fase 0 — Línea base

Benchmarks de conversión DOCX, PDF, `mdclean` e integridad, más mediciones de la CLI
con distintos números de workers. Ningún cambio de comportamiento.

### Fase 1 — Cambios que no alteran la salida

- Tests que leen el corpus de `word/` y `pdf/` por glob, sin copiar nada al repo.
- Regex compilados dentro de funciones o bucles → variables de paquete.
- Concatenación de strings en bucles → `strings.Builder`.
- El DOCX deja de parsearse dos veces (conversión e integridad).
- `fatih/color` sustituido por códigos ANSI. `go.mod` y `go.sum` sin dependencias.
- `.gitignore`: añadir `converted/`.
- `LICENSE` (MIT, Cristhian R. Guerrero Soto, 2026).
- Medición contra la línea base. La salida debe ser idéntica.

### Fase 2 — D10: integridad por tokens

Algoritmo nuevo, spec `quality/integridad.md` actualizada y cifras comparadas antes
y después sobre el corpus.

### Fase 3 — Documentación

- `README.md`: solo cómo usar. Fuera Estructura, Rendimiento, Desarrollo y
  Especificaciones. Sin mención a Windows.
- `ARCHITECTURE.md`: D1–D11, P1–P4 y P6.
- Specs: `factory.md`, `dispatcher.md`, `pdf.md`, `overview.md`, `integridad.md`,
  `cli/mdcaster.md`.
- `AGENTS.md`: versión corta, más regla de rendimiento y regla del corpus.

### Fase 4 — Fidelidad DOCX (D12, D13)

Cerrar la brecha con `docx2md-cli` sobre los mismos documentos, por orden de valor para el
corpus institucional:

1. **Tablas**: `<w:tbl>`, `<w:tr>`, `<w:tc>`, `<w:gridSpan>` y `<w:vMerge>` → tabla Markdown,
   con la primera fila como cabecera.
2. **Listas numeradas**: `word/numbering.xml` con los contadores de Word (D13).
3. **Frontmatter**: `docProps/core.xml` → `author`, `date`, `updated`.
4. **Notas al pie**: `word/footnotes.xml` → `[^id]` y sus definiciones.
5. **Imágenes**: `<w:drawing>` → `<a:blip r:embed>` → extraer y enlazar.

Decisiones acordadas: primera fila como cabecera por defecto; celdas combinadas como celdas
vacías conservando el texto una vez; etiquetas no decimales como `<ol type="a">`; frontmatter
solo con metadatos y notas con sintaxis `[^id]`; imágenes en `<nombre>_assets/` junto al `.md`.

Medido sobre los 4 DOCX del corpus institucional:

| Métrica | Antes | Después | `docx2md-cli` |
|---|---|---|---|
| Filas de tabla conservadas | 0 | 26 / 127 / 32 | 26 / 124 / 32 |
| Etiquetas de lista correctas | 0 | 49 / 49 y 72 de 72 | 12 de 49 y 27 de 72 |
| Asignaciones por conversión | 12.307 | 19.366 | — |
| Tiempo por conversión | ~3,4 ms | ~4,8 ms | ~225 ms |

### Fase 5 — Cierre de los huecos frente a docx2md (D14, D15)

Los dos puntos que quedaban por debajo de `docx2md-cli`:

1. **Subrayado**: `<w:u>` se emite como `<u>texto</u>`, por fuera del énfasis de Markdown. Hay
   273 elementos `<w:u>` en el corpus y 28 `<u>` en el Markdown generado.
2. **Descarte previo**: `--skip-before-heading` empieza la conversión en el primer encabezado.
   1.140 líneas de portada, índice y tablas de versión descartadas; la integridad no baja (99,16 %
   con y sin descarte) porque el descarte se aplica también al texto que se mide.

Ambos con sus specs, sus tests y el modelo de fragmentos del parser (D12) como base.

## Verificación

Cada fase termina con:

```bash
go vet ./...
go test ./...
go test ./... -bench . -benchmem
```

Sin Go instalado en el host, los comandos corren en Docker con `golang:1.21`.

## Corpus

| Origen | Contenido | Uso |
|---|---|---|
| `word/` | 1 DOCX (295 KB) | Tests y benchmark |
| `pdf/` | 1 PDF (631 KB) | Tests y benchmark |
| `soporte-erp-egob/07_otros/` | ~74 DOCX y PDF | Solo lectura, benchmark por lotes |

Las copias para el benchmark por lotes van a `/tmp/opencode`, fuera del repo, y se
borran al terminar. Nada se copia al repositorio.
