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

// FilterMusicVideos filters a slice of videos to only those in the Music category
// (categoryId == "10"). Processes in batches of 50 to stay within API limits.
// Quota cost: 1 unit per 50 videos.
func (c *Client) FilterMusicVideos(ctx context.Context, videos []Video) ([]Video, error) {
	if len(videos) == 0 {
		return videos, nil
	}

	// Build a map of videoID -> Video for quick lookup
	videoMap := make(map[string]Video, len(videos))
	ids := make([]string, 0, len(videos))
	for _, v := range videos {
		if v.ID != "" {
			videoMap[v.ID] = v
			ids = append(ids, v.ID)
		}
	}

	const batchSize = 50
	musicIDs := make(map[string]struct{})

	for i := 0; i < len(ids); i += batchSize {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		resp, err := c.service.Videos.
			List([]string{"snippet"}).
			Id(batch...).
			Fields("items(id,snippet/categoryId)").
			Do()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch video categories: %w", err)
		}

		for _, item := range resp.Items {
			if item.Snippet != nil && item.Snippet.CategoryId == "10" {
				musicIDs[item.Id] = struct{}{}
			}
		}
	}

	// Return only music videos in original order
	filtered := make([]Video, 0, len(musicIDs))
	for _, v := range videos {
		if _, ok := musicIDs[v.ID]; ok {
			filtered = append(filtered, v)
		}
	}

	return filtered, nil
}
