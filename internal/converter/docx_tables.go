package converter

import (
	"strings"
)

// writeTable escribe una tabla en Markdown.
func (r *docxRenderer) writeTable(tbl *docxTable) {
	if tbl == nil {
		return
	}

	rows := buildTableRows(tbl)
	if len(rows) == 0 {
		return
	}

	columns := 0
	for _, row := range rows {
		if len(row) > columns {
			columns = len(row)
		}
	}
	if columns == 0 {
		return
	}

	lines := make([]string, 0, len(rows)+1)
	separator := make([]string, columns)
	for i := range separator {
		separator[i] = "---"
	}

	for i, row := range rows {
		cells := make([]string, columns)
		copy(cells, row)

		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
		if i == 0 {
			lines = append(lines, "| "+strings.Join(separator, " | ")+" |")
		}
	}

	r.writeBlock(lines)
}

// buildTableRows convierte la tabla en filas de celdas de texto.
//
// Las celdas que continúan una unión vertical no llevan texto: el texto
// está en la celda que abre la unión. Las celdas que ocupan varias columnas
// (gridSpan) dejan vacías las columnas restantes.
func buildTableRows(tbl *docxTable) [][]string {
	rows := make([][]string, 0, len(tbl.rows))

	for _, row := range tbl.rows {
		cells := make([]string, 0, len(row.cells))
		for _, cell := range row.cells {
			cells = append(cells, cellContent(cell))
			for i := 1; i < cell.gridSpan; i++ {
				cells = append(cells, "")
			}
		}
		rows = append(rows, cells)
	}

	return rows
}

// cellContent devuelve el texto de una celda, listo para una tabla
// Markdown.
func cellContent(cell *docxCell) string {
	if cell.vMerge == vMergeContinue {
		return ""
	}

	text := strings.Join(cell.lines, "<br>")

	return escapeTableCell(text)
}

// escapeTableCell escapa los caracteres que rompen una tabla Markdown.
func escapeTableCell(text string) string {
	return strings.ReplaceAll(text, "|", "\\|")
}
