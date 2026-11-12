# render/gpu 收敛任务台账（2026-10-28，2026-11-12 收敛版）

> 性质：**可执行任务清单 + 状态台账 + 同步规则**。新会话照本文件即可开任务。
> 方案依据与理由（标准选型、删除/合并决策、双后端矩阵、缺失/半成品登记）在 `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md`；本文件不重复论证，只回答「有哪些任务、先后关系、怎么算完成、现在到哪一步」。
>
> **本文件是任务状态的唯一真源。** 外部任务列表是它的镜像；两者冲突以本文件为准，并在当次会话内把镜像改回一致。
>
> **历史全文存档**：`docs/render_gpu_converge/archive/ENGINE_RENDER_GPU_CONVERGE_TASKS_2026-11-11_full.md`（2026-11-12 收敛前逐字快照，内容截至 2026-11-11；含全部历史载体块、§7 全量流水、§8 全量修订与过程性数字）。本文件只保留当前状态、口径、关键结论与证据指针；单次交付的过程细节查 `evidence/` 正本或上述存档。旧引用中的 `tasks.md:NNN` 行号对应存档 `NNN+7` 行（存档头部新增 7 行说明，正文逐字未动）。

## 0 新会话开工协议（照顺序做）

**开场话术（新会话直接粘贴这段）：**

> 按 `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` 继续 render/gpu 收敛任务。先读 `AGENTS.md`、`docs/render_gpu_converge/00_README.md`、`docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md`、本文件；按 §2 重建任务列表镜像，从第一个未阻塞任务开工；开工前在 §2.1 登记能力单测与真窗载体并核对 §3 条件，完成后按 §6 交齐三证据、单测与双后端结果；遇到「停下报告」节点必须停下确认。

**协议步骤：**

1. 读 `AGENTS.md`（项目硬规则）。
2. 读 `docs/render_gpu_converge/00_README.md`（本批文档索引）。
3. 读 `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md`（方案与门禁口径，尤其 §7）。
4. 读本文件 §2.1（当前焦点）与 §2.2（待办池），找到**第一个未完成且未被阻塞**的任务（状态 ⬜，依赖项全 ✅）。
5. 跑 `git status`：有非本任务线的未提交改动，先报告归属，不提交、不覆盖、不顺手清理。
6. 核对台账与任务列表镜像是否一致；不一致先按本文件修正镜像，再开工。
7. 开工前确认该任务的「开工条件」全部满足；不满足就停在条件项，不硬开。
8. 完成后按 §6 同步规则更新：**先改 §2.1.1 主表对应行与 §2.1 当前载体（原位置更新，禁止另起历史块）** → 证据落盘 → PLAN 修订（若口径有变）→ 任务列表镜像。
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
- 门禁命令实际跑过，输出或摘要落盘；不是「应该能过」。
- 行为零变化阶段（P1/P2）与基线对比的差异逐条对上白名单（PLAN §7.7）；对不上就不能标 ✅。
- 三证据（逻辑探针 + 像素断言 + 金图）齐全；只有逻辑指标绿不得标绿。
- 跨平台项：做了哪端写哪端；没做的端按符号标注原因（占位 / 实现中 / 本机不可验），不留空白格。

## 2 任务总表（状态唯一入口）

### 2.1 当前焦点

**当前焦点（2026-11-12 起）：P3.1b 色空间路线复核（建议 opt-in 线性目标，待拍板）。**

- ✅ **22 窗金图漂移已收口**：44 轮双后端采集 → 21/22 判基线过期（未发现渲染洞）→ 用户确认 → 21 窗重种；native/go 42 轮复跑全 `0.0000%`；窗口级 1px 负向 12.9901% 红→还原绿；`ui_render_base_svg` 0.0000% 不重种。
- ✅ **I5 v2 已收口**：`ui_base_vram_cap` 8 窗按既有证据重种；native Mailbox + go/gles 各 300s cascade，8/8 窗 `0.0000%`、像素 16/16、rc=0。
- 🟡 **P3.1b 路线已拍板 + 首批探针 PASS**：用户确认 **per-surface opt-in 线性目标**（默认 sRGB 路径/基线不动）；P3.1b-0 native webgpu sRGB 目标 + GLES `SRGB8_ALPHA8`/`GL_FRAMEBUFFER_SRGB` 探针均 PASS（线性混合字节 188/165/149）。下一步：swapchain/blit 探针 → 显式转换函数 → linear 目标固定功能 13 模式。
- 余项：D2（余留既有红，有意保留）、N1（可选）、N3 ✅ 2026-11-12（snapshot 竞态已修）、soak 两项；主表 P0–P4 全 ✅。
- 已收口：I1、I2、C7、I3、C1–C6、E1/E3、G3、B2/B3 等，详见 §2.2。
- **任务总表（P0–P4）在本节末**；下面的专项索引只列不属于主表 / §2.2 待办池的已收口项。
- **本节只保留 1 个「当前任务验证载体」**；任务收口时用一行索引替换它，不保留完整历史块（规则见 §6.1）。

**当前任务验证载体（2026-11-12 开工登记，评估阶段）**

- 任务：P3.1b opt-in 线性目标（默认 sRGB 不变；fixed-function 13 模式在 linear 目标走硬件线性混合）
- 能力单测：新增 `render/p31b_linear_target_test.go`（目标格式选择/显式 sRGB↔linear/固定功能 13 模式 linear 路由）、`render/internal/gpu/color_space_test.go`、`color_space_gpu_test.go`、`offscreen_format_srgb_test.go`、`srgb_target_probe_test.go`、`gpu/gwgpu/gles/srgb_fbo_probe_test.go`、`gpu/gwgpu/gles/srgb_blit_probe_test.go`、`gpu/webgpu/srgb_swapchain_probe_test.go`；沿用 `render/p31_blend_space_test.go`、`render/internal/gpu/color_format_test.go`、`gpu/gwgpu/gles/convert_test.go`
- 真窗：新增 `examples/ui_wr_p31b_srgb_chain`（默认 sRGB 目标 vs opt-in linear 目标对照；固定功能 13 模式 + 半透明中间调；`-auto-only`）
- 金图：新窗新金图，容差 0；默认窗旧金图保持不动（opt-in 路线不重采全量）
- 双后端：webgpu native 必跑；gwgpu-gles go 必跑；metal darwin 仅交叉编译（本机不可验）
- 负向对照：强制 Unorm 目标 → linear 降级/像素差红；摘 linear 目标输出转换 → 颜色偏差异红；默认 sRGB 路径旧金图保持 0（证明没被改）
- 证据：`evidence/P3.1b/assessment.md`、`evidence/P3.1b/p31b-1-tests.txt`、`evidence/P3.1b/probes/`（native-webgpu / gles-srgb-fbo / webgpu-swapchain / gles-srgb-blit 已 PASS）

已收口：22 窗金图漂移（21 窗基线过期重种 + 双后端 0.0000% + 1px 负向）→ `evidence/P2.6/golden-drift/assessment.md`；I5 v2（Mailbox present + 8 窗金图债清）→ `evidence/P2.4e/i5-v2/summary.md`、`evidence/P2.6/golden-drift/i5-vram/closeout.md`；C7（gles 多窗 Wayland：EGLDisplay 按 native display 引用计数 + 借用 display 不 terminate；8 窗 300s 全开/全 present/正常退出）→ `evidence/P2.4e/c7-wayland-verify/closeout.md`。

**§2.1 专项索引（主表未展开的细节 / 非主表专项）**

| 专项 | 结论 | 正本 |
|---|---|---|
| I1 ladder Wayland 卡死 | ✅ 2026-11-11 取证收口：嵌套 Wayland n=2 50.5fps / n=4 23.8fps+拒增 478 / n=8 fps 0.0；gdb 实锤 hub 阻塞在 `wgpuSurfacePresent → Mesa WSI → poll(-1)`；H1 成立/H2 排除；健壮化转 I5；隔离 X11 :3 与 headless Wayland PASS | `evidence/P2.4e/i1-verification/native-stack-verification.md`、`evidence/P2.4e/i1-dissection.md` |
| I2 同尺寸 Resize | ✅ 2026-11-12 复跑 PASS：同尺寸 no-op + `MarkNeedsReconfigure` 显式重配，与 Impeller KHR/AHB 一致；e4 `03a630be` 已改绑，台账滞后销账 | `evidence/P2.4e/i2/closeout.md` |
| M5 HDR 全链 | P0–P5 全批 ✅：RGBA16Float、格式参数化（设计面 22 文件，P1 实落 12 文件 43 处）、float RT、tonemap、真窗双后端 0.0000%（native 352 / go 357 presents，9/9 maxdiff=0）；HDR 直出不在本项 | `evidence/M5/closeout.md` |
| SDF 覆盖线 | GPU 覆盖改解析真值预计算 LUT（1024 项 midrange）；P4.2 AA 带容差 24→8（19 场景 max 6）；K2 erode/dilate 传播 404px/max17→5、548px/max5（带外 0）；6 扇金图重种 0.0000% | `evidence/SDF/coverage-line.md` |
| P4.3 阴影专项 | 跨帧阴影 tile 缓存；native 58.91fps / go 58.46fps，双后端金图 0.0000%；滤波源帧内累积已治根（同帧两笔阴影 max diff 38→0） | `evidence/P4.3/cache-impl.md`、`evidence/P4.3/root-cause2-fix.md` |
| P4.3 余项 | backdrop 损伤区 / 抖动两项经二轮量测判不修（用户拍板，正确性设计保留）；零引擎改动、零公开 API、金图不重种 | `evidence/P4.3/closeout.md` |
| 22 窗金图漂移 | ✅ 2026-11-12 收口：44 轮采集 → 21/22 判基线过期（0.1880%–31.7164%），22 窗内容双后端逐字节同、平移 (0,0)、内部采样多数 100%；用户确认后 21 窗重种，42 轮复跑 0.0000%，1px 负向 12.9901% 红→还原绿；`ui_render_base_svg` 0.0000% 不重种 | `evidence/P2.6/golden-drift/assessment.md` |
| I5 金图债 | ✅ 2026-11-12：`ui_base_vram_cap` 8 窗按 i5-v2 当前图重种；native Mailbox/go gles 各 300s cascade 8/8 `0.0000%`、像素 16/16；首轮 win1 snapshot 竞态登记 N3 | `evidence/P2.6/golden-drift/i5-vram/closeout.md` |
| Wayland 原生截屏 | portal/PipeWire 需人工授权，GNOME Shell 接口 AccessDenied；另立后续独立批次 | 同上 |

**余留登记（不阻塞当前焦点）**

- `ui_l1_spinner` 2h soak 定性：唯一 60s 仍过线 35161，RSS 12MB→302MB；按 P0.1「2 小时 RSS 涨 ≤5%」判真泄漏 vs 缓存上限。
- `ui_text_m4_virtual` go 60s 稳态斜率波动（28586.8 / 37791.2 / 32694.3），与 l1_spinner 一并归 soak 定性，不在门禁批下结论。
- N1（backdrop 静止帧 + retained/hybrid 二轮量测）为可选后续，见 §2.2。
- 历史尾巴（P2.4e / P3.1 / P3.5 期立案，快照内未见销账，未销即仍挂）：`misc_draw` wgpu 执行态（需 GPU capture 工具）、`SeatCapCB` 回调 robustness（flaky 实证）、gles `DeviceLost` / `X11KeyRepeat` 两新立案、P3.1b「sRGB 目标格式整链」（HDR/广色域前置；M5 格式参数化只承接其中一部分，整链未单独收口）；ui 5 条既有红（app_test SIGSEGV / x11_s6_p0 / m1 / m5 / gpupixel）在红名单登记、不阻塞。以 `evidence/P2.4e/summary.md`、`baseline/tests/` 与各自证据目录的实际状态为准，销账时回写。

