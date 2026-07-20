# 渲染引擎正向优化任务卡（CPU · GPU · 内存/资源）

> 版本：1.3 | 日期：2026-07-20  
> 状态：**可执行（稳优先 + 正向优化）**  
> 范围：`render` 主路径 + `gpu/webgpu` / `gpu/rwgpu` 热路径（**不含控件层**）  
> 调用链（约定，以代码 import 为准）：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> **本阶段产品焦点：先「稳」，再在稳上做 CPU/GPU/内存正向优化；不实现控件层。**

---

## 0. 一句话目标

在 **不减内容、不降像素语义、不破坏 GPU 优先、不恶化内存斜率** 的前提下，对渲染引擎做 **可证明的正向优化**：

| 维度 | 目标 |
|------|------|
| **CPU** | 主线程占用 ↓ 或持平；减少每帧无效 Go 工作 |
| **GPU** | 真 GPU 主路径更省：少 pass / submit / upload / readback / 无效绘制 |
| **内存 RSS** | 稳态斜率 ≈ 0；峰值不无故上升 |
| **GPU 资源（显存代理）** | 池化复用、帧末释放 ephemeral；创建次数/常驻资源不无界涨 |

**每一刀必须可证明：同场景下更快或更省或更稳，且功能不回退。**

---

## 0b. v1.0 是否足够？（结论）

| 问题 | 结论 |
|------|------|
| 只做 **CPU/GPU/内存正向刷分** | v1.0 流程 **基本够**（F10→一刀→门禁） |
| 本阶段目标是 **先把「稳」做好**，再正向优化 | v1.0 **不够**，缺：**稳的定义、场景全覆盖表、AI 强制自动套件、正确性/生命周期与 perf 的优先级** |
| 要 **AI 自动处理、覆盖全面准确** | 必须固定 **场景矩阵 + 每刀必跑/选跑命令 + JSON 判定 + 失败裁决**（见 §0c–§0f） |

**v1.1 补齐：** 稳优先不变量、场景族 S0–S7、示例/探针映射、AI 自动门禁协议、每刀报告模板。  
**仍不宣称：** 数学上 100% 路径覆盖（组合爆炸）；「全面」= **主路径 + 压力轴 + 生命周期 + 能力窗 + 内存** 的工程全覆盖，并持续用矩阵扩。

---

## 0c. 「稳」的定义（本阶段最高优先）

「稳」= 在 **固定 env、真 GPU** 下，用 **真窗口 + 对口单元测试双轨** 证明下列全部成立（见 §0d0）：

| ID | 稳维度 | 通过标准（工程） |
|----|--------|------------------|
| **ST1** | 不崩 | 探针/单测无 panic、无 native abort、进程正常退出 |
| **ST2** | GPU 优先诚实 | 有加速器时 `cpu_fallback_ops=0`，`GPUOps>0`（宣称 GPU 的路径） |
| **ST3** | 呈现可用 | present 无持续错误；`present_err` 稳态可接受（以探针 JSON 字段为准） |
| **ST4** | 无 silent 错画 | probe_ok / 像素指纹 / 组合抽样不因本刀变坏；禁止空内容装绿 |
| **ST5** | 生命周期 | resize / 重建 / device lost 路径不崩；资源可关闭 |
| **ST6** | 内存平台化 | soak 稳态 RSS 斜率 ≈ 0（rate 门内）；无快速泄漏 |
| **ST7** | 帧可预期 | 主路径 hitch/jitter 不因本刀显著恶化；轻场景可维持门禁帧率 |

**稳 vs 正向：**

```text
每刀：先 ST1–ST6（触及面）→ 再比 FPS/CPU（F6 正向）
ST 红 → 回滚；不得用「FPS 更高」掩盖不稳
```

**本阶段不做：** 控件层、设计系统、为对标而堆冷门 API。

---

## 0d0. 双轨验收（硬性）：真窗口 + 对口单元测试

本阶段 **禁止** 只用其中一轨宣称「稳」或「正向完成」。

| 轨 | 名称 | 必须覆盖 | 不能替代 |
|----|------|----------|----------|
| **U** | **对口单元测试** | 改动符号/包的语义、GPU 路由、`cpu_fb`、像素/组合、资源生命周期 API | 不能代替真 present、真 swapchain、真窗口 resize/lost、墙钟 fps |
| **W** | **真窗口** | X11（或现网窗口后端）+ `libwgpu_native` 呈现链；PKS / capability_matrix / device_lost 等 | 不能代替细粒度断言（组合维度、精确像素、API 契约） |

### 硬规则

1. **每刀 Keep 必须同时有 U + W 证据**（写在 SUMMARY：命令 + 结果）。  
2. **U**：至少「触及包全测」+ **与主因对口的** 专项测（见下表）；有语义改动则加 Comp/Capability/F1 抽样。  
3. **W**：须满足 **§0d1**（场景覆盖 + 时长档 + 窗口管理组合）；默认至少 `P_SOLID` + `P_L3` 达 **D20** 时长，并按改动选 WM 组合。  
4. **无 DISPLAY / 无 GPU**：不得宣称 W 轨通过；可跑 U 轨作开发中检，**合入前必须在有显示+GPU 环境补 W**。  
5. **新建场景示例**：必须能 **无头脚本化出 JSON**（W），并尽量补 **对口 `Test*`（U）**；禁止「只能人工看窗口」。  
6. **单元测试要对口**：禁止只跑无关大包装绿；禁止用「随便一个 TestMem」代替 filter 刀的 filter 测。

### 对口单元测试怎么选（U 轨映射）

| 本刀主因 / 改动目录 | 对口 U（名称以现网 `-list` 为准，优先匹配） |
|--------------------|-----------------------------------------------|
| `render/internal/gpu` flush/submit/session | 触及包测 + present/S6 抽样 + `TestF1_*`（layer/submit 相关） |
| dual-tex / advanced blend | `TestP03_*` / `TestF1_*` / 相关 Comp |
| filter / glow / export | `TestP04_*` / filter 对口 / present coherence |
| damage / scissor | damage 相关 `Test*Damage*` / R74 类若仍存在 |
| text / atlas / layout | `TestS65_*` / `TestP11_*` / text 对口 |
| clip / mask | `TestP12_*` / clip 对口 |
| webgpu/rwgpu facade | `./gpu/webgpu` `./gpu/rwgpu` 包测 + ABI/Release 相关 |
| 池 / texture / lifecycle | `TestMem_T*` + 触及包 |
| 纯 color/math helper | 包测即可；**W 仍要 P_SOLID 烟囱**（防链路误伤） |

