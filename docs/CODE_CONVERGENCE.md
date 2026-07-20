# Render / GPU 代码收敛任务卡（功能·性能·内存不回退）

> 版本：1.3 | 日期：2026-07-20  
> 状态：**C9.0–C9.6 完成**（C9.7 可选跳过）  
> 范围：`render` 主路径 + 为 render 服务的 `gpu/webgpu` / `gpu/rwgpu`（**不含控件层**）  
> 调用链（约定，以代码 import 为准）：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

### 本文自包含

本卡 **不依赖** 其它 `docs/*` 计划文档作为规格或进度真源。  
历史主线 / GPU 路由清单 / R7·R8 性能计划 / mem 计划等 **可能与现网代码有出入**，C9 **不引用、不跟做**。

| 优先级 | 真源 |
|--------|------|
| **P0** | **现网生产代码路径**（被 examples / 主 API / 热路径实际调用的行为） |
| **P1** | **本文**（切片、禁止清单、checklist） |
| **P2** | **现网测试**（分层使用，见下；**不是**每一条断言都等于产品规格） |

需要原则时，以 §2 写死的不变量为准；需要命令时，以仓库内实际能跑的 `go test` / `scripts/*` 为准。

### 测试可信度（重要）

现有单测 **并非全部准确或通用**：

| 类型 | 特征（示例） | C9 怎么用 |
|------|----------------|-----------|
| **T0 回归锁** | 主路径 API、组合抽样、present/GPU 路由硬断言、与触及代码直接相关且近期仍绿 | **默认必跑**；红则优先怀疑本刀，回滚或修刀 |
| **T1 对口单测** | 同包/同符号的直接测试 | 必跑；若断言明显过时，**先对照生产代码**再决定 |
| **T2 特场景 / 门控** | `GPUI_*_VISUAL`、X11/window only、长 soak、device-lost、iGPU OOM 隔离、build tag | **按触及面选跑**；未触及对应场景可不跑；失败需区分环境/特场景 vs 真回归 |
| **T3 可能过时** | 断言与现网主路径长期不一致、只服务已删实验、阈值过严/过松、注释写 legacy | **不得**仅因 T3 红就大改生产代码“迁就测试”；也 **不得** 为过 C9 静默删测试装绿 |
| **T4 脆弱/噪声** | 依赖时序、机器负载、未固定 seed 的性能数字 | 不作 C9 合入唯一依据；需复现 2 次或换对口门禁 |

**冲突裁决：**

1. 生产主路径行为清晰 + T0/T1 绿 → 可合入  
2. 本刀后 **T0/T1 新红** 且与改动相关 → 回滚或修刀（默认）  
3. 仅 **T2/T3** 红 → 记录在 `tmp/c9_x_STATUS.md`：环境 / 特场景 / 疑似过时；**不**擅自删测试；不扩大重构去“修全世界”  
4. 发现测试确已过时 → **另开**「测试校正」项（可标 `TestC9Fix_*` 或单独 PR）；**禁止**与大范围删生产代码混在同一刀  
5. **禁止** 用放宽断言、删测试、关 AA/减内容 换 C9 绿  

选门禁时：**少而准**（触及包 + 相关 T0/T1），不要无脑全仓 `go test ./...` 当唯一真理，也不要忽略明显相关的主路径锁。

---

## 0. 一句话目标

在 **不改变功能语义、不劣化性能、不增加内存占用/泄漏** 的前提下，对渲染引擎与底层支撑代码做 **代码收敛**：

- 去除确认无用的死代码 / 过期实验分支  
- 收敛局部冗余（重复 helper、重复常量、复制粘贴的近同逻辑）  
- 提高局部可读性（命名、文件边界、过长函数拆分）  

**每一刀必须可证明：行为等价或更可维护，且正确性 / FPS / 稳态内存只许持平或变好。**

---

## 1. 范围边界（禁止混刀）

本卡只做 **结构收敛**（死代码 / 局部冗余 / 可读性）。

**同 PR 禁止混入：**

1. 性能专项（少 alloc、合并 submit、改 batch/atlas 算法等）  
2. 能力扩展或像素/路由语义变更  
3. 大范围删代码 + 改热路径 + 改资源生命周期  
4. “顺便”统一 fill/brush/GPU-CPU 双实现抽象  
5. 为变干净而改 public API 或默认行为  

与性能、能力、稳定性主线冲突时：**本卡让路**，不抢同一批热文件。

