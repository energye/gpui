# ui_wr_c1_boundary_nest — C1 combo (nested boundary)

**Combo:** C1 covers R2+R3+R3b (+ optional R12b tint)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=10`**

> Combo only — **does not** close solo R2/R3/R3b/R12b packages.

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/ui_wr_c1_boundary_nest
DEBUG_REPAINT=1 RUN_SECONDS=10 go run ./examples/ui_wr_c1_boundary_nest  # magenta-ish dirty tint
```

## Visible effect

Nested outer panel with mid panel (green static + pulsing hot) and a blue side bar. Only the hot cell pulses; static nest members stay put (`boundary_skip`).

## Gates

- `boundary_skip >= 1`, `boundary_rerecord >= 1`
- `boundary_count >= 3`, `boundary_max_depth >= 2`
- §2.2 schema + presents + fps
