---
name: gpui-wr-engine
description: §R 真窗底层修复回流——在 wr-implement（能力实现）/ wr-close（关 R 任一模式）/ wr-rewrite（W 矩阵重写）流程中发现 `ui/rendering` / `ui/embedder` / `ui/scene` / `render` / `gpu` / `ui/io` 不满足时，做跨层影响面评估 + 定点修底层 + 跑回归。render/ 和 gpu/ 直接对接 GPU 后端，**最谨慎改**——改前必须评估 GPU 驱动兼容性 + 对上下游层的影响，影响面超阈值停下报告用户。当用户说「修底层」「render 不满足」「gpu 谨慎改」「定点修引擎」「类 C 引擎洞」「底层依赖不满足」「BoundaryCache 文不 skip 修哪」「retained 下壳闪修 pipeline_app」时触发。
user_invocable: true
disable_model_invocation: false
---

# gpui-wr-engine — 底层修复回流（render / gpu / ui-scene 跨层谨慎改）

> **与收敛后 5 skill 的分工：**
> - `wr-implement` = 能力实现回流（ui/ 里实现 + 单测 + 指标接线）
> - `wr-close` = 关 R 完整生命周期（3 模式：首次关闭 / 反攻重关 / 优化重关）
> - `metrics-audit` = 指标族 A–J 正误审查与 bug 修复（指标层）
> - **本 skill** = 底层修复回流（render / gpu / ui-scene / ui-embedder / ui-rendering 跨层谨慎改 + 影响面评估）
> - `wr-rewrite` = W 矩阵级调度（推翻 W<n> 重写，批量调度 + W 矩阵级状态治理）
>
> **真源：** `docs/ENGINE_CODING_RULES.md` §0.4「ui → render → gpu，禁止 ui→gpu，禁止 CGO（purego）」+ §5「架构 vs 示例边界」；`docs/ENGINE_UI_RENDER_BASE.md` §1 架构对照 + §25.4 架构诚实点 6 条；`docs/ENGINE_UI_WIDGET_RENDER.md` §6 模块落点 + wr-close§5「高概率洞表」。若本 skill 与真源矛盾，以真源为准——发现矛盾停下报告，不要自决。

## 0. 为什么要这个 skill

历史底层修复有 3 类错误回流：

| 错误回流 | 表现 | 为何危险 |
|----------|------|----------|
| **在示例里绕引擎洞** | BoundaryCache 文/嵌套静区不 skip，却在 `examples/ui_wr_*/main.go` 里把静区改成纯 ColorBox 让 skip 能过 | 违反 wr-close 模式 1/2 定标准（U17 复杂场景硬要求）；降画质装绿；洞还在，下个 R 还会撞 |
| **render/gpu 擅自改** | 改 `gpu/swapchain.go` 的 LoadOpLoad 不评估 GPU 驱动兼容性 | 直接对接 GPU 后端，可能破坏所有 R 的 Present 路径；wgpu 版本升级会回归 |
| **跨层影响面不评估** | 改 `render/context.go` 的 SaveLayer 不评估对 `ui/scene` 层合成 + `gpu` RT 隔离的影响 | 单点改动引发多 R 多 C 回归；CI 机器跑不全才发现 |

本 skill 把「底层洞定位 → 跨层影响面评估 → 定点修 → 跑回归」这条回流**固化**，禁止在示例里绕洞、禁止 render/gpu 擅自改、禁止跨层影响面不评估就改。

---

## 1. 触发与输入

用户可能给：

- 一个底层洞描述（如「BoundaryCache 文不 skip 修哪」「retained 下壳闪修 pipeline_app」「render Picture 不支持回放」）→ 定点修底层
- 一个谨慎改请求（如「gpu 谨慎改」「render 改评估影响面」）→ 跨层影响面评估 + 定点修
- 一个从其他 skill 转交的「底层依赖不满足」（如 `wr-implement` 实现能力时发现 `render/picture.go` 不支持回放；`wr-close` 任一模式跑 GPU 时发现 `gpu/swapchain.go` 的 LoadOpLoad 不对）→ 跨层影响面评估 + 定点修 + 跑回归

从输入解析：

