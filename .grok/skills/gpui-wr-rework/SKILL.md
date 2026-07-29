---
name: gpui-wr-rework
description: §R 真窗的**反攻/修复回流**——把 metrics-audit 审出的指标装绿（fps 假值/vsource 假锁/slope 偷放/cpu 双 0/降画质装绿）或 wr-close 门禁 FAIL 的原因，导回真窗代码层修复或引擎洞定点修，然后重跑重审重关。当用户说「反攻 R3」「返工 R4 真窗」「R7 装绿了改掉」「重做 R5」「rework R3」「R7 修 bug」「重写 R7 真窗」「重写已关的 R4」时触发。与 gpui-wr-quality（写前定标准）、gpui-wr-close（执行关闭流程）、gpui-metrics-audit（指标层审查）互补，专门接「门禁不过 → 定位 → 修 → 重跑 → 重审 → 重关」这条回流。
user_invocable: true
disable_model_invocation: false
---

# gpui-wr-rework — §R 真窗反攻/修复回流（防删场景保门禁）

> **与现有 skill 的分工：**
> - `gpui-wr-quality` = 关 R **前**的质量标准（场景矩阵 + HUD + 实现点六维）——**写代码前**
> - `gpui-wr-close` = 关 R 的**执行流程**（建窗/跑/判门禁/回写）——**写完代码后**
> - `gpui-metrics-audit` = 指标族 A–J 的**正误审查与 bug 修复**——**指标层**（fps 假值/vsource 假锁/slope 偷放/cpu 双 0/降画质装绿）
> - **本 skill** = 门禁 FAIL / 指标装绿被识破后，**导回真窗代码层修复或引擎洞定点修**的回流——**代码层 + 引擎层**
>
> **真源：** `docs/ENGINE_UI_WIDGET_RENDER.md` §5「引擎若挂必修」+ §9「文档与状态治理」+ U9（关闭须代码+真窗绿+回写）；`docs/ENGINE_UI_RENDER_BASE.md` §25.5「收口后下一步」。若本 skill 与真源矛盾，以真源为准——发现矛盾停下报告，不要自决。

## 0. 为什么要这个 skill

历史 `ui_wr_*` 真窗在门禁 FAIL 后，最常见 3 类错误回流：

| 错误回流 | 表现 | 为何危险 |
|----------|------|----------|
| **删场景保门禁** | fps 低就删动画、damage_ratio 高就删静区 | 违反 wr-quality U17「复杂场景硬要求」；降画质装绿 |
| **降阈值过门禁** | README 把 fps 门禁从 55 放宽到 45 | 违反 metrics-audit §3「门禁阈值防偷放」；门禁是硬的不许放 |
| **引擎洞当示例 bug 修** | BoundaryCache 在文/嵌套下不 skip，却在 main.go 里绕过 | 该修 `ui/rendering/boundary_cache.go`，不是改示例绕洞 |

本 skill 把「门禁 FAIL → 分类 → 定位 → 修 → 重跑 → 重审 → 重关 → 回写状态」这条回流**固化**，禁止删场景保门禁、禁止降阈值、禁止在示例里绕引擎洞。

---

## 1. 触发与输入

用户可能给：

- 一个 R/C id + FAIL 原因（如「反攻 R3，boundary_skip=0」「R7 装绿了改掉」）→ 反攻该窗
- 一个具体怀疑（如「R4 fps 假值，改真窗代码」「R7b slope 偷放，返工」）→ 定向反攻该字段
- 一个「重写」请求（如「重写 R7 真窗」「重写已关的 R4」）→ 按 §2 分类走对应回流

从输入解析：

- `ABILITY_ID`（必给）：如 `R7`、`C3`、`R3b`
- `FAIL_REASON`（若给）：来自 metrics-audit 审查报告或 wr-close 第 3 步 FAIL 输出
- `SUSPECT_FIELD`（若给）：定向反攻该字段（如 `boundary_skip`、`bind_count`、`damage_ratio`）

## 第 0 步：读真源 + 定位 + 状态降级

**必做**——每次都先读，禁止凭记忆跑流程（真源会变）。

