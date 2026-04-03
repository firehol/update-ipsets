package markdown

import (
	"fmt"
	"strings"
)

type TableColumn struct {
	Header string
	Right  bool
}

type Table struct {
	Columns []TableColumn
	Rows    [][]string
}

func RenderTable(columns []TableColumn, rows [][]string) string {
	if len(columns) == 0 {
		return ""
	}

	widths := colWidths(columns, rows)

	var b strings.Builder
	b.WriteString(renderHeader(columns, widths))
	b.WriteString(renderSeparator(columns, widths))
	for _, row := range rows {
		b.WriteString(renderRow(row, columns, widths))
	}
	return b.String()
}

func colWidths(columns []TableColumn, rows [][]string) []int {
	w := make([]int, len(columns))
	for i, c := range columns {
		w[i] = max(len(c.Header), 3)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(w) && len(cell) > w[i] {
				w[i] = len(cell)
			}
		}
	}
	return w
}

func renderHeader(columns []TableColumn, widths []int) string {
	cells := make([]string, len(columns))
	for i, c := range columns {
		cells[i] = pad(c.Header, widths[i], false)
	}
	return "| " + strings.Join(cells, " | ") + " |\n"
}

func renderSeparator(columns []TableColumn, widths []int) string {
	cells := make([]string, len(columns))
	for i, c := range columns {
		dashes := strings.Repeat("-", max(widths[i], 3))
		if c.Right {
			cells[i] = dashes[:len(dashes)-1] + ":"
		} else {
			cells[i] = ":" + dashes[:len(dashes)-1]
		}
	}
	return "|" + strings.Join(cells, "|") + "|\n"
}

func renderRow(row []string, columns []TableColumn, widths []int) string {
	cells := make([]string, len(widths))
	for i := range widths {
		cell := ""
		if i < len(row) {
			cell = row[i]
		}
		right := false
		if i < len(columns) {
			right = columns[i].Right
		}
		cells[i] = pad(cell, widths[i], right)
	}
	return "| " + strings.Join(cells, " | ") + " |\n"
}

func pad(s string, width int, right bool) string {
	if len(s) >= width {
		return s
	}
	gap := width - len(s)
	if right {
		return fmt.Sprintf("%*s", width, s)
	}
	return s + strings.Repeat(" ", gap)
}
