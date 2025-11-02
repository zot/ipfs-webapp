package commands

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed demo/*
var demoFS embed.FS

// DemoCmd represents the demo command
var DemoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Run the demo chatroom application",
	Long: `Run the demo chatroom application.
The current directory must be empty.
Creates and serves an embedded chatroom example.`,
	RunE: runDemo,
}

func init() {
	DemoCmd.Flags().BoolVar(&noOpen, "noopen", false, "Do not open browser automatically")
	DemoCmd.Flags().CountVarP(&verbose, "verbose", "v", "Verbose output (can be specified multiple times: -v, -vv, -vvv)")
}

func runDemo(cmd *cobra.Command, args []string) error {
	// Check if directory is empty
	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Filter out hidden files and check if empty
	nonHidden := 0
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 0 && name[0] != '.' {
			nonHidden++
		}
	}

	if nonHidden > 0 {
		return fmt.Errorf("current directory must be empty (found %d non-hidden files/directories)", nonHidden)
	}

	// Extract demo files
	if err := extractDemoFiles(); err != nil {
		return fmt.Errorf("failed to extract demo files: %w", err)
	}

	fmt.Println("Demo files extracted successfully")
	fmt.Println("Starting demo server...")

	// Run serve command
	return runServe(cmd, args)
}

func extractDemoFiles() error {
	// Create directories
	if err := os.MkdirAll("html", 0755); err != nil {
		return err
	}
	if err := os.MkdirAll("ipfs", 0755); err != nil {
		return err
	}
	if err := os.MkdirAll("storage", 0755); err != nil {
		return err
	}

	// Recursively extract all files from demo directory
	return fs.WalkDir(demoFS, "demo", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the demo directory itself
		if path == "demo" {
			return nil
		}

		// Get relative path (remove "demo/" prefix)
		relPath := filepath.Join("html", path[5:])

		if d.IsDir() {
			return os.MkdirAll(relPath, 0755)
		}

		// Read and write file
		data, err := demoFS.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(relPath, data, 0644)
	})
}
