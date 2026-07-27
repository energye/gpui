# ui_render_base_text

**Text 轴**多语言样板 — `ENGINE_UI_RENDER_BASE` §8 / §24。

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

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_render_base_text
```
