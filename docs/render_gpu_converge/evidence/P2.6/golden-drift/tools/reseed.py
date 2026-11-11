#!/usr/bin/env python3
"""Reseed the confirmed 21-window golden set from the analyzed current PNGs.

- old base archived under old-bases/<window>/<basename>
- native current -> native base; when the go snapshot name differs (p38
  color_font/picture), go current -> go base as well
- shared-base windows use native content (native/go content is byte-identical)
"""
from __future__ import annotations
import csv
import hashlib
import os
import shutil
import sys

root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
rows = list(csv.DictReader(open(os.path.join(root, "run-index.tsv")), delimiter="\t"))
skip = {"ui_render_base_svg"}
by = {}
for r in rows:
    by.setdefault(r["window"], {})[r["backend"]] = r


def snapshot_path(r: dict) -> str:
    for line in reversed(open(os.path.join(root, "runs", r["window"], r["backend"], "stderr.log")).read().splitlines()):
        if line.startswith("snapshot: "):
            return line[len("snapshot: "):]
    raise SystemExit(f"no snapshot path for {r['window']}/{r['backend']}")


def sha(path: str) -> str:
    return hashlib.sha256(open(path, "rb").read()).hexdigest()


reseeded = []
for w in sorted(by):
    if w in skip:
        continue
    n = by[w]["native"]
    g = by[w].get("go")
    nsnap = snapshot_path(n)
    nbase = nsnap[:-4] + "_base.png" if nsnap.endswith(".png") else nsnap + "_base.png"
    ncur = os.path.join(root, "runs", w, "native", "current.png")
    targets = [(nbase, ncur)]
    if g:
        gsnap = snapshot_path(g)
        gbase = gsnap[:-4] + "_base.png" if gsnap.endswith(".png") else gsnap + "_base.png"
        if os.path.normpath(gbase) != os.path.normpath(nbase):
            targets.append((gbase, os.path.join(root, "runs", w, "go", "current.png")))
    for base, cur in targets:
        if not os.path.exists(base):
            raise SystemExit(f"base missing: {base}")
        old = os.path.join(root, "old-bases", w, os.path.basename(base))
        os.makedirs(os.path.dirname(old), exist_ok=True)
        if not os.path.exists(old):
            shutil.copy2(base, old)
        old_sha = sha(base)
        new_sha = sha(cur)
        shutil.copy2(cur, base)
        reseeded.append((w, base, old_sha[:16], new_sha[:16]))
        print(f"RESEED {w} {base} {old_sha[:16]} -> {new_sha[:16]}")

print(f"TOTAL {len(reseeded)} base files, {len({w for w, _, _, _ in reseeded})} windows")