---

## 2. 硬原则（任何切片违反即不得合入）

### P1 — 优先级（冲突时按序）

1. **正确性 / 像素语义 / 能力矩阵**  
2. **GPU 优先 + fallback 可观测**（禁止 silent CPU）  
3. **资源生命周期 / 无新增泄漏**  
4. **性能与 present 不回退**（持平或更好）  
5. **可读性 / 去冗余 / 行数减少**  

可读性 **永远排在最后**；不得为 5 牺牲 1–4。

### P2 — 功能不变（不变量）

| 不变量 | 含义 | 验收 |
|--------|------|------|
| 像素/语义 | fill/stroke/text/image/clip/blend/mask/layer/AA 与改前一致 | 相关单测 + 抽样 `TestP1_Comp_*` / visual |
| GPU 优先 | 有加速器时主路径 `GPUOps>0` 且 `cpu_fallback_ops=0`（书面例外除外） | `RenderPathStats` + 既有 GPU 门禁 |
| Fallback 可观测 | 不得 silent CPU；reason 标签不得消失 | `LastCPUFallbackReason` |
| 公共 API | `render` / 对外 `gpu/webgpu` 签名与语义不变 | 编译 + 调用方测试 |
| ABI / 生命周期 | enum、指针、Release 时机不漂 | facade/ABI 测 + 真 native 加载 |
| Present | present-only 主路径不因本刀恶化 | 触及 frame/present 时跑 S5/S6 L0 |
| 内存 | 无新增泄漏；稳态 RSS 斜率不恶化；热路径 alloc 不无故上升 | mem guard（资源刀必跑） |

### P3 — 变更分类（只允许 A/B；C 另立）

| 类 | 定义 | 允许 | 门禁强度 |
|----|------|------|----------|
| **A. 纯结构等价** | 删死代码、挪文件、重命名私有符号、抽无行为变化的局部 helper | 默认推进 | 触及包测试 + 编译 |
| **B. 局部逻辑合并** | 合并 **已证明** 同语义的重复私有实现（同输入同输出） | 需对照测试/golden | A + 相关语义门禁 |
| **C. 语义/路由/生命周期** | 改默认路径、改 Release、改 public API、跨层抽象 | **禁止混进本卡** | 走主线/能力/性能专项 |

### P4 — 小步收敛

1. **一刀一包或一刀一路径**（见 §5 切片）  
2. 每刀：**盘点 → 最小 diff → 门禁 → 证据 → 保留或回滚**  
3. 无收益、争议大、或正确性/性能/内存风险 → **回滚优先**  
4. 单 PR 建议：**< ~400 行净逻辑变更**（纯移动文件可放宽，但必须零行为）  
5. 合入前写清：删了什么、为何确认无用、风险面、证据路径  

### P5 — 分层不打穿

- `render` **不**直接依赖 `gpu/rwgpu`  
- facade 整理放在 `gpu/webgpu`；FFI 放在 `gpu/rwgpu`  
- 禁止为“少一层文件”把 native 细节泄漏进 `render` 公共 API  
- 禁止把 CPU / GPU 双路径“合成一个漂亮 interface”除非有独立设计评审  

### P6 — 环境可比

```bash
export GOROOT=.../go1.25.5.linux-amd64   # 或本机等价 1.25.x
export PATH=$GOROOT/bin:$PATH
export GOCACHE=$PWD/tmp/go-cache GOWORK=off GOTOOLCHAIN=local
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
export DISPLAY=:1
export GPUI_SURFACE_SAMPLE_COUNT=1
```

---

## 3. 明确禁止清单（AI / 人工均适用）

| # | 禁止 |
|---|------|
| X1 | 删除仅被 build tag / window / benchmark / visual STRICT / X11 window 测试引用的符号 |
| X2 | 删除 device lost / recover / export / filter stale / fallback reason 相关分支 |
| X3 | 把已 GPU 主路径改回 CPU，或去掉 `cpu_fallback_ops` / reason |
| X4 | 删除或放宽组合断言、visual STRICT、mem 门禁来“过绿” |
| X5 | 改 public API 签名、默认 AA、默认 present 策略、默认路由 |
| X6 | 跨包“统一抽象层”大重构（painter/fill/brush/session 一次揉平） |
| X7 | 热路径引入 interface / 反射 / 额外堆分配“为了优雅” |
| X8 | 合并 CPU 与 GPU 实现仅为减少重复行数 |
| X9 | 删除 `docs/` 历史计划、能力表、或仍被引用的测试夹具 |
| X10 | 单 PR 扫全仓 `render`+`gpu` 无边界清理 |
| X11 | 把特场景/门控/过时测试的失败当成必须改生产代码的规格 |
| X12 | 为 C9 过绿删除、跳过或放宽测试断言 |

