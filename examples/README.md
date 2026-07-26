# examples/

L1 验收示例（与旧示例无关）。

| 目录 | 阶段 | 说明 |
|------|------|------|
| [`ui_l1_blank`](./ui_l1_blank) | P0 | 清色真窗 + JSON 指标 |
| [`ui_l1_spinner`](./ui_l1_spinner) | P3 | Spinner + 异步 present + JSON |
| [`ui_l1_scroll`](./ui_l1_scroll) | P4 | VirtualList 1k 行 + 自动滚动；**窗口可缩放，列表随 client 尺寸 layout** |

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 默认各跑 60 秒；看卡帧可加长：
RUN_SECONDS=180 go run ./examples/ui_l1_blank
RUN_SECONDS=180 go run ./examples/ui_l1_spinner
RUN_SECONDS=180 go run ./examples/ui_l1_scroll
```

- 默认时长 **60s**；环境变量 **`RUN_SECONDS`** 覆盖（无 `MaxFrames` 上限，按时长结束）。
- 结束时 stderr 打印 `avg/max/last` 帧间隔与 `hitches(>33.4ms)`；stdout 为完整 JSON（含 `hitch_count`、`max_frame_interval_ms`）。
- 需要 `DISPLAY`。详见 `docs/ENGINE_L1_CLOSEOUT.md`。
