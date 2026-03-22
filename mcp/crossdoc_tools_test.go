package mcp_test

import (
	"encoding/json"
	"testing"
)

func TestMoveLinesToDocumentViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/src.md", "a\nb\nc\nd")
	seedDoc(t, store, "proj/dest.md", "x\ny")

	result := callTool(t, s, "move_lines", map[string]any{
		"path":        "proj/src.md",
		"start_line":  2,
		"end_line":    3,
		"dest_path":   "proj/dest.md",
		"target_line": 2,
	})

	var data map[string]any
	json.Unmarshal([]byte(result), &data)
	src := data["src"].(map[string]any)
	dest := data["dest"].(map[string]any)
	if src["line_count"] != float64(2) {
		t.Errorf("src line_count: expected 2, got %v", src["line_count"])
	}
	if dest["line_count"] != float64(4) {
		t.Errorf("dest line_count: expected 4, got %v", dest["line_count"])
	}

	// Verify content
	srcResult := callTool(t, s, "get_document", map[string]any{"path": "proj/src.md"})
	var srcDoc map[string]any
	json.Unmarshal([]byte(srcResult), &srcDoc)
	if srcDoc["content"] != "a\nd" {
		t.Errorf("src content: got %v", srcDoc["content"])
	}

	destResult := callTool(t, s, "get_document", map[string]any{"path": "proj/dest.md"})
	var destDoc map[string]any
	json.Unmarshal([]byte(destResult), &destDoc)
	if destDoc["content"] != "x\nb\nc\ny" {
		t.Errorf("dest content: got %v", destDoc["content"])
	}
}

func TestCopyLinesToDocumentViaAPI(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/src.md", "a\nb\nc")
	seedDoc(t, store, "proj/dest.md", "x\ny")

	callTool(t, s, "copy_lines", map[string]any{
		"path":        "proj/src.md",
		"start_line":  1,
		"end_line":    2,
		"dest_path":   "proj/dest.md",
		"target_line": 2,
	})

	// Source unchanged
	srcResult := callTool(t, s, "get_document", map[string]any{"path": "proj/src.md"})
	var srcDoc map[string]any
	json.Unmarshal([]byte(srcResult), &srcDoc)
	if srcDoc["content"] != "a\nb\nc" {
		t.Errorf("src should be unchanged, got %v", srcDoc["content"])
	}

	// Dest has copied lines
	destResult := callTool(t, s, "get_document", map[string]any{"path": "proj/dest.md"})
	var destDoc map[string]any
	json.Unmarshal([]byte(destResult), &destDoc)
	if destDoc["content"] != "x\na\nb\ny" {
		t.Errorf("dest content: got %v", destDoc["content"])
	}
}

func TestMoveLinesToDocumentMissingSrc(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/dest.md", "x")

	result := callTool(t, s, "move_lines", map[string]any{
		"path":        "proj/missing.md",
		"start_line":  1,
		"end_line":    1,
		"dest_path":   "proj/dest.md",
		"target_line": 1,
	})
	if result != "document not found" {
		t.Errorf("expected 'document not found', got %q", result)
	}
}
