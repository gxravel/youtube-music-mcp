package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	// GoogleClientID is the Google OAuth2 client ID (required).
	GoogleClientID string `env:"GOOGLE_CLIENT_ID,required"`

	// GoogleClientSecret is the Google OAuth2 client secret (required).
	GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET,required"`

	// OAuthRedirectURL is the OAuth callback URL (default: http://localhost:8080/callback).
	OAuthRedirectURL string `env:"OAUTH_REDIRECT_URL" envDefault:"http://localhost:8080/callback"`

	// OAuthPort is the port for the local OAuth callback server (default: 8080).
	OAuthPort int `env:"OAUTH_PORT" envDefault:"8080"`
}

// Load loads the configuration from environment variables.
// It first attempts to load a .env file (if present), then parses environment variables.
// Returns an error if required environment variables are missing.
func Load() (*Config, error) {
	// Load .env file if present (ignore error - .env is optional)
	_ = godotenv.Load()

	// Parse environment variables into Config struct
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
