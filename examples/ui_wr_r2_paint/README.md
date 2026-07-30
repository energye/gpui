# ui_wr_r2_paint — R2 local NeedsPaint

**Ability:** R2 (W1)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=5`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint
```

## Visible effect

| Region | Expectation |
|--------|-------------|
| Hot A (red) @ (276,70) 180×180 | Pulses red at ~5 Hz (different frequency from B/C) |
| Hot B (green) @ (740,70) 180×180 | Pulses green at ~3 Hz (different frequency) |
| Hot C (blue) @ (508,320) 180×180 | Pulses blue at ~7 Hz (different frequency) |
| Static boundary @ (276,540) 460×80 | Frozen — `NeedsPaint()` always false |
| Static labels under each hot | Frozen — never reflow when hot pulses |
| Dense static grid (4×2) @ (936,320) | Frozen — proves hot B/C does not dirty neighbors |
| LiveHUD band @ y=728 | R2 + phase + fps + p95 + hotA/B/C counts + staticClean |
| Legend @ (12,60) | 8 color-block + text rows explaining each region |

## Gates

- `RequireFullPaintPolicy` (default present_policy under W1)
- `RequirePersistentFPS` + `MinFPSWall=55` + `MinFPSElapsed=5`
- `MinPresents=1`
- `MinBoundarySkip=1` (static boundary Replays under full_paint)
- `static_clean_ticks ≥ 10` — R2 isolation invariant: static boundary
  `NeedsPaint()` must stay false after all three hots `MarkNeedsPaint`
- `hotA_dirty_ticks ≥ 10` / `hotB_dirty_ticks ≥ 10` / `hotC_dirty_ticks ≥ 10`
  — each hot must independently `MarkNeedsPaint` every tick

## U20 实现点六维

| 维度 | R2 说明 |
|------|---------|
| 正确性 | 仅目标 hot boundary MarkNeedsPaint；static boundary NeedsPaint() 恒 false |
| 脏区 | paint_count ∝ dirty boundary 数；FullPaint redraws all but only hots pulse |
| 缓存 | N/A for R2 (BoundaryCache is R3)；R2 proves local repaint isolation |
| 边界条件 | 3 hot boundaries at different frequencies；static neighbors with text labels |
| 失败模式 | static boundary becomes NeedsPaint=true after hot MarkNeedsPaint = leak = FAIL |
| 窗内如何看出 | 3 colored hot regions (A=red, B=green, C=blue) pulse at different rates；static text labels and color blocks stay frozen |
