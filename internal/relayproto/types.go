package relayproto

import "encoding/json"

type MachineMetadata struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// EnrollmentRequest exchanges a one-time enrollment token for connector identity assignment.
type EnrollmentRequest struct {
	EnrollmentToken  string          `json:"enrollment_token,omitempty"`
	EnrollmentSecret string          `json:"enrollment_secret,omitempty"`
	Label            string          `json:"label,omitempty"`
	PublicKey        string          `json:"public_key"`
	Machine          MachineMetadata `json:"machine"`
}

type RelayMetadata struct {
	RelayURL                 string `json:"relay_url,omitempty"`
	WebsocketURL             string `json:"websocket_url,omitempty"`
	HeartbeatIntervalSeconds int    `json:"heartbeat_interval_seconds"`
}

// EnrollmentResponse returns the issued connector and machine identities plus relay metadata.
type EnrollmentResponse struct {
	ConnectorID string        `json:"connector_id"`
	MachineID   string        `json:"machine_id"`
	Relay       RelayMetadata `json:"relay"`
}

type ChallengeRequest struct {
	ConnectorID string `json:"connector_id"`
	MachineID   string `json:"machine_id"`
}

type ChallengeResponse struct {
	ChallengeID string        `json:"challenge_id"`
	Nonce       string        `json:"nonce"`
	Relay       RelayMetadata `json:"relay"`
}

type HelloMessage struct {
	Type        string `json:"type"`
	ConnectorID string `json:"connector_id"`
	MachineID   string `json:"machine_id"`
	ChallengeID string `json:"challenge_id"`
	Signature   string `json:"signature"`
}

type InstanceAdvertisement struct {
	Instance json.RawMessage `json:"instance"`
}

type AdvertiseMessage struct {
	Type      string                  `json:"type"`
	Instances []InstanceAdvertisement `json:"instances"`
}

type HeartbeatMessage struct {
	Type        string   `json:"type"`
	ConnectorID string   `json:"connector_id"`
	MachineID   string   `json:"machine_id"`
	InstanceIDs []string `json:"instance_ids,omitempty"`
	SentAt      string   `json:"sent_at"`
}

// CommandRequest is a relay-to-connector proxy request.
type CommandRequest struct {
	Type            string          `json:"type"`
	RequestID       string          `json:"request_id"`
	InstanceID      string          `json:"instance_id"`
	Method          string          `json:"method"`
	Path            string          `json:"path"`
	Query           string          `json:"query,omitempty"`
	ContentType     string          `json:"content_type,omitempty"`
	ContentEncoding string          `json:"content_encoding,omitempty"`
	Body            json.RawMessage `json:"body,omitempty"`
	TimeoutMs       int             `json:"timeout_ms,omitempty"`
}

// CommandResponse is a connector-to-relay proxy response.
type CommandResponse struct {
	Type            string          `json:"type"`
	RequestID       string          `json:"request_id"`
	StatusCode      int             `json:"status_code"`
	ContentType     string          `json:"content_type,omitempty"`
	ContentEncoding string          `json:"content_encoding,omitempty"`
	Body            json.RawMessage `json:"body,omitempty"`
	Error           string          `json:"error,omitempty"`
}

type ErrorMessage struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}
