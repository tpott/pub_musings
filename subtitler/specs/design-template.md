# Design Template Analysis

Analysis of Anthropic.com and Ampcode.com design patterns to inform Subtitler's landing page design.

## Executive Summary

Both sites exemplify modern, professional design with strong focus on clarity and developer-friendly aesthetics. Key takeaways:

1. **Warm, neutral color palettes** - Not stark white, but warm cream/beige backgrounds
2. **Large, confident typography** - Bold headlines that make strong statements
3. **Generous whitespace** - Breathing room between sections
4. **Single prominent CTA** - One clear action above the fold
5. **Mobile-first responsive design** - Clean adaptation to smaller screens

## Site Analysis

### Anthropic.com

**Color Palette:**
- Background: `rgb(250, 249, 245)` - warm cream/off-white
- Text: `rgb(20, 20, 19)` - near-black
- Accent: coral/terracotta for illustrations and hover states

**Typography:**
- Primary: "Anthropic Serif", Georgia (fallback)
- Headlines: Large serif, confident statements
- Body: Clean, readable sans-serif

**Layout Patterns:**
- Hero: Large headline left, illustration right
- Cards: Subtle tan/beige backgrounds (`rgb(235, 232, 219)`)
- Navigation: Horizontal, minimal, primary CTA button on right
- Mobile: Hamburger menu, single-column, stacked layout

**Content Strategy:**
- Hero headline is a bold statement, not a product description
- Subtext provides context
- Feature cards use category labels + concise descriptions
- CTAs are action-oriented: "Try Claude", "Read announcement", "Join us"

**Navigation Structure:**
- Top-level: Research, Economic Futures, Commitments, Learn, News
- Primary CTA: "Try Claude" (button with arrow)
- Clear hierarchy: product → resources → company

### Ampcode.com

**Color Palette:**
- Background: `rgb(223, 223, 193)` - warm olive/khaki
- Text: `rgb(11, 13, 11)` - near-black
- Accent: green/gradient blob decorations

**Typography:**
- Primary: System UI stack (system-ui, -apple-system, etc.)
- Headlines: Mixed serif + sans-serif ("Engineered" italic serif, "For The Frontier" sans)
- Very large hero text

**Layout Patterns:**
- Hero: Large headline, subtext, single prominent CTA
- Product demo: Terminal/code preview in center
- Install options: Tab interface (Terminal vs Editor)
- Community section: Live project cards

**Content Strategy:**
- "Engineered For The Frontier" - evocative, technical positioning
- Immediate value prop in subtext
- Pricing transparency upfront ("Pay as you go... Or free, ad-supported")
- Single CTA: "Get Started for Free"

**Navigation Structure:**
- Minimal: Chronicle, Owner's Manual, Models, Amp Free, Pricing
- Account: Sign In, Get Started
- Uses dropdown on mobile

## Design Principles for Subtitler

Based on this analysis, here are recommended design principles:

### 1. Color Palette

**Recommended:**
```css
--bg-primary: #FAF9F5;      /* Warm cream, like Anthropic */
--bg-secondary: #EDEADB;    /* Tan card backgrounds */
--text-primary: #141413;    /* Near-black */
--text-secondary: #5A5A58;  /* Muted for descriptions */
--accent-primary: #C85E3E;  /* Coral/terracotta for CTAs */
--accent-secondary: #3E8A5C; /* Green for success states */
```

