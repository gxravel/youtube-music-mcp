# Project Research Summary

**Project:** YouTube Music MCP Server
**Domain:** Music Recommendation MCP Server
**Researched:** 2026-02-13
**Confidence:** MEDIUM-HIGH

## Executive Summary

This project aims to build an MCP server for YouTube Music recommendations using Go, the official YouTube Data API v3, and Claude's AI capabilities. Research reveals a critical constraint: **YouTube Music has no official API for accessing listening history**. The YouTube Data API v3 can handle playlist creation and management but cannot retrieve playback history. This fundamentally limits the project scope—the original vision of analyzing listening history to generate AI-powered recommendations is not achievable through official APIs.

The recommended approach is to **scope the project to playlist management and track search only**, using the official YouTube Data API v3 with OAuth2 authentication. Build an MCP server using Go 1.26+ and the official modelcontextprotocol/go-sdk (v1.3.0+) with stdio transport. Focus on tools that work without listening history: search tracks, create playlists, manage playlist items. The alternative—using unofficial ytmusicapi with browser cookies—violates YouTube's Terms of Service and creates long-term reliability and compliance risks.

Key risks include quota exhaustion (10,000 units/day default with expensive write operations), OAuth2 token refresh complexity (refresh token rotation can invalidate auth), and MCP stdio protocol pollution (any stdout logging breaks JSON-RPC communication). These are all addressable with proper architecture: quota tracking middleware, automatic token refresh with persistence, and strict stderr-only logging configuration from the start.

## Key Findings

### Recommended Stack

Go provides the best foundation for this MCP server with official SDKs for both MCP (modelcontextprotocol/go-sdk) and YouTube API (google.golang.org/api/youtube/v3). The stack is production-ready with minimal dependencies: Go 1.26+ for latest stdlib improvements, official MCP SDK v1.3.0+ supporting spec 2025-11-25, YouTube Data API v3 client v0.266.0+, and golang.org/x/oauth2 for Google authentication.

**Core technologies:**
- **Go 1.26+**: Runtime and language — latest stable with log/slog, best performance and security
- **modelcontextprotocol/go-sdk v1.3.0+**: Official MCP server SDK — maintained by Anthropic+Google, 819 projects depend on it
- **google.golang.org/api/youtube/v3 v0.266.0+**: YouTube Data API v3 client — Google's official, provides playlists, search, activities
- **golang.org/x/oauth2/google v0.35.0+**: OAuth2 authentication — standard library extension, security-vetted
- **log/slog**: Structured logging (stdlib) — zero dependencies, JSON/text handlers, context support
- **hashicorp/go-retryablehttp v0.7.7+**: HTTP retry with backoff — handles 429 rate limits, battle-tested

**Critical version requirements:**
- Go 1.21+ minimum for log/slog support; use 1.26 for latest improvements
- MCP SDK v1.3.0+ for backward compatibility with multiple MCP spec versions
- Stdio transport (not SSE, which is deprecated)

### Expected Features

Research reveals a fundamental gap between expected features and API capabilities. The core differentiator—analyzing listening history for AI-powered recommendations—is not possible with official APIs.

**Must have (table stakes):**
- **Search/verify tracks exist** — validate tracks are on YouTube Music before suggesting (LOW complexity, ytmusicapi provides)
- **Create playlists** — fundamental output format; recommendations delivered as playlists (LOW complexity)
- **Add tracks to playlists** — basic CRUD operations (LOW complexity)
- **OAuth2 authentication** — required for API access (MEDIUM complexity, token refresh tricky)
- **Basic error handling** — API failures, network issues, rate limits inevitable (MEDIUM complexity)
- **Respect rate limits** — YouTube has quota system, must track and throttle (MEDIUM complexity)

**Should have (competitive advantage, but requires listening history which is unavailable):**
- **AI-powered track verification** — Claude validates recommendations using music knowledge (feasible without history)
- **Mood/context-based recommendations** — "chill work music" queries (feasible but less personalized without history)
- **Deep listening history analysis** — NOT POSSIBLE with official API
- **Hidden gem discovery from favorite artists** — NOT POSSIBLE without history to identify favorites

