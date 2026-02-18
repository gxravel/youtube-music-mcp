---
phase: quick-4
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/auth/token_storage.go
  - internal/auth/oauth.go
  - internal/server/server.go
  - cmd/server/main.go
autonomous: true

must_haves:
  truths:
    - "Railway server starts immediately and responds to health checks even without a token"
    - "User can visit /auth in browser to begin Google OAuth2 consent flow"
    - "After completing OAuth consent, /callback receives code, exchanges for token, stores in memory"
    - "After authentication completes, /sse endpoint serves MCP sessions normally"
    - "Stdio mode continues to work exactly as before (no regression)"
  artifacts:
    - path: "internal/auth/token_storage.go"
      provides: "MemoryTokenStorage with thread-safe Load/Save"
      contains: "MemoryTokenStorage"
    - path: "internal/server/server.go"
      provides: "SSE server with /auth, /callback, /sse routing and auth gating"
      contains: "handleAuth"
    - path: "cmd/server/main.go"
      provides: "Revised startup flow — SSE mode starts server before auth"
  key_links:
    - from: "internal/server/server.go"
      to: "internal/auth/oauth.go"
      via: "OAuth config for AuthCodeURL and Exchange"
      pattern: "AuthCodeURL|Exchange"
    - from: "internal/server/server.go"
      to: "internal/auth/token_storage.go"
      via: "MemoryTokenStorage.Save after callback"
      pattern: "storage\\.Save"
    - from: "cmd/server/main.go"
      to: "internal/server/server.go"
      via: "Passes oauth config and storage to server for deferred auth"
---

<objective>
Implement server-side OAuth flow for Railway deployment so the MCP server handles
the full Google OAuth2 consent flow itself — no pre-generated tokens needed.

Purpose: Eliminate the need to manually generate and paste OAUTH_TOKEN_JSON for Railway.
The server starts immediately (passing health checks), exposes /auth and /callback
for browser-based OAuth, and gates /sse behind successful authentication.

Output: Modified auth and server packages; working OAuth flow on Railway.
</objective>

<execution_context>
@/home/gxravel/.claude/get-shit-done/workflows/execute-plan.md
@/home/gxravel/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/auth/oauth.go
@internal/auth/token_storage.go
@internal/server/server.go
@internal/config/config.go
@cmd/server/main.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add MemoryTokenStorage and refactor auth for server-side flow</name>
  <files>internal/auth/token_storage.go, internal/auth/oauth.go</files>
  <action>
In `internal/auth/token_storage.go`:
- Add `MemoryTokenStorage` struct implementing `TokenStorage`. Uses `sync.RWMutex` to protect a `*oauth2.Token` field. `Load()` returns error if no token stored yet. `Save()` stores the token. Add `HasToken() bool` method for quick auth state checks.
- Add compile-time interface check: `var _ TokenStorage = (*MemoryTokenStorage)(nil)`

In `internal/auth/oauth.go`:
- Extract the OAuth code exchange logic into a new exported function `ExchangeAndSave(ctx, cfg, code, storage, logger) (*http.Client, error)` that:
  1. Calls `cfg.Exchange(ctx, code)` to get token
  2. Saves token via `storage.Save(token)`
  3. Creates and returns an `*http.Client` with `PersistingTokenSource`
- The existing `Authenticate` function should call `ExchangeAndSave` internally (avoid duplication)
- Keep `Authenticate` working exactly as before for stdio mode (no behavior change)
  </action>
  <verify>
Run `go build ./...` — must compile cleanly. Run `go vet ./...` — no issues.
  </verify>
  <done>MemoryTokenStorage exists with Load/Save/HasToken. ExchangeAndSave is exported and reusable. Existing Authenticate still works for stdio.</done>
</task>

<task type="auto">
  <name>Task 2: Implement server-side OAuth HTTP endpoints and auth-gated SSE</name>
  <files>internal/server/server.go, cmd/server/main.go</files>
  <action>
