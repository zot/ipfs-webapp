package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
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
	)

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// If port is 0, find random available port
	if s.port == 0 {
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
		s.port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		s.handleWebSocket(w, r)
	})

	// Static file server
	fs := http.FileServer(http.Dir(s.htmlDir))
	mux.Handle("/", fs)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

func (s *Server) broadcastMessage(msg *protocol.Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for conn := range s.connections {
		if err := conn.SendMessage(msg); err != nil {
			fmt.Printf("Failed to send message to client: %v\n", err)
		}
	}
}
