package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// GlobalConfig represents ~/.specbox/config.yaml.
type GlobalConfig struct {
	SpecsDir string         `yaml:"specs_dir,omitempty"`
	Servers  []ServerConfig `yaml:"servers,omitempty"`
}

// ServerConfig represents a single server entry in the global config.
type ServerConfig struct {
	Name      string `yaml:"name,omitempty"`
	URL       string `yaml:"url"`
	AuthToken string `yaml:"auth_token,omitempty"`
}

// DisplayName returns the name if set, otherwise the URL.
func (s ServerConfig) DisplayName() string {
	if s.Name != "" {
		return s.Name
	}
	return s.URL
}

// readGlobalConfig reads and parses ~/.specbox/config.yaml.
func readGlobalConfig() (*GlobalConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &GlobalConfig{}, err
	}

	configPath := filepath.Join(home, ".specbox", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return &GlobalConfig{}, nil // file doesn't exist yet
	}

	var cfg GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &GlobalConfig{}, fmt.Errorf("invalid config %s: %w", configPath, err)
	}

	return &cfg, nil
}

// writeGlobalConfig writes the global config to ~/.specbox/config.yaml.
func writeGlobalConfig(cfg *GlobalConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	configDir := filepath.Join(home, ".specbox")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}
	encoder.Close()

	configPath := filepath.Join(configDir, "config.yaml")
	return os.WriteFile(configPath, buf.Bytes(), 0600)
}

// getServerConfig returns the server config for the given URL, or nil if not found.
func (cfg *GlobalConfig) getServerConfig(serverURL string) *ServerConfig {
	normalized := strings.TrimRight(serverURL, "/")
	for i := range cfg.Servers {
		if strings.TrimRight(cfg.Servers[i].URL, "/") == normalized {
			return &cfg.Servers[i]
		}
	}
	return nil
}

// setServerCredentials updates or adds a server entry with the given credentials.
func (cfg *GlobalConfig) setServerCredentials(serverURL, token string) {
	normalized := strings.TrimRight(serverURL, "/")
	for i := range cfg.Servers {
		if strings.TrimRight(cfg.Servers[i].URL, "/") == normalized {
			cfg.Servers[i].AuthToken = token
			return
		}
	}
	cfg.Servers = append(cfg.Servers, ServerConfig{
		URL:       normalized,
		AuthToken: token,
	})
}

// resolveAuthToken returns the auth token for the given server URL.
func resolveAuthToken(serverURL string) string {
	cfg, err := readGlobalConfig()
	if err != nil {
		return ""
	}
	if sc := cfg.getServerConfig(serverURL); sc != nil {
		return sc.AuthToken
	}
	return ""
}

// readSimpleYAML reads a flat key: value YAML file, skipping comments and blank lines.
// Used for project .specbox.yaml files which remain flat.
func readSimpleYAML(path string) map[string]string {
	values := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return values
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return values
}

// findGitRoot walks up the directory tree to find the .git directory.
func findGitRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository")
		}
		dir = parent
	}
}

// findProjectConfig walks up from cwd looking for .specbox.yaml and returns
// the value for the given key, or "" if not found.
func findProjectConfig(key string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		configPath := filepath.Join(dir, ".specbox.yaml")
		if _, err := os.Stat(configPath); err == nil {
			if v := readSimpleYAML(configPath)[key]; v != "" {
				return v
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// resolveConfig looks up a config key using the standard resolution order:
// env var (if provided) → nearest .specbox.yaml (walk up from cwd) → global ~/.specbox/config.yaml → defaultVal.
// Note: for server_url and auth_token, use resolveServerURL() and resolveAuthToken() instead.
func resolveConfig(key, envVar, defaultVal string) string {
	// 1. Environment variable
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			return v
		}
	}

	// 2. Walk up from cwd looking for .specbox.yaml
	if v := findProjectConfig(key); v != "" {
		return v
	}

	// 3. Global config (flat fields only — specs_dir)
	cfg, err := readGlobalConfig()
	if err == nil {
		switch key {
		case "specs_dir":
			if cfg.SpecsDir != "" {
				return cfg.SpecsDir
			}
		}
	}

	// 4. Default
	return defaultVal
}

// resolveDocsDir returns the specs directory.
func resolveDocsDir() string {
	return resolveConfig("specs_dir", "SPECBOX_DIR", ".")
}

// resolveServerURL returns the server URL from env var, project config, or default.
func resolveServerURL() string {
	// 1. Env var
	if u := os.Getenv("SPECBOX_SERVER_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}

	// 2. Project config
	if u := findProjectConfig("server_url"); u != "" {
		return strings.TrimRight(u, "/")
	}

	// 3. Default
	return "https://specbox.io"
}

// resolveAPIURL returns the API base URL (e.g. "https://specbox.io/api").
func resolveAPIURL() string {
	return resolveServerURL() + "/api"
}
