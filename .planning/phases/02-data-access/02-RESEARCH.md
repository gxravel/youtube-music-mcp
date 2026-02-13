# Phase 2: Data Access - Research

**Researched:** 2026-02-13
**Domain:** YouTube Data API v3 integration with MCP tools in Go
**Confidence:** HIGH

## Summary

Phase 2 focuses on implementing data retrieval capabilities for YouTube Music taste data (liked videos, playlists, subscriptions) and search functionality using the YouTube Data API v3. The official YouTube Data API v3 is the only viable option since YouTube Music has no official API. The existing `google.golang.org/api/youtube/v3` Go client provides comprehensive, idiomatic access to all required endpoints with automatic pagination support through the `Pages()` method.

The primary technical challenge is quota management—the default 10,000 units/day quota is consumed rapidly by search operations (100 units each) compared to list operations (1 unit each). Implementation should prioritize efficient quota usage through pagination limits, response field filtering, and avoiding redundant API calls.

MCP tool registration uses the type-safe `mcp.AddTool[In, Out]()` pattern from the official Go SDK, which automatically generates JSON schemas from Go structs, validates inputs, and marshals outputs—eliminating manual schema boilerplate.

**Primary recommendation:** Implement a service layer pattern with methods grouped by resource type (playlists, subscriptions, search) on the YouTube client wrapper. Use the automatic pagination `.Pages()` method for all list operations. Register MCP tools with typed input/output structs for automatic schema generation and validation.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| google.golang.org/api/youtube/v3 | v0.266.0 (installed) | YouTube Data API v3 access | Official Google API client, idiomatic Go patterns, automatic pagination, maintained by Google |
| github.com/modelcontextprotocol/go-sdk/mcp | v1.3.0 (installed) | MCP server implementation | Official MCP Go SDK, maintained with Google collaboration, automatic schema generation |
| github.com/googleapis/gax-go/v2 | v2.17.0 (installed as dependency) | Retry/backoff for Google APIs | Standard Google API auxiliary library, production-grade retry logic |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| google.golang.org/api/googleapi | (part of google.golang.org/api) | Error handling, API call options | Type assertions for `*googleapi.Error`, extracting HTTP status codes |
| github.com/google/jsonschema-go | v0.4.2 (installed as dependency) | Advanced schema customization | Custom validators, type overrides for MCP tool schemas |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| google.golang.org/api/youtube/v3 | Unofficial ytmusicapi (Python) | Python library has YouTube Music-specific features (listening history emulation), but requires calling external process or rewriting in Go—not justified since liked videos/playlists/subscriptions are available via official API |
| Type-safe mcp.AddTool[In, Out] | Raw Server.AddTool with manual schemas | More control over schemas, but loses automatic validation, JSON marshaling, and schema generation—adds boilerplate without benefit |

**Installation:**
```bash
# Already installed in go.mod
go get google.golang.org/api/youtube/v3@v0.266.0
go get github.com/modelcontextprotocol/go-sdk@v1.3.0
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── youtube/
│   ├── client.go           # Core client wrapper (already exists)
│   ├── playlists.go        # Playlist/liked videos methods
│   ├── subscriptions.go    # Subscription methods
│   └── search.go           # Search methods
├── server/
│   ├── server.go           # MCP server (already exists)
│   ├── tools_playlists.go  # Playlist-related MCP tools
│   ├── tools_search.go     # Search-related MCP tools
│   └── tools_subscriptions.go # Subscription-related MCP tools
└── types/
    └── youtube.go          # Shared types for tool inputs/outputs
```

### Pattern 1: Service Layer on YouTube Client

**What:** Extend the `youtube.Client` wrapper with domain methods that encapsulate API logic and quota-efficient patterns.

**When to use:** For all YouTube API operations—keeps business logic separate from MCP tool handlers.