---

## 4. 收敛流程（每刀 checklist）

```text
[1] 选刀     指定包/文件列表 + 变更类 A 或 B（禁止 C）
[2] 盘点     引用扫描：生产 + _test + examples + scripts + build tags
[3] 冻基线   触及包测试绿；若动热路径记 p50/alloc 抽样
[4] 最小 diff 只做本刀目标；不顺手改无关模块
[5] 窄测     本刀 TestC9x_*（若有）+ 触及包 **T0/T1** 测试
[6] 回归锁   按 §4.1 分级；资源/池刀加 mem；T2 仅在触及对应场景时跑
[7] 结论     收益 + 证据；区分本刀引入失败 vs 基线/特场景/过时测试；失败回滚
[8] 文档回写 本文 §7 状态表 + 可选 tmp/c9_x_STATUS.md
```

### 4.1 必跑门禁分级

| 级 | 何时 | 命令（示意） |
|----|------|----------------|
| **L0 切片** | 每刀必跑 | 触及包：`go test -count=1 ./<pkg>/...`（超时按包） |
| **L1 主路径** | 动到 `render` 核心 / gpu session / present | `go test -count=1 ./render -run 'TestS6_L0_|TestS61_|TestS62_Present|TestS63_Present' -timeout 600s` |
| **L1b 组合抽样** | 动到绘制/clip/blend/layer/text 语义 | `TestP1_Comp_(D01|D06|D08|D36|D63|D152)_` |
| **L1c GPU 路由** | 动到 fill/stroke/filter/layer/mask 路由 | `TestF1_|TestP03_|TestP04_|TestP12_` 等对口；断言 `cpu_fb=0` |
| **L2 组合/全量** | B 类合并 或 阶段关闭 | 扩大 `TestP1_Comp_` 或 `scripts/run_full_unit_tests.sh` 策略 |
| **L3 mem** | 池 / texture / atlas / Release / encoder 生命周期 | `./scripts/run_mem_guard.sh quick`（加深用 daily） |
| **L4 问题墙** | filter/layer/glow/session 大整理 | PKS 子集或 `PROBLEMS_LATEST` 对口探针 |

**失败策略：**

- **T0/T1 正确性红**（与本刀相关）→ 不得关闭切片，优先回滚  
- 新增 `cpu_fallback_ops>0` 或 silent 路径 → 回滚  
- present p50 恶化 >10% 无说明 → 修或回滚  
- mem 斜率/泄漏信号 → 回滚  
- 仅“看起来更干净”但主路径门禁不稳 → 不合入  
- **T2 特场景红** → 核对是否触及该场景；未触及可记“未跑/环境”；触及则修刀或回滚  
- **T3 疑似过时红** → 对照生产代码；不迁就改生产、不删测试装绿；另开测试校正  
- 全仓测试里本就存在的失败（与本刀无关）→ 在 STATUS 注明“基线已有”，不当作本刀引入  

### 4.2 死代码确认规则（删除前必须满足）

删除符号 / 文件前，**全部**满足：

1. 全仓 `rg` 无引用（含 `examples/`、`scripts/`、`standardtest/`、`widget/`）  
2. 无 `//go:build` 条件路径专用  
3. 非 CGO / 字符串反射 / 测试名动态拼接引用  
4. 非“保留给 ABI 对称 / 文档示例 / 对外兼容”的导出 API  
5. 在 PR 描述写明：**删除理由 + 最后引用消失的 commit/日期（若可知）**  

不确定 → **保留** 或标 `// retained: <reason>`，不得猜删。

### 4.3 冗余合并规则

允许合并的重复：

- 同包内 `min/max/clamp/abs`、bounds 矩形工具、重复常量  
- 仅命名不同、经测试证明同行为的私有 helper  
- 复制粘贴的错误处理样板（不改控制流）  

**默认不合并：**

- CPU vs GPU 管线  
- solid vs non-solid vs advanced blend 分路  
- GPU vs GPU\*（bootstrap/readback）分路  
- webgpu facade 与 rwgpu FFI 的“看似重复”转换  

