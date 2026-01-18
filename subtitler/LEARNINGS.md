# Learnings

This file captures lessons learned, failed approaches, and decisions made during development. Ralph should add entries here when:

- A plan fails and is deleted
- Dependencies are added or modified (explain why)
- An unexpected issue is encountered and resolved
- A design decision is made that future iterations should know about

---

## Task 16: Analytics Integration (2026-01-18)

**Status:** In progress - foundation complete

**What was completed:**
- Database schema (migration 002) with 3 tables: visitors, events, experiments
- Analytics service in `internal/analytics/` with tracking and query functions
- API endpoints: POST /api/analytics/events, GET /api/analytics/funnel, GET /api/analytics/experiments/{id}
- Frontend tracking client in `frontend/src/lib/analytics.ts`
- Page view tracking on index.astro
- Created comprehensive implementation plan in `010_ANALYTICS_INTEGRATION.md`

**Key decisions:**
- **Privacy-first design:** Anonymous visitor IDs in localStorage, no third-party services, minimal PII
- **Flexible event format:** JSON properties blob for extensibility without schema changes
- **Public tracking endpoint:** /api/analytics/events is public (no auth) to track anonymous visitors
- **Protected analysis:** Funnel and experiment result endpoints require authentication
- **SQLite storage:** Keep it simple, all data local, proper indexes for query performance

**Architectural patterns:**
- **Handler pattern:** Followed project convention of returning `http.HandlerFunc` with dependencies as parameters (not application struct with methods)
- **Database access:** DB struct embeds `*sql.DB`, access via `database.DB` field directly
- **Claims extraction:** Used `auth.GetClaims(r)` helper instead of accessing context directly

**What's next:**
- Add event tracking throughout the app (signup, login, upload, download)
- Write unit tests for analytics service
- Update README.md with analytics API documentation
- Verify full funnel with manual testing

**For next iteration:**
- Consider adding basic analytics dashboard page (Astro page with charts)
- May want to add cleanup job to delete events older than 1 year (data retention)
- Experiment framework is ready for actual A/B tests once we have traffic

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

---

## Task 15: Experimentation Plan (2026-01-18)

**Status:** Complete

**What was completed:**
- Created comprehensive EXPERIMENTATION_PLAN.md (500+ lines)
- Defined experimentation framework (hypothesis → design → implementation → measurement → analysis → action)
- Specified 12 priority experiments across 3 phases:
  - **Phase 1 (Validation)**: 4 experiments (landing page, signup flow, format preferences, Reddit messaging)
  - **Phase 2 (Growth)**: 4 experiments (pricing display, free tier limits, notifications, format education)
  - **Phase 3 (Scale)**: 4 experiments (batch upsell, referrals, re-engagement, content mix)
- Defined continuous optimization metrics with targets
- Documented analytics infrastructure requirements
- Created learning agenda for market understanding, pricing decisions, and product direction
- Included privacy and ethics guidelines for responsible experimentation
- Specified reporting cadence (weekly, monthly, quarterly)

**Key Decisions:**
1. **Experimentation culture**: "Test early, test often" - every assumption is a hypothesis to validate
2. **Decision criteria**: Ship if 95%+ confidence and 10%+ improvement; kill if 90%+ confidence variant is worse
3. **First experiments**: Focus on value proposition messaging (3 variants) and signup flow friction
4. **Analytics dependency**: Task 16 (Analytics integration) is prerequisite for executing experimentation plan
5. **Sample sizes**: 300-900 visitors/users per experiment for statistical significance
6. **Duration**: 2-6 weeks per experiment depending on conversion funnel depth
7. **Ethics**: No harmful variants, informed consent, data anonymization, transparent reporting

**Strategic Insights:**
- Landing page messaging experiments will validate which pain point resonates most (time vs. cost vs. quality)
- Signup flow friction test between email verification vs. guest mode will impact activation rate significantly
- Free tier sizing (10 vs. 30 vs. 60 min/month) requires 90-day experiment to capture habit formation and upgrade behavior
- Reddit outreach messaging needs A/B testing - direct promotion vs. helpful-first approach
- Pricing page experiments should test concrete examples vs. abstract pricing

**Infrastructure Requirements:**
- User tracking (anonymous visitor ID, user account ID, UTM parameters)
- Event tracking (page views, signups, uploads, downloads, conversions)
- A/B testing framework (variant assignment, metric aggregation, statistical significance)
- Experiment tracking directory structure: `experiments/EXP###_name.md`

**For future iterations:**
- Task 16 (Analytics integration) should implement:
  1. Event logging in SQLite (user_id, event_type, metadata, timestamp)
  2. Experiment assignment table (user_id, experiment_id, variant)
  3. Metric aggregation and significance testing
  4. Dashboard for experiment results
- Consider creating experiment tracking templates before launch
- Analytics implementation should support cohort analysis for retention tracking
- Learnings from experiments should feed back into MARKETING_PLAN.md iterations

**Alignment with Marketing Plan:**
- Experimentation plan directly supports marketing plan validation phase goals
- First 4 experiments (EXP001-004) align with Phase 1 (Months 1-2) validation tactics
- Growth phase experiments (EXP005-008) support Phase 2 (Months 3-6) monetization strategy
- Scale phase experiments (EXP009-012) enable Phase 3 (Months 7-12) retention and upselling

