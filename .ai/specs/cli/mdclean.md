# SPEC — `cmd/mdclean` (CLI de limpieza de Markdown)

**Módulo:** `cli/mdclean`
**Código:** `cmd/mdclean/main.go`, `cmd/mdclean/generic.go`, `cmd/mdclean/tdr.go`,
`cmd/mdclean/markdown.go`, `cmd/mdclean/corpus.go`
**Depende de:** solo biblioteca estándar

---

## Responsabilidad

Limpiar y normalizar un fichero Markdown **ya convertido**, para que sea legible y navegable.
Trabaja sobre el texto, no sobre el formato de origen: no sabe si el `.md` viene de un DOCX o de un
PDF.

## Flags

`cmd/mdclean/main.go:21-24`

| Flag | Tipo | Default | Descripción |
|---|---|---|---|
| `--inplace` | bool | `false` | Reescribe el fichero de entrada en el sitio |
| `--tdr` | bool | `false` | Modo TDR: limpieza/formatado de Markdown exportado de DOCX/PDF |

### Argumento posicional

Se exige **exactamente un** argumento: la ruta del `.md` a limpiar (`cmd/mdclean/main.go:43-47`).

| Situación | Comportamiento | Código de salida |
|---|---|---|
| 0 argumentos | `flag.Usage()` en stderr | `2` |
| ≥ 2 argumentos | `flag.Usage()` en stderr | `2` |
| 1 argumento | procesa | `0` / `1` |

El usage es:

```
Uso: mdclean [--inplace] <archivo.md>
```

(El nombre del binario se toma de `os.Args[0]`, línea 36.)

## Flujo de `main` — `cmd/mdclean/main.go:26-90`

```
1. flag.Usage = <custom>   → "Uso: <bin> [--inplace] <archivo.md>" + PrintDefaults
2. flag.Parse()
3. Si flag.NArg() != 1 → usage → exit 2
4. os.Open(path)            → error → stderr → exit 1
5. readAllLines(f)          → error → stderr → exit 1
6. splitFrontmatter(lines)  → (frontmatter, body)
7. clean = processTDR(body)  si --tdr
   clean = process(body)     si no
8. clean = append(frontmatter, clean...)
9. Si --inplace:
      os.Create(path)  ← TRUNCA el fichero
      writeLines(f, clean)
   Si no:
      writeLines(os.Stdout, clean)
```

### `splitFrontmatter` — `cmd/mdclean/main.go:136-149`

El bloque de frontmatter YAML se aparta **antes** de limpiar: son metadatos, no texto, y unirlo
con el cuerpo lo destrozaría.

| Entrada | Salida |
|---|---|
| La primera línea es `---` y hay otro `---` después | `frontmatter` = líneas hasta el cierre, `body` = el resto |
| La primera línea no es `---` | `frontmatter` = nil, `body` = todas las líneas |
| Hay un `---` inicial pero no se cierra | `frontmatter` = nil, `body` = todas las líneas |

El frontmatter se devuelve **tal cual**: no se normaliza ni se une con nada.

## Modo genérico (sin `--tdr`) — `process` (`cmd/mdclean/generic.go:45-64`)

Une líneas consecutivas que parecen haber sido partidas por el extractor de texto.

```
Para cada línea:
  Mientras shouldJoin(actual, siguiente):
    actual = actual + joinerBetween(actual, siguiente) + siguiente
  Emitir actual
```

### `shouldJoin` — `cmd/mdclean/generic.go:66-93`

Une solo si **todas** estas condiciones se cumplen:

1. Ni `actual` ni `siguiente` están vacías.
2. Ninguna de las dos es una **línea estructural** (`isStructuralLine`).
3. Al menos una de las dos **parece envuelta** (`looksWrappedLine`).
4. `actual` **no** termina en `.`, `!`, `?` ni `:` (se considera fin de frase intencionado).

Regla de diseño: **conservador**. Ante la duda, no une. El objetivo es no colapsar bloques de
direcciones ni títulos.

