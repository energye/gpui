# ui_render_base_maskops — 掩码写路径真窗（A 组 §3.1 第 6 项）

对应总账：`docs/RENDER_FEATURE_COVERAGE.md` §1 #14 / §3.1 A 组第 6 项「Mask 写路径族」。
本窗覆盖 **SetMask / ClearMask / InvertMask / ApplyMask / PushMaskLayer(+PopLayer)** 与 `Mask` 构造族，
性质 = **原子验证**（直调 `render` 接口验一族），不算生产接线。

窗口 1200×800（表头 + 图例 + 四卡 + 底部 HUD），卡内容固定不播放动画；HUD 不进 golden / 抓屏对照。

## 1. 四卡覆盖

| 卡 | 能力 | 画面 |
|----|------|------|
| A | `ApplyMask` 后处理（Skia **DstIn** 语义） | ramp 255/170/85/0 圆角内容 + 掩码矩形（quad-bounded）；矩形外像素原样不动 |
| B | `SetMask` × `ClearMask` | 左半掩码压住填充 → `ClearMask` 后整条填充复原（同一 DC 连续绘制） |
| C | `InvertMask` | 同一 `Mask` 对象原地反转 → 下一条带整条翻转（对象身份不变） |
| D | `PushMaskLayer` + `PopLayer` | 圆掩码图层：整层内容经 R8 掩码调制后合成到父目标，圆外透明 |

每卡都同时承担「能力演示 + 探针位置基准」：像素探针的期望值由调色板常量 + 掩码覆盖解析计算
（`scene.go` `tolSolid = 8/255` + 独立通道带），不读渲染结果反推期望。

## 2. 怎么跑

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

