package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gxravel/youtube-music-mcp/internal/youtube"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with YouTube API client
type Server struct {
	mcpServer *mcp.Server
	logger    *slog.Logger
	ytClient  *youtube.Client
	transport string
	port      int
}

// NewServer creates a new MCP server instance
func NewServer(logger *slog.Logger, ytClient *youtube.Client, transport string, port int) *Server {
	// Create MCP server with implementation info
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "youtube-music-mcp",
		Version: "0.1.0",
	}, nil)

	s := &Server{
		mcpServer: mcpServer,
		logger:    logger,
		ytClient:  ytClient,
		transport: transport,
		port:      port,
	}

	// Register MCP tools
	s.registerAnalyzeTools()
	s.registerRecommendTools()

	return s
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
func (s *Server) runSSE(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	s.logger.Info("starting MCP server", "transport", "sse", "addr", addr)

	// SSEHandler implements http.Handler and manages SSE-based MCP sessions
	sseHandler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: sseHandler,
	}

	// Run HTTP server in background; shut down on context cancellation
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
