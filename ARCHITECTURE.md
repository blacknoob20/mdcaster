# Arquitectura de mdcaster

Este documento registra las **decisiones de arquitectura**: qué se decidió, por qué y qué
consecuencias tiene.

No es un manual de uso (eso es [README.md](README.md)) ni la especificación del comportamiento
(eso es [`.ai/specs/`](.ai/specs/)). Aquí solo queda el "por qué".

**Formato:** cada decisión tiene un identificador (`D1`, `D2`…). Si una decisión cambia, se añade
una entrada nueva y la anterior **no se reescribe**: el historial forma parte del documento.

---

## Índice

| ID | Decisión | Estado |
|---|---|---|
| [D1](#d1--factory-pattern-para-los-formatos-de-entrada) | Factory Pattern para los formatos de entrada | Aceptada |
| [D2](#d2--una-converter-por-worker) | Una `Converter` por worker | Aceptada |
| [D3](#d3--docx-solo-con-la-biblioteca-estándar) | DOCX solo con la biblioteca estándar | Aceptada |
| [D4](#d4--pdf-con-pdftotext-excepción-documentada) | PDF con `pdftotext` (excepción documentada) | Aceptada como excepción |
| [D5](#d5--integridad-como-cobertura-de-palabras) | Integridad como cobertura de palabras | Reemplazada por D10 |
| [D6](#d6--mdclean-separado-y-agnóstico-al-origen) | `mdclean` separado y agnóstico al origen | Aceptada |
| [D7](#d7--inplace-trunca-y-reescribe) | `--inplace` trunca y reescribe | Aceptada |
| [D8](#d8--subcarpeta-con-timestamp-en-cada-ejecución) | Subcarpeta con timestamp en cada ejecución | Aceptada |
| [D9](#d9--el-rendimiento-es-un-requisito-de-primera-clase) | El rendimiento es un requisito de primera clase | Aceptada |
| [D10](#d10--la-integridad-se-mide-por-tokens) | La integridad se mide por tokens | Aceptada, reemplaza a D5 |
| [D11](#d11--cero-dependencias-externas) | Cero dependencias externas | Aceptada |
| [P1](#pendientes) | `ConvertDirectory` puede hacer `panic` | Pendiente |
| [P2](#pendientes) | Escaneo no recursivo frente a la API recursiva | Pendiente |
| [P3](#pendientes) | Hipervínculos DOCX parseados y no emitidos | Pendiente |
| [P4](#pendientes) | El PDF se muestra como 0 % en vez de "no medido" | Pendiente |
| [P6](#pendientes) | La normalización borra acentos y `ñ` | Pendiente |

---

## Decisiones

### D1 — Factory Pattern para los formatos de entrada

**Estado:** aceptada

**Contexto.** El proyecto nació como un conversor DOCX→Markdown. La pregunta era cómo añadir
formatos sin reescribir el código existente cada vez.

**Decisión.** Cada formato vive en su propio fichero `internal/converter/{formato}_processor.go` y
implementa la interfaz `FileProcessor` (`Process(inputPath, outputPath string) error`). El registro
está en un único punto, `NewProcessorFactory()`.

**Consecuencias.**

- `converter.go` y `cmd/mdcaster` **no cambian** al añadir un formato.
- La CLI filtra con `factory.IsSupportedFile` y muestra `factory.SupportedExtensions`, así que la
  lista de formatos soportados se actualiza sola.
- La interfaz no se toca: un procesador nuevo no puede meter mano en el dispatcher.
- El coste es una línea de registro por formato, y una spec por formato.

### D2 — Una `Converter` por worker

**Estado:** aceptada

**Contexto.** El paralelismo del CLI son goroutines, una por fichero.

**Decisión.** Cada worker crea **su propia** `Converter`, y por tanto su propia `ProcessorFactory`.

**Consecuencias.**

- `DocxProcessor` guarda estado entre llamadas (`relationships`, `lastText`). Compartir una
  instancia entre goroutines daría carreras.
- El mapa de procesadores de la factory no es seguro para escritura concurrente. Que cada worker
  tenga la suya lo evita sin necesidad de un mutex.
- El coste es una factory por worker: despreciable.

### D3 — DOCX solo con la biblioteca estándar

**Estado:** aceptada

**Contexto.** El paquete OOXML de Word es un ZIP con XML dentro. Cabía la tentación de usar una
librería de terceros.

**Decisión.** `archive/zip` para abrir el paquete y `encoding/xml` con un decoder de tokens para
recorrer `word/document.xml`. Ninguna dependencia externa.

**Consecuencias.**

- El binario no arrastra nada: `go.mod` no tenía requisitos antes de D11 y ahora tampoco.
- A cambio, el parseo de OOXML está escrito a mano. Lo que no está contemplado se ignora: tablas
  (su texto sí se conserva, como párrafos), hipervínculos (ver P3), imágenes, notas al pie,
  encabezados y pies de página.
- El decoder por tokens, y no `Unmarshal`, es lo que permite distinguir el cierre de cada
  elemento y aplicar el formato en el punto correcto.

### D4 — PDF con `pdftotext` (excepción documentada)

**Estado:** aceptada como excepción

**Contexto.** La biblioteca estándar de Go no incluye un parser de PDF. Escribir uno exige
implementar la tabla xref, los object streams, la decodificación de fuentes (ToUnicode, CID,
encodings) y los operadores de texto de contenido. No es un detalle: es un proyecto en sí mismo.

**Decisión.** Se usa `pdftotext -layout` de poppler-utils. Es la **única dependencia externa** del
proyecto, y es de runtime, no de build.

**Consecuencias.**

- `pdftotext` tiene que estar en el PATH. Si falta, solo falla la conversión de PDF y el error lo
  dice con el nombre del paquete que hay que instalar.
- La salida es texto plano con el layout original, sin estructura Markdown. Para darle forma está
  `mdclean`.
- Un extractor propio en Go sigue siendo posible, pero es un proyecto aparte, con su propio corpus
  de comparación contra poppler. Si seresa, esta decisión se sustituye por una entrada nueva.

### D5 — Integridad como cobertura de palabras

**Estado:** reemplazada por [D10](#d10--la-integridad-se-mide-por-tokens)

**Contexto.** La primera versión contaba una palabra del original como "encontrada" si aparecía
como **subcadena** en cualquier parte del Markdown.

**Consecuencias de la sustitución.** Con palabras cortas ("la", "de", "a") la coincidencia por
subcadena era casi siempre certainizada, y el resultado era 100 % en todos los documentos. La
métrica no distinguía nada. Ver D10.

### D6 — `mdclean` separado y agnóstico al origen

**Estado:** aceptada

**Contexto.** El Markdown generado por un DOCX y por un PDF tienen problemas distintos: el primero
viene troceado en párrafos con marcas, el segundo viene con el layout en crudo.

**Decisión.** `mdclean` es un binario aparte que trabaja sobre el `.md` ya convertido y **no sabe
de dónde vino**. Tiene dos modos: el genérico, que une líneas partidas por un extractor de texto, y
el TDR, que normaliza el corpus institucional.

**Consecuencias.**

- La limpieza no está acoplada al conversor: sirve para cualquier Markdown.
- El modo genérico es conservador por diseño: ante la duda, no une. El coste es que deja
  particiones sin fusionar.
- El modo TDR está **muy acoplado** al corpus: nombres de cabecera institutional, imágenes
  `media/rIdNN`, secciones concretas. Generalizarlo exige tocar esas listas.

### D7 — Inplace trunca y reescribe

**Estado:** aceptada

**Contexto.** El corpus se trabaja en carpetas compartidas (vboxsf), donde el rename atómico sobre
un fichero existente falla.

**Decisión.** Se usa `os.Create` (truncar) y se escribe encima, en vez de escribir un temporal y
renombrar.

**Consecuencias.**

- Funciona en vboxsf, que es donde se usa.
- **No es atómico**: un fallo a mitad deja el fichero vacío o corrupto, y no hay copia de
  seguridad. Es una decisión consciente, no un descuido.

### D8 — Subcarpeta con timestamp en cada ejecución

**Estado:** aceptada

**Contexto.** El destino por defecto es `converted/`, relativo a la carpeta de entrada.

**Decisión.** Cada ejecución escribe en `<destino>/<YYYYMMDD-HHMMSS>/`.

**Consecuencias.**

- Ejecuciones sucesivas no se pisan: se puede volver a lanzar sin perder el resultado anterior.
- El usuario tiene que entrar en la subcarpeta para ver los Markdown.
- Como el destino es relativo, se resuelve contra la carpeta de entrada.

### D9 — El rendimiento es un requisito de primera clase

**Estado:** aceptada

**Contexto.** El requisito es explícito: la conversión tiene que ser rápida y exploitando la
concurrencia. "Rápido" sin medir no es una decisión, es una intención.

**Decisión.** Cualquier cambio en las rutas de conversión se mide antes y después con
`go test -bench`. Nada se optimiza a ciegas. De esa medición salen tres reglas:

1. Las expresiones regulares se compilan **una vez**, al inicio del paquete. Nunca dentro de una
   función que se llame por línea o por párrafo.
2. Los párrafos se acumulan con `strings.Builder`, no con `+`, para no copiar la cadena en cada
   unión.
3. La verificación de integridad indexa los tokens en un conjunto, en vez de buscar cada palabra
   en el Markdown entero (ver D10).

**Consecuencias.** Medido con el corpus local (4 CPU, 20 iteraciones, mínimo de 5 rondas):

| Medición | Antes | Después |
|---|---|---|
| `mdclean` genérico, 10.000 líneas | 3,21 s · 1,00 GB · 8.390.926 allocs | 15,2 ms · 5,2 MB · 80.007 allocs |
| `mdclean`, predicado de línea estructural | 423 ms · 208 MB · 1.748.543 allocs | 0,85 ms · 52 B · **0 allocs** |
| `mdclean` modo TDR | 273 ms · 111 MB · 1.219.794 allocs | 81 ms · 17 MB · 83.153 allocs |
| Conversión DOCX | 2,80 ms · 15.924 allocs | 2,18 ms · 12.309 allocs |
| Verificación de integridad | 7,63 ms | 2,76 ms |
| Conversión PDF | 40,6 ms | 38,4 ms |

El número de alocaciones es la métrica fiable: los tiempos en esta máquina varían según la carga.

Lo que **no** se ha hecho, y sigue abierto: la conversión de un único DOCX es secuencial. Dentro
de un documento no hay paralelismo posible con el parseo actual. El paralelismo actual es por
fichero. Medir si un pipeline interno compensa es trabajo pendiente.

### D10 — La integridad se mide por tokens

**Estado:** aceptada. Reemplaza a [D5](#d5--integridad-como-cobertura-de-palabras)

**Contexto.** La métrica por subcadena era inútil: cualquier palabra corta del original ("la",
"de", "a") aparecía como fragmento dentro de otra palabra del Markdown, así que se contaba como
encontrada. El resultado era 100 % en todos los documentos, sin excepción.

**Decisión.** La coincidencia es por **token exacto**. Los tokens del Markdown se normalizan y se
indexan en un conjunto; después se comprueba la pertenencia de cada palabra del original.

**Consecuencias.**

- La métrica ahora discrimina. Medido sobre el corpus real: la métrica antigua daba 100,00 % en
  los cuatro documentos; la nueva da 97,68 %, 99,72 %, 99,89 % (media 99,32 %).
- **Las cifras bajan.** Es una medida más estricta, y por tanto más útil: un 100 % ya no
  significa "nada se perdió" sino "no se detectó nada perdido".
- El coste pasa de O(palabras × longitud del Markdown) a lineal.
- El DOCX ya no se parsea dos veces: el procesador guarda el texto que extrajo y la verificación
  lo reutiliza (`TextExtractor`).

### D11 — Cero dependencias externas

**Estado:** aceptada

**Contexto.** `github.com/fatih/color` era la única dependencia, y solo se usaba para colorear la
salida de `mdcaster`.

**Decisión.** El color sale de códigos ANSI con la biblioteca estándar. La detección de terminal
usa `os.Stdout.Stat()`; se respetan `NO_COLOR` y `TERM=dumb`; en Windows se activa el modo VT de
la consola.

**Consecuencias.**

- `go.mod` no tiene requisitos y `go.sum` desaparece. `pdftotext` (D4) es un binario externo, no
  una dependencia de Go.
- La salida visible es la misma: los códigos son los mismos.
- El código que activa el modo VT en Windows es específico de esa plataforma. Se ha **compilado**
  para Windows, pero no se ha podido **ejecutar** en un entorno Windows desde este repositorio.
- En consolas de Windows antiguas sin soporte de VT se verían las secuencias de escape.

---

## Pendientes

Decisiones abiertas. Cada una necesita una entrada nueva en este documento cuando se resuelva.

### P1 — `Converter.ConvertDirectory` puede hacer `panic`

`internal/converter/converter.go:242` calcula la ruta de salida con
`GetOutputPath(inputPath, "")[len(outputDir)+1:]`: el corte usa una longitud calculada sobre **otra**
cadena. Si `len(outputDir)+1` supera la longitud del nombre del fichero, el corte queda fuera de
rango.

La CLI no lo usa (hace su propio escaneo), así que `mdcaster` no se ve afectado. La biblioteca sí.

**Opciones:** corregir el cálculo, o eliminar el método y dejar solo la versión recursiva, que sí
funciona.

### P2 — Escaneo no recursivo frente a la API recursiva

`mdcaster` solo procesa el primer nivel de la carpeta. La biblioteca ofrece
`ConvertDirectoryRecursive`, que sí es recursiva. Ninguna de las dos se usa.

**Opciones:** un flag `--recursive` en la CLI, o eliminar el método recursivo de la biblioteca.

### P3 — Hipervínculos DOCX parseados y no emitidos

`DocxProcessor` lee `word/_rels/document.xml.rels` y guarda los hipervínculos, pero el mapa no se
usa en ninguna fase posterior: los enlaces se pierden.

**Opciones:** emitirlos como enlaces Markdown, o quitar el parseo y aceptar que se pierden.

### P4 — El PDF se muestra como 0 % en vez de "no medido"

La verificación de integridad solo sabe leer DOCX. Sobre un PDF devuelve 0 y la CLI muestra 🔴,
que parece un fallo de conversión cuando no lo es.

**Opciones:** mostrar "no medido" en la salida, o extender la verificación a PDF.

### P6 — La normalización borra acentos y `ñ`

`normalizeContent` filtra con `[^a-z0-9\s]`, que **elimina** los caracteres acentuados en vez de
transliterarlos: "técnica" queda como "tcnica". Como el mismo criterio se aplica al original y al
Markdown, la comparación sigue siendo coherente, pero la medida se calcula sobre tokens degradados.

**Opciones:** transliterar antes de filtrar, o aceptar el límite actual.

---

## Ver también

- [README.md](README.md) — cómo usar el proyecto
- [`.ai/specs/`](.ai/specs/) — el comportamiento vigente, especificado
- [AGENTS.md](AGENTS.md) — cómo trabajar en el repositorio
- [`.ai/plan/PLAN.md`](.ai/plan/PLAN.md) — plan de trabajo en curso
