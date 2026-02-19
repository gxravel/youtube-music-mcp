package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

// authServerMetadata is a local copy of RFC 8414 metadata fields.
// The SDK's oauthex.AuthServerMeta is behind a client-only build tag.
type authServerMetadata struct {
	Issuer                           string   `json:"issuer"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint"`
	TokenEndpoint                    string   `json:"token_endpoint"`
	JWKSURI                          string   `json:"jwks_uri"`
	RegistrationEndpoint             string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                  []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	GrantTypesSupported              []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	CodeChallengeMethodsSupported    []string `json:"code_challenge_methods_supported,omitempty"`
}

// dcrClient is a dynamically registered client.
type dcrClient struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURIs []string `json:"redirect_uris"`
}

// pendingAuth tracks an in-flight authorization request.
type pendingAuth struct {
	clientID      string
	redirectURI   string
	clientState   string
	codeChallenge string
	createdAt     time.Time
}

// authCode is a single-use MCP authorization code.
type authCode struct {
	clientID      string
	redirectURI   string
	codeChallenge string
	createdAt     time.Time
}

// accessToken tracks an issued access token.
type accessToken struct {
	clientID  string
	expiresAt time.Time
}

// refreshToken tracks an issued refresh token.
type refreshToken struct {
	clientID string
}

// MCPOAuthServer implements a full OAuth 2.0 Authorization Server
// for the MCP specification (RFC 9728 + RFC 8414 + DCR).
// It proxies authorization to Google and issues its own opaque tokens.
type MCPOAuthServer struct {
	baseURL  string
	googleCfg *oauth2.Config
	logger   *slog.Logger

	mu            sync.Mutex
	clients       map[string]*dcrClient    // client_id -> client
	pendingAuths  map[string]*pendingAuth  // google_state -> pending
	authCodes     map[string]*authCode     // code -> auth code record
	accessTokens  map[string]*accessToken  // token -> access token record
	refreshTokens map[string]*refreshToken // token -> refresh token record
	googleToken   *oauth2.Token            // single-tenant Google token
}

// NewMCPOAuthServer creates a new MCP OAuth Authorization Server.
func NewMCPOAuthServer(baseURL string, googleCfg *oauth2.Config, logger *slog.Logger) *MCPOAuthServer {
	return &MCPOAuthServer{
		baseURL:       baseURL,
		googleCfg:     googleCfg,
		logger:        logger,
		clients:       make(map[string]*dcrClient),
		pendingAuths:  make(map[string]*pendingAuth),
		authCodes:     make(map[string]*authCode),
		accessTokens:  make(map[string]*accessToken),
		refreshTokens: make(map[string]*refreshToken),
	}
}

// ResourceMetadataURL returns the URL for the protected resource metadata endpoint.
func (s *MCPOAuthServer) ResourceMetadataURL() string {
	return s.baseURL + "/.well-known/oauth-protected-resource"
}

// BaseURL returns the server's public base URL.
func (s *MCPOAuthServer) BaseURL() string {
	return s.baseURL
}

// ProtectedResourceMetadataHandler returns a handler for /.well-known/oauth-protected-resource.
func (s *MCPOAuthServer) ProtectedResourceMetadataHandler() http.Handler {
	return mcpauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:             s.baseURL,
		AuthorizationServers: []string{s.baseURL},
		BearerMethodsSupported: []string{"header"},
	})
}

// AuthServerMetadataHandler returns a handler for /.well-known/oauth-authorization-server.
func (s *MCPOAuthServer) AuthServerMetadataHandler() http.HandlerFunc {
	meta := &authServerMetadata{
		Issuer:                            s.baseURL,
		AuthorizationEndpoint:             s.baseURL + "/authorize",
		TokenEndpoint:                     s.baseURL + "/token",
		JWKSURI:                           s.baseURL + "/jwks",
		RegistrationEndpoint:              s.baseURL + "/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post"},
		CodeChallengeMethodsSupported:     []string{"S256"},
	}

	data, _ := json.Marshal(meta)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

// JWKSHandler returns a handler for /jwks (empty key set, we use opaque tokens).
func (s *MCPOAuthServer) JWKSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[]}`))
	}
}

// RegisterHandler returns a handler for POST /register (Dynamic Client Registration).
func (s *MCPOAuthServer) RegisterHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			RedirectURIs []string `json:"redirect_uris"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if len(req.RedirectURIs) == 0 {
			http.Error(w, "redirect_uris required", http.StatusBadRequest)
			return
		}

		clientID := generateToken(16)
		clientSecret := generateToken(32)

		client := &dcrClient{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURIs: req.RedirectURIs,
		}

		s.mu.Lock()
		s.clients[clientID] = client
		s.mu.Unlock()

		s.logger.Info("registered new client", "client_id", clientID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(client)
	}
}

