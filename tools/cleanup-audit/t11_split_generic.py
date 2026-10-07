"""T1.1 四刀通用拆分：SRC 按 GROUPS 分文件（签名不动）。
已入库同名文件走追加（原内容不动，import 取并集）。
用法：python3 tools/cleanup-audit/t11_split_generic.py render/present_target.json
清单文件为 JSON：{"src": ..., "groups": {newfile: [names]}}。
"""
import re, os, sys, json, subprocess

ROOT = "/home/yanghy/app/projects/gogpu/gpui"
spec = json.load(open(sys.argv[1], encoding="utf-8"))
SRC = os.path.join(ROOT, spec["src"])
GROUPS = spec["groups"]
NAME2FILE = {}


def key_of(entry):
    if "." in entry:
        r, n = entry.split(".", 1)
        return (r, n)
    return (None, entry)


for f, names in GROUPS.items():
    for n in names:
        k = key_of(n)
        assert k not in NAME2FILE, "duplicate map " + n
        NAME2FILE[k] = f

lines = open(SRC, encoding="utf-8").read().splitlines(keepends=True)
pat = re.compile(r'^func\s+(?:\(\s*\w+\s+\*?(\w+)\)\s+)?([A-Za-z_]\w*)')
idx = [i for i, ln in enumerate(lines) if pat.match(ln)]
print("FUNCS=" + str(len(idx)))

imp_start = next(i for i, ln in enumerate(lines) if ln.startswith("import ("))
depth = 0
imp_end = imp_start
for i in range(imp_start, len(lines)):
    depth += lines[i].count("(") - lines[i].count(")")
    if lines[i].strip() == ")" or (depth == 0 and i > imp_start):
        imp_end = i
        break
header = lines[:9]
import_block = lines[imp_start:imp_end + 1]
ctx_import_paths = re.findall(r'"([^"]+)"', "".join(import_block))

chunks = []
for k, i in enumerate(idx):
    end = idx[k + 1] if k + 1 < len(idx) else len(lines)
    m = pat.match(lines[i])
    chunks.append((m.group(1) or "<pkg>", m.group(2), i, end))


def doc_start(i):
    j = i - 1
    while j >= 0 and lines[j].startswith("//"):
        j -= 1
    return j + 1


def tracked(rel):
    r = subprocess.run(["git", "ls-files", rel], cwd=ROOT,
                       capture_output=True, text=True)
    return bool(r.stdout.strip())


buckets = {f: [] for f in GROUPS}
unmapped = []
for recv, name, i, end in chunks:
    f = NAME2FILE.get((recv, name), NAME2FILE.get((None, name)))
    if f is None:
        unmapped.append((i + 1, recv, name))
        continue
    buckets[f].append((recv + "." + name, doc_start(i), end))

cut = []
for f, items in buckets.items():
    for _, s, e in items:
        cut.append((s, e))
cut.sort()
keep_lines = []
prev = 0
for s, e in cut:
    keep_lines.extend(lines[prev:s])
    prev = e
keep_lines.extend(lines[prev:])
open(SRC, "w", encoding="utf-8").write("".join(keep_lines))
pkgdir = os.path.dirname(spec["src"])

for f, items in buckets.items():
    body = []
    for name, s, e in sorted(items, key=lambda x: x[1]):
        body.extend(lines[s:e])
    outpath = os.path.join(ROOT, pkgdir, f)
    if os.path.exists(outpath) and tracked(os.path.join(pkgdir, f)):
        cur = open(outpath, encoding="utf-8").read().splitlines(keepends=True)
        have = set(re.findall(r'"([^"]+)"', "".join(cur)))
        missing = [p for p in ctx_import_paths if p not in have]
        if missing:
            for i, ln in enumerate(cur):
                if ln.strip() == ")":
                    cur.insert(i, "".join('\t"' + p + '"\n' for p in missing))
                    break
        cur.append("\n")
        cur.extend(body)
        open(outpath, "w", encoding="utf-8").write("".join(cur))
    else:
        out = []
        out.extend(header)
        out.append("\n")
        out.append("package render\n")
        out.append("\n")
        out.extend(import_block)
        out.append("\n")
        out.extend(body)
        open(outpath, "w", encoding="utf-8").write("".join(out))
    print(f + " funcs=" + str(len(items)))

print("UNMAPPED=" + str(unmapped))
print("DONE")
