package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/specboxhq/specbox/internal/domain"
)

// NewServer creates an MCP server with all document tools registered.
func NewServer(svc domain.DocumentService) *server.MCPServer {
	s := server.NewMCPServer(
		"specbox",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	registerReadTools(s, svc)
	registerMutationTools(s, svc)
	registerLineTools(s, svc)
	registerMarkdownTools(s, svc)
	registerSpecboxTools(s, svc)

	return s
}

// jsonResult marshals data to JSON and returns it as a text tool result.
func jsonResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// docSummary returns a consistent mutation response shape.
func docSummary(doc *domain.Document) map[string]any {
	return map[string]any{
		"path":       doc.Path,
		"line_count": doc.GetLineCount(),
		"updated_at": doc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// changeEntry represents a single line change for show_changes output.
type changeEntry struct {
	Line   int    `json:"line"`
	Before string `json:"before"`
	After  string `json:"after,omitempty"`
}

// computeChanges compares old and new content and returns changed lines.
func computeChanges(oldContent, newContent string) []changeEntry {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	var changes []changeEntry

	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			changes = append(changes, changeEntry{
				Line:   i + 1,
				Before: oldLine,
				After:  newLine,
			})
		}
	}
	return changes
}

// docSummaryWithChanges returns docSummary plus a changes array.
func docSummaryWithChanges(doc *domain.Document, oldContent string) map[string]any {
	result := docSummary(doc)
	changes := computeChanges(oldContent, doc.Content)
	result["changes"] = changes
	result["changes_count"] = len(changes)
	return result
}
