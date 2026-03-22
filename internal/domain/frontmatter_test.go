package domain

import (
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	content := "---\nspecbox:\n  url: https://specbox.io/s/abc123\n  status: draft\n---\n# My Spec\n\nContent here\n"

	data, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}

	specbox, ok := data["specbox"].(map[string]any)
	if !ok {
		t.Fatal("expected specbox key in frontmatter")
	}
	if specbox["url"] != "https://specbox.io/s/abc123" {
		t.Errorf("expected url, got %v", specbox["url"])
	}
	if specbox["status"] != "draft" {
		t.Errorf("expected draft status, got %v", specbox["status"])
	}

	if !strings.Contains(body, "# My Spec") {
		t.Errorf("expected body to contain heading, got %q", body)
	}
}

func TestParseFrontmatter_None(t *testing.T) {
	content := "# No Frontmatter\n\nJust content\n"
	data, body, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Error("expected nil data")
	}
	if body != content {
		t.Error("expected body to equal original content")
	}
}

func TestParseFrontmatter_Invalid(t *testing.T) {
	content := "---\n: invalid yaml [[\n---\nContent\n"
	_, _, err := ParseFrontmatter(content)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestWriteFrontmatter(t *testing.T) {
	data := map[string]any{
		"specbox": map[string]any{
			"url":    "https://specbox.io/s/abc123",
			"status": "draft",
		},
	}
	content := "# My Spec\n\nContent\n"
	result := WriteFrontmatter(data, content)

	if !strings.HasPrefix(result, "---\n") {
		t.Error("expected result to start with ---")
	}
	if !strings.Contains(result, "specbox:") {
		t.Error("expected specbox key")
	}
	if !strings.Contains(result, "# My Spec") {
		t.Error("expected content to be present")
	}
}

func TestWriteFrontmatter_Empty(t *testing.T) {
	result := WriteFrontmatter(nil, "content")
	if result != "content" {
		t.Error("expected just content for nil data")
	}
}

func TestRoundTrip(t *testing.T) {
	original := "---\ntitle: Test\n---\n# Content\n"
	data, body, err := ParseFrontmatter(original)
	if err != nil {
		t.Fatal(err)
	}
	result := WriteFrontmatter(data, body)
	if !strings.Contains(result, "title: Test") {
		t.Error("expected title to survive round-trip")
	}
	if !strings.Contains(result, "# Content") {
		t.Error("expected content to survive round-trip")
	}
}
