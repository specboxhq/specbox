package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/specboxhq/specbox/internal/domain"
)

// Compile-time interface check.
var _ domain.DocumentService = (*FileStore)(nil)

// FileStore implements DocumentService using file storage with subdirectory support.
type FileStore struct {
	baseDir string
}

// NewFileStore creates a new FileStore. Creates baseDir if it doesn't exist.
func NewFileStore(baseDir string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &FileStore{baseDir: baseDir}, nil
}

// validatePath checks that a path is relative and doesn't escape the base directory.
func validatePath(p string) error {
	if p == "" {
		return domain.ErrInvalidPath
	}
	if filepath.IsAbs(p) {
		return domain.ErrInvalidPath
	}
	cleaned := filepath.Clean(p)
	if strings.HasPrefix(cleaned, "..") {
		return domain.ErrInvalidPath
	}
	if cleaned == "." {
		return domain.ErrInvalidPath
	}
	return nil
}

func (fs *FileStore) fullPath(docPath string) string {
	return filepath.Join(fs.baseDir, docPath)
}

// ensureParentDir creates parent directories for a document path if needed.
func (fs *FileStore) ensureParentDir(docPath string) error {
	dir := filepath.Dir(fs.fullPath(docPath))
	return os.MkdirAll(dir, 0755)
}

func (fs *FileStore) loadDoc(docPath string) (*domain.Document, error) {
	if err := validatePath(docPath); err != nil {
		return nil, err
	}
	p := fs.fullPath(docPath)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrDocumentNotFound
		}
		return nil, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	return &domain.Document{
		Path:      docPath,
		Content:   string(data),
		CreatedAt: info.ModTime(),
		UpdatedAt: info.ModTime(),
	}, nil
}

func (fs *FileStore) saveDoc(doc *domain.Document) error {
	if err := validatePath(doc.Path); err != nil {
		return err
	}
	if err := fs.ensureParentDir(doc.Path); err != nil {
		return err
	}
	return os.WriteFile(fs.fullPath(doc.Path), []byte(doc.Content), 0644)
}

func (fs *FileStore) exists(docPath string) bool {
	_, err := os.Stat(fs.fullPath(docPath))
	return err == nil
}

// walkFiles recursively walks baseDir and returns all file paths relative to baseDir.
func (fs *FileStore) walkFiles() ([]string, error) {
	var paths []string
	err := filepath.WalkDir(fs.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(fs.baseDir, path)
		if err != nil {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// --- Discovery ---

func (fs *FileStore) ListDocuments(filter string) ([]domain.Document, error) {
	paths, err := fs.walkFiles()
	if err != nil {
		return nil, err
	}
	var docs []domain.Document
	for _, p := range paths {
		if filter != "" && !strings.Contains(p, filter) {
			continue
		}
		doc, err := fs.loadDoc(p)
		if err != nil {
			continue
		}
		docs = append(docs, *doc)
	}
	return docs, nil
}

func (fs *FileStore) SearchDocuments(query string, contextLines int) ([]domain.SearchResult, error) {
	paths, err := fs.walkFiles()
	if err != nil {
		return nil, err
	}
	var results []domain.SearchResult
	for _, p := range paths {
		doc, err := fs.loadDoc(p)
		if err != nil {
			continue
		}
		lines := strings.Split(doc.Content, "\n")
		for i, line := range lines {
			if strings.Contains(line, query) {
				result := domain.SearchResult{
					Path:        doc.Path,
					LineNumber:  i + 1,
					LineContent: line,
				}
				if contextLines > 0 {
					start := i - contextLines
					if start < 0 {
						start = 0
					}
					end := i + contextLines
					if end >= len(lines) {
						end = len(lines) - 1
					}
					result.ContextBefore = lines[start:i]
					if i+1 <= end {
						result.ContextAfter = lines[i+1 : end+1]
					}
				}
				results = append(results, result)
			}
		}
	}
	return results, nil
}

func (fs *FileStore) SearchDocumentsRegex(pattern string, contextLines int) ([]domain.SearchResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}
	paths, err := fs.walkFiles()
	if err != nil {
		return nil, err
	}
	var results []domain.SearchResult
	for _, p := range paths {
		doc, err := fs.loadDoc(p)
		if err != nil {
			continue
		}
		lines := strings.Split(doc.Content, "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				result := domain.SearchResult{
					Path:        doc.Path,
					LineNumber:  i + 1,
					LineContent: line,
				}
				if contextLines > 0 {
					start := i - contextLines
					if start < 0 {
						start = 0
					}
					end := i + contextLines
					if end >= len(lines) {
						end = len(lines) - 1
					}
					result.ContextBefore = lines[start:i]
					if i+1 <= end {
						result.ContextAfter = lines[i+1 : end+1]
					}
				}
				results = append(results, result)
			}
		}
	}
	return results, nil
}

