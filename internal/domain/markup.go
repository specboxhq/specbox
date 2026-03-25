package domain

import (
	"crypto/rand"
	"fmt"
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

// idCharset is the set of characters used for markup IDs [a-zA-Z0-9].
const idCharset = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// GenerateID generates a 12-character random alphanumeric ID [a-zA-Z0-9].
func GenerateID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	for i := range b {
		b[i] = idCharset[int(b[i])%len(idCharset)]
	}
	return string(b)
}

// validMarkupTypes lists accepted specbox markup types.
var validMarkupTypes = map[string]bool{
	"decision": true, "feedback": true, "question": true,
}

// isIDChar returns true if c is valid in a markup ID [a-zA-Z0-9].
func isIDChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// parseOpenTag parses a specbox opening tag from a trimmed line.
// Returns (type, id, rest, ok). rest is everything after the ID.
func parseOpenTag(line string) (string, string, string, bool) {
	// Must start with <!-- then optional whitespace then specbox:
	if !strings.HasPrefix(line, "<!--") {
		return "", "", "", false
	}
	pos := 4
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	if !strings.HasPrefix(line[pos:], "specbox:") {
		return "", "", "", false
	}
	pos += 8 // skip "specbox:"

	// Read type (until next ':')
	typeStart := pos
	for pos < len(line) && line[pos] != ':' {
		pos++
	}
	if pos >= len(line) {
		return "", "", "", false
	}
	mType := line[typeStart:pos]
	if !validMarkupTypes[mType] {
		return "", "", "", false
	}
	pos++ // skip ':'

	// Read ID (alphanumeric, 10-12 chars)
	idStart := pos
	for pos < len(line) && isIDChar(line[pos]) {
		pos++
	}
	id := line[idStart:pos]
	idLen := len(id)
	if idLen < 10 || idLen > 12 {
		return "", "", "", false
	}

	// Must end at a word boundary (space, tab, or end of string)
	if pos < len(line) && line[pos] != ' ' && line[pos] != '\t' {
		return "", "", "", false
	}

	return mType, id, line[pos:], true
}

// parseCloseTag checks if a trimmed line is a specbox closing tag.
// Returns (type, ok).
func parseCloseTag(line string) (string, bool) {
	if !strings.HasPrefix(line, "<!--") {
		return "", false
	}
	pos := 4
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	if !strings.HasPrefix(line[pos:], "/specbox:") {
		return "", false
	}
	pos += 9 // skip "/specbox:"

	typeStart := pos
	for pos < len(line) && line[pos] != ' ' && line[pos] != '\t' && line[pos] != '-' {
		pos++
	}
	mType := line[typeStart:pos]
	if !validMarkupTypes[mType] {
		return "", false
	}

	// Skip whitespace then expect -->
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	if !strings.HasPrefix(line[pos:], "-->") {
		return "", false
	}
	return mType, true
}

// parseAttrs extracts key="value" pairs from a string using character-by-character
// parsing. Handles --> inside quoted values correctly. Returns the attrs map
// and the remaining text after removing all attrs.
func parseAttrs(s string) (map[string]string, string) {
	attrs := make(map[string]string)
	var remaining []string
	i := 0
	for i < len(s) {
		// Skip whitespace
		if s[i] == ' ' || s[i] == '\t' {
			i++
			continue
		}

		// Try to parse key="value": scan for a word followed by ="
		keyStart := i
		for i < len(s) && s[i] != '=' && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		if i < len(s) && s[i] == '=' && i+1 < len(s) && s[i+1] == '"' {
			key := s[keyStart:i]
			i += 2 // skip ="
			valStart := i
			for i < len(s) && s[i] != '"' {
				i++
			}
			if i < len(s) {
				attrs[key] = s[valStart:i]
				i++ // skip closing "
				continue
			}
			// Unclosed quote — treat whole thing as remaining
			i = keyStart
		}

		// Not an attr — the text from keyStart to i is a non-attr token
		// If we stopped at end of string or whitespace, the word is already scanned
		word := s[keyStart:i]
		if word != "" {
			remaining = append(remaining, word)
		}
	}
	return attrs, strings.TrimSpace(strings.Join(remaining, " "))
}

// endsWithCloseComment checks if s ends with --> accounting for possible
// content before it. Uses character scanning to avoid false matches inside quotes.
func endsWithCloseComment(s string) bool {
	// Scan for --> not inside quotes
	inQuote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && i+2 < len(s) && s[i] == '-' && s[i+1] == '-' && s[i+2] == '>' {
			// Check that nothing non-whitespace follows
			rest := strings.TrimSpace(s[i+3:])
			if rest == "" {
				return true
			}
		}
	}
	return false
}

// stripCloseComment removes the trailing --> (outside quotes) from s.
func stripCloseComment(s string) string {
	inQuote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && i+2 < len(s) && s[i] == '-' && s[i+1] == '-' && s[i+2] == '>' {
			return strings.TrimSpace(s[:i])
		}
	}
	return s
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

		mTypeStr, mID, rest, ok := parseOpenTag(trimmed)
		if !ok {
			i++
			continue
		}

		mType := MarkupType(mTypeStr)

		// Parse attributes and remaining text from the rest of the opening line
		attrs, remaining := parseAttrs(rest)

		// Extract status from attrs if present
		status := attrs["status"]
		delete(attrs, "status")

		// Determine mode based on whether the first line ends with --> (outside quotes)
		closed := endsWithCloseComment(rest)

		if closed {
			// Could be inline or wrapped
			// Remove trailing --> from remaining
			remaining = stripCloseComment(remaining)

			// Check if there's a closing tag further down
			wrappedEnd := -1
			for j := i + 1; j < len(lines); j++ {
				closeType, isClose := parseCloseTag(strings.TrimSpace(lines[j]))
				if isClose && closeType == mTypeStr {
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
				t := strings.TrimSpace(lines[j])
				if t == "-->" || endsWithCloseComment(t) {
					blockEnd = j
					break
				}
			}

			// Collect the YAML content lines (from after the opening tag line to before -->)
			var yamlLines []string
			for j := i + 1; j <= blockEnd; j++ {
				t := strings.TrimSpace(lines[j])
				if t == "-->" || endsWithCloseComment(t) {
					// If the line has content before -->, include it minus the -->
					before := stripCloseComment(t)
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
