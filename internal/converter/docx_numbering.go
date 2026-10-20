package converter

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// numberingLevel es la definición de un nivel de lista.
type numberingLevel struct {
	// fmt es el formato del número: decimal, lowerLetter, bullet, …
	fmt string
	// text es la plantilla de la etiqueta: "%1.", "%1.%2.", "•", …
	text string
	// start es el valor inicial del contador.
	start int
	// lvlRestart indica el nivel que reinicia este (-1 si no está
	// declarado, 0 si nunca se reinicia).
	lvlRestart int
	// isLgl obliga a formatar el nivel como decimal.
	isLgl bool
}

// docxNumbering guarda las definiciones de numeración de word/numbering.xml,
// indexadas por numId y nivel.
type docxNumbering struct {
	// levels: numId → ilvl → definición.
	levels map[string]map[int]*numberingLevel
	// order: numId → abstractNumId, para resolver los niveles heredados.
	order map[string]string
}

// valAttr es un elemento de OOXML cuyo valor está en el atributo w:val.
//
// En numbering.xml todos los valores (w:start, w:numFmt, w:lvlText,
// w:lvlRestart, w:isLgl, w:startOverride) van en el atributo w:val, nunca en
// el texto del elemento.
type valAttr struct {
	Val string `xml:"val,attr"`
}

// numberingXML es la estructura de word/numbering.xml.
type numberingXML struct {
	XMLName      xml.Name      `xml:"numbering"`
	AbstractNums []abstractNum `xml:"abstractNum"`
	Nums         []numDef      `xml:"num"`
}

// abstractNum es una definición de lista multinivel.
type abstractNum struct {
	ID     string     `xml:"abstractNumId,attr"`
	Levels []levelDef `xml:"lvl"`
}

// levelDef es un nivel dentro de un abstractNum.
type levelDef struct {
	ILvl       string   `xml:"ilvl,attr"`
	Start      *valAttr `xml:"start"`
	NumFmt     *valAttr `xml:"numFmt"`
	LvlText    *valAttr `xml:"lvlText"`
	LvlRestart *valAttr `xml:"lvlRestart"`
	IsLgl      *valAttr `xml:"isLgl"`
}

// numDef es una instancia de lista: relaciona numId con un abstractNum.
type numDef struct {
	NumID      string        `xml:"numId,attr"`
	AbstractID *valAttr      `xml:"abstractNumId"`
	Overrides  []lvlOverride `xml:"lvlOverride"`
}

// lvlOverride reinicia o redefine un nivel de una instancia de lista.
type lvlOverride struct {
	ILvl          string    `xml:"ilvl,attr"`
	StartOverride *valAttr  `xml:"startOverride"`
	Level         *levelDef `xml:"lvl"`
}

// parseDocxNumbering lee word/numbering.xml. Si el contenido está vacío o no
// se puede leer, devuelve una numeración vacía: los elementos de lista se
// degradarán a viñetas.
func parseDocxNumbering(content []byte) *docxNumbering {
	n := &docxNumbering{
		levels: make(map[string]map[int]*numberingLevel),
		order:  make(map[string]string),
	}

	if len(content) == 0 {
		return n
	}

	var doc numberingXML
	if err := xml.Unmarshal(content, &doc); err != nil {
		return n
	}

	abstracts := make(map[string]map[int]*numberingLevel, len(doc.AbstractNums))
	for _, abs := range doc.AbstractNums {
		levels := make(map[int]*numberingLevel, len(abs.Levels))
		for _, lvl := range abs.Levels {
			ilvl := atoiDefault(lvl.ILvl, 0)
			levels[ilvl] = buildLevel(lvl)
		}
		abstracts[abs.ID] = levels
	}

	for _, num := range doc.Nums {
		abstractID := ""
		if num.AbstractID != nil {
			abstractID = num.AbstractID.Val
		}
		n.order[num.NumID] = abstractID

		levels := make(map[int]*numberingLevel)
		base := abstracts[abstractID]
		for ilvl, lvl := range base {
			// Cada instancia de lista tiene su propia copia del nivel: un
			// startOverride no debe alterar las demás instancias.
			copied := *lvl
			levels[ilvl] = &copied
		}

		// Un lvlOverride puede redefinir el nivel o solo reiniciarlo.
		for _, ov := range num.Overrides {
			ilvl := atoiDefault(ov.ILvl, 0)
			lvl, ok := levels[ilvl]
			if !ok {
				lvl = &numberingLevel{fmt: "decimal", text: "%1.", start: 1, lvlRestart: -1}
				levels[ilvl] = lvl
			}
			if ov.StartOverride != nil {
				lvl.start = atoiDefault(ov.StartOverride.Val, lvl.start)
			}
			if ov.Level != nil {
				override := buildLevel(*ov.Level)
				override.start = lvl.start
				levels[ilvl] = override
			}
		}

		n.levels[num.NumID] = levels
	}

	return n
}

