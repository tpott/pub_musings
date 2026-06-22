# sail boats

*The Sailboat: A Wireframe Field Guide* — a small saddle-stitch technical manual
of sailing, illustrated entirely with inline wireframe SVGs (no photos, nothing
to fetch). It walks the underbody of a boat: what a keel does, the family of keel
shapes, how to compare hulls by overlaying their profiles, the family of rudders,
and how propeller flow drives both steering and docking.

Source files: `sailboats.html` is the print-ready book — open it in a browser to
read on screen, or print it (see the main [README](../../README.md)) to fold a
booklet. `sailboats.md` is the readable prose source (figures noted as captions).

## Page budget

This book is built at the **default minimum: 8 content pages → 12 total pages →
3 folded sheets**. See the [Page budget & blank covers](../../README.md#page-budget--blank-covers)
section of the main README for the convention. In short:

- The inside front cover (page 2) and inside back cover (page 11) are **blank**,
  as is the back cover (page 12).
- Pages 3–10 are the eight numbered content pages.

| Page | Contents | Number shown |
|------|----------|--------------|
| 1  | **Cover** — title + wireframe sloop | — |
| 2  | Blank (inside front cover) | — |
| 3  | Hull anatomy & what a keel does | 1 |
| 4  | Full & encapsulated keels (the Kraken "Zero Keel") | 2 |
| 5  | Fin, bulb & wing keels | 3 |
| 6  | Shoal-draft & exotic keels (bilge, centerboard, lifting, canting) | 4 |
| 7  | Comfort ratio, by overlaying two hull profiles | 5 |
| 8  | Rudders: transom-hung, spade, skeg-hung, keel-hung | 6 |
| 9  | The skeg, and balanced vs. unbalanced helm | 7 |
| 10 | Prop wash & prop walk | 8 |
| 11 | Blank (inside back cover) | — |
| 12 | Back cover (blank) | — |

Every content page is kept to **at most 200 words**.

## Drawing style

Figures are plain inline `<svg>` "wireframes": thin black strokes, a dashed
waterline, light grey fill only for ballast/solids. Small repeated figures use a
`thumb` class that thickens strokes so they stay legible when shrunk into a row.

The comfort-ratio page demonstrates the **superimposition trick**: two hull
profiles are drawn to the same waterline length, one at full opacity and one at
~40% (the `.ghost` class), then stacked so the difference in displacement and
underbody shape is obvious at a glance.
