---
phase: quick-3
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/youtube/playlists.go
  - internal/youtube/subscriptions.go
  - internal/youtube/client.go
  - internal/server/tools_analyze.go
  - internal/server/tools_recommend.go
  - internal/server/server.go
  - internal/auth/oauth.go
  - internal/auth/token_storage.go
  - internal/config/config.go
  - cmd/server/main.go
  - Dockerfile
  - Makefile
  - .dockerignore
autonomous: false
must_haves:
  truths:
    - "ym:analyze-my-tastes fetches ALL liked videos with no cap (not just 200)"
    - "ym:analyze-my-tastes fetches ALL playlists and ALL subscriptions with no cap"
    - "Liked videos are filtered to music-only using videoCategoryId=10 batch lookup"
    - "Server can run on Railway via SSE transport with token from environment variable"
    - "make build, make run, make test all work locally"
  artifacts:
    - path: "internal/youtube/playlists.go"
      provides: "Uncapped pagination for liked videos, playlists, playlist items"
    - path: "internal/youtube/subscriptions.go"
      provides: "Uncapped pagination for subscriptions"
    - path: "internal/youtube/client.go"
      provides: "FilterMusicVideos batch method using Videos.List categoryId check"
    - path: "Dockerfile"
      provides: "Multi-stage Docker build for Railway deployment"
    - path: "Makefile"
      provides: "Local dev commands: build, run, test"
  key_links:
    - from: "internal/server/tools_analyze.go"
      to: "internal/youtube/client.go"
      via: "FilterMusicVideos call after GetLikedVideos"
      pattern: "FilterMusicVideos"
    - from: "cmd/server/main.go"
      to: "internal/server/server.go"
      via: "SSE transport selection based on TRANSPORT env var"
      pattern: "SSETransport|StdioTransport"
---

<objective>
Remove pagination caps so the full YouTube Music library is analyzed, filter out non-music videos, add Railway deployment support (Dockerfile + SSE transport + env-based token storage), and add a Makefile for local development.

Purpose: Enable complete taste analysis instead of partial (200 songs max), deploy as a hosted MCP server, and streamline local dev workflow.
Output: Updated youtube client with uncapped pagination + music filter, SSE transport option, Dockerfile, Makefile.
</objective>

<execution_context>
@/home/gxravel/.claude/get-shit-done/workflows/execute-plan.md
@/home/gxravel/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/youtube/playlists.go
@internal/youtube/subscriptions.go
@internal/youtube/client.go
@internal/youtube/search.go
@internal/server/tools_analyze.go
@internal/server/tools_recommend.go
@internal/server/server.go
@internal/auth/oauth.go
@internal/auth/token_storage.go
@internal/config/config.go
@cmd/server/main.go
@go.mod
</context>

<tasks>

