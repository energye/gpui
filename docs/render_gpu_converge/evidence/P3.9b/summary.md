# P3.9b 交付记录：1x/4x 真实窗口动画性能基准（2026-11-09）

> 真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2/§2.1/§7、`PLAN.md` §7.4；方案 `assessment.md`、`design.md`。
> 结论：**测量补充完成**。满屏 1080p 混合通用动画真窗，1x/4x × native/go 四轮串行测量，拿到有效 1x↔4x 对照数据；**同时抓到并如实落位一条既有引擎洞（go/GL+4x 整窗全黑），不修、不标绿、不进结论**。

## 0 直话

- 用户问「4x 很占内存和性能吧 / mesh 和 4x 的使用场景」→ 补了一份**满屏 1920×1080、每帧全量重绘、通用复杂动画**真窗下的 1x/4x 对照，而不是静态小窗推断。
- **答案**：4x 在这台 940MX/1GiB + 满屏复杂动画下代价很实在——显存峰值 62.8→147.8 MB（2.35×，超预算触发淘汰）、帧率 55.95→42.01（−24.9%）、p95 22.3→27.45ms（破 22ms 红线）、hitch 2→8；首帧/RSS 几乎不变。默认 1x 是对的。
- **抓到洞**：go/GL + 4x MSAA 整窗全黑（既有存量洞，本库首次认真在 GLES 上测 4x 才暴露）。用户拍板「不修洞、继续测完、如实登记不可验」，P3.9b 保持零引擎改动。

## 1 交付物

- 新真窗 `examples/ui_wr_p39b_msaa_anim/`：满窗即内容的混合通用动画场景（3×3 mesh gradient 背景、三条贝塞尔曲线、四张旋转圆角卡片带文字、旋转缩放图片、半透明 SaveLayer 圆盘、40 粒子、线性/径向渐变、锚点色卡、HUD 指标行浮在内容上）；每帧 `PresentPolicyFullPaint` 全量重绘、相位持续推进。
  - 分辨率可配 `GPUI_P39B_W/H`（默认 1920×1080，留 3840×2160 档本机不跑）；MSAA 可配 `GPUI_P39B_MSAA=1|4`（开窗前 `SetMSAASampleCount`）。
  - 三模式：`-auto-only`（冻结相位门禁三证据）、`-measure`（动画测量出性能 JSON）、无参数（常驻人工窗，收真事件打日志改标题、关窗正常退出）。
  - **零引擎改动、零新增公开 API**（`RENDER_API_CATALOG.md` 未改，`apidoc` rc=0 验证）。
- 串行测量脚本 `run_measure.sh`：一轮一窗、每轮前健康检查（gnome-shell / wayland-0 套接字 / 崩溃签名）、程序自己 `RunFor` 正常退出、不并发、不加速、不硬杀、异常即停批。

## 2 验证结果（三证据）

| 项 | 结果 | 证据 |
|---|---|---|
| 门禁 3 有效组合（native 1x/4x、go 1x） | 金图逐位 **0.0000%**、logic **6/6**、非黑 **23504/23504** | `gate/*_verify.json` |
| 门禁 go4x | 预期红：`nonblack=0`、`logic=5/6`、exit 1、标记 INVALID | `gate/go_msaa4.INVALID` |
| 负向对照 | `GPUI_P39B_BREAK=freeze` → phase_advanced 红、exit 1 | `negative/freeze.json` |
| 4x 生效证据 | `msaa_setting==4`、显存峰值 2.35×、**无 OOM 降级日志** | `compare.md`、各 `.txt` |
| 三有效组合测量（18s） | 正常退出、拿全 fps/p95/hitch/显存/首帧/CPU/RSS | `measure/*.json` |
| go4x 测量 | 黑屏 INVALID、数字排除出结论 | `measure/go_msaa4.INVALID` |
| vet / apidoc / 五目标构建 | 全 rc=0 | `gates/` |
| 金图/快照 SHA | native1x 冻结快照与金图逐位相同 | `window/`、`golden-sha256.txt` |
| 常驻人工窗长跑（native 1x，431s / 25015 帧） | fps_interval **58.07**、p95 **20.83ms**、hitch **1.95/min**（<5 红线）、RSS 斜率 **251.7 KB/min**（valid，无泄漏迹象）、logic 6/6、非黑 23504/23504、零 CPU 回退、`close event (wayland)` 正常退出 | `/tmp/p39b_manual.json`（关窗自动 dump，`manual_events=217` 真实交互） |

## 3 有效结论（native 1x → 4x，同后端干净对照）

完整表见 `compare.md`。要点：

- 显存峰值 **2.35×**（62.8→147.8 MB），超 128 MiB 池预算 → 5 次淘汰。
- 帧率 **−24.9%**、p95 **+23.1%**（22.3→27.45ms）、hitch **+300%**。
- 首帧 +4.6%、RSS +0.1%、CPU 光栅占比 +5.8%（CPU 一直是主瓶颈）。
- 门禁：1x `fps=55.95` 勉强 PASS、`p95=22.30` 压线；**4x `fps=42.01` FAIL FPS 门禁、p95 破线**。
- 与上一轮静态小窗诊断（帧时不变）的差异：**满屏动画把 GPU 填满，4x 代价才暴露**；静态小窗「没掉帧」不能外推。

## 4 已知边界 / 如实登记

- **go/GL + 4x 整窗全黑**：既有引擎洞（GLES 4x resolve/readback），稳定复现、定位线索见 `findings-go4x-black.md`。用户拍板不修、登记为不可验；该组合**不标绿**、性能数字不进结论。不新建 GLES 修复 skill、不改引擎。
- **4K 档（3840×2160）本机不跑**：1 GiB 显存下 4K 4x 必 OOM 降级/驱动崩溃风险；代码留可配、如实登记原因。
- **metal 不可验**（无 macOS 真机）。
- 1x 的 p95=22.30 已略超 22ms 红线：满屏复杂动画对 940MX 本身偏重，非 4x 引入。
- 内容实际 1868×1021（窗口装饰缩小），场景与探针按真实 content 尺寸计算。

## 5 文件清单

- 新增：`examples/ui_wr_p39b_msaa_anim/`（main.go、testdata/快照、goldens/ 3 份有效金图）、`evidence/P3.9b/`（assessment、design、findings、compare、summary、run_measure.sh、gate/、measure/、negative/、window/、gates/、golden-sha256.txt）。
- 修改：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md`（§2 加 P3.9b 行、§2.1 载体、§7 流水）、`docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md`（§7.4）。
- **未改**：引擎所有包、`render.Mesh*`、`gpu/hal`、WGSL、MSAA 默认值、`ui/kit`、`RENDER_API_CATALOG.md`。
- 未动：别线的 `showcase_showcase.png` 与所有未跟踪文件。

## 6 完成判定

- ✅ 测量补充完成：三有效组合真窗正常退出、各拿到有效性能 JSON、逻辑探针全绿、4x 显存差异可观测、对照表与结论落盘、三证据齐。
- go4x 洞如实登记为不可验（不粉饰改绿）；4K 档本机不跑如实登记。
- 下一步方向（本会话未开）：若要修 GLES 4x 洞，需切到对应线、补齐「定位到行号 + 标准方案」后按 skill 流程走。
