# kit spin_min — single spinning icon (no chrome; stderr metrics + JSON)

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 go run ./examples/kit/spin_min
```

窗口 480×320，中央一个 64px 旋转的 sync 图标，其余全静。`RUN_SECONDS`
定时跑；结束时 stderr 打印呈现数 / 完整指标 / 进程 CPU%，stdout 打印同一份
指标 JSON（帧间隔、hitch、RSS 峰值与稳态斜率、gpu_ops、present_mode）。

门禁：必须出帧（`presents≥1`），`--auto-only` 默认开（离屏/无 GPU 窗直接 FAIL）；
`SPIN_SNAP=路径` 可另存末帧 PNG 目检。示例只调 kit 公开 API，行为全在 `ui/kit`。

60s 档的 `rss_slope_valid=true` 依赖 ticker 每帧 `proc.Sample()` 采 RSS 序列
（缺了它只有首尾两点，稳态斜率永远 `valid=false`）。
