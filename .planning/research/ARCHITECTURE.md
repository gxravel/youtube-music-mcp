# Architecture Research

**Domain:** MCP Server with YouTube Data API v3 Integration
**Researched:** 2026-02-13
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        MCP Host (Claude)                         │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                     MCP Client                              │  │
│  │  (Connects via stdio, handles JSON-RPC 2.0 messages)       │  │
│  └───────────────────────────┬────────────────────────────────┘  │
└────────────────────────────────┼───────────────────────────────────┘
                                 │ stdio transport
                                 │ (stdin/stdout)
┌────────────────────────────────┼───────────────────────────────────┐
│                                ▼                                   │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │              MCP Server Process (Your Code)                  │ │
│  ├──────────────────────────────────────────────────────────────┤ │
│  │                   Transport Layer                            │ │
│  │  ┌────────────────────────────────────────────────────────┐  │ │
│  │  │  StdioTransport (reads stdin, writes stdout)           │  │ │
│  │  │  - Message framing (JSON-RPC 2.0)                      │  │ │
│  │  │  - Serialization/deserialization                       │  │ │
│  │  └────────────────────┬───────────────────────────────────┘  │ │
│  ├───────────────────────┼──────────────────────────────────────┤ │
│  │                       ▼         Protocol Layer               │ │
│  │  ┌──────────────────────────────────────────────────────┐   │ │
│  │  │             MCP Server Core                          │   │ │
│  │  │  - Lifecycle management (initialize, shutdown)       │   │ │
│  │  │  - Capability negotiation                            │   │ │
│  │  │  - Session management                                │   │ │
│  │  │  - Request routing                                   │   │ │
│  │  └────────────┬─────────┬─────────┬─────────────────────┘   │ │
│  ├───────────────┼─────────┼─────────┼─────────────────────────┤ │
│  │               │         │         │      Business Layer      │ │
│  │  ┌────────────▼──┐  ┌──▼─────┐  ┌▼────────────┐             │ │
│  │  │ Tool Handlers │  │ Resour.│  │   Prompts   │             │ │
│  │  │               │  │ Handler│  │   Handlers  │             │ │
│  │  │ - get_history │  │        │  │             │             │ │
│  │  │ - search      │  │        │  │             │             │ │
│  │  │ - create_play │  │        │  │             │             │ │
│  │  └────────┬──────┘  └────────┘  └─────────────┘             │ │
│  ├───────────┼─────────────────────────────────────────────────┤ │
│  │           ▼             Integration Layer                    │ │
│  │  ┌──────────────────────────────────────────────────────┐   │ │
│  │  │         YouTube API Client Manager                   │   │ │
│  │  │  - Service initialization                            │   │ │
│  │  │  - API call wrapping (builder pattern)               │   │ │
│  │  │  - Error handling and retries                        │   │ │
│  │  │  - Response transformation                           │   │ │
│  │  └────────────┬─────────────────────────────────────────┘   │ │
│  ├───────────────┼─────────────────────────────────────────────┤ │
│  │               ▼        Authentication Layer                  │ │
│  │  ┌──────────────────────────────────────────────────────┐   │ │
│  │  │         OAuth2 Token Manager                         │   │ │
│  │  │  - Token acquisition (initial OAuth2 flow)           │   │ │
│  │  │  - Token persistence (file-based storage)            │   │ │
│  │  │  - Token refresh (automatic on expiry)               │   │ │
│  │  │  - Authenticated HTTP client creation                │   │ │
│  │  └──────────────────────────────────────────────────────┘   │ │
│  └──────────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────────┘
                                 │
                                 │ HTTPS (OAuth2 + API calls)
                                 ▼
            ┌─────────────────────────────────────┐
            │   YouTube Data API v3 (Google)      │
            │  - Activities.list (watch history)  │
            │  - Search.list (track search)       │
            │  - Playlists.insert (create)        │
            │  - PlaylistItems.insert (add)       │
            └─────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| **MCP Server Core** | Lifecycle, session management, request routing, capability declaration | `mcp.NewServer()`, tool/resource registration via `mcp.AddTool()` |
