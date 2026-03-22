package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runInit() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}
	globalDir := filepath.Join(home, ".specbox")
	globalConfig := filepath.Join(globalDir, "config.yaml")

	// Ensure global config directory exists
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create %s: %v\n", globalDir, err)
		os.Exit(1)
	}

	// Read global config if it exists, or create it
	globalValues := map[string]string{}
	if _, err := os.Stat(globalConfig); os.IsNotExist(err) {
		defaultConfig := "# Specbox global configuration\napi_url: https://api.specbox.io\n# Uncomment to set a default specs folder\n# specs_dir: specs\n"
		if err := os.WriteFile(globalConfig, []byte(defaultConfig), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", globalConfig, err)
			os.Exit(1)
		}
		fmt.Printf("Created %s\n", globalConfig)
	} else {
		globalValues = readSimpleYAML(globalConfig)
		fmt.Printf("Read %s\n", globalConfig)
	}

	// Detect git root
	projectRoot, err := findGitRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: not inside a git repository. Using current directory.")
		projectRoot, _ = os.Getwd()
	}

	// Determine specs_dir: use global config value if set, otherwise default commented out
	specsDir := globalValues["specs_dir"]

	// Create .specbox.yaml if it doesn't exist
	projectConfig := filepath.Join(projectRoot, ".specbox.yaml")
	if _, err := os.Stat(projectConfig); os.IsNotExist(err) {
		var cfg string
		if specsDir != "" {
			cfg = fmt.Sprintf("# Specbox project configuration\nspecs_dir: %s\n", specsDir)
		} else {
			cfg = "# Specbox project configuration\n# Uncomment to set a specs folder\n# specs_dir: specs\n"
		}
		if err := os.WriteFile(projectConfig, []byte(cfg), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", projectConfig, err)
			os.Exit(1)
		}
		fmt.Printf("Created %s\n", projectConfig)
	} else {
		fmt.Printf("Project config already exists: %s\n", projectConfig)
	}

	fmt.Println("Done! Run 'specbox integrate' to configure your AI tools.")
}

// readSimpleYAML reads a flat key: value YAML file, skipping comments and blank lines.
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
