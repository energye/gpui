"""T1.1 接收者口径清单：context.go 全函数按 接收者|名字|签名头 分组。"""
import re
fp = "/home/yanghy/app/projects/gogpu/gpui/render/context.go"
lines = open(fp, encoding="utf-8", errors="ignore").read().splitlines()
pat = re.compile(r'^func\s+(?:\(\s*\w+\s+\*?(\w+)\)\s+)?([A-Za-z_]\w*)(\(.*)?')
rows = []
for i, ln in enumerate(lines, 1):
    m = pat.match(ln)
    if m and not m.group(2).startswith(("Test", "Benchmark")):
        rows.append((i, m.group(1) or "<pkg>", m.group(2)))
print("TOTAL=" + str(len(rows)))
foreign = [r for r in rows if r[1] not in ("<pkg>", "Context")]
print("FOREIGN=" + str(foreign))
ctx = [r for r in rows if r[1] in ("<pkg>", "Context")]
print("MOVABLE=" + str(len(ctx)))
for r in rows:
    print(str(r[0]) + "|" + r[1] + "|" + r[2])
