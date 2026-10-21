package docx

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
