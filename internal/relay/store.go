package relay

import relaystore "agent-bridge/internal/relay/store"

type Store = relaystore.Store
type InstanceRecord = relaystore.InstanceRecord
type ProjectRecord = relaystore.ProjectRecord
type ConnectorRecord = relaystore.ConnectorRecord
type EnrollmentTokenRecord = relaystore.EnrollmentTokenRecord
type ChallengeRecord = relaystore.ChallengeRecord
type AuditEvent = relaystore.AuditEvent

func OpenStore(path string) (*Store, error) {
	return relaystore.Open(path)
}
