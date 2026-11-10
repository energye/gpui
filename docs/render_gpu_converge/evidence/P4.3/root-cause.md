# P4.3 治根 · 根因定位（2026-11-10）

> 性质：用户拍板「方案 1 治根、不打补丁、对齐标准、通用」后的**动手前证据**。
> 按 AGENTS「动手修门槛」两样齐备：① 真实问题结果（复现 + 实测数据 +
> file:line）② 对齐标准方案。本文不含代码改动。**停在「停下报告」等拍板。**

## 0 直话结论

**用户选定的方案 1 前提（P4.1 批 3 在册洞 #5「gpuTex tier 末尾重放」）被客观
数据推翻：#5 不是 ②③ 失败的根因。** 真根因是一个**通用架构缺口**：

> `render.Canvas` 契约里没有「设备自有纹理绘制」这一等能力。`DrawGPUTextureCommand`
> 只能由 `*Recorder` 具体类型旁路追加，且携带裸 `render.TextureView` 设备句柄；
> 因此任何在 **Canvas 命令实现内部**（如 `Context.DrawShadow`）产生的 GPU 纹理
> 绘制都无法进入录制/回放流，live/present 帧静默丢内容。

这与 P4.1 批 4 补正 3（DrawShadow 自身缺 `CmdDrawShadow` → live 回放无阴影）
**完全同构**，只是这次缺的是「命令内部新产生的纹理绘制」。所以治根对象应从
「GPU 层链 tier 顺序」改为「Canvas 录制契约对设备纹理的一等支持」——服务所有
需要「缓存离屏 GPU 纹理并跨帧复用」的能力，不只 DrawShadow。

## 1 客观数据：三组排除实验（都不用 mean，逐像素/逐断言）

| 实验 | 复现 | 结果 | 排除什么 |
|---|---|---|---|
| E1 tier5 盖序 | `render/p43_tier5_draw_order_test.go`，把 P3.7「盖序=提交序」钉从 tier 1/4 扩到 **tier 5（GPUTex）**，6 组（单贴图 / 1→5 / 5→1 / 1→5→1 / 5→5 / 5→1→5） | **6/6 PASS**（plate 读回 `[0 0 255 255]` / `[0 255 0 255]` 确认输入非空） | 排除 #5「gpuTex tier 末尾重放盖住后画」、`ensureDrawOrder`/`buildScissorGroups` 顺序缺陷 |
| E2 填充方式二分 | `render/p43_upload_plate_probe_test.go`，A=`FlushGPUWithView`（GPU 渲染 pass 填）vs B=`UploadRGBAToView`（CPU 直传，②③ 用的方式），各自读回 + `DrawGPUTexture` 贴回 | **A/B 皆绿**：view=`[0 0 255 255]`、composite=`[0 0 255 255]` | 排除纹理上传/layout/采样路径、`viewToResView` 注册解析 |
| E3 真窗（上会话） | `impl-attempt.md`，缓存开启 vs `no-cache` vs `store-only` | fps 3→59 达标，但 golden diff 恒定 ~33%；关缓存/只存不命中皆 0.0000%；red-tile 仅 1272/58800 生效；gpu_ops +1 却贴不回 | 差异只来自「命中贴图」，且只在**窗口录制回放链**出现 |

**E1 首轮曾全红，是测试构造错误**：`ClearWithColor` 只写 CPU pixmap，
`FlushGPUWithView` 没把它上传 → plate 是空纹理、贴图透明 → 假红。补一笔全画布
GPU `Fill` + `ReadbackViewRGBA` 校验后转绿。如实记录，避免把假红当根因。

**归纳**：离屏单 Context 内，独立离屏纹理的填充、上传、tier 盖序、贴回**全部正常**；
唯一失败场景是「窗口 live 帧（录制→回放→present）里，Canvas 命令实现内部旁路
产生的纹理绘制」。

## 2 file:line 证据链

| 位置 | 事实 |
|---|---|
| `render/recording/command_ops.go:139-150` | 明文「**Canvas deliberately has no DrawGPUTexture (the handle is device-owned), but a recording can carry it**」；`DrawGPUTextureCommand.View render.TextureView` = **裸设备句柄** |
| `render/recording/recorder_canvas_ops.go:207-211` | `func (r *Recorder) DrawGPUTexture(...)` 存在，append `DrawGPUTextureCommand`——但只有 `*Recorder` 具体类型可调，`render.Canvas` 接口没有该方法 |
| `render/recording/backend_canvas.go:375-382` | 回放 `DrawGPUTexture` 仅当 `b.dc.(GPUTextureBackend)` 成立才转发，**否则静默丢弃** |
| `render/recording/recorder.go:340-344` | Playback 分发 `case DrawGPUTextureCommand` 同样要求 backend 实现 `GPUTextureBackend` |
| `render/recording/draw_shadow_test.go:11-20` | **同构先例**：P4.1 补正 3「DrawShadow 是 Context 独有方法，录制回放路径无法承载（登记降级）」→ 修法 = Canvas 接口 + 命令 + Recorder/Playback/CanvasBackend/bounds 全链 |
| `render/context_image_gpu.go:263` | `*Context.DrawGPUTexture` 是 Context 方法，Queue 到**当前** render session，不产生任何录制命令 |

