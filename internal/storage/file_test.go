package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specboxhq/specbox/internal/domain"
)

func setupTestStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return store
}

func createTestDoc(t *testing.T, store *FileStore, path, content string) {
	t.Helper()
	if _, err := store.CreateDocument(path, content); err != nil {
		t.Fatalf("CreateDocument(%s): %v", path, err)
	}
}

// --- Discovery ---

func TestListDocuments(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/alpha.md", "aaa")
	createTestDoc(t, store, "proj/beta.md", "bbb")
	createTestDoc(t, store, "proj/gamma.txt", "ccc")

	docs, err := store.ListDocuments("")
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(docs) != 3 {
		t.Errorf("expected 3 docs, got %d", len(docs))
	}

	docs, err = store.ListDocuments(".md")
	if err != nil {
		t.Fatalf("ListDocuments with filter: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 .md docs, got %d", len(docs))
	}
}

func TestSearchDocuments(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/a.md", "hello world\ngoodbye world")
	createTestDoc(t, store, "proj/b.md", "foo bar\nhello there")

	results, err := store.SearchDocuments("hello", 0)
	if err != nil {
		t.Fatalf("SearchDocuments: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	results, err = store.SearchDocuments("nonexistent", 0)
	if err != nil {
		t.Fatalf("SearchDocuments: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- Read ---

func TestGetDocument(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "hello")

	doc, err := store.GetDocument("proj/test.md")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc.Content != "hello" {
		t.Errorf("expected 'hello', got %q", doc.Content)
	}

	_, err = store.GetDocument("proj/missing.md")
	if err != domain.ErrDocumentNotFound {
		t.Errorf("expected ErrDocumentNotFound, got %v", err)
	}
}

func TestGetLines(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "line1\nline2\nline3\nline4")

	content, err := store.GetLines("proj/test.md", 2, 3)
	if err != nil {
		t.Fatalf("GetLines: %v", err)
	}
	if content != "line2\nline3" {
		t.Errorf("expected 'line2\\nline3', got %q", content)
	}

	_, err = store.GetLines("proj/test.md", 0, 3)
	if err != domain.ErrLineOutOfRange {
		t.Errorf("expected ErrLineOutOfRange, got %v", err)
	}
}

func TestFindLine(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "apple\nbanana\napple pie\ncherry")

	lines, err := store.FindLine("proj/test.md", "apple")
	if err != nil {
		t.Fatalf("FindLine: %v", err)
	}
	if len(lines) != 2 || lines[0] != 1 || lines[1] != 3 {
		t.Errorf("expected [1, 3], got %v", lines)
	}
}

func TestGetTableOfContents(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "# Title\n\nSome text\n\n## Section A\n\nContent\n\n## Section B\n\n### Subsection")

	toc, err := store.GetTableOfContents("proj/test.md")
	if err != nil {
		t.Fatalf("GetTableOfContents: %v", err)
	}
	if len(toc) != 4 {
		t.Fatalf("expected 4 headings, got %d", len(toc))
	}
	if toc[0].Heading != "Title" || toc[0].Level != 1 {
		t.Errorf("heading 0: got %+v", toc[0])
	}
	if toc[1].Heading != "Section A" || toc[1].Level != 2 {
		t.Errorf("heading 1: got %+v", toc[1])
	}
	if toc[3].Heading != "Subsection" || toc[3].Level != 3 {
		t.Errorf("heading 3: got %+v", toc[3])
	}
}

// --- Create/Save/Update ---

func TestCreateDocumentDuplicate(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "hello")

	_, err := store.CreateDocument("proj/test.md", "again")
	if err != domain.ErrDocumentAlreadyExists {
		t.Errorf("expected ErrDocumentAlreadyExists, got %v", err)
	}
}

func TestSaveDocument(t *testing.T) {
	store := setupTestStore(t)

	// Create via save
	doc, err := store.SaveDocument("proj/test.md", "v1")
	if err != nil {
		t.Fatalf("SaveDocument create: %v", err)
	}
	if doc.Content != "v1" {
		t.Errorf("expected 'v1', got %q", doc.Content)
	}

	// Update via save
	doc, err = store.SaveDocument("proj/test.md", "v2")
	if err != nil {
		t.Fatalf("SaveDocument update: %v", err)
	}
	if doc.Content != "v2" {
		t.Errorf("expected 'v2', got %q", doc.Content)
	}
}

