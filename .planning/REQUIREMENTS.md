# Requirements: YouTube Music MCP

**Defined:** 2026-02-13
**Core Value:** Claude can analyze my YouTube Music taste and recommend genuinely interesting music I haven't heard — delivered as ready-to-play playlists

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Authentication

- [x] **AUTH-01**: User can authenticate with Google account via OAuth2 flow
- [x] **AUTH-02**: OAuth2 tokens persist to disk and survive server restarts
- [x] **AUTH-03**: Expired access tokens refresh automatically using refresh token

### Taste Data

- [ ] **TASTE-01**: Claude can retrieve user's liked videos/songs from YouTube
- [ ] **TASTE-02**: Claude can retrieve user's playlists and their contents
- [ ] **TASTE-03**: Claude can retrieve user's channel subscriptions

### Search

- [ ] **SRCH-01**: Claude can search YouTube Music for tracks by name, artist, or query
- [ ] **SRCH-02**: Claude can verify if a specific track exists on YouTube Music

### Playlist Management

- [ ] **PLST-01**: Claude can create a new playlist on user's YouTube Music account
- [ ] **PLST-02**: Claude can add tracks to an existing playlist
- [ ] **PLST-03**: Claude can list user's existing playlists

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Recommendations

- **RECC-01**: Claude can provide mood/context-based recommendations ("chill work music", "aggressive workout tracks")
- **RECC-02**: Claude can discover deep cuts from user's favorite artists (lesser-known tracks, B-sides)
- **RECC-03**: User can tune discovery bias (how adventurous recommendations should be)

### Analytics

- **ANLT-01**: Claude can show listening pattern analysis (genre distribution, favorite artists)
- **ANLT-02**: Claude can identify gaps in user's music collection

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Listening history access | No official YouTube API — would require unofficial cookie-based access violating YouTube ToS |
| Browser automation / playback control | Playlist links are sufficient delivery mechanism; browser automation is fragile |
| Unofficial ytmusicapi / cookie auth | Violates YouTube Terms of Service, unreliable, may break at any time |
| Multi-platform (Spotify, Apple Music) | YouTube Music only for clean scope; each platform multiplies complexity |
| Mobile app or standalone UI | MCP server consumed by Claude only |
| Social features / sharing | Personal use tool; user can share playlists via YouTube Music natively |
| Music generation | Different domain entirely; recommendation != creation |
| Real-time playback control | Adds complexity for minimal value; user controls their own player |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| AUTH-01 | Phase 1 | Pending |
| AUTH-02 | Phase 1 | Pending |
| AUTH-03 | Phase 1 | Pending |
| TASTE-01 | Phase 2 | Pending |
| TASTE-02 | Phase 2 | Pending |
| TASTE-03 | Phase 2 | Pending |
| SRCH-01 | Phase 2 | Pending |
| SRCH-02 | Phase 2 | Pending |
| PLST-01 | Phase 3 | Pending |
| PLST-02 | Phase 3 | Pending |
| PLST-03 | Phase 3 | Pending |

**Coverage:**
- v1 requirements: 11 total
- Mapped to phases: 11
- Unmapped: 0

---
*Requirements defined: 2026-02-13*
*Last updated: 2026-02-13 after roadmap creation*
