package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/specboxhq/specbox/internal/api"
	"github.com/specboxhq/specbox/internal/storage"
)

// --- test helpers (internal package, can't reuse mcp_test helpers) ---

func setupSync(t *testing.T) (*mcpserver.MCPServer, *storage.FileStore) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return NewServer(store), store
}

func seed(t *testing.T, store *storage.FileStore, path, content string) {
	t.Helper()
	if _, err := store.CreateDocument(path, content); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func call(t *testing.T, s *mcpserver.MCPServer, name string, args map[string]any) string {
	t.Helper()
	msg, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	result := s.HandleMessage(context.Background(), msg)
	resp := result.(gomcp.JSONRPCResponse)
	rb, _ := json.Marshal(resp.Result)
	var cr gomcp.CallToolResult
	json.Unmarshal(rb, &cr)
	if len(cr.Content) == 0 {
		t.Fatal("empty content")
	}
	return cr.Content[0].(gomcp.TextContent).Text
}

// mockAPI starts a test server and overrides newAPIClient for the duration of t.
func mockAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	orig := newAPIClient
	t.Cleanup(func() { newAPIClient = orig })
	newAPIClient = func() (*api.Client, error) {
		return &api.Client{APIURL: srv.URL, Token: "test-token"}, nil
	}
}

// specJSON builds a JSON API response with the given spec fields.
func specJSON(fields map[string]any) []byte {
	resp := map[string]any{
		"spec":   fields,
		"errors": []any{},
	}
	b, _ := json.Marshal(resp)
	return b
}

func errJSON(errors ...map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{
		"spec":   nil,
		"errors": errors,
	})
	return b
}

// ========== PUSH HANDLER TESTS ==========

func TestPushHandler_200_Normal(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n---\n# Spec\nContent")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(specJSON(map[string]any{
			"id": "abc123", "version": 2, "url": "https://specbox.io/s/abc123",
			"frontmatter": "---\nspecbox:\n  id: abc123\n  version: 2\n---",
			"merged":      false,
		}))
	})

	result := call(t, s, "push_spec", map[string]any{"path": "spec.md"})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	if resp["status"] != "pushed" {
		t.Errorf("expected pushed, got %v", resp["status"])
	}
	if resp["version"] != float64(2) {
		t.Errorf("expected version 2, got %v", resp["version"])
	}

	// Verify frontmatter was updated in document
	doc, _ := store.GetDocument("spec.md")
	if !strings.Contains(doc.Content, "version: 2") {
		t.Errorf("frontmatter not updated, got: %s", doc.Content)
	}
	// Content should be preserved
	if !strings.Contains(doc.Content, "# Spec\nContent") {
		t.Errorf("content lost after push, got: %s", doc.Content)
	}
}

func TestPushHandler_200_Merged(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n---\n# Spec\nOld content")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(specJSON(map[string]any{
			"id": "abc123", "version": 3,
			"frontmatter": "---\nspecbox:\n  id: abc123\n  version: 3\n---",
			"raw_content": "# Spec\nMerged content",
			"merged":      true,
		}))
	})

	result := call(t, s, "push_spec", map[string]any{"path": "spec.md"})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	if resp["merged"] != true {
		t.Errorf("expected merged=true, got %v", resp["merged"])
	}

	doc, _ := store.GetDocument("spec.md")
	if !strings.Contains(doc.Content, "Merged content") {
		t.Errorf("merged content not written, got: %s", doc.Content)
	}
}

func TestPushHandler_200_WithWarnings(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# Spec\nContent")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		resp := map[string]any{
			"spec": map[string]any{
				"id": "abc123", "version": 1,
				"frontmatter": "---\nspecbox:\n  id: abc123\n---",
				"merged":      false,
			},
			"errors": []map[string]any{
				{"field": "due_date", "message": "invalid date format"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	result := call(t, s, "push_spec", map[string]any{"path": "spec.md"})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	if resp["warnings"] == nil {
		t.Error("expected warnings in response")
	}
}

func TestPushHandler_409_Conflict(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n---\n# Spec\nLocal content")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		w.Write(specJSON(map[string]any{
			"id":             "abc123",
			"merged_content": "<<<<<<< local\nLocal\n=======\nServer\n>>>>>>> server",
		}))
	})

	result := call(t, s, "push_spec", map[string]any{"path": "spec.md"})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	if resp["status"] != "conflict" {
		t.Errorf("expected conflict, got %v", resp["status"])
	}

	// Document should have conflict markers
	doc, _ := store.GetDocument("spec.md")
	if !strings.Contains(doc.Content, "<<<<<<<") {
		t.Errorf("expected conflict markers in document, got: %s", doc.Content)
	}
}