---

## 5. 包优先级与切片计划（C9.x）

### 5.1 优先级（从低风险到高风险）

| 优先级 | 包 / 区域 | 风险 | 说明 |
|--------|-----------|------|------|
| **P0** | `render/internal/color`、`render/internal/testutil`、纯 math/helper | 低 | 首选开工 |
| **P1** | `render/internal/cache`、`parallel`、`wide`、`stroke`（私有重复） | 低–中 | 先盘点再删 |
| **P2** | `render` 根目录：过长文件 **仅拆分/挪移**（零行为） | 中 | 禁止顺手改逻辑 |
| **P3** | `render/text` 非热路径私有 helper | 中 | 避开 layout/atlas 热路径除非只删死代码 |
| **P4** | `render/internal/gpu` **死代码/未用私有** only | 中–高 | **禁止**改 submit/flush/blend 语义 |
| **P5** | `gpu/webgpu`、`gpu/rwgpu` 死代码 / 未用转换 helper | 中–高 | ABI/生命周期门禁 |
| **P6** | 跨路径 B 类合并（fill/brush 重复） | 高 | 单独评审；默认可跳过 |

### 5.2 切片卡

#### C9.0 — 盘点与基线（开工第 0 刀，可不改代码）

**目标：** 产出可删/可合并候选清单，避免盲扫。

| 动作 | 产出 |
|------|------|
| 静态未使用扫描 | `tmp/c9_0_unused.txt`（`staticcheck` / 手工 `rg` 结果） |
| 候选分级 | A 类死代码 / B 类重复 / 保留（含 reason） |
| 基线 | 触及包 `go test` 绿记录到 `tmp/c9_0_STATUS.md` |

**验收：** 有清单 + 基线；**不要求**本刀必须删代码。

```bash
# 示例：按环境调整
go test -count=1 ./render/internal/color/... ./render/internal/cache/... -timeout 120s
```

---

#### C9.1 — P0 低风险包死代码与局部 helper 收敛

**范围：** `render/internal/color`、`testutil`、确认安全的纯 helper。

**做：**

- 删除无引用私有类型/函数  
- 合并同包重复 min/max/clamp/颜色工具  
- 必要时补 1 个表驱动单测锁合并行为  

**不做：** 改颜色空间默认、改预乘语义。

**验收：**

```bash
go test -count=1 ./render/internal/color/... ./render/internal/testutil/... -timeout 120s
go test -count=1 ./render -run 'TestColor|TestP1_Comp_D01_' -timeout 300s
```

**退出：** L0 绿；无 API 变更；`tmp/c9_1_STATUS.md`。

---

#### C9.2 — P1 内部支撑包（cache / parallel / wide / stroke）

**做：** 未用缓存条目类型、重复键构造、死分支；**不**改缓存命中语义与失效策略。

**验收：**

```bash
go test -count=1 ./render/internal/cache/... ./render/internal/parallel/... \
  ./render/internal/wide/... ./render/internal/stroke/... -timeout 300s
go test -count=1 ./render -run 'TestS6_L0_|TestS66_|TestS67_|TestP1_Comp_D01_' -timeout 600s
```

动到池/生命周期 → 加 L3 mem quick。

---

#### C9.3 — 大文件零行为拆分（可读性）

**目标：** 将显著过长、职责混杂的 `.go` **按类型/职责切开**，**禁止改逻辑**。

**规则：**

- diff 以 move 为主；逻辑行应可被 review 为等价  
- 拆后包 API 不变；仅文件边界变化  
- 一次最多 1–2 个巨石文件  

**验收：** 原包全测 + 若属 `render` 根目录则 L1 抽样。

```bash
go test -count=1 ./render -run 'TestS6_L0_|TestS61_|TestP1_Comp_D01_' -timeout 600s
```

---

#### C9.4 — `render/text` 非热路径清理

**做：** 死解析分支、未用私有 struct、重复编码表（确认后）。

**不做：** 改 shaper 默认、atlas 上传、LCD/glyph 像素、layout cache 键语义。

**验收：**

```bash
go test -count=1 ./render/text/... -timeout 600s
go test -count=1 ./render -run 'TestS65_|TestP11_|TestText|TestP1_Comp_D76_' -timeout 600s
```

---

#### C9.5 — `render/internal/gpu` 死代码 only

