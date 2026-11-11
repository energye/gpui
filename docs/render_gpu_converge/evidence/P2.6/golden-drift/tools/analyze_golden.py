#!/usr/bin/env python3
"""Analyze one current-vs-baseline golden pair for the P2.6 22-window drift.

Outputs a JSON row per pair (full-image diff count, max channel delta, bbox,
HUD/content split, edge-adjacency fraction, interior sample matches, ±2px shift
search, channel-delta histogram) plus visual artifacts:
  diff.png     red = differing pixel (amplified), gray = identical
  compare.png  baseline (top) / current (bottom) side-by-side view
"""
from __future__ import annotations
import argparse
import json
import os
import sys
from collections import Counter
from PIL import Image, ImageChops, ImageFilter, ImageDraw

HEADER_H = 48
HUD_H = 72
EDGE_GRAD = 16


def max_channel_image(a: Image.Image, b: Image.Image) -> Image.Image:
    diff = ImageChops.difference(a, b)
    r, g, bl, al = diff.split()
    m = ImageChops.lighter(ImageChops.lighter(r, g), ImageChops.lighter(bl, al))
    return m


def count_nonzero(img: Image.Image) -> int:
    h = img.histogram()
    return sum(h[1:])


def content_box(size: tuple[int, int]) -> tuple[int, int, int, int]:
    w, h = size
    return (0, min(HEADER_H, h), w, max(0, h - HUD_H))


def region_counts(m: Image.Image) -> dict:
    w, h = m.size
    header = m.crop((0, 0, w, min(HEADER_H, h)))
    hud = m.crop((0, max(0, h - HUD_H), w, h))
    content = m.crop(content_box(m.size))
    return {
        "header_diff": count_nonzero(header),
        "hud_diff": count_nonzero(hud),
        "content_diff": count_nonzero(content),
        "content_px": content.size[0] * content.size[1],
    }


def edge_mask(img: Image.Image) -> Image.Image:
    g = img.convert("L")
    hi = g.filter(ImageFilter.MaxFilter(3))
    lo = g.filter(ImageFilter.MinFilter(3))
    grad = ImageChops.subtract(hi, lo)
    # 255 where local contrast >= EDGE_GRAD
    return grad.point(lambda v: 255 if v >= EDGE_GRAD else 0)


def interior_samples(cur: Image.Image, base: Image.Image, step: int = 40) -> dict:
    w, h = cur.size
    y0, y1 = HEADER_H + 2, max(HEADER_H + 2, h - HUD_H - 2)
    x0, x1 = 2, w - 2
    total = same = 0
    diffs = []
    for y in range(y0, y1, step):
        for x in range(x0, x1, step):
            a = base.getpixel((x, y))
            b = cur.getpixel((x, y))
            total += 1
            if a == b:
                same += 1
            elif len(diffs) < 8:
                diffs.append({"x": x, "y": y, "base": a, "current": b})
    return {"samples": total, "identical": same, "identical_pct": round(100.0 * same / max(1, total), 4), "first_diffs": diffs}


def shift_search(cur: Image.Image, base: Image.Image, max_shift: int = 2) -> dict:
    # Translation that best aligns current onto base inside the content band;
    # max_shift wraps are cropped away so HUD text does not dominate.
    box = content_box(cur.size)
    pc = cur.crop(box)
    pb = base.crop(box)
    w, h = pc.size
    pad = max_shift + 1
    base_c = pb.crop((pad, pad, w - pad, h - pad))
    rows = []
    for dy in range(-max_shift, max_shift + 1):
        for dx in range(-max_shift, max_shift + 1):
            shifted = pc.crop((pad + dx, pad + dy, w - pad + dx, h - pad + dy))
            m = max_channel_image(shifted, base_c)
            rows.append((count_nonzero(m), dx, dy))
    rows.sort()
    zero = next(n for n, dx, dy in rows if dx == 0 and dy == 0)
    best_n, best_dx, best_dy = rows[0]
    return {
        "zero_px": zero,
        "best_px": best_n,
        "best_dx": best_dx,
        "best_dy": best_dy,
        "best_gain_pct": round(100.0 * (zero - best_n) / max(1, zero), 4),
    }


def bucket_hist(m: Image.Image) -> dict:
    h = m.histogram()
    return {
        "delta_1": h[1],
        "delta_2_4": sum(h[2:5]),
        "delta_5_16": sum(h[5:17]),
        "delta_17_64": sum(h[17:65]),
        "delta_gt64": sum(h[65:]),
    }


