---
name: gpui-wr-rewrite
description: §R 真窗 W 矩阵级调度回流——推翻 W<n> 重写 / W0–W6 全部重写时，展开成该 W 涉及的所有 R 的逐个回流，调度 wr-implement / wr-close 3 模式 / wr-engine / metrics-audit 逐个执行，管 W 矩阵级状态治理（§5 分期表 + §4/§4b/§4c 关闭清单 + §10 修订表）。当用户说「推翻 W2 重写」「W0–W6 全部重写」「重写整个 W3」「rewrite W4」「批量重写 R7/R7b/R10」「重写整波」时触发。
user_invocable: true
disable_model_invocation: false
---

# gpui-wr-rewrite — W 矩阵级调度回流（推翻整波重写）

> **与收敛后 5 skill 的分工：**
> - `wr-implement` = 能力实现回流（ui/ 里实现 + 单测 + 指标接线）——**被本 skill 调度**
> - `wr-close` = 关 R 完整生命周期（3 模式：首次关闭 / 反攻重关 / 优化重关）——**被本 skill 调度**
> - `metrics-audit` = 指标族 A–J 正误审查与 bug 修复（指标层）——**被本 skill 调度**
> - `wr-engine` = 底层修复回流（render / gpu / ui-scene 谨慎改 + 跨层影响面评估）——**被本 skill 调度**
> - **本 skill** = W 矩阵级调度（推翻 W<n> 重写，批量调度 + W 矩阵级状态治理）——**调度层，不直接写代码**
>
> **真源：** `docs/ENGINE_UI_WIDGET_RENDER.md` §5 分期表 + §4/§4b/§4c 关闭清单 + §10 修订表 + §2 主表（该 W 所有 R 行）；`docs/ENGINE_UI_RENDER_BASE.md` §22.1 施工表（该 W 对应的序）。若本 skill 与真源矛盾，以真源为准——发现矛盾停下报告，不要自决。

## 0. 为什么要这个 skill

历史「推翻整波重写」有 3 类错误回流：

| 错误回流 | 表现 | 为何危险 |
|----------|------|----------|
| **逐个 R 独立修底层，重复修同一底层洞** | R4 修 boundary_cache.go，R11 又修一次，R18 第三次 | 底层洞修一次就好；重复修浪费时间 + 可能引入不一致 |
| **没有 W 矩阵级状态治理** | 推翻 W2 重写时，§5 分期表没标 🔄、§4c 关闭清单没标推翻、§10 修订表没记版本 | docs 状态漂移；重写完没人知道这波被推翻过 |
| **批量重写无调度** | 22 个 R + 12 个 C 全部推翻重写，没 skill 展开成逐个回流，用户得手动逐个触发 | 大规模重写时无序；漏触发某些 R |

本 skill 把「W 矩阵级状态降级 → 逐个 R 回流（调度现有 4 个 skill）→ 底层洞批量收集 + 定点修 → 批量重跑验证 → W 矩阵级状态升维」这条回流**固化**。

---

## 1. 触发与输入

用户可能给：

- 一个 W id + 「推翻重写」（如「推翻 W2 重写」「rewrite W4」）→ 单 W 矩阵重写
- 多个 W id + 「全部重写」（如「W0–W6 全部重写」「重写整波」）→ 多 W 矩阵重写
- 一组 R id + 「批量重写」（如「批量重写 R7/R7b/R10」）→ 指定 R 集合重写

从输入解析：

- `W_IDS`（若给）：如 `[W2]` / `[W0, W1, W2, W3, W4, W5, W6]`
- `R_IDS`（若给）：如 `[R7, R7b, R10]`
- **两者必给其一**。若都没给，停下问用户「要推翻哪个 W 重写？或批量重写哪些 R？」

## 第 0 步：读真源 + W 矩阵定位

**必做**——每次都先读，禁止凭记忆跑流程（真源会变）。