- `HOLE_DESC`（必给）：底层洞描述（如「BoundaryCache 文不 skip」「retained 下壳闪」「render Picture 不支持回放」）
- `SUSPECT_LAYER`（若给）：怀疑的层（如 `ui/rendering` / `ui/embedder` / `ui/scene` / `render` / `gpu` / `ui/io`）
- `SUSPECT_FILE`（若给）：怀疑的文件（如 `ui/rendering/boundary_cache.go` / `ui/embedder/pipeline_app.go` / `render/picture.go` / `gpu/swapchain.go`）
- `SOURCE_SKILL`（若从其他 skill 转交）：转交的 skill 名（如 `wr-implement` / `wr-close` / `wr-rewrite`）

## 第 0 步：读真源 + 定位底层洞

**必做**——每次都先读，禁止凭记忆跑流程（真源会变）。

### 0.1 读 CODING_RULES §0.4 + §5（工程纪律 + 架构边界）

`read_file docs/ENGINE_CODING_RULES.md` §0.4「ui → render → gpu，禁止 ui→gpu，禁止 CGO（purego）」+ §5「架构 vs 示例边界」——确认底层修复纪律。

### 0.2 读 RENDER_BASE §1 架构对照 + §25.4 架构诚实点

`read_file docs/ENGINE_UI_RENDER_BASE.md`：

- §1 架构对照（Flutter vs gpui 各层职责）
- §25.4 架构诚实点 6 条（PipelineApp 默认 RO 直绘 / Picture 显示列表子集 / Backdrop 全幅成本 / VSync / cpu_ui-raster proxy / gpu_ops 累计）

### 0.3 读 WIDGET_RENDER §6 模块落点 + wr-close 模式 1 第 0 步高概率洞表

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` §6 模块落点表，确认底层洞该落哪个包：

| 能力 | 包 |
|------|-----|
| Present / PaintPresentTree / policy | `ui/embedder` |
| 脏 layout/paint、VirtualList | `ui/rendering` |
| Picture、Layer、Composite | `ui/scene` |
| Overlay | `ui/overlay` |
| 指标 | `ui/scheduler` |
| **全部真窗** | `examples/ui_wr_*` only |

`read_file .grok/skills/gpui-wr-close/SKILL.md`（若已收敛）取其模式 1 第 0 步「高概率洞表」——这是底层洞的定位基准。

### 0.4 定位底层洞文件

按 `SUSPECT_LAYER` / `SUSPECT_FILE` / `HOLE_DESC` 定位：

```bash
# 例：BoundaryCache 文不 skip
grep -n "FrameSkip\|record\|OnPaint" ui/rendering/boundary_cache.go

# 例：retained 下壳闪
grep -n "useRetained\|CompositeOnly\|LoadOpLoad\|PresentPolicy" ui/embedder/pipeline_app.go

