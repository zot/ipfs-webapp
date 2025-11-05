package bundle

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// Magic marker to identify bundled binaries
	MagicMarker = "IPFSWAPP"
	// Footer size: 8 bytes magic + 8 bytes offset + 8 bytes size
	FooterSize = 24
)

// Footer contains metadata about the bundled ZIP
type Footer struct {
	Magic  [8]byte // "IPFSWAPP"
	Offset int64   // Offset to start of ZIP data
	Size   int64   // Size of ZIP data
}

// CreateBundle creates a new bundled binary
// sourceBinary: path to the p2p-webapp binary (can be bundled or unbundled)
// siteDir: directory containing html/, ipfs/, storage/
// outputPath: path for the bundled binary
func CreateBundle(sourceBinary, siteDir, outputPath string) error {
	// Get the size of the executable portion (excluding any existing bundle)
	binarySize, err := GetBinarySize(sourceBinary)
	if err != nil {
		return fmt.Errorf("failed to get binary size: %w", err)
	}

	// Open source binary
	srcFile, err := os.Open(sourceBinary)
	if err != nil {
		return fmt.Errorf("failed to open source binary: %w", err)
	}
	defer srcFile.Close()

	// Create output file
	outFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Copy only the executable portion (without any existing bundle)
	if _, err := io.CopyN(outFile, srcFile, binarySize); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Create ZIP in memory
	var zipBuf bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuf)

	// Add site files to ZIP
	if err := addDirToZip(zipWriter, siteDir, ""); err != nil {
		zipWriter.Close()
		return fmt.Errorf("failed to add files to ZIP: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close ZIP writer: %w", err)
	}

	zipData := zipBuf.Bytes()
	zipSize := int64(len(zipData))

	// Write ZIP data
	if _, err := outFile.Write(zipData); err != nil {
		return fmt.Errorf("failed to write ZIP data: %w", err)
	}

	// Write footer
	footer := Footer{
		Offset: binarySize,
		Size:   zipSize,
	}
	copy(footer.Magic[:], MagicMarker)

	if err := binary.Write(outFile, binary.LittleEndian, footer.Offset); err != nil {
		return fmt.Errorf("failed to write offset: %w", err)
	}
	if err := binary.Write(outFile, binary.LittleEndian, footer.Size); err != nil {
		return fmt.Errorf("failed to write size: %w", err)
	}
	if _, err := outFile.Write(footer.Magic[:]); err != nil {
		return fmt.Errorf("failed to write magic: %w", err)
	}

	return nil
}

// addDirToZip recursively adds directory contents to ZIP
func addDirToZip(zipWriter *zip.Writer, sourceDir, basePath string) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Create ZIP entry
		zipPath := filepath.Join(basePath, relPath)
		// Use forward slashes in ZIP paths
		zipPath = filepath.ToSlash(zipPath)

		writer, err := zipWriter.Create(zipPath)
		if err != nil {
			return err
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// GetBinarySize returns the size of the executable portion (excluding bundle)
// If the binary is bundled, returns the offset to the bundle
// If not bundled, returns the total file size
func GetBinarySize(binaryPath string) (int64, error) {
	file, err := os.Open(binaryPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open binary: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat binary: %w", err)
	}

	fileSize := info.Size()
	if fileSize < FooterSize {
		// Not bundled, return full size
		return fileSize, nil
	}

	// Read footer
	if _, err := file.Seek(fileSize-FooterSize, 0); err != nil {
		return 0, fmt.Errorf("failed to seek to footer: %w", err)
	}

	var footer Footer
	if err := binary.Read(file, binary.LittleEndian, &footer.Offset); err != nil {
		// Not bundled, return full size
		return fileSize, nil
	}
	if err := binary.Read(file, binary.LittleEndian, &footer.Size); err != nil {
		// Not bundled, return full size
		return fileSize, nil
	}
	if _, err := file.Read(footer.Magic[:]); err != nil {
		// Not bundled, return full size
		return fileSize, nil
	}

	// Check magic marker
	if bytes.Equal(footer.Magic[:], []byte(MagicMarker)) {
		// Bundled, return offset (size of executable without bundle)
		return footer.Offset, nil
	}

	// Not bundled, return full size
	return fileSize, nil
}

// IsBundled checks if the current binary has bundled content
func IsBundled() (bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, err
	}

	file, err := os.Open(exePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return false, err
	}

	fileSize := info.Size()
	if fileSize < FooterSize {
		return false, nil
	}

	// Read footer
	if _, err := file.Seek(fileSize-FooterSize, 0); err != nil {
		return false, err
	}

	var footer Footer
	if err := binary.Read(file, binary.LittleEndian, &footer.Offset); err != nil {
		return false, nil
	}
	if err := binary.Read(file, binary.LittleEndian, &footer.Size); err != nil {
		return false, nil
	}
	if _, err := file.Read(footer.Magic[:]); err != nil {
		return false, nil
	}

	// Check magic marker
	return bytes.Equal(footer.Magic[:], []byte(MagicMarker)), nil
}

