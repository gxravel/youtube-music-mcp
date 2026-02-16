---
phase: 03-playlist-management
plan: 01
subsystem: playlist-creation
tags: [youtube-api, mcp-tools, playlist-management]
dependency_graph:
  requires: [youtube-client, mcp-server, oauth2-auth]
  provides: [playlist-creation-api, video-addition-api]
  affects: [end-to-end-workflow]
tech_stack:
  added: []
  patterns: [duplicate-skip-pattern, privacy-validation, youtube-music-urls]
key_files:
  created: []
  modified:
    - path: internal/youtube/playlists.go
      lines_added: 101
      significance: Implements CreatePlaylist and AddVideosToPlaylist methods with validation and error handling
    - path: internal/server/tools_playlists.go
      lines_added: 83
      significance: Registers create_playlist and add_to_playlist MCP tools with schema validation
decisions:
  - choice: "Skip duplicate videos silently (HTTP 409)"
    rationale: "Better UX - users don't need to manually dedup, YouTube API returns 409 for duplicates"
    alternatives: ["Return error on duplicate", "Pre-check with GetPlaylistItems"]
  - choice: "Default privacy to 'private'"
    rationale: "Safe default - users can intentionally make playlists public, avoids accidental exposure"
    alternatives: ["Require privacy parameter", "Default to 'unlisted'"]
  - choice: "Return YouTube Music URLs instead of regular YouTube URLs"
    rationale: "Project targets YouTube Music users, music.youtube.com is the correct interface"
    alternatives: ["Regular YouTube URLs", "Both URLs"]
metrics:
  duration_minutes: 2
  tasks_completed: 2
  files_modified: 2
  lines_added: 184
  commits: 2
  completed_date: 2026-02-16
---

# Phase 03 Plan 01: Playlist Creation and Video Addition Summary

**One-liner:** Implement playlist creation and batch video addition with duplicate skip logic, enabling Claude to deliver recommendations as ready-to-play YouTube Music playlists.

## What Was Built

### YouTube Client Methods (internal/youtube/playlists.go)

**CreatePlaylist(ctx, title, description, privacyStatus) (\*Playlist, error):**
- Validates title non-empty, returns error if blank
- Defaults privacyStatus to "private" if not specified
- Validates privacyStatus is one of: "public", "private", "unlisted"
- Creates playlist via YouTube Data API with Snippet (title, description) and Status (privacy)
- Returns domain Playlist type with ID, Title, Description, ItemCount=0
- Quota cost: 50 units (documented in comment)

**AddVideosToPlaylist(ctx, playlistID, videoIDs) (int, error):**
- Validates playlistID and videoIDs non-empty
- Loops over video IDs, inserting each into playlist
- Checks ctx.Err() for cancellation on each iteration
- Detects duplicate videos via HTTP 409 or "videoAlreadyInPlaylist" message
- Skips duplicates silently (continue loop)
- Returns count of successfully added videos
- On non-duplicate errors: returns partial success count + wrapped error
- Quota cost: 50 units per video (documented in comment)

**Error handling:**
- Uses `errors.As(err, &apiErr)` to check for `*googleapi.Error`
- Distinguishes duplicate errors (409) from other API failures
- Wraps all errors with context (failed to create playlist, failed to add video X)

### MCP Tools (internal/server/tools_playlists.go)

**Tool: create_playlist**
- Input: title (required), description (optional), privacyStatus (enum: public/private/unlisted, defaults to private)
- Output: playlistId, title, description, url (YouTube Music link)
- Description includes quota cost (50 units)
- Text response: "Created playlist '{title}' (ID: {id})\nURL: {url}"

**Tool: add_to_playlist**
- Input: playlistId (required), videoIds (required array)
- Output: added (count), total (count), playlistUrl (YouTube Music link)
- Description mentions duplicate skip behavior and quota cost (50 units per video)
- Text response: "Added {N} of {total} videos to playlist\nURL: {url}"

**Schema validation:**
- jsonschema tags enforce required fields (title, playlistId, videoIds)
- Privacy status uses enum constraint (public, private, unlisted)
- Clear descriptions reference related tools (search_videos, list_playlists)

