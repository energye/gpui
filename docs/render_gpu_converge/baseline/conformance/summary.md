# P0.4 双后端一致性测试台：运行汇总

> 由 `render/conformance_test.go` 自动生成；`report.json` 与 `frame.png` 位于 `<scene>/<backend>/`。

回归命令（在仓库根目录执行；测试进程 cwd 是 render/，输出路径必须用绝对路径）：

```bash
WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \
GPUI_CONFORMANCE_OUT=$PWD/docs/render_gpu_converge/baseline/conformance \
  go test ./render -run '^TestConformanceDualBackend$' -count=1 -v
```

负向对照（改错期望值必须转红）：

```bash
WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \
GPUI_CONFORMANCE_OUT=$PWD/docs/render_gpu_converge/baseline/conformance \
GPUI_CONFORMANCE_MUTATE=solid_gradient:linear_left \
  go test ./render -run '^TestConformanceDualBackend$' -count=1 -v
```

| 场景 | native | go |
|---|---|---|
| clip_blend | PASS | PASS |
| solid_gradient | PASS | PASS |
| text_image | PASS | PASS |

## 已知差异（用例数据显式放宽，非静默跳过）

| 场景 | 后端 | 放宽项（实测/上限） | 回退原因 |
|---|---|---|---|
| solid_gradient | native | 账本 countΔ 2/2；账本 bytesΔ 131072/131072 |  |
| text_image | native | 账本 countΔ 2/2；账本 bytesΔ 131072/131072 |  |
| solid_gradient | go | 账本 countΔ 2/2；账本 bytesΔ 131072/131072 |  |
| text_image | go | 账本 countΔ 2/2；账本 bytesΔ 131072/131072 |  |