| **Transport Layer** | stdio communication, JSON-RPC 2.0 framing, message serialization | `mcp.StdioTransport{}` provided by SDK |
| **Tool Handlers** | MCP tool implementation (get_history, search_tracks, create_playlist) | Go functions matching `CallToolRequest → CallToolResult` signature |
| **YouTube API Client** | YouTube Data API v3 interactions, builder pattern calls, response handling | `youtube.NewService()` with authenticated HTTP client |
| **OAuth2 Token Manager** | Token lifecycle (acquire, persist, refresh), authenticated client creation | `golang.org/x/oauth2` with file-based token cache |

## Recommended Project Structure

```
youtube-music-mcp/
├── cmd/
│   └── youtube-music-mcp/
│       └── main.go              # Entry point: wires components, runs server
├── internal/
│   ├── server/
│   │   ├── server.go            # MCP server setup and registration
│   │   └── handlers.go          # Tool handler implementations
│   ├── youtube/
│   │   ├── client.go            # YouTube API client wrapper
│   │   ├── history.go           # Watch history operations
│   │   ├── search.go            # Search operations
│   │   └── playlist.go          # Playlist operations
│   ├── auth/
│   │   ├── oauth.go             # OAuth2 flow and token management
│   │   └── token_store.go       # Token persistence (file-based)
│   └── types/
│       └── types.go             # Shared types and schemas
├── go.mod
├── go.sum
└── README.md
```

### Structure Rationale

- **cmd/youtube-music-mcp/:** Single binary entry point. `main.go` kept minimal—only dependency wiring, configuration loading, and server startup. Follows Go standard project layout.
- **internal/:** All implementation code is private (not importable by other projects). Enforces encapsulation at compiler level.
- **internal/server/:** MCP-specific code. Isolated from YouTube API details. Makes testing with mock YouTube clients easy.
- **internal/youtube/:** YouTube Data API integration layer. Encapsulates all API calls, builder pattern usage, and response transformation. Can be tested independently with real API or mocks.
- **internal/auth/:** Authentication concerns separated from both MCP and YouTube layers. Token management is complex (refresh, persistence) so it gets its own package.
- **internal/types/:** Shared types avoid circular dependencies. Tool input/output schemas, common data structures.

## Architectural Patterns

### Pattern 1: Layered Architecture with Dependency Inversion

**What:** Core business logic (tool handlers) depends on abstractions (interfaces), not concrete implementations. YouTube client and auth manager implement interfaces.

**When to use:** Always. Critical for testability and future extensibility (e.g., switching from YouTube to Spotify).

**Trade-offs:**
- **Pros:** Easy testing with mocks, clear boundaries, swappable implementations
- **Cons:** Slightly more boilerplate (interface definitions), indirection can obscure flow

**Example:**
```go
// internal/youtube/client.go
type Client interface {
    GetWatchHistory(ctx context.Context, maxResults int64) ([]Video, error)
    SearchTracks(ctx context.Context, query string, limit int) ([]Video, error)
    CreatePlaylist(ctx context.Context, title, description string) (string, error)
    AddToPlaylist(ctx context.Context, playlistID string, videoIDs []string) error
}

type youtubeClient struct {
    service *youtube.Service
}

func NewClient(service *youtube.Service) Client {
    return &youtubeClient{service: service}
}

// internal/server/handlers.go
type ToolHandlers struct {
    ytClient youtube.Client // Depends on interface, not concrete type
}

func (h *ToolHandlers) GetHistoryHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    videos, err := h.ytClient.GetWatchHistory(ctx, 50)
    // Transform to MCP result
}
```

### Pattern 2: OAuth2 TokenSource with Automatic Refresh

**What:** Use `oauth2.Config.TokenSource()` to create a `TokenSource` that automatically refreshes expired tokens. Wrap in custom persistence layer for file-based storage.

**When to use:** Always with OAuth2. Automatic refresh is table stakes.

**Trade-offs:**
- **Pros:** Never manually handle token expiry, Go's oauth2 package handles all refresh logic
- **Cons:** Must implement `oauth2.TokenSource` interface if custom persistence needed

