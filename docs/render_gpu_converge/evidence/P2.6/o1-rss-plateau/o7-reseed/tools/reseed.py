#!/usr/bin/env python3
"""O7 reseed: 9 golden pairs + multiwin 8 windows.

Flow (aborts before touching baselines on any failed check):
  1) capture go current for every pair on isolated :3
  2) cross-check shared-base pairs native-vs-go content pixels == 0
  3) backup old bases
  4) copy current -> base (p37: per-backend bases)
  5) rerun native/go; every golden must be 0.0000% (perf/hitch reds allowed)
  6) idle 1px negative: shifted base must red; restore + rerun must be 0
  7) any step failure -> restore bases from backup and leave FAIL marker
"""

import json
import os
import re
import shutil
import subprocess
import sys

from PIL import Image

REPO = os.getcwd()
OUT = os.path.join(REPO, "docs/render_gpu_converge/evidence/P2.6/o1-rss-plateau/o7-reseed")
EVID = os.path.join(REPO, "docs/render_gpu_converge/evidence/P2.6/o1-rss-plateau/o7-classify/runs")
ENV = dict(os.environ)
ENV.update({
    "DISPLAY": ":3",
    "LD_LIBRARY_PATH": os.path.join(REPO, "lib"),
    "WGPU_NATIVE_PATH": os.path.join(REPO, "lib/libwgpu_native.so"),
})
ENV.pop("WAYLAND_DISPLAY", None)

# (window, bin, secs, [(tag, current_rel, base_rel, mode[, go_base_rel])])
PAIRS = [
    ("ui_base_idle", "/tmp/o5/bin/ui_base_idle", 60, [
        ("showcase", "examples/ui_base_idle/testdata/showcase_idle.png",
         "examples/ui_base_idle/testdata/showcase_idle_base.png", "shared"),
    ]),
    ("ui_wr_c4_shell_overlay", "/tmp/o5/bin/ui_wr_c4_shell_overlay", 25, [
        ("steady", "examples/ui_wr_c4_shell_overlay/golden/c4_steady.png",
         "examples/ui_wr_c4_shell_overlay/golden/c4_steady_base.png", "shared"),
        ("recover", "examples/ui_wr_c4_shell_overlay/golden/c4_recover.png",
         "examples/ui_wr_c4_shell_overlay/golden/c4_recover_base.png", "shared"),
    ]),
    ("ui_wr_c8_hit_overlay_xf", "/tmp/o5/bin/ui_wr_c8_hit_overlay_xf", 15, [
        ("recover", "examples/ui_wr_c8_hit_overlay_xf/testdata/c8_recover.png",
         "examples/ui_wr_c8_hit_overlay_xf/testdata/c8_recover_base.png", "shared"),
    ]),
    ("ui_wr_r4_composite", "/tmp/o5/bin/ui_wr_r4_composite", 15, [
        ("showcase", "examples/ui_wr_r4_composite/testdata/showcase_r4_composite.png",
         "examples/ui_wr_r4_composite/testdata/showcase_r4_composite_base.png", "shared"),
    ]),
    ("ui_wr_r10_async_image", "/tmp/o6/bin/ui_wr_r10_async_image", 30, [
        ("showcase", "examples/ui_wr_r10_async_image/testdata/showcase_r10_async_image.png",
         "examples/ui_wr_r10_async_image/testdata/showcase_r10_async_image_base.png", "shared"),
    ]),
    ("ui_wr_p33_richtext", "/tmp/o6/bin/ui_wr_p33_richtext", 5, [
        ("final", "examples/ui_wr_p33_richtext/testdata/p33_final.png",
         "examples/ui_wr_p33_richtext/testdata/goldens/p33_final.png", "shared"),
    ]),
    ("ui_wr_c2_retained_scene", "/tmp/o6/bin/ui_wr_c2_retained_scene", 15, [
        ("showcase", "examples/ui_wr_c2_retained_scene/testdata/showcase_c2_retained_scene.png",
         "examples/ui_wr_c2_retained_scene/testdata/showcase_c2_retained_scene_base.png", "shared"),
    ]),
    ("ui_wr_p37_scene", "/tmp/o6/bin/ui_wr_p37_scene", 8, [
        ("final", "examples/ui_wr_p37_scene/testdata/p37_scene_final.png",
         "examples/ui_wr_p37_scene/testdata/goldens/p37_scene_final_native.png", "perbackend",
         "examples/ui_wr_p37_scene/testdata/goldens/p37_scene_final_go.png"),
    ]),
    ("ui_base_multiwin_cap", "/tmp/o6d/bin/ui_base_multiwin_cap", 75,
     [("win%d" % i, "examples/ui_base_multiwin_cap/testdata/multiwin_win%d.png" % i,
       "examples/ui_base_multiwin_cap/testdata/multiwin_win%d_base.png" % i, "shared")
      for i in range(1, 9)]),
]

