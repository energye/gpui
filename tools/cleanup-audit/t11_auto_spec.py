"""T1.1 队列生成：render/ 顶层非测试文件逐个过。
大文件出拆分 spec（按接收者分组），小文件直接记 REVIEW-PASS。
用法：python3 tools/cleanup-audit/t11_auto_spec.py
产物：tools/cleanup-audit/queue/*.json + T11_PROGRESS.md 追加。
"""
import re, os, json, subprocess

ROOT = "/home/yanghy/app/projects/gogpu/gpui"
QDIR = os.path.join(ROOT, "tools/cleanup-audit/queue")
LOG = os.path.join(ROOT, "tools/cleanup-audit/T11_PROGRESS.md")
os.makedirs(QDIR, exist_ok=True)

KNOWN_FAIL = ["TestStrokeExpansion_ScaledDashedRect",
              "TestStrokeString_DifferentFromFill"]
DONE_BASES = {"context", "software", "text", "present_target", "video_direct"}

pat = re.compile(r'^func\s+(?:\(\s*\w+\s+\*?(\w+)\)\s+)?([A-Za-z_]\w*)')
testpat = re.compile(r'^func\s+(Test\w*)\(')


def file_funcs(fp):
    rows = []
    try:
        ls = open(fp, encoding="utf-8", errors="ignore").read().splitlines()
    except OSError:
        return rows, 0
    for i, ln in enumerate(ls, 1):
        m = pat.match(ln)
        if m and not m.group(2).startswith(("Test", "Benchmark")):
            rows.append((i, m.group(1) or "<pkg>", m.group(2)))
    return rows, len(ls)


def related_tests(base):
    names = []
    d = os.path.join(ROOT, "render")
    for f in sorted(os.listdir(d)):
        if f.startswith(base) and f.endswith("_test.go"):
            for ln in open(os.path.join(d, f), encoding="utf-8",
                           errors="ignore"):
                m = testpat.match(ln)
                if m:
                    names.append(m.group(1))
    return names


log = []
queue = []
topdir = os.path.join(ROOT, "render")
for f in sorted(os.listdir(topdir)):
    if not f.endswith(".go") or f.endswith("_test.go"):
        continue
    base = f[:-3]
    if base in DONE_BASES or ("_" in base and base.split("_")[0] in
                              DONE_BASES):
        continue
    rows, nline = file_funcs(os.path.join(topdir, f))
    recvs = sorted({r for _, r, _ in rows})
    if nline <= 400 and len(rows) <= 20:
        log.append("REVIEW-PASS " + f + " lines=" + str(nline) +
                   " funcs=" + str(len(rows)) + " 无需拆分")
        continue
    if len(recvs) <= 2 and len([1 for _, r, _ in rows if r == "<pkg>"]) < 10:
        log.append("REVIEW-PASS " + f + " lines=" + str(nline) +
                   " funcs=" + str(len(rows)) + " 内聚单域无需拆分")
        continue
    groups = {}
    for _, r, n in rows:
        gname = base + "_" + (r.lower() if r != "<pkg>" else "package") \
            + ".go"
        entry = (r + "." + n) if r != "<pkg>" else ("<pkg>." + n)
        groups.setdefault(gname, []).append(entry)
    tests = related_tests(base)
    pats = []
    if tests:
        cur = ""
        for t in tests:
            if len(cur) + len(t) + 1 > 1500:
                pats.append(cur)
                cur = t
            else:
                cur = cur + "|" + t if cur else t
        if cur:
            pats.append(cur)
    spec = {"src": "render/" + f, "groups": groups, "tests": pats,
            "known_fail": KNOWN_FAIL, "timeout": 900}
    sp = os.path.join(QDIR, base + ".json")
    json.dump(spec, open(sp, "w", encoding="utf-8"), indent=1,
              ensure_ascii=False)
    queue.append(base)
    log.append("QUEUED " + f + " lines=" + str(nline) + " funcs=" +
               str(len(rows)) + " groups=" + str(len(groups)) +
               " tests=" + str(len(tests)))

with open(LOG, "a", encoding="utf-8") as fh:
    fh.write("# T1.1 队列重建\n" + "\n".join(log) + "\n\n")
print("\n".join(log))
print("QUEUED_TOTAL=" + str(len(queue)))