**Example:**
```go
// Source: Repository pattern guidance from https://threedots.tech/post/repository-pattern-in-go/
// Adapted for YouTube API client wrapper

// In internal/youtube/playlists.go
package youtube

import (
    "context"
    "fmt"
)

// GetLikedVideos retrieves the user's liked videos playlist contents.
// Returns video IDs, titles, and thumbnail URLs.
// Quota cost: 1 unit (channels.list) + 1 unit per 50 items (playlistItems.list pagination)
func (c *Client) GetLikedVideos(ctx context.Context, maxResults int64) ([]Video, error) {
    // Step 1: Get the likes playlist ID from channel contentDetails
    channelResp, err := c.service.Channels.
        List([]string{"contentDetails"}).
        Mine(true).
        Do()
    if err != nil {
        return nil, fmt.Errorf("failed to get channel: %w", err)
    }

    if len(channelResp.Items) == 0 {
        return nil, fmt.Errorf("no channel found")
    }

    likesPlaylistID := channelResp.Items[0].ContentDetails.RelatedPlaylists.Likes
    if likesPlaylistID == "" {
        return nil, fmt.Errorf("no likes playlist found")
    }

    // Step 2: Retrieve playlist items using automatic pagination
    var videos []Video
    err = c.service.PlaylistItems.
        List([]string{"snippet"}).
        PlaylistId(likesPlaylistID).
        MaxResults(min(maxResults, 50)). // API max is 50 per page
        Pages(ctx, func(resp *youtube.PlaylistItemListResponse) error {
            for _, item := range resp.Items {
                videos = append(videos, Video{
                    ID:           item.Snippet.ResourceId.VideoId,
                    Title:        item.Snippet.Title,
                    ThumbnailURL: item.Snippet.Thumbnails.Default.Url,
                })
            }
            // Stop pagination if we've hit maxResults
            if int64(len(videos)) >= maxResults {
                return errStopPagination // sentinel error to stop Pages()
            }
            return nil
        })

    if err != nil && err != errStopPagination {
        return nil, fmt.Errorf("failed to get liked videos: %w", err)
    }

    return videos[:min(len(videos), int(maxResults))], nil
}

type Video struct {
    ID           string
    Title        string
    ThumbnailURL string
}

var errStopPagination = fmt.Errorf("stop pagination")
```

### Pattern 2: Type-Safe MCP Tool Registration

**What:** Use `mcp.AddTool[In, Out]()` with Go structs for automatic schema generation and validation.

**When to use:** For all MCP tool registrations—eliminates schema boilerplate and provides compile-time type safety.

**Example:**
```go
// Source: MCP Go SDK documentation from https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp
// Official pattern from https://github.com/modelcontextprotocol/go-sdk

// In internal/types/youtube.go
package types

type GetLikedVideosInput struct {
    MaxResults int64 `json:"maxResults" jsonschema:"description=Maximum number of videos to return (default 50),minimum=1,maximum=500"`
}

type VideoInfo struct {
    ID           string `json:"id" jsonschema:"description=YouTube video ID"`
    Title        string `json:"title" jsonschema:"description=Video title"`
    ThumbnailURL string `json:"thumbnailUrl" jsonschema:"description=Video thumbnail URL"`
}

type GetLikedVideosOutput struct {
    Videos []VideoInfo `json:"videos"`
    Count  int         `json:"count" jsonschema:"description=Number of videos returned"`
}

// In internal/server/tools_playlists.go
package server

import (
    "context"

    "github.com/gxravel/youtube-music-mcp/internal/types"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerPlaylistTools() {
    // AddTool automatically:
    // 1. Generates input schema from GetLikedVideosInput (with jsonschema tags)
    // 2. Generates output schema from GetLikedVideosOutput
    // 3. Validates input against schema before calling handler
    // 4. Marshals output to StructuredContent
    mcp.AddTool(s.mcpServer, &mcp.Tool{
        Name:        "get_liked_videos",
        Description: "Retrieve user's liked videos from YouTube",
    }, s.handleGetLikedVideos)
}

func (s *Server) handleGetLikedVideos(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input types.GetLikedVideosInput,
) (*mcp.CallToolResult, types.GetLikedVideosOutput, error) {
    // Input is already validated and typed
    maxResults := input.MaxResults
    if maxResults == 0 {
        maxResults = 50
    }

    // Delegate to service layer
    videos, err := s.ytClient.GetLikedVideos(ctx, maxResults)
    if err != nil {
        // Return error - MCP SDK automatically sets IsError=true
        return nil, types.GetLikedVideosOutput{}, err
    }

    // Convert to output type
    output := types.GetLikedVideosOutput{
        Videos: make([]types.VideoInfo, len(videos)),
        Count:  len(videos),
    }
    for i, v := range videos {
        output.Videos[i] = types.VideoInfo{
            ID:           v.ID,
            Title:        v.Title,
            ThumbnailURL: v.ThumbnailURL,
        }
    }

    // Return result with structured output (automatically marshaled)
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{
                Text: fmt.Sprintf("Retrieved %d liked videos", len(videos)),
            },
        },
    }, output, nil
}
```

