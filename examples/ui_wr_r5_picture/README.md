# ui_wr_r5_picture — R5 Picture record / replay

**Ability:** R5 (W1–W2)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=5`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture
```

## Visible effect

| Side | What |
|------|------|
| **Left** | Three overlapping rects painted **live** each frame |
| **Right** | Same three rects from a **Picture recorded once**, **Replay** each frame |

Both stacks should look equivalent (red / green / blue panels).

## Gates

- `picture_op_count ≥ 3`
- `replay_frames ≥ 10`
- §2.2 schema + presents + fps
