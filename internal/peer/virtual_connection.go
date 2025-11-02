package peer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// VirtualConnectionManager manages virtual connections and message queues per (peer, protocol) pair
type VirtualConnectionManager struct {
	ctx    context.Context
	peer   *Peer
	mu     sync.RWMutex
	queues map[string]*MessageQueue // key: "peerID:protocol"
}

// MessageQueue holds messages for a specific (peer, protocol) pair
type MessageQueue struct {
	peer             string
	protocol         string
	messages         []QueuedMessage
	stream           network.Stream
	retryCount       int
	unreachable      bool
	lastActivity     time.Time
	processing       bool
	mu               sync.Mutex
	manager          *VirtualConnectionManager
	streamReaderDone chan struct{}
}

// QueuedMessage represents a message in the queue
type QueuedMessage struct {
	id          string
	data        []byte
	attempts    int
	maxAttempts int
	timestamp   time.Time
}

// StreamMessage represents a message sent over the stream
type StreamMessage struct {
	Type string `json:"type"` // "data" or "ack"
	ID   string `json:"id"`
	Data []byte `json:"data,omitempty"`
}

// NewVirtualConnectionManager creates a new virtual connection manager for a peer
func NewVirtualConnectionManager(ctx context.Context, p *Peer) *VirtualConnectionManager {
	vcm := &VirtualConnectionManager{
		ctx:    ctx,
		peer:   p,
		queues: make(map[string]*MessageQueue),
	}

	// Start idle stream monitor
	go vcm.idleStreamMonitor()

	return vcm
}

// SendToQueue adds a message to the queue for (peer, protocol)
func (vcm *VirtualConnectionManager) SendToQueue(targetPeerID, protocolStr string, data any) error {
	// Encode data as JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	queueKey := fmt.Sprintf("%s:%s", targetPeerID, protocolStr)

	vcm.mu.Lock()
	queue, exists := vcm.queues[queueKey]
	if !exists {
		queue = &MessageQueue{
			peer:             targetPeerID,
			protocol:         protocolStr,
			messages:         make([]QueuedMessage, 0),
			lastActivity:     time.Now(),
			manager:          vcm,
			streamReaderDone: make(chan struct{}),
		}
		vcm.queues[queueKey] = queue
	}
	vcm.mu.Unlock()

	// Add message to queue
	queue.mu.Lock()
	msg := QueuedMessage{
		id:          fmt.Sprintf("%d-%d", time.Now().UnixNano(), len(queue.messages)),
		data:        jsonData,
		attempts:    0,
		maxAttempts: 3,
		timestamp:   time.Now(),
	}
	queue.messages = append(queue.messages, msg)
	shouldProcess := !queue.processing
	queue.mu.Unlock()

	// Trigger processing if not already processing
	if shouldProcess {
		go queue.processQueue()
	}

	return nil
}

// processQueue processes queued messages for this queue
func (q *MessageQueue) processQueue() {
	q.mu.Lock()
	if q.processing {
		q.mu.Unlock()
		return
	}
	q.processing = true
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		q.processing = false
		q.mu.Unlock()
	}()

	for {
		q.mu.Lock()
		if len(q.messages) == 0 {
			q.mu.Unlock()
			return
		}

		// Check if peer is unreachable
		if q.unreachable {
			q.mu.Unlock()
			return
		}

		// Get next message
		msg := q.messages[0]
		q.mu.Unlock()

		// Get or create stream
		if err := q.ensureStream(); err != nil {
			q.handleSendFailure(msg.id)
			continue
		}

		// Send message
		if err := q.sendMessage(msg); err != nil {
			q.handleSendFailure(msg.id)
			continue
		}

		// Wait for ACK with timeout
		acked := q.waitForAck(msg.id, 5*time.Second)
		if !acked {
			q.handleSendFailure(msg.id)
			continue
		}

		// Success - remove message from queue
		q.mu.Lock()
		if len(q.messages) > 0 && q.messages[0].id == msg.id {
			q.messages = q.messages[1:]
		}
		q.lastActivity = time.Now()
		q.retryCount = 0 // Reset retry count on success
		q.mu.Unlock()
	}
}

