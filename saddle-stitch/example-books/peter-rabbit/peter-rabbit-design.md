# Saddle-Stitch Print Design — *The Tale of Peter Rabbit* (PG #14838)

## Goal

Transform `pg14838-images.html` into a print-friendly HTML document that can be saved as a 32-page PDF (one half-letter page per logical page, in reading order), then run through an external booklet-imposition tool to produce a saddle-stitched booklet from 8 folded US Letter sheets.

## Output format

- A single self-contained HTML file. Modify `pg14838-images.html` in place; rely on git for diffs.
- Print pages are 5.5″ × 8.5″ (half of US Letter, portrait).
- Pages are emitted in **reading order** (1, 2, 3 … 32). The external imposition tool reorders them for saddle stitch.
- Screen view of the same file remains readable in a browser (no print-only artifacts visible on screen).

## Page sequence (32 pages = 8 sheets)

| Page | Contents | Page number shown |
|------|----------|-------------------|
| 1 | **Cover** — title "THE TALE OF PETER RABBIT" above, `peter04.jpg` frontispiece, "Beatrix Potter" below | — |
| 2 | Blank (inside front cover) | — |
| 3 | Colophon — `peter02.gif` device, "FREDERICK WARNE", "First published 1902", printer credit | — |
| 4 | Intro text — "Once upon a time there were four little Rabbits…" + the Flopsy/Mopsy/Cotton-tail/Peter indented names list | — |
| 5  | `peter08.jpg` + "They lived with their Mother in a sand-bank…" | 1 |
| 6  | `peter11.jpg` + "'Now my dears,' said old Mrs. Rabbit…" | 2 |
| 7  | `peter12.jpg` + "'Now run along, and don't get into mischief.'" | 3 |
| 8  | `peter15.jpg` + "Then old Mrs. Rabbit took a basket…" | 4 |
| 9  | `peter16.jpg` + "Flopsy, Mopsy, and Cotton-tail, who were good little bunnies…" | 5 |
| 10 | `peter19.jpg` + "But Peter, who was very naughty…" | 6 |
| 11 | `peter20.jpg` + "First he ate some lettuces and some French beans…" | 7 |
| 12 | `peter23.jpg` + "And then, feeling rather sick, he went to look for some parsley." | 8 |
| 13 | `peter24.jpg` + "But round the end of a cucumber frame…" | 9 |
| 14 | `peter27.jpg` + "Mr. McGregor was on his hands and knees…" | 10 |
| 15 | `peter28.jpg` + "Peter was most dreadfully frightened…" + "He lost one of his shoes…" | 11 |
| 16 | `peter31.jpg` + "After losing them, he ran on four legs…" | 12 |
| 17 | `peter32.jpg` + "Peter gave himself up for lost…" | 13 |
| 18 | `peter35.jpg` + "Mr. McGregor came up with a sieve…" | 14 |
| 19 | `peter36.jpg` + "And rushed into the tool-shed…" | 15 |
| 20 | `peter39.jpg` + "Mr. McGregor was quite sure…" + "Presently Peter sneezed—'Kertyschoo!'" | 16 |
| 21 | `peter40.jpg` + "And tried to put his foot upon Peter…" | 17 |
| 22 | `peter43.jpg` + "Peter sat down to rest…" + "After a time he began to wander about…" | 18 |
| 23 | `peter44.jpg` + "He found a door in a wall…" + "An old mouse was running…" | 19 |
| 24 | `peter47.jpg` + "Then he tried to find his way straight across the garden…" | 20 |
| 25 | `peter48.jpg` + "He went back towards the tool-shed…" | 21 |
| 26 | `peter51.jpg` + "Peter got down very quietly off the wheelbarrow…" + "Mr. McGregor caught sight of him…" | 22 |
| 27 | `peter52.jpg` + "Mr. McGregor hung up the little jacket…" + "Peter never stopped running…" | 23 |
| 28 | `peter55.jpg` + "He was so tired that he flopped down…" | 24 |
| 29 | `peter57.jpg` + "I am sorry to say that Peter was not very well…" + "His mother put him to bed…" + "'One table-spoonful to be taken at bed-time.'" | 25 |
| 30 | `peter58.jpg` + "But Flopsy, Mopsy, and Cotton-tail had bread and milk and blackberries for supper." + **THE END** | 26 |
| 31 | Blank (inside back cover) | — |
| 32 | Back cover (blank for now) | — |

**Counts:** 26 story pages (1–26 in story-page numbering), 4 front-matter pages, 2 back-matter pages = 32 total. Divisible by 4. ✓

## Page layout per scene

```
┌──────────────────────────┐  ← top margin 0.6"
│                          │
│        [image]           │  ← max-height ≈ 60% of printable area
│                          │     max-width: 100%
│                          │     preserves aspect ratio, centered
│                          │
│  Text body of the scene, │  ← Georgia/serif, ~12pt, justified
│  flowing below the image.│     line-height 1.4
│  Multiple paragraphs OK. │
│                          │
│ 1                        │  ← page number, outer corner, ~9pt grey
└──────────────────────────┘  ← bottom margin 0.5"
```

- Each scene is wrapped in a `<div class="scene">` with `page-break-inside: avoid` and `page-break-before: always`.
- Image is wrapped in `<div class="scene-image">` with flex centering.
- Text follows in normal flow.

## CSS approach

```css
@media print {
  @page {
    size: 5.5in 8.5in;
    margin: 0.6in 0.5in 0.5in 0.6in;  /* default: gutter on left */
  }
  @page :left {
    margin: 0.6in 0.6in 0.5in 0.5in;  /* gutter on right (verso) */
  }
  /* hide PG boilerplate sections — also removed from markup */
  /* page-break rules for .scene, .cover, .colophon, .intro */
}

@media screen {
  /* preserve existing screen margins (the 20% rules) */
}
```

