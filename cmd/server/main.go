package main

import (
	"context"
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

	// Create OAuth2 config
	oauthCfg := auth.NewOAuth2Config(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.OAuthRedirectURL)

	// Create token storage
	storage := auth.NewFileTokenStorage(auth.DefaultTokenPath())

	// Authenticate (either load existing token or run OAuth flow)
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

	// Create and run MCP server
	srv := server.NewServer(logger, ytClient)
	if err := srv.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
