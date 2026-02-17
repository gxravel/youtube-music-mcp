---
phase: quick-2
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/server/tools_recommend.go
  - internal/server/tools_analyze.go
  - internal/server/server.go
  - internal/server/tools_playlists.go
  - internal/server/tools_search.go
  - internal/server/tools_subscriptions.go
autonomous: true
requirements: [REDESIGN-01, REDESIGN-02, REDESIGN-03, REDESIGN-04]

must_haves:
  truths:
    - "MCP server exposes exactly 4 tools: ym:recommend-playlist, ym:analyze-my-tastes, ym:recommend-artists, ym:recommend-albums"
    - "Old 8 granular tools (get_liked_videos, list_playlists, etc.) are no longer registered as MCP tools"
    - "YouTube API client methods (GetLikedVideos, ListPlaylists, SearchVideos, etc.) remain unchanged and are used internally"
    - "ym:recommend-playlist gathers taste data, searches for songs, creates playlist, and adds songs in one call"
    - "ym:analyze-my-tastes returns structured taste analysis from liked videos, playlists, and subscriptions"
    - "ym:recommend-artists and ym:recommend-albums return recommendation lists based on taste data"
    - "All tools return well-structured text content that Claude can present to the user"
  artifacts:
    - path: "internal/server/tools_recommend.go"
      provides: "ym:recommend-playlist, ym:recommend-artists, ym:recommend-albums tools"
    - path: "internal/server/tools_analyze.go"
      provides: "ym:analyze-my-tastes tool"
    - path: "internal/server/server.go"
      provides: "Updated registration calling new tool registrations"
  key_links:
    - from: "internal/server/tools_recommend.go"
      to: "internal/youtube/playlists.go"
      via: "s.ytClient.GetLikedVideos, s.ytClient.ListPlaylists, s.ytClient.GetPlaylistItems, s.ytClient.SearchVideos, s.ytClient.CreatePlaylist, s.ytClient.AddVideosToPlaylist"
      pattern: "s\\.ytClient\\."
    - from: "internal/server/tools_analyze.go"
      to: "internal/youtube/playlists.go"
      via: "s.ytClient.GetLikedVideos, s.ytClient.ListPlaylists, s.ytClient.GetSubscriptions"
      pattern: "s\\.ytClient\\."
---

<objective>
Redesign MCP tool surface from 8 granular tools into 4 high-level commands that orchestrate multiple YouTube API calls internally.

Purpose: The current 8 low-level tools require the LLM client to make many round-trips (fetch taste, search, create playlist, add songs). The new 4 tools encapsulate entire workflows so the LLM makes ONE tool call per user intent. This dramatically reduces latency, token usage, and failure points.

Output: 4 new MCP tools registered, 8 old tools removed, YouTube client layer unchanged.
</objective>

<execution_context>
@/home/gxravel/.claude/get-shit-done/workflows/execute-plan.md
@/home/gxravel/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/server/server.go
@internal/server/tools_playlists.go
@internal/server/tools_search.go
@internal/server/tools_subscriptions.go
@internal/youtube/client.go
@internal/youtube/playlists.go
@internal/youtube/search.go
@internal/youtube/subscriptions.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create the 4 high-level MCP tools</name>
  <files>
    internal/server/tools_recommend.go
    internal/server/tools_analyze.go
  </files>
  <action>
Create two new files containing the 4 high-level MCP tools. Keep all existing YouTube client methods — they are called internally by these new tools.

**File: internal/server/tools_analyze.go**

Create `registerAnalyzeTools()` on `*Server` that registers:

