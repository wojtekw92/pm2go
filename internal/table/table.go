package table

import (
	"fmt"
	"strings"
)

// Table represents a table with dynamic column sizing
type Table struct {
	headers    []string
	rows       [][]string
	colWidths  []int
	minWidth   int
	maxWidth   int
	padding    int
}

// NewTable creates a new table with specified headers
func NewTable(headers ...string) *Table {
	t := &Table{
		headers:   headers,
		colWidths: make([]int, len(headers)),
		minWidth:  10,
		maxWidth:  100,
		padding:   1,
	}
	
	// Initialize column widths with header lengths
	for i, header := range headers {
		t.colWidths[i] = len(header)
	}
	
	return t
}

// SetMinWidth sets the minimum column width
func (t *Table) SetMinWidth(width int) *Table {
	t.minWidth = width
	return t
}

// SetMaxWidth sets the maximum column width
func (t *Table) SetMaxWidth(width int) *Table {
	t.maxWidth = width
	return t
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) *Table {
	// Pad row with empty strings if needed
	row := make([]string, len(t.headers))
	for i := range row {
		if i < len(cells) {
			row[i] = cells[i]
		} else {
			row[i] = ""
		}
	}
	
	t.rows = append(t.rows, row)
	
	// Update column widths
	t.updateColumnWidths(row)
	
	return t
}

// updateColumnWidths updates column widths based on content
func (t *Table) updateColumnWidths(row []string) {
	for i, cell := range row {
		if i < len(t.colWidths) {
			cellLen := len(cell)
			if cellLen > t.colWidths[i] {
				if cellLen > t.maxWidth {
					t.colWidths[i] = t.maxWidth
				} else {
					t.colWidths[i] = cellLen
				}
			}
		}
	}
	
	// Ensure minimum width
	for i := range t.colWidths {
		if t.colWidths[i] < t.minWidth {
			t.colWidths[i] = t.minWidth
		}
	}
}

// truncateCell truncates cell content if it exceeds column width
func (t *Table) truncateCell(content string, width int) string {
	if len(content) <= width {
		return content
	}
	if width <= 3 {
		return content[:width]
	}
	return content[:width-3] + "..."
}

// Render renders the table as a string
func (t *Table) Render() string {
	if len(t.headers) == 0 {
		return ""
	}
	
	var result strings.Builder
	
	// Top border
	result.WriteString("┌")
	for i, width := range t.colWidths {
		result.WriteString(strings.Repeat("─", width+2*t.padding))
		if i < len(t.colWidths)-1 {
			result.WriteString("┬")
		}
	}
	result.WriteString("┐\n")
	
	// Header row
	result.WriteString("│")
	for i, header := range t.headers {
		truncated := t.truncateCell(header, t.colWidths[i])
		padding := t.colWidths[i] - len(truncated)
		result.WriteString(strings.Repeat(" ", t.padding))
		result.WriteString(truncated)
		result.WriteString(strings.Repeat(" ", padding+t.padding))
		result.WriteString("│")
	}
	result.WriteString("\n")
	
	// Header separator
	if len(t.rows) > 0 {
		result.WriteString("├")
		for i, width := range t.colWidths {
			result.WriteString(strings.Repeat("─", width+2*t.padding))
			if i < len(t.colWidths)-1 {
				result.WriteString("┼")
			}
		}
		result.WriteString("┤\n")
	}
	
	// Data rows
	for _, row := range t.rows {
		result.WriteString("│")
		for i, cell := range row {
			if i < len(t.colWidths) {
				truncated := t.truncateCell(cell, t.colWidths[i])
				padding := t.colWidths[i] - len(truncated)
				result.WriteString(strings.Repeat(" ", t.padding))
				result.WriteString(truncated)
				result.WriteString(strings.Repeat(" ", padding+t.padding))
				result.WriteString("│")
			}
		}
		result.WriteString("\n")
	}
	
	// Bottom border
	result.WriteString("└")
	for i, width := range t.colWidths {
		result.WriteString(strings.Repeat("─", width+2*t.padding))
		if i < len(t.colWidths)-1 {
			result.WriteString("┴")
		}
	}
	result.WriteString("┘")
	
	return result.String()
}

// Print prints the table to stdout
func (t *Table) Print() {
	fmt.Println(t.Render())
}

// KeyValueTable creates a simple two-column key-value table
type KeyValueTable struct {
	*Table
}

// NewKeyValueTable creates a new key-value table
func NewKeyValueTable() *KeyValueTable {
	return &KeyValueTable{
		Table: NewTable("Key", "Value"),
	}
}

// AddKeyValue adds a key-value pair to the table
func (kvt *KeyValueTable) AddKeyValue(key, value string) *KeyValueTable {
	kvt.AddRow(key, value)
	return kvt
}

// SetKeyWidth sets the width for the key column
func (kvt *KeyValueTable) SetKeyWidth(width int) *KeyValueTable {
	if len(kvt.colWidths) > 0 {
		kvt.colWidths[0] = width
	}
	return kvt
}

// SetValueWidth sets the width for the value column  
func (kvt *KeyValueTable) SetValueWidth(width int) *KeyValueTable {
	if len(kvt.colWidths) > 1 {
		kvt.colWidths[1] = width
	}
	return kvt
}