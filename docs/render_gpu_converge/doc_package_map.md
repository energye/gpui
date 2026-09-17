# render / gpu 收敛：文档对照包关系（尺子表）

> 范围：只看 `docs/` 顶层 `*.md`，共 45 份（2026-09-17 清点），不含 `docs/antd/`、`docs/review/`、`docs/2.5D/` 等子目录。
> 核对方法：每份文档取头 10 行的标题加范围/地位声明原文，再对代码目录是否真实存在（`ls -d` 验过）。
> 代码规模实测：`render/*.go` 204 个、`render/internal/gpu/*.go` 213 个、`gpu/rwgpu/*.go` 85 个、`gpu/webgpu/*.go` 71 个、`render/text/*.go` 220 个、`render/scene/*.go` 40 个。
> 用法：改 `render/` 只认表 A-1，改 `gpu/` 只认表 A-2，两层都改再加表 A-3。表 B 的文档定案时不引用。

## A-1 render 层的尺子（13 份）

| 文档 | 文档原话（范围/地位） | 对应包（已验存在） | 拿它查啥 |
|---|---|---|---|
| `ENGINE_UI_RENDER_BASE.md` | 范围：渲染基座（画/排/合/滚/调度 + 帧与资源指标） | `render/` 全包 | 每个能力 A/B/C/D 状态，验收三项 |
| `ENGINE_FLUTTER_SKIA_ARCH.md` | UI 引擎架构真源 — Flutter 管线 × Skia 光栅（`render`） | `render/` 全包 | 分层与依赖方向 |
| `RENDER_API_CATALOG.md` | `render/`（含 `render/text`、`render/scene`、`render/recording`、`render/surface`、`render/svg`、`render/filters`、`render/raster`、`render/gpu`）全部公开 API 总账 | `render/`、`render/text`、`render/scene`、`render/recording`、`render/surface`、`render/svg`、`render/filters`、`render/raster`、`render/gpu` | 哪个接口真在用（✅/🔗）、哪个摆设（🔌）、哪个半成品（⚠️） |
| `RENDER_2D_ALIGNMENT_PLAN.md` | 按 Skia/Flutter 工业级渲染语义，修复三个已知 2D 能力洞 | `render/` 主包 2D 能力 | 三个洞补完没 |
| `ENGINE_UI_WIDGET_RENDER.md` | 自定义控件渲染基座 + 排期 + 真窗验收唯一真源 | `render/` 给控件那一面 | 控件口子齐没齐 |
| `ENGINE_FRAME_PRESENT_STANDARD.md` | 帧节奏与呈现模式真源 | `render/present_target.go`、调度相关 | 帧模式、空闲帧、呈现切换 |
| `ENGINE_TEXT_FREETYPE_PLAN.md` | SELF-RASTERIZER HARDENING（已转向自研优化） | `render/text/` | 光栅加固做完没 |
| `ENGINE_TEXT_HINT_LIGHT_PLAN.md` | 自研移植 FreeType light 渲染模式 | `render/text/` | light 模式 M0–M3 |
| `ENGINE_TEXT_SHAPING_PLAN.md` | HbShaper 接入（shaping 层替换为 typesetting/harfbuzz） | `render/text/` 整形层 | 整形接入状态 |
| `ENGINE_TEXT_RASTER_FT_ALIGN_PLAN.md` | 文本渲染 FT 全对齐计划（Raster-FT-ALIGN） | `render/text/` 光栅层 | 与 FT 逐字节一致 |
| `ENGINE_TEXT_OPTIMIZE_PLAN.md` | `render/text` 正向优化清单 | `render/text/` | 优化项逐项验证 |
| `ENGINE_TEXT_SCALE_PLAN.md` | 任意规模文本编辑排版架构（O(1) 击键 · 分期施工） | `render/text/` 排版层 | 规模架构分期 |
| `ENGINE_TEXT_SESSION_BRIEF.md` | 新会话开工简报，每次开工前先读，适用 SCALE_PLAN 每一期 | `render/text/`（方法） | 开工步骤 |

## A-2 gpu 层的尺子（3 份）

