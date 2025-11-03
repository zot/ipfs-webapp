package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/zot/ipfs-webapp/internal/peer"
	"github.com/zot/ipfs-webapp/internal/protocol"
)

// Server manages the HTTP server and WebSocket connections
type Server struct {
	ctx         context.Context
	httpServer  *http.Server
	peerManager    *peer.Manager
	handler        *protocol.Handler
	port           int
	htmlDir        string
	connections    map[*WSConnection]bool
	peerConnection map[string]*WSConnection // Maps peerID to WSConnection
	mu             sync.RWMutex
}

// NewServer creates a new HTTP server
func NewServer(ctx context.Context, pm *peer.Manager, htmlDir string, port int) *Server {
	s := &Server{
		ctx:            ctx,
		peerManager:    pm,
		port:           port,
		htmlDir:        htmlDir,
		connections:    make(map[*WSConnection]bool),
		peerConnection: make(map[string]*WSConnection),
	}

	// Create protocol handler
	s.handler = protocol.NewHandler(pm)

	// Set peer manager callbacks to send messages to WebSocket clients
	pm.SetCallbacks(
		s.onPeerData,
		s.onTopicData,
		s.onTopicJoined,
		s.onTopicLeft,
	)

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Determine starting port
	startPort := s.port
	if startPort == 0 {
		startPort = 10000
	}

	// Create HTTP server mux
	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		s.handleWebSocket(w, r)
	})

	// Static file server with SPA routing fallback
	mux.Handle("/", s.spaHandler(http.Dir(s.htmlDir)))

	// Try to find an available port
	var listener net.Listener
	var err error
	maxAttempts := 100

	for attempt := 0; attempt < maxAttempts; attempt++ {
		port := startPort + attempt
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			s.port = port
			break
		}
	}

	if listener == nil {
		return fmt.Errorf("failed to find available port starting from %d", startPort)
	}

	s.httpServer = &http.Server{
		Handler: mux,
	}

	// Start server in goroutine with the listener
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	fmt.Printf("Server started on http://localhost:%d\n", s.port)
	return nil
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	// Close all WebSocket connections
	s.mu.Lock()
	for conn := range s.connections {
		conn.Close()
	}
	s.mu.Unlock()

	// Stop HTTP server
	if s.httpServer != nil {
		return s.httpServer.Shutdown(s.ctx)
	}
	return nil
}

// Port returns the port the server is listening on
func (s *Server) Port() int {
	return s.port
}

// RegisterPeer registers a peer with its WebSocket connection
func (s *Server) RegisterPeer(peerID string, conn *WSConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peerConnection[peerID] = conn
}

// UnregisterPeer removes a peer's connection mapping
func (s *Server) UnregisterPeer(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.peerConnection, peerID)
}

// OpenBrowser opens the default browser to the server URL
func (s *Server) OpenBrowser() error {
	url := fmt.Sprintf("http://localhost:%d", s.port)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

// spaHandler wraps http.FileServer to provide SPA routing fallback
// For SPA routes (no file extension, file doesn't exist), serve index.html
// while preserving the URL path for client-side routing
func (s *Server) spaHandler(fs http.FileSystem) http.Handler {
	fileServer := http.FileServer(fs)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Try to open the file
		f, err := fs.Open(path)
		if err == nil {
			// File exists, serve it normally
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// File doesn't exist - check if it looks like a SPA route
		ext := filepath.Ext(path)

		// If path has an extension, it's probably a real file request = real 404
		if ext != "" && ext != ".html" {
			http.NotFound(w, r)
			return
		}

		// No extension or .html extension - likely a SPA route
		// Serve index.html content directly (preserves URL for client-side routing)
		indexPath := "/index.html"
		indexFile, err := fs.Open(indexPath)
		if err != nil {
			// index.html doesn't exist, return 404
			http.NotFound(w, r)
			return
		}
		defer indexFile.Close()

		// Get file info for modification time
		indexInfo, err := indexFile.Stat()
		if err != nil {
			http.Error(w, "Failed to stat index.html", http.StatusInternalServerError)
			return
		}

		// Serve the content directly without changing the URL
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", indexInfo.ModTime(), indexFile.(io.ReadSeeker))
	})
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to upgrade connection: %v\n", err)
		return
	}

	wsConn := NewWSConnection(conn, s.handler, s.peerManager, s.peerManager, s)

	// Register connection
	s.mu.Lock()
	s.connections[wsConn] = true
	s.mu.Unlock()

	// Start connection
	wsConn.Start()

	// Cleanup on disconnect
	go func() {
		<-wsConn.closeCh
		s.mu.Lock()
		delete(s.connections, wsConn)
		s.mu.Unlock()
		fmt.Printf("WebSocket connection closed\n")
	}()

	fmt.Printf("New WebSocket connection established\n")
}

// Callback methods to send server messages to clients

func (s *Server) onPeerData(receiverPeerID, senderPeerID, protocol string, data any) {
	msg := s.handler.CreatePeerDataMessage(senderPeerID, protocol, data)

	// Send only to the connection that owns the receiving peer
	s.mu.RLock()
	conn, exists := s.peerConnection[receiverPeerID]
	s.mu.RUnlock()

	if exists {
		if err := conn.SendMessage(msg); err != nil {
			fmt.Printf("Failed to send peer message to peer %s: %v\n", receiverPeerID, err)
		}
	}
}

func (s *Server) onTopicData(receiverPeerID, topic, senderPeerID string, data any) {
	msg := s.handler.CreateTopicDataMessage(topic, senderPeerID, data)

	// Send only to the connection that owns the receiving peer
	s.mu.RLock()
	conn, exists := s.peerConnection[receiverPeerID]
	s.mu.RUnlock()

	if exists {
		if err := conn.SendMessage(msg); err != nil {
			fmt.Printf("Failed to send topic message to peer %s: %v\n", receiverPeerID, err)
		}
	}
}

func (s *Server) onTopicJoined(receiverPeerID, topic, joinedPeerID string) {
	msg := s.handler.CreateJoinedMessage(topic, joinedPeerID)

	// Send only to the connection that owns the receiving peer
	s.mu.RLock()
	conn, exists := s.peerConnection[receiverPeerID]
	s.mu.RUnlock()

	if exists {
		if err := conn.SendMessage(msg); err != nil {
			fmt.Printf("Failed to send joined message to peer %s: %v\n", receiverPeerID, err)
		}
	}
}

func (s *Server) onTopicLeft(receiverPeerID, topic, leftPeerID string) {
	msg := s.handler.CreateLeftMessage(topic, leftPeerID)

	// Send only to the connection that owns the receiving peer
	s.mu.RLock()
	conn, exists := s.peerConnection[receiverPeerID]
	s.mu.RUnlock()

	if exists {
		if err := conn.SendMessage(msg); err != nil {
			fmt.Printf("Failed to send left message to peer %s: %v\n", receiverPeerID, err)
		}
	}
}

func (s *Server) broadcastMessage(msg *protocol.Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for conn := range s.connections {
		if err := conn.SendMessage(msg); err != nil {
			fmt.Printf("Failed to send message to client: %v\n", err)
		}
	}
}
