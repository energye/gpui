---
name: gpui-wr-quality
description: §R 主能力真窗的**开发质量验收标准**——复杂场景矩阵 + 窗内可见 HUD + 实现点清单 + 门禁防假绿。在写/改任何 `examples/ui_wr_*` 真窗代码**之前**加载，判定该 R 是否达到「可关闭」的质量条；与 gpui-wr-close（执行流程）互补。当用户说「写 R7 真窗」「改 R4 真窗」「返工 R3」「升维真窗」「wr-quality R5」或开始写/改任一 ui_wr_* 代码时触发。
user_invocable: true
disable_model_invocation: false
---

# gpui-wr-quality — §R 真窗开发质量标准（防门禁假绿）

> **与 gpui-wr-close 的分工：**
> - 本 skill = **关 R 前必须达到的质量条**（复杂度 / HUD / 场景矩阵 / 实现点清单）——**写代码前**加载
> - `gpui-wr-close` = **关 R 的执行流程**（建窗/跑/判门禁/回写状态）——**写完代码后**走
> - 顺序：先 quality（本 skill 定场景与 HUD 设计）→ 写代码 → 再 close（执行 + 判门禁 + 回写）
>
> **真源：** `docs/ENGINE_UI_WIDGET_RENDER.md` §0 U17–U20（本 skill 是其执行骨架）。若本 skill 与真源矛盾，以真源为准——发现矛盾停下报告，不要自决。

## 0. 为什么要这个 skill

历史 `ui_wr_*` 真窗形成了「门禁假绿」模式：场景 = 1–3 个 `RenderColorBox` + `sin` 变色，指标只在**结束时 stdout JSON**，人眼看不到 FPS/damage/skip，`damage_ratio<0.35` 在简陋场景下轻松通过却**不证明**复杂 UI 下正确。本 skill 把验收标准从「JSON 绿」升维到「复杂场景 + 窗内可见 + 实现点全面」。

**禁止的关闭证据（任一出现即不得标 ✅）：**
- 纯色块双节点关闭主能力
- 只有 JSON 知道、人眼看不出对应字段
- 简陋场景下过门禁（如 `damage_ratio` 在「绿静+红热」两色块下 ≪0.35）
- 无 HUD 运行中可见指标
- 实现点清单缺项（见 §4）

---

## 1. U17 复杂场景（每个主能力窗硬要求）

每个 `ui_wr_r*` / `ui_wr_c*` 必须**同时**具备五项，缺一即质量不达标：

| # | 要求 | 落地 |
|---|------|------|
| 1 | **多区域布局** | ≥ 壳层（TopBar 标题/相位/政策 + 主内容 + 侧栏/图例 + 底栏 HUD），或等价复杂度；禁止「整屏一坨」 |
| 2 | **静态密集内容** | ≥ 8 个文字标签 或 4×4 色格 + 嵌套 boundary；**不能只靠纯色块**（纯色块阵 = 假绿） |
| 3 | **动态热点** | ≥1 处持续动画/交互，与静态同屏；人眼能区分「静/动」 |
| 4 | **能力专属压力** | 见 §3 场景矩阵该 R 行；**禁止**用他能力场景冒充（如 R3 不许只测色块 skip，要测嵌套 + 文） |
| 5 | **相位脚本** | ≥2 相：稳态（Steady）/ 突变（Spike）/ 可选恢复（Recover）；人眼与门禁都能区分相位切换 |

### 壳布局约定（1200×800，所有窗共享）

```text
┌────────────── TopBar 64px（标题 / 相位 / 政策）──────────────┐
│ Side 200px │           Body 主场景                  │
│  图例/说明  │   静态密集 + 动态热点 + 能力专属      │
│            │                                       │
├────────────┴───────────────────────────────────────┤
│ Status/HUD 120px：实时指标条（大字，人眼可读）        │
└─────────────────────────────────────────────────────┘
```

