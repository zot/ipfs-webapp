package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zot/p2p-webapp/internal/bundle"
)

// ExtractCmd represents the extract command
var ExtractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract the bundled site to the current directory",
	Long: `Extract the bundled site to the current directory.
The current directory must be empty.

After extraction, run 'p2p-webapp serve --dir .' to start the server.`,
	RunE: runExtract,
}

func runExtract(cmd *cobra.Command, args []string) error {
	// Check if binary has bundled content
	isBundled, err := bundle.IsBundled()
	if err != nil {
		return fmt.Errorf("failed to check bundle status: %w", err)
	}

	if !isBundled {
		return fmt.Errorf("this binary does not have bundled content\nUse 'p2p-webapp bundle' to create a bundled binary")
	}

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

	// Extract bundled files
	fmt.Println("Extracting bundled site...")
	if err := bundle.ExtractBundle("."); err != nil {
		return fmt.Errorf("failed to extract bundle: %w", err)
	}

	fmt.Println("\nSite extracted successfully!")
	fmt.Println("\nTo run the site, use:")
	fmt.Println("  p2p-webapp serve --dir .")

	return nil
}