// buildLevel convierte la definición XML de un nivel.
func buildLevel(def levelDef) *numberingLevel {
	lvl := &numberingLevel{
		fmt:        "decimal",
		text:       "",
		start:      1,
		lvlRestart: -1,
	}

	if def.NumFmt != nil && def.NumFmt.Val != "" {
		lvl.fmt = def.NumFmt.Val
	}
	if def.LvlText != nil {
		lvl.text = def.LvlText.Val
	}
	if def.Start != nil {
		lvl.start = atoiDefault(def.Start.Val, 1)
	}
	if def.LvlRestart != nil {
		lvl.lvlRestart = atoiDefault(def.LvlRestart.Val, -1)
	}
	if def.IsLgl != nil {
		lvl.isLgl = attrIsOn(def.IsLgl.Val)
	}

	if lvl.text == "" {
		lvl.text = "%" + strconv.Itoa(atoiDefault(def.ILvl, 0)+1) + "."
	}

	return lvl
}

// attrIsOn interpreta el valor de un atributo de activación.
func attrIsOn(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "false", "off":
		return false
	}
	return true
}

// level devuelve la definición de un nivel de una instancia de lista.
func (n *docxNumbering) level(numID string, ilvl int) *numberingLevel {
	if n == nil {
		return nil
	}
	levels, ok := n.levels[numID]
	if !ok {
		return nil
	}
	return levels[ilvl]
}

// hasNumID indica si la instancia de lista existe.
func (n *docxNumbering) hasNumID(numID string) bool {
	if n == nil {
		return false
	}
	_, ok := n.levels[numID]
	return ok
}

// numCounter es el contador de un nivel de una lista.
type numCounter struct {
	// next es el próximo valor que se emitirá.
	next int
	// initialized indica si el contador ya se ha usado.
	initialized bool
}

// numState guarda los contadores de todas las listas del documento, indexados
// por (numId, nivel).
//
// Los contadores no se reinician al cambiar de numId: Word mantiene una
// numeración continua por instancia de lista. Solo se reinician los niveles
// profundos cuando Word lo haría, según w:lvlRestart.
type numState struct {
	counters map[string][]numCounter
}

// newNumState crea el estado de contadores.
func newNumState() *numState {
	return &numState{counters: make(map[string][]numCounter)}
}

// take devuelve el valor del contador del nivel y lo avanza.
func (s *numState) take(numID string, ilvl, start int) int {
	cs := s.counters[numID]
	for len(cs) <= ilvl {
		cs = append(cs, numCounter{next: start})
	}

	c := &cs[ilvl]
	if !c.initialized {
		c.next = start
		c.initialized = true
	}

	value := c.next
	c.next++
	s.counters[numID] = cs

	return value
}

// peek devuelve el valor vigente del contador del nivel sin avanzarlo: el
// último emitido, o el inicial si todavía no se ha usado.
func (s *numState) peek(numID string, ilvl, start int) int {
	cs := s.counters[numID]
	if ilvl >= len(cs) || !cs[ilvl].initialized {
		return start
	}
	return cs[ilvl].next - 1
}

// reset devuelve el contador del nivel a su valor inicial.
func (s *numState) reset(numID string, ilvl, start int) {
	cs := s.counters[numID]
	for len(cs) <= ilvl {
		cs = append(cs, numCounter{next: start})
	}
	cs[ilvl].next = start
	cs[ilvl].initialized = true
	s.counters[numID] = cs
}

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

// resetDeeperLevels reinicia los niveles más profundos que el indicado.
func (n *docxNumbering) resetDeeperLevels(numID string, ilvl int, state *numState) {
	levels, ok := n.levels[numID]
	if !ok {
		return
	}

	for deeper := ilvl + 1; deeper < len(levels)+9; deeper++ {
		lvl, exists := levels[deeper]
		if !exists {
			continue
		}

		switch {
		case lvl.lvlRestart < 0:
			// Sin declarar: se reinicia cuando cualquier nivel más
			// superficial avanza.
			state.reset(numID, deeper, lvl.start)
		case lvl.lvlRestart == 0:
			// Nunca se reinicia.
		default:
			// Se reinicia cuando avanza el nivel lvlRestart-1.
			if ilvl == lvl.lvlRestart-1 {
				state.reset(numID, deeper, lvl.start)
			}
		}
	}
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