func TestPushHandler_413(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# Spec")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(413)
		w.Write(errJSON(map[string]any{"message": "too large"}))
	})

	result := call(t, s, "push_spec", map[string]any{"path": "spec.md"})
	if !strings.Contains(result, "too large") {
		t.Errorf("expected too large error, got: %s", result)
	}
}

func TestPushHandler_422(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# Spec")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		resp := map[string]any{
			"spec": nil,
			"errors": []map[string]any{
				{"message": "invalid markup at line 5", "line": 5},
				{"message": "unclosed tag", "line": 12},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	result := call(t, s, "push_spec", map[string]any{"path": "spec.md"})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	if resp["status"] != "markup_error" {
		t.Errorf("expected markup_error, got %v", resp["status"])
	}
}

func TestPushHandler_423_Locked(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# Spec")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(423)
		w.Write(errJSON(map[string]any{"message": "locked"}))
	})

	result := call(t, s, "push_spec", map[string]any{"path": "spec.md"})
	if !strings.Contains(result, "locked") {
		t.Errorf("expected locked error, got: %s", result)
	}
}

func TestPushHandler_403(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# Spec")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		resp := map[string]any{
			"spec":   nil,
			"errors": []map[string]any{{"message": "not the owner"}},
		}
		json.NewEncoder(w).Encode(resp)
	})

	result := call(t, s, "push_spec", map[string]any{"path": "spec.md"})
	if !strings.Contains(result, "not the owner") {
		t.Errorf("expected forbidden error, got: %s", result)
	}
}

func TestPushHandler_UnknownStatus(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# Spec")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		resp := map[string]any{
			"spec":   nil,
			"errors": []map[string]any{{"message": "internal error"}},
		}
		json.NewEncoder(w).Encode(resp)
	})

	result := call(t, s, "push_spec", map[string]any{"path": "spec.md"})
	if !strings.Contains(result, "500") {
		t.Errorf("expected status code in error, got: %s", result)
	}
}

// ========== PULL HANDLER TESTS ==========

func TestPullHandler_200_UpToDate(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n  version: 2\n---\n# Spec\nContent")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(specJSON(map[string]any{
			"id": "abc123", "version": 2,
			"frontmatter": "---\nspecbox:\n  id: abc123\n  version: 2\n---",
			// No raw_content means up to date
		}))
	})

	result := call(t, s, "pull_spec", map[string]any{"path": "spec.md"})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	if resp["status"] != "up_to_date" {
		t.Errorf("expected up_to_date, got %v", resp["status"])
	}
}

func TestPullHandler_200_Behind(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n  version: 1\n---\n# Spec\nOld content")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(specJSON(map[string]any{
			"id": "abc123", "version": 3,
			"frontmatter": "---\nspecbox:\n  id: abc123\n  version: 3\n---",
			"raw_content": "# Spec\nNew content from server",
		}))
	})

	result := call(t, s, "pull_spec", map[string]any{"path": "spec.md"})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	if resp["status"] != "updated" {
		t.Errorf("expected updated, got %v", resp["status"])
	}
	if resp["version"] != float64(3) {
		t.Errorf("expected version 3, got %v", resp["version"])
	}

	doc, _ := store.GetDocument("spec.md")
	if !strings.Contains(doc.Content, "New content from server") {
		t.Errorf("content not updated, got: %s", doc.Content)
	}
}

func TestPullHandler_404(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n---\n# Spec")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write(errJSON(map[string]any{"message": "not found"}))
	})

	result := call(t, s, "pull_spec", map[string]any{"path": "spec.md"})
	if !strings.Contains(result, "not found") {
		t.Errorf("expected not found error, got: %s", result)
	}
}

func TestPullHandler_412_LocalChanges(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n---\n# Spec\nModified locally")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(412)
		w.Write(errJSON(map[string]any{"message": "content hash mismatch"}))
	})

	result := call(t, s, "pull_spec", map[string]any{"path": "spec.md"})
	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	if resp["status"] != "local_changes" {
		t.Errorf("expected local_changes, got %v", resp["status"])
	}
}

func TestPullHandler_UnknownStatus(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n---\n# Spec")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		resp := map[string]any{
			"spec":   nil,
			"errors": []map[string]any{{"message": "server error"}},
		}
		json.NewEncoder(w).Encode(resp)
	})

	result := call(t, s, "pull_spec", map[string]any{"path": "spec.md"})
	if !strings.Contains(result, "500") {
		t.Errorf("expected status code in error, got: %s", result)
	}
}