### Pattern 3: Automatic Pagination with Pages()

**What:** Use the `.Pages(ctx, func(*Response) error)` method for automatic pagination instead of manual NextPageToken loops.

**When to use:** For all list operations (playlists, playlistItems, subscriptions, search) that may return multiple pages.

**Example:**
```go
// Source: Official google.golang.org/api patterns from https://pkg.go.dev/google.golang.org/api/youtube/v3
// Pattern confirmed at https://tales.mbivert.com/on-youtube-api-golang/

// GOOD: Automatic pagination with Pages()
var allPlaylists []*youtube.Playlist
err := c.service.Playlists.
    List([]string{"snippet"}).
    Mine(true).
    MaxResults(50).
    Pages(ctx, func(resp *youtube.PlaylistListResponse) error {
        allPlaylists = append(allPlaylists, resp.Items...)
        return nil // Continue to next page
    })

// BAD: Manual pagination (more code, easy to get wrong)
var allPlaylists []*youtube.Playlist
nextPageToken := ""
for {
    call := c.service.Playlists.List([]string{"snippet"}).Mine(true).MaxResults(50)
    if nextPageToken != "" {
        call = call.PageToken(nextPageToken)
    }
    resp, err := call.Do()
    if err != nil {
        return nil, err
    }
    allPlaylists = append(allPlaylists, resp.Items...)
    if resp.NextPageToken == "" {
        break
    }
    nextPageToken = resp.NextPageToken
}
```

### Pattern 4: Quota-Efficient Field Filtering

**What:** Use the `Fields()` method to request only needed fields, reducing bandwidth and avoiding unnecessarily detailed responses.

**When to use:** When you only need specific fields from complex response objects (especially for search results).

**Example:**
```go
// Source: Quota optimization guidance from https://elfsight.com/blog/youtube-data-api-v3-limits-operations-resources-methods-etc/

// Request only snippet (not contentDetails, statistics, etc.)
// Reduces response size and processing time
call := c.service.Search.
    List([]string{"snippet"}). // Only request snippet part
    Q(query).
    Type("video").
    MaxResults(25)

// Advanced: Further filter to specific fields within snippet
call = call.Fields("items(id/videoId,snippet/title,snippet/thumbnails/default/url)")
```

### Anti-Patterns to Avoid

- **Calling search.list when list operations suffice:** Search costs 100 units vs 1 unit for list operations. Use playlists.list, playlistItems.list, subscriptions.list when possible.
- **Manual JSON schema generation:** Don't hand-write JSON schemas when `mcp.AddTool[In, Out]` with struct tags can auto-generate them.
- **Ignoring pagination limits:** Don't assume all results fit in one page. Always handle pagination or use `.Pages()`.
- **Manual NextPageToken loops:** Error-prone and verbose. Use `.Pages()` method for automatic pagination.
- **Trusting totalResults for allocation:** The `totalResults` field is unreliable and changes between calls—never use it for pre-allocation.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Retry with exponential backoff | Custom retry loop checking error types | `gax.Backoff` with `gensupport.Retry()` (available via googleapis/gax-go/v2 dependency) | Google APIs return transient errors (503, network timeouts) that need exponential backoff; gax-go encodes which errors are retryable and proper backoff timings (20ms initial, 32s max, 1.3 multiplier) |
| JSON schema generation for MCP tools | Manual JSON Schema objects | `mcp.AddTool[In, Out]()` with struct `jsonschema` tags | MCP SDK's `github.com/google/jsonschema-go` automatically generates schemas from Go types, validates inputs, and marshals outputs—eliminates 50+ lines of boilerplate per tool |
| Pagination logic | Manual NextPageToken tracking with for loops | `.Pages(ctx, func(*Response) error)` method on all ListCalls | Google API client automatically handles NextPageToken, provides clean callback interface, stops on error or when NextPageToken is empty—manual loops are bug-prone (easy to miss edge cases like missing NextPageToken) |
| OAuth2 token refresh | Custom token expiration checking and refresh calls | `oauth2.Config.TokenSource()` with `PersistingTokenSource` wrapper (already in Phase 1) | Token refresh is complex (race conditions, concurrent refresh attempts, expiration edge cases)—oauth2 library handles this correctly with automatic refresh via http.Client |

