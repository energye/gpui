"""T1.1 恢复：取出被覆盖的两文件中的已搬函数体，还原入库版后追加合并。"""
import re, subprocess, os
ROOT = "/home/yanghy/app/projects/gogpu/gpui"

def bodies(fp):
    lines = open(fp, encoding="utf-8").read().splitlines(keepends=True)
    i = next(n for n, ln in enumerate(lines) if ln.startswith("package "))
    # 跳过 import 块
    j = next(n for n in range(i, len(lines)) if lines[n].startswith("import ("))
    depth = 0
    for k in range(j, len(lines)):
        depth += lines[k].count("(") - lines[k].count(")")
        if lines[k].strip() == ")" or (depth == 0 and k > j):
            return lines[k + 1:]
    return []

saved = {}
for f in ["render/context_layer.go", "render/context_clip.go"]:
    saved[f] = bodies(os.path.join(ROOT, f))
    print(f + " saved_lines=" + str(len(saved[f])))

r = subprocess.run(["git", "checkout", "--", "render/context_layer.go",
                    "render/context_clip.go"], cwd=ROOT, capture_output=True, text=True)
print("checkout rc=" + str(r.returncode) + " " + r.stderr.strip())

for f, body in saved.items():
    fp = os.path.join(ROOT, f)
    with open(fp, "a", encoding="utf-8") as fh:
        fh.write("\n")
        fh.write("".join(body))
    print("appended " + f)
print("RECOVER_DONE")
