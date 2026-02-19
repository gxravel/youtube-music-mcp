package server

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// instructionalWords are words that indicate a phrase is an LLM instruction, not a search term.
var instructionalWords = []string{
	"focus", "include", "avoid", "exclude", "not ", "these are",
	"based on", "should", "must", "make sure", "prefer", "prioritize",
	"similar to what", "songs from", "mix of",
}

// splitDescriptionIntoTerms splits a description into individual search-friendly terms.
func splitDescriptionIntoTerms(description string) []string {
	// Split by commas, periods, semicolons, colons, and the word "and"
	re := regexp.MustCompile(`[,;.:\n]+|\band\b`)
	parts := re.Split(description, -1)

	var terms []string
	for _, part := range parts {
		term := strings.TrimSpace(part)
		if term == "" {
			continue
		}

		// Skip instructional/meta phrases
		lower := strings.ToLower(term)
		isInstructional := false
		for _, word := range instructionalWords {
			if strings.Contains(lower, word) {
				isInstructional = true
				break
			}
		}
		if isInstructional {
			continue
		}

		// Cap at 80 characters
		if len(term) > 80 {
			term = term[:80]
		}

		terms = append(terms, term)
	}

	return terms
}

// Input types for recommendation tools

type recommendPlaylistInput struct {
	NumberOfSongs int    `json:"numberOfSongs" jsonschema:"Number of songs to find and add to the playlist (1-50)"`
	Description   string `json:"description,omitempty" jsonschema:"What kind of music to find (genres/moods/artists/era). If empty recommendations are based purely on taste analysis."`
}

type recommendArtistsInput struct {
	Description string `json:"description,omitempty" jsonschema:"What kind of artists to recommend (genre preferences/mood/any guidance)"`
}

type recommendAlbumsInput struct {
	Description string `json:"description,omitempty" jsonschema:"What kind of albums to recommend (genre preferences/mood/era/any guidance)"`
}

