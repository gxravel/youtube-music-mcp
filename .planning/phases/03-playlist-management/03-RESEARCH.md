# Phase 3: Playlist Management - Research

**Researched:** 2026-02-16
**Domain:** YouTube Data API v3 playlist creation and modification using Go
**Confidence:** HIGH

## Summary

Phase 3 focuses on implementing playlist management capabilities: creating playlists, adding videos to playlists, and listing existing playlists (already implemented in Phase 2). The YouTube Data API v3 provides `playlists.insert` (create) and `playlistItems.insert` (add videos) methods, both costing 50 quota units per call—significantly more expensive than read operations (1 unit) but far cheaper than search (100 units).

The existing `google.golang.org/api/youtube/v3` Go client provides idiomatic access to these write operations through `PlaylistsService.Insert()` and `PlaylistItemsService.Insert()` methods. The implementation pattern follows Phase 1 and 2 established conventions: service layer methods on the YouTube client wrapper, MCP tool registration using `mcp.AddTool[In, Out]()` for automatic schema generation, and comprehensive error handling for quota limits and validation failures.

Key technical considerations: (1) write operations require OAuth2 authorization scopes that were configured in Phase 1, (2) playlists have a maximum limit of 5,000 videos and channels are limited to ~1,000 playlists total, (3) `videoAlreadyInPlaylist` errors occur when adding duplicates (API is NOT idempotent), (4) privacy status defaults to "private" unless explicitly set to "public" or "unlisted", and (5) the `snippet.position` field only works for playlists using manual sorting.

**Primary recommendation:** Implement `CreatePlaylist()` and `AddVideosToPlaylist()` methods on the YouTube client following established patterns. Use batch operations when adding multiple videos to conserve quota and reduce latency. Register two MCP tools: `create_playlist` (name, description, privacy) and `add_to_playlist` (playlistID, videoIDs array). The existing `list_playlists` tool from Phase 2 already satisfies PLST-03.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| google.golang.org/api/youtube/v3 | v0.266.0 (installed) | YouTube Data API v3 access | Official Google API client, same library used in Phase 1 & 2, provides `PlaylistsService.Insert()` and `PlaylistItemsService.Insert()` |
| github.com/modelcontextprotocol/go-sdk/mcp | v1.3.0 (installed) | MCP server implementation | Type-safe `mcp.AddTool[In, Out]()` pattern established in Phase 2 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| google.golang.org/api/googleapi | (part of google.golang.org/api) | Error handling, API call options | Type assertions for `*googleapi.Error`, extracting HTTP status codes, handling `videoAlreadyInPlaylist` and quota errors |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Individual playlistItems.insert calls | Batch HTTP requests via googleapi.BatchCall | Batch requests can reduce quota cost and latency when adding many videos, but add complexity—only worth it for 5+ videos at once |
| Manual duplicate checking | Accept videoAlreadyInPlaylist errors | Pre-checking requires listing playlist contents (1 unit + latency), catching error is simpler but fails the operation—depends on use case |

**Installation:**
```bash
# Already installed in go.mod from Phase 1
# No new dependencies required
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── youtube/
│   ├── client.go           # Core client wrapper (exists)
│   ├── playlists.go        # Playlist methods (exists - add CreatePlaylist, AddVideosToPlaylist)
│   ├── subscriptions.go    # Subscription methods (exists)
│   └── search.go           # Search methods (exists)
├── server/
│   ├── server.go           # MCP server (exists)
│   ├── tools_playlists.go  # Playlist-related MCP tools (exists - add create_playlist, add_to_playlist)
│   ├── tools_search.go     # Search-related MCP tools (exists)
│   └── tools_subscriptions.go # Subscription-related MCP tools (exists)
```

### Pattern 1: Creating a Playlist

**What:** Use `PlaylistsService.Insert()` with a `Playlist` resource containing `Snippet` (title, description) and `Status` (privacy).

**When to use:** When user wants to create a new playlist with custom name, description, and privacy settings.