# 例：render Picture 不支持回放
grep -n "Replay\|RasterizeDirty\|Picture" render/*.go ui/scene/picture.go
```

`read_symbol` / `list_symbols` 看缺陷函数的结构，定位需改的函数/字段。

## 第 1 步：跨层影响面评估（核心步骤）

**必做**——render/ 和 gpu/ 直接对接 GPU 后端，改前必须评估影响面，禁止擅自改。

### 1.1 确定改动层

按 §0.4 定位的文件，确定改动层：

| 层 | 文件示例 | 改动风险 |
|----|----------|----------|
| `ui/rendering/` | `boundary_cache.go` / `virtual_list.go` | 低——能力实现层，改了影响该能力相关 R |
| `ui/embedder/` | `pipeline_app.go` | 中——Present 路径，改了影响所有 R 的 Present |
| `ui/scene/` | `picture.go` / `composite.go` / `layer.go` | 中——Picture/Layer/Composite，改了影响 R5/R8/R20 等 |
| `render/` | `context.go` / `draw.go` / `path.go` / `image.go` | 高——画布/绘制 API，改了影响所有 R 的绘制 |
| `gpu/` | `device.go` / `swapchain.go` / `libwgpu_native` 绑定 | 最高——直接对接 GPU 后端，可能破坏所有 R 的 Present + wgpu 版本升级回归 |
| `ui/io/` | `decode.go` | 低——异步图 worker，改了影响 R10 |

### 1.2 评估跨层影响面

按改动层的风险等级评估：

#### `ui/rendering/` 改动（低风险）

- 影响该能力相关 R（如改 `boundary_cache.go` 影响 R3/R3b/C1/C2）
- **通过** → 进第 2 步定点修
- 不需停下报告

#### `ui/embedder/pipeline_app.go` 改动（中风险）

- 影响所有 R 的 Present 路径
- 评估：改的是 PresentPolicy 切换？还是 CompositeOnly 交界？还是 HUD damage 污染？
- **通过**（改动局部 + 有单测覆盖）→ 进第 2 步
- **停下报告**（改动涉及全局 Present 策略）→ 报告用户「改 pipeline_app.go 影响所有 R 的 Present，建议先跑全回归确认影响面」

#### `ui/scene/` 改动（中风险）

- 影响 Picture/Layer/Composite 相关 R（R5/R8/R20/C2/C4 等）
- 评估：改的是 Picture 录回放？还是 Composite walk？还是 Layer 隔离？
- **通过**（改动局部 + 有单测覆盖）→ 进第 2 步
- **停下报告**（改动涉及层合成主路径）→ 报告用户「改 ui/scene 影响层合成主路径，建议先跑全回归确认影响面」

#### `render/` 改动（高风险）

- 影响所有 R 的绘制（render 是画布/绘制 API 基座）
- 评估：
  - 改的是哪个 API？（`DrawRectangle` / `FillPath` / `DrawImage` / `SaveLayer` / `ClipRect` 等）
  - 改了影响哪些上游？（`ui/rendering/draw.go` 调 `render` / `ui/scene/picture.go` 调 `render`）
  - 改了影响哪些下游？（`gpu` 的 RT 隔离 / submit 路径）
- **通过**（改动局部 + 有单测覆盖 + 下游 gpu 不受影响）→ 进第 2 步
- **停下报告**（改动涉及 render 全局 API 或影响下游 gpu）→ 报告用户「改 render/ 影响所有 R 的绘制基座，建议先跑全回归 + gpu 单测确认影响面」

#### `gpu/` 改动（最高风险）

- 直接对接 GPU 后端（wgpu native + 系统句柄），改了可能破坏所有 R 的 Present + wgpu 版本升级回归
- 评估：
  - 改的是哪个 API？（`Device` / `Swapchain` / `Queue` / `BindGroup` / `libwgpu_native` 绑定等）
  - GPU 驱动兼容性：改了在 Mesa / AMD / Intel / NVIDIA 驱动下都能跑吗？
  - wgpu 版本兼容性：改了 `libwgpu_native` 绑定，wgpu 升级会回归吗？
  - 改了影响哪些上游？（`render` 的 submit 路径 / `ui/embedder` 的 Present 路径）
- **强制停下报告**（gpu/ 改动必须用户确认）→ 报告用户「改 gpu/ 直接影响 GPU 后端，需确认：
  1. GPU 驱动兼容性（Mesa/AMD/Intel/NVIDIA）
  2. wgpu 版本兼容性
  3. 是否有替代方案（改 render 层绕过？）
  4. 影响面是否超阈值（改了所有 R 的 Present 都要重跑）」
- **用户确认改** → 进第 2 步（但必须加 gpu 单测 + 全回归）
- **用户不确认 / 建议绕过** → 改 render 层绕过（降风险），或停下不修（该 R 保持 🔄）

### 1.3 影响面评估输出

```
📋 底层洞跨层影响面评估

底层洞描述：<HOLE_DESC>
改动层：<ui/rendering / ui/embedder / ui/scene / render / gpu / ui/io>
改动文件：<file.go>
改动函数/字段：<func/field>
风险等级：<低 / 中 / 高 / 最高>

跨层影响面：
  上游影响：<ui/rendering 调 render / ui/scene 调 render / 等>
  下游影响：<render 调 gpu / gpu 调 libwgpu_native / 等>
  影响的 R/C：<R3/R3b/C1/C2 等>

评估结论：
  ├─ 通过（改动局部 + 有单测覆盖 + 下游不受影响）→ 进第 2 步定点修
  ├─ 停下报告（改动涉及全局路径，需用户确认）→ 报告用户
  └─ 强制停下报告（gpu/ 改动，必须用户确认）→ 报告用户
```

## 第 2 步：定点修底层

按 §1.3 评估结论「通过」或「用户确认改」的，进本步。

### 2.1 修复纪律

- **禁止 ui→gpu 依赖**（CODING_RULES §0.4：ui → render → gpu）
- **禁止 CGO**（purego；CODING_RULES §1）
- **禁止在示例里绕洞**（该改 ui/rendering 或 render，不改 examples/ui_wr_*/main.go 绕洞）
- **禁止降画质装绿**（不在 render/gpu 层放宽门禁）
- 改动局部化，不破坏现有 API 契约（若破坏，必须同步改上游调用 + 跑全回归）

