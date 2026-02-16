package youtube

import (
	"context"
	"fmt"
)

// SearchResult represents a single YouTube search result
type SearchResult struct {
	VideoID      string
	Title        string
	ChannelTitle string
	Description  string
}

// VideoDetail represents detailed information about a YouTube video
type VideoDetail struct {
	ID           string
	Title        string
	ChannelTitle string
	Description  string
	Duration     string
	PublishedAt  string
}

// SearchVideos searches YouTube for videos matching the query.
// Returns only the first page of results (no pagination) to conserve quota.
// Each search costs 100 quota units.
func (c *Client) SearchVideos(ctx context.Context, query string, maxResults int64) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Default to 10 results if not specified
	if maxResults <= 0 {
		maxResults = 10
	}
	// Cap at 25 to keep single page
	if maxResults > 25 {
		maxResults = 25
	}

	// Search for videos in Music category (videoCategoryId=10)
	// Use single-page .Do() not .Pages() to conserve quota (100 units per page)
	call := c.service.Search.List([]string{"snippet"}).
		Q(query).
		Type("video").
		VideoCategoryId("10").
		MaxResults(maxResults)

	resp, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results := make([]SearchResult, 0, len(resp.Items))
	for _, item := range resp.Items {
		results = append(results, SearchResult{
			VideoID:      item.Id.VideoId,
			Title:        item.Snippet.Title,
			ChannelTitle: item.Snippet.ChannelTitle,
			Description:  item.Snippet.Description,
		})
	}

	return results, nil
}

// GetVideo retrieves detailed information about a specific video by ID.
// Returns nil, nil if the video is not found (not an error).
// Costs only 1 quota unit.
func (c *Client) GetVideo(ctx context.Context, videoID string) (*VideoDetail, error) {
	if videoID == "" {
		return nil, fmt.Errorf("video ID cannot be empty")
	}

	call := c.service.Videos.List([]string{"snippet", "contentDetails"}).
		Id(videoID)

	resp, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get video: %w", err)
	}

	// Video not found - not an error
	if len(resp.Items) == 0 {
		return nil, nil
	}

	item := resp.Items[0]
	return &VideoDetail{
		ID:           item.Id,
		Title:        item.Snippet.Title,
		ChannelTitle: item.Snippet.ChannelTitle,
		Description:  item.Snippet.Description,
		Duration:     item.ContentDetails.Duration,
		PublishedAt:  item.Snippet.PublishedAt,
	}, nil
}
