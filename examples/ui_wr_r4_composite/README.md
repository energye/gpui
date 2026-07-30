# ui_wr_r4_composite — R4 retained composite (§2.6)

**Ability:** R4 — retained Composite Present (CompositeOnly + damage Present)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=15`**  
**Quality bar:** §2.6 Shell+LiveHUD+PhaseClock · ≥4 regions · Legend≥8

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_r4_composite
RUN_SECONDS=30 go run ./examples/ui_wr_r4_composite   # observe CPU/RSS
```

`RUN_SECONDS < 5` → FAIL (U16).

## Layout (1200×800)

```
[======== TopBar =====================================================]
[ Legend ≥8 ] [ Static grid 4×4      | Hot anim pulse      ]
[             ] [ (retained·skip)     | (damage source)     ]
[================ LiveHUD: skip/dmg/multi =============================]
```

| Region | Expectation |
|--------|-------------|
| **Static grid** | 4×4 dense cells — boundary skip under retained (not damaged) |
| **Hot anim** | Pulse box — MarkNeedsPaint each tick; damage only in this region |
| **Side static** | 4 cells next to hot — must stay clean |
| **LiveHUD** | R4 · phase · fps · skip/dmg/multi · policy |

## Phases (15s)

| Phase | Window | Behavior |
|-------|--------|----------|
| Steady | 0–5s | hot ×4 pulse |
| Spike | 5–10s | hot ×10 (faster damage) |
| Recover | 10s–end | ×3 |

## Gates (FAIL → exit 1)

| Gate | Threshold | Source |
|------|-----------|--------|
| `present_policy` | `retained` | R4 |
| `fps_interval` | ≥55 after ≥5s | A |
| `boundary_skip` | 0（retained 下不设门禁） | R3 proof |
| `damage_ratio_avg` | **< 0.35** | R4 |
| `present_mode` | != full（retained steady） | R4 |
| `damage_samples` | ≥10 | retained present samples |
| `static_cells` | ≥8 | dense static proof |
| §2.2 full-family A–J | via wrgate | all |

JSON on **stdout**; human log on **stderr**.

## 实现点六维 (U20)

| 维度 | 本窗回答 |
|------|----------|
| **正确性** | retained 下 static 静在；CompositeOnly 跳过干净层 |
| **脏区** | damage ∝ 热区（+HUD ~10Hz）；网格不进 damage |
| **缓存** | static boundary Replay cached; hot boundary rerecord per dirty tick |
| **边界条件** | Spike 双倍热率；Recover 回到 Steady 节奏 |
| **失败模式** | policy≠retained / skip=0 / dmg≥0.35 / fps<55 |
| **窗内如何看出** | LiveHUD skip/dmg + 左网格 + 右热脉冲 |