### 0.1 读 WIDGET_RENDER §5 分期表

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` §5 分期表，定位该 W：

```
| W | 状态 | 必须绿的单能力窗 | 必须绿的组合窗 |
| W0 | ✅ | R0、R12、R16 子集 | C0 |
| W1 | ✅ | R2/R3/R3b/R5/R9/R12b | C1 |
| W2 | ✅ | R4/R4b/R5/R11/R13/R18 | C2/C7 |
| W3 | ⬜ | R7/R7b/R10 | C3 |
| ...
```

取该 W 的：`状态`（✅ / ⬜ / 🔄）/ 必须绿的单能力窗列表 / 必须绿的组合窗列表。

**⚠ §5 是 W 涉及集的权威来源（不是 §2 主表）。** §5 分期表该 W 行的「必须绿的单能力窗 + 组合窗」列就是该 W 的**完整涉及集**。**禁止只用 §2 主表筛「波次」列**反推涉及集——§2 波次列有「全程」「W1–W2」「W2+」等跨波词，肉眼筛不出某 R 归某 W（如 R12 波次列写「全程」但 §5 W0 行明写「R12（经 C0 schema）」是 W0 涉及集）。

**落地点：** §0.1 取出的 R 列表 + C 列表，是后续 §1 状态降级 / §2 逐个回流 / §4 状态升维的**唯一涉及集依据**。§0.2 只按这个列表逐行读详情，不自己再筛。

### 0.2 读 WIDGET_RENDER §2 主表该 W 涉及集的详情

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` §2 主表，**按 §0.1 §5 分期表取出的涉及集 R 列表**逐行读详情（**禁止**自己按「波次」列 筚筛——跨波词会漏 R），每行取：

- `ID`（如 R7）
- `PACKAGE`（如 `ui_wr_r7_virtlist`）
- `WINDOW` = **1200×800**
- `CLOSE_SECONDS`（如 R7=60）
- `当前状态`（⬜ / ✅ / 🔄）

### 0.3 读 §4/§4b/§4c 关闭清单

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` §4/§4b/§4c，定位该 W 的关闭清单：

```
### 4c. W2 关闭清单
| R4 | ui_wr_r4_composite | ✅ |
| R4b | ui_wr_r4b_multidamage | ✅ |
...
```

### 0.4 读 §10 修订表

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表，取当前最新版本号（如 3.1）。

### 0.5 读 RENDER_BASE §22.1 施工表（若需要跨 W 调度）

若 `W_IDS` 跨多个 W（如 `[W2, W3]`），`read_file docs/ENGINE_UI_RENDER_BASE.md` §22.1，确认这些 W 对应的施工序是否有前后依赖（如 W3 序 12 依赖 W2 序 7/8）。

## 第 1 步：W 矩阵级状态降级

**必做**——推翻重写时，该 W 的所有状态都要降级，docs 不能漂移。

### 1.1 §5 分期表状态降级

`edit_file docs/ENGINE_UI_WIDGET_RENDER.md`，§5 分期表该 W 行：

| 原状态 | 降级后 |
|--------|--------|
| `✅` | `🔄 W<n> 推翻重写` |
| `⬜` | 保持 `⬜`（本来就未关） |
| `🔄` | 保持 `🔄`（已在重写中） |

### 1.2 §4/§4b/§4c 关闭清单状态降级

`edit_file docs/ENGINE_UI_WIDGET_RENDER.md`，§4/§4b/§4c 该 W 关闭清单：

| 原状态 | 降级后 |
|--------|--------|
| `✅` | `🔄 推翻重写中` |
| `⬜` | 保持 `⬜` |
| `🔄` | 保持 `🔄` |

### 1.3 §2 主表该 W 所有 R 行状态降级

`edit_file docs/ENGINE_UI_WIDGET_RENDER.md`，§2 主表该 W 涉及的所有 R 行：

| 原状态 | 降级后 |
|--------|--------|
| `✅` | `🔄` |
| `⬜` | 保持 `⬜`（本来就未关，不需降级） |
| `🔄` | 保持 `🔄` |

### 1.4 §10 修订表加占位行

`edit_file docs/ENGINE_UI_WIDGET_RENDER.md`，§10 修订表加占位行：

```
| <下一版本号> | W<n> 推翻重写：<原因摘要>（进行中） |
```

**降级后**重新 `read_file` 确认 edit 落点对、没破坏表格 markdown。

## 第 2 步：逐个 R 回流（调度现有 4 个 skill）

**核心步骤**——把该 W 涉及的所有 R 逐个回流，每个 R 按「能力是否就绪 → 真窗是否要写/重写 → 底层是否要修 → 指标是否要审」调度对应 skill。

### 2.1 每个 R 的回流决策树

```
for each R in 该 W 的 R 列表:
  ├─ R 能力是否就绪？（ui/rendering/ 有实现 + 单测绿 + 指标接线）
  │   ├─ 否 → 调度 wr-implement 实现能力
  │   │       └─ wr-implement 第 2 步发现底层不满足 → 调度 wr-engine 修底层
  │   └─ 是 → 进下一步
  │
  ├─ R 真窗是否要写/重写？
  │   ├─ R 当前 ⬜ + 真窗未建 → 调度 wr-close 模式 1（首次关闭）
  │   ├─ R 当前 ✅ + 要推翻重写升维 → 调度 wr-close 模式 2（反攻重关）
  │   ├─ R 当前 ✅ + 要优化/增量 → 调度 wr-close 模式 3（优化重关）
  │   └─ R 当前 🔄 + 重写中 → 调度 wr-close 模式 2 继续重关
  │
  ├─ 跑 GPU 时发现底层不满足？
  │   └─ 是 → 调度 wr-engine 修底层
  │
  └─ 跑完 + 判门禁时审指标？
      └─ 是 → 调度 metrics-audit 审指标诚实性
