package domain

import (
	"strings"
	"testing"
	"time"
)

func testDoc(content string) *Document {
	now := time.Now()
	return &Document{Path: "test.md", Content: content, CreatedAt: now, UpdatedAt: now}
}

func TestApplyEdits_Insert(t *testing.T) {
	doc := testDoc("line1\nline2\nline3")
	results, err := doc.ApplyEdits([]Edit{
		{Op: "insert", Line: 2, Content: "new"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Op != "insert" {
		t.Errorf("unexpected results: %v", results)
	}
	lines := strings.Split(doc.Content, "\n")
	if lines[1] != "new" {
		t.Errorf("expected 'new' at line 2, got %q", lines[1])
	}
	if len(lines) != 4 {
		t.Errorf("expected 4 lines, got %d", len(lines))
	}
}

func TestApplyEdits_InsertAfter(t *testing.T) {
	doc := testDoc("line1\nline2\nline3")
	_, err := doc.ApplyEdits([]Edit{
		{Op: "insert", Line: 2, Content: "after2", Position: "after"},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(doc.Content, "\n")
	if lines[2] != "after2" {
		t.Errorf("expected 'after2' at line 3, got %q", lines[2])
	}
}

func TestApplyEdits_Delete(t *testing.T) {
	doc := testDoc("line1\nline2\nline3\nline4")
	_, err := doc.ApplyEdits([]Edit{
		{Op: "delete", StartLine: 2, EndLine: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "line1\nline4" {
		t.Errorf("expected 'line1\\nline4', got %q", doc.Content)
	}
}

func TestApplyEdits_Replace(t *testing.T) {
	doc := testDoc("line1\nline2\nline3\nline4")
	_, err := doc.ApplyEdits([]Edit{
		{Op: "replace", StartLine: 2, EndLine: 3, Content: "replaced"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "line1\nreplaced\nline4" {
		t.Errorf("expected 'line1\\nreplaced\\nline4', got %q", doc.Content)
	}
}

func TestApplyEdits_ReplaceText(t *testing.T) {
	doc := testDoc("hello world\nfoo bar")
	_, err := doc.ApplyEdits([]Edit{
		{Op: "replace_text", Line: 1, OldText: "world", NewText: "earth"},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(doc.Content, "\n")
	if lines[0] != "hello earth" {
		t.Errorf("expected 'hello earth', got %q", lines[0])
	}
}

func TestApplyEdits_MultipleBottomUp(t *testing.T) {
	// Both edits reference original line numbers — no offset needed
	doc := testDoc("line1\nline2\nline3\nline4\nline5")
	_, err := doc.ApplyEdits([]Edit{
		{Op: "delete", StartLine: 2, EndLine: 2},  // delete "line2"
		{Op: "delete", StartLine: 4, EndLine: 4},  // delete "line4" (original line 4)
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "line1\nline3\nline5" {
		t.Errorf("expected 'line1\\nline3\\nline5', got %q", doc.Content)
	}
}

func TestApplyEdits_InsertAndDeleteSameDoc(t *testing.T) {
	doc := testDoc("a\nb\nc\nd")
	_, err := doc.ApplyEdits([]Edit{
		{Op: "insert", Line: 1, Content: "top"},            // insert before line 1
		{Op: "replace_text", Line: 3, OldText: "c", NewText: "C"},  // line 3 in original = "c"
		{Op: "delete", StartLine: 4, EndLine: 4},           // delete "d"
	})
	if err != nil {
		t.Fatal(err)
	}
	// Applied bottom-up: delete 4, replace_text 3, insert 1
	if doc.Content != "top\na\nb\nC" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestApplyEdits_InsertMultiline(t *testing.T) {
	doc := testDoc("a\nb")
	_, err := doc.ApplyEdits([]Edit{
		{Op: "insert", Line: 2, Content: "x\ny\nz"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "a\nx\ny\nz\nb" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestApplyEdits_ReplaceMultiline(t *testing.T) {
	doc := testDoc("a\nb\nc")
	_, err := doc.ApplyEdits([]Edit{
		{Op: "replace", StartLine: 2, EndLine: 2, Content: "x\ny"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "a\nx\ny\nc" {
		t.Errorf("got %q", doc.Content)
	}
}

// --- Validation error tests ---

func TestApplyEdits_InvalidOp(t *testing.T) {
	doc := testDoc("a")
	_, err := doc.ApplyEdits([]Edit{{Op: "bad"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyEdits_InsertOutOfRange(t *testing.T) {
	doc := testDoc("a")
	_, err := doc.ApplyEdits([]Edit{{Op: "insert", Line: 0, Content: "x"}})
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = doc.ApplyEdits([]Edit{{Op: "insert", Line: 3, Content: "x"}})
	if err == nil {
		t.Fatal("expected error for line > lineCount+1")
	}
}

func TestApplyEdits_DeleteOutOfRange(t *testing.T) {
	doc := testDoc("a\nb")
	_, err := doc.ApplyEdits([]Edit{{Op: "delete", StartLine: 1, EndLine: 3}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyEdits_ReplaceTextNotFound(t *testing.T) {
	doc := testDoc("hello world")
	_, err := doc.ApplyEdits([]Edit{{Op: "replace_text", Line: 1, OldText: "xyz", NewText: "abc"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyEdits_Atomic(t *testing.T) {
	// If any edit fails validation, none should be applied
	doc := testDoc("a\nb\nc")
	original := doc.Content
	_, err := doc.ApplyEdits([]Edit{
		{Op: "delete", StartLine: 1, EndLine: 1}, // valid
		{Op: "delete", StartLine: 5, EndLine: 5}, // invalid — out of range
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if doc.Content != original {
		t.Error("document should be unchanged after validation failure")
	}
}

func TestApplyEdits_Empty(t *testing.T) {
	doc := testDoc("a")
	results, err := doc.ApplyEdits([]Edit{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
