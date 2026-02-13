# Technology Stack

**Project:** YouTube Music MCP Server
**Researched:** 2026-02-13
**Confidence:** HIGH

## Recommended Stack

### Core Framework

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go | 1.26+ | Runtime and language | Latest stable release (Feb 2026) with best performance, tooling, and security. Go 1.21+ required for log/slog. Use 1.26 for latest stdlib improvements. |
| modelcontextprotocol/go-sdk | v1.3.0+ | Official MCP server SDK | Official SDK maintained by Anthropic in collaboration with Google. Production-ready with 819 projects depending on it. Supports MCP spec 2025-11-25 and backward compatible. |
| google.golang.org/api/youtube/v3 | v0.266.0+ | YouTube Data API v3 client | Google's official Go client for YouTube API. Maintained, stable, BSD-licensed. Provides full access to playlists, activities, channels, and search. |
| golang.org/x/oauth2/google | v0.35.0+ | OAuth2 authentication | Standard library extension for Google OAuth2. Supports web server flow, service accounts, and Application Default Credentials. Security-vetted and actively maintained. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| log/slog | stdlib (Go 1.21+) | Structured logging | Always. Standard library solution introduced in Go 1.21. Zero external dependencies, JSON/text handlers, context support, dynamic level control. |
| joho/godotenv | v1.5.1+ | Environment variable loading | Development only. Load .env files for local testing. Feature complete, familiar to developers from other ecosystems. |
| caarlos0/env | v11.0.0+ | Env var parsing to structs | Always. Simple, efficient struct-based config with env tags. Pair with godotenv for complete solution. |
| hashicorp/go-retryablehttp | v0.7.7+ | HTTP retry with backoff | Always for YouTube API calls. Production-ready retry logic with exponential backoff. Used by Terraform, Vault, Consul. Handles 429 rate limits automatically. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| go test | Unit testing | Built-in testing framework. Use table-driven tests. |
| httptest | HTTP mocking | Standard library package for testing HTTP clients/servers. |
| httpmock (jarcoal/httpmock) | Advanced HTTP mocking | For YouTube API client tests. Easy setup of canned responses. |
| go vet | Static analysis | Built-in linter. Run before commits. |
| gofmt / goimports | Code formatting | goimports preferred (formats + manages imports). |

## Installation

```bash
# Initialize Go module (if not already done)
go mod init github.com/gxravel/youtube-music-mcp

# Core dependencies
go get github.com/modelcontextprotocol/go-sdk@latest
go get google.golang.org/api/youtube/v3@latest
go get golang.org/x/oauth2/google@latest

# Supporting libraries
go get github.com/caarlos0/env/v11@latest
go get github.com/hashicorp/go-retryablehttp@latest

# Development dependencies (not imported in main code)
go get github.com/joho/godotenv@latest
go get github.com/jarcoal/httpmock@latest
```

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| MCP SDK | modelcontextprotocol/go-sdk | mark3labs/mcp-go | mark3labs has 8.2k stars and good DX, but official SDK is maintained by Anthropic+Google and has better long-term support guarantees. Use official for production. |
| Logging | log/slog | zap, zerolog | For high-performance needs, zap/zerolog are faster. But slog is stdlib (no deps), good enough performance, and reduces dependency surface area. Prefer stdlib unless profiling shows logging bottleneck. |
| Config | caarlos0/env + godotenv | viper, koanf | Viper is feature-rich but heavy (file watching, remote config). For MCP server, simple env vars are sufficient. Lighter stack = fewer CVEs to track. |
| OAuth2 | golang.org/x/oauth2/google | Third-party OAuth libs | Use official Google OAuth library. Security-critical code should come from authoritative source. |
| HTTP Retry | hashicorp/go-retryablehttp | Custom retry logic | Don't roll your own. HashiCorp's library is battle-tested across Terraform/Vault. Handles edge cases you'll miss. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Logrus | In maintenance mode. No new features. | log/slog (stdlib) or zap if performance critical |
| SSE transport for MCP | Deprecated. Streamable HTTP is modern standard. | Stdio (local) or Streamable HTTP (remote) |
| localStorage for tokens (if building web UI) | Security vulnerability. Tokens exposed to XSS. | Server-side session storage or BFF pattern |
| YouTube Music API | No official Music API. YouTube Music uses same backend as YouTube. | YouTube Data API v3 with appropriate scopes |
| go get without version constraints | Reproducibility issues, breaking changes in deps. | Use go get package@version or go.mod replace directives |

## Transport Selection

**For this project: Use Stdio transport**

| Transport | When to Use | Why |
|-----------|-------------|-----|
| **Stdio** (RECOMMENDED) | Local CLI integration with Claude Desktop | Microsecond latency, no network overhead, simpler auth (runs as user). Perfect for desktop MCP servers. |
| Streamable HTTP | Remote access, multiple clients, cloud deployment | Adds network layer, requires TLS, auth headers. Overkill for single-user desktop tool. |
| SSE | Legacy compatibility only | Deprecated in favor of Streamable HTTP. Don't use for new projects. |

## OAuth2 Token Storage Strategy

**Recommended approach for Stdio MCP server:**