In `internal/server/server.go`:
- Add fields to `Server` struct: `oauthCfg *oauth2.Config`, `storage auth.TokenStorage`, `httpClient atomic.Pointer[http.Client]` (or use sync.Mutex + pointer), `ytClientReady chan struct{}` (closed when auth completes).
- Change `NewServer` signature to accept `oauthCfg *oauth2.Config` and `storage auth.TokenStorage` as additional params. The `ytClient` param becomes optional (nil pointer means not yet authenticated). When `ytClient` is non-nil (stdio mode), close `ytClientReady` immediately and register tools.
- Modify `runSSE` to use a custom `http.ServeMux` instead of passing `sseHandler` directly:
  - `GET /auth` — If already authenticated, return "Already authenticated". Otherwise, generate AuthCodeURL (with `oauth2.AccessTypeOffline` and `prompt=consent`) and redirect (HTTP 302) to Google consent.
  - `GET /callback` — Extract `code` query param. Call `auth.ExchangeAndSave(ctx, s.oauthCfg, code, s.storage, s.logger)` to get httpClient. Create `youtube.NewClient`, call `ValidateAuth`, store ytClient on server, register tools (call `s.registerAnalyzeTools()` and `s.registerRecommendTools()`), close `ytClientReady` channel. Return HTML: "Authentication successful! You can close this window."
  - `GET /health` — Return 200 "ok" (Railway health check).
  - All SSE paths (`/sse`, `/message`) — If `ytClientReady` is not yet closed, return 503 "Not authenticated. Visit /auth to authenticate." Otherwise, delegate to `sseHandler`.
- The SSE handler must still be created at server init (mcp.NewSSEHandler) since it handles routing for /sse and /message paths. Wrap it: check auth state before delegating.

In `cmd/server/main.go`:
- Restructure the startup flow:
  - For `stdio` transport: keep existing flow (Authenticate -> create ytClient -> validate -> NewServer -> Run). Pass oauthCfg=nil, storage=nil to NewServer (not needed for stdio).
  - For `sse` transport:
    1. Create `MemoryTokenStorage`.
    2. If `cfg.TokenJSON` is set, try to load from EnvTokenStorage and pre-populate MemoryTokenStorage with that token (bootstrap from existing env token). Then create ytClient immediately and pass to NewServer.
    3. If no token available, pass `nil` ytClient to NewServer. Server will gate /sse behind /auth flow.
    4. Pass `oauthCfg` and `memoryStorage` to NewServer.
    5. Call `srv.Run(ctx)`.
- Keep the existing `Authenticate` call only for stdio mode. SSE mode handles auth via HTTP endpoints.

Important: The `mcp.Server` instance is created once in NewServer. Tools are registered either immediately (if ytClient provided) or after OAuth callback. The SSE handler wraps the same mcp.Server.
  </action>
  <verify>
1. `go build ./...` compiles cleanly
2. `go vet ./...` passes
3. Test locally: `TRANSPORT=sse PORT=8080 GOOGLE_CLIENT_ID=test GOOGLE_CLIENT_SECRET=test go run ./cmd/server/` starts and `curl http://localhost:8080/health` returns 200
4. `curl http://localhost:8080/sse` returns 503 (not authenticated)
5. `curl http://localhost:8080/auth` returns 302 redirect to accounts.google.com
  </verify>
  <done>
SSE server starts immediately, /health returns 200, /auth redirects to Google consent, /callback exchanges code and enables /sse, stdio mode unchanged.
  </done>
</task>

</tasks>

<verification>
1. `go build ./...` — compiles
2. `go vet ./...` — clean
3. SSE mode: server starts without token, /health works, /auth redirects, /sse returns 503 before auth
4. Stdio mode: existing flow unchanged (Authenticate blocks until token obtained)
5. If OAUTH_TOKEN_JSON is set in SSE mode, server bootstraps with that token and /sse works immediately
</verification>

<success_criteria>
- Railway server starts and passes health checks without any token
- /auth endpoint initiates Google OAuth2 consent flow
- /callback receives authorization code, exchanges for token, enables MCP
- /sse returns 503 before auth, works normally after auth
- Stdio mode has zero behavior changes
- OAUTH_TOKEN_JSON bootstrap still works (backward compatible)
</success_criteria>

<output>
After completion, create `.planning/quick/4-server-side-oauth-flow-for-railway/4-SUMMARY.md`
</output>
