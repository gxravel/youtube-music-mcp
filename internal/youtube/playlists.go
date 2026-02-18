package youtube

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/api/googleapi"
	youtube_v3 "google.golang.org/api/youtube/v3"
)

// Domain types for playlist and video data
type Video struct {
	ID           string
	Title        string
	ChannelTitle string
}

type Playlist struct {
	ID          string
	Title       string
	Description string
	ItemCount   int64
}

// GetLikedVideos retrieves ALL of the user's liked videos with no pagination cap.
func (c *Client) GetLikedVideos(ctx context.Context) ([]Video, error) {
	// First, get the likes playlist ID
	channelsCall := c.service.Channels.List([]string{"contentDetails"}).Mine(true)
	channelsResp, err := channelsCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get likes playlist ID: %w", err)
	}

	if len(channelsResp.Items) == 0 {
		return nil, fmt.Errorf("no channel found for authenticated user")
	}

	likesPlaylistID := channelsResp.Items[0].ContentDetails.RelatedPlaylists.Likes
	if likesPlaylistID == "" {
		return nil, fmt.Errorf("no likes playlist found")
	}

	// Retrieve all liked videos using pagination (no cap)
	var videos []Video
	playlistItemsCall := c.service.PlaylistItems.
		List([]string{"snippet"}).
		PlaylistId(likesPlaylistID).
		MaxResults(50)

	err = playlistItemsCall.Pages(ctx, func(response *youtube_v3.PlaylistItemListResponse) error {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		// Extract videos from this page
		for _, item := range response.Items {
			videos = append(videos, Video{
				ID:           item.Snippet.ResourceId.VideoId,
				Title:        item.Snippet.Title,
				ChannelTitle: item.Snippet.VideoOwnerChannelTitle,
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve liked videos: %w", err)
	}

	return videos, nil
}

// ListPlaylists retrieves ALL of the user's playlists with no pagination cap.
func (c *Client) ListPlaylists(ctx context.Context) ([]Playlist, error) {
	var playlists []Playlist
	playlistsCall := c.service.Playlists.
		List([]string{"snippet", "contentDetails"}).
		Mine(true).
		MaxResults(50)

	err := playlistsCall.Pages(ctx, func(response *youtube_v3.PlaylistListResponse) error {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		// Extract playlists from this page
		for _, item := range response.Items {
			playlists = append(playlists, Playlist{
				ID:          item.Id,
				Title:       item.Snippet.Title,
				Description: item.Snippet.Description,
				ItemCount:   item.ContentDetails.ItemCount,
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list playlists: %w", err)
	}

	return playlists, nil
}

// GetPlaylistItems retrieves ALL videos from a specific playlist with no pagination cap.
func (c *Client) GetPlaylistItems(ctx context.Context, playlistID string) ([]Video, error) {
	// Validate input
	if playlistID == "" {
		return nil, fmt.Errorf("playlistID cannot be empty")
	}

	var videos []Video
	playlistItemsCall := c.service.PlaylistItems.
		List([]string{"snippet"}).
		PlaylistId(playlistID).
		MaxResults(50)

	err := playlistItemsCall.Pages(ctx, func(response *youtube_v3.PlaylistItemListResponse) error {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		// Extract videos from this page
		for _, item := range response.Items {
			videos = append(videos, Video{
				ID:           item.Snippet.ResourceId.VideoId,
				Title:        item.Snippet.Title,
				ChannelTitle: item.Snippet.VideoOwnerChannelTitle,
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve playlist items: %w", err)
	}

	return videos, nil
}

// CreatePlaylist creates a new playlist on the user's YouTube Music account.
// Quota cost: 50 units.
func (c *Client) CreatePlaylist(ctx context.Context, title, description, privacyStatus string) (*Playlist, error) {
	// Validate title is non-empty
	if title == "" {
		return nil, fmt.Errorf("title cannot be empty")
	}

	// Default privacyStatus to "private" if empty
	if privacyStatus == "" {
		privacyStatus = "private"
	}

	// Validate privacyStatus
	validPrivacy := map[string]bool{"public": true, "private": true, "unlisted": true}
	if !validPrivacy[privacyStatus] {
		return nil, fmt.Errorf("invalid privacyStatus: must be one of 'public', 'private', or 'unlisted'")
	}

	// Create playlist via YouTube API
	playlist := &youtube_v3.Playlist{
		Snippet: &youtube_v3.PlaylistSnippet{
			Title:       title,
			Description: description,
		},
		Status: &youtube_v3.PlaylistStatus{
			PrivacyStatus: privacyStatus,
		},
	}

	call := c.service.Playlists.Insert([]string{"snippet", "status"}, playlist)
	resp, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create playlist: %w", err)
	}

	// Return domain Playlist
	return &Playlist{
		ID:          resp.Id,
		Title:       resp.Snippet.Title,
		Description: resp.Snippet.Description,
		ItemCount:   0,
	}, nil
}

// AddVideosToPlaylist adds one or more videos to an existing playlist.
// Duplicates are skipped silently. Returns the count of successfully added videos.
// Quota cost: 50 units per video added.
func (c *Client) AddVideosToPlaylist(ctx context.Context, playlistID string, videoIDs []string) (int, error) {
	// Validate inputs
	if playlistID == "" {
		return 0, fmt.Errorf("playlistID cannot be empty")
	}
	if len(videoIDs) == 0 {
		return 0, fmt.Errorf("videoIDs cannot be empty")
	}

	successCount := 0

	// Add each video to the playlist
	for _, videoID := range videoIDs {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return successCount, err
		}

		// Create playlist item
		playlistItem := &youtube_v3.PlaylistItem{
			Snippet: &youtube_v3.PlaylistItemSnippet{
				PlaylistId: playlistID,
				ResourceId: &youtube_v3.ResourceId{
					Kind:    "youtube#video",
					VideoId: videoID,
				},
			},
		}

		// Insert the item
		call := c.service.PlaylistItems.Insert([]string{"snippet"}, playlistItem)
		_, err := call.Do()
		if err != nil {
			// Check for duplicate error
			var apiErr *googleapi.Error
			if errors.As(err, &apiErr) {
				// HTTP 409 or message contains "videoAlreadyInPlaylist" - skip silently
				if apiErr.Code == 409 || strings.Contains(apiErr.Message, "videoAlreadyInPlaylist") {
					continue
				}
			}
			// Other errors - return with current success count
			return successCount, fmt.Errorf("failed to add video %s to playlist: %w", videoID, err)
		}

		successCount++
	}

	return successCount, nil
}
