# P3.5 批 4 — PathEffect（D5 拍板形态）交付证据

- 日期：2026-11-02　分支：`feat/p2-收敛整合`　基线 HEAD：`c892ba29`（批 3）
- 真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2.1 载体块「批 4」条 + `evidence/P3.5/assessment.md` §表（PathEffect 行）、§批次（批 4）、§拍板 D5、§门禁负向对照条
- 口径：**原子验证**（单测 + 双后端像素对拍 + 录制回放钉），真窗 `examples/ui_wr_p35_pathgeom` 未建；生产接线未做 → 不标生产

## 1. 实现面（文件级）

| 文件 | 改动 | 内容 |
|---|---|---|
| `render/path_effect.go` | 新增 | `PathEffect` 接口（`FilterPath(*Path)(*Path,bool)`）+ 包内 `pathEffectInternal`（`effectHash`/`scaledEffect`）；`PathEffectHash`（**0=nil 或无键**）/`ScalePathEffect` 公开口；构造器 `NewDashPathEffect`/`NewCornerPathEffect`/`NewDiscretePathEffect`/`NewPath1DPathEffect`（`Path1DStyle` 三形态）/`NewPath2DPathEffect`；组合器 `SumPathEffects`/`ComposePathEffects`；**盖章密度护栏 `maxReasonableStamps=100000`**（1D 每轮廓、2D 网格总量，耗尽整条拒绝，对齐 Skia `MAX_REASONABLE_ITERATIONS`）；FNV-1a 哈希与 `hashPathForEffect` |
| `render/stroke.go` | 改 | `Stroke.PathEffect` 字段 + `WithPathEffect`/`HasPathEffect`；`Clone` 按引用携带效果（依赖确定性契约，注释写明） |
| `render/paint.go` | 改 | `EffectivePathEffect()`/`HasPathEffect()` 生效口 |
| `render/stroke_user_expand.go` | 改 | CPU 用户空间腿：效果先于 `Dash` 作用于 `userPath`，**不折算**（SkDashPathEffect 口径），else-if 链保持 `Dash` 原行为 |
| `render/software_stroke.go` | 改 | CPU 设备空间腿：效果先于 `Dash`，按 `TransformScale` 折算（`ScalePathEffect`，与 dash B2 口径一致） |
| `render/internal/gpu/path_geometry_cache.go` | 改 | `GetOrApplyEffect(path, effect, transformScale)`：键 =（路径哈希、效果哈希、scale 位）共用同一 LRU；拒绝存 nil 不条目；**效果哈希 0（值形态自定义效果=无键）时直接应用、不入库** |
| `render/internal/gpu/gpu_render_context.go` | 改 | `StrokePath`：效果分支先于 dash（`dashHash` 携 `render.PathEffectHash` 进 stroke 几何键；缓存 miss 走 `ScalePathEffect().FilterPath`）；`strokeSolidFastPathEligible` 加 `!paint.HasPathEffect()`；`StrokeShape` SDF 口对 effected 绕行 |
| `render/internal/gpu/vello_accelerator.go` | 改 | `StrokePath`：`IsDashed() || HasPathEffect()` → `ErrFallbackToCPU` |
| `render/recording/{command,recorder,recorder_state,recorder_style,recorder_path_draw,backend_canvas}.go` | 改 | `recording.Stroke` +`PathEffect` 字段；Recorder `pathEffect` 状态入 save/restore、`SetStroke` 取值、三处 Stroke 装配透传；`BackendCanvas.ApplyStroke` 带效果时单口 `dc.SetStroke(render.Stroke{...PathEffect})`；**命令结构零新增** |
| 测试 | 新增 4 文件 | `render/path_effect_test.go`（14 钉）、`render/path_effect_gpu_test.go`（双后端像素对拍）、`render/internal/gpu/path_effect_cache_test.go`（缓存契约）、`render/recording/path_effect_test.go`（录制透传+回放逐字节） |

## 2. 对齐标准（Skia 真值来源，非自创）

