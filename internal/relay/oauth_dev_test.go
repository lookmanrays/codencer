package relay_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"agent-bridge/internal/relay"
)

func TestOAuthDevMetadataPKCEAndBearerAcceptance(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cfg := &relay.Config{
		DBPath:        filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken:  "planner-token",
		PublicBaseURL: "https://relay.example.test",
		ChatGPTOAuthDev: relay.OAuthDevConfig{
			Enabled:          true,
			Issuer:           "https://relay.example.test",
			ClientID:         "codencer-chatgpt-dev",
			ClientSecretHash: hashSecret("client-secret"),
			OperatorCodeHash: hashSecret("approve-me"),
			Scopes:           []string{"projects:read", "reports:read"},
			TokenTTLSeconds:  600,
		},
	}
	server := relay.NewServer(cfg, store)
	relayHTTP := httptest.NewServer(server.Handler())
	defer relayHTTP.Close()

	resp, err := http.Get(relayHTTP.URL + "/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected metadata 200, got %d body=%s", resp.StatusCode, body)
	}
	var metadata map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["issuer"] != "https://relay.example.test" || !strings.Contains(stringValue(metadata["token_endpoint"]), "/oauth/token") {
		t.Fatalf("unexpected OAuth metadata: %+v", metadata)
	}

	verifier := "activation-test-verifier"
	challenge := pkceChallenge(verifier)
	authURL := relayHTTP.URL + "/oauth/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {"codencer-chatgpt-dev"},
		"redirect_uri":          {"https://chat.openai.com/aip/callback"},
		"scope":                 {"projects:read reports:read"},
		"state":                 {"state-1"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"resource":              {"https://relay.example.test/mcp"},
		"operator_code":         {"approve-me"},
	}.Encode()
	noRedirect := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	authResp, err := noRedirect.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	defer authResp.Body.Close()
	if authResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(authResp.Body)
		t.Fatalf("expected authorize redirect, got %d body=%s", authResp.StatusCode, body)
	}
	location := authResp.Header.Get("Location")
	parsedLocation, _ := url.Parse(location)
	code := parsedLocation.Query().Get("code")
	if code == "" || parsedLocation.Query().Get("state") != "state-1" {
		t.Fatalf("authorization redirect missing code/state: %s", location)
	}

	tokenResp, err := http.PostForm(relayHTTP.URL+"/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"codencer-chatgpt-dev"},
		"client_secret": {"client-secret"},
		"redirect_uri":  {"https://chat.openai.com/aip/callback"},
		"code":          {code},
		"code_verifier": {verifier},
		"resource":      {"https://relay.example.test/mcp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(tokenResp.Body)
		t.Fatalf("expected token response, got %d body=%s", tokenResp.StatusCode, body)
	}
	var tokenPayload map[string]any
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenPayload); err != nil {
		t.Fatal(err)
	}
	accessToken := stringValue(tokenPayload["access_token"])
	if accessToken == "" || strings.Contains(accessToken, "client-secret") || strings.Contains(accessToken, "approve-me") {
		t.Fatalf("bad token payload: %+v", tokenPayload)
	}

	req, _ := http.NewRequest(http.MethodPost, relayHTTP.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":"init","method":"initialize"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	mcpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer mcpResp.Body.Close()
	if mcpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(mcpResp.Body)
		t.Fatalf("OAuth access token was not accepted by MCP, got %d body=%s", mcpResp.StatusCode, body)
	}
}

func TestOAuthDevRejectsBadPKCE(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := relay.NewServer(&relay.Config{
		DBPath:        filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken:  "planner-token",
		PublicBaseURL: "https://relay.example.test",
		ChatGPTOAuthDev: relay.OAuthDevConfig{
			Enabled:          true,
			Issuer:           "https://relay.example.test",
			ClientID:         "codencer-chatgpt-dev",
			ClientSecretHash: hashSecret("client-secret"),
			OperatorCodeHash: hashSecret("approve-me"),
			Scopes:           []string{"projects:read"},
		},
	}, store)
	relayHTTP := httptest.NewServer(server.Handler())
	defer relayHTTP.Close()

	authURL := relayHTTP.URL + "/oauth/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {"codencer-chatgpt-dev"},
		"redirect_uri":          {"https://chat.openai.com/aip/callback"},
		"code_challenge":        {pkceChallenge("correct")},
		"code_challenge_method": {"S256"},
		"operator_code":         {"approve-me"},
	}.Encode()
	noRedirect := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	authResp, err := noRedirect.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	authResp.Body.Close()
	location := authResp.Header.Get("Location")
	parsedLocation, _ := url.Parse(location)

	tokenResp, err := http.PostForm(relayHTTP.URL+"/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"codencer-chatgpt-dev"},
		"client_secret": {"client-secret"},
		"redirect_uri":  {"https://chat.openai.com/aip/callback"},
		"code":          {parsedLocation.Query().Get("code")},
		"code_verifier": {"wrong"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(tokenResp.Body)
		t.Fatalf("expected bad PKCE to fail, got %d body=%s", tokenResp.StatusCode, body)
	}
}

func TestDevNoAuthRestrictsProjectsAndWritesByDefault(t *testing.T) {
	t.Parallel()

	store, err := relay.OpenStore(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveConnector(context.Background(), "connector-1", "machine-1", "pub", "label"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReplaceConnectorProjects(context.Background(), "connector-1", []relay.ProjectRecord{
		{ProjectID: "fake", ConnectorID: "connector-1", InstanceID: "inst-1", RepoRoot: "/repo/fake", ProjectJSON: `{"id":"fake","repo_root":"/repo/fake"}`},
		{ProjectID: "real", ConnectorID: "connector-1", InstanceID: "inst-1", RepoRoot: "/repo/real", ProjectJSON: `{"id":"real","repo_root":"/repo/real"}`},
	}); err != nil {
		t.Fatal(err)
	}
	server := relay.NewServer(&relay.Config{
		DBPath:       filepath.Join(t.TempDir(), "unused.db"),
		PlannerToken: "planner-token",
		ChatGPTDevNoAuth: relay.DevNoAuthConfig{
			Enabled:    true,
			Scopes:     []string{"projects:read", "reports:read"},
			ProjectIDs: []string{"fake"},
		},
	}, store)
	relayHTTP := httptest.NewServer(server.Handler())
	defer relayHTTP.Close()

	resp, err := http.Get(relayHTTP.URL + "/api/v2/projects")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"fake"`) || strings.Contains(string(body), `"real"`) {
		t.Fatalf("dev-noauth project listing was not restricted, status=%d body=%s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "/repo/") {
		t.Fatalf("project payload leaked repo path: %s", body)
	}

	postResp, err := http.Post(relayHTTP.URL+"/api/v2/projects/fake/runs", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(postResp.Body)
		t.Fatalf("expected dev-noauth write denial, got %d body=%s", postResp.StatusCode, body)
	}
}

func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
