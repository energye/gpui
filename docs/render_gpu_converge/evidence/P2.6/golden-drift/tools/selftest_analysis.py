#!/usr/bin/env python3
"""Negative self-test for analyze_golden.py.

1) identical pair -> 0 diff;
2) 1px shifted current -> non-zero diff and shift_search finds the inverse
   translation (not (0,0));
3) single-pixel mutation -> exactly 1 differing pixel.
"""
from __future__ import annotations
import json
import os
import subprocess
import sys
from PIL import Image, ImageChops

root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
src = os.path.join(root, "runs", "ui_render_base_svg", "native", "current.png")
tmp = os.path.join(root, "selftest-tmp")
os.makedirs(tmp, exist_ok=True)
base = Image.open(src).convert("RGBA")
identical = os.path.join(tmp, "identical.png")
base.save(identical)
shifted = os.path.join(tmp, "shift1px.png")
ImageChops.offset(base, 1, 0).save(shifted)
mutated = base.copy()
p = mutated.load()
r, g, b, a = p[100, 200]
p[100, 200] = ((r + 1) % 256, g, b, a)
mutated_path = os.path.join(tmp, "mutated1px.png")
mutated.save(mutated_path)


def run(cur: str, base_path: str, label: str) -> dict:
    out = os.path.join(tmp, label)
    cmd = [
        sys.executable, os.path.join(root, "tools", "analyze_golden.py"),
        "--current", cur, "--base", base_path, "--out-dir", out,
        "--window", label, "--backend", "selftest",
    ]
    p = subprocess.run(cmd, capture_output=True, text=True)
    if p.returncode != 0:
        raise SystemExit(f"analyzer failed: {p.stderr}")
    return json.loads(p.stdout)


ident = run(identical, src, "identical")
shift = run(shifted, src, "shift1px")
mut = run(mutated_path, src, "mutated1px")

checks = {
    "identical_diff_px": ident["full_diff_px"] == 0,
    "shift_diff_px_nonzero": shift["full_diff_px"] > 0,
    "shift_detected": shift["shift_search"]["best_dx"] != 0 or shift["shift_search"]["best_dy"] != 0,
    "shift_gain_positive": shift["shift_search"]["best_gain_pct"] > 0,
    "mutation_diff_px_one": mut["full_diff_px"] == 1,
}
print(json.dumps({
    "identical": {"diff_px": ident["full_diff_px"]},
    "shift1px": {"diff_px": shift["full_diff_px"], "best_dx": shift["shift_search"]["best_dx"], "best_dy": shift["shift_search"]["best_dy"], "gain_pct": shift["shift_search"]["best_gain_pct"]},
    "mutated1px": {"diff_px": mut["full_diff_px"]},
    "checks": checks,
    "pass": all(checks.values()),
}, ensure_ascii=False, indent=2))
sys.exit(0 if all(checks.values()) else 1)
