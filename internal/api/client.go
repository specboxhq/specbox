package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/specboxhq/specbox/internal/domain"
)

// Error represents an API error.
type Error struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
}

// SpecResponse is the standard API response for spec operations.
type SpecResponse struct {
	Spec *struct {
		ID            string         `json:"id"`
		Slug          string         `json:"slug"`
		Title         string         `json:"title"`
		URL           string         `json:"url"`
		Version       int            `json:"version"`
		ContentHash   string         `json:"content_hash"`
		PushedAt      string         `json:"pushed_at"`
		Frontmatter   string         `json:"frontmatter"`
		RawContent    string         `json:"raw_content,omitempty"`
		Merged        bool           `json:"merged,omitempty"`
		MergedContent string         `json:"merged_content,omitempty"`
		HasConflicts  bool           `json:"has_conflicts,omitempty"`
		Metadata      map[string]any `json:"metadata,omitempty"`
		Questions     []any          `json:"questions,omitempty"`
		Approvals     []any          `json:"approvals,omitempty"`
	} `json:"spec"`
	Errors []Error `json:"errors"`
}

// Client talks to the specbox.io API.
type Client struct {
	APIURL string
	Token  string
}

// Push sends a spec to the server.
func (c *Client) Push(rawContent, slug string, metadata map[string]any) (*SpecResponse, int, error) {
	body := map[string]any{
		"raw_content": rawContent,
		"slug":        slug,
	}
	if metadata != nil {
		body["metadata"] = metadata
	}
	return c.doJSON("POST", c.APIURL+"/v1/specs", body)
}

// Pull fetches a spec from the server.
func (c *Client) Pull(specID, hash string) (*SpecResponse, int, error) {
	return c.doJSON("GET", c.APIURL+"/v1/specs/"+specID+"/pull?hash="+hash, nil)
}

// Set updates spec metadata.
func (c *Client) Set(specID string, metadata map[string]any) (*SpecResponse, int, error) {
	body := map[string]any{"metadata": metadata}
	return c.doJSON("PATCH", c.APIURL+"/v1/specs/"+specID, body)
}

func (c *Client) doJSON(method, url string, body any) (*SpecResponse, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot reach server: %w", err)
	}
	defer resp.Body.Close()

	var result SpecResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("invalid response: %w", err)
	}

	return &result, resp.StatusCode, nil
}

// --- Content hashing (must match server normalization) ---

// ContentHash computes the SHA-256 hash of content after stripping frontmatter,
// normalizing line endings, and trimming whitespace. Matches the server's algorithm.
func ContentHash(rawContent string) string {
	body := StripAllFrontmatter(rawContent)
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.TrimSpace(body)
	h := sha256.Sum256([]byte(body))
	return hex.EncodeToString(h[:])
}

// StripAllFrontmatter removes YAML frontmatter from content.
func StripAllFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	rest := content[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx >= 0 {
		return rest[idx+5:]
	}
	if strings.HasSuffix(rest, "\n---") {
		return ""
	}
	return content
}

// --- Frontmatter helpers ---

// ExtractSpecID reads the specbox.id from frontmatter.
func ExtractSpecID(content string) string {
	fm, _, err := domain.ParseFrontmatter(content)
	if err != nil || fm == nil {
		return ""
	}
	specbox, ok := fm["specbox"].(map[string]any)
	if !ok {
		return ""
	}
	id, _ := specbox["id"].(string)
	return id
}

// ReplaceFrontmatter replaces the frontmatter block in content with newFrontmatter.
func ReplaceFrontmatter(content, newFrontmatter string) string {
	body := content
	if strings.HasPrefix(content, "---\n") {
		rest := content[4:]
		idx := strings.Index(rest, "\n---\n")
		if idx >= 0 {
			body = rest[idx+5:]
		} else if strings.HasSuffix(rest, "\n---") {
			body = ""
		}
	}

	if body != "" {
		return newFrontmatter + "\n" + body
	}
	return newFrontmatter + "\n"
}
