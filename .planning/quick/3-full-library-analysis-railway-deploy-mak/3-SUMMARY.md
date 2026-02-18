---
phase: quick-3
plan: 01
subsystem: youtube-client, server, deployment
tags: [pagination, music-filter, railway, sse, docker, makefile]
dependency_graph:
  requires: []
  provides:
    - uncapped-pagination-all-youtube-data
    - music-only-filtering-via-category-id
    - sse-transport-for-railway
    - env-token-storage-for-serverless
    - dockerfile-multi-stage
    - makefile-dev-commands
  affects:
    - internal/youtube/playlists.go
    - internal/youtube/subscriptions.go
    - internal/youtube/client.go
    - internal/server/tools_analyze.go
    - internal/server/tools_recommend.go
    - internal/server/server.go
    - internal/auth/token_storage.go
    - internal/auth/oauth.go
    - internal/config/config.go
    - cmd/server/main.go
    - Dockerfile
    - .dockerignore
    - Makefile
tech_stack:
  added: []
  patterns:
    - uncapped-pagination-via-pages-callback
    - batch-video-category-lookup-50-ids-per-call
    - sse-transport-via-mcp-NewSSEHandler
    - env-token-storage-no-op-save
    - multi-stage-docker-build
key_files:
  created:
    - Dockerfile
    - .dockerignore
    - Makefile
  modified:
    - internal/youtube/playlists.go
    - internal/youtube/subscriptions.go
    - internal/youtube/client.go
    - internal/server/tools_analyze.go
    - internal/server/tools_recommend.go
    - internal/server/server.go
    - internal/auth/token_storage.go
    - internal/auth/oauth.go
    - internal/config/config.go
    - cmd/server/main.go
decisions:
  - Removed errStopPagination sentinel entirely — pagination always fetches all pages
  - FilterMusicVideos uses Videos.List with Fields() to minimize response size (only categoryId)
  - SSE transport implemented via mcp.NewSSEHandler (go-sdk v1.3.0 SSEHandler pattern)
  - EnvTokenStorage.Save is a no-op with a warning log — refreshes not persisted in Railway
  - OAuth scope upgraded from YoutubeReadonlyScope to YoutubeScope (needed for playlist creation)
  - TRANSPORT env var selects transport (stdio default; sse for Railway)
  - PORT env var used by Railway automatically; same var used for SSE listen addr
metrics:
  duration: 5 min
  completed: 2026-02-18
  tasks_completed: 3
  files_changed: 13
---

# Phase quick-3 Plan 01: Full Library Analysis + Railway Deploy + Makefile Summary

**One-liner:** Uncapped YouTube library pagination with music-only filtering (categoryId=10), SSE transport for Railway deployment via `mcp.NewSSEHandler`, `EnvTokenStorage` for token injection, and Makefile dev commands.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Remove pagination caps and add music-only filtering | 8462a1b | playlists.go, subscriptions.go, client.go, tools_analyze.go, tools_recommend.go |
| 2 | Add Railway deployment support | 55c39ed | config.go, token_storage.go, oauth.go, server.go, main.go, Dockerfile, .dockerignore |
| 3 | Add Makefile for local development | d7d4c9b | Makefile |

## What Was Built

### Task 1: Uncapped Pagination + Music Filter

All YouTube data fetch methods now iterate ALL pages with no cap:
- `GetLikedVideos(ctx)` — removed `maxResults` param; uses `.Pages()` to fetch everything
- `ListPlaylists(ctx)` — same
- `GetPlaylistItems(ctx, id)` — same
- `GetSubscriptions(ctx)` — same

New `FilterMusicVideos(ctx, videos)` method in `client.go`:
- Batches video IDs (50 per call) through `Videos.List` with `Fields("items(id,snippet/categoryId)")`
- Keeps only videos where `categoryId == "10"` (Music)
- Quota cost: ~1 unit per 50 videos

`tools_analyze.go` now calls `FilterMusicVideos` after `GetLikedVideos`, so the analysis shows music-only liked songs.

`errStopPagination` sentinel removed entirely.

### Task 2: Railway Deployment

Config additions (`TRANSPORT`, `PORT`, `OAUTH_TOKEN_JSON`):
- `TRANSPORT=stdio` (default) for local MCP clients
- `TRANSPORT=sse` for Railway/HTTP deployments
- `PORT` used by Railway; SSE server binds to this port
- `OAUTH_TOKEN_JSON` injects a pre-obtained token as JSON string

`EnvTokenStorage`:
- `Load()` — unmarshals JSON string into `oauth2.Token`
- `Save()` — no-op with a warning log (refreshes not persisted in serverless env)
- Compile-time interface check: `var _ TokenStorage = (*EnvTokenStorage)(nil)`

SSE transport in `server.go`:
- Uses `mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return s.mcpServer }, nil)`
- HTTP server with graceful shutdown on context cancellation

OAuth scope upgraded from `YoutubeReadonlyScope` to `YoutubeScope` (full read-write, required for playlist creation).

Dockerfile: multi-stage build — `golang:1.25-alpine` builder, `alpine:3.21` runtime with ca-certificates.

### Task 3: Makefile

Targets: `build`, `run`, `run-sse`, `test`, `vet`, `lint`, `docker-build`, `docker-run`, `clean`

## Deviations from Plan

None — plan executed exactly as written.

## Railway Deployment Status

Railway CLI is installed at `/usr/local/bin/railway` (v4.5.3) but requires authentication. The CLI reported "Unauthorized" and no linked project.

**To complete Railway deployment:**
1. Run `railway login` in a terminal (requires browser)
2. Run `railway link` to link to your Railway project (or create one)
3. Set environment variables in Railway dashboard:
   - `GOOGLE_CLIENT_ID`
   - `GOOGLE_CLIENT_SECRET`
   - `OAUTH_TOKEN_JSON` (export your `~/.config/youtube-music-mcp/token.json` content)
   - `TRANSPORT=sse`
4. Run `railway up` to deploy

## Self-Check: PASSED

Files verified:
- internal/youtube/playlists.go: exists, no errStopPagination, no maxResults params
- internal/youtube/subscriptions.go: exists, no errStopPagination, no maxResults params
- internal/youtube/client.go: exists, FilterMusicVideos method at line 48
- internal/server/tools_analyze.go: exists, calls FilterMusicVideos
- internal/server/server.go: exists, SSE transport support
- internal/auth/token_storage.go: exists, EnvTokenStorage with compile-time check
- Dockerfile: exists, multi-stage build
- Makefile: exists, all targets present

Commits verified:
- 8462a1b: feat(quick-3): remove pagination caps and add music-only filtering
- 55c39ed: feat(quick-3): add Railway deployment support
- d7d4c9b: chore(quick-3): add Makefile for local development
