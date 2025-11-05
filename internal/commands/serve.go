package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/zot/p2p-webapp/internal/bundle"
	"github.com/zot/p2p-webapp/internal/ipfs"
	"github.com/zot/p2p-webapp/internal/peer"
	"github.com/zot/p2p-webapp/internal/server"
)

var (
	noOpen   bool
	verbose  int
	port     int
	dir      string
)

// ServeCmd represents the serve command
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve a peer-to-peer application",
	Long: `Serve a peer-to-peer application.

Default mode (no --dir flag):
  Serves directly from the bundled site without extracting to filesystem.

With --dir flag:
  Serves from the specified directory which must contain:
  - html/: website to serve (must contain index.html)
  - ipfs/: content to make available in IPFS (optional)
  - storage/: server storage (peer keys, etc.)`,
	RunE: runServe,
}

func init() {
	ServeCmd.Flags().BoolVar(&noOpen, "noopen", false, "Do not open browser automatically")
	ServeCmd.Flags().CountVarP(&verbose, "verbose", "v", "Verbose output (can be specified multiple times: -v, -vv, -vvv)")
	ServeCmd.Flags().IntVarP(&port, "port", "p", 0, "Port to listen on (default: auto-select starting from 10000)")
	ServeCmd.Flags().StringVar(&dir, "dir", "", "Directory to serve from (if not specified, serves from bundled site)")
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var srv *server.Server
	var storagePath string

	if dir != "" {
		// Directory mode: serve from filesystem
		if err := validateDirectoryStructure(dir); err != nil {
			return err
		}

		storagePath = filepath.Join(dir, "storage")

		// Create IPFS node
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

		// Create HTTP server from directory
		htmlDir := filepath.Join(dir, "html")
		srv = server.NewServerFromDir(ctx, peerManager, htmlDir, port)
	} else {
		// Bundle mode: serve from bundled site
		bundleReader, err := bundle.GetBundleReader()
		if err != nil {
			return fmt.Errorf("failed to read bundle: %w", err)
		}
		if bundleReader == nil {
			return fmt.Errorf("binary is not bundled\nUse --dir to serve from a directory, or use a bundled binary")
		}

		// Create temporary storage directory in current directory
		storagePath = ".p2p-webapp-storage"
		if err := os.MkdirAll(storagePath, 0755); err != nil {
			return fmt.Errorf("failed to create storage directory: %w", err)
		}

		// Create IPFS node
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

		// Create HTTP server from bundle
		srv = server.NewServerFromBundle(ctx, peerManager, bundleReader, port)
	}

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

func validateDirectoryStructure(baseDir string) error {
	// Check html directory
	htmlDir := filepath.Join(baseDir, "html")
	if _, err := os.Stat(htmlDir); os.IsNotExist(err) {
		return fmt.Errorf("html directory not found in %s", baseDir)
	}

	// Check for index.html
	indexPath := filepath.Join(htmlDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("html/index.html not found in %s", baseDir)
	}

	// Create ipfs directory if it doesn't exist
	ipfsDir := filepath.Join(baseDir, "ipfs")
	if _, err := os.Stat(ipfsDir); os.IsNotExist(err) {
		if err := os.Mkdir(ipfsDir, 0755); err != nil {
			return fmt.Errorf("failed to create ipfs directory: %w", err)
		}
	}

	// Create storage directory if it doesn't exist
	storageDir := filepath.Join(baseDir, "storage")
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		if err := os.Mkdir(storageDir, 0755); err != nil {
			return fmt.Errorf("failed to create storage directory: %w", err)
		}
	}

	return nil
}
