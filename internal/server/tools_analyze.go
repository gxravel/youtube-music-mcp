package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input type for analyze tool

type analyzeTastesInput struct {
	IncludePreviousRecommendations bool `json:"includePreviousRecommendations" jsonschema:"description=If true\\, also fetch songs from playlists previously created by this tool to adjust analysis"`
}

// registerAnalyzeTools registers the analyze-my-tastes MCP tool
func (s *Server) registerAnalyzeTools() {
	// Tool: ym:analyze-my-tastes
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ym:analyze-my-tastes",
		Description: "Analyzes the user's YouTube Music taste by gathering liked videos, subscriptions, playlists, and optionally previously recommended songs. Returns structured text analysis for the LLM to interpret. Quota cost: ~5-10 units (depending on whether previous recommendations are included).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input analyzeTastesInput) (*mcp.CallToolResult, any, error) {
		var output strings.Builder

		output.WriteString("# YouTube Music Taste Analysis\n\n")

		// 1. Fetch liked videos
		likedVideos, err := s.ytClient.GetLikedVideos(ctx, 200)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get liked videos: %w", err)
		}

		fmt.Fprintf(&output, "## Liked Songs (%d songs)\n\n", len(likedVideos))
		for _, v := range likedVideos {
			fmt.Fprintf(&output, "- %s - %s\n", v.Title, v.ChannelTitle)
		}
		output.WriteString("\n")

		// 2. Fetch subscriptions
		subscriptions, err := s.ytClient.GetSubscriptions(ctx, 100)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get subscriptions: %w", err)
		}

		fmt.Fprintf(&output, "## Subscribed Channels (%d channels)\n\n", len(subscriptions))
		for _, sub := range subscriptions {
			fmt.Fprintf(&output, "- %s\n", sub.Title)
		}
		output.WriteString("\n")

		// 3. Fetch user's playlists
		playlists, err := s.ytClient.ListPlaylists(ctx, 50)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list playlists: %w", err)
		}

		fmt.Fprintf(&output, "## Your Playlists (%d playlists)\n\n", len(playlists))
		for _, pl := range playlists {
			fmt.Fprintf(&output, "- %s (%d items)\n", pl.Title, pl.ItemCount)
		}
		output.WriteString("\n")

		// 4. If requested, fetch songs from previous recommendations
		if input.IncludePreviousRecommendations {
			output.WriteString("## Previously Recommended Songs\n\n")

			recommendedSongs := 0
			for _, pl := range playlists {
				// Check if playlist was created by this tool
				if strings.HasPrefix(pl.Title, "[YM-MCP]") {
					// Fetch playlist items
					items, err := s.ytClient.GetPlaylistItems(ctx, pl.ID, 100)
					if err != nil {
						// Log error but continue
						s.logger.Warn("failed to fetch items for playlist", "playlist", pl.Title, "error", err)
						continue
					}

					if len(items) > 0 {
						fmt.Fprintf(&output, "\nFrom playlist '%s':\n", pl.Title)
						for _, item := range items {
							fmt.Fprintf(&output, "- %s - %s\n", item.Title, item.ChannelTitle)
							recommendedSongs++
						}
					}
				}
			}

			if recommendedSongs == 0 {
				output.WriteString("No previously recommended songs found.\n")
			}
			output.WriteString("\n")
		}

		// Return as text content
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: output.String()},
			},
		}, nil, nil
	})
}
