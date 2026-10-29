#!/usr/bin/env python3
"""P1.7 performance baseline extraction from the P1.4 round store.

Pure re-analysis of docs/render_gpu_converge/baseline/windows/**/record.json
(no GPU).  Output: perf_baseline.tsv, one row per non-seed record.

Gate scope (thresholds.md §2/§3, nothing invented):
  H  first frame   time_to_first_present_ms: <2000 普通 UI / <1000 文本输入 /
                   <800 IME 首焦（窗名口径取自 IME 真源）; first_present_paint_count>0
  A  帧时硬门禁    只适用「动画/滚动/持续 tick」窗 = windows.md 帧时/调度 标签
                   ∪ thresholds §3 点名的 R6/R7/R7b/C3/C5/C10:
                   fps_interval>=55(无则 fps_wall), interval_p95_ms<=22,
                   hitch_rate_per_min<=5（仅 >=60s 轮）
  必采九字段       只对 A 族适用窗判 缺一即 FAIL；其余窗缺字段仅登记
  观测             p99（回归带 8ms 留给未来对照）、cpu_pct_avg、非 A 族窗的
                   fps/p95 一律 obs 不判

Every default-gate FAIL is cross-checked against the P1.4 window status:
agree-FAIL stays registered; PASS-vs-FAIL mismatch = 门禁口径差异 → P2.
"""
import glob
import json
import os

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../../.."))
BASE = os.path.join(ROOT, "docs/render_gpu_converge/baseline")
WIN = os.path.join(BASE, "windows")
OUT = os.path.join(BASE, "perf/perf_baseline.tsv")

# A-族适用集：windows.md 的 帧时/调度 标签行 ∪ thresholds §3 点名窗
NAMED = {"ui_wr_r6_layer_anim", "ui_wr_r7_virtlist", "ui_wr_r7b_scroll_reuse",
         "ui_wr_c3_list_scroll", "ui_wr_c5_anim_over_static", "ui_wr_c10_soak"}
ftag = set(NAMED)
for line in open(os.path.join(BASE, "windows.md"), encoding="utf-8"):
    if not line.startswith("| `"):
        continue
    cells = [c.strip() for c in line.split("|")[1:-1]]
    if len(cells) >= 4 and "帧时" in " ".join(cells):
        ftag.add(cells[0].strip("`"))

A_FIELDS = ["fps_wall", "interval_avg_ms", "interval_p50_ms", "interval_p95_ms",
            "interval_p99_ms", "hitch_count", "hitch_rate_per_min", "vsync_source", "target_hz"]
H_FIELD = "time_to_first_present_ms"
TEXT_INPUT = ("ui_wr_ime_r1_editor", "ui_wr_ime_r2_textlayout", "ui_wr_ime_r3_channel",
              "ui_wr_ime_r4_selection", "ui_wr_ime_r5_control", "ui_textinput_ime",
              "ui_text_edit_accept", "ui_text_m4_virtual", "ui_text_m5_anycase")
IME_FIRST_FOCUS = ("ui_wr_ime_r4_selection",)

cols = (["window", "variant", "backend", "p14_status", "family", "run_s",
         H_FIELD, "first_present_paint_count", "fps_interval"] + A_FIELDS +
        ["cpu_pct_avg", "elapsed_sec", "in_a_class",
         "first_frame_budget_ms", "first_frame_verdict", "fps_verdict",
         "p95_verdict", "hitch_verdict", "missing_a_fields", "gate_vs_p14"])
rows = ["\t".join(cols)]

stats = {"records": 0, "have_h": 0, "a_class": 0, "a_class_full": 0}
reg = {"first_frame_fail": [], "fps_fail": [], "p95_fail": [], "hitch_fail": [],
       "missing_a": [], "gate_mismatch": []}

