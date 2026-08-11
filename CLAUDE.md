## git 提交纪律（硬）

- **只提交自己本次改动的文件**：用 `git status` 对照改动清单，只 `git add` 本次会话我创建/修改的文件；工作区里其他线的工作（其他会话/他人遗留的未提交改动）**禁止一并提交**、禁止 `git add .` / `git add -A` / `git commit -am`。
- 提交前先 `git diff --stat` 确认清单，若发现无关改动，分文件提交并向用户说明哪些留给了哪条线。
- 未推送的本地提交不算完成同步；推送动作需用户确认。

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