**Defer (v2+):**
- **Temporal pattern detection** — requires history data (HIGH complexity, not available)
- **Collection gap analysis** — needs genre taxonomy + history (HIGH complexity)
- **Anti-mainstream filtering with external data** — requires external API integration (HIGH complexity)

**Critical architectural decision:** Accept that listening history is unavailable and rescope to playlist-centric tools, OR use unofficial ytmusicapi (violates ToS, high risk), OR wait for official YouTube Music API (timeline unknown).

### Architecture Approach

The architecture follows standard MCP server patterns: layered design with dependency inversion, stdio transport, OAuth2 token source with automatic refresh, and outcome-oriented tool design. The system has five layers: Transport (stdio JSON-RPC), Protocol (MCP Server Core), Business (Tool Handlers), Integration (YouTube API Client), and Authentication (OAuth2 Token Manager).

**Major components:**
1. **MCP Server Core** — lifecycle management, capability negotiation, request routing; uses mcp.NewServer() with tool registration
2. **Tool Handlers** — business logic for get_history (if unofficial API used), search_tracks, create_playlist, add_to_playlist; outcome-oriented responses
3. **YouTube API Client** — wraps YouTube Data API v3 with builder pattern, response transformation, retry logic; implements interface for testability
4. **OAuth2 Token Manager** — token acquisition via web flow, file-based persistence (~/.youtube-music-mcp/token.json), automatic refresh with rotation handling

**Key architectural patterns:**
- **Dependency inversion**: Tool handlers depend on youtube.Client interface, not concrete implementations (enables testing)
- **TokenSource pattern**: Use oauth2.Config.TokenSource() for automatic refresh with custom persistence wrapper
- **Typed tool handlers**: Use MCP SDK's auto-schema inference from Go structs with jsonschema tags
- **Outcome-oriented tools**: Return formatted, actionable results (not raw API dumps); "User Interfaces for Agents"

**Project structure:** Follow Go standard layout—cmd/ for entry point, internal/ for private implementation (server/, youtube/, auth/, types/ packages), go.mod for dependencies.

### Critical Pitfalls

Research identified seven critical pitfalls that can derail the project:

1. **Assuming YouTube Music has official API support** — The YouTube Data API v3 activities.list endpoint does NOT return music listening history. This must be validated in Phase 0 to avoid building impossible features. Prevention: Accept API limitations upfront, scope to playlist management only, or explicitly document use of unofficial ytmusicapi with ToS warnings.

2. **Ignoring quota costs leading to daily lockout** — Default 10,000 units/day quota depletes fast (write operations cost 50 units each, search costs 100 units). Quota resets at midnight PT regardless of when exceeded. Prevention: Calculate quota costs before building features, implement quota tracking middleware, request quota increase early (approval takes weeks).

3. **OAuth2 token storage without rotation strategy** — Google rotates refresh tokens on use; failing to persist new refresh tokens after refresh causes authentication failure. Concurrent refreshes can invalidate token chain. Prevention: Persist entire token after refresh with atomic updates, use oauth2.ReuseTokenSource, add token expiry buffer (refresh 5min early).

4. **Stdout pollution breaking MCP protocol** — MCP uses stdio transport; any fmt.Printf or default logging to stdout corrupts JSON-RPC stream. Prevention: Configure log.SetOutput(os.Stderr) at program start, never use fmt.Printf, test with MCP Inspector early.

5. **Strict JSON schema validation failures** — MCP clients enforce stricter validation than full JSON Schema spec; complex nested structs fail with "Invalid $ref" or "additionalProperties" errors. Prevention: Keep tool input schemas simple (flat structs, basic types), test schema generation with MCP Inspector before implementing complex parameters.

6. **Exponential backoff without jitter causing thundering herd** — Multiple concurrent requests retry at same time (1s, 2s, 4s), creating synchronized retry storms that keep hitting rate limits. Prevention: Add jitter to backoff formula, respect Retry-After headers, implement circuit breaker, limit max retries.

7. **Cookie-based YouTube Music access violating ToS** — Using unofficial ytmusicapi with browser cookies violates YouTube Terms of Service, breaks with new security mechanisms (PO Tokens required since March 2025), no rate limiting guidance risks IP bans. Prevention: DO NOT use for production; scope to official API capabilities only.

## Implications for Roadmap