// --- Read ---

func (fs *FileStore) GetDocument(path string) (*domain.Document, error) {
	return fs.loadDoc(path)
}

func (fs *FileStore) GetLines(path string, startLine int, endLine int) (string, error) {
	doc, err := fs.loadDoc(path)
	if err != nil {
		return "", err
	}
	return doc.GetLines(startLine, endLine)
}

func (fs *FileStore) FindLine(path string, text string) ([]int, error) {
	doc, err := fs.loadDoc(path)
	if err != nil {
		return nil, err
	}
	return doc.FindLine(text)
}

func (fs *FileStore) FindLineRegex(path string, pattern string) ([]int, error) {
	doc, err := fs.loadDoc(path)
	if err != nil {
		return nil, err
	}
	return doc.FindLineRegex(pattern)
}

func (fs *FileStore) GetTableOfContents(path string) ([]domain.HeadingInfo, error) {
	doc, err := fs.loadDoc(path)
	if err != nil {
		return nil, err
	}
	return doc.GetTableOfContents()
}

// --- Create/Save ---

func (fs *FileStore) CreateDocument(path string, content string) (*domain.Document, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}
	if fs.exists(path) {
		return nil, domain.ErrDocumentAlreadyExists
	}
	doc := &domain.Document{
		Path:      path,
		Content:   content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := fs.saveDoc(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (fs *FileStore) SaveDocument(path string, content string) (*domain.Document, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}
	doc := &domain.Document{
		Path:      path,
		Content:   content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if existing, err := fs.loadDoc(path); err == nil {
		doc.CreatedAt = existing.CreatedAt
	}
	if err := fs.saveDoc(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (fs *FileStore) UpdateDocument(path string, content string) (*domain.Document, error) {
	doc, err := fs.loadDoc(path)
	if err != nil {
		return nil, err
	}
	doc.Content = content
	doc.UpdatedAt = time.Now()
	if err := fs.saveDoc(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// --- Rename ---

func (fs *FileStore) RenameDocument(oldPath string, newPath string) (*domain.Document, error) {
	if err := validatePath(oldPath); err != nil {
		return nil, err
	}
	if err := validatePath(newPath); err != nil {
		return nil, err
	}
	if !fs.exists(oldPath) {
		return nil, domain.ErrDocumentNotFound
	}
	if fs.exists(newPath) {
		return nil, domain.ErrDocumentAlreadyExists
	}
	if err := fs.ensureParentDir(newPath); err != nil {
		return nil, err
	}
	if err := os.Rename(fs.fullPath(oldPath), fs.fullPath(newPath)); err != nil {
		return nil, err
	}
	return fs.loadDoc(newPath)
}

// --- Replace operations ---

func (fs *FileStore) mutateDoc(path string, fn func(*domain.Document) error) (*domain.Document, error) {
	doc, err := fs.loadDoc(path)
	if err != nil {
		return nil, err
	}
	if err := fn(doc); err != nil {
		return nil, err
	}
	doc.UpdatedAt = time.Now()
	if err := fs.saveDoc(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (fs *FileStore) ReplaceNth(path string, oldText string, newText string, n int, startLine int, endLine int) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.ReplaceNth(oldText, newText, n, startLine, endLine)
	})
}

func (fs *FileStore) ReplaceAll(path string, oldText string, newText string, startLine int, endLine int) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.ReplaceAll(oldText, newText, startLine, endLine)
	})
}

func (fs *FileStore) ReplaceRegex(path string, pattern string, replacement string, startLine int, endLine int) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.ReplaceRegex(pattern, replacement, startLine, endLine)
	})
}

// --- Line operations ---

func (fs *FileStore) InsertLines(path string, lineNum int, content string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.InsertLines(lineNum, content)
	})
}

func (fs *FileStore) MoveLines(path string, startLine int, endLine int, targetLine int) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.MoveLines(startLine, endLine, targetLine)
	})
}

func (fs *FileStore) DeleteLines(path string, startLine int, endLine int) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.DeleteLines(startLine, endLine)
	})
}

func (fs *FileStore) CopyLines(path string, startLine int, endLine int, targetLine int) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.CopyLines(startLine, endLine, targetLine)
	})
}

func (fs *FileStore) IndentLines(path string, startLine int, endLine int, levels int, prefix string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.IndentLines(startLine, endLine, levels, prefix)
	})
}

