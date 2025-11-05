package peer

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

// Manager manages multiple peers
type Manager struct {
	ctx            context.Context
	mu             sync.RWMutex
	peers        map[string]*Peer
	onPeerData   func(receiverPeerID, senderPeerID, protocol string, data any)
	onTopicData  func(receiverPeerID, topic, senderPeerID string, data any)
	onPeerChange func(receiverPeerID, topic, changedPeerID string, joined bool)
	peerAliases  map[string]string // peerID -> alias
	aliasCounter   int
	verbosity      int
}

// Peer represents a single libp2p peer with its own host and state
type Peer struct {
	ctx              context.Context
	host             host.Host
	pubsub           *pubsub.PubSub
	peerID           peer.ID
	alias            string
	mu               sync.RWMutex
	connections      map[string]*Connection // key: "peerID:protocol" (legacy, kept for compatibility)
	protocols        map[protocol.ID]*ProtocolHandler
	topics           map[string]*TopicHandler
	monitoredTopics  map[string]*TopicMonitor // topics being monitored for join/leave events
	manager          *Manager
	vcm              *VirtualConnectionManager // Virtual connection manager for reliability
}

// TopicMonitor tracks peers in a topic and monitors join/leave events
type TopicMonitor struct {
	Topic       string
	ctx         context.Context
	cancel      context.CancelFunc
	knownPeers  map[string]bool // track which peers we've seen
}

// Connection represents an active peer-to-peer stream
type Connection struct {
	Stream   network.Stream
	PeerID   peer.ID
	Protocol protocol.ID
	mu       sync.Mutex
}

// ProtocolHandler handles incoming streams for a protocol
type ProtocolHandler struct {
	Protocol protocol.ID
}

// TopicHandler handles pub/sub topic subscriptions
type TopicHandler struct {
	Topic        string
	PubsubTopic  *pubsub.Topic
	Subscription *pubsub.Subscription
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewManager creates a new peer manager
func NewManager(ctx context.Context, bootstrapHost host.Host, verbosity int) (*Manager, error) {
	return &Manager{
		ctx:         ctx,
		peers:       make(map[string]*Peer),
		peerAliases: make(map[string]string),
		verbosity:   verbosity,
	}, nil
}

// SetCallbacks sets the callback functions for events
func (m *Manager) SetCallbacks(
	onPeerData func(receiverPeerID, senderPeerID, protocol string, data any),
	onTopicData func(receiverPeerID, topic, senderPeerID string, data any),
	onPeerChange func(receiverPeerID, topic, changedPeerID string, joined bool),
) {
	m.onPeerData = onPeerData
	m.onTopicData = onTopicData
	m.onPeerChange = onPeerChange
}

// LogVerbose logs a message if the level is within the verbosity threshold
func (m *Manager) LogVerbose(peerID string, level int, format string, args ...any) {
	if level > m.verbosity {
		return
	}
	alias := m.getOrCreateAlias(peerID)
	message := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s\n", alias, message)
}

// getOrCreateAliasLocked returns the alias for a peer, creating one if it doesn't exist
// Caller must hold m.mu lock
func (m *Manager) getOrCreateAliasLocked(peerID string) string {
	if alias, exists := m.peerAliases[peerID]; exists {
		return alias
	}

	// Generate new alias (peer-a, peer-b, etc.)
	letter := rune('a' + m.aliasCounter)
	alias := fmt.Sprintf("peer-%c", letter)
	m.peerAliases[peerID] = alias
	m.aliasCounter++

	return alias
}

// getOrCreateAlias returns the alias for a peer, creating one if it doesn't exist
// This version acquires the lock
func (m *Manager) getOrCreateAlias(peerID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getOrCreateAliasLocked(peerID)
}

// allowPrivateGater is a ConnectionGater that allows all connections,
// including those on private/local addresses
type allowPrivateGater struct{}

// Ensure allowPrivateGater implements connmgr.ConnectionGater
var _ connmgr.ConnectionGater = (*allowPrivateGater)(nil)

func (g *allowPrivateGater) InterceptPeerDial(p peer.ID) (allow bool) {
	return true
}

func (g *allowPrivateGater) InterceptAddrDial(p peer.ID, m multiaddr.Multiaddr) (allow bool) {
	return true
}

func (g *allowPrivateGater) InterceptAccept(n network.ConnMultiaddrs) (allow bool) {
	return true
}

func (g *allowPrivateGater) InterceptSecured(dir network.Direction, p peer.ID, n network.ConnMultiaddrs) (allow bool) {
	return true
}

func (g *allowPrivateGater) InterceptUpgraded(c network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

// prepareCreatePeer generates/parses the private key and checks for duplicates
// Must be called with m.mu locked
func (m *Manager) prepareCreatePeer(requestedPeerKey string) (priv crypto.PrivKey, err error) {
	// Generate or parse peer identity
	if requestedPeerKey != "" {
		// Decode and unmarshal the private key
		keyBytes, err := crypto.ConfigDecodeKey(requestedPeerKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode peer key: %w", err)
		}
		priv, err = crypto.UnmarshalPrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal peer key: %w", err)
		}
	} else {
		// Generate new identity
		priv, _, err = crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate key pair: %w", err)
		}
	}

	// Derive peer ID to check for duplicates
	pid, err := peer.IDFromPublicKey(priv.GetPublic())
	if err != nil {
		return nil, fmt.Errorf("failed to derive peer ID: %w", err)
	}

	// Check if peer ID is already registered (duplicate)
	if _, exists := m.peers[pid.String()]; exists {
		return nil, fmt.Errorf("peer ID already in use (possible duplicate browser tab)")
	}

	return priv, nil
}

