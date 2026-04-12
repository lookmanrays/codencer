package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"agent-bridge/internal/domain"
	"agent-bridge/internal/relayproto"
	"github.com/gorilla/websocket"
)

type Server struct {
	cfg        *Config
	store      *Store
	hub        *Hub
	server     *http.Server
	upgrader   websocket.Upgrader
	enrollment *EnrollmentService
	auditor    *Auditor
	mcp        *mcpServer
}

func NewServer(cfg *Config, store *Store) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	_ = cfg.Validate()
	s := &Server{
		cfg:        cfg,
		store:      store,
		hub:        NewHub(time.Duration(cfg.SessionTTLSeconds) * time.Second),
		enrollment: NewEnrollmentService(cfg, store),
		auditor:    NewAuditor(store),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	s.mcp = newMCPServer(s)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/connectors/enroll", s.handleEnroll)
	mux.HandleFunc("/api/v2/connectors/challenge", s.handleChallenge)
	mux.HandleFunc("/api/v2/connectors/enrollment-tokens", s.withPlannerScope("connectors:enroll", nil, s.handleEnrollmentTokens))
	mux.HandleFunc("/api/v2/instances", s.withPlannerScope("instances:read", nil, s.handleInstances))
	mux.HandleFunc("/api/v2/instances/", s.withPlannerScope("", relayInstanceIDFromRequest, s.handleInstanceScoped))
	mux.HandleFunc("/api/v2/steps/", s.withPlannerScope("", nil, s.handleStepScoped))
	mux.HandleFunc("/api/v2/artifacts/", s.withPlannerScope("artifacts:read", nil, s.handleArtifactScoped))
	mux.HandleFunc("/api/v2/gates/", s.withPlannerScope("gates:write", nil, s.handleGateScoped))
	mux.HandleFunc("/ws/connectors", s.handleConnectorWebSocket)
	mux.HandleFunc("/mcp", s.withPlannerScope("", nil, s.mcp.Handle))
	mux.HandleFunc("/mcp/call", s.withPlannerScope("", nil, s.mcp.Handle))

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	return s
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var request relayproto.EnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
		return
	}
	response, record, err := s.enrollment.Enroll(r.Context(), request)
	if err != nil {
		writeAPIError(w, enrollmentErrorStatus(err), "auth_failed", err.Error())
		return
	}
	response.Relay = relayproto.RelayMetadata{
		RelayURL:                 relayBaseURL(r),
		WebsocketURL:             websocketURL(r, "/ws/connectors"),
		HeartbeatIntervalSeconds: s.cfg.HeartbeatIntervalSeconds,
	}
	s.auditor.Record(r.Context(), AuditEvent{
		ActorType:         "connector",
		Action:            "enroll",
		ResourceKind:      "connector",
		ResourceID:        record.ConnectorID,
		TargetConnectorID: record.ConnectorID,
		Outcome:           "ok",
	})
	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var request relayproto.ChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
		return
	}
	challenge, err := s.enrollment.CreateChallenge(r.Context(), request)
	if err != nil {
		writeAPIError(w, enrollmentErrorStatus(err), "auth_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, relayproto.ChallengeResponse{
		ChallengeID: challenge.ChallengeID,
		Nonce:       challenge.Nonce,
		Relay: relayproto.RelayMetadata{
			RelayURL:                 relayBaseURL(r),
			WebsocketURL:             websocketURL(r, "/ws/connectors"),
			HeartbeatIntervalSeconds: s.cfg.HeartbeatIntervalSeconds,
		},
	})
}

