# ui_wr_c7_resize_dpr — C7 resize/DPR combo

**Combo:** C7 covers R11 + R3 (+ R19 hairline awareness)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=15`**

> Combo only — does **not** close solo R11/R3.

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_c7_resize_dpr
```

## Gates

- cache invalidations ≥ 1
- boundary_skip ≥ 1, boundary_count ≥ 2
- §2.2 + fps
