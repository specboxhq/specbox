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
