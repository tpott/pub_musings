"""Render a book HTML to PDF via render.py, then check what is countable.

Usage:
    python verify.py books/{book}/{book}.html

This script only makes assertions that are exact. It deliberately does NOT ask
Claude for a verdict: in testing, a Claude audit of a 25-page book confidently
reported "24 pages, a multiple of 4". It had correctly spotted the overflow that
caused the odd count, but got the arithmetic wrong. Counting is not delegated.

The visual audit (underfull pages, figure defects) lives in the README as a
`claude -p` prompt you read with your own eyes, not as a pass/fail exit code.

The render goes through render.py so that verify and print agree: render.py
emulates print media and passes `preferCSSPageSize`, which is what makes the
book come out at its `@page { size: 5.5in 8.5in }`. An earlier version of this
script shelled out to a system Chrome with a bare `--print-to-pdf`, which does
neither, so it could paginate differently from the PDF you actually print.

Checks:
1. Total page count, and whether it is a multiple of 4.
2. Overflow — a `.scene` that spilled onto a second physical page. Two symptoms,
   both visible in extracted text: a page carrying real content but no page
   number (its in-flow `.pageno` rode along with the spilled tail), and the
   stub page that received the tail (a word-count outlier).
3. Page numbers — the folio is text, so the printed sequence must read
   1, 2, 3, ... A folio of "0" is a known CSS failure mode.
4. Spreads — a two-page event tagged `spread-left` / `spread-right` in the HTML
   must land on one physical opening, which means the left half has to sit on an
   even sheet page. See `check_spreads`.

Exit codes: 0 clean, 1 defects found, 2 tooling error.
"""

import argparse
import re
import sys
from pathlib import Path

from pypdf import PdfReader

import render

# A page holding this much text is carrying real content, not a cover or blank.
CONTENT_WORDS = 40

# Every element whose class *starts* with one of these is one physical page
# (they all carry `page-break-before: always` in the books' print CSS). The
# trailing (?=[\s"]) is what keeps `scene-image` from matching `scene`.
PAGE_CLASSES = ("cover", "blank", "colophon", "intro", "scene", "end")
PAGE_DIV = re.compile(
    r'<div\s+class="(?P<classes>(?:' + "|".join(PAGE_CLASSES) + r')(?=[\s"])[^"]*)"')


def render_pdf(html_path, background=False, fragment=""):
    """Render through render.py, writing the sibling {book}.pdf."""
    pdf_path = html_path.with_suffix(".pdf")
    render.render(str(html_path), str(pdf_path), background=background, fragment=fragment)
    if not pdf_path.exists():
        raise RuntimeError(f"render.py produced no {pdf_path}")
    return pdf_path


def read_pages(pdf_path):
    """Per page: (word count, folio) where folio is the trailing page number."""
    pages = []
    for page in PdfReader(pdf_path).pages:
        words = (page.extract_text() or "").split()
        folio = int(words[-1]) if words and re.fullmatch(r"\d+", words[-1]) else None
        pages.append((len(words), folio))

    # A folio cannot exceed the page count, so a bigger trailing number is some
    # other digit that happens to end the page — a colophon year, a citation.
    return [(w, f if f is None or f <= len(pages) else None) for w, f in pages]