- 静态壳（TopBar 背景、Side 说明）必须 `RepaintBoundary + 稳态 skip`
- 若 retained 下壳闪/残 → **引擎 FAIL**，不是「示例写坏了就降门禁」——停下报告引擎洞，不许删场景保门禁
- 壳布局走共享基座 `examples/wrkit`（见 §2），**不**塞进 `ui/` 产品路径

---

## 2. U18 窗内可见指标（运行中硬要求 · LiveHUD）

**禁止**只在结束时 stdout JSON。运行中窗内必须能人眼读出：

| 必须在窗内可见 | 实现 |
|----------------|------|
| 能力 ID + 当前相位 | TopBar 标题条 |
| 实时 FPS / interval p95 | 滚动数字（每帧或每 N 帧刷新） |
| 本能力核心计数器 | 如 `boundary_skip` / `damage_ratio` / `dirty_layer_ids` / `bind_count` / `boundary_count` / `paint_count` |
| PASS/FAIL 预览色 | 绿=当前达门禁趋势，红=已破线（结束仍以 JSON 为准，HUD 只是人眼预警） |
| 图例 | 静态区 / 脏区 / boundary 边界用色或描边约定 |

### 实现纪律（防止 HUD 污门禁）

- HUD 走 `examples/wrkit` 共享 `LiveHUD`（Overlay 顶层 或 root 上**非 Boundary** 的 HUD 节点，自脏每帧）
- 数据源：`app.Metrics().Snapshot()` / `LastBoundaryFrame()` / `DamageStats` / `LastPresentMode` / `LastDirtyLayerIDs` / 场景自报 `ability_extra`——**不新增 ui→gpu 依赖**
- 绘制：`DrawString` + 半透明底
- **关键：HUD 自身 dirty 不污染主能力门禁**——二选一：
  - (a) 门禁统计排除 HUD 层（content-only damage）
  - (b) HUD 独立 band，damage 只计 content band
- 默认每窗开 HUD；`WR_HUD=0` 可关（CI 仍可开）

---

## 3. 场景矩阵（每 R 关 R 前必达）

> 共同：从「两色块」→「迷你应用壳」。每窗至少：TopBar + 左图例 + Body 静态 dense（≥8 标签或 4×4 色格 + 嵌套 boundary）+ Body 动态（能力专属）+ 底栏 HUD。

| ID | 当前简陋点 | 复杂场景要求 | 窗内必见 | 引擎若挂必修 |
|----|------------|--------------|----------|--------------|
| **R0** | 两色块 | 静网 + 动块 + 文；Clear 后静不丢 | policy、静格存在 | FullPaint 路径 |
| **R2** | 一静一热 | 9 宫静 + 1 热；邻格不变色 | paint_count、仅热 debug 闪 | 局部 NeedsPaint |
| **R3** | 两 RB | 嵌套 3 层 RB + 静文 + 热；skip 累加可见 | skip↑ rerecord 仅热 | BoundaryCache 文/嵌套 |
| **R3b** | 计数 | 深嵌套 + 旁路 boundary；depth 可见 | boundary_count/depth | compositing bits |
| **R4** | 绿+红 | **retained 壳**：顶栏静、体热、侧栏静；damage≪1 | policy、dmg%、mode | CompositeOnly+LoadOpLoad |
| **R4b** | 两角色块 | 四角热 + 中心 dense 静 + 双远端同帧脏 | dirty_ids≥2、multi 次数 | multi damage |
| **R5** | 直绘/回放两区 | 同构图左右对照 + 多 op（path/文/图） | op_count、两侧一致 | Picture 完备 |
| **R9** | 单文本 | 多样式重复 measure + 热改一字 | hit/miss 数字 | measure cache |
| **R11** | 单 box 改 size | 周期改 client/逻辑 scale；文+线+网 | 一波 rerecord 再 skip | InvalidateBoundaryCache |
| **R12b** | 两色块 | debug 开：多节点，人眼见谁闪 | debug 描边计数 | DebugRepaint |
| **R13** | 脚本点 4 点 | 变换/重叠/裁剪下可点；高亮+HUD 命中名 | hit id 大字 | Hit≡Paint |
| **R18** | 预算 allow/reject | 组半透明 + 超预算拒绝可观测 | allow/reject 计数 | SaveLayer 预算 |
| **C1/C2/C7** | 略好但仍色块 | 组合：嵌套+retained+resize 同壳 | 各 R 字段同屏 | 集成回归 |

