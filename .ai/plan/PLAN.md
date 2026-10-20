# Plan de trabajo — mdcaster

> Plan local del proyecto. No es una spec: las specs viven en `.ai/specs/`.
> Este fichero registra el alcance, las decisiones tomadas y el orden de ejecución.

**Fecha:** 2026-10-20
**Estado:** completado

## Resultado

| Fase | Estado | Resultado |
|---|---|---|
| 0 — Línea base | completada | 7 benchmarks de referencia, con el número de alocaciones como métrica fiable |
| 1 — Sin cambio de salida | completada | `mdclean` genérico 207× más rápido, `isStructuralLine` 681× y con 0 alocaciones, DOCX con 23 % menos alocaciones, `fatih/color` sustituido por ANSI, `go.mod` sin dependencias, LICENSE y `.gitignore` actualizados |
| 2 — D10 | completada | Integridad por tokens: la métrica pasa de 100 % en todos los documentos a 99,32 % de media, y el coste de la verificación baja de O(palabras × longitud) a lineal |
| 3 — Documentación | completada | README solo para uso, ARCHITECTURE con D1–D11 y P1–P6, 8 specs actualizados, AGENTS.md reducido a 41 líneas |

Verificación final: `gofmt` limpio, `go vet` limpio, `go test ./...` en verde, compila para
linux, darwin y windows (amd64), 52 enlaces markdown y 111 referencias de código comprobadas.

## Objetivo

Dejar mdcaster coherente, rápido y documentado bajo la metodología SPEC-First:
specs como fuente de verdad, README para uso, ARCHITECTURE para decisiones, cero
dependencias externas y conversión lo más rápida posible.

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

### Pendientes

| ID | Pendiente |
|---|---|
| P1 | `Converter.ConvertDirectory` puede hacer `panic`. Corregir o eliminar (la CLI no lo usa) |
| P2 | `mdcaster` escanea sin recursión, pero la librería sí tiene versión recursiva |
| P3 | Los hipervínculos DOCX se parsean y no se emiten. Emitirlos o quitar el parseo |
| P4 | Un PDF aparece como 0 % (🔴). Mostrar "no medido" en lugar de 0 |
| P6 | La normalización borra acentos y `ñ` en vez de transliterarlos |

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
