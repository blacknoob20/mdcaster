package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"mdcaster/internal/converter"
	"mdcaster/internal/version"
)

const (
	banner = `
╔═══════════════════════════════════════════════════════════════╗
║                     Markdown Caster                           ║
║            Convertidor de Múltiples Formatos                  ║
║                    Powered by Go 🚀                           ║
╚═══════════════════════════════════════════════════════════════╝
`
)

func main() {
	// Definir flags
	inputPath := flag.String("input", "", "Ruta al archivo DOCX o carpeta con archivos DOCX")
	inputShort := flag.String("i", "", "Ruta al archivo DOCX o carpeta (forma corta)")
	outputDir := flag.String("output", "converted", "Carpeta de destino para los archivos Markdown")
	outputShort := flag.String("o", "", "Carpeta de destino (forma corta)")
	workers := flag.Int("workers", 0, "Número de workers para procesamiento paralelo (0 = auto)")
	workersShort := flag.Int("w", 0, "Número de workers (forma corta)")
	skipBeforeHeading := flag.Bool("skip-before-heading", false, "Empezar la conversión en el primer encabezado (descarta portada, índice y tablas de versión)")
	showVersion := flag.Bool("version", false, "Mostrar versión")
	showHelp := flag.Bool("help", false, "Mostrar ayuda")

	flag.Parse()

	// Mostrar banner
	highlight.Print(banner)

	// Mostrar versión si se solicita
	if *showVersion {
		info.Printf("Versión: %s\n", version.Version)
		os.Exit(0)
	}

	// Mostrar ayuda si se solicita
	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	// Usar valores cortos si no se proporcionaron los largos
	input := *inputPath
	if input == "" {
		input = *inputShort
	}

	output := *outputDir
	if *outputShort != "" {
		output = *outputShort
	}

	numWorkers := *workers
	if numWorkers == 0 && *workersShort > 0 {
		numWorkers = *workersShort
	}

	// El descarte antes del primer encabezado se aplica a los DOCX
	skip := *skipBeforeHeading

	// Validar entrada
	if input == "" {
		// Si no se proporcionó input, buscar en argumentos posicionales
		if flag.NArg() > 0 {
			input = flag.Arg(0)
		} else {
			errorMsg.Println("❌ Error: Debe especificar un archivo o carpeta de entrada")
			fmt.Println()
			printUsage()
			os.Exit(1)
		}
	}

	// Verificar que la entrada existe
	inputInfo, err := os.Stat(input)
	if os.IsNotExist(err) {
		errorMsg.Printf("❌ Error: La ruta '%s' no existe\n", input)
		os.Exit(1)
	}

	// Determinar archivos a procesar
	var filesToProcess []string

	// Usar la fábrica para detectar tipos soportados
	factory := converter.NewProcessorFactory()

	if inputInfo.IsDir() {
		fmt.Printf("%s %s %s\n", info.wrap("📁"), info.wrap("Escaneando carpeta:"), pathStyle.wrap(input))

		entries, err := os.ReadDir(input)
		if err != nil {
			errorMsg.Printf("❌ Error escaneando carpeta: %v\n", err)
			os.Exit(1)
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			p := filepath.Join(input, e.Name())
			if factory.IsSupportedFile(p) {
				filesToProcess = append(filesToProcess, p)
			}
		}

		if len(filesToProcess) == 0 {
			warning.Println("⚠️  No se encontraron archivos soportados en la carpeta")
			info.Printf("  Extensiones soportadas: %v\n", factory.SupportedExtensions())
			os.Exit(0)
		}
	} else {
		if !factory.IsSupportedFile(input) {
			errorMsg.Printf("❌ Error: El archivo '%s' no es de un tipo soportado\n", input)
			info.Printf("  Extensiones soportadas: %v\n", factory.SupportedExtensions())
			os.Exit(1)
		}
		filesToProcess = []string{input}
	}

	// Hacer la ruta de salida absoluta si es relativa
	if !filepath.IsAbs(output) {
		if inputInfo.IsDir() {
			output = filepath.Join(input, output)
		} else {
			output = filepath.Join(filepath.Dir(input), output)
		}
	}

	// Añadir subcarpeta con timestamp para agrupar conversiones
	timestamp := time.Now().Format("20060102-150405")
	output = filepath.Join(output, timestamp)

	// Crear carpeta de salida si no existe
	if err := os.MkdirAll(output, 0755); err != nil {
		errorMsg.Printf("❌ Error creando carpeta de salida: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s %s %s\n", info.wrap("📂"), info.wrap("Carpeta de salida:"), pathStyle.wrap(output))
	fmt.Printf("%s %s %s\n", info.wrap("📄"), info.wrap("Archivos a procesar:"), highlight.wrap(fmt.Sprint(len(filesToProcess))))

	// Determinar número de workers
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	fmt.Printf("%s %s %s\n", info.wrap("⚡"), info.wrap("Workers paralelos:"), highlight.wrap(fmt.Sprint(numWorkers)))
	fmt.Println()

	// Iniciar conversión con concurrencia
	startTime := time.Now()
	results := processFilesWithConcurrency(filesToProcess, output, numWorkers, skip)

	// Mostrar resultados
	printResults(results, startTime)
}

// processFilesWithConcurrency procesa archivos usando workers concurrentes
func processFilesWithConcurrency(files []string, outputDir string, numWorkers int, skipBeforeHeading bool) []ConversionResult {
	var (
		wg           sync.WaitGroup
		results      []ConversionResult
		resultsMutex sync.Mutex
		processed    int64
		totalFiles   = int64(len(files))
	)

	// Canal para distribuir trabajo
	jobs := make(chan string, len(files))

	// Iniciar workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			conv := converter.NewConverter()
			conv.SetSkipBeforeHeading(skipBeforeHeading)

			for inputFile := range jobs {
				startTime := time.Now()
				outputPath := converter.GetOutputPath(inputFile, outputDir)

				err := conv.ConvertFile(inputFile, outputPath)
				duration := time.Since(startTime).Seconds()

				// Calcular integridad si la conversión fue exitosa
				integrityPercent := 0.0
				if err == nil {
					integrityPercent = conv.CheckIntegrity(inputFile, outputPath)
				}

				result := ConversionResult{
					SourceFile:       inputFile,
					OutputFile:       outputPath,
					Success:          err == nil,
					Error:            err,
					Duration:         duration,
					IntegrityPercent: integrityPercent,
				}

				resultsMutex.Lock()
				results = append(results, result)
				resultsMutex.Unlock()

				// Actualizar progreso
				current := atomic.AddInt64(&processed, 1)
				printProgress(current, totalFiles, inputFile, err == nil, integrityPercent)
			}
		}(w)
	}

	// Enviar trabajos
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	// Esperar a que terminen todos los workers
	wg.Wait()

	return results
}