```

### 2.2 调度纪律

- **本 skill 是调度层，不直接写代码**——所有代码改动交被调度的 skill
- **逐个 R 串行调度**（不并行）——避免多个 R 同时改底层引发冲突
- **每个 R 回流完确认状态**——该 R 状态是否从 🔄 → ✅v2 或 ⬜ → ✅？没升维就停下排查

### 2.3 底层洞批量收集（关键）

在逐个 R 回流过程中，**收集所有底层洞**到 `HOLE_LIST`：

```python
HOLE_LIST = []  # 去重，按文件 + 函数去重

for each R in 该 W:
  回流该 R
  if 回流过程中发现底层洞:
    HOLE_LIST.add(底层洞)  # 去重
```

**为何批量收集**：多个 R 可能撞同一个底层洞（如 R4/R11/R18 都需要 BoundaryCache 文 skip）。批量收集后一次修，避免重复修。

### 2.4 底层洞批量定点修（调度 wr-engine）

`HOLE_LIST` 收集完后，**逐个底层洞调度 wr-engine**：

```
for each 底层洞 in HOLE_LIST:
  调度 wr-engine：
    第 0 步：读真源 + 定位底层洞
    第 1 步：跨层影响面评估（render/gpu 谨慎改）
    第 2 步：定点修底层
    第 3 步：跑回归（go test ./ui/... + 若改 gpu/ 跑 gpu 单测）
    第 4 步：交回本 skill 继续逐 R 回流
```

**修完底层后，重新跑该 W 所有相关 R 的真窗**，确认底层修复后这些 R 都能过门禁。

### 2.5 组合窗回流

该 W 的所有单能力 R 回流完后，回流该 W 的组合窗（C*）：

```
for each C in 该 W 的组合窗列表:
  调度 wr-close 模式 1/2（组合窗首次关闭或重关）
  组合窗只做集成回归，不重判单能力门禁
```

## 第 3 步：批量重跑验证

**必做**——W 矩阵级重写完，批量重跑该 W 所有 R + 组合窗，确认全绿。

### 3.1 批量重跑该 W 所有 R 的真窗

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 批量重跑该 W 所有 R
for R in 该 W 的 R 列表:
  RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<package>
  # 取 JSON + stderr
```

### 3.2 批量重跑该 W 的组合窗