| 文档 | 文档原话（范围/地位） | 对应包（已验存在） | 拿它查啥 |
|---|---|---|---|
| `ENGINE_GPU_RESOURCE_LIFECYCLE.md` | 范围：`render/internal/gpu/`（引擎层资源）· `gpu/rwgpu/`（FFI 防御）· `gpu/webgpu/`（facade 校验） | `render/internal/gpu/`、`gpu/rwgpu/`、`gpu/webgpu/` | 资源建用放、悬垂早放、池复用、设备丢失恢复 |
| `ENGINE_GPU_MULTIWINDOW_PLAN.md` | 地位：设计与施工总纲；目标：同机任意数量程序各窗正常渲染 | `render/present_target.go`、`render/adapter_policy.go`、`gpu/webgpu/swapchain.go` | 选卡、降级链、黑屏根除 |
| `ENGINE_THREAD_T_PLAN.md` | 三线重设计立项 T（唯一工作文档） | `gpu/webgpu/internal/thread`、渲染循环 | 线程划分与并发 |

## A-3 两层共用（7 份，含半份）

| 文档 | 文档原话（范围/地位） | 对应包 | 拿它查啥 |
|---|---|---|---|
| `ENGINE_CODING_RULES.md` | 性质：仓库级纪律 — 所有 `ui` / `render` / `gpu` / `examples` 必须遵守 | `ui/`、`render/`、`gpu/`、`examples/` | 禁 CGO、依赖单向 |
| `ENGINE_ARCH_OVERVIEW.md` | UI 架构总览 | 全仓 | 迷路回看方向 |
| `ENGINE_WINDOW_API.md` | 地位：`ui/platform` 窗口接口唯一设计真源 | `ui/platform`（`render` 建 surface 时对照） | 句柄与后端配对 |
| `ENGINE_WAYLAND_WINDOW_STANDARD.md` | 地位：`ui/platform` Wayland 窗口标准实现唯一设计真源 | `ui/platform`（对照） | Wayland 窗标准 |
| `UI_PIXEL_ASSERTION_STANDARD.md` | 像素级断言规范（真窗视觉正确性 · 硬）；已并入 WIDGET_RENDER §2.7，本文件为独立完整版 | 全仓真窗 | 画面对不对，三证据 |
| `RETAINED_DEFAULT_AUDIT.md` | 默认省帧模式全框架审计记录 | `render/present_target.go` 空闲帧 | 省帧模式问题地图 |
| `ENGINE_L1_CLOSEOUT.md`（半份） | 范围：L1（`ui/` + `render.PresentTarget`） | 仅 `render.PresentTarget` 部分 | present 目标收口；`ui/` 部分不算本表尺子 |

## B 非尺子（22 份，只说明归属，不作定案依据）

| 文档 | 归属 | 原因（一句话） |
|---|---|---|
| `ENGINE_PHASE_P0_P3.md` | `ui/` L1 | 范围原话：仅 L1 UI 引擎（包 `ui/`）+ 必要的 `render` 门面 |
| `ENGINE_PHASE_P4.md` | `ui/` L1 加深 | L1 加深任务（滚动·文本·图像 IO·裁剪），计划卡 |
| `ENGINE_PHASE_P4_P7_OUTLINE.md` | `ui/` 大纲 | 性质原话：大纲（非细任务卡） |
| `ENGINE_PHASE_P5.md` | `ui/` L2 | L2 框架壳任务计划 |
| `ENGINE_TEXT_WAYLAND_IME_REQUIREMENT.md` | `ui/` 输入法 | 文本编辑 + IME 需求，复盘范围在 `ui/textinput` 等 |
| `ENGINE_TEXT_X11_IME_REQUIREMENT.md` | `ui/` 输入法 | Wayland 版的 X11 完整镜像 |
| `ENGINE_VIDEO_DECODE_PLAN.md` | `video/` | 范围原话：只做 `MP4 + H.264` 独立根模块 |
| `RENDER_2_5D_GAP_PLAN.md` | `game/` | 范围原话：生产级 2.5D 游戏引擎；`render/` 只是它用的底层 |
| `RENDER_2_5D_PARALLEL_GROUPS.md` | `game/` | 来源原话：源自 GAP_PLAN 的并行分组 |
| `ENGINE_TEXT_SCALE_CHANGELOG.md` | `render/text/` 过程存档 | 变更过程存档，非需求口径 |
| `TEXT_EDIT_PROBLEMS.md` | 输入框线问题单 | 用途原话：只定义要解决什么，不记改法 |
| `REWRITE_ROUND_NOTES.md` | 输入框线存档 | 状态原话：本轮改动未能解决问题，已回滚 |
| `ENGINE_UI_WR_SKILLS_USAGE.md` | 方法手册 | 范围原话：6 个 skill 的使用方法，非能力需求 |
| `ENGINE_WAYLAND_NESTED_TEST.md` | 测试方法 | X11 下测 Wayland 真窗的操作法 |
| `LEGACY_00_M0_M5_BATCH.md` | 存档 | M0–M5 遗留收尾批次总览 |
| `LEGACY_01_W6_RETAINED_PLAN.md` | 存档 | W6 整波收口计划 |
| `LEGACY_02_SEGMENTTEXT_PLAN.md` | 存档 | 标题原话：已落地，本文留档 |
| `LEGACY_03_GOLDEN_BASELINE_PLAN.md` | 存档 | 标题原话：已归档 |
| `LEGACY_04_R_WINDOWS_PLAN.md` | 存档 | R1/R15 关窗计划 |
| `LEGACY_05_C10_SOAK_PLAN.md` | 存档 | C10 长跑收尾 |
| `LEGACY_06_M6_TREND_PLAN.md` | 存档 | M6 趋势例行记录 |
| `README.md` | 目录 | `docs/` 导读 |