// --- Markdown operations ---

func (fs *FileStore) CheckCheckbox(path string, lineNum int, checked bool) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.CheckCheckbox(lineNum, checked)
	})
}

func (fs *FileStore) Renumber(path string, startLine int, endLine int, prefix string, start string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.Renumber(startLine, endLine, prefix, start)
	})
}

func (fs *FileStore) RenumberRegex(path string, startLine int, endLine int, pattern string, start string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.RenumberRegex(startLine, endLine, pattern, start)
	})
}

func (fs *FileStore) GetSection(path string, heading string) (string, int, int, error) {
	doc, err := fs.loadDoc(path)
	if err != nil {
		return "", 0, 0, err
	}
	return doc.GetSection(heading)
}

func (fs *FileStore) InsertAfterHeading(path string, heading string, content string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.InsertAfterHeading(heading, content)
	})
}

func (fs *FileStore) MoveSection(path string, heading string, targetHeading string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.MoveSection(heading, targetHeading)
	})
}

func (fs *FileStore) DeleteSection(path string, heading string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.DeleteSection(heading)
	})
}

func (fs *FileStore) InsertTableRow(path string, lineNum int, values []string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.InsertTableRow(lineNum, values)
	})
}

func (fs *FileStore) UpdateTableRow(path string, lineNum int, values []string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.UpdateTableRow(lineNum, values)
	})
}

func (fs *FileStore) DeleteTableRow(path string, lineNum int) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.DeleteTableRow(lineNum)
	})
}

func (fs *FileStore) AppendToDocument(path string, content string) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.Append(content)
	})
}

// --- Checkboxes ---

func (fs *FileStore) GetCheckboxes(path string, filter string, format string, startLine int, endLine int) (any, error) {
	doc, err := fs.loadDoc(path)
	if err != nil {
		return nil, err
	}
	if format == "tree" {
		return doc.GetCheckboxTree(filter, startLine, endLine)
	}
	return doc.GetCheckboxes(filter, startLine, endLine)
}

// --- Formatting ---

func (fs *FileStore) ApplyEdits(path string, edits []domain.Edit) (*domain.Document, []domain.EditResult, error) {
	doc, err := fs.loadDoc(path)
	if err != nil {
		return nil, nil, err
	}
	results, err := doc.ApplyEdits(edits)
	if err != nil {
		return nil, nil, err
	}
	if err := fs.saveDoc(doc); err != nil {
		return nil, nil, err
	}
	return doc, results, nil
}

func (fs *FileStore) FormatDocument(path string, startLine int, endLine int) (*domain.Document, error) {
	return fs.mutateDoc(path, func(doc *domain.Document) error {
		return doc.Format(startLine, endLine)
	})
}

// --- Cross-document operations ---

func (fs *FileStore) MoveLinesToDocument(srcPath string, startLine int, endLine int, destPath string, targetLine int) (*domain.Document, *domain.Document, error) {
	srcDoc, err := fs.loadDoc(srcPath)
	if err != nil {
		return nil, nil, err
	}

	content, err := srcDoc.GetLines(startLine, endLine)
	if err != nil {
		return nil, nil, err
	}

	if err := srcDoc.DeleteLines(startLine, endLine); err != nil {
		return nil, nil, err
	}

	destDoc, err := fs.loadDoc(destPath)
	if err != nil {
		return nil, nil, err
	}
	if err := destDoc.InsertLines(targetLine, content); err != nil {
		return nil, nil, err
	}

	srcDoc.UpdatedAt = time.Now()
	destDoc.UpdatedAt = time.Now()
	if err := fs.saveDoc(srcDoc); err != nil {
		return nil, nil, err
	}
	if err := fs.saveDoc(destDoc); err != nil {
		return nil, nil, err
	}

	return srcDoc, destDoc, nil
}

func (fs *FileStore) CopyLinesToDocument(srcPath string, startLine int, endLine int, destPath string, targetLine int) (*domain.Document, error) {
	srcDoc, err := fs.loadDoc(srcPath)
	if err != nil {
		return nil, err
	}

	content, err := srcDoc.GetLines(startLine, endLine)
	if err != nil {
		return nil, err
	}

	destDoc, err := fs.loadDoc(destPath)
	if err != nil {
		return nil, err
	}
	if err := destDoc.InsertLines(targetLine, content); err != nil {
		return nil, err
	}

	destDoc.UpdatedAt = time.Now()
	if err := fs.saveDoc(destDoc); err != nil {
		return nil, err
	}

	return destDoc, nil
}
