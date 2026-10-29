# render/gpu 收敛任务台账（2026-10-28）

> 性质：**可执行任务清单 + 状态台账 + 同步规则**。新会话照本文件即可开任务。
> 方案依据与理由（标准选型、删除/合并决策、双后端矩阵、缺失/半成品登记）在 `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md`；本文件不重复论证，只回答「有哪些任务、先后关系、怎么算完成、现在到哪一步」。
>
> **本文件是任务状态的唯一真源。** 外部任务列表（工具里的 task list）是本文件的镜像；两者冲突时以本文件为准，并在当次会话内把镜像改回一致。

## 0 新会话开工协议（照顺序做）

**开场话术（新会话直接粘贴这段）：**

> 按 `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` 继续 render/gpu 收敛任务。先读 `AGENTS.md`、`docs/render_gpu_converge/00_README.md`、`docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md`、本文件；按 §2 重建任务列表镜像，从第一个未阻塞任务开工；开工前在 §2.1 登记能力单测与真窗载体并核对 §3 条件，完成后按 §6 交齐三证据、单测与双后端结果；遇到「停下报告」节点必须停下确认。

**协议步骤：**

1. 读 `AGENTS.md`（项目硬规则）。
2. 读 `docs/render_gpu_converge/00_README.md`（本批文档索引）。
3. 读 `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md`（方案与门禁口径，尤其 §7）。
4. 读本文件 §2.1（当前焦点）与 §2（任务总表），找到**第一个未完成且未被阻塞**的任务（状态 ⬜，依赖项全 ✅）。
5. 跑 `git status`：有非本任务线的未提交改动，先报告归属，不提交、不覆盖、不顺手清理。
6. 核对台账与任务列表镜像是否一致；不一致先按本文件修正镜像，再开工。
7. 开工前确认该任务的「开工条件」全部满足；不满足就停在条件项，不硬开。
8. 完成后按 §6 同步规则更新：本文件状态 → 证据落盘 → PLAN 修订（若口径有变）→ 任务列表镜像。
9. 新会话禁止凭记忆或聊天记录判断进度，只认本文件与证据目录。

## 1 状态图例

| 符号 | 含义 |
|---|---|
| ⬜ | 未开始 |
| 🟡 | 进行中（已开工，未过全部门禁） |
| ✅ | 完成（门禁全绿 + 证据落盘 + 本文件已更新） |
| ⛔ | 阻塞（写清阻塞原因与解除条件） |
| ⏸ | 主动暂停（用户决定，写清原因） |
| ➖ | 不适用 / 取消（写清依据） |

**标 ✅ 的硬条件**（缺一不可）：

- 该任务「完成标准」里的每一项都有对应证据文件（路径写进台账）。
- **该任务的能力单测与对应真窗都实际跑过**（§4.1），单测逐文件 PASS，真窗三证据齐。
- 门禁命令实际跑过，输出或摘要落盘；不是"应该能过"。
- 行为零变化阶段（P1/P2）与基线对比的差异逐条对上白名单（PLAN §7.7）；对不上就不能标 ✅。
- 三证据（逻辑探针 + 像素断言 + 金图）齐全；只有逻辑指标绿不得标绿。
- 跨平台项：做了哪端写哪端；没做的端按符号标注原因（占位 / 实现中 / 本机不可验），不留空白格。

## 2 任务总表（状态唯一入口）

### 2.1 当前焦点

- 当前进行中：**无**（P2.4a 已完成，提交 `55495fdd`，未推送）
- 下一任务：P2.4b（依赖 P2.4a ✅）；**正式基线 tag `baseline-p1` 已打在 `92ca9216`**——P2 从此点起分支，回滚锚点与 P2.6 零变化对账基准，详见 `baseline/rollback.md` §7
- 最近完成：P2.4a 设备创建/选卡收敛（三份 → 一份）——`webgpu.CreateInstance` 白名单 3→1（`render/backend.go createNativeInstance` 唯一句），新增导出 `render.OpenStandaloneDevice` 唯一独立入口，gpu_shared/vello 两处独立建链改调；删 device.go 镜像 2 组 + 死代码 6 符号；五目标构建绿、vet 61/58 逐条不变、逐测试文件 321 个（28 FAIL 全定性既有）、API 快照对账、负向对照红过复绿、-race 过；A3 解剖关闭（显存 OOM 假 Validation Error）、C7 根因落定修复归 P2.4e，见 `evidence/P2.4a/summary.md`
- 阻塞项：无（生命周期 multiwin go 档撞 C7 既有根因，native 档账本稳态 PASS）

**P2.4a 收口载体（2026-10-30 已完成，评估见 `evidence/P2.4a/assessment.md`，结果见 `summary.md`；D1–D5 经用户拍板）**

- 结果：创建入口唯一化（backend.go `createNativeInstance` 单句）+ 独立入口 `OpenStandaloneDevice`（严格 Vulkan 掩码、env 不劫持、policy 选卡、OOM 重试、半开回收）+ 重试助手归一；删 `render/internal/gpu` 9 符号（2 镜像 + 6 死码 + 常量）；`render/gpu` 包装标实验（D5）；vello 获 policy 选卡（D1 语义收敛，本机选卡不变）+ Close 补 adapter 释放（原局部句柄泄漏）
- 能力单测：`render/device_open_test.go` 5 用例（重试 4 + 真机探针）PASS；双分支真机验证（RenderSceneAuto/compute_pipeline 例 + TestGoldenSoftware_GPUvsCPU/TestLinearDiagonalRamp）；逐测试文件 321 个（280 PASS / 12 仅编译 / 1 标签排除 / 28 FAIL = 27 与 P1.3 逐文件一致 + 1 并发竞态单跑 7/7 PASS）
- 真窗：ui_l1_blank native+go 双档（嵌套 Wayland 隔离，rc=0，3582/3596 帧）；multiwin native `-auto-only` 8 窗三相位 PASS（rss 稳态 316MB）
- 金图：ui_l1_blank 设计内无金图（baseline/windows.md 记载），替代判定 = JSON 探针 + P1.4 对账；逐位对账归 P2.6
- 双后端：webgpu native 真窗 ✅；gwgpu-gles go 档真窗 ✅（multiwin go 第 8 窗撞 C7 既有根因，与本步无关）；gwgpu-metal 本机不可运行（无 macOS），darwin×2 交叉编译 ✅
- 负向对照：探针引用已删符号 → 编译红 5 处；撤销复绿零残留；漏斗唯一 grep 守卫
- 解剖产出：`a3-dissection.md`（A3 关闭：显存 OOM 假 Validation Error，46/46 同因）、`c7-dissection.md`（X11 机制链落定 + Wayland 第二机制待解剖，修复归 P2.4e）
- 审计发现：**P2.1/P2.3 的 `render/internal/gpu` 逐文件轮为无 GPU 假绿**（脚本未带 `WGPU_NATIVE_PATH`，GPU 用例全 SKIP 文件级 rc=0）；本轮带 lib 恢复 P1.3 纪律；待办池 H1 登记，P2.6 重放必须修正

**P2.1 收口载体（2026-10-29 已完成，评估见 `evidence/P2.1/assessment.md`，结果见 `summary.md`）**

- 结果：迁移 144 文件 / 116,060 行（根 `naga.go`→`shader.go`，D1）；删 `gpu/shader` 独有三后端 167 文件 / 132,395 行与 `gpu/gwgpu/naga` 目录；根 SPIR-V 5 符号 + `CompileShaderToSPIRV` 删除（D3）；`GLSLVersionToNaga` 名保留（D2）；补根冒烟测试（D4）
- 能力单测：naga 70 文件 / 2,139 Test / 21 Bench 全部随迁跑绿 + 新根冒烟；等价证明：GLSL/MSL 逐字节快照 681 文件 `diff -r` 为空（27 WGSL / 48 入口点 × GL330/430/450/ES300/ES310/ES320 + MSL 默认/mslmap）+ GL 离屏 md5 `59ca40337ff5e016579ea53a09549824` 不变
- 真窗：本步零行为，按 §4.1 例外不新增；全量真窗重放归 P2.6
- 金图：无新增；像素级等价用 `shader_pipeline_offscreen_test.go` md5 对 P1.3 基线
- 双后端：webgpu WGSL 直通不变；gles 真实 GL 离屏 + GLSL 快照；metal 本机无 Metal，用 darwin 交叉编译（含测试文件 `go vet`）+ MSL 快照，真机归 P2.6/macOS
- 负向对照：探针 import `gpu/shader/spirv`、`gpu/gwgpu/naga`、根 SPIR-V 符号、`CompileShaderToSPIRV` → 4 处全红；撤销复绿零残留
- 证据：`evidence/P2.1/`（`assessment.md`、`summary.md`、`snapshot-*`、`build/`、`vet-*.txt`、`per-file-tests.txt`+notes、`api-diff.txt`、`grep-residual.txt`、`negative.txt`、`darwin-test-compile.txt`、`gl-offscreen-pixel.txt`、`tools/`）

> 每次状态变更必须同时更新本小节；新会话从这里接着做，开工前先把上面这块换成新任务的载体登记。

> 依赖列里的编号指本表任务号。表内状态是唯一权威，更新时改这一列并同步 §7 状态流水。

