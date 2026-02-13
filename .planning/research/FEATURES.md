# Feature Research

**Domain:** Music Recommendation MCP Server
**Researched:** 2026-02-13
**Confidence:** MEDIUM

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Access listening history | Core data source for personalization; all music recommendation systems start here | LOW | ytmusicapi provides `get_history()` - straightforward retrieval |
| Create playlists | Fundamental output format; recommendations are useless without delivery mechanism | LOW | ytmusicapi provides `create_playlist()` and track management |
| Search/verify tracks exist | Must validate recommendations actually exist on platform before suggesting | LOW | ytmusicapi has comprehensive search with filters |
| Add tracks to playlists | Basic CRUD operations for playlist building | LOW | ytmusicapi provides `add_playlist_items()` |
| Handle authentication | Required to access personal data and perform operations | MEDIUM | ytmusicapi uses browser cookie extraction; needs secure storage |
| Basic error handling | API failures, network issues, rate limits are inevitable | MEDIUM | Need retry logic, graceful degradation, informative errors |
| Respect rate limits | YouTube Music has undocumented rate limits; excessive requests = blocking | MEDIUM | Implement backoff, request throttling, monitor response codes |

### Differentiators (Competitive Advantage)

Features that set product apart. Not expected, but valued.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Deep listening history analysis | Go beyond recent listens - analyze full history to find patterns, evolution, deep preferences | MEDIUM | ytmusicapi allows history retrieval; complexity in analysis logic and data volume handling |
| AI-powered track verification | Claude validates recommendations using music knowledge before adding to playlist | LOW | Leverages Claude's training data; prompt engineering to verify artist/track existence |
| Anti-mainstream filtering | Explicitly avoid popular/well-known tracks user likely already knows | HIGH | Requires popularity metrics (not in ytmusicapi), possibly track play counts, chart positions; may need external data sources |
| Mood/context-based recommendations | Similar to Discogs MCP mood queries; "chill work music" or "aggressive workout tracks" | MEDIUM | Natural language processing already handled by Claude; map to musical attributes |
| Temporal pattern detection | Identify when user listens to certain genres/moods; recommend accordingly | HIGH | Requires timestamp analysis, clustering, pattern recognition beyond basic history retrieval |
| Discovery bias tuning | Let user control familiarity vs novelty slider; how adventurous should recommendations be | MEDIUM | Parameter-based filtering; needs clear recommendation scoring system |
| Multi-pass recommendation refinement | Generate candidates, Claude evaluates, refines, re-generates until quality threshold met | MEDIUM | Iterative workflow; higher token usage but better results |
| Collection gap analysis | Identify underrepresented genres/artists in user's history; suggest diversification | HIGH | Requires genre classification, collection taxonomy, gap detection algorithms |
| Hidden gem discovery | Surface obscure tracks from artists user already likes (deep cuts, B-sides, rarities) | MEDIUM | Use artist discography from ytmusicapi, filter by user's existing artist preferences, prioritize less popular tracks |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Real-time playback control | Existing YouTube Music MCP does this; seems like natural feature | Adds complexity for minimal value in recommendation context; requires browser automation, platform-specific code, fragile | Focus on playlist creation as delivery; user controls playback in native app |
| Social sharing/collaboration | Many music platforms have this; seems table stakes | Out of scope for personal recommendation tool; introduces privacy, multi-user, permissions complexity | Single-user focus; user can share playlists via YouTube Music native features |
| Music generation/synthesis | AI music tools are trending (YourMusic.Fun MCP, MiniMax-MCP) | Completely different domain; recommendation != creation; massive scope expansion | Stick to curation from existing catalog |
| Multi-platform support (Spotify, Apple Music, etc.) | User might use multiple services | Each platform has different API, auth, data models; 5x complexity for marginal benefit | YouTube Music only; clean scope, single integration |
| DAW integration (like Ableton MCP) | Music production workflows are hot | Wrong use case; recommendation for listening != music production | Consumer music discovery, not production tooling |
| Live streaming/radio mode | Continuous playback feels engaging | Complex state management, real-time requirements, infrastructure needs; doesn't align with playlist-based delivery | Static playlist generation; user controls playback |
| Collaborative filtering | Industry-standard recommendation technique using "users like you" | Requires multi-user data; single-user tool has no peer comparison data | Content-based filtering using Claude's music knowledge + user's history |

