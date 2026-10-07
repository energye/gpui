"""T1.1 复验：152 个函数在拆后 render/ 非测试文件中各定义恰一次。"""
import re, os
ROOT = "/home/yanghy/app/projects/gogpu/gpui"
rows = [l.strip().split("|") for l in
        open(os.path.join(ROOT, "tools/cleanup-audit/t11_context_move.txt"),
             encoding="utf-8")
        if "|" in l and not l.startswith("TOTAL") and not l.startswith("GROUPS")]
print("listed=" + str(len(rows)))
defs = {}
for dp, _, fns in os.walk(os.path.join(ROOT, "render")):
    for f in fns:
        if not f.endswith(".go") or f.endswith("_test.go"):
            continue
        for i, ln in enumerate(open(os.path.join(dp, f), encoding="utf-8",
                                   errors="ignore"), 1):
            m = re.match(r'^func\s+(?:\(\s*\w+\s+\*?\w+\)\s+)?([A-Za-z_]\w*)', ln)
            if m:
                defs.setdefault(m.group(1), []).append(
                    os.path.relpath(os.path.join(dp, f), ROOT) + ":" + str(i))
missing = [r[2] for r in rows if r[2] not in defs]
multi = {k: v for k, v in defs.items()
         if k in {r[2] for r in rows} and len(v) > 1}
print("MISSING=" + str(missing))
print("MULTI=" + str(multi))
print("OK" if not missing and not multi else "PROBLEM")
