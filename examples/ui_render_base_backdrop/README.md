# ui_render_base_backdrop — Backdrop 层真窗（A 组 §3.1 第 7 项）

对应总账：`docs/RENDER_FEATURE_COVERAGE.md` §1 #16 / §3.1 A 组第 7 项「Backdrop 层」。
本窗覆盖 **PushBackdropLayer / ApplyBlur / PopLayer** 的 backdrop（背景快照层）组合族，
性质 = **原子验证**（直调 `render` 接口验一族），不算生产接线。

窗口 1200×800（表头 + 图例 + 四卡 + 底部 HUD），卡内容固定不播放动画；HUD 不进 golden / 抓屏对照。

## 1. 四卡覆盖

| 卡 | 能力 | 画面 |
|----|------|------|
| A | 基面直达 `PushBackdropLayer` + 50% 压暗 | 整面条纹上压一个面板；**面板内**条纹变暗、面板外条纹原样（全表面快照，无裁剪 → 层边界 = 全屏） |
| B | 隔离卡（repaint boundary）内毛玻璃 | 条纹只在玻璃面板内 → `PushBackdropLayer` + `ApplyBlur(6)` + 15% 白 tint；面板外不含条纹（隔离层里没有任何越界内容） |
| C | **有界** backdrop（推入时裁剪） | `ClipRect(面板)` → `PushBackdropLayer` → 压暗 → `PopLayer` → `ResetClip`（Flutter 惯用姿势）；滤镜输出按层边界裁剪，面板外父内容逐位保留 |
| D | 嵌套 backdrop | 外层隔离层内再推一个 backdrop 层（内层压暗带）；外层早先内容必须原样保留（父 RT 快照 + 返回父 RT 不清屏） |

每卡都同时承担「能力演示 + 探针位置基准」：像素探针的期望值由调色板常量 + 压暗/模糊解析计算
（`scene.go` `tolSolid = 8/255` + 独立通道带），不读渲染结果反推期望。

**1x / 4x 双模式**：B 卡额外覆盖「4x MSAA 底图 + 1x 图层 RT + 滤镜后 tint」这条组合路径。
`GPUI_MSAA=4` 时 base 窗口保持 4x，backdrop/隔离层走独立 1x 层上下文；Golden 用专属
`showcase_backdrop_msaa4*.png`，不会覆盖默认 1x 基线。

## 2. 怎么跑

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

