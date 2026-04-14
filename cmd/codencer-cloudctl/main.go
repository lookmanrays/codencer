package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"agent-bridge/internal/cloud"
	cloudconnectors "agent-bridge/internal/cloud/connectors"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: codencer-cloudctl <bootstrap|status|orgs|workspaces|projects|tokens|install|events|audit> [flags]")
	}
	switch args[0] {
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr)
	case "status":
		target, asJSON, err := parseSimpleTarget("status", args[1:], stderr)
		if err != nil {
			return err
		}
		return target.get("/api/cloud/v1/status", asJSON, stdout)
	case "orgs":
		return runResource("orgs", "/api/cloud/v1/orgs", args[1:], stdout, stderr)
	case "workspaces":
		return runResource("workspaces", "/api/cloud/v1/workspaces", args[1:], stdout, stderr)
	case "projects":
		return runResource("projects", "/api/cloud/v1/projects", args[1:], stdout, stderr)
	case "tokens":
		return runTokens(args[1:], stdout, stderr)
	case "install":
		return runInstall(args[1:], stdout, stderr)
	case "events":
		return runEvents(args[1:], stdout, stderr)
	case "audit":
		return runAudit(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown cloudctl command %q", args[0])
	}
}

func runBootstrap(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Cloud config path")
	orgSlug := fs.String("org-slug", "default-org", "Organization slug")
	orgName := fs.String("org-name", "Default Org", "Organization name")
	workspaceSlug := fs.String("workspace-slug", "default-workspace", "Workspace slug")
	workspaceName := fs.String("workspace-name", "Default Workspace", "Workspace name")
	projectSlug := fs.String("project-slug", "default-project", "Project slug")
	projectName := fs.String("project-name", "Default Project", "Project name")
	tokenName := fs.String("token-name", "operator", "Bootstrap token name")
	asJSON := fs.Bool("json", false, "Print JSON output")
	var scopes multiFlag
	fs.Var(&scopes, "scope", "Token scope; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(scopes) == 0 {
		scopes = append(scopes, "cloud:admin", "cloud:read", "orgs:read", "orgs:write", "workspaces:read", "workspaces:write", "projects:read", "projects:write", "tokens:read", "tokens:write", "installations:read", "installations:write", "events:read", "audit:read")
	}

	cfg, err := cloud.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	store, err := cloud.OpenStore(cfg.DBPath, cfg.MasterKey)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()

	org, err := store.CreateOrg(ctx, cloud.Org{Slug: *orgSlug, Name: *orgName})
	if err != nil && !strings.Contains(err.Error(), "UNIQUE") {
		return err
	}
	if org == nil {
		org, err = findOrCreateOrg(ctx, store, *orgSlug)
		if err != nil {
			return err
		}
	}
	workspace, err := store.CreateWorkspace(ctx, cloud.Workspace{OrgID: org.ID, Slug: *workspaceSlug, Name: *workspaceName})
	if err != nil && !strings.Contains(err.Error(), "UNIQUE") {
		return err
	}
	if workspace == nil {
		workspace, err = findOrCreateWorkspace(ctx, store, org.ID, *workspaceSlug)
		if err != nil {
			return err
		}
	}
	project, err := store.CreateProject(ctx, cloud.Project{OrgID: org.ID, WorkspaceID: workspace.ID, Slug: *projectSlug, Name: *projectName})
	if err != nil && !strings.Contains(err.Error(), "UNIQUE") {
		return err
	}
	if project == nil {
		project, err = findOrCreateProject(ctx, store, workspace.ID, *projectSlug)
		if err != nil {
			return err
		}
	}
	rawToken, err := cloud.GenerateAPIToken()
	if err != nil {
		return err
	}
	token, err := store.CreateAPIToken(ctx, cloud.APIToken{
		OrgID:       org.ID,
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		Name:        *tokenName,
		Scopes:      append([]string(nil), scopes...),
	}, rawToken)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"org":       org,
		"workspace": workspace,
		"project":   project,
		"token":     rawToken,
		"record":    token,
	}
	return printOutput(stdout, payload, *asJSON)
}

func runResource(name, path string, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "list" {
		target, query, asJSON, err := parseTargetWithFilter(name, args, stderr)
		if err != nil {
			return err
		}
		return target.get(path+query, asJSON, stdout)
	}
	if args[0] != "create" {
		return fmt.Errorf("usage: codencer-cloudctl %s [list|create] [flags]", name)
	}
	target, asJSON, body, err := parseCreateResourceTarget(name, args[1:], stderr)
	if err != nil {
		return err
	}
	return target.post(path, body, asJSON, stdout)
}