## Feature Dependencies

```
Authentication (browser cookies)
    └──requires──> API Access (ytmusicapi)
                       ├──requires──> Listening History Retrieval
                       │                  └──enables──> History Analysis
                       │                                    └──enables──> Pattern Detection
                       │                                    └──enables──> Mood-based Recommendations
                       │                                    └──enables──> Gap Analysis
                       └──requires──> Search/Verify Tracks
                                          └──requires──> Create Playlist
                                                            └──requires──> Add Tracks to Playlist

AI-powered Track Verification ──enhances──> Search/Verify Tracks
Claude Music Knowledge ──enables──> All recommendation features
Anti-mainstream Filtering ──requires──> Popularity Metrics (external data?)
Hidden Gem Discovery ──requires──> Artist Discography + Popularity Metrics
```

### Dependency Notes

- **Authentication requires API Access:** ytmusicapi needs browser cookies to authenticate; this is the foundation for all other features
- **History Analysis enables advanced features:** Deep analysis unlocks pattern detection, mood recommendations, gap analysis
- **Anti-mainstream filtering requires external data:** ytmusicapi doesn't provide track popularity/play counts; may need workaround or accept limitation
- **Claude acts as knowledge layer:** Leveraging Claude's music knowledge reduces need for external music databases
- **Playlist creation is final step:** All recommendation paths converge on playlist as delivery mechanism

## MVP Definition

### Launch With (v1)

Minimum viable product - what's needed to validate the concept.

- [ ] **Access listening history** - Foundation for all personalization; must retrieve and store full history
- [ ] **AI-powered track verification** - Core differentiator; Claude validates recommendations exist on YouTube Music before suggesting
- [ ] **Create playlists** - Output delivery; recommendations must materialize as actionable playlists
- [ ] **Basic authentication** - Secure cookie storage and ytmusicapi integration
- [ ] **Simple recommendation flow** - User describes what they want, Claude analyzes history + generates recommendations, creates playlist
- [ ] **Error handling** - Graceful failures for API issues, missing tracks, auth problems

**Why this is minimum:** Solves the core problem (frustrated with mainstream recommendations) with the key differentiator (AI-powered curation from personal history). Proves the concept before adding complexity.

### Add After Validation (v1.x)

Features to add once core is working.

- [ ] **Deep listening history analysis** - Trigger: Users want more sophisticated pattern-based recommendations
- [ ] **Mood/context-based recommendations** - Trigger: Users request specific use-case playlists
- [ ] **Hidden gem discovery** - Trigger: Users want deeper exploration of existing favorite artists
- [ ] **Discovery bias tuning** - Trigger: Users want control over recommendation adventurousness
- [ ] **Multi-pass refinement** - Trigger: Initial recommendations aren't high enough quality

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] **Temporal pattern detection** - Why defer: High complexity, requires significant data accumulation, unclear immediate value
- [ ] **Collection gap analysis** - Why defer: Advanced feature requiring taxonomy development, better as enhancement
- [ ] **Anti-mainstream filtering with external data** - Why defer: May require external API integration (complexity, cost, reliability); validate need first

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Access listening history | HIGH | LOW | P1 |
| AI-powered track verification | HIGH | LOW | P1 |
| Create playlists | HIGH | LOW | P1 |
| Basic authentication | HIGH | MEDIUM | P1 |
| Simple recommendation flow | HIGH | MEDIUM | P1 |
| Error handling | HIGH | MEDIUM | P1 |
| Deep history analysis | HIGH | MEDIUM | P2 |
| Mood/context recommendations | MEDIUM | MEDIUM | P2 |
| Hidden gem discovery | HIGH | MEDIUM | P2 |
| Discovery bias tuning | MEDIUM | MEDIUM | P2 |
| Multi-pass refinement | MEDIUM | MEDIUM | P2 |
| Temporal pattern detection | MEDIUM | HIGH | P3 |
| Collection gap analysis | MEDIUM | HIGH | P3 |
| Anti-mainstream filtering (external data) | HIGH | HIGH | P3 |

