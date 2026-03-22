package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSimpleYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	os.WriteFile(path, []byte("key1: value1\nkey2: value2\n# comment\n\nkey3: value3\n"), 0644)

	values := readSimpleYAML(path)

	if values["key1"] != "value1" {
		t.Errorf("key1 = %q, want %q", values["key1"], "value1")
	}
	if values["key2"] != "value2" {
		t.Errorf("key2 = %q, want %q", values["key2"], "value2")
	}
	if values["key3"] != "value3" {
		t.Errorf("key3 = %q, want %q", values["key3"], "value3")
	}
}

func TestReadSimpleYAMLSkipsComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	os.WriteFile(path, []byte("# this is a comment\n# key: should_be_skipped\nreal: value\n"), 0644)

	values := readSimpleYAML(path)

	if _, ok := values["# this is a comment"]; ok {
		t.Error("should skip comment lines")
	}
	if values["real"] != "value" {
		t.Errorf("real = %q, want %q", values["real"], "value")
	}
}

func TestReadSimpleYAMLMissingFile(t *testing.T) {
	values := readSimpleYAML("/nonexistent/path.yaml")
	if len(values) != 0 {
		t.Errorf("expected empty map, got %v", values)
	}
}

func TestGlobalConfigReadWrite(t *testing.T) {
	// Override HOME to use a temp dir
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &GlobalConfig{
		SpecsDir: "docs",
		Servers: []ServerConfig{
			{Name: "local", URL: "http://localhost:4000", AuthToken: "test-token"},
		},
	}

	if err := writeGlobalConfig(cfg); err != nil {
		t.Fatalf("writeGlobalConfig: %v", err)
	}

	loaded, err := readGlobalConfig()
	if err != nil {
		t.Fatalf("readGlobalConfig: %v", err)
	}

	if loaded.SpecsDir != "docs" {
		t.Errorf("SpecsDir = %q, want %q", loaded.SpecsDir, "docs")
	}
	if len(loaded.Servers) != 1 {
		t.Fatalf("len(Servers) = %d, want 1", len(loaded.Servers))
	}
	if loaded.Servers[0].Name != "local" {
		t.Errorf("Server name = %q, want %q", loaded.Servers[0].Name, "local")
	}
	if loaded.Servers[0].URL != "http://localhost:4000" {
		t.Errorf("Server URL = %q, want %q", loaded.Servers[0].URL, "http://localhost:4000")
	}
	if loaded.Servers[0].AuthToken != "test-token" {
		t.Errorf("AuthToken = %q, want %q", loaded.Servers[0].AuthToken, "test-token")
	}
}

func TestGlobalConfigMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg, err := readGlobalConfig()
	if err != nil {
		t.Fatalf("readGlobalConfig: %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected no servers, got %d", len(cfg.Servers))
	}
}

func TestSetServerCredentials(t *testing.T) {
	cfg := &GlobalConfig{}

	// Add new server
	cfg.setServerCredentials("http://localhost:4000", "token1")
	if len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(cfg.Servers))
	}
	if cfg.Servers[0].AuthToken != "token1" {
		t.Errorf("AuthToken = %q, want %q", cfg.Servers[0].AuthToken, "token1")
	}

	// Update existing server
	cfg.setServerCredentials("http://localhost:4000", "token2")
	if len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 server after update, got %d", len(cfg.Servers))
	}
	if cfg.Servers[0].AuthToken != "token2" {
		t.Errorf("AuthToken = %q, want %q", cfg.Servers[0].AuthToken, "token2")
	}

	// Add second server
	cfg.setServerCredentials("https://specbox.io", "token3")
	if len(cfg.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(cfg.Servers))
	}
}

func TestSetServerCredentialsNormalizesTrailingSlash(t *testing.T) {
	cfg := &GlobalConfig{}

	cfg.setServerCredentials("http://localhost:4000/", "token1")
	cfg.setServerCredentials("http://localhost:4000", "token2")

	if len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 server (trailing slash normalized), got %d", len(cfg.Servers))
	}
	if cfg.Servers[0].AuthToken != "token2" {
		t.Errorf("AuthToken = %q, want %q", cfg.Servers[0].AuthToken, "token2")
	}
}

