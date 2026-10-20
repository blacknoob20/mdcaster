package main

import (
	"fmt"
	"os"
	"strings"
)

// Códigos ANSI (SGR) para el color de la salida. Son los mismos que usaba la
// librería de color anterior, así que la salida visible no cambia.
const (
	sgrReset   = "\x1b[0m"
	sgrBold    = "1"
	sgrRed     = "31"
	sgrGreen   = "32"
	sgrYellow  = "33"
	sgrMagenta = "35"
	sgrCyan    = "36"
)

// colorEnabled indica si la salida lleva color. Se decide una sola vez al
// arrancar: sin terminal, con NO_COLOR o con TERM=dumb la salida es texto
// plano.
var colorEnabled = detectColor()

// detectColor decide si la salida admite color.
func detectColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	// En Windows hay que activar el modo VT de la consola antes de escribir
	// códigos ANSI.
	enableVirtualTerminal()

	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

// style es un estilo de salida: una secuencia ANSI y el texto que la lleva.
type style struct {
	codes string
}

// styleCodes construye una secuencia ANSI a partir de sus parámetros.
func styleCodes(params ...string) string {
	return "\x1b[" + strings.Join(params, ";") + "m"
}

// wrap aplica el estilo al texto. Sin color activo devuelve el texto tal cual.
func (s style) wrap(text string) string {
	if !colorEnabled || s.codes == "" {
		return text
	}
	return s.codes + text + sgrReset
}

// Print escribe sus argumentos con el estilo aplicado.
func (s style) Print(a ...interface{}) {
	fmt.Print(s.wrap(fmt.Sprint(a...)))
}

// Printf escribe el texto con el estilo aplicado.
func (s style) Printf(format string, a ...interface{}) {
	fmt.Print(s.wrap(fmt.Sprintf(format, a...)))
}

// Println escribe sus argumentos con el estilo aplicado y un salto de línea.
func (s style) Println(a ...interface{}) {
	fmt.Print(s.wrap(fmt.Sprintln(a...)))
}

// Colores para la salida
var (
	success   = style{styleCodes(sgrGreen, sgrBold)}
	info      = style{styleCodes(sgrCyan)}
	warning   = style{styleCodes(sgrYellow)}
	errorMsg  = style{styleCodes(sgrRed, sgrBold)}
	highlight = style{styleCodes(sgrMagenta, sgrBold)}
)