**硬限制：** 只删 **确认无引用** 的私有 helper / 废弃实验；**不**改：

- Flush / Submit / batch / damage scissor  
- dual-tex / filter / layer RT / glyph pipeline  
- encoder 录制顺序  

**验收：**

```bash
go test -count=1 ./render/internal/gpu/... -timeout 600s
go test -count=1 ./render -run 'TestS6_L0_|TestS62_|TestS63_|TestF1_|TestP03_|TestP04_|TestP1_Comp_(D01|D06|D08|D36)_' -timeout 900s
```

动资源 → L3 mem。

---

#### C9.6 — `gpu/webgpu` + `gpu/rwgpu` 死代码 / 重复私有转换

**做：** 未用 descriptor 转换、废弃 stub、重复小工具。

**不做：** 改 Release 顺序、改 Submit 语义、改 enum 数值、删 ABI 对称 API。

**验收：**

```bash
go test -count=1 ./gpu/webgpu/... ./gpu/rwgpu/... -timeout 300s
go test -count=1 ./render -run 'TestS6_L0_|TestF1_' -timeout 600s
```

---

#### C9.7 — （可选）B 类同语义合并

**仅当** C9.0 清单中有 **高置信 + 有测试锁** 的重复实现。

**流程：** 先写/确认对照测试 → 再合并 → L1b + 对口 GPU 测 → 文档记录合并点。

**默认可跳过**；不确定则不做。

---

### 5.3 建议执行顺序

```text
C9.0 盘点
  → C9.1 P0 color/testutil
  → C9.2 P1 cache/parallel/wide/stroke
  → C9.3 巨石文件拆分（穿插，零行为）
  → C9.4 text 非热路径
  → C9.5 internal/gpu 死代码
  → C9.6 webgpu/rwgpu 死代码
  → C9.7 可选 B 类合并
```

稳定性 / device lost / 控件入口 **主线冲突时**：本卡让路，不抢同一批热文件。

---

## 6. AI 执行规则（可直接当 system/task prompt）

```text
你在 gpui 仓库执行 docs/CODE_CONVERGENCE.md 的 C9.x 切片。

硬约束：
- 功能、像素语义、GPU 优先、fallback 可观测、公共 API、资源生命周期均不得回退
- 性能与内存只许持平或变好；可读性优先级最低
- 一刀一包/一路径；禁止全仓大扫除；禁止 C 类语义改动
- 删除前必须完成 §4.2 引用确认；不确定则保留
- 禁止把 CPU/GPU 双路径、GPU/GPU* 分路当成冗余删除或强行统一
- **真源优先：生产代码主路径**；测试分层（T0–T4），不把特场景/过时断言当唯一规格
- 不阅读、不跟做其它历史 plan 文档；不为过绿删测试或放宽断言

每刀必须：
1) 声明切片 ID 与文件列表
2) 最小 diff
3) 跑 §4.1 对应门禁
4) 写 tmp/c9_x_STATUS.md（命令、结果、删改摘要）
5) 失败则回滚该刀，不扩大修改面

工作目录：gpui；可写 tmp/；示例可在目标目录 go run。
```

### 6.1 单刀 PR 描述模板

```markdown
## C9.x — <标题>
- 类：A / B
- 包：...
- 删除/收敛摘要：...
- 为何确认安全：引用扫描 / 测试锁
- 门禁：L0 [ ] L1 [ ] L1b [ ] L1c [ ] L3 [ ]
- 证据：tmp/c9_x_STATUS.md
- 风险与回滚：...
```

---

## 7. 执行状态

| 切片 | 状态 | 证据 | 备注 |
|------|------|------|------|
| C9.0 盘点与基线 | ✅ 完成 (2026-07-20) | `tmp/c9_0_STATUS.md` · `tmp/c9_0_unused.txt` | P0/P1 基线绿 |
| C9.1 P0 color/testutil | ✅ 完成 (2026-07-20) | `tmp/c9_1_STATUS.md` | 删 `ColorSpace*` |
| C9.2 P1 支撑包 | ✅ 完成 (2026-07-20) | `tmp/c9_2_STATUS.md` | 删 `BlendSolidColorBatchAA` |
| C9.3 巨石拆分 | ✅ 完成 (2026-07-20) | `tmp/c9_3_STATUS.md` | `recorder.go` → 4 文件，零逻辑 |
| C9.4 text 非热路径 | ✅ 完成 (2026-07-20) | `tmp/c9_4_STATUS.md` | 删 rasterize 文件 + 无引用 helper |
| C9.5 internal/gpu 死代码 | ✅ 完成 (2026-07-20) | `tmp/c9_5_STATUS.md` | 6 处 A；未碰 dual-tex/glyph 热路径；D36/SDF 基线红 |
| C9.6 webgpu/rwgpu | ✅ 完成 (2026-07-20) | `tmp/c9_6_STATUS.md` | 删 DebugMode/call4/floatsEqual；ABI/Release 未动 |
| C9.7 可选 B 合并 | ⏭ 跳过 | — | 无高置信同语义重复 |

