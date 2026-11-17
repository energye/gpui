# 节点录制覆盖与性能面同步（立项书）

日期：2026-11-17 ｜ 分支：`feat/UI-组件` ｜ 基线：`c96a5d8c`
状态：**立项（计划文档，本会话零代码）——新会话按 §8 开工流程施工**。
真源：`ENGINE_FRAME_PRESENT_STANDARD.md`（按需帧块1）、`ENGINE_UI_RENDER_BASE.md`（渲染母表）、
成熟实现：`/home/yanghy/app/projects/gogpu/flutter`（RepaintBoundary + RasterCache）、
`/home/yanghy/app/projects/gogpu/skia`（PictureLayer + SkiaResourceCache）。

## 0. 一句话

引擎公开 14 种 RenderObject 节点类型，性能优化层（边界录制/滚动位移复用）只覆盖 4 种——
**能力面与性能面脱节，且脱节是静默的**（用了合法能力无告警掉出快路径）。本卡把性能面补齐到
能力面全量：录制从中央白名单改为**节点多态接口**（编译期强制，新增类型漏不掉），配套机器门禁
与组合态验收窗。完成后**任意节点组合**的滚动、交互、
静止场景自动走高效路径，无须各消费者自查。

## 0.5 对未来开发的覆盖保证（三条链 + 四条边界）

**保证链（结构性，不靠枚举）**：

1. **未来组件自动继承**：将来实现的所有组件（F1–F6 全部 + 更远）都是现有节点类型的
   组合——不产生新节点类型就不需要做任何性能事，组合树落在哪复用/隔离跟到哪；
2. **未来新节点类型编译期强制接入**：不实现 Record → record_coverage_test 红 → 合不进去；
   每帧必变的类型声明 Volatile 正确退化为直画。白名单悲剧（静默降级）机制上不可能重演；
3. **失效/复用作用于树本身**：判定对象是渲染树状态（谁脏、谁指纹变），与业务内容无关——
   将来任何窗口任何场景拼出任何树，机制照常工作。

**四条诚实边界**：

| 保证 | 不保证 |
| --- | --- |
| 不变的内容不重画不重放 | 变的内容画得快——绝对帧成本靠图元效率与各专项线 |
| 新类型编译期强制接入 | Volatile 类（视频/粒子/实时全变）不做缓存，正确直画 |
| 保留树内的一切组合 | 绕过保留树直接调 render/ 的用法不在覆盖内（出现需显式声明另评） |
| 本卡覆盖"缓存复用"维度 | 全新维度需求（GPU 计算内容/多窗共享层等）按归位纪律另立项 |

**收敛为一条对未来开发的纪律**（随 §5.4 写入 AGENTS.md）：将来任何组件、任何 2.5D 功能，
按契约写——动效走 Motion 自停协议、动效子树标 RepaintBoundary、新节点实现 Record/Volatile、
标脏在循环线程。**遵守即自动受益，违反被机器闸卡住。**

**适用范围（2026-11-17 实证）**：ui/rendering 是全部上层渲染的公共保留层——
UI 组件库（kit 经 prim）、**2.5D 引擎（examples/engine 的 tilemap/sprite/stage_chase
均 import ui/rendering，已核实）**、视频、以及一切基于引擎窗口的应用内容都构建在
这 14 种节点类型上。本卡的覆盖修复对三者同时生效，非 divider 专属。
诚实边界：2.5D 实时画面多数节点每帧全脏，录制复用收益小（正确退化为直画），
其专项优化（并行批处理/剔除/LOD）另有真源，与本卡正交；将来任何新自定义节点
类型被 Record 接口强制接入，静默降级不再可能。

**术语**：业界统称 Retained Mode Layer Caching + Invalidation Tracking；"滚动=位移不重画"叫
Composited Scrolling。对标：Flutter RepaintBoundary + RasterCache；Skia PictureLayer 资源缓存；
Android RenderNode DisplayList。本引擎对应物：`Base.SetRepaintBoundary` + `BoundaryCache`（已有雏形）。

