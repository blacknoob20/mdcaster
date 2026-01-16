# DOCX to Markdown Converter

<p align="center">
  <strong>🚀 Convertidor de alta velocidad de Word a Markdown</strong>
</p>

<p align="center">
  Convierte archivos `.docx` a formato Markdown utilizando concurrencia para máximo rendimiento.
</p>

---

## 📝 Características del Convertidor

- **Extrae TODO el contenido**: Párrafos, encabezados, listas, y contenido de tablas Word como texto formateado
- **Formato Markdown limpio**: Sin tablas Markdown complejas, solo texto fácil de leer
- **Preserva estilos**: Negrita, cursiva, encabezados y listas se mantienen
- **Concurrencia inteligente**: Goroutines para procesamiento paralelo ultra-rápido
- **Flexible**: Convierte un archivo o una carpeta completa
- **Salida personalizable**: Define tu propia carpeta de destino
- **Verificación de integridad**: Muestra el porcentaje de contenido que se conservó en la conversión
- **Indicador visual**: 🟢 Verde, 🟡 Amarillo, 🟠 Naranja, 🔴 Rojo según integridad
- **Multiplataforma**: Funciona en Linux, macOS y Windows

## ⚡ Características

## 📦 Instalación

### Requisitos

- Go 1.21 o superior

### Compilar desde código fuente

```bash
# Clonar o navegar al directorio del proyecto
cd docx-to-md

# Descargar dependencias
go mod tidy

# Compilar
go build -o docx-to-md ./cmd/docx-to-md

# O instalar globalmente
go install ./cmd/docx-to-md
```

## 🚀 Uso

### Uso Básico

```bash
# Convertir un archivo específico
./docx-to-md -i documento.docx

# Convertir todos los archivos de una carpeta
./docx-to-md -i ./word

# También funciona sin flags
./docx-to-md documento.docx
./docx-to-md ./word
```

### Opciones Avanzadas

```bash
# Especificar carpeta de salida
./docx-to-md -i ./word -o ./markdown

# Usar número específico de workers
./docx-to-md -i ./word -w 8

# Combinar opciones
./docx-to-md -i ./documentos -o ./salida -w 4
```

### Flags Disponibles

| Flag | Corto | Descripción | Default |
|------|-------|-------------|---------|
| `--input` | `-i` | Archivo DOCX o carpeta de entrada | - |
| `--output` | `-o` | Carpeta de destino | `converted` |
| `--workers` | `-w` | Número de workers paralelos | CPUs |
| `--version` | - | Mostrar versión | - |
| `--help` | - | Mostrar ayuda | - |

## 📊 Ejemplo de Salida

```
╔═══════════════════════════════════════════════════════════════╗
║                     DOCX to Markdown                          ║
║              Convertidor de Alta Velocidad                    ║
║                    Powered by Go 🚀                           ║
╚═══════════════════════════════════════════════════════════════╝

📁 Escaneando carpeta: ./word
📂 Carpeta de salida: ./word/converted
📄 Archivos a procesar: 10
⚡ Workers paralelos: 8

✅ [1/10] 🟢 Convertido: documento1.docx     100.0%
✅ [2/10] 🟢 Convertido: documento2.docx     100.0%
✅ [3/10] 🟡 Convertido: documento3.docx      87.5%
...

═══════════════════════════════════════════════════════════════
                      📊 RESUMEN DE CONVERSIÓN
═══════════════════════════════════════════════════════════════

  ✅ Archivos convertidos exitosamente: 10

  📋 INTEGRIDAD DE CONTENIDO:
    Promedio: 95.20%
    Mínima: 87.50%
    Máxima: 100.00%

  ⏱️  Tiempo total de ejecución: 0.45s
  📈 Tiempo promedio por archivo: 0.089s
  🚀 Velocidad: 22.22 archivos/segundo

  ⚡ Speedup por concurrencia: 1.98x más rápido
═══════════════════════════════════════════════════════════════
```

### Indicador de Integridad

El convertidor ahora muestra un indicador visual de la integridad del contenido:

| Símbolo | Rango | Significado |
|---------|-------|------------|
| 🟢 Verde | ≥95% | Integridad óptima - Todo el contenido se conservó |
| 🟡 Amarillo | 80-94% | Integridad buena - Casi todo el contenido se conservó |
| 🟠 Naranja | 50-79% | Integridad aceptable - Parte del contenido se conservó |
| 🔴 Rojo | <50% | Integridad deficiente - Se perdió mucho contenido |

Cada archivo muestra su porcentaje de integridad durante la conversión, y al final se muestra un resumen con:
- **Promedio**: Integridad promedio de todos los archivos convertidos
- **Mínima**: El archivo con menor integridad
- **Máxima**: El archivo con mayor integridad

## 🔧 Formato Soportado

La conversión preserva los siguientes elementos:

- ✅ **Encabezados** (H1-H6)
- ✅ **Texto en negrita**
- ✅ **Texto en cursiva**
- ✅ **Texto tachado**
- ✅ **Listas con viñetas**
- ✅ **Saltos de línea**
- ✅ **Contenido de tablas Word extraído como párrafos** (para mejor legibilidad)

## 📁 Estructura del Proyecto

```
docx-to-md/
├── cmd/
│   └── docx-to-md/
│       └── main.go          # Punto de entrada
├── internal/
│   └── converter/
│       └── converter.go     # Lógica de conversión
├── word/                    # Archivos DOCX de entrada
├── converted/               # Archivos MD convertidos
├── go.mod                   # Módulo Go
├── go.sum                   # Checksums de dependencias
└── README.md                # Este archivo
```

## 🏎️ Rendimiento

El convertidor utiliza las goroutines de Go para procesar múltiples archivos en paralelo:

- **Sin concurrencia**: 1 archivo a la vez
- **Con concurrencia**: N archivos simultáneos (donde N = número de CPUs)

Esto resulta en mejoras de velocidad de hasta **Nx** en sistemas multi-core.

## 📝 Licencia

MIT License - Siéntete libre de usar y modificar.

## 🤝 Contribuir

¡Las contribuciones son bienvenidas! Por favor, abre un issue o pull request.

---

<p align="center">
  Hecho con ❤️ y Go
</p>
