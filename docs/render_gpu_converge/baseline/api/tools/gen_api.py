#!/usr/bin/env python3
"""P1.5 per-package public-API snapshot + test-function counts.

Scope: the same 352-package list as the P1.2 vet baseline
(docs/render_gpu_converge/evidence/P1.2b/packages-default.txt), so the API
snapshot and the test counts share one ruler with P1.2/P1.3.

Outputs (relative to docs/render_gpu_converge/baseline/api/):
  packages.txt        the canonical package list (copied from P1.2b evidence)
  doc/<pkg-rel>.txt   `go doc -all <import>` per package
  test_counts.tsv     per-package static test-function counts
"""
import os
import re
import subprocess
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../../../"))
EVID = os.path.join(ROOT, "docs/render_gpu_converge/evidence/P1.2b/packages-default.txt")
OUT = "/home/yanghy/app/projects/gogpu/gpui/docs/render_gpu_converge/baseline/api"
MOD = "github.com/energye/gpui"
FUNC_RE = re.compile(r"^func\s+(Test|Benchmark|Example|Fuzz)[A-Za-z0-9_]*\(")


def main():
    os.makedirs(OUT, exist_ok=True)
    doc_dir = os.path.join(OUT, "doc")
    os.makedirs(doc_dir, exist_ok=True)

    with open(EVID, encoding="utf-8") as f:
        pkgs = sorted(x.strip() for x in f if x.strip())
    with open(os.path.join(OUT, "packages.txt"), "w", encoding="utf-8") as f:
        f.write("\n".join(pkgs) + "\n")

    # 1) go doc -all per package.  Empty output is expected for `package
    # main` example packages with no exported symbols; anything else empty
    # or any non-zero rc is a failure.
    doc_fail = []
    doc_empty = []
    for i, imp in enumerate(pkgs):
        rel = imp[len(MOD) + 1:] if imp.startswith(MOD + "/") else imp
        dst = os.path.join(doc_dir, rel + ".txt")
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        r = subprocess.run(["go", "doc", "-all", imp], cwd=ROOT,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        if r.returncode != 0:
            doc_fail.append((imp, r.returncode, r.stderr.strip()[:120]))
            continue
        with open(dst, "w", encoding="utf-8") as f:
            f.write(r.stdout)
        if not r.stdout.strip():
            d = os.path.join(ROOT, rel)
            is_main = False
            for fn in os.listdir(d):
                if fn.endswith(".go"):
                    with open(os.path.join(d, fn), encoding="utf-8", errors="replace") as f:
                        for line in f:
                            if line.startswith("package "):
                                is_main = line.strip() == "package main"
                                break
                    if is_main:
                        break
            if is_main:
                doc_empty.append(rel)
            else:
                doc_fail.append((imp, 0, "empty doc output, not package main"))
        if (i + 1) % 50 == 0:
            print(f"doc {i+1}/{len(pkgs)}", flush=True)
    print("doc done, failures:", len(doc_fail))
    for imp, rc, err in doc_fail[:10]:
        print("  FAIL", imp, rc, err)
    print("empty-but-main packages:", len(doc_empty))
    with open(os.path.join(OUT, "doc_empty_main.txt"), "w", encoding="utf-8") as f:
        f.write("\n".join(doc_empty) + ("\n" if doc_empty else ""))

    # 2) static test-function counts per package (one dir per Go package).
    rows = ["pkg\ttest_files\ttest_funcs\tbench_funcs\texample_funcs\tfuzz_funcs"]
    stray = []          # test files outside any listed package dir
    total_files = 0
    total_funcs = 0
    for imp in pkgs:
        rel = imp[len(MOD) + 1:] if imp.startswith(MOD + "/") else imp
        d = os.path.join(ROOT, rel)
        tf = te = tb = tx = tz = 0
        if os.path.isdir(d):
            for fn in sorted(os.listdir(d)):
                if not fn.endswith("_test.go"):
                    continue
                tf += 1
                with open(os.path.join(d, fn), encoding="utf-8", errors="replace") as f:
                    for line in f:
                        if FUNC_RE.match(line):
                            kind = line.split("(")[0].split()[1]
                            if kind.startswith("Test"):
                                te += 1
                            elif kind.startswith("Benchmark"):
                                tb += 1
                            elif kind.startswith("Example"):
                                tx += 1
                            else:
                                tz += 1
                            total_funcs += 1
        rows.append(f"{imp}\t{tf}\t{te}\t{tb}\t{tx}\t{tz}")
        total_files += tf
    with open(os.path.join(OUT, "test_counts.tsv"), "w", encoding="utf-8") as f:
        f.write("\n".join(rows) + "\n")

    # 3) stray check: any *_test.go on disk not under a listed package dir.
    pkg_dirs = set()
    for imp in pkgs:
        rel = imp[len(MOD) + 1:] if imp.startswith(MOD + "/") else imp
        pkg_dirs.add(os.path.join(ROOT, rel))
    for root, dirs, files in os.walk(ROOT):
        dirs[:] = [x for x in dirs if x not in (".git", "testdata", "node_modules")]
        if root not in pkg_dirs:
            for fn in files:
                if fn.endswith("_test.go"):
                    stray.append(os.path.relpath(os.path.join(root, fn), ROOT))

    print(f"packages {len(pkgs)} | test_files {total_files} | test+bench+example+fuzz funcs {total_funcs}")
    print(f"stray test files outside package dirs: {len(stray)}")
    for s in stray[:10]:
        print("  STRAY", s)
    return 0 if not doc_fail else 1


if __name__ == "__main__":
    sys.exit(main())