// ExtractBundle extracts bundled content to a directory
func ExtractBundle(targetDir string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	file, err := os.Open(exePath)
	if err != nil {
		return fmt.Errorf("failed to open executable: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat executable: %w", err)
	}

	fileSize := info.Size()

	// Read footer
	if _, err := file.Seek(fileSize-FooterSize, 0); err != nil {
		return fmt.Errorf("failed to seek to footer: %w", err)
	}

	var footer Footer
	if err := binary.Read(file, binary.LittleEndian, &footer.Offset); err != nil {
		return fmt.Errorf("failed to read offset: %w", err)
	}
	if err := binary.Read(file, binary.LittleEndian, &footer.Size); err != nil {
		return fmt.Errorf("failed to read size: %w", err)
	}
	if _, err := file.Read(footer.Magic[:]); err != nil {
		return fmt.Errorf("failed to read magic: %w", err)
	}

	// Verify magic
	if !bytes.Equal(footer.Magic[:], []byte(MagicMarker)) {
		return fmt.Errorf("invalid magic marker")
	}

	// Read ZIP data
	if _, err := file.Seek(footer.Offset, 0); err != nil {
		return fmt.Errorf("failed to seek to ZIP data: %w", err)
	}

	zipData := make([]byte, footer.Size)
	if _, err := io.ReadFull(file, zipData); err != nil {
		return fmt.Errorf("failed to read ZIP data: %w", err)
	}

	// Open ZIP reader
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), footer.Size)
	if err != nil {
		return fmt.Errorf("failed to open ZIP reader: %w", err)
	}

	// Extract files
	for _, f := range zipReader.File {
		if err := extractZipFile(f, targetDir); err != nil {
			return fmt.Errorf("failed to extract %s: %w", f.Name, err)
		}
	}

	return nil
}

// extractZipFile extracts a single file from ZIP
func extractZipFile(f *zip.File, targetDir string) error {
	targetPath := filepath.Join(targetDir, f.Name)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	// Open ZIP file entry
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Create target file
	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Copy content
	_, err = io.Copy(outFile, rc)
	return err
}

// GetBundleReader returns a zip.Reader for the bundled content
// Returns nil if the binary is not bundled
func GetBundleReader() (*zip.Reader, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	file, err := os.Open(exePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open executable: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat executable: %w", err)
	}

	fileSize := info.Size()
	if fileSize < FooterSize {
		return nil, nil // Not bundled
	}

	// Read footer
	if _, err := file.Seek(fileSize-FooterSize, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to footer: %w", err)
	}

	var footer Footer
	if err := binary.Read(file, binary.LittleEndian, &footer.Offset); err != nil {
		return nil, nil // Not bundled
	}
	if err := binary.Read(file, binary.LittleEndian, &footer.Size); err != nil {
		return nil, nil // Not bundled
	}
	if _, err := file.Read(footer.Magic[:]); err != nil {
		return nil, nil // Not bundled
	}

	// Check magic marker
	if !bytes.Equal(footer.Magic[:], []byte(MagicMarker)) {
		return nil, nil // Not bundled
	}

	// Read ZIP data
	if _, err := file.Seek(footer.Offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to ZIP data: %w", err)
	}

	zipData := make([]byte, footer.Size)
	if _, err := io.ReadFull(file, zipData); err != nil {
		return nil, fmt.Errorf("failed to read ZIP data: %w", err)
	}

	// Open ZIP reader
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), footer.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to open ZIP reader: %w", err)
	}

	return zipReader, nil
}
