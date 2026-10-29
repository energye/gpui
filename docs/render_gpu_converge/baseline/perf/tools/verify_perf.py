#!/usr/bin/env python3
"""P1.7 machine check for baseline/perf/.

Non-zero exit = 口径破坏.  Re-run any time:
  python3 docs/render_gpu_converge/baseline/perf/tools/verify_perf.py
"""
import csv
import glob
import hashlib
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
TSV = os.path.join(HERE, "..", "perf_baseline.tsv")
EXTRACT = os.path.join(HERE, "extract_perf.py")

fails = []


def check(name, ok, detail=""):
    print(("PASS " if ok else "FAIL ") + name + (" — " + detail if detail else ""))
    if not ok:
        fails.append(name)


rows = list(csv.DictReader(open(TSV), delimiter="\t"))
check("perf_baseline.tsv 366 行（P1.4 非种子轮）", len(rows) == 366, f"{len(rows)}")

ff_fail = [r for r in rows if r["first_frame_verdict"].startswith("FAIL")]
check("首帧 0 FAIL", len(ff_fail) == 0, f"{len(ff_fail)}")

blank = [r for r in rows if r["first_frame_verdict"] == "FAIL(first-frame-blank)"]
check("首帧黑屏 0", len(blank) == 0)

# A-族默认门禁 FAIL 必须与 P1.4 状态一致（全部 p14_status=FAIL，零 MISMATCH）
gate_fail = [r for r in rows
             if any("FAIL" in (r[k] or "") for k in ("fps_verdict", "p95_verdict", "hitch_verdict"))]
bad = [r for r in gate_fail if r["p14_status"] != "FAIL" or r["gate_vs_p14"] == "MISMATCH"]
check(f"A 族门禁 FAIL {len(gate_fail)} 轮全部与 P1.4 状态一致（零 MISMATCH）",
      len(bad) == 0, f"{len(gate_fail)} FAIL / 异常 {len(bad)}")

# 别名列只出现在两代命名并存的窗
OLD_GEN = {"kit/spin_min", "ui_l1_spinner", "ui_render_base_perfsoak",
           "ui_render_jumprope", "ui_render_particle", "ui_render_pelican"}
alias_rows = [r for r in rows if "alias-mapped" in r["missing_a_fields"]]
check("alias-mapped 仅限 6 个旧代命名窗 × 2 后端",
      len(alias_rows) == 12 and {r["window"] for r in alias_rows} == OLD_GEN,
      f"{len(alias_rows)} 行")

# A 族适用轮逐轮可解释：有 p95 值，或状态是登记过的采不了档
in_a = [r for r in rows if r["in_a_class"] == "True"]
unexplained = [r for r in in_a
               if r["interval_p95_ms"] == "" and r["p14_status"] not in ("NO_JSON", "BUILD_FAIL", "SKIP")]
check(f"A 族 {len(in_a)} 轮逐轮可解释（有值或登记采不了）", len(unexplained) == 0,
      f"未解释 {len(unexplained)}")

# p99 基线：A 族有 p95 值的轮必须也有 p99（除 video_vr7_perf 已登记缺口）
p99_missing = [r for r in in_a
               if r["interval_p95_ms"] != "" and r["interval_p99_ms"] == ""
               and r["window"] != "video_vr7_perf"]
check("A 族 p99 基线列完整（vr7 登记缺口除外）", len(p99_missing) == 0, f"{len(p99_missing)}")

# 再生成幂等
before = hashlib.md5(open(TSV, "rb").read()).hexdigest()
subprocess.run([sys.executable, EXTRACT], stdout=subprocess.DEVNULL, check=True)
after = hashlib.md5(open(TSV, "rb").read()).hexdigest()
check("extract_perf.py 再生成逐字节一致", before == after)

print("==", "ALL PASS" if not fails else f"{len(fails)} FAIL: {fails}")
sys.exit(1 if fails else 0)
