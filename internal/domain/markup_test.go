package domain

import (
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id := GenerateID()
	if len(id) != 10 {
		t.Errorf("expected length 10, got %d", len(id))
	}
	for _, c := range id {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789", c) {
			t.Errorf("unexpected character in ID: %c", c)
		}
	}

	// IDs should be unique
	id2 := GenerateID()
	if id == id2 {
		t.Error("two generated IDs should not be equal")
	}
}

func TestParseMarkups_Inline(t *testing.T) {
	content := `# Auth Design

<!-- specbox:decision:a7k2m9x4p1 JWT or sessions? -->

Some other text.
`
	markups := ParseMarkups(content)
	if len(markups) != 1 {
		t.Fatalf("expected 1 markup, got %d", len(markups))
	}
	m := markups[0]
	if m.ID != "a7k2m9x4p1" {
		t.Errorf("expected ID a7k2m9x4p1, got %s", m.ID)
	}
	if m.Type != MarkupType("decision") {
		t.Errorf("expected type decision, got %s", m.Type)
	}
	if m.Mode != MarkupInline {
		t.Errorf("expected mode inline, got %s", m.Mode)
	}
	if m.Content != "JWT or sessions?" {
		t.Errorf("expected content 'JWT or sessions?', got %q", m.Content)
	}
	if m.StartLine != 3 {
		t.Errorf("expected start_line 3, got %d", m.StartLine)
	}
	if m.EndLine != 3 {
		t.Errorf("expected end_line 3, got %d", m.EndLine)
	}
}

func TestParseMarkups_Block(t *testing.T) {
	content := `# Auth Design

<!-- specbox:decision:a7k2m9x4p1 status="open"
question: Should authentication use JWT or server-side sessions?
options:
  - JWT (stateless, simpler scaling)
  - Sessions (more control, revocable)
-->

Some text.
`
	markups := ParseMarkups(content)
	if len(markups) != 1 {
		t.Fatalf("expected 1 markup, got %d", len(markups))
	}
	m := markups[0]
	if m.ID != "a7k2m9x4p1" {
		t.Errorf("expected ID a7k2m9x4p1, got %s", m.ID)
	}
	if m.Type != MarkupType("decision") {
		t.Errorf("expected type decision, got %s", m.Type)
	}
	if m.Mode != MarkupBlock {
		t.Errorf("expected mode block, got %s", m.Mode)
	}
	if m.Status != "open" {
		t.Errorf("expected status open, got %s", m.Status)
	}
	if m.StartLine != 3 {
		t.Errorf("expected start_line 3, got %d", m.StartLine)
	}
	if m.EndLine != 8 {
		t.Errorf("expected end_line 8, got %d", m.EndLine)
	}
	if !strings.Contains(m.Content, "Should authentication use JWT") {
		t.Errorf("expected content to contain question, got %q", m.Content)
	}
}

func TestParseMarkups_Wrapped(t *testing.T) {
	content := `# Auth Design

<!-- specbox:decision:a7k2m9x4p1 status="open" -->
### Should we use JWT or sessions?
- JWT
- Sessions
<!-- /specbox:decision -->

More text.
`
	markups := ParseMarkups(content)
	if len(markups) != 1 {
		t.Fatalf("expected 1 markup, got %d", len(markups))
	}
	m := markups[0]
	if m.ID != "a7k2m9x4p1" {
		t.Errorf("expected ID a7k2m9x4p1, got %s", m.ID)
	}
	if m.Type != MarkupType("decision") {
		t.Errorf("expected type decision, got %s", m.Type)
	}
	if m.Mode != MarkupWrapped {
		t.Errorf("expected mode wrapped, got %s", m.Mode)
	}
	if m.Status != "open" {
		t.Errorf("expected status open, got %s", m.Status)
	}
	if m.StartLine != 3 {
		t.Errorf("expected start_line 3, got %d", m.StartLine)
	}
	if m.EndLine != 7 {
		t.Errorf("expected end_line 7, got %d", m.EndLine)
	}
	if !strings.Contains(m.Content, "### Should we use JWT or sessions?") {
		t.Errorf("expected wrapped content, got %q", m.Content)
	}
}

func TestParseMarkups_Multiple(t *testing.T) {
	content := `# Spec

<!-- specbox:decision:aaaaaaaaaa JWT or sessions? -->

<!-- specbox:feedback:bbbbbbbbbb status="open"
Please review the API design.
-->

<!-- specbox:question:cccccccccc status="open" -->
What about caching?
<!-- /specbox:question -->
`
	markups := ParseMarkups(content)
	if len(markups) != 3 {
		t.Fatalf("expected 3 markups, got %d", len(markups))
	}
	if markups[0].Mode != MarkupInline {
		t.Errorf("first should be inline, got %s", markups[0].Mode)
	}
	if markups[1].Mode != MarkupBlock {
		t.Errorf("second should be block, got %s", markups[1].Mode)
	}
	if markups[2].Mode != MarkupWrapped {
		t.Errorf("third should be wrapped, got %s", markups[2].Mode)
	}
}

func TestParseMarkups_WithAttrs(t *testing.T) {
	content := `<!-- specbox:decision:a7k2m9x4p1 status="open" priority="high" -->
Content here
<!-- /specbox:decision -->
`
	markups := ParseMarkups(content)
	if len(markups) != 1 {
		t.Fatalf("expected 1 markup, got %d", len(markups))
	}
	m := markups[0]
	if m.Status != "open" {
		t.Errorf("expected status open, got %s", m.Status)
	}
	if m.Attrs["priority"] != "high" {
		t.Errorf("expected priority=high, got %s", m.Attrs["priority"])
	}
}

func TestFindMarkupByID(t *testing.T) {
	content := `<!-- specbox:decision:aaaaaaaaaa test -->
<!-- specbox:question:bbbbbbbbbb other -->
`
	m := FindMarkupByID(content, "bbbbbbbbbb")
	if m == nil {
		t.Fatal("expected to find markup")
	}
	if m.Type != MarkupQuestion {
		t.Errorf("expected question, got %s", m.Type)
	}

	m = FindMarkupByID(content, "cccccccccc")
	if m != nil {
		t.Error("expected nil for non-existent ID")
	}
}
