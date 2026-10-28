#!/usr/bin/env python3
"""P1.5 machine check: zero missing / zero extra against the package set.

Re-run any time: python3 /tmp/p15/verify_api.py  (non-zero exit = check failed).
"""
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../../../"))
API = os.path.join(ROOT, "docs/render_gpu_converge/baseline/api")
CANON = os.path.join(ROOT, "docs/render_gpu_converge/evidence/P1.2b/packages-default.txt")
MOD = "github.com/energye/gpui"

fails = []


def check(name, ok, detail=""):
    print(("PASS " if ok else "FAIL ") + name + (" — " + detail if detail else ""))
    if not ok:
        fails.append(name)


def rel(imp):
    return imp[len(MOD) + 1:] if imp.startswith(MOD + "/") else imp


canon = sorted(x.strip() for x in open(CANON) if x.strip())
pkgs = sorted(x.strip() for x in open(os.path.join(API, "packages.txt")) if x.strip())
check("packages.txt 与 P1.2 权威清单逐行一致", pkgs == canon, f"{len(pkgs)} 包")

empty_main = sorted(x.strip() for x in open(os.path.join(API, "doc_empty_main.txt")) if x.strip())

doc_rel = set()
doc_empty = []
for root, _d, files in os.walk(os.path.join(API, "doc")):
    for fn in files:
        if fn.endswith(".txt"):
            p = os.path.join(root, fn)
            r = os.path.relpath(p, os.path.join(API, "doc"))[:-4]
            doc_rel.add(r)
            if not open(p, encoding="utf-8", errors="replace").read().strip():
                doc_empty.append(r)

# gpupixel：仅有测试文件的包（P1.2 同口径登记），go doc 无法产出。
expect_doc = {rel(x) for x in pkgs} - {"ui/scene/internal/gpupixel"}
check("doc 文件集 = 包集合 − gpupixel（零缺失零多余）", doc_rel == expect_doc,
      f"缺 {len(expect_doc - doc_rel)} 多 {len(doc_rel - expect_doc)}")
check("空 doc 文件恰为 doc_empty_main.txt 且都是 main 包", sorted(doc_empty) == empty_main,
      f"{len(doc_empty)} 个空文件")

# test_counts.tsv
lines = open(os.path.join(API, "test_counts.tsv")).read().strip().split("\n")
rows = [x.split("\t") for x in lines[1:]]
check("test_counts.tsv 行数 = 352 + 表头", len(rows) == 352, f"{len(rows)} 行")
tset = sorted(x[0] for x in rows)
check("test_counts 包集合与 packages.txt 一致", tset == canon)
tf = sum(int(x[1]) for x in rows)
te = sum(int(x[2]) for x in rows)
tb = sum(int(x[3]) for x in rows)
tx = sum(int(x[4]) for x in rows)
tz = sum(int(x[5]) for x in rows)
check("测试文件总数 = 1397（352 包目录内）", tf == 1397, f"{tf}")
check("四类函数合计 = 14388", te + tb + tx + tz == 14388,
      f"test {te} + bench {tb} + example {tx} + fuzz {tz}")

# 磁盘对账：1468 = 1397 包内 + 1 stress 标签 + 70 third_party（独立模块）
disk = 0
other = []
for root, dirs, files in os.walk(ROOT):
    dirs[:] = [d for d in dirs if d not in (".git", "testdata", "node_modules")]
    r = os.path.relpath(root, ROOT)
    if root in {os.path.join(ROOT, rel(x)) for x in pkgs}:
        disk += sum(1 for f in files if f.endswith("_test.go"))
    else:
        other += [os.path.join(r, f) for f in files if f.endswith("_test.go")]
check("包内测试文件数与 tsv 合计一致", disk == tf, f"{disk}")
stress = [x for x in other if x == "render/tests/stress/analytic_stress_test.go"]
tp = [x for x in other if x.startswith("third_party/")]
check("包外文件 = 1 stress 标签 + 70 third_party（独立模块，无其他）",
      len(stress) == 1 and len(tp) == 70 and len(other) == 71,
      f"stress {len(stress)} third_party {len(tp)} 其他 {len(other) - len(stress) - len(tp)}")

print("==", "ALL PASS" if not fails else f"{len(fails)} FAIL: {fails}")
sys.exit(1 if fails else 0)
