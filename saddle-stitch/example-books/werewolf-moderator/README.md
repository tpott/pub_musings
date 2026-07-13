# werewolf moderator

*The Moderator's Grimoire: Running Werewolf &amp; Mafia* — a saddle-stitch pocket
field guide for the person running the game. It treats Werewolf and Mafia as one
engine (an *informed minority* hidden in an *uninformed majority*) and walks the
whole loop: setup and the wolf-ratio balance dial, the night/dawn/day cycle and
the wake-order, win conditions and parity, a role bestiary (village, wolf,
neutral), thematic scenarios, a gentle "sleep-not-death" version for kids, the
One Night variant, and the moderator's own craft.

Source files:

- `werewolf-moderator.md` — the long-form readable source (prose per page,
  figures noted as *Figure:* captions).
- `werewolf-moderator-design.md` — the print design: page sequence, the figure
  plan, and the CSS approach.
- `werewolf-moderator.html` — the print-ready book. Open in a browser to read on
  screen, or print it to fold a booklet (see the main [README](../../README.md)).

Like [sailboats](../../example-books/sailboats/) and
[text-clustering](../text-clustering/), every figure is an inline SVG
wireframe/diagram — no `images/` subdir, nothing to fetch.

## Page budget

24 pages → 6 folded sheets — **18 numbered content pages** plus the cover, a
contents page, a colophon, and three blanks (the two inside covers and the back
cover). See [Page budget & blank covers](../../README.md#page-budget--blank-covers)
in the main README for the convention. This matches text-clustering (24pp) and
sits between sailboats (12pp / 3 sheets) and peter-rabbit (32pp / 8 sheets).

| Page | Contents | Number shown |
|------|----------|--------------|
| 1  | **Cover** — title + ring-of-villagers SVG | — |
| 2  | Blank (inside front cover) | — |
| 3  | Contents / how to read (one line per numbered page) | — |
| 4–5   | **Foundations** — the game + the wolf-count dial, setup/equipment | 1–2 |
| 6–10  | **Running a round** — the loop & first day, day vote, night wake-order, dawn, parity | 3–7 |
| 11–13 | **The roles** — village (condensed to one page), wolves, neutrals | 8–10 |
| 14–17 | **Making it yours** — scenario table + three "running the scenario" pages (first-morning prompts & dawn kill/save narrations for six scenarios) | 11–14 |
| 18–21 | Kids (with ages research), One Night (per the official rules), craft, reveal/balance | 15–18 |
| 22 | Colophon / THE END + numbered, fact-checked sources | — |
| 23 | Blank (inside back cover) | — |
| 24 | Back cover (blank) | — |

Each numbered content page is one idea: a wireframe figure (or the scenario
mapping table / scenario scripts), then a tight block of prose — see the
[Style](../../README.md#style-words-per-page) budgets in the main README.

Role names are **underlined** (`<span class="role">`) wherever they appear in the
text, and only roles with an entry in the bestiary (pages 8–10) are named at all.

## Print note

Page numbers use an **in-flow `.pageno` element** (pushed to the bottom of a
flexed `.scene`), not `@page` margin-box counters — the latter render as `0` in
practice. The stylesheet is adapted from `sailboats.html`. Reusable figure
glyphs (villager / wolf / dead heads) live in a single `<svg><defs>` block of
`<symbol>`s at the end of `<body>`, placed last so the cover keeps
`:first-child` (SVG resolves `<use>`/marker references by id regardless of DOM
order).

To produce the booklet, render the PDF with headless Chrome and impose it — see
[Printing](../../README.md#printing) in the main README.

## Verifying the layout

Run the shared verifier from the `saddle-stitch/` directory — it renders the
PDF with headless Chrome and has Claude visually audit every page (underfull /
overflow / figure defects / page numbers):

```bash
python verify.py books/werewolf-moderator/werewolf-moderator.html
```

See [Verifying books with Claude](../../README.md#verifying-books-with-claude)
in the main README for the audit criteria and the manual workflow.
