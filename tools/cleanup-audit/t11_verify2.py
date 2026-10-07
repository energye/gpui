"""T1.1 复验2：接收者口径——151 可搬函数在 Context 方法定义中各恰一次。"""
import re, os
ROOT = "/home/yanghy/app/projects/gogpu/gpui"
rows = [l.strip().split("|") for l in
        open(os.path.join(ROOT, "tools/cleanup-audit/t11_context_move2.txt"),
             encoding="utf-8") if l.count("|") == 2 and l[0].isdigit()]
ctx = [(r, n) for _, r, n in rows if r in ("<pkg>", "Context")]
print("movable=" + str(len(ctx)))
seen = {}
for dp, _, fns in os.walk(os.path.join(ROOT, "render")):
    for f in fns:
        if not f.endswith(".go") or f.endswith("_test.go"):
            continue
        for i, ln in enumerate(open(os.path.join(dp, f), encoding="utf-8",
                                   errors="ignore"), 1):
            m = re.match(r'^func\s+(?:\(\s*\w+\s+\*?(\w+)\)\s+)?([A-Za-z_]\w*)', ln)
            if not m:
                continue
            recv, name = m.group(1) or "<pkg>", m.group(2)
            if (recv, name) in set(ctx):
                seen.setdefault((recv, name), []).append(
                    os.path.relpath(os.path.join(dp, f), ROOT) + ":" + str(i))
missing = [p for p in set(ctx) if p not in seen]
multi = {k: v for k, v in seen.items() if len(v) > 1}
print("missing=" + str(missing))
print("multi=" + str(multi))
print("OK" if not missing and not multi else "PROBLEM")
