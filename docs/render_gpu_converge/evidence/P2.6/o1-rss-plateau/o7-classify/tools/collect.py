#!/usr/bin/env python3
"""O7 classification: run affected windows on isolated :3, capture current/base
pairs, and run the P2.6 golden analyzer on each pair.

No reseed: this only classifies baseline-expired vs render-hole.
"""

import json
import os
import shutil
import subprocess
import sys

REPO = os.getcwd()
OUT = os.path.join(REPO, "docs/render_gpu_converge/evidence/P2.6/o1-rss-plateau/o7-classify")
ANALYZER = os.path.join(REPO, "docs/render_gpu_converge/evidence/P2.6/golden-drift/tools/analyze_golden.py")
ENV = dict(os.environ)
ENV.update({
    "DISPLAY": ":3",
    "LD_LIBRARY_PATH": os.path.join(REPO, "lib"),
    "WGPU_NATIVE_PATH": os.path.join(REPO, "lib/libwgpu_native.so"),
})
ENV.pop("WAYLAND_DISPLAY", None)

# (window, binary, secs, args, [(current, base, tag), ...])
WINDOWS = [
    ("ui_base_idle", "/tmp/o5/bin/ui_base_idle", 60, ["-auto-only"], [
        ("examples/ui_base_idle/testdata/showcase_idle.png",
         "examples/ui_base_idle/testdata/showcase_idle_base.png", "showcase"),
    ]),
    ("ui_wr_c4_shell_overlay", "/tmp/o5/bin/ui_wr_c4_shell_overlay", 25, ["-auto-only"], [
        ("examples/ui_wr_c4_shell_overlay/golden/c4_steady.png",
         "examples/ui_wr_c4_shell_overlay/golden/c4_steady_base.png", "steady"),
        ("examples/ui_wr_c4_shell_overlay/golden/c4_recover.png",
         "examples/ui_wr_c4_shell_overlay/golden/c4_recover_base.png", "recover"),
    ]),
    ("ui_wr_c8_hit_overlay_xf", "/tmp/o5/bin/ui_wr_c8_hit_overlay_xf", 15, ["-auto-only"], [
        ("examples/ui_wr_c8_hit_overlay_xf/testdata/c8_recover.png",
         "examples/ui_wr_c8_hit_overlay_xf/testdata/c8_recover_base.png", "recover"),
    ]),
    ("ui_wr_r4_composite", "/tmp/o5/bin/ui_wr_r4_composite", 15, ["-auto-only"], [
        ("examples/ui_wr_r4_composite/testdata/showcase_r4_composite.png",
         "examples/ui_wr_r4_composite/testdata/showcase_r4_composite_base.png", "showcase"),
    ]),
    ("ui_wr_r10_async_image", "/tmp/o6/bin/ui_wr_r10_async_image", 30, ["-auto-only"], [
        ("examples/ui_wr_r10_async_image/testdata/showcase_r10_async_image.png",
         "examples/ui_wr_r10_async_image/testdata/showcase_r10_async_image_base.png", "showcase"),
    ]),
    ("ui_wr_p33_richtext", "/tmp/o6/bin/ui_wr_p33_richtext", 5, ["-auto-only"], [
        ("examples/ui_wr_p33_richtext/testdata/p33_final.png",
         "examples/ui_wr_p33_richtext/testdata/goldens/p33_final.png", "final"),
    ]),
    ("ui_wr_c2_retained_scene", "/tmp/o6/bin/ui_wr_c2_retained_scene", 15, ["-auto-only"], [
        ("examples/ui_wr_c2_retained_scene/testdata/showcase_c2_retained_scene.png",
         "examples/ui_wr_c2_retained_scene/testdata/showcase_c2_retained_scene_base.png", "showcase"),
    ]),
    ("ui_wr_p37_scene", "/tmp/o6/bin/ui_wr_p37_scene", 8, ["-auto-only"], [
        ("examples/ui_wr_p37_scene/testdata/p37_scene_final.png",
         "examples/ui_wr_p37_scene/testdata/goldens/p37_scene_final_native.png", "final"),
    ]),
]

