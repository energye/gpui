# P2.6 行为零变化总验收 — 收口报告（2026-11-01）

> 依据：TASKS §3 P2.6（P1.1–P1.8 全量重放逐项对照；未绿不得进 P3）。拍板：真窗重放=:3 Xwayland 同基线环境；I3/I4 不阻塞重放；a→h 分段。

## 1 八项重放终值

| P1 项 | 重放结果 | 判定 | 证据 |
|---|---|---|---|
| P1.1 五目标构建 | linux×2/windows/darwin×2 全绿（已知破包除外） | ✅ | `build/*.txt` |
| P1.2 go vet | 默认 56 / nogpu 56（全部 ui/platform 线外，与 P2.5 基线一致） | ✅ | `vet-replay.txt` |
| P1.3 逐测试文件 | 323 包 1438 行：**1165 PASS / 17 FAIL（15 条 P1.3 同红 + 2 条 A/B 定性非 P2 回归）/ 48 SKIP / 208 无测试**；基线 48 红已消除 33 个 | ✅ | `b-tests/per-file-tests/` |
| P1.4 全部真窗 | **374 轮同规模零缺漏：同红 100 / 双向一致 244 / 转绿 20 / 嫌疑 0**（5 条嫌疑全部定性为编排环境还原不全或瞬态，复跑 PASS） | ✅ | `c-windows/summary-clean.tsv` |
| P1.5 API 快照 | 352→323 包；删 31 包全落 PLAN §7.7（naga 14+shader 三后端 16+render/render）、增 2 包（gpu/hdr）；共有 322 包 doc 逐字节一致、19 包差异逐包对应 P2 各步登记（§3 表）；测试计数文件数精确 -167、函数 -3916（96 为 P1.5 统计口径差） | ✅ | `api-reconcile.txt` / `api-diff-full.txt` |
| P1.6 显存账本 | ladder 峰值 373.8MB（基线 372.0，带内 +0.5%）；oom `ledger_recovered=true`；multiwin crash_free；batch 转绿 | ✅ | `d-vram/vram-reconcile.md` |
| P1.7 性能 | p95_fail=0 / fps_fail=0 / 首帧 0 / 门禁失配 0；逐轮 p95 **零恶化**、10 轮改善（backdrop/clipop/maskops 等） | ✅ | `e-perf/perf-reconcile.md` |
| P1.8 负向对照 | 三破坏全红（偏移 1px coverage 0.92/0.87≠1.00；跳绘整块消失；K1 注入 steady delta=2/131072 与 P2.4c 原签名逐字一致）+ §8.2 阈值变异对（30→0 漏报 / 22→4 报警）；全部还原零残留 | ✅ | `f-negative/negative-final.md` |

**总判定：P2.6 通过，未绿项为零。P2 全阶段（P2.1–P2.6）行为零变化成立，可进 P3。**

## 2 P2 全阶段行为对账的三个「与基线不同」维度（全部正向或登记）

1. **转绿 20+33**：真窗 20 轮、单测 33 文件在基线是红的、现在绿——P2 修复的实际效果（nil ImageCache、共享布局、m5、T1、性能超限窗等），逐条可溯源到 P2.4c/d/e 的修复提交。
2. **同红 100+15+2**：真窗 100 轮、单测 15 文件基线同红放行；2 条（gles DeviceLostErrorScopeStubs、X11KeyRepeat）A/B 定性为 P2.4e 收口后浮现、与 P2 无关，立案随尾巴诊断。
3. **转差 0**：真窗嫌疑 0、p95 恶化 0、账本回退 0、门禁失配 0。

## 3 API 快照差异 → P2 阶段归因表（19 包）

| 包（差异行数） | 归属 |
|---|---|
| render (195) | P2.4a OpenStandaloneDevice/DeviceDescriptorForAdapter；P2.4b DefaultBudget/FairSplit/Budget 域；P2.4e PresentFramePlan |
| render/scene (85) | P2.4e e2 clip 簿记收敛 + e3 blend 边界方法 |
| render/gpu (11) | P2.4d TexturePool 删除面/A4 保留面 |
| render/internal/blend (98) | P2.4e e3 值域合一（三表删二） |
| render/internal/gpu (3003) | P2.3 老渲染器删除 + P2.4a–e 池/预算/令牌/呈现面 |
| render/internal/gpu/res (279) | P2.4c registry + P2.4d Cache/Owner/Gate |
| render/internal/image (6) | P2.4c 释放点后移 |
| ui/raster (12) | P2.4e e4 hub 投递（RunGLSync 导出） |
| ui/scheduler (22) | P2.5 A2 FPSWall/TargetHz + interval_* |
| gpu/shader (89) / ir (38) / wgsl (13) / lower (10) | P2.1 naga 迁移（LowerWithCapabilities 等） |
| gpu/rwgpu (54) | P2.4c/d FutureForget/CompletionToken |
| gpu/hal (45) / noop (4) | P2.4d token + P2.5 释放开关 hook |
| gpu/webgpu (11) | P2.4d token 面 |
| gpu/gwgpu/gles (24) | P2.4d token 面 |
| examples/wrgate (32) | P2.4d pool 探针段 |

## 4 编排器与口径（P1.4/P1.6 runner 重建）

P1.4/P1.6 的采集 runner 未入库（P1.6 的在 /tmp 已丢）——本轮重建为正本 `tools/window_replay.py`（P2.6 evidence 副本）+ `start-xwayland.sh`：轮次=基线 summary.tsv 183 对×双后端、args/RUN_SECONDS/env 按基线证据还原（ROUND_OVERRIDES：multiwin 75s / oom GPUI_FAULT_OOM=180:1 / ladder 300s）、0* 金图对冻结基线副本逐位 diff、断点续跑、瞬时态吸震重试（retry 记录在案）。过程中修复三处编排 bug（bytes 写入/json 初值/相对路径与 WAYLAND_DISPLAY 泄漏），负向还原事故一次（grep 猜还原目标拷错文件，git 当场恢复，教训入 f 报告）。

## 5 遗留（不阻塞 P2.6 ✅）

- **g 段 I3/I4/C4/T2 诊断**：R20 灰度矩阵重录路径、SnapshotAsync 黑图、四窗 rc=1 与 JSON 自报矛盾——基线同红放行，诊断独立立项（结论回写台账 §2.2）。
- ui 5 既有红（app_test SIGSEGV 等）+ b 段 2 条新立案（gles DeviceLostErrorScopeStubs / X11KeyRepeat）——归尾巴专项。
- extract_perf 正本的双代别名读取：待下一轮基线冻结替换 P1.4/P1.7 冻结证据时退役。
