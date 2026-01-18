# Subtitler Experimentation Plan

## Purpose

This document defines the A/B testing framework and experimentation strategy for validating marketing approaches, product features, and pricing models. The goal is to make data-driven decisions rather than assumptions about what will drive user acquisition, activation, and retention.

**Core Philosophy**: Test early, test often. Every assumption is a hypothesis to validate.

## Experimentation Framework

### Experiment Lifecycle

1. **Hypothesis**: What do we believe and why?
2. **Design**: How will we test it?
3. **Implementation**: What needs to be built/changed?
4. **Measurement**: What metrics will tell us if we're right?
5. **Analysis**: What did we learn?
6. **Action**: What do we do with the learning?

### Decision Criteria

**Ship It**: If confidence level > 95% and effect size > 10% improvement
**Keep Testing**: If inconclusive results but trend is positive
**Kill It**: If confidence level > 90% that variant is worse
**Learn & Iterate**: If results are unexpected, design follow-up experiment

### Experiment Tracking

All experiments will be documented in an `experiments/` directory with the following structure:

```
experiments/
├── EXP001_landing_page_headline.md
├── EXP002_pricing_display.md
├── EXP003_cta_button_color.md
└── template.md
```

Each experiment document should include:
- Hypothesis
- Variations tested
- Success metrics
- Sample size and duration
- Results and learnings
- Next actions

## Priority Experiments

### Phase 1: Validation (Months 1-2)

These experiments focus on validating core assumptions about the product and market.

#### EXP001: Landing Page Value Proposition

**Hypothesis**: Different value propositions will resonate with different creator segments. "Fast and accurate" will outperform "Pay only for what you use" in initial signups.

**Variations**:
- **Control (A)**: "Fast, accurate subtitles for your videos. Pay only for what you use."
- **Variant B**: "Professional-quality subtitles in minutes. Upload, process, download."
- **Variant C**: "Stop wasting hours on subtitles. Automated transcription for content creators."

**Success Metric**: Signup conversion rate (visitor → registered user)

**Sample Size**: 300 visitors per variation (900 total)
**Duration**: 2-3 weeks or until statistical significance

**Implementation**: Use client-side A/B testing (simple JS) or URL parameter variations

**Learnings To Capture**:
- Which pain point resonates most (time vs. cost vs. quality)?
- Do aspirational ("professional-quality") or practical ("stop wasting") messages work better?
- Does mentioning pricing upfront affect conversion?

#### EXP002: Signup Flow Friction

**Hypothesis**: Requiring email verification will reduce spam but may decrease activation rate. Allowing users to transcribe first without signup will increase conversion.

**Variations**:
- **Control (A)**: Email + password → email verification → transcribe
- **Variant B**: Email + password → immediate access (no verification)
- **Variant C**: Guest mode → transcribe first → signup prompted at download

**Success Metric**: Activation rate (signup → first completed job)

**Sample Size**: 150 users per variation (450 total)
**Duration**: 2 weeks

**Learnings To Capture**:
- What's the dropout rate at each stage?
- Does "try before signup" increase conversion?
- What's the spam/fake account rate for each approach?

#### EXP003: Output Format Preferences

**Hypothesis**: Most creators will use SRT or embedded formats, but format preference varies by platform (YouTube vs. TikTok).

**Variations**: No A/B test needed - this is observational data collection

**Success Metric**: Format distribution by user self-reported platform

**Implementation**:
- Track format selection for each job
- Add optional "What platform are you creating for?" question on upload
- Correlate format choice with platform

**Learnings To Capture**:
- Which formats are most popular?
- Do YouTube creators prefer different formats than TikTok creators?
- Should we default to different formats based on platform?

**Action**: Use learnings to optimize UI (show most popular formats first) and marketing (highlight relevant formats per platform)

#### EXP004: Reddit Outreach Messaging

**Hypothesis**: Helpful, non-promotional comments with a subtle mention will outperform direct tool promotion in creator subreddits.

**Variations**:
- **Control (A)**: Direct mention: "I built a tool for this - Subtitler.com"
- **Variant B**: Helpful first: "I've found whisper.cpp works well. There are also web tools like [Tool] and Subtitler."
- **Variant C**: Pure value: Answer question thoroughly, no mention of Subtitler

