# Pitfalls Research

**Domain:** YouTube Music MCP Server (Go + YouTube Data API v3 + OAuth2)
**Researched:** 2026-02-13
**Confidence:** MEDIUM

## Critical Pitfalls

### Pitfall 1: Assuming YouTube Music Has Official API Support

**What goes wrong:**
Developers assume YouTube Music has official API support similar to Spotify or Apple Music, spending weeks building against the YouTube Data API only to discover it cannot access listening history or music-specific features.

**Why it happens:**
The YouTube Data API v3 is well-documented and officially supported, leading developers to assume it covers YouTube Music functionality. The `activities` endpoint sounds like it should track listening activity, but it only tracks platform actions (uploads, likes, comments, subscriptions) and explicitly does not return music listening history.

**How to avoid:**
Accept upfront that official YouTube Music API does not exist. The YouTube Data API v3 can handle playlist creation and management but cannot access listening history. For listening history, you must either: (1) Use unofficial ytmusicapi approach with cookie authentication, (2) Scope the project to only playlist management without history, or (3) Use a different music service entirely.

**Warning signs:**
- No official "YouTube Music API" in Google Cloud Console
- activities.list returns empty or irrelevant results for music playback
- Documentation never mentions "listening history" or "playback history"

**Phase to address:**
Phase 0 (Research/Planning) - This must be validated before roadmap creation to avoid building an impossible feature.

---

### Pitfall 2: Ignoring Quota Costs Leading to Daily Lockout

**What goes wrong:**
The default 10,000 units/day quota depletes within hours of normal usage. Write operations cost 50 units each, so creating 200 playlists exhausts the entire daily quota. Once quota is exceeded, all API access is blocked until midnight PT (Pacific Time), potentially for up to 23 hours.

**Why it happens:**
Developers treat quota as "number of requests" rather than weighted cost units. Read operations cost 1 unit (cheap), but write operations cost 50 units (expensive). The quota calculator is buried in documentation, and developers don't calculate costs until after hitting limits. Quota resets at midnight PT regardless of when you hit the limit - if you exceed quota at 1 AM PT, you're blocked for 23 hours.

**How to avoid:**
- Calculate quota costs before building features: playlistItems.insert = 50 units, playlists.insert = 50 units, search.list = 100 units
- With 10,000 unit default quota: 10 searches (1,000 units) + 100 playlist item additions (5,000 units) + overhead = one moderate usage session
- Implement quota tracking in your server to warn users before depletion
- Add rate limiting and queue write operations
- Request quota increase through Google's audit process early (approval takes weeks)
- Never retry write operations without checking quota first

**Warning signs:**
- 403 Forbidden responses with "quotaExceeded" error
- Sudden API failures at specific times of day (quota reset boundary)
- Users reporting "it worked yesterday but not today"

**Phase to address:**
Phase 1 (Foundation) - Implement quota tracking and rate limiting before exposing any write operations to users.

---

### Pitfall 3: OAuth2 Token Storage Without Rotation Strategy

**What goes wrong:**
Access tokens expire (typically 1 hour), but the server doesn't handle refresh token rotation properly. When refresh tokens are used but not updated in storage, users get logged out unexpectedly. Worse, concurrent requests can cause multiple refresh attempts, invalidating the refresh token chain and forcing complete re-authentication.

**Why it happens:**
OAuth 2.1 (current standard in 2026) requires refresh tokens to be either sender-constrained or rotated on every use with reuse detection. The Go oauth2 package (golang.org/x/oauth2) handles token refresh automatically, but developers don't persist the new refresh token returned after refresh. Google's OAuth implementation rotates refresh tokens, so using an old refresh token after rotation causes authentication failure.

**How to avoid:**
- Store OAuth2 tokens with atomic update semantics (avoid race conditions)
- After successful token refresh, immediately persist the entire new token (access + refresh + expiry)
- Use token source from golang.org/x/oauth2 which handles refresh automatically
- Implement distributed lock (Redis/etcd) if running multiple server instances
- Add token expiry buffer (refresh 5 minutes before expiry, not at expiry)
- Handle 401 Unauthorized by attempting refresh once, then forcing re-auth if refresh fails
- Never hardcode or commit tokens to version control

**Warning signs:**
- Users experiencing random "unauthorized" errors after initial successful auth
- Logs showing 401 errors despite valid access token timestamps
- Multiple concurrent "token refresh" log entries
- "invalid_grant" errors when attempting refresh

