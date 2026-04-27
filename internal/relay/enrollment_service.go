package relay

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"agent-bridge/internal/relayproto"
)

type EnrollmentService struct {
	cfg   *Config
	store *Store
}

func NewEnrollmentService(cfg *Config, store *Store) *EnrollmentService {
	return &EnrollmentService{cfg: cfg, store: store}
}

func (s *EnrollmentService) CreateToken(ctx context.Context, createdBy, label string, expiresIn time.Duration) (*EnrollmentTokenRecord, string, error) {
	var expiresAt *time.Time
	if expiresIn > 0 {
		value := time.Now().UTC().Add(expiresIn)
		expiresAt = &value
	}
	return s.store.CreateEnrollmentToken(ctx, label, createdBy, expiresAt)
}

func (s *EnrollmentService) Enroll(ctx context.Context, request relayproto.EnrollmentRequest) (*relayproto.EnrollmentResponse, *ConnectorRecord, error) {
	if request.PublicKey == "" {
		return nil, nil, fmt.Errorf("public_key is required")
	}
	connectorID, err := randomID("connector")
	if err != nil {
		return nil, nil, err
	}
	machineID, err := randomID("machine")
	if err != nil {
		return nil, nil, err
	}
	token := request.EnrollmentToken
	if token == "" {
		token = request.EnrollmentSecret
	}
	if token == "" {
		return nil, nil, fmt.Errorf("enrollment token is required")
	}
	if s.cfg.EnrollmentSecret != "" && token == s.cfg.EnrollmentSecret {
		// Compatibility bootstrap path for existing self-host setups.
	} else {
		if _, err := s.store.ConsumeEnrollmentToken(ctx, token, connectorID); err != nil {
			return nil, nil, err
		}
	}
	machineJSON, _ := json.Marshal(request.Machine)
	record := &ConnectorRecord{
		ConnectorID:         connectorID,
		MachineID:           machineID,
		PublicKey:           request.PublicKey,
		Label:               request.Label,
		MachineMetadataJSON: string(machineJSON),
	}
	if err := s.store.SaveConnectorRecord(ctx, *record); err != nil {
		return nil, nil, err
	}
	return &relayproto.EnrollmentResponse{
		ConnectorID: connectorID,
		MachineID:   machineID,
	}, record, nil
}

func (s *EnrollmentService) CreateChallenge(ctx context.Context, request relayproto.ChallengeRequest) (*ChallengeRecord, error) {
	record, err := s.store.GetConnector(ctx, request.ConnectorID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.MachineID != request.MachineID {
		return nil, fmt.Errorf("unknown connector or machine")
	}
	if record.Disabled {
		return nil, fmt.Errorf("connector disabled")
	}
	challengeID, err := randomID("challenge")
	if err != nil {
		return nil, err
	}
	nonce, err := randomID("nonce")
	if err != nil {
		return nil, err
	}
	challenge := &ChallengeRecord{
		ChallengeID: challengeID,
		ConnectorID: request.ConnectorID,
		MachineID:   request.MachineID,
		Nonce:       nonce,
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(time.Duration(s.cfg.ChallengeTTLSeconds) * time.Second),
	}
	if err := s.store.SaveChallenge(ctx, *challenge); err != nil {
		return nil, err
	}
	return challenge, nil
}

func (s *EnrollmentService) VerifyHello(ctx context.Context, hello relayproto.HelloMessage) (*ConnectorRecord, error) {
	record, err := s.store.GetConnector(ctx, hello.ConnectorID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.MachineID != hello.MachineID {
		return nil, fmt.Errorf("unknown connector or machine")
	}
	if record.Disabled {
		return nil, fmt.Errorf("connector disabled")
	}
	challenge, err := s.store.ConsumeChallenge(ctx, hello.ChallengeID)
	if err != nil {
		return nil, err
	}
	if challenge == nil || time.Now().UTC().After(challenge.ExpiresAt) {
		return nil, fmt.Errorf("challenge expired or not found")
	}
	if challenge.ConnectorID != hello.ConnectorID || challenge.MachineID != hello.MachineID {
		return nil, fmt.Errorf("challenge identity mismatch")
	}
	publicKeyBytes, err := base64.StdEncoding.DecodeString(record.PublicKey)
	if err != nil {
		return nil, err
	}
	signature, err := base64.StdEncoding.DecodeString(hello.Signature)
	if err != nil {
		return nil, err
	}
	payload := []byte(hello.ChallengeID + ":" + challenge.Nonce + ":" + hello.ConnectorID + ":" + hello.MachineID)
	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), payload, signature) {
		return nil, fmt.Errorf("invalid connector signature")
	}
	_ = s.store.MarkConnectorSeen(ctx, hello.ConnectorID, time.Now().UTC())
	return record, nil
}

func enrollmentErrorStatus(err error) int {
	switch err.Error() {
	case "invalid enrollment token", "enrollment token already used", "unknown connector or machine", "connector disabled", "invalid connector signature", "challenge expired or not found", "challenge identity mismatch":
		return http.StatusUnauthorized
	default:
		return http.StatusBadRequest
	}
}