**Example:**
```go
// Source: https://pkg.go.dev/google.golang.org/api/youtube/v3
// Official YouTube Data API v3 Go client pattern

// In internal/youtube/playlists.go
package youtube

import (
    "context"
    "fmt"

    youtube_v3 "google.golang.org/api/youtube/v3"
)

// CreatePlaylist creates a new playlist on the user's YouTube account.
// Quota cost: 50 units per call.
func (c *Client) CreatePlaylist(ctx context.Context, title, description, privacyStatus string) (*Playlist, error) {
    // Validate inputs
    if title == "" {
        return nil, fmt.Errorf("playlist title is required")
    }

    // Default to private if not specified
    if privacyStatus == "" {
        privacyStatus = "private"
    }
    // Validate privacy status
    validPrivacy := map[string]bool{"public": true, "private": true, "unlisted": true}
    if !validPrivacy[privacyStatus] {
        return nil, fmt.Errorf("invalid privacy status: must be public, private, or unlisted")
    }

    // Construct playlist resource
    playlist := &youtube_v3.Playlist{
        Snippet: &youtube_v3.PlaylistSnippet{
            Title:       title,
            Description: description,
        },
        Status: &youtube_v3.PlaylistStatus{
            PrivacyStatus: privacyStatus,
        },
    }

    // Insert playlist
    call := c.service.Playlists.Insert([]string{"snippet", "status"}, playlist)
    resp, err := call.Do()
    if err != nil {
        return nil, fmt.Errorf("failed to create playlist: %w", err)
    }

    // Convert to domain type
    return &Playlist{
        ID:          resp.Id,
        Title:       resp.Snippet.Title,
        Description: resp.Snippet.Description,
        ItemCount:   0, // New playlist has no items
    }, nil
}
```

### Pattern 2: Adding Videos to a Playlist

**What:** Use `PlaylistItemsService.Insert()` with a `PlaylistItem` resource specifying `playlistId` and `resourceId` (videoId).

**When to use:** When user wants to add one or more videos to an existing playlist.

**Example:**
```go
// Source: https://pkg.go.dev/google.golang.org/api/youtube/v3
// Official YouTube Data API v3 Go client pattern

// AddVideosToPlaylist adds videos to an existing playlist.
// Each video insertion costs 50 quota units.
// Returns the number of videos successfully added and any errors encountered.
func (c *Client) AddVideosToPlaylist(ctx context.Context, playlistID string, videoIDs []string) (int, error) {
    if playlistID == "" {
        return 0, fmt.Errorf("playlist ID is required")
    }
    if len(videoIDs) == 0 {
        return 0, fmt.Errorf("at least one video ID is required")
    }

    successCount := 0
    for _, videoID := range videoIDs {
        // Check context cancellation
        if ctx.Err() != nil {
            return successCount, ctx.Err()
        }

        // Construct playlist item
        playlistItem := &youtube_v3.PlaylistItem{
            Snippet: &youtube_v3.PlaylistItemSnippet{
                PlaylistId: playlistID,
                ResourceId: &youtube_v3.ResourceId{
                    Kind:    "youtube#video",
                    VideoId: videoID,
                },
            },
        }

        // Insert item
        call := c.service.PlaylistItems.Insert([]string{"snippet"}, playlistItem)
        _, err := call.Do()
        if err != nil {
            // Check for specific errors
            var apiErr *googleapi.Error
            if errors.As(err, &apiErr) {
                // videoAlreadyInPlaylist is not fatal - skip this video
                if apiErr.Code == 409 || strings.Contains(apiErr.Message, "videoAlreadyInPlaylist") {
                    continue
                }
                // Other errors are failures
                return successCount, fmt.Errorf("failed to add video %s: %w", videoID, err)
            }
            return successCount, fmt.Errorf("failed to add video %s: %w", videoID, err)
        }

        successCount++
    }

    return successCount, nil
}
```

### Pattern 3: Type-Safe MCP Tool Registration (Established Pattern)

**What:** Continue using `mcp.AddTool[In, Out]()` with Go structs for automatic schema generation.