def find_defects(pages):
    defects = []
    numbered = [(i, w, f) for i, (w, f) in enumerate(pages, 1) if f is not None]

    # Overflow leaves a two-page signature: a content page whose .pageno rode
    # away with the spilled tail (so it has no folio), immediately followed by
    # the stub page that caught the tail (a word-count outlier that DOES carry
    # the folio). Requiring both halves is what keeps the contents page and the
    # "THE END" page — legitimately unnumbered — from reading as overflow.
    if numbered:
        typical = sorted(w for _, w, _ in numbered)[len(numbered) // 2]
        stub_max = max(30, typical * 0.25)
        for i, (words, folio) in enumerate(pages, 1):
            if folio is not None or words < CONTENT_WORDS or i >= len(pages):
                continue
            next_words, next_folio = pages[i]  # pages[i] is 1-based page i+1
            if next_folio is not None and next_words < stub_max:
                defects.append(
                    f"p{i} -> p{i + 1}: overflow — p{i} holds {words} words but no "
                    f"page number; its tail and folio (printed {next_folio}) spilled "
                    f"onto p{i + 1}, which carries only {next_words} words vs a "
                    f"{typical}-word median")

    # Page numbers must read 1, 2, 3, ... A "0" is the margin-box counter bug.
    folios = [f for _, _, f in numbered]
    if 0 in folios:
        defects.append("page numbers rendering as 0 — use an in-flow .pageno "
                       "element, not an @page margin-box counter")
    elif folios and folios != list(range(folios[0], folios[0] + len(folios))):
        defects.append(f"page numbers are not sequential: {folios}")

    return defects


def check_spreads(html_path, page_count):
    """A `spread-left` page must face its `spread-right` page.

    Page 1 is a recto (the front cover, a right-hand page), so once the book is
    folded the physical openings are (2,3), (4,5), (6,7) ... — every opening
    starts on an EVEN sheet page. A two-page event therefore only works if its
    left half lands on an even page; one page added or removed anywhere earlier
    in the book flips the parity of everything after it and silently splits the
    spread across two openings.

    Sheet page numbers come from the order of the page-level divs in the HTML,
    which is the print order as long as nothing overflowed — and overflow is
    already its own check, reported separately.
    """
    source = html_path.read_text(encoding="utf-8")
    pages = [m.group("classes").split() for m in PAGE_DIV.finditer(source)]
    defects = []

    if pages and len(pages) != page_count:
        defects.append(
            f"{len(pages)} page-level divs in the HTML but {page_count} pages in "
            f"the PDF — spread positions below are unreliable until that is fixed")

    for i, classes in enumerate(pages, 1):
        if "spread-left" in classes:
            if i % 2:
                defects.append(
                    f"p{i}: spread-left on an odd page — it faces p{i - 1}, not "
                    f"p{i + 1}, so the two halves land on different openings")
            nxt = pages[i] if i < len(pages) else None   # pages[i] is 1-based page i+1
            if nxt is None or "spread-right" not in nxt:
                defects.append(f"p{i}: spread-left is not followed by a spread-right page")
        elif "spread-right" in classes:
            prev = pages[i - 2] if i >= 2 else None
            if prev is None or "spread-left" not in prev:
                defects.append(f"p{i}: spread-right is not preceded by a spread-left page")

    return defects


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("html", type=Path, help="path to the book HTML")
    parser.add_argument("--skip-render", action="store_true",
                        help="check the existing sibling PDF without re-rendering")
    parser.add_argument("--background", action="store_true",
                        help="passed through to render.py: print CSS background fills")
    parser.add_argument("--fragment", default="",
                        help="passed through to render.py: URL #hash for books with "
                             "hash-selected print modes (e.g. ink-lite)")
    args = parser.parse_args()

    if not args.html.exists():
        sys.exit(f"error: {args.html} does not exist")

    if args.skip_render:
        pdf_path = args.html.with_suffix(".pdf")
        if not pdf_path.exists():
            sys.exit(f"error: {pdf_path} does not exist (drop --skip-render)")
    else:
        pdf_path = render_pdf(args.html, args.background, args.fragment)
        print(f"rendered: {pdf_path}")

    pages = read_pages(pdf_path)
    defects = find_defects(pages) + check_spreads(args.html, len(pages))

    print(f"pages: {len(pages)}", end="")
    if len(pages) % 4 == 0:
        print(f"  ({len(pages) // 4} folded sheets)")
    else:
        print(f"  <-- NOT a multiple of 4 (impose.py would pad "
              f"{-len(pages) % 4} blanks)")
        defects.append(f"page count {len(pages)} is not a multiple of 4")

    if not defects:
        print("\nno countable defects")
        print("next: the visual audit (underfull pages, figure defects) — "
              "see README, 'Verifying books'")
        sys.exit(0)

    print(f"\n{len(defects)} defect(s):")
    for defect in defects:
        print(f"  {defect}")
    sys.exit(1)


if __name__ == "__main__":
    main()
