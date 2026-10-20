package main

// ConversionResult representa el resultado de una conversión.
//
// Vive en el comando y no en la librería: es la CLI la que mide la duración y
// la integridad de cada fichero, y las rellena aquí.
type ConversionResult struct {
	SourceFile       string
	OutputFile       string
	Success          bool
	Error            error
	Duration         float64
	IntegrityPercent float64 // Porcentaje de contenido que se encontró en el markdown
}
