# ui_wr_r18_savelayer — R18 SaveLayer + budget

**Ability:** R18 (W2/W5)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=10`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/ui_wr_r18_savelayer
```

## Visible effect

Light background; a **semi-transparent red** group (SaveLayer opacity ~0.45) and a solid yellow box. Budget `MaxOps=1` rejects a second SaveLayer each paint (observable counters).

## Gates

- `savelayer_allow ≥ 1`
- `savelayer_reject ≥ 1`
- §2.2 + fps