**Success Metric**: Click-through rate from Reddit comment to landing page

**Sample Size**: 20 comments per variation (60 total)
**Duration**: 2 weeks

**Implementation**: Manually vary approach; track clicks via UTM parameters

**Learnings To Capture**:
- What's the upvote/downvote ratio for each approach?
- What's the click-through rate?
- Are any approaches resulting in bans or negative reactions?

### Phase 2: Growth (Months 3-6)

These experiments focus on optimizing conversion and monetization.

#### EXP005: Pricing Page Display

**Hypothesis**: Showing cost-per-video examples will increase conversion better than cost-per-minute pricing alone.

**Variations**:
- **Control (A)**: "$0.12/minute of audio"
- **Variant B**: "$0.12/minute (~$1.20 for a 10-minute video)"
- **Variant C**: Comparison table showing Subtitler vs. subscription tools for different usage levels

**Success Metric**: Free → Paid conversion rate

**Sample Size**: 200 users per variation (600 total)
**Duration**: 4 weeks (to capture monthly decision cycles)

**Learnings To Capture**:
- Do concrete examples increase conversion?
- What usage level do most users care about (5 min? 10 min? 60 min?)?
- Does competitor comparison increase or decrease trust?

#### EXP006: Free Tier Limits

**Hypothesis**: A generous free tier (60 minutes/month) will result in more paid conversions than a restrictive one (10 minutes/month) due to habit formation.

**Variations**:
- **Control (A)**: 30 minutes/month free tier
- **Variant B**: 10 minutes/month free tier
- **Variant C**: 60 minutes/month free tier

**Success Metric**:
- Primary: Free → Paid conversion rate at 90 days
- Secondary: Engagement (jobs per user per month)

**Sample Size**: 300 users per variation (900 total)
**Duration**: 90 days (long experiment to capture upgrade behavior)

**Learnings To Capture**:
- What free tier usage predicts paid conversion?
- Is there a "habit formation" threshold (e.g., 3+ uses)?
- Do users on generous free tiers ever convert, or do they churn?

#### EXP007: Job Status Notifications

**Hypothesis**: Email notifications increase return visits and job completion satisfaction.

**Variations**:
- **Control (A)**: No notifications (user must check dashboard)
- **Variant B**: Email notification on job completion
- **Variant C**: Email + in-app notification badge

**Success Metric**:
- Primary: Return rate (users with 2+ jobs)
- Secondary: Time to download after completion

**Sample Size**: 200 users per variation (600 total)
**Duration**: 4 weeks

**Learnings To Capture**:
- Do notifications increase return visits?
- What's the optimal notification timing (immediate vs. batched)?
- Do users find notifications valuable or annoying (track opt-out rate)?

#### EXP008: Embedded vs. Separate Subtitle Positioning

**Hypothesis**: Most creators don't know the difference between embedded and separate subtitles. Educating them upfront will reduce support requests and increase satisfaction.

**Variations**:
- **Control (A)**: Format dropdown with no explanation
- **Variant B**: Format dropdown with tooltips explaining each format
- **Variant C**: Two-step wizard: "What platform are you creating for?" → Recommended format

**Success Metric**:
- Primary: User satisfaction (post-download survey)
- Secondary: Support request rate about formats

**Sample Size**: 200 users per variation (600 total)
**Duration**: 3 weeks

**Learnings To Capture**:
- Do users understand format differences?
- Does education reduce support requests?
- Which platforms correlate with which formats?

### Phase 3: Scale (Months 7-12)

These experiments focus on retention, upselling, and growth optimization.

#### EXP009: Batch Processing Upsell

**Hypothesis**: Users who upload 3+ files in a day are good candidates for batch processing. Offering this feature as an upsell will increase ARPU.

**Variations**:
- **Control (A)**: No upsell prompt
- **Variant B**: "Upload multiple files at once with Batch Processing ($X/month)"
- **Variant C**: "You've uploaded 3 files today. Want to save time? Try Batch Processing (first month free)"

**Success Metric**: Batch processing feature adoption rate

**Sample Size**: 150 users per variation (450 total, targeting heavy users)
**Duration**: 4 weeks

**Learnings To Capture**:
- What usage threshold indicates batch processing need?
- Is a free trial or discount more effective?
- What's the retention rate for batch processing users?

