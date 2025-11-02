package protocol

import "encoding/json"

// Message envelope for all WebSocket communications
type Message struct {
	RequestID  int             `json:"requestid"`
	Method     string          `json:"method"`
	Params     json.RawMessage `json:"params,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      *ErrorResponse  `json:"error,omitempty"`
	IsResponse bool            `json:"isresponse"`
}

// Shared response structs (minimize duplication)

// EmptyResponse is used for operations that return "null or error"
type EmptyResponse struct{}

// StringResponse is used for operations that return a single string (peerid, connectionID)
type StringResponse struct {
	Value string `json:"value"`
}

// ErrorResponse provides standardized error structure
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client Request Messages

// PeerRequest creates or restores a peer
type PeerRequest struct {
	PeerID string `json:"peerid,omitempty"`
}

// StartRequest starts listening for a protocol
type StartRequest struct {
	Protocol string `json:"protocol"`
}

// StopRequest stops listening for a protocol
type StopRequest struct {
	Protocol string `json:"protocol"`
}

// SendRequest sends data to a peer on a protocol
type SendRequest struct {
	Peer     string `json:"peer"`
	Protocol string `json:"protocol"`
	Data     any    `json:"data"`
}

// SubscribeRequest subscribes to a topic
type SubscribeRequest struct {
	Topic string `json:"topic"`
}

// PublishRequest publishes data to a topic
type PublishRequest struct {
	Topic string `json:"topic"`
	Data  any    `json:"data"`
}

// UnsubscribeRequest unsubscribes from a topic
type UnsubscribeRequest struct {
	Topic string `json:"topic"`
}

// ListPeersRequest lists peers subscribed to a topic
type ListPeersRequest struct {
	Topic string `json:"topic"`
}

// ListPeersResponse returns list of peer IDs
type ListPeersResponse struct {
	Peers []string `json:"peers"`
}

// Server Request Messages (sent from server to client)

// PeerDataRequest delivers data from a peer on a protocol
type PeerDataRequest struct {
	Peer     string `json:"peer"`
	Protocol string `json:"protocol"`
	Data     any    `json:"data"`
}

// TopicDataRequest delivers data from a topic
type TopicDataRequest struct {
	Topic  string `json:"topic"`
	PeerID string `json:"peerid"`
	Data   any    `json:"data"`
}
