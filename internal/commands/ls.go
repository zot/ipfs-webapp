package commands

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

// LsCmd represents the ls command
var LsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List files in embedded demo directory",
	Long: `List files available in the embedded demo directory.
Shows files that can be copied with the cp command.`,
	RunE: runLs,
}

func runLs(cmd *cobra.Command, args []string) error {
	files := []string{}

	// Walk the embedded filesystem
	err := fs.WalkDir(demoFS, "demo", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the demo directory itself
		if path == "demo" {
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get filename without demo/ prefix
		relPath := path[5:] // Remove "demo/" prefix
		files = append(files, relPath)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	// Sort files alphabetically
	sort.Strings(files)

	// Display files
	if len(files) == 0 {
		fmt.Println("No files found in embedded demo directory")
		return nil
	}

	fmt.Printf("Files available in embedded demo directory (%d):\n\n", len(files))
	for _, file := range files {
		fmt.Println(filepath.Base(file))
	}

	return nil
}