func TestGetServerConfig(t *testing.T) {
	cfg := &GlobalConfig{
		Servers: []ServerConfig{
			{URL: "http://localhost:4000", AuthToken: "local-token"},
			{URL: "https://specbox.io", AuthToken: "prod-token"},
		},
	}

	sc := cfg.getServerConfig("http://localhost:4000")
	if sc == nil || sc.AuthToken != "local-token" {
		t.Errorf("expected local-token, got %v", sc)
	}

	sc = cfg.getServerConfig("https://specbox.io")
	if sc == nil || sc.AuthToken != "prod-token" {
		t.Errorf("expected prod-token, got %v", sc)
	}

	sc = cfg.getServerConfig("https://unknown.com")
	if sc != nil {
		t.Errorf("expected nil for unknown server, got %v", sc)
	}
}

func TestGetServerConfigNormalizesTrailingSlash(t *testing.T) {
	cfg := &GlobalConfig{
		Servers: []ServerConfig{
			{URL: "http://localhost:4000", AuthToken: "token"},
		},
	}

	sc := cfg.getServerConfig("http://localhost:4000/")
	if sc == nil {
		t.Error("expected to find server with trailing slash lookup")
	}
}

func TestServerConfigDisplayName(t *testing.T) {
	s := ServerConfig{Name: "local", URL: "http://localhost:4000"}
	if s.DisplayName() != "local" {
		t.Errorf("DisplayName = %q, want %q", s.DisplayName(), "local")
	}

	s = ServerConfig{URL: "http://localhost:4000"}
	if s.DisplayName() != "http://localhost:4000" {
		t.Errorf("DisplayName = %q, want %q", s.DisplayName(), "http://localhost:4000")
	}
}

func TestFindProjectConfigsShadowing(t *testing.T) {
	// Create nested dirs with configs
	root := t.TempDir()
	child := filepath.Join(root, "child")
	os.MkdirAll(child, 0755)

	os.WriteFile(filepath.Join(root, ".specbox.yaml"), []byte("server_url: https://specbox.io\nspecs_dir: specs\n"), 0644)
	os.WriteFile(filepath.Join(child, ".specbox.yaml"), []byte("server_url: http://localhost:4000\n"), 0644)

	// Change to child dir
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(child)

	// findProjectConfig should return child's value (closest wins)
	if v := findProjectConfig("server_url"); v != "http://localhost:4000" {
		t.Errorf("server_url = %q, want %q (child should shadow parent)", v, "http://localhost:4000")
	}

	// specs_dir should fall through to parent since child doesn't set it
	if v := findProjectConfig("specs_dir"); v != "specs" {
		t.Errorf("specs_dir = %q, want %q (should fall through to parent)", v, "specs")
	}

	// nonexistent key returns ""
	if v := findProjectConfig("nonexistent"); v != "" {
		t.Errorf("nonexistent = %q, want empty", v)
	}
}

func TestResolveConfigWithSourceEnvVar(t *testing.T) {
	t.Setenv("SPECBOX_DIR", "/custom/dir")

	entry := resolveConfigWithSource("specs_dir", "SPECBOX_DIR", ".")
	if entry.Value != "/custom/dir" {
		t.Errorf("Value = %q, want %q", entry.Value, "/custom/dir")
	}
	if entry.Source != "env SPECBOX_DIR" {
		t.Errorf("Source = %q, want %q", entry.Source, "env SPECBOX_DIR")
	}
}

func TestResolveConfigWithSourceDefault(t *testing.T) {
	// Ensure no env var and no config files
	t.Setenv("HOME", t.TempDir())

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(t.TempDir())

	entry := resolveConfigWithSource("specs_dir", "SPECBOX_DIR", "fallback")
	if entry.Value != "fallback" {
		t.Errorf("Value = %q, want %q", entry.Value, "fallback")
	}
	if entry.Source != "default" {
		t.Errorf("Source = %q, want %q", entry.Source, "default")
	}
}

func TestFindShadowedValues(t *testing.T) {
	allValues := []struct {
		Key    string
		Value  string
		Source string
	}{
		{"server_url", "http://localhost:4000", "/child/.specbox.yaml"},
		{"server_url", "https://specbox.io", "/root/.specbox.yaml"},
		{"specs_dir", "docs", "/root/.specbox.yaml"},
	}

	shadowed := findShadowedValues(allValues)
	if len(shadowed) != 1 {
		t.Fatalf("expected 1 shadowed value, got %d", len(shadowed))
	}
	if shadowed[0].Value != "https://specbox.io" {
		t.Errorf("shadowed value = %q, want %q", shadowed[0].Value, "https://specbox.io")
	}
}