查名：

```bash
go test ./render -list 'TestF1_|TestP0|TestP1_|TestS6|TestMem_' | head
```

### 真窗口怎么选（W 轨映射）

| 本刀主因 | 对口 W（在 Default 的 SOLID+L3 之外） |
|----------|--------------------------------------|
| 轻优化 / alloc | 可仅 SOLID+L3 |
| filter/glow | + `P_GLOW` 或 `P_BLEND_GLOW` 或 `P_FILTER_FLICKER` |
| 渐变 RT | + `P_GRAD_RT` 或 C06 |
| 文本 | + `P_TEXT` / `P_ATLAS_TEXT` 或 C10 |
| submit 风暴 | + `P_SUBMIT_PATH` |
| 组合 | + `P_COMBO_UI` |
| resize/surface | + `P_RESIZE`；device 路径 + `device_lost_redraw` |
| 能力语义 | + 对应 `Cxx`（capability_matrix） |
| 内存 | + `P_MEM_SOAK` 或 mem_guard（W/进程级） |

### 双轨都通过才 Keep

```text
Keep = U_pass AND W_pass AND ST(触及) AND F6(相对 baseline)
任一轨失败 → 回滚或修刀；禁止「U 绿就合」「W 帧率高就合」
```

---

## 0d1. 真窗口硬要求：场景覆盖 · 时长 · 窗口管理组合

W 轨 **不是**「随便开个窗口跑几秒」。必须同时满足：

1. **场景覆盖**（画什么）  
2. **测试时长**（跑多久才算数）  
3. **窗口管理 / 宿主组合**（窗口怎么被系统折腾）

三者缺一，不得宣称 W 轨通过。

### A. 时长档（`GPUI_ANIM_SECONDS` 或等价）

| 档 | 秒数 | 代号 | 用途 |
|----|------|------|------|
| **D8** | 8 | 烟囱 | 仅排障；**不得**单独作为 Keep 的 W 证据 |
| **D20** | **20** | **每刀默认** | 每刀 `P_SOLID` / `P_L3` / 主因对口探针的 **最低时长** |
| **D30** | 30 | 能力窗 | `capability_matrix` 单场景建议时长 |
| **D60** | 60 | 稳帧/短浸泡 | hitch/jitter 可信；`P_MEM_SOAK` 默认档 |
| **D180** | 180 | 日门 mem | `P_MEM_LONG` / mem daily 加深 |
| **D600+** | 600–1800 | 发布/深浸泡 | 仅阶段门或泄漏专项；非每刀 |

**硬规则：**

| 规则 | 说明 |
|------|------|
| R-T1 | 每刀 W 主证据（SOLID+L3+对口探针）时长 **≥ D20**，且 **与 baseline 完全相同**（改时长必须重冻 baseline） |
| R-T2 | 宣称 hitch/jitter 改善 → 该探针 **≥ D60**（或与 baseline 同为 D60+） |
| R-T3 | 宣称 mem 不回退且动资源 → 至少 **D60 soak**；阶段门 **D180+** |
| R-T4 | D8 只可出现在开发迭代日志，**SUMMARY 的 Keep 证据不得只有 D8** |
| R-T5 | 时长写进 JSON/SUMMARY；脚本必须传 `GPUI_ANIM_SECONDS`（或场景自带秒数并记录） |

```bash
# 每刀默认形态
GPUI_PROBE=P_SOLID GPUI_ANIM_SECONDS=20 GPUI_RESULT_FILE=$OUT/P_SOLID.json ...
GPUI_PROBE=P_L3    GPUI_ANIM_SECONDS=20 GPUI_RESULT_FILE=$OUT/P_L3.json ...
```

### B. 绘制场景覆盖（W-Scene 矩阵）

每刀 W 必须覆盖 **「默认集」**；再按主因加 **「对口集」**。阶段门跑 **「扩展集」**。

#### B1. 每刀默认集（必跑，均 ≥ D20）

| 场景 ID | 载体 | 覆盖意图 |
|---------|------|----------|
| **WS-SOLID** | `P_SOLID` | 轻主路径 present + 稳帧基线 |
| **WS-STACK** | `P_L3` | 叠压（层/混合/文本等档位压力） |

#### B2. 主因对口集（改到才跑，≥ D20；hitch 相关 ≥ D60）

| 主因 | 场景 ID | 载体 |
|------|---------|------|
| filter/glow | WS-GLOW / WS-FG | `P_GLOW` / `P_BLEND_GLOW` / `P_FILTER_FLICKER` |
| 渐变/RT | WS-GRAD | `P_GRAD_RT` 或 C06 |
| 文本/atlas | WS-TEXT | `P_TEXT` / `P_ATLAS_TEXT` 或 C10 |
| submit/路径风暴 | WS-SUBMIT | `P_SUBMIT_PATH` |
| 组合 UI | WS-COMBO | `P_COMBO_UI` |
| clip | WS-CLIP | `P_CLIP_NEST` 或 C05 |
| mesh/顶点 | WS-MESH | `P_MESH` / `P_MESH_WAVE` 或 C12 |
| image | WS-IMG | `P_IMAGE_PX` 或 C09 |
| blend 板 | WS-BLEND | `P_BLEND_SEP` / `P_BLEND_LAYER` 或 C07 |
| 能力语义 | WS-Cxx | `GPUI_SCENARIO=Cxx` capability_matrix |

#### B3. 阶段门 / F19 扩展集（抽样即可，须列清单）

| 集 | 内容 | 时长 |
|----|------|------|
| Cap-L0 | C01–C05 或脚本 L0 子集 | ≥ D30 / 场景 |
| Cap-关键 | C07/C08/C10/C11 等与近期改动相关 | ≥ D30 |
| PKS-core | `GPUI_PKS_FILTER=core` | 每探针 ≥ D20（矩阵可统一 SECONDS） |
| PKS-dig | 尖刺墙子集（可选） | ≥ D20，hitch 项 D60 |

**禁止：** 只用 WS-SOLID 就对 filter 刀宣称 W 全覆盖。

### C. 窗口管理 / 宿主使用场景组合（WM 矩阵）

绘制场景解决「画什么」；**WM 组合**解决「窗口被系统怎么用」。优化 present/surface/swapchain/device 时 **必跑**；其它刀至少跑 **WM-BASE**。

