#!/usr/bin/env python3
"""P2.6-c 真窗全量重放编排器 v2（P1.4 逐轮对齐重建）。

轮次清单 = baseline/windows/summary.tsv 的 (window, variant) 对 × 双后端；
每轮命令 = 基线 record.json 的 args（--case=…/-auto-only 原样重放）+ 基线 stdout
解析出的 RUN_SECONDS；cwd=仓库根（窗内相对 testdata/金图路径按 P1.4 口径解析）；
0* 运行期金图窗把 baseline/windows/<win>/default/native/snapshots 的冻结 *_base.png
拷到证据目录后以其为 <族>_SNAP_DIR（不带 GPUI_ACCEPT_GOLDEN，窗内对冻结基线逐位
diff）；baseline/ 只读。显示 :3 rootless Xwayland；轮前健康检查；rc!=0 或
fps_wall<15 自动重试一次（retry 记录在案）；断点续跑。
输出：evidence/P2.6/c-windows/<win>/<variant>/<backend>/{record.json,stdout.txt,stderr.log}
      + evidence/P2.6/c-windows/summary.tsv（列序与基线 summary.tsv 一致）。
"""
import json
import os
import re
import shutil
import subprocess
import sys
import time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../../../"))
BASEWIN = os.path.join(ROOT, "docs/render_gpu_converge/baseline/windows")
OUT = os.path.join(ROOT, "docs/render_gpu_converge/evidence/P2.6/c-windows")
MD = os.path.join(ROOT, "docs/render_gpu_converge/baseline/windows.md")
BIN = "/tmp/p26-bin"
SNAP_WINDOWS = {
    "ui_wr_r1_layout": "R1_SNAP_DIR", "ui_wr_r14_text": "R14_SNAP_DIR",
    "ui_wr_r15_clip": "R15_SNAP_DIR", "ui_wr_r20_filter": "R20_SNAP_DIR",
    "ui_wr_r22_selection_stub": "R22_SNAP_DIR", "ui_wr_c6_savelayer_group": "C6_SNAP_DIR",
    "ui_wr_c9_backdrop": "C9_SNAP_DIR", "ui_wr_c10_damage": "C10_SNAP_DIR",
}
# 基线 record.json 未记录采集器环境变量；这两组的注入/时长按 P1.4/P1.6 基线证据显式还原。
ROUND_OVERRIDES = {
    ("ui_base_multiwin_cap", "default"): {"secs": 75},
    ("ui_base_vram_cap", "oom"): {"secs": 60, "env": {"GPUI_FAULT_OOM": "180:1"}},
    ("ui_base_vram_cap", "ladder"): {"secs": 300},
}
PLATFORM_SKIP = {"ui_pf_win32": "平台探针：本机无 Windows（P1.4 同口径）",
                 "ui_pf_appkit": "平台探针：本机无 macOS（P1.4 同口径）"}
XDISPLAY = ":3"
COLS = ["window", "variant", "backend", "run_kind", "status", "rc", "tolerance",
        "golden_diff", "golden_total_px", "golden_first", "p95_ms", "p99_ms",
        "fps_interval", "fps_wall", "vsync_source", "gpu_backend",
        "cpu_fallback_ops", "snapshots", "goldens", "notes"]


def md_secs():
    """windows.md 逐窗运行档秒数 → {win: (kind, secs)}。"""
    import re as _re
    out, sec = {}, None
    for line in open(MD, encoding="utf-8"):
        if line.startswith("### ") or line.startswith("## "):
            sec = line.split()[1]
        if not line.startswith("| `") or not (sec == "2" or _re.match(r"3\.[1-6]", sec or "")):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) < 5:
            continue
        name = cells[0].strip("`").split("（")[0].split("(")[0].strip()
        run = cells[3].strip()
        m = _re.match(r"^A(?:\+M)?/(\d+)s$", run)
        if m:
            out[name] = ("auto", int(m.group(1)))
        elif run.startswith("A"):
            out[name] = ("auto", 20)
        m = _re.match(r"^R=(\d+)s$", run)
        if m:
            out[name] = ("raw", int(m.group(1)))
        elif run.startswith("R"):
            out[name] = ("raw", 20)
    return out


def resolve_dir(name):
    for cand in (f"examples/{name}", f"examples/engine/{name}", f"examples/video/{name}"):
        if os.path.isfile(os.path.join(ROOT, cand, "main.go")):
            return cand
    return None


def baseline_secs(win, variant, backend):
    """基线 stdout 的 'running Ns (RUN_SECONDS)' → N；无则 None。"""
    p = os.path.join(BASEWIN, win, variant, backend, "stdout.txt")
    if os.path.isfile(p):
        m = re.search(r"running (\d+)s \(RUN_SECONDS\)",
                      open(p, encoding="utf-8", errors="replace").read())
        if m:
            return int(m.group(1))
    return None


