package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCheckCheckboxViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "- [ ] todo1\n- [ ] todo2\n- [x] done")

	callTool(t, s, "set_checkbox", map[string]any{
		"path":     "proj/test.md",
		"line_num": 1,
		"checked":  true,
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	lines := strings.Split(doc["content"].(string), "\n")
	if lines[0] != "- [x] todo1" {
		t.Errorf("expected '- [x] todo1', got %q", lines[0])
	}
}

func TestCheckCheckboxUncheck(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "- [x] done")

	callTool(t, s, "set_checkbox", map[string]any{
		"path":     "proj/test.md",
		"line_num": 1,
		"checked":  false,
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	if doc["content"] != "- [ ] done" {
		t.Errorf("got %v", doc["content"])
	}
}

func TestCheckCheckboxNotACheckbox(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "just text")

	result := callTool(t, s, "set_checkbox", map[string]any{
		"path":     "proj/test.md",
		"line_num": 1,
		"checked":  true,
	})
	if result != "line is not a markdown checkbox" {
		t.Errorf("expected checkbox error, got %q", result)
	}
}

func TestGetSectionViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "# Title\n\nIntro\n\n## Section A\n\nContent A\n\n## Section B\n\nContent B")

	result := callTool(t, s, "get_section", map[string]any{
		"path":    "proj/test.md",
		"heading": "Section A",
	})
	var data map[string]any
	json.Unmarshal([]byte(result), &data)
	if data["start_line"] != float64(5) {
		t.Errorf("expected start_line 5, got %v", data["start_line"])
	}
	if !strings.Contains(data["content"].(string), "Content A") {
		t.Errorf("expected Content A in section")
	}
}

func TestGetSectionNotFound(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "# Title")

	result := callTool(t, s, "get_section", map[string]any{
		"path":    "proj/test.md",
		"heading": "Missing",
	})
	if result != "heading not found in document" {
		t.Errorf("expected heading error, got %q", result)
	}
}

func TestInsertTextByHeading(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "# Title\n\nOld content")

	callTool(t, s, "insert_text", map[string]any{
		"path":    "proj/test.md",
		"heading": "Title",
		"content": "New line",
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	lines := strings.Split(doc["content"].(string), "\n")
	if lines[1] != "New line" {
		t.Errorf("expected 'New line' at line 2, got %q", lines[1])
	}
}

func TestMoveSectionViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "## A\n\nContent A\n\n## B\n\nContent B\n\n## C\n\nContent C")

	callTool(t, s, "move_section", map[string]any{
		"path":           "proj/test.md",
		"heading":        "A",
		"target_heading": "C",
	})

	result := callTool(t, s, "get_table_of_contents", map[string]any{"path": "proj/test.md"})
	var toc []map[string]any
	json.Unmarshal([]byte(result), &toc)
	if len(toc) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(toc))
	}
	if toc[0]["heading"] != "B" || toc[1]["heading"] != "C" || toc[2]["heading"] != "A" {
		t.Errorf("expected B, C, A order, got %s, %s, %s", toc[0]["heading"], toc[1]["heading"], toc[2]["heading"])
	}
}

func TestDeleteSectionViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "## A\n\nContent A\n\n## B\n\nContent B")

	callTool(t, s, "delete_section", map[string]any{
		"path":    "proj/test.md",
		"heading": "A",
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	content := doc["content"].(string)
	if strings.Contains(content, "Content A") {
		t.Error("Content A should be removed")
	}
	if !strings.Contains(content, "Content B") {
		t.Error("Content B should remain")
	}
}

func TestInsertTableRowViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "| Name | Age |\n|---|---|\n| Alice | 30 |")

	callTool(t, s, "edit_table_row", map[string]any{
		"path":     "proj/test.md",
		"line_num": 4,
		"values":   []any{"Bob", "25"},
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	lines := strings.Split(doc["content"].(string), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[3], "Bob") || !strings.Contains(lines[3], "25") {
		t.Errorf("expected row with Bob/25, got %q", lines[3])
	}
}

func TestInsertTableRowNotATable(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "just text\nno table")

	result := callTool(t, s, "edit_table_row", map[string]any{
		"path":     "proj/test.md",
		"line_num": 2,
		"values":   []any{"a", "b"},
	})
	if result != "line is not within a markdown table" {
		t.Errorf("expected table error, got %q", result)
	}
}

