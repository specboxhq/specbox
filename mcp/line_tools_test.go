package mcp_test

import (
	"encoding/json"
	"testing"
)

func TestInsertTextByLineNum(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "line1\nline3")

	callTool(t, s, "insert_text", map[string]any{
		"path":     "proj/test.md",
		"line_num": 2,
		"position": "before",
		"content":  "line2",
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	if doc["content"] != "line1\nline2\nline3" {
		t.Errorf("got %v", doc["content"])
	}
}

func TestMoveLinesViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "a\nb\nc\nd\ne")

	callTool(t, s, "move_lines", map[string]any{
		"path":        "proj/test.md",
		"start_line":  2,
		"end_line":    3,
		"target_line": 5,
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	if doc["content"] != "a\nd\nb\nc\ne" {
		t.Errorf("got %v", doc["content"])
	}
}

func TestDeleteLinesViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "a\nb\nc\nd")

	callTool(t, s, "delete_lines", map[string]any{
		"path":       "proj/test.md",
		"start_line": 2,
		"end_line":   3,
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	if doc["content"] != "a\nd" {
		t.Errorf("got %v", doc["content"])
	}
}

func TestCopyLinesViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "a\nb\nc")

	callTool(t, s, "copy_lines", map[string]any{
		"path":        "proj/test.md",
		"start_line":  1,
		"end_line":    2,
		"target_line": 4,
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	if doc["content"] != "a\nb\nc\na\nb" {
		t.Errorf("got %v", doc["content"])
	}
}

func TestIndentLinesViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "a\nb\nc")

	callTool(t, s, "indent_lines", map[string]any{
		"path":       "proj/test.md",
		"start_line": 1,
		"end_line":   3,
		"levels":     1,
		"prefix":     "  ",
	})

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(result), &doc)
	if doc["content"] != "  a\n  b\n  c" {
		t.Errorf("got %v", doc["content"])
	}
}

func TestLineOutOfRangeViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "a\nb")

	result := callTool(t, s, "delete_lines", map[string]any{
		"path":       "proj/test.md",
		"start_line": 1,
		"end_line":   5,
	})
	if result != "line number out of range" {
		t.Errorf("expected line out of range error, got %q", result)
	}
}

func TestApplyEditsViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "line1\nline2\nline3\nline4\nline5")

	result := callTool(t, s, "apply_edits", map[string]any{
		"path": "proj/test.md",
		"edits": []any{
			map[string]any{"op": "delete", "start_line": 2, "end_line": 2},
			map[string]any{"op": "insert", "line": 4, "content": "inserted"},
			map[string]any{"op": "replace_text", "line": 3, "old_text": "line3", "new_text": "LINE3"},
		},
	})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	if resp["edits_applied"] != float64(3) {
		t.Errorf("expected 3 edits applied, got %v", resp["edits_applied"])
	}

	// Verify final content — applied bottom-up:
	// 1. insert at 4 → line1\nline2\nline3\ninserted\nline4\nline5
	// 2. replace_text 3 → line1\nline2\nLINE3\ninserted\nline4\nline5
	// 3. delete 2 → line1\nLINE3\ninserted\nline4\nline5
	doc := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var d map[string]any
	json.Unmarshal([]byte(doc), &d)
	if d["content"] != "line1\nLINE3\ninserted\nline4\nline5" {
		t.Errorf("got %q", d["content"])
	}
}

func TestApplyEditsValidationError(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "a\nb")

	result := callTool(t, s, "apply_edits", map[string]any{
		"path": "proj/test.md",
		"edits": []any{
			map[string]any{"op": "delete", "start_line": 1, "end_line": 1},
			map[string]any{"op": "delete", "start_line": 5, "end_line": 5},
		},
	})
	// Should be an error (line 5 out of range)
	if !contains(result, "out of range") {
		t.Errorf("expected out of range error, got %q", result)
	}

	// Document should be unchanged
	doc := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var d map[string]any
	json.Unmarshal([]byte(doc), &d)
	if d["content"] != "a\nb" {
		t.Errorf("document should be unchanged, got %q", d["content"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
