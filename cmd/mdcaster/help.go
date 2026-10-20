package main

import (
	"fmt"
	"time"
)

// printHelp muestra la ayuda completa
func printHelp() {
	info.Println("Markdown Caster - Convertidor universal de múltiples formatos a Markdown")
	fmt.Println()
	fmt.Println("DESCRIPCIÓN:")
	fmt.Println("  Convierte archivos de múltiples formatos (DOCX, PDF, etc.) a formato Markdown")
	fmt.Println("  utilizando procesamiento concurrente para máxima velocidad.")
	fmt.Println("  Incluye verificación de integridad del contenido para garantizar que")
	fmt.Println("  todo el texto del archivo origen se encuentre en el archivo generado.")
	fmt.Println()
	printUsage()
	fmt.Println()
	fmt.Println("OPCIONES:")
	fmt.Println("  -i, --input    Archivo o carpeta con archivos soportados a convertir")
	fmt.Println("  -o, --output   Carpeta de destino para archivos convertidos (default: 'converted')")
	fmt.Println("  -w, --workers  Número de workers paralelos (default: número de CPUs)")
	fmt.Println("      --skip-before-heading")
	fmt.Println("                 Empezar la conversión en el primer encabezado:")
	fmt.Println("                 descarta portada, índice y tablas de versión")
	fmt.Println("      --version  Mostrar versión del programa")
	fmt.Println("      --help     Mostrar esta ayuda")
	fmt.Println()
	fmt.Println("INDICADOR DE INTEGRIDAD:")
	fmt.Println("  🟢 Verde (≥95%)    - Integridad óptima")
	fmt.Println("  🟡 Amarillo (80%)  - Integridad buena")
	fmt.Println("  🟠 Naranja (50%)   - Integridad aceptable")
	fmt.Println("  🔴 Rojo (<50%)     - Integridad deficiente")
	fmt.Println()
	fmt.Println("EJEMPLOS:")
	fmt.Println("  # Convertir un archivo específico")
	fmt.Println("  mdcaster -i documento.docx")
	fmt.Println()
	fmt.Println("  # Convertir todos los archivos de una carpeta")
	fmt.Println("  mdcaster -i ./documentos")
	fmt.Println()
	fmt.Println("  # Especificar carpeta de salida")
	fmt.Println("  mdcaster -i ./documentos -o ./markdown")
	fmt.Println()
	fmt.Println("  # Usar 8 workers paralelos")
	fmt.Println("  mdcaster -i ./documentos -w 8")
	fmt.Println()
	fmt.Println("  # Descartar portada, índice y tablas de versión")
	fmt.Println("  mdcaster -i ./documentos --skip-before-heading")
}

// printUsage muestra el uso básico
func printUsage() {
	fmt.Println("USO:")
	fmt.Println("  mdcaster -i <archivo|carpeta> [-o <carpeta_salida>] [-w <workers>]")
	fmt.Println("  mdcaster <archivo|carpeta>")
}

// formatDuration formatea la duración de manera legible
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return fmt.Sprintf("%dm %.2fs", int(d.Minutes()), d.Seconds()-float64(int(d.Minutes()))*60)
}
