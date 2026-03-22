package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/specboxhq/specbox/internal/domain"
)

func registerMutationTools(s *server.MCPServer, svc domain.DocumentService) {
	// save_document (consolidated create/save/update)
	s.AddTool(
		mcp.NewTool("save_document",
			mcp.WithDescription("Save a document. Mode controls behavior: 'upsert' (default) creates or updates, 'create' errors if exists, 'update' errors if not found."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path (project/filename)")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Document content")),
			mcp.WithString("mode", mcp.Description("Save mode: 'upsert' (default), 'create' (error if exists), 'update' (error if not found)")),
		),
		saveDocumentHandler(svc),
	)

	// replace (consolidated replace_nth/replace_all/replace_regex)
	s.AddTool(
		mcp.NewTool("replace_text",
			mcp.WithDescription("Find and replace text in a document. Use old_text for literal matching OR pattern for regex. Set n to replace a specific occurrence (default 0 = all). Supports capture groups ($1, $2) and (?i) for regex."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("new_text", mcp.Required(), mcp.Description("Replacement text")),
			mcp.WithString("old_text", mcp.Description("Literal text to find. Use this OR pattern.")),
			mcp.WithString("pattern", mcp.Description("Regex pattern to find. Use this OR old_text.")),
			mcp.WithNumber("n", mcp.Description("Which occurrence to replace (0 = all, 1 = first, 2 = second, etc. Default 0)")),
			mcp.WithNumber("start_line", mcp.Description("Restrict to lines starting from (1-based, 0=full doc)")),
			mcp.WithNumber("end_line", mcp.Description("Restrict to lines ending at (1-based, 0=full doc)")),
			mcp.WithBoolean("show_changes", mcp.Description("Include before/after for each changed line in the response (default false)")),
		),
		replaceHandler(svc),
	)

	// move_document
	s.AddTool(
		mcp.NewTool("move_document",
			mcp.WithDescription("Move or rename a document. Can move between projects (e.g. 'old-proj/doc.md' to 'new-proj/doc.md')."),
			mcp.WithString("old_path", mcp.Required(), mcp.Description("Current document path")),
			mcp.WithString("new_path", mcp.Required(), mcp.Description("New document path")),
		),
		renameDocumentHandler(svc),
	)

	// format_document
	s.AddTool(
		mcp.NewTool("format_document",
			mcp.WithDescription("Format a document or line range. All files: removes trailing whitespace, collapses consecutive blank lines. Markdown files (.md, .mdown, .markdown, .mkd, .mdwn, .mdx): also aligns tables and normalizes heading spacing."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithNumber("start_line", mcp.Description("Restrict formatting to lines starting from (1-based, 0=full doc)")),
			mcp.WithNumber("end_line", mcp.Description("Restrict formatting to lines ending at (1-based, 0=full doc)")),
		),
		formatDocumentHandler(svc),
	)
}

func saveDocumentHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		content, err := request.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError("content is required"), nil
		}
		mode := request.GetString("mode", "upsert")

		var doc *domain.Document
		switch mode {
		case "create":
			doc, err = svc.CreateDocument(path, content)
		case "update":
			doc, err = svc.UpdateDocument(path, content)
		case "upsert":
			doc, err = svc.SaveDocument(path, content)
		default:
			return mcp.NewToolResultError("invalid mode: must be 'upsert', 'create', or 'update'"), nil
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}

func replaceHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		newText, err := request.RequireString("new_text")
		if err != nil {
			return mcp.NewToolResultError("new_text is required"), nil
		}

		oldText, _ := request.RequireString("old_text")
		pattern, _ := request.RequireString("pattern")
		n := request.GetInt("n", 0)
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)
		showChanges := request.GetBool("show_changes", false)

		if oldText == "" && pattern == "" {
			return mcp.NewToolResultError("either old_text or pattern is required"), nil
		}

		// Capture before content for show_changes
		var beforeContent string
		if showChanges {
			beforeDoc, err := svc.GetDocument(path)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			beforeContent = beforeDoc.Content
		}

		var doc *domain.Document
		if pattern != "" {
			doc, err = svc.ReplaceRegex(path, pattern, newText, startLine, endLine)
		} else if n == 0 {
			doc, err = svc.ReplaceAll(path, oldText, newText, startLine, endLine)
		} else {
			doc, err = svc.ReplaceNth(path, oldText, newText, n, startLine, endLine)
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if showChanges {
			return jsonResult(docSummaryWithChanges(doc, beforeContent))
		}
		return jsonResult(docSummary(doc))
	}
}

func renameDocumentHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		oldPath, err := request.RequireString("old_path")
		if err != nil {
			return mcp.NewToolResultError("old_path is required"), nil
		}
		newPath, err := request.RequireString("new_path")
		if err != nil {
			return mcp.NewToolResultError("new_path is required"), nil
		}
		doc, err := svc.RenameDocument(oldPath, newPath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}

func formatDocumentHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)

		doc, err := svc.FormatDocument(path, startLine, endLine)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}
