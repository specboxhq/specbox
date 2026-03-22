package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runConfig() {
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".specbox", "config.yaml")

	// Show config files found
	fmt.Println("Config files:")
	fmt.Println()

	projectConfigs := findProjectConfigs()
	if len(projectConfigs) == 0 {
		fmt.Println("  (no project .specbox.yaml found)")
	} else {
		for _, p := range projectConfigs {
			fmt.Printf("  project:  %s\n", p)
		}
	}

	if _, err := os.Stat(globalPath); err == nil {
		fmt.Printf("  global:   %s\n", globalPath)
	} else {
		fmt.Printf("  global:   %s (not found)\n", globalPath)
	}

	// Show resolved values
	fmt.Println()
	fmt.Println("Resolved values:")
	fmt.Println()

	for _, item := range allResolvedConfig() {
		fmt.Printf("  %-14s %s", item.Key+":", item.Entry.Value)
		if item.Entry.Source != "default" {
			fmt.Printf("  (%s)", shortenSource(item.Entry.Source, home))
		} else {
			fmt.Print("  (default)")
		}
		fmt.Println()
	}

	// Show server credentials
	globalCfg, err := readGlobalConfig()
	if err == nil && len(globalCfg.Servers) > 0 {
		fmt.Println()
		fmt.Println("Servers:")
		fmt.Println()
		activeServer := resolveServerURL()
		for _, s := range globalCfg.Servers {
			prefix := "  "
			if strings.TrimRight(s.URL, "/") == activeServer {
				prefix = "* "
			}
			tokenDisplay := "(no token)"
			if s.AuthToken != "" {
				tokenDisplay = s.AuthToken[:8] + "..."
			}
			name := s.DisplayName()
			if s.Name != "" {
				fmt.Printf("%s%-12s %s  %s\n", prefix, name, s.URL, tokenDisplay)
			} else {
				fmt.Printf("%s%s  %s\n", prefix, s.URL, tokenDisplay)
			}
		}
	}

	// Show shadowed values if any
	allValues := allProjectConfigValues()
	shadowed := findShadowedValues(allValues)
	if len(shadowed) > 0 {
		fmt.Println()
		fmt.Println("Shadowed values (overridden by a closer config):")
		fmt.Println()
		for _, s := range shadowed {
			fmt.Printf("  %-14s %s  (%s)\n", s.Key+":", s.Value, shortenSource(s.Source, home))
		}
	}
}

// findShadowedValues returns values that exist in project configs but are
// overridden by a closer config file (i.e. not the winning value).
func findShadowedValues(allValues []struct {
	Key    string
	Value  string
	Source string
}) []struct {
	Key    string
	Value  string
	Source string
} {
	// Track which keys we've seen — first occurrence wins (closest)
	seen := map[string]bool{}
	var shadowed []struct {
		Key    string
		Value  string
		Source string
	}

	for _, entry := range allValues {
		if seen[entry.Key] {
			shadowed = append(shadowed, entry)
		} else {
			seen[entry.Key] = true
		}
	}
	return shadowed
}

func shortenSource(source, home string) string {
	if strings.HasPrefix(source, "env ") {
		return source
	}
	if home != "" && strings.HasPrefix(source, home) {
		return "~" + source[len(home):]
	}
	return source
}
