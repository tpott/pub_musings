"""True saddle-stitch imposition: half-letter pages (reading order)
-> landscape-Letter faces (front/back interleaved).

Saddle stitch needs the page count to be a multiple of 4 (one folded sheet
holds 4 pages). Rather than require the source PDF to already be a multiple of
4, we pad the end with blank pages up to the next multiple of 4."""

import sys
from pypdf import PdfReader, PdfWriter, Transformation
from pypdf import PageObject

src, dst = sys.argv[1], sys.argv[2]
r = PdfReader(src)
pages = list(r.pages)
W = float(pages[0].mediabox.width)
H = float(pages[0].mediabox.height)

# Pad up to the next multiple of 4 with blank half-letter pages so the booklet
# folds cleanly. These trailing blanks land on the inside back cover / endpapers.
src_count = len(pages)
pad = (-src_count) % 4
for _ in range(pad):
    pages.append(PageObject.create_blank_page(width=W, height=H))
N = len(pages)
if pad:
    print(f"padded {src_count} -> {N} pages with {pad} blank page(s)")

# Saddle-stitch face order (1-based), front then back of each sheet, outside-in
order = []
for s in range(N // 4):
    order.append((N - 2*s, 1 + 2*s))   # front: outer-left, inner-right
    order.append((2 + 2*s, N - 1 - 2*s))  # back:  inner-left, outer-right

w = PdfWriter()
for left, right in order:
    sheet = w.add_blank_page(width=2*W, height=H)
    sheet.merge_transformed_page(pages[left-1],  Transformation())                 # left half
    sheet.merge_transformed_page(pages[right-1], Transformation().translate(W, 0)) # right half

with open(dst, "wb") as f:
    w.write(f)

# Report the layout
print(f"in: {N} pages @ {W/72:.2f}x{H/72:.2f}in  ->  out: {len(order)} faces @ {2*W/72:.2f}x{H/72:.2f}in")
for i,(l,r) in enumerate(order):
    side = "front" if i%2==0 else "back "
    print(f"  sheet {i//2+1} {side}:  [ left=p{l:<2}  | right=p{r:<2} ]")
