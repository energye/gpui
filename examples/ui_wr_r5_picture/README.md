# ui_wr_r5_picture — R5 Picture record / replay (§2.6 质量条)

**Ability:** R5 (W1–W2) Picture 录/回放  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=5`**; observe 15s  
**Quality bar:** U17 multi-region · U18 LiveHUD · U20 实现点六维

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture     # close R5 (U16 min)
RUN_SECONDS=15 go run ./examples/ui_wr_r5_picture    # observe CPU/RSS
WR_HUD=0 RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture  # optional: hide HUD
```

## Visible effects

Layout (1200×800 logical):

```
[======== TopBar (0,0) 1200×48 =====================================]
[ Legend (12,60) 260×300 ] [ Direct (284,60) 440×480 ] [ Replay (736,60) 440×480 ]
[================ LiveHUD (0, h-72) 1200×72 =========================]
```

| Side | Content |
|------|---------|
| **Left (DIRECT)** | 4 colored rects + triangle path + rounded-rect stroke + text label + solid border + circle — drawn live each frame via DC API |
| **Right (REPLAY)** | Same geometry recorded once as Picture display list, replayed each frame via `pic.Replay(dc)` |
| **Legend** | 8 color-coded lines explaining each op type |
| **LiveHUD** | R5 · phase · fps · p95 · policy · op/replay/direct counts |

Both sides should look **identical** — same colors, positions, shapes, text. If they differ, Picture replay is broken.

## Phases

| Phase | Window (default 5s) | Behavior |
|-------|---------------------|----------|
| Steady | 0–1.5s | MarkNeedsPaint rate ×4 |
| Spike | 1.5–3.5s | Rate ×10 (stress test replay under full_paint) |
| Recover | 3.5s–end | Rate ×3 |

## Picture ops demonstrated

| Op | Count | What |
|----|-------|------|
| `OpFillRect` | 4 | 2×2 colored block grid |
| `OpFillPath` | 2 | Triangle + circle (deep-cloned paths) |
| `OpStrokePath` | 1 | Rounded rectangle outline, 3px |
| `OpDrawString` | 2 | "Picture ≡ Direct" + "dashed border + circle path" |
| `OpStrokeRect` | 1 | Solid border rect (Picture has no SetDash — both sides solid for ≡) |

**Total: ≥9 ops** (gate: ≥5).

## Gates (FAIL → exit 1)

- `RUN_SECONDS >= 5`
- `present_count >= 1`
- `present_policy == full_paint`
- §2.2 full-family metrics schema (A–J via wrgate)
- `fps_interval` (or wall) **≥ 55** after full min run (60Hz-class)
- `picture_op_count >= 5`
- `replay_frames >= 10`

JSON on **stdout**; human log on **stderr**.

## 实现点六维 (U20)

| 维度 | 本窗回答 |
|------|----------|
| **正确性** | Picture Replay 的 6 类 op 产出像素 ≡ 直绘 DC 调用 |
| **脏区** | 两侧每 tick `MarkNeedsPaint`；FullPaint 重绘全部 |
| **缓存** | R5 不关 BoundaryCache（R3）；Picture 是 display list，不是缓存 |
| **边界条件** | Spike 加速脉冲；path 深拷贝隔离；空 text no-op |
| **失败模式** | 左右不一致 / ops<5 / replays<10 / fps<55 → FAIL |
| **窗内如何看出** | LiveHUD ops/replay 计数 + 左右两栏视觉对比 |

## Does not close

Boundary cache (R3), Retained policy, or layer-level transforms (R6) — those are separate R items. Picture has no Rotate/Scale API; transforms live at the layer level.