### 2.2 定点修动作

按 §0.4 定位的缺陷函数/字段，`edit_file` 定点修：

| 典型底层洞 | 修哪 | 修什么 |
|------------|------|--------|
| BoundaryCache 仅 Color/Absolute MVP，文/自定义 OnPaint 静区无法 skip | `ui/rendering/boundary_cache.go` | 扩 record 类型或强制静区走可录路径 |
| 壳在 retained 下闪/残；force/clear 与 CompositeOnly 交界；HUD damage 污染 | `ui/embedder/pipeline_app.go` | 修 PresentPolicy 切换 + LoadOpLoad + HUD 独立 band |
| DirtyLayerIDs 每帧重建，R4b id 不稳 | `ui/rendering/` 或 `ui/embedder/` | 稳定 cacheID 绑定 |
| 无像素采样门禁，纯 ratio 假绿 | 可选 `SAMPLE_PX` 探针 | 逻辑坐标读回或约定截图像素 |
| render Picture 不支持回放 | `ui/scene/picture.go` 或 `render/` | 补 Replay + RasterizeDirtyToContext |
| gpu/swapchain.go 的 LoadOpLoad 不对 | `gpu/swapchain.go`（**最高风险，必须用户确认**） | 修 LoadOpLoad 语义 + GPU 驱动兼容性评估 |

### 2.3 修完 → 进第 3 步跑回归

## 第 3 步：跑回归（确认没破坏其他 R/C）

### 3.1 跑改动的单测

```bash
# 例：改了 ui/rendering/boundary_cache.go
go test ./ui/rendering -run TestBoundaryCache -count=1 -v

# 例：改了 ui/embedder/pipeline_app.go
go test ./ui/embedder -count=1 -v
```

### 3.2 跑全 ui 包回归

```bash
go test ./ui/... -count=1
```

**全绿才能进第 3.3 步。** 若有回归（之前绿的测现在红了），回第 2 步排查改动是否破坏了已有能力。

### 3.3 若改了 gpu/，跑 gpu 单测 + 全回归

```bash
# gpu/ 改动必须跑 gpu 单测
go test ./gpu/... -count=1 -v

# + 全回归
go test ./... -count=1
```

**全绿才能进第 4 步。** 若有回归，回第 2 步；若 gpu 单测红，**停下报告用户**「gpu 改动引发回归，建议回退或改 render 层绕过」。

## 第 4 步：交回原 skill 继续流程

底层洞修完 + 跑回归全绿，交回 `SOURCE_SKILL` 继续流程：

| `SOURCE_SKILL` | 交回到哪一步 |
|----------------|--------------|
| `wr-implement` | 交回 `wr-implement` 第 2 步继续实现能力（底层已修，能力可实现） |
| `wr-close`（模式 1 首次关闭） | 交回 `wr-close` 模式 1 第 1 步继续写真窗（底层已修，真窗可写） |
| `wr-close`（模式 2 反攻重关） | 交回 `wr-close` 模式 2 第 2 步继续重写（底层已修，真窗可重写） |
| `wr-close`（模式 3 优化重关） | 交回 `wr-close` 模式 3 第 1 步继续优化（底层已修，优化可继续） |
| `wr-rewrite`（W 矩阵重写） | 交回 `wr-rewrite` 第 2 步继续逐 R 回流（底层已修，W 矩阵逐 R 重写可继续） |
| 无（用户直接触发「修底层」） | 修完 + 跑回归全绿，输出报告，结束 |

## 输出格式

完成后给用户一份结构化报告：