// CreatePeer creates a new peer with its own libp2p host
func (m *Manager) CreatePeer(requestedPeerKey string) (peerID string, peerKey string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Prepare and validate peer creation (checks for duplicates)
	priv, err := m.prepareCreatePeer(requestedPeerKey)
	if err != nil {
		return "", "", err
	}

	// Marshal and encode the private key for return
	keyBytes, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal private key: %w", err)
	}
	encodedKey := crypto.ConfigEncodeKey(keyBytes)

	// Create libp2p host
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"), // Random port
		libp2p.ConnectionGater(&allowPrivateGater{}),   // Allow private/local addresses
		libp2p.DisableRelay(),                           // Simplify for local dev
		libp2p.NATPortMap(),                             // Allow NAT traversal
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to create host: %w", err)
	}

	// Create pubsub
	ps, err := pubsub.NewGossipSub(m.ctx, h)
	if err != nil {
		h.Close()
		return "", "", fmt.Errorf("failed to create pubsub: %w", err)
	}

	// Create peer
	p := &Peer{
		ctx:             m.ctx,
		host:            h,
		pubsub:          ps,
		peerID:          h.ID(),
		connections:     make(map[string]*Connection),
		protocols:       make(map[protocol.ID]*ProtocolHandler),
		topics:          make(map[string]*TopicHandler),
		monitoredTopics: make(map[string]*TopicMonitor),
		manager:         m,
	}

	// Initialize virtual connection manager
	p.vcm = NewVirtualConnectionManager(m.ctx, p)

	// Connect to other peers in the same manager for local pubsub
	for _, otherPeer := range m.peers {
		// Try to connect peers to each other
		addrs := otherPeer.host.Addrs()
		if len(addrs) > 0 {
			peerInfo := peer.AddrInfo{
				ID:    otherPeer.peerID,
				Addrs: addrs,
			}
			// Best effort connection, ignore errors
			_ = h.Connect(m.ctx, peerInfo)
		}
	}

	// Set alias for the peer (we already hold the lock)
	p.alias = m.getOrCreateAliasLocked(p.peerID.String())

	m.peers[p.peerID.String()] = p

	// Log peer creation (we already hold the lock, so log directly)
	if m.verbosity >= 1 {
		fmt.Printf("[%s] Created peer\n", p.alias)
	}

	return p.peerID.String(), encodedKey, nil
}

// getPeer retrieves a peer by ID
func (m *Manager) getPeer(peerID string) (*Peer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.peers[peerID]
	if !exists {
		return nil, fmt.Errorf("peer not found: %s", peerID)
	}
	return p, nil
}