#### 2.1.1 任务总表（P0–P4，状态唯一入口）

| 编号 | 任务 | 依赖 | 状态 | 证据目录 |
|---|---|---|---|---|
| P0.1 | 验收阈值与环境清单（首帧/滚动帧率/p95/p99/显存上限/金图容差，写死数字并指明真源） | — | ✅ | `baseline/thresholds.md` |
| P0.2 | 真窗与金图清单（窗口名 → 金图 → 平台 → 容差 → 单窗/批量 → 覆盖能力标签） | — | ✅ | `baseline/windows.md` |
| P0.3 | 归档与回滚演练（打 tag/建分支，实际回退一次证明可回） | — | ✅ | `baseline/rollback.md` |
| P0.4 | 双后端一致性测试台（同场景 webgpu/gles/metal 逻辑断言，不比像素只比语义） | — | ✅ | `baseline/conformance/` |
| P1.1 | 六目标交叉编译 + `-tags nogpu` | P0.1–P0.3 | ✅ | `baseline/build/` |
| P1.1b | 跨平台构建修复（B1–B6，插队：P1.2 前完成并复跑 P1.1） | P1.1 | ✅ | `evidence/P1.1b/` |
| P1.2 | `go vet ./...` | P0.1–P0.3 | ✅ | `baseline/vet.txt` |
| P1.2b | nogpu 测试标签修复（20 个测试文件补 `//go:build !nogpu`） | P1.2 | ✅ | `evidence/P1.2b/` |
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
| P2.4b | 预算统一（六口径 → 一套 Budget，按域分账） | P2.4a | ✅ | `evidence/P2.4b/` |
| P2.4c | 延迟释放统一（三队列 → `res/registry`） | P2.4b | ✅ | `evidence/P2.4c/` |
| P2.4d | 同尺寸纹理分配收敛（六处 → `res/cache`） | P2.4c | ✅ | `evidence/P2.4d/` |
| P2.4e | 裁剪 / 混合枚举 / 呈现入口收敛 | P2.4d | ✅ | `evidence/P2.4e/` |
| P2.5 | 包组织收口：`go list` 对比、分层约束测试、索引与总账同步 | P2.4e | ✅ | `evidence/P2.5/` |
| P2.6 | 行为零变化总验收（P1 基线全量重放，是进入 P3 的硬闸） | P2.5、P0.4 | ✅ | `evidence/P2.6/` |
| P3.0 | 依赖图 + 文档更新（PLAN §5 + `RENDER_API_CATALOG.md`），再实现 | P2.6 | ✅ | `evidence/P3.0/` |
| P3.1 | 色彩管理地基（含线性混合接线，原半成品前移） | P3.0 | ✅ | `evidence/P3.1/` |
| P3.1b | opt-in 线性目标（默认 sRGB 不变；fixed-function 13 模式在 linear 目标走硬件线性混合；sRGB↔linear 显式转换；HDR/广色域前置） | P3.1、M5 | 🟡 2026-11-12 P3.1b-0 四项探针 + P3.1b-1 颜色转换地基 PASS；下一步 linear 目标固定功能 13 模式 | `evidence/P3.1b/` |
| P3.2 | 自定义 shader（依赖 P2.1 翻译器 + 新公开 API） | P3.0、P2.1 | ✅ | `evidence/P3.2/` |
| P3.3 | 富文本（含 `DrawParagraph` 逐 run 样式，原半成品前移） | P3.0 | ✅ | `evidence/P3.3/` |
| P3.4 | MaskFilter / ColorFilter / ImageFilter 家族 | P3.0、P3.1（软） | ✅ 2026-11-01 五批 + 真窗 6/6 双档 + 金图 diff 0；C 卡三根因闭合；2026-11-10 补 16 节点 CPU/GPU 单测 + `ui_wr_p34_filters` 4×4 十六卡（29 探针、0.0000%），displacement GPU 改 fail-closed | `evidence/P3.4/`、`evidence/P3.4/node-matrix.md` |
| P3.5 | 路径与几何五项：conic / 四角 RRect / Matrix4 透视 / PathEffect / 布尔保曲线；各有独立真窗 | P3.0 | ✅ 四扇窗（conic 8/10、rrect4 6/9、persp 9/17、patheffect 10/23）三证据合证 + 金图逐位；窗口抓出并修四处引擎洞（面积口径/RRect 退化 cubic/裁剪掩码越界/透视命令分组）；余留=真曲线布尔引擎 D6 远期另立 + 三支柱生产接线。注：rrect4 go 档收口时残留 0.0038% 角点差（M2 跟踪），2026-11-11 已由 P4.1 批 4 消除至 0.0000% | `evidence/P3.5/`（含 `windows-close.md`） |
| P3.6 | 图片格式（GIF/APNG/动图 WebP/EXIF 方向）+ GPU mipmap；AVIF/HEIF 剔除本项（认出但具名 `ErrUnsupportedFormat`） | P3.0 | ✅ 全六批收口（2026-11-03）：批 1a/1b + 1c + 2 + 1d + 批 3；`MipmapMode` 全量切换、图号=内容版本、解码注册表单一真源 | `evidence/P3.6/` |
| P3.7 | 2.5D 透视相机、世界高度/精灵 z 序、自定义材质、HDR/泛光 | P3.0、P3.1、P3.2、P3.5、P3.6（软） | ✅ 六批全收（2026-11-06）：批 1 透视/2 z 序与高度/3 材质（3a–3e3）/4 mip/5 泛光/6 六扇真窗（persp_batch、height_zsort、material、mip_exit、bloom、ui_wr_p37_scene）全像素收口，双后端金图 0.0000%；H19–H23 修复；批级门禁 64 包 903/12/24/1 只减不增、conformance 双后端、整包 `-race` DATA RACE=0 | `evidence/P3.7/`（含 `batch6/`） |
| P3.8 | SVG 完整语义、彩色字形、嵌套 Picture 引用 | P3.0、P3.3、P3.4、P3.5 | ✅ 六批全收（2026-11-08）+ 批 4b/4c/4d/4e/4f 补全；resvg v0.42 conformance 全量基线 73.9%→94.0%→94.1%，零 blank 零崩溃；2026-11-10 G14 marker 补齐（19 case + `ui_wr_p38_svg_structure` 卡 5） | `evidence/P3.8/` |
| P3.9 | mesh gradient 低层：Coons 12 点 / TensorProduct(bicubic) 16 点、4 角角色色、2×2/3×3 网格 | P3.0 | ✅ 2026-11-09 像素收口（native/go 各自 0.0000%/763584px、logic 6/6、pixel 16/16）；同批修 GLES `DrawIndexed` 忽略 `baseVertex` | `evidence/P3.9/` |
| P3.9a | mesh gradient 高层网格（阶段 1）：rows×columns patch、自动 C1 推断、Bilinear/CatmullRom、一次绘制合成 indexed Mesh | P3.9 | ✅ 2026-11-09 像素收口（native/go 各自 0.0000%/763584px、logic 3/3、pixel 8/8、跨后端 changed=0） | `evidence/P3.9a/` |
| P3.9b | 1x/4x MSAA 真实窗口动画性能基准（测量补充，非新能力）：满窗混合动画、分辨率/MSAA 可配、三模式 | P3.9a | ✅ 2026-11-09 测量收口（native 1x/4x + go 1x；1x→4x 显存 2.35×、fps −24.9%、p95 破线）。go4x 原整窗全黑，2026-11-10 由 P4.1 批 4 修掉 GLES 4x 链三洞后翻绿（复核 6/6 + 金图 0.0000%，go4x 金图补种），原「不可验」登记随之销账 | `evidence/P3.9b/` |
| P4.1 | 正确性：MSAA>1 even-odd 裁剪、分数裁剪 AA、顶点网格 alpha/blend、DrawShadow | P3.4、P3.8 | ✅ 拆四批（③→②→④→①）全部收口：批 1 B 卡 #3 GPU 复合 2026-11-10 修复；批 2 分数裁剪 9/9+10 探针；批 3 DrawShadow 9/9 探针、双后端 0.0000%，另登记 #4 投影×层×blur×GPU readback 时序、#5 tier 顺序洞（后被 P4.3 推翻为根因）、#6 局部 SetProjection×dpr×GPU 层组合域丢内容三个 latent 项（真窗已避开，未销账）；批 4 even-odd+go4x 三洞闭环、1x 全量金图零新增红、P3.9b go4x 翻绿 | `evidence/P4.1/` |
| P4.2 | 一致性：GPU/CPU 模糊对齐 | P4.1、P3.4 | ✅ 2026-11-10 收口：20 场景双腿对拍，内部 max ≤2；剩余只落 1px AA 带 max18，归 SDF 覆盖线；SDF 闭合后 AA 带容差 24→8（19 场景 max 6）+ 容差归零负向；零引擎改动 | `evidence/P4.2/parity.md` |
| P4.3 | 性能：mask 临时纹理池化、backdrop 损伤区合成、抖动路径优化；+ DrawShadow 阴影模糊整画布成本 | P4.1、P3.4 | ✅ 2026-11-10 整表收口：阴影专项（native 58.91/go 58.46fps、金图 0.0000%）+ mask 按尺寸池化（HEAD `c8f64d98`、稳态零分配）；backdrop/dither 量测后用户拍板不修（`closeout.md`）；零引擎改动、零公开 API | `evidence/P4.3/closeout.md` |
| P4.4 | 文本：COLRv1 完整层（若 P3.8 未覆盖） | P3.3、P3.8 | ✅ 2026-11-10 收口：15 个 `Var*` 格式 delta + VarColorStop + text 桥接（IVS/avar）+ ColorRasterCache variations + GPU 颜色图集接线；真字体 198 字形/74 变化；颜色窗卡 5（SWPS -90/0/+90，logic 11/11），双后端金图重种 0.0000% | `evidence/P4.4/` |

**并行说明**：P0.1–P0.4 互相独立，可并行；P1.1/P1.2/P1.5 可在 P0.1–P0.3 完成后并行；P2.1–P2.3 互相独立，可并行；P3.2–P3.8 在 P3.0（及各自前置）完成后可分批并行，但每批合入前都要跑所属域回归。P2.4a–e 必须串行。P2.6 除 P2.5 外还必须等 P0.4 的一致性测试台就位。

### 2.2 P2 待办池（2026-10-29 建池）

> 用途：P1 期间「登记 P2」散落在 §7 流水与各 baseline summary 里，加上实现层对账审计的新发现，统一收到这里；P2 开工先从这里取项，不再靠记忆。
> 取用规则：取走一项 → 在 §2.1 按 §4.2 登记载体 → 对应任务状态改 🟡 → 完成后证据落盘、本行改 ✅（或写清转去哪个任务正文）；不许绕过载体登记直接修。
> 归位口径：审计发现里未落到具体 P2 任务正文的进本池；其余在 `baseline/{tests,vram,perf,negative}/` 各 summary 与 `baseline/thresholds.md` 有案，不重复进池。合规面两条分层必查（`ui` 不直引 `gpu`、`render` 无后端特有类型）实测通过。