Based on research, the project requires a critical architectural decision in Phase 0, followed by a foundation-first approach to avoid pitfalls.

### Phase 0: API Validation & Scope Definition
**Rationale:** Must validate YouTube Data API v3 capabilities before any development. Research shows listening history is unavailable through official APIs. This phase determines project feasibility.

**Delivers:** Architectural decision document confirming API limitations, project scope (playlist management only vs unofficial API with ToS warnings vs wait for official API).

**Addresses:** Pitfall #1 (assuming YouTube Music API exists), Pitfall #7 (cookie-based auth violates ToS).

**Decision points:**
- Accept playlist-only scope using official API (lower risk, limited features)
- Use unofficial ytmusicapi for history (higher risk, full features, ToS violation)
- Defer project until official YouTube Music API available (no timeline)

**Research flag:** Standard decision, no additional research needed (APIs already validated in this research).

### Phase 1: Authentication & Foundation
**Rationale:** OAuth2 token management and quota tracking are the hardest parts with the most pitfalls. Build and test these in isolation before adding MCP complexity. Validates authentication flow and discovers quota behavior early.

**Delivers:** OAuth2 token manager with web flow, file-based persistence, automatic refresh with rotation handling; quota tracking middleware; stderr-only logging configuration.

**Addresses:** Pitfall #2 (quota exhaustion), Pitfall #3 (token refresh), Pitfall #4 (stdout pollution).

**Stack elements:**
- golang.org/x/oauth2/google for OAuth2 flow
- File-based token storage with 0600 permissions
- log/slog configured to stderr
- Quota calculation and tracking

**Research flag:** Skip research-phase (OAuth2 and quota patterns are well-documented, covered in STACK.md and PITFALLS.md).

### Phase 2: YouTube API Integration
**Rationale:** With authentication working, wrap YouTube Data API v3 in clean interface. Implement and test API calls (search, playlist CRUD) before MCP protocol layer. Discovers API quirks and validates quota costs.

**Delivers:** YouTube API client wrapper implementing interface for testability, search operations, playlist creation and management, retry logic with jitter, response transformation.

**Uses:** google.golang.org/api/youtube/v3 client, hashicorp/go-retryablehttp for retries.

**Implements:** Integration Layer component from architecture (YouTube API Client Manager).

**Addresses:** Pitfall #6 (exponential backoff without jitter).

**Research flag:** Skip research-phase (YouTube Data API v3 is well-documented, patterns covered in ARCHITECTURE.md).

### Phase 3: MCP Server Foundation
**Rationale:** With authentication and YouTube client working, add MCP protocol layer. Proves stdio transport and protocol handling before adding business logic. MCP Inspector validates protocol compliance early.

**Delivers:** MCP server with stdio transport, lifecycle management, capability negotiation, session management, MCP Inspector compatibility.

**Uses:** modelcontextprotocol/go-sdk v1.3.0+ with StdioTransport.

**Implements:** Transport Layer and Protocol Layer components from architecture.

**Addresses:** Pitfall #4 (stdout pollution validation with MCP Inspector).

**Research flag:** Skip research-phase (MCP server patterns covered in ARCHITECTURE.md, official SDK examples available).

### Phase 4: Tool Implementations
**Rationale:** All foundation layers working; now implement business logic. Tool handlers connect MCP requests to YouTube API client. Fastest iteration when foundation is solid.

**Delivers:** search_tracks tool, create_playlist tool, add_to_playlist tool, typed input/output schemas with auto-generated JSON schemas, outcome-oriented formatting.

**Uses:** Typed tool handlers with auto-schema inference (mcp.AddTool generic function).

**Implements:** Business Layer component (Tool Handlers) from architecture.

**Addresses:** Pitfall #5 (JSON schema validation—test with MCP Inspector).

**Research flag:** Skip research-phase (tool design patterns covered in ARCHITECTURE.md).

### Phase 5: Integration & Polish
**Rationale:** All components working individually; wire together and test end-to-end with Claude Desktop. Add error handling improvements, user-facing quota display, documentation.

**Delivers:** Complete integration in cmd/youtube-music-mcp/main.go, user-friendly error messages mapping API errors, quota status tool, comprehensive README with OAuth setup guide.