// Start starts a protocol handler for a peer
func (m *Manager) Start(peerID, protocolStr string) error {
	p, err := m.getPeer(peerID)
	if err != nil {
		return err
	}
	return p.Start(protocolStr)
}

// Stop removes a protocol handler for a peer
func (m *Manager) Stop(peerID, protocolStr string) error {
	p, err := m.getPeer(peerID)
	if err != nil {
		return err
	}
	return p.Stop(protocolStr)
}

// Send sends data to a peer on a protocol
func (m *Manager) Send(peerID, targetPeerID, protocolStr string, data any) error {
	p, err := m.getPeer(peerID)
	if err != nil {
		return err
	}
	return p.SendToPeer(targetPeerID, protocolStr, data)
}

// Subscribe subscribes a peer to a pub/sub topic
func (m *Manager) Subscribe(peerID, topic string) error {
	p, err := m.getPeer(peerID)
	if err != nil {
		return err
	}
	return p.Subscribe(topic)
}

// Publish publishes data to a topic from a peer
func (m *Manager) Publish(peerID, topic string, data any) error {
	p, err := m.getPeer(peerID)
	if err != nil {
		return err
	}
	return p.Publish(topic, data)
}

// Unsubscribe unsubscribes a peer from a topic
func (m *Manager) Unsubscribe(peerID, topic string) error {
	p, err := m.getPeer(peerID)
	if err != nil {
		return err
	}
	return p.Unsubscribe(topic)
}

// ListPeers returns the list of peers subscribed to a topic
func (m *Manager) ListPeers(peerID, topic string) ([]string, error) {
	p, err := m.getPeer(peerID)
	if err != nil {
		return nil, err
	}
	return p.ListPeers(topic)
}

// Monitor starts monitoring a topic for peer join/leave events
func (m *Manager) Monitor(peerID, topic string) error {
	p, err := m.getPeer(peerID)
	if err != nil {
		return err
	}
	return p.Monitor(topic)
}

// StopMonitor stops monitoring a topic for peer join/leave events
func (m *Manager) StopMonitor(peerID, topic string) error {
	p, err := m.getPeer(peerID)
	if err != nil {
		return err
	}
	return p.StopMonitor(topic)
}

// RemovePeer removes a peer and cleans up its resources
func (m *Manager) RemovePeer(peerID string) error {
	m.mu.Lock()
	p, exists := m.peers[peerID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("peer not found: %s", peerID)
	}
	delete(m.peers, peerID)
	m.mu.Unlock()

	// Clean up peer resources
	return p.Close()
}

// Peer methods

// logVerbose logs a message from this peer if the level is within the verbosity threshold
func (p *Peer) logVerbose(level int, format string, args ...any) {
	p.manager.LogVerbose(p.peerID.String(), level, format, args...)
}

func (p *Peer) Start(protocolStr string) error {
	pid := protocol.ID(protocolStr)

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.protocols[pid]; exists {
		return fmt.Errorf("already started protocol: %s", protocolStr)
	}

	handler := &ProtocolHandler{
		Protocol: pid,
	}
	p.protocols[pid] = handler

	// Set stream handler
	p.host.SetStreamHandler(pid, func(s network.Stream) {
		p.handleIncomingStream(s)
	})

	return nil
}

func (p *Peer) Stop(protocolStr string) error {
	pid := protocol.ID(protocolStr)

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.protocols[pid]; !exists {
		return fmt.Errorf("protocol not started: %s", protocolStr)
	}

	delete(p.protocols, pid)
	p.host.RemoveStreamHandler(pid)

	return nil
}

// SendToPeer sends data to a peer on a protocol using the virtual connection manager
func (p *Peer) SendToPeer(targetPeerIDStr, protocolStr string, data any) error {
	// Use virtual connection manager for reliable delivery with queuing, retry, and ACK
	return p.vcm.SendToQueue(targetPeerIDStr, protocolStr, data)
}

