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
	if doc["word_count"] != float64(3) {
		t.Errorf("expected word_count 3, got %v", doc["word_count"])
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

	// Verify start_line (renamed from line_number)
	if toc[0]["start_line"] != float64(1) {
		t.Errorf("expected start_line 1, got %v", toc[0]["start_line"])
	}
	if _, exists := toc[0]["line_number"]; exists {
		t.Error("line_number should be renamed to start_line")
	}
}

func TestGetTableOfContentsEndLine(t *testing.T) {
	s, store := setupTestServer(t)
	// Line 1: # Title
	// Line 2: (blank)
	// Line 3: ## Section A
	// Line 4: Content A
	// Line 5: ## Section B
	// Line 6: Content B
	// Line 7: Content B2
	seedDoc(t, store, "proj/test.md", "# Title\n\n## Section A\nContent A\n## Section B\nContent B\nContent B2")

	result := callTool(t, s, "get_table_of_contents", map[string]any{"path": "proj/test.md"})
	var toc []map[string]any
	if err := json.Unmarshal([]byte(result), &toc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(toc) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(toc))
	}

	// Title (h1) ends before Section A (h2 is lower level, so Title goes to EOF)
	if toc[0]["end_line"] != float64(7) {
		t.Errorf("Title end_line: expected 7, got %v", toc[0]["end_line"])
	}
	// Section A (h2) ends before Section B (same level)
	if toc[1]["end_line"] != float64(4) {
		t.Errorf("Section A end_line: expected 4, got %v", toc[1]["end_line"])
	}
	// Section B (h2) goes to EOF
	if toc[2]["end_line"] != float64(7) {
		t.Errorf("Section B end_line: expected 7, got %v", toc[2]["end_line"])
	}
}

func TestGetTableOfContentsTasksSummary(t *testing.T) {
	s, store := setupTestServer(t)
	content := "# Project\n\n## Tasks\n\n- [x] Done\n- [ ] Open\n- [ ] Also open\n\n## Notes\n\nNo checkboxes here."
	seedDoc(t, store, "proj/test.md", content)

	result := callTool(t, s, "get_table_of_contents", map[string]any{"path": "proj/test.md"})
	var toc []map[string]any
	if err := json.Unmarshal([]byte(result), &toc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(toc) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(toc))
	}

	// Project (h1) — contains all checkboxes (includes sub-sections)
	projectTasks := toc[0]["tasks"].(map[string]any)
	if projectTasks["total"] != float64(3) {
		t.Errorf("Project tasks total: expected 3, got %v", projectTasks["total"])
	}
	if projectTasks["checked"] != float64(1) {
		t.Errorf("Project tasks checked: expected 1, got %v", projectTasks["checked"])
	}

	// Tasks (h2) — has the checkboxes
	tasksTasks := toc[1]["tasks"].(map[string]any)
	if tasksTasks["total"] != float64(3) {
		t.Errorf("Tasks tasks total: expected 3, got %v", tasksTasks["total"])
	}

	// Notes (h2) — no checkboxes, tasks should be nil/absent
	if toc[2]["tasks"] != nil {
		t.Errorf("Notes should have no tasks, got %v", toc[2]["tasks"])
	}
}

func TestGetDocumentWordCount(t *testing.T) {
	s, store := setupTestServer(t)
	content := "# My Doc\n\nSome words here.\n\n## Tasks\n\n- [x] Done task\n- [ ] Open task\n- [ ] Another open\n\n## Notes\n\nMore content here and there."
	seedDoc(t, store, "proj/test.md", content)

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/test.md"})
	var doc map[string]any
	if err := json.Unmarshal([]byte(result), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if doc["word_count"] != float64(18) {
		t.Errorf("expected 18 words, got %v", doc["word_count"])
	}
}

func TestGetDocumentWordCountSkipsMarkups(t *testing.T) {
	s, store := setupTestServer(t)
	content := "# Spec\n\n<!-- specbox:question:abc1234567 status=\"open\"\nquestion: What color?\n-->\n\nSome text here."
	seedDoc(t, store, "proj/spec.md", content)

	result := callTool(t, s, "get_document", map[string]any{"path": "proj/spec.md"})
	var doc map[string]any
	if err := json.Unmarshal([]byte(result), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Only "Spec" + "Some text here." = 4 words (markup content skipped)
	if doc["word_count"] != float64(4) {
		t.Errorf("expected 4 words, got %v", doc["word_count"])
	}
}
