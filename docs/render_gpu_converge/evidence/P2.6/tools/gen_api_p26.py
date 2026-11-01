#!/usr/bin/env python3
"""P2.6-a API snapshot replay (P1.5 ruler, current tree).

Same mechanics as baseline/api/tools/gen_api.py, two changes:
  - package list = CURRENT `go list ./...` (minus the known-broken
    examples/ui_wr_t_stress), not the frozen P1.2b list;
  - output = evidence/P2.6/api (the frozen baseline/api is never written).

Baseline-vs-current package-set and per-package doc diffs are reconciled
against the whitelist in the stage report (PLAN §7.7 + P2.1–P2.5 registered
changes).
"""
import os
import re
import subprocess
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../../../"))
OUT = os.path.join(ROOT, "docs/render_gpu_converge/evidence/P2.6/api")
BASE = os.path.join(ROOT, "docs/render_gpu_converge/baseline/api")
MOD = "github.com/energye/gpui"
FUNC_RE = re.compile(r"^func\s+(Test|Benchmark|Example|Fuzz)[A-Za-z0-9_]*\(")


def main():
    os.makedirs(os.path.join(OUT, "doc"), exist_ok=True)
    r = subprocess.run(["go", "list", "./..."], cwd=ROOT, stdout=subprocess.PIPE,
                       stderr=subprocess.DEVNULL, text=True)
    pkgs = sorted(x for x in r.stdout.splitlines()
                  if x.strip() and not x.endswith("examples/ui_wr_t_stress"))
    with open(os.path.join(OUT, "packages.txt"), "w", encoding="utf-8") as f:
        f.write("\n".join(pkgs) + "\n")

    doc_fail = []
    doc_empty = []
    for i, imp in enumerate(pkgs):
        rel = imp[len(MOD) + 1:] if imp.startswith(MOD + "/") else imp
        dst = os.path.join(OUT, "doc", rel + ".txt")
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
    with open(os.path.join(OUT, "doc_failures.txt"), "w", encoding="utf-8") as f:
        for imp, rc, err in doc_fail:
            f.write(f"{imp}\t{rc}\t{err}\n")
    with open(os.path.join(OUT, "doc_empty_main.txt"), "w", encoding="utf-8") as f:
        f.write("\n".join(doc_empty) + ("\n" if doc_empty else ""))

    # Static test-function counts over the CURRENT package dirs.
    rows = ["pkg\ttest_files\ttest_funcs\tbench_funcs\texample_funcs\tfuzz_funcs"]
    total_files = 0
    total_funcs = 0
    for imp in pkgs:
        rel = imp[len(MOD) + 1:] if imp.startswith(MOD + "/") else imp
        d = os.path.join(ROOT, rel)
        tf = t = b = e = fz = 0
        if os.path.isdir(d):
            for fn in sorted(os.listdir(d)):
                if not fn.endswith("_test.go"):
                    continue
                tf += 1
                with open(os.path.join(d, fn), encoding="utf-8", errors="replace") as f:
                    for line in f:
                        m = FUNC_RE.match(line)
                        if not m:
                            continue
                        total_funcs += 1
                        kind = m.group(1)
                        if kind == "Test":
                            t += 1
                        elif kind == "Benchmark":
                            b += 1
                        elif kind == "Example":
                            e += 1
                        else:
                            fz += 1
        total_files += tf
        rows.append(f"{rel}\t{tf}\t{t}\t{b}\t{e}\t{fz}")
    rows.append(f"TOTAL\t{total_files}\t{total_funcs}")
    with open(os.path.join(OUT, "test_counts.tsv"), "w", encoding="utf-8") as f:
        f.write("\n".join(rows) + "\n")
    print(f"test files={total_files} funcs={total_funcs}")


if __name__ == "__main__":
    main()
