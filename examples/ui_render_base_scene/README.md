# ui_render_base_scene — render/scene 直调真窗

B 组「Scene（render_scene / render_scene_gpu）」的真窗。同一份 `scene.Scene` 分别走 GPU 场景渲染器与 CPU 分块渲染器，用逐点探针和 A/B 像素对比证明两个后端语义一致；另外覆盖 `Encoding.Hash` + `LayerCache`、`TaggedBounds` + `DamageTracker` 的局部重绘，以及场景文本与图像。

## API 覆盖

| 族 | 用到的公开 API |
|---|---|
| 构建 | `NewSceneBuilder`、`FillRect/FillRoundRect/StrokeCircle/Image/Layer/Clip/WithTransform`、`Build` |
| 场景/编码 | `Scene`、`Scene.Encoding/Flatten/Bounds/Version/TaggedBounds/Append/AppendWithTranslation/DrawText`、`Encoding.Tags/Brushes/Hash/CommandBounds/Clone`、`NewDecoder` 全量回放 |
| 渲染 | `NewGPUSceneRenderer.RenderScene`（GPU）、`NewRenderer/MarkAllDirty/RenderDirty/Render`（CPU 分块）、`WithWorkers` |
| 图层 | `BlendNormal/BlendMultiply`、`Layer(blend, alpha, clip, fn)`、`TagPushLayer/TagPopLayer` |
| 裁剪 | `NewCircleShape/NewRectShape` + `SceneBuilder.Clip`、`TagBeginClip/TagEndClip` |
| 缓存 | `NewLayerCache`、`Put/Get/Stats`、`Encoding.Hash` 作 key |
| 损伤 | `NewDamageTracker`、`Renderer.RenderWithDamage`、`Scene.TaggedBounds`、`Encoding.RecordCommandBounds` |
| 文本/图像 | `Text.Face`、`Scene.DrawText`、`Scene.Image`/`NewImage`/`DrawImage`、`ImageBufFromImage` |

四张卡（1200×800，2×2）：

- **A** 同一场景直接由 `scene.NewGPUSceneRenderer(dc).RenderScene` 画到窗口：背景、multiply 图层、50% alpha 图层、旋转方块、圆裁剪、矩形裁剪内的圆角矩形、圆描边。
- **B** 同一场景用 CPU 分块渲染器（64×64 tile）先渲染到 `render.Pixmap`，再 `DrawImageEx` 上屏；与 A 做容差像素对比。
- **C** 两个只有实色画刷不同的场景分别 `Encoding.Hash` 入 `LayerCache`，再用 `Get` 取出显示；修 Hash 不含 brushes 前两张缩略图会撞成同一张。
- **D** 文本 + PNG 图像 + 移动矩形：第一帧全量、第二帧只重绘移动矩形并集、第三帧无变化；增量结果与全量重渲染逐字节相同，旧位置必须被清掉。

## 同批修的引擎缺陷

1. **CPU 分块渲染丢弃图层 alpha/混合模式**：`TagPushLayer` 的 alpha 被 `_ =` 掉，50% 图层按不透明合成，multiply 也不生效。现在每个图层渲染到独立 pixmap，`PopLayer` 时按 `BlendMode + opacity` 合成（支持嵌套与未闭合图层的安全清栈），与 GPU `PushLayer/PopLayer` 语义对齐。永久回归 `TestCPUTileLayerOpacity`。
2. **`TagFillRoundRect` 绕过裁剪且不处理变换/瓦片原点**：圆角矩形直接填整块 tile 画布，越过当前裁剪；`Path.Transform` 返回新路径，旧代码忽略返回值，多瓦片时每块都从场景原点画起（缩略图出现竖向空洞）。现在先按当前变换与 tile 原点变换路径，再 `Fill` 到当前（可能是裁剪临时层的）pixmap。永久回归 `TestCPUTileRoundRectRespectsClip`、`TestCPUTileRoundRectMultiTile`。
3. **`Encoding.Hash` 不含 brushes**：几何相同、颜色不同的场景 Hash 相同，`LayerCache` 会把红色缩略图当蓝色缓存返回。现在 Hash 纳入 brush 的 Kind 与 RGBA。永久回归 `TestEncodingHashIncludesBrushes`。
4. **`Scene.TaggedBounds` 不展开根图层**：`SceneBuilder` 录制的内容在 root layer encoding 里，`TaggedBounds` 直接读 `s.encoding` 得到空列表——`DamageTracker` 看不到任何对象，移动矩形永远报零损伤。现在先 `flattenLayers()` 再取 command bounds。永久回归 `TestEncodingCommandBoundsSurviveFlattenAppendClone`。
5. **`Encoding.Append/AppendWithTranslation/Clone` 丢 commandBounds**：图层展开、RepaintBoundary 场景拼接、克隆路径都会让逐命令包围盒消失，损伤链再次断掉。现在 append 时按偏移搬运 commandBounds，Clone 深拷贝。同上回归。
6. **`TagTransform` 状态泄漏**：编码器只在非单位变换时写 `TagTransform`，一次旋转/平移后，后续单位变换的裁剪与填充都继续套用旧变换——圆裁剪和圆角裁剪整体画到别处，GPU/CPU 双后端一起错。现在 `EncodeDrawTransform` 跟踪流内当前变换，单位变换也会在需要时补写重置；`Append/AppendWithTranslation` 在拼接边界插入 identity 重置。永久回归 `TestSceneDrawTransformResetsAfterTransformedDraw`、`TestEncodingAppendResetsStaleTransform`。

