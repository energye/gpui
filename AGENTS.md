## git 提交纪律（硬）

- **只提交自己本次改动的文件**：用 `git status` 对照改动清单，只 `git add` 本次会话我创建/修改的文件；工作区里其他线的工作（其他会话/他人遗留的未提交改动）**禁止一并提交**、禁止 `git add .` / `git add -A` / `git commit -am`。
- 提交前先 `git diff --stat` 确认清单，若发现无关改动，分文件提交并向用户说明哪些留给了哪条线。
- 未推送的本地提交不算完成同步；推送动作需用户确认。

## 技能执行规则

- 所有 skill（wr-debug / wr-engine / wr-close / wr-rewrite / wr-implement / metrics-audit）的「停下报告」节点必须停，用 `question` 工具问用户确认方向后再继续，禁止跳过或自决。
- 禁止在示例层绕引擎洞（改 `examples/` 而不改引擎层 `ui/` / `render/` / `gpu/`）。
- JSON 门禁绿 ≠ 引擎洞已修；洞修了才能标 ✅v2。
- **所有分层（`ui/rendering/` `ui/embedder/` `ui/scene/` `ui/io/` `render/` `gpu/`）按同一标准处理**：在能力实际所在位置直接实现/修复（示例层只做验证，不做绕过），不再设「高风险须先 question」门槛。修复前照常做影响面评估并停在「停下报告」节点征求方向确认（技能流程要求），确认后即可动手，无需额外分级审批。
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

## 对话回答（硬，对所有对话生效）

- **回答一律用大白话**：不堆黑话、术语要用通俗话解释，像跟人聊天一样把事说清楚。适用所有对话、所有包，不只是 `render/text` 的字形对齐。
- 给结论时先一句直话（是什么/行不行/差多少），再解释为什么。别上来就贴代码或指令。

## 测试数据与阶段回归（硬）

> **适用范围**：仅针对 `render/text` 下 Go 自研文本渲染功能的**阶段完成验收**（M0–Mx 各阶段）。其他功能的开发/验收**不需要走本流程**。

- **测试数据一律入 `testdata/`**（各包 `xxx/testdata/`）：字体、字表（如 cjk3000.txt）、FT 对照工具等；`/tmp` 下仅允许 `t.TempDir()` 的进程级临时文件，禁止把外部路径（如 `/tmp/opencode`）写死在测试代码里。
- **各语言标准测试字表文件**（`render/text/hint/testdata/`）：`cjk3000.txt`（3000 常用字）、`kr_all.txt`（韩文 11172 音节）、`th_all.txt`（泰文 128 码位）、`latin_all.txt`（254 字 = ASCII 95 + accent 29 + confusable 17 + 西里尔 66 + 希腊 48，实测 254=111+29+66+48）；测试一律读文件，**禁止在测试代码里用 range 循环/硬编码生成字表**；新增语言测试子集必须先落字表文件。
- FT 度量衡二进制 = `render/text/hint/testdata/ftexp/`（源码 main.go + go.mod module ftexp，**不提交二进制**）；测试经 `ftexpBin(t)`（hint 包）解析：`$FTEXP_BIN` → 已构建产物 → 拷贝源码到 `t.TempDir()` 自动 `go build` 重建（需 go + purego 依赖）。禁止把 `/tmp/opencode/ftexp` 硬编码进测试。
- 每阶段开发/验收必须回归**之前所有已完结阶段**：跑全量 `go test ./render/text/...`（含 M0–M3 各阶段全字集扫描对照，确保不破坏旧阶段），新阶段扫描坏字数不得回退。
- **回归按测试文件逐个跑，禁止一次全量**：禁止直接跑 `go test ./render/text/...`（多包长测并发会卡死/超时）。回归时对每个 `*_test.go` 文件单独执行：`-run '^(文件内全部测试函数名，竖线分隔)$'` 按文件分组跑，一个文件跑完（PASS/SKIP/FAIL 记录）再跑下一个文件；长跑文件（全字集 scan）单独给足 `-timeout`。跑批脚本放 `/tmp/opencode/`，不写死进仓库测试代码。
- 测试长跑（全字集 scan）与 require 外部字体（系统 Noto CJK/TTC）的用例，字体缺失时用 `t.Skipf` 提示原因，**禁止静默假绿**。
