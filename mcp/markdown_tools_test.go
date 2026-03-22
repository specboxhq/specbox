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