// ensureStream ensures a stream exists for this queue
func (q *MessageQueue) ensureStream() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.stream != nil {
		return nil
	}

	// Parse target peer ID
	targetPeerID, err := peer.Decode(q.peer)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	// Check if target peer exists locally
	q.manager.peer.manager.mu.RLock()
	targetPeer, localExists := q.manager.peer.manager.peers[q.peer]
	q.manager.peer.manager.mu.RUnlock()

	if localExists {
		// Connect to local peer
		addrs := targetPeer.host.Addrs()
		if len(addrs) > 0 {
			peerInfo := peer.AddrInfo{
				ID:    targetPeerID,
				Addrs: addrs,
			}
			if err := q.manager.peer.host.Connect(q.manager.ctx, peerInfo); err != nil {
				return fmt.Errorf("failed to connect to peer: %w", err)
			}
		}
	}

	// Open stream
	pid := protocol.ID(q.protocol)
	stream, err := q.manager.peer.host.NewStream(q.manager.ctx, targetPeerID, pid)
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}

	q.stream = stream
	q.lastActivity = time.Now()

	// Start reading ACKs and data from stream
	go q.readFromStream()

	// Log connection
	targetAlias := q.manager.peer.manager.getOrCreateAlias(q.peer)
	q.manager.peer.logVerbose(1, "Connected to %s on protocol %s", targetAlias, q.protocol)

	return nil
}

// sendMessage sends a message over the stream
func (q *MessageQueue) sendMessage(msg QueuedMessage) error {
	q.mu.Lock()
	stream := q.stream
	q.mu.Unlock()

	if stream == nil {
		return fmt.Errorf("no stream available")
	}

	streamMsg := StreamMessage{
		Type: "data",
		ID:   msg.id,
		Data: msg.data,
	}

	msgBytes, err := json.Marshal(streamMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal stream message: %w", err)
	}

	if err := writeMessage(stream, msgBytes); err != nil {
		// Stream failed, close it
		q.mu.Lock()
		q.closeStreamLocked()
		q.mu.Unlock()
		return fmt.Errorf("failed to write message: %w", err)
	}

	// Log sent message
	targetAlias := q.manager.peer.manager.getOrCreateAlias(q.peer)
	q.manager.peer.logVerbose(2, "Sent message to %s on protocol %s", targetAlias, q.protocol)

	q.mu.Lock()
	q.lastActivity = time.Now()
	q.mu.Unlock()

	return nil
}

// waitForAck waits for an ACK for a specific message ID
func (q *MessageQueue) waitForAck(msgID string, timeout time.Duration) bool {
	// For now, simulate immediate ACK since we don't have the full ACK infrastructure
	// In a full implementation, this would wait for an actual ACK message from the remote peer
	time.Sleep(10 * time.Millisecond)
	return true
}

// handleSendFailure handles a failed send attempt
func (q *MessageQueue) handleSendFailure(msgID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.messages) == 0 || q.messages[0].id != msgID {
		return
	}

	q.messages[0].attempts++
	q.retryCount++

	if q.messages[0].attempts >= q.messages[0].maxAttempts {
		// Mark peer unreachable after max attempts
		q.unreachable = true
		targetAlias := q.manager.peer.manager.getOrCreateAlias(q.peer)
		q.manager.peer.logVerbose(1, "Peer %s marked unreachable after %d attempts", targetAlias, q.messages[0].attempts)
		return
	}

	// Exponential backoff
	backoff := time.Duration(1<<uint(q.messages[0].attempts)) * time.Second
	time.Sleep(backoff)
}

// readFromStream reads messages from the stream
func (q *MessageQueue) readFromStream() {
	defer func() {
		close(q.streamReaderDone)
	}()

	q.mu.Lock()
	stream := q.stream
	q.mu.Unlock()

	if stream == nil {
		return
	}

	for {
		data, err := readMessage(stream)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading from stream: %v\n", err)
			}
			q.mu.Lock()
			q.closeStreamLocked()
			q.mu.Unlock()
			return
		}

		var streamMsg StreamMessage
		if err := json.Unmarshal(data, &streamMsg); err != nil {
			fmt.Printf("Error unmarshaling stream message: %v\n", err)
			continue
		}

		switch streamMsg.Type {
		case "ack":
			// ACK received - would be handled by waitForAck in full implementation
			q.mu.Lock()
			q.lastActivity = time.Now()
			q.mu.Unlock()

		case "data":
			// Data received from peer
			q.handleIncomingData(streamMsg.ID, streamMsg.Data)
		}
	}
}

