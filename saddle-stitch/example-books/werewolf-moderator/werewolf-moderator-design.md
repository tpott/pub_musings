# Saddle-Stitch Print Design — *The Moderator's Grimoire: Running Werewolf &amp; Mafia*

A print design for turning the research report *The Moderator's Grimoire* into a
readable saddle-stitch booklet, in the same pipeline as the
[sailboats](../../example-books/sailboats/) and
[text-clustering](../text-clustering/) books.

## Goal

Transform a dense, wide-ranging reference report (rules, role tables by faction,
wake orders, balance formulas, reskins, kid variants, moderator tips) into a
single self-contained, print-ready HTML file (`werewolf-moderator.html`) that:

- emits pages in **reading order** at half-letter (5.5″ × 8.5″) so `impose.py`
  can reorder them for the booklet;
- distills each topic to **one page: one explanatory figure plus a tight block
  of prose** (see the main README's [Style](../../README.md#style-words-per-page)
  budgets for info-dense books), rather than reproducing the report verbatim;
- replaces the report's long role tables with **inline SVG diagrams** (no
  `images/` subdir, nothing to fetch — exactly as sailboats does), keeping the
  one genuinely tabular thing (the reskin mapping) as a compact table;
- is **theme-agnostic**: the figures teach the *engine* (informed minority vs.
  majority, the night/day loop, parity) so the reskin page lands naturally.

This is a *reference/how-to* book, so the per-page model is "diagram + tight
prose" (like sailboats/text-clustering), not "photo + sentence" (like
peter-rabbit).

## Output format

- One self-contained HTML file: `books/werewolf-moderator/werewolf-moderator.html`.
  `werewolf-moderator.md` stays as the long-form source; this design governs the
  HTML adaptation.
- Print pages are 5.5″ × 8.5″ (half US Letter, portrait), emitted 1…24 in
  reading order. `impose.py` needs no padding (24 is already a multiple of 4) and
  reorders for saddle stitch.
- Screen view of the same file stays readable in a browser (no print-only
  artifacts on screen) via the `@media screen` / `@media print` split.
