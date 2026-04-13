package relay

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"agent-bridge/internal/relayproto"
	"github.com/gorilla/websocket"
)

type session struct {
	conn        *websocket.Conn
	connectorID string
	machineID   string
	instanceIDs map[string]struct{}
	lastSeenAt  time.Time
	writeMu     sync.Mutex
	pendingMu   sync.Mutex
	pending     map[string]chan relayproto.CommandResponse
}

type Hub struct {
	mu         sync.RWMutex
	sessions   map[string]*session
	connectors map[string]*session
	sessionTTL time.Duration
}

func NewHub(sessionTTL time.Duration) *Hub {
	if sessionTTL <= 0 {
		sessionTTL = 45 * time.Second
	}
	return &Hub{
		sessions:   make(map[string]*session),
		connectors: make(map[string]*session),
		sessionTTL: sessionTTL,
	}
}

func (h *Hub) RegisterConnector(session *session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	session.lastSeenAt = time.Now().UTC()
	h.connectors[session.connectorID] = session
}

func (h *Hub) Register(instanceID string, session *session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	session.lastSeenAt = time.Now().UTC()
	h.sessions[instanceID] = session
}

func (h *Hub) Remove(instanceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, instanceID)
}

func (h *Hub) RemoveSession(session *session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for instanceID, candidate := range h.sessions {
		if candidate == session {
			delete(h.sessions, instanceID)
		}
	}
	if current := h.connectors[session.connectorID]; current == session {
		delete(h.connectors, session.connectorID)
	}
}

func (h *Hub) Touch(session *session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	session.lastSeenAt = time.Now().UTC()
}

func (h *Hub) Get(instanceID string) *session {
	h.mu.RLock()
	session := h.sessions[instanceID]
	h.mu.RUnlock()
	if session == nil {
		return nil
	}
	if time.Since(session.lastSeenAt) > h.sessionTTL {
		h.Remove(instanceID)
		return nil
	}
	return session
}

func (h *Hub) Connector(connectorID string) *session {
	h.mu.RLock()
	session := h.connectors[connectorID]
	h.mu.RUnlock()
	if session == nil {
		return nil
	}
	if time.Since(session.lastSeenAt) > h.sessionTTL {
		h.RemoveSession(session)
		return nil
	}
	return session
}

func (s *session) send(value any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteJSON(value)
}

func (s *session) proxy(ctx context.Context, request relayproto.CommandRequest) (*relayproto.CommandResponse, error) {
	responseCh := make(chan relayproto.CommandResponse, 1)

	s.pendingMu.Lock()
	s.pending[request.RequestID] = responseCh
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, request.RequestID)
		s.pendingMu.Unlock()
	}()

	if err := s.send(request); err != nil {
		return nil, err
	}

	select {
	case response := <-responseCh:
		return &response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *session) handleMessage(message []byte) error {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(message, &envelope); err != nil {
		return err
	}

	if envelope.Type != "response" {
		return nil
	}

	var response relayproto.CommandResponse
	if err := json.Unmarshal(message, &response); err != nil {
		return err
	}

	s.pendingMu.Lock()
	ch := s.pending[response.RequestID]
	s.pendingMu.Unlock()
	if ch != nil {
		ch <- response
	}
	return nil
}