**Example:**
```go
// internal/auth/oauth.go
type TokenManager struct {
    config    *oauth2.Config
    tokenPath string
}

func (tm *TokenManager) GetToken(ctx context.Context) (*oauth2.Token, error) {
    // 1. Try to load from file
    token, err := tm.loadTokenFromFile()
    if err == nil && token.Valid() {
        return token, nil
    }

    // 2. If expired or missing, trigger OAuth2 flow
    token, err = tm.authorizeUser(ctx)
    if err != nil {
        return nil, err
    }

    // 3. Save for next time
    return token, tm.saveTokenToFile(token)
}

func (tm *TokenManager) GetHTTPClient(ctx context.Context) (*http.Client, error) {
    token, err := tm.GetToken(ctx)
    if err != nil {
        return nil, err
    }

    // TokenSource automatically refreshes when token expires
    tokenSource := tm.config.TokenSource(ctx, token)

    // Wrap with callback to persist refreshed tokens
    persistingSource := oauth2.ReuseTokenSource(token, &tokenPersister{
        source:    tokenSource,
        saveFn:    tm.saveTokenToFile,
    })

    return oauth2.NewClient(ctx, persistingSource), nil
}

type tokenPersister struct {
    source oauth2.TokenSource
    saveFn func(*oauth2.Token) error
}

func (tp *tokenPersister) Token() (*oauth2.Token, error) {
    token, err := tp.source.Token()
    if err != nil {
        return nil, err
    }
    // Persist whenever token is refreshed
    tp.saveFn(token)
    return token, nil
}
```

### Pattern 3: Outcome-Oriented Tool Design

**What:** Tools should return complete, actionable results, not raw data dumps. A tool like `get_listening_history` returns formatted, contextualized results, not a JSON blob of API response.

**When to use:** Always when designing MCP tools. MCP servers are "User Interfaces for Agents," not REST APIs.

**Trade-offs:**
- **Pros:** Better agent UX, fewer round-trips, more useful responses
- **Cons:** Requires thinking about what the agent *needs*, not just what the API *provides*

**Example:**
```go
// BAD: Raw data dump
func (h *ToolHandlers) GetHistoryBad(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    videos, _ := h.ytClient.GetWatchHistory(ctx, 100)
    jsonBytes, _ := json.Marshal(videos)
    return &mcp.CallToolResult{
        Content: []mcp.Content{&mcp.TextContent{Text: string(jsonBytes)}},
    }, nil
}

// GOOD: Curated, outcome-oriented
func (h *ToolHandlers) GetHistoryGood(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    limit := 50 // Default
    if val, ok := req.Params.Arguments["limit"].(float64); ok {
        limit = int64(val)
    }

    videos, err := h.ytClient.GetWatchHistory(ctx, limit)
    if err != nil {
        return nil, err
    }

    // Format for readability
    var result strings.Builder
    result.WriteString(fmt.Sprintf("Found %d videos in watch history:\n\n", len(videos)))
    for i, v := range videos {
        result.WriteString(fmt.Sprintf("%d. %s\n", i+1, v.Title))
        result.WriteString(fmt.Sprintf("   Artist: %s | VideoID: %s\n", v.Artist, v.ID))
        result.WriteString(fmt.Sprintf("   Watched: %s\n\n", v.WatchedAt.Format("2006-01-02 15:04")))
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{&mcp.TextContent{Text: result.String()}},
    }, nil
}
```

### Pattern 4: Typed Tool Handlers with Auto-Schema Inference

**What:** Use Go SDK's `mcp.AddTool()` generic function with typed input/output structs. SDK automatically generates JSON schemas from struct tags.

**When to use:** Always. Eliminates manual schema writing and provides compile-time type safety.

**Trade-offs:**
- **Pros:** Type-safe, automatic validation, self-documenting via `jsonschema` tags
- **Cons:** Requires understanding of jsonschema struct tag syntax

**Example:**
```go
// internal/types/types.go
type GetHistoryInput struct {
    Limit int `json:"limit" jsonschema:"minimum=1,maximum=200,description=Number of videos to retrieve"`
}

type GetHistoryOutput struct {
    Videos []HistoryVideo `json:"videos" jsonschema:"description=List of watched videos"`
    Count  int            `json:"count" jsonschema:"description=Total number of videos returned"`
}

type HistoryVideo struct {
    VideoID   string `json:"video_id" jsonschema:"required,description=YouTube video ID"`
    Title     string `json:"title" jsonschema:"required,description=Video title"`
    Artist    string `json:"artist" jsonschema:"description=Artist or channel name"`
    WatchedAt string `json:"watched_at" jsonschema:"description=ISO 8601 timestamp"`
}

// internal/server/handlers.go
func (h *ToolHandlers) RegisterTools(server *mcp.Server) {
    mcp.AddTool(server, &mcp.Tool{
        Name:        "get_listening_history",
        Description: "Retrieve YouTube Music listening history",
    }, h.getHistory)
}

func (h *ToolHandlers) getHistory(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input GetHistoryInput, // Auto-parsed and validated
) (*mcp.CallToolResult, GetHistoryOutput, error) {
    videos, err := h.ytClient.GetWatchHistory(ctx, int64(input.Limit))
    if err != nil {
        return nil, GetHistoryOutput{}, err
    }

    historyVideos := make([]HistoryVideo, len(videos))
    for i, v := range videos {
        historyVideos[i] = HistoryVideo{
            VideoID:   v.ID,
            Title:     v.Title,
            Artist:    v.Artist,
            WatchedAt: v.WatchedAt.Format(time.RFC3339),
        }
    }

    // SDK automatically marshals GetHistoryOutput to JSON content
    return nil, GetHistoryOutput{
        Videos: historyVideos,
        Count:  len(historyVideos),
    }, nil
}
```

