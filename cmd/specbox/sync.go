package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/specboxhq/specbox/internal/api"
	"github.com/specboxhq/specbox/internal/domain"
)

// --- Push ---

func runPush() {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: specbox push <path>")
		os.Exit(1)
	}

	path := fs.Arg(0)
	client := requireAuthClient()

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot read %s: %v\n", path, err)
		os.Exit(1)
	}
	rawContent := string(content)

	// Derive slug from filename
	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	// Apply private config as default visibility
	var metadata map[string]any
	if resolvePrivate() {
		hasVisibility := false
		if fmData, _, fmErr := domain.ParseFrontmatter(rawContent); fmErr == nil && fmData != nil {
			if specbox, ok := fmData["specbox"].(map[string]any); ok {
				if meta, ok := specbox["metadata"].(map[string]any); ok {
					if _, ok := meta["visibility"]; ok {
						hasVisibility = true
					}
				}
			}
		}
		if !hasVisibility {
			metadata = map[string]any{"visibility": "private"}
		}
	}

	result, statusCode, err := client.Push(rawContent, slug, metadata)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch statusCode {
	case 200:
		handlePushSuccess(path, rawContent, result)

	case 409:
		handlePushConflict(path, rawContent, result)

	case 413:
		fmt.Fprintln(os.Stderr, "Error: spec is too large (max 1MB)")
		os.Exit(1)

	case 422:
		fmt.Fprintln(os.Stderr, "Error: markup validation failed:")
		for _, e := range result.Errors {
			if e.Line > 0 {
				fmt.Fprintf(os.Stderr, "  line %d: %s\n", e.Line, e.Message)
			} else {
				fmt.Fprintf(os.Stderr, "  %s\n", e.Message)
			}
		}
		os.Exit(1)

	case 423:
		fmt.Fprintln(os.Stderr, "Error: spec is locked. Use 'specbox set' for metadata-only changes.")
		os.Exit(1)

	case 403:
		fmt.Fprintln(os.Stderr, "Error: forbidden — "+result.Errors[0].Message)
		os.Exit(1)

	default:
		fmt.Fprintf(os.Stderr, "Error: server returned %d\n", statusCode)
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %s\n", e.Message)
		}
		os.Exit(1)
	}
}

func handlePushSuccess(path, rawContent string, result *api.SpecResponse) {
	spec := result.Spec
	if spec == nil {
		fmt.Fprintln(os.Stderr, "Error: unexpected empty response")
		os.Exit(1)
	}

	if spec.Merged {
		newContent := spec.Frontmatter + "\n" + spec.RawContent
		if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("Pushed %s (v%d, merged)\n", path, spec.Version)
	} else {
		newContent := api.ReplaceFrontmatter(rawContent, spec.Frontmatter)
		if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("Pushed %s (v%d)\n", path, spec.Version)
	}

	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "Warning: %s — %s\n", e.Field, e.Message)
	}
}

func handlePushConflict(path, rawContent string, result *api.SpecResponse) {
	spec := result.Spec
	if spec == nil {
		fmt.Fprintln(os.Stderr, "Error: merge conflict but no content returned")
		os.Exit(1)
	}

	// Keep existing frontmatter, replace body with merged content
	if strings.HasPrefix(rawContent, "---\n") {
		rest := rawContent[4:]
		idx := strings.Index(rest, "\n---\n")
		if idx >= 0 {
			frontmatter := rawContent[:4+idx+5]
			newContent := frontmatter + spec.MergedContent
			_ = os.WriteFile(path, []byte(newContent), 0644)
		}
	} else {
		_ = os.WriteFile(path, []byte(spec.MergedContent), 0644)
	}

	fmt.Fprintln(os.Stderr, "Merge conflict — resolve markers in "+path+" and push again.")
	os.Exit(1)
}

// --- Pull ---

