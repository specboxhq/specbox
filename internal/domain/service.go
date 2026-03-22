package domain

// DocumentService handles document lookup, persistence, and delegation
// to Document methods. All mutation methods load the doc, perform the
// operation, save, and return the updated document.
type DocumentService interface {
	// --- Discovery ---

	// ListDocuments lists documents with an optional path filter (substring match).
	ListDocuments(filter string) ([]Document, error)

	// SearchDocuments performs full-text search across all document content.
	// contextLines specifies how many lines of context to include before/after each match (0 = none).
	SearchDocuments(query string, contextLines int) ([]SearchResult, error)

	// SearchDocumentsRegex performs regex search across all document content.
	SearchDocumentsRegex(pattern string, contextLines int) ([]SearchResult, error)

	// --- Read ---

	// GetDocument returns a full document by path.
	GetDocument(path string) (*Document, error)

	// GetLines reads a specific line range from a document.
	GetLines(path string, startLine int, endLine int) (string, error)

	// FindLine returns all line numbers where text appears in a document.
	FindLine(path string, text string) ([]int, error)

	// FindLineRegex returns all line numbers matching a regex pattern.
	FindLineRegex(path string, pattern string) ([]int, error)

	// GetTableOfContents returns all headings with line numbers and levels.
	GetTableOfContents(path string) ([]HeadingInfo, error)

	// --- Create/Save ---

	// CreateDocument creates a new document. Errors if it already exists.
	CreateDocument(path string, content string) (*Document, error)

	// SaveDocument upserts a document (create if missing, update if exists).
	SaveDocument(path string, content string) (*Document, error)

	// UpdateDocument performs a full content replacement. Errors if not found.
	UpdateDocument(path string, content string) (*Document, error)

	// --- Rename ---

	// RenameDocument renames a document.
	RenameDocument(oldPath string, newPath string) (*Document, error)

	// --- Replace operations ---

	// ReplaceNth replaces the Nth occurrence of oldText with newText.
	ReplaceNth(path string, oldText string, newText string, n int, startLine int, endLine int) (*Document, error)

	// ReplaceAll replaces all occurrences of oldText with newText.
	ReplaceAll(path string, oldText string, newText string, startLine int, endLine int) (*Document, error)

	// ReplaceRegex performs a regex find/replace on all matches.
	ReplaceRegex(path string, pattern string, replacement string, startLine int, endLine int) (*Document, error)

	// --- Line operations ---

	// InsertLines inserts content at a specific line number.
	InsertLines(path string, lineNum int, content string) (*Document, error)

	// MoveLines moves a line range to a new position.
	MoveLines(path string, startLine int, endLine int, targetLine int) (*Document, error)

	// DeleteLines removes a line range.
	DeleteLines(path string, startLine int, endLine int) (*Document, error)

	// CopyLines duplicates a line range to a new position.
	CopyLines(path string, startLine int, endLine int, targetLine int) (*Document, error)

	// IndentLines indents or outdents a line range.
	IndentLines(path string, startLine int, endLine int, levels int, prefix string) (*Document, error)

	// --- Markdown operations ---

	// CheckCheckbox toggles a markdown checkbox at a line.
	CheckCheckbox(path string, lineNum int, checked bool) (*Document, error)

	// Renumber renumbers or reletters matching prefixed lines in a range.
	Renumber(path string, startLine int, endLine int, prefix string, start string) (*Document, error)

	// RenumberRegex renumbers lines matching a regex pattern. First capture group is replaced.
	RenumberRegex(path string, startLine int, endLine int, pattern string, start string) (*Document, error)

	// GetSection returns content under a heading, plus start/end line numbers.
	GetSection(path string, heading string) (string, int, int, error)

	// InsertAfterHeading inserts content immediately after a heading line.
	InsertAfterHeading(path string, heading string, content string) (*Document, error)

	// MoveSection moves an entire section to after a target heading.
	MoveSection(path string, heading string, targetHeading string) (*Document, error)

	// DeleteSection removes an entire section (heading + body).
	DeleteSection(path string, heading string) (*Document, error)

	// InsertTableRow inserts a row into a markdown table.
	InsertTableRow(path string, lineNum int, values []string) (*Document, error)

	// UpdateTableRow replaces cell values at an existing table row.
	UpdateTableRow(path string, lineNum int, values []string) (*Document, error)

	// DeleteTableRow removes a table row.
	DeleteTableRow(path string, lineNum int) (*Document, error)

	// AppendToDocument appends content to the end of a document.
	AppendToDocument(path string, content string) (*Document, error)

	// --- Checkboxes ---

	// GetCheckboxes extracts markdown checkboxes with optional filtering.
	GetCheckboxes(path string, filter string, format string, startLine int, endLine int) (any, error)

	// --- Formatting ---

	// FormatDocument normalizes formatting (trailing whitespace, blank lines).
	// For markdown files, also aligns tables and normalizes heading spacing.
	FormatDocument(path string, startLine int, endLine int) (*Document, error)

	// --- Cross-document operations ---

	// MoveLinesToDocument moves lines from one doc to another.
	MoveLinesToDocument(srcPath string, startLine int, endLine int, destPath string, targetLine int) (*Document, *Document, error)

	// CopyLinesToDocument copies lines from one doc to another.
	CopyLinesToDocument(srcPath string, startLine int, endLine int, destPath string, targetLine int) (*Document, error)
}
