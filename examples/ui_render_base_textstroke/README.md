# ui_render_base_textstroke — 文本描边/沿路径真窗（A 组 §3.1 第 8 项）

对应总账：`docs/RENDER_FEATURE_COVERAGE.md` §1 #29 / §3.1 A 组第 8 项「文本描边/沿路径」。
本窗覆盖 **StrokeString / StrokeStringAnchored / TextPath（+ FillPath / StrokePath / DrawString 对照）** 一族，
性质 = **原子验证**（直调 `render` 接口验一族），不算生产接线。

窗口 1200×800（表头 + 图例 + 四卡 + 底部 HUD），卡内容固定不播放动画；HUD 不进 golden / 抓屏对照。

## 1. 四卡覆盖

| 卡 | 能力 | 画面 |
|----|------|------|
| A | `StrokeString` 线宽 4/8/16 | 三个空心 "O"：描边心落在字形轮廓上，字碗中空（只描边、从不填充）；线宽变化只改墨带宽度 |
| B | `StrokeStringAnchored` 锚点三态 | 锚点 (0,0)/(0.5,0.5)/(1,1) 各一段 "Ag"，画琥珀十字标记锚点：左上贴锚点 / 中心对齐 / 右下收在锚点 |
| C | `TextPath` → `FillPath` / `StrokePath` | 青填 "O"（字碗留孔）/ 紫描 "O"（线宽 6，中空）/ 淡黄 `DrawString(Vector)` 对照（同轮廓同位置） |
| D | `TextPath` 变换 + 分数字号 | 直立橙 "I" + 绕字形 bbox 中心旋转 90° 的绿色副本（交叉）；下方 20.2 / 20.8 两行 "MMMMMMMMMMMMMMMM"（蓝 / 粉），两档字号必须各画各的轮廓 |

每卡同时是「能力演示 + 探针位置基准」：像素探针的期望值来自调色板常量，采样点来自**同一套布局 API 实测的字形几何**
（`scene.go` `buildTextLayout` 用 `TextPath(...).Bounds()` 量；D 卡探针点由离屏 ink mask 推出），不读渲染结果反推期望。

## 2. 怎么跑

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