// printProgress imprime el progreso de conversión
func printProgress(current, total int64, filename string, converted bool, integrity float64) {
	baseName := filepath.Base(filename)
	if len(baseName) > 40 {
		baseName = baseName[:37] + "..."
	}

	if converted {
		integritySymbol, integrityColor := integrityPresentation(integrity)
		fmt.Printf("\r%s %s %s %s %s %s\n",
			success.wrap("✅"),
			muted.wrap(fmt.Sprintf("[%d/%d]", current, total)),
			integrityColor.wrap(integritySymbol),
			success.wrap("Convertido:"),
			pathStyle.wrap(fmt.Sprintf("%-40s", baseName)),
			integrityColor.wrap(fmt.Sprintf("%.1f%%", integrity)))
	} else {
		fmt.Printf("\r%s %s %s %s\n",
			errorMsg.wrap("❌"),
			muted.wrap(fmt.Sprintf("[%d/%d]", current, total)),
			errorMsg.wrap("Error:"),
			pathStyle.wrap(fmt.Sprintf("%-40s", baseName)))
	}
}

// getIntegritySymbol retorna un símbolo basado en el porcentaje de integridad
func getIntegritySymbol(percent float64) string {
	symbol, _ := integrityPresentation(percent)
	return symbol
}

func integrityPresentation(percent float64) (string, style) {
	switch {
	case percent >= 95:
		return "🟢", success
	case percent >= 80:
		return "🟡", warning
	case percent >= 50:
		return "🟠", orange
	default:
		return "❌", errorMsg
	}
}

