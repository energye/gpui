# P2.5 包组织收口 — 影响面评估（2026-10-31）

> 状态：开工登记，**方向待用户拍板后才动代码**。调查方式：4 路只读调查（包树对比 / 分层测试与 runner / A2 指标标签 / 池项小项），承重事实本机复核（apidoc 实跑、render 后端引用 grep、gpu 反向 grep、scheduler 管道 grep）。
> 依据：PLAN §3（目标结构）+ §7.3 P2.5（go list 对比、分层约束测试、索引与总账同步）+ 台账 §2.2 池项 A2/A4/D2/D3/H1/J4。

## 0 一句话结论

包树与 PLAN §3 目标结构**已高度接近**（目标条目零缺失、无同名子包、无 render/render 残留），差异全是「目标树没列到的 EXTRA」——登记+修订 PLAN 即可达成「差异为零或已登记理由」；真正缺的是**守卫没有测试化**（分层约束只有 ui 一处 AST 扫描）与**总账失修**（apidoc 机检当前 rc=1）。池带入六项中两项已在 P2.4e 顺手修掉，A2 比台账预估小（target_hz 管道已存在）。

## 1 go list 对比 PLAN §3 目标结构

总况：gpu/ 35 包、render/ 33 包（`go list ./...`，另 examples 197 / ui 25 / engine 25 / tools 4 / video 3 / scripts 1；`examples/ui_wr_t_stress` 为已知加载失败）。

### 1.1 目标条目核验——全部 EXISTS，零缺失

- gpu：`hal`✅ `types`✅ `context`✅ `webgpu`✅ `gwgpu/{gles,metal}`✅ `shader/{ir,wgsl,glsl,msl}`✅。目标写的 `gwgpu/ffi/...` 实际名 `gpu/gwgpu/ffishim`（改名差）。
- render：门面根包✅ `scene`✅ `recording`（含 `backends/raster`）✅ `text`✅ `svg`✅ `hdr`✅ `internal/{gpu,image,color,blend,filter,raster,cache,parallel}` 8/8 ✅。
- 三条硬规则核验：无 `render/render` 残留；gpu/、render/ 下无任何 X/X 同名子包；`gpu/shader/tmp`（dxil 产物）不存在。

### 1.2 EXTRA 清单（实际有、目标树没列）

| 包 | 定性 | 处置推荐 |
|---|---|---|
| `gpu/rwgpu`（包名 `wgpu`） | wgpu-native 的 purego FFI 绑定层，`gpu/webgpu` 是它的门面（30 文件引）；web 浏览器路径走 `webgpu/internal/browser` 不经它 | 登记进 PLAN §3（webgpu 之下层）；见 §6 D2 |
| `gpu/gwgpu/ffishim` | purego C-ABI 垫片，gles+metal 共用 | PLAN §3 把 `ffi/...` 改写为 `ffishim` |
| `gpu/gwgpu/{indirect,mslmap}`、`gpu/hal/noop`、`gpu/webgpu/internal/{browser,thread}`、`gpu/shader/*/internal/*` | 后端/抽象的组成件与实现细节 | 登记即可 |
| `gpu/rwgpu/examples/*`（15 个 main）、`gpu/webgpu/examples/{browser-compute,browser-test}` | **后端目录里混了示例 main**（硬规则①轻微违） | 可选小步迁出（D1） |
| `render/internal/{clip,stroke,wide,conformance,testutil/imagediff,gpu/res,gpu/tilecompute}` | clip/stroke/wide=通用图形件；conformance=P0.4 测试台；imagediff=测试工具；res=P2.4c 产物；tilecompute=Vello CPU 移植（主路径组成） | PLAN §3 internal 清单补列 |
| `render/filters`、`render/integration/ggcanvas`、`render/examples/*`、`render/tests/stress` | CPU filter 注册、gg 桥、示例/压测 | 登记即可 |

### 1.3 硬规则②的真实分层债（唯一一处）

`render/internal/gpu/gpu_shared.go` 非测试直引 `gpu/rwgpu`（`WithForceNativeReleaseOnLost`，约 :478/:498）+ `gpu/webgpu`——绕过 hal。这是全仓 render 侧唯一的 rwgpu 直引（实测 grep）。另 `render/internal/gpu/vello_accelerator.go` 亦引 webgpu（P2.4a 已收敛其创建入口，剩引用待逐符号核）。

**render 非测试引后端的白名单全景（实测 7 文件）**：`render/backend.go`（创建入口唯一句）、`render/present_swapchain.go`、`render/present_swapchain_gl.go`、`render/register_gles.go`（创建入口族，AGENTS 明文豁免）、`render/gpu/gpu.go`（注册包本体）、`render/internal/gpu/gpu_shared.go`（rwgpu+webgpu，见 D2）、`render/internal/gpu/vello_accelerator.go`（webgpu）。

## 2 分层约束测试现状与落点

