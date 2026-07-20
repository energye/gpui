# device_lost_redraw

Device-lost soak sample: **RequestRedraw-driven ~60fps** on a **dedicated render goroutine**, with Skia/Flutter-style surface lifecycle.

Inspired by `mem_anim_window` S12-style composite motion (cards / orbs / bars / glow panels), but smaller and focused on **not dying** when the window is fully covered or the GPU device is lost.

## Run

```bash
cd /path/to/gpui
export LD_LIBRARY_PATH=$PWD/lib
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so   # optional if lib/ present
export DISPLAY=:1   # or your X11 display

go run ./examples/device_lost_redraw
```

Optional env:

| Env | Default | Meaning |
| --- | --- | --- |
| `GPUI_TARGET_FPS` | `60` | Animation `RequestRedraw` rate (15–120) |
| `GPUI_ANIM_SECONDS` | `0` | Auto-exit after N seconds of **visible** time; `0` = until close |
| `GPUI_FORCE_RENDER_WHEN_HIDDEN` | `0` | `1` = still acquire when unpresentable (debug only) |
| `GPUI_LOG_EVERY` | `60` | Log every N presented frames |

## Architecture

```
main (X11 thread)
  ├─ drain events (map, configure, visibility, focus, close)
  ├─ update presentable / size / focus flags
  └─ on Expose / resume visible → RequestRedraw()

ticker goroutine (~60Hz)
  └─ if presentable → RequestRedraw()   // coalesced (cap=1)

render goroutine (ONLY place that touches GPU)
  ├─ wait RequestRedraw
  ├─ FlushCallbacks + SyncLostState
  ├─ if !presentable → pump callbacks only, skip GCT
  ├─ draw composite scene → BeginFrame / Present
  └─ ErrDeviceLost → skip; EnableAutoRecover rebinds next frame
```

`RequestRedraw` never blocks and **coalesces** (at most one pending frame). GPU work is **single-threaded** on the render goroutine (not multi-threaded device use).

## Device-lost policy

1. Always install DeviceLost + Uncaptured (via `RequestDevice` / library).
2. `EnableAutoRecover` + host rebind of `SetDeviceProvider` (purge GPU caches).
3. Unpresentable (minimized / FullyObscured) → **no** `GetCurrentTexture`.
4. Unfocused but still visible → **keep** drawing.
5. After long hide (≥2s) resume → optional `MarkLost` force recreate.
6. Never `os.Exit` on `ErrDeviceLost`.

## Manual acceptance

1. Start: animation + `frame=` growing.
2. Fully cover window with another window ≥5–10 min: process lives; no SIGABRT.
3. Uncover: `visible again` / recover logs; animation continues with latest state.
4. Optional: minimize / restore.

See also `docs/GPU_修复_device_lost.md`, `docs/WGPU_NATIVE_DEVICE_LOST.md`.
