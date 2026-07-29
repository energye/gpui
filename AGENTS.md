## 技能执行规则

- 所有 skill（wr-engine / wr-close / wr-rewrite / wr-implement / metrics-audit）的「停下报告」节点必须停，用 `question` 工具问用户确认方向后再继续，禁止跳过或自决。
- 禁止在示例层绕引擎洞（改 `examples/` 而不改引擎层 `ui/` / `render/` / `gpu/`）。
- JSON 门禁绿 ≠ 引擎洞已修；洞修了才能标 ✅v2。
- wr-engine 分层风险与修复规则：
  - `ui/rendering/` `ui/embedder/` `ui/scene/` `ui/io/` — **低**（能力实现层，可直接按场景修）
  - `render/` — **高**（必须 question 确认后修）
  - `gpu/` — **最高**（必须 question 确认后修）
- wr-close 任一模式跑 GPU 发现底层不满足 → 交 wr-engine（禁止在示例里绕）。
- wr-rewrite 是调度层，不直接写代码。

## 修复流程纪律

- 任何 R/C 能力点 FAIL → 必须加载对应 skill（wr-close / wr-engine / wr-implement）并严格走完该 skill 的完整步骤。
- skill 内标记「停下报告」的节点必须用 question 工具停下，得到用户确认后再继续。
- 禁止跳过 skill 中的任何一步直接修代码。

## 每次新上下文

- 先读 AGENTS.md（本文），再读 `docs/` 真源，再执行。
- 所有状态以真源为准，不凭记忆。