- 现状守卫只有一处：`ui/depcheck_test.go`（AST `go/parser` 扫描，禁 `gpui/gpu...`，`_test.go` 测试缝豁免已在 P2.4e 落地，实测 PASS）。该文件 :54 有一条无函数体的悬空注释（`TestUISubpackagesDoNotImportGPU`），顺手清。
- **render 侧无守卫**（AGENTS「render 不出现后端特有类型」目前只有 grep 口径，无测试）；**gpu 反向无守卫**（gpu 非 test 引 render/ui 实测 **0**，守卫可从零钉起）。video 域已有同型先例（`video/compliance_imports_test.go` AST 扫描）。
- 落点推荐（D3）：新增 `render/depcheck_test.go`（禁后端包 import，白名单=§1.3 七文件显式列出）+ `gpu/depcheck_test.go`（禁 import `gpui/render|gpui/ui`，`_test.go` 豁免）；负向对照=临时加违例 import 转红再撤。
- **runner 固定化**：逐文件门禁脚本目前只在各阶段 evidence 目录有副本（P2.4e 最新，9 包口径写死在 `PKGS=()`），无正本；P2.4e 踩过「输出路径硬编码覆盖上阶段证据」的坑。推荐提正本到 `tools/per-file-tests.sh`：路径参数化、包清单外部可配、默认清单 9 包+ui 包（J4）。仓库无 CI（仅 ffmpeg 构建一条 workflow），「入固定 runner」= 正本脚本 + 所属域真源写明合入前必跑命令。

## 3 索引与总账同步现状（已失修）

- `go run ./scripts/apidoc` **实测 rc=1**：缺 `method PresentFramePlan`（P2.4e e4 新增，漏同步总账）。
- 脏行：catalog §8 仍列 e3 已删的 `scene.BlendModeFromInternal`/`scene.PaintBlendModeToScene`；存活的边界方法 `scene.BlendMode.ToPaintBlendMode()`（`render/scene/blend_integration.go:24`）未列（机检抓不到：只查缺失不查过时，且不扫 scene 方法）。
- 机检脚本范围 bug：`RENDER_API_CATALOG.md` 头部声称覆盖 `render/gpu`、`render/hdr`，但 `scripts/apidoc/main.go:145-149` 实际不扫这两个包。
- **gpu/hal 完全无目录覆盖**（`CompletionToken`/`OnSubmittedWorkDone` 等 P2.4d 新公开 API 零登记）——目录无 gpu 节（D6 拍板：同目录加节 vs 另立 GPU 目录）。

## 4 池带入项盘点（A2/A4/D2/D3/H1/J4）

### A2 + E2：帧指标两代标签一次切（接口全量切换制）

方向注（先纠正台账表述）：文档里 **`frame_interval_*`/`avg_frame_interval_ms` 是旧代**（`ui/scheduler/metrics.go:56-61` 发出），**`interval_*`+`fps_wall`/`target_hz` 是现役**（wrgate/video 窗）。A2 = 旧代窗一次切到现役命名。

- 改动核心小：metrics.go 4 个 json 标签改名；`fps_wall` 可由已有 `PresentCount`+`firstAt/lastAt` 导出（metrics.go:224-238）；**`target_hz` 管道已存在**——`FrameScheduler.SeedDisplayRefreshHz`（scheduler.go:528-532），只差接进 FrameMetrics（需一个公开配置口，公开 API +1 走总账）。
- 波及面：13 处 `MetricsStore.JSON()` 调用点编译安全（无标签字面量）；wrgate/video 窗从 Go 字段复制进自有结构，**不受改名影响**。会红的钉：`ui/scheduler/metrics_p0_test.go:107`（标签字面量）、`ui/scheduler/baseline.go:25,104`（标签字符串）；**`verify_perf.py:44-49` 硬断言 alias 行==12 且 OLD_GEN 六窗集合——改名后必红，须同步更新**；`extract_perf.py:104-107` 旧名别名列按 perf summary 既定规划退役。文档面走修订表追加（baseline 只增不改口径）：thresholds.md:87,97、baseline/perf/summary.md、windows README ~30 处。
- E2 随做：`video_vr7_perf` 补 `IntervalP99Ms`（结构 :475-477、buildReport :566 丢字段、requiredKeys :602）+ 退役 `verify_perf.py:58-62` 的 vr7 例外。
- 真窗验证：3 扇旧代窗（`ui_l1_blank`、`kit/spin_min`、`ui_render_base_perfsoak`）native+go 各一轮——JSON 新字段齐 + extractor 全列提取 + A 族判门禁不变。

### A4：`render/gpu/gpu_nogpu.go` 空包补丁

实测消费方 193 文件（190 空白导入；台账「163」已过时）。唯一生产非空白消费 = `examples/engine/stage_chase`（`!nogpu`/`nogpu` 拆分完好），另 2 个带标签测试文件。**推荐保留**（B1 的引擎层正解，导入恒安全），台账计数修正 + 消解路径登记（将来顶层包拆分时自然消解），无代码改动。