func TestUpdateDocumentMissing(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.UpdateDocument("proj/missing.md", "content")
	if err != domain.ErrDocumentNotFound {
		t.Errorf("expected ErrDocumentNotFound, got %v", err)
	}
}

// --- Rename ---

func TestRenameDocument(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/old.md", "content")

	doc, err := store.RenameDocument("proj/old.md", "proj/new.md")
	if err != nil {
		t.Fatalf("RenameDocument: %v", err)
	}
	if doc.Path != "proj/new.md" {
		t.Errorf("expected 'proj/new.md', got %q", doc.Path)
	}

	// Old should be gone
	_, err = store.GetDocument("proj/old.md")
	if err != domain.ErrDocumentNotFound {
		t.Errorf("old file should be gone")
	}
}

func TestRenameDocumentToExisting(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/a.md", "aaa")
	createTestDoc(t, store, "proj/b.md", "bbb")

	_, err := store.RenameDocument("proj/a.md", "proj/b.md")
	if err != domain.ErrDocumentAlreadyExists {
		t.Errorf("expected ErrDocumentAlreadyExists, got %v", err)
	}
}

func TestRenameDocumentMissing(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.RenameDocument("proj/missing.md", "proj/new.md")
	if err != domain.ErrDocumentNotFound {
		t.Errorf("expected ErrDocumentNotFound, got %v", err)
	}
}

// --- Replace ---

func TestReplaceNth(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "foo bar foo baz foo")

	doc, err := store.ReplaceNth("proj/test.md", "foo", "qux", 2, 0, 0)
	if err != nil {
		t.Fatalf("ReplaceNth: %v", err)
	}
	if doc.Content != "foo bar qux baz foo" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestReplaceNthDefault(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "foo bar foo")

	doc, err := store.ReplaceNth("proj/test.md", "foo", "qux", 0, 0, 0)
	if err != nil {
		t.Fatalf("ReplaceNth default: %v", err)
	}
	if doc.Content != "qux bar foo" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestReplaceNthNoMatch(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "hello world")

	_, err := store.ReplaceNth("proj/test.md", "xyz", "abc", 1, 0, 0)
	if err != domain.ErrEditNoMatch {
		t.Errorf("expected ErrEditNoMatch, got %v", err)
	}
}

func TestReplaceNthOutOfRange(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "foo bar foo")

	_, err := store.ReplaceNth("proj/test.md", "foo", "qux", 5, 0, 0)
	if err != domain.ErrNthOutOfRange {
		t.Errorf("expected ErrNthOutOfRange, got %v", err)
	}
}

func TestReplaceNthWithLineRange(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "foo\nbar\nfoo\nbaz")

	doc, err := store.ReplaceNth("proj/test.md", "foo", "qux", 1, 3, 4)
	if err != nil {
		t.Fatalf("ReplaceNth with range: %v", err)
	}
	if doc.Content != "foo\nbar\nqux\nbaz" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestReplaceAll(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "foo bar foo baz foo")

	doc, err := store.ReplaceAll("proj/test.md", "foo", "qux", 0, 0)
	if err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}
	if doc.Content != "qux bar qux baz qux" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestReplaceAllWithLineRange(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "foo\nfoo\nfoo")

	doc, err := store.ReplaceAll("proj/test.md", "foo", "bar", 2, 2)
	if err != nil {
		t.Fatalf("ReplaceAll with range: %v", err)
	}
	if doc.Content != "foo\nbar\nfoo" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestReplaceRegex(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "hello 123 world 456")

	doc, err := store.ReplaceRegex("proj/test.md", `\d+`, "NUM", 0, 0)
	if err != nil {
		t.Fatalf("ReplaceRegex: %v", err)
	}
	if doc.Content != "hello NUM world NUM" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestReplaceRegexCaptureGroups(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "John Smith\nJane Doe")

	doc, err := store.ReplaceRegex("proj/test.md", `(\w+) (\w+)`, "$2, $1", 0, 0)
	if err != nil {
		t.Fatalf("ReplaceRegex capture: %v", err)
	}
	if doc.Content != "Smith, John\nDoe, Jane" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestReplaceRegexCaseInsensitive(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "Hello HELLO hello")

	doc, err := store.ReplaceRegex("proj/test.md", `(?i)hello`, "hi", 0, 0)
	if err != nil {
		t.Fatalf("ReplaceRegex case insensitive: %v", err)
	}
	if doc.Content != "hi hi hi" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestReplaceRegexInvalid(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "hello")

	_, err := store.ReplaceRegex("proj/test.md", `[invalid`, "x", 0, 0)
	if err != domain.ErrInvalidRegex {
		t.Errorf("expected ErrInvalidRegex, got %v", err)
	}
}

