package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/specboxhq/specbox/internal/api"
	"github.com/specboxhq/specbox/internal/config"
	"github.com/specboxhq/specbox/internal/domain"
)

func registerSpecboxTools(s *server.MCPServer, svc domain.DocumentService) {
	// add_markup
	s.AddTool(
		mcp.NewTool("add_markup",
			mcp.WithDescription("Add a specbox markup annotation to a document. Use start_line or heading for standalone (block) mode. Use start_line + end_line to wrap existing content."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("type", mcp.Required(), mcp.Description("Markup type. Currently only 'question' is supported.")),
			mcp.WithString("question", mcp.Description("The question or topic text")),
			mcp.WithString("decision_type", mcp.Description("For decisions: choice, multi, yesno, number, date, ordered, approval, range, url, email, color, file. Written as type= in the markup.")),
			mcp.WithArray("options", mcp.Description("List of options for choice/multi/ordered. Written as pipe-separated options= attribute. Not needed if wrapped content has list items."), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithBoolean("other", mcp.Description("Include 'Other' freeform input for choice/multi/ordered (default true). Set false to restrict to listed options only.")),
			mcp.WithNumber("start_line", mcp.Description("Line number to insert at (1-based)")),
			mcp.WithNumber("end_line", mcp.Description("Last line to wrap (1-based). If provided with start_line, creates wrapped mode.")),
			mcp.WithString("heading", mcp.Description("Insert after this heading (alternative to start_line)")),
		),
		addMarkupHandler(svc),
	)

	// update_markup
	s.AddTool(
		mcp.NewTool("update_markup",
			mcp.WithDescription("Update an existing specbox markup by ID. Only provided fields are changed."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("id", mcp.Required(), mcp.Description("Markup ID (10-char alphanumeric)")),
			mcp.WithString("question", mcp.Description("Updated question text")),
			mcp.WithString("status", mcp.Description("Updated status (e.g. 'open', 'resolved')")),
			mcp.WithString("decision_type", mcp.Description("Updated decision type")),
			mcp.WithString("answer", mcp.Description("Answer for resolved decisions")),
			mcp.WithString("response", mcp.Description("Response for resolved feedback/questions")),
			mcp.WithArray("options", mcp.Description("Updated options list"), mcp.Items(map[string]any{"type": "string"})),
		),
		updateMarkupHandler(svc),
	)

	// delete_markup
	s.AddTool(
		mcp.NewTool("delete_markup",
			mcp.WithDescription("Remove a specbox markup by ID. For wrapped markups, removes tags but keeps the wrapped content."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("id", mcp.Required(), mcp.Description("Markup ID (10-char alphanumeric)")),
		),
		deleteMarkupHandler(svc),
	)

	// list_markups
	s.AddTool(
		mcp.NewTool("list_markups",
			mcp.WithDescription("List all specbox markups in a document. Filter by status and/or type."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithArray("status", mcp.Description("Status filter: e.g. ['open'], ['open', 'resolved']. Default: all statuses."), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithArray("types", mcp.Description("Type filter: e.g. ['decision'], ['question', 'feedback']. Default: all types."), mcp.Items(map[string]any{"type": "string"})),
		),
		listMarkupsHandler(svc),
	)

	// resolve_markup
	s.AddTool(
		mcp.NewTool("resolve_markup",
			mcp.WithDescription("Mark a specbox markup as resolved with an answer or response."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("id", mcp.Required(), mcp.Description("Markup ID (10-char alphanumeric)")),
			mcp.WithString("answer", mcp.Description("Answer text (for decisions)")),
			mcp.WithString("response", mcp.Description("Response text (for feedback/questions)")),
		),
		resolveMarkupHandler(svc),
	)

	// check_spec
	s.AddTool(
		mcp.NewTool("check_spec",
			mcp.WithDescription("Check a spec document: sync status, frontmatter validation, and markup summary."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
		),
		checkSpecHandler(svc),
	)

	// push_spec
	s.AddTool(
		mcp.NewTool("push_spec",
			mcp.WithDescription("Push a spec to specbox.io. Sends content and optional metadata, returns updated frontmatter. Requires specbox login."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithObject("metadata", mcp.Description("Optional metadata fields to set (status, priority, tags, visibility, locked, due_date, owner)")),
		),
		pushSpecHandler(svc),
	)

	// pull_spec
	s.AddTool(
		mcp.NewTool("pull_spec",
			mcp.WithDescription("Pull a spec from specbox.io. Fetches updated frontmatter and responses. Requires specbox login."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
		),
		pullSpecHandler(svc),
	)
}

// addMarkupHandler creates and inserts a specbox markup into a document.
func addMarkupHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		mType, err := request.RequireString("type")
		if err != nil {
			return mcp.NewToolResultError("type is required"), nil
		}
		if mType != "question" {
			return mcp.NewToolResultError("type must be 'question'"), nil
		}

		question, _ := request.RequireString("question")
		decisionType, _ := request.RequireString("decision_type")
		options := parseStringSliceParam(request, "options")
		other := request.GetBool("other", true)
		startLine := request.GetInt("start_line", 0)
		endLine := request.GetInt("end_line", 0)
		heading, _ := request.RequireString("heading")

		id := domain.GenerateID()

		// Resolve heading to a line number
		if heading != "" {
			_, headingLine, _, err := svc.GetSection(path, heading)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			startLine = headingLine + 1 // insert after the heading
		}

		if startLine == 0 {
			return mcp.NewToolResultError("one of start_line or heading is required"), nil
		}

		if startLine > 0 && endLine > 0 {
			// Wrapped mode: insert opening tag before start_line, closing tag after end_line
			openTag := fmt.Sprintf("<!-- specbox:%s:%s", mType, id)
			var parts []string
			parts = append(parts, "status=\"open\"")
			if decisionType != "" {
				parts = append(parts, fmt.Sprintf("type=\"%s\"", decisionType))
			}
			if len(options) > 0 {
				parts = append(parts, fmt.Sprintf("options=\"%s\"", strings.Join(options, "|")))
			}
			if !other {
				parts = append(parts, "other=\"false\"")
			}
			openTag += " " + strings.Join(parts, " ") + " -->"
			closeTag := fmt.Sprintf("<!-- /specbox:%s -->", mType)

			// Insert closing tag first (after end_line), then opening tag (before start_line)
			// so line numbers don't shift for the second insert
			doc, err := svc.InsertLines(path, endLine+1, closeTag)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			doc, err = svc.InsertLines(path, startLine, openTag)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			result := docSummary(doc)
			result["markup_id"] = id
			result["mode"] = "wrapped"
			return jsonResult(result)
		}

		// Standalone (block) mode: generate a YAML block comment
		var blockLines []string
		openTag := fmt.Sprintf("<!-- specbox:%s:%s status=\"open\"", mType, id)
		if decisionType != "" {
			openTag += fmt.Sprintf(" type=\"%s\"", decisionType)
		}
		if !other {
			openTag += " other=\"false\""
		}
		blockLines = append(blockLines, openTag)
		if question != "" {
			blockLines = append(blockLines, fmt.Sprintf("question: %s", question))
		}
		if len(options) > 0 {
			blockLines = append(blockLines, "options:")
			for _, opt := range options {
				blockLines = append(blockLines, fmt.Sprintf("  - %s", opt))
			}
		}
		blockLines = append(blockLines, "-->")

		content := strings.Join(blockLines, "\n")
		doc, err := svc.InsertLines(path, startLine, content)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result := docSummary(doc)
		result["markup_id"] = id
		result["mode"] = "block"
		return jsonResult(result)
	}
}

// updateMarkupHandler updates an existing markup by ID.
func updateMarkupHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError("id is required"), nil
		}

		doc, err := svc.GetDocument(path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		m := domain.FindMarkupByID(doc.Content, id)
		if m == nil {
			return mcp.NewToolResultError(fmt.Sprintf("markup not found: %s", id)), nil
		}

		// Collect updates
		question, _ := request.RequireString("question")
		status, _ := request.RequireString("status")
		decisionType, _ := request.RequireString("decision_type")
		answer, _ := request.RequireString("answer")
		response, _ := request.RequireString("response")
		options := parseStringSliceParam(request, "options")

		if status != "" {
			m.Status = status
		}
		if decisionType != "" {
			if m.Attrs == nil {
				m.Attrs = make(map[string]string)
			}
			m.Attrs["type"] = decisionType
		}
		if answer != "" {
			if m.Attrs == nil {
				m.Attrs = make(map[string]string)
			}
			m.Attrs["answer"] = answer
		}
		if response != "" {
			if m.Attrs == nil {
				m.Attrs = make(map[string]string)
			}
			m.Attrs["response"] = response
		}

		// Rebuild the markup text
		newMarkup := rebuildMarkup(m, question, options)

		// Replace old lines with new
		lines := strings.Split(doc.Content, "\n")
		var newLines []string
		newLines = append(newLines, lines[:m.StartLine-1]...)
		newLines = append(newLines, strings.Split(newMarkup, "\n")...)
		newLines = append(newLines, lines[m.EndLine:]...)

		updatedDoc, err := svc.UpdateDocument(path, strings.Join(newLines, "\n"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(docSummary(updatedDoc))
	}
}

// rebuildMarkup generates markup text from a Markup struct.
// parseStringSliceParam gets a string slice from the request, with fallback for JSON string format.
func parseStringSliceParam(request mcp.CallToolRequest, key string) []string {
	result := request.GetStringSlice(key, nil)
	if result == nil {
		if s, _ := request.RequireString(key); s != "" {
			s = strings.TrimPrefix(s, "[")
			s = strings.TrimSuffix(s, "]")
			for _, item := range strings.Split(s, ",") {
				item = strings.TrimSpace(item)
				item = strings.Trim(item, "\"")
				if item != "" {
					result = append(result, item)
				}
			}
		}
	}
	return result
}

func rebuildMarkup(m *domain.Markup, question string, options []string) string {
	// Store options as pipe-separated attribute if provided
	if len(options) > 0 {
		if m.Attrs == nil {
			m.Attrs = make(map[string]string)
		}
		m.Attrs["options"] = strings.Join(options, "|")
	}

	// Build the attribute string
	var attrParts []string
	if m.Status != "" {
		attrParts = append(attrParts, fmt.Sprintf("status=\"%s\"", m.Status))
	}
	for k, v := range m.Attrs {
		attrParts = append(attrParts, fmt.Sprintf("%s=\"%s\"", k, v))
	}
	attrStr := ""
	if len(attrParts) > 0 {
		attrStr = " " + strings.Join(attrParts, " ")
	}

	switch m.Mode {
	case domain.MarkupInline:
		content := m.Content
		if question != "" {
			content = question
		}
		return fmt.Sprintf("<!-- specbox:%s:%s%s %s -->", m.Type, m.ID, attrStr, content)

	case domain.MarkupWrapped:
		openTag := fmt.Sprintf("<!-- specbox:%s:%s%s -->", m.Type, m.ID, attrStr)
		closeTag := fmt.Sprintf("<!-- /specbox:%s -->", m.Type)
		content := m.Content
		return openTag + "\n" + content + "\n" + closeTag

	case domain.MarkupBlock:
		openTag := fmt.Sprintf("<!-- specbox:%s:%s%s", m.Type, m.ID, attrStr)
		var blockLines []string
		blockLines = append(blockLines, openTag)
		if question != "" {
			blockLines = append(blockLines, fmt.Sprintf("question: %s", question))
		} else {
			// Preserve existing content lines that start with known keys
			contentLines := strings.Split(m.Content, "\n")
			for _, cl := range contentLines {
				if strings.TrimSpace(cl) != "" {
					blockLines = append(blockLines, cl)
				}
			}
		}
		if len(options) > 0 {
			// Remove existing options from blockLines
			var filtered []string
			inOptions := false
			for _, bl := range blockLines {
				trimmed := strings.TrimSpace(bl)
				if trimmed == "options:" {
					inOptions = true
					continue
				}
				if inOptions && strings.HasPrefix(trimmed, "- ") {
					continue
				}
				inOptions = false
				filtered = append(filtered, bl)
			}
			blockLines = filtered
			blockLines = append(blockLines, "options:")
			for _, opt := range options {
				blockLines = append(blockLines, fmt.Sprintf("  - %s", opt))
			}
		}
		blockLines = append(blockLines, "-->")
		return strings.Join(blockLines, "\n")
	}
	return ""
}

// deleteMarkupHandler removes a markup by ID.
func deleteMarkupHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError("id is required"), nil
		}

		doc, err := svc.GetDocument(path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		m := domain.FindMarkupByID(doc.Content, id)
		if m == nil {
			return mcp.NewToolResultError(fmt.Sprintf("markup not found: %s", id)), nil
		}

		lines := strings.Split(doc.Content, "\n")
		var newLines []string

		switch m.Mode {
		case domain.MarkupWrapped:
			// Remove opening and closing tags but keep content between them
			newLines = append(newLines, lines[:m.StartLine-1]...)
			// Keep everything between start and end (the wrapped content)
			if m.StartLine < m.EndLine-1 {
				newLines = append(newLines, lines[m.StartLine:m.EndLine-1]...)
			}
			newLines = append(newLines, lines[m.EndLine:]...)
		default:
			// Remove all lines from start to end (inline or block)
			newLines = append(newLines, lines[:m.StartLine-1]...)
			newLines = append(newLines, lines[m.EndLine:]...)
		}

		updatedDoc, err := svc.UpdateDocument(path, strings.Join(newLines, "\n"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(docSummary(updatedDoc))
	}
}

// listMarkupsHandler lists all markups in a document.
func listMarkupsHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		statuses := parseStringSliceParam(request, "status")
		types := parseStringSliceParam(request, "types")

		doc, err := svc.GetDocument(path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		markups := domain.ParseMarkups(doc.Content)

		// Build type set for filtering
		statusSet := make(map[string]bool)
		for _, s := range statuses {
			statusSet[s] = true
		}
		typeSet := make(map[string]bool)
		for _, t := range types {
			typeSet[t] = true
		}

		// Apply filters
		var filtered []domain.Markup
		for _, m := range markups {
			if len(statusSet) > 0 && !statusSet[m.Status] {
				continue
			}
			if len(typeSet) > 0 && !typeSet[string(m.Type)] {
				continue
			}
			filtered = append(filtered, m)
		}

		// Build result
		type markupResult struct {
			ID           string   `json:"id"`
			Type         string   `json:"type"`
			Mode         string   `json:"mode"`
			Status       string   `json:"status"`
			DecisionType string   `json:"decision_type,omitempty"`
			Options      []string `json:"options,omitempty"`
			Other        bool     `json:"other"`
			StartLine    int      `json:"start_line"`
			EndLine      int      `json:"end_line"`
			Content      string   `json:"content,omitempty"`
		}
		var results []markupResult
		for _, m := range filtered {
			r := markupResult{
				ID:        m.ID,
				Type:      string(m.Type),
				Mode:      string(m.Mode),
				Status:    m.Status,
				Other:     true, // default
				StartLine: m.StartLine,
				EndLine:   m.EndLine,
				Content:   m.Content,
			}
			// Extract attrs into structured fields
			if m.Attrs != nil {
				if dt, ok := m.Attrs["type"]; ok {
					r.DecisionType = dt
				}
				if opts, ok := m.Attrs["options"]; ok {
					r.Options = strings.Split(opts, "|")
				}
				if other, ok := m.Attrs["other"]; ok && other == "false" {
					r.Other = false
				}
			}
			results = append(results, r)
		}

		return jsonResult(map[string]any{
			"path":    path,
			"count":   len(results),
			"markups": results,
		})
	}
}

// resolveMarkupHandler marks a markup as resolved.
func resolveMarkupHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError("id is required"), nil
		}

		answer, _ := request.RequireString("answer")
		response, _ := request.RequireString("response")

		doc, err := svc.GetDocument(path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		m := domain.FindMarkupByID(doc.Content, id)
		if m == nil {
			return mcp.NewToolResultError(fmt.Sprintf("markup not found: %s", id)), nil
		}

		// Set status to resolved
		m.Status = "resolved"
		if m.Attrs == nil {
			m.Attrs = make(map[string]string)
		}
		if answer != "" {
			m.Attrs["answer"] = answer
		}
		if response != "" {
			m.Attrs["response"] = response
		}

		// Rebuild
		newMarkup := rebuildMarkup(m, "", nil)
		lines := strings.Split(doc.Content, "\n")
		var newLines []string
		newLines = append(newLines, lines[:m.StartLine-1]...)
		newLines = append(newLines, strings.Split(newMarkup, "\n")...)
		newLines = append(newLines, lines[m.EndLine:]...)

		updatedDoc, err := svc.UpdateDocument(path, strings.Join(newLines, "\n"))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(docSummary(updatedDoc))
	}
}

// checkSpecHandler checks a spec document status.
func checkSpecHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}

		doc, err := svc.GetDocument(path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Parse frontmatter
		fmData, _, fmErr := domain.ParseFrontmatter(doc.Content)

		// Parse markups
		markups := domain.ParseMarkups(doc.Content)
		openCount := 0
		resolvedCount := 0
		for _, m := range markups {
			if m.Status == "resolved" {
				resolvedCount++
			} else {
				openCount++
			}
		}

		// Determine sync status
		syncStatus := "never_pushed"
		if fmData != nil {
			if specbox, ok := fmData["specbox"]; ok {
				if sbMap, ok := specbox.(map[string]any); ok {
					if _, ok := sbMap["url"]; ok {
						syncStatus = "up_to_date" // simplified — real impl would compare hashes
					}
				}
			}
		}

		// Frontmatter validation
		fmStatus := "valid"
		var fmErrors []string
		if fmErr != nil {
			fmStatus = "invalid"
			fmErrors = append(fmErrors, fmErr.Error())
		}

		result := map[string]any{
			"path": path,
			"sync": map[string]any{
				"status": syncStatus,
			},
			"frontmatter": map[string]any{
				"status": fmStatus,
				"errors": fmErrors,
			},
			"markups": map[string]any{
				"total":    len(markups),
				"open":     openCount,
				"resolved": resolvedCount,
			},
		}
		return jsonResult(result)
	}
}

// newAPIClient creates an API client using resolved config.
// The MCP server resolves config the same way the CLI does — env vars, .specbox.yaml, ~/.specbox/config.yaml.
var newAPIClient = func() (*api.Client, error) {
	serverURL := config.ResolveServerURL()
	token := config.ResolveAuthToken(serverURL)
	if token == "" {
		return nil, fmt.Errorf("not logged in — run 'specbox login' first")
	}
	return &api.Client{APIURL: serverURL + "/api", Token: token}, nil
}

func pushSpecHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}

		client, err := newAPIClient()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		doc, err := svc.GetDocument(path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Parse optional metadata
		var metadata map[string]any
		if m := request.GetString("metadata", ""); m != "" {
			// metadata comes as JSON string from MCP
			_ = json.Unmarshal([]byte(m), &metadata)
		}

		// Derive slug from path
		slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

		result, statusCode, err := client.Push(doc.Content, slug, metadata)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("push failed: %v", err)), nil
		}

		switch statusCode {
		case 200:
			spec := result.Spec
			if spec == nil {
				return mcp.NewToolResultError("unexpected empty response"), nil
			}

			if spec.Merged {
				// Clean merge — replace both frontmatter and content
				newContent := spec.Frontmatter + "\n" + spec.RawContent
				if _, err := svc.SaveDocument(path, newContent); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to save: %v", err)), nil
				}
			} else {
				// Normal push — replace frontmatter only
				newContent := api.ReplaceFrontmatter(doc.Content, spec.Frontmatter)
				if _, err := svc.SaveDocument(path, newContent); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to save: %v", err)), nil
				}
			}

			response := map[string]any{
				"status":    "pushed",
				"id":        spec.ID,
				"version":   spec.Version,
				"url":       spec.URL,
				"merged":    spec.Merged,
				"questions": spec.Questions,
			}
			if len(result.Errors) > 0 {
				response["warnings"] = result.Errors
			}
			return jsonResult(response)

		case 409:
			spec := result.Spec
			if spec == nil {
				return mcp.NewToolResultError("merge conflict but no content returned"), nil
			}

			// Write merged content with conflict markers
			if strings.HasPrefix(doc.Content, "---\n") {
				rest := doc.Content[4:]
				idx := strings.Index(rest, "\n---\n")
				if idx >= 0 {
					frontmatter := doc.Content[:4+idx+5]
					newContent := frontmatter + spec.MergedContent
					_, _ = svc.SaveDocument(path, newContent)
				}
			}

			return jsonResult(map[string]any{
				"status":         "conflict",
				"message":        "Merge conflicts detected. Resolve the <<<<<<< local / ======= / >>>>>>> server markers and push again.",
				"merged_content": spec.MergedContent,
			})

		case 413:
			return mcp.NewToolResultError("spec is too large (max 1MB)"), nil

		case 422:
			return jsonResult(map[string]any{
				"status": "markup_error",
				"errors": result.Errors,
			})

		case 423:
			return mcp.NewToolResultError("spec is locked — use set_spec_metadata for metadata-only changes"), nil

		case 403:
			msg := "forbidden"
			if len(result.Errors) > 0 {
				msg = result.Errors[0].Message
			}
			return mcp.NewToolResultError(msg), nil

		default:
			msg := fmt.Sprintf("server returned %d", statusCode)
			if len(result.Errors) > 0 {
				msg += ": " + result.Errors[0].Message
			}
			return mcp.NewToolResultError(msg), nil
		}
	}
}