<task type="auto">
  <name>Task 1: Remove pagination caps and add music-only filtering</name>
  <files>
    internal/youtube/playlists.go
    internal/youtube/subscriptions.go
    internal/youtube/client.go
    internal/server/tools_analyze.go
    internal/server/tools_recommend.go
  </files>
  <action>
  **1. Remove maxResults caps from all fetch methods:**

  In `playlists.go`:
  - `GetLikedVideos`: Change signature to `GetLikedVideos(ctx context.Context) ([]Video, error)`. Remove `maxResults` parameter entirely. Remove the early-stop `errStopPagination` check inside the Pages callback. Let `.Pages()` iterate through ALL pages naturally. Remove truncation logic. Keep the `MaxResults(50)` on the API call (that's per-page size, not total cap).
  - `ListPlaylists`: Same treatment — remove `maxResults` param, fetch ALL playlists.
  - `GetPlaylistItems`: Same treatment — remove `maxResults` param, fetch ALL items.
  - Remove the `errStopPagination` sentinel entirely from this file (move it or delete if unused).

  In `subscriptions.go`:
  - `GetSubscriptions`: Same treatment — remove `maxResults` param, fetch ALL subscriptions.
  - The `errStopPagination` sentinel is defined in `playlists.go`, so just remove the import/usage here.

  **2. Add music-only video filtering in `client.go`:**

  Add a new method `FilterMusicVideos(ctx context.Context, videos []Video) ([]Video, error)`:
  - Use `Videos.List([]string{"snippet"}).Id(ids...).Fields("items(id,snippet/categoryId)")` to batch-check video categories.
  - YouTube API allows up to 50 IDs per call. Process in batches of 50.
  - Keep only videos where `snippet.categoryId == "10"` (Music category).
  - Return the filtered slice.
  - Note: Use `Fields()` to minimize response size (we only need categoryId).
  - Quota cost: 1 unit per Videos.List call, ~1 call per 50 videos.

  **3. Update tool handlers to use new signatures:**

  In `tools_analyze.go`:
  - Change `s.ytClient.GetLikedVideos(ctx, 200)` to `s.ytClient.GetLikedVideos(ctx)`.
  - After fetching liked videos, add: `likedVideos, err = s.ytClient.FilterMusicVideos(ctx, likedVideos)` with error handling. Update the section header to say "Liked Songs (music only)".
  - Change `s.ytClient.GetSubscriptions(ctx, 100)` to `s.ytClient.GetSubscriptions(ctx)`.
  - Change `s.ytClient.ListPlaylists(ctx, 50)` to `s.ytClient.ListPlaylists(ctx)`.
  - Change `s.ytClient.GetPlaylistItems(ctx, pl.ID, 100)` to `s.ytClient.GetPlaylistItems(ctx, pl.ID)`.

  In `tools_recommend.go`:
  - Update all calls: `GetLikedVideos(ctx)`, `GetSubscriptions(ctx)`, `ListPlaylists(ctx)`.
  - In `recommend-playlist`, after fetching likedVideos for taste context, do NOT filter (we want all artists for taste, even non-music channels). Keep it simple.
  - In `recommend-artists` and `recommend-albums`, same — no filtering needed, just remove maxResults args.

  **4. Keep `errStopPagination` in `playlists.go`** if `SearchVideos` in `search.go` uses it. Check — it does not (search uses `.Do()` not `.Pages()`). So remove `errStopPagination` entirely.
  </action>
  <verify>
  Run `cd /home/gxravel/go/src/github.com/gxravel/youtube-music-mcp && go build ./...` — must compile with zero errors.
  Run `go vet ./...` — must pass.
  Grep for `maxResults` in youtube package — should only appear as the API per-page parameter `.MaxResults(50)`, not as function parameters.
  Grep for `errStopPagination` — should not exist anywhere.
  </verify>
  <done>
  All fetch methods have no pagination cap (fetch everything). FilterMusicVideos method exists and is called in analyze tool. All tool handlers updated. Code compiles and passes vet.
  </done>
</task>

<task type="auto">
  <name>Task 2: Add Railway deployment support (Dockerfile, SSE transport, env token storage)</name>
  <files>
    internal/config/config.go
    internal/auth/token_storage.go
    internal/auth/oauth.go
    internal/server/server.go
    cmd/server/main.go
    Dockerfile
    .dockerignore
  </files>
  <action>
  **1. Add SSE transport support to server:**

  In `config/config.go`, add:
  - `Transport string \`env:"TRANSPORT" envDefault:"stdio"\`` — accepts "stdio" or "sse".
  - `Port int \`env:"PORT" envDefault:"8080"\`` — port for SSE server (Railway sets PORT).
  - `TokenJSON string \`env:"OAUTH_TOKEN_JSON"\`` — base64 or raw JSON of the OAuth token for environments without filesystem token storage.

  In `auth/token_storage.go`, add `EnvTokenStorage`:
  - `type EnvTokenStorage struct { tokenJSON string }`
  - `func NewEnvTokenStorage(tokenJSON string) *EnvTokenStorage`
  - `Load()`: Unmarshal the JSON string into `oauth2.Token`. Return it.
  - `Save()`: No-op (log warning that token refresh won't persist in env mode). Return nil.
  - Add `var _ TokenStorage = (*EnvTokenStorage)(nil)` compile check.

  In `cmd/server/main.go`:
  - After loading config, select token storage: if `cfg.TokenJSON != ""`, use `auth.NewEnvTokenStorage(cfg.TokenJSON)`, else use `auth.NewFileTokenStorage(auth.DefaultTokenPath())`.
  - Pass `cfg.Transport` and `cfg.Port` to `server.NewServer` (update signature).

  In `server/server.go`:
  - Update `NewServer` to accept `transport string` and `port int` parameters. Store them on the Server struct.
  - In `Run()`: if `transport == "sse"`, use `mcp.NewSSETransport()` listening on the configured port. If "stdio", use `mcp.StdioTransport{}` as before.
  - For SSE: check go-sdk docs — the pattern is likely `sse := mcp.NewSSETransport()` then `http.ListenAndServe(":PORT", sse)` or similar. Look at go-sdk source for SSE transport. If SSE is not directly available in go-sdk v1.3.0, use `mcp.NewHTTPTransport()` or the streamable HTTP transport pattern. The key is: check what transports `go-sdk v1.3.0` actually provides beyond StdioTransport. If only stdio exists, implement a simple HTTP wrapper that accepts JSON-RPC over HTTP POST (the MCP spec supports this).

  **IMPORTANT**: Check `go doc github.com/modelcontextprotocol/go-sdk/mcp` for available transport types before implementing. Use whatever SSE/HTTP transport the SDK provides. If none exists, use streamable HTTP transport pattern from MCP spec.

  **2. Update OAuth scope:**

  In `auth/oauth.go`: The scope is currently `youtube.YoutubeReadonlyScope` but the app creates playlists and adds videos. Change to `youtube.YoutubeScope` (full read-write). This is needed for Railway deployment to work with playlist creation.

  **3. Create Dockerfile:**

  Multi-stage build:
  ```dockerfile
  FROM golang:1.25-alpine AS builder
  WORKDIR /app
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server

  FROM alpine:3.21
  RUN apk add --no-cache ca-certificates
  COPY --from=builder /bin/server /bin/server
  ENTRYPOINT ["/bin/server"]
  ```

  **4. Create .dockerignore:**
  ```
  .git
  .planning
  server.log
  bin/
  *.md
  ```

  **5. No railway.json needed** — Railway auto-detects Dockerfile. Environment variables (GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, OAUTH_TOKEN_JSON, TRANSPORT=sse) are set in Railway dashboard.
  </action>
  <verify>
  Run `cd /home/gxravel/go/src/github.com/gxravel/youtube-music-mcp && go build ./...` — must compile.
  Run `go vet ./...` — must pass.
  Run `docker build -t youtube-music-mcp .` — must build successfully (if Docker available).
  Verify EnvTokenStorage implements TokenStorage: grep for the compile-time check.
  </verify>
  <done>
  Server supports both stdio and SSE transports selected by TRANSPORT env var. EnvTokenStorage allows token injection via OAUTH_TOKEN_JSON. Dockerfile builds successfully. OAuth scope is read-write.
  </done>
</task>

<task type="auto">
  <name>Task 3: Add Makefile for local development</name>
  <files>
    Makefile
  </files>
  <action>
  Create a Makefile with these targets:

  - `build`: `go build -o bin/server ./cmd/server`
  - `run`: `go run ./cmd/server` (stdio mode, for MCP client testing)
  - `run-sse`: `TRANSPORT=sse go run ./cmd/server` (SSE mode, for browser/HTTP testing)
  - `test`: `go test ./...`
  - `vet`: `go vet ./...`
  - `lint`: `go vet ./... && go build ./...` (basic lint without external tools)
  - `docker-build`: `docker build -t youtube-music-mcp .`
  - `docker-run`: `docker run --rm --env-file .env -p 8080:8080 -e TRANSPORT=sse youtube-music-mcp`
  - `clean`: `rm -rf bin/`

  Use `.PHONY` for all targets. Add a brief comment header explaining usage.
  Default target: `build`.
  </action>
  <verify>
  Run `cd /home/gxravel/go/src/github.com/gxravel/youtube-music-mcp && make build` — must produce `bin/server`.
  Run `make vet` — must pass.
  Run `make clean && ls bin/` — bin/ should not exist.
  </verify>
  <done>
  Makefile exists with build, run, run-sse, test, vet, lint, docker-build, docker-run, clean targets. `make build` produces working binary.
  </done>
</task>

</tasks>

<verification>
1. `go build ./...` compiles cleanly
2. `go vet ./...` passes
3. `make build` produces `bin/server`
4. No `maxResults` function parameters remain in youtube package methods
5. No `errStopPagination` sentinel remains
6. `FilterMusicVideos` method exists in `client.go`
7. Server struct accepts transport config
8. EnvTokenStorage implements TokenStorage
9. Dockerfile builds successfully
</verification>

<success_criteria>
- All YouTube data fetching methods retrieve the FULL library (no pagination caps)
- Liked videos are filtered to music-only (categoryId=10) in the analyze tool
- Server supports both stdio (local/MCP) and SSE (Railway) transports
- Token can be provided via environment variable for serverless deployment
- Dockerfile builds and runs the server
- Makefile provides all common dev commands
</success_criteria>

<output>
After completion, create `.planning/quick/3-full-library-analysis-railway-deploy-mak/3-SUMMARY.md`
</output>