| 编号 | 任务 | 依赖 | 状态 | 证据目录 |
|---|---|---|---|---|
| P0.1 | 验收阈值与环境清单（首帧/滚动帧率/p95/p99/显存上限/金图容差，写死数字并指明真源） | — | ✅ | `baseline/thresholds.md` |
| P0.2 | 真窗与金图清单（窗口名 → 金图 → 平台 → 容差 → 单窗/批量 → 覆盖能力标签） | — | ✅ | `baseline/windows.md` |
| P0.3 | 归档与回滚演练（打 tag/建分支，实际回退一次证明可回） | — | ✅ | `baseline/rollback.md` |
| P0.4 | 双后端一致性测试台（同场景 webgpu/gles/metal 逻辑断言，不比像素只比语义） | — | ✅ | `baseline/conformance/` |
| P1.1 | 六目标交叉编译 + `-tags nogpu` | P0.1–P0.3 | ✅ | `baseline/build/` |
| P1.1b | 跨平台构建修复（B1–B6，用户决定插队：P1.2 前完成并复跑 P1.1） | P1.1 | ✅ | `evidence/P1.1b/` |
| P1.2 | `go vet ./...` | P0.1–P0.3 | ✅ | `baseline/vet.txt` |
| P1.2b | nogpu 测试标签修复（P1.2 采集发现：20 个测试文件补 `//go:build !nogpu`，用户决定插队） | P1.2 | ✅ | `evidence/P1.2b/` |
| P1.3 | 逐测试文件运行，记录 PASS/SKIP/FAIL（SKIP 带原因） | P0.1–P0.3 | ✅ | `baseline/tests/` |
| P1.4 | 全部真窗：JSON 探针 + PNG + 金图 diff 计数 + 容差声明 | P0.1、P0.2 | ✅ | `baseline/windows/` |
| P1.5 | 每包公开 API 快照（`go doc -all`）+ 测试计数快照 | P0.1–P0.3 | ✅ | `baseline/api/` |
| P1.6 | 开窗/关窗/压测后显存账本快照 | P0.1、P0.2 | ✅ | `baseline/vram/` |
| P1.7 | 首帧、滚动帧率、p95/p99 性能基线 | P0.1、P0.2 | ✅ | `baseline/perf/` |
| P1.8 | 负向对照（改 1px / 少画一次 / 漏释放一次，确认门禁会红） | P1.3–P1.7 | ✅ | `baseline/negative/` |
| P2.1 | shader 合并：naga 迁顶层 `gpu/shader`，删旧副本与 SPIR-V/HLSL/DXIL | P1.8 | ✅ | `evidence/P2.1/` |
| P2.2 | 删 `render/render` 旧原型；`FloatTarget` 迁 `render/hdr` | P1.8 | ✅ | `evidence/P2.2/` |
| P2.3 | 删老一代 GPU 场景渲染器闭合簇 | P1.8 | ✅ | `evidence/P2.3/` |
| P2.4a | 设备创建 / 选卡收敛（三份 → 一份） | P2.1–P2.3 | ✅ | `evidence/P2.4a/` |
| P2.4b | 预算统一（六口径 → 一套 Budget，按域分账） | P2.4a | ⬜ | `evidence/P2.4b/` |
| P2.4c | 延迟释放统一（三队列 → `res/registry`） | P2.4b | ⬜ | `evidence/P2.4c/` |
| P2.4d | 同尺寸纹理分配收敛（六处 → `res/cache`） | P2.4c | ⬜ | `evidence/P2.4d/` |
| P2.4e | 裁剪 / 混合枚举 / 呈现入口收敛 | P2.4d | ⬜ | `evidence/P2.4e/` |
| P2.5 | 包组织收口：`go list` 对比、分层约束测试、索引与总账同步 | P2.4e | ⬜ | `evidence/P2.5/` |
| P2.6 | 行为零变化总验收（P1 基线全量重放，是进入 P3 的硬闸） | P2.5、P0.4 | ⬜ | `evidence/P2.6/` |
| P3.0 | 依赖图 + 文档更新（PLAN §5 + `RENDER_API_CATALOG.md`），再实现 | P2.6 | ⬜ | `evidence/P3.0/` |
| P3.1 | 色彩管理地基（含线性混合接线，原半成品前移） | P3.0 | ⬜ | `evidence/P3.1/` |
| P3.2 | 自定义 shader（依赖 P2.1 翻译器 + 新公开 API） | P3.0、P2.1 | ⬜ | `evidence/P3.2/` |
| P3.3 | 富文本（含 `DrawParagraph` 逐 run 样式，原半成品前移） | P3.0、P3.1 | ⬜ | `evidence/P3.3/` |
| P3.4 | MaskFilter / ColorFilter / ImageFilter 家族 | P3.0、P3.1 | ⬜ | `evidence/P3.4/` |
| P3.5 | 路径与几何：conic、四角圆角、Matrix4 透视、路径效果、布尔保曲线 | P3.0 | ⬜ | `evidence/P3.5/` |
| P3.6 | 图片格式（GIF/APNG/动图 WebP/AVIF/HEIF/EXIF）+ GPU mipmap | P3.0 | ⬜ | `evidence/P3.6/` |
| P3.7 | 2.5D 透视相机、世界高度/精灵 z 序、自定义材质、HDR/泛光 | P3.0、P3.1、P3.5 | ⬜ | `evidence/P3.7/` |
| P3.8 | SVG 完整语义、彩色字形、嵌套 Picture 引用 | P3.0、P3.3、P3.5 | ⬜ | `evidence/P3.8/` |
| P4.1 | 正确性：MSAA>1 even-odd 裁剪、分数裁剪 AA、顶点网格 alpha/blend、DrawShadow | P3.4、P3.8 | ⬜ | `evidence/P4.1/` |
| P4.2 | 一致性：GPU/CPU 模糊对齐 | P4.1、P3.4 | ⬜ | `evidence/P4.2/` |
| P4.3 | 性能：mask 临时纹理池化、backdrop 损伤区合成、抖动路径优化 | P4.1、P3.4 | ⬜ | `evidence/P4.3/` |
| P4.4 | 文本：COLRv1 完整层（若 P3.8 未覆盖） | P3.3、P3.8 | ⬜ | `evidence/P4.4/` |

**并行说明**：P0.1–P0.4 互相独立，可并行；P1.1/P1.2/P1.5 可在 P0.1–P0.3 完成后并行；P2.1–P2.3 互相独立，可并行；P3.2–P3.8 在 P3.0（及各自前置）完成后可分批并行，但每批合入前都要跑所属域回归。P2.4a–e 必须串行。P2.6 除 P2.5 外还必须等 P0.4 的一致性测试台就位。

### 2.2 P2 待办池（2026-10-29 建池）

> 用途：P1 期间「登记 P2」散落在 §7 流水与各 baseline summary 里，加上 2026-10-29 实现层对账审计的 4 项新发现，统一收到这里；P2 开工先从这里取项，不再靠记忆。
>
> 取用规则：取走一项 → 在 §2.1 按 §4.2 登记载体 → 对应 P2 任务状态改 🟡 → 完成后证据落盘、本行改 ✅（或写清转去哪个任务正文）；不许绕过载体登记直接修。
>
> 归位口径：审计 26 项发现里，未落到具体 P2 任务正文的进本池；其余在 `baseline/{tests,vram,perf,negative}/` 各 summary 与 `baseline/thresholds.md` 有案，不重复进池。合规面两条分层必查（`ui` 不直引 `gpu`、`render` 无后端特有类型）实测通过，无需进池。

| 编号 | 来源 | 现象 / 位置 / 证据 | 拟归属 | 状态 |
|---|---|---|---|---|
| A1 | 审计 | AGENTS.md:78 称「`webgpu.Vram*` 仅 Rust 记账、GL 窗暂不依赖」已过时：账本在 `gpu/hal/vram_ledger.go`，`gpu/rwgpu/texture.go:342` 与 `gpu/gwgpu/gles/resource.go:103/174` 双端真实记账，`gpu/webgpu/vram.go:20` 只转发查询 | 就地修正 AGENTS（本次提交） | ✅ |
| A2 | 审计 + P1.7 | 帧指标两代 json tag 并存：`ui/scheduler/metrics.go:58-61` 发 `avg_frame_interval_ms` + `frame_interval_p50/p95/p99_ms`，`examples/wrgate/report.go:32-33` 等发 `interval_p50/p95_ms`；旧代窗还缺 `fps_wall`/`target_hz`（6 窗 × 2 后端，见 `baseline/perf/summary.md` §4） | P2.5（接口全量切换制：一次切到 `interval_*` 并补齐 fps_wall/target_hz；切换前以 perf 别名列为准） | ⬜ |
| A3 | 审计 | 46 个 `RequestDevice unavailable` SKIP 的根因未解剖，首轮与干净重跑均复现（`baseline/tests/summary.md:13`；原始错误 `:135` `RequestDevice: Validation Error`） | P2.4a 解剖；解剖前不定修复方向 | ✅ 2026-10-30 解剖闭合（`evidence/P2.4a/a3-dissection.md`）：真因是显存 OOM——wgpu 把 RequestDevice 阶段的 "Not enough memory left" 包装成 Validation Error，46 例全同因；显存压力决定 OOM 浮现层级（P1.3 在 RequestDevice 层、2026-10-30 在 CreateBuffer 层、单独跑全 PASS），环境依赖非代码缺陷；`IsGPUOutOfMemory` 对包装后文案仍命中（生产重试/降级健壮），不安排修复 |
| A4 | 审计 + P1.1b | `render/gpu/gpu_nogpu.go` 是 B1 的空包补丁（163 个消费方 blank import 不动）；最小侵入性质与消解路径（消费方按能力门控 / 顶层包拆分）未登记 | P2.5 评估归位 | ⬜ |
| B1 | P1.6 | `baseline/windows.md:213` 的 `ui_base_vram_cap` 运行档写 A/60s，代码 `examples/ui_base_vram_cap/main.go:67` 是 `gateSeconds=300`；P1.6 证伪后未回填 | P0.2 勘误（baseline 只增不改：在 windows.md 修订表追加勘误行） | ⬜ |
| B2 | P1.4 | `ui_wr_r6_layer_anim` 表内金图 `golden/baseline_igpu.png` 与代码真源不一致（代码读 `R6_SNAP_DIR`，默认 `/tmp/r6_layer_anim/r6_final_base.png`，仓内图未被读取）；`baseline/windows/README.md:42-44` | P2.6 对齐真源/代码 | ⬜ |
| B3 | P1.4 | `ui_text_edit_accept` 的 script 金图自动模式不逐位比较 | P2.6 决策（补逐位或写清替代判定） | ⬜ |
| C1 | P1.4 | 14 窗自带 x11grab screen check 在 rootless Xwayland 抓黑图 → 环境 FAIL（`backdrop` p95≈137ms 也被拉高）；`baseline/windows/README.md:33-37` | P2.6 重放环境（隔离真 Xorg/可抓图显示） | ⬜ |
| C2 | P1.4 | `ui_wr_t_stress` BUILD_FAIL：引用不存在的 `ui/kit/button`、`ui/kit/icon`；`baseline/windows/README.md:41` | 组件线：kit `button`/`icon` 能力补齐后示例才能建（P2.3 已确认与老 GPU 簇无因果，2026-10-29 移出） | ⬜ |
| C3 | P1.4 | `ui_ime_probe` 不响应 WM_DELETE，人工探针按兜底 SIGTERM 记 PROBE | P2.4e（窗口关闭协议） | ⬜ |
| C4 | P1.4 重审 | T2：`ui_wr_r1_layout`/`r20_filter`/`r22_selection_stub`/`c6_savelayer_group` 两轮 rc=1 而 JSON 自报通过、金图 diff=0，病因未解剖（§3 已知问题 T2） | P2.6 前诊断 | ⬜ |
| C5 | P1.6 | 10 条 PASS 记录 RSS 稳态斜率过 30000KB/min 泄漏线（`r17_bg_throttle` 88801 最高）；`baseline/vram/summary.md` §3 | P2 判定窗门禁集是否含 RSS 线 | ⬜ |
| C6 | P1.6 | `ui_base_batch_cap` p95 双后端 55.7/62.1ms 超 22ms 门禁（P1.4 同因同量级） | P2 性能线 | ⬜ |
| C7 | P1.6 | `ui_base_vram_cap` ladder 的 go 档在 GL 第 4 窗起 `eglMakeCurrent 0x3002`，8 窗阶梯跑不了；`baseline/vram/summary.md:18` | P2.4a/P2.4e（gles 多窗上下文） | 🔍 2026-10-30 根因已解剖（`evidence/P2.4a/c7-dissection.md`）：开窗的 EGL 绑定操作跑在控制器线程，撞上唯一 GL 线程（ui/raster/hub）的 Submit keep 窗口期 → EGL 单绑定拒绝；竞态非确定窗号（P2.4a 生命周期复跑在嵌套 Wayland multiwin 第 8 窗同码复现——Wayland 为 context-per-surface，具体触发路径是**第二个待解剖机制**，随 P2.4e 一并解剖）；修复归 **P2.4e**（X11 推荐方案：开窗 GL 步骤投递 hub FrameJob），P2.4a 建设备收敛与本根因无交集 |
| D1 | P1.3 | T1：`gpu/rwgpu/queue_workdone_test.go:45` 失败后 cleanup `Release()` 在 `device_misc.go:313/314` SIGSEGV（原始栈 `baseline/tests/failures/queue_workdone_test.log`） | P2.4c/P2.4e | ⬜ |
| D2 | P1.3 | 48 FAIL 分组归属：老 backend 并发 SIGABRT→P2.3；StubHost 假 X11 句柄→P2.4e；nil ImageCache / 共享布局未 pin / 无 GPU 前置 / depcheck 测试缝→P2.5；video 3 文件 + 顺序耦合→P2.6；CPU 彩色字形无 ink→P2.3（`baseline/tests/summary.md` §4） | 见左 | 🟡 P2.3 两组已结（2026-10-29）：老 backend 并发 SIGABRT 随 `integration_test.go` 一起删除；`m5_color_glyph_api` 4 用例在 `92ca9216` 与 HEAD 均 PASS（环境性 FAIL，非代码债）；其余归属不变 |
| D3 | P1.2b | vet 3 条测试伪造指针 → P2.5 测试卫生；2 条 `unreachable`（`render/internal/gpu/gpu_render_context.go:4296,4344`）→ P2.5（`evidence/P1.2b/summary.md:53-54`） | P2.5 | ⬜ |
| D4 | P0.4/P1.6 | K1：`solid_gradient`/`text_image` 每轮 +2 条/+131072B（native/go 逐位一致），conformance JSON 上界暂时放宽 2/131072 | P2.4c（已写进正文：修完收紧回 0） | ⬜ |
| E1 | P1.7 | A 族超限 8 处集中 4 轮：c3_list_scroll（fps+p95+hitch）、r7_virtlist（p95+hitch）、r7b_scroll_reuse p95 22.42 临界 | P2 性能修复，修后按 `baseline/perf/perf_baseline.tsv` + p99 8ms 带复测 | ⬜ |
| E2 | P1.7 | `video_vr7_perf` 缺 `interval_p99_ms`（观测缺口） | P2（随 A2 字段统一一起补） | ⬜ |
| E3 | P1.4/P1.7 | 6 轮采不了档（NO_JSON / BUILD_FAIL）沿用 P1.4 登记 | 随 C2 及对应窗修复 | ⬜ |
| F1 | P1.8 | 一致性台 `gpu_ops` 计数下限是 1，单次跳绘抓不住 | P2 收紧为用例精确 op 数 | ⬜ |
| F2 | P1.8 | oom probe 的 `pre==post` 对「整体抬底」型泄漏不敏感（只对注入窗内单笔退款失败敏感） | P2 probe 语义补强 | ⬜ |
| G1 | P2.3 | `gpu/shader/spirv/internal/codegen/shader_test.go` 6 个「生产 shader 编译」用例读 `../../gg/internal/gpu/shaders/*.wgsl`——该路径在本仓从未存在，一直 `t.Skipf` 空转；P2.3 删 4 份 shader 后其中 4 条永久失去目标，另 2 条（flatten/coarse/fine）也不能直接改读 `tilecompute/shaders/*`（阶段不同） | P2.1（shader 线） | ✅ 随 P2.1 删 `spirv/` 整包闭合（2026-10-29） |
| G2 | P2.3 | `render/internal/gpu` 第二层老抽象：`command_encoder.go`/`commands.go`/`atlas.go`/`gpu_texture.go` 只被测试/示例可达，但 `RectAllocator`（`atlas.go`）被 `glyph_color_atlas.go` 用、`IndexFormat` 被 `render_pass.go` 用、`*RenderPass` 出现在 `text_pipeline.go` 签名 → 不是整块死代码，需逐符号拆解 | P2.4e（入口收敛） | ⬜ |
| G3 | P2.1 | `gpu/gwgpu/gles/present_damage_x11_test.go:TestPresentDamageX11` 在当前 X11/NVIDIA 环境失败（`partial present never engaged: full=12 partial=0`，另一次表现为 `BadMatch` on `X_GetImage`）；在改动前 `4e348fd3` 干净 worktree 上同断言失败，与 P2.1 无因果 | GL 增量呈现线（诊断 EGL 局部提交为何未启用） | ⬜ |
| H1 | P2.4a | P2.1/P2.3 的 `render/internal/gpu` 逐文件轮为**无 GPU 假绿**：门禁脚本未导出 `WGPU_NATIVE_PATH`，native GPU 用例全部 `t.Skipf`（CreateInstance unavailable）→ 文件级 rc=0 记 PASS（本轮带 lib 重跑恢复 P1.3 真实红面 27 个，`evidence/P2.4a/per-file-tests.txt` 对照 `a3-dissection.md`） | P2.6 重放纪律：GPU 包逐文件门禁必须带 lib 或显式记「无 GPU 轮」；P2.5 顺手把 native_device_test skip 文案补 OOM 说明 | ⬜ |