| 组合 ID | 场景 | 测什么 | 载体 / 做法 | 每刀 | 阶段门 |
|---------|------|--------|-------------|------|--------|
| **WM-BASE** | 稳定可见窗 | 映射后持续 present；不崩；`cpu_fb=0` | PKS/Cxx 正常前台 | **必跑**（含在 SOLID/L3） | ✓ |
| **WM-RESIZE** | 连续/振荡改尺寸 | configure 风暴、surface 重建、无持续 present_err | `P_RESIZE`；device_lost 内 free-resize | 动 surface/尺寸 **必跑** | ✓ |
| **WM-RESIZE-IDLE** | 拖拽松手后 idle 再锐化 | quiet configure → 一次 Resize+full present | `device_lost_redraw` 路径 | 动 resize 策略 **必跑** | ✓ |
| **WM-MINIMIZE** | 最小化 / Iconic | 暂停 acquire；恢复后可再画 | `device_lost_redraw`（IsIconic） | 动 presentable 标志 **必跑** | ✓ |
| **WM-UNMAP** | unmap / 再 map | 不可 present 时不狂报错；恢复可画 | device_lost / 窗口宿主 | 动 mapped 逻辑 **必跑** | 建议 |
| **WM-OBSCURE** | 完全遮挡 | obscured 时策略（暂停/降频）与恢复 | 宿主 obscured 标志 | 动遮挡逻辑 **必跑** | 建议 |
| **WM-FOCUS** | 失焦/获焦 | 焦点变化不破坏 swapchain | device_lost focus 字段 | 选跑 | 建议 |
| **WM-LOST** | device lost → 恢复 | AutoRecover 后继续帧；非 sticky 死 | `examples/device_lost_redraw` | 动 device/recover **必跑** | ✓ |
| **WM-DAMAGE** | 局部 damage present | 脏区 present 正确、不闪全黑 | C16 类 / damage 探针或单测+窗 | 动 damage **必跑** | ✓ |
| **WM-RECREATE** | 关窗再开 / 多次 CreateClose | 无泄漏、可再 present | `TestMem_T0/T5` + 短窗烟囱 | 动生命周期 **必跑** | ✓ |
| **WM-LONG** | 长前台 | 斜率、present 稳定 | D60/D180 SOLID 或 MEM | 资源刀 / 阶段门 | ✓ |

#### C1. 每刀 WM 最低要求

```text
所有刀：     WM-BASE（由 P_SOLID+P_L3 前台跑满 D20 覆盖）
动 surface / resize / present 策略： + WM-RESIZE（≥D20），建议 + WM-RESIZE-IDLE
动 device / recover / swapchain：     + WM-LOST（脚本化时长见下）
动 最小化/presentable：               + WM-MINIMIZE（或 UNMAP）
动 damage：                           + WM-DAMAGE
动 资源池生命周期：                   + WM-RECREATE 证据 + mem
阶段门 F19： WM-BASE + WM-RESIZE + WM-LOST + WM-LONG(D60+) + 绘制扩展集抽样
```

#### C2. 窗口管理示例自动化

| 示例 | 自动化要点 |
|------|------------|
| `examples/device_lost_redraw` | 固定跑 **≥ D60**（或 `GPUI_ANIM_SECONDS`）；exit 0；日志无持续 panic；可选 RESULT JSON |
| `P_RESIZE` | **≥ D20**；JSON：status、present_err、cpu_fb |
| `P_MEM_SOAK` | **≥ D60**；斜率门 |
| 缺 JSON 的示例 | AI 必须包一层：超时、exit code、关键日志 grep（device recovered / resized）写入 SUMMARY |

**若某 WM 组合现网还不能全自动点「最小化」：**  
允许用 **单元/集成测模拟 presentable 标志**（U 轨）+ **真窗口前台长跑**（W 轨）组合证明；并在 STATUS 标 `WM-MINIMIZE=simulated+host`；**有条件时补真 WM 自动化**。

#### C3. 组合测试（绘制 × 窗口管理）

阶段门与高风险刀应显式跑 **乘积子集**（不必笛卡尔全表）：

| 组合例 | 含义 |
|--------|------|
| WS-STACK × WM-BASE | 叠压前台 D20/D60 |
| WS-GLOW × WM-BASE | 特效前台 |
| WS-SOLID × WM-RESIZE | 轻负载下 resize |
| WS-STACK × WM-RESIZE | 重负载下 resize（更易炸） |
| 任意 × WM-LOST | 恢复后场景仍可画 |
| WS-COMBO × WM-BASE D60 | 组合稳帧 |

SUMMARY 中写成：`W_scenes=[...]` + `W_wm=[...]` + `W_seconds=N`。

### D. W 轨 SUMMARY 必填字段

```markdown
## W 轨
- seconds: 20（与 baseline 一致）
- scenes: WS-SOLID, WS-STACK, （对口…）
- wm: WM-BASE, （WM-RESIZE…）
- commands: ...
- json: fps/cpu/cpu_fb/hitch/present_err/rss
- result: PASS/FAIL
```

缺 `seconds` / `scenes` / `wm` 任一 → 门禁视为 **W 证据不全**。

---

## 0d. 覆盖场景族（优化与回归都按族开）

场景可 **单独示例进程** 跑（已有则复用，缺则再加 example），但 **验收必须自动化**（JSON/exit code/`go test`）。

### 族总表

| 族 | 名称 | 覆盖意图 | 主载体（现网） | AI 默认 |
|----|------|----------|----------------|---------|
| **S0** | 烟囱/编译 | 包能编、基础单测 | `go test` 触及包 | **每刀必跑** |
| **S1** | 轻主路径 60 | solid/轻负载稳帧 | PKS `P_SOLID`、`P_L0` | **每刀 L2 必跑** |
| **S2** | UI 叠压 | 多层/混合/文本叠压 CPU+GPU | `P_L3`（及 `P_L2`） | **每刀 L2 必跑**（叠压刀加深） |
| **S3** | 能力窗 Present | Skia 向画布能力真窗口 | `examples/capability_matrix` C01–C20+ | **阶段门 / 动语义必跑子集** |
| **S4** | 组合与尖刺 | filter/glow/grad/combo hitch | `P_BLEND_GLOW` `P_GLOW` `P_GRAD_RT` `P_FILTER_*` `P_COMBO_UI` `P_FPS_JIT` | **按 pprof/痛点选跑** |
| **S5** | 正确性锁 | 像素/advanced layer/组合 | `go test`：`TestF1_*` `TestP1_Comp_*` 抽样 `TestP1_Capability_*` 对口 | **动绘制必跑** |
| **S6** | 内存/资源 | RSS 斜率、创建关闭、尺寸 churn | `run_mem_guard.sh`；`P_MEM_SOAK`/`P_MEM_LONG`；`TestMem_*` | **资源刀必跑；阶段门加深** |
| **S7** | 生命周期/恢复 | resize、重建、device lost | `P_RESIZE`；`examples/device_lost_redraw`；窗口 mem 测 | **动 surface/device 必跑** |