| 编号 | 来源 | 现象 / 结论 | 归属 | 状态 / 证据 |
|---|---|---|---|---|
| A1 | 审计 | `AGENTS.md:78` 称「`webgpu.Vram*` 仅 Rust 记账、GL 窗暂不依赖」已过时：账本在 `gpu/hal/vram_ledger.go`，`gpu/rwgpu/texture.go:342` 与 `gpu/gwgpu/gles/resource.go:103/174` 双端真实记账，`gpu/webgpu/vram.go:20` 只转发查询 | 就地修正 AGENTS | ✅ 已修 |
| A2 | 审计 + P1.7 | 帧指标两代 json tag 并存：`ui/scheduler/metrics.go:58-61` 发 `avg_frame_interval_ms` + `frame_interval_p50/p95/p99_ms`，`examples/wrgate/report.go:32-33` 等发 `interval_p50/p95_ms`；旧代窗还缺 `fps_wall`/`target_hz`（6 窗 × 2 后端） | P2.5（一次切 `interval_*` 并补齐字段） | ✅ 2026-10-31 闭合；3 旧代窗 × 2 后端真窗新键齐、旧键零残留；`evidence/P2.5/a2-window-runs.txt` |
| A3 | 审计 | 46 个 `RequestDevice unavailable` SKIP 的根因未解剖 | P2.4a 解剖，解剖前不定修复方向 | ✅ 2026-10-30：真因=显存 OOM，wgpu 把「Not enough memory」包装成 Validation Error；环境依赖非代码缺陷，不修；`evidence/P2.4a/a3-dissection.md` |
| A4 | 审计 + P1.1b | `render/gpu/gpu_nogpu.go` 是 B1 的空包补丁；最小侵入性质与消解路径未登记 | P2.5 评估归位 | ✅ 2026-10-31：保留（B1 引擎层正解），消费方实测 193 文件/190 空白导入；顶层包拆分时自然消解，零代码改动 |
| B1 | P1.6 | `baseline/windows.md:213` 写 `ui_base_vram_cap` 运行档 A/60s，代码 `main.go:67` 是 `gateSeconds=300` | P0.2 勘误（baseline 只增不改） | ✅ 2026-11-10 修订表追加勘误行，原表不动 |
| B2 | P1.4 | `ui_wr_r6_layer_anim` 表内金图 `golden/baseline_igpu.png` 与代码真源不一致（代码读 `R6_SNAP_DIR`，仓内图未被读取） | P2.6 对齐真源/代码 | ✅ 2026-11-11：默认读仓内 `testdata/goldens/r6_final_base.png`（go 用 `_go.png`），快照仍写 `R6_SNAP_DIR`；native/go 各两次 0.0000%（306280 px）、scripted 6/6；1px 负向 0.0003% 红；`evidence/P2.6/b2-r6-golden-source.md` |
| B3 | P1.4 | `ui_text_edit_accept` script 金图自动模式不逐位比较 | P2.6 补逐位或写替代判定 | ✅ 2026-11-11：`RetainedSnapshot` 按 mark 裁片与三片 golden 零容差逐位；两次 3/3 diff=0、1px 负向红；基线重种，旧 xwd 归档；`evidence/P2.6/b3-script-golden-bitwise.md` |
| C1 | P1.4 | 14 窗自带 x11grab screen check 在 rootless Xwayland 抓黑图 → 环境 FAIL（backdrop p95≈137ms 也被拉高） | P2.6 重放环境 | ✅ 2026-11-11：真因=解析洞（xwininfo 行按首个冒号切分，winID 恒空）+ rootless Xwayland 不绘制。22 窗收敛到 `examples/wrkit/screencheck.go`（xwd -id 为主 + 显式 SKIP + 负向 env）；X11 14+8 窗 0.0000%，Wayland `screen_check_skipped=true`；`evidence/P2.6/c1-screen-grab-black-assessment.md` |
| C2 | P1.4 | `ui_wr_t_stress` BUILD_FAIL：引用不存在的 `ui/kit/button`、`ui/kit/icon` | 组件线 | ✅ 2026-11-09 源码实证：示例目录已不存在；kit icon 已齐全，BUILD_FAIL 随示例删除消失 |
| C3 | P1.4 | `ui_ime_probe` 不响应 WM_DELETE，人工探针按兜底 SIGTERM 记 PROBE | P2.4e（关闭协议） | ✅ 2026-11-11 实测：修复已随 P2.4e e4 落地；X11 直发 WM_DELETE，probe 打印 `close requested` 并自行退出 |
| C4 | P1.4 重审 | T2：`ui_wr_r1_layout`/`r20_filter`/`r22_selection_stub`/`c6_savelayer_group` 两轮 rc=1 而 JSON 自报通过、金图 diff=0 | P2.6 前诊断 | ✅ 2026-11-10/11：机制=窗先打印 JSON 后评门，rc 才是判定；r1/c6=U21 快照回读顶破 hitch、r22 同族压线、r20=门与现行拓扑脱节；修 r20 门 + 引擎快照回读不计 hitch（可嵌套 Pause/ResumeHitchCount）；四窗双后端 8 轮 30s PASS；`evidence/P2.6/c4-dissection.md` |
| C5 | P1.6 | 10 条 PASS 记录 RSS 稳态斜率过 30000KB/min 泄漏线（r17_bg_throttle 88801 最高） | P2 判定窗门禁集是否含 RSS 线 | ✅ 2026-11-11：`wrgate.EvaluateGates` 无 RSS 判定，6 老窗不调用；`GateOptions` 加 30000/60s（短窗只观测）+ 5 窗接线；10 条复跑 9/10 不复现；60s 32694.3 判红；l1_spinner 另立 soak；`evidence/P2.6/c5-rss-slope-gate-assessment.md` |
| C6 | P1.6 | `ui_base_batch_cap` p95 双后端 55.7/62.1ms 超 22ms 门禁（P1.4 同因同量级） | P2 性能线 | ✅ 2026-11-11：非引擎洞——报告百分位是末 256 采样环（`ui/scheduler/metrics.go:24`），gate 收尾冻结在刻意超载档 vector-600（`main.go:420-437`），报告 p95≈档内 p95（52.98/59.89 ↔ 53.0/59.1）；容量阶梯窗不适用 22ms 硬线。零代码，thresholds/windows 修订表补勘误；`evidence/P2.6/c6-batch-cap-p95-scope-assessment.md` |
| C7 | P1.6 | `ui_base_vram_cap` ladder go 档 GL 第 4 窗起 `eglMakeCurrent 0x3002`，8 窗阶梯跑不了 | P2.4a/P2.4e（gles 多窗上下文） | ✅ 2026-11-12 闭合：X11/0x3002 随 e4/J5 已闭；Wayland 关窗 SIGSEGV 根因=per-surface context 各自 `eglTerminate` 同一 native display；修=EGLDisplay 按 native display 引用计数 + 借用 Wayland display 不 terminate；go/gles 8 窗 300s 全开/全 present/正常退出，X11 go smoke rc=0；金图 17.5273% 为 P2.6 预存漂移；`evidence/P2.4e/c7-wayland-verify/closeout.md` |
| D1 | P1.3 | `gpu/rwgpu/queue_workdone_test.go:45` 失败后 cleanup `Release()` 在 `device_misc.go:313/314` SIGSEGV | P2.4c/P2.4e | ✅ 2026-11-10：根因=92ca9216 时代 `queueWorkDoneCallbackInfo` 缺 v29 `Mode` 字段，结构错位、callback 槽拿到记录 id → 跳地址 1；v29 对齐后现行 HEAD 4 用例 PASS；布局钉 `TestQueueWorkDoneCallbackInfoV29Layout`；`evidence/P2.4e/d1-dissection.md` |
| D2 | P1.3 | 48 FAIL 分组归属：老 backend 并发 SIGABRT→P2.3；StubHost 假 X11 句柄→P2.4e；nil ImageCache/共享布局/无 GPU 前置/depcheck→P2.5；video 3 文件+顺序耦合→P2.6；CPU 彩色字形无 ink→P2.3 | 见左 | 🟡 分组基本已结；余 `ui/application` SIGSEGV 等残留红为有意保留，不擅改 |
| D3 | P1.2b | vet 3 条测试伪造指针 → P2.5 测试卫生；2 条 `unreachable`（`gpu_render_context.go:4296,4344`）→ P2.5 | P2.5 | ✅ 2026-10-31：3 条改真实地址 + 2 条死块删除；vet 61/58→56/56 |
| D4 | P0.4/P1.6 | K1：`solid_gradient`/`text_image` 每轮 +2 条/+131072B（native/go 逐位一致） | P2.4c（修完收紧回 0） | ✅ 2026-10-30：`TexturedQuadPipeline.destroyPipeline()` 漏销毁 bicubic 两管线 + `bicubicShader`，已补；上界收紧 0/0，双后端三场景 PASS；`evidence/P2.4c/summary.md` §3 |
| E1 | P1.7 | A 族超限 8 处集中 4 轮：c3_list_scroll（fps+p95+hitch）、r7_virtlist（p95+hitch）、r7b_scroll_reuse p95 22.42 临界 | P2 性能修复 | ✅ 2026-11-11：现行树 7 轮复测四项全部不复现（p95 19.46–21.22、hitch 0–3/min、p99 max 28.78 < 32.0）；显示/后端变量排除；候选=SDF LUT/P4.1 resolve/P4.3 mask 池化（未 bisect）；`evidence/P2.6/e1-afamily-overrun-assessment.md` |
| E2 | P1.7 | `video_vr7_perf` 缺 `interval_p99_ms` | P2（随 A2） | ✅ 2026-10-31 随 A2 闭合；vr7 报告/buildReport/requiredKeys 补字段 |
| E3 | P1.4/P1.7 | 6 轮采不了档（NO_JSON / BUILD_FAIL）沿用 P1.4 登记 | 随 C2 及对应窗修复 | ✅ 2026-11-11：6 轮零现存故障——t_stress×2 随 C2；zz_soak_control×2 是设计上不出 JSON 的多窗 soak（stderr CTRL p95 19.82/20.59 健康），采集契约勘误；r7b go / r15_soak go 的 EGL 0x3002/0x3000 现行树双路不复现；`evidence/P2.6/e3-nojson-recheck-assessment.md` |
| F1 | P1.8 | 一致性台 `gpu_ops` 计数下限是 1，单次跳绘抓不住 | P2 收紧为用例精确 op 数 | ✅ 2026-11-11：三份用例 `min_gpu_ops` 1→3/3/2；负向 `GPUI_NEG_SKIP_DRAW` 六路全红且首条即 `gpu_ops=2 < min=3`；`evidence/P2.6/f1-f2-gate-probes.md` |
| F2 | P1.8 | oom probe 的 `pre==post` 对「整体抬底」型泄漏不敏感 | P2 probe 语义补强 | ✅ 2026-11-11：加早期基线窗 + `EarlyMB/EarlyN/BaselineStable`，判定 `pre-early>tol`；造真泄漏（每 6 帧 512² 纹理不释放）跑红，还原零残留；同上证据 |
| G1 | P2.3 | `gpu/shader/spirv/internal/codegen/shader_test.go` 6 个用例读从未存在的 `../../gg/internal/gpu/shaders/*.wgsl`，一直 Skipf 空转 | P2.1（shader 线） | ✅ 随 P2.1 删 `spirv/` 整包闭合（2026-10-29） |
| G2 | P2.3 | `render/internal/gpu` 第二层老抽象：四文件只被测试/示例可达，但 `RectAllocator`/`IndexFormat`/`*RenderPass` 仍被用 | P2.4e（入口收敛） | ✅ 2026-11-09：四文件已删；`RectAllocator` 独立 `rect_allocator.go` 且仍被 `glyph_color_atlas.go:54` 使用 |
| G3 | P2.1 | `TestPresentDamageX11` 在 X11/NVIDIA 失败（`full=12 partial=0` / `BadMatch` on `X_GetImage`），干净树同败 | GL 增量呈现线 | ✅ 2026-11-11：display 无 `EGL_KHR_partial_update`，SetDamageRegion 逐帧 FALSE 被静默吞回落整帧；`resolvePresentDamageCapabilities`（符号∩扩展串）创建时解析一次 + 能力位决策 + 降级计数/首次 Warn；`:0` FAIL→SKIP（全帧像素断言先过），解析单测 4 例，全包/-race ok；公开 API 零变化；`evidence/P2.6/g3-partial-present-assessment.md` |
| H1 | P2.4a | `render/internal/gpu` 逐文件轮为无 GPU 假绿：门禁脚本未导出 `WGPU_NATIVE_PATH`，native 用例全 Skipf | P2.6 重放纪律 | ✅ 2026-10-31 skip 文案补 OOM 说明；规程=GPU 包逐文件门禁必须带 lib 或显式记「无 GPU 轮」 |
| I1 | P2.4b | `ui_base_vram_cap` ladder native 在隔离嵌套 Wayland 8 窗卡死：hub 阻塞 `rwgpu.Surface.Present → wgpuSurfacePresent`，fps 0、拒增 +0、RUN_SECONDS 到期也退不出；干净树复现 | P2.4e；取证完 → I5 | ✅ 2026-11-11 取证收口：Mesa WSI `poll(-1)` 等合成器 release；H2 排除；环境相关；健壮化转 I5；`evidence/P2.4e/i1-verification/native-stack-verification.md` |
| I2 | P2.4d | `TestS68_Swapchain_X11_MultiFramePresent` 曾期望第 6 帧同尺寸 `Resize(160,120)` 抬 `reconfigures`；产品侧 `swapchain_configure.go` 自 `34d9785e` 起把同尺寸 Resize 定为 no-op（避 `Surface.Configure` thrash/黑闪）；`8e4c0e99` 引入用例早于语义变更 | P2.4e：已拍板「同尺寸 no-op + `MarkNeedsReconfigure` 显式重配口」，与 Impeller KHR/AHB 一致；不改 `Resize` 签名 | ✅ 2026-11-12 复跑 PASS（`reconfig=0`；`MarkNeedsReconfigure` 后 `>=1`）；e4 `03a630be` 已改绑，台账滞后销账；`evidence/P2.4e/i2/closeout.md` |
| I3 | P2.4d | R20 `gray_red_band_matrix_exact @(408,274)` 红：GRAY 卡保持彩色，疑层录制未按新矩阵重录 | P2.6 前诊断 | ✅ 2026-11-10：现行树 scripted 5/5、金图 0.0000%、`final_gray_identity=false`，复现消失；按「核实后没问题如实收回」销账，无代码改动；`evidence/P2.6/c4-dissection.md` |
| I4 | P2.4d | R20 `r20_steady.png` / `r20_recover.png` 全黑（1200×800 RGB 全 0、5585B），stderr `snapshot request timed out`；`r20_final.png` 正常 | P2.6 前诊断（SnapshotAsync 黑图） | ✅ 2026-11-11：根因=saveSnap 用 `Context().Image()` 读 CPU pixmap，真 swapchain GPU 帧不回写 pixmap；按 r1/c8 `PaintPresentTree` 先重画再存；两窗 steady/recover 960000/960000 px 非零，门禁 PASS；`evidence/P2.6/i4-midrun-snapshot-black.md`、提交 `46aa5454` |
| I5 | I1 | **GPU present 健壮化**：Wayland 同步 Fifo present 在 Mesa WSI 等合成器 release，一窗卡住即全进程冻结；根因修复=native Wayland 默认非阻塞 present（Mailbox→Immediate→FifoRelaxed→Fifo）。旧 worker/每窗设备路线已被 2c/D1c 反证作废 | render 呈现口 + `gpu/webgpu` present 模式选择；旧设计标废弃 | ✅ 2026-11-12 收口：v2 实现 + 真窗 A/B/负向/X11 回归已落盘；`ui_base_vram_cap` 8 窗按既有证据重种，native Mailbox/go gles 各 300s cascade 8/8 `0.0000%`、像素 16/16、rc=0；`evidence/P2.4e/i5-v2/summary.md`、`evidence/P2.6/golden-drift/i5-vram/closeout.md` |
| J1 | P2.4d 收口审计 | `res.Registry` idle 回调电平触发 + `res.Cache.repool` 无查重：同一 `Ref` 双重 Release 时池桶重复入桶，后续可能把在借条目标死 | P2.4e：repool 查重 + 回调边沿触发 + 补负向用例 | ✅ 2026-11-09 源码实证：`res/cache.go:252-255` 已查重；`registry.go` idle 回调已边沿触发 |
| J2 | P2.4d 收口审计 | `rwgpu` work-done 记录包级 map 仅在 `FutureDone`/`WaitForFuture` 观察时删除；`directSubmit.drain/invalidate` 不解 token → 设备切换/丢失时记录滞留 | P2.4e：drain 补观察 + invalidate 清理出口 | ✅ 2026-11-09：`direct_submit.go:139-141` `drain()` 已 `token.Done()`；`invalidate()` 已标注丢弃 |
| J3 | P2.4d 收口审计 | 疑点「`Submission.Begin` 丢未决 jobs → inflight 泄漏」经全调用点核查收回：`Begin` 生产零调用，`endSubmission` 5 处同步点 + Destroy WaitIdle + Invalidate 配对闭合 | 无需修复（防重查） | ✅ 2026-10-31 核查闭合 |
| J4 | P2.4e e4 门禁 | `ui/` 不在逐测试文件门禁清单（9 包口径）→ `ui/depcheck_test.go` 与 `ui/application TestFocus_KeyWithNoPrimary` 潜伏既有红 | P2.5（清单补 ui + 测试缝豁免） | ✅ 2026-10-31：`tools/per-file-tests.sh` + `.pkgs` 正本 34 包（主链 9 + ui 25）；ui 5 条既有红实核登记；focus 随 P2.4e 转绿 |
| J5 | P2.4e e4 C7 修复后 | go 档 multiwin 关窗 SIGSEGV（3 轮 2 挂）：Wayland 每 surface 自有 EGL context，共享 Device 挂首窗 AdapterContext，首窗销毁后其余窗打空指针 | P2.4e：上下文所有权设计缺口 | ✅ 2026-10-31：AdapterContext 引用计数（Device.Open 持一笔，Surface.Destroy/Instance.Release release，末笔销毁）；multiwin go ×3 rc=0、panic=0；`evidence/P2.4e/j5-ownership-refcount.md` |
| K1 | P3.4 C 卡 | `render/recording/backends/raster/backend_coverage_test.go:24` vet 编译错：`FillRect` 少参（P3.1 签名变更） | recording 域补 BlendSpace | ✅ 2026-11-03：raster 两测试 11 处 + 示例 spy 4 方法同笔补；全仓 build/vet rc=0 |
| K2 | P3.4 C 卡 | 真窗 `ui_wr_p34_filters` 在 `GOGPU_RENDER_MODE=cpu` 下 A 卡 erode 探针红，CPU 滤镜链边缘语义与 GPU 有差 | P3.4 后续复核 CPU 链边缘语义 | ✅ 2026-11-11：CPU/GPU 窗口语义一致，真根因=GPU 栅格整数边界 AA 覆盖差（max 18 同族），被形态学半径放大；归 SDF 覆盖线。LUT 后 erode 404px/max17→5、dilate 548px/max5，带外 0；永久钉 `render/k2_morphology_propagation_test.go`；`evidence/SDF/coverage-line.md` |
| K3 | 审查修复批 P0-3 | cluster 兜底全零：`shape_result_cache.go:405-419` 不填 Cluster；`ui/rendering/text_layout.go:635,679` 无 face 估算兜底全零 | 文本专项（接线 cluster 前补全） | ✅ 2026-11-09：`shape_result_cache.go:414-415` 已 `Cluster: glyph.Index`（0608c605 批 6）；ui/rendering 兜底仍 0 属独立 latent 面 |
| K4 | 审查修复批 P0-3 | CRLF 富文本 span 边界偏 1：builder 按原文计数，layout 归一 `\r\n→\n` 后 rebase，多段+CRLF 时 run→style 边界偏 1 | render/paragraph Build 归一后算 spanEnds | ✅ 2026-11-11：新 `normalizeLineBreaks` 逐 span 归一后重算，AddText 保持原样（LF/无换行零变化）；3 钉先红后绿 + 变异负向；`evidence/P2.6/k4-crlf-span-boundary.md` |
| K5 | 审查修复批 P0-3 | `render/scene` GlyphEntry 不携带 cluster → `RenderedGlyph.Cluster` 恒 0 | 随 P3.8 定夺穿链或删面 | ✅ 2026-11-09：`scene/renderer.go:1298` 已 `Cluster: int(g.Cluster)`（0608c605 批 6，14B 全量切换） |
| K6 | P3.8 批 5/6 | `./render/` 全量测试红 20 枚，单跑全绿；`render/text` 全包多次超时=体量非失败；并发偶现一枚 | 交付后审查批：全局污染源定位 + text 分文件门禁 | ✅ 2026-11-09/11：真修 12 枚（advanced blend 两层、WebP 预乘、CS03 渐变契约、DrawShadow 1-LSB、TextTransform、PresentDamage、Rasterizer 双口径等）；干扰定性=单进程累积 device lost → `gpu_textures.go:170` 把 released 当 depth-OOM 降级 1x1 → 连锁；分片门禁 5 命令为正式口径；11-11 分片 1–4 全绿，S69 计时门波动 A/B 同红登记；text 入 `tools/per-file-tests.pkgs`；深度 OOM 分类 fail-closed（`surface_cache.go::createSurfaceTrio`）；`evidence/P2.6/k6-depth-oom-classification.md` |
| K7 | 审查修复批 P1-7 | WGSL 模糊核改动态循环界，gwgpu/gles 自动翻译链未单独复验 | 五目标外补 GL 真窗/直调一轮 `ui_wr_p34_filters` | ✅ 2026-11-03：go 后端 golden=0.0000% 且与 native 逐位一致（`evidence/review-fix/windows/window-runs.txt`） |
| L1 | 审查批·深审欠账 | P3.1 色彩管理地基逐文件深审未做（管线格式矩阵 ~50 处硬编码、sRGB 白名单、降级面未逐文件读） | 审查第二轮专项 | ✅ 2026-11-03 深审已做（`evidence/review-fix/deep-audit-l1l3/l1-p*.md`）；D4–D7 全修，p31 金图重采 |
| L2 | 审查批·深审欠账 | P3.2 自定义 shader 逐文件深审未做（翻译链 Validate、Brush 路由绕行、离屏 cache ensure、录制面欠账） | 审查第二轮专项 | ✅ 2026-11-03 深审已做（`l2-p*.md`）；D1–D2、D8–D10 全修，部分入 M1/存量 |
| L3 | 审查批·深审欠账 | P3.5 路径与几何五项逐文件深审未做（conic verb 边界、stroke panic 面、PathEffect 哈希/缓存键、透视命令分组） | 审查第二轮专项 | ✅ 2026-11-03 深审已做（`l3-p*.md`）；D3、D11–D12 修，A4–A6 入 M2/M3 |
| M1 | D9 标定 | `fillMaskedAsImage` 兄弟路把 premul 栅格喂进 straight 合成链，疑 GPU 半透明填充双乘 | 渲染专项：合成链色空间口径对账 | ✅ 2026-11-03 假问题收回：四链 GPU/CPU 逐位等值，原结论把 Pixmap premul 存储约定误套到 fill 源；长期钉 `render/m1_colorspace_audit_test.go`；`evidence/review-fix/deep-audit-l1l3/M1-retraction.md` |
| M2 | D12 对账 | `ui_wr_p35_rrect4` go 后端金图 0.0038%（29px 1-LSB 角点），与 D 批树/HEAD 差异集合逐点相同 | gles/native 角点覆盖率对账 | ✅ 2026-11-11：三点二分 `17574a71`=0.0038% → `4f8b4381`=0.0038% → `9798875b`=0.0000%，由 P4.1 批 4 GLES 链修复消除；金图未重种；`evidence/P2.6/m2-rrect4-gles-corner.md` |
| M3 | D10 疑点 | WGSL 前端接受非法 intrinsic 形态（`textureSampleLevel(samp,samp,…)` 三关全绿进缓存，绘制期才炸） | shader 前端专项 | ✅ 2026-11-11：降低关零验参/校验关只查句柄/绘制关 codegen 才炸；修法对齐 naga，`lowerTextureCall` 分派头挂 `checkTextureOperandTypes`（image/sampler 类型位，不可解析跳过）；5 钉先红后绿；`evidence/P2.6/m3-dissection.md` |
| M4 | D8 回照 | render 整包 SIGABRT（`conv.rs:519 invalid bind group layout`，`TestContextFrameDamage_MultipleFillsSeparateRects`）：HEAD 同红、D8 修后不消解 → 独立存量 | vello accelerator 管线家族专项 | ✅ 2026-11-09 带 lib 单跑该用例 `ok`，登记的整包 SIGABRT 不复现 |
| M5 | P3.7 D5 | 真 HDR float 全链在 render 层代码量为零（`FloatTarget` 仅 CPU、`GPURenderTarget` 无 Format、呈现固定 BGRA8、离屏 RT 全 8-bit） | 渲染专项（HDR 全链） | ✅ 2026-11-11 全批收口：RGBA16Float、格式参数化（设计面 22 文件，P1 实落 12 文件 43 处）、float RT、tonemap、真窗双后端 0.0000%；`evidence/M5/closeout.md` |
| M6 | P3.7 批 2 前置探针 | `SetEffectSurface(true)` 下 `ExportImageBuf` R/B 互换（同一绘制 `(255,0,0)` → `(0,0,255)`，两档都真走 GPU） | 色空间/通道口径单点化 | ✅ 2026-11-05：真病灶=`filter_ops_gpu.go:806-813` 无条件走不翻通道读回，而效果面发布 BGRA8；按纹理真实格式选读回 + 未知格式 fail-closed；两钉 + 负向 N15/N16；`evidence/P3.7/m6-export-byteorder.md` |
| N1 | P4.3 收口 | backdrop 损伤区裁剪的「静止帧 + retained/hybrid」增量二轮未做：二轮 `damage_area_px` 恒 960000 是测量窗每帧 `ScheduleFrame` 全量重绘 + `PresentPolicyFullPaint` 标签造成，非 backdrop 造成 | 渲染专项（可选）：改测量窗不每帧调度、切 retained/hybrid，看 `damage_area_px` 是否收敛到 153600；须给客观收益 + 双后端逐位 + 负向，否则维持不优化 | ⬜ 可选后续（2026-11-10 用户拍板 P4.3 收口时登记，不阻塞） |
| N2 | P2.6 22 窗漂移 | `ui_render_base_webp` 逻辑探针 `webp_lossless_format` 仍期望 `FormatRGBA8`，P3.8 `d5f9af60` 已把 `FromStdImage` 统一为 `FormatRGBAPremul`（`render/internal/image/io.go:313-355`）；像素探针 24/24、alpha 直方图 379/64/5701 全过，仅格式标签过期 | P2.6（随 21 窗金图重种批） | ✅ 2026-11-12 已改为期望 `FormatRGBAPremul`；重种后复跑 logic 19/19、pixel 24/24、golden 0.0000% |
| N3 | P2.6 I5 金图重种 | `ui_base_vram_cap` gate 首窗 `snapshot:` 偶发未写出：native 300s 第一轮 7/8 行（win1 缺），gate 读取陈旧 `vram_win1.png` 误报 17.5273%；第二轮 8/8 行、全 0.0000% | P2.6/P2.4e（查 `runOnGL` 关闭竞态，补「snapshot 未写出」显式失败） | ✅ 2026-11-12：根因=`runDone` 的 Wait→Add 竞态使主 goroutine 提前 `Close()`，首窗 target 置 nil、快照静默跳过；修=windowsOpened 屏障 + gate 前删旧 PNG + 缺文件/不可读显式 FAIL + embedder target/context unavailable 报错；5×60s、native/go 300s 均 8/8 `0.0000%`，只读负向 8/8 `snapshot missing` 红；`evidence/P2.6/golden-drift/n3-snapshot-race/assessment.md` |