## Data Flow

### Request Flow: Tool Invocation

```
[Claude decides to call tool]
    ↓
[MCP Client sends JSON-RPC request via stdio]
    ↓
[StdioTransport deserializes → JSON-RPC 2.0 message]
    ↓
[MCP Server Core routes to tool handler]
    ↓
[Tool Handler extracts arguments]
    ↓
[YouTube Client makes API call]
    ↓
[OAuth2 Manager provides authenticated HTTP client]
    │
    ├─→ [Token valid] → Use existing token
    │
    └─→ [Token expired] → Refresh token → Save refreshed token → Use new token
    ↓
[YouTube Data API v3 returns response]
    ↓
[YouTube Client transforms to internal types]
    ↓
[Tool Handler formats for agent consumption]
    ↓
[MCP Server Core wraps in CallToolResult]
    ↓
[StdioTransport serializes to JSON-RPC response]
    ↓
[MCP Client receives result]
    ↓
[Claude uses result in conversation]
```

### Initial Authentication Flow

```
[Server starts]
    ↓
[OAuth2 Manager attempts to load token from ~/.youtube-music-mcp/token.json]
    │
    ├─→ [Token found & valid] → Create authenticated client → Ready
    │
    ├─→ [Token found & expired] → Refresh token → Save → Ready
    │
    └─→ [No token] → Trigger OAuth2 flow
        ↓
    [Generate authorization URL with PKCE]
        ↓
    [User opens URL in browser]
        ↓
    [User authorizes app]
        ↓
    [Receive authorization code]
        ↓
    [Exchange code for access + refresh tokens]
        ↓
    [Save tokens to ~/.youtube-music-mcp/token.json]
        ↓
    [Create authenticated HTTP client]
        ↓
    [Server ready]
```

### State Management

MCP servers are **stateless per-request** but maintain:

1. **Session State**: MCP Server Core maintains session with client (initialized state, capabilities)
2. **Authentication State**: OAuth2 tokens persisted to disk, loaded on startup, refreshed as needed
3. **No Business State**: Each tool invocation is independent. No caching of YouTube data.

```
Server Lifecycle:
    Initialize → Load Tokens → Register Tools → Run (listen on stdio) → Shutdown

Per-Request:
    Receive → Parse → Validate → Execute → Respond
    (No state carried between requests)
```

### Key Data Flows

1. **Authentication Flow:** Server startup → Token load → Valid? → (Yes: Use) / (No: Refresh or reauth) → Authenticated client ready
2. **Tool Invocation Flow:** MCP request → Handler dispatch → YouTube API call → Response transform → MCP response
3. **Token Refresh Flow:** API call → 401 Unauthorized → Token refresh → Retry API call → Success

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Single user (target) | Monolith is perfect. Single binary, file-based token storage, stdio transport. No database needed. |
| Multi-user (not planned) | Would need: HTTP transport (SSE or Streamable HTTP), database for token storage per user, user-scoped YouTube clients, OAuth flow per user. Significant complexity increase. |
| High request volume | Would need: Request rate limiting (YouTube API has quotas), caching layer for expensive queries (watch history), circuit breaker pattern for API failures. |

### Scaling Priorities

1. **First bottleneck:** YouTube API quota limits (10,000 units/day default). **Fix:** Implement response caching, minimize API calls per tool invocation, request quota increase from Google.
2. **Second bottleneck:** Token refresh race conditions if concurrent requests. **Fix:** Mutex around token refresh, or use `oauth2.ReuseTokenSource` which handles this.

