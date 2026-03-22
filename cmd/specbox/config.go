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

// configEntry holds a resolved value and where it came from.
type configEntry struct {
	Value  string
	Source string // e.g. "env SPECBOX_DIR", "/home/doug/projects/.specbox.yaml", "~/.specbox/config.yaml", "default"
}

// findProjectConfigs walks up from cwd collecting all .specbox.yaml files found,
// ordered from closest (highest priority) to furthest (lowest priority).
func findProjectConfigs() []string {
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

// findProjectConfig looks up a key across all .specbox.yaml files walking up from cwd.
// Closest file wins (shadowing). Returns the value or "".
func findProjectConfig(key string) string {
	for _, configPath := range findProjectConfigs() {
		if v := readSimpleYAML(configPath)[key]; v != "" {
			return v
		}
	}
	return ""
}

// resolveConfigWithSource looks up a config key and returns both the value and its source.
func resolveConfigWithSource(key, envVar, defaultVal string) configEntry {
	// 1. Environment variable
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			return configEntry{Value: v, Source: "env " + envVar}
		}
	}

	// 2. Walk up from cwd — shadowed merge, closest wins
	for _, configPath := range findProjectConfigs() {
		if v := readSimpleYAML(configPath)[key]; v != "" {
			return configEntry{Value: v, Source: configPath}
		}
	}

	// 3. Global config (flat fields only — specs_dir)
	cfg, err := readGlobalConfig()
	if err == nil {
		home, _ := os.UserHomeDir()
		globalPath := filepath.Join(home, ".specbox", "config.yaml")
		switch key {
		case "specs_dir":
			if cfg.SpecsDir != "" {
				return configEntry{Value: cfg.SpecsDir, Source: globalPath}
			}
		}
	}

	// 4. Default
	return configEntry{Value: defaultVal, Source: "default"}
}

// resolveConfig looks up a config key using the standard resolution order:
// env var (if provided) → .specbox.yaml files (walk up from cwd, closest wins) → global ~/.specbox/config.yaml → defaultVal.
// Note: for server_url and auth_token, use resolveServerURL() and resolveAuthToken() instead.
func resolveConfig(key, envVar, defaultVal string) string {
	return resolveConfigWithSource(key, envVar, defaultVal).Value
}

// resolveDocsDir returns the specs directory.
func resolveDocsDir() string {
	return resolveConfig("specs_dir", "SPECBOX_DIR", ".")
}

// resolveServerURL returns the server URL from env var, project config, or default.
func resolveServerURL() string {
	entry := resolveConfigWithSource("server_url", "SPECBOX_SERVER_URL", "https://specbox.io")
	return strings.TrimRight(entry.Value, "/")
}

// resolveAPIURL returns the API base URL (e.g. "https://specbox.io/api").
func resolveAPIURL() string {
	return resolveServerURL() + "/api"
}

// allResolvedConfig returns all known config keys with their resolved values and sources.
// Used by the `specbox config` command.
func allResolvedConfig() []struct {
	Key   string
	Entry configEntry
} {
	return []struct {
		Key   string
		Entry configEntry
	}{
		{"specs_dir", resolveConfigWithSource("specs_dir", "SPECBOX_DIR", ".")},
		{"server_url", configEntry{Value: resolveServerURL(), Source: resolveConfigWithSource("server_url", "SPECBOX_SERVER_URL", "https://specbox.io").Source}},
	}
}

// allProjectConfigValues returns all key-value pairs from all .specbox.yaml files
// walking up from cwd, with source paths. Used by `specbox config` to show shadowed values.
func allProjectConfigValues() []struct {
	Key    string
	Value  string
	Source string
} {
	var entries []struct {
		Key    string
		Value  string
		Source string
	}
	for _, configPath := range findProjectConfigs() {
		for k, v := range readSimpleYAML(configPath) {
			entries = append(entries, struct {
				Key    string
				Value  string
				Source string
			}{k, v, configPath})
		}
	}
	return entries
}