def run_round(name, ddir, variant, backend, args, secs, env_extra, timeout, outdir):
    rec = {"window": name, "dir": ddir, "variant": variant, "backend": backend,
           "args": args, "run_seconds": secs, "date": time.strftime("%F %T"),
           "json": None}
    os.makedirs(outdir, exist_ok=True)
    env = dict(os.environ)
    env.update({"DISPLAY": XDISPLAY,
                "WGPU_NATIVE_PATH": os.path.join(ROOT, "lib/libwgpu_native.so")})
    env["LD_LIBRARY_PATH"] = os.path.join(ROOT, "lib") + ":" + env.get("LD_LIBRARY_PATH", "")
    # P1.4 基线是 X11/Xwayland（record: backend=x11）：掐掉启动合成器泄漏的
    # WAYLAND_DISPLAY 强制 X11；Wayland 专窗显式给。
    if "wayland" not in name:
        env.pop("WAYLAND_DISPLAY", None)
    else:
        env["WAYLAND_DISPLAY"] = "gpui-p26"
    if backend == "go":
        env["GPUI_BACKEND"] = "go"
    if secs:
        env["RUN_SECONDS"] = str(secs)
    env.update(env_extra)
    binname = os.path.basename(ddir)
    try:
        p = subprocess.run([os.path.join(BIN, binname)] + list(args), cwd=ROOT, env=env,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           timeout=timeout, text=True)
        rec["rc"] = p.returncode
        open(os.path.join(outdir, "stdout.txt"), "w").write(p.stdout)
        open(os.path.join(outdir, "stderr.log"), "w").write(p.stderr[-20000:])
        rec["json"] = None
        try:
            start = p.stdout.index("{")
            depth = 0
            for i in range(start, len(p.stdout)):
                if p.stdout[i] == "{":
                    depth += 1
                elif p.stdout[i] == "}":
                    depth -= 1
                    if depth == 0:
                        rec["json"] = json.loads(p.stdout[start:i + 1])
                        break
        except Exception:
            pass
        rec["status"] = ("PASS" if rec["rc"] == 0 else "FAIL") if rec["rc"] != 124 else "TIMEOUT"
    except subprocess.TimeoutExpired as e:
        rec["rc"] = 124
        rec["status"] = "TIMEOUT"
        # TimeoutExpired 的 stdout/stderr 在部分 Python 版本里是 bytes（text=True 不覆盖）
        out = e.stdout or b""
        err = e.stderr or b""
        if isinstance(out, bytes):
            out = out.decode("utf-8", errors="replace")
        if isinstance(err, bytes):
            err = err.decode("utf-8", errors="replace")
        open(os.path.join(outdir, "stdout.txt"), "w").write(out[-20000:])
        open(os.path.join(outdir, "stderr.log"), "w").write(err[-20000:])
    with open(os.path.join(outdir, "record.json"), "w") as f:
        json.dump(rec, f, ensure_ascii=False, indent=1)
    return rec


def tsv_row(rec):
    j = rec.get("json") or {}
    notes = rec.get("note", "")
    if j.get("vsync_source") == "true":
        notes = (notes + "; " if notes else "") + "vsync_source=true (not a source name)"
    snaps = j.get("snapshots")
    goldens = j.get("goldens")
    return "\t".join(str(x) for x in [
        rec["window"], rec["variant"], rec["backend"], rec.get("run_kind", "run"),
        rec["status"], rec["rc"], j.get("tolerance", ""),
        (str(j.get("golden_diff_pct")) if j.get("golden_diff_pct") is not None else
         (str(j.get("golden_diff")) if j.get("golden_diff") is not None else "")),
        j.get("golden_total_px", ""), j.get("golden_first_run", ""),
        j.get("interval_p95_ms", j.get("p95_ms", "")),
        j.get("interval_p99_ms", j.get("p99_ms", "")),
        j.get("fps_interval", ""), j.get("fps_wall", j.get("fps", "")),
        j.get("vsync_source", ""), j.get("gpu_backend", ""),
        j.get("cpu_fallback_ops", ""),
        len(snaps) if isinstance(snaps, list) else "",
        len(goldens) if isinstance(goldens, list) else "",
        notes])