#### EXP010: Referral Program Messaging

**Hypothesis**: Offering referral credits (vs. cash) will increase referral program participation.

**Variations**:
- **Control (A)**: No referral program
- **Variant B**: "Refer a friend, get 60 minutes free credit"
- **Variant C**: "Refer a friend, get $5 cash (via PayPal)"

**Success Metric**: Referral program participation rate and referral conversion rate

**Sample Size**: 300 users per variation (900 total)
**Duration**: 6 weeks

**Learnings To Capture**:
- Do users prefer credits or cash?
- What's the virality coefficient (referrals per user)?
- What's the LTV of referred users vs. organic users?

#### EXP011: Re-engagement Email Campaigns

**Hypothesis**: Personalized re-engagement emails will bring back churned users better than generic "we miss you" messages.

**Variations**:
- **Control (A)**: No re-engagement email
- **Variant B**: Generic: "We miss you! Come back and get 30 minutes free."
- **Variant C**: Personalized: "Your last video was about [topic]. Ready to transcribe your next one?"
- **Variant D**: Value-add: "New feature: embedded subtitles now available. Try it free."

**Success Metric**: Reactivation rate (churned user → new job created)

**Sample Size**: 200 users per variation (800 total, targeting 30-day inactive users)
**Duration**: 4 weeks

**Learnings To Capture**:
- What messaging brings back churned users?
- Do new features re-engage better than discounts?
- Is there a point of no return (e.g., 90 days inactive = unlikely to return)?

#### EXP012: Content Marketing Channel Mix

**Hypothesis**: Blog posts focused on "how-to" topics will drive more qualified traffic than "tool comparison" posts.

**Variations**:
- **Control (A)**: Mix of content types (current content calendar)
- **Variant B**: 80% how-to tutorials, 20% product updates
- **Variant C**: 50% how-to, 30% comparisons/reviews, 20% thought leadership

**Success Metric**:
- Primary: Organic traffic to signup conversion rate
- Secondary: Time on page, pages per session

**Sample Size**: N/A (observational over time)
**Duration**: 90 days (3 months of content publishing)

**Learnings To Capture**:
- Which content types drive the most qualified traffic?
- What's the conversion rate by content type?
- Which topics have the best SEO performance?

## Continuous Optimization Metrics

These metrics should be monitored continuously (not as discrete experiments) to identify optimization opportunities:

### User Acquisition

| Metric | Target | Tracking Method |
|--------|--------|-----------------|
| Cost per acquisition (CPA) | < $5 | UTM parameters + conversion tracking |
| Signup conversion rate | 5-10% | Landing page analytics |
| Traffic source quality | Varies | Source × activation rate correlation |
| Organic vs. paid split | 70/30 | Google Analytics |

### Activation & Engagement

| Metric | Target | Tracking Method |
|--------|--------|-----------------|
| Activation rate (signup → first job) | 70%+ | User journey funnel |
| Time to first job | < 10 minutes | Timestamp analysis |
| Jobs per user per month | 5+ | User activity aggregation |
| Format preference distribution | Observational | Job metadata |
| Job success rate | 95%+ | Job status tracking |

### Retention & Monetization

| Metric | Target | Tracking Method |
|--------|--------|-----------------|
| Return rate (users with 2+ jobs) | 40%+ | User cohort analysis |
| Free → Paid conversion rate | 10-15% | User tier tracking |
| Monthly churn rate (paid users) | < 10% | Subscription status changes |
| Average revenue per user (ARPU) | $5-10/month | Revenue / active users |
| Customer lifetime value (LTV) | $50+ | Cohort revenue tracking |

### Product Quality

| Metric | Target | Tracking Method |
|--------|--------|-----------------|
| Transcription accuracy | 90%+ | Sample manual review |
| Job failure rate | < 5% | Error logs |
| Average processing time | < 5 min (per 10 min of audio) | Job duration tracking |
| Support requests per 100 users | < 5 | Support ticket tracking |

## Experiment Infrastructure

### Analytics Requirements

To execute this experimentation plan, we need:

1. **User Tracking**:
   - Anonymous visitor ID (cookie-based)
   - User account ID (post-signup)
   - UTM parameter capture
   - Session recording (privacy-respecting)