RUN_SECONDS=6 go run ./examples/ui_render_base_maskops -auto-only   # 门禁：JSON 一行到 stdout，失败 exit 1
go run ./examples/ui_render_base_maskops                            # 人工常驻（关窗结束）
go run ./examples/ui_render_base_maskops -manual-seconds 15         # 人工限时
go run ./examples/ui_render_base_maskops -logic-only                # 只跑值级逻辑探针（子进程用 CPU 模式）
```

- 逻辑探针由窗口自己在**子进程**里跑（`os.Args[0] + GOGPU_RENDER_MODE=cpu`）：GUI 栈初始化后再切软件模式不可靠，
  离屏 CPU 上下文才是纯软件契约（见 `main.go` 顶部注释）。
- 门禁模式（`-auto-only` / `RUN_SECONDS`）**强制**跑抓屏对照，不能关；`-screencheck=false` 只影响人工模式。

## 3. 证据（本轮实测）

| 证据 | 结果 |
|------|------|
| 逻辑探针（CPU 离屏，值级契约） | **14/14** |
| 像素探针（GPU 快照，解析 oracle，容差 8/255 + 通道带） | **18/18** |
| Golden（表头 48px + 图例 260×656 + 四卡 430×232 = **627200 px**） | **diff = 0.0000%** |
| 抓屏 vs 快照（同 627200 px，per-channel tol ±2/255） | **diff = 0.0000%** |
| GPU 路线 | `gpu_ops = 11267`，`cpu_fallback_ops = 0`（掩码全部走 GPU） |
| 呈现 | `present_policy = full_paint`，`presents = 128`（6s），`vsync_source = true` |
| 本机性能参考（不门禁，见 §8） | `fps_wall ≈ 19`，`interval_p95 ≈ 54 ms`，1200×800 full_paint |

门禁 JSON 关键字段（`ability_extra`）：`logic_pass` / `logic_probes_ok` / `pixel_probes_ok` /
`pixel_golden_diff_pct` / `pixel_golden_total_px` / `screen_check_ok` / `screen_check_diff_pct` /
`screen_check_px` / `screen_check_detail` / `mask_gpu_route`。

## 4. 抓屏 vs 快照为什么单列（黑窗假绿防线）

**快照不是屏幕**：`SnapshotPath` 是收尾时在 GL 线程上**独立重绘一帧再读回**
（`ui/embedder/pipeline_app.go:1665-1684`）。所以「快照对 + 像素/golden 全绿」只能证明**重绘路径**对，
不能证明**呈现路径**对。

2026-10-19 人工验窗实测：本窗客户区除指标与文字外**整窗黑**，而快照四卡正常 → 像素 18/18、golden 零差全绿 = **假绿**。
根因与修法见 §5。本窗因此把「抓屏 vs 快照对照」写进回归口径：

1. `app.Run()` 返回后、`app.Close()` 之前抓屏（此时窗口还显示最后一帧；快照尚未生成）；
2. `xwininfo -name <唯一标题>` 取**客户区**绝对坐标与大小（`LC_ALL=C` 固定字段名）；
3. `ffmpeg -f x11grab -draw_mouse 0 -video_size WxH -i $DISPLAY+X,Y -frames:v 1` 抓一张 PNG 到临时目录；
4. 关窗后与快照在**同一组 golden 矩形**上比对：HUD 每帧在变，不在矩形内；
5. 判定：每通道差 > ±2/255 的像素 ≤ 1% 为过（同一渲染路径，实测通常为 0）。抓屏失败/尺寸不符/被遮挡 → 门禁红。

要求：X11 + `xwininfo` + `ffmpeg`，窗口尺寸固定（门禁不缩放）。抓屏图落在 `$TMPDIR/gpui_maskops_screen.png`（不入库）。

## 5. 本轮引擎修复（假绿根因）

黑窗根因链（含行号，修前状态）：
`render/context_layer_stack.go` 掩码 pop → 先做 nil-view `FlushGPU()`（把本帧此前的待画操作**物化回 CPU pixmap**）
→ `render/internal/gpu/layer_gpu_io.go` 在 CPU 上做掩码合成写 pixmap → `gpu_render_context.go` 只重放读回之后新排的操作；
本帧此前排队的内容再也进不了交换链 → 只呈现尾部。快照走 §4 的独立重绘，所以不黑。

修法（对齐 Skia saveLayer-with-mask：**掩码是合成时的覆盖，不是帧中回读**）：

1. **掩码 pop 改 GPU 可重放合成**：`ModulateMaskedLayerGPU`（`render/internal/gpu/layer_gpu_io.go:281`）
   用 R8 调制 pass 把图层 RT 与掩码相乘得到新纹理（`maskR8ModulateViewToTexture`，
   `render/internal/gpu/mask_r8_modulate.go:440`），再由 `queueMaskedLayerGPU`
   （`render/context_layer_blend.go:419`）排队一次可重放的 blit 合成；全程不读回、不做 nil-view flush。
   掩码面按源全尺寸构建、越界填 0（等价旧 crop 语义）；CPU 回退路径保留（无 GPU 会话 / CPU 画过的层）。
2. **lead submit drain**：图层 self-flush 在 `opt21DeferLayerSubmit` 下把 CB 挂进延迟队列；
   调制 pass 立即采样图层 RT，故先 `FlushLeadingSubmitsOnly()`（与 `RenderFrameGrouped` C5 drain 同契约），
   否则采样到未提交的空 RT。
3. **blit-only 离屏清屏透明**：`encodeBlitOnlyPass` 原来对任何目标都用不透明黑清屏（A=1）；
   嵌套隔离层（纯 `PushLayerIsolated` 也能复现）把父层 RT 首帧清成不透明黑，blit 只覆盖局部 → 父层全域黑。
   现在由 `FlushOffscreen` / `flushLayerViewGPU` / `gpuFlushOffscreenTarget` 标记离屏目标
   （`render/internal/gpu/render_session.go:4322` `SetOffscreenBlitTarget`），离屏清 A=0（Skia saveLayer 透明底），
   **窗口表面仍清不透明黑**（未覆盖区域就是窗口底色）。
4. **永久钉子**：`render/layer_rt_clear_test.go` 两例（嵌套隔离层透明底 / 嵌套掩码层只留交集象限），
   与 `render/mask_test.go:TestPushMaskLayer_Nested` 一起防回归。

**公开 API 无签名变更**：`GPURenderTarget` 未加字段；新增入口都在 `render/internal/gpu`（`FlushOffscreen` 等）与
`render` 包内部助手（`queueMaskedLayerGPU` / `flushLayerViewGPU` / `gpuFlushOffscreenTarget`）。

## 6. 回归清单

设备模式（`WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib`），**逐测试文件**跑，串行：

- 本域：`mask_test.go`、`context_layer_test.go`、`layer_rt_clear_test.go`、`p01_layer_gpu_test.go`、
  `p12_clip_mask_gpu_test.go`、`s6_4_layer_filter_test.go`、`s6_4_coalesce_matrix_test.go`、
  `f1_adv_layer_present_test.go`、`f14_effect_flush_publish_test.go`、`blend_porterduff_test.go`、
  `p03_advanced_blend_test.go`、`p13_frame_flush_test.go`
- 呈现/编码：`present_record_test.go`、`present_resize_test.go`、`pass_scratch_test.go`、
  `opt18_filter_submit_coalesce_test.go`、`r7_4_damage_coalesce_test.go`、`r7_2_filter_export_test.go`、
  `s6_1_frame_enforce_test.go`、`s6_2_submit_path_test.go`、`s6_3_batch_deepen_test.go`
- 邻近：`context_image_test.go`、`images_gpu_visual_test.go`、`y_orientation_gpu_test.go`、
  `mem_complex_offscreen_test.go`、`p1_gpu_fixed_pixel_test.go`
- 邻窗不回归：`ui_render_base_clipop -auto-only` golden **0.0000% / 627200 px**
  （同一套 blit-only / 图层路径，输出与既有基线逐位一致）

既有红（与本轮改动无关，已用 stash 基线逐条比对一致，勿在本窗追）：
`filter_present_coherence_test.go:TestPresentDamage_ImageMatchesView`、
`render/internal/gpu` 7 例（`TestBindGroupLayoutSwitch_StencilThenSDF`、Opt21/Opt34 ×2/Opt36/Opt38 ×2，
均为 `sdf pipeline: no mask layout`）、`TestS69_HeavyBudget_TierGates`、`TestS3c_M3_BlendHue`、
`TestClipCircle_CPUPixels`、`TestStencilRenderer_EnsureDepthClipPipelines`、clipping 严格档 2 区。

## 7. Golden 基线与重播种记录

- 基线资产：`testdata/showcase_maskops_base.png`（表头 + 图例 + 四卡逐位，HUD 不参与）；
  每次改动画面必须**人工评审后**重播种并随窗提交。
- 重播种命令（基线缺失时）：
  ```bash
  rm testdata/showcase_maskops_base.png
  GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_render_base_maskops -auto-only
  ```
- **2026-10-19 重播种（评审记录）**：掩码 pop 由「帧中 CPU 材料化」改为「GPU 调制 + 可重放合成」后，
  D 卡（圆掩码图层）圆边 **1105 px 全部 |Δ| = 1/255**（GPU 合成 vs 旧 CPU 整数 source-over 的舍入差；
  逐位直方图仅 delta 1，无结构性差异），另图例新增一行 `screen: 抓屏 vs 快照 同区对照`。
  重播种后连跑两次门禁均 `golden diff = 0.0000%`、抓屏 `diff = 0.0000%`，故接受。

## 8. 已知限界

| 限界 | 说明 |
|------|------|
| 抓屏依赖与遮挡 | 需要 X11 + `xwininfo` + `ffmpeg`，窗口须可见且未被其它窗遮挡；不满足时门禁红是**预期行为**（不能拿快照顶替屏幕证据） |
| 帧率不在本窗门禁 | 本窗是正确性窗：`fps_gate = off_correctness`、`slope_gate = off`；fps/p95 只记录不判定（本机约 19fps / p95 54ms，1200×800 full_paint 核显） |
| 掩码几何口径 | R8 掩码面与图层 1:1（图层大于掩码时越界为 0，等价旧 crop）；`ApplyMask` 的掩码矩形固定在表面原点（与 Skia 裁剪后固定位置口径一致），窗内 A 卡按此建模 |
| 每次掩码 pop 两张临时纹理 | 调制一次分配 R8 掩码纹理 + RGBA 输出纹理，随帧末 GPU 队列排空后释放（`layerGPUReleases`）；多掩码层场景暂未做纹理池化 |

## 9. 文件

- 窗口：`examples/ui_render_base_maskops/main.go`（框架/探针/门禁/抓屏）、`scene.go`（四卡与探针定义）、
  `testdata/showcase_maskops_base.png`（golden，随窗提交）、`testdata/showcase_maskops.png`（最近一次快照，随窗提交）
- 引擎修复：`render/context_layer_stack.go`、`render/context_layer_blend.go`、`render/context_gpu.go`、
  `render/internal/gpu/layer_gpu_io.go`、`render/internal/gpu/mask_r8_modulate.go`、
  `render/internal/gpu/render_session.go`、`render/internal/gpu/gpu_render_context.go`
- 永久回归：`render/layer_rt_clear_test.go`
