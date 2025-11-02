package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/zot/ipfs-webapp/internal/ipfs"
	"github.com/zot/ipfs-webapp/internal/peer"
	"github.com/zot/ipfs-webapp/internal/server"
)

var (
	noOpen   bool
	verbose  int
)

// ServeCmd represents the serve command
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve a peer-to-peer application",
	Long: `Serve a peer-to-peer application from the current directory.
The current directory must contain:
  - html/: website to serve (must contain index.html)
  - ipfs/: content to make available in IPFS
  - storage/: server storage (peer keys, etc.)`,
	RunE: runServe,
}

func init() {
	ServeCmd.Flags().BoolVar(&noOpen, "noopen", false, "Do not open browser automatically")
	ServeCmd.Flags().CountVarP(&verbose, "verbose", "v", "Verbose output (can be specified multiple times: -v, -vv, -vvv)")
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Validate directory structure
	if err := validateDirectoryStructure(); err != nil {
		return err
	}

	// Create IPFS node
	storagePath := "storage"
	ipfsNode, err := ipfs.NewNode(ctx, storagePath, 0) // Random port
	if err != nil {
		return fmt.Errorf("failed to create IPFS node: %w", err)
	}
	defer ipfsNode.Close()

	fmt.Printf("Peer ID: %s\n", ipfsNode.PeerID())

	// Create peer manager
	peerManager, err := peer.NewManager(ctx, ipfsNode.Host(), verbose)
	if err != nil {
		return fmt.Errorf("failed to create peer manager: %w", err)
	}

	// Create HTTP server
	htmlDir := "html"
	srv := server.NewServer(ctx, peerManager, htmlDir, 0) // Random port

	// Start server
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	defer srv.Stop()

	// Open browser unless --noopen flag is set
	if !noOpen {
		if err := srv.OpenBrowser(); err != nil {
			fmt.Printf("Failed to open browser: %v\n", err)
			fmt.Printf("Please open http://localhost:%d manually\n", srv.Port())
		}
	} else {
		fmt.Printf("Server running at http://localhost:%d\n", srv.Port())
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	return nil
}

func validateDirectoryStructure() error {
	// Check html directory
	htmlDir := "html"
	if _, err := os.Stat(htmlDir); os.IsNotExist(err) {
		return fmt.Errorf("html directory not found")
	}

	// Check for index.html
	indexPath := filepath.Join(htmlDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("html/index.html not found")
	}

	// Create ipfs directory if it doesn't exist
	ipfsDir := "ipfs"
	if _, err := os.Stat(ipfsDir); os.IsNotExist(err) {
		if err := os.Mkdir(ipfsDir, 0755); err != nil {
			return fmt.Errorf("failed to create ipfs directory: %w", err)
		}
	}

	// Create storage directory if it doesn't exist
	storageDir := "storage"
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		if err := os.Mkdir(storageDir, 0755); err != nil {
			return fmt.Errorf("failed to create storage directory: %w", err)
		}
	}

	return nil
}
