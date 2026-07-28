# ui_wr_r3b_compbits — R3b compositing bits / boundary discovery

**Ability:** R3b (W1)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=8`** (≥5 hard min)

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits
```

## Visible effect

Nested panels (outer blue-gray → mid → green static + red pulsing leaf). Only the red leaf should visibly pulse; outer chrome stays stable.

## Gates

- `boundary_count >= 3`, `boundary_max_depth >= 2`
- `UpdateCompositingBits` → outer `NeedsCompositing`
- `boundary_skip >= 1` (clean nest members Replay when fully clean)
- §2.2 schema + presents + fps (≥5s)