// AuthorizeHandler returns a handler for GET /authorize.
// Validates client_id, redirect_uri, PKCE; stores pending auth; redirects to Google.
func (s *MCPOAuthServer) AuthorizeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		q := r.URL.Query()
		clientID := q.Get("client_id")
		redirectURI := q.Get("redirect_uri")
		codeChallenge := q.Get("code_challenge")
		codeChallengeMethod := q.Get("code_challenge_method")
		clientState := q.Get("state")

		// Validate client
		s.mu.Lock()
		client, ok := s.clients[clientID]
		s.mu.Unlock()

		if !ok {
			http.Error(w, "Unknown client_id", http.StatusBadRequest)
			return
		}

		// Validate redirect_uri
		if !slices.Contains(client.RedirectURIs, redirectURI) {
			http.Error(w, "Invalid redirect_uri", http.StatusBadRequest)
			return
		}

		// Validate PKCE
		if codeChallenge == "" || codeChallengeMethod != "S256" {
			http.Error(w, "PKCE S256 required", http.StatusBadRequest)
			return
		}

		// Generate state for Google OAuth
		googleState := generateToken(16)

		s.mu.Lock()
		s.pendingAuths[googleState] = &pendingAuth{
			clientID:      clientID,
			redirectURI:   redirectURI,
			clientState:   clientState,
			codeChallenge: codeChallenge,
			createdAt:     time.Now(),
		}
		s.mu.Unlock()

		// Redirect to Google consent
		authURL := s.googleCfg.AuthCodeURL(googleState,
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("prompt", "consent"),
		)
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

// GoogleCallbackHandler returns a handler for GET /google-callback.
// Exchanges Google code for token, generates MCP auth code, redirects to client.
func (s *MCPOAuthServer) GoogleCallbackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		googleCode := r.URL.Query().Get("code")
		googleState := r.URL.Query().Get("state")

		if googleCode == "" || googleState == "" {
			http.Error(w, "Missing code or state", http.StatusBadRequest)
			return
		}

		// Look up pending auth
		s.mu.Lock()
		pending, ok := s.pendingAuths[googleState]
		if ok {
			delete(s.pendingAuths, googleState)
		}
		s.mu.Unlock()

		if !ok {
			http.Error(w, "Unknown or expired state", http.StatusBadRequest)
			return
		}

		// Exchange Google code for token
		token, err := s.googleCfg.Exchange(r.Context(), googleCode)
		if err != nil {
			s.logger.Error("Google token exchange failed", "error", err)
			http.Error(w, "Google authentication failed", http.StatusInternalServerError)
			return
		}

		// Store Google token
		s.mu.Lock()
		s.googleToken = token
		s.mu.Unlock()

		s.logger.Info("Google token obtained successfully")

		// Generate MCP auth code
		mcpCode := generateToken(32)
		s.mu.Lock()
		s.authCodes[mcpCode] = &authCode{
			clientID:      pending.clientID,
			redirectURI:   pending.redirectURI,
			codeChallenge: pending.codeChallenge,
			createdAt:     time.Now(),
		}
		s.mu.Unlock()

		// Redirect back to client with MCP code
		redirectURL, err := url.Parse(pending.redirectURI)
		if err != nil {
			http.Error(w, "Invalid redirect_uri", http.StatusInternalServerError)
			return
		}
		q := redirectURL.Query()
		q.Set("code", mcpCode)
		if pending.clientState != "" {
			q.Set("state", pending.clientState)
		}
		redirectURL.RawQuery = q.Encode()

		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	}
}

// TokenHandler returns a handler for POST /token.
// Supports authorization_code and refresh_token grant types.
func (s *MCPOAuthServer) TokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		grantType := r.FormValue("grant_type")
		clientID := r.FormValue("client_id")
		clientSecret := r.FormValue("client_secret")

		// Validate client credentials
		s.mu.Lock()
		client, ok := s.clients[clientID]
		s.mu.Unlock()

		if !ok || client.ClientSecret != clientSecret {
			jsonError(w, "invalid_client", "Invalid client credentials", http.StatusUnauthorized)
			return
		}

		switch grantType {
		case "authorization_code":
			s.handleAuthorizationCodeGrant(w, r, clientID)
		case "refresh_token":
			s.handleRefreshTokenGrant(w, r, clientID)
		default:
			jsonError(w, "unsupported_grant_type", "Unsupported grant_type", http.StatusBadRequest)
		}
	}
}