```bash
for C in 该 W 的组合窗列表:
  RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<c_package>
```

### 3.3 批量审指标（调度 metrics-audit）

逐个 R 的 JSON 交 metrics-audit 审：

```
for each R in 该 W:
  调度 metrics-audit 审该 R 的 JSON
  ├─ 全 PASS（EARNED + HONEST + CONSISTENT + AT_DEFAULT + PRESENT）→ 该 R 真绿
  └─ 任一 FAIL → 该 R 回第 2 步重回流
```

### 3.4 W 矩阵级真绿判定

该 W 所有 R + 组合窗都真绿，才进第 4 步 W 矩阵级状态升维。

任一 R FAIL → 该 R 回第 2 步重回流；若该 R 反复 FAIL → 停下报告用户「R<id> 反复 FAIL，可能需要更深层的底层修复或能力重设计」。

## 第 4 步：W 矩阵级状态升维

**必做**——W 矩阵级重写完真绿，docs 状态升维。

### 4.1 §5 分期表状态升维

`edit_file docs/ENGINE_UI_WIDGET_RENDER.md`，§5 分期表该 W 行：

| 原状态 | 升维后 |
|--------|--------|
| `🔄 W<n> 推翻重写` | `✅v2（推翻重写后）` |

### 4.2 §4/§4b/§4c 关闭清单状态升维

`edit_file docs/ENGINE_UI_WIDGET_RENDER.md`，§4/§4b/§4c 该 W 关闭清单：

| 原状态 | 升维后 |
|--------|--------|
| `🔄 推翻重写中` | `✅v2` |

### 4.3 §2 主表该 W 所有 R 行状态升维

`edit_file docs/ENGINE_UI_WIDGET_RENDER.md`，§2 主表该 W 涉及的所有 R 行：

| 原状态 | 升维后 |
|--------|--------|
| `🔄` | `✅v2（推翻重写后）` |
| `⬜` → 重写后 ✅ | `✅`（首次关闭，无 v2） |

### 4.4 §10 修订表把占位行改为正式行

`edit_file docs/ENGINE_UI_WIDGET_RENDER.md`，§10 修订表把占位行改为正式版本说明：

```
| <版本号> | W<n> 推翻重写 ✅v2：<重写摘要 + 涉及的 R 列表 + 底层洞修复摘要> |
```

**升维后**重新 `read_file` 确认 edit 落点对、没破坏表格 markdown。

## 输出格式

完成后给用户一份结构化报告：

```
✅ W<n> 推翻重写收口

重写范围：
  W 矩阵：<W2 等>
  涉及 R：<R4/R4b/R11/R13/R18 等>
  涉及组合窗：<C2/C7 等>

W 矩阵级状态降级：
  §5 分期表 W<n> 行：✅ → 🔄 W<n> 推翻重写
  §4c 关闭清单：✅ → 🔄 推翻重写中
  §2 主表该 W 所有 R 行：✅ → 🔄
  §10 修订表：加占位行

逐个 R 回流（调度 wr-implement / wr-close 3 模式 / metrics-audit）：
  R4：wr-close 模式 2（反攻重关）→ ✅v2
  R4b：wr-implement（补 scroll_rerecord）→ wr-close 模式 1 → ✅
  R11：wr-close 模式 2 → ✅v2
  ...

底层洞批量收集 + 定点修（调度 wr-engine）：
  底层洞 1：BoundaryCache 文不 skip → wr-engine 修 ui/rendering/boundary_cache.go
  底层洞 2：retained 下壳闪 → wr-engine 修 ui/embedder/pipeline_app.go
  修完跑 go test ./ui/... PASS + go test ./gpu/... PASS（若改 gpu）

批量重跑验证：
  批量重跑该 W 所有 R 真窗：PASS
  批量重跑该 W 组合窗：PASS
  批量审指标（调度 metrics-audit）：全 PASS

W 矩阵级状态升维：
  §5 分期表 W<n> 行：🔄 → ✅v2（推翻重写后）
  §4c 关闭清单：🔄 → ✅v2
  §2 主表该 W 所有 R 行：🔄 → ✅v2
  §10 修订表：占位行改为正式版本说明

下一步：<W<n+1> 推翻重写？或全部 W0–W6 重写完？>
```