RUN_SECONDS=6 go run ./examples/ui_render_base_textstroke -auto-only   # 门禁：JSON 一行到 stdout，失败 exit 1
go run ./examples/ui_render_base_textstroke                            # 人工常驻（关窗结束）
go run ./examples/ui_render_base_textstroke -manual-seconds 20         # 人工限时
go run ./examples/ui_render_base_textstroke -logic-only                # 只跑值级逻辑探针（子进程用 CPU 模式）
```

- 逻辑探针由窗口自己在**子进程**里跑（`os.Args[0]` + `GOGPU_RENDER_MODE=cpu`）：GUI 栈初始化后再切软件模式不可靠，
  离屏 CPU 上下文才是纯软件契约（与同族 `ui_render_base_*` 窗一致）。
- 门禁模式（`-auto-only` / `RUN_SECONDS`）**强制**跑抓屏对照，不能关；`-screencheck=false` 只影响人工模式。

## 3. 字体策略（单 face 是硬前提）

`StrokeString` / `StrokeStringAnchored` / `TextPath` 都需要**单一 `FontSource`**：`c.face.Source()` 为 nil 时
（`MultiFace`，例如 `wrkit.EnsureUIFace` 装的 UI 字体链）直接返回 nil，画不出任何东西。

本窗因此：

1. 用 `rendering.TryLoadDefaultFace(size)` 加载系统默认字体（本机 = `/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf`）；
2. 再用 `text.NewFontSourceFromFile(path)` 单独建两个分数字号 face（20.2 / 20.8）；
3. 门禁 JSON 的 `font` 字段记录实际字体路径，启动 stderr 也打印。
4. 找不到字体时按 `needs_font` 直接 `exit 1`，不静默假绿。

**金标随机器**：字体文件换了，字形几何就变，golden 与像素 oracle 会一起移动。跨机复跑前先确认 `font` 字段一致；
不一致时按 §7 重播种并人工评审。

## 4. 证据（本轮实测）

GPU X11 1200×800 真窗，最近三次运行：

| 证据 | 结果 |
|------|------|
| 逻辑探针（CPU 离屏，值级契约） | **14/14** |
| 像素探针（GPU 快照，布局实测 + 调色板 oracle，容差 8/255 + 通道带） | **21/21** |
| Golden（表头 48px + 图例 + 四卡 430×232 = **627200 px**） | **diff = 0.0000%**（seed 后复跑两次同为 0） |
| 抓屏 vs 快照（同 627200 px，per-channel tol ±2/255） | **diff = 0.0000%** |
| GPU 路线 | `gpu_ops ≈ 4.10–4.12 万`，`cpu_fallback_ops = 0` |
| 呈现 | `present_policy = full_paint`，`presents = 360`（6s），`vsync_source = true` |
| 本机性能参考（不门禁，见 §8） | `fps_wall ≈ 54.9–55.8`，`interval_p95 ≈ 17.3–17.5 ms` |

门禁 JSON 关键字段（`ability_extra`）：`logic_pass` / `logic_probes_ok` / `pixel_probes_ok` /
`pixel_golden_diff_pct` / `pixel_golden_total_px` / `screen_check_ok` / `screen_check_diff_pct` /
`screen_check_px` / `screen_check_detail` / `text_gpu_route` / `font`。

窗里还钉了两条**缺陷回归探针**（逻辑族）：

- `stroke_damage_covers_width`：24×24 矩形 + 线宽 12 时，`FrameDamageUnion` 必须覆盖 14..50（路径 bounds 只有 20..44）；
- `textpath_fractional_sizes_distinct`：20.2 与 20.8 的 "MMMM" 轮廓墨迹跨度差必须在 1.5..3.5px（旧 key 会得 0）。

## 5. 本轮引擎修复（建窗前发现的两个缺陷）

### 5.1 描边损伤不按线宽外扩

**现象**：`Stroke()` 与 `StrokeString()` 只按 `path.Bounds()` 记损伤。24×24 矩形 + 线宽 12 的真实覆盖是 14..50，
记账却是 20..44。损伤合成只 blit `damage+pad(2)`，外缘 (14,32)/(32,14) 直接丢透明，隔离层里厚描边被切边。

**修法**（对齐 Skia `SkStrokeRec::applyToPath`：描边 bounds = 描边转填充后的路径 bounds，含 caps/miter）：

- `render/context_damage.go` 新增 `trackStrokeOutlineDamage(outline *Path)`：把 `expandStrokeToPathSpace()`
  的展开轮廓按逆 CTM 映回用户空间 AABB（旋转/剪切下保守取大），再走既有 `trackDamage`；
- `render/context_draw.go` 的 `doStroke` 在 **GPU 成功分支**与 **CPU 分支**各记一次；`Stroke()` 撤掉旧的
  path-bounds 记账；
- `render/text_entry.go` 的 `StrokeString` 撤掉旧记账（统一由 `doStroke` 管）。

### 5.2 字形轮廓缓存 key 截断分数磅值

**现象**：`OutlineCacheKey.Size` 原来是 `int16(fontSize)`，小数被截断：face 20.2 与 20.8 共用同一个 key
（同 FontID+GID），后取者命中前者轮廓。实测 20.8 的 "A" 轮廓 `maxX` 仍是 30.2（应为 ≈30.8），画面取决于取用顺序。

**修法**（对齐 Skia/FreeType 的精确字号缓存）：

- `OutlineCacheKey.Size` 改 **26.6 定点 int32**，新增 `text.FixedSize(size float64) int32`（`render/text/glyph_cache.go`）；
- 四处 key 构建点同批切换：`render/text_outline.go`、`render/scene/text.go`、`render/text/glyph_run.go`（×2）、
  `render/text/glyph_renderer.go`；删 `sizeToInt16` / `computeSizeKey`，原 `TestSizeToInt16` 改 `TestFixedSize`。

**永久钉子**：`render/stroke_damage_bounds_test.go`

- `TestStrokeDamageCoversLineWidth` — 24×24 矩形 + 线宽 12，损伤并集覆盖 14..50；隔离层四外缘像素 + 四个外部像素
  在合成后仍在；
- `TestTextOutlineCacheKeepsFractionalSizes` — 20.2 与 20.8 取到不同轮廓，墨迹跨度分别 ≈30.2 / ≈30.8。

**公开 API 变更**：新增 `render/text.FixedSize`（1 个导出函数）；`OutlineCacheKey.Size` 字段类型 `int16 → int32`
（导出字段，调用方需同步，本仓四处已一次改齐）。`go run ./scripts/apidoc` exit 0。

## 6. 抓屏 vs 快照为什么单列

**快照不是屏幕**：`SnapshotPath` 是收尾时在 GL 线程上独立重绘一帧再读回
（`ui/embedder/pipeline_app.go`）。所以「快照对 + 像素/golden 全绿」只证明重绘路径对，不证明呈现路径对。
同族的 maskops / backdrop 窗都曾出现「快照四卡正常、实屏整窗黑」的假绿。

本窗沿用同族口径：

1. `app.Run()` 返回后、`app.Close()` 之前抓屏（此时窗口还显示最后一帧；快照尚未生成）；
2. `xwininfo -name <唯一标题>` 取**客户区**绝对坐标与大小（`LC_ALL=C` 固定字段名）；
3. `ffmpeg -f x11grab -draw_mouse 0 -video_size WxH -i $DISPLAY+X,Y -frames:v 1` 抓一张 PNG 到临时目录；
4. 关窗后与快照在**同一组 golden 矩形**上比对（HUD 每帧在变，不在矩形内）；
5. 判定：每通道差 > ±2/255 的像素 ≤ 1% 为过（同一渲染路径，实测通常为 0）；抓屏失败/尺寸不符/被遮挡 → 门禁红。

要求：X11 + `xwininfo` + `ffmpeg`，窗口尺寸固定（门禁不缩放）。抓屏图落在 `$TMPDIR/gpui_textstroke_screen.png`（不入库）。

## 7. Golden 基线与重播种记录

- 基线资产：`testdata/showcase_textstroke_base.png`（表头 + 图例 + 四卡逐位，HUD 不参与）；
  每次改动画面必须**人工评审后**重播种并随窗提交。
- 重播种命令（基线缺失时）：
  ```bash
  rm examples/ui_render_base_textstroke/testdata/showcase_textstroke_base.png
  GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_render_base_textstroke -auto-only
  RUN_SECONDS=6 go run ./examples/ui_render_base_textstroke -auto-only   # ×2 确认 0.0000%
  ```
- **2026-10-19 重播种（评审记录）**：两个引擎缺陷修完后首次 seed；画面人工确认 = A 三个空心 "O"（线宽递增）、
  B 三个锚点十字、C 青填 / 紫描 / 淡黄对照、D 橙竖 "I" + 绿横 "I" 交叉 + 两行分数字号 "MMMM"。
  seed 后连跑两次门禁 `golden diff = 0.0000%`、抓屏 `diff = 0.0000%`，故接受。

## 8. 已知限界

| 限界 | 说明 |
|------|------|
| 字体绑定 | golden 与像素 oracle 都按本机系统字体实测几何生成；换字体（或系统字体升级）后必须重播种并人工评审，否则门禁红是**预期行为** |
| 单 face 前提 | `TextPath`/`StrokeString` 在 `MultiFace` 上返回 nil（`c.face.Source()==nil`）；本窗用 `TryLoadDefaultFace` + `NewFontSourceFromFile` 显式取单 face，UI 字体链不能直接套用 |
| 抓屏依赖与遮挡 | 需要 X11 + `xwininfo` + `ffmpeg`，窗口须可见且未被其它窗遮挡；不满足时门禁红是**预期行为**（不能拿快照顶替屏幕证据） |
| 帧率不在本窗门禁 | 本窗是正确性窗：`fps_gate = off_correctness`、`slope_gate = off`；fps/p95 只记录不判定（本机 ≈54.9–55.8fps / p95 ≈17.3–17.5ms，1200×800 full_paint） |
| RSS 仅记录 | 6s 短跑含 warmup，`rss_end ≈286–300MB`、`rss_slope ≈40–47 万 KB/min` 只作观察，不做门禁；长跑/soak 口径见性能窗 |
| TextPath 的公共契约 | `TextPath` 返回 nil 的两种已知情况：无 face、`MultiFace`（无单 `FontSource`）；调用方需判空 |
| 分数字号粒度 | 缓存 key 量化到 1/64 px（26.6 定点）；比 1/64 px 更细的字号差会合并，这是与 Skia 一致的取舍 |

## 9. 文件

- 窗口：`examples/ui_render_base_textstroke/main.go`（框架/探针/门禁/抓屏）、`scene.go`（四卡、布局实测、逻辑探针）、
  `testdata/showcase_textstroke_base.png`（golden，随窗提交）、`testdata/showcase_textstroke.png`（最近一次快照，随窗提交）
- 引擎修复：`render/context_damage.go`（`trackStrokeOutlineDamage`）、`render/context_draw.go`（`doStroke` 两分支记账）、
  `render/text_entry.go`（撤旧记账）、`render/text_outline.go`、`render/scene/text.go`、`render/text/glyph_cache.go`
  （`FixedSize` + key 类型）、`render/text/glyph_run.go`、`render/text/glyph_renderer.go`
- 永久回归：`render/stroke_damage_bounds_test.go`
- 真源：`docs/RENDER_FEATURE_COVERAGE.md` §1 #29 / §3.1 / §6 / §11、`docs/ENGINE_UI_RENDER_BASE.md` §23 / §24、
  `docs/RENDER_API_CATALOG.md` §3.8 / §6.1 / §7.2