### S1–S2 主路径探针（性能+稳）

| ID | 测什么 | 自动判定（字段以 JSON 为准） |
|----|--------|------------------------------|
| `P_SOLID` | 轻实心主路径 | status PASS；`cpu_fb=0`；fps 过门；无 present 持续失败 |
| `P_L0`/`P_L1` | 档位基线 | 同上 |
| `P_L3` | 叠压主战场 | fps/cpu/hitch；`cpu_fb=0` |
| `P_L4` | 更重档（选跑） | 压力；不作为唯一 Keep 依据时可降权 |

### S4 压力/质量轴（按需；矩阵脚本已分组）

| 过滤 | 脚本 | 典型探针 |
|------|------|----------|
| gate | `GPUI_PKS_FILTER=gate` | 主轴门禁类 |
| core | `=core` | 日常墙 + 关键 stress |
| combo | `=combo` | 组合压力 |
| dig | `=dig` | 尖刺/质量墙（hitch/jitter/combo） |
| mem | `=mem` | 浸泡类 |

**单探针：**

```bash
GPUI_PROBE=P_SOLID GPUI_ANIM_SECONDS=20 GPUI_RESULT_FILE=.../P_SOLID.json \
  go run ./examples/particle_kitchen_sink
# 或 scripts/run_pks_matrix.sh / 已构建的 pks_bin
```

### S3 能力窗场景（功能稳 + GPU-first）

| ID | 场景 | 应锁 |
|----|------|------|
| C01 | 清屏/present | 窗口链通 |
| C02 | 变换栈 | T 路径 |
| C03–C04 | path/stroke/dash | 几何 |
| C05 | clip | 裁剪 |
| C06 | 渐变/pattern | 填充 |
| C07 | blend | 混合 |
| C08–C09 | layer/image | 层与图 |
| C10 | 中英文本 | 文本 |
| C11 | filter | 滤镜 |
| C12–C15 | mesh/evenodd/mask/backdrop | 合成扩展 |
| C16+ | 矩阵扩展 | 以 `scenarios.go` 现网为准 |

```bash
GPUI_SCENARIO=C01 GPUI_ANIM_SECONDS=30 GPUI_RESULT_FILE=.../C01.json \
  go run ./examples/capability_matrix
# 批量：scripts/run_capability_matrix.sh
```

**判定：** `cpu_fallback_ops=0`、`gpu_ops>0`、`probe_ok`、fps 门（场景自带 AllowLowFPS 除外）。

### S5 单测正确性（无窗口也可锁语义）

| 套 | 命令意图 | 何时 |
|----|----------|------|
| 触及包 | `go test -count=1 ./render/internal/gpu -timeout ...` | 每刀 L0 |
| Present/帧 | 现网 `TestS6*` / present 相关（`-list` 确认） | 动 flush/present |
| Advanced layer | `TestF1_*` | 动 layer/blend/submit |
| 组合抽样 | `TestP1_Comp_D01|D06|D08|D36|...` | 动绘制语义 |
| 能力对口 | `TestP1_Capability_*` 与改动相关 | 动该能力 |
| Mem 单测 | `TestMem_T0`… / `scripts/run_mem_leak_tests.sh` | 资源/生命周期 |

**名称以现网 `go test -list` 为准**；过时名写进 STATUS 并换能跑通的。

### S6 内存

| 档 | 命令 | 何时 |
|----|------|------|
| quick | `./scripts/run_mem_guard.sh quick` | 资源刀；F10 基线 |
| daily | `./scripts/run_mem_guard.sh daily` | 阶段门 / F19 |
| 探针 | `P_MEM_SOAK` / `P_MEM_LONG` | 矩阵 mem 过滤 |

**判定：** 稳态斜率/rate，不是绝对涨了多少 MiB。

### S7 生命周期

| 场景 | 载体 | 测什么 |
|------|------|--------|
| 尺寸振荡 | `P_RESIZE` | 重建不崩、err 可控 |
| Device lost 重绘 | `examples/device_lost_redraw` | 丢失后恢复可画 |
| 窗口复杂 churn | `TestMem_T4_*` 等 | 窗口路径资源 |
| 离屏尺寸 churn | `TestMem_T3_*` | 离屏 RT |

### 可选独立示例（可加、须可脚本化）

| 示例目录 | 用途 | 自动化要求 |
|----------|------|------------|
| `particle_kitchen_sink` | 性能/压力/mem 主探针 | **已有** JSON + matrix |
| `capability_matrix` | 能力 present | **已有** JSON + script |
| `device_lost_redraw` | 恢复 | 退出码 + 日志/可选 JSON |
| `window_present` | present 烟囱 | 短跑 PASS |
| `mem_window_stress` / `mem_anim_window` | 窗口 mem（历史/加深） | 默认不挡每刀；阶段门可选 |
| **新建** `examples/perf_scenarios/<id>` | 只当 PKS/C 不够表达某热路径 | 必须：`GPUI_RESULT_FILE` JSON + 文档一行判定 |

**禁止** 只靠人工盯屏当 Keep 依据；人工只作辅助。

---

## 0e. AI 自动处理协议（每刀强制）

执行者（人/AI）**不得跳过**下列自动化，除非 STATUS 写明「未触及 + 跳过理由」。

### 每刀最低套件（Default Gate）= **U 轨 + W 轨**

```text
G0  钉 env（P6）；确认 DISPLAY + WGPU_NATIVE_PATH（W 轨前置）
── U 轨（对口单元测试）──
G1  触及包：go test -count=1 ./<changed_pkgs>/...
G2  对口专项：按 §0d0 映射选 Test*（F1/P0x/P1x/S6x/Mem…）；无语义改动可窄但不能为零（至少 G1）
G3  记录：失败是本刀引入还是 baseline_known
── W 轨（真窗口：场景 + 时长 + WM）──
G4  时长：主证据 ≥ D20，且与 baseline 相同（§0d1-A）；写入 SUMMARY
G5  场景：WS-SOLID + WS-STACK 必跑；主因对口 WS-*（§0d1-B）
G6  WM：WM-BASE 必含；按改动加 WM-RESIZE/LOST/MINIMIZE/DAMAGE…（§0d1-C）
G7  JSON：cpu_fb=0；fps/cpu/hitch/present_err；对比 baseline（F6）
── 条件 ──
G8  动池/RT/Release → mem quick + 时长档 D60 视深度
G9  动 surface/device/presentable → 对应 WM 组合真窗口
G10 SUMMARY：U 列表 + W 的 scenes/wm/seconds/commands/json + ST + F6
```

