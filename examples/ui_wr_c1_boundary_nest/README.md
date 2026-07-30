# ui_wr_c1_boundary_nest — C1 nested boundary + debug combo

**Ability:** C1 (W1) · integrates R2+R3+R3b+R12b  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=10`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/ui_wr_c1_boundary_nest
# enable R12b debug overlay:
DEBUG_REPAINT=1 RUN_SECONDS=10 go run ./examples/ui_wr_c1_boundary_nest
```

## Visible effect

| Region | Effect |
|--------|--------|
| Outer RB (blue) @ (280,90) 500×400 | depth 1, pulses blue ~2 Hz — only outer rerecord |
| Mid RB (steel) @ nested 360×280 | depth 2, nest container — NeedsCompositing propagates |
| Static leaf (green) @ mid (30,30) 100×100 | depth 3, frozen — Replay skip |
| Hot leaf (red) @ mid (200,120) 110×110 | depth 3, pulses red ~4 Hz — rerecord (magenta tint if debug=1) |
| Outer-side (cyan) @ outer (400,40) 70×200 | depth 2 sibling, frozen — outer dirty does not leak inward |
| Static grid (6×2) @ (820,90) | Frozen — proves nest pulsing does not dirty neighbors |
| LiveHUD band @ y=728 | C1 + phase + fps + p95 + skip + rr + cnt + depth + draws |
| Legend @ (12,60) | 8 color-block + text rows explaining each ability |

## Gates

- `RequireFullPaintPolicy` (default present_policy under W1)
- `RequirePersistentFPS` + `MinFPSWall=55` + `MinFPSElapsed=5`
- `MinPresents=1`
- `MinBoundarySkip=1` (clean static nest Replays)
- `MinBoundaryRerecord=1` (hot dirty re-records)
- `MinBoundaryCount=3` (multi-level nesting: outer + mid + 2 leaves + side)
- `MinBoundaryMaxDepth=2` (nested RB depth ≥2)
- `NeedsCompositing propagation`: outer + mid both `NeedsCompositing()` (R3b)
- `inner_static_clean ≥ 10` (内脏不外溢)
- `outer_side_clean ≥ 10` (外脏不内传)
- `inner_hot_dirty ≥ 10` + `outer_dirty ≥ 10`
- `debug=1`: `debug_repaint_draws ≥ 1` (R12b)

## U20 实现点六维 + impl_interaction

| 维度 | C1 说明 |
|------|---------|
| impl_interaction | R2 局部 NeedsPaint 驱动 R3 boundary 缓存 skip/rerecord; R3b UpdateCompositingBits 发现嵌套链; R12b debug overlay 染脏区 — 四能力同树同时工作 |
| 正确性 | 内脏→只内 rerecord (inner-static stays clean); 外脏→只外 rerecord (outer-side stays clean); debug 色只在脏区闪 (static 无 magenta) |
| 脏区 | boundary_skip accumulates from clean static Replays; boundary_rerecord only from dirty boundary |
| 缓存 | BoundaryCache: text + nested static in boundary can skip via Replay; dirty boundary forces re-record; compositing bits walk discovers nest depth |
| 边界条件 | 3-level nesting (root→outer RB→mid RB→leaf RB); outer-side sibling RB proves outer dirty does not leak inward; inner-static proves inner dirty does not leak outward |
| 失败模式 | boundary_count<3 / depth<2 / inner-static dirty after inner-hot pulse / outer-side dirty after outer pulse / debug tint on static = FAIL |
| 窗内如何看出 | outer+mid nest outlines; inner-hot pulses red (magenta tint if debug=1); inner-static green frozen; outer-side cyan frozen; HUD shows skip/rr/cnt/depth/draws |
