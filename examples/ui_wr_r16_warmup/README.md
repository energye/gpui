# ui_wr_r16_warmup (R16 · W0)

**Ability:** First-frame / WarmUp full present with **visible content** (surface not black).

**Hard rule (U5):** this is the **solo** R16 window. `ui_wr_c0_smoke` is integration only and **must not** close R16.

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup
```

**Window:** **1200×800**. **RUN_SECONDS &lt; 5 → FAIL.**

## Visible effects

| Region | Expectation |
|--------|-------------|
| Body static cells | Bright colored tiles after open (WarmUp already painted) |
| HOT | Optional pulse after steady |
| LiveHUD | R16 · warmup · first present |
| If black after open | WarmUp / first present **broken** |

## Gates

- `PipelineOptions.WarmUp = true` (engine `presentSyncFull` before loop)
- `warmup: true` in JSON
- `present_count >= 1`
- `present_policy == full_paint`
- `time_to_first_present_ms < 2000` (Open → first observed PresentCount≥1)
- fps_interval ≥ 55 after full min run (60Hz-class continuous tick)

## Does not close

R0 FullPaint correctness or R12 schema — those need their own solo windows.