2. **Event Tracking**:
   - Page views (landing, pricing, signup, dashboard)
   - Key actions (signup, upload, download, format selection)
   - Conversion events (first job, paid upgrade)
   - Retention events (return visit, multi-job user)

3. **A/B Testing Tools**:
   - Variant assignment (consistent per user)
   - Metric aggregation per variant
   - Statistical significance calculation
   - Experiment dashboard

### Implementation Plan (Task 16: Analytics Integration)

This experimentation plan depends on analytics infrastructure:

1. **Phase 1**: Basic event tracking (Task 16)
   - Implement event logging (signup, job creation, job completion)
   - Store events in SQLite with user_id, event_type, metadata, timestamp
   - Create basic dashboard for event counts

2. **Phase 2**: A/B testing framework
   - Add experiment assignment table (user_id, experiment_id, variant)
   - Implement variant assignment logic
   - Create experiment results aggregation

3. **Phase 3**: Advanced analytics
   - Cohort analysis (retention by signup date)
   - Funnel visualization (signup → activation → retention)
   - Revenue tracking and LTV calculation

### Privacy & Ethics

All experiments must comply with:

1. **Informed Consent**: Users should know they may see different versions
2. **No Harmful Variants**: Never test something that degrades core functionality
3. **Data Privacy**: Anonymize data; don't share personally identifiable information
4. **Transparency**: Document results publicly when possible (builds trust)

## Learning Agenda

Beyond specific experiments, we want to learn:

### Market Understanding

1. **Who are our users?**
   - Platform distribution (YouTube, TikTok, Instagram, other)
   - Creator size (followers, subscribers)
   - Publishing frequency (posts per week)
   - Geographic distribution

2. **What are their workflows?**
   - Do they edit before or after transcription?
   - What tools do they use alongside Subtitler?
   - How many takes/versions do they transcribe?

3. **What drives retention?**
   - What separates power users from one-time users?
   - What usage patterns predict churn?
   - What features drive stickiness?

### Pricing & Monetization

1. **What's the right price?**
   - Price sensitivity by user segment
   - Perceived value vs. actual price
   - Optimal free tier size

2. **What's the right model?**
   - Pay-per-use vs. subscription preferences
   - Volume discount effectiveness
   - Prepaid credits vs. pay-as-you-go

3. **What drives upgrades?**
   - What triggers free → paid conversion?
   - What features justify premium pricing?
   - What's the LTV by acquisition channel?

### Product Decisions

1. **What formats matter?**
   - Format preference by platform and use case
   - Do users understand format differences?
   - Should we add more formats (ASS, TTML)?

2. **What features drive value?**
   - Which features are used vs. ignored?
   - What features correlate with retention?
   - What features should we build next?

3. **What's the right UX?**
   - Where do users get stuck?
   - What causes confusion or support requests?
   - What delights users (capture testimonials)?

## Reporting & Review Cadence

### Weekly

- Review key metrics (signups, jobs, conversions)
- Identify anomalies or concerning trends
- Triage user feedback and support requests

### Monthly

- In-depth experiment review (complete, ongoing, upcoming)
- Cohort analysis (retention by signup month)
- Adjust marketing and product roadmap based on learnings

### Quarterly

- Comprehensive performance review vs. targets
- Revisit assumptions and hypotheses
- Plan next quarter's experiments
- Share learnings with stakeholders

## Success Criteria

This experimentation plan is successful if:

1. **Data-Driven Decisions**: 80%+ of product and marketing decisions backed by data
2. **Velocity**: 2+ experiments running concurrently during growth phase
3. **Learning Rate**: Documented learnings from every experiment (even "failed" ones)
4. **Impact**: 20%+ improvement in key metrics (conversion, retention, ARPU) vs. baseline
5. **Culture**: Team comfortable with experimentation, failure, and iteration

## Next Steps

1. **Implement Task 16 (Analytics Integration)** to enable experimentation infrastructure
2. **Create experiment template** in `experiments/template.md`
3. **Launch EXP001-004** during validation phase (free tier)
4. **Document learnings** from each experiment
5. **Iterate** on hypothesis and experiment design based on results

---

**Last Updated**: 2026-01-18
**Version**: 1.0
**Owner**: Subtitler Team
**Related Documents**: MARKETING_PLAN.md, 001_RALPH_SUBTITLER.md
