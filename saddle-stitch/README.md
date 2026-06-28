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

# Impose Install

Assuming this is for a Mac/Ubuntu that doesn't have `python` but does have `python3`:
`python3 -m venv .venv && source .venv/bin/activate && python -m pip install -r requirements.txt` (to get `pypdf` to run `python impose.py {source pdf} {target pdf}`)

Saddle-stitch imposition takes a reading order pdf as input and outputs landscape letter faces interleaved ((last page, 1st page), (last-1, 2nd page), and so on). Saddle-stitch face order is 1-based, front then back of each sheet, outside-in.

# History

1. First attempt was writing a story about Nora and Rayna's morning ritual with chatgpt. I had it write latex. I used my `scratch` remote vm. IIRC it was something like: `ssh scratch 'cd morning-story/ && pdflatex your-story.tex' && scp scratch:morning-story/your-story.pdf ~/Downloads/`. I don't remember how I printed it.
2. I started with [peter rabbit](https://www.gutenberg.org/ebooks/14838), with the HTML.zip. I tried out [superpowers](https://github.com/obra/superpowers) via `claude plugin marketplace add obra/superpowers-marketplace && claude plugin install superpowers@superpowers-marketplace` and then tried some of the skills. I like this method of installing plugins because it's easy to revert with `claude plugin uninstall superpowers && claude plugin marketplace remove superpowers-marketplace`. I used `brainstorming` to write a print design doc, `executing-plans` to modify the main HTML, and `systemic-debugging` to fix the implementation by writing `impose.py`. I later tried adding a nanobana image to fill in a page that was sorely missing one.
