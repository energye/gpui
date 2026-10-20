# ui_render_base_miscdraw — 杂项绘制五 API 真窗

A 组第 15 项（`RENDER_FEATURE_COVERAGE` §1 #42）的独立真窗。把 `render.Context` 的五个杂项绘制入口摆到画面上：

- `DrawShadow(path, col, elevation, transparentOccluder)` — 路径海拔阴影
- `DrawColor(col, mode)` — 按指定混合模式填满当前裁剪
- `DrawPaint()` — 用当前 Paint 填满当前裁剪
- `ClipBounds()` — 当前裁剪的本地包围盒
- `DrawParagraph(layout, x, y)` — 按预排版逐行/逐 run 绘段落

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 门禁：5 秒脚本跑，stdout 出 §2.2 JSON，失败退出 1
RUN_SECONDS=5 go run ./examples/ui_render_base_miscdraw -auto-only

# 人工常驻：开窗直到手动关闭（事件写日志并改标题）
go run ./examples/ui_render_base_miscdraw

# 人工 + 15 秒自动收尾
go run ./examples/ui_render_base_miscdraw -manual-seconds 15
```

Golden 首次播种（只在基图不存在时生效，播种后必须复跑确认 `0.0000%`）：

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_miscdraw -auto-only
RUN_SECONDS=5 go run ./examples/ui_render_base_miscdraw -auto-only
```

## 画面（1200×800，五卡）

每张卡的内容先通过真实 `render.Context` 烘焙成图像，再由窗口每帧 `DrawImageEx` 直绘；指标行浮在内容左上角，Golden 时被盖住。

| 卡 | 内容 | 证明什么 |
|---|---|---|
| A DrawShadow | 矩形/圆/三角三个遮挡体，elevation 8/16/24；只画投影，彩色遮挡体另画 | 阴影沿下方偏移、软边随海拔变宽；遮挡体内部不被阴影盖住；elevation≤0 与 nil 路径 no-op |
| B DrawColor | 四条纯色竖带 + 顶部 Multiply 灰带 + 底部 Screen 蓝带 + 圆角白块 | `DrawColor` 忽略当前画刷 alpha/blend；只填当前裁剪（含圆角）；两次高级混合顺序生效，第一次不丢 |
| C DrawPaint | 两个裁剪区分别用线性渐变 `DrawPaint`；先摆一条当前路径再 DrawPaint，最后 `Fill` | `DrawPaint` 用当前 Paint 填满 clip；当前路径与 Paint 状态保留，后续 `Fill` 仍画原路径 |
| D ClipBounds | 矩形裁剪 / 嵌套裁剪 / 20° 旋转路径裁剪；每态画橙色填充 + 用 `ClipBounds()` 画蓝/绿/红边框 | 无裁剪=整画布；嵌套=交集；路径裁剪=本地保守 AABB（含控制点），不再返回全画布 |
| E DrawParagraph | 左侧英文三行、右侧中英混排右对齐三行；MultiFace 18px | `DrawParagraph` 逐行按 `line.Y` 基线、逐 run 按 Face/字形位置绘制；三段字形数 36/34/10，空白处不落墨 |

Golden 只比表头 + 图例 + 五卡（737760px），逐位零容差。

## 证据

