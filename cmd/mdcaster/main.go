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
)

const (
	version = "1.0.0"
	banner  = `
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
		info.Printf("Versión: %s\n", version)
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
		info.Printf("📁 Escaneando carpeta: %s\n", input)

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

	info.Printf("📂 Carpeta de salida: %s\n", output)
	info.Printf("📄 Archivos a procesar: %d\n", len(filesToProcess))

	// Determinar número de workers
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	info.Printf("⚡ Workers paralelos: %d\n", numWorkers)
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
func printProgress(current, total int64, filename string, success bool, integrity float64) {
	baseName := filepath.Base(filename)
	if len(baseName) > 40 {
		baseName = baseName[:37] + "..."
	}

	if success {
		integritySymbol := getIntegritySymbol(integrity)
		fmt.Printf("\r✅ [%d/%d] %s Convertido: %-40s %.1f%%\n", current, total, integritySymbol, baseName, integrity)
	} else {
		fmt.Printf("\r❌ [%d/%d] Error: %-40s\n", current, total, baseName)
	}
}

// getIntegritySymbol retorna un símbolo basado en el porcentaje de integridad
func getIntegritySymbol(percent float64) string {
	switch {
	case percent >= 95:
		return "🟢"
	case percent >= 80:
		return "🟡"
	case percent >= 50:
		return "🟠"
	default:
		return "🔴"
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
	fmt.Println("═══════════════════════════════════════════════════════════════")
	highlight.Println("                      📊 RESUMEN DE CONVERSIÓN")
	fmt.Println("═══════════════════════════════════════════════════════════════")

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
			errorMsg.Printf("  ❌ %s: %v\n", filepath.Base(result.SourceFile), result.Error)
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
		fmt.Println("  📋 INTEGRIDAD DE CONTENIDO:")
		info.Printf("    Promedio: %.2f%%\n", avgIntegrity)
		info.Printf("    Mínima: %.2f%%\n", minIntegrity)
		info.Printf("    Máxima: %.2f%%\n", maxIntegrity)
		fmt.Println()
	}

	info.Printf("  ⏱️  Tiempo total de ejecución: %s\n", formatDuration(totalDuration))
	if successCount > 0 {
		avgTime := totalConversionTime / float64(successCount)
		info.Printf("  📈 Tiempo promedio por archivo: %.3fs\n", avgTime)
		info.Printf("  🚀 Velocidad: %.2f archivos/segundo\n", float64(successCount)/totalDuration.Seconds())
	}

	fmt.Println()

	// Mostrar métricas de rendimiento
	if successCount > 1 {
		speedup := totalConversionTime / totalDuration.Seconds()
		highlight.Printf("  ⚡ Speedup por concurrencia: %.2fx más rápido\n", speedup)
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
}
