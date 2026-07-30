# ui_wr_r12_metrics (R12 · W0)

**Ability:** §2.2 full-family metrics **schema completeness** on a real GPU window.

**Hard rule (U5):** this is the **solo** R12 window. `ui_wr_c0_smoke` is integration only and **must not** close R12.

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r12_metrics
```

**Window:** **1200×800**. **RUN_SECONDS &lt; 5 → FAIL.**

## Visible effects

| Region | Expectation |
|--------|-------------|
| TopBar / Legend / Body | Shell with static grid + hot pulse |
| LiveHUD | Ability R12, phase, FPS, schema note |
| stdout JSON | Every `wrgate.RequiredSchemaKeys` entry present |

## Gates

- `present_count >= 1` (real window, not stub)
- **Schema:** all RequiredSchemaKeys present (`SchemaOnly` primary)
- `warmup: true` field present in JSON

## Does not close

Business gates of R0/R3/… — only field presence. Each ability still needs its own solo window.