func TestBulkSetCheckboxesCheckAll(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "- [ ] one\n- [ ] two\n- [x] three\n- [ ] four")

	result := callTool(t, s, "bulk_set_checkboxes", map[string]any{
		"path":    "proj/test.md",
		"checked": true,
	})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)
	if resp["toggled"] != float64(3) {
		t.Errorf("expected 3 toggled, got %v", resp["toggled"])
	}

	// Verify all checked
	doc := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var d map[string]any
	json.Unmarshal([]byte(doc), &d)
	for i, line := range strings.Split(d["content"].(string), "\n") {
		if !strings.HasPrefix(line, "- [x]") {
			t.Errorf("line %d not checked: %q", i+1, line)
		}
	}
}

func TestBulkSetCheckboxesUncheckAll(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "- [x] one\n- [x] two\n- [ ] three")

	result := callTool(t, s, "bulk_set_checkboxes", map[string]any{
		"path":    "proj/test.md",
		"checked": false,
	})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)
	if resp["toggled"] != float64(2) {
		t.Errorf("expected 2 toggled, got %v", resp["toggled"])
	}
}

func TestBulkSetCheckboxesWithFilter(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "- [x] done\n- [ ] open\n- [x] also done")

	// Only uncheck items that are currently checked
	result := callTool(t, s, "bulk_set_checkboxes", map[string]any{
		"path":    "proj/test.md",
		"checked": false,
		"filter":  "checked",
	})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)
	if resp["toggled"] != float64(2) {
		t.Errorf("expected 2 toggled, got %v", resp["toggled"])
	}

	// Verify: all should be unchecked now
	doc := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var d map[string]any
	json.Unmarshal([]byte(doc), &d)
	for i, line := range strings.Split(d["content"].(string), "\n") {
		if !strings.HasPrefix(line, "- [ ]") {
			t.Errorf("line %d not unchecked: %q", i+1, line)
		}
	}
}

func TestBulkSetCheckboxesWithHeading(t *testing.T) {
	s, store := setupTestServer(t)
	content := "# Project\n\n## Done\n\n- [ ] a\n- [ ] b\n\n## Todo\n\n- [ ] c\n- [ ] d"
	seedDoc(t, store, "proj/test.md", content)

	// Only check items under "Done" heading
	result := callTool(t, s, "bulk_set_checkboxes", map[string]any{
		"path":    "proj/test.md",
		"checked": true,
		"heading": "Done",
	})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)
	if resp["toggled"] != float64(2) {
		t.Errorf("expected 2 toggled, got %v", resp["toggled"])
	}

	// Verify: "Done" items checked, "Todo" items still unchecked
	doc := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var d map[string]any
	json.Unmarshal([]byte(doc), &d)
	lines := strings.Split(d["content"].(string), "\n")
	if !strings.HasPrefix(lines[4], "- [x]") {
		t.Errorf("line 5 should be checked: %q", lines[4])
	}
	if !strings.HasPrefix(lines[5], "- [x]") {
		t.Errorf("line 6 should be checked: %q", lines[5])
	}
	if !strings.HasPrefix(lines[9], "- [ ]") {
		t.Errorf("line 10 should still be unchecked: %q", lines[9])
	}
}

func TestBulkSetCheckboxesLineRange(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "- [ ] one\n- [ ] two\n- [ ] three\n- [ ] four")

	result := callTool(t, s, "bulk_set_checkboxes", map[string]any{
		"path":       "proj/test.md",
		"checked":    true,
		"start_line": 2,
		"end_line":   3,
	})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)
	if resp["toggled"] != float64(2) {
		t.Errorf("expected 2 toggled, got %v", resp["toggled"])
	}

	doc := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var d map[string]any
	json.Unmarshal([]byte(doc), &d)
	lines := strings.Split(d["content"].(string), "\n")
	if !strings.HasPrefix(lines[0], "- [ ]") {
		t.Errorf("line 1 should be unchecked: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "- [x]") {
		t.Errorf("line 2 should be checked: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "- [x]") {
		t.Errorf("line 3 should be checked: %q", lines[2])
	}
	if !strings.HasPrefix(lines[3], "- [ ]") {
		t.Errorf("line 4 should be unchecked: %q", lines[3])
	}
}