## C 反查：包对照文档（收敛施工用）

| 包（已验存在） | 只认的尺子 |
|---|---|
| `render/` 主包（204 个 go 文件） | UI_RENDER_BASE、FLUTTER_SKIA_ARCH、API_CATALOG、2D_ALIGNMENT、WIDGET_RENDER、FRAME_PRESENT、CODING_RULES、PIXEL_ASSERTION |
| `render/text/`（220 个 go 文件） | TEXT 系列 7 份 + UI_RENDER_BASE + API_CATALOG |
| `render/scene/`、`render/recording/`、`render/surface/`、`render/svg/`、`render/filters/`、`render/raster/`、`render/gpu/` | API_CATALOG（接线状态）+ UI_RENDER_BASE（能力行） |
| `render/internal/gpu/`（213 个 go 文件） | GPU_RESOURCE_LIFECYCLE、GPU_MULTIWINDOW、FRAME_PRESENT、THREAD_T（循环部分） |
| `render/internal/blend|cache|clip|color|filter|image|parallel|raster|stroke|wide|testutil/` | UI_RENDER_BASE 对应能力节 |
| `gpu/rwgpu/`（85 个 go 文件） | GPU_RESOURCE_LIFECYCLE（FFI 防御节）、CODING_RULES（禁 CGO） |
| `gpu/webgpu/`（71 个 go 文件） | GPU_RESOURCE_LIFECYCLE（facade 节）、GPU_MULTIWINDOW（交换链节）、THREAD_T |
| `gpu/types/`、`gpu/context/`、`gpu/common/`、`gpu/shader/` | FLUTTER_SKIA_ARCH（类型层）+ RESOURCE_LIFECYCLE 引用处 |
| `render/present_target.go`、`render/adapter_policy.go` | GPU_MULTIWINDOW、FRAME_PRESENT、L1_CLOSEOUT（present 部分）、RETAINED_DEFAULT_AUDIT |

## D 遗漏检查

- `docs/*.md` 清点 45 份：表 A 共 23 份（含 L1 半份），表 B 共 22 份，合计 45，无遗漏。
- 子目录（`antd/`、`review/`、`2.5D/` 等）不在本文范围。
- 本表只写“文档对包”，需求对代码的逐行大表另起表，以本表为尺子清单。

## E 无尺子代码的处理（rwgpu 等使用驱动型代码）

> 实测结论（2026-09-17，`grep -l rwgpu docs/*.md`）：`gpu/rwgpu/` 没有独立需求文档，只在三份里被点名 —
> `ENGINE_GPU_RESOURCE_LIFECYCLE.md`（范围原话点名 FFI 防御）、`ENGINE_GPU_MULTIWINDOW_PLAN.md`（后端进门两件与 EGL 坑）、
> `ENGINE_CODING_RULES.md`（禁 CGO、purego 路径一致）。它的多数接口是上层要用才加的（`render/internal/gpu` 要啥，它就包啥），
> 所以不能用“文档对代码”正向查，只能反向查。

- **E1 建反向表**：`gpu/rwgpu` 每个导出符号 → 谁在调（`render/internal/gpu` 哪文件、哪函数）→ 对应表 A 哪份尺子 → 单测在哪 → 真窗证据在哪。
  无调用者的进删除候选，有调用者的看调用得对不对。
- **E2 定留删**：调用链通、测试窗都在的留；两处干同样事的并成一条（留被调得多、错少的那个）；无人调的删，删前先查 `examples/` 与测试是否真没人用。
- **E3 补尺子**：留下的接口补进 `RENDER_API_CATALOG.md` 对应分类（按那份文件头顶的维护义务），`gpu/rwgpu` 新增 FFI 绑定同步在 `doc.go` 或包头注明上层调用方，不另起新需求文档。
- 同样规则适用于 `render/` 里有功能无尺子的代码：先反向表，再定留删，再补目录，不凭名字删。
