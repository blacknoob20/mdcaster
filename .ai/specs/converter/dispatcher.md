# SPEC — `internal/converter/converter.go` (Dispatcher)

**Módulo:** `converter/dispatcher`
**Código:** `internal/converter/converter.go`
**Depende de:** `internal/converter/processor.go` (Factory), `internal/converter/integrity.go` (Integridad)

---

## Responsabilidad

Exponer la **API interna** de conversión y orquestar la selección de procesador. No contiene lógica
de formato: delega en los procesadores. La verificación de integridad vive en
[`integrity.md`](integrity.md).

> **Alcance de uso.** El paquete vive en `internal/`, así que **no es importable desde fuera del
> módulo `mdcaster`**. Es una API interna, no una librería pública. Si alguna vez se quiere
> publicar, hay que mover el paquete fuera de `internal/` y definir su contrato de estabilidad.

## `Converter` — `internal/converter/converter.go:8-11`

```go
type Converter struct {
    factory *ProcessorFactory
}
```

`NewConverter()` crea la factory con los procesadores registrados. **Cada `Converter` tiene su
propia factory**: no hay estado compartido. Por eso `cmd/mdcaster` crea un `Converter` por worker.

## API interna

| Método | Líneas | Contrato |
|---|---|---|
| `NewConverter() *Converter` | 13-18 | Crea el dispatcher con la factory por defecto. |
| `Convert(input, output string) error` | 20-23 | Convierte un fichero. Delegación total en `factory.Convert`. |
| `ConvertFile(input, output string) error` | 25-28 | **Alias** de `Convert` (compatibilidad hacia atrás). |
| `SetSkipBeforeHeading(enabled bool)` | 30-40 | Hace que la conversión empiece en el primer encabezado. |
| `GetOutputPath(inputPath, outputDir string) string` | 42-50 | Nombre base sin extensión + `.md`, dentro de `outputDir`. |

### `Convert` y `ConvertFile` — `internal/converter/converter.go:20-28`

`Convert` delega en `factory.Convert`, que hace tres cosas: comprueba que la entrada existe
(`os.Stat`), resuelve el procesador por extensión y crea el directorio de salida (`os.MkdirAll`)
antes de convertir.

`ConvertFile` es un alias sin comportamiento propio: la CLI y los tests lo usan. Se conserva por
compatibilidad.

### `SetSkipBeforeHeading` — `internal/converter/converter.go:30-40`

Hace que la conversión de los DOCX empiece en el primer encabezado: se descartan la portada, el
índice y las tablas de versión que lo anteceden.

1. El descarte lo aplica el processor, no el dispatcher: el texto que se extrae para la
   verificación de integridad se recorta también, de modo que la mide sobre el contenido que se
   conserva.
2. Si el documento no tiene ningún encabezado, se conserva todo.
3. Los formatos que no lo soportan (el PDF) lo ignoran: la propagación va por la interfaz
   opcional `SkipBeforeHeadingSetter`.

### `GetOutputPath` — `internal/converter/converter.go:42-50`

```go
baseName := filepath.Base(inputPath)
nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
return filepath.Join(outputDir, nameWithoutExt+".md")
```

Reglas:

1. Usa **solo el nombre base**: se pierde la estructura de directorios.
2. La extensión se sustituye por `.md`; si el original ya era `.md`, el resultado es el mismo nombre.
3. `filepath.Join` normaliza separadores y limpia la ruta.

## Lo que ya no está aquí

Estas piezas se movieron o se eliminaron. Ver
[D17](../../../ARCHITECTURE.md#d17--el-dispatcher-se-parte-por-responsabilidad-y-se-elimina-el-código-muerto).

| Antes | Ahora |
|---|---|
| `Converter.CheckIntegrity`, `CheckIntegrity`, `CheckIntegrityAgainst`, `normalizeContent` | [`integrity.go`](integrity.md) |
| `ConversionResult` | `cmd/mdcaster/result.go`: solo la CLI rellena `Duration` e `IntegrityPercent` |
| `ConvertDirectory` | Eliminada: tenía un `panic` y no la usaba nadie |
| `ConvertDirectoryRecursive` | Eliminada: no la usaba nadie |
| `ConvertPDFFile` | Eliminada: no la usaba nadie y no era un alias de `Convert` |
| `ByteCountSI` | Eliminada: no la usaba nadie |

## Gotchas

1. `Convert` no crea el directorio de salida si `outputPath` no tiene directorio: `filepath.Dir`
   devuelve `.`, que ya existe.
2. `GetOutputPath` descarta la estructura de directorios: solo es válido para el escaneo plano.
3. `ConvertFile` y `Convert` son lo mismo: el alias no añade nada.
