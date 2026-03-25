package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateQuestionStatus(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&gotBody)

		json.NewEncoder(w).Encode(SpecResponse{
			Spec: &struct {
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
			}{
				ID:      "spec123",
				Version: 5,
			},
			Errors: []Error{},
		})
	}))
	defer server.Close()

	client := &Client{APIURL: server.URL, Token: "test-token"}

	result, statusCode, err := client.UpdateQuestionStatus("spec123", "q456", "resolved", "Blue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusCode != 200 {
		t.Errorf("expected 200, got %d", statusCode)
	}
	if result.Spec.ID != "spec123" {
		t.Errorf("expected spec123, got %s", result.Spec.ID)
	}

	// Verify request
	if gotMethod != "PATCH" {
		t.Errorf("expected PATCH, got %s", gotMethod)
	}
	if gotPath != "/v1/specs/spec123/questions/q456" {
		t.Errorf("expected /v1/specs/spec123/questions/q456, got %s", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("expected Bearer test-token, got %s", gotAuth)
	}
	if gotBody["status"] != "resolved" {
		t.Errorf("expected status resolved, got %v", gotBody["status"])
	}
	if gotBody["resolved_value"] != "Blue" {
		t.Errorf("expected resolved_value Blue, got %v", gotBody["resolved_value"])
	}
}

func TestUpdateQuestionStatusReopen(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(SpecResponse{
			Spec: &struct {
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
			}{ID: "spec123"},
			Errors: []Error{},
		})
	}))
	defer server.Close()

	client := &Client{APIURL: server.URL, Token: "test-token"}

	_, _, err := client.UpdateQuestionStatus("spec123", "q456", "open", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody["status"] != "open" {
		t.Errorf("expected status open, got %v", gotBody["status"])
	}
	// resolved_value should not be sent when empty
	if _, exists := gotBody["resolved_value"]; exists {
		t.Errorf("resolved_value should not be sent for reopen, got %v", gotBody["resolved_value"])
	}
}

func TestSetMetadata(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(SpecResponse{
			Spec: &struct {
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
			}{ID: "spec123"},
			Errors: []Error{},
		})
	}))
	defer server.Close()

	client := &Client{APIURL: server.URL, Token: "test-token"}

	// Test lock
	_, _, err := client.Set("spec123", map[string]any{"locked": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metadata := gotBody["metadata"].(map[string]any)
	if metadata["locked"] != true {
		t.Errorf("expected locked true, got %v", metadata["locked"])
	}

	// Test status change
	_, _, err = client.Set("spec123", map[string]any{"status": "approved", "status_reason": "Reviewed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metadata = gotBody["metadata"].(map[string]any)
	if metadata["status"] != "approved" {
		t.Errorf("expected status approved, got %v", metadata["status"])
	}
	if metadata["status_reason"] != "Reviewed" {
		t.Errorf("expected status_reason Reviewed, got %v", metadata["status_reason"])
	}
}
