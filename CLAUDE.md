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

## API 文档同步纪律（硬）

- **render 公开 API 总账** = `docs/RENDER_API_CATALOG.md`（分类目录 + §7 接线状态 + §8 方法速查 + §9 常量总表 + §11 附录）。**每次增加/修改/删除 render（或 render/text、render/scene、render/recording、render/surface、render/svg、render/filters、render/raster、render/gpu）的公开 API（顶层 func/type/const/var、导出方法、签名、接线状态），必须同步更新该文档**：对应分类表补/改/删行、更新 §0 统计、§7 状态、§8 速查、§9 常量、§11 附录；并在主体文档 `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表追加一行（版本列写 `API目录同步`）。
- **机器校验（合入前必跑）**：`go run ./scripts/apidoc` 检查目录文档是否覆盖主包 + scene/recording/surface/svg 全部公开符号；非零退出 = 文档漏了，补完再合入。
- 例外：`render/text` 目录文档采用族级归纳（226 项以 `go doc ./render/text` 为准）；`render/filters`、`render/raster` 为纯 init 副作用包（无顶层符号）不参与逐符号校验，但改其 init 行为要更新 §6 状态说明。
- 状态标注必须可溯源：生产消费者 → 文件:行号；GPU 实测 → 真窗名/日期/现象（示例：任意路径裁剪 `Clip()`/`PushClipPath` GPU 画穿，见目录文档 §7.3）。禁止凭印象标注。