**R1 / R19 / R21 / R16 完整窗**：若本次返工顺带，按同标准；否则明确 ⬜ 不占 ✅。

---

## 4. U20 能力点清单（每 R 关 R 前自检）

每个 R 在 `docs/ENGINE_UI_WIDGET_RENDER.md` §2 主表行须有**「实现点」子表**（或在 README 里），覆盖六项：

| 维度 | 要回答 | 示例（R3） |
|------|--------|------------|
| **正确性** | 能力语义对了吗 | 嵌套 RB 的子树脏不重录父 Picture |
| **脏区** | 成本 ∝ 脏吗 | skip 累加、rerecord 仅脏、damage 不含静区 |
| **缓存** | 命中/失效正确吗 | 文/嵌套静区能 skip；size 变 invalidate 后一波重录再 skip |
| **边界条件** | 极端不炸吗 | 深嵌套、空子树、同帧多次 invalidate |
| **失败模式** | 挂了怎么表现 | boundary 数=0、cache miss 全路径、超预算 reject |
| **窗内如何看出** | HUD/图例对应字段 | skip 数字、rerecord debug 描边、静区色稳定 |

**缺任一项 → 不得标 ✅**（即使 JSON 门禁绿）。

---

## 5. 开发流程（与 wr-close 串接）

写/改任一 `ui_wr_*` 真窗时：

1. **加载本 skill**（`wr-quality <id>` 或自动）——取该 R 的场景矩阵行 + 实现点六维
2. **加载 `gpui-wr-close`**——取执行流程（建窗/跑/判门禁/回写）
3. **先写场景设计**（不先敲代码）——在对话里确认：
   - 壳布局（TopBar/Side/Body/Status 走 `wrkit`）
   - 静态 dense 内容（标签/色格/嵌套）
   - 动态热点（能力专属）
   - 相位脚本（Warm→Steady→Spike→Recover）
   - HUD 字段清单（对应门禁字段名）
   - 与场景矩阵该 R 行对照，缺口先列
4. **写代码**——`main.go` 用 `wrkit` 壳 + `LiveHUD`；`README.md` 可见效果表与场景一一对应
5. **跑 + 判门禁**——走 wr-close 的第 2–3 步；HUD 预览色与结束 JSON 一致
6. **实现点自检**——六维清单逐条过；缺项补代码或补 README，不许跳
7. **回写文档**——wr-close 第 5 步：§2 主表状态列 + 「实现点」子表 + 若升维则 §0 U17–U20

### 若引擎在复杂场景下炸

**禁止**为过门禁删场景降复杂度。停下报告引擎洞，走「引擎定点修」：

| 高概率洞（复杂场景后会炸） | 修哪 |
|----------------------------|------|
| BoundaryCache 仅 Color/Absolute MVP，文/自定义 OnPaint 静区无法 skip | `ui/rendering/boundary_cache.go` 扩 record 类型或强制静区走可录路径 |
| 壳在 retained 下闪/残；force/clear 与 CompositeOnly 交界；HUD damage 污染 | `ui/embedder/pipeline_app.go` |
| DirtyLayerIDs 每帧重建，R4b id 不稳 | 稳定 cacheID 绑定 |
| 无像素采样门禁，纯 ratio 假绿 | 可选 `SAMPLE_PX` 探针（逻辑坐标读回或约定截图像素） |

**不做（本波）：** 全量 multi-RT 层纹理 compositor、默认全局 retained（仍属 W6）。

---

## 6. 质量自检清单（关 R 前过一遍）