- `SkPathEffect::filterPath` 契约 + `SkDashPathEffect`/`SkCornerPathEffect`/`SkDiscretePathEffect` 委托本机既有引擎（`ApplyDash`=`dashPath`、`Path.WithCorners`、`Path.Discrete`），**不复制第二套 dasher**。
- `SkPath1DPathEffect`（`chrome/m135` 源码 `src/effects/Sk1DPathEffect.cpp` 下载核对后逐点转写）：
  - Translate：原型原点锚在采样点；Rotate：`getMatrix` 随切线旋转；
  - **Morph 逐点**：原型点 (sx,sy) → 沿弧长 `d+sx` 取位置/切线，再沿法向偏移 sy（x 走弧长、y 走法向），单个 verb 取不到切线就整段丢弃；
  - phase 清理：负值取反 → 超 advance 取模 → `advance − phase`（PostScript 口径）→ ≥advance 归 0；
  - 行走界：`while (distance < length)`，**恰在轮廓终点不盖章**；闭包轮廓度量已含收口边，不重复接缝；
  - 构造拒绝：advance ≤0 / 非有限 / 空原型 → nil（负 phase 不拒，折算）。
- `SkPath2DPathEffect`：平铺 dx×dy 网格覆盖源包围盒（仅平移）。
- `SkSumPathEffect`：两效果各施同一源路径后并集（不折叠）；`SkComposePathEffect`：`outer(inner(src))`，首参后施。
- 空间口径与 `Stroke.Dash` 两腿一致：设备空间腿按 CTM 折算、用户空间腿不折算（经内部 `scaledEffect(scale)` 契约统一）。

## 3. 门禁结果（日志同目录）

| 门禁 | 结果 | 日志 |
|---|---|---|
| 定向批 4 全量 19 条（render 16 + internal/gpu 2 + recording 1） | 全绿 `-count=1`（含两处自审修复后复跑） | `targeted-batch4/targeted.txt`（复跑记录见 §6 与 `negative-control-batch4/12-*`） |
| 三包整包（render/internal-gpu/recording） | internal/gpu 21.95s ok、recording ok；**render 整包 native abort（既有，见 §5 归因）** | `pkg-batch4/packages.txt`、`pkg-batch4/ab-head-worktree.txt` |
| `CGO_ENABLED=1 -race -count=2`（三组） | 三组 ok | `race-batch4/race.txt` |
| conformance 双后端（最终树复跑） | PASS | `conformance-batch4/conformance.txt` |
| 五目标构建（per-package + nogpu） | 5/5 目标各 2/2 绿（raw `./...` rc=1 为存量坏包白名单，口径同批 1–3）；两处自审修复后，批 4 触及三包再按 5 目标 × {默认, nogpu} 交叉构建 10/10 rc=0 | `build-batch4/*.txt`、`vet-batch4/vet-apidoc-build-final.txt` |
| `go vet`（批 4 触及三包，最终树复跑） | rc=0；`./render/...` 全扫仅命中 HEAD 存量坏包 `render/recording/backends/raster`（P3.1 线债，工作区未改动） | `vet-batch4/vet.txt` |
| apidoc 机器校验（最终树独立 rc） | **rc=0**（`render` 主包 + scene/recording/svg/gpu/hdr 全符号覆盖）；计数复核 144/184/219/18/172/26 不变（护栏常量为包内非导出） | `apidoc-batch4.txt` |
| 负向对照 7 路（5 摘除 + 2 自审：补钉先红后绿、护栏钳小） | 全部红→还原绿 | `negative-control-batch4/README.md` + 01–15 |
| 主逐文件门禁（最终树） | 693 PASS/21 SKIP/10 FAIL/1 NO_TESTS，两枚漂移红经 HEAD A/B 定性=零新增回归（§4） | `per-file-tests-batch4/`、`ab-batch4/` |
| 补充分包逐文件 | 72 PASS/2 SKIP/6 FAIL（6 枚全为既有 raster 坏包） | `per-file-tests-batch4-extra/` |

## 4. 逐文件回归与基线对账（批 3 基线：主 692 PASS/21 SKIP_TAGGED/8 FAIL/1 NO_TESTS，补充 71/2/6）

主门禁（`per-file-tests-batch4/`，最终树含两处自审修复后重跑）：**693 PASS / 21 SKIP_TAGGED / 10 FAIL / 1 NO_TESTS**。

- 10 枚红里 **8 枚与批 3 基线名单逐名一致**：`render/context_misc_draw_gpu`、`render/filter_present_coherence`、
  `render/rasterizer_tile_value`、`render/s6_9_heavy_budget`、`render/text_transform_gpu`、
  `render/video_direct_upload`、`ui/application/app_test`、`ui/scene/internal/gpupixel/blit_rrect_corner`。
