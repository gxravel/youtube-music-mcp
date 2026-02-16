package server

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input/output types for search tools

type searchVideosInput struct {
	Query      string `json:"query" jsonschema:"required,description=Search query (e.g. artist name + song title or general music query)"`
	MaxResults int64  `json:"maxResults" jsonschema:"description=Maximum results to return (default 10, max 25). WARNING: each search costs 100 API quota units,minimum=1,maximum=25"`
}

type searchResultInfo struct {
	VideoID      string `json:"videoId" jsonschema:"description=YouTube video ID (use with get_video or for playlist building)"`
	Title        string `json:"title" jsonschema:"description=Video title"`
	ChannelTitle string `json:"channelTitle" jsonschema:"description=Channel that uploaded the video"`
	Description  string `json:"description" jsonschema:"description=Video description snippet"`
}

type searchOutput struct {
	Results []searchResultInfo `json:"results"`
	Query   string             `json:"query" jsonschema:"description=The search query that was executed"`
	Count   int                `json:"count" jsonschema:"description=Number of results returned"`
}

type getVideoInput struct {
	VideoID string `json:"videoId" jsonschema:"required,description=YouTube video ID to look up"`
}

type videoDetailInfo struct {
	ID           string `json:"id" jsonschema:"description=YouTube video ID"`
	Title        string `json:"title" jsonschema:"description=Video title"`
	ChannelTitle string `json:"channelTitle" jsonschema:"description=Channel that uploaded the video"`
	Description  string `json:"description" jsonschema:"description=Video description"`
	Duration     string `json:"duration" jsonschema:"description=Video duration in ISO 8601 format (e.g. PT4M30S)"`
	PublishedAt  string `json:"publishedAt" jsonschema:"description=Video publish date"`
	Found        bool   `json:"found" jsonschema:"description=Whether the video was found"`
}

// registerSearchTools registers all search-related MCP tools
func (s *Server) registerSearchTools() {
	// Tool 1: search_videos
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "search_videos",
		Description: "Search YouTube for music videos. Results are filtered to the Music category. WARNING: Each search costs 100 API quota units (daily limit is 10,000 units). Use sparingly — prefer get_video (1 unit) when you already have a video ID.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchVideosInput) (*mcp.CallToolResult, searchOutput, error) {
		// Call YouTube client
		results, err := s.ytClient.SearchVideos(ctx, input.Query, input.MaxResults)
		if err != nil {
			return nil, searchOutput{}, fmt.Errorf("failed to search videos: %w", err)
		}

		// Convert to output format
		searchResults := make([]searchResultInfo, len(results))
		for i, r := range results {
			searchResults[i] = searchResultInfo{
				VideoID:      r.VideoID,
				Title:        r.Title,
				ChannelTitle: r.ChannelTitle,
				Description:  r.Description,
			}
		}

		output := searchOutput{
			Results: searchResults,
			Query:   input.Query,
			Count:   len(searchResults),
		}

		// Return result with summary
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Found %d music videos for query '%s'", len(results), input.Query)},
			},
		}, output, nil
	})

	// Tool 2: get_video
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_video",
		Description: "Look up a specific YouTube video by its ID. Returns video details including title, channel, duration, and whether it exists. Use this to verify a video exists before adding it to a playlist. Quota cost: 1 unit.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getVideoInput) (*mcp.CallToolResult, videoDetailInfo, error) {
		// Call YouTube client
		video, err := s.ytClient.GetVideo(ctx, input.VideoID)
		if err != nil {
			return nil, videoDetailInfo{}, fmt.Errorf("failed to get video: %w", err)
		}

		// Video not found
		if video == nil {
			output := videoDetailInfo{
				ID:    input.VideoID,
				Found: false,
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Video not found"},
				},
			}, output, nil
		}

		// Video found
		output := videoDetailInfo{
			ID:           video.ID,
			Title:        video.Title,
			ChannelTitle: video.ChannelTitle,
			Description:  video.Description,
			Duration:     video.Duration,
			PublishedAt:  video.PublishedAt,
			Found:        true,
		}

		// Return result with summary
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Found video: %s by %s (duration: %s)", video.Title, video.ChannelTitle, video.Duration)},
			},
		}, output, nil
	})
}
