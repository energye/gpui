"""T1.1 import 修剪：循环跑 go build，按“imported and not used”删行，直到通过。"""
import re, subprocess, os
ROOT = "/home/yanghy/app/projects/gogpu/gpui"
pat = re.compile(r'(render/\w+\.go):\d+:\d+: "([^"]+)" imported (?:as \w+ )?and not used')
for rnd in range(20):
    p = subprocess.run(["go", "build", "./render/"], cwd=ROOT,
                       capture_output=True, text=True)
    if p.returncode == 0:
        print("BUILD_OK round=" + str(rnd))
        break
    ms = pat.findall(p.stdout + p.stderr)
    if not ms:
        print("BUILD_FAIL_OTHER")
        print((p.stdout + p.stderr)[:3000])
        break
    for f, pkg in ms:
        fp = os.path.join(ROOT, f)
        ls = open(fp, encoding="utf-8").read().splitlines(keepends=True)
        target = '"' + pkg + '"'
        for i, ln in enumerate(ls):
            s = ln.strip()
            if target in ln and not s.startswith("//") and not s.startswith("import"):
                del ls[i]
                break
        open(fp, "w", encoding="utf-8").write("".join(ls))
    print("round=" + str(rnd) + " pruned=" + str(len(ms)))
else:
    print("PRUNE_EXHAUSTED")
