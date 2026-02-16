package server

import (
	"context"
	"log/slog"

	"github.com/gxravel/youtube-music-mcp/internal/youtube"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with YouTube API client
type Server struct {
	mcpServer *mcp.Server
	logger    *slog.Logger
	ytClient  *youtube.Client
}

// NewServer creates a new MCP server instance
func NewServer(logger *slog.Logger, ytClient *youtube.Client) *Server {
	// Create MCP server with implementation info
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "youtube-music-mcp",
		Version: "0.1.0",
	}, nil)

	s := &Server{
		mcpServer: mcpServer,
		logger:    logger,
		ytClient:  ytClient,
	}

	// Register MCP tools
	s.registerPlaylistTools()
	s.registerSubscriptionTools()

	return s
}

// Run starts the MCP server with stdio transport
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("starting MCP server")

	// Run the server with stdio transport (blocks until shutdown)
	if err := s.mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return err
	}

	return nil
}
