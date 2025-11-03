package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/zot/ipfs-webapp/internal/peer"
	"github.com/zot/ipfs-webapp/internal/protocol"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for localhost development
	},
}

// WSConnection represents a WebSocket connection
type WSConnection struct {
	conn          *websocket.Conn
	peerID        string
	peerCreated   bool
	handler       *protocol.Handler
	peerManager   protocol.PeerManager
	manager       *peer.Manager // For verbose logging
	server        *Server        // Reference to server for peer registration
	sendCh        chan *protocol.Message
	mu            sync.Mutex
	closed        bool
	closeCh       chan struct{}
}

// NewWSConnection creates a new WebSocket connection handler
func NewWSConnection(conn *websocket.Conn, handler *protocol.Handler, pm protocol.PeerManager, manager *peer.Manager, server *Server) *WSConnection {
	return &WSConnection{
		conn:        conn,
		handler:     handler,
		peerManager: pm,
		manager:     manager,
		server:      server,
		sendCh:      make(chan *protocol.Message, 100),
		closeCh:     make(chan struct{}),
	}
}

// Start begins processing the WebSocket connection
func (ws *WSConnection) Start() {
	go ws.readPump()
	go ws.writePump()
}

// SendMessage queues a message to be sent to the client
func (ws *WSConnection) SendMessage(msg *protocol.Message) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.closed {
		return fmt.Errorf("connection closed")
	}

	select {
	case ws.sendCh <- msg:
		return nil
	default:
		return fmt.Errorf("send buffer full")
	}
}

// Close closes the WebSocket connection
func (ws *WSConnection) Close() {
	ws.mu.Lock()
	if ws.closed {
		ws.mu.Unlock()
		return
	}
	ws.closed = true
	peerID := ws.peerID
	peerCreated := ws.peerCreated
	ws.mu.Unlock()

	// Clean up peer if it was created
	if peerCreated && peerID != "" && ws.peerManager != nil {
		// Unregister peer from server
		if ws.server != nil {
			ws.server.UnregisterPeer(peerID)
		}

		if err := ws.peerManager.RemovePeer(peerID); err != nil {
			fmt.Printf("Failed to remove peer %s: %v\n", peerID, err)
		}
	}

	close(ws.closeCh)
	ws.conn.Close()
}

// SetPeerID sets the peer ID for this connection
func (ws *WSConnection) SetPeerID(peerID string) {
	ws.peerID = peerID
}

// GetPeerID returns the peer ID for this connection
func (ws *WSConnection) GetPeerID() string {
	return ws.peerID
}

// readPump reads messages from the WebSocket
func (ws *WSConnection) readPump() {
	defer ws.Close()

	for {
		_, data, err := ws.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket read error: %v\n", err)
			}
			return
		}

		// Parse message
		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			fmt.Printf("Failed to unmarshal message: %v\n", err)
			continue
		}

		// Log received message
		if ws.manager != nil {
			peerID := ws.peerID
			if peerID == "" {
				peerID = "unknown"
			}
			ws.manager.LogVerbose(peerID, 2, "WS received: %s (req: %d)", msg.Method, msg.RequestID)
		}

		// Handle message
		response, err := ws.handler.HandleClientMessage(&msg, ws.peerID)
		if err != nil {
			fmt.Printf("Failed to handle message: %v\n", err)
			continue
		}

		// Special handling for "peer" command - set peer ID on first call
		if msg.Method == "peer" && response.Error == nil {
			var resp protocol.PeerResponse
			if err := json.Unmarshal(response.Result, &resp); err == nil {
				ws.mu.Lock()
				ws.peerID = resp.PeerID
				ws.peerCreated = true
				ws.mu.Unlock()

				// Register peer with server
				if ws.server != nil {
					ws.server.RegisterPeer(resp.PeerID, ws)
				}
			}
		}

		// Send response
		if err := ws.SendMessage(response); err != nil {
			fmt.Printf("Failed to send response: %v\n", err)
			return
		}
	}
}

// writePump writes messages to the WebSocket
func (ws *WSConnection) writePump() {
	defer ws.Close()

	for {
		select {
		case msg := <-ws.sendCh:
			data, err := json.Marshal(msg)
			if err != nil {
				fmt.Printf("Failed to marshal message: %v\n", err)
				continue
			}

			if err := ws.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				fmt.Printf("Failed to write message: %v\n", err)
				return
			}

			// Log sent message
			if ws.manager != nil {
				peerID := ws.peerID
				if peerID == "" {
					peerID = "unknown"
				}
				methodOrResponse := "response"
				if msg.Method != "" {
					methodOrResponse = msg.Method
				}
				ws.manager.LogVerbose(peerID, 2, "WS sent: %s (req: %d)", methodOrResponse, msg.RequestID)
			}

		case <-ws.closeCh:
			return
		}
	}
}

// HandleWebSocket handles WebSocket upgrade and connection
func HandleWebSocket(w http.ResponseWriter, r *http.Request, handler *protocol.Handler, pm protocol.PeerManager) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to upgrade connection: %v\n", err)
		return
	}

	wsConn := NewWSConnection(conn, handler, pm, nil, nil)
	wsConn.Start()

	// Store connection for later (would be managed by server in production)
	fmt.Printf("New WebSocket connection established\n")
}
