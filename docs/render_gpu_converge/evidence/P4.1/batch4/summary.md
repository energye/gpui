# P4.1 批 4 收口报告：子项① MSAA>1 even-odd 裁剪 + go4x 黑洞修复（2026-11-10）

> 修了三个 GLES 4x 链上的存量洞 + even-odd stencil-invert 落地。
> 用户拍板：D1a（go4x 修进批 4）、resolve DRAW-only 修法、不归并 #3–#6、
> 插桩诊断后删、depth clear/range 按草案修两处、1x 全量金图回归。

## 0 一句话

MSAA=4 下 even-odd 裁剪孔洞从「被 winding stencil 错误保留」修为 Skia 标准
stencil INVERT 语义；途中定位并修掉同链两个更深的存量洞（离屏 resolve READ
绑定被毁=go4x 快照全黑、桌面 GL 深度清屏/映射按 float32 传参=深度门控从未
生效）。双后端 4x 真窗金图 0.0000%，P3.9b go4x「登记不可验」格翻绿。

## 1 三洞与修法（file:line）

| # | 洞 | 位置 | 修法 | 依据 |
|---|---|---|---|---|
| 1 | 离屏 MSAA resolve 自拷贝 → 一切 4x 离屏读回全零（真窗快照全黑、无头 4x 内容丢失） | `gpu/gwgpu/gles/command_execute.go` `ensureResolveFBO` 首用分支（HEAD :259） | 裸 `BindFramebuffer(gl.FRAMEBUFFER)` → `gl.DRAW_FRAMEBUFFER`（READ 保持 msaa FBO） | GLES3 §4.4.1（FRAMEBUFFER 同时设 READ+DRAW）；Skia GrGLGpu resolve 显式双绑纪律；仓内 toSurface 分支=正确孪生 |
| 2 | even-odd 孔洞被 winding stencil 保留（本批主目标） | `render/internal/gpu/depth_clip.go` | 新增 `eoStencilFillPipeline`（双面 PassOp=Invert，sampleCount>1 才建），`BuildClipResources` 携 fillRule，`RecordDraw` 按 group.ClipFillRule 分流；Phase 2 cover 的 NotEqual-0 门控天然复用；sampleCount==1 mask 路与 winding 管线零变化 | Skia `GrPathStencilSettings.h:18-26` gEOStencilPass=kInvert / `:29-37` gEOColorPass=kNotEqual |
| 3 | 桌面 GL 上下文 `glClearDepth`/`glDepthRange`（double 签名）被按 float32 FFI 传参 → 深度清屏≈0、深度范围 [≈0,≈0] → GreaterEqual 深度门控自上线起在桌面 GL 从未生效（1x 被 R8 mask 掩盖，4x mask 跳过才暴露） | `gpu/gwgpu/gles/gl/context_linux.go:55-58/857-885` | 新增 `cifVoid1Double`/`cifVoid2Double`，ClearDepth/DepthRange 按 `isGLES` 分派 float/double | GL 规范签名：桌面 glClearDepth(GLdouble)/glDepthRange(GLdouble,GLdouble)；GLES glClearDepthf/glDepthRangef。全文件审计仅此两处 double 签名 |

详证：`go4x-diagnosis.md`（洞 1）、`depth-clear-range-diagnosis.md`（洞 2/3 判别实验链）。

## 2 先红后绿（能力单测）

- `render/clip_evenodd_msaa_test.go`（新增；数据 `render/testdata/clip_evenodd_msaa_cases.json`）：
  - 红证据：evenodd_ring_clip_msaa4 GPU 两孔探针得红（洞未挖）、TestClipEvenOddMSAAMatchesCPU
    gpu=红 cpu=绿（会话记录 + negative/ 目录红输出同证）。
  - 绿：`TestClipEvenOddMSAACasesGPU`（evenodd 孔洞挖出 + winding 锁定）、
    `TestClipEvenOddMSAACasesCPU`（CPU 恒绿护栏）、`TestClipEvenOddMSAAMatchesCPU`
    （GPU==CPU）——`unit-tests/eo-msaa-gles-resolve-green.txt`。
- `render/gles_msaa_resolve_test.go`（新增；数据 `render/testdata/gles_msaa_resolve_cases.json`）：
  go4x 洞红绿对——红=修复前 4x 离屏读回内容全丢（实心矩形消失），绿=修复后内容到位；
  1x 对照恒绿。同文件 `unit-tests/eo-msaa-gles-resolve-green.txt`。
