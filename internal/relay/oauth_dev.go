package relay

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type oauthDevService struct {
	cfg    OAuthDevConfig
	now    func() time.Time
	mu     sync.Mutex
	codes  map[string]oauthAuthCode
	tokens map[string]oauthAccessToken
}

type oauthAuthCode struct {
	ClientID      string
	RedirectURI   string
	Scope         string
	Resource      string
	State         string
	CodeChallenge string
	ExpiresAt     time.Time
}

type oauthAccessToken struct {
	Name       string
	TokenHash  string
	Scopes     []string
	ProjectIDs []string
	Resource   string
	ExpiresAt  time.Time
}

func newOAuthDevService(cfg *Config) *oauthDevService {
	if cfg == nil || !cfg.ChatGPTOAuthDev.Enabled {
		return nil
	}
	return &oauthDevService{
		cfg:    cfg.ChatGPTOAuthDev,
		now:    func() time.Time { return time.Now().UTC() },
		codes:  make(map[string]oauthAuthCode),
		tokens: make(map[string]oauthAccessToken),
	}
}

func (s *Server) handleOAuthAuthorizationServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if s.oauthDev == nil {
		writeAPIError(w, http.StatusNotFound, "oauth_dev_not_configured", "ChatGPT OAuth dev mode is not enabled")
		return
	}
	writeJSON(w, http.StatusOK, s.oauthDev.Metadata(s.relayBaseURL(r)))
}

func (s *Server) handleOpenIDConfiguration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if s.oauthDev == nil {
		writeAPIError(w, http.StatusNotFound, "oauth_dev_not_configured", "ChatGPT OAuth dev mode is not enabled")
		return
	}
	metadata := s.oauthDev.Metadata(s.relayBaseURL(r))
	metadata["claims_supported"] = []string{"sub", "aud"}
	metadata["id_token_signing_alg_values_supported"] = []string{}
	writeJSON(w, http.StatusOK, metadata)
}

func (s *Server) handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if s.oauthDev == nil {
		writeAPIError(w, http.StatusNotFound, "oauth_dev_not_configured", "ChatGPT OAuth dev mode is not enabled")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
			return
		}
	}
	values := r.URL.Query()
	if r.Method == http.MethodPost {
		values = r.Form
	}
	operatorCode := strings.TrimSpace(values.Get("operator_code"))
	if operatorCode == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(s.oauthDev.AuthorizeForm(values)))
		return
	}
	code, redirectURI, state, apiErr := s.oauthDev.CreateAuthorizationCode(values, s.relayBaseURL(r))
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	location, err := appendRedirectParams(redirectURI, map[string]string{"code": code, "state": state})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
		return
	}
	http.Redirect(w, r, location, http.StatusFound)
}

func (s *Server) handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	if s.oauthDev == nil {
		writeAPIError(w, http.StatusNotFound, "oauth_dev_not_configured", "ChatGPT OAuth dev mode is not enabled")
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, http.StatusBadRequest, "malformed_request", err.Error())
		return
	}
	response, apiErr := s.oauthDev.ExchangeCode(r.Form, r.Header.Get("Authorization"), s.relayBaseURL(r))
	if apiErr != nil {
		writeAPIError(w, apiErr.Status, apiErr.Code, apiErr.Message)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (o *oauthDevService) Metadata(baseURL string) map[string]any {
	issuer := o.issuer(baseURL)
	return map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/oauth/authorize",
		"token_endpoint":                        issuer + "/oauth/token",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
		"scopes_supported":                      o.cfg.Scopes,
		"resource_documentation":                issuer + "/mcp",
		"refresh_token_supported":               false,
	}
}

func (o *oauthDevService) AuthorizeForm(values url.Values) string {
	hidden := ""
	for _, key := range []string{"response_type", "client_id", "redirect_uri", "scope", "state", "code_challenge", "code_challenge_method", "resource"} {
		hidden += fmt.Sprintf(`<input type="hidden" name="%s" value="%s">`, html.EscapeString(key), html.EscapeString(values.Get(key)))
	}
	return `<!doctype html><html><head><meta charset="utf-8"><title>Codencer ChatGPT OAuth Dev Approval</title></head><body>` +
		`<h1>Codencer ChatGPT OAuth Dev Approval</h1>` +
		`<p>Enter the operator approval code generated by <code>codencer setup relay --enable-chatgpt-oauth-dev</code>.</p>` +
		`<form method="post">` + hidden +
		`<label>Operator approval code <input name="operator_code" type="password" autocomplete="off"></label>` +
		`<button type="submit">Approve</button></form></body></html>`
}