1. `read_file docs/ENGINE_UI_WIDGET_RENDER.md`：
   - §2 主表该 id 行（取 `PACKAGE`、`WINDOW=1200×800`、`CLOSE_SECONDS`、`WAVE`、当前「状态」列）
   - §2.2.1 指标族 × 真窗义务表 + §2.2.2/§2.2.3/§2.2.4 FAIL 线（判定基准）
   - §5「引擎若挂必修」表（高概率引擎洞 → 修哪）
   - §9「文档与状态治理」+ §10 修订表（返工时状态降级 + 版本回写）
2. `read_file docs/ENGINE_UI_RENDER_BASE.md` §20.2 M-\* 全表 + §25.4 架构诚实点 + §25.5 收口后下一步（若涉及引擎洞）
3. `read_file examples/ui_wr_<package>/main.go` + `README.md`——定位现有真窗代码与门禁阈值声明
4. `read_file .atomcode/skills/gpui-wr-quality/SKILL.md` §3 场景矩阵该 R 行 + §4 实现点六维——**反攻后必须重新达 U17/U18/U20**
5. `read_file .atomcode/skills/gpui-metrics-audit/SKILL.md` §1–§5——若 FAIL 来自 metrics-audit，对照其审查结论

### 状态降级（若该 R 当前 ✅）

按 WIDGET_RENDER §9「文档与状态治理」：

1. `read_file docs/ENGINE_UI_WIDGET_RENDER.md` 定位 §2 主表该 R 行的「状态」列
2. 用 `edit_file` 把 `✅` 改成 `🔄 质量返工`（保留「基础门禁曾绿」备注，但**不得宣称关闭**）
3. §4/§4b/§4c 关闭清单（若该 R 在列）：改为「基础门禁曾通 / 质量条重写中」
4. §10 修订表加一行：`<版本号> | R<id> 质量返工：<FAIL 原因摘要>`

**若该 R 当前 ⬜（未关）**：跳过状态降级，直接走第 1 步（这是「重写未关 R」场景）。

**若该 R 当前 🔄**：说明上一轮返工没收尾，继续本 skill 流程，不重复降级。

## 第 1 步：FAIL 原因分类（3 类）

把 `FAIL_REASON` 或 metrics-audit 审查结论分进 3 类，每类走不同的修复路径：

| 类别 | 特征 | 修哪 | 走第几步 |
|------|------|------|----------|
| **A. 指标层 bug** | 字段默默省略、`unavailable` 无原因、JSON 字段名与 `FrameMetrics` struct 不一致 | `examples/ui_wr_*/main.go` 的 JSON marshal 段 | 交回 `gpui-metrics-audit` 第 1/4 步修，**不**走本 skill |
| **B. 真窗代码层 bug** | 场景未达 U17（两色块/纯色块阵）、HUD 不见、相位脚本缺、实现点六维缺项、门禁阈值偷放 | `examples/ui_wr_*/main.go` + `README.md` | 走第 2 步（本 skill 主战场） |
| **C. 引擎洞** | 复杂场景下引擎炸：BoundaryCache 文/嵌套静区不 skip、壳在 retained 下闪/残、DirtyLayerIDs 每帧重建、无像素采样门禁 | `ui/rendering/*` 或 `ui/embedder/*`（按 wr-quality §5 高概率洞表） | 走第 3 步（引擎定点修） |

### 分类决策树

```text
FAIL 原因是「JSON 字段缺/无名/null 无原因」？
  ├─ 是 → 类 A，交回 metrics-audit
  └─ 否 → FAIL 原因是「场景简陋/HUD 不见/相位缺/实现点缺/阈值偷放」？
            ├─ 是 → 类 B，走第 2 步
            └─ 否 → FAIL 原因是「复杂场景下引擎炸/BoundaryCache 不 skip/壳 retained 闪」？
                      ├─ 是 → 类 C，走第 3 步
                      └─ 否 → 停下问用户，不要猜
```

### 「重写」请求的分类