## 1. 问题与实测（证据，全部可复现）

场景：kit divider 长页（1786px、7 段演示、~200 节点、40+ 文字标签）滚动/拖拽
（该页已按指示回退，数据保留作证据；divider 用的节点类型全部是引擎公开 API）。

| 场景 | 帧 | CPU（四核合计） | 每帧光栅 |
| --- | --- | --- | --- |
| 静止 15s | **2 帧 / 3 present** | **0.02%** | — |
| 滚轮 | 38 帧 | 13.1% | ~26ms |
| 按住拖动 | 152 帧（跑满 vsync） | **38.1%** | ~26ms |

**覆盖完备性的三层定位（2026-11-17 验证）**："覆盖全"= 三层齐备，缺一即静默掉出快路径：

| 层 | 内容 | 状态 |
| --- | --- | --- |
| 组合层 | 任意嵌套（容器/裁剪/透明/变换/层）递归录制 | ✅ 已完备 |
| **算子层** | 录制器实现完整 `render.Canvas`——**118 个绘制算子逐个对账，缺口 0**（编译器强制，结构上不可能漏） | ✅ 已完备 |
| **节点层** | 14 种节点类型的录制覆盖 | ❌ **4/14，唯一缺口（本卡）** |

即：引擎能画的任何东西在算子层天然可录；唯一的洞在节点遍历层（白名单 4/14）。
本卡补齐节点层后，三层闭环——**任意图形渲染组合都受益是完备性结论，不是经验期望**。

**根因（代码事实）**：边界录制器（`ui/rendering/boundary_cache.go`）的节点类型覆盖只有
`RenderColorBox/RenderText/RenderImage/RenderPictureBox` 4 种（`recordOwnContent`/
`absoluteContentCacheable` 白名单）；divider 内容含 `RenderBox(OnPaint 自绘轨线)` 与
`RenderClipRRect(圆角裁剪代码块)`——白名单外 → 整棵内容树判"不可缓存" → 滚动复用（R7b
原点位移回放）静默失效 → 每滚动帧全窗重录+重放（26ms/帧）。

**证伪记录（防新会话走弯路，2026-11-17 实测）**：落地"内容边界零重录"（paint 层重录消除，
paint_count 62→22）后，**每帧光栅 26.7→27.4ms 不变**——paint 层重录不是光放成本的主因。
滚动/拖拽帧的光放成本归**合成侧**，归 `ENGINE_PHASE_P6_DAMAGE_COMPOSITE.md`（已立项：
持久合成面 + 只合成脏层 + 层纹理缓存）修复。本卡与 P6 分工见 §4，两卡合并验收 divider 场景。

## 2. 现状盘点（已有机制——本卡不重做，全部复用）

| 已有件 | 位置 | 说明 |
| --- | --- | --- |
| 边界录制回放 + 预算淘汰 | `ui/rendering/boundary_cache.go` | tryReplay/Store/预算/淘汰计数齐全 |
| R7b 原点位移回放 | `boundary_cache.go` tryReplay（"scroll reuse (R7b)"） | 录制按当前原点平移回放，滚动位移天然支持 |
| 失效传播 | `ui/rendering/node.go` MarkNeedsPaint/MarkNeedsLayout/SubtreeNeedsPaint | 脏标记→边界阻断→按路径重录 |
| RepaintBoundary | `node.go:193` SetRepaintBoundary | 边界声明机制 |
| 视口层结构（滚动=独立变换层） | `layer_build.go:353` Boundary+ClipRect+PushTransform(-scroll) | 内容 picture 指纹与滚动无关（Flutter ViewportLayer 语义，C4 修复） |
| scene 层结构 | `ui/scene/layer.go` OffsetLayer/ClipLayer/PictureLayer | 合成期位移已可表达 |
| O11 层显示列表缓存 | `ui/rendering/layer_cache.go` StoreRecording/按指纹复用 | 干净层免重建 |
| GPU 层纹理（PictureTextureCache/offscreenPass） | `render/internal/gpu/gpu_render_context.go:108,661` | picture→离屏纹理已有（P6 消费方） |
| 按需帧循环 | `ENGINE_FRAME_PRESENT_STANDARD.md` 块1 | 不动不画（静止 0.02% 实测） |
| transient 节拍（排帧睡到帧边界，非 0 轮询热自旋） | `ui/scheduler/scheduler.go` pacingDeadlineLocked（**已实现已验证，随批回滚撤销**——历史 84c04b92 可找回） | 拖拽 35% 的最大单项根因；新会话**步骤 0** 与钩子一起重落地 |
| 录制格式裁剪命令 | `render/recording/recorder_canvas_ops.go:107` ClipRect/ClipRoundRect + command.go:313 + 回放 | Push/Pop/Clip 命令已可录制回放 |
| 脏→排帧唤醒钩子 | `ui/rendering/pipeline.go` SetNeedsFrameHook（**已实现已验证，随 divider 批回滚撤销**——`git show 84c04b92 -- ui/rendering/pipeline.go` 可找回） | 跨线程标脏必醒；新会话作为**步骤 0** 重落地 |

