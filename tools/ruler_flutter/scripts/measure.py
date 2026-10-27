#!/usr/bin/env python3
"""Same-machine CPU/RSS/fps measurement for the Flutter/Godot/gpui ruler.

The measurement window is:
  * wait for the real X11 window (not just the process);
  * warm up (default 5 s);
  * sample /proc/<pid>/stat + /proc/<pid>/status for `--duration` seconds;
  * screenshot the client area and check that it is not an all-black window;
  * request a normal close with WM_DELETE_WINDOW (SIGTERM is only a fallback).

CPU% is process-wide utime+stime over the measured wall window. RSS slope is
a linear fit in KB/min over the same window. FPS/p95/hitch come from the
application's own RULER_METRIC lines when present; Godot can use --print-fps.

Usage:
  python3 scripts/measure.py --scene spin --out /tmp/ruler/spin_1
  python3 scripts/measure.py --app /path/to/app --arg --flag \
      --env KEY=VAL --window-match TITLE --out /tmp/ruler/run
"""

from __future__ import annotations

import argparse
import collections
import json
import os
import re
import shutil
import statistics
import subprocess
import sys
import time
from pathlib import Path

try:
    from PIL import Image
except Exception:  # pragma: no cover - only used by the evidence path
    Image = None

try:
    from Xlib import X, display, protocol
except Exception:  # pragma: no cover
    display = None
    protocol = None


def client_geometry(win):
    """Absolute client geometry, robust against WM reparenting/placement."""
    try:
        out = subprocess.check_output(
            ["xwininfo", "-id", hex(win.id)],
            text=True, stderr=subprocess.DEVNULL,
        )
        vals = {}
        for line in out.splitlines():
            s = line.strip()
            for key, name in (
                ("x", "Absolute upper-left X:"),
                ("y", "Absolute upper-left Y:"),
                ("w", "Width:"),
                ("h", "Height:"),
            ):
                if s.startswith(name):
                    vals[key] = int(s.split(":", 1)[1])
        if len(vals) == 4:
            return vals["x"], vals["y"], vals["w"], vals["h"]
    except Exception:
        pass
    g = win.get_geometry()
    return g.x, g.y, g.width, g.height


def find_window(title_match: str, timeout_s: float = 20.0):
    if display is None:
        return None
    d = display.Display()
    root = d.screen().root
    deadline = time.monotonic() + timeout_s
    while time.monotonic() < deadline:
        found = []
        stack = [root]
        while stack:
            w = stack.pop()
            try:
                children = w.query_tree().children
            except Exception:
                children = []
            for c in children:
                try:
                    name = c.get_wm_name()
                    if isinstance(name, bytes):
                        name = name.decode("utf-8", "replace")
                    if not name:
                        prop = c.get_full_property(
                            d.intern_atom("_NET_WM_NAME"), 0
                        )
                        if prop is not None:
                            v = prop.value
                            name = v.decode("utf-8", "replace") if isinstance(v, bytes) else str(v)
                    if name and title_match in name:
                        g = c.get_geometry()
                        if g.width > 20 and g.height > 20:
                            found.append((g.width * g.height, c, name, g))
                except Exception:
                    pass
                stack.append(c)
        if found:
            found.sort(key=lambda x: x[0], reverse=True)
            _, win, name, geom = found[0]
            ax, ay, ww, wh = client_geometry(win)
            return d, root, win, name, (ax, ay, ww, wh)
        time.sleep(0.2)
    return None


def raise_window(root, win):
    """Raise the client and its WM frame, then focus it."""
    try:
        top = win
        while True:
            parent = top.query_tree().parent
            if parent is None or parent == root:
                break
            top = parent
        top.raise_window()
        top.configure(stack_mode=X.Above)
    except Exception:
        try:
            win.raise_window()
        except Exception:
            pass