### `looksWrappedLine` — `cmd/mdclean/generic.go:95-106`

Una línea "parece envuelta" si:

1. tiene **≥ 72 caracteres**, o
2. contiene un tabulador, o
3. contiene 2+ espacios consecutivos.

### `isStructuralLine` — `cmd/mdclean/generic.go:108-133`

Una línea es estructural (y por tanto **no** se une) si:

| Categoría | Patrón |
|---|---|
| Vacía | `""` |
| Encabezado Markdown | empieza por `#` |
| Imagen Markdown | empieza por `![](` |
| Fila de tabla Markdown | empieza por `|` |
| Marcadores de sección | `NOTA GENERAL:`, `^(TITULO\|Capítulo\|Sección)\b`, `^Art\.`, `^(Concordancias\|Jurisprudencia):$`, `^(FUNCION EJECUTIVA\|ASAMBLEA NACIONAL\|PRESIDENCIA DE LA REPUBLICA)\b`, `^EL PLENO$`, `^CONSIDERANDO:$` |
| Viñetas | `^[-*]\s+` |
| Enumeraciones | `^[a-zA-Z]\)\s+` |
| Items numerados TDR | `^\d+\.-\s+` |
| Cabecera institucional | contiene `LEXIS FINDER` |

### `joinerBetween` — `cmd/mdclean/generic.go:135-167`

Decide el separador entre dos líneas que se van a unir:

| Situación | Separador | Motivo |
|---|---|---|
| Ya hay espacio en el borde | `""` | se conserva tal cual |
| `siguiente` empieza por mayúscula | `" "` | es una palabra nueva |
| `actual` termina en `,` `;` `.` `)` | `" "` | puntuación separadora |
| `actual` termina con una palabra corta ES (`de`, `del`, `la`, `el`, `y`, `o`, `a`, `al`, `en`, `por`, `para`, `con`, `sin`, `un`, `una`, `unos`, `unas`, `los`, `las`, `lo`, `que`, `se`, `su`, `sus`, `mi`, `mis`, `tu`, `tus`, `ya`, `no`) | `" "` | no pegar preposiciones |
| `siguiente` empieza con una de esas palabras cortas ES | `" "` | no pegar preposiciones |
| resto | `""` | se asume que la palabra estaba partida |

Las funciones auxiliares `firstWordToken` y `lastWordToken` (`cmd/mdclean/markdown.go:57-67`) extraen la primera y la
última palabra alfabética de una línea, saltando puntuación y dígitos.

## Modo TDR (`--tdr`) — `processTDR` (`cmd/mdclean/tdr.go:26-39`)

Nueve etapas sobre el array de líneas: siete pasadas y dos sub-pasos (4b y 5b). El orden es
normativo: cada etapa consume el resultado de la anterior.

### Pasada 1 — Normalización (`tdrPass1Normalize`, `cmd/mdclean/tdr.go:41-102`)

1. **Une líneas de imagen partidas** (`combineSplitImageLine`, `cmd/mdclean/tdr.go:401-418`): si una línea empieza
   por `![](` y contiene `{width=` sin `}`, y la siguiente contiene `height=` y `}`, las fusiona.