// --- Line operations ---

func TestInsertLines(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "line1\nline3")

	doc, err := store.InsertLines("proj/test.md", 2, "line2")
	if err != nil {
		t.Fatalf("InsertLines: %v", err)
	}
	if doc.Content != "line1\nline2\nline3" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestMoveLines(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "a\nb\nc\nd\ne")

	// Move lines 2-3 to after line 4
	doc, err := store.MoveLines("proj/test.md", 2, 3, 5)
	if err != nil {
		t.Fatalf("MoveLines: %v", err)
	}
	if doc.Content != "a\nd\nb\nc\ne" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestDeleteLines(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "a\nb\nc\nd")

	doc, err := store.DeleteLines("proj/test.md", 2, 3)
	if err != nil {
		t.Fatalf("DeleteLines: %v", err)
	}
	if doc.Content != "a\nd" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestCopyLines(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "a\nb\nc")

	doc, err := store.CopyLines("proj/test.md", 1, 2, 4)
	if err != nil {
		t.Fatalf("CopyLines: %v", err)
	}
	if doc.Content != "a\nb\nc\na\nb" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestIndentLines(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "a\nb\nc")

	doc, err := store.IndentLines("proj/test.md", 1, 3, 1, "  ")
	if err != nil {
		t.Fatalf("IndentLines: %v", err)
	}
	if doc.Content != "  a\n  b\n  c" {
		t.Errorf("got %q", doc.Content)
	}

	// Outdent
	doc, err = store.IndentLines("proj/test.md", 1, 3, -1, "  ")
	if err != nil {
		t.Fatalf("IndentLines outdent: %v", err)
	}
	if doc.Content != "a\nb\nc" {
		t.Errorf("got %q", doc.Content)
	}
}

func TestIndentLinesOutdentNoPrefix(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "  a\nb\n  c")

	doc, err := store.IndentLines("proj/test.md", 1, 3, -1, "  ")
	if err != nil {
		t.Fatalf("IndentLines outdent partial: %v", err)
	}
	// b has no prefix, should be unchanged
	if doc.Content != "a\nb\nc" {
		t.Errorf("got %q", doc.Content)
	}
}

// --- Markdown operations ---

func TestCheckCheckbox(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "- [ ] todo1\n- [ ] todo2\n- [x] done")

	doc, err := store.CheckCheckbox("proj/test.md", 1, true)
	if err != nil {
		t.Fatalf("CheckCheckbox check: %v", err)
	}
	lines := strings.Split(doc.Content, "\n")
	if lines[0] != "- [x] todo1" {
		t.Errorf("got %q", lines[0])
	}

	doc, err = store.CheckCheckbox("proj/test.md", 3, false)
	if err != nil {
		t.Fatalf("CheckCheckbox uncheck: %v", err)
	}
	lines = strings.Split(doc.Content, "\n")
	if lines[2] != "- [ ] done" {
		t.Errorf("got %q", lines[2])
	}
}

func TestCheckCheckboxNotACheckbox(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "just text")

	_, err := store.CheckCheckbox("proj/test.md", 1, true)
	if err != domain.ErrNotACheckbox {
		t.Errorf("expected ErrNotACheckbox, got %v", err)
	}
}