**Addresses:** All remaining UX pitfalls (error messaging, quota visibility, OAuth setup friction).

**Research flag:** Skip research-phase (standard integration and polish work).

### Phase Ordering Rationale

- **Phase 0 first:** Architectural decision determines project feasibility; cannot proceed without scope definition.
- **Phase 1 before MCP:** OAuth2 and quota tracking have most pitfalls; isolate and test before adding MCP complexity. Token refresh with rotation requires careful implementation.
- **Phase 2 before MCP:** Validate YouTube API works independently; discover quota costs and API quirks without MCP layer confusing debugging.
- **Phase 3 before tools:** Prove MCP protocol works (stdio transport, JSON-RPC) before adding business logic. MCP Inspector catches stdout pollution early.
- **Phase 4 once foundation solid:** Tool implementation is straightforward when all layers work; rapid iteration possible.
- **Phase 5 for integration:** Edge cases and UX issues emerge during end-to-end testing; need all components working first.

**Dependency chain:** OAuth Manager (no deps) → YouTube Client (needs auth) → MCP Core (parallel to YouTube) → Tool Handlers (needs both) → Main (wires all).

### Research Flags

**Phases with standard patterns (skip research-phase):**
- **Phase 1 (Auth & Foundation):** OAuth2 patterns well-documented, covered in STACK.md and PITFALLS.md
- **Phase 2 (YouTube Integration):** YouTube Data API v3 has comprehensive official docs, patterns in ARCHITECTURE.md
- **Phase 3 (MCP Foundation):** MCP SDK is official with examples, patterns in ARCHITECTURE.md
- **Phase 4 (Tool Implementations):** Tool design patterns covered in ARCHITECTURE.md and FEATURES.md
- **Phase 5 (Integration):** Standard integration work, no additional research needed

**No phases require /gsd:research-phase** during planning—all technical patterns are well-documented and covered in this research.

**Validation needed during implementation:**
- Phase 0: Confirm YouTube Data API v3 activities.list behavior (test with real account)
- Phase 1: Test OAuth2 refresh token rotation with Google's implementation
- Phase 2: Measure actual quota costs for operations (search, playlist CRUD)
- Phase 3: Validate MCP Inspector detects all stdout pollution
- Phase 4: Test tool schemas with Claude and MCP Inspector (not just schema validation)

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Official SDKs from Anthropic+Google and Google; version compatibility verified; patterns established |
| Features | MEDIUM | Feature analysis solid but constrained by API limitations; listening history unavailable through official API creates major scope limitation |
| Architecture | HIGH | Standard MCP patterns well-documented; official SDK examples available; layered architecture proven across MCP ecosystem |
| Pitfalls | MEDIUM-HIGH | Quota and OAuth2 pitfalls verified in official docs (HIGH); MCP stdio and schema pitfalls based on community reports (MEDIUM); cookie auth risks clear (HIGH) |

**Overall confidence:** MEDIUM-HIGH

Confidence is high for technical implementation (stack, architecture, known pitfalls) but medium for feature scope due to fundamental API limitation. The critical unknown—whether unofficial ytmusicapi is acceptable—is a business decision, not a technical one.

### Gaps to Address

**API Limitation Resolution:**
- **Gap:** YouTube Music listening history not available via official API; ytmusicapi violates ToS.
- **Handle during:** Phase 0 architectural decision; explicitly document scope limitations or ToS risks.
- **Validation:** Test YouTube Data API v3 activities.list with real account to confirm it doesn't return music playback.

**Quota Cost Empirical Validation:**
- **Gap:** Quota cost calculations are based on documentation; actual costs may vary with field selection and response size.
- **Handle during:** Phase 2 implementation; measure real quota usage with YouTube API quota dashboard.
- **Validation:** Track quota consumption for search (claimed 100 units), playlist creation (claimed 50 units), playlist item insertion (claimed 50 units).

**Token Refresh Rotation Behavior:**
- **Gap:** Google's OAuth2 refresh token rotation timing unclear (immediate rotation vs threshold-based).
- **Handle during:** Phase 1 implementation; test refresh behavior with manual token expiry.
- **Validation:** Monitor token file for refresh token changes after multiple refresh cycles.

