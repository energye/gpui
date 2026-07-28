# ui_wr_r2_paint — R2 local NeedsPaint

**Ability:** R2 (W1)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=5`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint
```

## Visible effect

| Region | Effect |
|--------|--------|
| Blue static @ (80,100) | Never pulses; clean RepaintBoundary |
| Orange hot @ (700,280) | Color pulses each tick |

## Gates

- static stays `!NeedsPaint` after hot `MarkNeedsPaint` (tick counters)
- `boundary_skip ≥ 1`, presents, §2.2 schema, fps≥55 when ≥5s
