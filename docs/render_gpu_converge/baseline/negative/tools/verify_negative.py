#!/usr/bin/env python3
"""P1.8 machine check for baseline/negative/.  Non-zero exit = 证据缺失."""
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
BASE = os.path.dirname(HERE)  # baseline/negative/
fails = []


def check(name, ok, detail=""):
    print(("PASS " if ok else "FAIL ") + name + (" — " + detail if detail else ""))
    if not ok:
        fails.append(name)


def has(path, *needles):
    try:
        text = open(os.path.join(BASE, path), encoding="utf-8", errors="replace").read()
    except OSError:
        return False
    return all(n in text for n in needles)


# 破坏 1：红 + 绿
check("偏移1px 红输出含 coverage 断言失败", has("shift1px/red-run.txt", "FAIL", "coverage=0.92"))
check("绿基线 PASS", has("green-baseline.txt", "ok  \tgithub.com/energye/gpui/render"))
# 破坏 2：红（像素断言兜住）+ 计数门禁松的证据
check("跳绘红输出含 FAIL", has("skipdraw/red-run.txt", "FAIL"))
check("跳绘 report 留档（gpu_ops 计数门禁偏松的发现）",
      os.path.exists(os.path.join(BASE, "skipdraw/out/solid_gradient/native/report.json")))
# 破坏 3：K1 收紧红 + 还原绿
check("少释放红输出含 resource 断言失败",
      has("skipforget/red-run-k1tighten.txt", "FAIL", "bytes delta=131072", "max 0"))
check("还原后复绿", has("skipforget/green-after-revert.txt", "ok  \tgithub.com/energye/gpui/render"))
check("补丁 diff 留档（含两处临时补丁）", has("patch.diff", "GPUI_NEG_SHIFT_1PX", "GPUI_NEG_SKIP_DRAW", "GPUI_NEG_SKIP_FORGET"))
# §8.2 成对
check("阈值变异三态留档（22 红 / 30 漏报 / 还原报警）",
      has("threshold-mutation.txt", "p95 门禁（读自 thresholds.md）: 22ms", "p95_fail: 4",
          "p95 门禁（读自 thresholds.md）: 30ms", "p95_fail: 0"))
# 汇总
check("summary.md 在位", os.path.exists(os.path.join(BASE, "summary.md")))

print("==", "ALL PASS" if not fails else f"{len(fails)} FAIL: {fails}")
sys.exit(1 if fails else 0)
