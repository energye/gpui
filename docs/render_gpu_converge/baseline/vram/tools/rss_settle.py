#!/usr/bin/env python3
"""P1.6 附属盘点：全窗 RSS 关窗回落（用 P1.4 存量数据，零 GPU）。

口径（thresholds.md §5，:122-124）：
  - 斜率 > 30000 KB/min 且 slope_valid=true 才判 FAIL；slope_valid=false 的窗
    禁止下结论（不是没漏），只登记。
  - RSS 含 Go 堆，关窗后不立即归还 OS 属正常行为；本盘点是 RSS 侧观察，
    不是显存账本门禁（账本门禁在 ui_base_vram_cap / conformance K1）。

Output: rss_settle.tsv（逐窗）+ stdout 汇总（写进 summary.md 用）。
"""
import glob
import json
import os
import statistics
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../../.."))
WIN = os.path.join(ROOT, "docs/render_gpu_converge/baseline/windows")
OUT = os.path.join(ROOT, "docs/render_gpu_converge/baseline/vram/rss_settle.tsv")

rows = ["window\tvariant\tbackend\tstatus\trss_start_kb\trss_after_close_kb\tdelta_kb"
        "\tslope_steady_kb_per_min\tslope_raw_kb_per_min\tslope_valid\tjudge"]
have = 0
both = []
for p in sorted(glob.glob(os.path.join(WIN, "**", "record.json"), recursive=True)):
    d = json.load(open(p))
    if d.get("run_kind") == "seed":
        continue
    m = d.get("metrics") or {}
    s, a = m.get("rss_start_kb"), m.get("rss_after_close_kb")
    if not isinstance(s, (int, float)) or not isinstance(a, (int, float)):
        continue
    have += 1
    delta = a - s
    # Gates judge the STEADY slope (warmup excluded); the raw slope includes
    # warmup allocations (fonts/caches) and is observation-only.
    slope = m.get("rss_slope_steady_kb_per_min", m.get("rss_slope_kb_per_min"))
    raw_slope = m.get("rss_slope_kb_per_min")
    valid = m.get("rss_slope_valid") is True
    if valid and isinstance(slope, (int, float)) and slope > 30000:
        judge = "LEAK-LINE"
    elif valid:
        judge = "steady-within-band" if slope <= 2000 else "steady-over-band"
    else:
        judge = "short-window-no-verdict"
    rows.append(f"{d.get('window')}\t{d.get('variant')}\t{d.get('backend')}\t{d.get('status')}"
                f"\t{s}\t{a}\t{delta}\t{slope if slope is not None else ''}"
                f"\t{raw_slope if raw_slope is not None else ''}\t{valid}\t{judge}")
    both.append((d.get("window"), d.get("variant"), d.get("backend"), delta, judge))

os.makedirs(os.path.dirname(OUT), exist_ok=True)
with open(OUT, "w", encoding="utf-8") as f:
    f.write("\n".join(rows) + "\n")

deltas = [x[3] for x in both]
deltas.sort()
neg = sum(1 for x in deltas if x <= 0)
pos = [x for x in deltas if x > 0]
judges = {}
for x in both:
    judges[x[4]] = judges.get(x[4], 0) + 1
print(f"records with start+after_close: {have}")
print("NOTE: after_close > start for every Go window is expected (heap + warmup")
print("caches are not returned to the OS); RSS close-settle is NOT a leak gate.")
print("The leak judgment below uses the STEADY slope, exactly like the gates.")
judges = {}
for x in both:
    judges[x[4]] = judges.get(x[4], 0) + 1
print("judges:", judges)
print("valid steady slope > 30000 (leak line), by P1.4 status:")
crossed = [x for x in both if x[4] == "LEAK-LINE"]
for w, v, b, d, j in sorted(crossed, key=lambda x: -x[3]):
    st = dict(((x[0], x[1], x[2]), x[3]) for x in both).get((w, v, b))
    print(f"  {w}/{v}/{b}  delta +{d/1024:.1f}MB")
pass_crossed = []
for p2 in sorted(glob.glob(os.path.join(WIN, "**", "record.json"), recursive=True)):
    d2 = json.load(open(p2))
    if d2.get("run_kind") == "seed" or d2.get("status") != "PASS":
        continue
    m2 = d2.get("metrics") or {}
    s2 = m2.get("rss_slope_steady_kb_per_min", m2.get("rss_slope_kb_per_min"))
    if m2.get("rss_slope_valid") is True and isinstance(s2, (int, float)) and s2 > 30000:
        pass_crossed.append((d2.get("window"), d2.get("variant"), d2.get("backend"), round(s2)))
print(f"PASS records above the 30000 line (gate-coverage question -> P2): {len(pass_crossed)}")
for x in pass_crossed:
    print("  ", x)
