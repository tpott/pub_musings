# saddle stitch

[Saddle stitching](https://en.wikibooks.org/wiki/Bookbinding/Saddle_stitch) may not be the right word, but
that's what I've been using to describe what I have been printing. Maybe [zine](https://en.wikipedia.org/wiki/Zine)
is more appropriate.

# Project directories

Each `books/{book}/` dir should have a `images/` subdir. All `books/` dirs get `.gitignore`'d

# Page budget & blank covers

The front cover, inside front cover, inside back cover, and back cover are 4 overhead
pages; the inside covers (and back cover) are always left blank. Books are sized by
the remaining **content pages**: minimum 6, default 8. Total pages must be a multiple
of 4 (`impose.py` pads with blanks), so 6 content → 12 pages and 8 content → 12 pages,
both 3 folded sheets.

Max is probably about 36 content pages, or 9 real pages. That way we can include a cover
and the back rear for a single real page..

# Style: words per page

Different kinds of books want different page budgets. These are suggestions, not
rules — the real test is the print preview (see [Verifying](#verifying-books)).

- **Children's picture books** (e.g. `example-books/peter-rabbit`): **≤ 200 words
  per page**, one picture per page or spread. The picture carries the page; the
  text is read aloud.
- **Info-dense reference books** (e.g. `example-books/werewolf-moderator`): one idea per
  page — a figure plus roughly **250–300 words**. A page with no figure (or a small one)
  can carry **~350–400 words**.

# Install

Assuming this is for a Mac/Ubuntu that doesn't have `python` but does have `python3`:
`python3 -m venv .venv && source .venv/bin/activate && python -m pip install -r requirements.txt`
(`pypdf` for `impose.py`, `playwright` for `render.py`).

`render.py` drives headless Chromium. It uses whatever chromium playwright already
has in `~/.cache/ms-playwright` (any other playwright install on the machine puts
them there); if there is none, run `playwright install chromium` once.

# Printing

Two steps: render the book HTML to a reading-order PDF, then impose it into
printable letter faces. With the venv active (see [Install](#install)):

**1. Render.**

```bash
python render.py books/{book}/{book}.html books/{book}/{book}.pdf
```

`render.py` is the scripted equivalent of opening the book in a browser and
hitting Print to PDF: it emulates print media and passes `preferCSSPageSize`, so
the book's `@page { size: 5.5in 8.5in }` is what comes out. Backgrounds are off
by default, like a browser's print dialog — pass `--background` to keep them. For
books with hash-selected print modes (see
[peter rabbit](example-books/peter-rabbit/README.md)), pass e.g.
`--fragment ink-lite`. It prints the resulting page count and page size so the
page budget is easy to check.

**2. Impose.**

```bash
python impose.py books/{book}/{book}.pdf books/{book}/{book}.print.pdf
```

Saddle-stitch imposition takes a reading order pdf as input and outputs landscape letter faces
interleaved ((last page, 1st page), (last-1, 2nd page), and so on). Saddle-stitch face order is
1-based, front then back of each sheet, outside-in. `impose.py` pads to a multiple of 4. Print
the `.print.pdf` duplex (flip on **short edge**), fold the stack in half, and staple the spine.

# Verifying books

Needs a PDF rasterizer for Claude's `Read` tool to see pages: `brew install poppler`
(without `pdftoppm`, `Read` on a PDF just errors).

**1. Deterministic scan.** Asserts on the PDF's text — page count, overflow, folio sequence:

```bash
python verify.py books/{book}/{book}.html
```

Note that `verify.py` does its own render rather than calling `render.py`: it shells out to a
system `chrome --headless=new --print-to-pdf` found on `PATH`, and writes `{book}.pdf` next to
the HTML.

**2. Visual audit.** For what only eyes catch — underfull pages and figure defects:

```bash
claude --allowedTools Read --print <<'EOF'
Use the Read tool on books/{book}/{book}.pdf. It may have more than 10 pages, so
read it in chunks with the `pages` parameter until you have looked at EVERY page.
Then report:

1. **underfull** numbered pages (content stops with >~40% blank below)
2. **figure defects** — SVG text overlapping art, clipped diagrams, stray or
   placeholder content left in a figure

Ignore the cover, the inside-cover blanks, and the back cover. Do not report page
counts or totals; verify.py owns those.
EOF
```

# History

1. First attempt was writing a story about Nora and Rayna's morning ritual with chatgpt. I had it write latex. I used my `scratch` remote vm. IIRC it was something like: `ssh scratch 'cd morning-story/ && pdflatex your-story.tex' && scp scratch:morning-story/your-story.pdf ~/Downloads/`. I don't remember how I printed it.
2. I started with [peter rabbit](https://www.gutenberg.org/ebooks/14838), with the HTML.zip. I tried out [superpowers](https://github.com/obra/superpowers) via `claude plugin marketplace add obra/superpowers-marketplace && claude plugin install superpowers@superpowers-marketplace` and then tried some of the skills. I like this method of installing plugins because it's easy to revert with `claude plugin uninstall superpowers && claude plugin marketplace remove superpowers-marketplace`. I used `brainstorming` to write a print design doc, `executing-plans` to modify the main HTML, and `systemic-debugging` to fix the implementation by writing `impose.py`. I later tried adding a nanobana image to fill in a page that was sorely missing one.
3. I tried adding some AI assisted content, to see if I could make something that I enjoyed reading myself. The sailboats example was testing the content size and pages. The werewolf content was more fun and I spent more time on refining it.
