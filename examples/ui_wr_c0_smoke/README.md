# ui_wr_c0_smoke (C0 · W0)

**Combo:** R0 FullPaint + R12 metrics schema + R16 WarmUp first present.

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_c0_smoke
```

**Window:** **1200×800**. **RUN_SECONDS &lt; 5 → FAIL.**

## Visible effects

- Large blue-ish static box on dark background (WarmUp + FullPaint).
- Surface stays filled (not black).

## Gates

- `RUN_SECONDS >= 5`
- `present_count >= 1`
- `present_policy == full_paint`
- Full §2.2 metrics schema
- `warmup: true`
- Effective FPS ≥ 55 (prefer `fps_interval`)

Does **not** replace `ui_wr_r0_fullpaint` for closing R0 alone.