// handleIncomingData handles data received from a peer
func (q *MessageQueue) handleIncomingData(msgID string, data []byte) {
	// Decode JSON
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		fmt.Printf("Error unmarshaling data: %v\n", err)
		return
	}

	// Clear unreachable flag - peer is reachable again
	q.mu.Lock()
	if q.unreachable {
		q.unreachable = false
		targetAlias := q.manager.peer.manager.getOrCreateAlias(q.peer)
		q.manager.peer.logVerbose(1, "Peer %s is reachable again", targetAlias)

		// Retry queued messages
		if !q.processing && len(q.messages) > 0 {
			go q.processQueue()
		}
	}
	q.lastActivity = time.Now()
	q.mu.Unlock()

	// Send ACK
	ackMsg := StreamMessage{
		Type: "ack",
		ID:   msgID,
	}
	ackBytes, err := json.Marshal(ackMsg)
	if err != nil {
		fmt.Printf("Error marshaling ACK: %v\n", err)
		return
	}

	q.mu.Lock()
	stream := q.stream
	q.mu.Unlock()

	if stream != nil {
		if err := writeMessage(stream, ackBytes); err != nil {
			fmt.Printf("Error sending ACK: %v\n", err)
		}
	}

	// Log received message
	remoteAlias := q.manager.peer.manager.getOrCreateAlias(q.peer)
	q.manager.peer.logVerbose(2, "Received message from %s on protocol %s", remoteAlias, q.protocol)

	// Deliver to application
	if q.manager.peer.manager.onPeerData != nil {
		q.manager.peer.manager.onPeerData(
			q.manager.peer.peerID.String(),
			q.peer,
			q.protocol,
			decoded,
		)
	}
}

// closeStreamLocked closes the stream (caller must hold mu)
func (q *MessageQueue) closeStreamLocked() {
	if q.stream != nil {
		q.stream.Close()
		q.stream = nil
	}
}

// idleStreamMonitor monitors streams for inactivity
func (vcm *VirtualConnectionManager) idleStreamMonitor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-vcm.ctx.Done():
			return
		case <-ticker.C:
			vcm.checkIdleStreams()
		}
	}
}

// checkIdleStreams closes streams that have been idle for too long
func (vcm *VirtualConnectionManager) checkIdleStreams() {
	now := time.Now()
	idleTimeout := 30 * time.Second

	vcm.mu.RLock()
	defer vcm.mu.RUnlock()

	for _, queue := range vcm.queues {
		queue.mu.Lock()
		if queue.stream != nil && now.Sub(queue.lastActivity) > idleTimeout {
			// Close idle stream
			targetAlias := vcm.peer.manager.getOrCreateAlias(queue.peer)
			vcm.peer.logVerbose(2, "Closing idle stream to %s on protocol %s", targetAlias, queue.protocol)
			queue.closeStreamLocked()
		}
		queue.mu.Unlock()
	}
}

// HandleIncomingStream handles a stream initiated by a remote peer
func (vcm *VirtualConnectionManager) HandleIncomingStream(stream network.Stream) {
	remotePeerID := stream.Conn().RemotePeer().String()
	protocolStr := string(stream.Protocol())
	queueKey := fmt.Sprintf("%s:%s", remotePeerID, protocolStr)

	vcm.mu.Lock()
	queue, exists := vcm.queues[queueKey]
	if !exists {
		queue = &MessageQueue{
			peer:             remotePeerID,
			protocol:         protocolStr,
			messages:         make([]QueuedMessage, 0),
			lastActivity:     time.Now(),
			manager:          vcm,
			streamReaderDone: make(chan struct{}),
		}
		vcm.queues[queueKey] = queue
	}
	vcm.mu.Unlock()

	// Set the stream on the queue
	queue.mu.Lock()
	if queue.stream != nil {
		// Close old stream
		queue.stream.Close()
	}
	queue.stream = stream
	queue.lastActivity = time.Now()
	queue.mu.Unlock()

	// Log incoming connection
	remoteAlias := vcm.peer.manager.getOrCreateAlias(remotePeerID)
	vcm.peer.logVerbose(1, "Accepted connection from %s on protocol %s", remoteAlias, protocolStr)

	// Start reading from stream
	go queue.readFromStream()
}

// Close cleans up all queues and streams
func (vcm *VirtualConnectionManager) Close() error {
	vcm.mu.Lock()
	defer vcm.mu.Unlock()

	for _, queue := range vcm.queues {
		queue.mu.Lock()
		queue.closeStreamLocked()
		queue.mu.Unlock()
	}

	return nil
}
