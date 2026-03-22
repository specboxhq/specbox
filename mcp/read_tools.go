package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/specboxhq/specbox/internal/domain"
)

func registerReadTools(s *server.MCPServer, svc domain.DocumentService) {
	s.AddTool(
		mcp.NewTool("list_documents",
			mcp.WithDescription("List documents, optionally filtered by path substring. Paths can be any depth under the specs folder."),
			mcp.WithString("filter",
				mcp.Description("Substring filter on document paths (e.g. 'api/' to list docs in the api folder)"),
			),
		),
		listDocumentsHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("search_documents",
			mcp.WithDescription("Full-text search across all document content. Use optional context_lines to see surrounding lines for each match, like grep -C. Set regex=true for pattern matching."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Text or regex pattern to search for"),
			),
			mcp.WithBoolean("regex",
				mcp.Description("Treat query as a regex pattern (default false)"),
			),
			mcp.WithNumber("context_lines",
				mcp.Description("Number of lines to include before/after each match (default 0)"),
			),
		),
		searchDocumentsHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("get_document",
			mcp.WithDescription("Read a document by path, returning full content and metadata. For large documents, prefer get_lines or get_section to read specific parts."),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Document path relative to docs root"),
			),
		),
		getDocumentHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("get_lines",
			mcp.WithDescription("Read a specific line range from a document (1-based, inclusive). Omit end_line to read to end of document. Use get_table_of_contents or find_line first to locate the lines you need."),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Document path"),
			),
			mcp.WithNumber("start_line",
				mcp.Required(),
				mcp.Description("Start line number (1-based)"),
			),
			mcp.WithNumber("end_line",
				mcp.Description("End line number (1-based, inclusive). Omit to read to end of document."),
			),
			mcp.WithBoolean("show_line_numbers",
				mcp.Description("Prefix each line with its line number (default false)"),
			),
		),
		getLinesHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("find_text",
			mcp.WithDescription("Find all line numbers where text appears in a document. Use context_lines to see surrounding content without a follow-up get_lines call. Set regex=true for pattern matching."),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Document path"),
			),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Text or regex pattern to search for within the document"),
			),
			mcp.WithBoolean("regex",
				mcp.Description("Treat query as a regex pattern (default false)"),
			),
			mcp.WithNumber("context_lines",
				mcp.Description("Number of lines to include before/after each match (default 0)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of matches to return (default 0 = all)"),
			),
		),
		findLineHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("get_table_of_contents",
			mcp.WithDescription("Return all markdown headings with line numbers and levels. Use this to understand document structure before using get_section, move_section, or line-based operations."),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Document path"),
			),
		),
		getTableOfContentsHandler(svc),
	)
}

func listDocumentsHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filter := request.GetString("filter", "")

		docs, err := svc.ListDocuments(filter)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		type docEntry struct {
			Path      string `json:"path"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		}
		entries := make([]docEntry, len(docs))
		for i, doc := range docs {
			entries[i] = docEntry{
				Path:      doc.Path,
				CreatedAt: doc.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt: doc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			}
		}

		return jsonResult(entries)
	}
}

func searchDocumentsHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query is required"), nil
		}

		useRegex := request.GetBool("regex", false)
		contextLines := request.GetInt("context_lines", 0)

		var results []domain.SearchResult
		if useRegex {
			results, err = svc.SearchDocumentsRegex(query, contextLines)
		} else {
			results, err = svc.SearchDocuments(query, contextLines)
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		type searchEntry struct {
			Path          string   `json:"path"`
			LineNumber    int      `json:"line_number"`
			LineContent   string   `json:"line_content"`
			ContextBefore []string `json:"context_before,omitempty"`
			ContextAfter  []string `json:"context_after,omitempty"`
		}
		entries := make([]searchEntry, len(results))
		for i, r := range results {
			entries[i] = searchEntry{
				Path:          r.Path,
				LineNumber:    r.LineNumber,
				LineContent:   r.LineContent,
				ContextBefore: r.ContextBefore,
				ContextAfter:  r.ContextAfter,
			}
		}

		return jsonResult(entries)
	}
}

func getDocumentHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}

		doc, err := svc.GetDocument(path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result := map[string]any{
			"path":       doc.Path,
			"content":    doc.Content,
			"line_count": doc.GetLineCount(),
			"created_at": doc.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"updated_at": doc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}

		return jsonResult(result)
	}
}

func getLinesHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)

		// Resolve endLine=0 to EOF
		if endLine == 0 {
			doc, err := svc.GetDocument(path)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			endLine = doc.GetLineCount()
		}

		content, err := svc.GetLines(path, startLine, endLine)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		showLineNumbers := request.GetBool("show_line_numbers", false)
		if showLineNumbers {
			lines := strings.Split(content, "\n")
			numbered := make([]string, len(lines))
			for i, line := range lines {
				numbered[i] = fmt.Sprintf("%d\t%s", startLine+i, line)
			}
			content = strings.Join(numbered, "\n")
		}

		result := map[string]any{
			"path":       path,
			"start_line": startLine,
			"end_line":   endLine,
			"content":    content,
		}

		return jsonResult(result)
	}
}

func findLineHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query is required"), nil
		}

		useRegex := request.GetBool("regex", false)
		contextLines := request.GetInt("context_lines", 0)
		limit := request.GetInt("limit", 0)

		var lines []int
		if useRegex {
			lines, err = svc.FindLineRegex(path, query)
		} else {
			lines, err = svc.FindLine(path, query)
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if limit > 0 && len(lines) > limit {
			lines = lines[:limit]
		}

		return findLineResult(svc, path, lines, contextLines)
	}
}

func findLineResult(svc domain.DocumentService, path string, lines []int, contextLines int) (*mcp.CallToolResult, error) {
	if contextLines <= 0 {
		result := map[string]any{
			"path":  path,
			"lines": lines,
		}
		return jsonResult(result)
	}

	// Fetch document to extract context
	doc, err := svc.GetDocument(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	docLines := strings.Split(doc.Content, "\n")
	lineCount := len(docLines)

	type matchEntry struct {
		Line          int      `json:"line"`
		Content       string   `json:"content"`
		ContextBefore []string `json:"context_before,omitempty"`
		ContextAfter  []string `json:"context_after,omitempty"`
	}
	matches := make([]matchEntry, len(lines))
	for i, lineNum := range lines {
		idx := lineNum - 1
		entry := matchEntry{
			Line:    lineNum,
			Content: docLines[idx],
		}
		start := idx - contextLines
		if start < 0 {
			start = 0
		}
		end := idx + contextLines
		if end >= lineCount {
			end = lineCount - 1
		}
		entry.ContextBefore = docLines[start:idx]
		if idx+1 <= end {
			entry.ContextAfter = docLines[idx+1 : end+1]
		}
		matches[i] = entry
	}

	result := map[string]any{
		"path":    path,
		"matches": matches,
	}
	return jsonResult(result)
}

func getTableOfContentsHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}

		toc, err := svc.GetTableOfContents(path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		type tocEntry struct {
			Heading    string `json:"heading"`
			Level      int    `json:"level"`
			LineNumber int    `json:"line_number"`
		}
		entries := make([]tocEntry, len(toc))
		for i, h := range toc {
			entries[i] = tocEntry{
				Heading:    h.Heading,
				Level:      h.Level,
				LineNumber: h.LineNumber,
			}
		}

		return jsonResult(entries)
	}
}