**缺 U、或缺 W 场景/时长/WM 字段 → 不得 Keep。**

### 按主因加跑（Add-on）

| 主因 | 加跑 |
|------|------|
| filter/glow/readback | `P_GLOW`/`P_BLEND_GLOW`/`P_FILTER_FLICKER`/`P_GRAD_RT` 选相关 |
| submit/encoder | `P_SUBMIT_PATH` + `TestF1_*` |
| 文本/atlas | `P_TEXT`/`P_ATLAS_TEXT` + text 对口测 |
| damage | damage 单测 + present 抽样 |
| 组合 UI | `P_COMBO_UI` + 可选 `GPUI_PKS_FILTER=dig` 子集 |
| 阶段关闭 F19 | `run_pks_matrix.sh` core 或 problem_suite 子集 + mem daily + C01–C05 或 L0 cap 子集 |

### AI 选场景算法（无人工点菜时）

```text
1. 读本刀 diff 路径 → 映射到子系统（flush/submit/filter/text/mem/surface…）
2. Default Gate 必跑
3. 子系统 → Add-on 表
4. F10 归因 top 探针若未包含 → 加入 L2
5. 全量 all PKS / 全量 unit：仅 F19 或发布门；不作每刀默认（太慢且噪声）
```

### 结果判定（机器可读）

| 结果 | 动作 |
|------|------|
| 相关 ST/T0 红且为本刀引入 | **回滚** |
| 基线已有失败（F10 已记录）且未恶化 | STATUS 标注 **baseline_known**，不挡 Keep |
| FPS 掉 >3% 或 CPU 明显升 | 复跑 1 次 → 仍差则回滚 |
| 仅 T2 环境失败（无 DISPLAY 等） | 不得在无显示环境宣称窗口门禁通过；有 DISPLAY 的 CI/本机必须跑 |
| 探针名/测试名不存在 | `go test -list` / `GPUI_LIST_PROBES=1` 校正后重跑，禁止装绿跳过 |

### 每刀 SUMMARY 必须含

```markdown
- commit / env
- 主因
- 命令列表 + exit code
- ST1–ST7 勾选（触及的）
- P_SOLID / P_L3 对比表
- cpu_fb
- Keep/回滚
```

---

## 0f. 「全面准确」的边界（诚实范围）

| 承诺 | 不承诺 |
|------|--------|
| 主路径 + 叠压 + 能力窗抽样 + mem + 生命周期的 **工程全覆盖** | 所有 clip×blend×filter×text 组合穷尽 |
| 每刀可复现 JSON/测试证据 | 单次 run 无噪声（允许复测 2 次） |
| 矩阵可扩展（新 example 必须可脚本化） | 旧文档场景表与现网 100% 一致（以代码探针列表为准） |
| AI 按协议自动选跑 | 无 DISPLAY/无 GPU 机器上的真窗口结果 |

**准确** 靠：固定 env、同探针对比、cpu_fb、probe_ok、斜率，而不是感觉。

---

## 1. 本文自包含与真源

本卡 **不依赖** 其它历史 plan 文档的进度表或文件级实现描述（那些可能与现网代码不一致）。  
需要时只继承 **原则**；实现与数字以现网为准。

| 优先级 | 真源 |
|--------|------|
| **P0** | **现网生产代码** + **本机测量**（探针 JSON / pprof / mem 脚本输出） |
| **P1** | **本文**（流程、禁止清单、切片、门禁） |
| **P2** | **测试**（分层使用，见 §3；特场景/过时断言不单决定合入） |

冲突时：**代码行为 + 本机对比数据** 胜出。  
禁止按旧文档“热点文件表”盲目改代码。

与 **结构收敛（C9）** 关系：

- C9 = 死代码/可读性；**本卡 = 性能/资源正向**
- **分 PR / 分刀**；禁止同一 diff 大删代码又改热路径算法

---

## 2. 硬原则（违反即不得合入）

### P1 — 优先级（冲突时按序）

1. **正确性 / 像素语义**  
2. **GPU 优先 + fallback 可观测**（有加速器：`GPUOps>0` 且 `cpu_fallback_ops=0`，禁止 silent CPU）  
3. **资源生命周期 / 无新增泄漏**（RSS 斜率不恶化）  
4. **性能正向**（FPS/CPU/hitch 相对本轮基线）  
5. **代码整洁**（本卡不追求删行数）

### P2 — 正向不变量

| ID | 不变量 | 验收 |
|----|--------|------|
| F1 | 不减场景内容、不关 AA/特效装绿 | 探针配置与基线相同 |
| F2 | 有 GPU 时 `cpu_fallback_ops=0` | stats / 既有 GPU 断言 |
| F3 | 像素与组合语义不退 | 相关 T0/T1 + 必要组合抽样 |
| F4 | 公共 API / ABI / Release 时机不漂 | 编译 + 触及 facade 测 |
| F5 | RSS 稳态斜率不显著变差 | mem guard / soak JSON |
| F6 | 相对 **本轮 baseline**：目标探针 `fps_ema` ≥ **97%** 且仍过硬门；`cpu` **≤ 基线**（噪声内持平可过）；hitch/jitter 不恶化 | 同机同 env 对比 JSON |

### P3 — 优化类别

| 类 | 定义 | 默认 |
|----|------|------|
| **A. 纯等价** | 同输入同像素；少 alloc/拷贝/bind/draw/submit/readback | **主通道** |
| **B. 算法等价** | 实现不同，目标像素一致 | 需加强像素/组合门禁 |
| **C. 语义/产品变更** | 改默认质量、API、能力 | **禁止混进本卡** |

### P4 — 小步

1. **一刀一主因**（一个 pprof 热点或一个探针痛点）  
2. 冻基线 → 最小 diff → 门禁 → Keep/回滚  
3. 无收益或正确性/mem 风险 → **回滚优先**  
4. 单 PR 建议聚焦单一子系统；禁止“顺手重构”  
5. 证据写入 `tmp/perf_fwd_<date>/`（见 §6）

### P5 — 分层不打穿

- `render` 不直接依赖 `gpu/rwgpu`  
- facade 优化在 `webgpu`；FFI 装箱在 `rwgpu`  
- 禁止为刷分把 native 细节泄漏进公共 API  

### P6 — 环境钉死（数字不可比则刀无效）

