# ui_wr_c2_retained_scene — C2 retained multi-boundary + Picture (§3.1)

**Combo:** C2 covers **R3+R4+R4b+R5** (integration only — does **not** close solo R)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=15`**  
**Quality bar:** §3.1 Shell+LiveHUD+PhaseClock · ≥6 regions · Legend≥8 · gate ∪

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_c2_retained_scene
WR_HUD=0 RUN_SECONDS=15 go run ./examples/ui_wr_c2_retained_scene  # optional: hide HUD
```

`RUN_SECONDS < 5` → FAIL (U16).

## Layout (1200×800)

```
[======== TopBar =====================================================]
[ Legend ≥8 ] [ Nest R3 | Static grid | Picture R5 ]
[             ] [ Hot-TL ………… mid static ………… Hot-BR  (R4b dual)  ]
[================ LiveHUD: skip/dmg/dirty/ops =========================]
```

| Region | Expectation |
|--------|-------------|
| **R3 Nest** | outer→mid→static boundaries; static colors/text stay; skip accumulates |
| **Static grid** | 4×4 retained cells — never pulse |
| **R5 Picture** | multi-op display list painted once, then boundary skip (not every-frame rerecord) |
| **Hot-TL / Hot-BR** | two distant pulses; middle static between them stays |
| **LiveHUD** | C2 · phase · fps · skip/rr · dmg · dirty · multi · ops |

## Phases (15s)

| Phase | Window | Behavior |
|-------|--------|----------|
| Steady | 0–4s | both hots pulse ×4 |
| Spike | 4–10s | both hots ×10 (integration stress) |
| Recover | 10s–end | ×3 |

## Gates (FAIL → exit 1) — ∪ of R3/R4/R4b/R5

| Gate | Threshold | Source |
|------|-----------|--------|
| `present_policy` | `retained` | R4 |
| `fps_interval` (or wall) | ≥55 after ≥5s | A / continuous tick |
| `boundary_skip` | 0（retained 下不设门禁） | R3 |
| `boundary_count` | ≥3 | R3/R3b-ish nest |
| `boundary_max_depth` | ≥2 | R3 nest |
| `damage_ratio_avg` | **< 0.45** | R4 (dual-hot+HUD headroom; still ≪1) |
| `dirty_layer_id_max` | ≥2 | R4b |
| `damage_multi_frames` | ≥1 **or** dmg already low | R4b planner |
| `picture_op_count` | ≥5 | R5 |
| `picture_paint_frames` | ≥1 | R5 painted |
| `static_cells` | ≥8 | dense static |
| `damage_samples` | ≥10 | retained present samples |
| §2.2 full-family A–J | via wrgate | all |

JSON on **stdout**; human log on **stderr**.

## 实现点六维 (U20 · integration)

| 维度 | 本窗回答 |
|------|----------|
| **正确性** | retained 下 nest+Picture 静在；双热独立脏；中段静不闪 |
| **脏区** | damage ∝ 两 hot（+HUD ~10Hz）；中心 grid/nest 不进 damage |
| **缓存** | 嵌套 RB skip；Picture host 首帧 Replay 后 skip |
| **边界条件** | Spike 双热加速；multi vs union；深 nest skip |
| **失败模式** | policy≠retained / skip=0 / dirty\<2 / ops\<5 / dmg≥0.45 / fps\<55 |
| **窗内如何看出** | LiveHUD 计数 + 左右热脉冲 + 中静 + Picture 色块 |

## Does not close

Solo **R3 / R4 / R4b / R5** — each has its own `ui_wr_r*` package. C2 is integration only.
