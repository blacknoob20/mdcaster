# AGENTS.md

Proyecto Go 1.21: CLI `mdcaster` y `mdclean` sobre la librería `internal/converter`
(DOCX/PDF → Markdown). Uso en `README.md`, decisiones en `ARCHITECTURE.md`.

## Specs (SDD)

- Antes de modificar un fichero, lee `.ai/specs/overview.md` y la spec del módulo afectado.
- La spec define el comportamiento **deseado**; el código, el **actual**.
- Si el código contradice la spec, para y pregunta. No lo resuelvas por tu cuenta.
- Todo cambio de comportamiento actualiza su spec en el mismo commit.
- Para añadir un formato, sigue `.ai/specs/converter/factory.md`.
- Las decisiones de arquitectura se registran en `ARCHITECTURE.md` como **entradas nuevas**.
  Las anteriores no se reescriben.

## Rendimiento

- Mide antes y después: `go test ./... -bench . -benchmem`. Nada se optimiza a ciegas.
- Las expresiones regulares se compilan **una vez**, al inicio del paquete. Nunca dentro de
  una función que se llame por línea o por párrafo.
- Acumula párrafos con `strings.Builder`, no con `+`.

## Reglas

- **Sin dependencias de Go**: `go.mod` no debe tener requisitos. `pdftotext` es un binario
  externo, no una dependencia.
- Los errores se devuelven al llamador, envueltos con `%w`, y se describen en español.
- No añadas comportamiento sin documentarlo antes en la spec.

## Verificación

- `go vet ./...` y `go test ./...` antes de commitear.
- Si `go` no está instalado, dilo explícitamente. No afirmes que los tests pasan.
- Todo cambio de código lleva test nuevo o actualizado.
- El corpus (`word/`, `pdf/`) es local y **no se versiona**: los tests se omiten si no está.
  No copies ficheros al repositorio.

## Estilo

- `gofmt` antes de commitear.
- Conventional commits con scope, en español: `tipo(scope): descripción`.
  - El tipo va en inglés: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`.
  - El scope es el **dominio**, no el fichero: `mdclean`, `converter`, `color`, `specs`.
  - La descripción va en minúscula y con verbo: "simplifica", "extrae", "actualizar".
  - El cuerpo lleva las viñetas con lo concreto: cifras medidas, qué cambió y por qué.