**Key insight:** YouTube Data API quota is the primary constraint. Prevent quota exhaustion by using list operations over search when possible (100x quota savings), limiting MaxResults, and caching results at the application layer (MCP clients may call tools repeatedly). Never build custom retry logic—Google's APIs have specific transient error codes that gax-go already handles correctly.

## Common Pitfalls

### Pitfall 1: Pagination Edge Case - maxResults > totalResults

**What goes wrong:** When `maxResults` exceeds `totalResults`, the API may return fewer items than requested and omit `nextPageToken` even though you expected pagination to continue.

**Why it happens:** API documentation states: "If maxResults > totalResults, you do not get all the data and nextPageToken is not provided." For example, if totalResults is 25 and maxResults is 100, you get 25 items with no nextPageToken.

**How to avoid:**
1. Use `.Pages()` method which correctly handles missing `nextPageToken`
2. Set reasonable `maxResults` values (25-50, max allowed is 50)
3. Check `len(resp.Items)` not `totalResults` to determine if you got all data
4. Never rely on `totalResults` for allocation—it's unreliable and changes between calls

**Warning signs:**
- Receiving fewer items than `maxResults` without a `nextPageToken`
- `totalResults` changing value between pagination calls
- Source: https://github.com/youtube/api-samples/issues/500

### Pitfall 2: Search Quota Exhaustion

**What goes wrong:** Running out of daily quota (10,000 units) after only 100 search requests (100 units each).

**Why it happens:** Search operations (100 units) are 100x more expensive than list operations (1 unit). Pagination multiplies this cost—each page is a separate 100-unit request.

**How to avoid:**
1. Use list operations (playlists.list, playlistItems.list, subscriptions.list) instead of search when possible
2. Implement application-layer caching for search results (TTL: 1-24 hours depending on use case)
3. Limit search results with `MaxResults` and pagination—don't fetch all pages if client only needs top results
4. Monitor quota usage in Google Cloud Console's Quotas page
5. Consider requesting quota increase for production use (default 10,000 can be increased via Google Cloud Console)

**Warning signs:**
- Quota exceeded errors (403 with `quotaExceeded` reason)
- Rapid quota consumption during testing
- Multiple search calls for the same query
- Sources: https://getlate.dev/blog/youtube-api-limits-how-to-calculate-api-usage-cost-and-fix-exceeded-api-quota, https://developers.google.com/youtube/v3/determine_quota_cost

### Pitfall 3: Missing Liked Videos Playlist

**What goes wrong:** Calling `playlistItems.list` with a hardcoded "liked videos" playlist ID fails because liked videos playlists are user-specific.

**Why it happens:** Each channel has a unique liked videos playlist ID stored in `channel.contentDetails.relatedPlaylists.likes`. There's no universal "LL" prefix or hardcoded ID.

**How to avoid:**
1. Always fetch the channel's `contentDetails` first to get the `relatedPlaylists.likes` ID
2. Call `channels.list` with `mine=true` and `part="contentDetails"`
3. Extract `channel.ContentDetails.RelatedPlaylists.Likes` before calling `playlistItems.list`
4. Cache the likes playlist ID per user session (it rarely changes)

**Warning signs:**
- "Playlist not found" errors when using hardcoded playlist IDs
- Code assumes a standard "LL" prefix exists
- Sources: https://youtube-data-api.readthedocs.io/en/latest/youtube_api.html, https://developers.google.com/youtube/v3/guides/implementation/playlists

### Pitfall 4: Context Deadline Exceeded Without Cleanup

**What goes wrong:** Long-running pagination operations hit context deadlines, but resources aren't cleaned up properly, leading to goroutine leaks or partial results.

**Why it happens:** The `.Pages()` callback doesn't automatically cancel when context deadline is exceeded—it relies on the callback returning an error or the API call timing out.

**How to avoid:**
1. Always check `ctx.Err()` at the start of pagination callbacks
2. Set reasonable context timeouts based on operation (e.g., 30s for single-page list, 2min for multi-page pagination)
3. Use `defer cancel()` immediately after creating context with timeout
4. Return early from callbacks if context is cancelled

**Warning signs:**
- "context deadline exceeded" errors in logs
- Partial results returned without error indication
- Goroutine count increasing over time
- Source: https://uptrace.dev/glossary/context-deadline-exceeded