**MCP Schema Compatibility:**
- **Gap:** JSON schema validation strictness varies by MCP client; documented issues with Azure AI Foundry.
- **Handle during:** Phase 4 tool implementation; test with both MCP Inspector and Claude Desktop.
- **Validation:** All tools must load and execute in Claude, not just pass schema validation tests.

**Anti-mainstream Filtering Feasibility:**
- **Gap:** YouTube Data API v3 does not provide track popularity metrics (play counts, chart positions).
- **Handle during:** Phase 0 scope definition; determine if this feature is essential or can be deferred.
- **Validation:** Research alternative popularity data sources (Last.fm API, Spotify API for cross-reference) if feature is critical.

## Sources

### Primary (HIGH confidence)
- [Go SDK for MCP - GitHub](https://github.com/modelcontextprotocol/go-sdk) — Official SDK v1.3.0, API reference, compatibility
- [Go SDK for MCP - pkg.go.dev](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) — API documentation
- [YouTube Data API v3 - Google Developers](https://developers.google.com/youtube/v3) — Official API capabilities, quota system
- [YouTube Go Quickstart](https://developers.google.com/youtube/v3/quickstart/go) — OAuth2 setup, API initialization
- [youtube/v3 package - pkg.go.dev](https://pkg.go.dev/google.golang.org/api/youtube/v3) — Go client v0.266.0
- [oauth2/google package - pkg.go.dev](https://pkg.go.dev/golang.org/x/oauth2/google) — OAuth2 implementation v0.35.0
- [Quota and Compliance Audits - YouTube API](https://developers.google.com/youtube/v3/guides/quota_and_compliance_audits) — Quota costs, limits
- [Activities API - YouTube](https://developers.google.com/youtube/v3/docs/activities) — Confirms activities.list does not return music playback
- [OAuth2 Best Practices - Google](https://developers.google.com/identity/protocols/oauth2/resources/best-practices) — Token refresh, rotation

### Secondary (MEDIUM confidence)
- [Building MCP Server in Go - Navendu Pottekkat](https://navendu.me/posts/mcp-server-go/) — Implementation patterns
- [MCP Best Practices 2026 - CData](https://www.cdata.com/blog/mcp-server-best-practices-2026) — Production guidelines
- [15 Best Practices for MCP Servers - The New Stack](https://thenewstack.io/15-best-practices-for-building-mcp-servers-in-production/) — Architecture patterns
- [MCP Transport Comparison - MCPcat](https://mcpcat.io/guides/comparing-stdio-sse-streamablehttp/) — Stdio vs SSE vs HTTP
- [OAuth 2.1 Features 2026](https://rgutierrez2004.medium.com/oauth-2-1-features-you-cant-ignore-in-2026-a15f852cb723) — Refresh token rotation
- [OAuth 2.0 Refresh Token Rotation](https://hhow09.github.io/blog/oauth2-refresh-token/) — Rotation patterns
- [Debugging MCP Servers](https://www.mcpevals.io/blog/debugging-mcp-servers-tips-and-best-practices) — Stdout pollution, protocol errors
- [MCP Security Survival Guide](https://towardsdatascience.com/the-mcp-security-survival-guide-best-practices-pitfalls-and-real-world-lessons/) — Security patterns
- [How to Handle API Rate Limits](https://apistatuscheck.com/blog/how-to-handle-api-rate-limits) — Exponential backoff with jitter

### Tertiary (LOW confidence, needs validation)
- [Does YouTube Music have an API? - Musicfetch](https://musicfetch.io/services/youtube-music/api) — Confirms no official API
- [ytmusicapi - GitHub](https://github.com/sigma67/ytmusicapi) — Unofficial API, ToS implications
- [YouTube Music AI Playlist](https://9to5google.com/2026/02/10/youtube-music-adding-ai-playlist-with-text-based-playlist-generation/) — Native feature, not API
- [Discogs MCP Server](https://blog.willchatham.com/2026/01/04/discogs-mcp-server/) — Similar domain, patterns
- [Music Recommendation System Guide](https://stratoflow.com/music-recommendation-system-guide/) — Industry patterns

---
*Research completed: 2026-02-13*
*Ready for roadmap: YES with critical caveat—Phase 0 architectural decision required before roadmap execution*