func (s *Server) handleConnectorWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var hello relayproto.HelloMessage
	if err := conn.ReadJSON(&hello); err != nil {
		_ = conn.WriteJSON(relayproto.ErrorMessage{Type: "error", Code: "malformed_request", Message: err.Error()})
		return
	}
	if hello.Type != "hello" {
		_ = conn.WriteJSON(relayproto.ErrorMessage{Type: "error", Code: "auth_failed", Message: "hello message required"})
		return
	}
	record, err := s.enrollment.VerifyHello(r.Context(), hello)
	if err != nil {
		_ = conn.WriteJSON(relayproto.ErrorMessage{Type: "error", Code: "auth_failed", Message: err.Error()})
		s.auditor.Record(r.Context(), AuditEvent{
			ActorType:         "connector",
			Action:            "session_connect",
			ResourceKind:      "connector",
			ResourceID:        hello.ConnectorID,
			TargetConnectorID: hello.ConnectorID,
			Outcome:           "error",
			ErrorCode:         "auth_failed",
		})
		return
	}

	session := &session{
		conn:        conn,
		connectorID: hello.ConnectorID,
		machineID:   hello.MachineID,
		instanceIDs: make(map[string]struct{}),
		pending:     make(map[string]chan relayproto.CommandResponse),
		lastSeenAt:  time.Now().UTC(),
	}
	s.hub.RegisterConnector(session)
	s.auditor.Record(r.Context(), AuditEvent{
		ActorType:         "connector",
		Action:            "session_connect",
		ResourceKind:      "connector",
		ResourceID:        record.ConnectorID,
		TargetConnectorID: record.ConnectorID,
		Outcome:           "ok",
	})
	defer s.hub.RemoveSession(session)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(message, &envelope); err != nil {
			_ = conn.WriteJSON(relayproto.ErrorMessage{Type: "error", Code: "malformed_request", Message: err.Error()})
			return
		}
		switch envelope.Type {
		case "advertise":
			if err := s.handleAdvertise(r.Context(), session, message); err != nil {
				_ = conn.WriteJSON(relayproto.ErrorMessage{Type: "error", Code: "malformed_request", Message: err.Error()})
				return
			}
		case "heartbeat":
			s.handleHeartbeat(r.Context(), session, message)
		case "response":
			if err := session.handleMessage(message); err != nil {
				_ = conn.WriteJSON(relayproto.ErrorMessage{Type: "error", Code: "malformed_request", Message: err.Error()})
				return
			}
		case "error":
			return
		default:
			_ = conn.WriteJSON(relayproto.ErrorMessage{Type: "error", Code: "malformed_request", Message: "unsupported session message"})
			return
		}
	}
}

func (s *Server) handleAdvertise(ctx context.Context, session *session, message []byte) error {
	var advertise relayproto.AdvertiseMessage
	if err := json.Unmarshal(message, &advertise); err != nil {
		return err
	}
	s.hub.Touch(session)
	next := make(map[string]struct{})
	for _, advertised := range advertise.Instances {
		var info domain.InstanceInfo
		if err := json.Unmarshal(advertised.Instance, &info); err != nil {
			return err
		}
		record := InstanceRecord{
			InstanceID:   info.ID,
			ConnectorID:  session.connectorID,
			RepoRoot:     info.RepoRoot,
			BaseURL:      info.BaseURL,
			InstanceJSON: string(advertised.Instance),
			LastSeenAt:   time.Now().UTC(),
		}
		if err := s.store.SaveInstance(ctx, record); err != nil {
			return err
		}
		s.hub.Register(info.ID, session)
		next[info.ID] = struct{}{}
	}
	for instanceID := range session.instanceIDs {
		if _, ok := next[instanceID]; !ok {
			s.hub.Remove(instanceID)
		}
	}
	session.instanceIDs = next
	return nil
}

func (s *Server) handleHeartbeat(ctx context.Context, session *session, message []byte) {
	var heartbeat relayproto.HeartbeatMessage
	if err := json.Unmarshal(message, &heartbeat); err != nil {
		return
	}
	s.hub.Touch(session)
	_ = s.store.MarkConnectorSeen(ctx, session.connectorID, time.Now().UTC())
	for _, instanceID := range heartbeat.InstanceIDs {
		_ = s.store.TouchInstance(ctx, instanceID)
	}
}

func relayInstanceIDFromRequest(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/instances/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