## End-to-End Flow Complete

This plan completes the project's core workflow:

1. **Phase 1:** OAuth2 authentication + token persistence → User authenticated
2. **Phase 2:** Taste data access (liked videos, playlists, subscriptions) → Claude learns user preferences
3. **Phase 2:** Search and video lookup → Claude finds new tracks matching user taste
4. **Phase 3 (this plan):** Playlist creation + video addition → Claude delivers recommendations as playable playlist

**Result:** Claude can now analyze listening history, search for similar/interesting music, create a custom playlist, and populate it with tracks — all through the MCP interface.

## Deviations from Plan

None - plan executed exactly as written.

## Technical Highlights

**Duplicate skip pattern:**
- API returns HTTP 409 when video already in playlist
- Error message contains "videoAlreadyInPlaylist"
- Both conditions checked via `errors.As` and `strings.Contains`
- Silent skip allows batch operations without pre-checking playlist contents

**Privacy validation:**
- Map-based validation: `validPrivacy := map[string]bool{"public": true, "private": true, "unlisted": true}`
- Clear error message lists valid options
- Default to "private" for safety (avoids accidental public exposure)

**YouTube Music URLs:**
- Format: `https://music.youtube.com/playlist?list={playlistID}`
- Returned in both tool outputs for immediate user access
- Aligns with project focus (YouTube Music, not regular YouTube)

**Quota awareness:**
- CreatePlaylist: 50 units (fixed cost)
- AddVideosToPlaylist: 50 units per video (scales with batch size)
- Costs documented in method comments AND tool descriptions
- Users see quota impact before using tools

## Verification Results

All verification checks passed:

1. ✅ `go build ./...` compiles successfully
2. ✅ `go vet ./...` passes with no issues
3. ✅ CreatePlaylist method exists at line 202 of internal/youtube/playlists.go
4. ✅ AddVideosToPlaylist method exists at line 248 of internal/youtube/playlists.go
5. ✅ create_playlist tool registered at line 177 of internal/server/tools_playlists.go
6. ✅ add_to_playlist tool registered at line 207 of internal/server/tools_playlists.go
7. ✅ Existing tools (get_liked_videos, list_playlists, get_playlist_items, search_videos, get_video, list_subscriptions) still compile and register

## Commits

| Task | Description | Commit | Files Modified |
|------|-------------|--------|----------------|
| 1 | Implement YouTube client methods | 7d78c71 | internal/youtube/playlists.go |
| 2 | Register MCP tools | ed3746c | internal/server/tools_playlists.go |

## What's Next

Phase 3 complete. All planned functionality implemented:
- Playlist read operations (list_playlists, get_playlist_items) ✅
- Playlist write operations (create_playlist, add_to_playlist) ✅

**Project status:** Core workflow complete. Claude can now:
- Authenticate with YouTube (Phase 1)
- Access user taste data (Phase 2)
- Search for music (Phase 2)
- Create and populate playlists (Phase 3)

**Potential enhancements (future work):**
- Remove videos from playlist
- Reorder playlist items
- Update playlist metadata (title, description, privacy)
- Delete playlists
- Playlist collaboration features

## Self-Check: PASSED

**Created files:** None (only modifications to existing files)

**Modified files:**
- FOUND: internal/youtube/playlists.go (101 lines added)
- FOUND: internal/server/tools_playlists.go (83 lines added)

**Commits:**
- FOUND: 7d78c71 (feat(03-01): implement YouTube client methods for playlist creation and video addition)
- FOUND: ed3746c (feat(03-01): register MCP tools for playlist creation and video addition)

**Methods exist:**
- FOUND: CreatePlaylist at line 202 of internal/youtube/playlists.go
- FOUND: AddVideosToPlaylist at line 248 of internal/youtube/playlists.go

**Tools registered:**
- FOUND: create_playlist tool at line 177 of internal/server/tools_playlists.go
- FOUND: add_to_playlist tool at line 207 of internal/server/tools_playlists.go

All claims verified successfully.