## 3 每个任务的开工条件与完成标准

### P0 准备（不改代码）

- **P0.1 阈值与环境清单**
  - 开工条件：无。
  - 内容：首帧时间、滚动帧率、单帧显存上限、p95/p99、金图容差，全部写死数字并指明来自哪份真源文档 / 哪次实测。
  - 完成标准：`baseline/thresholds.md` 落盘；每个数字有来源（真源章节或实测窗名+日期）；不留"待定"。
  - 下游义务：阈值负向对照（改错一项阈值必须判绿、还原必须判红）在 P1.7 数据与 P1.8 破坏上成对执行，记入 `baseline/negative/threshold-mutation.txt`。
- **P0.2 真窗与金图清单**
  - 内容：窗口名 → 金图文件 → 平台/后端 → 容差 → 单窗或批量、运行方式（自动自检 / 人工常驻）→ **覆盖能力标签**（着色器 / 裁剪 / 混合 / 文本 / 图片 / 滤镜 / 2.5D / 显存与生命周期 等）。
  - 完成标准：清单覆盖 `examples/` 下全部真窗；每行能直接照着跑；缺金图的窗口显式标"无金图"及原因；PLAN §5/§6 登记的每项能力至少对应一扇窗，没有窗的写清替代验证方式。后续每个任务按能力标签挑窗，不许拿不相关的窗充数。
  - 下游义务：P1.4 严格按 `baseline/windows.md` §5 的顺序与产物清单采集；8 扇运行期金图（`0*`）归档到 `baseline/windows/<窗名>/` 后回填清单并把容差改成 `0`；P1.6 按「显存/生命周期」标签挑窗；P2.6 按本清单全量重放。
- **P0.3 归档与回滚演练**
  - 内容：建演练分支或临时 tag，实际回退一次再恢复，记录命令与结果。
  - 完成标准：`baseline/rollback.md` 有完整命令与输出；证明"退得回去、恢复得回来"。
  - 边界：这里的 tag 只用于演练，不是基线 tag；正式基线 tag 在 P1.8 通过后打（见下）。
  - 安全：回退演练只动本地、只动自建分支；不碰远端、不 `reset --hard` 他人工作。
  - 执行结果（2026-10-28）：R1–R8 全 PASS（R5 按「相对起点无新增失败」口径，首轮红保留）；负向对照 R3 转红。执行中发现既有构建缺陷，移交 P1.1：`examples/ui_wr_t_stress` 引用不存在的 `ui/kit/button`、`ui/kit/icon`；`ui/scene/internal/gpupixel` 只有测试文件。
- **P0.4 双后端一致性测试台**
  - 内容：同一场景分别跑 webgpu 与 gwgpu（gles/metal），对语义做逻辑断言（几何覆盖、颜色通道、绘制次数、资源计数），不做像素相等。
  - 完成标准：至少 3 个代表场景（纯色/渐变、裁剪+混合、文本/位图）双后端可跑；不支持的后端显式标原因；用例集可一条命令回归。
  - 开工条件：P0.1–P0.3 无硬依赖（P0.3 已完成）；**不改引擎行为**，只加测试代码与 testdata。
  - 测试台落点：断言器 + 场景运行器放 `render/internal/conformance/`（测试专用，不进公开 API）；用例数据放 `render/testdata/conformance/*.json` + 输入图/文本（数据入 testdata，测试代码不硬编码标准数据）；入口测试放 `render/conformance_test.go`。
  - 后端切换：用子进程跑同一入口（参照 `render/p1_gl_present_x11_test.go`、`examples/ui_render_pelican/p1_online_test.go` 的既有做法）；若想同进程切换，必须先证明设备/缓存隔离，否则不用。**注意 `GPUI_BACKEND` 只切呈现层，不切 accelerator**：实际实现用 `GPUI_CONFORMANCE_BACKEND=native|go` 选装箱设备（native 走 `render/gpu.SetDeviceProvider`，go 走 GL + `SetAcceleratorDeviceProvider`），详见下方执行结果。
  - 场景与断言（三个场景各一份用例 JSON，期望值写在数据里）：
    1. `solid_gradient`：纯色矩形 + 线性/径向渐变 → 采样点通道 ±2/255、区域覆盖率 ±5 个百分点、无裁剪。
    2. `clip_blend`：矩形/圆角裁剪 + 半透明图层 → 裁剪内外采样点、混合结果通道 ±2/255、边界重绘计数。
    3. `text_image`：文本字形 + testdata 位图 → 字形覆盖率 >0 且包围盒在预期内、图片采样颜色 ±2/255、纹理创建与释放配平。
  - 双后端格：`native` 必跑、`go` 必跑；`metal` / Win32 本机不可验（Linux），写「未验（无 macOS/Windows 真机）」；某后端建设备失败要写错误与原因，禁止静默 skip。
  - 负向对照：改一个期望值（或注入 1px 偏移）→ 对应用例必须红，输出留证据；断言器自带单测（同输入 PASS、改一项 FAIL）。
  - 证据：`baseline/conformance/summary.md`（含一条回归命令）+ `<scene>/<backend>/report.json` + `run.txt`。
  - 环境前提：`replace ../purego` 是相对路径，测试机需存在 sibling 模块（本机 `/home/yanghy/app/projects/gogpu/purego`），否则先补路径再判失败。
  - 下游义务：P2.6 用本测试台做语义层「零变化」复核；像素与金图层仍归 P1.4 真窗，二者不互相替代。
  - 执行结果（2026-10-28）：3 场景 × 2 后端全 PASS，负向对照转红。落盘：
    - 入口：`render/conformance_test.go`（`TestConformanceDualBackend` 子进程编排 + `TestConformanceInner` 场景执行）；断言器/场景运行器：`render/internal/conformance/{case,run,report}.go`；比较器单测：`render/internal/conformance/comparator_test.go`（PASS）。
    - 用例：`render/testdata/conformance/{solid_gradient,clip_blend,text_image}.json` + `quadrants.png` + `comparator/` 夹具。
    - 证据：`baseline/conformance/summary.md`、`<scene>/<backend>/{report.json,frame.png,run.txt}`、`negative/solid_gradient_linear_left/`。
    - 实测计数：`gpu_ops` 3/3/2（solid_gradient/clip_blend/text_image，两后端一致），CPU 回退 0；跨后端像素差 solid_gradient 最大 1/255、clip_blend 最大 2/255、text_image 逐字节一致（SHA `618a13ac…`）。
    - 负向对照：`GPUI_CONFORMANCE_MUTATE=solid_gradient:linear_left` → `sample linear_left @(80,32): got [184 189 0 255] want [249 190 0 255] ±0`，按预期红。
  - 口径修正（执行中发现，务必按此理解）：
    1. **`GPUI_BACKEND` 只管呈现层**（`render/present_swapchain*.go`）；上下文级 GPU 工作永远走已注册的 accelerator。所以测试台不是「一个环境变量切后端」，而是子进程 + 显式装箱设备：native 走 `webgpu.CreateInstance/RequestAdapter/RequestDevice` + `render/gpu.SetDeviceProvider`，go 走 `hal.GetBackend(BackendGL)` + `memOpenX11`（不呈现的 X11 窗口）交 `SetAcceleratorDeviceProvider`。
    2. **「真窗：无、不开窗」不准确**：GL 后端必须有一个不呈现的 X11 上下文提供窗口（`memOpenX11`），否则出不了 surface/adapter；它不是验收窗，不渲染到屏幕，但依赖 X11 连接。headless 说法只对 native 成立。
    3. **native 缺 `WGPU_NATIVE_PATH` 会静默降级**：默认 `SDFAccelerator` 无设备提供者时，渐变/字形/图片全部回落 CPU `gg`（仅纯色 SDF 上 GPU）。测试台因此把「`WGPU_NATIVE_PATH` 未设置」显式记为 unavailable 原因，并强制装箱设备，避免把 CPU 回退误判成后端差异。
  - 已知差异 K1（已登记，不修）：`solid_gradient`/`text_image` 每跑一轮 context 记账 +2 条 / +131072 字节，**native 与 go 表现一致**（`clip_blend` 只有纯色/图层，稳态 9 条不增长）。用例按上界 2 / 131072 显式放宽并写明原因；P1.6/P2.4c 修掉释放缺口后把两处阈值收紧回 0。
  - 跨平台格：`native`（Linux Vulkan）✅ 已跑；`go`（Linux GL/X11）✅ 已跑；`metal` 未验（无 macOS 真机）；Win32/DX12 未验（无 Windows 真机）。