**When to use:** For all MCP tool registrations—consistent with Phase 2 implementation.

**Example:**
```go
// In internal/server/tools_playlists.go

type createPlaylistInput struct {
    Title         string `json:"title" jsonschema:"required,description=Playlist title/name (required)"`
    Description   string `json:"description" jsonschema:"description=Playlist description (optional)"`
    PrivacyStatus string `json:"privacyStatus" jsonschema:"description=Privacy setting (optional - defaults to 'private'),enum=public,enum=private,enum=unlisted"`
}

type playlistCreatedOutput struct {
    PlaylistID  string `json:"playlistId" jsonschema:"description=Unique YouTube playlist ID"`
    Title       string `json:"title" jsonschema:"description=Playlist title"`
    Description string `json:"description" jsonschema:"description=Playlist description"`
    URL         string `json:"url" jsonschema:"description=Direct URL to view the playlist on YouTube"`
}

func (s *Server) registerPlaylistTools() {
    // Existing tools: get_liked_videos, list_playlists, get_playlist_items...

    // NEW Tool: create_playlist
    mcp.AddTool(s.mcpServer, &mcp.Tool{
        Name:        "create_playlist",
        Description: "Create a new playlist on the user's YouTube Music account with custom name, description, and privacy settings. The playlist will appear immediately in YouTube Music. Quota cost: 50 units.",
    }, func(ctx context.Context, req *mcp.CallToolRequest, input createPlaylistInput) (*mcp.CallToolResult, playlistCreatedOutput, error) {
        // Delegate to service layer
        playlist, err := s.ytClient.CreatePlaylist(ctx, input.Title, input.Description, input.PrivacyStatus)
        if err != nil {
            return nil, playlistCreatedOutput{}, err
        }

        // Construct output
        output := playlistCreatedOutput{
            PlaylistID:  playlist.ID,
            Title:       playlist.Title,
            Description: playlist.Description,
            URL:         fmt.Sprintf("https://music.youtube.com/playlist?list=%s", playlist.ID),
        }

        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: fmt.Sprintf("Created playlist '%s' (ID: %s)", playlist.Title, playlist.ID)},
            },
        }, output, nil
    })
}
```

### Pattern 4: Handling Write Operation Errors

**What:** Use type assertions to extract `*googleapi.Error` and check specific error codes for quota exhaustion, validation failures, and duplicate videos.

**When to use:** For all write operations (playlists.insert, playlistItems.insert) to provide clear error messages.

**Example:**
```go
// Source: Error handling pattern from https://pkg.go.dev/google.golang.org/api/googleapi

import (
    "errors"
    "google.golang.org/api/googleapi"
)

_, err := call.Do()
if err != nil {
    var apiErr *googleapi.Error
    if errors.As(err, &apiErr) {
        switch apiErr.Code {
        case 400:
            // Bad request - validation errors
            if strings.Contains(apiErr.Message, "playlistTitleRequired") {
                return fmt.Errorf("playlist title is required")
            }
            if strings.Contains(apiErr.Message, "maxPlaylistExceeded") {
                return fmt.Errorf("channel has reached maximum playlist limit (1000)")
            }
            return fmt.Errorf("invalid request: %s", apiErr.Message)
        case 403:
            // Forbidden or quota exceeded
            if strings.Contains(apiErr.Message, "quotaExceeded") {
                return fmt.Errorf("daily API quota exceeded (10,000 units)")
            }
            return fmt.Errorf("access forbidden: %s", apiErr.Message)
        case 404:
            // Resource not found
            if strings.Contains(apiErr.Message, "playlistNotFound") {
                return fmt.Errorf("playlist not found (may have been deleted)")
            }
            return fmt.Errorf("resource not found: %s", apiErr.Message)
        }
    }
    return fmt.Errorf("API call failed: %w", err)
}
```

### Anti-Patterns to Avoid