# multiwin: 8 per-window pairs, one run.
MULTIWIN = ("ui_base_multiwin_cap", "/tmp/o6d/bin/ui_base_multiwin_cap", 75, ["-auto-only"],
            [(f"examples/ui_base_multiwin_cap/testdata/multiwin_win{i}.png",
              f"examples/ui_base_multiwin_cap/testdata/multiwin_win{i}_base.png", f"win{i}") for i in range(1, 9)])


def run_one(window, binary, secs, args, pairs):
    wdir = os.path.join(OUT, "runs", window)
    os.makedirs(wdir, exist_ok=True)
    env = dict(ENV)
    env["RUN_SECONDS"] = str(secs)
    with open(os.path.join(wdir, "stdout.txt"), "w") as so, open(os.path.join(wdir, "stderr.txt"), "w") as se:
        p = subprocess.run([binary] + args, stdout=so, stderr=se, env=env, timeout=secs + 90)
    with open(os.path.join(wdir, "rc"), "w") as f:
        f.write(str(p.returncode))
    rows = []
    for cur, base, tag in pairs:
        cdir = os.path.join(wdir, tag)
        os.makedirs(cdir, exist_ok=True)
        cur_src = os.path.join(REPO, cur)
        base_src = os.path.join(REPO, base)
        if not os.path.exists(cur_src) or not os.path.exists(base_src):
            rows.append({"window": window, "pair": tag, "error": f"missing current={os.path.exists(cur_src)} base={os.path.exists(base_src)}"})
            continue
        cur_dst = os.path.join(cdir, "current.png")
        base_dst = os.path.join(cdir, "base.png")
        shutil.copyfile(cur_src, cur_dst)
        shutil.copyfile(base_src, base_dst)
        out = subprocess.run([sys.executable, ANALYZER, "--current", cur_dst, "--base", base_dst,
                              "--out-dir", cdir, "--window", window, "--backend", "native"],
                             capture_output=True, text=True)
        try:
            row = json.loads(out.stdout.strip().splitlines()[-1])
        except Exception:
            row = {"window": window, "pair": tag, "error": out.stderr[-300:]}
        row["pair"] = tag
        with open(os.path.join(cdir, "analysis.json"), "w") as f:
            json.dump(row, f, ensure_ascii=False, indent=1)
        rows.append(row)
    return p.returncode, rows


def main():
    os.makedirs(OUT, exist_ok=True)
    all_rows = []
    print("window\tpair\trc\tfull_diff_pct\tmax_delta\tbest_shift\tinterior_ident%\tcontent_bbox")
    for w in WINDOWS:
        rc, rows = run_one(*w)
        for r in rows:
            if "error" in r:
                print(f"{w[0]}\t{r.get('pair','')}\t{rc}\tERROR {r['error']}")
                continue
            sh = r.get("shift_search", {})
            ins = r.get("interior_samples", {})
            print(f"{w[0]}\t{r['pair']}\t{rc}\t{r.get('full_diff_pct')}\t{r.get('max_channel_delta')}\t"
                  f"({sh.get('best_dx')},{sh.get('best_dy')},{sh.get('best_gain_pct')}%)\t"
                  f"{ins.get('identical_pct')}\t{r.get('diff_bbox')}")
            all_rows.append(r)
    rc, rows = run_one(*MULTIWIN)
    for r in rows:
        if "error" in r:
            print(f"{MULTIWIN[0]}\t{r.get('pair','')}\t{rc}\tERROR {r['error']}")
            continue
        sh = r.get("shift_search", {})
        ins = r.get("interior_samples", {})
        print(f"{MULTIWIN[0]}\t{r['pair']}\t{rc}\t{r.get('full_diff_pct')}\t{r.get('max_channel_delta')}\t"
              f"({sh.get('best_dx')},{sh.get('best_dy')},{sh.get('best_gain_pct')}%)\t"
              f"{ins.get('identical_pct')}\t{r.get('diff_bbox')}")
        all_rows.append(r)
    with open(os.path.join(OUT, "analysis.jsonl"), "w") as f:
        for r in all_rows:
            f.write(json.dumps(r, ensure_ascii=False) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
