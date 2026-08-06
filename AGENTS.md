## 技能执行规则

- 所有 skill（wr-debug / wr-engine / wr-close / wr-rewrite / wr-implement / metrics-audit）的「停下报告」节点必须停，用 `question` 工具问用户确认方向后再继续，禁止跳过或自决。
- 禁止在示例层绕引擎洞（改 `examples/` 而不改引擎层 `ui/` / `render/` / `gpu/`）。
- JSON 门禁绿 ≠ 引擎洞已修；洞修了才能标 ✅v2。
- wr-engine 分层风险与修复规则：
  - `ui/rendering/` `ui/embedder/` `ui/scene/` `ui/io/` — **低**（能力实现层，可直接按场景修）
  - `render/` — **高**（必须 question 确认后修）
  - `gpu/` — **最高**（必须 question 确认后修）
- wr-close 任一模式跑 GPU 发现底层不满足 → 交 wr-engine（禁止在示例里绕）。
- wr-rewrite 是调度层，不直接写代码。

## 真窗与状态真源（硬）

- **状态只认** `docs/ENGINE_UI_WIDGET_RENDER.md` 的 **§2 主表（R）+ §3 组合表（C）+ §5 分期（W）+ §10 修订**。
- 每个 W 的**每一个主能力 R** 必须有独立 `examples/ui_wr_r*` 真窗（U5）；**禁止**用组合窗代替单 R; R 对应的 example 以`docs/ENGINE_UI_WIDGET_RENDER.md` 的 2.6 为实现标准。
- 每个 W 的**每一个组合窗 C** 必须有独立 `examples/ui_wr_c*` 真窗（U6）；C 只做集成，**不能**代替任何 R; C 对应的 example 以`docs/ENGINE_UI_WIDGET_RENDER.md` 的 3 为实现标准。
- 空目录（无 main.go）= 未建，不得标 ✅。

## 修复流程纪律

- 任何 R/C 能力点 FAIL → 必须加载对应 skill（wr-debug / wr-close / wr-engine / wr-implement / metrics-audit 等）并严格走完该 skill 的完整步骤。
- skill 内标记「停下报告」的节点必须用 question 工具停下，得到用户确认后再继续。
- 禁止跳过 skill 中的任何一步直接修代码。

## 每次新上下文

- 先读 AGENTS.md（本文），再读 `docs/` 真源，再执行。
- 所有状态以真源为准，不凭记忆。

## 测试数据与阶段回归（硬）

> **适用范围**：仅针对 `render/text` 下 Go 自研文本渲染功能的**阶段完成验收**（M0–Mx 各阶段）。其他功能的开发/验收**不需要走本流程**。

- **测试数据一律入 `testdata/`**（各包 `xxx/testdata/`）：字体、字表（如 cjk3000.txt）、FT 对照工具等；`/tmp` 下仅允许 `t.TempDir()` 的进程级临时文件，禁止把外部路径（如 `/tmp/opencode`）写死在测试代码里。
- **各语言标准测试字表文件**（`render/text/hint/testdata/`）：`cjk3000.txt`（3000 常用字）、`kr_all.txt`（韩文 11172 音节）、`th_all.txt`（泰文 128 码位）、`latin_all.txt`（L1 255 字 = ASCII 95 + accent 29 + confusable 17 + 西里尔 66 + 希腊 48）；测试一律读文件，**禁止在测试代码里用 range 循环/硬编码生成字表**；新增语言测试子集必须先落字表文件。
- FT 度量衡二进制 = `render/text/hint/testdata/ftexp/`（源码 main.go + go.mod module ftexp，**不提交二进制**）；测试经 `ftexpBin(t)`（hint 包）解析：`$FTEXP_BIN` → 已构建产物 → 拷贝源码到 `t.TempDir()` 自动 `go build` 重建（需 go + purego 依赖）。禁止把 `/tmp/opencode/ftexp` 硬编码进测试。
- 每阶段开发/验收必须回归**之前所有已完结阶段**：跑全量 `go test ./render/text/...`（含 M0–M3 各阶段全字集扫描对照，确保不破坏旧阶段），新阶段扫描坏字数不得回退。
- 测试长跑（全字集 scan）与 require 外部字体（系统 Noto CJK/TTC）的用例，字体缺失时用 `t.Skipf` 提示原因，**禁止静默假绿**。
