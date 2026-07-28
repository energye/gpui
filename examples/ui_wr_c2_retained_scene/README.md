# ui_wr_c2_retained_scene — C2 retained multi-boundary + Picture

**Combo:** C2 covers R3+R4+R4b+R5  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=15`**

> Combo only — does **not** close solo R packages.

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_c2_retained_scene
```

## Visible effect

Nested static panel + pulsing hot, distant second hot, and a Picture-replay stack. Under retained present, damage stays local.

## Gates

- `present_policy=retained`
- `damage_ratio_avg < 0.45`
- `boundary_skip ≥ 1`, `boundary_count ≥ 3`
- picture ops + dirty ids present