### P1 基线冻结（不改代码）

- **P1.1 六目标交叉编译**
  - 命令口径：`linux/amd64`、`linux/arm64`、`windows/amd64`、`darwin/amd64`、`darwin/arm64`、`js/wasm` 各跑一次 `go build ./...`；另跑 `-tags nogpu`。
  - 完成标准：每个目标的结果（成功/失败 + 失败包与错误）逐条落盘；失败项登记为基线已知问题，不隐藏。
  - 已知问题（P0.3 移交）：`examples/ui_wr_t_stress/main.go:38-39` 引用不存在的 `ui/kit/button`、`ui/kit/icon`，起点 `go build ./...` 即红（`876b7835` 与 `404d8e75` 同错）；`ui/scene/internal/gpupixel` 只有测试文件，显式列出构建时报告 `no non-test Go files`。P1.1 落盘时按此登记，修复另立任务。
  - 执行结果（2026-10-28，修复前）：六目标 × 默认/nogpu × 原始/过滤/库范围全部落盘（go1.25.5，`CGO_ENABLED=0`，仓库 `1bd1a06e`）；该版输出留在提交 `0b97eeb3` 的 `baseline/build/`，现目录已由 P1.1b 刷新为修复后基线。修复前数字：
    - linux/amd64、linux/arm64：默认主体 351 包 PASS；nogpu 主体 163 包加载失败；nogpu 库范围 140 包 PASS。
    - windows/amd64：默认主体 3 包编译失败；nogpu 主体 163 包加载失败；库范围 2 包编译失败。
    - darwin/amd64、darwin/arm64：默认主体 2 包编译失败；nogpu 主体 163 包加载失败；库范围 2 包编译失败。
    - js/wasm：默认主体 214 包加载失败；nogpu 主体 209 包加载失败；库范围 134 包加载失败。
  - 基线红项（P1 不改代码，逐条登记；修复需单列任务）：
    - **B1** `-tags nogpu` 下 163 包（161 examples + 2 tools）加载失败：各 `main.go` 无条件 `import _ "…/render/gpu"`；`render_visual_*` 的 `gpu_enabled.go` 标签化导入只做了一半。
    - **B2** `ui/platform/wayland_csd_font.go:36,203,207` 缺 `//go:build linux`，引用 Linux 专属 `csdBitmapGlyphH` → darwin/windows 编译失败。
    - **B3** `gpu/gwgpu/gles` 未约束到 `linux || windows`：`command_encoder.go:43` 等未打标签文件引用 `gl.Context`/`Command`/`RenderPipeline`/`Buffer`/`Texture`/`BindGroup`，`gl` 只有 linux/windows 上下文 → darwin 编译失败；wasm 下 `gl` 无文件。
    - **B4** `render/backend.go:23`、`present_construct.go:25`、`present_frame.go:24`、`present_target.go:16`、`present_swapchain_gl.go:18` 无条件注册/导入 gles → wasm 下 214 包断链。
    - **B5** `gpu/rwgpu` Windows 缺 `unixProc`/`call1..call5`（`adapter.go:203`、`bindgroup.go:288,364`、`buffer.go:311,342`、`command.go:360,379,509,557,583`）；这些只在 `loader_unix.go`（`linux || darwin`）定义，`loader_windows.go` 无等价实现。webgpu facade 依赖 rwgpu，故 Windows webgpu 无法编译。
    - **B6** `gpu/gwgpu/ffishim/ffishim.go:113,122,131` 使用 `purego.Dlopen/Dlsym/Dlclose`（仅 unix），Windows 需 `LoadLibrary/GetProcAddress` 或按平台拆文件。
  - 归属（2026-10-28 用户决定）：B1–B6 插队修，列为 **P1.1b**，在 P1.2 之前完成并复跑 P1.1，使基线 tag 全绿；不塞进 P2.5。任务细则见下。
- **P1.1b 跨平台构建修复（B1–B6）**
  - 开工依据：P1.1 红项 B1–B6（见上）；用户决定插队到 P1.2 之前，修完复跑六目标并把 `baseline/build/` 刷新为修复后基线。
  - 范围：
    - **B1** 消费方的无条件 `render/gpu` 导入不逐个改：根因在引擎层——「导入即注册」的包在自身特性标签下不可导入。改为 `render/gpu` 增加 `//go:build nogpu` 空实现（不注册任何东西，导入恒安全），消费方一行不改；唯一调用 GPU 专用 API 的消费方 `examples/engine/stage_chase`（`ResetAccelerator`）按 `!nogpu`/`nogpu` 拆一个 `resetGPUAccelerator` 小函数。
    - **B2** `ui/platform/wayland_csd_font.go` 及其测试补 `//go:build linux`。
    - **B3** `gpu/gwgpu/gles` 未打标签文件统一补 `(windows || linux) && !(js && wasm)`，与已打标签文件一致。
    - **B4** `render` 的 gles 注册/导入按平台门禁（`linux || windows`，排除 js/wasm）：四处重复空白导入合并到 `render/register_gles.go`（原 `present_target.go` 只剩注册一行，随改名为 register_gles.go 并打标签）。
    - **B5** `gpu/rwgpu` Windows 加载层补齐 `call1..call6` 与 `resolvedFnPtr`（走 `loader_windows.go` 的 `syscall.LazyProc`），两个共享文件的 `*unixProc` 断言改为平台无关的 `resolvedFnPtr`；不改 unix 路径。顺带修 `gpu/rwgpu/examples/mrt/main.go` 的 `rrwgpu.` 拼写（13 处，Windows 标签示例，从未编译过）。
    - **B6** `gpu/gwgpu/ffishim` 按平台拆加载实现：Windows 走 `golang.org/x/sys/windows` 的 `LoadLibrary`/`GetProcAddress`/`FreeLibrary`，unix 保持 `purego.Dlopen`。
  - 完成标准：五目标（linux×2、windows、darwin×2）默认 + nogpu 主体 + nogpu 库范围全绿（两个已登记问题包 `ui_wr_t_stress`、`gpupixel` 除外）；P0.4 一致性测试台 native/go 复跑与修复前逐字节一致；js/wasm 按用户决定登记为不支持（见下）。
  - js/wasm 口径（2026-10-28 用户决定：登记为不支持）：纯 Go FFI 库 `../purego` 只支持 `darwin || freebsd || linux || netbsd || windows`，无 wasm 实现（`struct_other.go:21: undefined: syscallArgs`）；`gpu/render/ui/engine/video` 栈 302 个包传递依赖它，只有 44 个叶子包能在 wasm 下单独编译。wasm 不再是基线必绿目标；将来要做须为浏览器 WebGPU 新写 `syscall/js` 后端（等同新增后端能力，单独立项）。原始输出 `evidence/P1.1b/js-wasm.txt`。
  - 执行结果（2026-10-28，修复后）：linux/amd64、linux/arm64 默认 351 / nogpu 347 / 库范围 141 包全 PASS；windows/amd64 357 / 353 / 140 全 PASS；darwin/amd64、darwin/arm64 349 / 345 / 140 全 PASS；默认与 nogpu 加载失败计数（去已知问题包）均为 0。一致性复跑 3 场景 × native/go 全 PASS，6 份 `report.json` 与 `summary.md` 与 P0.4 基线逐字节一致。
  - 证据：`evidence/P1.1b/`（`summary.md` + 6 份目标输出 + `js-wasm.txt` + `conformance-rerun.txt`），并已把 `baseline/build/` 与 `summary.md` 刷新为修复后基线。
- **P1.2 `go vet ./...`**
  - 命令口径：默认与 `-tags nogpu` 各跑一次；`./...` 原始输出落盘，再按「去掉已知加载失败包 + 保留有 Go 文件或测试文件的包」过滤后逐包采集。
  - 完成标准：两种标签组合都能完成分析；输出与逐包状态落盘；既有告警逐条登记为基线已知问题。
  - 执行结果（2026-10-28，P1.2b 修复后）：默认 352 包完成分析（351 个有 Go 文件的包 + 仅有 7 个测试文件的 `ui/scene/internal/gpupixel`），3 包 61 条诊断；nogpu 348 包完成分析，2 包 58 条诊断；两模式类型错误 0。
    - 默认诊断：59 条 `possible misuse of unsafe.Pointer`（`ui/platform` 56、`render/internal/gpu` 1、`render/internal/gpu/res` 2）+ 2 条 `unreachable code`（`render/internal/gpu/gpu_render_context.go:4296,4344`）。
    - nogpu 诊断：58 条 unsafe.Pointer（`ui/platform` 56 + `render/internal/gpu/res` 2），无 unreachable（该文件为 `!nogpu`）。
    - 原始 `go vet ./...` 与 `go vet -tags nogpu ./...` 均只剩 `examples/ui_wr_t_stress` 的已知加载错误（`ui/kit/button`、`ui/kit/icon` 不存在），该包无法分析，按 P0.3/P1.1 已登记问题保留。
    - 告警归属：`ui/platform` 56 条为 FFI 指针写法（本收敛线外，只登记）；测试伪造指针 3 条归 P2.5 测试卫生；2 条死代码归 P2.5 零行为变化清理。逐条清单见 `baseline/vet.txt` §4/§5。
  - 采集时发现的红项：nogpu 下 `render/internal/gpu` 与 `render` 共 20 个测试文件缺 `//go:build !nogpu`，引用 `GPURenderContext`/`requireNativeGPU`/`sampleRGBA` 等 `!nogpu` 符号，导致 vet/测试无法编译。用户决定插队修复 → P1.2b，修完两模式均完成分析。
  - 证据：`baseline/vet.txt`（环境、命令、原始输出、逐包状态、61 条告警登记、已知问题）；`evidence/P1.2b/`（修复前后对照与 348 包测试编译验证）。
