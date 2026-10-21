package converter

import (
	"strconv"
	"strings"
)

// listLabel es la etiqueta resuelta de un elemento de lista.
type listLabel struct {
	// text es la etiqueta completa: "1.", "a)", "2.1.".
	text string
	// counter es el valor numérico del nivel.
	counter int
	// levelFmt es el formato del nivel: decimal, lowerLetter, …
	levelFmt string
	// composite indica que la etiqueta combina varios niveles.
	composite bool
}

// resolveLabel calcula la etiqueta de un elemento de lista, actualizando los
// contadores según las reglas de Word.
func (n *docxNumbering) resolveLabel(numID string, ilvl int, state *numState) (listLabel, bool) {
	lvl := n.level(numID, ilvl)
	if lvl == nil {
		return listLabel{}, false
	}

	counter := state.take(numID, ilvl, lvl.start)

	// Al avanzar un nivel se reinician los más profundos, salvo que
	// w:lvlRestart diga lo contrario.
	n.resetDeeperLevels(numID, ilvl, state)

	label := listLabel{
		counter:   counter,
		levelFmt:  lvl.fmt,
		composite: isCompositeTemplate(lvl.text),
	}

	text := lvl.text
	for k := 1; k <= 9; k++ {
		placeholder := "%" + strconv.Itoa(k)
		if !strings.Contains(text, placeholder) {
			continue
		}

		ref := n.level(numID, k-1)
		refFmt := "decimal"
		refStart := 1
		if ref != nil {
			refFmt = ref.fmt
			refStart = ref.start
			if ref.isLgl {
				refFmt = "decimal"
			}
		}

		value := state.peek(numID, k-1, refStart)
		text = strings.ReplaceAll(text, placeholder, formatCounter(value, refFmt))
	}

	label.text = text

	return label, true
}

// isCompositeTemplate indica si la plantilla de etiqueta combina varios
// niveles: "%1.%2." no es representable como marcador de Markdown.
func isCompositeTemplate(text string) bool {
	count := 0
	for k := 1; k <= 9; k++ {
		if strings.Contains(text, "%"+strconv.Itoa(k)) {
			count++
		}
	}
	return count > 1
}

// formatCounter formatea un valor de contador según el formato del nivel.
func formatCounter(value int, format string) string {
	switch format {
	case "lowerLetter":
		return alphaCounter(value)
	case "upperLetter":
		return strings.ToUpper(alphaCounter(value))
	case "lowerRoman":
		return strings.ToLower(romanCounter(value))
	case "upperRoman":
		return strings.ToUpper(romanCounter(value))
	case "decimalZero":
		return fmtDecimalZero(value)
	default:
		return strconv.Itoa(value)
	}
}

// fmtDecimalZero formatea con dos dígitos, como hace Word con decimalZero.
func fmtDecimalZero(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

// alphaCounter convierte un número en letras: 1 → a, 26 → z, 27 → aa.
func alphaCounter(value int) string {
	if value <= 0 {
		return strconv.Itoa(value)
	}

	var sb strings.Builder
	for value > 0 {
		value--
		sb.WriteByte(byte('a' + value%26))
		value /= 26
	}

	// Se ha escrito de la unidad a la más alta: hay que invertirlo.
	runes := []rune(sb.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

// romanCounter convierte un número en numeración romana.
func romanCounter(value int) string {
	if value <= 0 || value >= 4000 {
		return strconv.Itoa(value)
	}

	values := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	symbols := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}

	var sb strings.Builder
	for i, v := range values {
		for value >= v {
			sb.WriteString(symbols[i])
			value -= v
		}
	}

	return sb.String()
}
