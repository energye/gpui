# ui_render_base_image

**Image 轴**窗测 — `ENGINE_UI_RENDER_BASE` FImg-* / §24。

## 覆盖

| 项 | 方式 |
|----|------|
| FImg-DRAW | `RenderImage` sync `SetImage` |
| FImg-CODEC | `ui/io.DecodeFile` 工作线程解码 → ticker hop 上 UI |
| FImg-RECT | `DrawImageEx` + `SrcRect` |
| FImg-ROUND / CIRCLE | `DrawImageRounded` / `DrawImageCircular` |
| FImg-NINE | `DrawImageNine` |
| 指标 | layout 不因换图风暴；JSON RSS/CPU/fps |

**不做：** GIF 多帧、TextureLayer、P6 dirty Present。

## 三证据（2026-10-21）

| 证据 | 结果 |
|---|---|
| 逻辑探针 | 8/8（presents / paint / no layout storm / decode / swap / status=ready / gpu ops / fallback=0） |
| 像素断言 | 8/8（根底、同步棋盘 blue/yellow、异步渐变、DC 底、DC 图、pulse 条、标题墨色） |
| Golden | 整窗 **499200 px**，连续两跑 **0.0000%** |
| 自动门禁 | `-auto-only` 5s，连续两跑 exit 0；`presents≈300` |
| 人工窗 | `-manual-seconds 2` exit 0；动画保留 |

Gate 档冻结 pulse 相位，但保留真实异步解码：`async_decode_callbacks=1`、`swaps=1`、`status=ready`。由于首帧后异步解码会造成一次可观测 hitch，本窗**不设 p95 硬门禁**；逻辑门禁只看 decode/swap/status 与画面三证据，避免用短窗 p95 假绿或误杀。

产物：

- GPU 快照：`testdata/showcase_image.png`（780×640）
- Golden：`testdata/showcase_image_base.png`
- JSON：gate 档 stdout 输出 `wrgate` 报告；逻辑/像素字段为 `logic_probes_*` / `pixel_probes_*`

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动门禁：5s，冻结 pulse，异步解码仍真实执行
go run ./examples/ui_render_base_image -auto-only

# 人工窗：保留动画，不写快照
go run ./examples/ui_render_base_image -manual-seconds 5
```

播种 Golden 只在新基线时执行一次，之后必须不带种子连跑两次 0.0000%：

```bash
GPUI_ACCEPT_GOLDEN=1 go run ./examples/ui_render_base_image -auto-only
go run ./examples/ui_render_base_image -auto-only
go run ./examples/ui_render_base_image -auto-only
```

`RUN_SECONDS=N` 仍可覆盖 gate 时长；`SNAP_PNG=/path.png` 可覆盖快照路径。manual 档不会覆盖 Golden。