func (o *oauthDevService) CreateAuthorizationCode(values url.Values, baseURL string) (string, string, string, *apiError) {
	if values.Get("response_type") != "code" {
		return "", "", "", &apiError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "response_type=code is required"}
	}
	if values.Get("client_id") != o.cfg.ClientID {
		return "", "", "", &apiError{Status: http.StatusUnauthorized, Code: "invalid_client", Message: "unknown OAuth client"}
	}
	redirectURI := strings.TrimSpace(values.Get("redirect_uri"))
	if redirectURI == "" {
		return "", "", "", &apiError{Status: http.StatusBadRequest, Code: "invalid_redirect_uri", Message: "redirect_uri is required"}
	}
	if _, err := url.ParseRequestURI(redirectURI); err != nil {
		return "", "", "", &apiError{Status: http.StatusBadRequest, Code: "invalid_redirect_uri", Message: err.Error()}
	}
	if values.Get("code_challenge_method") != "S256" || strings.TrimSpace(values.Get("code_challenge")) == "" {
		return "", "", "", &apiError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "PKCE S256 code_challenge is required"}
	}
	if !constantTimeHashEqual(values.Get("operator_code"), o.cfg.OperatorCodeHash) {
		return "", "", "", &apiError{Status: http.StatusUnauthorized, Code: "approval_denied", Message: "operator approval code is invalid"}
	}
	resource := strings.TrimRight(strings.TrimSpace(values.Get("resource")), "/")
	if resource == "" {
		resource = o.issuer(baseURL) + "/mcp"
	}
	if !o.resourceAllowed(resource, baseURL) {
		return "", "", "", &apiError{Status: http.StatusBadRequest, Code: "invalid_resource", Message: "OAuth token resource must target the relay MCP endpoint"}
	}
	scope := normalizeRequestedScopes(values.Get("scope"), o.cfg.Scopes)
	code, err := randomOpaqueToken(32)
	if err != nil {
		return "", "", "", &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pruneLocked(o.now())
	o.codes[code] = oauthAuthCode{
		ClientID:      o.cfg.ClientID,
		RedirectURI:   redirectURI,
		Scope:         scope,
		Resource:      resource,
		State:         strings.TrimSpace(values.Get("state")),
		CodeChallenge: strings.TrimSpace(values.Get("code_challenge")),
		ExpiresAt:     o.now().Add(time.Duration(o.cfg.AuthorizationCodeTTL) * time.Second),
	}
	return code, redirectURI, strings.TrimSpace(values.Get("state")), nil
}

func (o *oauthDevService) ExchangeCode(values url.Values, authorizationHeader, baseURL string) (map[string]any, *apiError) {
	if values.Get("grant_type") != "authorization_code" {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "unsupported_grant_type", Message: "authorization_code grant is required"}
	}
	clientID := strings.TrimSpace(values.Get("client_id"))
	clientSecret := strings.TrimSpace(values.Get("client_secret"))
	if basicID, basicSecret, ok := parseBasicAuth(authorizationHeader); ok {
		clientID = firstNonEmpty(clientID, basicID)
		clientSecret = firstNonEmpty(clientSecret, basicSecret)
	}
	if clientID != o.cfg.ClientID || !constantTimeHashEqual(clientSecret, o.cfg.ClientSecretHash) {
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "invalid_client", Message: "OAuth client authentication failed"}
	}
	code := strings.TrimSpace(values.Get("code"))
	codeVerifier := strings.TrimSpace(values.Get("code_verifier"))
	if code == "" || codeVerifier == "" {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "code and code_verifier are required"}
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.now()
	o.pruneLocked(now)
	authCode, ok := o.codes[code]
	if !ok {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_grant", Message: "authorization code is invalid or expired"}
	}
	delete(o.codes, code)
	if authCode.ExpiresAt.Before(now) {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_grant", Message: "authorization code is expired"}
	}
	if authCode.RedirectURI != strings.TrimSpace(values.Get("redirect_uri")) {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_grant", Message: "redirect_uri does not match authorization request"}
	}
	if !pkceS256Matches(codeVerifier, authCode.CodeChallenge) {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_grant", Message: "PKCE verifier does not match"}
	}
	if requestedResource := strings.TrimRight(strings.TrimSpace(values.Get("resource")), "/"); requestedResource != "" && requestedResource != authCode.Resource {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_resource", Message: "resource does not match authorization request"}
	}
	if !o.resourceAllowed(authCode.Resource, baseURL) {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_resource", Message: "OAuth token resource must target the relay MCP endpoint"}
	}
	accessToken, err := randomOpaqueToken(32)
	if err != nil {
		return nil, &apiError{Status: http.StatusInternalServerError, Code: "relay_internal_error", Message: err.Error()}
	}
	expiresIn := o.cfg.TokenTTLSeconds
	tokenRecord := oauthAccessToken{
		Name:       "chatgpt-oauth-dev",
		TokenHash:  plannerTokenHash(accessToken),
		Scopes:     strings.Fields(authCode.Scope),
		ProjectIDs: append([]string(nil), o.cfg.ProjectIDs...),
		Resource:   authCode.Resource,
		ExpiresAt:  now.Add(time.Duration(expiresIn) * time.Second),
	}
	o.tokens[tokenRecord.TokenHash] = tokenRecord
	return map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   expiresIn,
		"scope":        authCode.Scope,
		"resource":     authCode.Resource,
	}, nil
}