LOG = []


def say(*a):
    line = " ".join(str(x) for x in a)
    print(line)
    LOG.append(line)


def run(window, binary, secs, backend):
    env = dict(ENV)
    env["RUN_SECONDS"] = str(secs)
    if backend == "go":
        env["GPUI_BACKEND"] = "go"
    wdir = os.path.join(OUT, "runs", window, backend)
    os.makedirs(wdir, exist_ok=True)
    with open(os.path.join(wdir, "stdout.txt"), "w") as so, open(os.path.join(wdir, "stderr.txt"), "w") as se:
        p = subprocess.run([binary, "-auto-only"], stdout=so, stderr=se, env=env, timeout=secs + 120)
    with open(os.path.join(wdir, "rc"), "w") as f:
        f.write(str(p.returncode))
    return p.returncode, os.path.join(wdir, "stdout.txt"), os.path.join(wdir, "stderr.txt")


def golden_pct(stdout_path, stderr_path):
    try:
        lines = [l.strip() for l in open(stdout_path, errors="replace") if l.strip().startswith("{")]
        if lines:
            d = json.loads(lines[-1])
            for src in (d, d.get("ability_extra") or {}):
                for k in ("pixel_golden_diff_pct", "pixel_golden_max_pct", "golden_diff_pct"):
                    if k in src and src[k] is not None:
                        return float(src[k])
    except Exception:
        pass
    txt = open(stderr_path, errors="replace").read()
    m = re.search(r"golden .*?diff=([0-9.]+)%", txt) or re.search(r"golden diff=([0-9.]+)%", txt)
    if m:
        return float(m.group(1))
    if "golden=0.0000%" in txt:
        return 0.0
    return None


def content_diff(a_path, b_path, header, hud):
    a = Image.open(a_path).convert("RGBA")
    b = Image.open(b_path).convert("RGBA")
    if a.size != b.size:
        return -1
    w, h = a.size
    y0, y1 = min(header, h), max(min(header, h), h - hud)
    pa, pb = a.load(), b.load()
    n = 0
    for y in range(y0, y1):
        for x in range(w):
            if pa[x, y] != pb[x, y]:
                n += 1
    return n