- **Not validating privacy status:** The API accepts only "public", "private", or "unlisted"—invalid values fail silently or return errors.
- **Assuming playlists.insert is idempotent:** Each call creates a NEW playlist even with identical title/description—no duplicate checking.
- **Using position field without manual sorting:** Setting `snippet.position` fails if the playlist doesn't use manual sorting—most playlists use automatic ordering.
- **Not handling videoAlreadyInPlaylist errors:** Adding duplicate videos fails with 409 conflict—decide whether to skip silently or surface to user.
- **Exceeding playlist/channel limits:** Channels have ~1,000 playlist max, playlists have 5,000 video max—check limits before batch operations.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Batch HTTP requests for multiple playlistItems.insert | Custom HTTP batching with manual request/response correlation | google-api-go-client batch support (if adding 5+ videos at once) | Batching multiple API calls requires proper boundary formatting, multipart encoding, and response demultiplexing—easy to implement incorrectly and violate API quotas |
| Duplicate video detection before insert | List playlist contents (playlistItems.list) and check for videoID | Catch `videoAlreadyInPlaylist` error and handle gracefully | Pre-checking costs 1 quota unit + network latency for every playlist, scales poorly—error handling is faster and simpler |
| Retry with exponential backoff | Custom retry loop checking error types | `gax.Backoff` with `gensupport.Retry()` (already available via googleapis/gax-go/v2 dependency) | Google APIs return transient errors (503, network timeouts) that need exponential backoff—gax-go handles retryable error detection and proper backoff timings |
| Privacy status validation | String comparisons in multiple places | Centralized const/map validation in service layer | Privacy status appears in multiple contexts (create, update)—centralize to avoid inconsistency |

**Key insight:** Write operations (50 units each) are 50x more expensive than reads (1 unit) but 2x cheaper than search (100 units). Unlike read operations that can paginate freely, write operations should be minimized through input validation, de-duplication at the application layer, and batch operations when adding multiple videos. Never retry non-transient errors (400, 403, 404)—only network failures and 503 Service Unavailable.

## Common Pitfalls

### Pitfall 1: Quota Exhaustion from Write Operations

**What goes wrong:** Creating multiple playlists or adding many videos rapidly exhausts the 10,000 daily quota (50 units per operation = max 200 writes/day).

**Why it happens:** Write operations cost 50 units each—10x more than list operations (5 units) and half the cost of search (100 units). Each `playlists.insert` or `playlistItems.insert` call consumes 50 units regardless of success or failure.

**How to avoid:**
1. Validate inputs BEFORE calling the API (title required, privacy status valid, playlist/video IDs exist)
2. Use batch operations when adding multiple videos (can reduce quota cost)
3. Implement rate limiting for user-facing features (e.g., max 10 playlist creates per session)
4. Monitor quota usage in Google Cloud Console and implement quota warnings
5. Consider requesting quota increase for production use (10,000 can be increased via Google Cloud Console)

**Warning signs:**
- `quotaExceeded (403)` errors
- Rapid quota consumption during testing
- Users creating many playlists in quick succession

**Sources:**
- https://developers.google.com/youtube/v3/determine_quota_cost
- https://developers.google.com/youtube/v3/docs/errors

### Pitfall 2: videoAlreadyInPlaylist Errors

**What goes wrong:** Attempting to add a video that's already in the playlist fails with a 409 conflict error or `videoAlreadyInPlaylist` error.

**Why it happens:** The YouTube Data API is NOT idempotent for `playlistItems.insert`—adding the same video twice is treated as an error. Each playlist item has a unique `playlistItemId` separate from the `videoId`.

**How to avoid:**
1. Catch `videoAlreadyInPlaylist` errors and treat as success (video is in playlist, which is the goal)
2. For batch operations, continue adding remaining videos after encountering duplicates
3. If duplicate checking is critical, list playlist contents first (costs 1 quota unit + latency)
4. Document that duplicate additions are skipped silently (not errors)

**Warning signs:**
- 409 HTTP status codes
- Error messages containing "videoAlreadyInPlaylist"
- Users reporting "failed to add video" when video is actually in the playlist