**Phase to address:**
Phase 1 (Foundation) - Implement proper token storage and rotation before any multi-user or concurrent usage.

---

### Pitfall 4: Stdout Pollution Breaking MCP Protocol

**What goes wrong:**
The MCP server uses stdio transport (stdin/stdout for JSON-RPC). Any debug logs, print statements, or error messages written to stdout corrupt the protocol stream, causing clients (like Claude) to fail with parsing errors or silent disconnection. The server appears to work in isolation but completely fails when connected.

**Why it happens:**
Go's default logger and fmt.Printf write to stdout. Developers add debug statements during development and forget to remove them or redirect to stderr. The MCP protocol requires that ONLY valid JSON-RPC messages are written to stdout - everything else must go to stderr. This is particularly easy to miss because testing individual functions works fine; only the full stdio transport integration fails.

**How to avoid:**
- Configure Go's log package to write to stderr: `log.SetOutput(os.Stderr)`
- Use structured logging library (zerolog, zap) configured for stderr
- Never use `fmt.Printf` or `fmt.Println` in MCP server code - use `fmt.Fprintf(os.Stderr, ...)` if needed
- Set up linting rule to catch stdout writes during code review
- Test with MCP Inspector tool which validates stdio transport compliance
- Wrap stdout in debug builds to assert only JSON-RPC is written

**Warning signs:**
- MCP Inspector shows "invalid JSON" or parsing errors
- Connection drops immediately after server start
- "Unexpected token" errors in client logs
- Server works in unit tests but fails in integration

**Phase to address:**
Phase 1 (Foundation) - Enforce stderr-only logging from the very first commit to avoid debugging this later.

---

### Pitfall 5: Strict JSON Schema Validation Failures in MCP Tools

**What goes wrong:**
MCP tool schemas generated from Go structs fail validation with cryptic errors like "Invalid $ref" or "additionalProperties not allowed". Claude rejects the tool definitions even though the schema looks valid. The server starts successfully but tools are unusable.

**Why it happens:**
As of January 2026, Azure AI Foundry and other MCP clients enforce stricter JSON Schema validation than the full JSON Schema spec. The MCP Go SDK (github.com/modelcontextprotocol/go-sdk) uses jsonschema-go which generates schemas with `"additionalProperties": {"not": {}}` by default to disallow extra properties. Go structs must have schema type "object", and clients may reject schemas with complex $ref paths, nested allOf/oneOf, or certain JSON Schema draft features.

**How to avoid:**
- Keep tool input schemas simple - flat structs with basic types (string, int, bool, arrays)
- Avoid nested struct pointers and complex type hierarchies in tool parameters
- Test schema generation with real MCP clients early (Claude, MCP Inspector)
- Use json struct tags to control field names and omitempty behavior
- Avoid optional parameters using pointers - use explicit omitempty with zero values
- Validate generated schemas against JSON Schema Draft 7 (most compatible)
- Include integration tests that verify tool schemas are accepted by MCP clients

**Warning signs:**
- Server connects successfully but tools don't appear in client
- Client logs show schema validation errors with error code -32602
- "Invalid $ref" or "additionalProperties" errors
- Tools work with simple types but fail when structs are nested

**Phase to address:**
Phase 2 (Core Features) - Validate schema compatibility before implementing complex tool parameters.

---

### Pitfall 6: Exponential Backoff Without Jitter Causing Thundering Herd

**What goes wrong:**
When quota is exceeded or rate limits hit, multiple concurrent requests retry at exactly the same time (1s, 2s, 4s, 8s), causing synchronized retry storms that keep hitting rate limits. The server becomes unresponsive and quota never recovers.

**Why it happens:**
Developers implement exponential backoff correctly (doubling delay) but forget jitter (randomization). When multiple goroutines hit rate limits simultaneously, they all wait exactly 1 second, then 2 seconds, then 4 seconds - creating synchronized retry waves. YouTube API returns 403 with "quotaExceeded" but doesn't provide Retry-After headers in all cases, so implementations don't know when quota resets (midnight PT).

