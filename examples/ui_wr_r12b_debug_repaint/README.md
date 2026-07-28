# ui_wr_r12b_debug_repaint — R12b debug repaint overlay

**Ability:** R12b (W1)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=8`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint          # debug ON (default)
DEBUG_REPAINT=0 RUN_SECONDS=5 go run ./examples/ui_wr_r12b_debug_repaint  # must draws=0
```

## Visible effect

When debug is on, the **hot** box shows a **magenta translucent flash** on each live re-paint. The static green box stays clean (Replay — no overlay).

## Gates

- `debug_repaint_draws ≥ 1` when debug on; `== 0` when `DEBUG_REPAINT=0`
- §2.2 schema + presents + fps
