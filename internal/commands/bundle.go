package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zot/p2p-webapp/internal/bundle"
)

var (
	bundleOutput string
)

var BundleCmd = &cobra.Command{
	Use:   "bundle [site-directory]",
	Short: "Bundle a site into a standalone binary",
	Long: `Bundle a site into a standalone binary that can be distributed.

The site directory must contain:
  html/     - Website files (required)
  ipfs/     - IPFS content (optional)
  storage/  - Storage directory (optional, will be created if missing)

The output binary will automatically serve the bundled site when run.
No compilation tools needed - works out of the box!`,
	Args: cobra.ExactArgs(1),
	RunE: runBundle,
}

func init() {
	BundleCmd.Flags().StringVarP(&bundleOutput, "output", "o", "", "output binary path (required)")
	BundleCmd.MarkFlagRequired("output")
}

func runBundle(cmd *cobra.Command, args []string) error {
	siteDir := args[0]

	// Validate site directory
	if _, err := os.Stat(siteDir); os.IsNotExist(err) {
		return fmt.Errorf("site directory does not exist: %s", siteDir)
	}

	// Check for required html directory
	htmlDir := filepath.Join(siteDir, "html")
	if _, err := os.Stat(htmlDir); os.IsNotExist(err) {
		return fmt.Errorf("site directory must contain an 'html' subdirectory")
	}

	// Check for index.html
	indexHTML := filepath.Join(htmlDir, "index.html")
	if _, err := os.Stat(indexHTML); os.IsNotExist(err) {
		return fmt.Errorf("html directory must contain index.html")
	}

	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve to absolute path (in case it's a symlink)
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	fmt.Printf("Creating bundled binary...\n")
	fmt.Printf("  Source binary: %s\n", exePath)
	fmt.Printf("  Site directory: %s\n", siteDir)
	fmt.Printf("  Output: %s\n", bundleOutput)

	// Create bundle
	if err := bundle.CreateBundle(exePath, siteDir, bundleOutput); err != nil {
		return fmt.Errorf("failed to create bundle: %w", err)
	}

	// Get output file size
	info, err := os.Stat(bundleOutput)
	if err == nil {
		sizeMB := float64(info.Size()) / 1024 / 1024
		fmt.Printf("\nBundle created successfully!\n")
		fmt.Printf("  Size: %.2f MB\n", sizeMB)
		fmt.Printf("\nYou can now distribute '%s' as a standalone application.\n", bundleOutput)
		fmt.Printf("Run it with: %s\n", bundleOutput)
	}

	return nil
}
