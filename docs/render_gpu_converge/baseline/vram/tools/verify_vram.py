#!/usr/bin/env python3
"""P1.6 machine check: evidence integrity for baseline/vram/.

Re-run any time (non-zero exit = check failed):
  python3 docs/render_gpu_converge/baseline/vram/tools/verify_vram.py
"""
import glob
import json
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../../.."))
API = os.path.join(ROOT, "docs/render_gpu_converge/baseline/vram")
CONF = os.path.join(API, "conformance")

fails = []


def check(name, ok, detail=""):
    print(("PASS " if ok else "FAIL ") + name + (" — " + detail if detail else ""))
    if not ok:
        fails.append(name)


# 1) eight cap rounds (4 protocol-runs x 2 backends), each with record+stdout+stderr.
expect = {
    ("ui_base_vram_cap", "ladder"): {"RUN_SECONDS": "300"},
    ("ui_base_vram_cap", "oom"): {"RUN_SECONDS": "60"},
    ("ui_base_multiwin_cap", "default"): {"RUN_SECONDS": "75"},
    ("ui_base_batch_cap", "default"): {"RUN_SECONDS": "200"},
}
recs = {}
for p in sorted(glob.glob(os.path.join(API, "ui_base_*", "*", "*", "record.json"))):
    d = json.load(open(p))
    rel = os.path.relpath(os.path.dirname(p), API)
    recs[tuple(rel.split(os.sep)[:3])] = d
n = 0
for (w, v), envs in expect.items():
    for be in ("native", "go"):
        d = recs.get((w, v, be))
        if d is None:
            check(f"{w}/{v}/{be} record 存在", False)
            continue
        n += 1
        ok = (d.get("status") in ("PASS", "FAIL", "NO_JSON")
              and d.get("run_seconds") is not None
              and (d.get("json") is not None or d.get("status") == "NO_JSON"))
        check(f"{w}/{v}/{be} 记录完整（status/run_seconds/raw json）", ok,
              f"status={d.get('status')} run={d.get('run')}")
check("8 轮 cap 记录齐全", n == 8, f"{n}/8")

# 2) ledger fields present where the protocol emits them.
oom_ledger = 0
for (w, v, be), d in recs.items():
    if (w, v) != ("ui_base_vram_cap", "oom"):
        continue
    ae = ((d.get("json") or {}).get("ability_extra") or {})
    if all(k in ae for k in ("ledger_peak_mb",)) and "pre_inject_live_mb" in (ae.get("oom") or {}):
        oom_ledger += 1
check("oom 两后端均有注入前后账本字段", oom_ledger == 2, f"{oom_ledger}/2")

# ladder: native emits the ledger (ability_extra); go cannot run the 8-window
# ladder on GL (registered: eglMakeCurrent 0x3002 at the 4-window open) — the
# checker asserts that reason is actually evidenced in its stderr.
d = recs.get(("ui_base_vram_cap", "ladder", "native"))
ae = ((d or {}).get("json") or {}).get("ability_extra") or {}
check("ladder/native 账本三要素（峰值/水线/拒增曲线）",
      all(k in ae for k in ("ledger_peak_mb", "ledger_waterline_pct", "refusal_curve")),
      f"peak={ae.get('ledger_peak_mb')}MB waterline={ae.get('ledger_waterline_pct')}")
d = recs.get(("ui_base_vram_cap", "ladder", "go"))
err = open(os.path.join(API, "ui_base_vram_cap", "ladder", "go", "stderr.log"),
           encoding="utf-8", errors="replace").read() if d else ""
check("ladder/go 为登记的跑不了档且 stderr 带 EGL 证据",
      d is not None and d.get("status") == "NO_JSON" and "eglMakeCurrent failed" in err)

# 3) K1 conformance rerun: 3 scenes x 2 backends with /resource/ section.
scenes = ("clip_blend", "solid_gradient", "text_image")
k1 = 0
for sc in scenes:
    for be in ("native", "go"):
        p = os.path.join(CONF, sc, be, "report.json")
        if not os.path.exists(p):
            check(f"conformance/{sc}/{be} report 存在", False)
            continue
        r = json.load(open(p))
        res = r.get("resource") or {}
        if "live_count" in res and "live_bytes" in res:
            k1 += 1
check("K1 复跑 6 份报告含 /resource/ 段", k1 == 6, f"{k1}/6")

# 4) RSS settle inventory file.
tsv = os.path.join(API, "rss_settle.tsv")
lines = open(tsv).read().strip().split("\n") if os.path.exists(tsv) else []
check("rss_settle.tsv 在位且含表头+数据", len(lines) > 100, f"{len(lines)} 行")

print("==", "ALL PASS" if not fails else f"{len(fails)} FAIL: {fails}")
sys.exit(1 if fails else 0)
