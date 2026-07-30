# ui_wr_r3_boundary — R3 Boundary true cache

**Ability:** R3 (W1)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=10`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary
```

## Visible effect

| Region | Effect |
|--------|--------|
| Outer hot RB (red) @ (280,80) 420×320 | Pulsing red ~4 Hz, re-records each tick |
| Inner-static RB (green) @ (300,140) 180×180 | Frozen — `NeedsPaint()` always false |
| Inner-hot RB (blue) @ (500,140) 180×180 | Pulsing blue ~6 Hz, re-records each tick |
| Static text inside outer @ (300,100) | Frozen — text in boundary can skip |
| Dense static grid (6×2) @ (740,80) | Frozen — outer/inner pulsing does not dirty these |
| LiveHUD band @ y=728 | R3 + phase + fps + p95 + skip + rr + cnt + depth |
| Legend @ (12,60) | 9 color-block + text rows explaining each region |

## Gates

- `RequireFullPaintPolicy` (default present_policy under W1)
- `RequirePersistentFPS` + `MinFPSWall=55` + `MinFPSElapsed=5`
- `MaxP95Ms=22`
- `MinPresents=1`
- `MinBoundarySkip=1` (clean static layer Replays at least once)
- `MinBoundaryRerecord=1` (hot layer re-records)
- `boundary_count ≥ 3` (multi-level nesting: root + outer + inner-static + inner-hot)
- `boundary_max_depth ≥ 2` (nested RB)
- `outer_clean_ticks ≥ 10` (outer boundary isolation invariant)
- `inner_static_clean ≥ 10` (inner static boundary stays clean)

## U20 实现点六维

| 维度 | R3 说明 |
|------|---------|
| 正确性 | nested RB: inner hot dirty re-records only inner; outer hot dirty re-records only outer; static layers Replay (skip) |
| 脏区 | boundary_skip accumulates from clean static Replays; boundary_rerecord only from dirty boundary; FullPaint redraws all but cache hits still skip |
| 缓存 | BoundaryCache: text + nested static in boundary can skip via Replay; dirty boundary forces re-record |
| 边界条件 | 3-level nesting (root→outer RB→inner RB); outer + inner independent dirty cycles; empty subtree handled |
| 失败模式 | boundary_count=0 (no RB discovered) / cache miss all paths / inner dirty leaks to outer Picture = FAIL |
| 窗内如何看出 | outer=red pulsing re-records; inner-static=green frozen; inner-hot=blue pulsing; HUD shows skip↑/rr↑ |