func (p *Peer) Subscribe(topic string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If already subscribed, return success (idempotent)
	if _, exists := p.topics[topic]; exists {
		return nil
	}

	// Join topic
	t, err := p.pubsub.Join(topic)
	if err != nil {
		return fmt.Errorf("failed to join topic: %w", err)
	}

	// Subscribe
	sub, err := t.Subscribe()
	if err != nil {
		t.Close()
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	// Create handler
	ctx, cancel := context.WithCancel(p.ctx)
	handler := &TopicHandler{
		Topic:        topic,
		PubsubTopic:  t,
		Subscription: sub,
		ctx:          ctx,
		cancel:       cancel,
	}
	p.topics[topic] = handler

	// Start reading messages
	go p.readFromTopic(handler)

	return nil
}

func (p *Peer) Publish(topic string, data any) error {
	p.mu.RLock()
	handler, exists := p.topics[topic]
	p.mu.RUnlock()

	var t *pubsub.Topic
	var err error

	if exists {
		// Use existing topic
		t = handler.PubsubTopic
	} else {
		// Join new topic
		t, err = p.pubsub.Join(topic)
		if err != nil {
			return fmt.Errorf("failed to join topic: %w", err)
		}
		defer t.Close()
	}

	// Encode data
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Publish
	if err := t.Publish(p.ctx, jsonData); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}

func (p *Peer) Unsubscribe(topic string) error {
	p.mu.Lock()
	handler, exists := p.topics[topic]
	if !exists {
		p.mu.Unlock()
		// If not subscribed, return success (idempotent)
		return nil
	}
	delete(p.topics, topic)
	p.mu.Unlock()

	handler.cancel()
	handler.Subscription.Cancel()
	handler.PubsubTopic.Close()

	return nil
}

func (p *Peer) ListPeers(topic string) ([]string, error) {
	p.mu.RLock()
	monitor, isMonitored := p.monitoredTopics[topic]
	p.mu.RUnlock()

	// If monitoring this topic, return the monitored peer list
	if isMonitored {
		peerStrs := make([]string, 0, len(monitor.knownPeers))
		for peerID := range monitor.knownPeers {
			peerStrs = append(peerStrs, peerID)
		}
		return peerStrs, nil
	}

	// Otherwise, query gossipsub directly
	peers := p.pubsub.ListPeers(topic)

	// Convert peer.ID slice to string slice
	peerStrs := make([]string, len(peers))
	for i, pid := range peers {
		peerStrs[i] = pid.String()
	}

	return peerStrs, nil
}

func (p *Peer) Monitor(topic string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If already monitoring, return success (idempotent)
	if _, exists := p.monitoredTopics[topic]; exists {
		return nil
	}

	// Create monitor
	ctx, cancel := context.WithCancel(p.ctx)
	monitor := &TopicMonitor{
		Topic:      topic,
		ctx:        ctx,
		cancel:     cancel,
		knownPeers: make(map[string]bool),
	}
	p.monitoredTopics[topic] = monitor

	// Start monitoring
	go p.monitorTopic(monitor)

	return nil
}

func (p *Peer) StopMonitor(topic string) error {
	p.mu.Lock()
	monitor, exists := p.monitoredTopics[topic]
	if !exists {
		p.mu.Unlock()
		// If not monitoring, return success (idempotent)
		return nil
	}
	delete(p.monitoredTopics, topic)
	p.mu.Unlock()

	monitor.cancel()

	return nil
}

func (p *Peer) Close() error {
	// Close virtual connection manager
	if p.vcm != nil {
		p.vcm.Close()
	}

	// Close all topics
	p.mu.Lock()
	for _, handler := range p.topics {
		handler.cancel()
		handler.Subscription.Cancel()
		handler.PubsubTopic.Close()
	}
	p.topics = make(map[string]*TopicHandler)

	// Close all monitors
	for _, monitor := range p.monitoredTopics {
		monitor.cancel()
	}
	p.monitoredTopics = make(map[string]*TopicMonitor)

	// Close all connections (legacy)
	for _, conn := range p.connections {
		conn.Stream.Close()
	}
	p.connections = make(map[string]*Connection)
	p.mu.Unlock()

	// Close host
	return p.host.Close()
}

// Internal methods

func (p *Peer) handleIncomingStream(s network.Stream) {
	// Route incoming stream to virtual connection manager for reliable handling
	p.vcm.HandleIncomingStream(s)
}

func (p *Peer) readFromStream(conn *Connection) {
	remotePeerID := conn.PeerID.String()
	protocolStr := string(conn.Protocol)
	connKey := fmt.Sprintf("%s:%s", remotePeerID, protocolStr)

	defer func() {
		p.mu.Lock()
		delete(p.connections, connKey)
		p.mu.Unlock()

		conn.Stream.Close()
	}()

	for {
		data, err := readMessage(conn.Stream)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading from stream: %v\n", err)
			}
			return
		}

		// Decode JSON
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			fmt.Printf("Error unmarshaling data: %v\n", err)
			continue
		}

		// Log received message
		remoteAlias := p.manager.getOrCreateAlias(remotePeerID)
		p.logVerbose(2, "Received message from %s on protocol %s", remoteAlias, protocolStr)

		if p.manager.onPeerData != nil {
			p.manager.onPeerData(p.peerID.String(), remotePeerID, protocolStr, decoded)
		}
	}
}

