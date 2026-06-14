"""True saddle-stitch imposition: 32 half-letter pages (reading order)
-> 16 landscape-Letter faces (8 sheets, front/back interleaved)."""

import sys
from pypdf import PdfReader, PdfWriter, Transformation

src, dst = sys.argv[1], sys.argv[2]
r = PdfReader(src)
pages = list(r.pages)
N = len(pages)
assert N % 4 == 0, f"page count {N} not divisible by 4"
W = float(pages[0].mediabox.width)
H = float(pages[0].mediabox.height)

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
