# ui_wr_r9_text_cache — R9 measure cache

**Ability:** R9 (W1)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=5`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r9_text_cache
```

## Visible effect

| Region | Effect |
|--------|--------|
| TopBar @ (0,0) 1200×48 | "R9 measure cache — 4 text styles, repeated MarkNeedsLayout, hits≫miss" |
| Legend @ (12,60) 240×540 | 8 color-block + text rows |
| Text panel 1 @ (276,70) 460×100 | FontSize 14, repeated wrap words |
| Text panel 2 @ (276,200) 460×100 | FontSize 18, different color |
| Text panel 3 @ (756,70) 432×140 | FontSize 24, big text |
| Text panel 4 @ (756,230) 432×100 | FontSize 14, another style |
| Static grid (4×4) @ (276,320) | Frozen — proves cache warming does not dirty neighbors |
| LiveHUD band @ y=728 | R9 + phase + fps + p95 + hits + miss + layouts |

## Gates

- `RequireFullPaintPolicy` (default present_policy under W1)
- `RequirePersistentFPS` + `MinFPSWall=55` + `MinFPSElapsed=5`
- `MinPresents=1`
- `measure_cache_hit ≥ 1` (cache warmed)
- `hits ≥ miss` (cache effective after warm)

## U20 实现点六维

| 维度 | R9 说明 |
|------|---------|
| 正确性 | same text + same style under repeated MarkNeedsLayout → width/height stable, cache hits dominate |
| 脏区 | MarkNeedsLayout forces layout each tick but text content unchanged → MeasureCache hits |
| 缓存 | MeasureCache hits≥1 + hits≥miss; cache keyed by (text, face, size, maxWidth) |
| 边界条件 | 4 distinct styles (size 14/18/24 + color variants); empty text no-op; same text different style = different cache entry |
| 失败模式 | hits=0 (cache never warmed) / hits<miss (cache ineffective) = FAIL |
| 窗内如何看出 | hits/miss counter in HUD; text stable not jittering |
