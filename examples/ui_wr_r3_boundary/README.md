# ui_wr_r3_boundary — R3 Boundary true cache

**Ability:** R3 (W1)  
**Window:** **1200×800** (U15)  
**Close duration:** **`RUN_SECONDS=10`** (≥5 hard min, U16 / §2.5)

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary
```

`RUN_SECONDS < 5` → **FAIL** (cannot close R3).

## Visible effect

| Region | What you should see |
|--------|---------------------|
| Green box @ (48,48) 180×180 | **Static** — stays put every frame (Picture Replay / `boundary_skip`) |
| Red box @ (700,280) 140×140 | **Pulsing** — color changes every tick (`boundary_rerecord`) |

## Gates (FAIL + exit 1)

- `present_count >= 1`, `present_policy=full_paint`
- §2.2 metrics schema (CPU/RSS/FPS family)
- **`boundary_skip >= 1`** (clean static must Replay)
- **`boundary_rerecord >= 1`** (hot dirty re-records)
- Steady FPS ≥55 when run ≥5s (`fps_interval` preferred)

## Implementation note

Picture-backed `BoundaryCache` on `PipelineOwner`, enabled under FullPaint present so skip is observable without Retained Present (Replay still draws after GPU Clear).
