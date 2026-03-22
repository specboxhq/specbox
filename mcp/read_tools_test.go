package mcp_test

import (
	"encoding/json"
	"testing"
)

func TestListDocuments(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/a.md", "aaa")
	seedDoc(t, store, "proj/b.md", "bbb")
	seedDoc(t, store, "specs/c.md", "ccc")

	result := callTool(t, s, "list_documents", map[string]any{})
	var docs []map[string]any
	if err := json.Unmarshal([]byte(result), &docs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(docs) != 3 {
		t.Errorf("expected 3 docs, got %d", len(docs))
	}

	// With filter
	result = callTool(t, s, "list_documents", map[string]any{"filter": "specs/"})
	if err := json.Unmarshal([]byte(result), &docs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("expected 1 doc, got %d", len(docs))
	}
}

func TestSearchDocuments(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/a.md", "hello world\ngoodbye")
	seedDoc(t, store, "proj/b.md", "hello there")

	result := callTool(t, s, "search_documents", map[string]any{"query": "hello"})
	var results []map[string]any
	if err := json.Unmarshal([]byte(result), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestGetDocument(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "line1\nline2\nline3")

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	if err := json.Unmarshal([]byte(result), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["path"] != "proj/test.md" {
		t.Errorf("expected path 'test.md', got %v", doc["path"])
	}
	if doc["content"] != "line1\nline2\nline3" {
		t.Errorf("unexpected content: %v", doc["content"])
	}
	if doc["line_count"] != float64(3) {
		t.Errorf("expected line_count 3, got %v", doc["line_count"])
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	s, _ := setupTestServer(t)

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/missing.md"})
	if result != "document not found" {
		t.Errorf("expected 'document not found', got %q", result)
	}
}

func TestGetLines(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "a\nb\nc\nd")

	result := callTool(t, s, "get_lines", map[string]any{
		"path":       "proj/test.md",
		"start_line": 2,
		"end_line":   3,
	})
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data["content"] != "b\nc" {
		t.Errorf("expected 'b\\nc', got %v", data["content"])
	}
}

func TestFindLine(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "apple\nbanana\napple pie")

	result := callTool(t, s, "find_text", map[string]any{
		"path": "proj/test.md",
		"query": "apple",
	})
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	lines, ok := data["lines"].([]any)
	if !ok {
		t.Fatalf("expected lines array, got %T", data["lines"])
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 matches, got %d", len(lines))
	}
}

func TestGetTableOfContents(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "# Title\n\n## Section A\n\n### Sub")

	result := callTool(t, s, "get_table_of_contents", map[string]any{"path": "proj/test.md"})
	var toc []map[string]any
	if err := json.Unmarshal([]byte(result), &toc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(toc) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(toc))
	}
	if toc[0]["heading"] != "Title" || toc[0]["level"] != float64(1) {
		t.Errorf("heading 0: %v", toc[0])
	}
	if toc[1]["heading"] != "Section A" || toc[1]["level"] != float64(2) {
		t.Errorf("heading 1: %v", toc[1])
	}
}