- **P1.2b nogpu 测试标签修复（P1.2 采集发现，用户决定插队）**
  - 开工依据：P1.2 实测 `go vet -tags nogpu` 在 `render/internal/gpu`、`render` 两包无法编译测试代码，共 20 个测试文件缺 `//go:build !nogpu`，引用只在 `!nogpu` 下存在的 GPU 类型/辅助函数。
  - 范围：只给 20 个测试文件补 `//go:build !nogpu`（9 个在 `render/internal/gpu`，11 个在 `render`）；不改生产代码、不改测试内容、不改默认标签行为。
  - 完成标准：`go vet -tags nogpu <348 包>` 完成分析且 0 类型错误；`go test -tags nogpu -run '^$' <348 包>` 全部测试包编译通过；默认标签 vet 结果除预期行号位移外不变。
  - 执行结果（2026-10-28）：nogpu vet 348 包完成，58 条 unsafe 告警、0 类型错误；nogpu 测试编译 137 ok / 211 no test files / 0 FAIL；默认 vet 61 条诊断不变（仅 `view_raw_attack_test.go` 行号 11→13）。
  - 证据：`evidence/P1.2b/summary.md` + 修复前后原始输出 + 两模式包清单。
- **P1.3 逐测试文件**：每个 `*_test.go` 单独执行并记录 PASS/SKIP/FAIL；SKIP 必须带原因（缺数据、缺 GPU、平台不支持等）；禁止一次全量后只记总数。
  - 已知问题 T1（P1.1b 复跑时发现，修复前后一致，非本线引入）：`gpu/rwgpu` 的 `TestQueue_OnSubmittedWorkDone` 在本机先 `queue_workdone_test.go:45` 失败——`wgpu: Queue.OnSubmittedWorkDone: wgpu returned null future`；随后 cleanup 的 `device.Release()` 在 native `wgpuDeviceRelease` 内 SIGSEGV（`PC=0x1`，栈 `unixProc.Call → releaseNativeHandle → Device.Release`）。在 HEAD `0b97eeb3` 的隔离 worktree 复跑同一用例，栈帧与偏移完全一致（`loader_unix.go:96`、`safety.go:285`、`device_misc.go:313/314`），故与 B5 的 `resolvedFnPtr` 改动无关。P1.3 按 FAIL 记录并保留原始栈；native 释放路径的健壮性归 P2.4c/P2.4e 生命周期收敛时处理，P1 不改。
- **P1.4 全部真窗**：按 P0.2 清单逐个跑；每个窗口三证据齐；金图 diff 给像素计数并对照容差；批量压测遵守 `AGENTS.md` 的 GPU 真窗安全条。
  - 已知问题 T2（P1.4 采集发现，2026-10-29 重审登记）：`ui_wr_r1_layout`/`ui_wr_r20_filter`/`ui_wr_r22_selection_stub`/`ui_wr_c6_savelayer_group` 四窗**播种轮与确认轮均 rc=1**，而窗口 JSON 自报正常、金图 diff=0.0000%——退出码与自报结果矛盾的原因未解剖（疑似窗口内部某个非金图断言在报错）。P1.4 按 FAIL 如实登记（不粉饰、不标绿），病因诊断归 P2 对应条目。
  - 口径：播种轮（seed）原始 FAIL 且 `golden_first_run=true` 的在汇总显示 `SEED`（首跑只产基线不做门禁）；确认轮不做此映射，FAIL 就是 FAIL（已写入 `baseline/windows/summary.md` 口径段）。
- **P1.5 API 与测试计数快照**：每包 `go doc -all` 落盘；测试函数计数统计落盘。这两份是 P2 对账的尺子。
- **P1.6 显存账本**：开窗、关窗、切场景、压测后各取一次；关窗后应回落到基线值，差异记录。
  - 已知差异 K1（P0.4 移交）：`solid_gradient`/`text_image` 每跑一轮 context 记账 +2 条 / +131072 字节，**native 与 go 一致**（`clip_blend` 纯色/图层不增长）。本项按「每轮增量」口径实测复核并落盘；释放缺口修在 P2.4c。
- **P1.7 性能基线**：首帧、滚动帧率、p95/p99 按 P0.1 口径实测并落盘。
- **P1.8 负向对照**：三种人为破坏（偏移 1px、跳过一次绘制、少释放一次）各跑一次，确认对应门禁会红；记录门禁输出。做完立即还原。通过后把 P1 证据一并提交，并打正式基线 tag（P2 从该 tag 起分支）。

### P2 框架代码调整与整合（行为零变化）

- **P2.1 shader 合并**
  - 开工条件：P1.8 ✅；迁移前已对全部 WGSL 用例的 GLSL/MSL 输出做全量快照。
  - 动作：naga 实现迁到顶层 `gpu/shader`（保持 `ir/wgsl/glsl/msl` 结构，统一包名）；删旧副本与 `spirv/` `hlsl/` `dxil/` `internal/backend` `internal/dxcvalidator`；调用方只换 import。
  - 完成标准：迁移后 GLSL/MSL 输出与快照逐字节一致；GL/Linux 真窗三证据重放一致；Metal 标"本机不可运行验证"（交叉编译必须过），不得标绿；API 快照差异仅限白名单。
- **P2.2 删 `render/render`，`FloatTarget` 迁出**
  - 动作：删 15 个旧文件及测试/testdata；`float_target.go` 迁 `render/hdr`，单测与 testdata 随迁。
  - 完成标准：全仓无残留引用；迁移后的 HDR 单测绿；API 快照差异仅限白名单。
- **P2.3 删老一代 GPU 渲染器**
  - 动作：删 `render/internal/gpu` 下 backend/renderer/pipeline/pipeline_cache_core/memory/shaders/sparse_strips_gpu/gpu_coarse/gpu_fine/gpu_flatten 及其测试；`doc.go` 重写。保留在用文件（segment/flatten/fine/coarse/sparse_strips/tilecompute_filler）。
  - 完成标准：`webgpu.CreateInstance` 只剩白名单入口；相关测试按白名单减少且有账；真窗重放一致。
- **P2.4a 设备创建/选卡收敛**：三份 → 一份，创建入口唯一。
- **P2.4b 预算统一**：六口径 → 一套 Budget，按域分账；淘汰语义有单测。
- **P2.4c 延迟释放统一**：三队列 → `res/registry`；释放时序有单测。
  - 已知差异 K1（P0.4 移交）：修掉 brush/glyph/image 纹理每轮 +2 条 / +131072 字节的释放缺口；修完把 `render/testdata/conformance/{solid_gradient,text_image}.json` 的 `max_steady_count_delta` / `max_steady_bytes_delta` 收紧回 0，并用 P0.4 回归命令重跑（`baseline/conformance/summary.md` 已知差异表应清空）。
- **P2.4d 纹理分配收敛**：六处同尺寸管理器 → `res/cache`；复用命中率有探针。
- **P2.4e 入口收敛**：裁剪栈、混合枚举、呈现入口各归一处；旧入口删除且调用方全改。
  - 每条子步附加门禁：生命周期测试台（开窗/关窗/切场景 → 显存账本归零、堆无增长）+ 触及共享数据跑 `-race` + 真窗三证据。
  - 每条子步独立分支与提交点；出问题整步回退，不带着半截往前走。
- **P2.5 包组织收口**
  - 动作：按 PLAN §3 目标结构 `go list` 逐包对比归位；加分层约束测试（`ui` 不直引 `gpu`、`render` 无后端特有类型、调用方向单向）；同步索引与总账（`RENDER_API_CATALOG.md` 等）。
  - 完成标准：分层约束测试入 CI/固定 runner；`go list` 与目标结构差异为零或已登记理由。
- **P2.6 行为零变化总验收**
  - 动作：按 P1.1–P1.8 全量重放，逐项与基线对照。
  - 完成标准：单测与真窗结果与基线一致；金图逐位差为 0（非确定性路径按 P0.1 容差）；API/测试计数差异逐条对上 PLAN §7.7 白名单；显存与性能基线不回退。**未绿不得进入 P3。**

### P3 缺失能力（先文档后实现）

- **P3.0**：先出依赖图与文档更新（PLAN §5 + `RENDER_API_CATALOG.md`），再按图排实现顺序；文档没写完不写代码。
- **P3.1–P3.8**：每项先补文档（能力定义、公开 API、双后端实现方式、验证方案），再实现。
- 每项完成标准：逻辑探针 + 像素断言 + 金图三证据；webgpu 与 gwgpu 分别验；不支持的端显式标注原因；公开 API 同步总账。
- 前置前移项已固定在 P3 序列里：P3.1 线性混合、P3.3 逐 run 样式——不得挪回 P4。

### P4 半成品完整实现（P3 完成后）

- 只处理非前置项：P4.1 正确性、P4.2 GPU/CPU 模糊一致、P4.3 性能、P4.4 COLRv1。
- 若 P4.1 的正确性缺陷在 P3 真窗里阻塞，允许前移到对应 P3 项，并在本文件与 PLAN §7.5 同时登记调整。
- 每项完成标准同 P3（三证据 + 双后端 + 总账同步）。

## 4 统一门禁口径

| 阶段 | 门禁 |
|---|---|
| P0 | 四份产出落盘；回滚演练真的退回过一次 |
| P1 | 五真机目标（linux×2/windows/darwin×2）编译 + `go vet` + 逐测试文件 + 全部真窗 + API/计数快照 + 显存账本 + 性能 + 负向对照，八项全有证据；js/wasm 按 P1.1b 决定登记不支持 |
| P2 每子步 | 全平台编译（linux×2/windows/darwin×2；wasm 不支持，见 P1.1b）+ `go vet` + 逐测试文件 + 残留 grep + API 快照 diff（仅白名单差异） |
| P2.4 每子步 | 上述 + 生命周期测试台 + `-race` + 真窗三证据 |
| P2.6 | 基线全量重放一致；差异逐条对白名单 |
| P3 / P4 每项 | 三证据 + 双后端分别验 + 不支持端显式标注 + 公开 API 总账同步 |

> P1.2 的 vet 基线不是「零告警」：61 条既有告警已逐条登记到 `baseline/vet.txt` §4/§5 并写明归属；门禁判的是「默认与 nogpu 两模式都能完成分析 + 告警有账」，不是把既有告警当新红项。

### 4.1 能力验证要求（每个任务必带，硬）

**每个任务都必须有两样东西：对应的能力单测 + 对应的真窗。** 没有就不算完成，也不许开工。

开工前，在 §2.1 登记四类载体，缺一项不得开工：

1. **能力单测**：写明文件路径 + 测试函数名；标清是新增还是沿用。重构/删除任务必须写明「用哪些测试证明等价」；新能力任务必须先写**会红的能力测试**（先红后绿）。
2. **真窗**：写明 `examples/<窗口名>` + 运行方式（`-auto-only` 自动判 / 常驻人工窗）。新能力必须新增或扩展真窗；只跑通用回归不算。
3. **金图与容差**：写明金图路径 + 容差数值（数值口径来自 P0.1）。没有金图的窗口必须写明原因和替代判定（像素断言 / 逻辑探针）。
4. **双后端**：webgpu 与 gwgpu（gles / metal）分别写结果；本机不能跑的端写原因，不许空着。

