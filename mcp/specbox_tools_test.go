package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAddMarkupBlock(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Auth\n\nSome content\n")

	result := callTool(t, s, "add_markup", map[string]any{
		"path":       "spec.md",
		"type":       "question",
		"question":   "JWT or sessions?",
		"start_line": 3,
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["markup_id"] == nil || resp["markup_id"] == "" {
		t.Error("expected markup_id in response")
	}
	if resp["mode"] != "block" {
		t.Errorf("expected mode=block, got %v", resp["mode"])
	}

	// Verify the markup is in the document
	doc, err := store.GetDocument("spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Content, "specbox:question:") {
		t.Error("expected specbox markup in document content")
	}
	if !strings.Contains(doc.Content, "question: JWT or sessions?") {
		t.Error("expected question in markup")
	}
}

func TestAddMarkupWrapped(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Auth\n\nSome content\nMore content\nEnd\n")

	result := callTool(t, s, "add_markup", map[string]any{
		"path":       "spec.md",
		"type":       "question",
		"start_line": 3,
		"end_line":   4,
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["mode"] != "wrapped" {
		t.Errorf("expected mode=wrapped, got %v", resp["mode"])
	}

	doc, err := store.GetDocument("spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Content, "<!-- /specbox:question -->") {
		t.Error("expected closing tag")
	}
}

func TestAddMarkupWithHeading(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Auth\n\n## Design\n\nContent here\n")

	result := callTool(t, s, "add_markup", map[string]any{
		"path":     "spec.md",
		"type":     "question",
		"question": "What about caching?",
		"heading":  "Design",
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["mode"] != "block" {
		t.Errorf("expected mode=block, got %v", resp["mode"])
	}
}

func TestUpdateMarkup(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Auth\n\n<!-- specbox:question:a7k2m9x4p1 status=\"open\" JWT or sessions? -->\n\nContent\n")

	callTool(t, s, "update_markup", map[string]any{
		"path":   "spec.md",
		"id":     "a7k2m9x4p1",
		"status": "resolved",
		"answer": "JWT",
	})

	doc, err := store.GetDocument("spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Content, "resolved") {
		t.Error("expected status to be updated to resolved")
	}
	if !strings.Contains(doc.Content, "answer=\"JWT\"") {
		t.Error("expected answer attribute")
	}
}

func TestDeleteMarkupInline(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Auth\n\n<!-- specbox:question:a7k2m9x4p1 JWT or sessions? -->\n\nContent\n")

	callTool(t, s, "delete_markup", map[string]any{
		"path": "spec.md",
		"id":   "a7k2m9x4p1",
	})

	doc, err := store.GetDocument("spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doc.Content, "specbox:question") {
		t.Error("expected markup to be removed")
	}
	if !strings.Contains(doc.Content, "Content") {
		t.Error("expected other content to remain")
	}
}

func TestDeleteMarkupWrapped(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Auth\n\n<!-- specbox:question:a7k2m9x4p1 status=\"open\" -->\nWrapped content\nMore wrapped\n<!-- /specbox:question -->\n\nAfter\n")

	callTool(t, s, "delete_markup", map[string]any{
		"path": "spec.md",
		"id":   "a7k2m9x4p1",
	})

	doc, err := store.GetDocument("spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doc.Content, "specbox:question") {
		t.Error("expected tags to be removed")
	}
	if !strings.Contains(doc.Content, "Wrapped content") {
		t.Error("expected wrapped content to be preserved")
	}
	if !strings.Contains(doc.Content, "More wrapped") {
		t.Error("expected wrapped content to be preserved")
	}
}

func TestListMarkups(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Spec\n\n<!-- specbox:question:aaaaaaaaaa status=\"open\" D1 -->\n\n<!-- specbox:question:bbbbbbbbbb status=\"resolved\" Q1 -->\n")

	result := callTool(t, s, "list_markups", map[string]any{
		"path": "spec.md",
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	count := int(resp["count"].(float64))
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}

	// Filter open
	result = callTool(t, s, "list_markups", map[string]any{
		"path":   "spec.md",
		"status": []string{"open"},
	})
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	count = int(resp["count"].(float64))
	if count != 1 {
		t.Errorf("expected 1 open, got %d", count)
	}

	// Filter resolved
	result = callTool(t, s, "list_markups", map[string]any{
		"path":   "spec.md",
		"status": []string{"resolved"},
	})
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	count = int(resp["count"].(float64))
	if count != 1 {
		t.Errorf("expected 1 resolved, got %d", count)
	}
}

func TestResolveMarkup(t *testing.T) {
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Spec\n\n<!-- specbox:question:a7k2m9x4p1 status=\"open\" JWT or sessions? -->\n")

	callTool(t, s, "resolve_markup", map[string]any{
		"path":   "spec.md",
		"id":     "a7k2m9x4p1",
		"answer": "JWT",
	})

	doc, err := store.GetDocument("spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Content, "status=\"resolved\"") {
		t.Error("expected status resolved")
	}
	if !strings.Contains(doc.Content, "answer=\"JWT\"") {
		t.Error("expected answer attribute")
	}
}

func TestCheckSpec(t *testing.T) {
	s, store := setupTestServer(t)
	content := "---\nspecbox:\n  url: https://specbox.io/s/abc123\n  status: draft\n---\n# Spec\n\n<!-- specbox:question:aaaaaaaaaa status=\"open\" D1 -->\n<!-- specbox:question:bbbbbbbbbb status=\"resolved\" Q1 -->\n"
	seedDoc(t, store, "spec.md", content)

	result := callTool(t, s, "check_spec", map[string]any{
		"path": "spec.md",
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	markups := resp["markups"].(map[string]any)
	if int(markups["total"].(float64)) != 2 {
		t.Errorf("expected 2 total markups")
	}
	if int(markups["open"].(float64)) != 1 {
		t.Errorf("expected 1 open")
	}
	if int(markups["resolved"].(float64)) != 1 {
		t.Errorf("expected 1 resolved")
	}
}

func TestPushSpecRequiresAuth(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // ensure no auth config found
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Spec\n")

	result := callTool(t, s, "push_spec", map[string]any{
		"path": "spec.md",
	})

	if !strings.Contains(result, "not logged in") {
		t.Errorf("expected auth error, got: %s", result)
	}
}

func TestPullSpecRequiresAuth(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "---\nspecbox:\n  id: aBcDeFgHiJkL\n  version: 1\n---\n# Spec\n")

	result := callTool(t, s, "pull_spec", map[string]any{
		"path": "spec.md",
	})

	if !strings.Contains(result, "not logged in") {
		t.Errorf("expected auth error, got: %s", result)
	}
}

func TestPullSpecRequiresSpecID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, store := setupTestServer(t)
	seedDoc(t, store, "spec.md", "# Spec with no frontmatter\n")

	// Auth check comes first, so without login we get auth error.
	result := callTool(t, s, "pull_spec", map[string]any{
		"path": "spec.md",
	})

	if !strings.Contains(result, "not logged in") {
		t.Errorf("expected auth error, got: %s", result)
	}
}
