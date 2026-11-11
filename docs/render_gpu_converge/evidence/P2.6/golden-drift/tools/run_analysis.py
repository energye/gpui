#!/usr/bin/env python3
"""Run analyze_golden.py over every row of run-index.tsv."""
from __future__ import annotations
import csv
import json
import os
import subprocess
import sys

root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
index = os.path.join(root, "run-index.tsv")
out_jsonl = os.path.join(root, "analysis.jsonl")
rows = []
with open(index, newline="") as f:
    for row in csv.DictReader(f, delimiter="\t"):
        cur = os.path.join(root, "runs", row["window"], row["backend"], "current.png")
        base = os.path.join(root, "runs", row["window"], row["backend"], "base.png")
        out_dir = os.path.join(root, "analysis", row["window"], row["backend"])
        cmd = [
            sys.executable,
            os.path.join(root, "tools", "analyze_golden.py"),
            "--current", cur,
            "--base", base,
            "--out-dir", out_dir,
            "--window", row["window"],
            "--backend", row["backend"],
        ]
        if row["golden_pct"]:
            cmd += ["--official-pct", row["golden_pct"], "--official-px", row["total_px"] or "0"]
        p = subprocess.run(cmd, capture_output=True, text=True)
        if p.returncode != 0:
            print("ANALYZE_FAIL", row["window"], row["backend"], p.returncode, p.stderr[-300:], file=sys.stderr)
            continue
        rec = json.loads(p.stdout)
        rows.append(rec)
        print(f"{row['window']}/{row['backend']} official={rec.get('official_golden_pct')} full={rec.get('full_diff_pct')} max={rec.get('max_channel_delta')} edge%={rec.get('content_edge_diff_pct_of_diff')} interior_same%={rec.get('interior_samples',{}).get('identical_pct')}")
with open(out_jsonl, "w") as f:
    for rec in rows:
        f.write(json.dumps(rec, ensure_ascii=False) + "\n")
print(f"WROTE {out_jsonl} rows={len(rows)}")
