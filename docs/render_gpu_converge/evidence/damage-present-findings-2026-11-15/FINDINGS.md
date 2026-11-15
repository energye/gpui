# 损伤呈现语义实证（O12 批 1a W1 建窗期发现，2026-11-15）

来源：`examples/ui_render_base_framepresent` 建窗期的 offscreen 探针矩阵 + swapchain 抓屏对照。
探针程序：`fpprobe_session_state.go`（本目录，`go run` 可复现；GL 线程纪律无关——纯 offscreen）。
门禁 JSON：`w1_gate_native_wgpu.json`（native wgpu）；GL 后端 stderr：`gl_backend_run_stderr.log`。

## 实证矩阵（native wgpu，Xwayland :3，940MX 集成适配器）

| # | 场景 | 实测 | 判读 |
|---|---|---|---|
| 1 | 同 view 连续两次 `PresentFrameDamage`（部分内容 blit + 损伤矩形） | 损伤外像素保留 v1 ✓，且损伤外 blit 被裁掉（画在损伤外的补丁不上屏） | LoadOpLoad + 画笔剪刀**都生效** |
| 2 | 跨 view（先 flush 别的 view）后回来做同样的损伤呈现 | 同样保留 ✓、同样裁剪 ✓ | 与 #1 同 |
| 3 | **UV 变体**（DrawGPUTextureWithOpacityUV）整面 blit + 损伤矩形 | 损伤外被 blit 内容覆盖（`got=(229,153,38) want=保留(76,87,140)`） | **画笔剪刀未生效（漏剪）**——与 #1 同一调用族不同结果 |
| 4 | 多矩形损伤呈现后，「未触点」（不在任何 blit dst、不在损伤内） | `got=(41,46,76)` ≈ v1 变暗 46% | 损伤外未触点被会话状态相关的混合改写（疑似 dual-tex 合成路径） |
| 5 | 矢量填充帧 + 损伤矩形（offscreen） | 损伤外全黑（MSAA LoadOpClear） | 文档明载限界 ✓（`FlushGPUWithViewDamage` doc："frames with Fill/Stroke → MSAA path always LoadOpClear"） |
| 6 | **swapchain 表面**路径：blit-only 损伤呈现（两块远距补丁 + Invalidate） | 抓屏对照：补丁上屏 ✓ 但图例/缝隙全黑（整面清屏） | **wgpu 表面路径的损伤呈现 = 清屏 + 只画损伤内内容**——与 §3.3 已记录的「wgpu-native 仍全量（平台限界）」同族，且更精确定位为「清屏而非全量重写」 |
| 7 | 同 #6 场景在 **GL 后端**（GPUI_BACKEND=go） | 抓屏对照：patchL/R 变 ✓ **gap_kept=true legend_kept=true**——保留完美生效 | GL 损伤链（SetDamageRegionKHR + 剪刀 blit）工作正常 ✓；但 os partial 计数 Δ=0（buffer-age 升格全量路径，`presentDamageHistLen` 未热身时损伤呈现走全量 blit——保留靠全量重写成立，计数语义归 P1 指标链） |

## 归位

- **#3/#4（wgpu 会话状态相关的损伤裁剪/混合不一致）**：新发现。修复面在 `render/internal/gpu` 会话状态机（frameRendered / 视图切换 auto-reset / dual-tex 合成的剪裁决策），需要先吃透状态机再动手（动手修门槛：复现=本探针矩阵 ✓，标准依据=Flutter partial-repaint 的 damageRegion 裁剪语义，待补对齐说明）。
- **#6（wgpu 表面损伤呈现清屏）**：既有「wgpu-native 仍全量（平台限界）」的精确定位，非新洞；当前全部生产流（FullPaint 策略 / packet 全内容合成帧）不触此路径。
- **#5**：文档明载限界，本窗把它钉成行为探针（vector_msaa_clear 两点读回）。
- **#7**：GL 链健全的正面证据；partial 计数语义待 P1 线核（buffer-age 升格）。

## W1 窗门禁口径

门禁（`-auto-only`，native wgpu）只断言跨后端一致的已验证语义：模式直派（full/idle/multi/union）、PlanFramePresent 五态、blit 落点像素、空闲零上屏、快照逐位黄金图、终帧抓屏对照。#3/#4/#6 以 findings 进 JSON（`offscreen_findings`）+ 本文档，不判死；GL 后端为冒烟证据跑（黄金图基线按 wgpu 播种，不做双基线）。