```
✅ 底层洞修复收口

底层洞描述：<HOLE_DESC>
改动层：<ui/rendering / ui/embedder / ui/scene / render / gpu / ui/io>
风险等级：<低 / 中 / 高 / 最高>

跨层影响面评估：
  上游影响：<摘要>
  下游影响：<摘要>
  影响的 R/C：<R3/R3b/C1/C2 等>
  评估结论：<通过 / 停下报告 / 强制停下报告>

定点修：
  改了哪些文件：
    - <file.go>（<修复摘要>）
  改了哪些函数/字段：<func/field>

回归验证：
  go test ./ui/rendering -run Test<Func>  PASS
  go test ./ui/...  PASS（无回归）
  go test ./gpu/...  PASS（若改了 gpu/）
  go test ./...  PASS（若改了 gpu/，跑全回归）

交回原 skill：
  SOURCE_SKILL：<wr-implement / wr-close 模式N / wr-rewrite / 无（用户直接触发）>
  交回到哪一步：<摘要>
```

若有任一 FAIL 或停下报告：

```
⚠️ 底层洞修复未达标 / 需用户确认

底层洞描述：<HOLE_DESC>
改动层：<层>
风险等级：<等级>

评估结论：<停下报告 / 强制停下报告>
停下原因：
  - gpu/ 改动必须用户确认（GPU 驱动兼容性 + wgpu 版本兼容性 + 替代方案 + 影响面超阈值）
  - 或：改动涉及全局 Present 路径，需跑全回归确认影响面

需用户确认的 4 点：
  1. GPU 驱动兼容性（Mesa/AMD/Intel/NVIDIA）
  2. wgpu 版本兼容性
  3. 是否有替代方案（改 render 层绕过？）
  4. 影响面是否超阈值（改了所有 R 的 Present 都要重跑？）
```

## 禁令自检（每次结束前过一遍）

- [ ] **禁止 ui→gpu 依赖**（CODING_RULES §0.4）
- [ ] **禁止 CGO**（purego；CODING_RULES §1）
- [ ] **禁止在示例里绕洞**（该改 ui/rendering 或 render，不改 examples/ui_wr_*/main.go）
- [ ] **禁止降画质装绿**（不在 render/gpu 层放宽门禁）
- [ ] 改动局部化，不破坏现有 API 契约（若破坏，同步改上游 + 跑全回归）
- [ ] **跨层影响面评估做了**（第 1 步，render/gpu 改动必须评估）
- [ ] **gpu/ 改动强制停下报告用户**（第 1.2 步，最高风险）
- [ ] **render/ 改动评估了对 gpu/ 下游 + ui/rendering 上游的影响**（第 1.2 步）
- [ ] 跑了改动的单测（第 3.1 步）
- [ ] 跑了全 ui 包回归（第 3.2 步）
- [ ] 若改了 gpu/，跑了 gpu 单测 + 全回归（第 3.3 步）
- [ ] 交回了原 skill 继续流程（第 4 步，若 SOURCE_SKILL 非空）

任一项未过 → 回对应步骤修，或停下报告用户。

---

## 附：与其他 4 个 skill 的接口

| 接口方向 | 内容 |
|----------|------|
| **入：wr-implement 转交** | `wr-implement` 第 2 步实现能力时发现底层不满足 → 交本 skill 第 0 步 |
| **入：wr-close 任一模式转交** | `wr-close` 模式 1/2/3 跑 GPU 时发现底层不满足 → 交本 skill 第 0 步 |
| **入：wr-rewrite 转交** | `wr-rewrite` 第 2 步逐 R 回流时发现底层不满足 → 交本 skill 第 0 步 |
| **入：用户直接触发** | 「修底层」「render 不满足」「gpu 谨慎改」→ 交本 skill 第 0 步 |
| **出：交回原 skill** | 第 4 步，底层修完 + 跑回归全绿，交回 `SOURCE_SKILL` 继续流程 |
| **不接：能力实现** | ui/rendering 里实现能力归 wr-implement（本 skill 只修底层洞，不实现能力） |
| **不接：真窗写代码** | examples/ui_wr_*/main.go 写代码归 wr-close（本 skill 不写真窗） |
| **不接：指标层审查** | 指标诚实性审查归 metrics-audit（本 skill 不审 JSON） |
| **不接：W 矩阵级调度** | 推翻 W<n> 重写批量调度归 wr-rewrite（本 skill 是被调度者，不是调度者） |

**关键边界：** 本 skill 只接「底层洞定位 → 跨层影响面评估 → 定点修底层 → 跑回归」。5 个 skill 各管一段：实现能力（wr-implement）→ 关 R 生命周期（wr-close 3 模式）→ 审指标（metrics-audit）→ 修底层（wr-engine）→ W 矩阵级调度（wr-rewrite），全闭环。