- 逻辑 14/14（子进程 `GOGPU_RENDER_MODE=cpu` 离屏值级）：nil 安全、`DrawColor` 填裁剪与 Copy、`DrawPaint` 渐变填 clip、`DrawPaint` 保留路径、`ClipBounds` 无裁剪/嵌套/变换/路径四态、段落行数与原点、阴影软边不盖遮挡体、elevation no-op、spot alpha。
- 像素 17/17（GPU 快照）：A 卡阴影形/软边/上方近白；B 卡 Multiply `(229,27,22)`、中间纯红 `(255,30,25)`、Screen `(255,98,209)`、圆角内外；C 卡渐变左右/路径绿/路径外底色；D 卡四态各一点；E 卡左右墨量与空白。容差 8–23/255，按探针显式声明。
- Golden：表头 + 图例 + 五卡 `737760px`，`diff=0.0000%`。
- 抓屏：`xwininfo` 取窗口 id + `xwd -id`/`ffmpeg` 读窗口自身像素，与快照同 `737760px` 对照 `diff=0.0000%`（per-channel tol ±2/255）。
- GPU 路线：五卡烘焙路由 `A 13/0`、`B 7/0`、`C 3/0`、`D 20/0`、`E 6/0`（`gpu_ops/cpu_fallback_ops`）；窗口整体 `gpu_ops=39003`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`、`presents=300`。
- 门禁 JSON：`present_policy=full_paint`、`vsync_source=true`、`presents=300`、`fps_gate=off_correctness`（正确性窗，不拿 fps 当及格线）。

## 同批修的六处引擎缺陷

1. **连续高级混合层丢第一次合成**（`render/internal/gpu/gpu_render_context.go`、`render/context_layer_blend.go`）
   第二次图层自 flush 时，上一次待解层被当成本层目标消费；B 卡先 Multiply 后 Screen，顶部灰带曾完全消失。修法：每个待解层记录 `dstView`（父层 RT；零=基面），`FlushOffscreen` 只解目标为当前层 RT 的 pop，父表面 pop 留到父/呈现 flush；父隔离层退出时若自身没有待画命令但仍有子层指向它，先用 `HasPendingAdvancedFor` 触发一次父 RT flush 再释放；效果发布 RT 走 `FlushOffscreenBase`，基面 pop 会解进发布纹理；同时禁止离屏自 flush 切 `frameScratch`，否则递归。
2. **MultiFace 段落整形 0 字形**（`render/text/layout.go`）
   `MultiFace.Source()==nil`，HarfBuzz 直接整形返回 0 个字形，E 卡整张空白。修法：`shapeSegments` 先按 `MultiFace.Runs` 切成逐 face 连续段再整形，cluster 偏移用 `seg.Start+part.start`；RTL 仍走旧单 face 路径（见限界）。
3. **`ClipBounds` 对路径裁剪报全画布**（`render/internal/clip/stack.go`）
   `MaskClipper` 分配全画布覆盖缓冲，`mask.Bounds()` 是画布不是路径 AABB。修法：`PushPath` 用路径坐标 AABB（控制点含在内）与当前界相交；mask 覆盖仍全画布。
4. **HiDPI 图层 RT 按逻辑尺寸分配**（`render/context_layer_stack.go`）
   高分屏下隔离层只覆盖左上逻辑象限，右侧/下方被裁。修法：`pushLayerSurface` 用 `pixmap.Width()/Height()` 物理尺寸，与基面一致。
5. **模糊/投影未标全量合成**（`render/filter_ops_cpu.go`）
   `ApplyBlur`/`ApplyBlurXY`/`ApplyDropShadow` 改写了像素但只按原 damage+pad 复合，模糊/投影尾部被裁。修法：三条路径（GPU 成功与 CPU 回退）都补 `markLayerFullComposite()`。
6. **CPU 模式仍创建 GPU 上下文**（`render/context_gpu.go`）
   `GOGPU_RENDER_MODE=cpu` 文档承诺全部走软件光栅，但 `ensureGPUCtx` 仍装 GPU 上下文，`PushLayerIsolated`、`DrawShapedGlyphs`、图像/滤镜继续排队 GPU，CPU 契约直读 pixmap 时看不到像素。修法：`ensureGPUCtx` 最前按 `CPUOnlyMode()` 返回，CPU 模式下离屏层走 CPU 池、结构化字形走轮廓回退。

永久回归：

- `render/context_misc_draw_test.go` — CPU 契约（五 API 状态/几何/混合，含 `TestDrawColorAdvancedBlendUsesLayer`）
- `render/context_misc_draw_gpu_test.go` — GPU 五 API + 连续两层高级混合 + 隔离父层内高级混合 + 效果面高级着色（`GPUOps>0`、`CPUFallbackOps==0`）
- `render/text/layout_test.go` — `TestLayoutText_MultiFace`（逐 face 分段与 cluster 偏移）

## 限界（`RENDER_FEATURE_COVERAGE` §6）

1. **`DrawShadow` 的 `transparentOccluder` 当前不参与绘制**：参数为签名兼容保留，传 true/false 画面一致；需要透明遮挡时调用方先自行合成遮挡体。
2. **`DrawParagraph` 只按 `text.Layout` 逐行/run 画单色字形**：不接管 Paragraph 样式/装饰/省略号/命中模型；run 级颜色与下划线不生效。RTL 段与 `MultiFace` 同时出现时仍退回整段单 face 整形，本窗 E 卡用非 RTL 多 face 段落验证。
3. **高级混合 `DrawColor`/`DrawPaint` 每次调用开一张全画布隔离层**：普通模式零层直填；整屏频繁高级着色时可合并为一次大层或改用预乘纹理。
4. **仍是原子验证**：本窗直接调 `render.Context`，只证明能力本身；五个 API 目前没有组件库或 2.5D/视频模块的生产调用方。

## 性质

原子验证：直接调 `render.Context` 公开接口，一次验五个杂项绘制 API。示例不算生产接线。