RUN_SECONDS=6 go run ./examples/ui_render_base_backdrop -auto-only   # 门禁：JSON 一行到 stdout，失败 exit 1
GPUI_MSAA=4 RUN_SECONDS=6 go run ./examples/ui_render_base_backdrop -auto-only  # 4x 门禁（专属 Golden）
go run ./examples/ui_render_base_backdrop                            # 人工常驻（关窗结束）
GPUI_MSAA=4 go run ./examples/ui_render_base_backdrop                 # 人工 4x（关窗结束）
go run ./examples/ui_render_base_backdrop -manual-seconds 15         # 人工限时
go run ./examples/ui_render_base_backdrop -logic-only                # 只跑值级逻辑探针（子进程用 CPU 模式）
```

- 逻辑探针由窗口自己在**子进程**里跑（`os.Args[0] + GOGPU_RENDER_MODE=cpu`）：GUI 栈初始化后再切软件模式不可靠，
  离屏 CPU 上下文才是纯软件契约（与同族 `ui_render_base_*` 窗一致）。
- 门禁模式（`-auto-only` / `RUN_SECONDS`）**强制**跑抓屏对照，不能关；`-screencheck=false` 只影响人工模式。
- `SNAP_DIR=<dir>` 可把当前快照/基线写到临时目录（先拷入对应 `_base.png`），用于 4x 取证时不覆盖提交的 1x Golden。

## 3. 证据（本轮实测）

| 证据 | 结果 |
|------|------|
| 逻辑探针（CPU 离屏，值级契约） | **14/14** |
| 像素探针（GPU 快照，解析 oracle，容差 8/255 + 通道带） | **21/21** |
| Golden（表头 48px + 图例 260×656 + 四卡 430×232 = **627200 px**） | **diff = 0.0000%**（1x / 4x 各自专属基线） |
| 抓屏 vs 快照（同 627200 px，per-channel tol ±2/255） | **diff = 0.0000%**（1x / 4x 均零差） |
| 4x MSAA B 卡（黑/白条纹中心，期望 0.575） | **0.588 / 0.561**（容差内；修复前 0.937 / 0.212） |
| GPU 路线 | `gpu_ops ≈ 1.07 万`，`cpu_fallback_ops = 0`（backdrop 全部走 GPU 快照） |
| 呈现 | `present_policy = full_paint`，`presents ≈ 62`（6s），`vsync_source = true` |
| 本机性能参考（不门禁，见 §8） | `fps_wall ≈ 9.0`，`interval_p95 ≈ 116 ms`，1200×800 full_paint |

门禁 JSON 关键字段（`ability_extra`）：`logic_pass` / `logic_probes_ok` / `pixel_probes_ok` /
`pixel_golden_diff_pct` / `pixel_golden_total_px` / `screen_check_ok` / `screen_check_diff_pct` /
`screen_check_px` / `screen_check_detail` / `backdrop_gpu_route`。

## 4. 抓屏 vs 快照为什么单列（黑窗假绿防线）

**快照不是屏幕**：`SnapshotPath` 是收尾时在 GL 线程上**独立重绘一帧再读回**
（`ui/embedder/pipeline_app.go:1665-1684`）。所以「快照对 + 像素/golden 全绿」只能证明**重绘路径**对，
不能证明**呈现路径**对。

2026-10-19 人工验窗实测：本窗客户区**除面板区域外整窗黑**，而快照四卡正常 → 像素/golden 全绿 = **假绿**。
根因与修法见 §5。本窗因此把「抓屏 vs 快照对照」写进回归口径：

1. `app.Run()` 返回后、`app.Close()` 之前抓屏（此时窗口还显示最后一帧；快照尚未生成）；
2. `xwininfo -name <唯一标题>` 取**客户区**绝对坐标与大小（`LC_ALL=C` 固定字段名）；
3. `ffmpeg -f x11grab -draw_mouse 0 -video_size WxH -i $DISPLAY+X,Y -frames:v 1` 抓一张 PNG 到临时目录；
4. 关窗后与快照在**同一组 golden 矩形**上比对：HUD 每帧在变，不在矩形内；
5. 判定：每通道差 > ±1/255 的像素 ≤ 1% 为过（同一渲染路径，实测通常为 0）。抓屏失败/尺寸不符/被遮挡 → 门禁红。

要求：X11 + `xwininfo` + `ffmpeg`，窗口尺寸固定（门禁不缩放）。抓屏图落在 `$TMPDIR/gpui_backdrop_screen.png`（不入库）。

## 5. 本轮引擎修复（假绿根因）

黑窗根因链（修前状态）：

1. **层 RT 首 pass 清掉种子**：`PushBackdropLayer` 把父内容当种子上传/拷进层 RT，但 GPU 会话逐帧视角把
   「每次进入新视图」都当新帧、首 pass `LoadOpClear` → 种子被清，层内只剩子元素（A 卡只剩压暗矩形）。
   与快照无关的原因：快照走收尾独立重绘，那一帧里层 RT 的种子恰好被 CPU 渲染路径直接合成。
2. **父 pixmap 是未播种的池化缓冲**：图层内的父内容只存在于父 RT，`parentPixmap` 只是 CPU 侧的池化缓冲
   （取到陈旧内容）；Pop 时整面贴回就把父层真内容盖掉。
3. **滤镜输出不按层边界裁剪**：`ApplyBlur` 在整层内容上跑，模糊把层边界外的父内容糊掉（Skia 的
   saveLayer 滤镜输出只落在层 bounds 内）。

修法（对齐 Skia saveLayer-with-backdrop + Flutter BackdropFilterLayer「paints everywhere in cull rect」）：

1. `render/internal/gpu/render_session.go` 新增 `MarkColorContent` / `colorContentViews` / `renderedViews`
   （逐帧复位）：**种子视图**首 pass 改 `LoadOpLoad`；嵌套返回父 RT 也不清屏。
2. `PushBackdropLayer` 有 GPU 父视图时改抓**父 RT** 走 RT→RT 快照（`copyGPUTextureToTarget`，忽略裁剪与
   层组 alpha；父无 GPU 视图时保留原 pixmap 种子路径）。推入时裁剪即层边界
   （`clipDeviceBounds` → `markLayerBackdrop(bounds, bounded)`，无裁剪则 `bounded=false` 全屏效果）。
3. 新增 `cropFilterOutputToLayerBounds`（`render/context_layer_blend.go`）：滤镜结果落进层 pixmap 后、
   RT 重播种前，从父 pixmap 还原层边界外两带 —— 等价 Skia「saveLayer 滤镜输出裁剪到层 bounds」。

**Pop 合成保持整面**（`drawGPUTextureWithOpacityFullSurface`，不带裁剪）：窗口呈现只重放帧尾操作
（帧中 nil-view flush 把早先操作消费进 pixmap 后，present 只画剩余 pending），**整面贴回是把帧中物化内容
送上屏的承重动作**。试过两版收窄合成（scissor 版 / UV 限界版）均实测客户区大面积黑
（`diff=78.4313%`，确定性重跑同值），故恢复整面；`backdropBounds` 只用于裁剪滤镜输出。

**公开 API 无签名变更**：新增助手均包内小写（`copyGPUTextureToTarget` / `clipDeviceBounds` /
`cropFilterOutputToLayerBounds` / `drawGPUTextureWithOpacityFullSurface`）；
`markLayerBackdrop` 内部签名加 `(bounds image.Rectangle, bounded bool)`。`scripts/apidoc` exit 0。

### 5.1 4x MSAA 修复（2026-10-20）

4x 下 B 卡的模糊本身已经执行：去掉 tint 时 B 卡实测 0.518 / 0.482，说明 `ApplyBlur` 正确；
加上 tint 后中心又变回 0.937 / 0.212（锐利条纹 + 15% 白）。调试采样显示滤镜结果已经是 132，
紧接着的 tint 首 pass 把它清掉。

根因是**目标采样域错配**：图层 RT 是 1x 纹理，却由 base 的 4x 会话绘制；CPU 种子只进 1x
resolve 目标，4x MSAA attachment 没有种子，tint 首 pass 仍走 `LoadOpClear`。按「采样数属于
RenderTarget」的标准做法，本次先落第一步：

1. `Context` 拆 `baseGpuCtx`（窗口，保持配置的 4x）与 `layerGpuCtx`（离屏层，固定 1x）。
   Push/Pop GPU 层只切 active 指针，不销毁 base 会话、不全局改采样；嵌套层共用 1x 层上下文，
   最外层 Pop 后回 base 合成。
2. `GPURenderContext` 新增 `seededViews`：会话尚未创建时 `MarkViewSeeded` 先记账，会话创建后
   重放；`BeginFrame` / 设备重建清空，`PopLayer` 回收视图时 `UnmarkViewSeeded`，避免池化视图
   继承陈旧 LoadOp。
3. 嵌套层 filter 的 mid-frame fold 只在 `!hasLayer` 时启用；有图层时走图层相干的 GPU 路径。

4x B 黑/白中心恢复 0.588 / 0.561（期望 0.575，容差内）；1x/4x 均 21/21，4x 专属 Golden
627200px 零差、抓屏 vs 快照零差。后续「图层可选 4x MSAA + 1x resolve」仍按同一目标模型扩展，
不回到 Context 全局采样开关。

## 6. 回归清单

设备模式（`WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib`），**逐测试文件**跑，串行：

- 本域永久钉子：`render/backdrop_layer_present_test.go` 4 例（1x 种子/裁剪/嵌套/有界）
  + `render/backdrop_layer_msaa_test.go` 2 例（4x 嵌套 filter 种子 / 4x filter 后 tint 保种子）
  - `TestBackdropLayerPresentKeepsSeed` — 种子存活（层 RT 首 pass 不吃掉 CPU 种子）
  - `TestBackdropLayerClipCompositeFullSurface` — 有裁剪的背板 Pop 仍整面合成（父层裁剪外内容上屏）
  - `TestBackdropLayerNestedKeepsParentContent` — 嵌套背板返回父 RT 不清屏
  - `TestBackdropLayerBoundedFilterKeepsParentOutside` — 有界背板滤镜不越界（裁剪钩子；kill-switch 禁用后
    边界外像素实测 (233,72,72) vs 期望 (229,25,25) 转红，鉴别力已验证）
- 本批逐文件回归（全绿）：`layer_rt_clear_test.go`、`blend_porterduff_test.go`、`context_layer_test.go`、
  `f1_adv_layer_present_test.go`、`f14_effect_flush_publish_test.go`、`clip_ops_semantics_test.go`、
  `g2_damage_contract_test.go`、`damage_scale_test.go`、`mask_test.go`、
  `render/internal/gpu/registry_retire_test.go`、`render/internal/gpu/memory_test.go`、
  `ui/scene/composite_test.go`（`TestCompositeToContext_BackdropFilter`）、
  `render/p1_capability_matrix_closers_test.go`（`TestP1_Capability_L05_BackdropLayerGPU`）

既有红（与本轮改动无关，基线即红，勿在本窗追）：
`filter_present_coherence_test.go:TestPresentDamage_ImageMatchesView`、
`render/internal/gpu` 7 例（`sdf pipeline: no mask layout`）、
`go vet render/present_frame.go:310:36: possible misuse of unsafe.Pointer`。

## 7. Golden 基线与重播种记录

- 基线资产：`testdata/showcase_backdrop_base.png`（1x，表头 + 图例 + 四卡逐位，HUD 不参与）；
  `testdata/showcase_backdrop_msaa4_base.png`（4x 专属）；每次改动画面必须**人工评审后**重播种并随窗提交。
- 重播种命令（基线缺失时）：
  ```bash
  rm testdata/showcase_backdrop_base.png
  GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_render_base_backdrop -auto-only

  rm testdata/showcase_backdrop_msaa4_base.png
  GPUI_MSAA=4 GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_render_base_backdrop -auto-only
  ```
- **2026-10-19 重播种（评审记录）**：旧基线是黑窗缺陷帧（`diff=55.3481%`）；三处引擎修复落地后画面为
  A 面板内压暗 / B 面板内毛玻璃 / C 面板内压暗且面板外条纹原样 / D 嵌套压暗带，人工看图确认 +
  重播种，随后连跑两次门禁 `golden diff = 0.0000%`、抓屏 `diff = 0.0000%`，故接受。

## 8. 已知限界

| 限界 | 说明 |
|------|------|
| 抓屏依赖与遮挡 | 需要 X11 + `xwininfo` + `ffmpeg`，窗口须可见且未被其它窗遮挡；不满足时门禁红是**预期行为**（不能拿快照顶替屏幕证据） |
| 帧率不在本窗门禁 | 本窗是正确性窗：`fps_gate = off_correctness`、`slope_gate = off`；fps/p95 只记录不判定（本机约 9fps / p95 116ms，1200×800 full_paint 核显） |
| 每次推入/弹出都是全表面拷贝/blit | 推入 = 帧中物化 + RT→RT 全表面快照；弹出 = 全表面贴回（呈现恢复承重机制，不能简单收窄为损伤区）。四卡窗因此约 9fps（对照 maskops 窗约 19fps）；优化方向是把「上屏恢复」从整面贴回改为显式基底层，之后弹出即可收窄到损伤/层边界（记 `RENDER_FEATURE_COVERAGE.md` §6 性能子项） |
| 有界背板的层边界 | 层边界 = 推入时**设备空间**裁剪（`clipDeviceBounds`），不是 Skia 的 cull rect；未裁剪时 `bounded=false`，模糊输出按全屏处理 |
| CPU 回退路径 | 无 GPU 会话 / 父层由 CPU 画过时仍走 pixmap 种子 + CPU 合成；本轮修复不改 CPU 合成结果（整面合成与裁剪在 CPU 侧同样生效） |
| 图层可选 4x 尚未开放 | 本次落「采样数归渲染目标」第一步：base 按配置 4x，离屏图层固定 1x。2.5D 若需要图层内 4x 几何 AA，后续按 `RenderTarget + resolve` 增加 opt-in；当前不要用全局 `SetPreferSampleCount1` 在 Push/Pop 间切换 |

## 9. 文件

- 窗口：`examples/ui_render_base_backdrop/main.go`（框架/探针/门禁/抓屏/1x·4x 切换）、`scene.go`（四卡与探针定义）、
  `testdata/showcase_backdrop_base.png`（1x Golden，随窗提交）、`testdata/showcase_backdrop.png`（最近一次 1x 快照，随窗提交）、
  `testdata/showcase_backdrop_msaa4_base.png`（4x Golden，随窗提交）、`testdata/showcase_backdrop_msaa4.png`（最近一次 4x 快照，随窗提交）
- 引擎修复：`render/m4_quadimage.go`（RT→RT 快照 + 层边界）、`render/context_layer_blend.go`
  （`markLayerBackdrop` / `cropFilterOutputToLayerBounds`）、`render/context_layer_stack.go`（整面 Pop 合成 +
  1x/4x 上下文切换）、`render/context_gpu.go`（base/layer 双 GPU 上下文）、`render/context_clip.go`
  （`clipDeviceBounds`）、`render/context_image_gpu.go`（`copyGPUTextureToTarget` /
  `drawGPUTextureWithOpacityFullSurface`）、`render/filter_ops_gpu.go` / `render/filter_ops_cpu.go`（裁剪钩子 /
  `hasLayer` fold 条件）、`render/internal/gpu/gpu_render_context.go`（`seededViews` 跨会话记账）、
  `render/internal/gpu/render_session.go`（`MarkColorContent` 记账）
- 永久回归：`render/backdrop_layer_present_test.go`、`render/backdrop_layer_msaa_test.go`
