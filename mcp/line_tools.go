package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/specboxhq/specbox/internal/domain"
)

func registerLineTools(s *server.MCPServer, svc domain.DocumentService) {
	s.AddTool(
		mcp.NewTool("insert_text",
			mcp.WithDescription("Insert content at a position in a document. Anchor by line_num, heading, or text match. Use position 'end' with no anchor to append. Position defaults to 'after'."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Content to insert")),
			mcp.WithString("position", mcp.Description("'before', 'after' (default), or 'end' (append to document, no anchor needed)")),
			mcp.WithNumber("line_num", mcp.Description("Anchor by line number (1-based)")),
			mcp.WithString("heading", mcp.Description("Anchor by markdown heading text (without # prefix)")),
			mcp.WithString("text", mcp.Description("Anchor by first line containing this text")),
		),
		insertTextHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("move_lines",
			mcp.WithDescription("Move a line range to a new position. Provide dest_path to move to another document. For moving markdown sections by heading, prefer move_section instead."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Source document path")),
			mcp.WithNumber("start_line", mcp.Required(), mcp.Description("First line to move (1-based)")),
			mcp.WithNumber("end_line", mcp.Required(), mcp.Description("Last line to move (1-based, inclusive)")),
			mcp.WithNumber("target_line", mcp.Required(), mcp.Description("Line to insert the block before")),
			mcp.WithString("dest_path", mcp.Description("Destination document path. If omitted, moves within the same document.")),
		),
		moveLinesHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("delete_lines",
			mcp.WithDescription("Remove a line range from a document. For removing a markdown section by heading, prefer delete_section instead."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithNumber("start_line", mcp.Required(), mcp.Description("First line to delete (1-based)")),
			mcp.WithNumber("end_line", mcp.Required(), mcp.Description("Last line to delete (1-based, inclusive)")),
			mcp.WithBoolean("show_changes", mcp.Description("Include deleted lines in the response (default false)")),
		),
		deleteLinesHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("copy_lines",
			mcp.WithDescription("Duplicate a line range to a new position. Provide dest_path to copy to another document (source unchanged)."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Source document path")),
			mcp.WithNumber("start_line", mcp.Required(), mcp.Description("First line to copy (1-based)")),
			mcp.WithNumber("end_line", mcp.Required(), mcp.Description("Last line to copy (1-based, inclusive)")),
			mcp.WithNumber("target_line", mcp.Required(), mcp.Description("Line to insert the copy before")),
			mcp.WithString("dest_path", mcp.Description("Destination document path. If omitted, copies within the same document.")),
		),
		copyLinesHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("indent_lines",
			mcp.WithDescription("Indent or outdent a line range. Positive levels = indent, negative = outdent."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithNumber("start_line", mcp.Required(), mcp.Description("First line (1-based)")),
			mcp.WithNumber("end_line", mcp.Required(), mcp.Description("Last line (1-based, inclusive)")),
			mcp.WithNumber("levels", mcp.Required(), mcp.Description("Number of levels to indent (positive) or outdent (negative)")),
			mcp.WithString("prefix", mcp.Required(), mcp.Description("Indent string per level (e.g. two spaces, a tab)")),
		),
		indentLinesHandler(svc),
	)
}

func insertTextHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		content, err := request.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError("content is required"), nil
		}
		position := request.GetString("position", "after")
		lineNum := request.GetInt("line_num", 0)
		heading, _ := request.RequireString("heading")
		text, _ := request.RequireString("text")

		// Handle "end" position — append to document
		if position == "end" {
			doc, err := svc.AppendToDocument(path, content)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(docSummary(doc))
		}

		if lineNum == 0 && heading == "" && text == "" {
			return mcp.NewToolResultError("one of line_num, heading, or text is required (or use position: 'end')"), nil
		}

		// Resolve anchor to a line number
		if heading != "" {
			_, startLine, _, err := svc.GetSection(path, heading)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			lineNum = startLine
		} else if text != "" {
			lines, err := svc.FindLine(path, text)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(lines) == 0 {
				return mcp.NewToolResultError("no line found containing text: " + text), nil
			}
			lineNum = lines[0]
		}

		// Adjust for position
		if position == "after" {
			lineNum = lineNum + 1
		}

		doc, err := svc.InsertLines(path, lineNum, content)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}

func moveLinesHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)
		targetLine := request.GetInt("target_line", 0)
		destPath, _ := request.RequireString("dest_path")

		if destPath != "" {
			// Cross-document move
			src, dest, err := svc.MoveLinesToDocument(path, startLine, endLine, destPath, targetLine)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result := map[string]any{
				"src":  docSummary(src),
				"dest": docSummary(dest),
			}
			return jsonResult(result)
		}

		// Same-document move
		doc, err := svc.MoveLines(path, startLine, endLine, targetLine)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}

func deleteLinesHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)
		showChanges := request.GetBool("show_changes", false)

		var beforeContent string
		if showChanges {
			beforeDoc, err := svc.GetDocument(path)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			beforeContent = beforeDoc.Content
		}

		doc, err := svc.DeleteLines(path, startLine, endLine)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if showChanges {
			return jsonResult(docSummaryWithChanges(doc, beforeContent))
		}
		return jsonResult(docSummary(doc))
	}
}

func copyLinesHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)
		targetLine := request.GetInt("target_line", 0)
		destPath, _ := request.RequireString("dest_path")

		if destPath != "" {
			// Cross-document copy
			dest, err := svc.CopyLinesToDocument(path, startLine, endLine, destPath, targetLine)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result := map[string]any{
				"dest": docSummary(dest),
			}
			return jsonResult(result)
		}

		// Same-document copy
		doc, err := svc.CopyLines(path, startLine, endLine, targetLine)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}

func indentLinesHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)
		levels := request.GetInt("levels", 0)
		prefix, err := request.RequireString("prefix")
		if err != nil {
			return mcp.NewToolResultError("prefix is required"), nil
		}
		doc, err := svc.IndentLines(path, startLine, endLine, levels, prefix)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}