func (p *Peer) readFromTopic(handler *TopicHandler) {
	for {
		msg, err := handler.Subscription.Next(handler.ctx)
		if err != nil {
			if err != context.Canceled {
				fmt.Printf("Error reading from topic: %v\n", err)
			}
			return
		}

		// Decode JSON
		var decoded any
		if err := json.Unmarshal(msg.Data, &decoded); err != nil {
			fmt.Printf("Error unmarshaling topic data: %v\n", err)
			continue
		}

		if p.manager.onTopicData != nil {
			p.manager.onTopicData(p.peerID.String(), handler.Topic, msg.GetFrom().String(), decoded)
		}
	}
}

func (p *Peer) monitorTopic(monitor *TopicMonitor) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-monitor.ctx.Done():
			return
		case <-ticker.C:
			// Get current peers in topic
			currentPeers := p.pubsub.ListPeers(monitor.Topic)

			// Build current peer set
			currentPeerSet := make(map[string]bool)
			for _, pid := range currentPeers {
				peerIDStr := pid.String()
				currentPeerSet[peerIDStr] = true

				// Check if this is a new peer (joined)
				if !monitor.knownPeers[peerIDStr] {
					monitor.knownPeers[peerIDStr] = true
					if p.manager.onPeerChange != nil {
						p.manager.onPeerChange(p.peerID.String(), monitor.Topic, peerIDStr, true)
					}
				}
			}

			// Check for peers that left
			for peerIDStr := range monitor.knownPeers {
				if !currentPeerSet[peerIDStr] {
					delete(monitor.knownPeers, peerIDStr)
					if p.manager.onPeerChange != nil {
						p.manager.onPeerChange(p.peerID.String(), monitor.Topic, peerIDStr, false)
					}
				}
			}
		}
	}
}

// Message framing helpers

func writeMessage(w io.Writer, data []byte) error {
	// Write length as 4-byte big-endian
	length := uint32(len(data))
	lengthBytes := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if _, err := w.Write(lengthBytes); err != nil {
		return err
	}

	_, err := w.Write(data)
	return err
}

func readMessage(r io.Reader) ([]byte, error) {
	// Read length
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(r, lengthBytes); err != nil {
		return nil, err
	}

	length := uint32(lengthBytes[0])<<24 |
		uint32(lengthBytes[1])<<16 |
		uint32(lengthBytes[2])<<8 |
		uint32(lengthBytes[3])

	// Read message
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	return data, nil
}

// Bootstrap connects to a bootstrap peer (helper method)
func (m *Manager) Bootstrap(peerID, bootstrapAddr string) error {
	p, err := m.getPeer(peerID)
	if err != nil {
		return err
	}

	addr, err := multiaddr.NewMultiaddr(bootstrapAddr)
	if err != nil {
		return err
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return err
	}

	return p.host.Connect(p.ctx, *peerInfo)
}
