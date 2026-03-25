package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPush(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&gotBody)

		json.NewEncoder(w).Encode(specResp("spec123", 3))
	}))
	defer server.Close()

	client := &Client{APIURL: server.URL, Token: "tok"}

	result, statusCode, err := client.Push("# My Spec\nContent", "my-spec", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusCode != 200 {
		t.Errorf("expected 200, got %d", statusCode)
	}
	if result.Spec.ID != "spec123" {
		t.Errorf("expected spec123, got %s", result.Spec.ID)
	}
	if result.Spec.Version != 3 {
		t.Errorf("expected version 3, got %d", result.Spec.Version)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/v1/specs" {
		t.Errorf("expected /v1/specs, got %s", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("expected Bearer tok, got %s", gotAuth)
	}
	if gotBody["raw_content"] != "# My Spec\nContent" {
		t.Errorf("unexpected raw_content: %v", gotBody["raw_content"])
	}
	if gotBody["slug"] != "my-spec" {
		t.Errorf("unexpected slug: %v", gotBody["slug"])
	}
}

func TestPushWithMetadata(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(specResp("spec123", 1))
	}))
	defer server.Close()

	client := &Client{APIURL: server.URL, Token: "tok"}
	meta := map[string]any{"status": "draft", "tags": []string{"v1"}}

	_, _, err := client.Push("content", "slug", meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metadata := gotBody["metadata"].(map[string]any)
	if metadata["status"] != "draft" {
		t.Errorf("expected status draft, got %v", metadata["status"])
	}
}

func TestPull(t *testing.T) {
	var gotMethod, gotPath, gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("hash")

		json.NewEncoder(w).Encode(specResp("spec123", 5))
	}))
	defer server.Close()

	client := &Client{APIURL: server.URL, Token: "tok"}

	result, statusCode, err := client.Pull("spec123", "abc123hash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusCode != 200 {
		t.Errorf("expected 200, got %d", statusCode)
	}
	if result.Spec.Version != 5 {
		t.Errorf("expected version 5, got %d", result.Spec.Version)
	}

	if gotMethod != "GET" {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/v1/specs/spec123/pull" {
		t.Errorf("expected /v1/specs/spec123/pull, got %s", gotPath)
	}
	if gotQuery != "abc123hash" {
		t.Errorf("expected hash abc123hash, got %s", gotQuery)
	}
}

func TestClientServerDown(t *testing.T) {
	client := &Client{APIURL: "http://127.0.0.1:1", Token: "tok"}

	_, _, err := client.Push("content", "slug", nil)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestClientInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := &Client{APIURL: server.URL, Token: "tok"}

	_, statusCode, err := client.Push("content", "slug", nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if statusCode != 200 {
		t.Errorf("expected status 200 even with parse error, got %d", statusCode)
	}
}

func TestClientErrorStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"conflict", 409},
		{"too large", 413},
		{"validation", 422},
		{"locked", 423},
		{"forbidden", 403},
		{"not found", 404},
		{"precondition", 412},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(SpecResponse{
					Errors: []Error{{Message: tt.name}},
				})
			}))
			defer server.Close()

			client := &Client{APIURL: server.URL, Token: "tok"}
			result, code, err := client.Push("content", "slug", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if code != tt.statusCode {
				t.Errorf("expected %d, got %d", tt.statusCode, code)
			}
			if len(result.Errors) == 0 {
				t.Error("expected errors in response")
			}
		})
	}
}

// specResp builds a minimal SpecResponse for test assertions.
func specResp(id string, version int) SpecResponse {
	return SpecResponse{
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
		}{ID: id, Version: version},
		Errors: []Error{},
	}
}