## 3. 缺口（本卡全部要做的）

| # | 缺口 | 证据 |
| --- | --- | --- |
| G1 | 录制覆盖 4/14：缺 AlignBox/Box(OnPaint)/ClipRRect/ColorFilter/ImageFilter/Opacity/Spinner/Transform/Video/Viewport 共 10 种 | `recordOwnContent` 类型分发 switch |
| G2 | 覆盖判定是中央白名单 switch（新增类型要人记得改，漏=静默降级） | 同上；divider 实测即后果 |
| G3 | 无机器门禁：新增 RenderObject 类型不强制同步性能层 | 无对应测试/检查 |
| G4 | 视口内容子树未自动成为缓存边界；滚动帧走全量重录路径（CompositeOnly=false） | `viewport.go` Paint |
| G5 | 无组合态验收窗：混合节点类型的滚动/交互/静止资源行为从未验证 | divider 为首个暴露场景 |

## 4. 与 P6 的分工（合并验收 divider 拖拽场景）

| 卡 | 层 | 解决 |
| --- | --- | --- |
| **P6**（已立项，`ENGINE_PHASE_P6_DAMAGE_COMPOSITE.md`） | 合成/GPU 侧 | 持久合成面 + 只合成脏层 + 层纹理缓存——**滚动/拖拽帧的 26ms 光放成本在这里消**（blit 量级） |
| **本卡** | paint/录制层 | 录制覆盖全覆盖 + 接口化防静默降级 + 门禁——CompositeOnly 重录路径的遍历成本与边界可用性 |

两卡互相独立、可并行；divider 拖拽场景达标 = 两卡合并验收（合成侧 P6 出光放数字，本卡出零重录/覆盖数字）。

## 5. 方案（当前框架怎么做）

### 5.1 节点多态录制接口（替代中央白名单）

```go
// ui/rendering：节点自己负责忠实录制；编译期接口，新增类型漏不掉。
type Recordable interface {
    // 忠实录制自身+可录子树（裁剪/透明/变换按语义）；scale 是绘制走查的
    // 设备缩放（PaintContext.Scale），OnPaint 里的吸附必须与活画同网格。
    Record(rec Canvas, ox, oy, scale float64)
}
type Volatile interface {
    Volatile() bool  // 每帧必变（Spinner/Video）→ 声明不缓存，退化为直画
}
```

- `BoundaryCache.Store/tryReplay` 改走接口判定：实现了 Recordable 且非 Volatile → 可录制；
  否则该边界不含它（父边界诚实降级，不静默丢内容）。
- 实现偏差（施工记录）：Record 增加 `scale`（原签名只有 ox/oy）——OnPaint 闭包会用
  `pc.Scale` 做设备网格吸附，缺少它会在 DPR≠1 时录制出与活画不同的亚像素结果；
  `RenderPictureBox` 的公开字段 `Record` 改名 `RecordContent`（Go 不允许同名字段与方法，
  控制动词 `SetRecord` 不变，调用方同步切换）。