func (o *oauthDevService) Authenticate(token string) (*plannerPrincipal, *apiError) {
	if o == nil {
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "OAuth dev mode is not enabled"}
	}
	hash := plannerTokenHash(token)
	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.now()
	o.pruneLocked(now)
	record, ok := o.tokens[hash]
	if !ok {
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "OAuth access token is invalid"}
	}
	if record.ExpiresAt.Before(now) {
		delete(o.tokens, hash)
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "OAuth access token is expired"}
	}
	principal := &plannerPrincipal{
		Name:        record.Name,
		TokenHash:   record.TokenHash,
		Scopes:      append([]string(nil), record.Scopes...),
		InstanceIDs: make(map[string]struct{}),
		ProjectIDs:  make(map[string]struct{}),
	}
	for _, projectID := range record.ProjectIDs {
		principal.ProjectIDs[projectID] = struct{}{}
	}
	return principal, nil
}

func (o *oauthDevService) issuer(baseURL string) string {
	issuer := strings.TrimRight(strings.TrimSpace(o.cfg.Issuer), "/")
	if issuer == "" {
		issuer = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
	return issuer
}

func (o *oauthDevService) resourceAllowed(resource, baseURL string) bool {
	return strings.TrimRight(resource, "/") == o.issuer(baseURL)+"/mcp"
}

func (o *oauthDevService) pruneLocked(now time.Time) {
	for code, record := range o.codes {
		if record.ExpiresAt.Before(now) {
			delete(o.codes, code)
		}
	}
	for hash, record := range o.tokens {
		if record.ExpiresAt.Before(now) {
			delete(o.tokens, hash)
		}
	}
}

func normalizeRequestedScopes(requested string, allowed []string) string {
	requestedFields := strings.Fields(requested)
	if len(requestedFields) == 0 {
		return strings.Join(allowed, " ")
	}
	allowedSet := map[string]struct{}{}
	for _, scope := range allowed {
		allowedSet[scope] = struct{}{}
	}
	out := make([]string, 0, len(requestedFields))
	for _, scope := range requestedFields {
		if _, ok := allowedSet[scope]; ok {
			out = append(out, scope)
		}
	}
	if len(out) == 0 {
		return strings.Join(allowed, " ")
	}
	return strings.Join(out, " ")
}

func constantTimeHashEqual(secret, expectedHex string) bool {
	sum := sha256.Sum256([]byte(secret))
	got := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(strings.TrimSpace(expectedHex))) == 1
}

func pkceS256Matches(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(strings.TrimSpace(challenge))) == 1
}

func randomOpaqueToken(bytesLen int) (string, error) {
	data := make([]byte, bytesLen)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func appendRedirectParams(redirectURI string, params map[string]string) (string, error) {
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func parseBasicAuth(header string) (string, string, bool) {
	header = strings.TrimSpace(header)
	if len(header) < len("Basic ") || !strings.EqualFold(header[:len("Basic ")], "Basic ") {
		return "", "", false
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[len("Basic "):]))
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(data), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
