package main

import (
	"fmt"
	"os"

	"github.com/specboxhq/specbox/internal/domain"
	"github.com/specboxhq/specbox/internal/storage"
)

func runCheck() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: specbox check <path>")
		os.Exit(1)
	}
	specPath := os.Args[2]

	docsDir := resolveDocsDir()
	store, err := storage.NewFileStore(docsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	doc, err := store.GetDocument(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Frontmatter
	_, _, fmErr := domain.ParseFrontmatter(doc.Content)
	if fmErr != nil {
		fmt.Printf("Frontmatter: INVALID — %v\n", fmErr)
	} else {
		fmt.Println("Frontmatter: valid")
	}

	// Markups
	markups := domain.ParseMarkups(doc.Content)
	open, resolved := 0, 0
	for _, m := range markups {
		if m.Status == "resolved" {
			resolved++
		} else {
			open++
		}
	}
	fmt.Printf("Markups: %d total (%d open, %d resolved)\n", len(markups), open, resolved)

	// Sync status (stub — no server yet)
	fmt.Println("Sync: never pushed")
}

// resolveDocsDir determines the specs directory.
// Resolution order: SPECBOX_DIR env → .specbox.yaml → ~/.specbox/config.yaml → "." (project root).
