# ui_wr_r19_snap — R19 1px / 设备像素对齐

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# Gate: 5s, frozen Steady, GPU readback PNG + pixel/Golden gates.
RUN_SECONDS=5 go run ./examples/ui_wr_r19_snap -auto-only

# First-time Golden seed (review the PNG before committing it):
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_wr_r19_snap -auto-only

# Manual: 15s Steady -> Spike -> Recover, no gate PNG is written.
go run ./examples/ui_wr_r19_snap -manual-seconds 15
```

`-auto-only` uses the required 5s gate when `RUN_SECONDS` is unset. A gate
shorter than 5s fails. Manual mode requires at least 15s so all three phases
and the Recover settle window are actually observed.

## Window / close duration

1200x800 logical pixels, GPU window required. Gate close duration is 5s.
Manual close duration is 15s. The real host window on this machine is X11
DPR 1; the HUD shows the host DPR. DPR 1.5/2 are separate offscreen engine
pixel probes, not a second physical window.

## Scene

The shell is the standard wrkit layout: top bar, left legend, body, and a
72px live HUD band. HUD is excluded from Golden because its counters refresh.

| Region | Content | Expected |
|---|---|---|
| SNAP COORD (left, (296,72) 440x260) | green `SnapCoord` filled bars; red same-width bars offset by +0.5 device px | green bars are exactly one full device pixel; red bars split into two half-covered pixels |
| SNAP LINE (right, (748,72) 428x260) | 10 vertical + 5 horizontal `SnapLine` hairlines; orange `SnapRect` border drawn as four filled 1px edges | each hairline/border edge has exactly one full device pixel; no double-width line |
| DPR CONTRACT (left, (296,346) 440x160) | deterministic snap contract values for DPR 1, 1.5, 2 | body text is static and Golden-checked |
| HOT (right, (748,346) 428x160) | 1px crosshair using `SnapCoord` on both axes | gate fixes it at (180,90); manual moves it |
| STATIC DENSE (bottom, (296,518) 880x180) | 16 exact-colour chips + 16 `c00..c15` labels + 4 summary labels | chips are pixel-probed; labels are ink-probed |
| HUD (bottom 72px) | FPS, p95, DPR, snapped/raw spec counts, PASS/FAIL preview | updates live; not part of Golden |

## Three-evidence gate

Gate mode freezes the scene in Steady (`phases_seen=["Steady"]`,
`hot_moves=0`) so the readback PNG is deterministic.

- **Logic**: 29/29 probes. Includes snap math for `SnapCoord` / `SnapLine` /
  `SnapRect` at DPR 1/1.5/2, actual spec counts, frozen phase/hot state,
  full-paint policy, CPU path fields, and zero CPU fallback.
- **Pixel**: 52/52 probes. 8 backgrounds, 16 exact chip colours, 10 text-ink
  regions, and 18 per-axis coverage probes:
  - 4 snapped bar probes: exactly one full pixel (`full_px=1`);
  - 4 raw bar probes: no full pixel and two mixed pixels (`mixed_px=2`);
  - 4 `SnapLine` hairline probes: exactly one full pixel;
  - 4 `SnapRect` border probes: exactly one full pixel;
  - 2 frozen hot-crosshair probes: exactly one full pixel.
- **Golden**: header 1200x48 + legend 260x656 + body 904x656 =
  **821184 px**, zero tolerance. Two consecutive non-seed runs:
  `diff=0.0000%` over `821184 px`.
- **DPR cross-check**: `render.NewContext(WithDeviceScale(1/1.5/2))` with the
  registered GPU accelerator. Each scale reports `snapped_full_px=1`,
  `raw_mixed_px=2`, `hairline_full_px=1`.

Latest gate runs (2026-10-22, X11 DPR1, two consecutive non-seed runs):

```text
presents=300/300  fps_interval=60.17/60.16  p95=17.29/17.19ms
pixel=52/52  logic=29/29  golden=0.0000% / 821184px
dpr_probes_ok=true  snapped=4/4 raw=4/4 hairline=4/4 border=4/4 hot=2/2
```

Manual 15s run: `presents=899`, `fps_interval=59.98`, `p95=17.30ms`,
logic `26/26`, phases `Steady/Spike/Recover`, hot moves `119/180/1`,
`manual_elapsed=15.10s`, `recover_settled=true`,
`recover_moves_after_settle=0`. Manual explicitly reports
`pixel_evidence=manual_not_asserted` and `pixel_golden_diff_pct=null`; it
does not write the committed gate PNG or Golden baseline. Closing the window
early or missing a phase fails the manual run.

## Honest boundaries

- **Raw-vs-snapped contrast uses filled bars, not stroked hairlines.** The GPU
  backend force-snaps axis-aligned 1px strokes in
  `render/internal/gpu/stroke_hairline.go`, so a raw stroked hairline does not
  blur on this backend and cannot be used as blur bait. The filled 1-device-px
  bars show the real coverage difference.
- **The physical window is DPR1.** DPR 1.5/2 values come from offscreen
  `render.Context` runs with an explicit device scale and are labeled as such.
- **FullPaint is intentional.** This is a correctness window; `damage_ratio=1`
  is the real full-window damage and is not presented as retained locality.
- **Short-run RSS is not a leak verdict.** `rss_slope_valid=false` on both
  gate and manual runs; the raw startup-inclusive slope is reported but is not
  used to claim stability. Steady slope is omitted with a note because the run
  is shorter than the >=30s steady-fit window.
- **No production consumer is claimed.** This is atomic verification for R19.

## U20 implementation points

| Dimension | What this window proves |
|---|---|
| Correctness | `SnapCoord` lands on pixel boundaries, `SnapLine` returns a half-pixel stroke center, `SnapRect` edges stay integral; pixel probes verify actual coverage, not just math. |
| Dirty / cost | Gate freezes Steady and pins FullPaint; HUD is outside Golden. Manual moves only the hot crosshair (plus its own HUD), so motion is attributable and does not rewrite committed evidence. |
| Cache | No cache capability is claimed. DPR probes are independent contexts; the window does not pretend to test boundary caching. |
| Edge conditions | Both X and Y axes are probed; DPR 1, 1.5, and 2 are covered; rectangular border uses four filled edges to avoid centered-stroke ambiguity at fractional scale. |
| Failure modes | A snapped bar with two full pixels, a raw bar without two mixed pixels, a hairline split across two pixels, a non-frozen gate hot spot, `diff!=0`, or a missing DPR probe fails the gate. |
| In-window visibility | Green/red bars, cyan hairlines, orange border, moving crosshair, 16 chips + labels, and a live HUD make the capability visible without reading JSON. |

## Gates

- GPU window opens; `present_count >= 1`; `present_policy=full_paint`.
- Gate runs at least 5s; `fps_interval >= 55`; `interval_p95_ms <= 22`.
- Gate scene is frozen: only Steady is observed and `hot_moves=0`.
- Snapped/raw/hairline/border/hot coverage probes all pass.
- Golden diff is exactly `0.0000%` over `821184 px`.
- Offscreen DPR 1/1.5/2 probes all pass.
- `cpu_fallback_ops=0`; `vsync_source=true` (not fallback); CPU UI/rasters are
  not both zero; `gpu_ops` is cumulative.
- Manual: wall-clock run >=15s, Steady/Spike/Recover all seen, hot moves in
  each phase, Recover settles with zero moves after settle, hot stays in
  bounds. Closing the window early fails.
- Gate and manual emit the §2.2 A-J shell. `g_metrics=skipped` with a reason
  because R19 does not measure text-cache metrics.
- J: this repo has no `TestNoGPUImport` file (`go test ./ui -run
  TestNoGPUImport` reports no tests to run, same as R0/C0). The direct checks
  `grep -rn '^import "C"' ui/ --include='*.go'` and `grep -rn
  '^\s*"github.com/energye/gpui/gpu' ui/ --include='*.go'` both have no output;
  that is what `depcheck=passed_no_cgo_no_ui_gpu_import` records.
