package api

import "testing"

func TestExtractSpecID_NewFormat(t *testing.T) {
	content := "---\nspecbox:\n  id: aBcDeFgHiJkL\n  url: https://specbox.io/test\n  version: 3\n  metadata:\n    status: draft\n---\n\n# Hello"

	id := ExtractSpecID(content)
	if id != "aBcDeFgHiJkL" {
		t.Errorf("expected aBcDeFgHiJkL, got %q", id)
	}
}

func TestExtractSpecID_LegacyFormat(t *testing.T) {
	content := "---\nspecbox:\n  id: aBcDeFgHiJkL\n  url: https://specbox.io/test\n  version: 2\n  status: draft\n---\n\n# Hello"

	id := ExtractSpecID(content)
	if id != "aBcDeFgHiJkL" {
		t.Errorf("expected aBcDeFgHiJkL, got %q", id)
	}
}

func TestExtractSpecID_NoFrontmatter(t *testing.T) {
	id := ExtractSpecID("# Just content")
	if id != "" {
		t.Errorf("expected empty, got %q", id)
	}
}

func TestExtractSpecID_NoSpecbox(t *testing.T) {
	content := "---\ntitle: Test\n---\n# Content"
	id := ExtractSpecID(content)
	if id != "" {
		t.Errorf("expected empty, got %q", id)
	}
}

func TestContentHash(t *testing.T) {
	// Same content with different frontmatter should produce same hash
	a := "---\nspecbox:\n  id: abc\n  version: 1\n---\n# Hello\n\nContent"
	b := "---\nspecbox:\n  id: abc\n  version: 2\n  metadata:\n    status: draft\n---\n# Hello\n\nContent"

	hashA := ContentHash(a)
	hashB := ContentHash(b)
	if hashA != hashB {
		t.Errorf("expected same hash, got %q vs %q", hashA, hashB)
	}
}

func TestExtractSpecID_WithResolvedQuestions(t *testing.T) {
	content := "---\nspecbox:\n  id: aBcDeFgHiJkL\n  url: https://specbox.io/test\n  version: 3\n  metadata:\n    resolved_questions:\n      q1:\n        question: Which API?\n        answer: REST\n    status: draft\n---\n\n# Hello"

	id := ExtractSpecID(content)
	if id != "aBcDeFgHiJkL" {
		t.Errorf("expected aBcDeFgHiJkL, got %q", id)
	}
}

func TestReplaceFrontmatter(t *testing.T) {
	content := "---\nspecbox:\n  id: abc\n---\n# Content"
	newFM := "---\nspecbox:\n  id: abc\n  version: 2\n---"

	result := ReplaceFrontmatter(content, newFM)
	if result != "---\nspecbox:\n  id: abc\n  version: 2\n---\n# Content" {
		t.Errorf("unexpected result: %q", result)
	}
}
