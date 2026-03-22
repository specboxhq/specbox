package domain

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
)

// MarkupType identifies the kind of specbox markup.
type MarkupType string

const (
	MarkupQuestion MarkupType = "question"
)

// MarkupMode identifies how the markup is represented in the document.
type MarkupMode string

const (
	MarkupInline  MarkupMode = "inline"
	MarkupBlock   MarkupMode = "block"
	MarkupWrapped MarkupMode = "wrapped"
)

// Markup represents a parsed specbox markup tag found in a document.
type Markup struct {
	ID        string            `json:"id"`
	Type      MarkupType        `json:"type"`
	Mode      MarkupMode        `json:"mode"`
	Status    string            `json:"status,omitempty"`
	Attrs     map[string]string `json:"attrs,omitempty"`
	Content   string            `json:"content,omitempty"`
	StartLine int               `json:"start_line"`
	EndLine   int               `json:"end_line"`
}

// idCharset is the set of characters used for markup IDs (a-z, 0-9).
const idCharset = "abcdefghijklmnopqrstuvwxyz0123456789"

// GenerateID generates a 10-character random alphanumeric ID (a-z, 0-9).
func GenerateID() string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	for i := range b {
		b[i] = idCharset[int(b[i])%len(idCharset)]
	}
	return string(b)
}

// openTagRe matches the opening specbox comment tag.
// Format: <!-- specbox:type:id ... -->  (or without --> for block mode)
var openTagRe = regexp.MustCompile(`^<!--\s+specbox:(decision|feedback|question):([a-z0-9]{10})\b(.*)$`)

// closeTagRe matches a closing specbox comment tag.
// Format: <!-- /specbox:type -->
var closeTagRe = regexp.MustCompile(`^<!--\s+/specbox:(decision|feedback|question)\s*-->`)

// attrRe matches key="value" attribute pairs.
var attrRe = regexp.MustCompile(`(\w+)="([^"]*)"`)

// parseAttrs extracts key="value" pairs from a string, returning the attrs map
// and the remaining text after removing all attrs.
func parseAttrs(s string) (map[string]string, string) {
	attrs := make(map[string]string)
	matches := attrRe.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return attrs, strings.TrimSpace(s)
	}
	for _, m := range matches {
		key := s[m[2]:m[3]]
		val := s[m[4]:m[5]]
		attrs[key] = val
	}
	// Remove all attr matches from the string to get remaining content
	remaining := attrRe.ReplaceAllString(s, "")
	return attrs, strings.TrimSpace(remaining)
}

// ParseMarkups finds all specbox markups in document content.
// Skips markups inside markdown code fences (``` blocks).
func ParseMarkups(content string) []Markup {
	lines := strings.Split(content, "\n")
	var markups []Markup

	inCodeFence := false
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])

		// Track code fence state
		// Opening fence: ``` or ```language (has content after ```)
		// Closing fence: ``` only (bare, no language identifier)
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeFence {
				// Any ``` line opens a fence
				inCodeFence = true
			} else if trimmed == "```" {
				// Only bare ``` closes a fence
				inCodeFence = false
			}
			// ```language while inside a fence is just content, skip
			i++
			continue
		}
		if inCodeFence {
			i++
			continue
		}

		line := trimmed
		m := openTagRe.FindStringSubmatch(line)
		if m == nil {
			i++
			continue
		}

		mType := MarkupType(m[1])
		mID := m[2]
		rest := m[3] // everything after specbox:type:id

		// Parse attributes and remaining text from the rest of the opening line
		attrs, remaining := parseAttrs(rest)

		// Extract status from attrs if present
		status := attrs["status"]
		delete(attrs, "status")

		// Determine mode based on whether the first line ends with -->
		endsWithClose := strings.HasSuffix(strings.TrimSpace(rest), "-->")

		if endsWithClose {
			// Could be inline or wrapped
			// Remove trailing --> from remaining
			remaining = strings.TrimSpace(remaining)
			if strings.HasSuffix(remaining, "-->") {
				remaining = strings.TrimSpace(remaining[:len(remaining)-3])
			}

			// Check if there's a closing tag further down
			wrappedEnd := -1
			closePrefix := fmt.Sprintf("/specbox:%s", mType)
			for j := i + 1; j < len(lines); j++ {
				trimmed := strings.TrimSpace(lines[j])
				if closeTagRe.MatchString(trimmed) && strings.Contains(trimmed, closePrefix) {
					wrappedEnd = j
					break
				}
			}

			if wrappedEnd >= 0 {
				// Wrapped mode — content is the lines between opening and closing tags
				var contentLines []string
				for j := i + 1; j < wrappedEnd; j++ {
					contentLines = append(contentLines, lines[j])
				}
				markups = append(markups, Markup{
					ID:        mID,
					Type:      mType,
					Mode:      MarkupWrapped,
					Status:    status,
					Attrs:     attrs,
					Content:   strings.Join(contentLines, "\n"),
					StartLine: i + 1,
					EndLine:   wrappedEnd + 1,
				})
				i = wrappedEnd + 1
			} else {
				// Inline mode
				markups = append(markups, Markup{
					ID:        mID,
					Type:      mType,
					Mode:      MarkupInline,
					Status:    status,
					Attrs:     attrs,
					Content:   remaining,
					StartLine: i + 1,
					EndLine:   i + 1,
				})
				i++
			}
		} else {
			// Block mode — multiline YAML comment, find closing -->
			blockEnd := i
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "-->" || strings.HasSuffix(strings.TrimSpace(lines[j]), "-->") {
					blockEnd = j
					break
				}
			}

			// Collect the YAML content lines (from after the opening tag line to before -->)
			var yamlLines []string
			for j := i + 1; j <= blockEnd; j++ {
				trimmed := strings.TrimSpace(lines[j])
				if trimmed == "-->" || strings.HasSuffix(trimmed, "-->") {
					// If the line has content before -->, include it minus the -->
					before := strings.TrimSuffix(trimmed, "-->")
					before = strings.TrimSpace(before)
					if before != "" {
						yamlLines = append(yamlLines, before)
					}
					break
				}
				yamlLines = append(yamlLines, lines[j])
			}

			// The remaining text on the opening tag line is also part of content
			// e.g. <!-- specbox:decision:abc status="open"
			// The "remaining" after attrs might have leftover text
			var blockContent string
			if remaining != "" {
				blockContent = remaining + "\n" + strings.Join(yamlLines, "\n")
			} else {
				blockContent = strings.Join(yamlLines, "\n")
			}

			markups = append(markups, Markup{
				ID:        mID,
				Type:      mType,
				Mode:      MarkupBlock,
				Status:    status,
				Attrs:     attrs,
				Content:   blockContent,
				StartLine: i + 1,
				EndLine:   blockEnd + 1,
			})
			i = blockEnd + 1
		}
	}

	return markups
}

// FindMarkupByID finds a markup by its ID in the given content.
func FindMarkupByID(content string, id string) *Markup {
	markups := ParseMarkups(content)
	for _, m := range markups {
		if m.ID == id {
			return &m
		}
	}
	return nil
}
