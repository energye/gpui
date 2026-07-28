# ui_wr_r4b_multidamage — R4b multi dirty / DirtyLayerIDs

**Ability:** R4b (W2)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=15`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_r4b_multidamage
```

## Visible effect

| Region | Effect |
|--------|--------|
| Orange TL @ (40,40) | Pulses |
| Cyan BR @ (1050,650) | Pulses |
| Gray mid @ (400,250) | **Static** — stays between the two hots |

## Gates

- `present_policy=retained`
- `dirty_layer_id_max ≥ 2`
- **`damage_multi_frames ≥ 1`** (distant dirties keep independent scissors; union AABB may be large)
- §2.2 + fps