func pullSpecHandler(svc domain.DocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError("path is required"), nil
		}

		client, err := newAPIClient()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		doc, err := svc.GetDocument(path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		specID := api.ExtractSpecID(doc.Content)
		if specID == "" {
			return mcp.NewToolResultError("spec has not been pushed yet (no specbox.id in frontmatter)"), nil
		}

		hash := api.ContentHash(doc.Content)

		result, statusCode, err := client.Pull(specID, hash)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("pull failed: %v", err)), nil
		}

		switch statusCode {
		case 200:
			spec := result.Spec
			if spec == nil {
				return mcp.NewToolResultError("unexpected empty response"), nil
			}

			if spec.RawContent != "" {
				// Behind — replace frontmatter + content
				newContent := spec.Frontmatter + "\n" + spec.RawContent
				if _, err := svc.SaveDocument(path, newContent); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to save: %v", err)), nil
				}
				return jsonResult(map[string]any{
					"status":    "updated",
					"version":   spec.Version,
					"questions": spec.Questions,
				})
			} else {
				// Current — replace frontmatter only
				newContent := api.ReplaceFrontmatter(doc.Content, spec.Frontmatter)
				if _, err := svc.SaveDocument(path, newContent); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to save: %v", err)), nil
				}
				return jsonResult(map[string]any{
					"status":    "up_to_date",
					"version":   spec.Version,
					"questions": spec.Questions,
				})
			}

		case 404:
			return mcp.NewToolResultError("spec not found on server"), nil

		case 412:
			return jsonResult(map[string]any{
				"status":  "local_changes",
				"message": "Local content has changed. Push first to sync your changes.",
			})

		default:
			msg := fmt.Sprintf("server returned %d", statusCode)
			if len(result.Errors) > 0 {
				msg += ": " + result.Errors[0].Message
			}
			return mcp.NewToolResultError(msg), nil
		}
	}
}