2. `normalizeLine` (`cmd/mdclean/tdr.go:446-455`): convierte NBSP (`\u00a0`) a espacio, quita el marcador de
   salto de línea `\` al final de la línea y sus espacios.
3. **Elimina números de página**: líneas de 1-3 dígitos (`digitsOnlyLine`) y líneas `Pág. N`.
4. **Elimina imágenes de cabecera repetidas** (`isRepeatedTDRHeaderImage`, `cmd/mdclean/tdr.go:372-386`): las
   referencias `media/rId40.png`, `media/rId43.png`, `media/rId46.png`, `media/rId49.jpg`,
   `media/rId52.png`, `media/rId55.png` se conservan **solo la primera vez**.
5. **Elimina cabeceras institucionales repetidas** (`isRepeatedTDRHeaderLine`, `cmd/mdclean/tdr.go:388-399`): solo
   la primera aparición de
   `GAD MUNICIPAL DE MACHALA`, `DIRECCIÓN DE TECNOLOGÍA DE LA INFORMACIÓN`,
   `COORDINACIÓN DE DESARROLLO DE SISTEMAS`,
   `COORDINACIÓN DE DESARROLLO DE SISTEMAS INFORMÁTICOS`.
   La comparación normaliza mayúsculas y espacios.
6. Sustituye la secuencia `\.` (líder de puntos escapado) por `.` y elimina el resto de `\` sueltos.
7. **Normaliza las viñetas**: `➢`, `•`, `–`, `—` → `-`.
8. Colapsa espacios múltiples (`collapseSpaces`), **excepto** en las líneas de imagen.

### Pasada 2 — Líneas en blanco (`collapseBlankLines`, `cmd/mdclean/tdr.go:104-122`)

Colapsa 2+ líneas en blanco consecutivas en una sola.

### Pasada 3 — Fusión de líneas solo-negrita (`mergeEmphasisLines`, `cmd/mdclean/tdr.go:124-146`)

Une líneas consecutivas que son únicamente negrita (`^\*\*(.+?)\*\*$`) en una sola línea
`**a b c**`.

### Pasada 4 — Negrita → encabezados (`tdrPass4BoldToHeadings`, `cmd/mdclean/tdr.go:148-178`)

Convierte una línea solo-negrita en encabezado según su contenido:

| Condición (sobre el texto en mayúsculas) | Salida |
|---|---|
| `INDICE` | `## INDICE` |
| `CONTENIDO` | `### CONTENIDO` |
| primera vez que aparece `TÉRMINOS` | `# <contenido>` |
| empieza por `GOBIERNO`, `GAD`, `MUNICIPAL`, `DIRECCIÓN`, `COORDINACIÓN`, `SECCIÓN`, `OBJETO`, `NOMBRE`, `CONTRATACIÓN`, `TÉRMINOS`, `ANTECEDENTES`, `JUSTIFICACION`, `OBJETIVOS`, `ALCANCE`, `METODOLOGÍA`, `INFORMACIÓN`, `PRODUCTOS`, `PLAZO`, `PERSONAL`, `FORMA`, `CONDICIONES`, `ANEXO` | `## <contenido>` |
| resto | `**<contenido>**` |

Antes de decidir, elimina los `*` internos del contenido (artefacto de énfasis anidado tipo
`**"***texto***"**`).

La regla de `TÉRMINOS` aplica **solo la primera vez** (`hasMainTitle`): el título principal del
documento. Las apariciones posteriores quedan como `##`.

### Pasada 4b — Normalización del índice (`tdrPass4bNormalizeTOC`, `cmd/mdclean/tdr.go:180-229`)

Activa el modo índice al ver `### CONTENIDO` y lo desactiva en el siguiente encabezado `##` (salvo
`## INDICE`). Dentro del índice:

1. Una línea `1.` seguida de la línea siguiente se fusiona en `1. <texto>`.
2. Una entrada terminada en `.` seguida de un número (número de página) se fusiona.
3. Los líderes de puntos (5+ `.`) se sustituyen por ` … `.

### Pasada 5 — Fusión de líneas solo-cursiva (`mergeEmphasisLines`, `cmd/mdclean/tdr.go:124-146`)

Une líneas consecutivas que son únicamente cursiva (`^\*(.+?)\*$`) en un único párrafo `*a b c*`.

### Pasada 5b — Cursivas entre comillas partidas (`fixQuotedItalicSpans`, `cmd/mdclean/tdr.go:249-298`)

`fixQuotedItalicSpans` repara el caso `"*texto` que continúa en la línea siguiente hasta `*"`.
Elimina los `*` internos del fragmento y colapsa espacios.

### Pasada 6 — Desenvoltura de párrafos (`tdrPass6UnwrapParagraphs`, `cmd/mdclean/tdr.go:300-328`)