## 3 每个任务的开工条件与完成标准

### P0 准备（不改代码）

- **P0.1 阈值与环境清单**
  - 开工条件：无。
  - 内容：首帧时间、滚动帧率、单帧显存上限、p95/p99、金图容差，全部写死数字并指明来自哪份真源文档 / 哪次实测。
  - 完成标准：`baseline/thresholds.md` 落盘；每个数字有来源（真源章节或实测窗名+日期）；不留「待定」。
  - 下游义务：阈值负向对照（改错一项阈值必须判绿、还原必须判红）在 P1.7 数据与 P1.8 破坏上成对执行，记入 `baseline/negative/threshold-mutation.txt`。
  - 已执行：环境 940MX/580.178.04/go1.25.5/Wayland:XWayland；首帧 <2000/<1000/<800ms；fps_interval≥55 + p95≤22ms；p99 只观测 + 8ms 回归带；显存 768MB 预算 / 0.9 水位 / 0.8 低显存自动档；金图默认零容差。
- **P0.2 真窗与金图清单**
  - 内容：窗口名 → 金图文件 → 平台/后端 → 容差 → 单窗或批量、运行方式（自动自检 / 人工常驻）→ **覆盖能力标签**（着色器 / 裁剪 / 混合 / 文本 / 图片 / 滤镜 / 2.5D / 显存与生命周期 等）。
  - 完成标准：清单覆盖 `examples/` 下全部真窗；每行能直接照着跑；缺金图的窗口显式标「无金图」及原因；PLAN §5/§6 登记的每项能力至少对应一扇窗，没有窗的写清替代验证方式。
  - 下游义务：P1.4 严格按 `baseline/windows.md` §5 的顺序与产物清单采集；8 扇运行期金图归档后回填清单并把容差改成 `0`；P1.6 按「显存/生命周期」标签挑窗；P2.6 按本清单全量重放。
  - 已执行：162 真窗 + 28 headless；逐窗金图/容差/运行档/能力标签；仓内金图 114、运行期 8、无金图 40（均写原因与替代判定）；自校验 rows=162、114/8/40、missing=0。