| 用户说 | 类别 | 走法 |
|--------|------|------|
| 「重写 R7 真窗」（R7 未关 ⬜） | 类 B（推倒重写未关 R） | 第 2 步 + 交 wr-close 关 |
| 「重写已关的 R4」（R4 ✅） | 类 B（推倒重写已关 R 升维） | 第 0 步状态降级 + 第 2 步 + 交 wr-close 重关 |
| 「R7 引擎炸了改引擎」 | 类 C | 第 3 步引擎定点修 |

## 第 2 步：真窗代码层修复（类 B 主战场）

> **禁止：** 删场景降复杂度保门禁、降 README 阈值过门禁、在示例里绕引擎洞（该走第 3 步）。

### 2.1 重新对照 wr-quality 场景矩阵

`read_file .atomcode/skills/gpui-wr-quality/SKILL.md` §3 场景矩阵该 R 行，确认：

- 该 R 行的「复杂场景要求」列（如 R3：嵌套 3 层 RB + 静文 + 热；skip 累加可见）
- 该 R 行的「窗内必见」列（如 R3：skip↑ rerecord 仅热）
- 该 R 行的「引擎若挂必修」列（若本步触发引擎洞，交第 3 步）

### 2.2 按 wr-quality §1 U17 五项 + §2 U18 HUD + §4 实现点六维重写

定位 `examples/ui_wr_<package>/main.go` 的缺陷段，按 wr-quality 标准重写：

| 缺陷 | 重写动作 |
|------|----------|
| 场景简陋（两色块/纯色块阵） | 加多区域布局（TopBar/Side/Body/Status）+ 静态密集（≥8 标签或 4×4 色格 + 嵌套 boundary）+ 动态热点 + 能力专属压力 |
| HUD 不见 | 加 wrkit LiveHUD：能力 ID + 相位 + FPS/p95 + 核心计数器 + PASS/FAIL 预览色 + 图例 |
| 相位脚本缺 | 加 ≥2 相（稳态 Steady / 突变 Spike / 可选恢复 Recover） |
| 实现点六维缺项 | 补对应维度代码或 README 说明（正确性/脏区/缓存/边界条件/失败模式/窗内如何看出） |
| 门禁阈值偷放 | README 把阈值降回文档默认（fps≥55、slope≤30000、cpu≤85%、damage_ratio≤0.35 等） |

### 2.3 重写 README

`examples/ui_wr_<package>/README.md` 必含四段（wr-close §1.3）：

1. **Run**：`RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<package>`（+ LD_LIBRARY_PATH/WGPU_NATIVE_PATH）
2. **Window / Close duration**：`1200×800` + `<CLOSE_SECONDS>s` + `RUN_SECONDS<5 → FAIL`
3. **Visible effect**：表格列「Region / Expectation」，写人眼能见的预期
4. **Gates**：逐条列该 R 的 §2 主表门禁 + §2.2 全族 FAIL 线（**阈值不偷放**）

### 2.4 重写完 → 交第 4 步重跑

## 第 3 步：引擎洞定点修（类 C）

> **禁止：** 在 `examples/ui_wr_*/main.go` 里绕洞（如把静区改成纯 ColorBox 让 BoundaryCache 能 skip）——这违反 wr-quality §5「禁止为过门禁删场景降复杂度」。

### 3.1 定位引擎洞

按 wr-quality §5「引擎若挂必修」表对照：

| 高概率洞（复杂场景后会炸） | 修哪文件 |
|----------------------------|----------|
| BoundaryCache 仅 Color/Absolute MVP，文/自定义 OnPaint 静区无法 skip | `ui/rendering/boundary_cache.go`（扩 record 类型或强制静区走可录路径） |
| 壳在 retained 下闪/残；force/clear 与 CompositeOnly 交界；HUD damage 污染 | `ui/embedder/pipeline_app.go` |
| DirtyLayerIDs 每帧重建，R4b id 不稳 | 稳定 cacheID 绑定（`ui/rendering/` 或 `ui/embedder/`） |
| 无像素采样门禁，纯 ratio 假绿 | 可选 `SAMPLE_PX` 探针（逻辑坐标读回或约定截图像素） |

### 3.2 定点修引擎

