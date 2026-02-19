package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gxravel/youtube-music-mcp/internal/auth"
	"github.com/gxravel/youtube-music-mcp/internal/config"
	"github.com/gxravel/youtube-music-mcp/internal/server"
	"github.com/gxravel/youtube-music-mcp/internal/youtube"
)

func main() {
	// CRITICAL: Redirect standard log output to stderr first (before any logging)
	log.SetOutput(os.Stderr)

	// Create structured logger (JSON format to stderr)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Create context with signal handling for clean shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	switch cfg.Transport {
	case "sse":
		runSSEMode(ctx, cfg, logger)
	default:
		runStdioMode(ctx, cfg, logger)
	}
}

// runStdioMode is the original flow: authenticate first (blocking), then serve MCP on stdio.
func runStdioMode(ctx context.Context, cfg *config.Config, logger *slog.Logger) {
	oauthCfg := auth.NewOAuth2Config(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.OAuthRedirectURL)

	// Select token storage: env-based (Railway) or file-based (local)
	var storage auth.TokenStorage
	if cfg.TokenJSON != "" {
		logger.Info("using environment-based token storage (OAUTH_TOKEN_JSON)")
		storage = auth.NewEnvTokenStorage(cfg.TokenJSON, logger)
	} else {
		logger.Info("using file-based token storage", "path", auth.DefaultTokenPath())
		storage = auth.NewFileTokenStorage(auth.DefaultTokenPath())
	}

	// Authenticate (either load existing token or run local OAuth callback flow)
	httpClient, err := auth.Authenticate(ctx, oauthCfg, storage, cfg.OAuthPort, logger)
	if err != nil {
		logger.Error("authentication failed", "error", err)
		os.Exit(1)
	}

	// Create YouTube API client
	ytClient, err := youtube.NewClient(ctx, httpClient)
	if err != nil {
		logger.Error("failed to create youtube client", "error", err)
		os.Exit(1)
	}

	// Validate authentication by fetching channel info
	channelName, err := ytClient.ValidateAuth(ctx)
	if err != nil {
		logger.Error("auth validation failed", "error", err)
		os.Exit(1)
	}
	logger.Info("authenticated with youtube", "channel", channelName)

	// Create and run MCP server (stdio transport)
	srv := server.NewServer(logger, ytClient, cfg.Transport, cfg.Port, nil)
	if err := srv.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// runSSEMode starts the HTTP server with MCP OAuth specification support.
// The server acts as its own OAuth Authorization Server, proxying auth to Google.
// YouTube client is created lazily after the first successful OAuth flow.
func runSSEMode(ctx context.Context, cfg *config.Config, logger *slog.Logger) {
	if cfg.BaseURL == "" {
		fmt.Fprintln(os.Stderr, "BASE_URL is required for SSE mode")
		os.Exit(1)
	}

	// Google OAuth config with redirect to our /google-callback endpoint
	googleCfg := auth.NewOAuth2Config(
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.BaseURL+"/google-callback",
	)

	// Create MCP OAuth Authorization Server
	mcpOAuth := auth.NewMCPOAuthServer(cfg.BaseURL, googleCfg, logger)
	mcpOAuth.StartCleanup(ctx)

	// Create and run MCP server (SSE transport, nil ytClient — lazy init after OAuth)
	srv := server.NewServer(logger, nil, cfg.Transport, cfg.Port, mcpOAuth)
	if err := srv.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