func TestGetSection(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "# Title\n\nIntro\n\n## Section A\n\nContent A\n\n## Section B\n\nContent B")

	content, start, end, err := store.GetSection("proj/test.md", "Section A")
	if err != nil {
		t.Fatalf("GetSection: %v", err)
	}
	if start != 5 || end != 8 {
		t.Errorf("expected lines 5-8, got %d-%d", start, end)
	}
	if !strings.Contains(content, "Content A") {
		t.Errorf("expected content to contain 'Content A', got %q", content)
	}
}

func TestGetSectionNotFound(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "# Title\n\nContent")

	_, _, _, err := store.GetSection("proj/test.md", "Missing")
	if err != domain.ErrHeadingNotFound {
		t.Errorf("expected ErrHeadingNotFound, got %v", err)
	}
}

func TestInsertAfterHeading(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "# Title\n\nOld content")

	doc, err := store.InsertAfterHeading("proj/test.md", "Title", "New line")
	if err != nil {
		t.Fatalf("InsertAfterHeading: %v", err)
	}
	lines := strings.Split(doc.Content, "\n")
	if lines[1] != "New line" {
		t.Errorf("expected 'New line' at line 2, got %q", lines[1])
	}
}

func TestMoveSection(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "## A\n\nContent A\n\n## B\n\nContent B\n\n## C\n\nContent C")

	doc, err := store.MoveSection("proj/test.md", "A", "C")
	if err != nil {
		t.Fatalf("MoveSection: %v", err)
	}
	// A should now be after C
	toc, _ := doc.GetTableOfContents()
	if len(toc) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(toc))
	}
	if toc[0].Heading != "B" || toc[1].Heading != "C" || toc[2].Heading != "A" {
		t.Errorf("expected B, C, A order, got %s, %s, %s", toc[0].Heading, toc[1].Heading, toc[2].Heading)
	}
}

func TestDeleteSection(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "## A\n\nContent A\n\n## B\n\nContent B")

	doc, err := store.DeleteSection("proj/test.md", "A")
	if err != nil {
		t.Fatalf("DeleteSection: %v", err)
	}
	if strings.Contains(doc.Content, "Content A") {
		t.Errorf("expected Content A to be removed")
	}
	if !strings.Contains(doc.Content, "Content B") {
		t.Errorf("expected Content B to remain")
	}
}

func TestInsertTableRow(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "| Name | Age |\n|---|---|\n| Alice | 30 |")

	doc, err := store.InsertTableRow("proj/test.md", 4, []string{"Bob", "25"})
	if err != nil {
		t.Fatalf("InsertTableRow: %v", err)
	}
	lines := strings.Split(doc.Content, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[3], "Bob") || !strings.Contains(lines[3], "25") {
		t.Errorf("expected row with Bob/25, got %q", lines[3])
	}
}

func TestInsertTableRowNotATable(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "just text\nno table here")

	_, err := store.InsertTableRow("proj/test.md", 2, []string{"a", "b"})
	if err != domain.ErrNotATable {
		t.Errorf("expected ErrNotATable, got %v", err)
	}
}

func TestAppendToDocument(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "line1")

	doc, err := store.AppendToDocument("proj/test.md", "line2")
	if err != nil {
		t.Fatalf("AppendToDocument: %v", err)
	}
	if doc.Content != "line1\nline2" {
		t.Errorf("got %q", doc.Content)
	}
}

// --- Cross-document operations ---

func TestMoveLinesToDocument(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/src.md", "a\nb\nc\nd")
	createTestDoc(t, store, "proj/dest.md", "x\ny")

	src, dest, err := store.MoveLinesToDocument("proj/src.md", 2, 3, "proj/dest.md", 2)
	if err != nil {
		t.Fatalf("MoveLinesToDocument: %v", err)
	}
	if src.Content != "a\nd" {
		t.Errorf("src: got %q", src.Content)
	}
	if dest.Content != "x\nb\nc\ny" {
		t.Errorf("dest: got %q", dest.Content)
	}

	// Verify persistence
	srcRead, _ := store.GetDocument("proj/src.md")
	destRead, _ := store.GetDocument("proj/dest.md")
	if srcRead.Content != "a\nd" {
		t.Errorf("persisted src: got %q", srcRead.Content)
	}
	if destRead.Content != "x\nb\nc\ny" {
		t.Errorf("persisted dest: got %q", destRead.Content)
	}
}

