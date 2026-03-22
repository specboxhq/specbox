package mcp_test

import (
	"encoding/json"
	"testing"
)

func TestSaveDocumentCreate(t *testing.T) {
	s, store := setupTestServer(t)
	_ = store

	result := callTool(t, s, "save_document", map[string]any{
		"path":    "proj/new.md",
		"content": "hello world",
		"mode":    "create",
	})
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data["path"] != "proj/new.md" {
		t.Errorf("expected path 'proj/new.md', got %v", data["path"])
	}
	if data["line_count"] != float64(1) {
		t.Errorf("expected line_count 1, got %v", data["line_count"])
	}
}

func TestSaveDocumentCreateDuplicate(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/exists.md", "content")

	result := callTool(t, s, "save_document", map[string]any{
		"path":    "proj/exists.md",
		"content": "again",
		"mode":    "create",
	})
	if result != "document already exists" {
		t.Errorf("expected 'document already exists', got %q", result)
	}
}

func TestSaveDocumentUpsert(t *testing.T) {
	s, _ := setupTestServer(t)

	// Create via upsert
	result := callTool(t, s, "save_document", map[string]any{
		"path":    "proj/test.md",
		"content": "v1",
	})
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data["path"] != "proj/test.md" {
		t.Errorf("got %v", data["path"])
	}

	// Update via upsert
	callTool(t, s, "save_document", map[string]any{
		"path":    "proj/test.md",
		"content": "v2",
	})
	docResult := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	if err := json.Unmarshal([]byte(docResult), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["content"] != "v2" {
		t.Errorf("expected 'v2', got %v", doc["content"])
	}
}

func TestSaveDocumentUpdate(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "old")

	callTool(t, s, "save_document", map[string]any{
		"path":    "proj/test.md",
		"content": "new",
		"mode":    "update",
	})

	docResult := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(docResult), &doc)
	if doc["content"] != "new" {
		t.Errorf("expected 'new', got %v", doc["content"])
	}
}

func TestSaveDocumentUpdateMissing(t *testing.T) {
	s, _ := setupTestServer(t)

	result := callTool(t, s, "save_document", map[string]any{
		"path":    "proj/missing.md",
		"content": "content",
		"mode":    "update",
	})
	if result != "document not found" {
		t.Errorf("expected 'document not found', got %q", result)
	}
}

func TestReplaceNth(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "foo bar foo baz foo")

	callTool(t, s, "replace_text", map[string]any{
		"path":     "proj/test.md",
		"old_text": "foo",
		"new_text": "qux",
		"n":        2,
	})

	docResult := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(docResult), &doc)
	if doc["content"] != "foo bar qux baz foo" {
		t.Errorf("got %v", doc["content"])
	}
}

func TestReplaceNthNoMatch(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "hello")

	result := callTool(t, s, "replace_text", map[string]any{
		"path":     "proj/test.md",
		"old_text": "xyz",
		"new_text": "abc",
		"n":        1,
	})
	if result != "text or pattern not found in document" {
		t.Errorf("expected no match error, got %q", result)
	}
}

func TestReplaceAll(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "foo bar foo baz foo")

	callTool(t, s, "replace_text", map[string]any{
		"path":     "proj/test.md",
		"old_text": "foo",
		"new_text": "qux",
	})

	docResult := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(docResult), &doc)
	if doc["content"] != "qux bar qux baz qux" {
		t.Errorf("got %v", doc["content"])
	}
}

func TestReplaceRegex(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "hello 123 world 456")

	callTool(t, s, "replace_text", map[string]any{
		"path":     "proj/test.md",
		"pattern":  `\d+`,
		"new_text": "NUM",
	})

	docResult := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(docResult), &doc)
	if doc["content"] != "hello NUM world NUM" {
		t.Errorf("got %v", doc["content"])
	}
}

func TestReplaceRegexInvalid(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "hello")

	result := callTool(t, s, "replace_text", map[string]any{
		"path":     "proj/test.md",
		"pattern":  "[invalid",
		"new_text": "x",
	})
	if result != "invalid regex pattern" {
		t.Errorf("expected 'invalid regex pattern', got %q", result)
	}
}

func TestRenameDocument(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/old.md", "content")

	result := callTool(t, s, "move_document", map[string]any{
		"old_path": "proj/old.md",
		"new_path": "proj/new.md",
	})
	var data map[string]any
	json.Unmarshal([]byte(result), &data)
	if data["path"] != "proj/new.md" {
		t.Errorf("expected 'proj/new.md', got %v", data["path"])
	}

	// Old should be gone
	errResult := callTool(t, s, "get_document", map[string]any{"path": "proj/old.md"})
	if errResult != "document not found" {
		t.Errorf("old doc should be gone")
	}
}

func TestInsertTextEnd(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "proj/test.md", "line1")

	callTool(t, s, "insert_text", map[string]any{
		"path":     "proj/test.md",
		"content":  "line2",
		"position": "end",
	})

	docResult := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	json.Unmarshal([]byte(docResult), &doc)
	if doc["content"] != "line1\nline2" {
		t.Errorf("got %v", doc["content"])
	}
}
