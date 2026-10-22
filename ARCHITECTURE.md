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
| [D12](#d12--el-documento-se-parsea-como-un-modelo-de-bloques) | El documento se parsea como un modelo de bloques | Aceptada |
| [D13](#d13--la-numeración-de-listas-se-resuelve-contra-numberingxml) | La numeración de listas se resuelve contra `numbering.xml` | Aceptada |
| [D14](#d14--el-subrayado-se-emite-como-html-en-línea) | El subrayado se emite como HTML en línea | Aceptada |
| [D15](#d15--el-descarte-inicial-es-una-opción-del-processor) | El descarte inicial es una opción del processor | Aceptada |
| [D16](#d16--mdclean-se-parte-por-responsabilidad) | `mdclean` se parte por responsabilidad | Aceptada |
| [D17](#d17--el-dispatcher-se-parte-por-responsabilidad-y-se-elimina-el-código-muerto) | El dispatcher se parte por responsabilidad y se elimina el código muerto | Aceptada |
| [D18](#d18--el-parser-del-docx-se-parte-por-responsabilidad) | El parser del DOCX se parte por responsabilidad | Aceptada |
| [D19](#d19--el-render-del-docx-se-parte-por-responsabilidad) | El render del DOCX se parte por responsabilidad | Aceptada |
| [D20](#d20--cada-formato-vive-en-su-propio-paquete) | Cada formato vive en su propio paquete | Aceptada |
| [D21](#d21--versionado-semántico-y-tags-solo-en-las-releases-relevantes) | Versionado semántico y tags solo en las releases relevantes | Aceptada |
| [P1](#pendientes) | `ConvertDirectory` puede hacer `panic` | Resuelto por D17 |
| [P2](#pendientes) | Escaneo no recursivo frente a la API recursiva | Resuelto por D17 |
| [P3](#pendientes) | Hipervínculos DOCX parseados y no emitidos | Pendiente |
| [P4](#pendientes) | El PDF se muestra como 0 % en vez de "no medido" | Pendiente |
| [P6](#pendientes) | La normalización borra acentos y `ñ` | Pendiente |
| [P7](#pendientes) | Las etiquetas de lista dentro de una celda se pierden | Pendiente |
| [P8](#pendientes) | El subrayado no se conserva | Resuelto por D14 |

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

- `docx.Processor` guarda estado entre llamadas (`relationships`, `lastText`). Compartir una
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
  **Actualizado por [D12](#d12--el-documento-se-parsea-como-un-modelo-de-bloques) y
  [D13](#d13--la-numeración-de-listas-se-resuelve-contra-numberingxml):** las tablas, las
  imágenes, las notas al pie y la numeración de listas ya se convierten.
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

### D12 — El documento se parsea como un modelo de bloques

**Estado:** aceptada

**Contexto.** El parser DOCX era un autómata de estado sobre el flujo de tokens: escribía el
Markdown a medida que leía el XML. Con las tablas, las listas y las notas al pie eso ya no
servía: una tabla no es una secuencia de párrafos, y un párrafo dentro de una celda no es un
párrafo del cuerpo. La comparación con `docx2md` (que usa el modelo real de `python-docx` + `lxml`)
mostraba que la diferencia de fidelidad venía de ahí, no del algoritmo de conversion.

**Decisión.** El parseo produce un **modelo de bloques** (`docxDocument` → `docxBlock` →
`docxParagraph` / `docxTable`) y el render es una fase separada. Los fragmentos de texto se
guardan con su formato (`inlineSpan`) en vez de premarcados, de modo que el mismo párrafo se
puede renderizar como Markdown o como HTML en línea.

**Consecuencias.**

- El texto plano para la integridad se sigue acumulando en el mismo recorrido: no hay un segundo
  parseo.
- El orden del documento se conserva: párrafos y tablas se entrelazan como en el original.
- Se puede emitir el mismo contenido en Markdown o en HTML según lo que admita el destino (las
  listas con etiqueta no decimal, por ejemplo).
- **Coste medido** (corpus local, DOCX de 295 KB): las asignaciones suben de 12.307 a 19.366 por
  conversión (+57 %) y la memoria de 1,02 MB a 1,42 MB. El tiempo de conversión pasa de ~3,4 ms a
  ~4,8 ms. Sigue siendo unas 40 veces más rápido que `docx2md-cli` (225 ms por fichero).
- El emphasis se aplica sobre el texto sin sus espacios, y los runs contiguos con el mismo
  formato se fusionan: `**a****b**` (Markdown roto) ya no se produce.

### D13 — La numeración de listas se resuelve contra `numbering.xml`

**Estado:** aceptada

**Contexto.** `mdcaster` emitía todas las listas como viñetas, y `docx2md-cli` leía
`numbering.xml` solo para distinguir ordenada de viñeta: inventaba su propio contador arábigo y
perdía las etiquetas (`a)`, `I.`, `2.1.`). En el corpus institucional eso eran 37 de 49 etiquetas
mal en cuatro documentos.

**Decisión.** La numeración se resuelve como la resuelve Word: `w:num` → `w:abstractNum` →
`w:lvl`, con los contadores por `(numId, ilvl)`, el `w:startOverride`, el `w:lvlRestart` y la
composición de la `w:lvlText` (`%1.%2.`). Los contadores no se reinician al cambiar de `numId`.

**Consecuencias.**

- Las etiquetas del Markdown son las que Word muestra. Medido sobre el mismo corpus: 49 de 49
  etiquetas correctas en un documento y 72 de 72 en otro, frente a 12 de 49 y 27 de 72 de
  `docx2md-cli`.
- Markdown solo admite marcadores de lista ordenada con dígitos. Las etiquetas alfabéticas y
  romanas se emiten como `<ol type="a" start="N">`, y las compuestas (`2.1.`) como línea literal,
  que es la única forma de que la etiqueta llegue intacta. Es la misma degradación que aplica
  `pandoc`, pero conservando la etiqueta.
- En un bloque `<ol>` el Markdown no se procesa: el contenido del `<li>` se emite con HTML en
  línea (`<strong>`, `<em>`, `<del>`).
- Un párrafo numerado vacío no se escribe, pero **consume su número**: es lo que hace Word, y de
  lo contrario las etiquetas siguientes se descuadran.

### D14 — El subrayado se emite como HTML en línea

**Estado:** aceptada. Resuelve [P8](#pendientes)

**Contexto.** Word subraya con `<w:u>`; Markdown no tiene subrayado. `docx2md-cli` lo resuelve
emitiendo `<u>texto</u>`, que es HTML en línea. En el corpus institucional hay 273 elementos
`<w:u>` en 73 documentos: no es un detalle, es contenido que se perdía.

**Decisión.** El subrayado se emite como `<u>texto</u>`, y se aplica **por fuera** del énfasis de
Markdown, para que las marcas de dentro sigan interpretándose: un texto en negrita y subrayado sale
`<u>**texto**</u>`.

**Consecuencias.**

- El resultado es Markdown con una isla de HTML en línea. Lo renderizan CommonMark y GitHub.
- Un subrayado con `w:val="none"` no se emite: es el equivalente de la negrita desactivada.
- Es la misma decisión que ya se tomaba dentro de los bloques `<ol type="a">`, donde el Markdown
  tampoco se procesa y el contenido se emite con HTML en línea.

### D15 — El descarte inicial es una opción del processor

**Estado:** aceptada

**Contexto.** `docx2md-cli` tiene `--skip-before-heading`: descarta la portada, el índice y las
tablas de versión que anteceden al primer encabezado. En el corpus institucional son 1.140 líneas
que no son cuerpo del documento.

**Decisión.** La opción la aplica el **processor**, no la CLI: el texto plano que se extrae para
la verificación de integridad se recorta en el mismo punto, de modo que la métrica mide el
contenido que se conserva y no baja por un descarte intencionado. Si el documento no tiene ningún
encabezado, se conserva todo.

La opción llega al processor por una interfaz opcional, `SkipBeforeHeadingSetter`, igual que
`TextExtractor`: los formatos que no la soportan (el PDF) la ignoran.

**Consecuencias.**

- `FileProcessor.Process` **no cambia de firma**: la opción va por el estado del procesador.
- El descarte es **por bloques**, no por líneas: una tabla que está antes del primer encabezado
  desaparece entera, no se descuartan sus celdas.
- Medido sobre el corpus: 51.231 líneas sin el descarte y 50.091 con él, con la integridad en
  99,16 % en ambos casos.

---

### D16 — `mdclean` se parte por responsabilidad

**Estado:** aceptada

**Contexto.** `cmd/mdclean/main.go` llegó a 914 líneas con 25 funciones: la CLI, el modo genérico, el
modo TDR, las expresiones regulares y los datos del corpus institucional (cabeceras, imágenes,
secciones) en un solo fichero. El modo TDR era el peor caso: `processTDR` tenía 194 líneas con nueve
etapas dentro, y el código se repetía. El colapso de líneas en blanco estaba en dos sitios, la fusión
de negrita y la de cursiva eran el mismo bucle con otra expresión regular, la comprobación de estructura
de Markdown estaba duplicada en los dos modos y la lista de palabras cortas del español aparecía dos
veces.

**Decisión.** El comando se parte en cinco ficheros **del mismo paquete**, sin cambiar el comportamiento
ni la interfaz:

| Fichero | Responsabilidad |
|---|---|
| `cmd/mdclean/main.go` | CLI: flags, lectura y escritura, frontmatter |
| `cmd/mdclean/generic.go` | Modo genérico: unir líneas partidas por un extractor |
| `cmd/mdclean/tdr.go` | Modo TDR: la tubería de nueve etapas, una función por etapa |
| `cmd/mdclean/corpus.go` | Datos del corpus institucional |
| `cmd/mdclean/markdown.go` | Primitivas compartidas por los dos modos |

Y se deduplica: `collapseBlankLines`, `mergeEmphasisLines`, `isMarkdownStructureLine`, `isShortWord` y
una sola implementación de `wordToken` para las dos direcciones.

**Consecuencias.**

- El fichero mayor pasa de 914 a 503 líneas, y el código real (sin comentarios ni líneas en blanco)
  baja de 754 a 743: la duplicación retirada pesa más que el esqueleto de las etapas con nombre.
- Los datos del corpus quedan aislados: generalizar el modo TDR es tocar `corpus.go`, no el comando.
  Es la consecuencia que D6 dejaba abierta.
- **Comportamiento idéntico, verificado**: 312 documentos (300 generados al azar más 12 casos límite:
  fichero vacío, frontmatter sin cerrar, UTF-8 inválido, CRLF, línea de 1 MB, imagen partida) producen
  una salida byte a byte igual que la implementación anterior.
- El modo TDR reserva un **17 % menos de memoria** (17,3 MB → 14,4 MB con 10.000 líneas) y 80
  alocaciones menos, al prealoctar el resultado de cada etapa. El modo genérico no cambia.
- La lista de palabras cortas se comprueba con un `switch` y no con un mapa: el `switch` cuesta 3,5 ns
  y la búsqueda 48 ns, y se consulta dos veces por cada par de líneas que se une. Medido con las dos
  implementaciones en el mismo binario, porque el ruido de la máquina (±40 %) supera el efecto de
  cualquier cambio pequeño.

---

### D17 — El dispatcher se parte por responsabilidad y se elimina el código muerto

**Estado:** aceptada

**Contexto.** `internal/converter/converter.go` llegó a 411 líneas con cinco responsabilidades
mezcladas: el dispatcher, la verificación de integridad, un segundo parser del DOCX, la API de
conversión por directorio y un formateador de tamaños. Casi 40 % del fichero no lo usaba nadie:
`ConvertDirectory` (60 líneas) tenía un `panic` documentado, su variante recursiva (70 líneas)
tampoco, `ConvertPDFFile` (15 líneas) era un "alias" que no era un alias, `ByteCountSI` (13
líneas) formateaba tamaños que nadie formateaba, y `IsDocxFile`, `GetDocxFiles`, `IsPdfFile` y
`GetPdfFiles` eran auxiliares muertos: la CLI usa `factory.IsSupportedFile`.

Además, la verificación de integridad tenía **dos** caminos para obtener el texto original: el que
reutiliza el del processor y otro que releía el DOCX con un parser propio (`extractDocxContent`, 96
líneas), duplicando el parseo que ya hace `docx.Processor`.

**Decisión.** El dispatcher se parte en dos ficheros **del mismo paquete** y se elimina el código
muerto:

| Fichero | Responsabilidad |
|---|---|
| `internal/converter/converter.go` | Dispatcher: `Converter`, `Convert`, `ConvertFile`, `SetSkipBeforeHeading`, `GetOutputPath` |
| `internal/converter/integrity.go` | Verificación: `CheckIntegrity`, `CheckIntegrityAgainst`, `normalizeContent` |

Y se elimina:

| Eliminado | Motivo |
|---|---|
| `ConvertDirectory` | Tenía un `panic` (P1) y no lo usaba nadie |
| `ConvertDirectoryRecursive` | No lo usaba nadie (P2) |
| `ConvertPDFFile` | No lo usaba nadie y no era un alias de `Convert` |
| `ByteCountSI` | No lo usaba nadie |
| `IsDocxFile`, `GetDocxFiles`, `IsPdfFile`, `GetPdfFiles` | Auxiliares muertos: la CLI usa `factory.IsSupportedFile` |
| `extractDocxContent`, `extractTextFromXML` y la función de paquete `CheckIntegrity` | Segundo parser del DOCX: el texto del processor es la única fuente |

`ConversionResult` se mueve a `cmd/mdcaster/result.go`: es la CLI la que mide la duración y la
integridad de cada fichero, y las rellena ahí.

**Consecuencias.**

- El dispatcher pasa de 411 a 50 líneas, y la verificación vive en un fichero de 97.
- La verificación **deja de releer el original**: el texto del processor es la única fuente.
  Medido sobre el corpus institucional, pasa de 127.237 asignaciones y 8,9 MB por documento a
  **57 asignaciones y 0,9 MB**: 2.230 veces menos asignaciones y 9,5 veces menos memoria. El
  tiempo baja de 35-39 ms a 4-19 ms, con el ruido de la máquina (±40 %) ya aceptado en D16.
- **Comportamiento idéntico, verificado**: los 75 DOCX del corpus institucional y el PDF del
  corpus se convierten **byte a byte igual** que antes, con la misma integridad (99,19 % de media,
  81,77 % mínima, 100 % máxima) y la misma salida por consola.
- La segunda vía de la integridad se sustituye por un test que lee el original por una vía
  independiente (`TestIntegrity_ReusedTextMatchesIndependentRead`) y compara las dos cifras: sobre
  el corpus dan 99,4075 % las dos.
- P1 y P2 quedan resueltos: el método que podía hacer `panic` y el que no se usaba ya no existen.

---

### D18 — El parser del DOCX se parte por responsabilidad

**Estado:** aceptada

**Contexto.** `internal/converter/docx/parser.go` llegó a 752 líneas con cuatro responsabilidades
mezcladas: el vocabulario XML (los espacios de nombres y la lectura de atributos), el modelo de
bloques del documento (D12), el parser en sí y el render en línea de los fragmentos. Encima
tenía cuatro dependencias cruzadas invertidas: helpers que vivían en un fichero y que otro
necesitaba.

| Helper | Estaba en | Lo necesitaban |
|---|---|---|
| `isWordNamespace` | `parser.go` | el parser y `footnotes.go` |
| `attrValue` | `parser.go` | el parser y `footnotes.go` |
| `cleanFormatting` | `processor.go` | el parser y `images.go` |
| `paragraphPrefix` | `render.go` | el parser y el render |

El caso de `startElement` era el peor: 113 líneas con veinte casos. Es el mismo patrón que
`processTDR` (194 líneas, nueve etapas) que se partió en D16.

**Decisión.** El parser se parte en cuatro ficheros **del mismo paquete**, y `startElement` se
convierte en un despachador que delega en un método por familia de elementos:

| Fichero | Responsabilidad |
|---|---|
| `internal/converter/docx/parser.go` | El parser: el bucle de tokens y los handlers de cada familia de elementos |
| `internal/converter/docx/model.go` | El modelo de bloques (D12) y `paragraphPrefix` |
| `internal/converter/docx/spans.go` | El render en línea y `cleanFormatting` |
| `internal/converter/docx/xml.go` | El vocabulario XML: espacios de nombres y atributos |

`startElement` queda en 30 líneas y delega en `startParagraphProps`, `startRunProps`, `startText`,
`startTable`, `startFootnote` y `startImage`. `endElement` aplica el filtro de espacio de nombres
una sola vez, en vez de en cada caso.

**Consecuencias.**

- El fichero mayor del paquete pasa de 752 a 476 líneas (`parser.go`), y el módulo DOCX queda
  repartido en diez ficheros de 43 a 476 líneas.
- Las cuatro dependencias invertidas desaparecen: cada helper está en el fichero que lo usa su
  consumidor, y `xml.go` no depende de nada del módulo.
- `cleanMarkdown` deja de compilar su expresión regular en cada llamada: pasa a una variable de
  paquete, como ya se hacía con `emptyBoldAre`. Medido, **18 asignaciones menos por documento**.
- **Comportamiento idéntico, verificado**: los 75 DOCX del corpus institucional y el PDF del
  corpus se convierten **byte a byte igual**, con los mismos 76 pares fichero/porcentaje de
  integridad.
- El tiempo de conversión se mantiene en el mismo rango (33-52 ms antes, 31-36 ms después), con
  el ruido de la máquina (±40 %) ya aceptado en D16.

---

### D19 — El render del DOCX se parte por responsabilidad

**Estado:** aceptada

**Contexto.** Tras D18 quedaban dos ficheros grandes en el módulo DOCX, y los dos mezclaban tres
responsabilidades:

| Fichero | Líneas | Responsabilidades mezcladas |
|---|---|---|
| `render.go` | 450 | el renderer, el render de las listas y el de las tablas |
| `numbering.go` | 439 | los tipos y el parseo de `numbering.xml`, los contadores y el formato de las etiquetas |

**Decisión.** Los dos se parten, en el mismo paquete `converter`:

| Fichero | Responsabilidad |
|---|---|
| `internal/converter/docx/render.go` | El render del modelo a Markdown: recorre los bloques y escribe el resultado |
| `internal/converter/docx/lists.go` | El render de las listas: marcador, sangría y los tres modos (Markdown, HTML y literal) |
| `internal/converter/docx/tables.go` | El render de las tablas: filas, celdas y el escapado de `|` |
| `internal/converter/docx/numbering.go` | La lectura de `word/numbering.xml`: los tipos y las definiciones de nivel |
| `internal/converter/docx/numbering_state.go` | Los contadores de las listas: su avance y sus reinicios |
| `internal/converter/docx/numbering_labels.go` | La etiqueta resuelta y su formato (decimal, letra, romano) |

El corte de la numeración sigue una regla: **el XML en un fichero, el estado en otro y la
etiqueta en otro**. El render de las listas se queda aparte, en `lists.go`, porque es un
problema de Markdown y no de OOXML.

**Consecuencias.**

- El módulo DOCX queda repartido en catorce ficheros. El mayor del paquete sigue siendo
  `parser.go` (476 líneas); el segundo baja de 450 a 269, y el tercero de 439 a 207.
- Las asignaciones por documento no cambian: 144.802-144.808 antes y después. Es lo esperable en
  una reorganización que no toca la lógica.
- **Comportamiento idéntico, verificado**: los 76 DOCX del corpus institucional y el PDF del
  corpus se convierten **byte a byte igual**, con los mismos 77 pares fichero/porcentaje de
  integridad.

---

### D20 — Cada formato vive en su propio paquete

**Estado:** aceptada

**Contexto.** Tras D17, D18 y D19, los ficheros de `internal/converter` estaban separados por
responsabilidad, pero todos en **un solo paquete**: la fachada, los contratos, la verificación de
integridad y los dos formatos. Con 20 ficheros y unas 2.600 líneas, el nombre del paquete ya no
decía qué contenía, y un formato nuevo se mezclaba con la API pública.

La pregunta era si segmentar por dominios o por subdominios. La respuesta: **dominios** (la
fachada, la integridad y los formatos), no un subdominio por fichero.

**Decisión.** Cada formato pasa a su propio paquete, bajo `internal/converter/`:

```
internal/converter/            package converter  — la fachada y los contratos
internal/converter/docx/       package docx       — el módulo DOCX
internal/converter/pdf/        package pdf        — el módulo PDF
```

Reglas:

1. **La dependencia va en un solo sentido**: la raíz registra los constructores de los formatos.
   Un paquete de formato **no** importa la raíz: si necesitara algo de ella, el contrato estaría
   en el sitio equivocado.
2. Las interfaces se satisfacen **de forma implícita**: el paquete de formato implementa
   `Process`, `ExtractedText` y `SetSkipBeforeHeading` sin nombrar las interfaces del dispatcher.
3. La **API pública** de un paquete de formato es su constructor y su processor:
   `docx.NewProcessor()`, `pdf.NewProcessor()`. Los tipos internos conservan sus nombres
   (`docxParagraph`, `docxTable`): renombrar veinte tipos no mejora el código y ensucia el diff.
4. Las **interfaces opcionales** se comprueban con una aserción en un test del paquete raíz
   (`TestDocxProcessorExposesText`): si el processor de DOCX dejara de exponer su texto, la
   verificación de integridad de los DOCX caería a 0 sin avisar.

**Consecuencias.**

- El paquete raíz queda con cinco ficheros de producción: la fachada, los contratos y la
  integridad. Los formatos, con sus catorce y un ficheros.
- Los tests se reparten con el código: el módulo DOCX tiene su suite (`internal/converter/docx/`),
  el PDF tiene la suya, y el paquete raíz conserva los tests de la fachada y de la integridad. El
  test que comprueba que el descarte previo no altera la integridad se queda en la raíz, porque
  necesita la métrica.
- La garantía de que el texto que el processor expone no pierde nada pasa a estar **en el paquete
  DOCX** (`TestDocx_ExtractedTextMatchesIndependentRead`): compara los tokens del texto del
  processor con los de una lectura independiente del original, en vez de comparar porcentajes.
- **Comportamiento idéntico, verificado**: los 76 DOCX del corpus institucional y el PDF del
  corpus se convierten **byte a byte igual**, con los mismos 77 pares fichero/porcentaje de
  integridad. Las asignaciones por documento no cambian: 144.803-144.806 antes y después.

### D21 — Versionado semántico y tags solo en las releases relevantes

**Estado:** aceptada

**Contexto.** El repositorio declaraba su versión en una constante local de `cmd/mdcaster/main.go`
(`version = "1.0.0"`) y no tenía **ningún tag**: treinta commits sin un solo punto de retorno
publicado. Además, la spec de `cli/mdcaster.md` copiaba el número literal en su descripción del
flag `--version`, de modo que cualquier release dejaba la spec desfasada.

El problema de fondo no era el número, sino que no había una **fuente de verdad**: el valor
vivía en un fichero de producción, el tag no existía, y no había ninguna regla que decidiera
cuándo se publica una versión.

**Decisión.** La versión sigue SemVer y tiene una fuente única:

```
internal/version/version.go   package version   — const Version = "X.Y.Z"
```

Los dos binarios la leen de ahí para su flag `--version`. Ningún otro fichero copia el número.

El tipo de bump se deduce del tipo del commit (Conventional Commits, ya en uso en el proyecto):

| Tipo de commit | Bump | ¿Tag? |
|---|---|---|
| `!` en el tipo (`feat(converter)!: ...`) | major | sí |
| `feat` | minor | sí |
| `fix` | patch | sí |
| `docs`, `refactor`, `chore`, `test`, `perf` | — | no |

Reglas:

1. **Un tag es una release.** Se crea solo cuando el lote de commits desde el último tag incluye
   `feat`, `fix` o un cambio breaking. `docs`, `refactor`, `chore`, `test` y `perf` no bumpean ni
   etiquetan: no cambian el comportamiento observable.
2. El bump se aplica **una vez por lote**, no por commit, y se toma el **tipo mayor** del lote.
3. El tag es **anotado** (`git tag -a vX.Y.Z -m "mdcaster vX.Y.Z"`) y apunta al commit que ya
   lleva la versión bumpeada.
4. El tag se crea **en local**. Lo publica la persona con `git push origin vX.Y.Z`.
5. Un cambio incompatible (flags, formato de salida, API) se marca con `!` en el tipo del commit.
   Es la única señal que hace subir a major.

**Consecuencias.**

- `internal/version.Version` es el único sitio que se edita al publicar. Las specs y el README
  describen "la versión actual" en vez de copiar el número, así que **no envejecen en cada
  release**: el número solo aparece en el código y en el tag.
- `mdclean` gana `--version`: antes solo `mdcaster` declaraba la suya, y cada binario la
  tenía o no la tenía por su cuenta. Los dos leen el mismo sitio.
- El `--version` de `mdclean` se comprueba **antes** del recuento de argumentos: consultar la
  versión no exige un fichero de entrada.
- Un test del paquete `version` comprueba que el número cumple `MAJOR.MINOR.PATCH`. Es la guarda
  contra un bump mal escrito, que el tag haría inmutable.
- El tag queda **en local** hasta que alguien lo publica. Un tag borrado en GitHub no se puede
  volver a crear con el mismo nombre.

---

## Pendientes

Decisiones abiertas. Cada una necesita una entrada nueva en este documento cuando se resuelva.

### P1 — `Converter.ConvertDirectory` puede hacer `panic`

En la versión anterior de `internal/converter/converter.go` (línea 242) calculaba la ruta de salida
con `GetOutputPath(inputPath, "")[len(outputDir)+1:]`: el corte usa una longitud calculada sobre
**otra** cadena. Si `len(outputDir)+1` supera la longitud del nombre del fichero, el corte queda
fuera de rango.

La CLI no lo usa (hace su propio escaneo), así que `mdcaster` no se ve afectado. La biblioteca sí.

**Opciones:** corregir el cálculo, o eliminar el método y dejar solo la versión recursiva, que sí
funciona.

**Resuelto por [D17](#d17--el-dispatcher-se-parte-por-responsabilidad-y-se-elimina-el-código-muerto).**
El método se ha eliminado: el `panic` deja de existir.

### P2 — Escaneo no recursivo frente a la API recursiva

`mdcaster` solo procesa el primer nivel de la carpeta. La biblioteca ofrece
`ConvertDirectoryRecursive`, que sí es recursiva. Ninguna de las dos se usa.

**Opciones:** un flag `--recursive` en la CLI, o eliminar el método recursivo de la biblioteca.

**Resuelto por [D17](#d17--el-dispatcher-se-parte-por-responsabilidad-y-se-elimina-el-código-muerto).**
Se han eliminado los dos métodos de conversión por directorio. La CLI sigue escaneando solo el
primer nivel: un eventual `--recursive` es una decisión de la CLI, no de la librería.

### P3 — Hipervínculos DOCX parseados y no emitidos

`docx.Processor` lee `word/_rels/document.xml.rels` y guarda **todas** las relaciones, pero solo
las usa para localizar las imágenes: los enlaces del cuerpo se pierden.

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

### P7 — Las etiquetas de lista dentro de una celda se pierden

El texto de una celda se emite tal cual: los párrafos numerados que hay dentro de una tabla no
llevan marcador, porque una celda de Markdown no admite una lista. Es la misma limitación que
tiene `docx2md-cli` y que el fork `docx2md_fiel` documenta.

Medido: 8 etiquetas perdidas en 4 documentos del corpus institucional.

**Opciones:** aceptar el límite, o emitir la etiqueta como texto plano dentro de la celda
(`a) texto`), que no es una lista pero conserva la etiqueta.

### P8 — El subrayado no se conserva

**Resuelto por [D14](#d14--el-subrayado-se-emite-como-html-en-línea).**

`<w:u>` se ignoraba: Markdown no tiene subrayado. Ahora se emite como `<u>texto</u>`, el mismo
HTML en línea que usa `docx2md-cli`.

---

## Ver también

- [README.md](README.md) — cómo usar el proyecto
- [`.ai/specs/`](.ai/specs/) — el comportamiento vigente, especificado
- [AGENTS.md](AGENTS.md) — cómo trabajar en el repositorio
- [`.ai/plan/PLAN.md`](.ai/plan/PLAN.md) — plan de trabajo en curso