```bash
# 按本机实际 Go 路径调整；原则：固定 toolchain + native + display
export GOROOT=...   # 本机 go1.25.x
export PATH=$GOROOT/bin:$PATH
export GOCACHE=$PWD/tmp/go-cache GOWORK=off GOTOOLCHAIN=local
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
export DISPLAY=:1
export GPUI_SURFACE_SAMPLE_COUNT=1
```

性能对比必须：**同机、同 env、同探针名、同时长/warmup、同场景参数**。  
改探针定义则 **bump baseline 版本**，不得与旧 JSON 混比。

---

## 3. 测试与测量可信度

### 3.1 测试分层

| 层 | 用途 |
|----|------|
| **T0** | 主路径 / GPU 路由 / present 相关；本刀触及则必跑 |
| **T1** | 同包对口单测 |
| **T2** | 特场景（visual 门控、X11、device lost、长 soak）— **触及才跑** |
| **T3** | 疑似过时 — 不迁就改生产装绿，不删测过刀 |
| **T4** | 噪声性能断言 — 不作唯一依据 |

### 3.2 测量（性能真源）

| 手段 | 用途 |
|------|------|
| 真窗口探针 JSON（fps_ema / cpu / hitch / jitter / cpu_fb / rss） | **墙钟与产品体感** |
| `pprof` CPU（同探针） | **归因 Go 可动 vs cgo 墙** |
| mem guard / soak | **RSS 斜率护栏** |
| 资源代理（可选日志）：每帧 CreateTexture/Buffer、pool 命中 | **显存感/资源膨胀** |

**精确 VRAM 字节会计** 若现网无稳定 API：**不作为本卡硬门**；用创建次数、池、soak、OOM/device-lost 作代理。

### 3.3 冲突裁决

1. 正确性红（相关 T0/T1）→ 回滚  
2. `cpu_fb` 上升或 silent CPU → 回滚  
3. FPS 掉 >3% 或 CPU 明显变差 → 回滚或书面噪声复测 2 次  
4. RSS 斜率恶化 → 回滚  
5. 仅 T2/T3 红 → STATUS 记录；不扩大改生产迁就  
6. pprof 显示纯 cgo/native 墙且 Go 侧已无 ≥3% 可动热点 → 标 **平台期**，停空卷  

---

## 4. 三维优化指南（怎么动手）

### 4.1 CPU

**看：** 主线程 `cpu%`、pprof 中 Go 符号 cum。  
**做（A 类优先）：**

- 每帧 `make`/装箱 → scratch、pool、小 N 栈  
- 重复 shape/layout/几何 → cache / sticky（键语义不变）  
- mesh/uniform pack、memmove → 微优化须 bit 或像素门禁  
- 多余 ensure / 描述符重建 → 热路径复用  

**不做：** 把 GPU 活改回 CPU；为省 CPU 降内容。

### 4.2 GPU

**看：** `GPUOps`/`cpu_fb`、Submit/pass 次数、glow/filter/combo 探针 hitch 与 fps。  
**做（A/B 谨慎）：**

- 减 **readback** / 错误 export 尖刺  
- RT / bind group / pipeline 描述复用  
- 同帧可合并的 CB（**有正确性锁**；禁止无验证的 full-frame singleSubmit）  
- batch / atlas / path-image cache **增量**（有数据再动）  
- damage/scissor 收窄无效绘制  

**不做：** 降级已有 GPU 主路径；silent fallback；用 GPU\* blit 冒充“原生完成”却改记账。

### 4.3 内存 RSS 与 GPU 资源

**RSS：**

- 指标：warmup 后斜率 ≈ 0（plateau rate / steady delta）  
- 修：泄漏引用、未 Release、每帧切片、无界 map  
- 入口：`scripts/run_mem_guard.sh`（quick / daily）  

**GPU 资源（显存代理）：**

- 少每帧 Create*；提高 pool/atlas 命中  
- ephemeral 帧末释放；缓存设上限  
- filter/layer 常驻 RT vs 每帧创建要有数据支持  
- 动生命周期 → **必跑 mem**；必要时加长 soak  

---

## 5. 每刀流程（checklist）

```text
[1] 选因     基线 JSON + pprof top；写清主因一句话
[2] 定刀     类 A 或 B；文件列表；禁止项自检
[3] 冻基线   若尚无本轮目录则先 F10；否则复用当日 baseline
[4] 最小 diff 只碰主因路径
[5] 稳+正确性  §0e Default Gate（ST1–ST6 触及面）；cpu_fb=0
[6] 性能       同探针对比 baseline（F6）→ Keep/回滚
[7] 内存       动池/RT/Release/缓存 → mem quick（加深按需）
[8] 证据       tmp/perf_fwd_<date>/optN/ + §9 状态表；命令与 JSON 齐
```

### 5.1 Keep 标准（同时满足）

- 相关正确性门禁绿（或仅登记 **基线已有** 且与本刀无关的失败）  
- F2–F6 满足  
- STATUS 写明：改动摘要、对比表、pprof 前后（若本刀宣称 CPU 结构变化）  

### 5.2 回滚条件（任一条）

1. 像素/组合回归  
2. `cpu_fallback_ops>0` 或 silent CPU  
3. 目标探针 fps 相对 baseline 掉 >3% 且复测确认  
4. CPU 明显变差（超出噪声）  
5. hitch/jitter 显著恶化  
6. RSS 斜率/泄漏信号变差  
7. ABI/UAF/double-free / device lost 新路径  
8. 为过绿删测试、放宽断言、减内容  

---

## 6. 基线与证据目录约定

```text
tmp/perf_fwd_YYYYMMDD/
  BASELINE.md                 # env、探针、时长、git 短 hash
  baseline/
    P_SOLID.json              # 或现网探针实际文件名
    P_L3.json
    <pain_probe>.json         # 可选痛点
    pl3.pprof                 # 或等价
  optN/
    SUMMARY.md                # Keep/回滚、对比表、文件列表
    *.json
    *.pprof                   # 可选
  CLOSEOUT.md                 # 阶段结束时
```

`BASELINE.md` 最少字段：日期、commit、Go/WGPU 路径、DISPLAY、探针列表与秒数、硬门数值。

---

## 7. 门禁分级

| 级 | 何时 | 内容（示意，以现网能跑通的命令为准） |
|----|------|----------------------------------------|
| **L0（U）** | 每刀 | 触及包 `go test -count=1 ./<pkg>/...` |
| **L1（U 对口）** | 每刀 | 按 §0d0 映射的专项 `Test*`（路由/F1/P0x/Comp…）；`cpu_fb=0` 类断言 |
| **L1b（U 组合）** | 动绘制语义 | 抽样 `TestP1_Comp_*` / Capability |
| **L2（W 真窗口）** | **每刀必做** | `P_SOLID`+`P_L3` JSON 对比 baseline；主因加对口探针 |
| **L3 mem** | 池/生命周期 | `run_mem_guard.sh quick` + 相关 `TestMem_*` |
| **L4 问题墙** | 尖刺/阶段门 | PKS filter / problem_suite 子集 |

