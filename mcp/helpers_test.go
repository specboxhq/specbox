package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/specboxhq/specbox/internal/storage"
	"github.com/specboxhq/specbox/mcp"
)

func setupTestServer(t *testing.T) (*server.MCPServer, *storage.FileStore) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	s := mcp.NewServer(store)
	return s, store
}

// callTool sends a JSON-RPC tool call to the server and returns the text content.
func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) string {
	t.Helper()
	ctx := context.Background()

	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}

	msg, err := json.Marshal(rpcReq)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	result := s.HandleMessage(ctx, msg)

	resp, ok := result.(gomcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T: %+v", result, result)
	}

	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var callResult gomcp.CallToolResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("unmarshal CallToolResult: %v", err)
	}

	if len(callResult.Content) == 0 {
		t.Fatal("empty content in result")
	}

	textContent, ok := callResult.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", callResult.Content[0])
	}

	return textContent.Text
}

func seedDoc(t *testing.T, store *storage.FileStore, path, content string) {
	t.Helper()
	if _, err := store.CreateDocument(path, content); err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
}
