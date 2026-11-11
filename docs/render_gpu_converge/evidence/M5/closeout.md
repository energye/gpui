# M5 HDR 全链：收口（2026-11-11）

> 台账来源：§2.2 M5（P3.7 D5 派生）。本文件是 M5 的收口正本：范围、决策、交付、
> 双后端矩阵、证据索引、残留登记。设计正本 assessment-design.md，逐批证据见 p0–p4。

## 0 范围与决策回顾

M5 登记缺口：render 层 GPU 半边 HDR 代码量为零——CPU float 目标 TextureView/Pixels 恒 nil、
GPURenderTarget 无 Format、离屏 RT 固定 BGRA8、呈现固定 8bit。

拍板（用户确认，全按推荐）：

| 决策 | 结果 |
|---|---|
| D1 面格式 | RGBA16Float（8B/px） |
| D2 管线范围 | 全格式参数化（12 文件 43 处 ColorTargetState） |
| D3 呈现 | tonemap→8bit（本项）；10-bit/float swapchain 另立 |
| D4 曲线/曝光 | 复用 hdr.Encode（曝光(0,16]→Reinhard→sRGB） |
| D5 MSAA | float 首版 1x；MSAA+float 未开 |
| D6 失败语义 | 不可渲染具名 ErrOffscreenFormatUnsupported + 回退计数 |

P4 API 形状：方案 B「格式跟随目标视图」（Skia surface color type 形态），不加 SetHDR 开关。

## 1 分批交付

| 批 | 交付 | 关键证据 |
|---|---|---|
| P0 | 16F 面建/渲/混合探针（half 位型 4100/3400/3000/3a00） | evidence/M5/p0-probe-native.txt |
| P1 | color_format.go 单一口径（零值=BGRA8）+ 9 owner 字段 + 43 处描述符 | p1-format-parameterization.md、p1-unit-tests.txt |
| P2 | 池键含格式、hal.VramTextureBytes（16F 8B/px）、白名单+1×1 clear pass 能力门、具名拒绝/回退计数 | p2-float-offscreen-rt.md、p2-unit-tests.txt |
| P3 | tonemap pass（与 hdr.Encode 对拍 max_diff=0） | p3-tonemap-pass.md、p3-unit-tests.txt |
| P4 | +3 公开 API、Flush 格式驱动 1x owned 管线、真窗 ui_wr_m5_hdr | p4-hdr-window.md、p4-windows/、p4-unit-tests.txt |
| P5 | 负向守卫（tonemap 拒 8bit 源 / 深度格式零分配拒绝）、双后端复跑、catalog/apidoc、收口 | p5-unit-tests.txt、p5-windows/ |

## 2 双后端矩阵

| 项 | webgpu (native) | gwgpu-gles (GPUI_BACKEND=go) | metal |
|---|---|---|---|
| 16F 面建/渲/混合（P0） | ✅ | 能力位在案（GL_EXT_color_buffer_half_float/float）；真实绘制由真窗覆盖 | 本机不可验（无 macOS） |
| tonemap vs hdr.Encode | ✅ max_diff=0 | ✅（同一 WGSL） | 本机不可验 |
| 真窗 ui_wr_m5_hdr | ✅ 9/9 探针 maxdiff=0、金图 0.0000% | ✅ 9/9 maxdiff=0、独立金图 0.0000% | 本机不可验 |
| 回退计数 | 0 | 0 | — |

## 3 最终数字

- 真窗：native 352 presents / go 357 presents；6 卡（3 曝光 × GPU/CPU）逐位金图 0.0000%。
- 像素：9/9 探针 GPU vs CPU maxdiff=0；曝光 1/4 红通道 213/242（>1 未被 8bit 钳）。
- 定标：M5 单测 + conformance 全绿；build/vet/nogpu rc=0；apidoc rc=0。
- 8bit 回归（P4 批）：p35_rrect4 native/go、p34_filters、p41_drawshadow native/go、
  base_pathshape 金图全 0.0000%。

## 4 残留登记（明确不在本项范围）

1. HDR 直出：10-bit/float swapchain + 色彩空间元数据（当前仓无窗口系统 HDR 元数据链）。
2. float MSAA：4x resolve 到 RGBA16Float 未验证；float 面当前 1x（D5）。
3. float 面进 layer/filter 链与纯 GPU 合成（当前经 tonemap→8bit 贴回呈现）。
4. 连续动画 HDR 的每帧成本（真窗为静态内容烘焙一次）。
5. 宽色域（Display P3 等）色彩空间语义。

以上均不阻塞 M5 登记的「目标格式概念 + float 离屏 RT + tonemap/曝光 pass + 呈现输出 +
双后端逐位」范围。

## 5 结论

M5 登记范围全部交付并有双后端真窗证据；台账 M5 行翻 ✅。
