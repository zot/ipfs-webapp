package protocol

import (
	"encoding/json"
	"testing"
)

func TestMessageSerialization(t *testing.T) {
	tests := []struct {
		name    string
		message Message
	}{
		{
			name: "empty response",
			message: Message{
				RequestID:  1,
				IsResponse: true,
				Result:     json.RawMessage("null"),
			},
		},
		{
			name: "string response",
			message: Message{
				RequestID:  2,
				IsResponse: true,
				Result:     json.RawMessage(`{"value":"test-peer-id"}`),
			},
		},
		{
			name: "error response",
			message: Message{
				RequestID:  3,
				IsResponse: true,
				Error: &ErrorResponse{
					Code:    500,
					Message: "test error",
				},
			},
		},
		{
			name: "peer request",
			message: Message{
				RequestID:  4,
				Method:     "peer",
				Params:     json.RawMessage(`{"peerid":"test-id"}`),
				IsResponse: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			// Deserialize
			var decoded Message
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal message: %v", err)
			}

			// Verify
			if decoded.RequestID != tt.message.RequestID {
				t.Errorf("RequestID mismatch: got %d, want %d", decoded.RequestID, tt.message.RequestID)
			}

			if decoded.Method != tt.message.Method {
				t.Errorf("Method mismatch: got %s, want %s", decoded.Method, tt.message.Method)
			}

			if decoded.IsResponse != tt.message.IsResponse {
				t.Errorf("IsResponse mismatch: got %v, want %v", decoded.IsResponse, tt.message.IsResponse)
			}
		})
	}
}

func TestPeerRequestSerialization(t *testing.T) {
	req := PeerRequest{
		PeerID: "test-peer-123",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal PeerRequest: %v", err)
	}

	var decoded PeerRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal PeerRequest: %v", err)
	}

	if decoded.PeerID != req.PeerID {
		t.Errorf("PeerID mismatch: got %s, want %s", decoded.PeerID, req.PeerID)
	}
}

func TestStartRequestSerialization(t *testing.T) {
	req := StartRequest{
		Protocol: "/test/1.0.0",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal StartRequest: %v", err)
	}

	var decoded StartRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal StartRequest: %v", err)
	}

	if decoded.Protocol != req.Protocol {
		t.Errorf("Protocol mismatch: got %s, want %s", decoded.Protocol, req.Protocol)
	}
}

func TestSendRequestSerialization(t *testing.T) {
	testData := map[string]any{
		"message": "hello",
		"count":   42,
	}

	req := SendRequest{
		Peer:     "target-peer",
		Protocol: "/test/1.0.0",
		Data:     testData,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal SendRequest: %v", err)
	}

	var decoded SendRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal SendRequest: %v", err)
	}

	if decoded.Peer != req.Peer {
		t.Errorf("Peer mismatch: got %s, want %s", decoded.Peer, req.Peer)
	}

	if decoded.Protocol != req.Protocol {
		t.Errorf("Protocol mismatch: got %s, want %s", decoded.Protocol, req.Protocol)
	}

	if decoded.Data == nil {
		t.Error("Data should not be nil")
	}
}

func TestPublishRequestWithAnyData(t *testing.T) {
	tests := []struct {
		name string
		data any
	}{
		{
			name: "string data",
			data: "hello world",
		},
		{
			name: "number data",
			data: 42,
		},
		{
			name: "object data",
			data: map[string]any{
				"message": "test",
				"count":   3,
			},
		},
		{
			name: "array data",
			data: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := PublishRequest{
				Topic: "test-topic",
				Data:  tt.data,
			}

			data, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("Failed to marshal PublishRequest: %v", err)
			}

			var decoded PublishRequest
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal PublishRequest: %v", err)
			}

			if decoded.Topic != req.Topic {
				t.Errorf("Topic mismatch: got %s, want %s", decoded.Topic, req.Topic)
			}

			// Data is preserved as interface{}
			if decoded.Data == nil {
				t.Error("Data is nil after unmarshaling")
			}
		})
	}
}