**How to avoid:**
- Implement exponential backoff with jitter: `delay = baseDelay * (2^attempt) * (0.5 + rand.Float64()*0.5)`
- Respect Retry-After headers when present
- For quota exceeded errors, implement per-day quota tracking and stop retrying until midnight PT
- Use a shared rate limiter across all API calls (github.com/golang/time/rate)
- Limit max retries (3-5 attempts) before failing gracefully
- Add circuit breaker pattern to stop attempts during extended outages
- Log retry metrics to detect thundering herd patterns

**Warning signs:**
- Spikes in API request logs at exact second intervals
- Multiple concurrent "quota exceeded" errors
- Retries that never succeed despite waiting
- Server becomes unresponsive during retry storms

**Phase to address:**
Phase 2 (Core Features) - Implement before any concurrent operation support.

---

### Pitfall 7: Cookie-Based YouTube Music Access Violating Terms of Service

**What goes wrong:**
Using unofficial ytmusicapi approach with browser cookies works initially but: (1) Violates YouTube Terms of Service, (2) Breaks when Google implements new security mechanisms (PO Tokens required since March 2025), (3) Cookies expire after 2 years or on logout, causing silent failures, (4) No rate limiting guidance leading to IP bans.

**Why it happens:**
Official YouTube Music API doesn't exist, forcing developers toward unofficial solutions. ytmusicapi is well-documented and widely used, making it seem legitimate. The GitHub project has thousands of stars and appears maintained. Developers prioritize "getting it working" over long-term compliance and reliability.

**How to avoid:**
**DO NOT use cookie-based authentication for production MCP server.** This is explicitly an anti-pattern. Instead:
- Scope project to YouTube Data API v3 capabilities (playlist management only)
- Clearly document that listening history is not available due to API limitations
- Wait for official YouTube Music API if listening history is critical
- Consider alternative music services (Spotify, Apple Music) with official APIs
- If prototyping only, clearly mark as unofficial and include ToS warnings

**Warning signs:**
- Requiring users to manually extract browser cookies
- Using packages like ytmusicapi, youtube-music-api
- Handling SAPISID or __Secure-3PAPISID cookies
- PO Token extraction and management

**Phase to address:**
Phase 0 (Research/Planning) - Architectural decision that determines project scope.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcoding OAuth client credentials in code | Faster development setup | Security vulnerability if code is shared/committed | Never - use environment variables or secret manager |
| Single-user token storage (one global token) | Simple implementation | Cannot support multiple users, forces shared rate limits | Only for personal single-user prototypes |
| No quota tracking, rely on API errors | Less code complexity | Users hit quota unexpectedly, poor UX, difficult debugging | Never - quota is the primary constraint |
| Synchronous API calls without timeout | Simpler error handling | Server hangs on API timeouts, no cancellation support | Never - MCP requires proper context handling |
| Skipping MCP Inspector testing | Faster iteration | Protocol violations discovered in production by users | Only in earliest prototyping, must use before Phase 2 |
| Using fmt.Println for debugging instead of proper logging | Quick debugging | Breaks stdio transport, hard to diagnose | Only in non-MCP unit tests |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| YouTube Data API | Assuming quotaExceeded errors are transient and retrying | Track daily quota locally, stop attempts until midnight PT, implement user-facing quota display |
| YouTube Data API | Using search.list for everything | search.list costs 100 units (expensive) - use direct ID lookups when possible (1 unit each) |
| YouTube Activities API | Trying to get listening history from activities.list | activities.list does NOT return music playback - only uploads, likes, comments. No workaround exists via official API |
| OAuth2 golang.org/x/oauth2 | Not persisting refreshed tokens | oauth2.TokenSource automatically refreshes but doesn't persist - wrap with custom TokenSource that saves on refresh |
| MCP stdio transport | Logging to stdout "temporarily" for debugging | Configure logging to stderr from the start: `log.SetOutput(os.Stderr)` before any other code runs |
| MCP tool schemas | Using complex nested Go structs | Keep tool parameters flat, use basic types, test schema generation early with MCP Inspector |
| OAuth2 redirect URI | Using localhost without exact port match | Google OAuth requires exact URI match - `http://localhost:8080/callback` != `http://localhost:8081/callback` |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Making sequential API calls in loops | Slow response times, quota depletion | Batch API calls using YouTube's batch request API (send up to 50 operations in one HTTP request) | 10+ sequential calls (10+ seconds delay) |
| Refreshing OAuth token on every MCP request | Added latency on all operations | Check token expiry, only refresh when needed (5min before expiry), cache token in memory | Every request adds 200-500ms |
| No caching of API responses | Repeated quota costs for same data | Cache playlist metadata, user info with TTL (5-15 min), invalidate on write operations | >50 API calls/day per user |
| Blocking main goroutine waiting for API calls | Server unresponsive during slow API calls | Use context with timeout (10-30s), run API calls in background goroutines with proper cancellation | Any API call >1s blocks entire server |
| Creating new HTTP client for each request | Connection overhead, TLS handshake cost | Reuse oauth2.Client across requests, uses connection pooling automatically | >100 requests/hour shows measurable overhead |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Storing OAuth tokens in plaintext files | Token theft allows full account access | Encrypt token storage, use OS keychain, or system secret manager |
| Committing client_secret.json to git | Anyone with repo access gets API credentials | Use .gitignore for all credential files, environment variables for CI/CD |
| No token revocation on user logout | Stolen tokens remain valid indefinitely | Call Google token revocation endpoint on logout/disconnect |
| Insufficient OAuth scope validation | Requesting more permissions than needed | Use minimum scopes: youtube.readonly for read, youtube.force-ssl only if writing |
| Missing state parameter in OAuth flow | CSRF attacks during authentication | Generate random state value, verify on callback (oauth2.Config handles this automatically) |
| No rate limiting per user | One user can exhaust quota for all | Implement per-user quota tracking and rate limiting |
| Trusting client-provided tool parameters | Injection attacks, resource exhaustion | Validate all parameters, limit string lengths, restrict allowed characters in IDs |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Silent failure when quota exceeded | User thinks tool is broken, no feedback | Return clear error message: "Daily API quota exceeded. Resets at midnight PT (X hours). Current usage: 9,800/10,000 units" |
| Requiring manual OAuth setup | High friction, many users can't complete setup | Provide clear step-by-step guide with screenshots, test OAuth redirect locally before documenting |
| No progress indication for slow operations | User doesn't know if operation is working | Use MCP progress notifications for operations >2 seconds |
| Cryptic YouTube API errors passed through | "Error 403" means nothing to users | Map API errors to user-friendly messages: "quotaExceeded" -> "Daily limit reached" |
| Assuming users understand OAuth permissions | Users deny critical permissions, tool breaks | Explain why each OAuth scope is needed before redirect |
| No way to check quota status | Users can't plan usage | Provide tool to check current quota usage and reset time |