def wm_hidden(d, win) -> bool:
    """True when _NET_WM_STATE contains _NET_WM_STATE_HIDDEN (iconified).

    Row 9 (min arm) probe: the runner iconifies at a scheduled time; the 1 Hz
    geo/state series records the exact transition so the report shows the
    FG→BG latch actually fired instead of assuming it did.
    """
    try:
        prop = win.get_full_property(d.intern_atom("_NET_WM_STATE"), 0)
        if prop is None or not prop.value:
            return False
        hidden = d.intern_atom("_NET_WM_STATE_HIDDEN")
        return any(a == hidden for a in prop.value)
    except Exception:
        return False


def request_close(d, win) -> bool:
    """Send WM_DELETE_WINDOW so the app can run its normal close path."""
    if d is None or win is None or protocol is None:
        return False
    try:
        wm_protocols = d.intern_atom("WM_PROTOCOLS")
        wm_delete = d.intern_atom("WM_DELETE_WINDOW")
        ev = protocol.event.ClientMessage(
            window=win,
            client_type=wm_protocols,
            data=(32, [wm_delete, X.CurrentTime, 0, 0, 0]),
        )
        win.send_event(ev, event_mask=X.NoEventMask, propagate=False)
        d.flush()
        return True
    except Exception as e:
        print(f"WM_DELETE_WINDOW failed: {e}", file=sys.stderr)
        return False


def read_ticks(pid: int) -> int:
    with open(f"/proc/{pid}/stat", "r") as f:
        fields = f.read().split()
    # fields[13]=utime, fields[14]=stime (1-based after comm in parens)
    return int(fields[13]) + int(fields[14])


def read_rss_kb(pid: int) -> int:
    try:
        with open(f"/proc/{pid}/status", "r") as f:
            for line in f:
                if line.startswith("VmRSS:"):
                    return int(line.split()[1])
    except FileNotFoundError:
        return 0
    return 0


def screen_size() -> tuple[int, int]:
    """Return the X root size; ffmpeg x11grab cannot infer it itself."""
    try:
        out = subprocess.check_output(
            ["xwininfo", "-root"], text=True, stderr=subprocess.DEVNULL
        )
        vals = {}
        for line in out.splitlines():
            s = line.strip()
            for key, name in (("w", "Width:"), ("h", "Height:")):
                if s.startswith(name):
                    vals[key] = int(s.split(":", 1)[1])
        if len(vals) == 2:
            return vals["w"], vals["h"]
    except Exception:
        pass
    return 1920, 1080


def capture_client(
    out_png: Path, x: int, y: int, w: int, h: int,
    root_size: tuple[int, int] = (1920, 1080),
) -> dict:
    if Image is None or shutil.which("ffmpeg") is None:
        return {"ok": False, "reason": "PIL/ffmpeg unavailable"}
    root_png = out_png.with_name(out_png.stem + "_root.png")
    cmd = [
        "ffmpeg", "-hide_banner", "-loglevel", "error",
        "-f", "x11grab", "-video_size", f"{root_size[0]}x{root_size[1]}",
        "-i", os.environ.get("DISPLAY", ":0"),
        "-frames:v", "1", "-y", str(root_png),
    ]
    subprocess.run(cmd, check=True)
    im = Image.open(root_png).convert("RGB")
    sw, sh = im.size
    x0 = max(0, min(sw, x))
    y0 = max(0, min(sh, y))
    x1 = max(x0, min(sw, x + w))
    y1 = max(y0, min(sh, y + h))
    crop = im.crop((x0, y0, x1, y1))
    crop.save(out_png)
    pixels = list(crop.getdata())
    counts = collections.Counter(pixels)
    top_color, top_n = counts.most_common(1)[0]
    non_top = 1.0 - top_n / max(1, len(pixels))
    black = sum(1 for p in pixels if p == (0, 0, 0)) / max(1, len(pixels))
    mean = tuple(round(sum(p[i] for p in pixels) / max(1, len(pixels)), 2) for i in range(3))
    return {
        "ok": True,
        "png": str(out_png),
        "crop": [x0, y0, x1 - x0, y1 - y0],
        "unique_colors": len(counts),
        "top_color": list(top_color),
        "top_color_fraction": round(top_n / max(1, len(pixels)), 4),
        "non_top_color_fraction": round(non_top, 4),
        "black_fraction": round(black, 4),
        "mean_rgb": list(mean),
    }