- **实现原则**：各节点类型的现有 Paint 逻辑是唯一真值——录制必须与 Paint 逐语义等价
  （裁剪=Push/Clip/Pop、透明=层命令、变换=矩阵命令、OnPaint=隔离 Push/Pop 调闭包），
  录制与 Paint 的任何不一致都是陈帧 bug（验收窗像素断言卡）。

### 5.2 14 种类型逐个实现（缺的 10 种 + 已有 4 种迁移）

| 类型 | 处理 | 要点 |
| --- | --- | --- |
| RenderColorBox / RenderText / RenderImage / RenderPictureBox | 迁移现有实现 | 行为不变 |
| RenderBox（OnPaint 叶子/带子） | 新增 | Push/Pop 隔离调 OnPaint（闭包经 pc.Abs 落进录制）+ 子节点递归 |
| RenderClipRRect | 新增 | Push + ClipRoundRect(半径) / ClipRect + 子递归 + Pop |
| RenderAlignBox | 新增 | 子节点按对齐偏移录制 |
| RenderOpacity | 新增 | 层命令（Recorder 按需补 PushLayerIsolated）或声明子树录制+回放期整组透明 |
| RenderColorFilter / RenderImageFilter | 新增 | 滤镜命令录制（Recorder 按需补） |
| RenderTransform | 新增 | 矩阵命令录制（recorder_projection 已有基础） |
| RenderSpinner / RenderVideo | **Volatile** | 每帧必脏，显式不缓存 |
| RenderViewport | **结构性** | 自身不入缓存（内容子层为缓存单元）；已由 layer_build Clip+Transform 表达 |

### 5.3 视口内容层接入（G4）

- `SetContent`/构造时内容子树自动 `SetRepaintBoundary(true)`；
- 滚动帧（视口脏、内容子树干净）→ 保持 CompositeOnly，内容边界走 R7b 位移回放（零重录）；
- 内容脏 → 全量重录 + 以内容本地 (0,0) 为锚存储（**锚点必须与滚动无关**——滚动锚定会烤进
  位移导致其他偏移像素错误，divider 会话实测踩过）；
- 超长内容上限（建议 8192px 逻辑高）：超限回退逐帧重录（虚拟列表内容本就视口大小，不受影响）。

### 5.4 机器门禁（G3，防复发）

- `ui/rendering/record_coverage_test.go`：源码级枚举 `type Render* struct` 全部节点类型，
  逐一断言"已实现 Recordable 或已声明 Volatile/结构性不可缓存"——新增类型漏实现即红；
- AGENTS.md 增补一条流程纪律：「新增 RenderObject 类型必须同步录制实现或 Volatile 声明，
  `record_coverage_test` 红即打回」（与 API 总账同步同级）。

### 5.5 组合态验收窗（G5）

- `examples/ui_wr_record_coverage/`：一窗混摆全部 14 种节点类型 + 视口滚动 + 一个 Spinner
  + 异步图片；三段证据：① 静止期 0 present；② 滚动期内容层零重录（R7b 命中计数）+ 像素
  逐位正确（含新暴露条带）；③ Spinner 期只它重录、其余 skip；窗口/后端标记照
  AGENTS.md 跨平台纪律一次标齐（X11+Wayland/GL+wgpu）。

## 6. 施工分解（每步独立可验收，逐文件跑测试）

