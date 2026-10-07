"""T2 全框架导出归类：先建全仓标识索引，再给每个导出贴三态标签。
三态：prod（非测试非示例的跨包调用）/ testonly（仅测试或示例调用）/ unused（零调用）。
用法：python3 tools/cleanup-audit/t2_audit.py
产物：tools/cleanup-audit/t2_exports.json + 终端汇总。
"""
import re, os, json
from collections import defaultdict

ROOT = "/home/yanghy/app/projects/gogpu/gpui"
AREAS = ["render", "gpu", "ui", "engine", "video"]
OUT = os.path.join(ROOT, "tools/cleanup-audit/t2_exports.json")

ident = re.compile(r'[A-Za-z_]\w*')
exp_func = re.compile(r'^func\s+([A-Z]\w*)\s*[\(<]')
exp_type = re.compile(r'^type\s+([A-Z]\w*)\b')
exp_vc = re.compile(r'^(?:var|const)\s+(?:\(\s*)?([A-Z]\w*)')
method = re.compile(r'^func\s+\(\s*\w+\s+\*?(\w+)\)\s+([A-Z]\w*)')

files = []
for area in AREAS:
    base = os.path.join(ROOT, area)
    for dp, _, fns in os.walk(base):
        for f in fns:
            if f.endswith(".go"):
                files.append(os.path.join(dp, f))
print("files=" + str(len(files)), flush=True)

occ = defaultdict(set)
texts = {}
for fp in files:
    try:
        s = open(fp, encoding="utf-8", errors="ignore").read()
    except OSError:
        continue
    texts[fp] = s
    for tok in set(ident.findall(s)):
        if tok[0].isupper():
            occ[tok].add(fp)

exports = []
for fp in files:
    if fp.endswith("_test.go"):
        continue
    rel = os.path.relpath(fp, ROOT)
    area = rel.split("/")[0]
    for i, ln in enumerate(texts[fp].splitlines(), 1):
        m = exp_func.match(ln) or exp_type.match(ln) or exp_vc.match(ln)
        mm = method.match(ln)
        if m:
            exports.append({"sym": m.group(1), "kind": "top",
                            "file": rel, "line": i, "area": area})
        elif mm:
            exports.append({"sym": mm.group(2), "recv": mm.group(1),
                            "kind": "method", "file": rel, "line": i,
                            "area": area})
print("exports=" + str(len(exports)), flush=True)


def classify(e):
    prod, test, ex = [], [], []
    for fp in occ.get(e["sym"], ()):
        if fp == os.path.join(ROOT, e["file"]):
            continue
        if fp.endswith("_test.go"):
            test.append(os.path.relpath(fp, ROOT))
        elif "/examples/" in fp:
            ex.append(os.path.relpath(fp, ROOT))
        else:
            prod.append(os.path.relpath(fp, ROOT))
    if prod:
        return "prod", prod[:5]
    if test or ex:
        return "testonly", (test + ex)[:5]
    return "unused", []


from collections import Counter
c = Counter()
for e in exports:
    tag, ev = classify(e)
    e["tag"] = tag
    e["evidence"] = ev
    c[e["area"] + ":" + tag] += 1
print(json.dumps(dict(sorted(c.items())), indent=1, ensure_ascii=False))
json.dump(exports, open(OUT, "w", encoding="utf-8"), ensure_ascii=False)
print("WROTE " + OUT, flush=True)
