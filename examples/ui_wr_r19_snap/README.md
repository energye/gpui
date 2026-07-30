# ui_wr_r19_snap — R19 1px / device-pixel snap

**Ability:** R19 (W1)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=5`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r19_snap
```

## Visible effect

| Region | Effect |
|--------|--------|
| TopBar @ (0,0) 1200×48 | "R19 1px pixel snap — hairline grid crisp across DPR wobble" |
| Legend @ (12,60) 240×540 | 8 color-block + text rows |
| Hairline grid @ (276,70) 660×420 | 1px stroke border + dividers every 60/80px + 3×2 cell borders |
| DPR compare @ (936,70) 240×80 | DPR 1.0/1.5/2.0 labels — line stays 1px crisp |
| Static grid (5×1) @ (936,170) | Frozen — proves hairline redraw does not dirty neighbors |
| LiveHUD band @ y=728 | R19 + phase + fps + p95 + lines + dpr |

## Gates

- `RequireFullPaintPolicy` (default present_policy under W1)
- `RequirePersistentFPS` + `MinFPSWall=55` + `MinFPSElapsed=5`
- `MinPresents=1`
- `line_draws ≥ 10` (1px hairlines drawn each tick)

## U20 实现点六维

| 维度 | R19 说明 |
|------|---------|
| 正确性 | 约定 scale 下 1px 线不糊; StrokeRect/StrokeLine with SetLineWidth(1.0) produce crisp 1px hairlines |
| 脏区 | line_draws 累计每帧 1px stroke 调用数; FullPaint redraws all hairlines each tick |
| 缓存 | N/A for R19 (BoundaryCache is R3); R19 proves 1px pixel snap, not cache hits |
| 边界条件 | 1px hairline grid (StrokeRect 边框 + StrokeLine 分割线); DPR wobble across Spike; 像素对齐坐标 |
| 失败模式 | 1px 线糊/断 (antialiasing 污开) / line_draws=0 (hairline 未画) = FAIL |
| 窗内如何看出 | 1px hairline 网格清晰不糊; DPR box 对比区域; HUD shows lines/dpr |