## "Looks Done But Isn't" Checklist

- [ ] **OAuth2 implementation:** Often missing token refresh persistence - verify tokens survive server restart
- [ ] **MCP stdio transport:** Often missing stderr logging config - verify MCP Inspector connects successfully
- [ ] **Error handling:** Often missing user-friendly error messages - verify API errors are mapped to clear messages
- [ ] **Quota tracking:** Often missing local tracking - verify quota warnings before hitting API limit
- [ ] **Tool schemas:** Often missing MCP client validation - verify tools appear and work in Claude, not just Inspector
- [ ] **Context cancellation:** Often missing timeout handling - verify long API calls can be cancelled by client
- [ ] **Concurrent requests:** Often missing race condition handling - verify multiple simultaneous requests don't corrupt state
- [ ] **Token storage:** Often missing atomic updates - verify concurrent token refreshes don't cause invalidation

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Quota exceeded | LOW | Wait for midnight PT reset (automatic), implement quota tracking for future, request quota increase through Google audit (2-4 weeks) |
| Stdout pollution breaking MCP | LOW | Add `log.SetOutput(os.Stderr)` at program start, search codebase for fmt.Print statements and redirect to stderr, test with MCP Inspector |
| Invalid refresh token | MEDIUM | Detect 401/invalid_grant errors, force user re-authentication, implement proper token persistence to prevent recurrence |
| Schema validation failures | MEDIUM | Simplify tool parameter structs, regenerate schemas, test against MCP Inspector and Claude, avoid complex nested types |
| Thundering herd from retries | MEDIUM | Add jitter to retry logic, implement circuit breaker, add per-endpoint rate limiting, monitor retry metrics |
| Building on unofficial API | HIGH | Complete architecture rewrite to use official YouTube Data API only, remove cookie auth, rescope features, update documentation |
| No quota tracking in production | MEDIUM | Add quota middleware, instrument all API calls, implement user-facing quota dashboard, backfill historical usage if possible |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| No official YouTube Music API | Phase 0 (Planning) | Architectural decision document confirms API limitations, README explains scope constraints |
| Quota exceeded unexpectedly | Phase 1 (Foundation) | Quota tracking middleware in place, unit tests verify quota calculation |
| OAuth token not refreshing | Phase 1 (Foundation) | Integration test: server restart preserves auth, token refresh updates storage |
| Stdout breaking MCP protocol | Phase 1 (Foundation) | MCP Inspector connects successfully, no parsing errors in logs |
| Schema validation failures | Phase 2 (Core Features) | All tools load in Claude and MCP Inspector, schema unit tests pass |
| Exponential backoff issues | Phase 2 (Core Features) | Load test shows jittered retries, no synchronized retry storms |
| Cookie-based auth violations | Phase 0 (Planning) | Architectural decision explicitly rejects unofficial APIs |
| No error message mapping | Phase 3 (Polish) | User testing confirms error messages are clear and actionable |