func (s *MCPOAuthServer) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request, clientID string) {
	code := r.FormValue("code")
	codeVerifier := r.FormValue("code_verifier")

	// Look up and consume auth code (single-use)
	s.mu.Lock()
	ac, ok := s.authCodes[code]
	if ok {
		delete(s.authCodes, code)
	}
	s.mu.Unlock()

	if !ok {
		jsonError(w, "invalid_grant", "Invalid or expired authorization code", http.StatusBadRequest)
		return
	}

	// Check TTL (10 minutes)
	if time.Since(ac.createdAt) > 10*time.Minute {
		jsonError(w, "invalid_grant", "Authorization code expired", http.StatusBadRequest)
		return
	}

	// Verify client
	if ac.clientID != clientID {
		jsonError(w, "invalid_grant", "Client mismatch", http.StatusBadRequest)
		return
	}

	// Verify PKCE
	if !verifyPKCE(codeVerifier, ac.codeChallenge) {
		jsonError(w, "invalid_grant", "PKCE verification failed", http.StatusBadRequest)
		return
	}

	s.issueTokens(w, clientID)
}

func (s *MCPOAuthServer) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request, clientID string) {
	rt := r.FormValue("refresh_token")

	s.mu.Lock()
	rtRecord, ok := s.refreshTokens[rt]
	if ok {
		delete(s.refreshTokens, rt) // Rotate: old refresh token consumed
	}
	s.mu.Unlock()

	if !ok {
		jsonError(w, "invalid_grant", "Invalid refresh token", http.StatusBadRequest)
		return
	}

	if rtRecord.clientID != clientID {
		jsonError(w, "invalid_grant", "Client mismatch", http.StatusBadRequest)
		return
	}

	s.issueTokens(w, clientID)
}

func (s *MCPOAuthServer) issueTokens(w http.ResponseWriter, clientID string) {
	accessTok := generateToken(32)
	refreshTok := generateToken(32)
	expiresIn := 3600 // 1 hour

	s.mu.Lock()
	s.accessTokens[accessTok] = &accessToken{
		clientID:  clientID,
		expiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	s.refreshTokens[refreshTok] = &refreshToken{
		clientID: clientID,
	}
	s.mu.Unlock()

	resp := map[string]any{
		"access_token":  accessTok,
		"token_type":    "Bearer",
		"expires_in":    expiresIn,
		"refresh_token": refreshTok,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(resp)
}

// TokenVerifier returns a function compatible with auth.RequireBearerToken middleware.
func (s *MCPOAuthServer) TokenVerifier() mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
		s.mu.Lock()
		at, ok := s.accessTokens[token]
		s.mu.Unlock()

		if !ok {
			return nil, fmt.Errorf("unknown token: %w", mcpauth.ErrInvalidToken)
		}

		if time.Now().After(at.expiresAt) {
			s.mu.Lock()
			delete(s.accessTokens, token)
			s.mu.Unlock()
			return nil, fmt.Errorf("token expired: %w", mcpauth.ErrInvalidToken)
		}

		return &mcpauth.TokenInfo{
			Expiration: at.expiresAt,
		}, nil
	}
}

// GetGoogleHTTPClient returns an HTTP client authenticated with the stored Google token.
func (s *MCPOAuthServer) GetGoogleHTTPClient(ctx context.Context) (*http.Client, error) {
	s.mu.Lock()
	token := s.googleToken
	s.mu.Unlock()

	if token == nil {
		return nil, fmt.Errorf("no Google token available")
	}

	return s.googleCfg.Client(ctx, token), nil
}

// HasGoogleToken reports whether a Google token has been stored.
func (s *MCPOAuthServer) HasGoogleToken() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.googleToken != nil
}

// StartCleanup runs a background goroutine that prunes expired state every 5 minutes.
func (s *MCPOAuthServer) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanup()
			}
		}
	}()
}

func (s *MCPOAuthServer) cleanup() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range s.pendingAuths {
		if now.Sub(v.createdAt) > 10*time.Minute {
			delete(s.pendingAuths, k)
		}
	}
	for k, v := range s.authCodes {
		if now.Sub(v.createdAt) > 10*time.Minute {
			delete(s.authCodes, k)
		}
	}
	for k, v := range s.accessTokens {
		if now.After(v.expiresAt) {
			delete(s.accessTokens, k)
		}
	}
}

// verifyPKCE checks that SHA256(verifier) base64url-encoded matches the challenge.
func verifyPKCE(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	h := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}

// generateToken produces a cryptographically random hex string of n bytes.
func generateToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// jsonError writes an OAuth 2.0 error response.
func jsonError(w http.ResponseWriter, errCode, description string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             errCode,
		"error_description": description,
	})
}
