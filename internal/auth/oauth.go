package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

// NewOAuth2Config creates a new OAuth2 configuration for Google YouTube API.
func NewOAuth2Config(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes:       []string{youtube.YoutubeScope},
	}
}

// Authenticate performs OAuth2 authentication, either by loading a saved token
// or initiating a web-based OAuth2 flow with a local callback server.
// Returns an authenticated HTTP client.
func Authenticate(ctx context.Context, cfg *oauth2.Config, storage TokenStorage, port int, logger *slog.Logger) (*http.Client, error) {
	// Try to load saved token
	token, err := storage.Load()
	if err == nil {
		// Token loaded successfully - create client with persisting token source
		logger.Info("Loaded token from storage")
		baseSource := cfg.TokenSource(ctx, token)
		persistingSource := NewPersistingTokenSource(baseSource, storage, logger)
		return oauth2.NewClient(ctx, persistingSource), nil
	}

	logger.Info("No saved token found, starting OAuth2 flow", "error", err.Error())

	// No saved token - start OAuth2 web flow
	authURL := cfg.AuthCodeURL("state",
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"), // Force refresh token on re-auth
	)

	fmt.Fprintf(os.Stderr, "\nVisit this URL to authorize:\n%s\n\n", authURL)

	// Start local callback server
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code in callback")
			http.Error(w, "Authorization failed: no code", http.StatusBadRequest)
			return
		}
		codeCh <- code
		fmt.Fprintf(w, "Authorization successful! You can close this window.")
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server failed: %w", err)
		}
	}()

	logger.Info("Callback server started", "port", port)

	// Wait for authorization code or context cancellation
	var code string
	select {
	case code = <-codeCh:
		logger.Info("Received authorization code")
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Shut down callback server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Failed to shut down callback server", "error", err)
	}

	// Exchange authorization code for token
	token, err = cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	logger.Info("Successfully exchanged code for token")

	// Save token
	if err := storage.Save(token); err != nil {
		logger.Error("Failed to save token", "error", err)
		// Continue anyway - token is still valid for this session
	} else {
		logger.Info("Token saved to storage")
	}

	// Create client with persisting token source
	baseSource := cfg.TokenSource(ctx, token)
	persistingSource := NewPersistingTokenSource(baseSource, storage, logger)
	return oauth2.NewClient(ctx, persistingSource), nil
}
