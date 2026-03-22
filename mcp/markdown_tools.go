package mcp

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/specboxhq/specbox/internal/domain"
)

func registerMarkdownTools(s *server.MCPServer, svc domain.DocumentService) {
	s.AddTool(
		mcp.NewTool("set_checkbox",
			mcp.WithDescription("Set a markdown checkbox checked or unchecked (- [ ] ↔ - [x]). Provide line_num OR text to identify the checkbox. When text is provided, finds the first checkbox line containing that text."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithNumber("line_num", mcp.Description("Line number of the checkbox (1-based). Optional if text is provided.")),
			mcp.WithString("text", mcp.Description("Text to match within a checkbox line. Finds and toggles the first checkbox containing this text.")),
			mcp.WithBoolean("checked", mcp.Required(), mcp.Description("true to check, false to uncheck")),
		),
		checkCheckboxHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("renumber_lines",
			mcp.WithDescription("Renumber or reletter matching lines in a range. Two modes: prefix mode (provide prefix) matches lines starting with that prefix; regex mode (provide pattern) replaces the first capture group. Zero-padding is preserved from start value width."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithNumber("start_line", mcp.Required(), mcp.Description("First line (1-based)")),
			mcp.WithNumber("end_line", mcp.Required(), mcp.Description("Last line (1-based, inclusive)")),
			mcp.WithString("start", mcp.Required(), mcp.Description("Starting number or letter (e.g. '1', 'a', 'A', '0023')")),
			mcp.WithString("prefix", mcp.Description("Line prefix to match (e.g. '- ', '## '). Use this OR pattern.")),
			mcp.WithString("pattern", mcp.Description("Regex pattern with a capture group for the number/letter to replace (e.g. 'Step (\\\\d{4})'). Use this OR prefix.")),
			mcp.WithBoolean("show_changes", mcp.Description("Include before/after for each renumbered line in the response (default false)")),
		),
		renumberHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("get_section",
			mcp.WithDescription("Read content under a markdown heading (up to next same-level heading). Prefer this over get_lines for reading structured markdown sections."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("heading", mcp.Required(), mcp.Description("Heading text (without # prefix)")),
		),
		getSectionHandler(svc),
	)

	// insert_text is registered in line_tools.go (consolidated from insert_lines + insert_after_heading)

	s.AddTool(
		mcp.NewTool("move_section",
			mcp.WithDescription("Move an entire markdown section (heading + body, including sub-sections) to after another heading. Prefer this over manual get_lines/delete_lines/insert_lines for restructuring documents."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("heading", mcp.Required(), mcp.Description("Heading of the section to move")),
			mcp.WithString("target_heading", mcp.Required(), mcp.Description("Heading to move the section after")),
		),
		moveSectionHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("delete_section",
			mcp.WithDescription("Remove an entire markdown section (heading + body). WARNING: also removes all sub-sections (lower-level headings) until the next same-level or higher heading. Use get_section first to preview what will be deleted. Prefer this over delete_lines for removing structured sections."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("heading", mcp.Required(), mcp.Description("Heading of the section to delete")),
			mcp.WithBoolean("show_changes", mcp.Description("Include deleted content in the response (default false)")),
		),
		deleteSectionHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("edit_table_row",
			mcp.WithDescription("Insert, update, or delete a markdown table row. Action: 'insert' (default) adds a row, 'update' replaces cell values at a line, 'delete' removes a row. Values are auto-formatted with pipes."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithNumber("line_num", mcp.Required(), mcp.Description("Line number of the row (1-based)")),
			mcp.WithString("action", mcp.Description("'insert' (default), 'update', or 'delete'")),
			mcp.WithArray("values", mcp.Description("Cell values for insert/update (not needed for delete)"), mcp.Items(map[string]any{"type": "string"})),
		),
		editTableRowHandler(svc),
	)

	// get_checkboxes
	s.AddTool(
		mcp.NewTool("get_checkboxes",
			mcp.WithDescription("Extract markdown checkboxes from a document. Filter by checked/unchecked status. Use tree format to see checkboxes grouped under their parent headings."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("filter", mcp.Description("Filter: 'all' (default), 'checked', or 'unchecked'")),
			mcp.WithString("format", mcp.Description("Output format: 'list' (default) or 'tree' (grouped by headings)")),
			mcp.WithNumber("start_line", mcp.Description("Restrict to lines starting from (1-based, 0=full doc)")),
			mcp.WithNumber("end_line", mcp.Description("Restrict to lines ending at (1-based, 0=full doc)")),
		),
		getCheckboxesHandler(svc),
	)
}

func checkCheckboxHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		lineNum := request.GetInt("line_num", 0)
		text, _ := request.RequireString("text")
		checked := request.GetBool("checked", false)

		// If text is provided, find the checkbox line
		if text != "" && lineNum == 0 {
			doc, err := svc.GetDocument(path)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			lines := strings.Split(doc.Content, "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				isCheckbox := strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]")
				if isCheckbox && strings.Contains(line, text) {
					lineNum = i + 1
					break
				}
			}
			if lineNum == 0 {
				return mcp.NewToolResultError("no checkbox found containing text: " + text), nil
			}
		}

		if lineNum == 0 {
			return mcp.NewToolResultError("either line_num or text is required"), nil
		}

		doc, err := svc.CheckCheckbox(path, lineNum, checked)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}

func renumberHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)
		start, err := request.RequireString("start")
		if err != nil {
			return mcp.NewToolResultError("start is required"), nil
		}
		showChanges := request.GetBool("show_changes", false)

		prefix, _ := request.RequireString("prefix")
		pattern, _ := request.RequireString("pattern")

		if prefix == "" && pattern == "" {
			return mcp.NewToolResultError("either prefix or pattern is required"), nil
		}

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
			doc, err = svc.RenumberRegex(path, startLine, endLine, pattern, start)
		} else {
			doc, err = svc.Renumber(path, startLine, endLine, prefix, start)
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

func getSectionHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		heading, err := request.RequireString("heading")
		if err != nil {
			return mcp.NewToolResultError("heading is required"), nil
		}
		content, startLine, endLine, err := svc.GetSection(path, heading)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result := map[string]any{
			"path":       path,
			"heading":    heading,
			"start_line": startLine,
			"end_line":   endLine,
			"content":    content,
		}
		return jsonResult(result)
	}
}

func moveSectionHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		heading, err := request.RequireString("heading")
		if err != nil {
			return mcp.NewToolResultError("heading is required"), nil
		}
		targetHeading, err := request.RequireString("target_heading")
		if err != nil {
			return mcp.NewToolResultError("target_heading is required"), nil
		}
		doc, err := svc.MoveSection(path, heading, targetHeading)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}

func deleteSectionHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		heading, err := request.RequireString("heading")
		if err != nil {
			return mcp.NewToolResultError("heading is required"), nil
		}
		showChanges := request.GetBool("show_changes", false)

		var beforeContent string
		if showChanges {
			beforeDoc, err := svc.GetDocument(path)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			beforeContent = beforeDoc.Content
		}

		doc, err := svc.DeleteSection(path, heading)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if showChanges {
			return jsonResult(docSummaryWithChanges(doc, beforeContent))
		}
		return jsonResult(docSummary(doc))
	}
}

func editTableRowHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		lineNum := request.GetInt("line_num", 0)
		action := request.GetString("action", "insert")
		values := request.GetStringSlice("values", nil)

		var doc *domain.Document
		switch action {
		case "insert":
			if len(values) == 0 {
				return mcp.NewToolResultError("values is required for insert"), nil
			}
			doc, err = svc.InsertTableRow(path, lineNum, values)
		case "update":
			if len(values) == 0 {
				return mcp.NewToolResultError("values is required for update"), nil
			}
			doc, err = svc.UpdateTableRow(path, lineNum, values)
		case "delete":
			doc, err = svc.DeleteTableRow(path, lineNum)
		default:
			return mcp.NewToolResultError("invalid action: must be 'insert', 'update', or 'delete'"), nil
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docSummary(doc))
	}
}

func getCheckboxesHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		filter := request.GetString("filter", "all")
		format := request.GetString("format", "list")
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)

		result, err := svc.GetCheckboxes(path, filter, format, startLine, endLine)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}
