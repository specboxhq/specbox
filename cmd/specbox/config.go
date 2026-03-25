package main

import (
	"github.com/specboxhq/specbox/internal/config"
)

// Thin wrappers and type aliases around the shared config package.
// These exist so existing call sites don't need to change their function names.

type GlobalConfig = config.GlobalConfig
type ServerConfig = config.ServerConfig

func readGlobalConfig() (*config.GlobalConfig, error) { return config.ReadGlobalConfig() }
func writeGlobalConfig(cfg *config.GlobalConfig) error { return config.WriteGlobalConfig(cfg) }
func resolveDocsDir() string                           { return config.ResolveDocsDir() }
func resolveServerURL() string                         { return config.ResolveServerURL() }
func resolveAPIURL() string                            { return config.ResolveAPIURL() }
func resolveAuthToken(serverURL string) string         { return config.ResolveAuthToken(serverURL) }
func saveCredentials(serverURL, token string) error    { return config.SaveCredentials(serverURL, token) }
func findGitRoot() (string, error)                     { return config.FindGitRoot() }
func findProjectConfigs() []string                     { return config.FindProjectConfigs() }
func findProjectConfig(key string) string {
	for _, configPath := range config.FindProjectConfigs() {
		if v := config.ReadSimpleYAML(configPath)[key]; v != "" {
			return v
		}
	}
	return ""
}
func readSimpleYAML(path string) map[string]string     { return config.ReadSimpleYAML(path) }
func resolvePrivate() bool                              { return config.ResolvePrivate() }

// configEntry is an alias for display in configcmd.go.
type configEntry = config.Entry

func resolveConfigWithSource(key, envVar, defaultVal string) configEntry {
	return config.ResolveWithSource(key, envVar, defaultVal)
}

func allResolvedConfig() []struct {
	Key   string
	Entry configEntry
} {
	return config.AllResolved()
}

func allProjectConfigValues() []struct {
	Key    string
	Value  string
	Source string
} {
	return config.AllProjectConfigValues()
}
