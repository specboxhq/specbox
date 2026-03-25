package domain

import (
	"strings"
	"testing"
	"time"
)

func newTestDoc(path, content string) *Document {
	now := time.Now()
	return &Document{Path: path, Content: content, CreatedAt: now, UpdatedAt: now}
}

func TestAlignTableRecalculatesWidths(t *testing.T) {
	// Table where later rows have wider content than the header
	input := "| Name | Desc |\n| --- | --- |\n| x | Short |\n| longername | A much longer description |"
	doc := newTestDoc("test.md", input)
	if err := doc.Format(0, 0); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(doc.Content, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	// All rows should have the same total width
	headerWidth := len(lines[0])
	for i, line := range lines {
		if len(line) != headerWidth {
			t.Errorf("line %d width %d != header width %d\n  header: %q\n  line:   %q", i+1, len(line), headerWidth, lines[0], line)
		}
	}

	// The "Name" column cell should be padded with spaces to fit "longername"
	// parseTableRow trims whitespace, so check the raw line instead
	if !strings.Contains(lines[0], "Name       ") {
		t.Errorf("header not padded to fit 'longername': %q", lines[0])
	}
}

func TestAlignTableSeparatorMatchesColumnWidths(t *testing.T) {
	input := "| Short | X |\n| --- | --- |\n| A very long cell value | Another long cell |"
	doc := newTestDoc("test.md", input)
	if err := doc.Format(0, 0); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(doc.Content, "\n")
	// Separator dashes should match the widened column widths
	sepCols := parseTableRow(lines[1])
	dataCols := parseTableRow(lines[2])
	for i := range sepCols {
		sepWidth := len(strings.TrimSpace(sepCols[i]))
		dataWidth := displayWidth(strings.TrimSpace(dataCols[i]))
		if sepWidth != dataWidth {
			t.Errorf("col %d: separator width %d != data width %d", i, sepWidth, dataWidth)
		}
	}
}

func TestInsertTableRowRealigns(t *testing.T) {
	// Start with a narrow aligned table
	input := "| Name | Desc  |\n| ---- | ----- |\n| x    | Short |"
	doc := newTestDoc("test.md", input)

	// Insert a row with wider content — should realign the whole table
	if err := doc.InsertTableRow(4, []string{"longername", "A much longer description"}); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(doc.Content, "\n")
	// All rows should have the same width (realigned automatically)
	w := len(lines[0])
	for i, line := range lines {
		if len(line) != w {
			t.Errorf("line %d width %d != header width %d\n  header: %q\n  line:   %q", i+1, len(line), w, lines[0], line)
		}
	}

	// Header should be padded to fit "longername"
	if !strings.Contains(lines[0], "Name       ") {
		t.Errorf("header not padded: %q", lines[0])
	}
}

func TestUpdateTableRowRealigns(t *testing.T) {
	input := "| Name | Desc  |\n| ---- | ----- |\n| x    | Short |"
	doc := newTestDoc("test.md", input)

	// Update row 3 with wider content
	if err := doc.UpdateTableRow(3, []string{"widername", "A longer description value"}); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(doc.Content, "\n")
	w := len(lines[0])
	for i, line := range lines {
		if len(line) != w {
			t.Errorf("line %d width %d != header width %d\n  header: %q\n  line:   %q", i+1, len(line), w, lines[0], line)
		}
	}
}

func TestFormatTableInLargerDocument(t *testing.T) {
	input := "# Heading\n\nSome text.\n\n| Col | Description |\n| --- | --- |\n| tiny | A much longer value here |\n\nMore text."
	doc := newTestDoc("test.md", input)
	if err := doc.Format(0, 0); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(doc.Content, "\n")
	// Find the table rows
	var tableLines []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			tableLines = append(tableLines, line)
		}
	}
	if len(tableLines) != 3 {
		t.Fatalf("expected 3 table lines, got %d", len(tableLines))
	}

	// All table rows should be same width
	w := len(tableLines[0])
	for i, line := range tableLines {
		if len(line) != w {
			t.Errorf("table line %d width %d != %d: %q", i, len(line), w, line)
		}
	}
}
