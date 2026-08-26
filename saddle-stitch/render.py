"""Render a book's HTML to a reading-order PDF at its CSS @page size.

The books set `@page { size: 5.5in 8.5in }` inside a `@media print` block, so
this drives headless Chromium with print media emulated and
`preferCSSPageSize`, which is what a browser's Print-to-PDF does. The output is
still in *reading order* (page 1, 2, 3, ...); feed it to impose.py to get the
folded-sheet layout:

    python render.py books/drip-zine/drip-zine.html books/drip-zine/drip-zine.pdf
    python impose.py books/drip-zine/drip-zine.pdf books/drip-zine/drip-zine-imposed.pdf

Chromium comes from the `playwright` package. If `playwright install chromium`
has not been run (or downloaded a different build than this package expects),
we fall back to the newest chromium already sitting in ~/.cache/ms-playwright,
which is where any other playwright install on the machine puts them.
"""

import argparse
import glob
import os
import re
import sys
from pathlib import Path

from playwright.sync_api import sync_playwright


def find_chromium():
    """Newest ~/.cache/ms-playwright/chromium-<build>/chrome-linux/chrome, if any.

    Returns None when there is nothing cached, in which case we let playwright
    pick its own bundled browser and surface its error if that is missing too.
    """
    root = os.environ.get("PLAYWRIGHT_BROWSERS_PATH") or os.path.expanduser("~/.cache/ms-playwright")
    candidates = []
    for pattern in ("chromium-*/chrome-linux/chrome", "chromium-*/chrome-mac/Chromium.app/Contents/MacOS/Chromium"):
        for path in glob.glob(os.path.join(root, pattern)):
            m = re.search(r"chromium-(\d+)", path)
            candidates.append((int(m.group(1)) if m else 0, path))
    if not candidates:
        return None
    return max(candidates)[1]


def render(src, dst, background=False, fragment="", chrome=None):
    url = Path(src).resolve().as_uri()
    if fragment:
        url += "#" + fragment.lstrip("#")

    launch = {}
    exe = chrome or os.environ.get("CHROME_PATH") or find_chromium()
    if exe:
        launch["executable_path"] = exe

    with sync_playwright() as p:
        browser = p.chromium.launch(**launch)
        page = browser.new_page()
        page.goto(url, wait_until="networkidle")
        # Emulate print media so the @media print rules -- including @page
        # size -- are the ones that apply, then let fonts settle.
        page.emulate_media(media="print")
        page.evaluate("document.fonts.ready")
        page.pdf(
            path=dst,
            prefer_css_page_size=True,   # honor @page { size: 5.5in 8.5in }
            print_background=background, # off by default, matching a browser's print default
            margin={"top": "0", "right": "0", "bottom": "0", "left": "0"},
        )
        browser.close()
    return exe


if __name__ == "__main__":
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("src", help="book HTML file")
    ap.add_argument("dst", help="output PDF (reading order)")
    ap.add_argument("--background", action="store_true",
                    help="print CSS background fills (browsers drop them by default)")
    ap.add_argument("--fragment", default="",
                    help="URL #hash to append, for books with hash-selected print modes (e.g. ink-lite)")
    ap.add_argument("--chrome", default=None, help="path to a chromium/chrome binary")
    args = ap.parse_args()

    exe = render(args.src, args.dst, args.background, args.fragment, args.chrome)
    size = os.path.getsize(args.dst)
    print(f"chromium: {exe or 'playwright default'}")
    print(f"wrote {args.dst} ({size/1024:.0f} KB)")

    # Report the page count and size so the page budget is easy to sanity-check.
    try:
        from pypdf import PdfReader
        r = PdfReader(args.dst)
        pg = r.pages[0]
        w, h = float(pg.mediabox.width), float(pg.mediabox.height)
        print(f"{len(r.pages)} pages @ {w/72:.2f}x{h/72:.2f}in "
              f"({'multiple of 4' if len(r.pages) % 4 == 0 else f'impose.py will pad {(-len(r.pages)) % 4}'})")
    except ImportError:
        pass
