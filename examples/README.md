# examples/

L1 验收示例（与旧示例无关）。

| 目录 | 阶段 | 说明 |
|------|------|------|
| [`ui_l1_blank`](./ui_l1_blank) | P0 | 清色真窗 + JSON 指标 |
| [`ui_l1_spinner`](./ui_l1_spinner) | P3 | Spinner + 异步 present + JSON |

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_l1_blank
go run ./examples/ui_l1_spinner
```

需要 `DISPLAY`。详见 `docs/ENGINE_L1_CLOSEOUT.md`。
