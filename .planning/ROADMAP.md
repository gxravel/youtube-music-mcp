# Roadmap: YouTube Music MCP

## Overview

Build an MCP server in Go that gives Claude access to YouTube Music data and playlist creation. Start with foundation layer (MCP protocol, OAuth2, API integration), then add data access capabilities (reading user taste data and searching tracks), and finish with playlist management (creating and populating playlists). Each phase delivers a complete, testable capability that builds toward the core value: Claude analyzing listening patterns and delivering recommendations as playable playlists.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation & Authentication** - MCP server core with OAuth2 flow and YouTube API client
- [ ] **Phase 2: Data Access** - Retrieve user taste data and search YouTube Music tracks
- [ ] **Phase 3: Playlist Management** - Create and populate playlists on user's account

## Phase Details

### Phase 1: Foundation & Authentication
**Goal**: MCP server running with YouTube API access via OAuth2
**Depends on**: Nothing (first phase)
**Requirements**: AUTH-01, AUTH-02, AUTH-03
**Success Criteria** (what must be TRUE):
  1. MCP server starts successfully and communicates via stdio transport
  2. User can authenticate with Google account through OAuth2 web flow
  3. OAuth2 tokens persist across server restarts and refresh automatically when expired
  4. Server can make authenticated YouTube API calls (tested with basic API query)
**Plans**: 2 plans

Plans:
- [x] 01-01-PLAN.md — Project scaffolding, config, and OAuth2 auth package
- [x] 01-02-PLAN.md — YouTube client, MCP server wiring, and end-to-end verification

### Phase 2: Data Access
**Goal**: Claude can retrieve user's YouTube Music taste data and search for tracks
**Depends on**: Phase 1
**Requirements**: TASTE-01, TASTE-02, TASTE-03, SRCH-01, SRCH-02
**Success Criteria** (what must be TRUE):
  1. Claude can retrieve user's liked videos/songs from YouTube
  2. Claude can retrieve user's playlists with track contents
  3. Claude can retrieve user's channel subscriptions
  4. Claude can search YouTube Music for tracks by artist, name, or query
  5. Claude can verify whether a specific track exists on YouTube Music
**Plans**: TBD

Plans:
- TBD (created during plan-phase)

### Phase 3: Playlist Management
**Goal**: Claude can create playlists and populate them with tracks
**Depends on**: Phase 2
**Requirements**: PLST-01, PLST-02, PLST-03
**Success Criteria** (what must be TRUE):
  1. Claude can create a new playlist on user's YouTube Music account with custom name and description
  2. Claude can add tracks to an existing playlist by video ID
  3. Claude can list user's existing playlists to avoid duplicates or manage existing ones
  4. Created playlists appear immediately in user's YouTube Music interface
**Plans**: TBD

Plans:
- TBD (created during plan-phase)

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation & Authentication | 2/2 | ✓ Complete | 2026-02-13 |
| 2. Data Access | 0/TBD | Not started | - |
| 3. Playlist Management | 0/TBD | Not started | - |

---
*Roadmap created: 2026-02-13*
*Last updated: 2026-02-13 (Phase 1 complete)*
