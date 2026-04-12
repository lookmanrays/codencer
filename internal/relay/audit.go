package relay

import (
	"context"
)

type Auditor struct {
	store *Store
}

func NewAuditor(store *Store) *Auditor {
	return &Auditor{store: store}
}

func (a *Auditor) Record(ctx context.Context, event AuditEvent) {
	if a == nil || a.store == nil {
		return
	}
	_ = a.store.AppendAudit(ctx, event)
}