## Sources

**YouTube Data API - Quota and Compliance:**
- [Quota and Compliance Audits | YouTube Data API | Google for Developers](https://developers.google.com/youtube/v3/guides/quota_and_compliance_audits)
- [YouTube API Quota Exceeded? Here's How to Fix It [2026]](https://getlate.dev/blog/youtube-api-limits-how-to-calculate-api-usage-cost-and-fix-exceeded-api-quota)
- [Your Complete Guide to YouTube Data API v3](https://elfsight.com/blog/youtube-data-api-v3-limits-operations-resources-methods-etc/)

**YouTube Music API Limitations:**
- [Does YouTube Music have an API? - Musicfetch](https://musicfetch.io/services/youtube-music/api)
- [GitHub - sigma67/ytmusicapi: Unofficial API for YouTube Music](https://github.com/sigma67/ytmusicapi)
- [Activities | YouTube Data API | Google for Developers](https://developers.google.com/youtube/v3/docs/activities)

**OAuth2 Token Management:**
- [Best Practices | Authorization Resources | Google for Developers](https://developers.google.com/identity/protocols/oauth2/resources/best-practices)
- [OAuth 2.1 Features You Can't Ignore in 2026](https://rgutierrez2004.medium.com/oauth-2-1-features-you-cant-ignore-in-2026-a15f852cb723)
- [OAuth 2.0 - Refresh Token and Rotation](https://hhow09.github.io/blog/oauth2-refresh-token/)
- [oauth2 package - golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2)

**MCP Protocol Implementation:**
- [Error Handling And Debugging MCP Servers - Stainless MCP Portal](https://www.stainless.com/mcp/error-handling-and-debugging-mcp-servers)
- [Debugging Model Context Protocol (MCP) Servers](https://www.mcpevals.io/blog/debugging-mcp-servers-tips-and-best-practices)
- [Demystifying LLM MCP Servers: Debugging stdio Transports Like a Pro](https://jianliao.github.io/blog/debug-mcp-stdio-transport)

**MCP JSON Schema:**
- [A JSON schema package for Go | Google Open Source Blog](https://opensource.googleblog.com/2026/01/a-json-schema-package-for-go.html)
- [Tool schema validation errors · Issue #181](https://github.com/Softeria/ms-365-mcp-server/issues/181)
- [mcp package - github.com/modelcontextprotocol/go-sdk](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp)

**Rate Limiting and Exponential Backoff:**
- [How to Handle API Rate Limits Gracefully (2026 Guide)](https://apistatuscheck.com/blog/how-to-handle-api-rate-limits)
- [Dealing with Rate Limiting Using Exponential Backoff](https://substack.thewebscraping.club/p/rate-limit-scraping-exponential-backoff)

**YouTube Music Cookie Authentication Security:**
- [YTMusicAPI Browser Authentication: A Simple Guide](https://vault.nimc.gov.ng/blog/ytmusicapi-browser-authentication-a-simple-guide-1767647885)
- [GitHub - sigma67/ytmusicapi issues](https://github.com/sigma67/ytmusicapi/issues/10)

---
*Pitfalls research for: YouTube Music MCP Server*
*Researched: 2026-02-13*
*Confidence: MEDIUM - Based on official YouTube Data API documentation (HIGH confidence), OAuth2 best practices (HIGH confidence), MCP protocol documentation (MEDIUM confidence), and community experience reports (MEDIUM-LOW confidence)*