| 步 | 内容 | 验收 |
| --- | --- | --- |
| 0 | 流程前置修复重落地（从 84c04b92 历史找回）：① PipelineOwner.SetNeedsFrameHook 脏→排帧唤醒钩子 + PipelineApp 接线；② transient 排帧睡到帧边界（pacingDeadlineLocked）+ ScheduleFrame 翻转返回 | TestPipelineOwner_NeedsFrameHook 绿 + scheduler/embedder 全量绿 + 拖拽帧率 vsync 正常（无热自旋） |
| 1 | Recordable/Volatile 接口 + 4 个已有类型迁移（行为不变） | 现有 rendering/kit 全量回归绿 |
| 2 | 10 种缺口类型逐个实现（每类型：实现 + 单测 + 1 个录制/回放像素断言） | record_coverage_test 绿；每类型像素断言绿 |
| 3 | 视口内容边界 + 滚动位移复用（含锚点/上限/回退） | TestViewport_ScrollReuse 类单测（位移像素正确+零重录） |
| 4 | 机器门禁 + AGENTS.md 纪律条目 | record_coverage_test 红绿双向验证（删实现→红→还原→绿） |
| 5 | 组合态验收窗（§5.5） | 三段证据 + Golden + -race + 双端标记 |
| 6 | ENGINE_API_CATALOG.md 同步（Recordable/Volatile 公开接口） | 目录覆盖检查绿 |

## 7. 明确不做（防重复清单）

- 不重做按需帧循环（块1 已有）、失效传播（已有）、边界预算/淘汰（已有）、scene 层结构（已有）；
- 不重做 GPU PictureTextureCache 本体（已有，本卡只接滚动/内容层路径）；
- 不动合成侧 damage/持久面（**P6 卡的活**，本卡不碰 render_session 合成选择逻辑）；
- 不动 2.5D 专项优化线（并行批处理/剔除/LOD 另有真源）与文本专项线（glyph/整形另有计划）；
- Spinner/Video 不做缓存（Volatile 直画，正确且不亏）。

## 8. 新会话开工流程

1. 读 `AGENTS.md` → 本文件 → `ENGINE_PHASE_P6_DAMAGE_COMPOSITE.md`（协同卡）→
   `ENGINE_FRAME_PRESENT_STANDARD.md`；
2. 源码对照（不猜）：`boundary_cache.go`（tryReplay/Store/recordOwnContent/白名单）、
   `viewport.go`、`layer_build.go:353`（视口层结构）、`recorder_canvas_ops.go`（录制命令面）；
3. 按 §6 步骤施工，每步：逐文件跑测试 + `-race` + 本步验收；
4. 验收门禁：§6 各步验收 + `CGO_ENABLED=0 go build ./ui/...` + 相关能力窗回归；
5. 完成一步同步本文件 §9 台账 + AGENTS.md 纪律条目（第 4 步起）。

## 9. 状态台账