### D3：vet 测试卫生 + 死代码

- 3 条测试伪造指针仍在（`res/view_test.go:12,13`、`view_raw_attack_test.go:13:55`），改写保持断言语义（真实变量地址或 `unsafe.Slice` 等正路）。
- 2 条 unreachable 移位到 `gpu_render_context.go:4363,4411`：两个 `tryFillMasked*Inline` 函数在 `return false`（暂弃 inline 走 fillMaskedAsImage 的注释意图保留）后留整段死块——删死块，注释意图与禁用行为不动（零行为）。
- 清理后 vet 基线 61/58→**56/56**（ui/platform 56 条线外不动），台账登记基线变更。

### H1：skip 文案补 OOM 说明

`render/internal/gpu/native_device_test.go:21/29/36` 三处 skipf 补 A3 结论（显存 OOM 被包装成 Validation Error）。同类站点十余处（p1_gl_offscreen_probe 等），推荐只动 H1 点名文件、其余登记不动。

### D2 收账

- nil ImageCache 组 **✅ 已修**（P2.4e 红修批次4，7 用例带 lib 实测 PASS）；depcheck 测试缝 **✅ 已修**（`9cca54e6`）。
- 剩余：**ui/scene「无 GPU 前置」组 4 条带 lib 实测仍红**（`caret_transparent_rerecord_test.go:115`、`textured_budget_test.go:33`、`textured_test.go:35,80`——用例假设无 GPU 回退，GPU 环境下期望失效）→ 随 J4 先修再进清单。

### J4：门禁清单补 ui 包

ui/ 25 包加入逐文件门禁清单（D4 拍板全量 or 子集）；进清单前先修潜伏红：ui/scene 4 条（上）+ `ui/focus/focus_test.go:189 TestFocus_KeyWithNoPrimary`（干净树同红已验，病因开工后解剖）。

## 5 实施顺序与门禁（推荐）

| 子步 | 内容 | 提交 |
|---|---|---|
| a 文档对账 | 本评估落盘 + go list 对比文档 + PLAN §3 修订（D1）+ 台账 | 纯文档 |
| b 分层守卫+runner | 新增 render/gpu 两个 depcheck 守卫（先红后绿）+ 白名单显式化 + `tools/per-file-tests.sh` 正本化 + ui 清单扩展 + 潜伏红 5 条先修 +（若拍板）gpu_shared rwgpu→hal 收敛 | 代码 |
| c A2/E2 指标契约 | 标签改名 + fps_wall/target_hz + extractor/verify 同步 + 文档修订行 + 3 窗×2 后端真窗验证 | 独立提交 |
| d 总账+小项收尾 | apidoc rc=0 + gpu/hal 节（D6）+ A4/D3/H1/D2 收账 + 负向对照 | 代码+文档 |

门禁（P2 每子步口径）：五目标构建 + vet（新基线 56/56 登记）+ 逐测试文件（扩容清单）+ 残留 grep + API 快照 diff（白名单=A2 新增配置口等）+ conformance 双后端 0/0 + 触共享数据处 `-race`。负向对照：①ui 非测试临时 import gpu→depcheck 红；②render 白名单外临时 import webgpu→新守卫红；③A2 改回旧标签→新钉红。金图：本步无新增金图（A2 是 JSON 契约非像素），替代判定=extractor 全列提取 + 真窗 JSON 探针；全窗重放归 P2.6。双后端：conformance native+go 每子步；A2 验证窗 native+go；metal darwin×2 交叉编译。

## 6 决策点（开工拍板）

- **D1 包树差异处置**：推荐「零物理搬移 + EXTRA 全登记 + PLAN §3 目标树修订（ffishim 改名、补列 internal extras 与 rwgpu 层）」；可选加做「rwgpu/webgpu 的 examples 迁出后端目录」（17 个 main，git mv 无 import 变化）。
- **D2 gpu_shared 直引 rwgpu**：推荐收敛到 hal（新增 hal 口 + webgpu 转发，小改，符合 AGENTS）；备选登记豁免留待。vello_accelerator 的 webgpu 引用随本子步逐符号核。
- **D3 分层守卫形态**：推荐 3 个 Go depcheck 测试（ui 已有 + render/gpu 新增）+ 白名单显式化 + runner 正本化。
- **D4 J4 清单范围**：推荐 ui 全 25 包进清单 + 潜伏红 5 条先修；备选只进核心子集。
- **D5 A2 执行窗口**：推荐本步内做、独立提交（改动面已探明、有真窗验证载体）；备选拆独立小步。
- **D6 gpu/hal 总账归属**：推荐 `RENDER_API_CATALOG.md` 加「gpu/hal 节」+ apidoc 扩扫；备选另立 GPU 目录文档。
- **D7 小项打包**：A4 保留登记 / D3 清理（vet 56/56）/ H1 文案 / D2 收账——无异议按推荐执行。
