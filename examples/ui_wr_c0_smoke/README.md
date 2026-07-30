# ui_wr_c0_smoke (C0 · W0)

**Combo:** R0 FullPaint + R12 metrics schema + R16 WarmUp first present.

**Quality bar:** mini app shell + static dense + hot + LiveHUD + phases（推翻重写集成回归）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_c0_smoke     # close C0
RUN_SECONDS=15 go run ./examples/ui_wr_c0_smoke    # optional longer sample
```

**Window:** client **1200×800** (U15, fixed). **RUN_SECONDS &lt; 5 → FAIL.**  
**Close duration:** **5s** (WIDGET_RENDER §2.5 C0).

## Visible effects

Panel layout (local Place inside each region; labels use system Face):

```
[ TopBar 1200×48 ]
[ Legend 280×300 @16,60 ] [ Grid 280×300 @320,60 ] [ Hot 560×300 @620,60 ]
[ LiveHUD bottom ]
```

| Region | Expectation |
|--------|-------------|
| TopBar | Title with Face (not blank holes) |
| Legend | 8 faced lines covering R0/R12/R16 |
| STATIC 3×3 | Always filled after WarmUp |
| HOT pulse | Continuously animates + extra static |
| LiveHUD | Ability, phase, FPS, policy |

## Gates

- `RUN_SECONDS >= 5`
- `present_count >= 1`
- `present_policy == full_paint`
- Full §2.2 metrics schema (R12 via C0)
- `warmup: true` (R16 subset)
- Effective FPS ≥ 55 (prefer `fps_interval`)
- `static_cells >= 8`

Does **not** replace solo windows:

- `ui_wr_r0_fullpaint` (R0)
- `ui_wr_r12_metrics` (R12)
- `ui_wr_r16_warmup` (R16)

C0 is **integration only** (U5/U6).