- 另 2 枚 `ui/rendering/m1_noquadratic`、`ui/rendering/m5_keystroke_g1` 为**墙钟比率门漂移**，已按
  同机同命令 HEAD A/B 定性：clean `c892ba29` worktree 同命令隔离复跑 5/5 红（×1.5–×3.1，门禁 ≤1.5），
  批 4 工作区同命令 5/5 红同量级（×1.8–×2.5）；`TestKeystrokeRatio_M1` 单跑两树各 3 次全绿。
  该门测的是文本排版耗时比，不在批 4 改动面上（本批对 `ui/rendering` 只增两个 nil 判定）→ **零新增回归**。
  证据 `ab-batch4/{README.md,head-m1m5.txt,worktree-m1m5.txt}`；口径同批 3 对 s6_9 的漂移定性。
- 行数与状态逐项对账（两份 summary.tsv 差分）：722→725 行 = 本批 3 个新测试文件
  `render/path_effect_test.go`、`render/path_effect_gpu_test.go`、`render/internal/gpu/path_effect_cache_test.go`
  全 PASS；消失行 0；状态变化只有 `ui/rendering/m1_noquadratic`、`ui/rendering/m5_keystroke_g1` 两行 PASS→FAIL
  （即上面定性的漂移门）。算术闭合：692 + 3（新绿）− 2（漂移红）= **693 PASS**，21 SKIP_TAGGED/1 NO_TESTS 不变。
- 金图：`render/internal/gpu/golden_test.go`、`golden_software_test.go` 均 PASS → **存量金图零变化**。
- 补充分包（`per-file-tests-batch4-extra/`）：**72 PASS / 2 SKIP_TAGGED / 6 FAIL**，
  6 枚 FAIL 与批 3 名单逐名一致，全为 HEAD 既有坏包 `render/recording/backends/raster`（P3.1 参数漏改债，本批未动）。

纪律核对：批 3 的 8 枚基线红**一枚未转绿也未转好**、无一名新增（除两枚经 A/B 定性的漂移门）→ 符合「只减不增」。

## 5. 顺带发现（非批 4 引入，已归位登记、本批未修）

`go test ./render` **单进程整包**跑（非本仓门禁口径——AGENTS 要求按测试文件分进程逐个跑）在
`TestContextFrameDamage_MultipleFillsSeparateRects` 的 `Context.Close` → `flushGPUAccelerator` →
`GPURenderContext.Flush` → `GPUShared.ensurePipelines` → `SDFRenderPipeline.createPipeline` →
`wgpuDeviceCreatePipelineLayout` 处**确定性 native abort**：
`thread panicked at src/conv.rs:519: invalid bind group layout for pipeline layout descriptor`。

- 复现：工作区两次整包跑同一测试同一处红（`pkg-batch4/packages.txt`、`pkg-batch4/render-rerun-triage.txt`）。
- 归因 A/B：clean HEAD `c892ba29` worktree（`/tmp/p35b4_ab`，配 `/tmp/purego` 软链解 `replace ../purego`）
  同命令**同一处红** → 批 4 之前既有，不是本批引入（`pkg-batch4/ab-head-worktree.txt`）。
- 边界：两两测试配对（GPU parity+该钉、conformance+该钉）均不复现 → 需更长执行前缀才触发，
  疑为多设备/多上下文生命周期下缓存的 bind group layout 跨设备复用；未在本批继续深挖。
- 归位：GPU 资源生命周期线（`docs/ENGINE_GPU_RESOURCE_LIFECYCLE.md`），台账 §2.1 载体块「顺带发现」条已登记，
  待该线立案后按对应 skill 流程走；本批未动该路径代码。

## 6. 覆盖洞的补钉过程（如实记录）

第一轮按 assessment 口径摘 `render/software_stroke.go` 的 effect 挂载分支时，批 4 原有两条 CPU 挂载钉
**全绿**——它们走的是 `Context.Stroke()` → `stroke_user_expand.go` 链，说明 `SoftwareRenderer.Stroke`
这条设备空间腿当时没有任何钉覆盖（若本批就此收口，即为「挂了但没人验证」的假绿风险）。
补 `TestStrokePathEffectSoftwareRendererMount`（经 `SoftwareRenderer.Stroke` 直绘：`DashPathEffect` 与
`Stroke.Dash` 字段逐字节同图 + 与纯描边必须不同 + 1D stamp 必须改画）后，同一摘除才拿到红。
另 `TestComposeOrderPins` 初版反向断言写反（`ComposePathEffects(dash, corner)` 的正确语义就是
corner 先施、dash 后施，等于手工反序链，不是「必须不等」）；改为把两个操作数次序**各自**逐坐标
钉成对应手工链（bit-for-bit），顺序摘除才必然红。

