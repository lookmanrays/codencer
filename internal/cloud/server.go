package cloud

import (
	"context"
	"fmt"
	"net/http"
	"time"

	cloudconnectors "agent-bridge/internal/cloud/connectors"
	"agent-bridge/internal/relay"
)

// AdminStore captures the cloud persistence operations used by the HTTP admin
// surface. The concrete implementation lives in the cloud foundation files.
type AdminStore interface {
	Close() error
	LookupAPIToken(ctx context.Context, rawToken string) (*APIToken, error)
	TouchAPITokenUsage(ctx context.Context, tokenID string) error
	CreateOrg(ctx context.Context, org Org) (*Org, error)
	ListOrgs(ctx context.Context) ([]Org, error)
	CreateWorkspace(ctx context.Context, workspace Workspace) (*Workspace, error)
	ListWorkspaces(ctx context.Context, orgID string) ([]Workspace, error)
	CreateProject(ctx context.Context, project Project) (*Project, error)
	GetProject(ctx context.Context, id string) (*Project, error)
	ListProjects(ctx context.Context, workspaceID string) ([]Project, error)
	CreateMembership(ctx context.Context, membership Membership) (*Membership, error)
	GetMembership(ctx context.Context, id string) (*Membership, error)
	ListMemberships(ctx context.Context, orgID, workspaceID, projectID string) ([]Membership, error)
	CreateAPIToken(ctx context.Context, token APIToken, rawToken string) (*APIToken, error)
	ListAPITokens(ctx context.Context, orgID, workspaceID, projectID string) ([]APIToken, error)
	RevokeAPIToken(ctx context.Context, tokenID string) error
	CreateConnectorInstallation(ctx context.Context, installation ConnectorInstallation) (*ConnectorInstallation, error)
	DeleteConnectorInstallation(ctx context.Context, installationID string) error
	GetConnectorInstallation(ctx context.Context, installationID string) (*ConnectorInstallation, error)
	ListConnectorInstallations(ctx context.Context, orgID, workspaceID, projectID string) ([]ConnectorInstallation, error)
	PutInstallationSecret(ctx context.Context, installationID, secretName string, plaintext []byte) (*InstallationSecret, error)
	GetInstallationSecret(ctx context.Context, installationID, secretName string) ([]byte, error)
	CreateConnectorEvent(ctx context.Context, event ConnectorEvent) (*ConnectorEvent, error)
	ListConnectorEvents(ctx context.Context, installationID string, limit int) ([]ConnectorEvent, error)
	CreateConnectorActionLog(ctx context.Context, log ConnectorActionLog) (*ConnectorActionLog, error)
	CreateCloudAuditEvent(ctx context.Context, event CloudAuditEvent) (*CloudAuditEvent, error)
	ListCloudAuditEvents(ctx context.Context, limit int) ([]CloudAuditEvent, error)
	CreateRuntimeConnectorInstallation(ctx context.Context, installation RuntimeConnectorInstallation) (*RuntimeConnectorInstallation, error)
	UpsertRuntimeConnectorInstallation(ctx context.Context, installation RuntimeConnectorInstallation) (*RuntimeConnectorInstallation, error)
	UpdateRuntimeConnectorInstallation(ctx context.Context, installation RuntimeConnectorInstallation) (*RuntimeConnectorInstallation, error)
	GetRuntimeConnectorInstallation(ctx context.Context, id string) (*RuntimeConnectorInstallation, error)
	ListRuntimeConnectorInstallations(ctx context.Context, orgID, workspaceID, projectID string) ([]RuntimeConnectorInstallation, error)
	CreateRuntimeInstance(ctx context.Context, instance RuntimeInstance) (*RuntimeInstance, error)
	UpsertRuntimeInstance(ctx context.Context, instance RuntimeInstance) (*RuntimeInstance, error)
	UpdateRuntimeInstance(ctx context.Context, instance RuntimeInstance) (*RuntimeInstance, error)
	GetRuntimeInstance(ctx context.Context, id string) (*RuntimeInstance, error)
	ListRuntimeInstances(ctx context.Context, orgID, workspaceID, projectID, runtimeConnectorInstallationID string) ([]RuntimeInstance, error)
}

// RelayRuntime holds the trusted in-process relay runtime bridge used by the
// cloud control plane when runtime control is enabled.
type RelayRuntime struct {
	Server *relay.Server
	Store  *relay.Store
}

// Server is the cloud control-plane HTTP service. It composes cloud admin APIs
// with the in-process relay runtime bridge so tenant-scoped runtime control can
// reuse the existing local execution path without exposing raw relay state.
type Server struct {
	cfg        *Config
	store      AdminStore
	connectors *cloudconnectors.Registry
	runtime    *RelayRuntime
	mcp        *mcpServer
	handler    http.Handler
	server     *http.Server
	startedAt  time.Time
}

func NewServer(cfg *Config, store AdminStore, connectors *cloudconnectors.Registry, runtime *RelayRuntime) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	s := &Server{
		cfg:        cfg,
		store:      store,
		connectors: connectors,
		runtime:    runtime,
		startedAt:  time.Now().UTC(),
	}
	s.mcp = newMCPServer(s)
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	s.handler = mux
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	return s
}

func (s *Server) Handler() http.Handler {
	return s.handler
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