- **P0.3 归档与回滚演练**
  - 内容：建演练分支或临时 tag，实际回退一次再恢复，记录命令与结果。
  - 完成标准：`baseline/rollback.md` 有完整命令与输出；证明「退得回去、恢复得回来」。
  - 边界：演练 tag 只用于演练，不是基线 tag；正式基线 tag 在 P1.8 通过后打。
  - 安全：只动本地、只动自建分支；不碰远端、不 `reset --hard` 他人工作。
  - 已执行（2026-10-28）：R1–R8 全 PASS；R5 按「相对起点无新增失败」口径；回退/恢复树哈希与三文件 sha1 逐位一致；演练 tag/隔离 worktree/`/tmp/purego` 软链清干净。
- **P0.4 双后端一致性测试台**
  - 内容：同一场景分别跑 webgpu 与 gwgpu（gles/metal），对语义做逻辑断言（几何覆盖、颜色通道、绘制次数、资源计数），不做像素相等。
  - 完成标准：至少 3 个代表场景（纯色/渐变、裁剪+混合、文本/位图）双后端可跑；不支持的后端显式标原因；用例集可一条命令回归。
  - 开工条件：P0.1–P0.3 无硬依赖；**不改引擎行为**，只加测试代码与 testdata。
  - 测试台落点：断言器 + 场景运行器放 `render/internal/conformance/`（测试专用，不进公开 API）；用例数据放 `render/testdata/conformance/*.json` + 输入图/文本；入口测试放 `render/conformance_test.go`。
  - 后端切换：用子进程跑同一入口；`GPUI_BACKEND` 只切呈现层，不切 accelerator。实际用 `GPUI_CONFORMANCE_BACKEND=native|go` 选装箱设备（native 走 `render/gpu.SetDeviceProvider`，go 走 GL + `SetAcceleratorDeviceProvider`）。
  - 场景与断言（三个场景各一份用例 JSON，期望值写在数据里）：
    1. `solid_gradient`：纯色矩形 + 线性/径向渐变 → 采样点通道 ±2/255、区域覆盖率 ±5 个百分点、无裁剪。
    2. `clip_blend`：矩形/圆角裁剪 + 半透明图层 → 裁剪内外采样点、混合结果通道 ±2/255、边界重绘计数。
    3. `text_image`：文本字形 + testdata 位图 → 字形覆盖率 >0 且包围盒在预期内、图片采样颜色 ±2/255、纹理创建与释放配平。
  - 双后端格：`native` 必跑、`go` 必跑；`metal` / Win32 本机不可验（Linux），写「未验（无 macOS/Windows 真机）」；某后端建设备失败要写错误与原因，禁止静默 skip。
  - 负向对照：改一个期望值（或注入 1px 偏移）→ 对应用例必须红，输出留证据；断言器自带单测（同输入 PASS、改一项 FAIL）。
  - 证据：`baseline/conformance/summary.md`（含一条回归命令）+ `<scene>/<backend>/report.json` + `run.txt`。
  - 环境前提：`replace ../purego` 是相对路径，测试机需存在 sibling 模块，否则先补路径再判失败。
  - 下游义务：P2.6 用本测试台做语义层「零变化」复核；像素与金图层仍归 P1.4 真窗，二者不互相替代。
  - 已执行（2026-10-28）：3 场景 × 2 后端全 PASS；`gpu_ops` 3/3/2、回退 0；跨后端像素差 solid_gradient max 1/255、clip_blend max 2/255、text_image 逐字节一致；负向对照转红；K1（每轮 +2 条/+131072B）登记 P1.6/P2.4c。
  - 口径修正：① `GPUI_BACKEND` 只管呈现层；② GL 后端必须有不呈现的 X11 上下文（`memOpenX11`），headless 说法只对 native 成立；③ native 缺 `WGPU_NATIVE_PATH` 会静默回落 CPU，测试台显式记 unavailable 并强制装箱设备。