推论：P4.3 ②③ 在 `Context.DrawShadow` 内直接调 `c.DrawGPUTexture`，而
`DrawShadow` 本身是 `CmdDrawShadow` 回放时由 `CanvasBackend.DrawShadow` 转发到
`b.dc.DrawShadow` 执行的——这个内部的纹理绘制是**回放期的旁路副作用**，不属于
录制命令序列，present 重放时没有对应记录 → E3 的 1272/58800。

## 3 对齐标准（通用方案依据）

| 标准 | 做法 | 本库现状 |
|---|---|---|
| **Skia** | `SkCanvas::drawImage/drawImageRect(const SkImage*)`：GPU-backed `SkImage` 是**一等绘制参数**，可被 `SkPicture`/`SkRecord` 录制；`GrSurfaceProxyView` 延迟实例化，纹理可跨录制存活（不存裸设备指针） | `Canvas` 故意不暴露纹理绘制；`DrawGPUTextureCommand` 存裸 `render.TextureView` |
| **Flutter** | `ui.Image` 句柄 + `TextureLayer`/`ImageFilterLayer` 在 Layer 树传递；`Picture` 可录 `drawImageRect`；`RepaintBoundary` 缓存离屏纹理后按 Layer 树重放 | 无对应一等 Canvas 能力 |
| **本库已有底座** | `render/internal/gpu/res/types.go:12-17` 明文「**SourceKey + Ref ≈ GrSurfaceProxy**（logical + strong reference）」；`GPUTextureDrawCommand.View` 已是 `res.View`（`:69`），支持 SourceKey 延迟解析 / 强 Ref | **缺的是把 `res.View` 接到 Canvas/Recorder 契约**，而不是重新发明缓存 |

**结论**：通用治根 = 让「设备自有纹理绘制」成为 Canvas 一等、可录制的能力，句柄用
`res.View`（延迟解析，对齐 `GrSurfaceProxy`）而非裸 `TextureView`。这样：
- 任何 Canvas 命令实现内部都能安全地产生可重放的纹理绘制（DrawShadow、backdrop、
  RepaintBoundary 式缓存、未来的 Picture 嵌套都受益）；
- 跨帧缓存的纹理生命周期由 SourceKey/Ref 管理，不依赖 Context 是否复用；
- 符合「对齐标准、通用、不打补丁」。

## 4 影响面（wr-engine 第 1 步，render/ 高风险）

- 改动层：`render/recording`（Canvas 接口、命令、Recorder、Playback、CanvasBackend、
  bounds）+ `render`（Context 实现、res.View 桥）。**零公开 API 变化预期？需评估**：
  Canvas 加方法 = 公开接口变化（AGENTS「接口全量切换制」要一次改完所有调用方 +
  同步 catalog/apidoc）。
- 上游 `ui/rendering`、`ui/scene`、`examples/` 的 Canvas 实现/spy/stub 都要跟随
  （P4.1 补正 3 同批曾改 `backend_stub_test.go`、多个 spy）。
- 下游 `gpu`：纹理生命周期走既有 `res.View`/registry，不动后端。
- 影响的 R/C：所有 retained/录制回放真窗（maskops、recording、video retained、
  P4.1 四扇 p41 窗）、以及 `DrawGPUTextureCommand` 现有消费者。
- 门禁：`recording/`、`render/` 逐文件回归 + 双后端 + `-race`/vet/apidoc +
  真窗三证据 + 负向对照。

## 5 状态

- ① 分段归因 ✅；②③ 实现已回退（上会话 `82a4985d`）。
- 本会话新增两个**通用常驻钉**（工作区，未提交）：`p43_tier5_draw_order_test.go`
  （把 P3.7 契约扩到 tier 5，6/6 绿）、`p43_upload_plate_probe_test.go`（A/B 双绿）。
- **停在「停下报告」**：治根对象从 #5 改为 Canvas 录制契约，方向变更需用户拍板；
  确认前不动代码。
