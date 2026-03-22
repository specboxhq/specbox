package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func runWhoami() {
	serverURL := resolveServerURL()
	apiURL := serverURL + "/api"
	token := resolveAuthToken(serverURL)

	if token == "" {
		fmt.Fprintf(os.Stderr, "Not logged in to %s. Run 'specbox login' to authenticate.\n", serverURL)
		os.Exit(1)
	}

	me, err := verifyToken(apiURL, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Your token may be invalid or expired. Run 'specbox login' to re-authenticate.")
		os.Exit(1)
	}

	fmt.Printf("Logged in as %s (%s)\n", me.User.Username, me.User.Email)
	fmt.Printf("Server: %s\n", serverURL)

	home, _ := os.UserHomeDir()
	fmt.Printf("Token stored in %s\n", filepath.Join(home, ".specbox", "config.yaml"))
}