---

## 8. 回滚条件（预设）

出现任一条立即回滚该刀：

1. 像素/组合/visual 门禁失败且非环境噪声  
2. 有 GPU 场景新增 `cpu_fallback_ops>0` 或 silent 路径  
3. present 主路径 p50 恶化 >10% 无合理解释  
4. ABI/生命周期 UAF、double-free、新增 mem leak / 稳态斜率恶化  
5. full_unit 逻辑失败（solo 亦失败）  
6. 公共 API 或默认行为被意外改变  
7. 为过绿而删测试/放宽断言  
8. 为迁就明显过时/特场景测试而改坏生产主路径  

---

## 9. 真源与工具索引

| 用途 | 路径 |
|------|------|
| **本任务卡（C9 权威）** | `docs/CODE_CONVERGENCE.md` |
| **实现真源（P0）** | `render/` · `gpu/webgpu/` · `gpu/rwgpu/` |
| **回归（分层）** | 触及包 `go test`；优先 T0/T1，T2 按场景，勿迷信全量每一条 |
| Full unit（可选加深） | `scripts/run_full_unit_tests.sh`（批次噪声/OOM 按现网策略） |
| Mem（资源/池/生命周期刀） | `scripts/run_mem_guard.sh` · `scripts/run_mem_leak_tests.sh` |
| 切片证据 | `tmp/c9_x_STATUS.md`（含：跑了哪些测、跳过哪些特场景、基线已有失败） |

不把其它 `docs/*` 计划文档列为 C9 依赖。  
不把「全仓测试 100% 绿」当作 C9 前提（除非该失败确由本刀引入）。

---

## 10. 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-20 | 首版：代码收敛任务卡 C9.0–C9.7；与 R7 性能刀分轨；硬不变量与 AI 执行规则 |
| 1.1 | 2026-07-20 | 明确文档可信度：关联 docs 可能与现网代码有出入；真源=代码+测试；只继承原则/门禁习惯 |
| 1.2 | 2026-07-20 | 去掉对其它 plan 文档的依赖引用；C9 自包含，真源仅代码+测试+本文 |
| 1.3 | 2026-07-20 | 测试分层 T0–T4：部分单测可能过时/特场景；不以全部测试为规格，禁止删测装绿 |

---

## 附录 A — 一页纸派发卡（复制即用）

```text
任务：C9.1（或指定切片）代码收敛
文档：docs/CODE_CONVERGENCE.md
范围：仅 <包列表>
类：A（纯结构等价）或 B（有测试锁的局部合并）

必须遵守：
- P1 优先级：正确性 > GPU 优先 > 无泄漏 > 性能不回退 > 可读性
- 不改 public API / 默认路由 / Release 时机
- 不删 fallback / device-lost / 测试专用 / build-tag 路径
- 一刀一范围；失败回滚
- 以代码+测试为准；不依赖其它 docs 计划文档

验收：
- 相关 T0/T1 门禁绿（与本刀相关）
- tmp/c9_x_STATUS.md：命令、删改摘要、特场景是否跑、基线已有失败
- 无性能/内存无故回退；不删测试装绿

非目标：
- 性能专项 / 能力扩展
- 控件层
- 跨层统一抽象
```

## 附录 B — 与“优化渲染引擎”表述的对照

| 模糊说法 | 本卡落地 |
|----------|----------|
| 优化渲染引擎 | **结构收敛**，不是能力扩展 |
| 代码收敛/去冗余 | 死代码 + 局部重复 helper；保留路由分路 |
| 可读性强 | 拆文件/命名；禁止热路径 interface 化 |
| 去除无用代码 | §4.2 确认后删除；不确定保留 |
| 不影响功能性能内存 | P1–P2 不变量 + L0–L3 门禁 |
