# Markdown Caster

<p align="center">
  <strong>🚀 Convertidor universal de múltiples formatos a Markdown</strong>
</p>

<p align="center">
  Convierte archivos de múltiples formatos (`.docx`, `.pdf`, etc.) a formato Markdown utilizando concurrencia para máximo rendimiento.
</p>

---

## 📝 Características

- **Soporte múltiple**: Convierte archivos de múltiples formatos a Markdown (DOCX, PDF, etc.)
- **Extrae TODO el contenido**: Párrafos, encabezados, listas, tablas, notas al pie e imágenes
- **Markdown estructurado**: Tablas como tablas Markdown y listas con su numeración real
- **Preserva estilos**: Negrita, cursiva, tachado, encabezados y listas se mantienen
- **Concurrencia inteligente**: Goroutines para procesamiento paralelo ultra-rápido
- **Flexible**: Convierte un archivo o una carpeta completa
- **Salida personalizable**: Define tu propia carpeta de destino
- **Verificación de integridad**: Muestra el porcentaje de contenido que se conservó en la conversión
- **Indicador visual**: 🟢 Verde, 🟡 Amarillo, 🟠 Naranja, 🔴 Rojo según integridad
- **Limpieza posterior**: `mdclean` normaliza el Markdown generado (modo genérico y modo TDR)
- **Sin dependencias externas**: Solo la biblioteca estándar de Go

## 📦 Instalación

### Requisitos

- Go 1.21 o superior
- `pdftotext` (poppler-utils) — **solo si vas a convertir PDF**

```bash
# Ubuntu/Debian
sudo apt install poppler-utils
# macOS
brew install poppler
```

### Compilar desde código fuente

```bash
# Clonar o navegar al directorio del proyecto
cd mdcaster

# Compilar el conversor
go build -o mdcaster ./cmd/mdcaster

# Compilar el cleaner
go build -o mdclean ./cmd/mdclean

# O instalar globalmente
go install ./cmd/mdcaster
go install ./cmd/mdclean
```

## 🚀 Uso

### Uso Básico

```bash
# Convertir un archivo específico
./mdcaster -i documento.docx

# Convertir todos los archivos de una carpeta
./mdcaster -i ./documentos

# También funciona sin flags
./mdcaster documento.docx
./mdcaster ./documentos
```

### Opciones Avanzadas

```bash
# Especificar carpeta de salida
./mdcaster -i ./documentos -o ./markdown

# Usar número específico de workers
./mdcaster -i ./documentos -w 8

# Combinar opciones
./mdcaster -i ./documentos -o ./salida -w 4
```

### Flags Disponibles

| Flag | Corto | Descripción | Default |
|------|-------|-------------|---------|
| `--input` | `-i` | Archivo o carpeta de entrada | - |
| `--output` | `-o` | Carpeta de destino | `converted` |
| `--workers` | `-w` | Número de workers paralelos | CPUs |
| `--version` | - | Mostrar versión | - |
| `--help` | - | Mostrar ayuda | - |

> La carpeta de salida siempre recibe un subdirectorio con timestamp (`YYYYMMDD-HHMMSS`) para
> agrupar cada ejecución. Ejemplo: `./documentos/converted/20261020-091500/`.

> Si un documento tiene imágenes en el cuerpo, se extraen a una carpeta `<nombre>_assets/` junto
> al `.md` que las referencia.

> El escaneo de una carpeta es **no recursivo**: solo se procesa su primer nivel.

## 📊 Ejemplo de Salida

