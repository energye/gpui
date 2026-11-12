#!/usr/bin/env python3
"""Summarize O1 probe runs: RSS sub-window slopes, Go heap, VMA attribution."""

import csv
import json
import os
import sys


def load_rss(path):
    pts = []
    with open(path) as f:
        for r in csv.DictReader(f, delimiter="\t"):
            v = int(r["VmRSS"])
            if v > 0:
                pts.append((float(r["t_sec"]), v))
    return pts


def slope(sel):
    if len(sel) < 2:
        return 0.0
    mt = sum(p[0] for p in sel) / len(sel)
    mv = sum(p[1] for p in sel) / len(sel)
    num = sum((p[0] - mt) * (p[1] - mv) for p in sel)
    den = sum((p[0] - mt) ** 2 for p in sel)
    return num / den * 60.0 if den else 0.0


def load_smaps(path):
    if not os.path.exists(path):
        return {}
    return {r["mapping"]: int(r["rss_kb"]) for r in csv.DictReader(open(path), delimiter="\t")}


def official_json(path):
    if not os.path.exists(path):
        return {}
    last = None
    for line in open(path, errors="replace"):
        line = line.strip()
        if line.startswith("{"):
            last = line
    if not last:
        return {}
    try:
        return json.loads(last)
    except json.JSONDecodeError:
        return {}


def run(dirpath):
    rss = load_rss(os.path.join(dirpath, "rss.tsv"))
    tmax = rss[-1][0] if rss else 0
    bands = [(0, 10), (10, 20), (20, 30), (30, 40), (40, 50), (15, tmax), (30, tmax), (40, tmax)]
    sub = []
    for a, b in bands:
        sel = [p for p in rss if a <= p[0] <= b]
        if len(sel) > 1:
            sub.append((f"{a}-{b:.0f}s", slope(sel), sel[0][1] / 1024, sel[-1][1] / 1024))
    gcs = []
    gpath = os.path.join(dirpath, "gctrace.tsv")
    if os.path.exists(gpath):
        gcs = list(csv.DictReader(open(gpath), delimiter="\t"))
    heap_last = gcs[-1] if gcs else None
    maps = {}
    for n in (8, 25, 45):
        s = load_smaps(os.path.join(dirpath, f"smaps_{n}s.tsv"))
        if s:
            maps[n] = s
    growth = []
    if 8 in maps and 45 in maps:
        a, c = maps[8], maps[45]
        for k in set(a) | set(c):
            d = c.get(k, 0) - a.get(k, 0)
            if abs(d) > 512:
                growth.append((d, k, a.get(k, 0), c.get(k, 0)))
        growth.sort(reverse=True)
    off = official_json(os.path.join(dirpath, "stdout.txt"))
    return {
        "dir": dirpath,
        "tmax": tmax,
        "rss_start_mb": rss[0][1] / 1024 if rss else 0,
        "rss_end_mb": rss[-1][1] / 1024 if rss else 0,
        "rss_peak_mb": max(p[1] for p in rss) / 1024 if rss else 0,
        "sub": sub,
        "heap_last": heap_last,
        "growth": growth,
        "official": {
            k: off.get(k)
            for k in (
                "fps_wall",
                "rss_start_kb",
                "rss_end_kb",
                "rss_peak_kb",
                "rss_slope_steady_kb_per_min",
                "rss_slope_valid",
                "present_count",
                "pool",
                "measure_cache_miss",
            )
            if k in off
        },
    }


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else "."
    names = [n for n in sorted(os.listdir(root)) if os.path.isdir(os.path.join(root, n))]
    rows = []
    for n in names:
        if not os.path.exists(os.path.join(root, n, "rss.tsv")):
            continue
        rows.append(run(os.path.join(root, n)))
    out = os.path.join(root, "summary.tsv")
    with open(out, "w") as f:
        f.write("run\trss_start_mb\trss_end_mb\trss_peak_mb\tslope15_end\tslope30_end\tslope40_end"
                "\theap_live_mb\theap_size_mb\tgo_heap_growth_mb\tother_growth_mb\tofficial_steady\tofficial_valid\n")
        for r in rows:
            sub = {s[0]: s[1] for s in r["sub"]}
            g = r["growth"]
            go_g = next((x[0] for x in g if "Go: heap" in x[1]), 0) / 1024
            other_g = sum(x[0] for x in g if "Go: heap" not in x[1]) / 1024
            hl = r["heap_last"]
            f.write(
                f"{os.path.basename(r['dir'])}\t{r['rss_start_mb']:.1f}\t{r['rss_end_mb']:.1f}\t"
                f"{r['rss_peak_mb']:.1f}\t{sub.get('15-56s', sub.get('15-55s', 0)):.0f}\t"
                f"{sub.get('30-56s', sub.get('30-55s', 0)):.0f}\t"
                f"{sub.get('40-56s', sub.get('40-55s', 0)):.0f}\t"
                f"{hl['heap_live_mb'] if hl else ''}\t{hl['heap_end_mb'] if hl else ''}\t"
                f"{go_g:.1f}\t{other_g:.1f}\t"
                f"{r['official'].get('rss_slope_steady_kb_per_min', '')}\t{r['official'].get('rss_slope_valid', '')}\n"
            )
    print(open(out).read())
    for r in rows:
        print(f"== {os.path.basename(r['dir'])}")
        for s in r["sub"]:
            print(f"   {s[0]:>8} slope={s[1]:8.0f} KB/min  {s[2]:6.1f}->{s[3]:6.1f} MB")
        if r["heap_last"]:
            h = r["heap_last"]
            print(f"   heap last: size={h['heap_end_mb']}MB live={h['heap_live_mb']}MB goal={h['goal_mb']}MB")
        for d, k, a, c in r["growth"][:5]:
            print(f"   VMA {k[:48]:<48} {a/1024:7.1f}->{c/1024:7.1f} MB ({d/1024:+.1f})")
        print(f"   official: {r['official']}")


if __name__ == "__main__":
    main()