| 步 | 状态 | 证据 | 日期 |
| --- | --- | --- | --- |
| 立项 | ✅ 本文档 | divider 场景实测数据（§1）+ 证伪记录（paint 重录≠光放成本） | 2026-11-17 |
| 步 0（流程前置重落地） | ✅ | 钩子按 84c04b92 原样恢复（pipeline.go:SetNeedsFrameHook/翻转守卫 + pipeline_app.go 接线）；节拍按 §2 语义重实现（scheduler.go:pacingDeadlineLocked，transient+pending 睡到帧边界；ScheduleFrame 用 pending.Swap 返回 idle→pending 翻转，调用方只在翻转时 WakeUp）。证据：TestPipelineOwner_NeedsFrameHook、TestWaitTimeout_TransientPacing、TestScheduleFrame_Flip 绿；scheduler/rendering/embedder 逐文件 127 PASS / 1 SKIP_TAGGED（m1_bench 带构建标签）/ 0 FAIL；-race 定向绿；真窗 icon 10s×3 fps_interval 60.03/60.06/60.15、p95 18.12/18.12/17.62ms、vsync=true、cpu_ui 1.86/1.94/1.98%，基线（HEAD 7d6c6865）60.14/60.03/60.11、17.67/17.76/17.83ms、1.99/1.98/1.88%——无回归。注：icon 巡览有在册 ticker（persistent），transient 拖拽热自旋由单测 `TestWaitTimeout_TransientPacing`/`TestScheduleFrame_Flip` 锁定；本轮无自动指针拖拽注入，真拖拽帧率留人工观察（见 §9.1）。icon 门禁因 testdata 缺 showcase_icon_base_go.png 报红属测试数据缺失，非本步回归 | 2026-11-17 |
| 步 1（接口 + 4 类型迁移） | ✅ | `record.go`：Recordable/Volatile + recordableTree/recordChildInto；ColorBox/Text/Image/PictureBox 迁移到 Record，行为不变（PictureBox 字段 Record→RecordContent）。证据：rendering 包逐文件 100 PASS / 1 SKIP_TAGGED / 0 FAIL；kit/embedder/scheduler 逐文件 68 PASS / 0 FAIL；`CGO_ENABLED=0 go build ./ui/...` 绿 | 2026-11-17 |
| 步 2（10 类型逐个实现） | ✅ | Box(OnPaint)/AlignBox/ClipRRect/Opacity/ColorFilter/ImageFilter/Transform 实现 Record；Spinner/Video 声明 Volatile；Viewport 结构性不入缓存。每类型像素断言单测 `record_pixel_test.go`：活画 vs 录制 vs 干净回放逐像素对比（显式 3/255 AA 容差，最差 3/255，实心区 0 差；非均值判法），Volatile/Viewport 断言拒绝缓存且画面仍正确。证据同上逐文件全绿 | 2026-11-17 |
| 步 3（视口内容边界 + 滚动复用） | ✅ | `viewport.go`：内容子树自动 RepaintBoundary；滚动改为画布变换（内容本地 (0,0) 锚点，录制不含 scroll）；>8192 回退 `UseBoundaryCache=false` + 丢条目。证据：`TestViewport_ScrollReuse`（干净滚动零重录 + skip 命中；内容脏重录后换 0/40/120 偏移回放与活画逐像素一致，锚点不烤 scroll）、`TestViewport_OversizeContentFallsBackLive`；窗内 scrollRR=0 / scrollSkip≈108 | 2026-11-17 |
| 步 4（机器门禁 + 纪律） | ✅ | `record_coverage_test.go` 源码枚举全部会 Paint 的类型，Record/Volatile/structuralNode 三选一；红绿双向：临时加 `RenderGateProbe` → 红（exit 1，点名该类型）、删探针 → 绿（exit 0）。AGENTS.md 增补「节点录制覆盖（硬）」小节。渲染包逐文件 100 PASS / 1 SKIP_TAGGED / 0 FAIL | 2026-11-17 |
| 步 5（组合态验收窗） | ✅ | `examples/ui_wr_record_coverage`（1200×800，800×2400 长页 + 侧栏）。gate（full_paint，BoundaryCache 计数观测面）：static 1.5s 0 present；scroll 10s **rerecord=0 / skip≈108**；anim 3.5s **rerecord=0**、Spinner 相位推进；异步图片由 goroutine 落地并装进树（无输入事件）；冻结帧 11 个像素探针全过 + Golden **0.0000%（960000 px）**；`ui_wr_record_coverage: OK`。manual（retained）留人工看效果。后端矩阵见 §9.2 | 2026-11-17 |
| 步 6（API 总账同步） | ✅ | `ui/rendering` 不在 `ENGINE_API_CATALOG.md` 的域内（该账=engine/，apidoc 只扫 render+engine），实际同步到所属域总账 `ENGINE_UI_RENDER_BASE.md` **v1.32**：§13 增 FRO-RECORD-COVERAGE 行 + **§13.1 节点录制契约**（Recordable/Volatile/structuralNode/门禁/视口内容层/已知缺口）；`go run ./scripts/apidoc` 绿 | 2026-11-17 |

### 9.1 遗留与新发现（2026-11-17）