func runPull() {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: specbox pull <path>")
		os.Exit(1)
	}

	path := fs.Arg(0)
	client := requireAuthClient()

	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot read %s: %v\n", path, err)
		os.Exit(1)
	}
	rawContent := string(content)

	specID := api.ExtractSpecID(rawContent)
	if specID == "" {
		fmt.Fprintln(os.Stderr, "Error: spec has not been pushed yet (no specbox.id in frontmatter)")
		os.Exit(1)
	}

	hash := api.ContentHash(rawContent)

	result, statusCode, err := client.Pull(specID, hash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch statusCode {
	case 200:
		spec := result.Spec
		if spec == nil {
			fmt.Fprintln(os.Stderr, "Error: unexpected empty response")
			os.Exit(1)
		}

		if spec.RawContent != "" {
			newContent := spec.Frontmatter + "\n" + spec.RawContent
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to write %s: %v\n", path, err)
				os.Exit(1)
			}
			fmt.Printf("Pulled %s (v%d, content updated)\n", path, spec.Version)
		} else {
			newContent := api.ReplaceFrontmatter(rawContent, spec.Frontmatter)
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to write %s: %v\n", path, err)
				os.Exit(1)
			}
			fmt.Printf("Pulled %s (v%d, up to date)\n", path, spec.Version)
		}

	case 404:
		fmt.Fprintln(os.Stderr, "Error: spec not found on server")
		os.Exit(1)

	case 412:
		fmt.Fprintln(os.Stderr, "Error: local content has changed. Push first to sync your changes.")
		os.Exit(1)

	default:
		fmt.Fprintf(os.Stderr, "Error: server returned %d\n", statusCode)
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %s\n", e.Message)
		}
		os.Exit(1)
	}
}

// --- Set ---

func runSet() {
	fs := flag.NewFlagSet("set", flag.ExitOnError)
	specID := fs.String("id", "", "Spec ID (skip local file)")
	fs.Parse(os.Args[2:])

	args := fs.Args()
	var path string
	var kvPairs []string

	if *specID != "" {
		kvPairs = args
	} else {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: specbox set <path> <key=value> ...")
			fmt.Fprintln(os.Stderr, "       specbox set --id <id> <key=value> ...")
			os.Exit(1)
		}
		path = args[0]
		kvPairs = args[1:]
	}

	metadata := map[string]any{}
	for _, kv := range kvPairs {
		key, val, err := parseKVPair(kv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		metadata[key] = val
	}

	if len(metadata) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no metadata fields provided")
		os.Exit(1)
	}

	client := requireAuthClient()

	id := *specID
	var rawContent string
	if id == "" {
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot read %s: %v\n", path, err)
			os.Exit(1)
		}
		rawContent = string(content)
		id = api.ExtractSpecID(rawContent)
		if id == "" {
			fmt.Fprintln(os.Stderr, "Error: spec has not been pushed yet (no specbox.id in frontmatter)")
			os.Exit(1)
		}
	}

	result, statusCode, err := client.Set(id, metadata)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch statusCode {
	case 200:
		spec := result.Spec
		if spec == nil {
			fmt.Fprintln(os.Stderr, "Error: unexpected empty response")
			os.Exit(1)
		}

		if path != "" && spec.Frontmatter != "" {
			newContent := api.ReplaceFrontmatter(rawContent, spec.Frontmatter)
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to write %s: %v\n", path, err)
				os.Exit(1)
			}
		}

		fmt.Printf("Updated %s\n", id)
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "Warning: %s — %s\n", e.Field, e.Message)
		}

	case 404:
		fmt.Fprintln(os.Stderr, "Error: spec not found")
		os.Exit(1)

	default:
		fmt.Fprintf(os.Stderr, "Error: server returned %d\n", statusCode)
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %s\n", e.Message)
		}
		os.Exit(1)
	}
}

// parseKVPair parses a "key=value" string into a key and typed value.
// Booleans, null/nil/empty, comma-separated arrays, and plain strings are supported.
func parseKVPair(kv string) (string, any, error) {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid key=value: %s", kv)
	}
	key, value := parts[0], parts[1]
	switch {
	case value == "true":
		return key, true, nil
	case value == "false":
		return key, false, nil
	case value == "null" || value == "nil" || value == "":
		return key, nil, nil
	case strings.Contains(value, ","):
		items := strings.Split(value, ",")
		trimmed := make([]string, 0, len(items))
		for _, item := range items {
			if s := strings.TrimSpace(item); s != "" {
				trimmed = append(trimmed, s)
			}
		}
		return key, trimmed, nil
	default:
		return key, value, nil
	}
}

// --- Auth helper ---

func requireAuthClient() *api.Client {
	serverURL := resolveServerURL()
	token := resolveAuthToken(serverURL)
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: not logged in. Run 'specbox login' first.")
		os.Exit(1)
	}
	return &api.Client{APIURL: resolveAPIURL(), Token: token}
}