**For this project:** Single-user stdio MCP server. Scaling is not a concern. Keep it simple.

## Anti-Patterns

### Anti-Pattern 1: Exposing Raw API Endpoints as Tools

**What people do:** Create tools that mirror YouTube API methods 1:1, like `activities_list`, `search_list`, `playlists_insert`, each taking raw API parameters.

**Why it's wrong:**
- Forces agent to understand YouTube API details
- Requires multiple round-trips for simple tasks
- Returns unformatted API responses (bad UX for agents)
- Violates MCP design principle: "MCP servers are UIs for agents, not REST API wrappers"

**Do this instead:** Design outcome-oriented tools that solve user problems:
- `get_listening_history(limit)` → Returns formatted, human-readable list
- `search_tracks(query, limit)` → Returns curated search results
- `create_playlist(name, description, video_ids)` → Single call to create and populate playlist

### Anti-Pattern 2: Inline OAuth2 Flow in Tool Handlers

**What people do:** Put OAuth2 authorization logic directly in tool handler functions. Check token validity, trigger refresh, handle errors—all mixed with business logic.

**Why it's wrong:**
- Violates separation of concerns
- Tool handlers become untestable (can't mock OAuth)
- Token refresh logic duplicated across handlers
- Hard to handle token persistence consistently

**Do this instead:**
- OAuth2 in dedicated `auth` package
- Tool handlers receive pre-authenticated `youtube.Client`
- Token refresh transparent to handlers via `oauth2.TokenSource`
- Server initialization fails fast if auth fails (don't defer to first tool call)

### Anti-Pattern 3: Storing Secrets in MCP Configuration

**What people do:** Put client ID, client secret, or refresh tokens in Claude Desktop's `claude_desktop_config.json` or pass as command-line args.

**Why it's wrong:**
- Config files often committed to version control
- Secrets visible in process list (`ps aux`)
- No encryption at rest
- Violates least-privilege principle

**Do this instead:**
- Client ID/Secret: Environment variables or separate config file (`.env`) excluded from git
- Tokens: Encrypted file storage in user's home directory (`~/.youtube-music-mcp/token.json`)
- Permissions: Token file should be `chmod 600` (user read/write only)
- Never log tokens or secrets

### Anti-Pattern 4: No Error Context for Agent

**What people do:** Return raw error messages from YouTube API: `"Error 403: Forbidden"` or generic `"failed to fetch history"`.

**Why it's wrong:**
- Agent can't help user troubleshoot
- No actionable guidance
- Poor UX

**Do this instead:** Return rich error context:
```go
if err != nil {
    if strings.Contains(err.Error(), "forbidden") {
        return nil, fmt.Errorf("YouTube API access forbidden. This usually means:\n"+
            "1. OAuth token expired (try re-authorizing)\n"+
            "2. YouTube Data API not enabled in Google Cloud Console\n"+
            "3. Insufficient OAuth scopes (need youtube.readonly)\n"+
            "Raw error: %v", err)
    }
    return nil, fmt.Errorf("failed to fetch watch history: %w", err)
}
```

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| YouTube Data API v3 | Official Go SDK (`google.golang.org/api/youtube/v3`) with builder pattern | Rate limited (10k units/day). Use `.Fields()` to minimize quota usage. |
| Google OAuth2 | `golang.org/x/oauth2` with file-based token cache | Requires OAuth2 client credentials from Google Cloud Console. Scopes: `youtube.readonly`, `youtube` (for playlists). |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| MCP Server ↔ Tool Handlers | Direct function calls | Handlers are Go functions registered via `mcp.AddTool()`. No RPC, just function invocation. |
| Tool Handlers ↔ YouTube Client | Interface-based | Handlers depend on `youtube.Client` interface, not concrete type. Enables testing with mocks. |
| YouTube Client ↔ OAuth Manager | Authenticated HTTP client | YouTube client receives `*http.Client` from OAuth manager. YouTube SDK handles rest. |
| OAuth Manager ↔ Filesystem | File I/O for token persistence | Tokens stored in `~/.youtube-music-mcp/token.json`. Consider using OS keychain in future. |

## Build Order Implications

Based on component dependencies, recommended build order:

1. **Phase 1: Authentication Foundation**
   - Build `internal/auth/` package
   - Implement OAuth2 flow (authorization URL, code exchange, token persistence)
   - Test with manual token acquisition
   - **Why first:** Nothing works without auth. De-risks hardest part early.

2. **Phase 2: YouTube API Integration**
   - Build `internal/youtube/` package
   - Wrap YouTube Data API SDK
   - Implement `GetWatchHistory()`, `SearchTracks()`, `CreatePlaylist()`, `AddToPlaylist()`
   - Test with authenticated client from Phase 1
   - **Why second:** Validates auth works, discovers API quirks before MCP complexity.

3. **Phase 3: MCP Server Foundation**
   - Build `internal/server/` package
   - Create MCP server with stdio transport
   - Implement lifecycle (initialize, shutdown)
   - Test with MCP Inspector (no tools yet)
   - **Why third:** Proves MCP connection works before adding business logic.

4. **Phase 4: Tool Implementations**
   - Build tool handlers in `internal/server/handlers.go`
   - Connect handlers to YouTube client
   - Define typed input/output schemas in `internal/types/`
   - Test each tool with MCP Inspector
   - **Why fourth:** Brings everything together. Fastest iteration when foundation solid.

5. **Phase 5: Integration & Polish**
   - Wire everything in `cmd/youtube-music-mcp/main.go`
   - Add error handling, logging
   - Test end-to-end with Claude Desktop
   - **Why last:** Integration reveals edge cases. Need all components working individually first.

**Dependency Graph:**
```
OAuth Manager (no dependencies)
    ↓
YouTube Client (depends on OAuth Manager)
    ↓
MCP Server Core (no dependencies, parallel to YouTube Client)
    ↓
Tool Handlers (depends on YouTube Client + MCP Server Core)
    ↓
Main (wires all together)
```

## Sources

**MCP Architecture & Specification:**
- [Architecture overview - Model Context Protocol](https://modelcontextprotocol.io/docs/learn/architecture)
- [Model Context Protocol architecture patterns for multi-agent AI systems](https://developer.ibm.com/articles/mcp-architecture-patterns-ai-systems/)
- [Authorization - Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/authorization)

**Go MCP SDK:**
- [GitHub - modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk)
- [mcp package - github.com/modelcontextprotocol/go-sdk/mcp - Go Packages](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp)
- [GitHub - mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)
- [Model Context Protocol (MCP): Lets Implement an MCP server in Go](https://prasanthmj.github.io/ai/mcp-go/)

**OAuth2 & Token Management:**
- [Understanding OAuth2 and implementing identity-aware MCP servers](https://heeki.medium.com/understanding-oauth2-and-implementing-identity-aware-mcp-servers-221a06b1a6cf)
- [Use MCP OAuth2 Flow to access Quarkus MCP Server](https://quarkus.io/blog/secure-mcp-server-oauth2/)
- [oauth2 package - golang.org/x/oauth2 - Go Packages](https://pkg.go.dev/golang.org/x/oauth2)
- [How to Handle Token Refresh in OAuth2](https://oneuptime.com/blog/post/2026-01-24-oauth2-token-refresh/view)

**YouTube Data API v3:**
- [youtube package - google.golang.org/api/youtube/v3 - Go Packages](https://pkg.go.dev/google.golang.org/api/youtube/v3)
- [Go Quickstart | YouTube Data API](https://developers.google.com/youtube/v3/quickstart/go)
- [YouTube Data API | Google for Developers](https://developers.google.com/youtube/v3)

**Go Architecture Patterns:**
- [How I Structure Services in Go](https://medium.com/@ott.kristian/how-i-structure-services-in-go-19147ad0e6bd)
- [Clean Architecture in Go](https://pkritiotis.io/clean-architecture-in-golang/)
- [GitHub - golang-standards/project-layout](https://github.com/golang-standards/project-layout)

**MCP Best Practices:**
- [MCP is Not the Problem, It's your Server: Best Practices](https://www.philschmid.de/mcp-best-practices)
- [15 Best Practices for Building MCP Servers in Production](https://thenewstack.io/15-best-practices-for-building-mcp-servers-in-production/)
- [The MCP Security Survival Guide](https://towardsdatascience.com/the-mcp-security-survival-guide-best-practices-pitfalls-and-real-world-lessons/)

---
*Architecture research for: YouTube Music MCP Server*
*Researched: 2026-02-13*
