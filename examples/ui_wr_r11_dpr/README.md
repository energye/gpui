# ui_wr_r11_dpr — R11 size / DPR cache invalidation

**Ability:** R11 (W2)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=15`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_r11_dpr
```

## Visible effect

A blue box that **resizes** at ~3s and ~6s (simulates size/DPR change). After each change the boundary cache is cleared → one **rerecord** wave, then clean frames **skip** again (no residual wrong size).

## Gates

- `cache_invalidations ≥ 1`
- `boundary_rerecord ≥ 1` after invalidation
- `boundary_skip ≥ 1` (recovery)
- §2.2 + presents + fps

## Implementation

`PipelineApp.InvalidateBoundaryCache` + `EventResize` path clears `BoundaryCache` (R11).