func TestCopyLinesToDocument(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/src.md", "a\nb\nc")
	createTestDoc(t, store, "proj/dest.md", "x\ny")

	dest, err := store.CopyLinesToDocument("proj/src.md", 1, 2, "proj/dest.md", 2)
	if err != nil {
		t.Fatalf("CopyLinesToDocument: %v", err)
	}
	if dest.Content != "x\na\nb\ny" {
		t.Errorf("dest: got %q", dest.Content)
	}

	// Source should be unchanged
	src, _ := store.GetDocument("proj/src.md")
	if src.Content != "a\nb\nc" {
		t.Errorf("src should be unchanged, got %q", src.Content)
	}
}

// --- Edge cases ---

func TestLineOutOfRange(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj/test.md", "a\nb")

	_, err := store.DeleteLines("proj/test.md", 1, 5)
	if err != domain.ErrLineOutOfRange {
		t.Errorf("expected ErrLineOutOfRange, got %v", err)
	}

	_, err = store.MoveLines("proj/test.md", 1, 1, 10)
	if err != domain.ErrLineOutOfRange {
		t.Errorf("expected ErrLineOutOfRange for target, got %v", err)
	}
}

func TestNewFileStoreCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "docs")
	_, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore should create nested dir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

// --- Path validation ---

func TestProjectFilenamePaths(t *testing.T) {
	store := setupTestStore(t)

	// Valid: project/filename
	doc, err := store.CreateDocument("my-project/notes.md", "content")
	if err != nil {
		t.Fatalf("CreateDocument with project path: %v", err)
	}
	if doc.Path != "my-project/notes.md" {
		t.Errorf("expected 'my-project/notes.md', got %q", doc.Path)
	}

	// Verify it can be read back
	doc, err = store.GetDocument("my-project/notes.md")
	if err != nil {
		t.Fatalf("GetDocument with project path: %v", err)
	}
	if doc.Content != "content" {
		t.Errorf("expected 'content', got %q", doc.Content)
	}
}

func TestPathValidationRejections(t *testing.T) {
	store := setupTestStore(t)

	tests := []struct {
		name string
		path string
		err  error
	}{
		{"path traversal", "../escape.md", domain.ErrInvalidPath},
		{"nested traversal", "foo/../../escape.md", domain.ErrInvalidPath},
		{"absolute path", "/absolute/path.md", domain.ErrInvalidPath},
		{"empty path", "", domain.ErrInvalidPath},
	}

	for _, tt := range tests {
		_, err := store.CreateDocument(tt.path, "content")
		if err == nil {
			t.Errorf("%s: expected error for path %q, got nil", tt.name, tt.path)
		}
	}
}

func TestListDocumentsAcrossProjects(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj-a/doc.md", "aaa")
	createTestDoc(t, store, "proj-b/doc.md", "bbb")
	createTestDoc(t, store, "proj-b/other.md", "ccc")

	docs, err := store.ListDocuments("")
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(docs) != 3 {
		t.Errorf("expected 3 docs, got %d", len(docs))
	}

	// Filter by project
	docs, err = store.ListDocuments("proj-b/")
	if err != nil {
		t.Fatalf("ListDocuments filter: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 proj-b docs, got %d", len(docs))
	}
}

func TestSearchDocumentsAcrossProjects(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "proj-a/doc.md", "hello world")
	createTestDoc(t, store, "proj-b/doc.md", "hello there")

	results, err := store.SearchDocuments("hello", 0)
	if err != nil {
		t.Fatalf("SearchDocuments: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestRenameDocumentAcrossProjects(t *testing.T) {
	store := setupTestStore(t)
	createTestDoc(t, store, "old-proj/doc.md", "content")

	doc, err := store.RenameDocument("old-proj/doc.md", "new-proj/doc.md")
	if err != nil {
		t.Fatalf("RenameDocument across projects: %v", err)
	}
	if doc.Path != "new-proj/doc.md" {
		t.Errorf("expected 'new-proj/doc.md', got %q", doc.Path)
	}

	_, err = store.GetDocument("old-proj/doc.md")
	if err != domain.ErrDocumentNotFound {
		t.Errorf("old path should be gone")
	}
}
