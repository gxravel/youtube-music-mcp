# Quick Task 4: Server-side OAuth Flow for Railway

## What Was Built

### Task 1: MemoryTokenStorage + ExchangeAndSave (223091f)
- **MemoryTokenStorage** in `internal/auth/token_storage.go` — thread-safe in-memory token storage with `Load()`, `Save()`, `HasToken()`
- **ExchangeAndSave** in `internal/auth/oauth.go` — extracted reusable function that exchanges OAuth code for token, saves to storage, returns authenticated HTTP client
- Existing `Authenticate()` refactored to call `ExchangeAndSave` internally (no behavior change for stdio)

### Task 2: Server-side OAuth Endpoints + Auth-Gated SSE (4a30967)
- **`/health`** — always returns 200 (Railway liveness check)
- **`/auth`** — redirects to Google OAuth2 consent page (or returns "Already authenticated")
- **`/callback`** — receives OAuth code, exchanges for token, creates YouTube client, registers MCP tools, signals readiness
- **`/sse` + `/message`** — gated behind auth; returns 503 "Not authenticated" until /auth flow completes
- **`cmd/server/main.go`** restructured: `runStdioMode()` (unchanged) vs `runSSEMode()` (starts server before auth)
- **OAUTH_TOKEN_JSON bootstrap** — if set, server pre-authenticates and /sse works immediately (backward compatible)

## Verification
- `go build ./...` — compiles cleanly
- `go vet ./...` — passes
- SSE mode starts without token, /health returns 200
- Stdio mode unchanged

## How to Use on Railway
1. Set env vars: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `TRANSPORT=sse`, `OAUTH_REDIRECT_URL=https://youtube-music-mcp-production.up.railway.app/callback`
2. Add redirect URI in Google Cloud Console: `https://youtube-music-mcp-production.up.railway.app/callback`
3. Deploy — server starts and passes health checks
4. Visit `https://youtube-music-mcp-production.up.railway.app/auth` in browser to authenticate
5. After consent, /sse is ready for MCP clients