完成时必须交齐五份证据：

- 能力单测逐文件 PASS 日志（不是只报总数）；
- 真窗 JSON 探针输出；
- 真窗 PNG；
- 金图 diff 计数 + 对照容差的结论；
- 双后端结果 + 一个针对本能力的负向对照（人为破坏后门禁必须红，红了再还原）。

**「正确」的判定标准**：能力单测绿 + 真窗三证据齐 + diff 在容差内 + 双后端结果一致（或差异有书面依据）+ 负向对照红过。缺任意一项，状态只能 🟡，不得 ✅。

**按任务类型的落地方式**：

| 任务类型 | 能力单测怎么写 | 真窗怎么跑 |
|---|---|---|
| 治理/基线（P0.1–P0.3、P1） | 无能力变更：P0 用产出物自校验；P1 本身是采集，逐窗口/逐文件落盘 | P1 按 P0.2 清单全量采集；P0 不跑新窗 |
| 一致性测试台（P0.4） | 测试台自带单测 + 至少 3 个代表场景 | 3 个代表场景在双后端跑逻辑断言 |
| 删除/合并/重构（P2.1–P2.4） | **动手前先补能力单测并跑绿**；没有测试的能力禁止重构；删除对象的能力用同一组断言证明等价 | 按基线逐窗重放，diff 必须为 0（或 P0.1 声明的容差）；P2.4 每子步额外跑生命周期窗 |
| 包组织（P2.5） | 分层约束测试 + `go list` 对账 | 基线全窗重放 |
| 总验收（P2.6） | 全量测试重放 | 全量真窗重放 |
| 新能力（P3/P4） | **先写红测**，再实现；覆盖正常路径 + 边界 + 双后端分支 | 新增或扩展真窗，出新金图；无真窗的能力不得标 ✅ |

**例外只有一种**：零生产消费者的死代码删除（例如 `FloatTarget` 这类），确实没有真窗可跑。此时必须同时给出三样替代证据：全仓无引用 grep、迁移前后单测输出一致、P2.6 基线重放无差异；并在 §7 流水写清理由编号。

### 4.2 验证载体登记模板（开工前复制到 §2.1）

```
**当前任务验证载体（开工前必须填）**

- 任务：Px.x <名称>
- 能力单测：<文件路径> :: <TestName>（新增 / 沿用；重构：证明等价）
- 真窗：examples/<窗口名>（自动 / 人工）
- 金图：<路径>，容差：<数值 + 口径来源>
- 双后端：webgpu <结果>；gwgpu-gles <结果>；gwgpu-metal <结果或不可验原因>
- 负向对照：<人为破坏方式> → 预期哪道门禁红
- 证据：evidence/Px.x/
```

残留 grep 骨架（按 PLAN §2/§7.7 的对象逐个跑，命令随对象变化，结果落盘）：

- 后端特有类型：`grep -rn "webgpu\.\|gwgpu/\|gles\." render/*.go | grep -v _test.go`
- 分层：`grep -rn "gpu/" ui/ | grep -v _test.go`
- 删除对象：对白名单每个路径跑 `grep -rn "<pkg 名>" --include=*.go .`，确认无残留引用

## 5 证据目录约定

```
docs/render_gpu_converge/
  baseline/              # P1 冻结产物，只增不改；后续对账的参照
    thresholds.md
    windows.md
    rollback.md
    conformance/
    build/ vet.txt tests/ windows/ api/ vram/ perf/ negative/
  evidence/
    P2.1/ P2.2/ ... P4.4/   # 每个任务的证据
```

每个证据目录至少包含：执行命令、原始输出或摘要、结论（PASS/FAIL）、对照对象的路径。真窗证据含 JSON 探针输出、PNG、金图 diff 计数。

## 6 即时同步规则（硬）

1. **状态改动当次落盘**：任何任务开工、完成、阻塞、暂停，都在当次会话内改本文件 §2 状态列、§2.1 当前焦点，并追加 §7 流水；禁止"攒到最后一起写"。
2. **代码提交必须带台账**：凡是改代码的提交，同一提交内必须包含本文件的状态/证据路径更新；没更新台账的提交视为未完成。
3. **任务列表镜像**：新会话按 §2 逐项重建工具任务列表；工具条目数不足时可按阶段聚合，聚合条目名必须写明覆盖的编号（如 `P0.1–P0.4`），状态仍以 §2 逐项为准。发现不一致先按本文件修正镜像并记录差异。
4. **方案口径变更回写 PLAN**：任务顺序、门禁、白名单变化，先改 `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md` 对应章节，再改本文件；两份不允许互相矛盾。
5. **证据先于状态**：没有证据文件不得把状态改成 ✅；证据路径必须能在仓库里打开。
6. **删除必须对账**：任何删除动作在 §7 流水写清「对象 / 理由（PLAN 章节）/ API 与测试计数变化 / 白名单对应行」。
7. **跨平台格写满**：涉及平台/后端的任务，完成时在同一行写清哪端做了、哪端没做及原因，不留空格。
8. **失败不掩盖**：门禁红了就记 FAIL 与原因，状态退回 🟡 或 ⛔，禁止把失败写成"基本通过"。
9. **验证载体未登记不得开工**：§2.1 的能力单测、真窗、金图容差、双后端四项没填，任务状态不许从 ⬜ 变 🟡。
10. **单测与真窗缺一不得标 ✅**：只有单测绿 = 不算；只有真窗看着对 = 不算；两样齐 + 三证据 + 负向对照红过才算正确。

## 7 状态流水（按时间追加，最新在下）