func TestPullHandler_NoSpecID(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# Spec with no frontmatter")

	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	})

	result := call(t, s, "pull_spec", map[string]any{"path": "spec.md"})
	if !strings.Contains(result, "not been pushed") {
		t.Errorf("expected not pushed error, got: %s", result)
	}
}

// ========== PULL sends hash in request ==========

func TestPullHandler_SendsHash(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n---\n# Spec\nContent")

	var gotPath string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.WriteHeader(200)
		w.Write(specJSON(map[string]any{
			"id": "abc123", "version": 1,
			"frontmatter": "---\nspecbox:\n  id: abc123\n---",
		}))
	})

	call(t, s, "pull_spec", map[string]any{"path": "spec.md"})

	if !strings.Contains(gotPath, "hash=") {
		t.Errorf("expected hash in pull request, got path: %s", gotPath)
	}
}

// ========== PUSH sends correct body ==========

func TestPushHandler_SendsBody(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# My Spec\nBody here")

	var gotBody map[string]any
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		w.Write(specJSON(map[string]any{
			"id": "x", "version": 1,
			"frontmatter": "---\nspecbox:\n  id: x\n---",
		}))
	})

	call(t, s, "push_spec", map[string]any{"path": "spec.md"})

	if gotBody["raw_content"] != "# My Spec\nBody here" {
		t.Errorf("unexpected raw_content: %v", gotBody["raw_content"])
	}
	if gotBody["slug"] != "spec" {
		t.Errorf("expected slug 'spec', got %v", gotBody["slug"])
	}
}

// ========== PRIVATE CONFIG TESTS ==========

func TestPushHandler_PrivateConfigSendsVisibility(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# New spec with no frontmatter")

	// Set private=true in a .specbox.yaml in the working directory
	dir, _ := os.Getwd()
	configPath := filepath.Join(dir, ".specbox.yaml")
	os.WriteFile(configPath, []byte("private: true\n"), 0644)
	t.Cleanup(func() { os.Remove(configPath) })

	var gotBody map[string]any
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		w.Write(specJSON(map[string]any{
			"id": "x", "version": 1,
			"frontmatter": "---\nspecbox:\n  id: x\n  metadata:\n    visibility: private\n---",
		}))
	})

	call(t, s, "push_spec", map[string]any{"path": "spec.md"})

	metadata, ok := gotBody["metadata"].(map[string]any)
	if !ok {
		t.Fatal("expected metadata in push body")
	}
	if metadata["visibility"] != "private" {
		t.Errorf("expected visibility=private, got %v", metadata["visibility"])
	}
}

func TestPushHandler_PrivateConfigSkipsIfVisibilityInFrontmatter(t *testing.T) {
	s, store := setupSync(t)
	// Spec already has visibility set in frontmatter
	seed(t, store, "spec.md", "---\nspecbox:\n  id: abc123\n  metadata:\n    visibility: public\n---\n# Spec")

	dir, _ := os.Getwd()
	configPath := filepath.Join(dir, ".specbox.yaml")
	os.WriteFile(configPath, []byte("private: true\n"), 0644)
	t.Cleanup(func() { os.Remove(configPath) })

	var gotBody map[string]any
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		w.Write(specJSON(map[string]any{
			"id": "abc123", "version": 2,
			"frontmatter": "---\nspecbox:\n  id: abc123\n  metadata:\n    visibility: public\n---",
		}))
	})

	call(t, s, "push_spec", map[string]any{"path": "spec.md"})

	// Should NOT send visibility in metadata — frontmatter already has it
	if gotBody["metadata"] != nil {
		metadata := gotBody["metadata"].(map[string]any)
		if _, ok := metadata["visibility"]; ok {
			t.Error("should not send visibility when frontmatter already has it")
		}
	}
}

func TestPushHandler_NoPrivateConfigNoVisibility(t *testing.T) {
	s, store := setupSync(t)
	seed(t, store, "spec.md", "# Spec")

	// Ensure no private config — use temp HOME with no .specbox.yaml
	t.Setenv("HOME", t.TempDir())
	// Remove any .specbox.yaml in cwd (shouldn't exist but be safe)

	var gotBody map[string]any
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		w.Write(specJSON(map[string]any{
			"id": "x", "version": 1,
			"frontmatter": "---\nspecbox:\n  id: x\n---",
		}))
	})

	call(t, s, "push_spec", map[string]any{"path": "spec.md"})

	// Should not have metadata at all (no private config)
	if gotBody["metadata"] != nil {
		t.Errorf("expected no metadata, got %v", gotBody["metadata"])
	}
}
