package domain

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFrontmatter extracts YAML frontmatter from document content.
// Returns the parsed data (map), the content without frontmatter, and any error.
// If no frontmatter is present, returns nil, original content, nil.
func ParseFrontmatter(content string) (map[string]any, string, error) {
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return nil, content, nil
	}

	// Find the closing ---
	rest := content[4:] // skip opening "---\n"
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		// Try with trailing --- at end of file
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - 4
		} else {
			return nil, content, nil
		}
	}

	yamlContent := rest[:idx]
	bodyStart := 4 + idx + 4 // opening "---\n" + yaml + "\n---\n"
	body := ""
	if bodyStart <= len(content) {
		body = content[bodyStart:]
	}

	var data map[string]any
	if err := yaml.Unmarshal([]byte(yamlContent), &data); err != nil {
		return nil, content, err
	}

	return data, body, nil
}

// WriteFrontmatter writes YAML frontmatter followed by content.
// If data is nil or empty, returns just the content.
func WriteFrontmatter(data map[string]any, content string) string {
	if len(data) == 0 {
		return content
	}

	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return content
	}

	return "---\n" + string(yamlBytes) + "---\n" + content
}