### P1 基线冻结（不改代码）

- **P1.1 六目标交叉编译**：`linux/amd64`、`linux/arm64`、`windows/amd64`、`darwin/amd64`、`darwin/arm64`、`js/wasm` 各跑一次 `go build ./...`；另跑 `-tags nogpu`。完成标准：每个目标结果逐条落盘；失败项登记为基线已知问题，不隐藏。
  - 已执行：修复前红项 B1–B6（nogpu 163 包加载失败、wayland_csd_font 缺 linux 标签、gles 未限平台、render 无条件注册 gles、rwgpu Windows 缺加载层、ffishim 用 purego.Dlopen）；用户决定插队 P1.1b。
- **P1.1b 跨平台构建修复（B1–B6）**：B1 用 `render/gpu/gpu_nogpu.go` 空实现，消费方不逐个改；B2 补 `//go:build linux`；B3 gles 统一 `(windows || linux) && !(js && wasm)`；B4 四处 gles 注册合并 `render/register_gles.go`；B5 rwgpu Windows 补 `call1..call6`/`resolvedFnPtr`；B6 ffishim 按平台拆 `LoadLibrary/GetProcAddress/FreeLibrary`。
  - 完成标准：五目标（linux×2、windows、darwin×2）默认 + nogpu 主体 + nogpu 库范围全绿（两个已登记问题包除外）；P0.4 一致性台 native/go 复跑逐字节一致；js/wasm 登记不支持。
  - 已执行：linux×2 351/347/141、windows 357/353/140、darwin×2 349/345/140 全 PASS；P0.4 6 份 report 与 summary 逐字节一致；js/wasm 因 `../purego` 无 wasm 实现、302 包传递依赖，登记不支持；`baseline/build/` 已刷新。
- **P1.2 `go vet ./...`**：默认与 `-tags nogpu` 各跑一次；`./...` 原始输出落盘，再按「去掉已知加载失败包 + 保留有 Go 文件或测试文件的包」过滤后逐包采集。完成标准：两种标签组合都能完成分析；输出与逐包状态落盘；既有告警逐条登记。
  - 已执行：默认 352 包 61 条（59 unsafe + 2 unreachable）；nogpu 348 包 58 条 unsafe；两模式类型错误 0；告警归属：ui/platform 56 条线外、P2.5 测试卫生 3 条、P2.5 死代码 2 条。
- **P1.2b nogpu 测试标签修复**：给 20 个测试文件补 `//go:build !nogpu`（9 个在 `render/internal/gpu`、11 个在 `render`）；不改生产代码、不改测试内容、不改默认标签行为。完成标准：nogpu vet 348 包完成且 0 类型错误；`go test -tags nogpu -run '^$'` 全部编译通过；默认 vet 除行号位移外不变。
  - 已执行：nogpu vet 348 包完成、58 条 unsafe、0 类型错误；nogpu 测试编译 137 ok / 211 no test files / 0 FAIL；默认 vet 61 条不变。
- **P1.3 逐测试文件**：每个 `*_test.go` 单独执行并记录 PASS/SKIP/FAIL；SKIP 必须带原因；禁止一次全量后只记总数。
  - 已执行：1398 文件 = 1264 PASS / 48 FAIL / 86 SKIP；0 缺失/0 重复/0 多余；48 条逐条分类并给 P2 归属；46 个 RequestDevice SKIP 首轮与干净重跑均复现；长跑 `zz_b2_bitmap_scan` 1256.2s 等 PASS。
  - 已知 T1：`gpu/rwgpu` `TestQueue_OnSubmittedWorkDone` 失败后 cleanup SIGSEGV（`PC=0x1`）；HEAD 隔离 worktree 同栈，与 B5 无关；归 D1/P2.4c/P2.4e。
- **P1.4 全部真窗**：按 P0.2 清单逐个跑；每窗三证据齐；金图 diff 给像素计数并对照容差；批量压测遵守 AGENTS 的 GPU 真窗安全条。
  - 已执行：162 窗 / 183 变体两后端；native 191 轮（PASS 111 / FAIL 60 / NO_JSON 8 / SEED 8 / BUILD_FAIL 1 / PROBE 1 / SKIP 2）+ go 183 轮（108 / 51 / 20 / 1 / 1 / 2）；374 行无 MISSING；8 张运行期金图播种后次遍 diff=0，基线归档、容差回填 0。
  - 口径：播种轮原始 FAIL 且 `golden_first_run=true` 汇总显示 `SEED`；确认轮 FAIL 就是 FAIL。
  - 已知 T2：r1/r20/r22/c6 四窗 rc=1 而 JSON 自报正常、金图 diff=0；病因归 C4/P2.6。
- **P1.5 API 与测试计数快照**：每包 `go doc -all` 落盘；测试函数计数统计落盘。完成标准：机器校验零缺失零多余，可重复生成。
  - 已执行：352 包 / 351 doc 文件（`ui/scene/internal/gpupixel` 仅测试文件例外）；静态测试 1397 文件 / 14388 函数（Test 13602 / Benchmark 753 / Example 18 / Fuzz 15）；`tools/verify_api.py` ALL PASS 幂等。
- **P1.6 显存账本**：开窗、关窗、切场景、压测后各取一次；关窗后应回落到基线值，差异记录。
  - 已执行：三 cap 窗 × 双后端 8 轮（隔离 :3 串行、标准关闭）；ladder native 峰值 372.0MB 未破 0.9 水线 691.2，拒增曲线 `[2,0]→[4,550]→[8,747]`，hold 斜率 1533KB/min；oom 双后端 12.06MB → 8.5459（delta 0）`ledger_recovered=true`；multiwin 双 PASS；batch 双 FAIL（p95 55.7/62.1，归 C6）；ladder go 档 `eglMakeCurrent 0x3002`（C7）；RSS 10 条 PASS 记录过 30000KB/min（C5）。
  - K1 复核：clip_blend 稳态 9 条不增长；solid_gradient/text_image 每轮 +2/+131072B 精确复现，native/go 逐位一致；`tools/verify_vram.py` 14 断言 ALL PASS。
- **P1.7 性能基线**：首帧、滚动帧率、p95/p99 按 P0.1 口径实测并落盘。
  - 已执行：366 轮；首帧 275 轮 0 FAIL（median 174ms / p90 248 / max 515）；A 族 44 轮 8 处超限集中 4 轮（c3/r7/r7b，与 P1.4 FAIL 零冲突）；p99 median 20.7 / max 32.0；`tools/verify_perf.py` 8 断言 ALL PASS。
- **P1.8 负向对照**：三种人为破坏（偏移 1px、跳过一次绘制、少释放一次）各跑一次，确认对应门禁会红；记录门禁输出；做完立即还原。通过后把 P1 证据一并提交，并打正式基线 tag。
  - 已执行：三破坏全红（coverage 0.92/0.87；跳绘红；K1 释放缺口上界 131072→0 红）；阈值变异对 22→4 红 / 30→0 漏报 / 还原→4 报警成立；`tools/verify_negative.py` 9 断言 ALL PASS；正式基线 tag `baseline-p1` 打在 `92ca9216`。

### P2 框架代码调整与整合（行为零变化）

- **P2.1 shader 合并**：naga 迁顶层 `gpu/shader`，删旧副本与 `spirv/hlsl/dxil/internal/backend/internal/dxcvalidator`；调用方只换 import。完成标准：GLSL/MSL 输出与快照逐字节一致；GL/Linux 真窗三证据重放一致；Metal 交叉编译过但标不可运行验证；API 快照差异仅白名单。
  - 已执行：naga 144 文件落 `gpu/shader`，删三后端 167 文件 + 5 个根 SPIR-V 符号 + `CompileShaderToSPIRV`；GLSL/MSL 快照 681 文件逐字节一致；API 41→11 包；测试 1226→1210 文件 / 10600→10353 函数；1 FAIL 为 G3 既有环境红。
- **P2.2 删 `render/render`，`FloatTarget` 迁出**：删 15 个旧文件及测试/testdata；`float_target.go` 迁 `render/hdr`，单测与 testdata 随迁。完成标准：全仓无残留引用；HDR 单测绿；API 快照差异仅白名单。
  - 已执行：`render/render` 86 PASS → `render/hdr` 3 PASS；五目标 351/347/141、357/353/140、349/345/140 逐项一致；API 仅 `render/render.txt` 消失 + `render/hdr.txt` 新增；测试 1397→1390 文件 / 14388→14302 函数。
- **P2.3 删老一代 GPU 渲染器**：删 `render/internal/gpu` 下 backend/renderer/pipeline/... 及测试；`doc.go` 重写；保留在用文件。完成标准：`webgpu.CreateInstance` 只剩白名单入口；相关测试按白名单减少且有账；真窗重放一致。
  - 已执行：10 源 6515 行 + 10 测试 5635 行 + 7 WGSL 2071 行；3 处存活面迁移；`render/internal/gpu` 761→630 符号（−131/+0）；测试 141/1008/109→131/852/79；`webgpu.CreateInstance` 白名单 4→3；无真窗（零生产消费者死代码删除，按 §4.1 例外，P2.6 基线重放复核）。
- **P2.4a 设备创建/选卡收敛**：三份 → 一份，创建入口唯一。已执行：`webgpu.CreateInstance` 白名单 3→1；新增 `render.OpenStandaloneDevice` 唯一独立入口；删 9 符号；321 文件 280 PASS / 28 FAIL（27 既有 + 1 竞态单跑 7/7）；`ui_l1_blank` native/go 双档 rc=0、multiwin native 8 窗三相位 PASS；multiwin go 第 8 窗撞 C7。
- **P2.4b 预算统一**：六口径 → 一套 `Budget`，按域分账；淘汰语义有单测。已执行：GPU 侧 10 域 + 全局口径 → `render.DefaultBudget` 唯一声明表（22 调用点），`FairSplit` 合一，`OpenStandaloneDevice` 改 `DeviceDescriptorForAdapter`；473 文件 423 PASS / 17 仅编译 / 1 标签排除 / 32 FAIL 全定性既有；`render/text` 零新增 import。
- **P2.4c 延迟释放统一**：三队列 → `res/registry`；释放时序有单测。已执行：`pendingTex/Buf`、`ImageCache.ephemeral+pending`、`res.Cache.pending` 全删，统一 `Register→Track→Release→Retire` + 5 个 `SubmitDone` 同步点；`Cache` 回池改 idle 回调；K1 已闭合，上界 0/0。
- **P2.4d 纹理分配收敛**：六处同尺寸管理器 → `res/cache`；复用命中率有探针。已执行：五域（`device-surface`/`layer-offscreen`/`filter-graph`/`dualtex-snap` + 测试缝 `session-surface`），textureSet 四 owner 合并、`TexturePool` 删除、`BudgetSurfacePool` 改名、新增 `BudgetFilterGraph`。
- **P2.4e 入口收敛**：裁剪栈、混合枚举、呈现入口各归一处；旧入口删除且调用方全改。每条子步附加：生命周期测试台（开窗/关窗/切场景 → 显存账本归零、堆无增长）+ 触及共享数据 `-race` + 真窗三证据；独立分支与提交点，出问题整步回退。
  - 已执行（提交 `705244f5`）：四子步 e1 G2 老抽象/J1J2、e2 B2 裁剪、e3 B3 混合枚举、e4 B4 呈现单计划化 + I2/C3/C7 拆半；三真产品 bug（整页饥饿/上下文所有权/Tab 吞键）修复；遗留 misc_draw wgpu 执行态 + SeatCapCB robustness 两专项立案。