// registerRecommendTools registers the 3 recommendation MCP tools
func (s *Server) registerRecommendTools() {
	// Tool 1: ym:recommend-playlist
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ym:recommend-playlist",
		Description: "Creates a playlist with recommended music based on the user's taste and an optional description. Gathers taste data, searches for songs, creates a playlist, and adds songs in one call. WARNING: Each search costs 100 quota units. This tool will use multiple searches to find diverse songs. Quota cost: ~200-500 units depending on number of songs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input recommendPlaylistInput) (*mcp.CallToolResult, any, error) {
		// Gather taste context (uses full library - no caps)
		likedVideos, err := s.ytClient.GetLikedVideos(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get liked videos: %w", err)
		}

		subscriptions, err := s.ytClient.GetSubscriptions(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get subscriptions: %w", err)
		}

		playlists, err := s.ytClient.ListPlaylists(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list playlists: %w", err)
		}

		// Build taste summary - extract unique artists/channels
		artistMap := make(map[string]int)
		for _, v := range likedVideos {
			if v.ChannelTitle != "" {
				artistMap[v.ChannelTitle]++
			}
		}
		for _, sub := range subscriptions {
			if sub.Title != "" {
				artistMap[sub.Title]++
			}
		}

		// Get top 10 most frequent artists
		type artistCount struct {
			name  string
			count int
		}
		var artists []artistCount
		for name, count := range artistMap {
			artists = append(artists, artistCount{name, count})
		}

		// Sort by count (simple bubble sort for small data)
		for i := 0; i < len(artists); i++ {
			for j := i + 1; j < len(artists); j++ {
				if artists[j].count > artists[i].count {
					artists[i], artists[j] = artists[j], artists[i]
				}
			}
		}

		// Take top 10
		topArtists := make([]string, 0, 10)
		for i := 0; i < len(artists) && i < 10; i++ {
			topArtists = append(topArtists, artists[i].name)
		}

		// Construct search queries
		maxQueries := min(int(math.Ceil(float64(input.NumberOfSongs)/3.0)), 10)

		var searchQueries []string
		if input.Description != "" {
			// Extract individual search terms from description
			terms := splitDescriptionIntoTerms(input.Description)
			for _, term := range terms {
				if len(searchQueries) >= maxQueries {
					break
				}
				searchQueries = append(searchQueries, term)
			}
		}

		// Fall back to top artists if description yielded insufficient queries
		if len(searchQueries) < maxQueries {
			for i := 0; i < len(topArtists) && len(searchQueries) < maxQueries; i++ {
				searchQueries = append(searchQueries, topArtists[i])
			}
		}

		// Execute searches and collect video IDs
		videoIDMap := make(map[string]struct{}) // Deduplication
		var videoIDs []string
		var searchSummary strings.Builder

		searchSummary.WriteString("Search queries executed:\n")
		for _, query := range searchQueries {
			results, err := s.ytClient.SearchVideos(ctx, query, 5)
			if err != nil {
				// Log error but continue with other searches
				s.logger.Warn("search failed", "query", query, "error", err)
				fmt.Fprintf(&searchSummary, "- '%s' (failed)\n", query)
				continue
			}

			fmt.Fprintf(&searchSummary, "- '%s' (%d results)\n", query, len(results))

			for _, result := range results {
				if _, exists := videoIDMap[result.VideoID]; !exists {
					videoIDMap[result.VideoID] = struct{}{}
					videoIDs = append(videoIDs, result.VideoID)

					// Stop if we have enough songs
					if len(videoIDs) >= input.NumberOfSongs {
						break
					}
				}
			}

			if len(videoIDs) >= input.NumberOfSongs {
				break
			}
		}

		// Truncate to requested number
		if len(videoIDs) > input.NumberOfSongs {
			videoIDs = videoIDs[:input.NumberOfSongs]
		}

		if len(videoIDs) == 0 {
			return nil, nil, fmt.Errorf("no videos found for the given criteria")
		}

		// Generate playlist title
		playlistTitle := "[YM-MCP] Recommended Mix"
		if input.Description != "" {
			// Use first few words of description
			words := strings.Fields(input.Description)
			titleWords := words
			if len(words) > 4 {
				titleWords = words[:4]
			}
			playlistTitle = fmt.Sprintf("[YM-MCP] %s", strings.Join(titleWords, " "))
		}

		// Create playlist
		playlist, err := s.ytClient.CreatePlaylist(ctx, playlistTitle, input.Description, "private")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create playlist: %w", err)
		}

		// Add videos to playlist
		added, err := s.ytClient.AddVideosToPlaylist(ctx, playlist.ID, videoIDs)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to add videos to playlist: %w", err)
		}

		// Build response
		playlistURL := fmt.Sprintf("https://music.youtube.com/playlist?list=%s", playlist.ID)

		var output strings.Builder
		fmt.Fprintf(&output, "# Playlist Created: %s\n\n", playlist.Title)
		fmt.Fprintf(&output, "**YouTube Music URL:** %s\n\n", playlistURL)
		fmt.Fprintf(&output, "**Songs added:** %d of %d requested\n\n", added, input.NumberOfSongs)
		fmt.Fprintf(&output, "**Taste context:** %d liked songs, %d subscriptions, %d playlists analyzed\n\n", len(likedVideos), len(subscriptions), len(playlists))
		fmt.Fprintf(&output, "**Top artists in your taste:** %s\n\n", strings.Join(topArtists[:min(5, len(topArtists))], ", "))
		output.WriteString(searchSummary.String())
		fmt.Fprintf(&output, "\n**Estimated quota usage:** ~%d units (%d searches x 100 + 50 playlist creation + %d x 50 adds)\n", len(searchQueries)*100+50+added*50, len(searchQueries), added)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: output.String()},
			},
		}, nil, nil
	})

	// Tool 2: ym:recommend-artists
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ym:recommend-artists",
		Description: "Recommends artists the user would like based on their YouTube Music taste. Returns structured taste data for the LLM to use its own knowledge to generate recommendations. Does not search YouTube. Quota cost: ~5 units.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input recommendArtistsInput) (*mcp.CallToolResult, any, error) {
		// Gather full taste data (no caps)
		likedVideos, err := s.ytClient.GetLikedVideos(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get liked videos: %w", err)
		}

		subscriptions, err := s.ytClient.GetSubscriptions(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get subscriptions: %w", err)
		}

		// Extract unique artists
		artistMap := make(map[string]bool)
		for _, v := range likedVideos {
			if v.ChannelTitle != "" {
				artistMap[v.ChannelTitle] = true
			}
		}
		for _, sub := range subscriptions {
			if sub.Title != "" {
				artistMap[sub.Title] = true
			}
		}

		artists := make([]string, 0, len(artistMap))
		for artist := range artistMap {
			artists = append(artists, artist)
		}

		// Build output
		var output strings.Builder
		output.WriteString("# Artist Recommendation Context\n\n")

		if input.Description != "" {
			fmt.Fprintf(&output, "**User request:** %s\n\n", input.Description)
		}

		fmt.Fprintf(&output, "## Your Current Artists (%d unique artists)\n\n", len(artists))
		for _, artist := range artists {
			fmt.Fprintf(&output, "- %s\n", artist)
		}
		output.WriteString("\n")

		output.WriteString("## Taste Profile\n\n")
		fmt.Fprintf(&output, "- Based on %d liked songs and %d subscriptions\n", len(likedVideos), len(subscriptions))
		output.WriteString("- The artists listed above are already known to the user\n\n")

		output.WriteString("## Instruction for LLM\n\n")
		output.WriteString("Based on this taste data, recommend artists the user hasn't heard. Use your knowledge of music genres, similar artists, and musical styles to suggest new artists that align with the user's demonstrated preferences.\n")

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: output.String()},
			},
		}, nil, nil
	})

	// Tool 3: ym:recommend-albums
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ym:recommend-albums",
		Description: "Recommends albums the user would like based on their YouTube Music taste. Returns structured taste data for the LLM to use its own knowledge to generate recommendations. Does not search YouTube. Quota cost: ~5 units.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input recommendAlbumsInput) (*mcp.CallToolResult, any, error) {
		// Gather full taste data (no caps)
		likedVideos, err := s.ytClient.GetLikedVideos(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get liked videos: %w", err)
		}

		subscriptions, err := s.ytClient.GetSubscriptions(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get subscriptions: %w", err)
		}

		// Extract unique artists
		artistMap := make(map[string]bool)
		for _, v := range likedVideos {
			if v.ChannelTitle != "" {
				artistMap[v.ChannelTitle] = true
			}
		}
		for _, sub := range subscriptions {
			if sub.Title != "" {
				artistMap[sub.Title] = true
			}
		}

		artists := make([]string, 0, len(artistMap))
		for artist := range artistMap {
			artists = append(artists, artist)
		}

		// Build output
		var output strings.Builder
		output.WriteString("# Album Recommendation Context\n\n")

		if input.Description != "" {
			fmt.Fprintf(&output, "**User request:** %s\n\n", input.Description)
		}

		fmt.Fprintf(&output, "## Your Current Artists (%d unique artists)\n\n", len(artists))
		for _, artist := range artists {
			fmt.Fprintf(&output, "- %s\n", artist)
		}
		output.WriteString("\n")

		output.WriteString("## Taste Profile\n\n")
		fmt.Fprintf(&output, "- Based on %d liked songs and %d subscriptions\n", len(likedVideos), len(subscriptions))
		output.WriteString("- The artists listed above are already known to the user\n\n")

		output.WriteString("## Instruction for LLM\n\n")
		output.WriteString("Based on this taste data, recommend albums the user would enjoy. Use your knowledge of music genres, discographies, and musical styles to suggest albums that align with the user's demonstrated preferences.\n")

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: output.String()},
			},
		}, nil, nil
	})
}
