---
phase: quick
plan: 1
type: execute
wave: 1
depends_on: []
files_modified: []
autonomous: true
requirements: []

must_haves:
  truths:
    - "User understands how to build and run the MCP server"
    - "User understands how to test each MCP tool"
  artifacts: []
  key_links: []
---

<objective>
Answer: How to run and test this YouTube Music MCP server.

This is an informational response -- no code changes needed.
</objective>

<context>
This is a Go MCP server that connects to the YouTube Data API via OAuth2.
It exposes 7 MCP tools for music taste analysis and playlist management.
</context>

<answer>

## How to Run

### Prerequisites

1. **Go 1.25.6+** installed
2. **Google OAuth2 credentials** from Google Cloud Console (APIs & Services > Credentials)
   - Create an "OAuth 2.0 Client ID" (type: Desktop app or Web app)
   - Enable the "YouTube Data API v3" in your project

### Setup

1. Copy `.env.example` to `.env` and fill in your credentials:
   ```
   GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=your-client-secret
   ```

2. Build the binary:
   ```bash
   go build -o bin/youtube-music-mcp ./cmd/server
   ```

3. Run the server directly (for initial OAuth flow):
   ```bash
   ./bin/youtube-music-mcp
   ```
   - On first run, it prints an OAuth URL to stderr -- open it in your browser
   - Authorize the app; it saves the token to `~/.config/youtube-music-mcp/token.json`
   - After auth, the MCP server starts on stdio (JSON-RPC protocol)

### Running with Claude Code

The `.mcp.json` is already configured. Claude Code reads this file and launches the server automatically. Once authenticated (token saved), it works seamlessly.

You can also add it to Claude Desktop's config (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "youtube-music": {
      "command": "/full/path/to/bin/youtube-music-mcp",
      "env": {
        "GOOGLE_CLIENT_ID": "your-id",
        "GOOGLE_CLIENT_SECRET": "your-secret"
      }
    }
  }
}
```

## How to Test

### Build verification
```bash
go build ./...
go vet ./...
```

There are **no unit tests** in the codebase currently. All testing is manual/integration via MCP tool calls.

### Manual testing via Claude

Once the server is running as an MCP server in Claude Code, you can test each tool by asking Claude to use them:

| Tool | Test prompt | Quota cost |
|------|------------|------------|
| `get_liked_videos` | "Show me my liked videos" | ~2 units |
| `list_playlists` | "List my YouTube playlists" | ~1 unit |
| `get_playlist_items` | "Show tracks in playlist {id}" | ~1 unit |
| `get_subscriptions` | "Show my YouTube subscriptions" | ~1 unit |
| `search_videos` | "Search for 'Radiohead Creep'" | **100 units** |
| `get_video` | "Look up video {videoId}" | 1 unit |
| `create_playlist` | "Create a playlist called 'Test'" | **50 units** |
| `add_to_playlist` | "Add video {id} to playlist {id}" | **50 units/video** |

### Manual testing via MCP Inspector

You can also use the [MCP Inspector](https://github.com/modelcontextprotocol/inspector):
```bash
npx @modelcontextprotocol/inspector ./bin/youtube-music-mcp
```

### Quota awareness

Daily quota limit is **10,000 units**. Search (100 units) and playlist writes (50 units) are expensive. Reads are cheap (1-2 units).

### Re-authenticating

Delete the saved token to force re-auth:
```bash
rm ~/.config/youtube-music-mcp/token.json
```

</answer>

<success_criteria>
User understands how to build, run, authenticate, and test each MCP tool.
</success_criteria>
