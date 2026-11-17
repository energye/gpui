# P6 合成正确性卡：视口圆角裁剪 + 隔层组透明

日期：2026-11-17 ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui` ｜ 分支：`feat/UI-组件`
状态：**问题 1（视口内圆角裁剪丢失）已修；问题 2（滤镜层之后的隔层组透明失效）已修（blur 家族）；
其它 fullComposite 效果的同类风险待审计**。
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

## 2. 问题 2（已修 · 模糊家族）：模糊层之后隔层组透明失效

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
- **不是 blend state**：三条 0.5 纹理合成命令的 opacity 都是 0.5（`GPU 真机 trace`：blur=1、A/B/C=0.5），
  顶点/批次/绘制调用都正确；把三条合成改成不透明全层 blit 时 A/B/C 都变实心（证明层内容本身在）。

**根因**：模糊层被 `markLayerFullComposite()` 标记成**全表面合成**，Popup 时把整张 600×400 的层 RT
以 opacity=1 铺满父级。层 RT 来自池化复用，内容只画在模糊框范围内，**框外仍是上一轮/上一层留下的像素**
（真机 trace 里 B 的矩形处正好有上一帧的纯蓝）；随后 B 自己的 0.5 blit 叠在这块实色上 → 看起来不透明。
把模糊层改成按 content+halo 的脏区合成（诊断开关）后，A/B/C 立刻全部半蓝，根因闭环。

**修复**：`markLayerEffect(pad)` —— 滤镜层合成限制在「内容 damage + 滤镜扩散半径」内，不再整表铺；
damage 为空（bounds 不可知）才回退全量。`ApplyBlur` / `ApplyBlurXY` 用 `pad = 3r + 2`；
与 Skia 把 saveLayer filter 输出裁剪到 layer/effect bounds 的语义一致。单测
`render/layer_effect_bounds_test.go` 锁 damage 扩张与空 damage 回退。

**验证（真机单窗，逐轮查 Xorg）**：
- 最小窗 blur + A/B/C：修复前 A 半蓝、B 纯蓝、C 半蓝；修复后 **A/B/C 全 (147,166,242)**；
  模糊中心 (217,51,51)，框外 8px 仍有轻微晕染（halo 未被裁掉）。
- 验收窗：透明度块 (116,340) 由纯蓝变 **(147,172,242)**；圆角块角点保持白；blur 中心 (136,460) 不变；
  像素探针 8/8、逻辑 8/8；Golden 差 **0.5333%（5120px）**，正是透明度块恢复半透明的面积，待人工过目后重播种。
- 无模糊 / full-paint / 4 层透明度变体：全部正确，无回归。

**遗留**：其它仍用 `markLayerFullComposite()` 的效果（颜色矩阵/灰度、drop shadow、SVG 光照/湍流等）
有同一类风险（池化层 RT 的旧像素被整表铺出）；本次只按真机复现修了 blur 家族，其余需用同样的
「效果边界合成」审计，不声称全族关闭。

**归位**：合成侧 / 层合成（`render/context_layer_stack.go` + `render/filter_ops_cpu.go`）。

## 3. 验证与门禁

- **安全回归（不碰 GPU 设备）**：`TestClipRect_TranslateStaysRect` + `TestClipRoundRect_TranslateKeepsRadius` +
  `TestMarkLayerEffect_BoundsDamage` / `TestMarkLayerEffect_EmptyDamageFallsBackFull` 绿；
  `ui/rendering` 逐文件 102 PASS / 1 SKIP_TAGGED / 0 FAIL；`CGO_ENABLED=0 go build ./ui/... ./render/...` 绿。
- **GPU 真窗证据**：修复前后的最小窗/验收窗采样在事故前完成（见上）；修复后的最小窗截图（视口内圆角块：角白、中心红）
  已留存。Golden 按单窗流程做了**两次**播种，均复核 0.0000%（播种/复核各一次 20s 完整相位，正常自关，逐轮查 Xorg）：
  第一次对应圆角修复；第二次对应透明度修复（差 0.5333% / 5120px，透明度块恢复半透明）。
  当前基线 `showcase_record_coverage_base.png` 与快照逐字节一致：角点 (58,262)=白、透明度块 (116,340)=(147,172,242)、
  blur 中心 (136,460) 不变。
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
| 2026-11-17 | 问题 2 根因与修复：模糊层 fullComposite 把池化层 RT 的旧像素整表铺出，隔层透明度被实色盖住；新增 markLayerEffect(pad) 按内容+滤镜扩散合成 blur/blurXY，单测 + 真窗 A/B/C 全半蓝；验收窗透明度恢复 (147,172,242)。 |
| 2026-11-17 | 第二次 Golden 重播种完成（对应透明度修复 0.5333%）：播种轮 SEEDED、复核轮 OK 0.0000%（960000px）、像素 8/8、逻辑 8/8、scrollRR=0/skip≈113、animRR=0；基线与快照逐字节一致。 |