for p in sorted(glob.glob(os.path.join(WIN, "**", "record.json"), recursive=True)):
    d = json.load(open(p))
    if d.get("run_kind") == "seed":
        continue
    m = d.get("metrics") or {}
    j = d.get("json") or {}
    ae = j.get("ability_extra") or {}
    w, v, be = d.get("window"), d.get("variant"), d.get("backend")
    status = d.get("status")
    stats["records"] += 1

    def num(*keys):
        for k in keys:
            for src in (m, j, ae):
                if isinstance(src.get(k), (int, float)):
                    return src[k]
        return ""

    def present(*keys):
        for k in keys:
            for src in (m, j, ae):
                if k in src and src.get(k) not in (None, ""):
                    return src[k]
        return ""

    ttfp = num(H_FIELD)
    fpc = num("first_present_paint_count")
    # Two field generations coexist in window JSONs (P1.4 finding): the
    # current `interval_*` family and the older `frame_interval_*` /
    # `avg_frame_interval_ms` one.  Alias-map the old names so the baseline
    # covers those windows; the naming unification itself is registered P2.
    vals = {k: num(k) for k in A_FIELDS}
    vals["interval_avg_ms"] = num("interval_avg_ms", "avg_frame_interval_ms")
    vals["interval_p50_ms"] = num("interval_p50_ms", "frame_interval_p50_ms")
    vals["interval_p95_ms"] = num("interval_p95_ms", "frame_interval_p95_ms")
    vals["interval_p99_ms"] = num("interval_p99_ms", "frame_interval_p99_ms")
    vals["vsync_source"] = present("vsync_source")
    fps_interval = num("fps_interval")
    if fps_interval == "" and vals["interval_avg_ms"] not in ("", 0):
        fps_interval = 1000.0 / vals["interval_avg_ms"]   # thresholds §3 公式
    alias_used = (num("interval_avg_ms") == "" and vals["interval_avg_ms"] != "")
    cpu = num("cpu_pct_avg")
    elapsed = num("elapsed_sec")
    in_a = w in ftag

    budget = 800 if w in IME_FIRST_FOCUS else 1000 if w in TEXT_INPUT else 2000
    ff_v = ""
    if ttfp != "":
        stats["have_h"] += 1
        if isinstance(fpc, (int, float)) and fpc <= 0:
            ff_v = "FAIL(first-frame-blank)"
            reg["first_frame_fail"].append((w, v, be, "blank"))
        elif ttfp >= budget:
            ff_v = "FAIL"
            reg["first_frame_fail"].append((w, v, be, ttfp))
        else:
            ff_v = "ok"

    fi = fps_interval if fps_interval != "" else vals["fps_wall"]
    fps_v = p95_v = hitch_v = ""
    if in_a:
        stats["a_class"] += 1
        if fi != "" and not (fi < 1):   # fps <1 = 秒/帧量纲窗（如 idle 零脏零帧），只记
            fps_v = "FAIL(<55)" if fi < 55 else "ok"
            if fps_v == "FAIL(<55)":
                reg["fps_fail"].append((w, v, be, fi))
        if vals["interval_p95_ms"] != "":
            p95_v = "FAIL(>22)" if vals["interval_p95_ms"] > 22 else "ok"
            if p95_v == "FAIL(>22)":
                reg["p95_fail"].append((w, v, be, vals["interval_p95_ms"]))
        if (vals["hitch_rate_per_min"] != "" and isinstance(elapsed, (int, float))
                and elapsed >= 60 and vals["hitch_rate_per_min"] > 5):
            hitch_v = "FAIL(>5)"
            reg["hitch_fail"].append((w, v, be, vals["hitch_rate_per_min"]))
        missing = [k for k in A_FIELDS if vals[k] in ("", None)]
        if missing:
            reg["missing_a"].append((w, v, be, status,
                                     ",".join(missing) + (";alias-mapped" if alias_used else "")))
        else:
            stats["a_class_full"] += 1
    else:
        missing = [k for k in A_FIELDS if vals[k] in ("", None)]
    if in_a and status == "PASS" and "FAIL" in (fps_v, p95_v, hitch_v):
        reg["gate_mismatch"].append((w, v, be, status, fps_v, p95_v, hitch_v))

    missing_note = ",".join(missing) + (";alias-mapped" if (in_a and alias_used) else "")
    rows.append("\t".join(str(x) for x in
                [w, v, be, status, d.get("family") or "", d.get("run_seconds") or "",
                 ttfp, fpc, fps_interval] + [vals[k] for k in A_FIELDS] +
                [cpu, elapsed, in_a, budget, ff_v, fps_v or ("obs" if fi != "" else ""),
                 p95_v or ("obs" if vals["interval_p95_ms"] != "" else ""),
                 hitch_v or ("obs" if vals["hitch_rate_per_min"] != "" else ""),
                 missing_note,
                 "MISMATCH" if (in_a and status == "PASS" and "FAIL" in (fps_v, p95_v, hitch_v)) else ""]))

os.makedirs(os.path.dirname(OUT), exist_ok=True)
open(OUT, "w", encoding="utf-8").write("\n".join(rows) + "\n")

print(f"records {stats['records']} | 有首帧 {stats['have_h']} | A 族适用轮 {stats['a_class']}（九字段齐 {stats['a_class_full']}）")
print("A 族适用集:", len(ftag), "窗")
for k, lst in reg.items():
    print(f"{k}: {len(lst)}")
    for x in lst[:20]:
        print("  ", x)