**Avoid:**
- Pure white (#FFFFFF) backgrounds - too stark
- Pure black (#000000) text - too harsh
- Overly saturated colors - feels cheap

### 2. Typography Scale

**Headlines:**
```css
--font-display: "Georgia", serif;  /* Or a premium serif */
--font-body: system-ui, -apple-system, sans-serif;

h1 { font-size: 3.5rem; font-weight: 700; line-height: 1.1; }
h2 { font-size: 2rem; font-weight: 600; line-height: 1.2; }
h3 { font-size: 1.25rem; font-weight: 500; line-height: 1.4; }
body { font-size: 1.125rem; line-height: 1.6; }
```

### 3. Layout Structure

**Hero Section:**
```
┌──────────────────────────────────────────────────────┐
│  [Logo]                    [Nav Links]   [CTA Button]│
├──────────────────────────────────────────────────────┤
│                                                      │
│   [Bold Headline]              [Illustration        │
│   [Subtext/Value Prop]          or Product Demo]    │
│   [Primary CTA Button]                              │
│                                                      │
├──────────────────────────────────────────────────────┤
│   [Feature Card 1]  [Feature Card 2]  [Feature Card 3]│
└──────────────────────────────────────────────────────┘
```

**Mobile:**
```
┌─────────────────────────┐
│  [Logo]          [Menu] │
├─────────────────────────┤
│                         │
│  [Illustration]         │
│                         │
│  [Bold Headline]        │
│                         │
│  [Subtext]              │
│                         │
│  [Primary CTA Button]   │
│                         │
├─────────────────────────┤
│  [Feature Card 1]       │
│  [Feature Card 2]       │
│  [Feature Card 3]       │
└─────────────────────────┘
```

### 4. Component Patterns

**Primary CTA Button:**
```css
.btn-primary {
  background: #141413;
  color: #FAF9F5;
  padding: 0.75rem 1.5rem;
  border-radius: 2rem;
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}
.btn-primary::after {
  content: "→";  /* Arrow indicator */
}
```

**Feature Card:**
```css
.feature-card {
  background: #EDEADB;
  padding: 2rem;
  border-radius: 0.5rem;
}
.feature-card h3 {
  margin-bottom: 0.5rem;
}
.feature-card p {
  color: #5A5A58;
}
```

**Navigation:**
```css
nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 2rem;
}
nav a {
  color: #141413;
  text-decoration: none;
  padding: 0.5rem 1rem;
}
nav a:hover {
  text-decoration: underline;
}
```

### 5. Content Writing Guidelines

**Headlines:**
- Make bold statements, not feature lists
- Good: "Create perfect subtitles in seconds"
- Avoid: "AI-Powered Subtitle Generation Tool"

**Subtext:**
- Explain the value proposition clearly
- Good: "Upload any video. Get accurate, editable subtitles. Download or burn-in."
- Avoid: Technical jargon or feature dumps

**CTAs:**
- Single clear action
- Good: "Try it free", "Upload video", "Get started"
- Avoid: "Learn more", "Sign up now", multiple CTAs

### 6. Spacing System

```css
:root {
  --space-xs: 0.25rem;   /* 4px */
  --space-sm: 0.5rem;    /* 8px */
  --space-md: 1rem;      /* 16px */
  --space-lg: 2rem;      /* 32px */
  --space-xl: 4rem;      /* 64px */
  --space-2xl: 8rem;     /* 128px */
}

/* Section spacing */
section { padding: var(--space-2xl) var(--space-lg); }

/* Card spacing */
.card { padding: var(--space-lg); gap: var(--space-md); }

/* Text spacing */
h1 + p { margin-top: var(--space-md); }
p + .btn { margin-top: var(--space-lg); }
```

### 7. Responsive Breakpoints

```css
/* Mobile-first approach */
@media (min-width: 640px)  { /* sm: tablets portrait */ }
@media (min-width: 768px)  { /* md: tablets landscape */ }
@media (min-width: 1024px) { /* lg: small desktops */ }
@media (min-width: 1280px) { /* xl: large desktops */ }
```

## Recommended Landing Page Structure for Subtitler

```markdown
## Hero Section
- Headline: "Generate accurate subtitles for any video"
- Subtext: "Upload, edit, and export subtitles in seconds. AI-powered transcription with manual editing controls."
- CTA: "Upload a Video →"
- Visual: Screenshot of subtitle editor or video player with subtitles

## Features Section (3 cards)
1. **AI Transcription** - "Whisper-powered accuracy for any language"
2. **Smart Editing** - "Edit timing and text with precision"
3. **Multiple Formats** - "Download SRT or burn subtitles into video"

## How It Works Section
1. Upload your video
2. AI generates subtitles
3. Edit and refine
4. Export or embed

## Footer
- Links: About, Privacy, Contact
- Copyright
```

## Implementation Checklist

- [ ] Update color variables in CSS
- [ ] Add serif font for headlines
- [ ] Increase whitespace in hero section
- [ ] Make primary CTA more prominent
- [ ] Add warm background color to cards
- [ ] Simplify navigation
- [ ] Add mobile hamburger menu
- [ ] Update button styles to match pill design
- [ ] Add arrow indicators to CTAs

## Data Files

The raw capture data is in `design-analysis/`:
- `anthropic/` - Screenshots and metadata for anthropic.com
- `ampcode/` - Screenshots and metadata for ampcode.com

Each site directory contains:
- `screenshots/` - Full page, viewport, and mobile screenshots
- `har/` - HTTP Archive files for network analysis
- `*-metadata.json` - Extracted page metadata (colors, headings, CTAs)

## References

- [Anthropic.com](https://www.anthropic.com)
- [Ampcode.com](https://ampcode.com)
- Captured: January 2026