**L0+L1 与 L2 同时满足才算双轨通过。**

**命令名以仓库现状为准**：文档中的测试名若过时，用 `go test -list` 与现网脚本替换，并在 STATUS 记录实际命令。

### 7.1 推荐探针集（可裁剪）

| 探针意图 | 典型用途 |
|----------|----------|
| 轻主路径 | 近 60 基线、回归面小（如 P_SOLID） |
| UI 叠压 | CPU/提交主战场（如 P_L3） |
| 痛点 | glow/filter/combo/grad RT 等 **本机重跑后仍差** 的一个 |
| Mem | P_MEM_SOAK 等（护栏，非刷 FPS） |

具体二进制/包路径以 `examples/particle_kitchen_sink` 或现网 harness 为准。

---

## 8. 切片计划（F1x）

### F10 — 冻基线 + 归因 + 稳基线（第 0 刀，可无代码 diff）

**做：**

1. 钉 env（§2 P6）  
2. **S1+S2：** `P_SOLID` + `P_L3`（**D20 强制**，建议另存 D60 作 jitter 参考）；存 JSON  
3. **S6：** `run_mem_guard.sh quick`（或至少 `P_MEM_SOAK`）  
4. **U 轨基线：** 触及核心包 + 对口抽样；记录「基线已有失败」测试名  
4b. **W 轨基线：** 真窗口 JSON（与上 SOLID/L3 同一套）  
5. 对 `P_L3`（或最差探针）打 CPU pprof  
6. 可选：`GPUI_PKS_FILTER=core` 短秒矩阵，建立问题清单  
7. 写 `tmp/perf_fwd_<date>/BASELINE.md` + **ST 基线** + 归因 top5（Go 可动 vs cgo 墙）  

**退出：** 有可比 perf/mem 基线；已知失败列表；下一刀主因一句话。  
**不要求**本刀改生产代码。

---

### F11 — CPU 等价：减每帧 alloc / 描述符 / ensure

**当：** pprof 显示 Go 侧 ensure、desc 构建、小对象 alloc 靠前。  
**做：** 复用 scratch/pool/静态 desc；**不改** GPU 命令语义。  
**验收：** L0 + L1 抽样 + L2 对比；CPU↓ 或持平且 FPS 不掉。  

---

### F12 — GPU 上传 / Write* 路径

**当：** WriteBuffer/WriteTexture/pack 靠前。  
**做：** 合并上传、sticky 资源、减少重复 upload；键与失效语义不变。  
**验收：** L1 + L2；动画/网格场景不像素回归。  
**禁：** 错误复用已 Release 句柄。

---

### F13 — Submit / pass / encoder 合并（高风险）

**当：** Finish/Submit 次数驱动墙钟，且有明确可合并点。  
**做：** 同帧合法合并；**先**有对口正确性测。  
**禁：** 无验证默认 full-frame singleSubmit（历史像素回归风险）。  
**验收：** L1 加强 advanced layer/blend + L2；任何像素疑点 → 回滚。

---

### F14 — filter / glow / readback 尖刺

**当：** hitch 高或 filter export 探针差。  
**做：** 减 readback、RT 复用、避免 stale 路径错误恢复导致的额外同步。  
**验收：** L1 filter 对口 + L2 痛点探针 hitch↓；L3 mem 若动 RT。  

---

### F15 — damage / scissor / 无效绘制

**当：** 大屏局部动画仍接近全屏工。  
**做：** 收窄 damage 相关 scissor/blit；**语义：** 无 damage 行为与改前一致。  
**验收：** damage 单测 + present 抽样 + L2。  

---

### F16 — 文本 / atlas / layout 热路径

**当：** 文本滚动/多 run 场景 CPU 高。  
**做：** 缓存命中、少 reshape、上传合并；**不改**字形像素。  
**验收：** text 对口测 + L2（含文本场景若有）。  

---

### F17 — 资源池 / 缓存上限 / 生命周期

**当：** 创建次数涨、soak 斜率差、可疑泄漏。  
**做：** 池命中、上限淘汰、Release 次序、ephemeral 帧末释放。  
**验收：** **L3 mem 必跑**；L0/L1；性能 L2 不显著回退。  
**本切片优先修对，再谈 FPS。**

---

### F18 — 组合重场景（combo / 多层）

**当：** combo UI 或 L 重层 fps 明显低于轻路径。  
**做：** 仅针对 pprof/统计指向的子路径；禁止减特效装绿。  
**验收：** L1b + L2 痛点；可选 L4。  

---

### F19 — 阶段收口

**做：**

- 汇总 Keep 刀与收益表  
- 重跑 baseline 同探针确认仍正向  
- mem daily 或 quick（若阶段内动过资源）  
- 写 `CLOSEOUT.md`：平台期与否、残留热点、明确 **停** 或 **仅保留清单**  

**退出：** 连续 2 刀无收益 **或** 目标探针达到你设定的产品线 **或** 主因已是不可动 cgo 墙。

---

### 8.1 建议执行顺序

```text
F10 基线与归因
  → 按 pprof 选 F11 / F12 / F14 / F15 / F16 之一（低风险 A）
  → 有数据再 F13 / F18（更高风险）
  → 斜率/资源问题插入 F17（可提前）
  → F19 收口
```

**不规定必须跑满 F11–F18。** 无对应热点则跳过。  
与稳定性 / device lost 主线冲突时：**本卡让路**。

---

## 9. 执行状态

| 切片 | 状态 | 证据 | 备注 |
|------|------|------|------|
| F10 基线+归因 | ✅ 完成 | `tmp/perf_fwd_20260720/BASELINE.md` | 2026-07-20；已知 T4 OOM / S68 reconfig |
| F11 CPU alloc/desc | ↩️ 回滚 | `opt1/SUMMARY.md` | packMesh re-inline 无收益 |
| F12 上传 Write* | ⏭ 跳过 | — | WriteBuffer 多为 cgo 墙 |
| F13 submit/pass | ⏭ 跳过 | — | 高风险；无验证 singleSubmit |
| F14 filter/glow/readback | ✅ Keep | `opt2/SUMMARY.md` | effect FlushGPU 零读回；GRAD -6.8cpu COMBO -5.2cpu |
| F15 damage | ⏭ 跳过 | — | 本轮 pprof 无主因 |
| F16 text/atlas | ⏭ 跳过 | — | 次要 |
| F17 资源/生命周期 | ⏭ 跳过 | — | T4 baseline_known OOM；未动池 |
| F18 重场景 combo | 部分 | opt2 含 COMBO | 随 F14 收割 |
| F19 收口 | ✅ 完成 | `tmp/perf_fwd_20260720/CLOSEOUT.md` | 平台期：cgo≈50% |