// printResults muestra el resumen de resultados
func printResults(results []ConversionResult, startTime time.Time) {
	totalDuration := time.Since(startTime)
	var successCount, errorCount int
	var totalConversionTime float64
	var totalIntegrity float64
	var minIntegrity float64 = 100
	var maxIntegrity float64 = 0

	fmt.Println()
	muted.Println("═══════════════════════════════════════════════════════════════")
	highlight.Println("                      📊 RESUMEN DE CONVERSIÓN")
	muted.Println("═══════════════════════════════════════════════════════════════")

	for _, result := range results {
		if result.Success {
			successCount++
			totalConversionTime += result.Duration
			totalIntegrity += result.IntegrityPercent
			if result.IntegrityPercent < minIntegrity {
				minIntegrity = result.IntegrityPercent
			}
			if result.IntegrityPercent > maxIntegrity {
				maxIntegrity = result.IntegrityPercent
			}
		} else {
			errorCount++
			fmt.Printf("  %s %s: %s\n",
				errorMsg.wrap("❌"),
				pathStyle.wrap(filepath.Base(result.SourceFile)),
				errorMsg.wrap(fmt.Sprint(result.Error)))
		}
	}

	fmt.Println()
	success.Printf("  ✅ Archivos convertidos exitosamente: %d\n", successCount)
	if errorCount > 0 {
		errorMsg.Printf("  ❌ Archivos con errores: %d\n", errorCount)
	}
	fmt.Println()

	// Mostrar estadísticas de integridad
	if successCount > 0 {
		avgIntegrity := totalIntegrity / float64(successCount)
		highlight.Println("  📋 INTEGRIDAD DE CONTENIDO:")
		_, avgColor := integrityPresentation(avgIntegrity)
		_, minColor := integrityPresentation(minIntegrity)
		_, maxColor := integrityPresentation(maxIntegrity)
		fmt.Printf("    %s %s\n", info.wrap("Promedio:"), avgColor.wrap(fmt.Sprintf("%.2f%%", avgIntegrity)))
		fmt.Printf("    %s %s\n", info.wrap("Mínima:"), minColor.wrap(fmt.Sprintf("%.2f%%", minIntegrity)))
		fmt.Printf("    %s %s\n", info.wrap("Máxima:"), maxColor.wrap(fmt.Sprintf("%.2f%%", maxIntegrity)))
		fmt.Println()
	}

	fmt.Printf("  %s %s\n", info.wrap("⏱️  Tiempo total de ejecución:"), highlight.wrap(formatDuration(totalDuration)))
	if successCount > 0 {
		avgTime := totalConversionTime / float64(successCount)
		fmt.Printf("  %s %s\n",
			info.wrap("📈 Tiempo promedio por archivo:"),
			highlight.wrap(fmt.Sprintf("%.3fs", avgTime)))
		fmt.Printf("  %s %s\n",
			info.wrap("🚀 Velocidad:"),
			highlight.wrap(fmt.Sprintf("%.2f archivos/segundo", float64(successCount)/totalDuration.Seconds())))
	}

	fmt.Println()

	// Mostrar métricas de rendimiento
	if successCount > 1 {
		speedup := totalConversionTime / totalDuration.Seconds()
		highlight.Printf("  ⚡ Speedup por concurrencia: %.2fx más rápido\n", speedup)
	}

	muted.Println("═══════════════════════════════════════════════════════════════")
}