- 洞 3 无独立单测：由 even-odd 4x 绿测承担（深度门控是其前提）；负向 B 单独红证。

## 3 真窗（双后端 + 负向 + 人工）

`examples/ui_wr_p41_evenodd_msaa`（新增，`-auto-only` + 常驻人工窗二合一；
开窗前 SetMSAASampleCount(4)）四卡：A even-odd 环孔 / B winding 锁定 /
C {5/2} 五角星自交中心孔 / D 孔洞透条纹下层。

- native 4x：logic 3/3 + pixel 12/12 + golden 0.0000%/763584px（seed→verify 双跑）。
- go/GL 4x：logic 3/3 + pixel 12/12 + golden 0.0000%/763584px（seed→verify 双跑）。
  —— 三修复在真窗同窗生效。
- 跨后端：433/960000 px（0.0451%）AA 级差（MSAA resolve 实现差异，按后端分基线，
  底座存量口径登记）；各自基线逐位 0。
- 人工窗：`-manual-seconds 5` rc=0 正常退出（windows/manual-run.txt）。
- 负向（示例层）：`GPUI_P41E_BREAK=nonzero` → 孔洞探针 5 处红、exit 1（windows/
  negative-nonzero-break.txt）；还原复绿。引擎级负向见 §4。

## 4 负向对照（引擎级，红→还原绿）

- negA `eo-to-wind`（RecordDraw 强制 even-odd 走 winding 管线=现状洞）→
  TestClipEvenOddMSAACasesGPU 红 exit1 → 还原绿（negative/negA-*.txt）。
- negB `depth-clear-revert`（ClearDepth 还原 float32 传参=深度门控死）→ 同测试红
  exit1 → 还原绿（negative/negB-*.txt）。
- go4x 洞负向：DRAW-only 改回 FRAMEBUFFER（洞行为）→ TestGLESMSAAOffscreenReadback 红
  → 还原绿（会话记录，修复本体由 gles_msaa_resolve_test.go 常驻锁定）。

## 5 门禁与回归

- 触及包按测试文件：render/internal/gpu TestDepthClip ✓、render Clip 族 ✓
  （TestClipRRectGPUTextureAndText HEAD 同败=先在红，登记别线）、gles 包 ✓
  （TestPresentDamageX11 交接在册环境红）。
- `-race`（CGO）：render 新测试组 ✓、render/internal/gpu TestDepthClip ✓、gles 包
  无 DATA RACE（唯一红=在册 PresentDamageX11）。
- vet rc=0；apidoc rc=0（**零公开 API 变化**：eo 管线/BuildClipResources 签名均为
  internal，catalog 不动）。
- **1x 全量金图回归（桌面 GL 深度门控首次激活）**：15 存量窗 × 双后端串行，
  native 14 绿 + go 13 绿（golden 0.0000%）；两处先在红双后端 HEAD 同败非本批引入：
  p31_linear_blend 0.7278%（别线）、ui_wr_p41_mesh_paint_blend B 卡（批 1 #3 在册
  挂账）。**零新增红**——深度门控激活在 1x 被证明视觉中性（golden-regression-1x.txt）。
- **P3.9b 显存复核四组合**：native 1x/4x、go 1x/4x 全部 logic 6/6 + golden 0.0000%
  + surface 62.8/76.3MB（4x 峰值与 P3.9b native 记录一致，无回归）
  （windows/p39b-*.txt）。**go4x 金图补种**：`p39b_anim_go_msaa4.png` 已 seed+verify
  ——P3.9b 载体块「go4x 因全黑洞不 seed」的登记格正式翻绿。

## 6 边界与登记

- metal 不可验（无 macOS 真机，交叉编译仅编译通过口径）。
- 跨后端 433px AA 级差登记（按后端分基线既有口径）。
- #3/#4/#5/#6（GPU 层链族）确认与三洞不同根（1x 双后端可复现 vs 桌面 GL 深度链），
  维持归并诊断批建议，未在本批扩战线。
- 诊断期临时插桩（GPUI_GO4X_DEBUG/GPUI_EO_DEBUG/探针）已全部还原删除，
  证据落盘本目录。
- 示例层教训登记：窗面 SetPath=raw 拷贝（不吃当前矩阵，文档明示），带变换上下文
  必须 DrawPath——even-odd 窗首跑踩中，改 DrawPath 后正确（非引擎洞）。
- 五角星首版用 10 点交替轮廓（简单多边形，两规则都无孔）属测试预期错误，改 {5/2}
  跳点连线后符合 even-odd 语义（已在窗注释登记）。