`read_file` 对应 `.go` 文件，`list_symbols` 看结构，定位缺陷函数，`edit_file` 定点修。

**修引擎的纪律：**

- **不**改门禁阈值（门禁是硬的）
- **不**改真窗场景（场景是 wr-quality 定的）
- **只**改引擎实现，让复杂场景下的指标真绿
- 修完跑 `go test ./ui/...` 确认没回归

### 3.3 引擎洞修完 → 交第 4 步重跑真窗

**若引擎洞无法在本波修**（如需 multi-RT 层纹理 compositor，属 W6）：停下报告用户，该 R 状态保持 🔄，不强行装 ✅。

## 第 4 步：重跑 GPU 真窗取新 JSON

**禁止跳过**——反攻的核心是「重跑验真绿」。

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<package>
```

- 若环境无 GPU/X11：**停下**，告诉用户「需要 GPU 真窗环境（needs_gpu_window）」
- 跑完取 stderr + JSON。若 JSON 缺族字段，回第 2 步补 main.go
- 若 `exit 1` 且 `FAIL:` 原因是门禁不达标，**回第 1 步重新分类**（是示例 bug 还是引擎洞？不要在没分类清楚前盲目重修）

## 第 5 步：重跑 metrics-audit 验真绿

`use_skill` 加载 `gpui-metrics-audit`，对**新 JSON** 跑 §1–§5 全套审查：

- 字段完备性（族 A–J 不默默省略）
- 诚实性（vsync_source 不假锁、fps_interval 不靠静帧刷高）
- 门禁阈值防偷放（README 阈值不高于文档默认）
- 指标与代码观测一致性（JSON 累计数 = 代码实际）
- 降画质装绿检测（场景达 U17、RUN_SECONDS ≥ 关闭用值）

### 真绿判定

| metrics-audit 输出 | 动作 |
|--------------------|------|
| 全 PASS（EARNED + HONEST + CONSISTENT + AT_DEFAULT + PRESENT） | 交第 6 步重关 |
| 任一 FAIL | 回第 1 步重新分类（指标层 bug 交 metrics-audit；真窗代码层 bug 回第 2 步；引擎洞回第 3 步） |

**关键：本步不许降门禁。** 若 fps 真低就修引擎/减装饰（回第 3 步），**禁止**把 fps 门禁从 55 放宽到 45。

## 第 6 步：交 wr-close 重关 + 状态升维

`use_skill` 加载 `gpui-wr-close`，走其完整流程（第 0–5 步）重关该 R：

- 第 0 步：读真源 + 定位包（取当前 `CLOSE_SECONDS`、`WAVE`）
- 第 1 步：跳过（真窗代码已在第 2 步重写）
- 第 2 步：重跑 GPU 取新 JSON（与第 4 步一致，wr-close 自己再跑一遍确认）
- 第 3 步：判门禁（逐族 FAIL 线）
- 第 4 步：若该 R 有组合窗，跑组合窗
- 第 5 步：回写 docs §2 状态列

### 状态升维（WIDGET_RENDER §9）

按 wr-close 第 5 步回写时，**反攻场景**的状态转换：

| 反攻前状态 | 反攻后状态 | §10 修订表 |
|------------|------------|------------|
| ⬜（未关） | ✅（首次关闭） | `<版本> \| W<n> ✅ <R id> 真窗 + C<组合>` |
| ✅（曾关，质量返工中 🔄） | ✅v2（质量条） | `<版本> \| R<id> 质量升维 ✅v2：<反攻摘要>` |

`edit_file` 回写 `docs/ENGINE_UI_WIDGET_RENDER.md`：

1. §2 主表该 R 行「状态」列：`🔄 → ✅v2（质量条）` 或 `⬜ → ✅`
2. §4/§4b/§4c 关闭清单（若该 R 在列）：`🔄 质量返工 → ✅v2`
3. §10 修订表加一行版本说明
4. 重新 `read_file` 确认 edit 落点对、没破坏表格 markdown

## 输出格式

完成后给用户一份结构化报告：

```
✅ R<id> 反攻收口

