# zz_soak_control

- 来源：`windows.md` §3.6（demo）
- 目录：`examples/zz_soak_control`
- 运行：`批量(3 窗) R=180s`（mode=R，seconds=180，batch=True）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：帧时/窗口 · 多窗小损伤对照

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | NO_JSON | 0 |  |  |  |  | 0 | 0 | no parseable JSON on stdout |
| default | go | run | NO_JSON | 0 |  |  |  |  | 0 | 0 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：no parseable JSON on stdout; CTRL win1 presents=10229 ~56.7 fps avg=17.60 p50=17.50 p95=20.04 hitches=61 raster=16.92 flush=0.48 acquire=1.95 presentWait=0.12 rerecord=15 skip=81800; CTRL win2 presents=10226 ~56.7 fps avg=17.61 p50=17.46 p95=19.91 hitches=66 raster=5.46 flush=0.35 acquire=1.40 presentWait=0.17 rerecord=15 skip=81776; CTRL win3 presents=10250 ~56.8 fps avg=17.57 p50=17.49 p95=19.71 hitches=33 raster=6.45 flush=0.41 acquire=0.25 presentWait=0.08 rerecord=15 skip=81968
- `default` / `go` / `run`：no parseable JSON on stdout; CTRL win1 presents=10357 ~57.4 fps avg=17.38 p50=17.32 p95=18.09 hitches=4 raster=0.39 flush=0.24 acquire=0.00 presentWait=0.10 rerecord=15 skip=82824; CTRL win2 presents=10357 ~57.4 fps avg=17.38 p50=17.35 p95=18.20 hitches=6 raster=18.20 flush=0.17 acquire=0.00 presentWait=9.98 rerecord=15 skip=82824; CTRL win3 presents=10356 ~57.4 fps avg=17.39 p50=17.35 p95=18.02 hitches=3 raster=2.20 flush=0.33 acquire=0.01 presentWait=6.07 rerecord=15 skip=82816

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：帧时/窗口 · 多窗小损伤对照）。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

