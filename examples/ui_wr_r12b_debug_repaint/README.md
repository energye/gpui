# ui_wr_r12b_debug_repaint — R12b debug repaint overlay

**Ability:** R12b (W1)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=8`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint
# off path:
DEBUG_REPAINT=0 RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint
```

## Visible effect

| Region | Effect |
|--------|--------|
| Hot A (red) @ (276,70) 180×180 | Pulses red ~4 Hz, magenta overlay marks dirty |
| Hot B (green) @ (740,70) 180×180 | Pulses green ~3 Hz, magenta overlay marks dirty |
| Hot C (blue) @ (508,320) 180×180 | Pulses blue ~5 Hz, magenta overlay marks dirty |
| Static boundary @ (276,540) 460×80 | Frozen — no magenta overlay (clean) |
| Static grid (4×2) @ (936,320) | Frozen — no magenta overlay |
| LiveHUD band @ y=728 | R12b + phase + fps + p95 + debug + draws + hot + staticClean |
| Legend @ (12,60) | 9 color-block + text rows explaining each region |
| DEBUG_REPAINT=0 | off path — 0 overlay draws |

## Gates

- `RequireFullPaintPolicy` (default present_policy under W1)
- `RequirePersistentFPS` + `MinFPSWall=55` + `MinFPSElapsed=5`
- `MinPresents=1`
- `debug=1`: `debug_repaint_draws ≥ 1` + `hot_dirty_ticks ≥ 10` + `static_clean_ticks ≥ 10`
- `DEBUG_REPAINT=0`: `debug_repaint_draws = 0` (off path)

## U20 实现点六维

| 维度 | R12b 说明 |
|------|---------|
| 正确性 | debug=1 时脏区染色 (magenta overlay)、静区不染; off=0 overlay; 染色区域与脏区一致 |
| 脏区 | debug_repaint_draws 累计 live 重绘数 (非 cache Replay); hot MarkNeedsPaint each tick |
| 缓存 | N/A for R12b (BoundaryCache is R3); R12b proves debug overlay visualization, not cache hits |
| 边界条件 | 3 hot boundaries at different frequencies; static boundary frozen; off path (DEBUG_REPAINT=0) must 0 overlay |
| 失败模式 | debug=1 时 draws=0 (overlay 不渲染) / off 时 draws>0 (overlay 泄漏) = FAIL |
| 窗内如何看出 | 3 hot regions (A/B/C) flash magenta overlay each tick; static boundary and neighbors stay clean (no magenta) |
