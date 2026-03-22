// Package config provides configuration resolution for the specbox CLI and MCP server.
//
// Resolution order: env var → .specbox.yaml (walk up from cwd, closest wins) → ~/.specbox/config.yaml → default.
package config

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

// GetServerConfig returns the server config for the given URL, or nil if not found.
func (cfg *GlobalConfig) GetServerConfig(serverURL string) *ServerConfig {
	normalized := strings.TrimRight(serverURL, "/")
	for i := range cfg.Servers {
		if strings.TrimRight(cfg.Servers[i].URL, "/") == normalized {
			return &cfg.Servers[i]
		}
	}
	return nil
}

// SetServerCredentials updates or adds a server entry with the given credentials.
func (cfg *GlobalConfig) SetServerCredentials(serverURL, token string) {
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

// ReadGlobalConfig reads and parses ~/.specbox/config.yaml.
func ReadGlobalConfig() (*GlobalConfig, error) {
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

// WriteGlobalConfig writes the global config to ~/.specbox/config.yaml.
func WriteGlobalConfig(cfg *GlobalConfig) error {
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

// Entry holds a resolved value and where it came from.
type Entry struct {
	Value  string
	Source string // e.g. "env SPECBOX_DIR", "/path/.specbox.yaml", "~/.specbox/config.yaml", "default"
}

// FindProjectConfigs walks up from cwd collecting all .specbox.yaml files found,
// ordered from closest (highest priority) to furthest (lowest priority).
func FindProjectConfigs() []string {
	var configs []string
	dir, err := os.Getwd()
	if err != nil {
		return configs
	}
	for {
		configPath := filepath.Join(dir, ".specbox.yaml")
		if _, err := os.Stat(configPath); err == nil {
			configs = append(configs, configPath)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return configs
}

// ReadSimpleYAML reads a flat key: value YAML file, skipping comments and blank lines.
func ReadSimpleYAML(path string) map[string]string {
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

// ResolveWithSource looks up a config key and returns both the value and its source.
func ResolveWithSource(key, envVar, defaultVal string) Entry {
	// 1. Environment variable
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			return Entry{Value: v, Source: "env " + envVar}
		}
	}

	// 2. Walk up from cwd — shadowed merge, closest wins
	for _, configPath := range FindProjectConfigs() {
		if v := ReadSimpleYAML(configPath)[key]; v != "" {
			return Entry{Value: v, Source: configPath}
		}
	}

	// 3. Global config (flat fields only — specs_dir)
	cfg, err := ReadGlobalConfig()
	if err == nil {
		home, _ := os.UserHomeDir()
		globalPath := filepath.Join(home, ".specbox", "config.yaml")
		switch key {
		case "specs_dir":
			if cfg.SpecsDir != "" {
				return Entry{Value: cfg.SpecsDir, Source: globalPath}
			}
		}
	}

	// 4. Default
	return Entry{Value: defaultVal, Source: "default"}
}

// Resolve looks up a config key using the standard resolution order.
func Resolve(key, envVar, defaultVal string) string {
	return ResolveWithSource(key, envVar, defaultVal).Value
}

// ResolveDocsDir returns the specs directory.
func ResolveDocsDir() string {
	return Resolve("specs_dir", "SPECBOX_DIR", ".")
}

// ResolveServerURL returns the server URL from env var, project config, or default.
func ResolveServerURL() string {
	entry := ResolveWithSource("server_url", "SPECBOX_SERVER_URL", "https://specbox.io")
	return strings.TrimRight(entry.Value, "/")
}

// ResolveAPIURL returns the API base URL (e.g. "https://specbox.io/api").
func ResolveAPIURL() string {
	return ResolveServerURL() + "/api"
}

// ResolveAuthToken returns the auth token for the given server URL.
func ResolveAuthToken(serverURL string) string {
	cfg, err := ReadGlobalConfig()
	if err != nil {
		return ""
	}
	if sc := cfg.GetServerConfig(serverURL); sc != nil {
		return sc.AuthToken
	}
	return ""
}

// SaveCredentials stores an auth token for a server in the global config.
func SaveCredentials(serverURL, token string) error {
	cfg, err := ReadGlobalConfig()
	if err != nil {
		return err
	}
	cfg.SetServerCredentials(serverURL, token)
	return WriteGlobalConfig(cfg)
}

// FindGitRoot walks up the directory tree to find the .git directory.
func FindGitRoot() (string, error) {
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

// AllResolved returns all known config keys with their resolved values and sources.
func AllResolved() []struct {
	Key   string
	Entry Entry
} {
	return []struct {
		Key   string
		Entry Entry
	}{
		{"specs_dir", ResolveWithSource("specs_dir", "SPECBOX_DIR", ".")},
		{"server_url", Entry{Value: ResolveServerURL(), Source: ResolveWithSource("server_url", "SPECBOX_SERVER_URL", "https://specbox.io").Source}},
	}
}

// AllProjectConfigValues returns all key-value pairs from all .specbox.yaml files.
func AllProjectConfigValues() []struct {
	Key    string
	Value  string
	Source string
} {
	var entries []struct {
		Key    string
		Value  string
		Source string
	}
	for _, configPath := range FindProjectConfigs() {
		for k, v := range ReadSimpleYAML(configPath) {
			entries = append(entries, struct {
				Key    string
				Value  string
				Source string
			}{k, v, configPath})
		}
	}
	return entries
}
