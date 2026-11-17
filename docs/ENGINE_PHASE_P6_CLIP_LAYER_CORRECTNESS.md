# P6 合成正确性卡：视口圆角裁剪 + 隔层组透明

日期：2026-11-17 ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui` ｜ 分支：`feat/UI-组件`
状态：**问题 1（视口内圆角裁剪丢失）已修；问题 2（滤镜层之后的隔层组透明失效）已定位到最小复现，未修**。
关联：`ENGINE_NODE_RECORD_COVERAGE_PLAN.md`（验收窗暴露）、`ENGINE_PHASE_P6_DAMAGE_COMPOSITE.md`（合成侧总卡）、
`ENGINE_UI_RENDER_BASE.md` §13.1（母表登记）。

## 0. 一句话

`ui_wr_record_coverage` 冻结帧里有两处肉眼可见的错像素：视口里的圆角块被裁成直角、模糊层之后的隔层透明度变实心。
圆角根因已定位并修复——**画布平移被当成斜切，圆角裁剪被降级成直角 quad path**；隔层透明缩到了最小复现
（模糊层 → 透明度 A → 透明度 B：A 对、B 实心、C 又对），**根因未定，未盲修**。

## 1. 问题 1（已修）：视口中的圆角裁剪丢失

**现象**：`examples/ui_wr_record_coverage` 冻结帧圆角块左上角 (58,262) 是填充色 (140,76,229)，应为页面白。
最小窗对照：同一个 `RenderClipRRect` 单独摆时角是白的；套进 `RenderViewport` 后角变填充色。
（`RenderViewport` 是唯一「画布 Translate + 内容原点归零」的节点。）

**复现**：`render/clip_translate_regression_test.go`（CPU，确定性）——
`dc.Translate(40,30); dc.ClipRoundRect(0,0,100,100,24)`，修复前裁剪栈退化为 path（半径丢失）、角点填充；
修复后 `IsRRectOnly=true`、`RRectBounds().Radius=24`、角白/心红。

**根因**：`render/context_clip.go` 的圆角裁剪分支写成
`if tm.B != 0 || tm.C != 0 { deviceQuadClipPath(...) }`。
`render.Matrix` 约定 `x'=A*x+B*y+C; y'=D*x+E*y+F`：**C 是 x 平移**，B/D 才是斜切项。
于是任何 x 平移（视口内容本地原点）都被当成「旋转/斜切」，走四角 quad 路径——quad 是直角矩形，半径整块丢失。
同类判定共 4 处：`render/clip_op.go`、`render/context_clip.go`（两处）、`render/recording/recorder_path_draw.go`。

**标准**：Skia `SkMatrix::isScaleTranslate()` 只排除 skew/rotate/perspective，**翻译不算**；
Flutter 的 rrect clip 在纯平移下保持 analytic RRect（scissor + SDF），不降级为路径。

**修复**：四处统一改为 `tm.B != 0 || tm.D != 0`（只查线性部分的斜切项）；平移保持 RRect/SDF 快路。

**证据**：
- 单测 `TestClipRect_TranslateStaysRect` / `TestClipRoundRect_TranslateKeepsRadius`：修前红（rect clip 退化为 path；
  rrect 半径丢），修后绿。
- 最小窗（修前/修后同一场景）：视口内圆角块角点由填充色 (229,38,38) 变白 (255,255,255)，中心仍红。
- 验收窗冻结帧 (58,262) 由 (140,76,229) 变白；重播种前 Golden 差 0.0592%（568px，正好是圆角面积）。
  **已重播种**（2026-11-17，单窗两轮）：播种轮 `SEEDED`、像素探针 8/8、逻辑 8/8、scrollRR=0 / skip=113、
  animRR=0；复核轮 `OK`、Golden **0.0000%（960000px）**、skip=114；两轮均正常自关、Xorg 存活。

## 2. 问题 2（未修，已归位）：模糊层之后隔层组透明失效

**现象**：验收窗冻结帧透明度块 (116,340) 是纯蓝 (38,89,229)，期望 0.5 组透明
（0.5×蓝 + 0.5×白 = (147,172,242)）。

**最小复现**（临时窗，已删；数据留档）：400×1200 长页、viewport 滚动 600：
1. `RenderImageFilter(4)`（模糊，占一层）→ `RenderOpacity(0.5)` A → `RenderOpacity(0.5)` B → C；
   实测 A 半蓝、**B 纯蓝**、C 半蓝（隔层交替）。
2. 把模糊换成 `RenderGrayscale`（颜色矩阵层）：B 正确。
3. 去掉模糊层：A/B/C 全正确。
4. 4 个连续透明度层（无模糊）：全正确。

**已排除**：
- 录制命令序列平衡：页面录制 6 个 `PushLayerIsolated` / 6 个 `PopLayer`，顺序正确。
- A/B/C 三个 0.5 层都走 `canGPUComposite`，调用 `DrawGPUTextureWithOpacityUV(op=0.5, mul=1)`，层 view 指针互不相同。
- CPU 隔离路径（`PushLayerIsolated(0.5)` + 填充）像素正确；同场景单测不重现（GPU-only）。

**怀疑点（下一步验证）**：`ApplyBlur` 的 GPU filter graph（`tryApplyFilterGraphGPU`）之后，纹理合成 pipeline 的
blend state 可能被留成「替换/不混合」，于是下一次（隔一次）0.5 blit 变成覆盖而不是 alpha 混合，表现为纯色。
下一步在 `render/internal/gpu` 对 textured_quad draw 逐条打印/断言 blend state，对照 Skia `saveLayer` 的
「layer 内容按 opacity 与父级 SourceOver 混合」语义；修后加真窗逐像素门禁（A/B/C 三点：半蓝/半蓝/半蓝）。

**归位**：合成侧 / GPU session 线（`render/internal/gpu` 的 pipeline/blend 状态 + `render/context_layer_stack.go` 的层合成）；
本卡不盲修。

## 3. 验证与门禁

- **安全回归（不碰 GPU 设备）**：`TestClipRect_TranslateStaysRect` + `TestClipRoundRect_TranslateKeepsRadius` 绿；
  `ui/rendering` 逐文件 102 PASS / 1 SKIP_TAGGED / 0 FAIL；`CGO_ENABLED=0 go build ./ui/... ./render/...` 绿。
- **GPU 真窗证据**：修复前后的最小窗/验收窗采样在事故前完成（见上）；修复后的最小窗截图（视口内圆角块：角白、中心红）
  已留存。Golden 已按单窗流程重播种并复核 0.0000%（播种轮/复核轮各一次 20s 完整相位，正常自关，逐轮查 Xorg）；
  新基线 `showcase_record_coverage_base.png` 与快照逐字节一致，角点 (58,262)=白、中心仍为填充色。
- **GPU 事故记录（2026-11-17，必须遵守）**：在桌面跑 `tools/per-file-tests.sh render`（`render` 测试二进制含
  native GPU consumer 测试）导致当前会话注销。以后：
  1. GPU 测试与真窗一律走隔离显示环境（`ENGINE_WAYLAND_NESTED_TEST.md` / 独立 Xorg），桌面只做单窗验收；
  2. 桌面回归只跑 CPU-only/定点测试，禁止整包 `render` 逐文件；
  3. 每轮前查 Xorg/显示连接，异常立即停批。

## 4. 修订

| 日期 | 内容 |
|---|---|
| 2026-11-17 | 立项：圆角裁剪根因修复（平移≠斜切，4 处）；隔层组透明缩到最小复现并登记为未修；修正此前「retained 纹理合成忽略圆角/组透明」的错误归因（圆角是 paint 层问题，全路径受影响）；记录桌面跑 render 整包测试导致注销的事故与后续门禁。 |
| 2026-11-17 | Golden 重播种完成：单窗播种轮 SEEDED + 复核轮 OK 0.0000%（960000px）、像素 8/8、逻辑 8/8、scrollRR=0/skip≈114、animRR=0；新基线与快照逐字节一致。 |