```go
// GOOD: Check context in pagination callback
ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
defer cancel()

err := c.service.PlaylistItems.List([]string{"snippet"}).
    PlaylistId(playlistID).
    Pages(ctx, func(resp *youtube.PlaylistItemListResponse) error {
        // Check context before processing
        if ctx.Err() != nil {
            return ctx.Err()
        }
        // Process items...
        return nil
    })
```

### Pitfall 5: YouTube Music Search Limitations

**What goes wrong:** Expecting to filter search results to only "official music" tracks, but the YouTube Data API v3 has no such filter.

**Why it happens:** YouTube Music has no official API. The YouTube Data API v3 doesn't distinguish between official music videos, user uploads, covers, or other video types beyond basic category filters.

**How to avoid:**
1. Document this limitation for users: search returns all video types, not just official music
2. Use `videoCategoryId=10` to filter to "Music" category (not perfect—includes unofficial uploads)
3. Use topic filters like `topicId=/m/04rlf` (music parent topic) for broader music content
4. Consider search query patterns like including "official audio" or "official video" in query string
5. Accept that perfect music-only filtering requires unofficial ytmusicapi (Python) which this project explicitly avoids

**Warning signs:**
- User expectations for "official music only" results
- Search returning covers, live performances, remixes when user wanted studio recordings
- Sources: https://developers.google.com/youtube/v3/docs/search/list, https://musicfetch.io/services/youtube-music/api

## Code Examples

Verified patterns from official sources:

### Getting User's Playlists
```go
// Source: https://pkg.go.dev/google.golang.org/api/youtube/v3
// Retrieves all playlists owned by the authenticated user
// Quota cost: 1 unit per 50 playlists

ctx := context.Background()
var playlists []*youtube.Playlist

err := c.service.Playlists.
    List([]string{"snippet", "contentDetails"}).
    Mine(true).
    MaxResults(50).
    Pages(ctx, func(resp *youtube.PlaylistListResponse) error {
        playlists = append(playlists, resp.Items...)
        return nil
    })

if err != nil {
    return fmt.Errorf("failed to get playlists: %w", err)
}

// Each playlist has:
// - Id: Unique playlist identifier
// - Snippet.Title: Playlist name
// - Snippet.Description: Playlist description
// - ContentDetails.ItemCount: Number of videos in playlist
```

### Getting Playlist Contents
```go
// Source: https://pkg.go.dev/google.golang.org/api/youtube/v3
// Retrieves all videos from a specific playlist
// Quota cost: 1 unit per 50 videos

ctx := context.Background()
var items []*youtube.PlaylistItem

err := c.service.PlaylistItems.
    List([]string{"snippet", "contentDetails"}).
    PlaylistId(playlistID).
    MaxResults(50).
    Pages(ctx, func(resp *youtube.PlaylistItemListResponse) error {
        items = append(items, resp.Items...)
        return nil
    })

if err != nil {
    return fmt.Errorf("failed to get playlist items: %w", err)
}

// Each item has:
// - Snippet.ResourceId.VideoId: YouTube video ID
// - Snippet.Title: Video title
// - Snippet.Thumbnails: Thumbnail images (default, medium, high, standard, maxres)
// - ContentDetails.VideoPublishedAt: When video was published
```

### Getting User's Subscriptions
```go
// Source: https://pkg.go.dev/google.golang.org/api/youtube/v3
// Retrieves all channels the user is subscribed to
// Quota cost: 1 unit per 50 subscriptions

ctx := context.Background()
var subscriptions []*youtube.Subscription

err := c.service.Subscriptions.
    List([]string{"snippet"}).
    Mine(true).
    MaxResults(50).
    Pages(ctx, func(resp *youtube.SubscriptionListResponse) error {
        subscriptions = append(subscriptions, resp.Items...)
        return nil
    })

if err != nil {
    return fmt.Errorf("failed to get subscriptions: %w", err)
}

// Each subscription has:
// - Snippet.ResourceId.ChannelId: Subscribed channel ID
// - Snippet.Title: Channel name
// - Snippet.Description: Channel description
// - Snippet.Thumbnails: Channel thumbnail
```