**Sources:**
- https://developers.google.com/youtube/v3/docs/playlistItems/insert
- https://developers.google.com/youtube/v3/docs/errors

### Pitfall 3: Privacy Status Defaults and Misunderstandings

**What goes wrong:** Users expect playlists to be public by default, but newly created playlists default to "private" if `privacyStatus` is not specified.

**Why it happens:** The YouTube Data API requires explicit `status.privacyStatus` in the request body. If omitted, the API defaults to "private" for safety (prevents accidental public exposure).

**How to avoid:**
1. Always explicitly set `privacyStatus` in playlist creation requests
2. Document the default behavior clearly in tool descriptions
3. Make privacy status a required input parameter (no implicit defaults)
4. Validate privacy status values ("public", "private", "unlisted" only)

**Warning signs:**
- Users complaining playlists aren't visible publicly
- Playlists showing as "private" when user expected "public"
- Invalid privacy status values causing 400 errors

**Sources:**
- https://developers.google.com/youtube/v3/docs/playlists/insert
- https://developers.google.com/youtube/v3/guides/implementation/playlists

### Pitfall 4: Channel and Playlist Limits

**What goes wrong:** Creating more than ~1,000 playlists per channel or adding more than 5,000 videos to a single playlist triggers `maxPlaylistExceeded` or `playlistContainsMaximumNumberOfVideos` errors.

**Why it happens:** YouTube enforces hard limits: approximately 1,000 playlists per channel (exact limit varies) and exactly 5,000 videos per playlist. These are platform limits, not API quota limits.

**How to avoid:**
1. Check current playlist count before creating new playlists (use `list_playlists` tool)
2. Validate video count before adding to playlist (check `itemCount` from playlist details)
3. Return clear error messages when limits are hit (not generic API errors)
4. Document these limits in tool descriptions and user-facing messages

**Warning signs:**
- `maxPlaylistExceeded (400)` errors
- `playlistContainsMaximumNumberOfVideos (403)` errors
- Users reporting inability to create new playlists or add more videos

**Sources:**
- https://www.clrn.org/how-many-playlists-can-you-have-on-youtube/
- https://outofthe925.com/youtube-playlist-limit/
- https://developers.google.com/youtube/v3/docs/errors

### Pitfall 5: Position Field Requires Manual Sorting

**What goes wrong:** Setting `snippet.position` when adding playlist items fails with `manualSortRequired (400)` error if the playlist uses automatic sorting.

**Why it happens:** YouTube playlists can use either automatic sorting (by date added, alphabetical, etc.) or manual sorting (user-defined order). The `snippet.position` field only works for manually sorted playlists.

**How to avoid:**
1. Don't use `snippet.position` unless user explicitly requests manual ordering
2. Most playlists use automatic sorting—let YouTube handle ordering
3. If manual ordering is required, document that playlists must be set to manual sort first
4. Catch `manualSortRequired` errors and provide clear explanation

**Warning signs:**
- `manualSortRequired (400)` errors when adding items
- Users expecting specific video order but seeing chronological order
- Confusion about why videos appear in unexpected order

**Sources:**
- https://developers.google.com/youtube/v3/docs/playlistItems/insert
- https://developers.google.com/youtube/v3/guides/implementation/playlists

## Code Examples

Verified patterns from official sources:

### Creating a Playlist
```go
// Source: https://pkg.go.dev/google.golang.org/api/youtube/v3
// Creates a new playlist with title, description, and privacy status
// Quota cost: 50 units

ctx := context.Background()

playlist := &youtube.Playlist{
    Snippet: &youtube.PlaylistSnippet{
        Title:       "My Awesome Playlist",
        Description: "A collection of my favorite music tracks",
    },
    Status: &youtube.PlaylistStatus{
        PrivacyStatus: "private", // or "public", "unlisted"
    },
}

call := c.service.Playlists.Insert([]string{"snippet", "status"}, playlist)
resp, err := call.Do()
if err != nil {
    return fmt.Errorf("failed to create playlist: %w", err)
}

// resp.Id contains the new playlist ID
// resp.Snippet.Title contains the title
// resp.Status.PrivacyStatus contains the privacy setting
```