```
1. Initial auth flow:
   - Server runs `oauth2.Config.AuthCodeURL()` → prints URL to stderr
   - User opens browser, grants access
   - Local callback server captures auth code
   - Exchange for tokens

2. Token persistence:
   - Store tokens in ~/.config/youtube-music-mcp/token.json
   - File permissions: 0600 (user read/write only)
   - Encrypt at rest using OS keyring (future enhancement)

3. Token refresh:
   - Check token expiry before each API call
   - Auto-refresh using refresh_token if expired
   - Save new tokens atomically (write to temp, rename)
```

**Security notes:**
- Never commit token files to git
- Never log token values (use slog.LogValuer to redact)
- Set .env and token.json in .gitignore
- Refresh tokens are long-lived; revoke via Google Console if compromised

## YouTube API Quota Management

YouTube Data API v3 has quota limits (10,000 units/day default):
- Playlist.list: 1 unit
- PlaylistItems.list: 1 unit
- Activities.list: 1 unit (for history)
- Search.list: 100 units (expensive!)

**Mitigation:**
- Cache playlist metadata (changes infrequently)
- Use Activities.list for history, not Search
- Implement request deduplication
- Monitor quota usage via Google Cloud Console
- Consider requesting quota increase if >10k units needed

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| go-sdk v1.3.0 | MCP spec 2025-11-25, 2025-06-18, 2024-11-05 | Backward compatible with older specs |
| youtube/v3 v0.266.0 | oauth2 v0.35.0+ | Both from google.golang.org/api family |
| Go 1.26 | All listed packages | Minimum Go 1.21 for log/slog |
| go-retryablehttp v0.7.7 | net/http (stdlib) | Drop-in replacement for http.Client |

## Stack Patterns by Variant

**If building for cloud deployment (future):**
- Switch transport: Stdio → Streamable HTTP
- Add: TLS termination (Let's Encrypt)
- Add: API key auth middleware
- Add: Rate limiting per client
- Consider: Vault for secret management

**If performance becomes critical:**
- Switch logger: log/slog → zap
- Add: Connection pooling (built into google api client)
- Add: Response caching (gcache or go-cache)
- Profile first, optimize second

**If multi-user support needed:**
- Add: User session management
- Add: Token encryption at rest
- Add: Database for user mappings (SQLite for small scale)
- Separate: Auth server from MCP server

## Sources

### Official Documentation (HIGH confidence)
- [Go SDK for MCP - GitHub](https://github.com/modelcontextprotocol/go-sdk) - v1.3.0, production-ready status
- [Go SDK for MCP - pkg.go.dev](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) - API reference
- [YouTube Data API v3 - Google Developers](https://developers.google.com/youtube/v3) - Official API docs
- [YouTube Go Quickstart](https://developers.google.com/youtube/v3/quickstart/go) - Setup guide
- [youtube/v3 package - pkg.go.dev](https://pkg.go.dev/google.golang.org/api/youtube/v3) - v0.266.0
- [oauth2/google package - pkg.go.dev](https://pkg.go.dev/golang.org/x/oauth2/google) - v0.35.0
- [log/slog - pkg.go.dev](https://pkg.go.dev/log/slog) - Go 1.21+ stdlib
- [Go 1.26 Release Notes](https://go.dev/doc/go1.26) - Latest stable (Feb 2026)
- [OAuth2 Best Practices - Google](https://developers.google.com/identity/protocols/oauth2/resources/best-practices) - Security guidance

### Verified Community Sources (MEDIUM-HIGH confidence)
- [Building MCP Server in Go - Navendu Pottekkat](https://navendu.me/posts/mcp-server-go/) - Patterns and examples
- [MCP Best Practices 2026 - CData](https://www.cdata.com/blog/mcp-server-best-practices-2026) - Production guidelines
- [15 Best Practices for MCP Servers - The New Stack](https://thenewstack.io/15-best-practices-for-building-mcp-servers-in-production/) - Architecture patterns
- [MCP Transport Comparison - MCPcat](https://mcpcat.io/guides/comparing-stdio-sse-streamablehttp/) - Transport selection
- [Go Logging Best Practices 2026 - Reliable Software](https://reliasoftware.com/blog/golang-logging-libraries) - Logger comparison
- [Go Retry with Exponential Backoff - OneUpTime](https://oneuptime.com/blog/post/2026-01-07-go-retry-exponential-backoff/view) - Retry strategies
- [godotenv - GitHub](https://github.com/joho/godotenv) - Most popular .env loader
- [httpmock - GitHub](https://github.com/jarcoal/httpmock) - HTTP mocking for tests

### Additional Research (MEDIUM confidence)
- [MCP Stdio vs SSE - Medium](https://medium.com/@vkrishnan9074/mcp-clients-stdio-vs-sse-a53843d9aabb) - Transport tradeoffs
- [OAuth2 Token Refresh - OneUpTime](https://oneuptime.com/blog/post/2026-01-24-oauth2-token-refresh/view) - Token handling
- [Go Testing with Mocking - Speedscale](https://speedscale.com/blog/testing-golang-with-httptest/) - Test patterns

---
*Stack research for: YouTube Music MCP Server*
*Researched: 2026-02-13*
*Confidence: HIGH (official docs + official SDKs + verified community sources)*