- **验收窗暴露的合成正确性问题（非本卡，已定案）**：详见 `ENGINE_PHASE_P6_CLIP_LAYER_CORRECTNESS.md`。
  1. **视口内圆角裁剪丢失（已修）**：根因是 `render` 的轴对齐判定把 `Matrix.C`（x 平移）当斜切，
     圆角被降级为直角 quad path；4 处统一修为只查 B/D，单测 + 真窗角点采样锁死。此前归因
     「retained 纹理合成忽略圆角」有误——这是 paint 层问题，全路径都会踩。
  2. **隔层组透明失效（已修 · 模糊家族）**：模糊层 `markLayerFullComposite` 把池化层 RT 的旧像素整表铺出，
     B 的 0.5 合成叠在实色上看起来不透明；改为 `markLayerEffect(pad)` 按内容+滤镜扩散有界合成后，
     真机 blur → A → B → C 全部半透明。灰度/颜色矩阵已审计：本就按 damage 有界，定向窗无泄漏；剩余
     drop shadow、SVG 光照/湍流等 fullComposite 效果同类风险待审计。
- **步骤 0 真窗口径**：transient 拖拽节拍由单测锁死（`WaitTimeout_TransientPacing`：transient+pending
  返回帧边界等待且不 0 轮询）；本轮无自动指针拖拽注入，真窗只跑 icon 巡览（persistent 有 ticker）验证
  vsync 正常/无回归；如需真拖拽数，人工拖任意窗用 HUD 观察。
- **验收窗策略**：gate 用 `full_paint`——BoundaryCache 的 skip/rerecord 计数只在 `walkPresentTree`
  路径可观测（retained 走层树 picture cache，计的是另一套）；manual 用 `retained` 保证观感。
  gate fps≈8.7 是 full_paint 的绘制成本，性能归 P6 合成侧，不是本卡的验收项；
  manual（retained）本机实测 44.6fps / p95 19.7ms（12s 短跑，含 2400px 长页 + Spinner），
  短于相位总长时只报告不判（scroll/anim 相位未跑完）。

### 9.2 验收窗后端矩阵（2026-11-17）

| 端 | 状态 | 证据 / 未跑原因 |
| --- | --- | --- |
| X11 + GL（`GPUI_BACKEND=go`） | ✅ | gate OK：static 0 present / scrollRR=0 & skip≈108 / animRR=0；冻结帧 11 探针 + Golden 0.0000%（960000 px） |
| Wayland + GL | ⬜ 未跑 | 本轮无隔离 Wayland 嵌套会话；窗代码无后端分支，待隔离环境按 `ENGINE_WAYLAND_NESTED_TEST.md` 跑 |
| X11 + wgpu | ⬜ 未跑 | 本轮未切原生 wgpu 后端；窗代码无后端分支，属同一验证面 |

## 10. 修订

| 版本 | 日期 | 修订 |
| --- | --- | --- |
| 1.0 | 2026-11-17 | 立项。范围：录制覆盖 4/14→14/14 + 接口化 + 门禁 + 组合验收窗；与 P6 分工划清（拖拽光放成本归 P6）；divider 场景实测数据与证伪记录入档。 |
| 1.1 | 2026-11-17 | 步 0 落地：钩子恢复 + transient 节拍重实现（pacingDeadlineLocked / 翻转返回 / 只在翻转时唤醒）；真窗与逐文件门禁证据入 §9。 |
| 1.2 | 2026-11-17 | 步 1–2 落地：接口化录制 + 14 类型覆盖（Spinner/Video Volatile、Viewport 结构性）；Record 增加 scale、PictureBox 字段改名 RecordContent（理由入 §5.1）；每类型像素断言入 `record_pixel_test.go`。 |
| 1.3 | 2026-11-17 | 步 3–6 落地：视口内容本地锚点 + 滚动零重录；覆盖门禁红绿双向 + AGENTS 纪律；组合窗 `examples/ui_wr_record_coverage` 三段证据（static 0 present / scroll 0 rerecord + skip / anim 0 rerecord）+ 冻结帧像素+Golden 0.0000%；UI 渲染母表 v1.32 同步。新发现（非本卡）：视口内圆角裁剪丢失（平移被当斜切，已修）+ 模糊层后隔层组透明失效（未修），详见 `ENGINE_PHASE_P6_CLIP_LAYER_CORRECTNESS.md`。 |