```
╔═══════════════════════════════════════════════════════════════╗
║                     Markdown Caster                           ║
║            Convertidor de Múltiples Formatos                  ║
║                    Powered by Go 🚀                            ║
╚═══════════════════════════════════════════════════════════════╝

📁 Escaneando carpeta: ./documentos
📂 Carpeta de salida: ./documentos/converted/20261020-091500
📄 Archivos a procesar: 10
⚡ Workers paralelos: 8

✅ [1/10] 🟢 Convertido: documento1.docx     100.0%
✅ [2/10] 🟡 Convertido: documento3.docx      87.5%
...

════════════════════════════════════════════════════════════════
                      📊 RESUMEN DE CONVERSIÓN
════════════════════════════════════════════════════════════════

  ✅ Archivos convertidos exitosamente: 10

  📋 INTEGRIDAD DE CONTENIDO:
    Promedio: 95.20%
    Mínima: 87.50%
    Máxima: 100.00%

  ⏱️  Tiempo total de ejecución: 0.45s
  📈 Tiempo promedio por archivo: 0.089s
  🚀 Velocidad: 22.22 archivos/segundo

  ⚡ Speedup por concurrencia: 1.98x más rápido
════════════════════════════════════════════════════════════════
```

### Indicador de Integridad

El convertidor muestra un indicador visual de la integridad del contenido:

| Símbolo | Rango | Significado |
|---------|-------|-------------|
| 🟢 Verde | ≥95% | Integridad óptima - Todo el contenido se conservó |
| 🟡 Amarillo | 80-94% | Integridad buena - Casi todo el contenido se conservó |
| 🟠 Naranja | 50-79% | Integridad aceptable - Parte del contenido se conservó |
| 🔴 Rojo | <50% | Integridad deficiente - Se perdió mucho contenido |

La integridad mide qué porcentaje de las palabras del texto original aparece en el Markdown
generado. Cada archivo muestra su porcentaje durante la conversión, y al final un resumen con el
**Promedio**, la **Mínima** y la **Máxima**.

> **Nota**: la verificación de integridad solo es válida para **DOCX**. En archivos PDF el porcentaje
> se muestra como 0 % (🔴) porque el verificador solo sabe leer el original en formato DOCX. En un
> PDF el 🔴 no indica un fallo de conversión.

## 🧹 Limpieza de Markdown (`mdclean`)

`mdclean` post-procesa un `.md` ya convertido para que quede legible y navegable.

```bash
# Limpiar y mostrar por stdout
./mdclean documento.md

# Limpiar y reescribir el archivo en el sitio
./mdclean --inplace documento.md

# Modo TDR: limpieza específica para exportaciones DOCX/PDF institucionales
./mdclean --tdr documento.md
./mdclean --tdr --inplace documento.md
```

| Flag | Descripción |
|------|-------------|
| `--inplace` | Reescribe el archivo de entrada en el sitio (en vez de imprimir por stdout) |
| `--tdr` | Modo TDR: normaliza encabezados, índices, viñetas, cabeceras repetidas y números de página |

`mdclean` exige **exactamente un** argumento: la ruta del `.md` a limpiar.

> `--inplace` no es atómico: trunca y reescribe el fichero. Prueba siempre sobre una copia.

## 🔧 Formato Soportado

La conversión de DOCX preserva los siguientes elementos:

- ✅ **Encabezados** (H1-H6)
- ✅ **Texto en negrita**
- ✅ **Texto en cursiva**
- ✅ **Texto tachado**
- ✅ **Listas con viñetas y numeradas** (con la numeración real del documento)
- ✅ **Tablas** (como tablas Markdown)
- ✅ **Notas al pie**
- ✅ **Imágenes** (extraídas junto al `.md`)
- ✅ **Metadatos del documento** (autor y fechas, en el frontmatter)
- ✅ **Saltos de línea**

La conversión de PDF produce **texto plano** con el layout original preservado (`pdftotext -layout`),
sin estructura Markdown. Usa `mdclean` para normalizarlo.

## 📝 Licencia

MIT. Ver [LICENSE](LICENSE).

## 🤝 Contribuir

Antes de tocar el código, lee las especificaciones: son la fuente de verdad del comportamiento
deseado.

- [`.ai/specs/overview.md`](.ai/specs/overview.md) — Visión general, reglas de negocio, taxonomía de errores
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — Decisiones de arquitectura y pendientes
- [`AGENTS.md`](AGENTS.md) — Protocolo de trabajo para agentes

¡Las contribuciones son bienvenidas! Por favor, abre un issue o pull request.

---

<p align="center">
  Hecho con ❤️ y Go
</p>
