# YouTube Music MCP

## What This Is

An MCP (Model Context Protocol) server written in Go that gives Claude access to a user's YouTube Music listening history and the ability to create playlists. Claude analyzes listening patterns, recommends music from its own knowledge combined with YouTube Music search, and delivers recommendations as playable playlists in the user's Firefox pinned tab.

## Core Value

Claude can analyze my full YouTube Music listening history and recommend genuinely interesting music I haven't heard — not the popular stuff YouTube's algorithm pushes — and deliver it as a ready-to-play playlist.

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

(None yet — ship to validate)

### Active

<!-- Current scope. Building toward these. -->

- [ ] MCP server exposes tools for Claude to read YouTube Music listening history
- [ ] MCP server exposes tools to search YouTube Music for tracks
- [ ] MCP server exposes tools to create/manage playlists on YouTube Music
- [ ] OAuth2 authentication with Google/YouTube for accessing user data
- [ ] Full listening history retrieval (all time, not just recent)
- [ ] Claude can analyze taste patterns (artists, genres, listening frequency)
- [ ] Claude recommends from its music knowledge + verifies tracks exist on YouTube Music
- [ ] Recommendations delivered as YouTube Music playlists the user can open and play

### Out of Scope

- Browser automation / Playwright control of Firefox — playlist links are sufficient
- Real-time playback control (play/pause/skip) — user controls their own player
- Mobile app or standalone UI — this is an MCP server consumed by Claude
- Offline music analysis — requires active YouTube API connection
- Social features / sharing — personal use only

## Context

- The user is frustrated with YouTube Music's recommendation algorithm, which surfaces popular/well-known songs rather than genuinely new discoveries
- The MCP approach means Claude acts as the recommendation engine, using its broad music knowledge to suggest tracks that match the user's taste profile but go beyond what YouTube's algorithm would surface
- The user runs YouTube Music in a Firefox pinned tab — recommendations become playlists they open in that tab
- YouTube Data API v3 provides access to liked videos, playlists, and search — but "listening history" specifically requires careful handling as it's not directly exposed the same way
- Go is chosen as the implementation language, with `mcp-go` SDK and Google's official Go API client libraries

## Constraints

- **Tech stack**: Go with `github.com/mark3labs/mcp-go` for MCP protocol, `google.golang.org/api/youtube/v3` for YouTube API
- **Auth**: OAuth2 required for YouTube API access to user data (reading history, creating playlists)
- **API limits**: YouTube Data API has quota limits (10,000 units/day default) — need efficient API usage
- **History access**: YouTube doesn't expose full "listening history" directly via API — may need to work with liked videos, playlists, and watch history as proxies

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over TypeScript | User preference + solid MCP/YouTube API library support in Go | — Pending |
| YouTube API over browser automation | Cleaner, more reliable data access; browser automation is fragile | — Pending |
| Playlist creation over browser playback control | Simpler delivery mechanism; user opens playlist link in their pinned tab | — Pending |
| Claude as recommendation engine | Leverages Claude's broad music knowledge to find genuinely interesting tracks, not just algorithm-driven popular picks | — Pending |

---
*Last updated: 2026-02-13 after initialization*
