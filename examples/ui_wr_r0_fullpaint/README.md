# ui_wr_r0_fullpaint (R0 · W0)

**Ability:** FullPaint correctness — static + animated content on a real GPU window.

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint    # minimum (U16)
RUN_SECONDS=15 go run ./examples/ui_wr_r0_fullpaint   # better CPU/RSS sample
```

**Window:** client **1200×800** (U15). **RUN_SECONDS &lt; 5 → FAIL** (cannot close R0).

## Visible effects

| Region | Expectation |
|--------|-------------|
| **Green** 160×160 @ (40,40) | Always visible (static boundary) |
| **Red/orange** 120×120 @ (600,300) | Pulses continuously |
| Dark gray background | Clear color |

If green disappears while red still moves, FullPaint is broken.

## Gates (FAIL → exit 1)

- `RUN_SECONDS >= 5`
- `present_count >= 1`
- `present_policy == full_paint`
- §2.2 full-family metrics schema
- `fps_interval` (or wall) **≥ 55** after full min run (60Hz-class)
- CPU% / RSS fields present (Linux)

JSON on **stdout**; human log on **stderr**.

## Does not close

Boundary cache (R3), Retained policy, or damage≪fullscreen — those are later W items.
