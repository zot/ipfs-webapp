package protocol

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Handler routes and processes protocol messages
type Handler struct {
	peerManager PeerManager
	nextReqID   int
	mu          sync.Mutex
	pending     map[int]chan *Message
}

// PeerManager interface for peer operations (Dependency Inversion)
type PeerManager interface {
	CreatePeer(requestedPeerKey string) (peerID string, peerKey string, err error)
	RemovePeer(peerID string) error
	Start(peerID, protocol string) error
	Stop(peerID, protocol string) error
	Send(peerID, targetPeer, protocol string, data any) error
	Subscribe(peerID, topic string) error
	Publish(peerID, topic string, data any) error
	Unsubscribe(peerID, topic string) error
	ListPeers(peerID, topic string) ([]string, error)
}

// NewHandler creates a new protocol handler
func NewHandler(pm PeerManager) *Handler {
	return &Handler{
		peerManager: pm,
		pending:     make(map[int]chan *Message),
	}
}

// HandleClientMessage processes messages from the client
func (h *Handler) HandleClientMessage(msg *Message, peerID string) (*Message, error) {
	switch msg.Method {
	case "peer":
		return h.handlePeer(msg)
	case "start":
		return h.handleStart(msg, peerID)
	case "stop":
		return h.handleStop(msg, peerID)
	case "send":
		return h.handleSend(msg, peerID)
	case "subscribe":
		return h.handleSubscribe(msg, peerID)
	case "publish":
		return h.handlePublish(msg, peerID)
	case "unsubscribe":
		return h.handleUnsubscribe(msg, peerID)
	case "listpeers":
		return h.handleListPeers(msg, peerID)
	default:
		return h.errorResponse(msg.RequestID, 400, fmt.Sprintf("unknown method: %s", msg.Method))
	}
}

// Client request handlers

func (h *Handler) handlePeer(msg *Message) (*Message, error) {
	var req PeerRequest
	if msg.Params != nil {
		if err := json.Unmarshal(msg.Params, &req); err != nil {
			return h.errorResponse(msg.RequestID, 400, "invalid params")
		}
	}

	peerID, peerKey, err := h.peerManager.CreatePeer(req.PeerKey)
	if err != nil {
		return h.errorResponse(msg.RequestID, 500, err.Error())
	}

	return h.peerResponse(msg.RequestID, peerID, peerKey)
}

func (h *Handler) handleStart(msg *Message, peerID string) (*Message, error) {
	var req StartRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return h.errorResponse(msg.RequestID, 400, "invalid params")
	}

	if err := h.peerManager.Start(peerID, req.Protocol); err != nil {
		return h.errorResponse(msg.RequestID, 500, err.Error())
	}

	return h.emptyResponse(msg.RequestID)
}

func (h *Handler) handleStop(msg *Message, peerID string) (*Message, error) {
	var req StopRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return h.errorResponse(msg.RequestID, 400, "invalid params")
	}

	if err := h.peerManager.Stop(peerID, req.Protocol); err != nil {
		return h.errorResponse(msg.RequestID, 500, err.Error())
	}

	return h.emptyResponse(msg.RequestID)
}

func (h *Handler) handleSend(msg *Message, peerID string) (*Message, error) {
	var req SendRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return h.errorResponse(msg.RequestID, 400, "invalid params")
	}

	if err := h.peerManager.Send(peerID, req.Peer, req.Protocol, req.Data); err != nil {
		return h.errorResponse(msg.RequestID, 500, err.Error())
	}

	return h.emptyResponse(msg.RequestID)
}

func (h *Handler) handleSubscribe(msg *Message, peerID string) (*Message, error) {
	var req SubscribeRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return h.errorResponse(msg.RequestID, 400, "invalid params")
	}

	if err := h.peerManager.Subscribe(peerID, req.Topic); err != nil {
		return h.errorResponse(msg.RequestID, 500, err.Error())
	}

	return h.emptyResponse(msg.RequestID)
}

func (h *Handler) handlePublish(msg *Message, peerID string) (*Message, error) {
	var req PublishRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return h.errorResponse(msg.RequestID, 400, "invalid params")
	}

	if err := h.peerManager.Publish(peerID, req.Topic, req.Data); err != nil {
		return h.errorResponse(msg.RequestID, 500, err.Error())
	}

	return h.emptyResponse(msg.RequestID)
}

func (h *Handler) handleUnsubscribe(msg *Message, peerID string) (*Message, error) {
	var req UnsubscribeRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return h.errorResponse(msg.RequestID, 400, "invalid params")
	}

	if err := h.peerManager.Unsubscribe(peerID, req.Topic); err != nil {
		return h.errorResponse(msg.RequestID, 500, err.Error())
	}

	return h.emptyResponse(msg.RequestID)
}

func (h *Handler) handleListPeers(msg *Message, peerID string) (*Message, error) {
	var req ListPeersRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return h.errorResponse(msg.RequestID, 400, "invalid params")
	}

	peers, err := h.peerManager.ListPeers(peerID, req.Topic)
	if err != nil {
		return h.errorResponse(msg.RequestID, 500, err.Error())
	}

	resp := ListPeersResponse{Peers: peers}
	result, _ := json.Marshal(resp)
	return &Message{
		RequestID:  msg.RequestID,
		IsResponse: true,
		Result:     result,
	}, nil
}

// Server message senders (to be called by peer manager)

func (h *Handler) NextRequestID() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.nextReqID
	h.nextReqID++
	return id
}

func (h *Handler) CreatePeerDataMessage(peer, protocol string, data any) *Message {
	req := PeerDataRequest{
		Peer:     peer,
		Protocol: protocol,
		Data:     data,
	}
	params, _ := json.Marshal(req)
	return &Message{
		RequestID: h.NextRequestID(),
		Method:    "peerData",
		Params:    params,
	}
}

func (h *Handler) CreateTopicDataMessage(topic, peerID string, data any) *Message {
	req := TopicDataRequest{
		Topic:  topic,
		PeerID: peerID,
		Data:   data,
	}
	params, _ := json.Marshal(req)
	return &Message{
		RequestID: h.NextRequestID(),
		Method:    "topicData",
		Params:    params,
	}
}

// Response helpers

func (h *Handler) emptyResponse(requestID int) (*Message, error) {
	return &Message{
		RequestID:  requestID,
		IsResponse: true,
		Result:     json.RawMessage("null"),
	}, nil
}

func (h *Handler) stringResponse(requestID int, value string) (*Message, error) {
	resp := StringResponse{Value: value}
	result, _ := json.Marshal(resp)
	return &Message{
		RequestID:  requestID,
		IsResponse: true,
		Result:     result,
	}, nil
}

func (h *Handler) peerResponse(requestID int, peerID, peerKey string) (*Message, error) {
	resp := PeerResponse{PeerID: peerID, PeerKey: peerKey}
	result, _ := json.Marshal(resp)
	return &Message{
		RequestID:  requestID,
		IsResponse: true,
		Result:     result,
	}, nil
}

func (h *Handler) errorResponse(requestID, code int, message string) (*Message, error) {
	return &Message{
		RequestID:  requestID,
		IsResponse: true,
		Error: &ErrorResponse{
			Code:    code,
			Message: message,
		},
	}, nil
}