### Adding a Single Video to Playlist
```go
// Source: https://pkg.go.dev/google.golang.org/api/youtube/v3
// Adds a video to an existing playlist
// Quota cost: 50 units

ctx := context.Background()

playlistItem := &youtube.PlaylistItem{
    Snippet: &youtube.PlaylistItemSnippet{
        PlaylistId: "PLxxxxxxxxxxxxxx", // Target playlist ID
        ResourceId: &youtube.ResourceId{
            Kind:    "youtube#video",
            VideoId: "dQw4w9WgXcQ", // Video to add
        },
    },
}

call := c.service.PlaylistItems.Insert([]string{"snippet"}, playlistItem)
resp, err := call.Do()
if err != nil {
    // Handle errors
    var apiErr *googleapi.Error
    if errors.As(err, &apiErr) && apiErr.Code == 409 {
        // Video already in playlist - not a fatal error
        return nil
    }
    return fmt.Errorf("failed to add video: %w", err)
}

// resp.Id contains the playlist item ID (NOT the video ID)
// resp.Snippet.Position contains the item's position in playlist
```

### Adding Multiple Videos with Error Handling
```go
// Source: Composite pattern from official YouTube API error docs
// Adds multiple videos to playlist, continuing on duplicates
// Quota cost: 50 units per video

func AddVideosToPlaylist(ctx context.Context, service *youtube.Service, playlistID string, videoIDs []string) (int, error) {
    successCount := 0

    for _, videoID := range videoIDs {
        // Check context cancellation
        if ctx.Err() != nil {
            return successCount, ctx.Err()
        }

        playlistItem := &youtube.PlaylistItem{
            Snippet: &youtube.PlaylistItemSnippet{
                PlaylistId: playlistID,
                ResourceId: &youtube.ResourceId{
                    Kind:    "youtube#video",
                    VideoId: videoID,
                },
            },
        }

        call := service.PlaylistItems.Insert([]string{"snippet"}, playlistItem)
        _, err := call.Do()
        if err != nil {
            var apiErr *googleapi.Error
            if errors.As(err, &apiErr) {
                // Skip duplicates, fail on other errors
                if apiErr.Code == 409 || strings.Contains(apiErr.Message, "videoAlreadyInPlaylist") {
                    continue
                }
                return successCount, fmt.Errorf("failed to add video %s: %w", videoID, err)
            }
            return successCount, fmt.Errorf("failed to add video %s: %w", videoID, err)
        }

        successCount++
    }

    return successCount, nil
}
```

### Error Handling for Quota and Limits
```go
// Source: https://developers.google.com/youtube/v3/docs/errors
// Comprehensive error handling for playlist write operations

import (
    "errors"
    "strings"
    "google.golang.org/api/googleapi"
)

_, err := call.Do()
if err != nil {
    var apiErr *googleapi.Error
    if errors.As(err, &apiErr) {
        switch apiErr.Code {
        case 400:
            // Validation errors
            if strings.Contains(apiErr.Message, "playlistTitleRequired") {
                return fmt.Errorf("playlist title is required")
            }
            if strings.Contains(apiErr.Message, "maxPlaylistExceeded") {
                return fmt.Errorf("channel has reached the maximum number of playlists (1000)")
            }
            if strings.Contains(apiErr.Message, "manualSortRequired") {
                return fmt.Errorf("cannot set position: playlist must use manual sorting")
            }
            return fmt.Errorf("invalid request: %s", apiErr.Message)

        case 403:
            // Forbidden or quota exceeded
            if strings.Contains(apiErr.Message, "quotaExceeded") {
                return fmt.Errorf("daily API quota exceeded (10,000 units)")
            }
            if strings.Contains(apiErr.Message, "playlistContainsMaximumNumberOfVideos") {
                return fmt.Errorf("playlist has reached maximum capacity (5,000 videos)")
            }
            return fmt.Errorf("access forbidden: %s", apiErr.Message)

        case 404:
            // Resource not found
            if strings.Contains(apiErr.Message, "playlistNotFound") {
                return fmt.Errorf("playlist not found or has been deleted")
            }
            if strings.Contains(apiErr.Message, "videoNotFound") {
                return fmt.Errorf("video not found or is private")
            }
            return fmt.Errorf("resource not found: %s", apiErr.Message)

        case 409:
            // Conflict - video already in playlist
            if strings.Contains(apiErr.Message, "videoAlreadyInPlaylist") {
                // Not an error - video is in playlist (goal achieved)
                return nil
            }
            return fmt.Errorf("conflict: %s", apiErr.Message)
        }
    }
    return fmt.Errorf("API call failed: %w", err)
}
```