| 日期 | 任务 | 变更 | 证据 / 说明 |
|---|---|---|---|
| 2026-10-28 | 全表 | 建台账；P0/P1/P2.1–P2.6/P3.0–P3.8/P4.1–P4.4 共 32 项登记，全部 ⬜ | 方案见 PLAN §7；本文件为状态真源 |
| 2026-10-28 | P0.1 | ⬜→✅ 阈值与环境清单落盘 | `docs/render_gpu_converge/baseline/thresholds.md`：环境实测（940MX/580.178.04/go1.25.5/Wayland:XWayland）；首帧 <2000/<1000/<800ms；fps_interval≥55 + p95≤22ms；p99 只观测 + 8ms 治理回归带；显存 768MB 预算 / 0.9 水位 / 0.8 低显存自动档；金图默认零容差；阈值负向对照留给 P1.7/P1.8 成对验证 |
| 2026-10-28 | P0.2 | ⬜→✅ 真窗与金图清单落盘 | `docs/render_gpu_converge/baseline/windows.md`：162 真窗 + 28 headless，逐窗金图/容差/运行档/能力标签；仓内金图 114、运行期 8、无金图 40（均写原因与替代判定）；PLAN §5/§6 能力逐项映射，无窗项给替代 + 计划窗名；§6 自校验脚本实测 rows=162、114/8/40、missing=0 |
| 2026-10-28 | P0.3 | 演练模板落盘，**未执行**（状态仍 ⬜） | `docs/render_gpu_converge/baseline/rollback.md`：R1–R8 判据、8 条边界、起点 `404d8e75` / 回退点 `876b7835` / `/tmp/gpui-p0.3-drill` 隔离 worktree / 演练 tag 用完即删；R8 与边界 5 明确 4 个既有 prunable worktree 原样保留；§6 结果列待执行回填 |
| 2026-10-28 | P0.3 | ⬜→✅ 归档与回滚演练执行完毕 | `baseline/rollback.md` §6：R1–R8 全 PASS；回退点 `876b7835` 树 `893b1c9d…`≠起点 `eee499f5…`、`baseline/` 不存在且台账「下一任务：P0.1」；恢复后树哈希与三文件 sha1 逐位一致、status 7 行不变；演练 tag/隔离 worktree/`/tmp/purego` 软链全部清干净。R5 首轮两次 FAIL 保留：`/tmp` 下 `../purego` 不存在（环境）、`ui_wr_t_stress` 引用不存在的包（起点同错），判据改为「相对起点无新增失败」，350 包构建通过。既有缺陷移交 P1.1 |
| 2026-10-28 | P0.4 | ⬜→✅ 双后端一致性测试台建成并跑绿 | `render/conformance_test.go` + `render/internal/conformance/{case,run,report}.go` + `render/testdata/conformance/`；3 场景 × native/go 全 PASS，`gpu_ops` 3/3/2、回退 0，跨后端像素差 ≤2/255（text_image 逐字节一致），比较器单测 PASS；负向对照转红；证据 `baseline/conformance/`（summary + 6 组 report/frame/run（3 场景 × 2 后端）+ negative 一套；2026-10-29 重审勘误：原记「12 组」有误）。口径修正三条：`GPUI_BACKEND` 只切呈现层（测试台改用 `GPUI_CONFORMANCE_BACKEND` + 装箱设备）、GL 需不呈现的 X11 上下文窗、native 缺 `WGPU_NATIVE_PATH` 会静默回落 CPU。新登记 K1（每轮 +2 条/+131072 字节，双后端一致）→ P1.6/P2.4c；metal/Win32 未验（无真机） |
| 2026-10-28 | P1.1 | ⬜→✅ 六目标交叉编译基线落盘（4 个目标存在红项） | `baseline/build/`：summary + 6 份原始输出。linux/amd64、linux/arm64 默认主体 351 包 PASS、nogpu 库范围 PASS；windows/amd64、darwin/amd64、darwin/arm64、js/wasm 红项 B1–B6 登记：gles 未限 `linux || windows`、render 无条件注册 gles（wasm 214 包断链）、rwgpu Windows 缺 unix 调用层、ffishim 用 purego.Dlopen、wayland_csd_font 缺 linux 标签、163 个消费方无条件导入 render/gpu。用户决定插队修 → P1.1b，P1.2 前完成并复跑 |
| 2026-10-28 | P1.1b | ⬜→🟡 用户决定插队修复 B1–B6 | 范围与验收见 §3 P1.1b；证据 `evidence/P1.1b/`；修完复跑六目标并刷新 `baseline/build/`，P0.4 一致性测试台作 Linux 零变化守卫 |
| 2026-10-28 | P1.1b | 🟡→✅ B1–B6 修完，五目标全绿；js/wasm 登记不支持 | 改动：`render/gpu/gpu_nogpu.go`（B1 引擎层空包，163 消费方不动）、`wayland_csd_font{,_test}.go` 补 linux（B2）、gles 五个未打标签文件补平台标签（B3）、四处 gles 注册合并为 `render/register_gles.go`（B4）、rwgpu `loader_windows.go` 补 `call1..call6`/`resolvedFnPtr` 且共享代码去 `*unixProc` 断言 + mrt 示例 `rrwgpu` 拼写（B5）、ffishim 拆 `libload_unix/windows.go`（B6）。实测：linux×2 351/347/141、windows 357/353/140、darwin×2 349/345/140 全 PASS，默认/nogpu 加载失败 0。P0.4 复跑 6 份 report 与 summary 逐字节一致。js/wasm：`../purego` 无 wasm 实现（345 包中 302 包传递依赖），用户决定登记不支持，将来需 `syscall/js` 后端单独立项。`baseline/build/` 已刷新，修复前红基线留在 `0b97eeb3` |
| 2026-10-28 | P1.2 | ⬜→🟡 开始采集；发现 nogpu 2 包测试缺标签 | 默认 352 包 61 条诊断；nogpu 2 包类型检查失败（`render/internal/gpu`、`render`，20 个测试文件缺 `//go:build !nogpu`，引用 `GPURenderContext`/`requireNativeGPU`/`sampleRGBA` 等 `!nogpu` 符号），用户决定插队修 → P1.2b |
| 2026-10-28 | P1.2b | ⬜→✅ 20 个测试文件补 `//go:build !nogpu`（插队当日完成） | nogpu vet 348 包完成、58 条 unsafe、0 类型错误；nogpu 测试编译 137 ok / 211 no test files / 0 FAIL；默认 vet 61 条诊断不变（仅 `view_raw_attack_test.go` 行号 11→13）。证据 `evidence/P1.2b/` |
| 2026-10-28 | P1.2 | 🟡→✅ 两模式完成分析，告警逐条登记 | `baseline/vet.txt`：默认 352 包 / 3 包 61 条（59 unsafe + 2 unreachable）；nogpu 348 包 / 2 包 58 条 unsafe；原始 `./...` 仅剩 `ui_wr_t_stress` 已知加载错误；告警归属：ui/platform 线外 56、P2.5 测试卫生 3、P2.5 死代码 2 |
| 2026-10-28 | P1.3 | ⬜→✅ 1398 文件逐文件跑完，T1 原始栈保留 | `baseline/tests/`：1264 PASS / 48 FAIL / 86 SKIP；1398 行 0 缺失、0 重复、0 多余；失败 48 条逐条分类并给 P2 归属；46 个 `RequestDevice` SKIP 首轮与干净重跑均复现（不是瞬跳）；长跑 `zz_b2_bitmap_scan` 1256.2s、`zz_b1_ttf_scan` 704.2s、`zz_b1_vf` 484.7s PASS；`ui/scene`/`video` 整包交叉复核日志在 `logs/crosscheck/` |
| 2026-10-29 | P1.4 | ⬜→✅ 162 窗 / 183 变体两后端采集完成 | `baseline/windows/`：native 191 轮（PASS 111 / FAIL 60 / NO_JSON 8 / SEED 8 / BUILD_FAIL 1 / PROBE 1 / SKIP 2）+ go 183 轮（PASS 108 / FAIL 51 / NO_JSON 20 / BUILD_FAIL 1 / PROBE 1 / SKIP 2），`summary.tsv` 374 行无 MISSING。8 张运行期金图先 `GPUI_ACCEPT_GOLDEN=1` 播种、次遍 diff=0，基线归档 `default/native/snapshots/`、容差回填 `0`；逐窗 JSON/stdout/stderr/GPU readback 快照/goldens + 全局与逐窗 README。FAIL/SKIP/BUILD/NO_JSON 逐条原因登记（不修，归 P2）：14 窗自带 x11grab screen check 在 rootless Xwayland 抓黑图为环境 FAIL；`ui_wr_t_stress` 引用不存在的 `ui/kit/button`、`ui/kit/icon` BUILD_FAIL；`ui_ime_probe` 人工探针 WM_DELETE 不响应按兜底 SIGTERM 记 PROBE；`ui_text_edit_accept` 需 `GPUI_ACCEPT_RUN_SECONDS=30`，补跑后两后端 PASS（script 金图自动模式不逐位比较，登记 P2 决策）；`ui_pf_win32`/`ui_pf_appkit` 平台例外 SKIP；`ui_l1_blank`/`ui_pf_x11_device`/`ui_pf_x11_dnd` 等缺 A 族字段移交 P1.7；`ui_wr_r6_layer_anim` 表内金图与代码真源不一致（代码读 `R6_SNAP_DIR`，仓内 `golden/baseline_igpu.png` 仅归档参考）登记 P2 对齐；四窗两轮 rc=1 与 JSON 自报 pass 矛盾见 T2 |
| 2026-10-29 | P1.5 | ⬜→✅ 每包 API 快照 + 测试计数快照落盘 | `baseline/api/`：包集合沿用 P1.2 权威清单 **352 包**（与 `go list ./...` 实测差集仅 t_stress，P1.1 已登记）；`go doc -all` 逐包落盘 **351 文件**——`ui/scene/internal/gpupixel` 仅测试文件无源码包按 P1.2 同口径登记例外，16 个 `package main` 示例包空输出入 `doc_empty_main.txt` 白名单；静态测试计数 **1397 文件 / 14388 函数**（Test 13602 / Benchmark 753 / Example 18 / Fuzz 15）；磁盘 1468 = 1397 + 1 stress 标签 + 70 third_party（独立模块 replace 引入）逐项对账零缺口；与 P1.3 `funcs` 列口径差（运行动态发现 vs 静态声明）逐文件核对为 17 个平台排除文件；机器校验 `tools/verify_api.py` **ALL PASS** 连跑幂等，`tools/gen_api.py` 随仓可复现，P2 改 API 后 diff 即变化清单 |
| 2026-10-29 | P1.6 | ⬜→✅ 显存账本快照落盘 | `baseline/vram/`：三 cap 窗全协议 × 双后端 8 轮（隔离 :3 串行、轮前健康检查、标准关闭）。vram ladder native（RUN_SECONDS=300，P1.4 的 60s 是饿死门禁，`gateSeconds=300` 写死窗内）PASS：账本峰值 **372.0MB** 未破 0.9 水线（691.2），拒增曲线 `[2,0]→[4,550]→[8,747]`，hold 斜率 1533KB/min，峰值在资源矩阵带内；ladder **go 档跑不了**：GL 4 窗起 `eglMakeCurrent 0x3002`（stderr 证据在案）登记 P2；oom 双后端 PASS：注入峰值 12.06MB → 恢复 **8.5459→8.5459MB（delta 0.00）** `ledger_recovered=true` 资源谁建谁放成立；multiwin 双 PASS（crash_free、8 窗未触顶）；batch 双 FAIL（p95 55.7/62.1 vs 22ms，与 P1.4 同因同量级，归 P2 性能线）。K1 复核（P0.4 测试台 + :3 复跑）：clip_blend 三采样 9 条不增长，solid_gradient/text_image **每轮 +2 条/+131072B** 精确复现，native/go 逐位一致，ctx_after 全 0。全窗 RSS 回落盘点（P1.4 存量 223 条）：RSS 关窗不回落是 Go 运行时常态非泄漏判据；稳态斜率过 30000 泄漏线 14 条，其中 **10 条 PASS 记录**（r17_bg_throttle 88801 最高等）登记 P2 判门禁覆盖面。机器校验 `tools/verify_vram.py` 14 断言 **ALL PASS** 幂等 |
| 2026-10-29 | P1.7 | ⬜→✅ 性能基线落盘（P1.4 存量纯分析，零 GPU 零补采） | `baseline/perf/`：`perf_baseline.tsv` 366 轮逐轮 A/H 族字段 + 门禁判定 + P1.4 交叉。首帧 275 轮 **0 FAIL**（median 174ms / p90 248 / max 515，文本/IME 紧预算 18 轮 max 267ms 全达标，黑屏 0）。A 族 44 轮（windows.md 帧时/调度 17 窗 ∪ §3 点名 6 窗 = 22 窗）：九字段全齐 24 轮；默认门禁超限 **8 处集中 4 轮**（c3_list_scroll native 三红 54.66fps/26.43ms/52.0每分、r7_virtlist 双后端 p95+hitch、r7b native p95 22.42 临界）**全部与 P1.4 FAIL 状态一致，零新增零口径冲突**；p99 观测基线 median 20.7 / max 32.0ms 落盘（P2 起按 8ms 回归带对照）。P1.4 移交的「缺 A 族字段」定性为**字段两代命名并存**（旧代 `frame_interval_*`/`avg_frame_interval_ms` vs 现口径 `interval_*`），抽取器别名映射后数字齐全，零补采；契约统一（含 fps_wall/target_hz 补齐）登记 P2。引擎窗 71 轮 p95 median 19.11ms 全高于 2.5D 目标 16ms，与 S52 豁免口径一致不作判定。机器校验 `tools/verify_perf.py` 8 断言 **ALL PASS** 幂等 |
| 2026-10-29 | P1.8 | ⬜→✅ 负向对照三破坏全红 + §8.2 阈值变异对成立 | `baseline/negative/`：①偏移 1px（`GPUI_NEG_SHIFT_1PX=1` 临时补丁）→ 像素断言双后端红（coverage 0.92/0.87 ≠ 1.00）；②跳过一次绘制（`GPUI_NEG_SKIP_DRAW=1`）→ 红，诚实发现 `gpu_ops` 计数下限 1 抓不住单跳（计数收紧登记 P2）；③少释放一次 → K1 释放缺口真实现场（solid_gradient 每轮 +2 条/+131072B），把用例 JSON 放宽上界临时收紧 131072→0 → resource 断言双后端红，还原即绿；第一方案（全局跳 VramForget）不红的 probe 语义覆盖面发现（pre==post 对整体抬底不敏感）登记 P2。三破坏全部当场还原、代码零残留、复绿留档。§8.2：extractor 的 p95 门禁改为读 thresholds.md（阈值文件承重），22→4 红 / 30→0 漏报 / 还原→4 报警成对成立，thresholds.md 零残留。机器校验 `tools/verify_negative.py` 9 断言 **ALL PASS** 幂等。P1 基线冻结全部收口，待打正式基线 tag |
| 2026-10-29 | P1 收口 | 正式基线 tag `baseline-p1` 打在 `92ca9216`（附注；P2 起分支点/回滚锚点/P2.6 对账基准） | `baseline/rollback.md` §7 登记用途与回退方法；tag 本体信息见 `git show baseline-p1` |
| 2026-10-29 | P2 待办池 | 建池：§2.2 收 **23 项**——审计 4 项新发现（A1–A4）+ P1 期间散落的真窗/测试/性能/门禁覆盖面登记；A1（AGENTS.md:78 过时 Vram 描述）当场修正 | 台账 §2.2；AGENTS.md:78；审计其余发现归位见各 baseline summary（tests §4、vram §3、perf §4、negative 结论） |
| 2026-10-29 | P2.2 | 🟡→✅ 删 `render/render` 15 文件（含测试/testdata，PLAN §7.7 白名单），`FloatTarget` 迁 `render/hdr` | `evidence/P2.2/`：迁移前 `render/render` 86 PASS / 迁移后 `render/hdr` 3 PASS（`TestFloatTargetSoftwareDeclines` 的被测对象随包删除）；五目标默认/nogpu 主体与库范围 351/347/141、357/353/140、349/345/140 全绿，与 P1.1b **逐项一致**，加载失败 0；全仓 vet 默认 61 / nogpu 58 与 P1.2 逐条一致；残留 grep rc=1 零引用；API 快照 352 包仅 `render/render.txt` 消失 + `render/hdr.txt` 新增（其余 350 份逐字节不变），测试 1397→1390 文件 / 14388→14302 函数（差=被删 8 文件/89 函数 − 新包 1 文件/3 函数）；负向对照：临时 `import "…/render/render"` 编译红 → 删除探针复绿零残留。无真窗：零生产消费者死代码删除，按 §4.1 例外，P2.6 基线重放复核。文档同步：`RENDER_API_CATALOG.md` §0/§7.2/§9 注、`ENGINE_UI_WIDGET_RENDER.md` §10、apidoc rc=0 |
| 2026-10-29 | P2.3 | 🟡→✅ 删老一代 GPU 场景渲染器闭合簇（10 源 6515 行 + 10 测试 5635 行 + 7 WGSL 2071 行；3 处存活面迁移；`doc.go` 重写） | `evidence/P2.3/`：删前先做能力替代核对（§0.1：8 项有新载体、2 项特殊——`PipelineCacheCore` 真实创建被注释从未工作，`MemoryManager` 功能完整但唯一实例化点在旧渲染器内、能力已分域承接），确认「功能等价」后执行；`Backend` 精简落 `device.go`（字段名不变，7 处既有测试零改动）、4 个 `Stub*ID` 搬 `commands.go`、`GPUTexture` 摘 manager 钩子；`benchmark_test.go` 12→1（留 `BenchmarkSceneCreation`）；五目标构建 step3/4 全绿（raw `./...` 仅 `ui_wr_t_stress` 缺组件红）、vet 61/58 与 P2.2 **逐条一致**；逐测试文件 131 个：125 PASS / 5 仅编译 / 1 构建标签排除 / **0 FAIL**；API `render/internal/gpu` 761→630 符号（**−131 / +0**），测试 141/1008/109 → **131/852/79**，其余包零差异；负向对照 `NewBackend`/`newGPURenderer`/`NewGPUSceneRenderer` 编译红 → 撤销复绿零残留；`webgpu.CreateInstance` 白名单 4→3。无真窗：删除不触生产路径，按 §4.1 例外，1 扇基类窗金图对账并入 P2.6。公开 API 零变化（仅 internal 包）。归位：C2 移出本任务、D2 两组已结、新增 G1（spirv 空转用例）/G2（第二层老抽象） |
| 2026-10-29 | P2.1 | ⬜→🟡 开工；影响面评估落盘（未动工作区） | `evidence/P2.1/assessment.md`：两树同名 138 文件（128 仅 import 差、10 处真实差且方向全是 naga 更新：能力校验/swizzle 赋值/MSL 括号/64 位原子）；`gpu/shader` 独有 140 文件（57 源 + 83 测试，126,048 行）全是 SPIR-V/HLSL/DXIL 后端；合并后包 41→11、路径限定符号 1386→428（−962/+4）、测试 240/5850/68/4 → 70/2139/21/0（−170/−3711/−47/−4）；快照探针预演 27 WGSL/48 入口点/681 文件，两次逐字节一致；`gpu/shader` 根连带删 5 个 SPIR-V 符号、`render/internal/gpu` 连带删 `CompileShaderToSPIRV`（0 调用）——待 D1–D4 拍板后执行 |
| 2026-10-29 | P2.1 | 🟡→✅ 迁移完成：naga 144 文件落 `gpu/shader`，删三后端 167 文件 + 5 个根 SPIR-V 符号 + `CompileShaderToSPIRV`，15 处 import 改路径，补根冒烟（D4） | `evidence/P2.1/summary.md`：GLSL/MSL 快照 **681 文件逐字节一致**（sha256 清单相等、diff rc=0）、GL 离屏 md5 `59ca40…9824` 不变；五目标 step3/4 全绿（raw 仅 `ui_wr_t_stress` 缺组件红）、vet **61/58 逐条不变**；逐测试文件 248 个：220 PASS / 9 仅编译 / 18 构建标签排除 / **1 FAIL 为既有环境红**（`TestPresentDamageX11` 在 `4e348fd3` 同样失败，登记 G3）；API 41→11 包，根 9→5 符号（−5 SPIR-V / +`LowerWithCapabilities`），`ir` 增能力 API，`render/internal/gpu` 761→629（−132/+0 = P2.3 131 + 死函数 1），322 包同集合测试 1226→1210 文件 / 10,600→10,353 函数；负向对照 4 探针全红、撤销复绿零残留；残留 grep 全 0（唯一 `naga.` 为注释）；G1 随 `spirv/` 整包删除闭合。无真窗（零行为，§4.1 例外），P2.6 全量重放 |
| 2026-10-30 | P2.4a | ⬜→🟡 开工登记：影响面评估落盘，验证载体已登记，**方向待用户确认后才动代码** | `evidence/P2.4a/assessment.md`：三份现状 = 标准呈现链（`present_construct.go`，不动）+ `gpu_shared.initGPU`（先借共享、借不到自开 Vulkan）+ `vello_accelerator.initGPU`（`RequestAdapter(nil)` 绕过 policy）；`webgpu.CreateInstance` 白名单 3 处（backend.go + 两处 internal 直调）；`device.go` 镜像 2 组 + 死代码簇 6 符号（含测试 0 引用已证）；A3 现行探针：设备创建正常（TestBuffer 8 PASS），P1.3 Validation Error 当前不复现、P1.3 当时 conformance 编排用例 SKIP 无同环境对照，按 §4 四步解剖；C7 现场 = GL 第 4 窗 `eglMakeCurrent 0x3002`（EGL_BAD_ACCESS），本步只解剖定位；收敛设计 = render 导出唯一独立设备入口（严格 Vulkan 掩码 env 不劫持）+ 上收重试助手 + 删 8 符号；5 项拍板 D1–D5 待用户确认 |
| 2026-10-30 | P2.4a | 🟡→✅ 收敛完成（提交 `55495fdd`）：`webgpu.CreateInstance` 白名单 3→1（backend.go `createNativeInstance` 唯一句），新增 `render.OpenStandaloneDevice` 唯一独立入口，两处独立建链改调，删 device.go 镜像+死码 9 符号；D1–D5 用户拍板（方案 A/Vulkan 钉死/descriptor 归 P2.4b/先解剖/标实验） | `evidence/P2.4a/summary.md`：五目标构建段3/4 全绿、vet 61/58 逐条不变、残留 grep 漏斗唯一+零引用、API render +1func+1type（重试助手保持未导出）、逐测试文件 321 个 280 PASS（28 FAIL=27 既有 P1.3 同红 + 1 竞态单跑 7/7）、负向对照红过复绿、-race 过、能力单测 5 用例 + 双独立分支真机验证（compute_pipeline 例 CPU/GPU 像素一致）；真窗 ui_l1_blank native/go 双档 rc=0（嵌套 Wayland 隔离 3582/3596 帧）、multiwin native 8 窗三相位 PASS（rss 稳态）；multiwin go 第 8 窗撞 C7 既有根因（Wayland 第二机制待解剖，归 P2.4e）；A3 关闭（显存 OOM 假 Validation Error，`a3-dissection.md`）、C7 机制落定（`c7-dissection.md`）；**审计发现 P2.1/P2.3 internal/gpu 逐文件轮无 GPU 假绿 → 池 H1**。公开 API render +2 符号需同步 `RENDER_API_CATALOG.md`（提交前跑目录覆盖检查） |

