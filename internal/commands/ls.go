package commands

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zot/p2p-webapp/internal/bundle"
)

// LsCmd represents the ls command
var LsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List files in bundled site",
	Long: `List files available in the bundled site.
Shows files that can be copied with the cp command.`,
	RunE: runLs,
}

func runLs(cmd *cobra.Command, args []string) error {
	// Get bundle reader
	zipReader, err := bundle.GetBundleReader()
	if err != nil {
		return fmt.Errorf("failed to read bundle: %w", err)
	}
	if zipReader == nil {
		return fmt.Errorf("binary is not bundled")
	}

	files := []string{}

	// List files from the html/ directory in the bundle
	for _, f := range zipReader.File {
		// Only include files from html/ directory
		if strings.HasPrefix(f.Name, "html/") && !f.FileInfo().IsDir() {
			// Remove html/ prefix
			relPath := strings.TrimPrefix(f.Name, "html/")
			files = append(files, relPath)
		}
	}

	// Sort files alphabetically
	sort.Strings(files)

	// Display files
	if len(files) == 0 {
		fmt.Println("No files found in bundled site")
		return nil
	}

	fmt.Printf("Files available in bundled site (%d):\n\n", len(files))
	for _, file := range files {
		fmt.Println(filepath.Base(file))
	}

	return nil
}