1. **ym:analyze-my-tastes** — Analyzes the user's YouTube Music taste.
   - Input struct: `analyzeTastesInput` with field `IncludePreviousRecommendations bool` (optional, jsonschema description: "If true, also fetch songs from playlists previously created by this tool to adjust analysis")
   - Implementation:
     - Fetch liked videos via `s.ytClient.GetLikedVideos(ctx, 200)` (get a good sample)
     - Fetch subscriptions via `s.ytClient.GetSubscriptions(ctx, 100)`
     - Fetch user's playlists via `s.ytClient.ListPlaylists(ctx, 50)`
     - If `IncludePreviousRecommendations` is true, for each playlist whose title starts with "[YM-MCP]", fetch its items via `s.ytClient.GetPlaylistItems` (up to 100 each)
     - Build a structured text response containing:
       - Section "Liked Songs" — list each song as "Title - ChannelTitle"
       - Section "Subscribed Channels" — list each channel title
       - Section "Your Playlists" — list each playlist title with item count
       - If previous recommendations fetched, section "Previously Recommended" with those songs
     - Return as `mcp.TextContent` (NOT structured JSON output — just text). The LLM client will interpret this text to form taste understanding.
   - Output type: just use `any` (nil) for the typed output since we return text only. Use the pattern: `func(ctx context.Context, req *mcp.CallToolRequest, input analyzeTastesInput) (*mcp.CallToolResult, any, error)`

**File: internal/server/tools_recommend.go**

Create `registerRecommendTools()` on `*Server` that registers 3 tools:

2. **ym:recommend-playlist** — Creates a playlist with recommended music.
   - Input struct: `recommendPlaylistInput` with fields:
     - `NumberOfSongs int` (required, min 1, max 50, jsonschema description: "Number of songs to find and add to the playlist")
     - `Description string` (optional, jsonschema description: "What kind of music to find — genres, moods, artists, era, or any description. If empty, recommendations are based purely on taste analysis.")
   - Implementation:
     - First gather taste context: fetch liked videos (50), subscriptions (50), playlist names (25) — same as analyze but lighter
     - Build a taste summary string from this data (compact: just artist names from liked songs + subscription names, deduplicated)
     - Construct search queries based on the taste summary + user's description. Strategy:
       - Extract unique artist/channel names from liked videos (top 10 most frequent)
       - If description provided, use it to build search queries like "{description} {artist}" for variety
       - If no description, search for "music similar to {artist}" or just search by top artist names
       - Run `s.ytClient.SearchVideos` for each query (limit to ceil(NumberOfSongs/5) queries, 10 results each, to stay within quota). IMPORTANT: each search costs 100 quota units — be conservative.
     - Collect video IDs, deduplicate, take first `NumberOfSongs` videos
     - Create playlist via `s.ytClient.CreatePlaylist` with title "[YM-MCP] {generated title}" (e.g., "[YM-MCP] Indie Rock Mix" or "[YM-MCP] Chill Evening" based on description, or "[YM-MCP] Recommended Mix" if no description), privacy "private"
     - Add videos via `s.ytClient.AddVideosToPlaylist`
     - Return text with: playlist name, YouTube Music URL, list of added songs (title - channel), quota usage estimate
   - Output type: `any` (nil), text-only response

3. **ym:recommend-artists** — Recommends artists the user would like.
   - Input struct: `recommendArtistsInput` with field:
     - `Description string` (optional, jsonschema description: "What kind of artists to recommend — genre preferences, mood, or any guidance")
   - Implementation:
     - Gather taste: liked videos (100), subscriptions (100)
     - Extract unique artist/channel names from liked videos
     - Build a structured text response containing:
       - Section "Your Current Artists" — deduplicated list of artists from liked songs + subscriptions
       - Section "Taste Profile" — summary of genres/styles based on artist names (just list the raw data, let the LLM interpret)
       - Note: "Based on this taste data, recommend artists the user hasn't heard. The artists listed above are already known to the user."
     - Return as text. The LLM client will use its own knowledge to generate recommendations based on this taste data.
   - This tool does NOT search YouTube — it provides taste data for the LLM to make recommendations from its own knowledge.

4. **ym:recommend-albums** — Recommends albums the user would like.
   - Input struct: `recommendAlbumsInput` with field:
     - `Description string` (optional, jsonschema description: "What kind of albums to recommend — genre preferences, mood, era, or any guidance")
   - Implementation: Same pattern as recommend-artists — gather taste data, return structured text for LLM to interpret.
   - Return as text with same sections as recommend-artists plus note about recommending albums.

