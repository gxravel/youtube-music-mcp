package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gxravel/youtube-music-mcp/internal/auth"
	"github.com/gxravel/youtube-music-mcp/internal/youtube"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// Server wraps the MCP server with YouTube API client
type Server struct {
	mcpServer *mcp.Server
	logger    *slog.Logger
	transport string
	port      int

	// OAuth / deferred auth fields (SSE mode only)
	oauthCfg *oauth2.Config
	storage  auth.TokenStorage

	mu           sync.Mutex
	ytClient     *youtube.Client
	toolsReady   bool          // true once tools are registered
	ytClientCh   chan struct{}  // closed when ytClient is available
}

// NewServer creates a new MCP server instance.
//
// For stdio mode: pass a non-nil ytClient; oauthCfg and storage may be nil.
// For SSE mode without pre-existing token: pass nil ytClient; oauthCfg and storage required.
// For SSE mode with pre-existing token: pass non-nil ytClient; oauthCfg and storage required for re-auth.
func NewServer(logger *slog.Logger, ytClient *youtube.Client, transport string, port int, oauthCfg *oauth2.Config, storage auth.TokenStorage) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "youtube-music-mcp",
		Version: "0.1.0",
	}, nil)

	s := &Server{
		mcpServer:  mcpServer,
		logger:     logger,
		transport:  transport,
		port:       port,
		oauthCfg:   oauthCfg,
		storage:    storage,
		ytClientCh: make(chan struct{}),
	}

	if ytClient != nil {
		// Auth already done — register tools immediately and mark ready.
		s.ytClient = ytClient
		s.registerAnalyzeTools()
		s.registerRecommendTools()
		s.toolsReady = true
		close(s.ytClientCh)
	}

	return s
}

// enableYTClient stores the authenticated client, registers tools, and signals
// readiness. Safe to call once only.
func (s *Server) enableYTClient(ctx context.Context, httpClient *http.Client) error {
	ytClient, err := youtube.NewClient(ctx, httpClient)
	if err != nil {
		return fmt.Errorf("failed to create youtube client: %w", err)
	}

	channelName, err := ytClient.ValidateAuth(ctx)
	if err != nil {
		return fmt.Errorf("auth validation failed: %w", err)
	}
	s.logger.Info("authenticated with youtube via /callback", "channel", channelName)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.toolsReady {
		// Already authenticated — ignore duplicate callbacks
		return nil
	}

	s.ytClient = ytClient
	s.registerAnalyzeTools()
	s.registerRecommendTools()
	s.toolsReady = true
	close(s.ytClientCh)
	return nil
}

// isAuthenticated reports whether the server has a valid YouTube client.
func (s *Server) isAuthenticated() bool {
	select {
	case <-s.ytClientCh:
		return true
	default:
		return false
	}
}

// Run starts the MCP server with the configured transport.
// Use TRANSPORT=stdio (default) for local MCP clients or TRANSPORT=sse for Railway/HTTP deployments.
func (s *Server) Run(ctx context.Context) error {
	switch s.transport {
	case "sse":
		return s.runSSE(ctx)
	default:
		return s.runStdio(ctx)
	}
}

// runStdio runs the MCP server on the stdio transport (for local MCP clients).
func (s *Server) runStdio(ctx context.Context) error {
	s.logger.Info("starting MCP server", "transport", "stdio")

	if err := s.mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return err
	}

	return nil
}

// runSSE runs the MCP server as an HTTP server using SSE transport (for Railway/hosted deployments).
// Routes:
//
//	GET  /health   — Railway health check (always 200)
//	GET  /auth     — Initiate Google OAuth2 consent flow
//	GET  /callback — Receive OAuth code, exchange for token, enable MCP
//	*    /sse      — MCP SSE session (gated behind auth)
//	*    /message  — MCP message endpoint (gated behind auth)
func (s *Server) runSSE(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	s.logger.Info("starting MCP server", "transport", "sse", "addr", addr)

	// SSEHandler is created once at startup; wraps the same mcp.Server instance.
	sseHandler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	mux := http.NewServeMux()

	// Health check — always responds 200, used by Railway to determine liveness.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// /auth — redirect user to Google OAuth consent page.
	mux.HandleFunc("GET /auth", func(w http.ResponseWriter, r *http.Request) {
		if s.isAuthenticated() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Already authenticated. The MCP server is ready.")
			return
		}
		if s.oauthCfg == nil {
			http.Error(w, "OAuth not configured", http.StatusInternalServerError)
			return
		}
		authURL := s.oauthCfg.AuthCodeURL("state",
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("prompt", "consent"),
		)
		http.Redirect(w, r, authURL, http.StatusFound)
	})

	// /callback — Google redirects here after consent; exchange code for token.
	mux.HandleFunc("GET /callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Authorization failed: no code in callback", http.StatusBadRequest)
			return
		}

		if s.oauthCfg == nil || s.storage == nil {
			http.Error(w, "OAuth not configured", http.StatusInternalServerError)
			return
		}

		httpClient, err := auth.ExchangeAndSave(ctx, s.oauthCfg, code, s.storage, s.logger)
		if err != nil {
			s.logger.Error("OAuth exchange failed", "error", err)
			http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusInternalServerError)
			return
		}

		if err := s.enableYTClient(ctx, httpClient); err != nil {
			s.logger.Error("Failed to enable YouTube client", "error", err)
			http.Error(w, fmt.Sprintf("YouTube client setup failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
<h2>Authentication successful!</h2>
<p>You can close this window. The MCP server is now ready.</p>
</body></html>`)
	})

	// /sse and /message — gated behind authentication.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthenticated() {
			http.Error(w, "Not authenticated. Visit /auth to authenticate.", http.StatusServiceUnavailable)
			return
		}
		sseHandler.ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Run HTTP server in background; shut down on context cancellation.
	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("SSE HTTP server failed: %w", err)
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("shutting down SSE server")
		if err := httpServer.Shutdown(context.Background()); err != nil {
			s.logger.Error("failed to shut down SSE server", "error", err)
		}
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