**Priority key:**
- P1: Must have for launch (MVP)
- P2: Should have, add when possible (post-validation)
- P3: Nice to have, future consideration (v2+)

## Competitor Feature Analysis

| Feature | Existing MCP Servers | Music Analysis Tools | Our Approach |
|---------|----------------------|----------------------|--------------|
| **Listening History Access** | YouTube Music MCP (playback only), Spotify MCP (library access), Last.fm (scrobbling + history) | Spotify Playlist Analyzer (limited to playlists), Last.fm (full history) | Full YouTube Music history via ytmusicapi; analyze all available data |
| **AI-Powered Recommendations** | Discogs MCP (mood-based from collection), Epidemic Sound MCP (context metadata), Navidrome MCP (taste-aware playlisting) | YouTube Music AI Playlist (Feb 2026, natural language, Premium only), Spotify Discover Weekly (collaborative filtering) | Claude analyzes history + validates recommendations using music knowledge; no external ML needed |
| **Playlist Creation** | All music MCPs support this | Standard across all platforms | ytmusicapi playlist creation; straightforward implementation |
| **Anti-Mainstream Focus** | None explicitly address this | Some tools (Unchartify, Radiooooo) help escape algorithmic mainstream bias; Bandcamp/SoundCloud for indie/deep cuts | Unique differentiator; explicitly designed to counter YouTube's mainstream push |
| **Mood/Context Awareness** | Discogs MCP (strong), Epidemic Sound MCP (metadata-driven), Navidrome MCP (listening history-based) | YouTube Music AI Playlist (natural language prompts), Spotify's various mood playlists | Natural language via Claude; no need for predefined mood taxonomy |
| **Collection Analysis** | Discogs MCP (detailed collection insights, taste profile) | Spotify Playlist Analyzer (basic stats), Chartmetric (professional analytics), SONOTELLER (deep track analysis) | Focus on recommendation generation, not analytics reporting |
| **Deep Cut Discovery** | None specifically target this | Community platforms (Bandcamp, SoundCloud) better for this; human curators favored over algorithms | Use artist discography + popularity filtering; combine algorithmic + human (Claude) curation |

## Research Confidence Assessment

**Overall Confidence: MEDIUM**

### High Confidence Areas
- **Table stakes features:** ytmusicapi documentation is comprehensive; standard MCP server patterns are well-established
- **Basic recommendation flow:** Multiple MCP music servers provide reference architectures
- **Playlist creation mechanics:** ytmusicapi provides clear, documented methods

### Medium Confidence Areas
- **Anti-mainstream filtering effectiveness:** Unclear how to reliably determine track popularity without external data sources (play counts not in ytmusicapi)
- **Deep history analysis patterns:** Implementation approaches vary widely; need to validate what works best
- **AI verification accuracy:** Claude's music knowledge is broad but may have gaps for obscure tracks; false positives/negatives possible

### Low Confidence Areas
- **Rate limiting specifics:** YouTube Music API is unofficial and undocumented; rate limits are unknown, may require trial-and-error
- **Authentication reliability:** Browser cookie extraction can be fragile; YouTube may change auth mechanisms
- **External data requirements:** May discover need for popularity metrics, genre taxonomies, or other data not available in ytmusicapi

