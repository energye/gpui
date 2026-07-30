# ui_wr_r3b_compbits — R3b Compositing bits / nested boundary discovery

**Ability:** R3b (W1)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=8`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits
```

## Visible effect

| Region | Effect |
|--------|--------|
| Outer RB (blue) @ (280,90) 460×380 | depth 1, nest root — frozen outline |
| Mid RB (steel) @ nested 320×280 | depth 2, NeedsCompositing propagates up |
| Static leaf (green) @ mid (24,24) 90×90 | depth 3, frozen — boundary_count++ |
| Hot leaf (red) @ mid (180,130) 100×100 | depth 3, pulses each tick |
| Dense static grid (6×2) @ (780,90) | Frozen — proves nest pulsing does not dirty neighbors |
| LiveHUD band @ y=728 | R3b + phase + fps + p95 + cnt + depth + ncOuter |
| Legend @ (12,60) | 9 color-block + text rows explaining each region |

## Gates

- `RequireFullPaintPolicy` (default present_policy under W1)
- `RequirePersistentFPS` + `MinFPSWall=55` + `MinFPSElapsed=5`
- `MinPresents=1`
- `MinBoundaryCount=3` (multi-level nesting: outer + mid + 2 leaves = 4)
- `MinBoundaryMaxDepth=2` (nested RB depth ≥2)
- `MinBoundarySkip=1` (clean static nest contributes skip)
- `NeedsCompositing propagation`: outer + mid both `NeedsCompositing()` after `UpdateCompositingBits`

## U20 实现点六维

| 维度 | R3b 说明 |
|------|---------|
| 正确性 | UpdateCompositingBits walks tree; outer + mid boundaries become NeedsCompositing (child is RB) |
| 脏区 | boundary_count≥3 + boundary_max_depth≥2 validates compositing chain depth discovery |
| 缓存 | N/A for R3b (BoundaryCache is R3); R3b proves compositing bits / discovery, not cache hits |
| 边界条件 | 3-level nesting (root→outer RB→mid RB→leaf RB); static + hot leaves independent dirty cycles |
| 失败模式 | boundary_count=0 (no RB discovered) / depth=1 (no nesting) / NeedsCompositing not propagating up = FAIL |
| 窗内如何看出 | outer + mid boundary outlines visible; depth=3 counter in HUD; NeedsCompositing=yes |
