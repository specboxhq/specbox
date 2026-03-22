package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	var globalCfg *GlobalConfig
	if _, err := os.Stat(globalConfig); os.IsNotExist(err) {
		globalCfg = &GlobalConfig{}
		if err := writeGlobalConfig(globalCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", globalConfig, err)
			os.Exit(1)
		}
		fmt.Printf("Created %s\n", globalConfig)
	} else {
		globalCfg, err = readGlobalConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot read %s: %v\n", globalConfig, err)
			os.Exit(1)
		}
		fmt.Printf("Read %s\n", globalConfig)
	}

	// Detect git root
	projectRoot, err := findGitRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: not inside a git repository. Using current directory.")
		projectRoot, _ = os.Getwd()
	}

	// Determine specs_dir: use global config value if set, otherwise default commented out
	specsDir := globalCfg.SpecsDir

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

