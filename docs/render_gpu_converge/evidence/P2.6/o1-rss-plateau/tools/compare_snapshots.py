#!/usr/bin/env python3
"""Pixel proof for the l1_spinner retained-path fix.

Compares two RetainedSnapshot PNGs taken 2s apart. Every differing pixel must
fall inside the spinner's 48x48 replay box (the snapshot replays the dirty
scope at layer-local origin 0,0); a static retained window would diff 0.
"""

import sys
from PIL import Image

a_path, b_path = sys.argv[1], sys.argv[2]
a = Image.open(a_path).convert("RGBA")
b = Image.open(b_path).convert("RGBA")
if a.size != b.size:
    raise SystemExit(f"size mismatch {a.size} vs {b.size}")
w, h = a.size
pa, pb = a.load(), b.load()

box = (0, 0, 48, 48)  # spinner layer-local box (ReplayBox in l1_spinner)
diff_in = diff_out = 0
for y in range(h):
    for x in range(w):
        if pa[x, y] == pb[x, y]:
            continue
        if box[0] <= x < box[2] and box[1] <= y < box[3]:
            diff_in += 1
        else:
            diff_out += 1

print(f"size={a.size} spinner_box={box}")
print(f"inside_diff={diff_in} outside_diff={diff_out}")
if diff_in == 0:
    raise SystemExit("FAIL: spinner replay is static (no inside diff)")
if diff_out != 0:
    raise SystemExit("FAIL: diffs outside the spinner box")
print("PASS: animated spinner, diff confined to its 48x48 box")
