# ui_render_base_text

**Text 轴**多语言样板 — `ENGINE_UI_RENDER_BASE` §8 / §24。

## 三证据（2026-10-21）

| 证据 | 结果 |
|---|---|
| 逻辑探针 | 5/5（presents / paint / no layout storm / gpu ops / fallback=0） |
| 像素断言 | 5/5（root/card/DC panel 底色，ColorPulse 与标题墨色） |
| Golden | 整窗 **576000 px**，连续两跑 **0.0000%** |
| 自动门禁 | `-auto-only` 5s，连续两跑 exit 0；`presents≈300` |
| 人工窗 | `-manual-seconds 2` exit 0；动画保留 |

Gate 档冻结 `ColorPulse` 与 `DCPanel` 的 phase（初始色固定为 0.5/0.7/1），manual 档保留颜色脉动。整窗都是被测文本画面，没有 HUD/装饰分区。

产物：

- GPU 快照：`testdata/showcase_text.png`（800×720）
- Golden：`testdata/showcase_text_base.png`
- JSON：gate 档 stdout 输出 `wrgate` 报告；逻辑/像素字段为 `logic_probes_*` / `pixel_probes_*`

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动门禁：5s，冻结相位，采快照并逐位对比 Golden
go run ./examples/ui_render_base_text -auto-only

# 人工窗：保留动画，不写快照
go run ./examples/ui_render_base_text -manual-seconds 5
```

播种 Golden 只在新基线时执行一次，之后必须不带种子连跑两次 0.0000%：

```bash
GPUI_ACCEPT_GOLDEN=1 go run ./examples/ui_render_base_text -auto-only
go run ./examples/ui_render_base_text -auto-only
go run ./examples/ui_render_base_text -auto-only
```

`RUN_SECONDS=N` 仍可覆盖 gate 时长；`SNAP_PNG=/path.png` 可覆盖快照路径。manual 档不会覆盖 Golden。

## 字体策略（函数配置，无环境变量）

| 层 | API |
|----|-----|
| **库默认** | `LoadDefaultFace` → 平台一个常用系统字体 |
| **进程覆盖** | `text.SetDefaultFontPath("…/App.ttf")` |
| **实例（推荐 App）** | `text.NewFontResolver().SetFontFile("…").Load(16)` |
| **本示例多语** | `text.LoadMultiFace(16)`（示例自己选；失败退回库默认） |

```go
// 库默认
face, _, _ := rendering.TryLoadDefaultFace(16)

// 自己的文件（函数）
text.SetDefaultFontPath("assets/App.ttf")
// 或
r := text.NewFontResolver().SetFontFile("assets/App.ttf")
face, _, _ := r.Load(16)
```
