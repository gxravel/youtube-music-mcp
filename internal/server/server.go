package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gxravel/youtube-music-mcp/internal/auth"
	"github.com/gxravel/youtube-music-mcp/internal/youtube"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with YouTube API client
type Server struct {
	mcpServer *mcp.Server
	logger    *slog.Logger
	transport string
	port      int

	// MCP OAuth server (SSE mode only)
	mcpOAuth *auth.MCPOAuthServer

	mu         sync.Mutex
	ytClient   *youtube.Client
	toolsReady bool // true once tools are registered
}

// NewServer creates a new MCP server instance.
//
// For stdio mode: pass a non-nil ytClient; mcpOAuth may be nil.
// For SSE mode: pass nil ytClient and a configured mcpOAuth; YouTube client is created lazily.
func NewServer(logger *slog.Logger, ytClient *youtube.Client, transport string, port int, mcpOAuth *auth.MCPOAuthServer) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "youtube-music-mcp",
		Version: "0.1.0",
	}, nil)

	s := &Server{
		mcpServer: mcpServer,
		logger:    logger,
		transport: transport,
		port:      port,
		mcpOAuth:  mcpOAuth,
	}

	if ytClient != nil {
		s.ytClient = ytClient
		s.registerAnalyzeTools()
		s.registerRecommendTools()
		s.toolsReady = true
	}

	return s
}

// ensureYTClient lazily creates the YouTube client from the MCP OAuth server's Google token.
func (s *Server) ensureYTClient(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.toolsReady {
		return nil
	}

	httpClient, err := s.mcpOAuth.GetGoogleHTTPClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Google HTTP client: %w", err)
	}

	ytClient, err := youtube.NewClient(ctx, httpClient)
	if err != nil {
		return fmt.Errorf("failed to create youtube client: %w", err)
	}

	channelName, err := ytClient.ValidateAuth(ctx)
	if err != nil {
		return fmt.Errorf("auth validation failed: %w", err)
	}
	s.logger.Info("authenticated with youtube", "channel", channelName)

	s.ytClient = ytClient
	s.registerAnalyzeTools()
	s.registerRecommendTools()
	s.toolsReady = true
	return nil
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
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// runSSE runs the MCP server as an HTTP server using Streamable HTTP transport
// (for Railway/hosted deployments).
// Implements MCP OAuth specification (RFC 9728 + RFC 8414 + DCR).
func (s *Server) runSSE(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	s.logger.Info("starting MCP server", "transport", "streamable-http", "addr", addr)

	streamHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		if err := s.ensureYTClient(req.Context()); err != nil {
			s.logger.Error("failed to initialize YouTube client", "error", err)
			return nil
		}
		return s.mcpServer
	}, &mcp.StreamableHTTPOptions{
		Logger: s.logger,
	})

	// Wrap streamable handler with bearer token middleware.
	resourceMetadataURL := s.mcpOAuth.BaseURL() + "/.well-known/oauth-protected-resource"
	bearerMiddleware := mcpauth.RequireBearerToken(s.mcpOAuth.TokenVerifier(), &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: resourceMetadataURL,
	})
	protectedMCP := bearerMiddleware(streamHandler)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// MCP OAuth discovery endpoints
	mux.Handle("GET /.well-known/oauth-protected-resource", s.mcpOAuth.ProtectedResourceMetadataHandler())
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.mcpOAuth.AuthServerMetadataHandler())
	mux.HandleFunc("GET /jwks", s.mcpOAuth.JWKSHandler())

	// MCP OAuth flow endpoints
	mux.HandleFunc("POST /register", s.mcpOAuth.RegisterHandler())
	mux.HandleFunc("GET /authorize", s.mcpOAuth.AuthorizeHandler())
	mux.HandleFunc("GET /callback", s.mcpOAuth.GoogleCallbackHandler())
	mux.HandleFunc("POST /token", s.mcpOAuth.TokenHandler())

	// Streamable HTTP MCP endpoint — gated behind bearer token auth
	mux.Handle("/mcp", protectedMCP)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

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