### Searching for Videos
```go
// Source: https://developers.google.com/youtube/v3/docs/search/list
// Searches for videos matching a query
// Quota cost: 100 units per page (expensive!)

ctx := context.Background()

// Limit to first page only to conserve quota
resp, err := c.service.Search.
    List([]string{"snippet"}).
    Q(query).
    Type("video").
    VideoCategoryId("10"). // Music category filter
    MaxResults(25).
    Do()

if err != nil {
    return nil, fmt.Errorf("search failed: %w", err)
}

// Process results
var videos []SearchResult
for _, item := range resp.Items {
    videos = append(videos, SearchResult{
        VideoID:      item.Id.VideoId,
        Title:        item.Snippet.Title,
        Description:  item.Snippet.Description,
        ChannelTitle: item.Snippet.ChannelTitle,
        ThumbnailURL: item.Snippet.Thumbnails.Default.Url,
    })
}

// IMPORTANT: Each additional page costs 100 units
// Only paginate if absolutely necessary:
// if resp.NextPageToken != "" {
//     nextPageCall := c.service.Search.List([]string{"snippet"}).
//         Q(query).Type("video").PageToken(resp.NextPageToken)
//     // Another 100 units...
// }
```

### Checking if Video Exists
```go
// Source: https://developers.google.com/youtube/v3/docs/videos/list
// Verifies a video exists and retrieves basic info
// Quota cost: 1 unit

ctx := context.Background()

resp, err := c.service.Videos.
    List([]string{"snippet"}).
    Id(videoID).
    Do()

if err != nil {
    var apiErr *googleapi.Error
    if errors.As(err, &apiErr) && apiErr.Code == 404 {
        return false, nil // Video doesn't exist
    }
    return false, fmt.Errorf("failed to check video: %w", err)
}

if len(resp.Items) == 0 {
    return false, nil // Video doesn't exist or is private
}

// Video exists and is accessible
return true, nil
```