- [ ] 场景达 U17 五项（多区域 / 静态密集 / 动态热点 / 能力专属压力 / 相位脚本）
- [ ] 走 `wrkit` 壳布局，不是「整屏一坨」
- [ ] 静态密集 ≥ 8 标签或 4×4 色格 + 嵌套 boundary（不是纯色块阵）
- [ ] HUD 运行中可见：能力 ID + 相位 + FPS/p95 + 核心计数器 + PASS/FAIL 预览色 + 图例
- [ ] HUD dirty 不污染主能力门禁（排除层 或 独立 band）
- [ ] README 可见效果表与场景一一对应（区域坐标/颜色/行为）
- [ ] 门禁字段在 HUD 上有同名或等价标签
- [ ] 场景矩阵该 R 行全覆盖（窗内必见列每项都在）
- [ ] 实现点六维清单全（正确性/脏区/缓存/边界条件/失败模式/窗内如何看出）
- [ ] 相位 ≥2（稳态/突变/可选恢复），人眼与门禁都能区分
- [ ] 引擎若挂——报告洞 + 定点修，**没**删场景保门禁
- [ ] 没降画质装绿（如把 fps 门禁放宽过 55、把 slope 阈值抬高、把 damage_ratio 阈值松到简陋场景能过）

任一项未过 → 不许标 ✅，回去补。

---

## 7. 与 wr-close 的接口

本 skill 输出一份「场景设计 + HUD 字段清单 + 实现点六维」给 wr-close 作为**验收基准**。wr-close 的第 3 步（判门禁）必须同时判：

- JSON 族 A–J 门禁（wr-close 负责）
- U17 场景复杂度（本 skill 负责，对照场景矩阵）
- U18 HUD 可见（本 skill 负责）
- U20 实现点清单（本 skill 负责）

**wr-close 判 JSON 绿但本 skill 判 U17/U18/U20 任一未过 → 综合 FAIL，不得 ✅。** 此时回 wr-close 第 1 步补代码，不许降标准。

---

## 8. 共享基座 `examples/wrkit`（先做再改各窗）

> **落点纪律：** 在 `examples/wrkit/`，**不**塞进 `ui/`（CODING_RULES §5：架构 vs 示例边界）。

```text
examples/wrkit/
  hud.go          # LiveHUD：帧末读 Metrics + 自绘文本面板
  legend.go       # 区域图例、damage 高亮（debug）
  harness.go      # 相位：Warm→Steady→Spike→Recover
  sceneutil.go    # 壳布局：TopBar / Body / Side / Status
  labels.go       # 静态多行标签、网格、嵌套 Absolute
```

**P0 样板流程（建议）：**
1. 新建 `examples/wrkit`（LiveHUD + ShellLayout + PhaseHarness）
2. 选 **R4** 作样板窗完整升级（最能暴露 retained 真问题）
3. 样板达 U17–U19 后，提炼模板供其它窗复用
4. 不并行改 15 个窗——先打透 R4，再按 P1/P2/P3 分波返工

---

## 9. 文档与状态治理（返工时）

若该 R 是因质量升维而返工：

1. `docs/ENGINE_UI_WIDGET_RENDER.md` §2 主表该 R 行：
   - 增「场景复杂度」/「HUD 字段」两列（若主表尚未有）
   - 状态 `✅ → 🔄 质量返工`（保留「基础门禁曾绿」备注，但**不得宣称关闭**）
2. §4/§4b/§4c 关闭清单改为「基础路径曾通 / 质量条未过」
3. README 增可见效果表 + HUD 截图式文字描述 + 相位说明
4. §8 宣称纪律追加：禁止无 HUD 关闭 / 禁止纯色块双节点关闭主能力 / 禁止仅 JSON 无窗内对应字段

**升维绿后**：状态 `🔄 → ✅v2（质量条）`，并在 §10 修订表加版本说明。

---

## 10. 一句话

> **关 R 前：场景复杂（U17）+ 窗内 HUD 可见（U18）+ 可见效果≡门禁（U19）+ 实现点六维全（U20）。** 与 `gpui-wr-close` 串用：先 quality 定场景与 HUD 设计 → 写代码 → 再 close 执行判门禁 + 回写。禁止删场景保门禁；引擎挂了报告洞定点修。