- Reuse the sailboats stylesheet wholesale (typography, `svg.wire` classes,
  `.diagram`, the `@page` rules, and the **in-flow `.pageno`** page-number
  mechanism — see [Page numbers](#page-numbers)).

## Page sequence (24 pages = 6 folded sheets)

> **Note (July 2026 revision):** the shipped book diverged from this original
> sequence: the balance dial merged into page 1, the day vote moved to follow
> the loop, the three village-role pages condensed to one, and three "running
> the theme" pages were added. The README's page table is the current map;
> the table below is the original design.

**18 numbered content pages** + 6 overhead/front-matter pages, matching
text-clustering's budget.

| Page | Contents | Figure | No. shown |
|------|----------|--------|-----------|
| 1  | **Cover** — title, subtitle, hero "ring of villagers (2 shaded wolves)" | F0 | — |
| 2  | Blank (inside front cover) | — | — |
| 3  | **Contents / how to read** | — | — |
| 4  | The game in one picture — informed minority vs. majority | F1 | 1 |
| 5  | Setup: pick a moderator, deal the minimal deck | F2 | 2 |
| 6  | How many wolves? the balance dial (~25%, √N, no-reveal) | F3 | 3 |
| 7  | The loop: night → dawn → day → repeat | F4 | 4 |
| 8  | Night: eyes closed, the wake-order | F5 | 5 |
| 9  | Dawn: resolving protections → kills → triggers | F6 | 6 |
| 10 | Day: accuse, defend, vote | F7 | 7 |
| 11 | Winning: parity and the endgame | F8 | 8 |
| 12 | Village roles I — the informers (Seer &c.) | F9 | 9 |
| 13 | Village roles II — protectors & muscle (Doctor, Hunter, Witch) | F10 | 10 |
| 14 | Village roles III — trust/traps (Cupid/Lovers, Masons, Little Girl) | F11 | 11 |
| 15 | Wolf & mafia roles (Godfather, Minion, Sorcerer, Cursed) | F12 | 12 |
| 16 | Neutral & third-party roles (Tanner, Serial Killer, Piper) | F13 | 13 |
| 17 | Reskins — one skeleton, many costumes (**mapping table**) | table | 14 |
| 18 | For kids — the Sandman, sleep not death | F14 | 15 |
| 19 | One Night — no moderator, ten minutes | F15 | 16 |
| 20 | The moderator's craft — script, silence, keep waking the dead | F16 | 17 |
| 21 | Reveal-on-death & balancing | F17 | 18 |
| 22 | **Colophon / THE END** (sources & caveats) | — | — |
| 23 | Blank (inside back cover) | — | — |
| 24 | Back cover (blank) | — | — |

**Counts:** 18 numbered content pages + cover + contents + colophon + 3 blanks =
24 total. 24 ÷ 4 = 6 sheets ✓. If any page overflows in print preview, promote
the most crowded page to a spread → 28 pages / 7 sheets (still ≤ peter-rabbit).

## Figure / SVG plan

All figures are inline `<svg class="wire">` reusing the sailboats classes
(`.ln`, `.thin`, `.fill`, `.fill2`, `.ghost`, `.lead`, `.dash`, `.flow`,
`text.sm`/`text.it`). This book adds a small idiom for **people, arrows, boxes,
and cards**:

```css
svg.wire .dark { fill: #3a3a3a; stroke: #1b1b1b; stroke-width: 1.5; } /* wolf/evil */
svg.wire .arr  { fill: none; stroke: #1b1b1b; stroke-width: 1.5; marker-end: url(#ah); }
svg.wire .arrl { fill: none; stroke: #1b1b1b; stroke-width: 1.1; marker-end: url(#ah); }
svg.wire .box  { fill: #fff; stroke: #1b1b1b; stroke-width: 1.6; }  /* card / node */
svg.wire .chip { fill: #f0f0f0; stroke: #1b1b1b; stroke-width: 1.2; }
```

**Reusable glyphs.** Four `<symbol>`s (drawn with presentation attributes, not
classes, so `<use>` styles correctly) live in one `<svg><defs>` block placed
**last in `<body>`** so the cover keeps `:first-child`:

- `#ig-vil` — villager: outlined head + shoulders.
- `#ig-wolf` — werewolf: dark head, pointed ears, white eyes.
- `#ig-dark` — generic dark figure (neutral killer).
- `#ig-dead` — outlined head with X eyes.

Plus one shared arrowhead `<marker id="ah">`. SVG resolves `<use>`/marker
references by id document-wide regardless of DOM order, so defining them after
they are used is fine.

| ID | Page | Sketch | Teaches |
|----|------|--------|---------|
| F0 | 1  | Ring of 9 heads inside a dashed circle; 2 are `#ig-wolf` | the whole premise on the cover |
| F1 | 4  | 7 villagers + 2 wolves in a dashed ring, wolves labelled "you alone know" | informed minority hidden in the majority |
| F2 | 5  | 9 face-up cards: 2 dark W, 1 Seer (eye), 6 V | the minimal deck |
| F3 | 6  | 3 rows of unit squares (8/12/16 players) with 2/3/4 shaded | ~1 wolf per 4 (~25%) |
| F4 | 7  | 3-node cycle NIGHT→DAWN→DAY with return arrow | the loop |
| F5 | 8  | Numbered vertical wake-order, each row an eye glyph | fixed wake-order, silent signalling |
| F6 | 9  | Shielded head ✓ survives; unshielded ✗ dies + Hunter arrow | dawn resolution order |
| F7 | 10 | Row of heads, accusation arrows converge on one, tally | the day vote |
| F8 | 11 | Level balance beam: 2 wolves = 2 villagers | parity → wolf win |
| F9 | 12 | Seer + crystal ball → target; moderator ✓/✗ | the Seer & the informers |
| F10| 13 | Doctor shield saves; Hunter (dead) arrow takes one | protectors & muscle |
| F11| 14 | Two Lovers + heart; two Masons + handshake | trust/trap social roles |
| F12| 15 | Dark wolf behind a white innocent mask; Seer eye reads ✓ | the Godfather deceiver |
| F13| 16 | Jester-capped Tanner grinning at incoming votes; lone dark SK | neutral win conditions |
| F14| 18 | Sandman sprinkling sleep; head-down villager with "zzz" | sleep, not death, for kids |
| F15| 19 | NIGHT box → DAY box (one vote); swap arrows between two cards | the One Night compression |
| F16| 20 | Moderator + script at circle center; a dead head still "woken" | moderator discipline |
| F17| 21 | Face-up card (Seer, 3:1) vs. face-down card (?, 4–5:1) | reveal vs. no-reveal ratio |

Keep figures to `max-height: 3.1in` (sailboats' value); the reskin-table page
(17) uses the `.dense` class so the table + prose share one sheet.

## Reskin mapping table (page 17)

The report's 8-column reskin table is trimmed to the **6 themes that carry the
point** (Werewolf, Mafia, Western, Pirate, Zombie, Aliens) × the **6 functions**
that never change (Killers, Majority, Investigator, Protector, Evil leader,
"Death"). Rendered with the text-clustering `table.cmp` styling at ~8.4pt so it
fits a single portrait page above the prose.

## Per-page layout pattern

Mirror the sailboats `.scene` block: centered `h2` heading → one `.diagram` svg
(or the mapping table) → a tight block of justified serif prose → `.pageno` pushed
to the bottom by `margin-top:auto` on the flexed `.scene`. Front-matter /
colophon / blanks use `.cover` / `.contents` / `.colophon` / `.blank-page`
wrappers so they don't increment the page counter.

## Page numbers

⚠️ **Known pitfall (carried from text-clustering):** page numbers rendered with
`@page` margin-box counters print as **0**. Use the sailboats fix — an **in-flow
`.pageno` element** pushed to the bottom of a flexed `.scene`, carrying the
number via a CSS counter:

```css
@media print {
  body { counter-reset: page; }
  .scene { counter-increment: page; display: flex; flex-direction: column;
           min-height: 7.1in; }
  .scene .pageno { margin-top: auto; text-align: center; font-size: 9pt;
                   color: #888; }
  .scene .pageno::after { content: counter(page); }
}
.pageno { display: none; }   /* hidden on screen */
```

Only `.scene` blocks increment, so the cover, contents, colophon and blanks stay
unnumbered while content pages read 1…18.

## Content adaptation rules

The HTML is a *distillation*, not a copy:

- **Collapse the faction role tables into three "village" pages, one wolf page,
  and one neutral page**, each foregrounding the 4–6 roles a first-time
  moderator actually needs; deeper roles from the report stay in the source `.md`.
- **Terminology:** use "vote out / eliminate," never "lynch" (per the report's
  own caveat).
- **Keep the actual heuristics** the report settles on: ~1 wolf per 4 players,
  the third wolf near 17, √N with no Seer, 4:1–5:1 under no-reveal, one or two
  special roles per game, change one thing at a time.
- **Preserve the two provenance points** — Davidoff 1986 (Moscow State
  University) and Plotkin 1997 — with the 1986/1987 discrepancy flagged on the
  colophon.

## Imposition workflow

Same as the other books (see main [README](../../README.md)):

1. **Render:** open `werewolf-moderator.html` in Chrome → Print → Save as PDF →
   custom paper size 5.5in × 8.5in → a 24-page half-letter PDF in reading order
   (or use the headless `--print-to-pdf` command in this book's README).
2. **Impose:** `python impose.py werewolf-moderator.pdf werewolf-moderator-imposed.pdf`
   (no padding needed — 24 is already a multiple of 4 — and it interleaves faces).
3. **Print** the imposed Letter PDF duplex (short-edge), fold the 6 sheets,
   staple the spine.

## Testing / verification

- **Screen pass:** open the HTML in a browser, scroll — 24 logical sections in
  order, every figure renders (heads, arrows, cards), no print-only artifacts
  visible.
- **Print-preview pass:** Chrome print preview at 5.5×8.5in — confirm each
  `.scene` fits on one page (no split figures, no orphaned table rows on page
  17), figures don't overflow `max-height`, and page numbers read **1…18**
  (not 0).
- **Impose pass:** run `impose.py`; confirm it reports `in: 24 pages … out: 12
  faces`, no padding.
- **Trial sheet:** print sheets 1 and 6 only (cover + back + first/last content)
  before committing all six.

## Out of scope (this iteration)

- Re-running or re-researching the source material — the HTML adapts the existing
  report only.
- Photographic figures — all figures are SVG wireframes/diagrams.
- An exhaustive role encyclopedia — the booklet is a moderator's field guide, not
  the full multi-edition role list (that stays in the source report).
- Expanding past 24 pages unless print preview forces a spread (then 28 / 7
  sheets, per the page-sequence note).
- Custom fonts beyond the system serif / sans / mono stacks.