自审（门禁跑完后逐文件复查）又抓到一个正确性洞：值形态自定义效果（value receiver 实现 `PathEffect`）
原本统一哈希成 `1`，注释理由「等值必等输出」不成立——**不同参数值**会共用一个 GPU 几何缓存条目，
第二次直接取回第一次的几何，画错形状。先加钉拿到红（`TestDashGeomCacheUnkeyableEffectNeverSharesAnEntry`），
再按 Skia 口径修（`SkPathEffect` 是引用计数对象、缓存按对象身份键）：非引用形态效果哈希返回 **0=无键**，
`GetOrApplyEffect` 见 0 直接应用不入库；`PathEffectHash` 文档与 `TestPathEffectHashIsParameterStable` 同步加钉。
详见 `negative-control-batch4/12-unkeyable-custom-effect-red-to-green.md`。

同一轮自审还发现盖章行走缺了 Skia 的密度护栏：`advance` 小到低于取样距离的浮点 ULP 时 `d += advance`
永不进位（死循环），或病态小 `advance` 需要数十亿次盖章（每次新建原型副本 → 吃满内存）。
本机 `Sk1DPathEffect.cpp`（chrome/m135）原文即为此设 `MAX_REASONABLE_ITERATIONS 100000` 每轮廓计数、耗尽 `return false`。
危害量级用只数次数不分配的复刻循环实测（`negative-control-batch4/15-walk-iteration-measure.{md,go}`）：
length=20/advance=1e-9 需 2×10^10 次，1e9 长度级 + advance<ULP 的第三例永不结束。
补 `maxReasonableStamps=100000`（1D 每轮廓、2D 网格总量）+ `TestStampEffectsRejectPathologicalDensity`
（病态必须拒、正常密度不得误伤）；把常量钳到 3 即红（`13-*.log`），还原 100000 即绿（`14-*.log`）。

## 7. 显式不做与遗留（按 D5 与口径登记）

- **fill+effect（stroke-and-fill）不做**：Skia 里 PathEffect 同样作用于填充；本批只接描边链，填充+效果留后续批次立案。
- GPU 侧 effected stroke 一律走 CPU path→path 变换 + 既有展开缓存：关闭 `strokeSolidFastPathEligible` fast 尾、
  `StrokeShape` SDF 口绕行、Vello 计算路 `ErrFallbackToCPU`（三条路由均有钉；非本批新洞，是既有绕行先例）。
- 真窗 `examples/ui_wr_p35_pathgeom` 仍未建（批 1–4 同口径，登记为「已验证待接线」）；
  生产消费者=组件库/2.5D 侧尚未调用 `Stroke.PathEffect` → 状态只到原子验证，不标生产。
- `PathEffect` 录制面按引用携带（录制期取值、回放只读，同 `CustomPatternBrush` 先例）：
  效果对象若在录制后被调用方改写会同时影响回放——已在类型注释与钉里写明契约，不额外加防御拷贝。

## 8. 文档同步核对

- `docs/RENDER_API_CATALOG.md`：§0 计数行（144/184/219/18/172/26，批 4 增量逐项）、§2 路径表（`PathEffect` 族 + `Path1DStyle`/哈希/折算口两行）、§5 描边与绘制状态行（`Stroke.PathEffect`/`WithPathEffect`/`HasPathEffect`、`Paint.EffectivePathEffect`/`HasPathEffect`）、§8 方法速查（`PathEffect`/`Path1DStyle` 行 + `Stroke`/`Paint` 行补方法）、§9 常量成员表（`Path1DStyle` 行）、§13 P3.5 行（批 1–4 已交付 + 批 4 交付明细 + 显式不做）；apidoc 机器校验 rc=0。
- `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md`：§2.1 焦点（当前进行中/下一任务转批 5/最近完成）、载体块批 4 交付条 + 顺带发现条 + 证据条、§13 P3.5 状态行仍 🟡、修订表在批 3 行**之后**追加批 4 行（最新在下）。
- `docs/ENGINE_UI_WIDGET_RENDER.md` §10：**不补行**——口径同批 2：P3.x 各批修订统一落收敛台账，本表只记组件渲染卡相关变更（批 4 无组件面变更）。
