# ui_wr_r0_fullpaint (R0 · W0)

**Ability:** FullPaint correctness — static dense + animated hot + text labels on a real GPU window under `present_policy=full_paint` (Clear each frame must not drop static).

**Quality bar:** U17 multi-region · U18 LiveHUD · U20 实现点六维（**✅v2** 推翻重写后）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint     # close R0 (U16 min)
RUN_SECONDS=15 go run ./examples/ui_wr_r0_fullpaint    # observe CPU/RSS
WR_HUD=0 RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint  # optional: hide LiveHUD
```

**Window:** client **1200×800** (U15, fixed). **RUN_SECONDS &lt; 5 → FAIL** (cannot close R0).  
**Close duration:** **5s** min; **15s** for better CPU/RSS observation (WIDGET_RENDER §2.5).

## Visible effects

Layout map (1200×800 logical; child coords are **panel-local**):

```
[======== TopBar (0,0) 1200×48 =====================================]
[ Legend (12,60) 272×320 ] [ Grid (300,60) 420×360 ] [ Hot (740,60) 440×360 ]
[================ LiveHUD (0, h-72) 1200×72 =========================]
```

| Region | Expectation |
|--------|-------------|
| **TopBar** | Title text with system Face (not blank) |
| **Legend panel** | Color swatches + ≥8 faced labels |
| **STATIC GRID 4×4** | 16 colored cells always visible under Clear |
| **HOT panel** | 160×160 pulse + side static strip + labels |
| **LiveHUD** | Ability, phase, FPS, p95, policy (Face set on DrawString) |
| Dark gray root | Clear color each frame |

**If labels are blank but color boxes show:** font failed to load (`wrkit.EnsureUIFace`) — check stderr for `UI font`.  
If the static grid disappears while hot still moves, **FullPaint is broken**.

## Phases (U17)

| Phase | Window (default 5s) | Behavior |
|-------|---------------------|----------|
| Steady | 0–1.5s | Hot pulse rate ×4 |
| Spike | 1.5–3.5s | Hot pulse rate ×10 (still FullPaint) |
| Recover | 3.5s–end | Hot pulse rate ×3 |

## Gates (FAIL → exit 1)

- `RUN_SECONDS >= 5`
- `present_count >= 1`
- `present_policy == full_paint`
- §2.2 full-family metrics schema (A–J via wrgate)
- `fps_interval` (or wall) **≥ 55** after full min run (60Hz-class)
- `interval_p95_ms ≤ 22` when FPS gated
- CPU% / RSS fields present (Linux)
- Structural: `static_cells >= 16`, `labels >= 8` (U17)

JSON on **stdout**; human log on **stderr**.

## 实现点六维 (U20)

| 维度 | 本窗回答 |
|------|----------|
| **正确性** | FullPaint 每帧全树 paint；Clear 后静格仍在 |
| **脏区** | hot 每 tick `MarkNeedsPaint`；静格首帧后 clean（FullPaint 仍全树提交） |
| **缓存** | R0 不关 BoundaryCache（R3）；本窗不依赖 skip |
| **边界条件** | Spike 加速 pulse；Recover 回稳；WarmUp 首帧有内容 |
| **失败模式** | 静格消失 / policy≠full_paint / fps&lt;55 → FAIL |
| **窗内如何看出** | LiveHUD policy/fps + 4×4 静格 + 图例 + 热块变色 |

## Does not close

Boundary cache (R3), Retained policy, or damage≪fullscreen — those are later W items. R16 full window remains optional (C0 covers WarmUp subset).
