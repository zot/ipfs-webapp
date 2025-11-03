package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// CpCmd represents the cp command
var CpCmd = &cobra.Command{
	Use:   "cp SOURCE... DEST",
	Short: "Copy files from embedded demo directory",
	Long: `Copy files from the embedded demo directory to a target location.
Supports glob patterns for source selection (e.g., *.js, client.*).
Similar to UNIX cp command but operates on embedded demo files.

Examples:
  ipfs-webapp cp client.js my-project/          # copy single file
  ipfs-webapp cp client.* my-project/           # copy client.js and client.d.ts
  ipfs-webapp cp *.js *.html my-project/        # copy multiple patterns`,
	Args: cobra.MinimumNArgs(2),
	RunE: runCp,
}

func runCp(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("requires at least 2 arguments: SOURCE... DEST")
	}

	// Last argument is destination
	dest := args[len(args)-1]
	patterns := args[:len(args)-1]

	// Collect all matching files
	matchedFiles := make(map[string]bool)

	for _, pattern := range patterns {
		matches, err := findMatchingFiles(pattern)
		if err != nil {
			return err
		}
		for _, match := range matches {
			matchedFiles[match] = true
		}
	}

	if len(matchedFiles) == 0 {
		return fmt.Errorf("no files match the specified patterns: %v", patterns)
	}

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Verify destination is a directory
	destInfo, err := os.Stat(dest)
	if err != nil {
		return fmt.Errorf("failed to stat destination: %w", err)
	}
	if !destInfo.IsDir() {
		return fmt.Errorf("destination must be a directory when copying multiple files")
	}

	// Copy each matched file
	filesCopied := 0
	for file := range matchedFiles {
		if err := copyFile(file, dest); err != nil {
			return fmt.Errorf("failed to copy %s: %w", file, err)
		}
		filesCopied++
		fmt.Printf("Copied: %s\n", file)
	}

	fmt.Printf("\nSuccessfully copied %d file(s) to %s\n", filesCopied, dest)
	return nil
}

// findMatchingFiles returns all files in the embedded demo FS matching the pattern
func findMatchingFiles(pattern string) ([]string, error) {
	var matches []string

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
		fileName := filepath.Base(relPath)

		// Match against pattern
		matched, err := filepath.Match(pattern, fileName)
		if err != nil {
			return fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}

		if matched {
			matches = append(matches, relPath)
		}

		return nil
	})

	return matches, err
}

// copyFile copies a single file from the embedded FS to the destination directory
func copyFile(srcPath string, destDir string) error {
	// Read from embedded FS
	data, err := demoFS.ReadFile(filepath.Join("demo", srcPath))
	if err != nil {
		return fmt.Errorf("failed to read embedded file: %w", err)
	}

	// Write to destination
	destPath := filepath.Join(destDir, filepath.Base(srcPath))
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