### Validation Needed
- Test ytmusicapi history retrieval limits (how far back can we access?)
- Verify Claude's accuracy on YouTube Music track verification across genres
- Determine if anti-mainstream filtering is viable without external popularity data
- Assess rate limiting through experimentation with ytmusicapi

## Sources

**MCP Server Ecosystem:**
- [Discogs MCP Server](https://blog.willchatham.com/2026/01/04/discogs-mcp-server/) - Mood-based collection recommendations
- [YouTube Music MCP Server](https://github.com/mondweep/youtube-music-mcp-server) - Playback control implementation
- [Epidemic Sound MCP Server](https://www.epidemicsound.com/blog/mcp-server/) - Context-aware music catalog access
- [YourMusic.Fun MCP Server](https://lobehub.com/mcp/yourmusic-fun-yourmusic-fun-mcp) - AI music generation
- [Navidrome MCP Server](https://skywork.ai/skypage/en/navidrome-mcp-server-ai-engineer-guide/1981593206305910784) - Personal music server AI control
- [Music Collection MCP Server](https://mcpservers.org/servers/gorums/music-mcp-rules) - Smart discovery and analytics
- [Spotify MCP Server](https://glama.ai/mcp/servers/@superseoworld/mcp-spotify) - Spotify integration
- [Best MCP Servers for Developers 2026](https://www.builder.io/blog/best-mcp-servers-2026) - Ecosystem overview

**Music Analysis Tools:**
- [YouTube Music AI Playlist](https://9to5google.com/2026/02/10/youtube-music-adding-ai-playlist-with-text-based-playlist-generation/) - Natural language playlist generation (Feb 2026)
- [SONOTELLER](https://www.topmediai.com/ai-music/ai-song-analyzer/) - Deep song analysis including mood, genre, instruments
- [Spotify Playlist Analyzer](https://www.chosic.com/spotify-playlist-analyzer/) - Playlist statistics and insights
- [Best New Music Discovery Platforms 2026](https://resources.onestowatch.com/best-new-music-discovery-platforms/) - Discovery ecosystem overview
- [Understanding Deep Cuts in Music](https://www.oreateai.com/blog/understanding-deep-cuts-in-music-the-hidden-gems-of-your-playlist/62caed23d389badeca75ca15990c30a7) - Hidden gems definition

**YouTube Music API:**
- [ytmusicapi Official Documentation](https://ytmusicapi.readthedocs.io/) - Comprehensive API reference
- [ytmusicapi GitHub](https://github.com/sigma67/ytmusicapi) - Source repository

**Recommendation Systems:**
- [Music Recommendation System Guide](https://stratoflow.com/music-recommendation-system-guide/) - How streaming platforms use AI
- [How Spotify Algorithm Works](https://attractgroup.com/blog/how-spotify-algorithm-works-for-music-recommendation/) - Collaborative filtering and content-based approaches
- [Fairness in Music Streaming Algorithms 2025](https://www.music-tomorrow.com/blog/fairness-transparency-music-recommender-systems) - Mainstream bias issues
- [Breaking Free from Algorithms](https://distromono.com/how-to/music-discovery-break-free-algorithms/) - Alternative discovery approaches
- [How Spotify's Algorithm Shapes Global Discovery](https://digital.hec.ca/en/blog/how-spotifys-algorithm-shapes-global-music-discovery-and-cultural-diversity/) - Cultural diversity challenges

**Anti-Mainstream Discovery:**
- [Playlist Curation: Music Discovery Across Platforms](https://freeyourmusic.com/blog/playlist-curation-music-discovery) - Human vs algorithmic curation
- [Best Spotify Alternatives for Discovery 2026](https://resources.onestowatch.com/best-spotify-playlists-alternatives-2026/) - Alternative platforms
- [Beyond Top 10 Charts: DJ Music Discovery](https://learningtodj.com/blog/the-art-of-dj-music-discovery/) - Deep cut discovery techniques

---
*Feature research for: YouTube Music MCP Server*
*Researched: 2026-02-13*