- **P2.5 包组织收口**：按 PLAN §3 目标结构 `go list` 逐包对比归位；加分层约束测试（`ui` 不直引 `gpu`、`render` 无后端特有类型、调用方向单向）；同步索引与总账。完成标准：分层约束测试入 CI/固定 runner；差异为零或已登记理由。
  - 已执行：四子步（文档对账/分层守卫+hal 收敛/A2+E2 指标契约/总账+小项）；`tools/per-file-tests.pkgs` 34 包正本；vet 61/58→56/56；apidoc rc=0；附带修 ui/scene refuseChanged 损伤缺口。
- **P2.6 行为零变化总验收**：按 P1.1–P1.8 全量重放，逐项与基线对照。完成标准：单测与真窗结果与基线一致；金图逐位差 0（非确定性路径按 P0.1 容差）；API/测试计数差异逐条对上 PLAN §7.7 白名单；显存与性能基线不回退。**未绿不得进入 P3。**
  - 已执行：374 轮真窗全跑完；对账 364 去重轮、基线同红 100、嫌疑 0、转绿 20；逐测试文件 1165 PASS / 17 FAIL / 48 SKIP / 208 无测试（17 中 15 条基线同红、2 条 A/B 非 P2 回归）；vram 峰值 373.8MB、oom recovered、perf p95 fail 0；负向三破坏全红；总判定=P2 四阶段行为零变化成立，可进 P3。

### P3 缺失能力（先文档后实现）

- **P3.0**：先出依赖图与文档更新（PLAN §5 + `RENDER_API_CATALOG.md`），再按图排实现顺序；文档没写完不写代码。
- **P3.1–P3.9b**：每项先补文档（能力定义、公开 API、双后端实现方式、验证方案），再实现。P3.9/3.9a 由 P3.6 D2 从图片格式项拆出单列，P3.9b 为 1x/4x 测量补充。
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
| P2 每子步 | 全平台编译（linux×2/windows/darwin×2；wasm 不支持）+ `go vet` + 逐测试文件 + 残留 grep + API 快照 diff（仅白名单差异） |
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

**状态两态（2026-11-05 拍板，替换此前一个 ✅ 兼表两义的用法）**：

- **✅ = 像素收口**：该行交付的**新能力本身**的画面，被某个真窗的金图逐位比对或显式像素探针断言过（窗源码里要能找到「这一臂被画出来并被测」的位置）。
- **☑ 已交付待金图**：代码 + 能力单测 + 负向对照 + conformance/门禁齐，但**新能力的画面没有被任何窗断言过**（多因还没有生产消费者）。状态列写 `☑ 已交付待金图（收口批=N）`，不写 ✅。
- **「存量画面零变化（golden 0.0000% / changed=0 / 未重出金图）只算无回归证据，不构成新能力收口」**——挂新功能后旧画面不变，恰恰可能意味着新功能没在窗里被画出来。
- 延后集中出金图允许（例如 P3.7 批 6 五扇窗），但延后期间不得标 ✅，且 §2/§2.1 必须显式挂着收口批号；批 6 出完金图后逐行改回 ✅ 并附窗名 + 差异 + 提交号。
- **例外只有一种**：零生产消费者的死代码删除（例如 `FloatTarget` 这类），确实没有真窗可跑。此时必须同时给出三样替代证据：全仓无引用 grep、迁移前后单测输出一致、P2.6 基线重放无差异；并在 §7 流水写清理由编号。

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
  archive/
    ENGINE_RENDER_GPU_CONVERGE_TASKS_2026-11-11_full.md   # 收敛前全文快照
