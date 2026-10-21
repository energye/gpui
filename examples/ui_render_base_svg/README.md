# ui_render_base_svg — SVG 图标栅格化真窗

C 组 `render/svg` 项的独立真窗。此前整包只有单测、没有真窗，也没有生产消费者；
本窗把 `svg.Render` / `svg.RenderWithColor` 生成的图标经 `ImageBufFromImage` 上传，
再用正常 UI/GPU 图像路径 `DrawImageEx` 画到四张卡上，同时修掉两个已知语义 bug：

- 根元素 `fill="none"` 被 SVG 默认黑色填充覆盖（祖先 none、子元素无 fill/stroke 时）。
- 根 `width` / `height` 百分比被剥掉 `%` 当绝对数用（`50%` 错成 `50`）。

子元素几何里的百分比长度不在这个图标子集内；现在会明确报错，而不是静默当 0。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 门禁：5 秒脚本跑，stdout 出 §2.2 JSON，失败退出 1
RUN_SECONDS=5 go run ./examples/ui_render_base_svg -auto-only

# 人工常驻：开窗直到手动关闭（事件写日志并改标题）
go run ./examples/ui_render_base_svg

# 人工 + 15 秒自动收尾
go run ./examples/ui_render_base_svg -manual-seconds 15
```

Golden 首次播种（只在基图不存在时生效，播种后必须复跑确认 `0.0000%`）：

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_svg -auto-only
RUN_SECONDS=5 go run ./examples/ui_render_base_svg -auto-only
```

## 画面（1200×800，2×2 四卡）

每张卡先画卡片底色，再把 16×16 的 SVG 渲染成 256×256 RGBA，缩放到 180×180 画在卡内；
像素探针按同一份坐标常量取样。

| 卡 | SVG 内容 | 证明什么 |
|---|---|---|
| A | 红色填充圆 | 最基础的 `svg.Render` → `ImageBufFromImage` → GPU `DrawImageEx` 路线；圆心红、角透明透卡底 |
| B | 根 `fill="none"` + 蓝色描边圆 | `none` 继承不再被黑填充覆盖；圆心必须是卡底，圆环必须是蓝色 |
| C | `<g fill="#3CCF7A" transform="translate(2 2)">` 内两个 4×4 方块 | 嵌套组变换被正确遍历；两块绿方块跟随平移，中间空隙保持卡底 |
| D | 白色填充圆 + `RenderWithColor(紫色)` | 主题色覆盖只替换非 `none` 颜色；圆心紫色、角透明透卡底 |

Golden 只比表头 + 图例 + 四卡（627200px），逐位零容差。抓屏用 `xwininfo` 取窗口
id + `xwd -id` 读窗口自身像素（`ffmpeg` 转 PNG），与快照同区对比，per-channel
tol ±2/255。

## 证据

- 逻辑 7/7（子进程 `GOGPU_RENDER_MODE=cpu` 离屏值级）：
  `fill_none_inherited` / `root_percent_dims` / `filled_center` /
  `stroke_center_transparent` / `override_keeps_none` / `nested_transform` /
  `invalid_xml_rejected`。
- 像素 9/9（GPU 快照）：A 圆心/角；B 圆心/圆环；C 第一块/第二块/空隙；
  D 圆心/角。
- Golden：表头 + 图例 + 四卡 `627200px`，`diff=0.0000%`。
- 抓屏 vs 快照：同 `627200px`，`diff=0.0000%`（tol ±2/255）。
- GPU 路线：`gpu_ops=26703`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`、
  `presents=300`；`present_policy=full_paint`、`vsync_source=true`；
  `fps_gate=off_correctness`（正确性窗，不拿 fps 当及格线）。
- 门禁 `-auto-only` 复跑退出 0；人工档 `-manual-seconds 8` 退出 0：`presents=480`、`gpu_ops=42723`、`cpu_fallback_ops=0`，逻辑/像素/Golden/抓屏全绿。

## 同批修的 SVG 缺陷

1. **`fill="none"` 继承被黑填充覆盖**（`render/svg/renderer.go`）
   `fillAndStroke` 的默认填充兜底只看当前元素的原始 `a.Fill` / `a.Stroke`，没看
   `state.parentFill` / `parentStroke`。祖先设 `fill="none"`、子元素不写 fill/stroke
   时，`shouldFill` 已经是 false，但兜底又强制 `hasFill=true`，画成黑块。
   修法：兜底改为检查解析后的有效值 `resolveFill` / `resolveStroke`；只有两者都为空
   时才使用 SVG 初始黑色填充。

2. **根 `width`/`height` 百分比错解**（`render/svg/parser.go`）
   `parseDimension` 直接 `TrimSuffix("%")` 后当绝对数解析。修法：先读 `viewBox`，
   再按参考轴解析百分比（`50%` × 16 = 8）；`parseDimension` 接收参考值。
   同时给 `<circle>/<rect>/<ellipse>/<line>` 的几何属性换成 `parseLength`：非法的
   数值和子元素百分比不再静默吞成 0，而是返回带元素/属性名的错误。

永久回归（`render/svg/svg_test.go`）：

- `TestRenderInheritedFillNoneWithoutStroke` — 根 `fill="none"` + 无 fill/stroke 子元素，
  16×16 全透明。
- `TestParseRootPercentDimensions` — 根 `width="50%" height="25%"`、viewBox 16×8
  解析为 8×2。
- `TestParseChildPercentLengthRejected` — 子元素 `width="50%"` 明确报错。
- `TestParseInvalidRootDimensionRejected` — 根 `width="wide"` 明确报错。

## 限界

1. **仍是图标子集**：不支持 `defs` / `use` / 渐变 / `clipPath` / `preserveAspectRatio`；
   `url(#grad)` 填充仍会落黑，非均匀拉伸不做等比内嵌。
2. **子元素百分比不支持**：根 `width`/`height` 百分比已按 viewBox 解析；子元素几何
   百分比会报错，不做静默降级。常见图标使用绝对坐标，不受影响。
3. **本窗是原子验证**：直接调 `render/svg` 公开接口，只证明 SVG 解析/栅格化能力；
   目前没有组件库或 2.5D/视频模块把它当生产图标加载器用。
4. **烘焙发生在窗口进程**：四张 256×256 图标只在启动时渲染一次，之后每帧走
   `DrawImageEx`；窗口的 `gpu_ops` 证明呈现路线，不证明每帧都在跑 SVG 解析。

## 性质

原子验证：直接调 `render/svg` 公开接口，一次验一族图标解析/栅格化路径。示例不算
生产接线。
