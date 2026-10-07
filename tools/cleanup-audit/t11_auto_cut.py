"""T1.1 全自动单刀：执行拆分 + 三轮自测，全绿才过，红即停。
用法：python3 tools/cleanup-audit/t11_auto_cut.py tools/cleanup-audit/<spec>.json
spec 字段：src, groups, tests(单测 -run 列表), known_fail(基线存量红),
  timeout(单测秒数)。产物：拆分文件 + T11_PROGRESS.md 追加记录。
绝不提交，只改工作区。
"""
import re, os, sys, json, subprocess, datetime

ROOT = "/home/yanghy/app/projects/gogpu/gpui"
LOG = os.path.join(ROOT, "tools/cleanup-audit/T11_PROGRESS.md")

REC = []


def note(s):
    REC.append(s)
    print(s, flush=True)


def run(cmd, timeout=600):
    p = subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True,
                       timeout=timeout)
    return p.returncode, p.stdout + p.stderr


def funcs_in_file(fp):
    out = []
    try:
        content = open(fp, encoding="utf-8", errors="ignore").read()
    except OSError:
        return out
    pat = re.compile(r'^func\s+(?:\(\s*\w+\s+\*?(\w+)\)\s+)?([A-Za-z_]\w*)')
    for i, ln in enumerate(content.splitlines(), 1):
        m = pat.match(ln)
        if m:
            out.append(((m.group(1) or "<pkg>"), m.group(2), i))
    return out


def main():
    ok = True
    spec = json.load(open(sys.argv[1], encoding="utf-8"))
    src = spec["src"]
    names = [n for ns in spec["groups"].values() for n in ns]
    tmo = spec.get("timeout", 900)
    note("# " + src + " " +
         datetime.datetime.now().strftime("%Y-%m-%d %H:%M"))

    for script in ("t11_split_generic.py", "t11_prune_imports.py"):
        arg = [sys.argv[1]] if "generic" in script else []
        rc, out = run(["python3", "tools/cleanup-audit/" + script] + arg,
                      tmo)
        tail = out.strip().splitlines()[-3:]
        note("RUN " + script + " rc=" + str(rc) + " " + " / ".join(tail))
        if rc != 0:
            note("RED 执行失败")
            ok = False

    st = subprocess.run(["git", "status", "--short", "render/"], cwd=ROOT,
                        capture_output=True, text=True).stdout.splitlines()
    touched = sorted(l[3:].strip().strip('"') for l in st if l.strip())
    note("TOUCHED=" + str(touched))
    gofiles = [os.path.join(ROOT, t) for t in touched if t.endswith(".go")]
    if gofiles:
        subprocess.run(["gofmt", "-w"] + gofiles, cwd=ROOT,
                       capture_output=True)

    left = [f for f in funcs_in_file(os.path.join(ROOT, src))
            if not f[1].startswith(("Test", "Benchmark"))]
    fmt = ""
    if gofiles:
        fmt = subprocess.run(["gofmt", "-l"] + gofiles, cwd=ROOT,
                             capture_output=True, text=True).stdout.strip()
    note("ROUND1 residual_funcs=" + str(len(left)) + " gofmt_dirty=" +
         str(bool(fmt)))
    if left or fmt:
        note("RED 第一轮: 有残留函数或格式未收敛")
        ok = False

    exp = set()
    for ns in spec["groups"].values():
        for e in ns:
            if "." in e:
                r, n = e.split(".", 1)
                exp.add((r, n))
    seen = {}
    topdir = os.path.join(ROOT, "render")
    for f in sorted(os.listdir(topdir)):
        if not f.endswith(".go") or f.endswith("_test.go"):
            continue
        for recv, name, i in funcs_in_file(os.path.join(topdir, f)):
            if (recv, name) in exp:
                seen.setdefault((recv, name), []).append(f)
    missing = sorted(n for n in exp if n not in seen)
    real_multi = {k: v for k, v in seen.items() if len(v) > 1}
    note("ROUND2 missing=" + str(missing) + " multi_in_cut=" +
         str(real_multi))
    if missing or real_multi:
        note("RED 第二轮: 丢失或本刀内重复")
        ok = False
    elif not exp:
        note("RED 第二轮: 分组为空")
        ok = False

    rc, out = run(["go", "build", "./render/"], tmo)
    note("ROUND3 build_rc=" + str(rc))
    if rc != 0:
        note("RED 构建失败: " + out.strip().splitlines()[-5:])
        ok = False
    rc, out = run(["go", "vet", "./render/"], tmo)
    vet_new = [l for l in out.splitlines()
               if "possible misuse" not in l and l.strip()
               and not l.startswith("#")]
    note("ROUND3 vet_new=" + str(vet_new))
    if vet_new:
        note("RED vet 新增问题")
        ok = False

    fails = set()
    passed = 0
    for pat in spec.get("tests", []):
        tcmd = ["go", "test", "./render/", "-run", pat, "-count=1", "-v",
                "-timeout", str(tmo) + "s"]
        rc, out = run(tcmd, tmo + 120)
        for l in out.splitlines():
            m = re.match(r"^--- FAIL: (\S+)", l)
            if m:
                fails.add(m.group(1))
            m2 = re.match(r"^\s*--- PASS", l)
            if m2:
                passed += 1
        note("TESTS pat=" + pat[:40] + " rc=" + str(rc))
    known = set(spec.get("known_fail", []))
    note("ROUND3 passed=" + str(passed) + " fails=" + str(sorted(fails)) +
         " known=" + str(sorted(known)))
    if fails - known:
        note("RED 单测新增失败")
        ok = False
    if spec.get("tests") and passed == 0:
        note("RED 单测零通过: 用例名可能没命中")
        ok = False

    note("VERDICT=" + ("GREEN" if ok else "RED"))
    with open(LOG, "a", encoding="utf-8") as fh:
        fh.write("\n".join(REC) + "\n\n")
    return 0 if ok else 1


sys.exit(main())