### MCP Tool with Struct Tags for Schema
```go
// Source: https://github.com/modelcontextprotocol/go-sdk (official examples)
// Automatic schema generation from Go types

type SearchInput struct {
    Query      string `json:"query" jsonschema:"required,description=Search query for YouTube videos"`
    MaxResults int    `json:"maxResults" jsonschema:"description=Maximum results to return (1-25),minimum=1,maximum=25,default=10"`
    Category   string `json:"category" jsonschema:"description=Video category filter (optional),enum=music,enum=entertainment,enum=education"`
}

type SearchOutput struct {
    Videos     []VideoInfo `json:"videos"`
    Query      string      `json:"query"`
    TotalFound int         `json:"totalFound"`
}

type VideoInfo struct {
    ID           string `json:"id"`
    Title        string `json:"title"`
    ChannelTitle string `json:"channelTitle"`
    Description  string `json:"description"`
}

// Register tool with automatic schema generation
mcp.AddTool(server, &mcp.Tool{
    Name:        "search_videos",
    Description: "Search YouTube for music videos and tracks",
}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
    // Input is validated against generated schema
    // Output is automatically marshaled to StructuredContent

    videos, err := ytClient.SearchVideos(ctx, input.Query, input.MaxResults)
    if err != nil {
        return nil, SearchOutput{}, err
    }

    output := SearchOutput{
        Videos:     convertVideos(videos),
        Query:      input.Query,
        TotalFound: len(videos),
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: fmt.Sprintf("Found %d videos", len(videos))},
        },
    }, output, nil
})
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual NextPageToken loops | `.Pages(ctx, callback)` method | Present since google.golang.org/api initial release | Eliminates 10+ lines of pagination boilerplate per operation, handles edge cases automatically |
| Manual JSON Schema objects for MCP tools | `mcp.AddTool[In, Out]()` with struct tags | MCP Go SDK v1.0 (2025) | Reduces tool registration from ~60 lines to ~15 lines, provides compile-time type safety |
| No retry logic or custom implementations | `gax.Backoff` with `gensupport.Retry()` | Available via googleapis/gax-go/v2 (dependency of google.golang.org/api) | Production-grade exponential backoff (20ms-32s) with jitter, handles transient Google API errors correctly |
| Hard-coded playlist IDs for liked videos | Dynamic retrieval via `channels.list` contentDetails | Always required (user-specific IDs) | No universal liked videos playlist ID exists—must fetch per user |
| YouTube Music unofficial scraping | YouTube Data API v3 (official) | Ongoing (no official YouTube Music API exists) | Limited to public API capabilities (liked videos, playlists, subscriptions)—no listening history or music-specific recommendations |

**Deprecated/outdated:**
- **Manual pageToken loops:** All `*ListCall` types have `.Pages()` method—no reason to manually track NextPageToken
- **YouTube Data API v2:** Shut down in 2015, replaced by v3
- **Hardcoded "LL" playlist prefix for likes:** Never existed—each user has a unique likes playlist ID

## Open Questions

1. **Should we implement application-layer caching for search results?**
   - What we know: Search costs 100 units (10% of daily quota per call), results change slowly for most queries
   - What's unclear: Cache invalidation strategy (TTL? user-triggered refresh?), storage mechanism (in-memory? persistent?)
   - Recommendation: Start without caching, add if quota becomes an issue in practice. If added, use simple in-memory TTL cache (5-60 minutes) via `patrickmn/go-cache` or similar.

2. **What maxResults defaults should we use for MCP tools?**
   - What we know: API max is 50, each page costs quota (1 unit for lists, 100 for search), users may want different limits
   - What's unclear: Balance between comprehensive results and quota efficiency
   - Recommendation: Default to 25 for lists, 10 for search. Make maxResults an optional input parameter with these defaults. Document quota costs in tool descriptions.

3. **Should we expose pagination to MCP tool callers?**
   - What we know: MCP tools are called by AI agents who may not understand pagination, but some operations return hundreds/thousands of items
   - What's unclear: Whether to expose pageToken in tool inputs or always return all results (up to maxResults)
   - Recommendation: Don't expose pageToken—use maxResults limit and retrieve up to that many items automatically. For very large datasets (500+ items), consider adding a separate "list next page" tool if needed.

4. **How to handle "verify track exists" without search quota cost?**
   - What we know: `search.list` costs 100 units, `videos.list` costs 1 unit, but videos.list requires knowing the exact video ID
   - What's unclear: What "verify track exists" means—verify by ID (1 unit) or search by name/artist and verify existence (100 units)?
   - Recommendation: Implement two tools: `verify_video_by_id` (videos.list, 1 unit) and `search_videos` (search.list, 100 units). Clarify in descriptions that search is expensive.

## Sources

### Primary (HIGH confidence)
- YouTube Data API v3 Documentation: https://developers.google.com/youtube/v3/docs
- YouTube Data API v3 Quota Calculator: https://developers.google.com/youtube/v3/determine_quota_cost
- YouTube Data API v3 Pagination Guide: https://developers.google.com/youtube/v3/guides/implementation/pagination
- google.golang.org/api/youtube/v3 Package Docs: https://pkg.go.dev/google.golang.org/api/youtube/v3
- MCP Go SDK Package Docs: https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp
- MCP Go SDK GitHub (official): https://github.com/modelcontextprotocol/go-sdk
- googleapis/gax-go/v2 Package Docs: https://pkg.go.dev/github.com/googleapis/gax-go/v2

### Secondary (MEDIUM confidence)
- YouTube API Complete Guide 2026: https://getlate.dev/blog/youtube-api
- YouTube API Quota Guide 2026: https://getlate.dev/blog/youtube-api-limits-how-to-calculate-api-usage-cost-and-fix-exceeded-api-quota
- Elfsight YouTube API v3 Guide: https://elfsight.com/blog/youtube-data-api-v3-limits-operations-resources-methods-etc/
- Go Repository Pattern Guide: https://threedots.tech/post/repository-pattern-in-go/
- MCP Go Server Tutorial: https://medium.com/@xcoulon/writing-your-first-mcp-server-with-the-go-sdk-62fada87e5eb
- Tales on YouTube API with Go: https://tales.mbivert.com/on-youtube-api-golang/
- Context Timeout Handling in Go: https://uptrace.dev/glossary/context-deadline-exceeded

### Tertiary (LOW confidence - validation needed)
- GitHub Issue: Search maxResults edge case: https://github.com/youtube/api-samples/issues/500 (confirms edge case but not official docs)
- Phyllo YouTube API Limits Guide: https://www.getphyllo.com/post/youtube-api-limits-how-to-calculate-api-usage-cost-and-fix-exceeded-api-quota (third-party but corroborates official quota info)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official Google and MCP libraries, versions confirmed in go.mod
- Architecture: HIGH - Patterns verified in official pkg.go.dev documentation and GitHub examples
- Pitfalls: MEDIUM-HIGH - Mix of official documentation (HIGH) and community-reported issues (MEDIUM)
- Quota costs: HIGH - Official quota calculator and documentation
- YouTube Music limitations: HIGH - Confirmed via official docs (no YouTube Music API exists)

**Research date:** 2026-02-13
**Valid until:** 2026-03-13 (30 days - stable APIs, unlikely to change rapidly)