若有任一 FAIL 或停下报告：

```
⚠️ W<n> 推翻重写未达标

当前阶段：<第 1 步状态降级 / 第 2 步逐 R 回流 / 第 3 步批量重跑 / 第 4 步状态升维>
FAIL 原因：<具体>

若第 2 步某 R 反复 FAIL：
  R<id> 反复 FAIL，可能需要：
  1. 更深层的底层修复（调度 wr-engine）
  2. 能力重设计（调度 wr-implement）
  3. 场景矩阵调整（调度 wr-close 模式 2 重新定标准）
  停下报告用户决定方向
```

## 禁令自检（每次结束前过一遍）

- [ ] **本 skill 是调度层，没直接写代码**（所有代码改动交被调度的 skill）
- [ ] **逐个 R 串行调度**（没并行，避免多 R 同时改底层冲突）
- [ ] **底层洞批量收集**（第 2.3 步，去重，避免重复修）
- [ ] **底层洞批量定点修**（第 2.4 步，调度 wr-engine 逐个修）
- [ ] **修完底层重新跑该 W 所有相关 R 的真窗**（确认底层修复后这些 R 都能过门禁）
- [ ] **W 矩阵级状态降级做了**（第 1 步，§5 分期表 + §4/§4b/§4c + §2 主表 + §10 修订表）
- [ ] **W 矩阵级状态升维做了**（第 4 步，§5 分期表 + §4/§4b/§4c + §2 主表 + §10 修订表）
- [ ] **批量重跑验证做了**（第 3 步，该 W 所有 R + 组合窗）
- [ ] **批量审指标做了**（第 3.3 步，调度 metrics-audit 逐个 R 审）
- [ ] **组合窗回流做了**（第 2.5 步，该 W 的组合窗也重关）

任一项未过 → 回对应步骤修，或停下报告用户。

---

## 附：与其他 4 个 skill 的接口

| 接口方向 | 内容 |
|----------|------|
| **出：调度 wr-implement** | 第 2.1 步，该 W 的 R 能力不满足时，调度 wr-implement 实现能力 |
| **出：调度 wr-close 模式 1** | 第 2.1 步，该 W 的 R 当前 ⬜ + 真窗未建时，调度 wr-close 模式 1 首次关闭 |
| **出：调度 wr-close 模式 2** | 第 2.1 步，该 W 的 R 当前 ✅ 推翻重写升维时，调度 wr-close 模式 2 反攻重关 |
| **出：调度 wr-close 模式 3** | 第 2.1 步，该 W 的 R 当前 ✅ 优化/增量时，调度 wr-close 模式 3 优化重关 |
| **出：调度 wr-engine** | 第 2.4 步，HOLE_LIST 收集完后，逐个底层洞调度 wr-engine 修底层 |
| **出：调度 metrics-audit** | 第 3.3 步，批量审指标时，逐个 R 调度 metrics-audit 审 JSON |
| **不接：单 R 的首次关闭** | 单 R 首次关闭仍走 wr-implement → wr-close 模式 1（不需本 skill 调度） |
| **不接：已关 R 的单 R 优化/增量** | 仍走 wr-close 模式 3（不需本 skill 调度） |
| **不接：底层修复执行** | 底层修复执行归 wr-engine（本 skill 只调度 wr-engine 去修） |
| **不接：指标审查执行** | 指标审查执行归 metrics-audit（本 skill 只调度 metrics-audit 去审） |

**关键边界：** 本 skill 只接「W 矩阵粒度的批量推翻重写调度 + W 矩阵级状态治理」。5 个 skill 各管一段：实现能力（wr-implement）→ 关 R 生命周期（wr-close 3 模式）→ 审指标（metrics-audit）→ 修底层（wr-engine）→ W 矩阵级调度（wr-rewrite），全闭环。
