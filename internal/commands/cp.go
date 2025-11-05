package commands

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zot/p2p-webapp/internal/bundle"
)

// CpCmd represents the cp command
var CpCmd = &cobra.Command{
	Use:   "cp SOURCE... DEST",
	Short: "Copy files from bundled site",
	Long: `Copy files from the bundled site to a target location.
Supports glob patterns for source selection (e.g., *.js, client.*).
Similar to UNIX cp command but operates on bundled site files.

Examples:
  p2p-webapp cp client.js my-project/          # copy single file
  p2p-webapp cp client.* my-project/           # copy client.js and client.d.ts
  p2p-webapp cp *.js *.html my-project/        # copy multiple patterns`,
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

// findMatchingFiles returns all files in the bundled site matching the pattern
func findMatchingFiles(pattern string) ([]string, error) {
	// Get bundle reader
	zipReader, err := bundle.GetBundleReader()
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle: %w", err)
	}
	if zipReader == nil {
		return nil, fmt.Errorf("binary is not bundled")
	}

	var matches []string

	// List files from the html/ directory in the bundle
	for _, f := range zipReader.File {
		// Only include files from html/ directory
		if strings.HasPrefix(f.Name, "html/") && !f.FileInfo().IsDir() {
			// Remove html/ prefix
			relPath := strings.TrimPrefix(f.Name, "html/")
			fileName := filepath.Base(relPath)

			// Match against pattern
			matched, err := filepath.Match(pattern, fileName)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
			}

			if matched {
				matches = append(matches, relPath)
			}
		}
	}

	return matches, nil
}

// copyFile copies a single file from the bundle to the destination directory
func copyFile(srcPath string, destDir string) error {
	// Get bundle reader
	zipReader, err := bundle.GetBundleReader()
	if err != nil {
		return fmt.Errorf("failed to read bundle: %w", err)
	}
	if zipReader == nil {
		return fmt.Errorf("binary is not bundled")
	}

	// Find the file in the bundle
	targetPath := "html/" + srcPath
	var targetFile *zip.File
	for _, f := range zipReader.File {
		if f.Name == targetPath {
			targetFile = f
			break
		}
	}

	if targetFile == nil {
		return fmt.Errorf("file not found in bundle: %s", srcPath)
	}

	// Open file from bundle
	rc, err := targetFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open file from bundle: %w", err)
	}
	defer rc.Close()

	// Write to destination
	destPath := filepath.Join(destDir, filepath.Base(srcPath))
	outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}