Une líneas consecutivas en un párrafo **solo** si ninguna de las dos es estructural para TDR
(`isTDRStructuralLine`: vacía, solo-negrita, solo-cursiva, encabezado, imagen, viñeta, fila de
tabla, o un `N.` de índice) y la línea actual no termina en `.`, `!`, `?` ni `:`.

El separador lo decide `joinerBetweenTDR` (`cmd/mdclean/tdr.go:420-444`): espacio salvo que la línea termine en
`-` o `/` (partida de palabra) o la siguiente empiece por `.`, `,`, `;`, `:`, `)`, `]`, `}`, `?`,
`!`, `%`.

### Pasada 7 — Post-proceso (`tdrPass7PostProcess`, `cmd/mdclean/tdr.go:330-356`)

1. Corrige énfasis anidado: `**"***` → `**"`, `***"**` → `"**`.
2. En las líneas de encabezado, colapsa letras sueltas mayúsculas en una sola palabra
   (`collapseSpacedLetterRunsInHeading`, `cmd/mdclean/tdr.go:463-497`): `D E L  N O M B R E` → `DEL NOMBRE`. Solo
   para Runs de **2 a 4** letras.
3. Colapsa espacios (excepto en imágenes).
4. Vuelve a colapsar las líneas en blanco consecutivas.

## Lectura y escritura

### `readAllLines` — `cmd/mdclean/main.go:92-108`

Usa `bufio.Scanner` con buffer inicial de 64 KiB y **máximo de 4 MiB** por línea. Las líneas más
largas que eso provocan error.

### `writeLines` — `cmd/mdclean/main.go:110-134`

1. Escribe cada línea separada por `\n`.
2. **Siempre** añade un `\n` final, incluso con fichero vacío (en ese caso escribe solo el `\n`).
3. Usa un `bufio.Writer` y hace `Flush` al final.

### Modo `--inplace` — `cmd/mdclean/main.go:64-85`

```
os.Create(path)   ← TRUNCA el fichero de entrada
writeLines(f, clean)
```

**Decisión deliberada** (comentario en el código, línea 66): en carpetas compartidas (p. ej.
`vboxsf`) el rename atómico sobre un fichero existente falla, así que se usa truncate + write
directo para mantener un comportamiento consistente.

Consecuencia: si el proceso falla a mitad, el fichero de entrada queda **vacío o corrupto**. No
hay fichero temporal ni backup.

## Gotchas

1. **`--inplace` es destructivo y no atómico.** No "optimizar" a rename + replace: el comentario del
   código explica por qué.
2. `--inplace` **no** hace copia de seguridad. Probar siempre sobre una copia.
3. El modo TDR está **muy acoplado al corpus institucional**: los nombres de cabecera, las imágenes
   `media/rIdNN` y la lista de encabezados de sección son específicos de los export DOCX/PDF de ese
   tipo de documentos. **Todos esos datos están en `cmd/mdclean/corpus.go`**: generalizarlos es tocar
   ese fichero, no el comando.
4. El modo TDR **elimina contenido**: números de página, cabeceras repetidas e imágenes de cabecera
   repetidas se pierden de forma irreversible. Las líneas de 1-3 dígitos se eliminan **sin
   comprobar** que sean números de página: un párrafo como `2024` desaparecería.
5. `digitsOnlyLine` (`^\d{1,3}$`) borra también cifras sueltas de contenido legítimo.
6. El límite de 4 MiB por línea en `readAllLines` es duro: una línea mayor hace fallar el comando.
7. `--tdr` y `--inplace` son combinables. El orden de aplicación es: TDR primero, escritura después.
8. `mdclean` nunca falla por "contenido no convertible": solo falla por E/S o por argumentos.
9. Las **filas de tabla** son líneas estructurales: sin eso, la limpieza unía la tabla entera en
   una sola línea y la destruía. El conversor emite tablas Markdown desde la fase de fidelidad
   DOCX, así que la protección es obligatoria.
10. El **frontmatter** se aparta antes de limpiar y se devuelve tal cual.
