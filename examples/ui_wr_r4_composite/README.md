# ui_wr_r4_composite — R4 retained layer/composite present

**Ability:** R4 (W2)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=15`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_r4_composite
```

## Visible effect

| Region | Effect |
|--------|--------|
| Green static @ (48,60) | Stays put every frame under **retained** (not full-tree repaint) |
| Red hot @ (900,480) | Pulses; only this region should dirty |

## Gates

- `present_policy=retained`
- **`damage_ratio_avg < 0.35`** (≪ full screen)
- presents + §2.2 + fps≥55

## Implementation

`PipelineApp.SetPresentPolicy(retained)` → steady frames **CompositeOnly** paint + `PresentWithAuto` (damage / LoadOpLoad). Warm-up/resize still full paint. Global default remains `full_paint` until W6.
