# SPEC — `internal/converter/pdf_processor.go` (PDF → texto plano)

**Módulo:** `converters/pdf`
**Código:** `internal/converter/pdf_processor.go`
**Implementa:** `converter.FileProcessor`
**Dependencias:** `os/exec` (binario externo `pdftotext`)

---

## Responsabilidad

Convertir un `.pdf` a un fichero de texto plano en `outputPath`. **No** se genera Markdown
estructurado: el PDF no aporta la semántica de títulos y listas que sí tiene el DOCX.

> **Excepción documentada a la regla de cero dependencias.**
> Ver [decisión D4](../../../ARCHITECTURE.md#d4--pdf-con-pdftotext-excepción-documentada).
> La biblioteca estándar de Go no incluye un parser de PDF: escribirlo exigiría implementar la
> tabla xref, los object streams, la decodificación de fuentes (ToUnicode, CID, encodings) y los
> operadores de texto de contenido. Es un proyecto en sí mismo, con su propio corpus de
> comparación. Mientras tanto, `pdftotext` es la única dependencia externa del proyecto, y es de
> **runtime**, no de build.

## Entrada / salida

| Aspecto | Contrato |
|---|---|
| Entrada | Ruta a un `.pdf` legible por `pdftotext` |
| Salida | Texto plano en `outputPath`, permisos `0644` |
| Extensión registrada | `.pdf` (minúsculas) |
| Dependencia | `pdftotext` (poppler-utils) **en el PATH** |

## Flujo de `Process` — `internal/converter/pdf_processor.go:20-32`

```
1. extractText(inputPath)          → ERR_PDFTOTEXT
2. os.WriteFile(outputPath, text, 0644) → ERR_WRITE_MARKDOWN
```

`PdfProcessor` no tiene estado: es un struct vacío. `NewPdfProcessor()` (líneas 15-17) devuelve un
`&PdfProcessor{}`.

## `extractText` — `internal/converter/pdf_processor.go:34-43`

```
cmd := exec.Command("pdftotext", "-layout", inputPath, "-")
out, err := cmd.Output()
```

Reglas:

1. El comando es **exactamente** `pdftotext -layout <entrada> -`: la salida va a stdout (`-`) y
   `-layout` preserva la disposición original del texto.
2. `-layout` es lo que permite que las columnas del PDF conserven su alineación en el `.md`.
3. `cmd.Output()` captura **solo stdout**. La stderr de `pdftotext` no se propaga al usuario; el
   error que se ve es el de exec.
4. El texto se normaliza con `strings.TrimSpace` antes de escribirse: sin saltos de línea al
   principio ni al final.
5. No se post-procesa el texto: no se capitalizan títulos, no se convierten listas, no se eliminan
   cabeceras repetidas. Para eso está `mdclean`.

### Error de dependencia ausente

Si `pdftotext` no está en el PATH, `exec` falla con `exec: "pdftotext": executable file not found in $PATH`
y el mensaje de mdcaster es:

```
error extrayendo texto del PDF con pdftotext: %w. Instala 'poppler-utils' o asegura que 'pdftotext' esté en PATH
```

Este mensaje es **parte del contrato**: nombra la dependencia y la acción correctiva.

## Relación con la verificación de integridad

**`CheckIntegrity` no soporta PDF.** `PdfProcessor` no implementa la interfaz `TextExtractor`, de
modo que no expone el texto que extrajo y la verificación no tiene con qué comparar: el resultado
es `0`.

Consecuencia visible: al convertir un PDF, `mdcaster` muestra **🔴 0.0 %** aunque la conversión haya
sido correcta. No es un fallo de conversión: es una limitación de la medición.

Ver [../quality/integridad.md](../quality/integridad.md).

## Gotchas

1. **La salida no es Markdown estructurado.** Un PDF escaneado llega como texto plano con saltos de
   línea según el layout original. El pipeline previsto es `mdcaster` → `mdclean` para esa parte.
2. **`-layout` es obligatorio en la práctica**: sin él, `pdftotext` entrega el texto en orden de
   lectura y las tablas se desarman.
3. El processor no valida que el fichero sea realmente un PDF: delega en `pdftotext`, que es quien
   decide. Un fichero renombrado a `.pdf` produce un error de `pdftotext`, no un error de validación.
4. `PdfProcessor` es stateless y seguro para uso concurrente.
5. Un PDF protegido con contraseña o corrupto produce `ERR_PDFTOTEXT`. El mensaje no distingue
   ambos casos: el detalle real de `pdftotext` no se propaga (solo stdout se captura).