---

## 10. 明确禁止清单

| # | 禁止 |
|---|------|
| X1 | 减粒子/内容、关 AA、降分辨率刷 FPS |
| X2 | silent CPU 或去掉 fallback 可观测 |
| X3 | 有 GPU 仍把已 GPU 主路径改回 CPU |
| X4 | 无像素门禁的 blend/AA/coverage“巧改” |
| X5 | 无验证的 full-frame singleSubmit 默认开 |
| X6 | 无界缓存换帧率导致 RSS 爬升 |
| X7 | 与 C9 大删代码、改 public API 混 PR |
| X8 | 只跑 unit 不跑真窗口就宣称完成 |
| X9 | 用过时 plan 的文件表代替本机 pprof |
| X10 | 删测试/放宽断言/关 STRICT 装绿 |
| X11 | 精确 VRAM 数字不存在时伪造“显存优化完成” |
| X12 | 平台期仍空卷无 ≥3% 可动热点 |
| X13 | **只跑单元测试、不跑真窗口** 就宣称稳/正向完成 |
| X14 | **只跑真窗口、无对口单元测试** 就宣称语义不回退 |
| X15 | 用无关测试/无关探针冒充「对口」 |
| X16 | 真窗口 **无场景清单** 或只用 D8 烟囱冒充 Keep |
| X17 | 真窗口 **未声明时长** 或与 baseline 时长不一致却比分 |
| X18 | 动 surface/present 却 **不跑 WM-RESIZE/LOST 等窗口管理组合** |

---

## 11. AI / 执行者规则（可当 task prompt）

```text
你在 gpui 执行 docs/PERF_ENGINE_FORWARD.md 的 F1x 切片。
本阶段：先稳（ST1–ST7）再正向（F6）。不含控件层。

硬约束：
- ST 优先于 FPS；ST 红不得 Keep
- 功能/像素、GPU 优先、cpu_fb=0、API/生命周期不回退
- 相对本轮 baseline：FPS≥97% 且过硬门；CPU≤基线；hitch 不恶化；RSS 斜率不坏
- 一刀一主因；class-A 优先；禁止减内容装绿
- 真源：代码 + 本机 JSON/pprof/mem；不跟历史 plan 实现表
- 严格遵守 §0e；**双轨 U+W 强制**（§0d0）；场景按 §0d；不删测装绿

每刀必须：
1) 声明 F1x、主因、文件列表、U 测试列表、W 场景列表、WM 组合、时长档
2) 最小 diff
3) §0e Default Gate（U + W 场景/时长/WM）+ Add-on
4) SUMMARY：U/W 双轨 + scenes/wm/seconds + ST + F6
5) 缺一轨、缺时长或缺 WM 要求则回滚

工作目录：gpui；证据写 tmp/；示例可 go run。
```

### 11.1 单刀记录模板

```markdown
## F1x / optN — <标题>
- 主因（pprof/探针）：...
- 类：A / B
- 文件：...
- U 轨：触及包 [ ] 对口 Test* [ ] 列表：...
- W 轨 scenes: WS-SOLID [ ] WS-STACK [ ] 对口: ...
- W 轨 wm: WM-BASE [ ] 其它: ...
- W 轨 seconds: 20（与 baseline 一致）
- ST / cpu_fb
- 性能：baseline vs after（fps/cpu/hitch）
- 内存：quick [ ] 跳过原因
- 结论：Keep / 回滚（须 U∧W）
- 证据目录：tmp/perf_fwd_.../optN/
```

---

## 12. 真源与工具索引

| 用途 | 路径 |
|------|------|
| **本任务卡** | `docs/PERF_ENGINE_FORWARD.md` |
| **实现** | `render/` · `gpu/webgpu/` · `gpu/rwgpu/` |
| **证据** | `tmp/perf_fwd_YYYYMMDD/` |
| **Mem 脚本** | `scripts/run_mem_guard.sh` · `scripts/run_mem_leak_tests.sh` |
| **问题挖掘（可选）** | `scripts/run_problem_suite.sh` · `examples/particle_kitchen_sink/` |
| **结构收敛（分轨）** | `docs/CODE_CONVERGENCE.md`（不跟做其切片作 perf） |

---

## 13. 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-20 | 首版：CPU/GPU/内存·资源正向优化自包含任务卡；F10–F19；测量驱动、与 C9 分轨 |
| 1.1 | 2026-07-20 | 稳优先：ST1–ST7；场景族 S0–S7；AI 自动门禁 §0e；F10 含稳基线；明确 v1.0 不足点 |
| 1.2 | 2026-07-20 | 硬性双轨：真窗口 + 对口单元测试（§0d0）；Default Gate 拆 U/W；禁止单轨宣称完成 |
| 1.3 | 2026-07-20 | W 轨强制：场景覆盖 + 时长档 D8–D600 + 窗口管理组合 WM-*（§0d1） |

---

## 附录 A — 一页纸派发卡

```text
任务：F10 或 pprof 指定的 F1x
文档：docs/PERF_ENGINE_FORWARD.md v1.1
焦点：先稳后正向；无控件

必须：
- §0e：U 对口单测 + W 真窗口（场景×时长×WM）
- ST 红则回滚；缺 scenes/seconds/wm 不得 Keep
- 每刀 W ≥ D20 与 baseline 同时长
- 不减内容、cpu_fb=0、RSS 斜率不坏

验收：
- U 对口 Test 绿
- W：WS-SOLID+WS-STACK(+对口) + WM-BASE(+对口) + 时长达标 + F6
- 资源刀 mem；动 surface 则 WM-RESIZE/LOST

非目标：
- 控件层
- 穷尽组合 100%
- 无 JSON 的人工瞄屏 Keep
```

## 附录 B — 与模糊说法对照

| 说法 | 本卡 |
|------|------|
| 功能优化 | 语义不变更快更省；不是加功能 |
| CPU 优化 | pprof + 主线程 cpu% |
| GPU 优化 | 真 GPU 路径减工；不是改回 CPU |
| 内存/现存 | RSS 斜率 + 资源创建/池代理 |
| 优化引擎 | F10 起按刀推进，可停于平台期 |