```

每个证据目录至少包含：执行命令、原始输出或摘要、结论（PASS/FAIL）、对照对象的路径。真窗证据含 JSON 探针输出、PNG、金图 diff 计数。

## 6 即时同步规则（硬）

> **本节只保证台账一致与可追溯，不保证任务本身正确。** 任务正确性只认 §1 的 ✅ 硬条件、§4.1 的五份证据与可复跑门禁；台账写 ✅ 不等于实现正确。

1. **状态改动当次落盘**：任何任务开工、完成、阻塞、暂停，都在当次会话内**先改 §2.1.1 主表对应行与 §2.1 当前载体**（原位置更新，禁止另起历史块），再在 §7 追加一条 ≤200 字摘要；禁止「攒到最后一起写」或复制证据正文。
2. **代码提交必须带台账**：凡是改代码的提交，同一提交内必须包含本文件的状态/证据路径更新；没更新台账的提交视为未完成。
3. **任务列表镜像**：新会话按 §2 逐项重建工具任务列表；工具条目数不足时可按阶段聚合，聚合条目名必须写明覆盖的编号（如 `P0.1–P0.4`），状态仍以 §2 逐项为准。发现不一致先按本文件修正镜像并记录差异。
4. **方案口径变更回写 PLAN**：任务顺序、门禁、白名单变化，先改 `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md` 对应章节，再改本文件；两份不允许互相矛盾。
5. **证据先于状态**：没有证据文件不得把状态改成 ✅；证据路径必须能在仓库里打开。
6. **删除必须对账**：任何删除动作在 §7 流水写一句摘要（对象 / 理由（PLAN 章节）/ API 与测试计数变化 / 白名单对应行），细节与清单进 `evidence/`。
7. **跨平台格写满**：涉及平台/后端的任务，完成时在同一行写清哪端做了、哪端没做及原因，不留空格。
8. **失败不掩盖**：门禁红了就记 FAIL 与原因，状态退回 🟡 或 ⛔，禁止把失败写成「基本通过」。
9. **验证载体未登记不得开工**：§2.1 的能力单测、真窗、金图容差、双后端四项没填，任务状态不许从 ⬜ 变 🟡。
10. **单测与真窗缺一不得标 ✅**：只有单测绿 = 不算；只有真窗看着对 = 不算；两样齐 + 三证据 + 负向对照红过才算正确。

### 6.1 唯一位置与更新决策树（硬，防重复追加）

**唯一位置表**

| 内容 | 只写在哪里 | 写法 | 禁止 |
|---|---|---|---|
| 任务状态 / 依赖 / 证据目录 | §2.1.1 主表 | 改对应行 | 另起「历史更新线」或「上一任务」整块 |
| 当前进行中细节 | §2.1 当前任务验证载体 | 只保留 1 个活动块 | 收口后改名「上一任务验证载体」继续保留 |
| 已收口专项摘要 | §2.1 专项索引 | 一项一行 | 新增整段载体块 |
| P2 待办池 | §2.2 表 | 一项一行，原行更新 | 表格下面另写闭段落 |
| 开工 / 完成标准 | §3 | 原条目更新 | 复制到 §7 |
| 门禁 / 模板 | §4 | 原条目更新 | 复制到 §7 |
| 时间线 | §7 | 一条事件一行，≤200 字 | 复制证据正文 / file:line 清单 |
| 口径修订 | §8 | 一条修订一行，≤200 字 | 追加长叙事 |
| 过程细节 / 复现 / 失败尝试 | `evidence/` | 证据文档 | 抄进台账 |

**更新决策树**（拿到新事实先问）：

1. 任务状态变了？→ 改 §2.1.1 主表对应行；§2.1 活动块只在任务进行中维护。
2. 待办池项有进展？→ 改 §2.2 对应行，不另起段落。
3. 新任务 / 新问题？→ 主表加一行或 §2.2 加一行，同时 §7 加一条短摘要。
4. 只是过程细节 / 数据 / 失败重试？→ 只进 `evidence/`，台账留一句结论 + 路径。
5. 口径 / 门禁变了？→ 改 §3/§4/§6 原条目，§8 加一条修订摘要。
6. 任务收口？→ 主表改 ✅；§2.1 当前载体替换为一行索引；§2.2 行改 ✅；§7 一条摘要；不复制证据正文。

**收口替换（硬）**

- 开工时：§2.1 只写一个「当前任务验证载体」，按 §4.2 模板填。
- 收口时：把该块替换为「已收口索引」一行（编号、一句话结论、证据路径），不得改名为「上一任务验证载体」后整块保留。
- 同一任务多次尝试（失败回退 / 二次实现）只更新当前块和证据文档，不新增并列块。

**长度预算（超出即搬 evidence）**

- §2.1 当前载体 ≤ 20 行；专项索引一行 ≤ 250 字。
- §2.2 单行 ≤ 500 字；§7 / §8 单行 ≤ 250 字。
- 主表单行 ≤ 500 字；整文件 ≤ 100KB / 600 行。
- 超过预算 = 写证据文档，不是继续加台账。

### 6.2 提交前自检（硬）

- `python3 scripts/check_tasks_ledger.py` 退出 0（同时检查当前载体数量、重复编号、表格、长度预算与证据路径）。
- `grep -c '^\*\*上一任务验证载体' docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` = 0。
- 主表 / §2.2 无重复编号；所有表格列数一致、分隔行合法。
- `git diff --stat` 若本文件新增超过 100 行，先问：这些内容是不是该进 `evidence/`？

## 7 状态流水（摘要，最新在下；全文见存档）

> 收敛说明：本节只保留日期、任务、状态变更、关键结论与证据指针；每条的过程性数字、file:line 与失败重试细节在 `archive/ENGINE_RENDER_GPU_CONVERGE_TASKS_2026-11-11_full.md` §7 与对应 `evidence/` 正本。

| 日期 | 任务 | 变更 / 结论 | 证据 |
|---|---|---|---|
| 2026-10-28 | 全表 | 建台账；P0/P1/P2.1–P2.6/P3.0–P3.8/P4.1–P4.4 共 32 项登记，全部 ⬜ | 本文件 |
| 2026-10-28 | P0.1–P0.4 | 阈值清单、真窗清单、回滚演练、双后端一致性台全部 ✅；P0.4 3 场景 × 2 后端 PASS、负向红、K1 登记 | `baseline/thresholds.md`、`windows.md`、`rollback.md`、`conformance/` |
| 2026-10-28 | P1.1/P1.1b | 六目标基线落盘，B1–B6 插队修复；五目标全绿，js/wasm 登记不支持 | `baseline/build/`、`evidence/P1.1b/` |
| 2026-10-28 | P1.2/P1.2b | vet 两模式完成分析；20 个测试文件补 `!nogpu`；61/58 条告警有账 | `baseline/vet.txt`、`evidence/P1.2b/` |
| 2026-10-29 | P1.3–P1.8 | 1398 文件逐文件、162 窗/183 变体、API/计数、显存、性能、负向全部落盘；正式基线 tag `baseline-p1` @ `92ca9216` | `baseline/tests/`、`windows/`、`api/`、`vram/`、`perf/`、`negative/` |
| 2026-10-29 | P2.2/P2.3/P2.1 | 删 `render/render`、删老 GPU 场景渲染器、naga 迁 `gpu/shader`；API/测试计数差异逐条对白名单 | `evidence/P2.1/`–`P2.3/` |
| 2026-10-30 | P2.4a–P2.4d | 设备创建/预算/延迟释放/纹理分配收敛；K1 闭合；C7/A3/A4 登记 | `evidence/P2.4a/`–`P2.4d/` |
| 2026-10-31/11-01 | P2.4e/P2.5/P2.6 | 入口收敛 + 包组织 + 行为零变化总验收；P2.6 374 轮、364 去重、基线同红 100、嫌疑 0、转绿 20；判定可进 P3 | `evidence/P2.4e/`、`P2.5/`、`P2.6/` |
| 2026-11-01 | P3.0–P3.4 | 依赖图/文档、色彩管理、自定义 shader、富文本、滤镜家族五批；P3.4 C 卡三根因闭合 | `evidence/P3.0/`–`P3.4/` |
| 2026-11-02 | P3.5 | conic/RRect/Matrix4/PathEffect/布尔保曲线五批 + 四扇独立真窗；窗口抓出并修四处引擎洞 | `evidence/P3.5/`（含 `windows-close.md`） |
| 2026-11-02 | P3.6 | 图片格式（GIF/APNG/动图 WebP/EXIF）与 GPU mipmap 批 1a/1b/1c/2 交付；`MipmapMode` 全量切换、图号=内容版本 | `evidence/P3.6/` |
| 2026-11-03 | P3.6/P3.7 | P3.6 全六批收口（含批 1d/批 3 真窗）；P3.7 开工、拍板 D1–D10；L1–L3 深审 + D1–D12 修复闭环；新增 M1–M4 | `evidence/P3.6/`、`P3.7/`、`review-fix/` |
| 2026-11-04 | P3.7 | 批 1a–1d 透视合批/染色/批绘四角权重/Canvas 4×4；批 2 z 序与高度；批 3e-1/3e-2 scene 投影口与 quad 通道 | `evidence/P3.7/` |
| 2026-11-05 | P3.7 | 批 3a–3e3 材质骨架/三入口/绑定预算/描边洞/scene 材质；批 4 mip 接线 + M6 修复；批 5 泛光；批 6 ① 透视窗收口 | `evidence/P3.7/` |
| 2026-11-06 | P3.7 | 批 6 ②–⑥ 高度 z 序/材质/mip/泛光/scene 五窗收口；批级门禁收官（64 包 903/12/24/1 只减不增、conformance 双后端、整包 `-race` DATA RACE=0）；H19–H23 陆续修复（H22/H23 于 11-07 收口） | `evidence/P3.7/batch6/` 与收官行 |
| 2026-11-07 | P3.7/P3.8 | 材质×透视/bicubic 收口；P3.8 开工、D1–D7 拍板；批 1–6 连续交付（svg_doc/svg_paint/svg_text/svg_filter/color_font/picture） | `evidence/P3.7/`、`P3.8/` |
| 2026-11-08 | P3.8 | 收官：conformance 基线 73.9%、零 blank 零崩溃；批 4b/4c/4d/4e/4f 补全滤镜、CSS、paint-server、文字与 SDF 烘焙根修 | `evidence/P3.8/` |
| 2026-11-09 | P3.8/P3.9/9a/9b | P3.8 4g 1:1 烘焙 + acc=2 维持拍板；P3.9 mesh patch、P3.9a 高层 mesh gradient、P3.9b 1x/4x 基准全收口；K6 修复批 1–5 | `evidence/P3.8/`–`P3.9b/`、`review-fix/` |
| 2026-11-10 | P3.4/P4.1/P4.2/P4.3/P4.4 | P3.4 16 节点矩阵 + 4×4 真窗；P4.1 四批全收（含 B 卡 #3、live 漏改、DrawShadow 录制命令）；P4.2 20 场景 ≤2；P4.3 阴影专项 + mask 池化，backdrop/dither 判不修；P4.4 COLRv1 变量轴收口；G14 marker 补齐 | `evidence/P3.4/`、`P4.1/`–`P4.4/` |
| 2026-11-11 | SDF/M5 | SDF 覆盖 LUT 实现批（P4.2 24→8、K2 传播、6 窗重种）；M5 HDR P0–P5 全批收口；M5 后主链 13 包 725 文件回归 | `evidence/SDF/coverage-line.md`、`evidence/M5/` |
| 2026-11-11 | §2.2 池 | B2/B3/C1/C5/C6/E1/E3/G3/I1 全部收口；D1/D3/D4/E2/F1/F2/J1–J5/K1–K7/L1–L3/M1–M6 源码实证销账；D2/C7 仍部分保留；I5 立项、I2 待取、N1 可选 | `evidence/P2.6/`、`P2.4e/`、`review-fix/` |
| 2026-11-12 | I5 | 换向：旧 worker/每窗设备设计作废（2c/D1c 反证）；当前树级联 8 窗 Fifo 冻结 vs Mailbox 8 窗全 present+正常退出；新设计 `i5-v2/design.md` | `evidence/P2.4e/i5-v2/` |
| 2026-11-12 | I5 | v2 实现：`SetPreferNonBlocking` + render 创建口 + `PresentModeName`；正轮 8 窗 mailbox/n=8 11.3fps/正常退出，负向 fifo/n=8 冻结，X11 回归 fifo rc=0；金图 17.5273% 预存漂移待 P2.6 | `evidence/P2.4e/i5-v2/summary.md` |
| 2026-11-12 | I5 | 用户拍板：I5 留 🟡，17.5273% 交 P2.6「22 窗金图漂移」逐窗判定，禁止直接重种；功能/真窗/负向证据已齐 | `evidence/P2.4e/i5-v2/golden/assessment.md` |
| 2026-11-12 | I2 | 复跑 PASS：同尺寸 Resize no-op + `MarkNeedsReconfigure` 显式口，Impeller KHR/AHB 对照一致；e4 已实现，台账滞后销账 | `evidence/P2.4e/i2/` |
| 2026-11-12 | C7 | 闭合 Wayland 第二机制：per-surface context 各自 `eglTerminate` 同一 native display → 关窗 SIGSEGV；修=EGLDisplay 按 native display 引用计数 + 借用 Wayland display 不 terminate；8 窗 300s 全开/全 present/正常退出 | `evidence/P2.4e/c7-wayland-verify/closeout.md` |
| 2026-11-12 | 22 窗金图漂移/I5 | 44 轮双后端采集 → 21/22 判基线过期（0.1880%–31.7164%，svg 0.0000% 对照）；用户确认后 21 窗重种，42 轮复跑 0.0000%、1px 负向 12.9901% 红→还原绿；I5 `ui_base_vram_cap` 8 窗重种，native/go 各 300s 8/8 0.0000%；webp 探针改 premul；首窗 snapshot 竞态登记 N3 | `evidence/P2.6/golden-drift/` |
| 2026-11-12 | N3 | 首窗 snapshot 未写出根因=`runDone` Wait→Add 竞态（主 goroutine 提前 Close 置 target nil）；修=windowsOpened 屏障 + gate 前删旧 PNG + 缺文件显式 FAIL + embedder 报错；5×60s 与 native/go 300s 全 8/8，只读负向 8/8 红 | `evidence/P2.6/golden-drift/n3-snapshot-race/assessment.md` |
| 2026-11-12 | P3.1b | 标准对齐复核后改 per-surface opt-in 线性目标（默认 sRGB 不动）；P3.1b-0 四项探针 PASS：native/GLES sRGB target、X11 sRGB swapchain、GLES sRGB↔Unorm blit | `evidence/P3.1b/` |
| 2026-11-12 | P3.1b | P3.1b-1 颜色转换地基：`colorSpaceWGSL` + Go sRGB↔linear 单一真源；GPU/CPU 256 点逐值对拍 PASS；`CreateOffscreenTextureFormat` sRGB 白名单 + 4B/px 账本/clear PASS；dual-tex 共享 snippet，既有测试全过 | `evidence/P3.1b/p31b-1-tests.txt` |

## 8 修订记录（摘要；全文见存档）

| 日期 | 修订 |
|---|---|
| 2026-10-28 | 首版：从 PLAN §7 拆出可执行任务清单，补状态图例、开工协议、开工/完成标准、门禁口径、证据约定、即时同步规则 |
| 2026-10-28 | 补新会话可续跑能力：§0 开场话术、§2.1 当前焦点、镜像允许按阶段聚合；同步规则改为同步 §2.1 与 PLAN |
| 2026-10-28 | 新增 §4.1 能力验证硬要求：每个任务必须带能力单测 + 真窗；开工前在 §2.1 登记四类载体，完成交五份证据；同步规则加第 9、10 条 |
| 2026-10-29 | 新增 §2.2 P2 待办池：审计新发现 + P1 散落「登记 P2」条目统一收口；P1.7/P1.8 两条流水归位回 §7 |
| 2026-11-01 | P3.0 依赖图定稿：§2 依赖列修订 5 条；实现顺序定稿 P3.1→P3.8 串行主序；catalog 新增 §13 规划登记节 |
| 2026-11-05 | 状态两态口径落地：✅=像素收口、☑=已交付待金图；「存量画面零变化」只算无回归；P3.7 批 1–3 十四行逐行重判为 ☑ |
| 2026-11-06 | 批 6 六扇真窗收口；五目标×{默认,nogpu} 全绿、64 包 sweep 只减不增、conformance 双后端、整包 `-race` DATA RACE=0 |
| 2026-11-10 | §2.2 台账整理（P0 文档清理）：11 行补回状态列、K6 重号改 K7、表格连续化；「无消费者」不作删除理由的判定口径写入证据 |
| 2026-11-12 | 收敛版（内容截至 2026-11-11）：主文件压缩为当前状态/口径/决策/证据指针；§2.1 历史载体块、§7 全量流水、§8 全量修订冻结到 `docs/render_gpu_converge/archive/ENGINE_RENDER_GPU_CONVERGE_TASKS_2026-11-11_full.md`，逐字保留不丢精度 |
| 2026-11-12 | I5 设计换向：从 present worker/设备隔离改为 native Wayland 非阻塞 present（Mailbox）根因修复；旧设计标废弃，v2 设计与诊断证据落盘 |
| 2026-11-12 | 金图重种口径：只对「基线过期」重种，须有双后端内容逐字节同 + 平移 (0,0) + 内部采样 + 对照窗（SDF LUT 后重种窗 0%）证据；重种后双后端复跑 0.0000% + 1px 负向；禁止无证据重种 |
| 2026-11-12 | N3 证据纪律：gate 开始前清旧快照，缺文件/不可读显式 FAIL；embedder 快照 target/context 不可用不得静默，必须留错误行 |