### MCP Tool Registration with Input Validation
```go
// Source: MCP Go SDK pattern from Phase 2 implementation
// Type-safe tool registration with automatic schema generation

type createPlaylistInput struct {
    Title         string `json:"title" jsonschema:"required,description=Playlist name/title"`
    Description   string `json:"description" jsonschema:"description=Playlist description (optional)"`
    PrivacyStatus string `json:"privacyStatus" jsonschema:"description=Privacy setting (defaults to private),enum=public,enum=private,enum=unlisted"`
}

type createPlaylistOutput struct {
    PlaylistID  string `json:"playlistId" jsonschema:"description=Unique YouTube playlist ID"`
    Title       string `json:"title" jsonschema:"description=Playlist title"`
    URL         string `json:"url" jsonschema:"description=Direct URL to the playlist"`
}

mcp.AddTool(server, &mcp.Tool{
    Name:        "create_playlist",
    Description: "Create a new playlist on the user's YouTube Music account. The playlist will appear immediately in YouTube Music. Quota cost: 50 units.",
}, func(ctx context.Context, req *mcp.CallToolRequest, input createPlaylistInput) (*mcp.CallToolResult, createPlaylistOutput, error) {
    // Input validation
    if input.Title == "" {
        return nil, createPlaylistOutput{}, fmt.Errorf("title is required")
    }

    // Default privacy to private if not specified
    privacy := input.PrivacyStatus
    if privacy == "" {
        privacy = "private"
    }

    // Create playlist via service layer
    playlist, err := ytClient.CreatePlaylist(ctx, input.Title, input.Description, privacy)
    if err != nil {
        return nil, createPlaylistOutput{}, err
    }

    output := createPlaylistOutput{
        PlaylistID: playlist.ID,
        Title:      playlist.Title,
        URL:        fmt.Sprintf("https://music.youtube.com/playlist?list=%s", playlist.ID),
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: fmt.Sprintf("Created playlist '%s' (ID: %s)", playlist.Title, playlist.ID)},
        },
    }, output, nil
})
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Individual playlistItems.insert calls | Batch HTTP requests (when adding 5+ videos) | Available since google-api-go-client initial release | Can reduce quota cost by batching multiple inserts into single HTTP request—up to 50x efficiency improvement |
| Manual privacy status constants | Enum validation in jsonschema tags | MCP Go SDK v1.0 (2025) | MCP tool schemas automatically validate privacy status values, preventing invalid inputs before API calls |
| Silent duplicate failures | Graceful videoAlreadyInPlaylist error handling | Best practice pattern (not API change) | Treat duplicates as success (video is in playlist) rather than failing the operation |
| HTTP 200 on all writes | Proper error code usage (400, 403, 404, 409) | YouTube Data API v3 from inception | Clear error codes enable specific error handling (quota vs validation vs conflict) |

**Deprecated/outdated:**
- **Assuming playlists.insert is idempotent:** Never was—each call creates a new playlist regardless of title
- **Using position field by default:** Requires manual sorting mode which most playlists don't use
- **Pre-checking for duplicate videos:** More expensive than catching the error (1 quota unit + latency)

## Open Questions

1. **Should we support batch operations for adding multiple videos?**
   - What we know: Batch requests can reduce quota cost and latency when adding 5+ videos, but add implementation complexity
   - What's unclear: Whether users will commonly add many videos at once (>5), or if single-video additions are sufficient
   - Recommendation: Start with individual video additions (simpler), add batching if users frequently add 10+ videos per playlist

2. **How should we handle videoAlreadyInPlaylist errors?**
   - What we know: API returns 409 conflict when adding duplicate videos, not idempotent
   - What's unclear: Whether to treat as success (video is in playlist = goal achieved) or surface as error to user
   - Recommendation: Treat as success (skip silently), log for debugging. Document that duplicate additions are ignored.

3. **Should we expose position parameter for manual playlist ordering?**
   - What we know: `snippet.position` only works if playlist uses manual sorting, fails otherwise with `manualSortRequired` error
   - What's unclear: Whether users need manual ordering, or if automatic chronological ordering is sufficient
   - Recommendation: Don't expose position in v1—most playlists use automatic sorting. Add if users request explicit ordering.

4. **What default privacy status should we use?**
   - What we know: API defaults to "private" if not specified, users may expect "public"
   - What's unclear: Whether to match API default ("private") or match user expectations (potentially "public")
   - Recommendation: Make privacy status a REQUIRED parameter (no default)—forces user to make explicit choice, prevents surprises

5. **Should we implement quota budget tracking?**
   - What we know: Write operations cost 50 units each, daily limit is 10,000 units (200 writes max)
   - What's unclear: Whether to track quota usage internally or rely on API errors when quota is exceeded
   - Recommendation: Don't track quota in v1—adds complexity, Google Cloud Console provides quota monitoring. If quota becomes an issue, add warnings in tool descriptions.

## Sources

### Primary (HIGH confidence)
- YouTube Data API v3 Playlists.insert Documentation: https://developers.google.com/youtube/v3/docs/playlists/insert
- YouTube Data API v3 PlaylistItems.insert Documentation: https://developers.google.com/youtube/v3/docs/playlistItems/insert
- YouTube Data API v3 Error Reference: https://developers.google.com/youtube/v3/docs/errors
- YouTube Data API v3 Quota Calculator: https://developers.google.com/youtube/v3/determine_quota_cost
- YouTube Data API v3 Playlist Implementation Guide: https://developers.google.com/youtube/v3/guides/implementation/playlists
- google.golang.org/api/youtube/v3 Package Docs: https://pkg.go.dev/google.golang.org/api/youtube/v3
- MCP Go SDK Package Docs: https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp

### Secondary (MEDIUM confidence)
- YouTube Playlist Limits Guide: https://www.clrn.org/how-many-playlists-can-you-have-on-youtube/
- YouTube Playlist Video Limit: https://outofthe925.com/youtube-playlist-limit/
- Elfsight YouTube API v3 Complete Guide: https://elfsight.com/blog/youtube-data-api-v3-limits-operations-resources-methods-etc/
- YouTube API Quota Guide: https://getlate.dev/blog/youtube-api-limits-how-to-calculate-api-usage-cost-and-fix-exceeded-api-quota

### Tertiary (LOW confidence - verification needed)
- YouTube API Samples Repository (Go examples): https://github.com/youtube/api-samples/tree/master/go
- Batch Requests Discussion: https://www.pythonsos.com/libraries/batch-call-to-playlistitems-insert-youtube-api-v3-with-google-api-python-client/

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using existing libraries from Phase 1 & 2, no new dependencies
- Architecture: HIGH - Patterns verified in official pkg.go.dev documentation and existing codebase
- Pitfalls: HIGH - Mix of official documentation (HIGH) and verified community patterns (MEDIUM)
- Quota costs: HIGH - Official quota calculator and documentation
- Limits: MEDIUM - Community-verified but not in official API docs (1,000 playlist limit, 5,000 video limit)

**Research date:** 2026-02-16
**Valid until:** 2026-03-16 (30 days - stable APIs, unlikely to change rapidly)
