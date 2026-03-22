package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type mcpConfig struct {
	McpServers map[string]mcpServerEntry `json:"mcpServers,omitempty"`
	Servers    map[string]mcpServerEntry `json:"servers,omitempty"`
}

type mcpServerEntry struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

func runIntegrate() {
	// Find project root
	root, err := findGitRoot()
	if err != nil {
		root, _ = os.Getwd()
	}

	// Find specbox binary
	binary, err := os.Executable()
	if err != nil {
		binary = "specbox"
	}

	// Determine specs dir
	specsDir := "specs"
	if _, err := os.Stat(filepath.Join(root, ".specbox.yaml")); err == nil {
		// TODO: read specs_dir from .specbox.yaml
	}

	fmt.Println("Detecting AI tools...")
	fmt.Println()

	detected := false

	// Check for VSCode
	vscodeDir := filepath.Join(root, ".vscode")
	if dirExists(vscodeDir) {
		detected = true
		configureVSCode(root, vscodeDir, binary, specsDir)
	}

	// Check for Claude Code
	claudeDir := filepath.Join(root, ".claude")
	if dirExists(claudeDir) || fileExists(filepath.Join(root, "CLAUDE.md")) {
		detected = true
		configureClaude(root, binary, specsDir)
	}

	// Check for Cursor
	cursorConfig := filepath.Join(root, ".cursor")
	if dirExists(cursorConfig) {
		detected = true
		configureCursor(root, binary, specsDir)
	}

	// Check for Windsurf
	windsurfConfig := filepath.Join(root, ".windsurf")
	if dirExists(windsurfConfig) {
		detected = true
		fmt.Println("  Windsurf: detected")
		fmt.Println("    Add MCP config manually — see https://specbox.dev/docs/setup")
		fmt.Println()
	}

	// Always offer .mcp.json
	mcpJSON := filepath.Join(root, ".mcp.json")
	if !fileExists(mcpJSON) {
		detected = true
		configureMCPJSON(root, mcpJSON, binary, specsDir)
	}

	if !detected {
		fmt.Println("No AI tools detected.")
		fmt.Println("Create a .mcp.json manually or see https://specbox.dev/docs/setup")
	}
}

func configureVSCode(root, vscodeDir, binary, specsDir string) {
	mcpFile := filepath.Join(vscodeDir, "mcp.json")
	fmt.Print("  VSCode: detected")

	if fileExists(mcpFile) {
		// Check if specbox is already configured
		data, err := os.ReadFile(mcpFile)
		if err == nil {
			var cfg mcpConfig
			if json.Unmarshal(data, &cfg) == nil {
				if _, ok := cfg.Servers["specbox"]; ok {
					fmt.Println(" (specbox already configured)")
					return
				}
			}
		}
	}

	cfg := mcpConfig{
		Servers: map[string]mcpServerEntry{
			"specbox": {
				Type:    "stdio",
				Command: binary,
				Args:    []string{"mcp"},
				Env:     map[string]string{"SPECBOX_DIR": filepath.Join(root, specsDir)},
			},
		},
	}

	if err := writeMCPConfig(mcpFile, cfg); err != nil {
		fmt.Printf("\n    Error writing %s: %v\n", mcpFile, err)
		return
	}
	fmt.Printf("\n    Wrote %s\n", mcpFile)
	fmt.Println()
}

func configureClaude(root, binary, specsDir string) {
	mcpFile := filepath.Join(root, ".mcp.json")
	fmt.Print("  Claude Code: detected")

	if fileExists(mcpFile) {
		data, err := os.ReadFile(mcpFile)
		if err == nil {
			var cfg mcpConfig
			if json.Unmarshal(data, &cfg) == nil {
				if _, ok := cfg.McpServers["specbox"]; ok {
					fmt.Println(" (specbox already configured)")
					return
				}
			}
		}
	}

	cfg := mcpConfig{
		McpServers: map[string]mcpServerEntry{
			"specbox": {
				Command: binary,
				Args:    []string{"mcp"},
				Env:     map[string]string{"SPECBOX_DIR": filepath.Join(root, specsDir)},
			},
		},
	}

	if err := writeMCPConfig(mcpFile, cfg); err != nil {
		fmt.Printf("\n    Error writing %s: %v\n", mcpFile, err)
		return
	}
	fmt.Printf("\n    Wrote %s\n", mcpFile)
	fmt.Println()
}

func configureCursor(root, binary, specsDir string) {
	mcpFile := filepath.Join(root, ".cursor", "mcp.json")
	fmt.Print("  Cursor: detected")

	if fileExists(mcpFile) {
		data, err := os.ReadFile(mcpFile)
		if err == nil {
			var cfg mcpConfig
			if json.Unmarshal(data, &cfg) == nil {
				if _, ok := cfg.McpServers["specbox"]; ok {
					fmt.Println(" (specbox already configured)")
					return
				}
			}
		}
	}

	cfg := mcpConfig{
		McpServers: map[string]mcpServerEntry{
			"specbox": {
				Command: binary,
				Args:    []string{"mcp"},
				Env:     map[string]string{"SPECBOX_DIR": filepath.Join(root, specsDir)},
			},
		},
	}

	if err := writeMCPConfig(mcpFile, cfg); err != nil {
		fmt.Printf("\n    Error writing %s: %v\n", mcpFile, err)
		return
	}
	fmt.Printf("\n    Wrote %s\n", mcpFile)
	fmt.Println()
}

func configureMCPJSON(root, mcpFile, binary, specsDir string) {
	fmt.Print("  .mcp.json: not found")

	cfg := mcpConfig{
		McpServers: map[string]mcpServerEntry{
			"specbox": {
				Command: binary,
				Args:    []string{"mcp"},
				Env:     map[string]string{"SPECBOX_DIR": filepath.Join(root, specsDir)},
			},
		},
	}

	if err := writeMCPConfig(mcpFile, cfg); err != nil {
		fmt.Printf("\n    Error writing %s: %v\n", mcpFile, err)
		return
	}
	fmt.Printf("\n    Created %s\n", mcpFile)
	fmt.Println()
}

func writeMCPConfig(path string, cfg mcpConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