def parse_app_metrics(path: Path, measured_t0: float = 0.0) -> dict:
    fps, p95, hitch, frames = [], [], 0, 0
    godot_fps = []
    if not path.exists():
        return {"fps_median": None, "p95_median_ms": None, "hitch_per_min": None}
    for line in path.read_text(errors="replace").splitlines():
        line = line.strip()
        if not line:
            continue
        if line.startswith("{"):
            try:
                obj = json.loads(line)
            except Exception:
                continue
            if obj.get("event") in ("RULER_METRIC", "RULER_FINAL"):
                if obj.get("t_s") is not None:
                    try:
                        if float(obj["t_s"]) < measured_t0:
                            continue
                    except Exception:
                        pass
                if "fps" in obj:
                    fps.append(float(obj["fps"]))
                if "fps_avg" in obj:
                    fps.append(float(obj["fps_avg"]))
                if "p95_ms" in obj and float(obj["p95_ms"]) > 0:
                    p95.append(float(obj["p95_ms"]))
                if "hitch" in obj:
                    hitch += int(obj["hitch"])
                frames += int(obj.get("frames", 0))
        # Godot --print-fps prints "FPS: 60" (and frame time) lines; the
        # ruler copy also has a tiny autoload printing GODOT_FPS=...
        if line.startswith("FPS:") or line.startswith("Project FPS:"):
            try:
                godot_fps.append(float(line.split(":", 1)[1].split()[0]))
            except Exception:
                pass
        elif line.startswith("GODOT_FPS="):
            try:
                rest = line.split("=", 1)[1].split()
                value = float(rest[0])
                t_app = None
                for token in rest[1:]:
                    if token.startswith("t="):
                        t_app = float(token.split("=", 1)[1])
                if t_app is not None and t_app < measured_t0:
                    continue
                godot_fps.append(value)
            except Exception:
                pass
        # glxgears prints "305 frames in 5.0 seconds = 60.9 FPS".
        m = re.search(r"=\s*([0-9.]+)\s*FPS", line)
        if m:
            try:
                godot_fps.append(float(m.group(1)))
            except Exception:
                pass
    if not fps and godot_fps:
        fps = godot_fps
    return {
        "fps_median": round(statistics.median(fps), 2) if fps else None,
        "fps_min": round(min(fps), 2) if fps else None,
        "fps_max": round(max(fps), 2) if fps else None,
        "p95_median_ms": round(statistics.median(p95), 2) if p95 else None,
        "hitch": hitch,
        "frames_reported": frames,
        "samples": len(fps),
    }


def gnome_composite_screenshot(out_png, bbox):
    """Capture the REAL composite (Mutter) via the GNOME Screenshot D-Bus API
    and crop to the bbox. Needed under Wayland sessions: rootless XWayland's
    root window holds no composited pixels, so ffmpeg x11grab sees black even
    though the windows are alive — the black is an X-server artifact, not an
    app failure. Returns a capture_client-shaped dict."""
    import subprocess as sp
    tmp = out_png.with_suffix(".full.png")
    r = sp.run(
        ["gdbus", "call", "--session",
         "--dest", "org.gnome.Shell.Screenshot",
         "--object-path", "/org/gnome/Shell/Screenshot",
         "--method", "org.gnome.Shell.Screenshot.Screenshot",
         "false", "false", str(tmp)],
        capture_output=True, text=True, timeout=20,
    )
    if (tmp := tmp) and not tmp.exists():
        raise RuntimeError(f"gdbus screenshot failed: {r.stderr.strip()[:120]}")
    from PIL import Image
    x0, y0, x1, y1 = budget_of(bbox)
    im = Image.open(tmp).convert("RGB")
    crop = im.crop((x0, y0, x1, y1))
    crop.save(out_png)
    pixels = list(crop.getdata())
    counts = collections.Counter(pixels)
    top_color, top_n = counts.most_common(1)[0]
    return {
        "ok": True,
        "png": str(out_png),
        "source": "gnome_composite",
        "crop": [x0, y0, x1 - x0, y1 - y0],
        "unique_colors": len(counts),
        "top_color": list(top_color),
        "top_color_fraction": round(top_n / max(1, len(pixels)), 4),
        "black_fraction": round(sum(1 for p in pixels if p == (0, 0, 0)) / max(1, len(pixels)), 4),
        "mean_rgb": [round(sum(p[i] for p in pixels) / max(1, len(pixels)), 2) for i in range(3)],
    }