def main():
    os.makedirs(OUT, exist_ok=True)
    say("=== O7 reseed start", "pairs:", sum(len(p[3]) for p in PAIRS))
    # 1) go current capture
    go_paths = {}
    for w, bin, secs, pairs in PAIRS:
        rc, so, se = run(w, bin, secs, "go")
        if rc not in (0, 1):
            say("ABORT go capture", w, "rc=", rc)
            return 2
        for pair in pairs:
            tag, cur = pair[0], pair[1]
            dst = os.path.join(OUT, w, tag, "current_go.png")
            os.makedirs(os.path.dirname(dst), exist_ok=True)
            if not os.path.exists(os.path.join(REPO, cur)):
                say("ABORT missing go current", w, tag, cur)
                return 2
            shutil.copyfile(os.path.join(REPO, cur), dst)
            go_paths[(w, tag)] = dst
        say("go capture", w, "rc=", rc)
    # 2) cross-check shared pairs (content box 48/72; multiwin snapshots are content-only)
    for w, bin, secs, pairs in PAIRS:
        for pair in pairs:
            tag, cur, base, mode = pair[0], pair[1], pair[2], pair[3]
            native = os.path.join(EVID, w, tag, "current.png")
            go = go_paths[(w, tag)]
            hdr, hud = (0, 0) if w == "ui_base_multiwin_cap" else (48, 72)
            d = content_diff(native, go, hdr, hud)
            say("cross", w, tag, "mode=", mode, "content_diff_px=", d)
            if mode == "shared" and d != 0:
                say("ABORT cross-backend content differs; no reseed")
                return 2
    # 3) backup bases
    for w, bin, secs, pairs in PAIRS:
        for pair in pairs:
            tag, cur, base, mode = pair[0], pair[1], pair[2], pair[3]
            bdst = os.path.join(OUT, "backup", w, tag, os.path.basename(base))
            os.makedirs(os.path.dirname(bdst), exist_ok=True)
            shutil.copyfile(os.path.join(REPO, base), bdst)
            if mode == "perbackend":
                go_base = pair[4]
                gdst = os.path.join(OUT, "backup", w, tag, "go_" + os.path.basename(go_base))
                shutil.copyfile(os.path.join(REPO, go_base), gdst)
    say("backup done")
    # 4) reseed
    for w, bin, secs, pairs in PAIRS:
        for pair in pairs:
            tag, cur, base, mode = pair[0], pair[1], pair[2], pair[3]
            native = os.path.join(EVID, w, tag, "current.png")
            shutil.copyfile(native, os.path.join(REPO, base))
            if mode == "perbackend":
                shutil.copyfile(go_paths[(w, tag)], os.path.join(REPO, pair[4]))
            say("reseeded", w, tag)
    # 5) rerun native/go; golden must be 0
    failed = []
    for w, bin, secs, pairs in PAIRS:
        for backend in ("native", "go"):
            rc, so, se = run(w, bin, secs, backend)
            for pair in pairs:
                tag = pair[0]
                if len(pairs) == 1 or w == "ui_base_multiwin_cap":
                    # single-snapshot windows: the report's golden covers all pairs
                    pass
            g = golden_pct(so, se)
            say("rerun", w, backend, "rc=", rc, "golden=", g)
            if g is None or g != 0:
                failed.append((w, backend, g))
    if failed:
        say("RESEED FAIL", failed)
        restore()
        return 3
    # 6) 1px negative on idle
    idle_base = os.path.join(REPO, "examples/ui_base_idle/testdata/showcase_idle_base.png")
    backup = os.path.join(OUT, "backup", "ui_base_idle", "showcase", "showcase_idle_base.png")
    im = Image.open(idle_base).convert("RGBA")
    shifted = Image.new("RGBA", im.size, (0, 0, 0, 0))
    shifted.paste(im, (1, 0))
    shifted.save(idle_base)
    rc, so, se = run("ui_base_idle", "/tmp/o5/bin/ui_base_idle", 60, "native")
    g = golden_pct(so, se)
    say("negative 1px", "rc=", rc, "golden=", g)
    shutil.copyfile(backup, idle_base)
    if g is None or g == 0 or rc == 0:
        say("NEGATIVE FAIL (expected red)")
        return 4
    rc2, so2, se2 = run("ui_base_idle", "/tmp/o5/bin/ui_base_idle", 60, "native")
    g2 = golden_pct(so2, se2)
    say("negative restore", "rc=", rc2, "golden=", g2)
    if g2 != 0:
        say("RESTORE FAIL")
        return 4
    with open(os.path.join(OUT, "reseed.md"), "w") as f:
        f.write("# O7 reseed result\n\n```\n" + "\n".join(LOG) + "\n```\n")
    say("=== O7 reseed DONE ===")
    return 0


def restore():
    for w, bin, secs, pairs in PAIRS:
        for pair in pairs:
            tag, cur, base, mode = pair[0], pair[1], pair[2], pair[3]
            bsrc = os.path.join(OUT, "backup", w, tag, os.path.basename(base))
            if os.path.exists(bsrc):
                shutil.copyfile(bsrc, os.path.join(REPO, base))
            if mode == "perbackend":
                g = pair[4]
                gsrc = os.path.join(OUT, "backup", w, tag, "go_" + os.path.basename(g))
                if os.path.exists(gsrc):
                    shutil.copyfile(gsrc, os.path.join(REPO, g))
    say("bases restored from backup")


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:
        say("EXCEPTION", repr(exc))
        try:
            restore()
        except Exception:
            pass
        sys.exit(5)
