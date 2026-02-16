# YouTube Music MCP

## What This Is

An MCP server written in Go that gives Claude access to a user's YouTube Music data (liked videos, playlists, subscriptions) and the ability to search for tracks, create playlists, and populate them with recommendations. Claude uses its broad music knowledge combined with the user's taste profile to recommend genuinely interesting music delivered as playable YouTube Music playlists.

## Core Value

Claude can analyze my YouTube Music taste and recommend genuinely interesting music I haven't heard — not the popular stuff YouTube's algorithm pushes — and deliver it as a ready-to-play playlist.

## Requirements

### Validated

- ✓ OAuth2 authentication with token persistence and auto-refresh — v1.0
- ✓ Taste data retrieval (liked videos, playlists, subscriptions) — v1.0
- ✓ YouTube Music search with music category filtering — v1.0
- ✓ Video lookup and verification — v1.0
- ✓ Playlist creation with privacy controls — v1.0
- ✓ Video addition to playlists with duplicate handling — v1.0

### Active

- [ ] Mood/context-based recommendations ("chill work music", "aggressive workout tracks")
- [ ] Deep cuts discovery from favorite artists (lesser-known tracks, B-sides)
- [ ] Tunable discovery bias (how adventurous recommendations should be)
- [ ] Listening pattern analysis (genre distribution, favorite artists)
- [ ] Music collection gap identification

### Out of Scope

- Browser automation / Playwright control of Firefox — playlist links are sufficient
- Real-time playback control (play/pause/skip) — user controls their own player
- Mobile app or standalone UI — this is an MCP server consumed by Claude
- Offline music analysis — requires active YouTube API connection
- Social features / sharing — personal use only
- Listening history access — no official YouTube API, would violate ToS
- Multi-platform (Spotify, Apple Music) — YouTube Music only for clean scope
- Unofficial ytmusicapi / cookie auth — violates YouTube ToS, unreliable

## Context

Shipped v1.0 with 1,343 LOC Go across 3 phases in 3 days.
Tech stack: Go, MCP Go SDK (github.com/modelcontextprotocol/go-sdk), Google YouTube Data API v3, caarlos0/env for config.

8 MCP tools available: get_liked_videos, list_playlists, get_playlist_items, list_subscriptions, search_videos, get_video, create_playlist, add_to_playlist.

YouTube Data API quota: 10,000 units/day. Search costs 100 units, playlist writes cost 50 units, reads cost 1-5 units.

## Constraints

- **Tech stack**: Go with `github.com/modelcontextprotocol/go-sdk/mcp` for MCP protocol, `google.golang.org/api/youtube/v3` for YouTube API
- **Auth**: OAuth2 required for YouTube API access to user data (reading taste data, creating playlists)
- **API limits**: YouTube Data API has quota limits (10,000 units/day default) — search is most expensive at 100 units
- **History access**: YouTube doesn't expose listening history via API — taste data limited to liked videos, playlists, and subscriptions

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over TypeScript | User preference + solid MCP/YouTube API library support in Go | ✓ Good |
| YouTube API over browser automation | Cleaner, more reliable data access; browser automation is fragile | ✓ Good |
| Playlist creation over browser playback control | Simpler delivery mechanism; user opens playlist link in their pinned tab | ✓ Good |
| Claude as recommendation engine | Leverages Claude's broad music knowledge beyond algorithm-driven picks | ✓ Good |
| caarlos0/env for config | Type-safe struct tags, cleaner than manual os.Getenv | ✓ Good |
| Atomic token writes (temp+rename) | Prevents corruption on crash during token refresh | ✓ Good |
| MCP typed handlers (AddTool generic) | Automatic schema generation, compile-time type safety | ✓ Good |
| Single-page search (no pagination) | Conserves quota (100 units/page), sufficient for recommendations | ✓ Good |
| videoCategoryId=10 for music filtering | Not perfect but best available filter via official API | ✓ Good |
| Skip duplicate videos silently (409) | Better UX than erroring; goal is "video in playlist" | ✓ Good |
| Default privacy to "private" | Safe default prevents accidental public exposure | ✓ Good |
| YouTube Music URLs in responses | Project targets YouTube Music users, not regular YouTube | ✓ Good |

---
*Last updated: 2026-02-16 after v1.0 milestone*