## 8 修订记录

| 日期 | 修订 |
|---|---|
| 2026-10-28 | 首版：从 PLAN §7 拆出可执行任务清单，补状态图例、开工协议、开工/完成标准、门禁口径、证据约定、即时同步规则 |
| 2026-10-28 | 补新会话可续跑能力：§0 开场话术、§2.1 当前焦点、镜像允许按阶段聚合；同步规则改为同步 §2.1 与 PLAN |
| 2026-10-28 | 依赖校正：P1.4 补 P0.1；P2.6 补 P0.4；P3.7 补 P3.5；P3.8 补 P3.3/P3.5；P4.1/P4.2/P4.3 补 P3.4；P4.4 补 P3.3；P0.3 演练 tag 与正式基线 tag 分开，正式 tag 在 P1.8 通过后打 |
| 2026-10-28 | 新增 §4.1 能力验证硬要求：每个任务必须带能力单测 + 真窗；开工前在 §2.1 登记四类载体，完成交五份证据；同步规则加第 9、10 条 |
| 2026-10-28 | P0.4 收口：按 §4.1 一致性测试台口径（自带单测 + 3 场景双后端）标 ✅；资源断言以 K1 上界显式放宽（非静默跳过，已知差异表可见），收紧条件挂到 P1.6/P2.4c；修正「`GPUI_BACKEND` 切后端」「headless 不开窗」两处旧描述 |
| 2026-10-28 | P1.3 收口：1398 文件逐文件跑完，1264 PASS / 48 FAIL / 86 SKIP；T1 原始栈保留；46 个 RequestDevice SKIP 复现；证据 `baseline/tests/`；§2.1 焦点切到 P1.4 |
| 2026-10-29 | P1.4 收口：162 窗 / 183 变体 native+go 两档采集完成，native 191 轮 / go 183 轮，汇总 374 行无 MISSING；8 张运行期金图归档；FAIL/SKIP/BUILD/NO_JSON 逐条登记不修，修复归 P2；r6 金图真源不一致移交 P2；证据 `baseline/windows/`；§2.1 焦点切到 P1.5 |
| 2026-10-29 | P0–P1.4 重审（数字逐项重算核对，无假绿）：P0.4 「12 组」勘误为 6 组；SEED 显示口径写入 summary.md；新登记 T2（r1/r20/r22/c6 四窗两轮 rc=1 与 JSON 自报 pass 矛盾，病因归 P2 诊断） |
| 2026-10-29 | P1.5 收口：352 包 API 快照 + 1397 文件/14388 函数测试计数落盘 `baseline/api/`；机器校验零缺失零多余 ALL PASS；§2.1 焦点切到 P1.6 |
| 2026-10-29 | P1.7 收口：性能基线落盘 `baseline/perf/`，首帧 0 FAIL、A 族超限 4 轮与 P1.4 零冲突、字段两代命名定性并别名映射；§2.1 焦点切到 P1.8 |
| 2026-10-29 | 新增 §2.2 P2 待办池：审计 4 项新发现 + P1 散落「登记 P2」条目统一收口（23 项）；§2.1 注明 P2 开工先取池；P1.7/P1.8 两条流水从 §8 末尾归位回 §7；AGENTS.md:78 Vram 描述按代码事实修正（hal 账本 + rwgpu/gles 双端真实记账） |
| 2026-10-29 | P2.3 收口：删除依据从「无生产消费者」改为「能力替代核对」（§0.1 逐项载体），确认功能等价后执行；修正评估 §5 测试计数 142→147（+integration 9 = 156）；C2 移出 P2.3 归组件线，D2 两组已结，新增 G1（spirv 空转用例）/G2（第二层老抽象） |
| 2026-10-29 | P2.1 收口：按用户拍板 D1（根文件改 `shader.go`）/D2（`GLSLVersionToNaga` 留 P5）/D3（删根 SPIR-V API + `CompileShaderToSPIRV`）/D4（补根冒烟）执行；GLSL/MSL 快照 681 文件逐字节一致、GL 离屏 md5 不变、vet 61/58 不变、API 41→11 包；G1 随 `spirv/` 整包删除闭合；新增 G3（`TestPresentDamageX11` 既有环境红，`4e348fd3` 复现）；§2.1 焦点切到 P2.4a |
