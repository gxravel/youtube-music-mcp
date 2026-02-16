package server

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input/output types for playlist tools

type getLikedVideosInput struct {
	MaxResults int64 `json:"maxResults" jsonschema:"description=Maximum number of liked videos to return (default 50),minimum=1,maximum=500"`
}

type videoInfo struct {
	ID           string `json:"id" jsonschema:"description=YouTube video ID"`
	Title        string `json:"title" jsonschema:"description=Video title"`
	ChannelTitle string `json:"channelTitle" jsonschema:"description=Channel that uploaded the video"`
}

type videosOutput struct {
	Videos []videoInfo `json:"videos"`
	Count  int         `json:"count" jsonschema:"description=Number of videos returned"`
}

type listPlaylistsInput struct {
	MaxResults int64 `json:"maxResults" jsonschema:"description=Maximum number of playlists to return (default 25),minimum=1,maximum=500"`
}

type playlistInfo struct {
	ID          string `json:"id" jsonschema:"description=YouTube playlist ID"`
	Title       string `json:"title" jsonschema:"description=Playlist title"`
	Description string `json:"description" jsonschema:"description=Playlist description"`
	ItemCount   int64  `json:"itemCount" jsonschema:"description=Number of items in the playlist"`
}

type playlistsOutput struct {
	Playlists []playlistInfo `json:"playlists"`
	Count     int            `json:"count" jsonschema:"description=Number of playlists returned"`
}

type getPlaylistItemsInput struct {
	PlaylistID string `json:"playlistId" jsonschema:"required,description=YouTube playlist ID (from list_playlists)"`
	MaxResults int64  `json:"maxResults" jsonschema:"description=Maximum number of playlist items to return (default 50),minimum=1,maximum=500"`
}

// registerPlaylistTools registers all playlist-related MCP tools
func (s *Server) registerPlaylistTools() {
	// Tool 1: get_liked_videos
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_liked_videos",
		Description: "Retrieve the user's liked videos/songs from YouTube. These represent songs the user has explicitly liked. Quota cost: ~2 units.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getLikedVideosInput) (*mcp.CallToolResult, videosOutput, error) {
		// Call YouTube client
		videos, err := s.ytClient.GetLikedVideos(ctx, input.MaxResults)
		if err != nil {
			return nil, videosOutput{}, fmt.Errorf("failed to get liked videos: %w", err)
		}

		// Convert to output format
		videoInfos := make([]videoInfo, len(videos))
		for i, v := range videos {
			videoInfos[i] = videoInfo{
				ID:           v.ID,
				Title:        v.Title,
				ChannelTitle: v.ChannelTitle,
			}
		}

		output := videosOutput{
			Videos: videoInfos,
			Count:  len(videoInfos),
		}

		// Return result with summary
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Retrieved %d liked videos", len(videos))},
			},
		}, output, nil
	})

	// Tool 2: list_playlists
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_playlists",
		Description: "List all playlists on the user's YouTube account with their titles and track counts. Quota cost: ~1 unit per 50 playlists.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input listPlaylistsInput) (*mcp.CallToolResult, playlistsOutput, error) {
		// Call YouTube client
		playlists, err := s.ytClient.ListPlaylists(ctx, input.MaxResults)
		if err != nil {
			return nil, playlistsOutput{}, fmt.Errorf("failed to list playlists: %w", err)
		}

		// Convert to output format
		playlistInfos := make([]playlistInfo, len(playlists))
		for i, p := range playlists {
			playlistInfos[i] = playlistInfo{
				ID:          p.ID,
				Title:       p.Title,
				Description: p.Description,
				ItemCount:   p.ItemCount,
			}
		}

		output := playlistsOutput{
			Playlists: playlistInfos,
			Count:     len(playlistInfos),
		}

		// Return result with summary
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Retrieved %d playlists", len(playlists))},
			},
		}, output, nil
	})

	// Tool 3: get_playlist_items
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_playlist_items",
		Description: "Retrieve the videos/tracks in a specific playlist by playlist ID. Use list_playlists first to get playlist IDs. Quota cost: ~1 unit per 50 items.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getPlaylistItemsInput) (*mcp.CallToolResult, videosOutput, error) {
		// Call YouTube client
		videos, err := s.ytClient.GetPlaylistItems(ctx, input.PlaylistID, input.MaxResults)
		if err != nil {
			return nil, videosOutput{}, fmt.Errorf("failed to get playlist items: %w", err)
		}

		// Convert to output format
		videoInfos := make([]videoInfo, len(videos))
		for i, v := range videos {
			videoInfos[i] = videoInfo{
				ID:           v.ID,
				Title:        v.Title,
				ChannelTitle: v.ChannelTitle,
			}
		}

		output := videosOutput{
			Videos: videoInfos,
			Count:  len(videoInfos),
		}

		// Return result with summary
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Retrieved %d videos from playlist", len(videos))},
			},
		}, output, nil
	})
}
