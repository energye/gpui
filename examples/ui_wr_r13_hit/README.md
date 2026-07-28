# ui_wr_r13_hit — R13 hit ≡ paint

**Ability:** R13 (W2)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=5`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r13_hit
```

## Visible effect

| Box | Position | DebugName |
|-----|----------|-----------|
| Green | (80,100) 200×200 | `green` |
| Red | (700,280) 160×160 | `red` |
| Cyan | (500,500) 140×140 | `cyan` |

**Scripted probes** hit each center (+ empty bg). Optional **pointer click** also logs hit name and brightens the target.

## Gates

- scripted probes **4/4** (`green`/`red`/`cyan`/empty)
- §2.2 + presents + fps
