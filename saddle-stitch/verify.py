"""Render a book HTML to PDF with headless Chrome, then check what is countable.

Usage:
    python verify.py books/{book}/{book}.html

This script only makes assertions that are exact. It deliberately does NOT ask
Claude for a verdict: in testing, a Claude audit of a 25-page book confidently
reported "24 pages, a multiple of 4". It had correctly spotted the overflow that
caused the odd count, but got the arithmetic wrong. Counting is not delegated.

The visual audit (underfull pages, figure defects) lives in the README as a
`claude -p` prompt you read with your own eyes, not as a pass/fail exit code.

Checks:
1. Total page count, and whether it is a multiple of 4.
2. Overflow — a `.scene` that spilled onto a second physical page. Two symptoms,
   both visible in extracted text: a page carrying real content but no page
   number (its in-flow `.pageno` rode along with the spilled tail), and the
   stub page that received the tail (a word-count outlier).
3. Page numbers — the folio is text, so the printed sequence must read
   1, 2, 3, ... A folio of "0" is a known CSS failure mode.

Exit codes: 0 clean, 1 defects found, 2 tooling error.
"""

import argparse
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

from pypdf import PdfReader

CHROME_CANDIDATES = [
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "google-chrome",
    "chromium",
    "chromium-browser",
]

# A page holding this much text is carrying real content, not a cover or blank.
CONTENT_WORDS = 40


def find_chrome():
    # An explicit $CHROME wins: containers and CI images park Chrome in paths
    # no candidate list can guess (e.g. Playwright's /opt/pw-browsers).
    override = os.environ.get("CHROME")
    if override:
        if Path(override).exists() or shutil.which(override):
            return override
        raise RuntimeError(f"$CHROME is set to {override!r}, which is not executable")
    for candidate in CHROME_CANDIDATES:
        if candidate.startswith("/"):
            if Path(candidate).exists():
                return candidate
        elif shutil.which(candidate):
            return candidate
    return None


def render_pdf(chrome, html_path):
    pdf_path = html_path.with_suffix(".pdf")
    cmd = [
        chrome,
        "--headless=new",
        "--disable-gpu",
        "--no-pdf-header-footer",
    ]
    # Chrome's setuid sandbox refuses to start as root, which is the normal
    # user inside a container. Nothing untrusted is being rendered here.
    if hasattr(os, "geteuid") and os.geteuid() == 0:
        cmd.append("--no-sandbox")
    cmd += [
        f"--print-to-pdf={pdf_path}",
        html_path.resolve().as_uri(),
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=120)
    if result.returncode != 0 or not pdf_path.exists():
        raise RuntimeError(
            f"Chrome render failed (exit {result.returncode}):\n{result.stderr}")
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


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("html", type=Path, help="path to the book HTML")
    parser.add_argument("--skip-render", action="store_true",
                        help="check the existing sibling PDF without re-rendering")
    args = parser.parse_args()

    if not args.html.exists():
        sys.exit(f"error: {args.html} does not exist")

    if args.skip_render:
        pdf_path = args.html.with_suffix(".pdf")
        if not pdf_path.exists():
            sys.exit(f"error: {pdf_path} does not exist (drop --skip-render)")
    else:
        try:
            chrome = find_chrome()
        except RuntimeError as exc:
            sys.exit(f"error: {exc}")
        if not chrome:
            sys.exit("error: no Chrome/Chromium binary found "
                     "(set $CHROME to its path)")
        pdf_path = render_pdf(chrome, args.html)
        print(f"rendered: {pdf_path}")

    pages = read_pages(pdf_path)
    defects = find_defects(pages)

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