def x_alive():
    return subprocess.run(["xdpyinfo", "-display", XDISPLAY],
                          stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0


def main():
    global MD_SECS
    MD_SECS = md_secs()
    only = sys.argv[1] if len(sys.argv) > 1 else ""
    # 轮次清单：基线 summary.tsv 的 (window, variant) 对，保基线文件序（批量殿后）。
    pairs, seen = [], set()
    for line in open(os.path.join(BASEWIN, "summary.tsv"),
                     encoding="utf-8").read().splitlines()[1:]:
        c = line.split("\t")
        if len(c) < 3:
            continue
        key = (c[0], c[1])
        if key not in seen:
            seen.add(key)
            pairs.append(key)
    if only:
        pairs = [p for p in pairs if only in p[0]]
    print(f"round pairs: {len(pairs)}")

    # 预构建（按目录 basename 落 /tmp/p26-bin）
    os.makedirs(BIN, exist_ok=True)
    build_fail = {}
    for win, _v in pairs:
        ddir = resolve_dir(win)
        if not ddir:
            build_fail[win] = "dir not found"
            continue
        binname = os.path.basename(ddir)
        if not os.path.isfile(os.path.join(BIN, binname)):
            r = subprocess.run(["go", "build", "-o", os.path.join(BIN, binname), "./" + ddir],
                               cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               text=True)
            if r.returncode != 0:
                build_fail[win] = r.stderr[-200:]
                print(f"BUILD_FAIL {win}: {r.stderr[-100:]}")
    print("build done")

    os.makedirs(OUT, exist_ok=True)
    summary = open(os.path.join(OUT, "summary.tsv"), "a")
    if os.path.getsize(os.path.join(OUT, "summary.tsv")) == 0:
        summary.write("\t".join(COLS) + "\n")
    for win, variant in pairs:
        ddir = resolve_dir(win)
        for be in ("native", "go"):
            outdir = os.path.join(OUT, win, variant, be)
            if os.path.isfile(os.path.join(outdir, "record.json")):
                print(f"skip(done) {win}/{variant}/{be}")
                continue
            if win in PLATFORM_SKIP:
                rec = {"window": win, "variant": variant, "backend": be, "status": "SKIP",
                       "rc": "", "note": PLATFORM_SKIP[win]}
                summary.write(tsv_row(rec) + "\n"); summary.flush()
                print(f"SKIP      {win}/{variant}/{be}")
                continue
            if win in build_fail or not ddir:
                rec = {"window": win, "variant": variant, "backend": be,
                       "status": "BUILD_FAIL", "rc": "",
                       "note": build_fail.get(win, "dir not found")}
                summary.write(tsv_row(rec) + "\n"); summary.flush()
                print(f"BUILD_FAIL {win}/{variant}/{be}")
                continue
            # 基线 args 与 RUN_SECONDS
            bp = os.path.join(BASEWIN, win, variant, be, "record.json")
            args, run_kind = [], "run"
            if os.path.isfile(bp):
                d = json.load(open(bp, encoding="utf-8"))
                args = d.get("args") or []
                run_kind = d.get("run_kind", "run")
            ov = ROUND_OVERRIDES.get((win, variant), {})
            secs = ov.get("secs") or baseline_secs(win, variant, be)
            if secs is None:
                secs = MD_SECS.get(win, (None, 20))[1]
            env_extra = dict(ov.get("env") or {})
            if win in SNAP_WINDOWS:
                src = os.path.join(BASEWIN, win, "default/native/snapshots")
                dst = os.path.join(OUT, win, "snapshots")
                os.makedirs(dst, exist_ok=True)
                for f in (os.listdir(src) if os.path.isdir(src) else []):
                    if f.endswith("_base.png") and not os.path.isfile(os.path.join(dst, f)):
                        shutil.copy2(os.path.join(src, f), os.path.join(dst, f))
                env_extra[SNAP_WINDOWS[win]] = dst
            if not x_alive():
                print("ABORT: :3 display dead"); summary.close(); sys.exit(1)
            timeout = max(90, secs * 4 + 90)
            rec = run_round(win, ddir, variant, be, args, secs, env_extra, timeout, outdir)
            rec["run_kind"] = run_kind
            j = rec.get("json") or {}
            fps = j.get("fps_wall") or 0
            # 瞬时环境态吸震：rc!=0 或 fps<15 重试一次（retry 记录在案，不粉饰）。
            if rec["rc"] != 0 or (fps and fps < 15):
                r2 = run_round(win, ddir, variant, be, args, secs, env_extra, timeout, outdir)
                rec["first_try"] = {"status": rec["status"], "rc": rec["rc"], "fps_wall": fps}
                rec["note"] = "retry(" + rec["status"] + "->" + r2["status"] + ")"
                rec["status"], rec["rc"], rec["json"] = r2["status"], r2["rc"], r2["json"]
            summary.write(tsv_row(rec) + "\n"); summary.flush()
            print(f"{rec['status']:9} {win}/{variant}/{be} rc={rec['rc']} {rec.get('note','')}")
    summary.close()
    print("REPLAY DONE")


if __name__ == "__main__":
    main()