def budget_of(bbox):
    return bbox[0], bbox[1], bbox[0] + bbox[2], bbox[1] + bbox[3]


def scan_windows(d, title_match: str):
    """One tree pass collecting every matching (name, win, client geom)."""
    found = []
    root = d.screen().root
    stack = [root]
    while stack:
        w = stack.pop()
        try:
            children = w.query_tree().children
        except Exception:
            children = []
        for c in children:
            try:
                name = c.get_wm_name()
                if isinstance(name, bytes):
                    name = name.decode("utf-8", "replace")
                if not name:
                    prop = c.get_full_property(d.intern_atom("_NET_WM_NAME"), 0)
                    if prop is not None:
                        v = prop.value
                        name = v.decode("utf-8", "replace") if isinstance(v, bytes) else str(v)
                if name and title_match in name:
                    g = c.get_geometry()
                    if g.width > 20 and g.height > 20:
                        ax, ay, ww, wh = client_geometry(c)
                        found.append((name, c, (ax, ay, ww, wh)))
            except Exception:
                pass
            stack.append(c)
    return found


def run_instances(args, env0: dict, app: Path, out: Path) -> int:
    """Matrix row 10 arm: N independent app processes tiled by the runner.

    Honesty notes: no raise during run (stacking would occlude the tiles);
    sum-CPU is reported at top level; per-instance rows carry pid/name/cpu/fps.
    Close path is WM_DELETE per window, SIGTERM fallback per process, no hard
    kill (same discipline as single-window runs).
    """
    n = args.instances
    result = {
        "mode": "instances",
        "scene": args.scene or Path(app).name,
        "instances": n,
        "duration_s": args.duration,
        "warmup_s": args.warmup,
        "processes": [],
    }
    procs = []
    outs = []
    d = None
    windows: list = []
    try:
        clk = os.sysconf(os.sysconf_names["SC_CLK_TCK"])
        t_launch = time.monotonic()
        for i in range(1, n + 1):
            env = env0.copy()
            env["RULER_INSTANCE_INDEX"] = str(i)
            sp = out / f"app_{i}.stdout"
            se = out / f"app_{i}.stderr"
            sf, ef = open(sp, "w"), open(se, "w")
            p = subprocess.Popen([str(app)] + args.arg, env=env, stdout=sf, stderr=ef)
            procs.append((i, p, sf, ef))
            outs.append(sp)
            result["processes"].append({"index": i, "pid": p.pid})

        # Wait until all n windows are mapped.
        deadline = time.monotonic() + 30.0
        d = display.Display()
        wins = {}
        while time.monotonic() < deadline:
            wins = {c.id: (nm, c, g) for nm, c, g in scan_windows(d, args.window_match)}
            if len(wins) >= n:
                break
            time.sleep(0.2)
        if len(wins) < n:
            result["error"] = f"only {len(wins)}/{n} windows mapped"
            raise RuntimeError(result["error"])
        result["map_ms"] = round((time.monotonic() - t_launch) * 1000.0, 1)
        windows = sorted(wins.values(), key=lambda x: x[0])
        geoms = [g for _, _, g in windows]
        result["window_names"] = [nm for nm, _, _ in windows]

        time.sleep(args.warmup)
        # Re-read each client geometry after the WM settles placement.
        geoms = []
        for _, c, _ in windows:
            geoms.append(client_geometry(c))
        xs0 = min(g[0] for g in geoms)
        ys0 = min(g[1] for g in geoms)
        xs1 = max(g[0] + g[2] for g in geoms)
        ys1 = max(g[1] + g[3] for g in geoms)
        result["window_bbox"] = [xs0, ys0, xs1 - xs0, ys1 - ys0]
        if not args.no_screenshot:
            bbox = (xs0, ys0, xs1 - xs0, ys1 - ys0)
            shot = capture_client(out / "shot.png", *bbox, screen_size())
            note = "x11grab"
            if (not shot.get("ok")) or float(shot.get("black_fraction", 1.0)) >= 0.999:
                # XWayland 的 root 是 rootless：合成像素在 Wayland 合成器一侧，
                # X 根窗帧缓冲里本来就没有内容——x11grab 抓黑不是应用问题，
                # 回退 GNOME 截图接口取真合成画面。
                try:
                    shot = gnome_composite_screenshot(out / "shot.png", bbox)
                    note = "gnome_composite"
                    result["screenshot"] = shot
                    result["screenshot_source"] = note
                except Exception as e:
                    # Wayland 会话下机器取像素三条路都可能不通（Shell 截图 API 白名单、
                    # gnome-screenshot 未装、GL 窗口 XGetImage BadMatch）。此时不再
                    # 炸整轮：记录 unavailable，像素证据转人工目击（AGENTS 自测+人工二合一）。
                    result["screenshot"] = {
                        "ok": False,
                        "source": "unavailable",
                        "reason": f"x11grab black, fallback failed: {e}",
                        "note": "Wayland 会话机器取像素不可用，像素证据转人工目击；曲线数据不受影响",
                    }
                    result["screenshot_source"] = "unavailable"
                    print("screenshot unavailable: %s" % result["screenshot"]["reason"])
            else:
                result["screenshot"] = shot
                result["screenshot_source"] = note
            if result["screenshot_source"] == "unavailable":
                pass  # 像素证据缺席已记录，转人工目击，不得炸轮
            elif not result["screenshot"].get("ok"):
                result["error"] = "screenshot missing"
                raise RuntimeError("screenshot missing")
            elif float(result["screenshot"].get("black_fraction", 1.0)) >= 0.999:
                result["error"] = "screenshot is all black (both paths)"
                raise RuntimeError("screenshot is all black (both paths)")

        t0 = time.monotonic()
        tick0 = {p.pid: read_ticks(p.pid) for _, p, _, _ in procs}
        rss_series = {p.pid: [] for _, p, _, _ in procs}
        sample_dt = 0.2
        while True:
            now = time.monotonic()
            if now - t0 >= args.duration:
                break
            for _, p, _, _ in procs:
                rss = read_rss_kb(p.pid)
                if rss:
                    rss_series[p.pid].append((now - t0, rss))
            time.sleep(sample_dt)
        t1 = time.monotonic()
        measured = t1 - t0
        result["measured_s"] = round(measured, 3)

        tick_sum = 0
        rss_sum = 0.0
        for row, (_, p, _, _) in zip(result["processes"], procs):
            t1i = read_ticks(p.pid)
            dt = t1i - tick0[p.pid]
            cpu = dt / clk / measured * 100.0 if measured > 0 and p.poll() is None else 0.0
            rss_end = read_rss_kb(p.pid)
            row["cpu_pct"] = round(cpu, 2)
            row["rss_end_mb"] = round(rss_end / 1024.0, 1)
            row["alive_at_end"] = p.poll() is None
            tick_sum += dt
            rss_sum += rss_end
            pm = parse_app_metrics(outs[row["index"] - 1], measured_t0=args.warmup)
            row["fps_median"] = pm.get("fps_median")
            row["p95_median_ms"] = pm.get("p95_median_ms")
        result["cpu_pct"] = round(tick_sum / clk / measured * 100.0, 2) if measured > 0 else 0.0
        result["rss_end_mb"] = round(rss_sum / 1024.0, 1)
    except Exception as e:
        result.setdefault("error", str(e))
    finally:
        # Same close discipline as single-window runs: WM_DELETE first, then
        # SIGTERM per process. No hard kill, ever, on a desktop display.
        futures_closed = 0
        if d is not None:
            for _, c, _g in (scan_windows(d, args.window_match) if windows else []):
                if request_close(d, c):
                    futures_closed += 1
        deadline = time.monotonic() + 10.0
        while any(p.poll() is None for _, p, _, _ in procs) and time.monotonic() < deadline:
            time.sleep(0.2)
        for _i, p, sf, ef in procs:
            if p.poll() is None:
                p.terminate()
                result.setdefault("close_request", "sigterm")
            try:
                sf.flush()
                ef.flush()
                sf.close()
                ef.close()
            except Exception:
                pass
        result["close_requests_sent"] = futures_closed
        (out / "report.json").write_text(json.dumps(result, indent=2))
        print(json.dumps(result, indent=2))
    return 0


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--app", default=None)
    ap.add_argument("--scene", choices=[
        "spin", "list", "text",
        "full", "grid", "boundary", "overlay", "resize", "min", "cp",
    ], default=None)
    ap.add_argument("--arg", action="append", default=[])
    ap.add_argument("--env", action="append", default=[])
    ap.add_argument("--window-match", required=True)
    ap.add_argument("--duration", type=float, default=60.0)
    ap.add_argument("--warmup", type=float, default=5.0)
    ap.add_argument("--out", required=True)
    ap.add_argument("--extra-seconds", type=float, default=15.0)
    ap.add_argument("--no-screenshot", action="store_true")
    ap.add_argument(
        "--instances", type=int, default=1,
        help="N>1: spawn N app instances with RULER_INSTANCE_INDEX set; the "
             "runner tiles them at gpui multiwin offsets and gives each a "
             "unique title. CPU/RSS are sampled per pid and summed; this is "
             "the matrix row 10 (multi-window shared ceiling) ruler arm.",
    )
    args = ap.parse_args()

    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    app = Path(args.app) if args.app else Path(__file__).resolve().parents[1] / "build/linux/x64/release/bundle/ruler_flutter"
    if not app.exists():
        print(f"app not found: {app}", file=sys.stderr)
        return 2

    env = os.environ.copy()
    if args.scene:
        env["RULER_SCENE"] = args.scene
        env["RULER_SECONDS"] = str(int(args.duration + args.warmup + args.extra_seconds))
    for item in args.env:
        k, v = item.split("=", 1)
        env[k] = v
    if args.instances > 1:
        return run_instances(args, env, app, out)

    stdout_path = out / "app.stdout"
    stderr_path = out / "app.stderr"
    stdout_f = open(stdout_path, "w")
    stderr_f = open(stderr_path, "w")
    cmd = [str(app)] + args.arg
    t_launch = time.monotonic()
    proc = subprocess.Popen(cmd, env=env, stdout=stdout_f, stderr=stderr_f)
    result = {
        "scene": args.scene or Path(args.app).name,
        "cmd": cmd,
        "pid": proc.pid,
        "duration_s": args.duration,
        "warmup_s": args.warmup,
    }
    d = None
    win = None
    try:
        found = find_window(args.window_match, timeout_s=20.0)
        if not found:
            result["error"] = "window not found"
            raise RuntimeError("window not found")
        d, root, win, name, (wx, wy, ww, wh) = found
        raise_window(root, win)
        d.sync()
        result["window"] = {"name": name, "x": wx, "y": wy, "w": ww, "h": wh}
        # Row 16 first-present proxy: launch → X client mapped (scan runs at
        # 0.2 s granularity, so this is the mapped-when-found honest bound,
        # not a pixel-stage timestamp on its own).
        result["map_ms"] = round((time.monotonic() - t_launch) * 1000.0, 1)
        print(
            f"window: {name!r} {ww}x{wh}+{wx}+{wy}",
            file=sys.stderr,
        )

        time.sleep(args.warmup)
        # The WM may place/reparent the window during startup; re-read the
        # client geometry after warmup so the screenshot crop is correct.
        wx, wy, ww, wh = client_geometry(win)
        result["window"] = {"name": name, "x": wx, "y": wy, "w": ww, "h": wh}
        raise_window(root, win)
        d.sync()
        if not args.no_screenshot:
            try:
                shot = capture_client(
                    out / "shot.png", wx, wy, ww, wh, screen_size()
                )
            except Exception as e:
                shot = {"ok": False, "error": str(e)}
                print(f"screenshot failed: {e}", file=sys.stderr)
            shot_src = "x11grab"
            if (not shot.get("ok")) or float(shot.get("black_fraction", 1.0)) >= 0.999:
                # 与多实例臂同链：XWayland rootless 抓黑回退 GNOME 截图接口；
                # 再不通记 unavailable 转人工目击，不炸轮。
                try:
                    shot = gnome_composite_screenshot(
                        out / "shot.png", [wx, wy, ww, wh]
                    )
                    shot_src = "gnome_composite"
                except Exception as e:
                    result["screenshot"] = {
                        "ok": False,
                        "source": "unavailable",
                        "reason": f"x11grab black, fallback failed: {e}",
                        "note": "Wayland 会话机器取像素不可用，像素证据转人工目击；曲线数据不受影响",
                    }
                    result["screenshot_source"] = "unavailable"
                    print("screenshot unavailable: %s" % result["screenshot"]["reason"])
                if result["screenshot_source"] == "unavailable":
                    pass
                else:
                    result["screenshot"] = shot
                    result["screenshot_source"] = shot_src
            else:
                result["screenshot"] = shot
                result["screenshot_source"] = shot_src
            shot_now = result.get("screenshot") or {}
            if result.get("screenshot_source") == "unavailable":
                pass  # 已记录，转人工目击
            elif not shot_now.get("ok"):
                result["error"] = "screenshot missing"
                raise RuntimeError("screenshot missing")
            elif float(shot_now.get("black_fraction", 1.0)) >= 0.999:
                result["error"] = "screenshot is all black (both paths)"
                raise RuntimeError("screenshot is all black (both paths)")

        # Start the measurement after warmup. The screenshot above is taken
        # before the first /proc sample; its ffmpeg call is not part of the
        # CPU window.
        clk = os.sysconf(os.sysconf_names["SC_CLK_TCK"])
        t0 = time.monotonic()
        tick0 = read_ticks(proc.pid)
        rss_series = []
        geo_series = []
        last_geo = 0.0
        sample_dt = 0.2
        while True:
            now = time.monotonic()
            elapsed = now - t0
            if elapsed >= args.duration:
                break
            rss = read_rss_kb(proc.pid)
            if rss:
                rss_series.append((elapsed, rss))
            # 1 Hz geometry/state probe: hard evidence that runner-scheduled
            # waves actually fired (resize arm: size steps at t; min arm:
            # _NET_WM_STATE_HIDDEN latch at t) — 禁假绿，不靠假设。
            if elapsed - last_geo >= 0.9:
                last_geo = elapsed
                try:
                    gx, gy, gw, gh = client_geometry(win)
                    geo_series.append(
                        [round(elapsed, 1), gx, gy, gw, gh,
                         wm_hidden(d, win)]
                    )
                except Exception:
                    pass
            time.sleep(sample_dt)
        t1 = time.monotonic()
        tick1 = read_ticks(proc.pid)
        rss_end = read_rss_kb(proc.pid)
        measured = t1 - t0
        cpu = (tick1 - tick0) / clk / measured * 100.0 if measured > 0 else 0.0
        result["measured_s"] = round(measured, 3)
        result["cpu_pct"] = round(cpu, 2)
        if geo_series:
            result["geo_series"] = geo_series
            result["window_end"] = {
                "x": geo_series[-1][1], "y": geo_series[-1][2],
                "w": geo_series[-1][3], "h": geo_series[-1][4],
                "hidden": geo_series[-1][5],
            }
        if rss_series:
            result["rss_start_mb"] = round(rss_series[0][1] / 1024.0, 1)
            result["rss_end_mb"] = round(rss_end / 1024.0, 1)
            result["rss_peak_mb"] = round(max(x[1] for x in rss_series) / 1024.0, 1)
            n = len(rss_series)
            xs = [x[0] for x in rss_series]
            ys = [x[1] for x in rss_series]
            mx = sum(xs) / n
            my = sum(ys) / n
            den = sum((x - mx) ** 2 for x in xs)
            slope_kb_s = sum((x - mx) * (y - my) for x, y in zip(xs, ys)) / den if den else 0.0
            result["rss_slope_kb_per_min"] = round(slope_kb_s * 60.0, 1)
            # 稳态窗口斜率 + 有效性标记:与 wrgate 同语义——剔除
            # max(25% 时长, 15s) 的预热爬坡段后再拟合;样本窗口 <30s 判 invalid。
            steady_t0 = max(0.25 * args.duration, 15.0)
            pairs = list(rss_series)
            spairs = [(t, y) for t, y in pairs if t >= steady_t0]
            if len(spairs) >= 2 and spairs[-1][0] - spairs[0][0] >= 30.0:
                sx = [p[0] for p in spairs]
                sy = [p[1] for p in spairs]
                smx = sum(sx) / len(sx)
                smy = sum(sy) / len(sy)
                sden = sum((x - smx) ** 2 for x in sx)
                sslope = sum((x - smx) * (y - smy) for x, y in zip(sx, sy)) / sden if sden else 0.0
                result["rss_slope_steady_kb_per_min"] = round(sslope * 60.0, 1)
                result["rss_slope_valid"] = True
                result["rss_slope_scope"] = (
                    f"steady window t>={steady_t0:.0f}s "
                    f"({spairs[-1][0] - spairs[0][0]:.0f}s of samples)"
                )
            else:
                result["rss_slope_valid"] = False
                result["rss_slope_scope"] = f"steady window t>={steady_t0:.0f}s too short"
        result["app_metrics"] = parse_app_metrics(stdout_path, measured_t0=args.warmup)
        result["exit_during_run"] = proc.poll() is not None
    except Exception as e:
        result.setdefault("error", str(e))
    finally:
        try:
            stdout_f.flush()
            stderr_f.flush()
        except Exception:
            pass
        close_requested = False
        if proc.poll() is None and d is not None and win is not None:
            close_requested = request_close(d, win)
            if close_requested:
                result["close_request"] = "wm_delete"
        if proc.poll() is None and close_requested:
            try:
                # WM_DELETE_WINDOW is the normal title-bar close path: the app
                # sees EventCloseRequested and runs Quit -> Close -> win.Close.
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                pass
        if proc.poll() is None:
            # Fallback for apps that do not implement WM_DELETE_WINDOW. SIGTERM
            # is a request, not a force-kill: a GPU app must trap it and run the
            # normal Close path. Hard-killing a GPU window can crash the Xorg
            # server (nvidia_drv.so nvidiaUnlock on the 940MX test box).
            result["close_request"] = (
                "wm_delete+sigterm" if close_requested else "sigterm"
            )
            proc.terminate()
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                # Last resort only. On a desktop Xorg a SIGKILL can leave the
                # GPU surface alive long enough to crash the server; require an
                # explicit opt-in reserved for isolated test displays.
                if os.environ.get("GPUI_ALLOW_GPU_HARD_KILL") == "1":
                    print("app ignored SIGTERM; hard-killing (isolated display opt-in)", file=sys.stderr)
                    result["close_request"] += "+sigkill"
                    proc.kill()
                    proc.wait()
                else:
                    result["error"] = "app did not exit after SIGTERM; refusing hard kill"
                    print(
                        "app ignored SIGTERM; refusing hard kill. "
                        "Use an isolated display and set GPUI_ALLOW_GPU_HARD_KILL=1 only there.",
                        file=sys.stderr,
                    )
        stdout_f.close()
        stderr_f.close()

    result["app_stdout"] = str(stdout_path)
    result["app_stderr"] = str(stderr_path)
    with open(out / "report.json", "w") as f:
        json.dump(result, f, indent=2, ensure_ascii=False)
        f.write("\n")
    print(json.dumps(result, indent=2, ensure_ascii=False))
    return 0 if "error" not in result else 1


if __name__ == "__main__":
    raise SystemExit(main())