反攻前状态：<✅ / 🔄 / ⬜>
FAIL 原因：<类 B 真窗代码层 / 类 C 引擎洞 / 类 A 指标层（交 metrics-audit）>

修复动作：
  类别：<B 真窗代码层 / C 引擎洞>
  改了哪些文件：
    - examples/<package>/main.go（<重写摘要>）
    - examples/<package>/README.md（<阈值回写摘要>）
    - ui/rendering/<file>.go（<引擎洞定点修摘要>，若类 C）
  引擎洞修了吗：<否 / 是，修了 <file> 的 <函数>>

重跑验证：
  GPU 真窗：RUN_SECONDS=<CLOSE_SECONDS> PASS
  metrics-audit 重审：全 PASS（EARNED + HONEST + CONSISTENT + AT_DEFAULT + PRESENT）
  wr-close 重关：族 A–J 全绿

状态升维：
  docs §2 R<id> 行：<🔄 → ✅v2（质量条）> 或 <⬜ → ✅>
  §10 修订表：加版本说明

下一步：<W<n+1> 剩 R<下个 id>> 或 <反攻完成，无后续>
```

若有任一 FAIL：

```
⚠️ R<id> 反攻未达标

当前类别：<B / C / 未分类>
FAIL 原因：<具体>
下一步建议：
  - 若类 B 真窗代码层：回第 2 步重写
  - 若类 C 引擎洞：回第 3 步定点修，或停下报告「引擎洞需 W<n> 才修」
  - 若类 A 指标层：交回 metrics-audit
```

## 禁令自检（每次结束前过一遍）

- [ ] **没删场景保门禁**（违反 wr-quality U17 + §5）
- [ ] **没降 README 阈值过门禁**（违反 metrics-audit §3）
- [ ] **没在示例里绕引擎洞**（该走第 3 步定点修 `ui/`）
- [ ] 类 A 指标层 bug 交回了 metrics-audit，没在本 skill 里修 JSON marshal
- [ ] 类 C 引擎洞走第 3 步定点修了 `ui/rendering/*` 或 `ui/embedder/*`，没改示例绕洞
- [ ] 状态降级做了（已关 R 反攻时 ✅ → 🔄）
- [ ] 状态升维做了（反攻真绿后 🔄 → ✅v2 或 ⬜ → ✅）
- [ ] §10 修订表加了版本说明
- [ ] 重跑用了 `RUN_SECONDS ≥ 关闭用值`，没靠短时窗刷低 slope/hitch
- [ ] 引擎洞修完跑了 `go test ./ui/...` 确认没回归
- [ ] 若引擎洞无法本波修（如需 W6 multi-RT），停下报告了，没强行装 ✅

任一项未过 → 回对应步骤修。

---

## 附：与 wr-quality / wr-close / metrics-audit 的接口

| 接口方向 | 内容 |
|----------|------|
| **入：metrics-audit 审查报告** | FAIL 字段 + 假绿模式（SUSPECT/FAKE/LOOSE/GATE_OFF/SCENE_CHEAT/TIME_CHEAT/IDLE_CHEAT）→ 本 skill 第 1 步分类 |
| **入：wr-close 第 3 步 FAIL** | 门禁不过的族 + 原因 → 本 skill 第 1 步分类 |
| **出：交 wr-close 重关** | 本 skill 第 2/3 步修完 + 第 4 步重跑 + 第 5 步重审真绿后，交 wr-close 走第 5 步回写 docs §2 状态列 |
| **出：交 metrics-audit 重审** | 本 skill 第 4 步取新 JSON 后，交 metrics-audit 跑 §1–§5 验真绿（第 5 步） |
| **不接：类 A 指标层 bug** | 字段缺/null 无原因/JSON 字段名不一致 → 交回 metrics-audit 第 1/4 步修 JSON marshal，**不**走本 skill |

**关键边界：** 本 skill 只修**真窗代码层（类 B）+ 引擎洞（类 C）**。指标层 JSON marshal bug（类 A）归 metrics-audit；新 R 首次关闭归 wr-close（不经过本 skill）；新 R 写代码前的标准定归 wr-quality（不经过本 skill）。
