# Learnings

This file captures lessons learned, failed approaches, and decisions made during development. Ralph should add entries here when:

- A plan fails and is deleted
- Dependencies are added or modified (explain why)
- An unexpected issue is encountered and resolved
- A design decision is made that future iterations should know about

---

## Task 11: Cloudflare Tunnel Setup (2026-01-18)

**Status:** Blocked - requires human intervention

**What was completed:**
- Implemented `/api/health` endpoint with JSON response
- Added configurable CORS via `FRONTEND_URL` environment variable
- Created comprehensive setup documentation in `009_CLOUDFLARE_TUNNEL.md`
- All tests passing, code ready for deployment

**Blocker:**
Task 11's "done_when" criterion requires a live tunnel: `curl https://subtitler.yourdomain.com/api/health returns 200 via tunnel`

This requires:
1. **Domain decision** - Need to choose domain name (e.g., subtitler.yourdomain.com)
2. **VM access** - Need SSH access to Ubuntu VM to install cloudflared
3. **Cloudflare account** - Need authentication to create tunnel
4. **DNS configuration** - Need to configure DNS routes in Cloudflare dashboard

**Decision:** Mark task as "blocked" rather than "complete" because the verification step cannot be performed by an autonomous agent. The implementation is complete, but deployment requires human action.

**For next iteration:**
- If human has completed VM setup, unblock this task and verify the tunnel
- Otherwise, skip to Task 12 (CI/CD) or Task 13 (Secrets) which may also have deployment dependencies

---

## Task 14: Marketing Plan (2026-01-18)

**Status:** Complete

**What was completed:**
- Created comprehensive MARKETING_PLAN.md (506 lines)
- Market research using WebSearch tool on content creator pain points (2026 data)
- Competitive landscape analysis with pricing comparison
- Defined target audiences (emerging and professional creators)
- 3-phase go-to-market strategy (validation → growth → scale)
- Distribution channels with Reddit as primary channel (70% effort)
- Messaging framework and value propositions
- 90-day content calendar with 24 blog post topics
- Success metrics and KPIs for each phase
- Budget allocation ($500 validation → $3000 growth → $10k+ scale)

**Key Decisions:**
1. **Primary distribution channel**: Reddit creator communities (r/NewTubers, r/VideoEditing, r/PartneredYoutube) for validation phase
2. **Value proposition**: "Fast, accurate subtitles. Pay only for what you use." - focuses on pay-per-use vs. subscription fatigue
3. **Target audience**: Emerging creators (1k-100k YouTube subscribers, 10k-500k TikTok followers) who are price-sensitive and time-constrained
4. **Competitive positioning**: Market gap identified - most tools require monthly subscriptions ($16-55/month), few offer pay-per-use
5. **Content strategy**: Educational content first (accessibility, platform limitations), then use cases, then advanced topics
6. **Launch sequence**: Product Hunt after soft launch and Reddit engagement to build momentum

**Research Insights:**
- Platform auto-captions only ~70% accurate, don't meet WCAG accessibility standards
- "If you aren't putting text on your videos in 2026, subtitles are now the difference between a viral video and one that gets forgotten"
- TikTok doesn't allow caption export, YouTube's built-in captions unreliable
- Time consumption is major pain point - manual transcription takes hours per video
- VEED.IO pricing: $24-55/month; Kapwing: $16+/month; Free tiers available but with limitations

**For future iterations:**
- Task 15 (EXPERIMENTATION_PLAN.md) should complement this marketing plan with specific A/B testing frameworks
- Analytics integration (Task 16) should track the KPIs defined in this plan
- Consider creating some of the early blog posts from the content calendar to have content ready for launch

