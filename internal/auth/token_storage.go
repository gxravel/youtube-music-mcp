package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
)

// TokenStorage defines the interface for persisting OAuth2 tokens.
type TokenStorage interface {
	// Load retrieves a token from storage.
	Load() (*oauth2.Token, error)

	// Save persists a token to storage.
	Save(token *oauth2.Token) error
}

// FileTokenStorage implements TokenStorage using file-based persistence.
type FileTokenStorage struct {
	path string
}

// NewFileTokenStorage creates a new FileTokenStorage with the specified path.
func NewFileTokenStorage(path string) *FileTokenStorage {
	return &FileTokenStorage{path: path}
}

// DefaultTokenPath returns the default path for token storage:
// ~/.config/youtube-music-mcp/token.json
func DefaultTokenPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to $HOME/.config
		home, err := os.UserHomeDir()
		if err != nil {
			return "token.json" // Last resort fallback
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "youtube-music-mcp", "token.json")
}

// Load reads the token from the file.
func (f *FileTokenStorage) Load() (*oauth2.Token, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// Save persists the token to the file atomically.
// It creates the parent directory if it doesn't exist, writes to a temporary file,
// and then renames it to the target path.
func (f *FileTokenStorage) Save(token *oauth2.Token) error {
	// Ensure parent directory exists
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// Marshal token to indented JSON
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Write to temporary file
	tmpPath := f.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary token file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, f.path); err != nil {
		return fmt.Errorf("failed to rename token file: %w", err)
	}

	return nil
}

// EnvTokenStorage implements TokenStorage using a token provided as a JSON string
// in an environment variable. Useful for Railway/serverless deployments where
// filesystem persistence is not available.
// Note: Save is a no-op — token refreshes will not be persisted between restarts.
type EnvTokenStorage struct {
	tokenJSON string
	logger    *slog.Logger
}

// NewEnvTokenStorage creates a new EnvTokenStorage from a raw JSON token string.
func NewEnvTokenStorage(tokenJSON string, logger *slog.Logger) *EnvTokenStorage {
	return &EnvTokenStorage{tokenJSON: tokenJSON, logger: logger}
}

// Load parses the JSON token string and returns the OAuth2 token.
func (e *EnvTokenStorage) Load() (*oauth2.Token, error) {
	if e.tokenJSON == "" {
		return nil, fmt.Errorf("OAUTH_TOKEN_JSON is empty")
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(e.tokenJSON), &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OAUTH_TOKEN_JSON: %w", err)
	}

	return &token, nil
}

// Save is a no-op for EnvTokenStorage. Token refreshes are not persisted.
// A warning is logged to alert operators that refresh tokens may expire.
func (e *EnvTokenStorage) Save(_ *oauth2.Token) error {
	if e.logger != nil {
		e.logger.Warn("EnvTokenStorage: token refresh cannot be persisted; update OAUTH_TOKEN_JSON when token expires")
	}
	return nil
}

// PersistingTokenSource wraps an oauth2.TokenSource to automatically persist
// refreshed tokens to storage.
type PersistingTokenSource struct {
	base      oauth2.TokenSource
	storage   TokenStorage
	logger    *slog.Logger
	mu        sync.Mutex
	lastToken *oauth2.Token
}

// NewPersistingTokenSource creates a new PersistingTokenSource.
func NewPersistingTokenSource(base oauth2.TokenSource, storage TokenStorage, logger *slog.Logger) *PersistingTokenSource {
	return &PersistingTokenSource{
		base:    base,
		storage: storage,
		logger:  logger,
	}
}

// Token returns a valid token, refreshing if necessary, and persists any refreshed token.
func (p *PersistingTokenSource) Token() (*oauth2.Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Get token from base source (may trigger refresh)
	token, err := p.base.Token()
	if err != nil {
		return nil, err
	}

	// Check if token was refreshed (access token changed)
	if p.lastToken == nil || p.lastToken.AccessToken != token.AccessToken {
		// Token was refreshed - persist it
		if err := p.storage.Save(token); err != nil {
			p.logger.Error("Failed to persist refreshed token", "error", err)
			// Don't fail the request - return the token anyway
		} else {
			p.logger.Info("Persisted refreshed token")
		}
		p.lastToken = token
	}

	return token, nil
}

// MemoryTokenStorage implements TokenStorage using an in-memory token.
// It is safe for concurrent use. Useful for server-side OAuth flows where
// the token is obtained via the /callback endpoint and stored in memory.
// Note: Token is lost on process restart — use as a short-lived holder.
type MemoryTokenStorage struct {
	mu    sync.RWMutex
	token *oauth2.Token
}

// NewMemoryTokenStorage creates a new empty MemoryTokenStorage.
func NewMemoryTokenStorage() *MemoryTokenStorage {
	return &MemoryTokenStorage{}
}

// Load returns the stored token, or an error if no token has been saved yet.
func (m *MemoryTokenStorage) Load() (*oauth2.Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.token == nil {
		return nil, fmt.Errorf("no token stored in memory")
	}
	return m.token, nil
}

// Save stores the token in memory.
func (m *MemoryTokenStorage) Save(token *oauth2.Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
	return nil
}

// HasToken reports whether a token has been stored.
func (m *MemoryTokenStorage) HasToken() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token != nil
}

// Verify interfaces are implemented at compile time
var _ TokenStorage = (*FileTokenStorage)(nil)
var _ TokenStorage = (*EnvTokenStorage)(nil)
var _ TokenStorage = (*MemoryTokenStorage)(nil)
var _ oauth2.TokenSource = (*PersistingTokenSource)(nil)