IMPORTANT design notes:
- All tools use `mcp.AddTool` with the same pattern as existing tools
- For tools that only return text (no structured output), use `any` as the output type and return `nil` for the output value
- Tool names use colons: "ym:recommend-playlist" not "ym_recommend_playlist"
- Keep quota usage conservative — document estimated costs in tool descriptions
- Handle errors gracefully — if search fails for one query, continue with others
- Use `fmt.Sprintf` to build readable text sections with newlines
  </action>
  <verify>
Run `go build ./...` — must compile with no errors. Verify the new files exist and contain the 4 tool registrations.
  </verify>
  <done>
Two new files exist with 4 tool handler functions. Code compiles. Each tool gathers taste data from YouTube client methods and returns structured text responses.
  </done>
</task>

<task type="auto">
  <name>Task 2: Remove old tools and update server registration</name>
  <files>
    internal/server/server.go
    internal/server/tools_playlists.go
    internal/server/tools_search.go
    internal/server/tools_subscriptions.go
  </files>
  <action>
1. **Delete** `internal/server/tools_playlists.go` — all playlist MCP tool registrations (get_liked_videos, list_playlists, get_playlist_items, create_playlist, add_to_playlist) are removed. The YouTube client methods they called (GetLikedVideos, ListPlaylists, etc.) remain in `internal/youtube/playlists.go` unchanged.

2. **Delete** `internal/server/tools_search.go` — search_videos and get_video MCP tools removed. Client methods remain in `internal/youtube/search.go`.

3. **Delete** `internal/server/tools_subscriptions.go` — get_subscriptions MCP tool removed. Client method remains in `internal/youtube/subscriptions.go`.

4. **Update** `internal/server/server.go`:
   - Replace the 3 old registration calls:
     ```go
     s.registerPlaylistTools()
     s.registerSubscriptionTools()
     s.registerSearchTools()
     ```
   - With the 2 new registration calls:
     ```go
     s.registerAnalyzeTools()
     s.registerRecommendTools()
     ```
   - No other changes to server.go needed.

5. Run `go build ./...` to confirm clean compilation.
6. Run `go vet ./...` to check for issues.
  </action>
  <verify>
Run `go build ./...` — compiles cleanly. Run `go vet ./...` — no warnings. Verify old tool files are deleted: `ls internal/server/tools_*.go` should show only `tools_recommend.go` and `tools_analyze.go`. Verify no references to old tool names remain: `grep -r "get_liked_videos\|list_playlists\|get_playlist_items\|search_videos\|get_video\|get_subscriptions\|create_playlist\|add_to_playlist" internal/server/` should return nothing.
  </verify>
  <done>
Old 3 tool files deleted. server.go updated with new registrations. Project compiles and vets cleanly. Only 4 MCP tools exposed: ym:recommend-playlist, ym:analyze-my-tastes, ym:recommend-artists, ym:recommend-albums.
  </done>
</task>

</tasks>

<verification>
1. `go build ./...` passes
2. `go vet ./...` passes
3. `ls internal/server/tools_*.go` shows exactly: tools_analyze.go, tools_recommend.go
4. `ls internal/youtube/*.go` still contains: client.go, playlists.go, search.go, subscriptions.go (unchanged)
5. No references to old tool names in server package
</verification>

<success_criteria>
- MCP server registers exactly 4 tools: ym:recommend-playlist, ym:analyze-my-tastes, ym:recommend-artists, ym:recommend-albums
- All 8 old granular tools are removed from MCP registration
- All YouTube API client methods are preserved and used internally by new tools
- Project compiles and vets cleanly
- Each new tool gathers taste data internally and returns text responses
</success_criteria>

<output>
After completion, create `.planning/quick/2-redesign-mcp-tools-4-high-level-commands/2-SUMMARY.md`
</output>