## 取证

自动档（`RUN_SECONDS=5 -auto-only`，三次复跑）：

- 逻辑 `21/21`：`SceneBuilder` 产出的 tag 族（Fill/Stroke/FillRoundRect/Layer/Clip/Transform）、场景 bounds/version、编码全量解码回环、`Flatten` 克隆 Hash 一致、带位移 Append 的 bounds、brush 敏感/稳定 Hash、缓存两色不串 + 命中统计、CPU 图层 alpha、multiply、圆角裁剪内外、圆裁剪内外、首帧全量损伤、移动帧最小损伤 `(20,138)-(214,178)`、增量 SHA = 全量 SHA、无变化帧零损伤、文本墨量、图像四象限、空场景渲染。
- 像素点 `17/17`（A/B 各 5 点 + C 2 点 + D 5 点），墨量区域 `1/1`。
- A/B 后端对比：`mismatch=1.4376%`（容差 10/255，门限 2%）、`high=0.0259%`（>96/255，门限 0.1%）、`worst=111`；热点集中在旋转方块/圆/圆角的 1–2px AA 边缘（逐 32×32 格统计最大 128 px，无成片实心区），属于两种光栅器的覆盖采样差，不是语义差。最差 delta 只报告不设门，避免把 AA 边缘当语义错误。
- Golden：表头 + 图例 + 四卡，`627200 px` 逐位 `diff=0.0000%`（seed 后两次复跑均为零差）。
- `presents=300`、`fps_interval≈60.02`、`interval_p95≈18.71ms`、`hitch_count=0`、`gpu_ops=15003`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`、`present_policy=full_paint`、`vsync_source=true`。

人工常驻档（`-manual-seconds 15`，退出 0）：

- `presents=900`、`fps_interval≈59.95`、`interval_p95≈19.16ms`、`hitch_count=1`、`gpu_ops=45003`、`cpu_fallback_ops=0`。
- 逻辑/像素/墨量/Golden/parity 与自动档同绿。

字体 `SourceSans3-Regular.otf` SHA-256 前缀 `08df266400933d31`；图像 `scene_quad.png` SHA-256 前缀 `51dbe2872c3e63e2`。

## 已知限界

- **原子验证，无生产消费者**：`render/scene` 仍无组件/2.5D/视频调用；本窗是直调渲染接口一次验一族，不算生产接线。
- **A/B 只比 CPU 分块 vs GPU 场景路径**：两者共享同一编码，能抓图层、裁剪、变换、画刷语义差；不覆盖 GPU 设备丢失/驱动差异，也不替代 GPU stencil/depth 裁剪的独立压测。
- **损伤链只验 CPU 分块路径**：`RenderWithDamage` 输出损伤矩形后，本窗验证的是 tile 重绘与全量逐字节一致；GPU scissor / OS 级增量呈现（FrameDamage→PresentWithDamage）不在本窗范围。
- **资源债**：自动档 `rss_start≈82.2MB → rss_end≈287–291MB`、`rss_slope≈130,000–248,000 KB/min`（多次波动）；人工 15s `82.2MB → 298.3MB`、`slope≈73,656 KB/min`，`rss_after_close` 仍约 298MB。与 Pattern/VF/Recording 窗同量级，先按跨窗资源债记入 `RENDER_FEATURE_COVERAGE.md` §6，显存/堆账本留 §7/§8 统一处理。
- **未做抓屏 vs 快照**：本窗只有自动快照 + 人工常驻运行，没有把 `xwd/x11grab` 抓屏写进门禁。
- **缓存卡只验单线程 `LayerCache`**：不覆盖缓存淘汰/并发 Get 的竞争场景；`maxSizeMB=8` 足够放两张 176×150 缩略图，不做逐出压力。
- **文本只验一条 Latin 字形 run**：CJK/Color/复杂脚本属于文本子系统各窗，本窗只证明 `Scene.DrawText → Encoding → GPU/CPU 解析` 这条链有墨。
