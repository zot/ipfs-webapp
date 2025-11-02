package peer

import (
	"context"
	"testing"
	"time"
)

// TestVirtualConnectionManager_QueueCreation tests that queues are created correctly
func TestVirtualConnectionManager_QueueCreation(t *testing.T) {
	ctx := context.Background()

	// Create a mock peer (minimal setup)
	manager := &Manager{
		ctx:         ctx,
		peers:       make(map[string]*Peer),
		peerAliases: make(map[string]string),
		verbosity:   0,
	}

	peer := &Peer{
		ctx:     ctx,
		manager: manager,
	}

	vcm := NewVirtualConnectionManager(ctx, peer)

	// Test that VCM is initialized
	if vcm == nil {
		t.Fatal("VirtualConnectionManager should not be nil")
	}

	if vcm.queues == nil {
		t.Fatal("queues map should be initialized")
	}

	// Initially should have no queues
	vcm.mu.RLock()
	queueCount := len(vcm.queues)
	vcm.mu.RUnlock()

	if queueCount != 0 {
		t.Errorf("Expected 0 queues initially, got %d", queueCount)
	}
}

// TestVirtualConnectionManager_SendToQueue tests basic queue sending
func TestVirtualConnectionManager_SendToQueue(t *testing.T) {
	ctx := context.Background()

	manager := &Manager{
		ctx:         ctx,
		peers:       make(map[string]*Peer),
		peerAliases: make(map[string]string),
		verbosity:   0,
	}

	peer := &Peer{
		ctx:     ctx,
		manager: manager,
	}

	vcm := NewVirtualConnectionManager(ctx, peer)

	// Send a message (will fail to actually send since we have no real peer, but should create queue)
	targetPeerID := "test-peer-123"
	protocol := "/test/1.0.0"
	data := map[string]string{"message": "hello"}

	err := vcm.SendToQueue(targetPeerID, protocol, data)
	if err != nil {
		t.Fatalf("SendToQueue should not return error for valid data: %v", err)
	}

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)

	// Check that queue was created
	vcm.mu.RLock()
	queueKey := "test-peer-123:/test/1.0.0"
	queue, exists := vcm.queues[queueKey]
	vcm.mu.RUnlock()

	if !exists {
		t.Fatal("Queue should have been created")
	}

	if queue.peer != targetPeerID {
		t.Errorf("Expected peer ID %s, got %s", targetPeerID, queue.peer)
	}

	if queue.protocol != protocol {
		t.Errorf("Expected protocol %s, got %s", protocol, queue.protocol)
	}
}

// TestMessageQueue_BasicProperties tests MessageQueue properties
func TestMessageQueue_BasicProperties(t *testing.T) {
	ctx := context.Background()

	manager := &Manager{
		ctx:         ctx,
		peers:       make(map[string]*Peer),
		peerAliases: make(map[string]string),
		verbosity:   0,
	}

	peer := &Peer{
		ctx:     ctx,
		manager: manager,
	}

	vcm := NewVirtualConnectionManager(ctx, peer)

	queue := &MessageQueue{
		peer:             "test-peer",
		protocol:         "/test/1.0.0",
		messages:         make([]QueuedMessage, 0),
		lastActivity:     time.Now(),
		manager:          vcm,
		streamReaderDone: make(chan struct{}),
	}

	// Test initial state
	if queue.unreachable {
		t.Error("Queue should not be marked unreachable initially")
	}

	if queue.processing {
		t.Error("Queue should not be processing initially")
	}

	if queue.retryCount != 0 {
		t.Error("Retry count should be 0 initially")
	}

	if len(queue.messages) != 0 {
		t.Error("Messages slice should be empty initially")
	}
}

// TestVirtualConnectionManager_MultipleQueues tests that multiple queues can coexist
func TestVirtualConnectionManager_MultipleQueues(t *testing.T) {
	ctx := context.Background()

	manager := &Manager{
		ctx:         ctx,
		peers:       make(map[string]*Peer),
		peerAliases: make(map[string]string),
		verbosity:   0,
	}

	peer := &Peer{
		ctx:     ctx,
		manager: manager,
	}

	vcm := NewVirtualConnectionManager(ctx, peer)

	// Send messages to different peer/protocol combinations
	combinations := []struct {
		peerID   string
		protocol string
	}{
		{"peer1", "/proto1/1.0.0"},
		{"peer1", "/proto2/1.0.0"},
		{"peer2", "/proto1/1.0.0"},
		{"peer2", "/proto2/1.0.0"},
	}

	for _, combo := range combinations {
		err := vcm.SendToQueue(combo.peerID, combo.protocol, map[string]string{"test": "data"})
		if err != nil {
			t.Fatalf("SendToQueue failed for %s/%s: %v", combo.peerID, combo.protocol, err)
		}
	}

	// Give queues time to be created
	time.Sleep(100 * time.Millisecond)

	// Check that all queues were created
	vcm.mu.RLock()
	queueCount := len(vcm.queues)
	vcm.mu.RUnlock()

	expectedCount := len(combinations)
	if queueCount != expectedCount {
		t.Errorf("Expected %d queues, got %d", expectedCount, queueCount)
	}
}

// TestVirtualConnectionManager_Close tests cleanup
func TestVirtualConnectionManager_Close(t *testing.T) {
	ctx := context.Background()

	manager := &Manager{
		ctx:         ctx,
		peers:       make(map[string]*Peer),
		peerAliases: make(map[string]string),
		verbosity:   0,
	}

	peer := &Peer{
		ctx:     ctx,
		manager: manager,
	}

	vcm := NewVirtualConnectionManager(ctx, peer)

	// Create some queues
	vcm.SendToQueue("peer1", "/test/1.0.0", map[string]string{"test": "data"})
	vcm.SendToQueue("peer2", "/test/1.0.0", map[string]string{"test": "data"})

	time.Sleep(50 * time.Millisecond)

	// Close should not panic
	err := vcm.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

// TestQueuedMessage_Properties tests QueuedMessage structure
func TestQueuedMessage_Properties(t *testing.T) {
	msg := QueuedMessage{
		id:          "msg-123",
		data:        []byte(`{"test":"data"}`),
		attempts:    0,
		maxAttempts: 3,
		timestamp:   time.Now(),
	}

	if msg.id != "msg-123" {
		t.Errorf("Expected id 'msg-123', got '%s'", msg.id)
	}

	if msg.attempts != 0 {
		t.Errorf("Expected 0 attempts, got %d", msg.attempts)
	}

	if msg.maxAttempts != 3 {
		t.Errorf("Expected 3 maxAttempts, got %d", msg.maxAttempts)
	}

	if string(msg.data) != `{"test":"data"}` {
		t.Errorf("Data mismatch: got %s", string(msg.data))
	}
}
