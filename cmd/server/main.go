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
	"golang.org/x/oauth2"
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

	switch cfg.Transport {
	case "sse":
		runSSEMode(ctx, cfg, oauthCfg, logger)
	default:
		runStdioMode(ctx, cfg, oauthCfg, logger)
	}
}

// runStdioMode is the original flow: authenticate first (blocking), then serve MCP on stdio.
// Kept exactly as before — no behavior changes.
func runStdioMode(ctx context.Context, cfg *config.Config, oauthCfg *oauth2.Config, logger *slog.Logger) {
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
	srv := server.NewServer(logger, ytClient, cfg.Transport, cfg.Port, nil, nil)
	if err := srv.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// runSSEMode starts the HTTP server immediately (passes Railway health checks before auth),
// then gates /sse behind a browser-based OAuth flow at /auth.
//
// If OAUTH_TOKEN_JSON is set, the server bootstraps with that token immediately and
// /sse works without going through /auth (backward compatible).
func runSSEMode(ctx context.Context, cfg *config.Config, oauthCfg *oauth2.Config, logger *slog.Logger) {
	// Always use MemoryTokenStorage for SSE mode.
	memStorage := auth.NewMemoryTokenStorage()

	var ytClient *youtube.Client // nil unless we can bootstrap from existing token

	if cfg.TokenJSON != "" {
		// Bootstrap: load token from env, populate memory storage, create ytClient now.
		logger.Info("bootstrapping from OAUTH_TOKEN_JSON")
		envStorage := auth.NewEnvTokenStorage(cfg.TokenJSON, logger)
		token, err := envStorage.Load()
		if err != nil {
			logger.Warn("failed to load OAUTH_TOKEN_JSON; server will require /auth flow", "error", err)
		} else {
			if err := memStorage.Save(token); err != nil {
				logger.Warn("failed to save bootstrap token to memory storage", "error", err)
			} else {
				// Create HTTP client from bootstrapped token (persisting source saves refreshes to memStorage)
				baseSource := oauthCfg.TokenSource(ctx, token)
				persistingSource := auth.NewPersistingTokenSource(baseSource, memStorage, logger)
				httpClient := oauth2.NewClient(ctx, persistingSource)

				yt, err := youtube.NewClient(ctx, httpClient)
				if err != nil {
					logger.Warn("failed to create youtube client from bootstrap token; server will require /auth", "error", err)
				} else {
					channelName, err := yt.ValidateAuth(ctx)
					if err != nil {
						logger.Warn("bootstrap token invalid; server will require /auth", "error", err)
					} else {
						logger.Info("bootstrapped from OAUTH_TOKEN_JSON", "channel", channelName)
						ytClient = yt
					}
				}
			}
		}
	}

	// Create and run MCP server.
	// ytClient is nil when no valid bootstrap token — /sse will return 503 until /auth completes.
	srv := server.NewServer(logger, ytClient, cfg.Transport, cfg.Port, oauthCfg, memStorage)
	if err := srv.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