func runTokens(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "list" {
		target, query, asJSON, err := parseTargetWithFilter("tokens", args, stderr)
		if err != nil {
			return err
		}
		return target.get("/api/cloud/v1/tokens"+query, asJSON, stdout)
	}
	switch args[0] {
	case "create":
		target, asJSON, body, err := parseCreateTokenTarget(args[1:], stderr)
		if err != nil {
			return err
		}
		return target.post("/api/cloud/v1/tokens", body, asJSON, stdout)
	case "revoke":
		fs := flag.NewFlagSet("tokens revoke", flag.ContinueOnError)
		fs.SetOutput(stderr)
		tokenID := fs.String("token-id", "", "Token record id")
		target, asJSON, err := parseHTTPFlags(fs, args[1:])
		if err != nil {
			return err
		}
		if strings.TrimSpace(*tokenID) == "" {
			return fmt.Errorf("--token-id is required")
		}
		return target.post("/api/cloud/v1/tokens/"+*tokenID+"/revoke", nil, asJSON, stdout)
	default:
		return fmt.Errorf("usage: codencer-cloudctl tokens [list|create|revoke]")
	}
}

func runInstall(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "list" {
		target, query, asJSON, err := parseTargetWithFilter("install", args, stderr)
		if err != nil {
			return err
		}
		return target.get("/api/cloud/v1/installations"+query, asJSON, stdout)
	}
	switch args[0] {
	case "create":
		target, asJSON, body, err := parseCreateInstallTarget(args[1:], stderr)
		if err != nil {
			return err
		}
		return target.post("/api/cloud/v1/installations", body, asJSON, stdout)
	case "get":
		fs := flag.NewFlagSet("install get", flag.ContinueOnError)
		fs.SetOutput(stderr)
		installationID := fs.String("installation-id", "", "Installation id")
		target, asJSON, err := parseHTTPFlags(fs, args[1:])
		if err != nil {
			return err
		}
		if *installationID == "" {
			return fmt.Errorf("--installation-id is required")
		}
		return target.get("/api/cloud/v1/installations/"+*installationID, asJSON, stdout)
	case "validate":
		fs := flag.NewFlagSet("install validate", flag.ContinueOnError)
		fs.SetOutput(stderr)
		installationID := fs.String("installation-id", "", "Installation id")
		target, asJSON, err := parseHTTPFlags(fs, args[1:])
		if err != nil {
			return err
		}
		if *installationID == "" {
			return fmt.Errorf("--installation-id is required")
		}
		return target.post("/api/cloud/v1/installations/"+*installationID+"/validate", nil, asJSON, stdout)
	case "enable", "disable":
		fs := flag.NewFlagSet("install "+args[0], flag.ContinueOnError)
		fs.SetOutput(stderr)
		installationID := fs.String("installation-id", "", "Installation id")
		target, asJSON, err := parseHTTPFlags(fs, args[1:])
		if err != nil {
			return err
		}
		if *installationID == "" {
			return fmt.Errorf("--installation-id is required")
		}
		return target.post("/api/cloud/v1/installations/"+*installationID+"/"+args[0], nil, asJSON, stdout)
	case "action":
		target, installationID, asJSON, body, err := parseInstallActionTarget(args[1:], stderr)
		if err != nil {
			return err
		}
		return target.post("/api/cloud/v1/installations/"+installationID+"/actions", body, asJSON, stdout)
	default:
		return fmt.Errorf("usage: codencer-cloudctl install [list|create|get|validate|enable|disable|action]")
	}
}

func runEvents(args []string, stdout, stderr io.Writer) error {
	target, query, asJSON, err := parseTargetWithFilter("events", args, stderr)
	if err != nil {
		return err
	}
	return target.get("/api/cloud/v1/events"+query, asJSON, stdout)
}

func runAudit(args []string, stdout, stderr io.Writer) error {
	target, query, asJSON, err := parseTargetWithFilter("audit", args, stderr)
	if err != nil {
		return err
	}
	return target.get("/api/cloud/v1/audit"+query, asJSON, stdout)
}

type target struct {
	cloudURL string
	token    string
}