def find_max_pixels(m: Image.Image, limit: int = 5) -> list:
    w, h = m.size
    data = list(m.getdata())
    mx = max(data) if data else 0
    out = []
    for i, v in enumerate(data):
        if v == mx:
            out.append({"x": i % w, "y": i // w})
            if len(out) >= limit:
                break
    return out


def make_diff(cur: Image.Image, base: Image.Image, m: Image.Image, path: str) -> None:
    w, h = m.size
    out = Image.new("RGB", (w, h), (32, 32, 32))
    px = out.load()
    mp = m.load()
    for y in range(h):
        for x in range(w):
            v = mp[x, y]
            if v:
                s = min(255, 80 + v * 3)
                px[x, y] = (s, 40, 40)
    out.save(path)


def make_compare(cur: Image.Image, base: Image.Image, path: str) -> None:
    w, h = cur.size
    canvas = Image.new("RGB", (w, h * 2 + 6), (255, 255, 255))
    canvas.paste(base.convert("RGB"), (0, 0))
    canvas.paste(cur.convert("RGB"), (0, h + 6))
    d = ImageDraw.Draw(canvas)
    d.text((4, h + 1), "baseline (top) / current (bottom)", fill=(0, 0, 0))
    canvas.save(path)


def main() -> int:
    global HEADER_H, HUD_H
    ap = argparse.ArgumentParser()
    ap.add_argument("--current", required=True)
    ap.add_argument("--base", required=True)
    ap.add_argument("--out-dir", required=True)
    ap.add_argument("--window", required=True)
    ap.add_argument("--backend", required=True)
    ap.add_argument("--official-pct", type=float, default=None)
    ap.add_argument("--official-px", type=int, default=None)
    ap.add_argument("--header-h", type=int, default=HEADER_H)
    ap.add_argument("--hud-h", type=int, default=HUD_H)
    args = ap.parse_args()

    HEADER_H, HUD_H = args.header_h, args.hud_h

    cur = Image.open(args.current).convert("RGBA")
    base = Image.open(args.base).convert("RGBA")
    if cur.size != base.size:
        print(json.dumps({"window": args.window, "backend": args.backend, "error": f"size mismatch {cur.size} vs {base.size}"}))
        return 2

    os.makedirs(args.out_dir, exist_ok=True)
    m = max_channel_image(cur, base)
    diff_count = count_nonzero(m)
    total = cur.size[0] * cur.size[1]
    regions = region_counts(m)
    em_base = edge_mask(base)
    em_cur = edge_mask(cur)
    em = ImageChops.lighter(em_base, em_cur)
    cbox = content_box(cur.size)
    content_m = m.crop(cbox)
    content_em = em.crop(cbox)
    m_mask = content_m.point(lambda v: 255 if v else 0)
    edge_diff = count_nonzero(ImageChops.multiply(m_mask, content_em))
    edge_frac = round(100.0 * edge_diff / max(1, count_nonzero(content_m)), 4)
    content_diff = ImageChops.difference(cur.crop(cbox), base.crop(cbox))
    extrema = content_diff.getextrema()
    alpha_diff = count_nonzero(ImageChops.difference(cur.split()[3], base.split()[3]))
    samples = interior_samples(cur, base)
    shift = shift_search(cur, base)
    hist = bucket_hist(content_m)
    mx = max(v[1] for v in extrema)
    max_pixels = find_max_pixels(content_m)
    for p in max_pixels:
        p["x"] += cbox[0]
        p["y"] += cbox[1]
    cbbox = content_m.getbbox()
    content_bbox = [cbbox[0] + cbox[0], cbbox[1] + cbox[1], cbbox[2] + cbox[0], cbbox[3] + cbox[1]] if cbbox else None

    make_diff(cur, base, m, os.path.join(args.out_dir, "diff.png"))
    make_compare(cur, base, os.path.join(args.out_dir, "compare.png"))

    row = {
        "window": args.window,
        "backend": args.backend,
        "size": list(cur.size),
        "official_golden_pct": args.official_pct,
        "official_golden_px": args.official_px,
        "full_diff_px": diff_count,
        "full_diff_pct": round(100.0 * diff_count / total, 6),
        "max_channel_delta": mx,
        "channel_extrema": [list(e) for e in extrema],
        "diff_bbox": content_bbox,
        "alpha_diff_px": alpha_diff,
        **regions,
        "content_edge_diff_px": edge_diff,
        "content_edge_diff_pct_of_diff": edge_frac,
        "interior_samples": samples,
        "shift_search": shift,
        "delta_hist": hist,
        "max_pixels": max_pixels,
    }
    print(json.dumps(row, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