- All existing top-of-file CSS that affects layout gets wrapped in `@media screen` so it doesn't bleed into print.
- New `@media print` block defines page size, margins, page-break rules, image sizing, page-number content via `@page` content / counters.
- Page numbers use CSS counters scoped to `.scene` elements so front matter doesn't count.

## Content removals

- **Remove entirely:** `<section id="pg-header">` (lines ~171–198) and `<section id="pg-footer">` (lines ~384–737). The Peter Rabbit text (1902) is public domain; PG's license permits removing all PG references in that case.
- **Keep:** `<head>` metadata (dc tags, OpenGraph) — invisible on print.
- **Restructure:** the story body is currently a flat sequence of `<div class="fig">` and `<p>` elements. Wrap each image + its accompanying text paragraph(s) in a `<div class="scene">` to enable per-page break control.
- **Cover, colophon, intro** become explicitly wrapped in `<div class="cover">`, `<div class="colophon">`, `<div class="intro">` with their own page-break rules.

## Margins and gutter for saddle stitch

- Saddle-stitch booklets need a slightly larger **inner (gutter)** margin so binding doesn't eat content.
- Final choice: outer 0.5″, inner 0.6″, top 0.6″, bottom 0.5″.
- `@page :left` / `@page :right` rules flip inner/outer for verso vs recto pages.

## Page numbers

- Story pages only (pages 5–30 of the booklet → story-page numbers 1–26).
- Bottom outer corner, ~9pt, light grey.
- Implemented via CSS counter on `.scene` elements, surfaced through `@page :left { @bottom-left { content: counter(scene); } }` and `:right { @bottom-right ... }`.

## Ink-saving / pencil image modes

Optional, **print-only** image treatments that reduce printer ink usage. They are
opt-in via a single class on `<body>` and change nothing on screen (all rules live
inside `@media print`). They apply uniformly to the three image hooks
(`.scene-image img`, `.cover-image img`, `.colophon-device img`) and leave the
existing image-sizing rules untouched.

| `<body>` class | Effect | Ink |
|----------------|--------|-----|
| *(none)* | Full-tone images — current default, unchanged | most |
| `ink-lite` | Grayscale + brightened + softened contrast; photo preserved | less |
| `pencil` | Edge-detect line-art via an SVG filter; hand-drawn sketch look | least |

Two ways to select a mode (both print-only, screen is untouched):
- **Per-view:** append a URL `#hash` — `peter-rabbit.html#ink-lite` or `#pencil`
  (bare URL = full tone). This is pure CSS: two empty `.mode-anchor` spans at the
  top of `<body>` match `:target`, and `~ *` sibling selectors reach the images.
  Handy for toggling right before "Save as PDF". (CSS can't read `?query` params,
  only the `#hash`.)
- **Default:** change the `class` attribute on `<body>` (e.g. `<body class="pencil">`)
  to set what a bare URL shows. A `#hash` overrides the body class.
`ink-lite` is pure CSS (`grayscale + brightness + contrast`). `pencil` references a
hidden, inert `<svg>` filter (`#pencil`) defined just inside `<body>`; that filter
carries inline comments for its three tuning knobs (line strength, line darkness,
line softness). We intentionally do **not** set `print-color-adjust: exact`, so the
printer's economy default still saves ink.

**Chrome-print caveat:** SVG `filter:url()` rendering in Chrome's print path can be
version-sensitive. Check Print Preview at 5.5″ × 8.5″ and print a single trial sheet
before committing to a full run, especially in `pencil` mode.

## Imposition workflow (manual, this time)

1. **Render**: open the modified HTML in Chrome → Print → "Save as PDF" → set custom paper size 5.5in × 8.5in. Result: a 32-page half-letter PDF in reading order.
2. **Impose**: use any booklet-imposition tool to convert into duplex Letter sheets:
   - macOS Preview: File → Print → Layout → "2 Pages per Sheet", "Two-Sided: Short-Edge". *Caveat: this gives 2-up but not true saddle-stitch order. For true ordering, use one of the next options.*
   - Adobe Acrobat: Print → Booklet (handles saddle-stitch order automatically).
   - CLI: `pdfbook2 --short-edge --paper=letter input.pdf` (from `texlive-extra-utils` / `pdfjam`).
   - Web: bookletcreator.com or similar free tool.
3. **Print** the imposed Letter PDF duplex, fold in half, staple along the spine.

### Future automation notes

- Wrap step 1 + step 2 in a script that calls a headless browser + `pdfbook2`.
- Auto-pad page count to next multiple of 4 with blank pages.
- Template the per-scene markup so other PG image+text books can flow through the same pipeline.

## Testing / verification

- **Visual sanity check**: open modified HTML in browser, scroll through — all 32 logical sections present in order, no PG boilerplate visible.
- **Print preview check**: Chrome Print Preview with paper size 5.5×8.5″ → verify each scene fits on its own page, images don't overflow, text doesn't orphan, page numbers appear on story pages only.
- **Git diff**: review the diff against `pg14838-images.html` initial commit to confirm only intended changes.
- **End-to-end**: produce a single trial sheet (sheets 1 and 8 of the booklet, for cover + last page) before printing all 8.

## Out of scope (this iteration)

- Pre-imposed HTML output (Letter-landscape spreads) — explicitly chosen against.
- Automated PDF imposition pipeline — noted for future, not built now.
- Cover redesign beyond "frontispiece + title text" (Option A) — not building Option B / C.
- Custom fonts beyond the system serif stack.
- Color/decoration on back cover.
