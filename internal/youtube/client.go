package youtube

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// Client wraps the YouTube API service with helper methods
type Client struct {
	service *youtube.Service
}

// NewClient creates a new YouTube API client using the provided HTTP client
func NewClient(ctx context.Context, httpClient *http.Client) (*Client, error) {
	service, err := youtube.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create youtube service: %w", err)
	}

	return &Client{
		service: service,
	}, nil
}

// ValidateAuth validates the authenticated user has access to YouTube API
// by fetching their channel information. Returns the channel name on success.
func (c *Client) ValidateAuth(ctx context.Context) (string, error) {
	call := c.service.Channels.List([]string{"snippet"}).Mine(true)
	resp, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("auth validation failed: %w", err)
	}

	if len(resp.Items) == 0 {
		return "", fmt.Errorf("no channel found for authenticated user")
	}

	return resp.Items[0].Snippet.Title, nil
}