func (t target) get(path string, asJSON bool, stdout io.Writer) error {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(t.cloudURL, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.do(req, asJSON, stdout)
}

func parseSimpleTarget(name string, args []string, stderr io.Writer) (target, bool, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return parseHTTPFlags(fs, args)
}

func (t target) post(path string, body []byte, asJSON bool, stdout io.Writer) error {
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(t.cloudURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+t.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return t.do(req, asJSON, stdout)
}

func (t target) do(req *http.Request, asJSON bool, stdout io.Writer) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if asJSON {
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	_, err = fmt.Fprintln(stdout, pretty.String())
	return err
}

func parseHTTPFlags(fs *flag.FlagSet, args []string) (target, bool, error) {
	cloudURL := fs.String("cloud-url", "http://127.0.0.1:8190", "Cloud base URL")
	token := fs.String("token", "", "Cloud API bearer token")
	asJSON := fs.Bool("json", false, "Print raw JSON")
	if err := fs.Parse(args); err != nil {
		return target{}, false, err
	}
	if strings.TrimSpace(*token) == "" {
		return target{}, false, fmt.Errorf("--token is required")
	}
	return target{cloudURL: *cloudURL, token: *token}, *asJSON, nil
}

func parseTargetWithFilter(name string, args []string, stderr io.Writer) (target, string, bool, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	orgID := fs.String("org-id", "", "Organization id")
	workspaceID := fs.String("workspace-id", "", "Workspace id")
	projectID := fs.String("project-id", "", "Project id")
	installationID := fs.String("installation-id", "", "Installation id")
	limit := fs.Int("limit", 0, "List limit")
	target, asJSON, err := parseHTTPFlags(fs, args)
	if err != nil {
		return target, "", false, err
	}
	query := []string{}
	if *orgID != "" {
		query = append(query, "org_id="+*orgID)
	}
	if *workspaceID != "" {
		query = append(query, "workspace_id="+*workspaceID)
	}
	if *projectID != "" {
		query = append(query, "project_id="+*projectID)
	}
	if *installationID != "" {
		query = append(query, "installation_id="+*installationID)
	}
	if *limit > 0 {
		query = append(query, fmt.Sprintf("limit=%d", *limit))
	}
	if len(query) == 0 {
		return target, "", asJSON, nil
	}
	return target, "?" + strings.Join(query, "&"), asJSON, nil
}

func parseCreateResourceTarget(name string, args []string, stderr io.Writer) (target, bool, []byte, error) {
	fs := flag.NewFlagSet(name+" create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Optional id")
	orgID := fs.String("org-id", "", "Organization id")
	workspaceID := fs.String("workspace-id", "", "Workspace id")
	slug := fs.String("slug", "", "Slug")
	displayName := fs.String("name", "", "Display name")
	target, asJSON, err := parseHTTPFlags(fs, args)
	if err != nil {
		return target, false, nil, err
	}
	payload := map[string]string{
		"id":           *id,
		"org_id":       *orgID,
		"workspace_id": *workspaceID,
		"slug":         *slug,
		"name":         *displayName,
	}
	data, err := json.Marshal(payload)
	return target, asJSON, data, err
}

func parseCreateTokenTarget(args []string, stderr io.Writer) (target, bool, []byte, error) {
	fs := flag.NewFlagSet("tokens create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	orgID := fs.String("org-id", "", "Organization id")
	workspaceID := fs.String("workspace-id", "", "Workspace id")
	projectID := fs.String("project-id", "", "Project id")
	name := fs.String("name", "", "Token name")
	kind := fs.String("kind", "", "Token kind")
	var scopes multiFlag
	fs.Var(&scopes, "scope", "Token scope; repeatable")
	target, asJSON, err := parseHTTPFlags(fs, args)
	if err != nil {
		return target, false, nil, err
	}
	payload := map[string]any{
		"org_id":       *orgID,
		"workspace_id": *workspaceID,
		"project_id":   *projectID,
		"name":         *name,
		"kind":         *kind,
		"scopes":       []string(scopes),
	}
	data, err := json.Marshal(payload)
	return target, asJSON, data, err
}

func parseCreateInstallTarget(args []string, stderr io.Writer) (target, bool, []byte, error) {
	fs := flag.NewFlagSet("install create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	orgID := fs.String("org-id", "", "Organization id")
	workspaceID := fs.String("workspace-id", "", "Workspace id")
	projectID := fs.String("project-id", "", "Project id")
	connectorKey := fs.String("connector", "", "Connector key")
	name := fs.String("name", "", "Installation name")
	externalInstallationID := fs.String("external-installation-id", "", "External installation id")
	externalAccount := fs.String("external-account", "", "External account")
	var configs kvFlag
	var secrets kvFlag
	fs.Var(&configs, "config", "Config key=value; repeatable")
	fs.Var(&secrets, "secret", "Secret key=value; repeatable")
	target, asJSON, err := parseHTTPFlags(fs, args)
	if err != nil {
		return target, false, nil, err
	}
	payload := map[string]any{
		"org_id":                   *orgID,
		"workspace_id":             *workspaceID,
		"project_id":               *projectID,
		"connector_key":            *connectorKey,
		"name":                     *name,
		"external_installation_id": *externalInstallationID,
		"external_account":         *externalAccount,
		"config":                   map[string]string(configs),
		"secrets":                  map[string]string(secrets),
	}
	data, err := json.Marshal(payload)
	return target, asJSON, data, err
}

func parseInstallActionTarget(args []string, stderr io.Writer) (target, string, bool, []byte, error) {
	fs := flag.NewFlagSet("install action", flag.ContinueOnError)
	fs.SetOutput(stderr)
	installationID := fs.String("installation-id", "", "Installation id")
	action := fs.String("action", "", "Connector action")
	repository := fs.String("repository", "", "Repository (owner/name or namespaced project)")
	project := fs.String("project", "", "Project identifier")
	issueNumber := fs.Int("issue-number", 0, "Issue number")
	issueKey := fs.String("issue-key", "", "Issue key")
	channel := fs.String("channel", "", "Slack channel id")
	threadTS := fs.String("thread-ts", "", "Slack thread ts")
	teamID := fs.String("team-id", "", "Linear team id")
	title := fs.String("title", "", "Title")
	description := fs.String("description", "", "Description")
	body := fs.String("body", "", "Body")
	target, asJSON, err := parseHTTPFlags(fs, args)
	if err != nil {
		return target, "", false, nil, err
	}
	if *installationID == "" {
		return target, "", false, nil, fmt.Errorf("--installation-id is required")
	}
	payload := cloudconnectors.ActionRequest{
		Action:      cloudconnectors.ActionName(*action),
		Repository:  *repository,
		Project:     *project,
		IssueNumber: *issueNumber,
		IssueKey:    *issueKey,
		Channel:     *channel,
		ThreadTS:    *threadTS,
		TeamID:      *teamID,
		Title:       *title,
		Description: *description,
		Body:        *body,
	}
	data, err := json.Marshal(payload)
	return target, *installationID, asJSON, data, err
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		*m = append(*m, value)
	}
	return nil
}

type kvFlag map[string]string

func (k *kvFlag) String() string {
	if k == nil {
		return ""
	}
	data, _ := json.Marshal(*k)
	return string(data)
}

func (k *kvFlag) Set(value string) error {
	if *k == nil {
		*k = map[string]string{}
	}
	key, val, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return fmt.Errorf("expected key=value, got %q", value)
	}
	(*k)[strings.TrimSpace(key)] = val
	return nil
}

func printOutput(stdout io.Writer, payload any, asJSON bool) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if asJSON {
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	_, err = fmt.Fprintln(stdout, pretty.String())
	return err
}

func findOrCreateOrg(ctx context.Context, store *cloud.Store, slug string) (*cloud.Org, error) {
	orgs, err := store.ListOrgs(ctx)
	if err != nil {
		return nil, err
	}
	for _, org := range orgs {
		if org.Slug == slug {
			return &org, nil
		}
	}
	return nil, fmt.Errorf("org %q not found after create attempt", slug)
}

func findOrCreateWorkspace(ctx context.Context, store *cloud.Store, orgID, slug string) (*cloud.Workspace, error) {
	workspaces, err := store.ListWorkspaces(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, workspace := range workspaces {
		if workspace.Slug == slug {
			return &workspace, nil
		}
	}
	return nil, fmt.Errorf("workspace %q not found after create attempt", slug)
}

func findOrCreateProject(ctx context.Context, store *cloud.Store, workspaceID, slug string) (*cloud.Project, error) {
	projects, err := store.ListProjects(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if project.Slug == slug {
			return &project, nil
		}
	}
	return nil, fmt.Errorf("project %q not found after create attempt", slug)
}
