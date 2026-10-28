# P0.3 归档与回滚演练

> 状态：✅ 已执行（2026-10-28，R1–R8 全 PASS；R5 判据在执行中修订，原因见 §1 说明）
> 状态真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2 的 P0.3 行；本文件只装演练设计与实测记录。
> 依据：TASKS §3「P0.3 归档与回滚演练」、§2.1 验证载体、`AGENTS.md`「执行动作需谨慎」「GPU 真窗与压测安全」。

---

## 0 目的

证明两件事：

1. **退得回去**：能回到 P0.1 文档写入之前的仓库状态，且回退态可构建、可辨认。
2. **恢复得回来**：回到起点后，树内容与演练前**逐位一致**，无关未提交改动一个字节都没动。

本演练**不**验证业务功能回退（P0 阶段还没有行为变更），只验证「归档点可复现 + 反向操作安全」。

---

## 1 判据（全部满足才算 PASS）

| 编号 | 判据 | 判定方法 | 期望 | 实测 |
|---|---|---|---|---|
| R1 | 回退点提交精确 | 隔离目录内 `git rev-parse HEAD` | `876b7835ed3169f3ec14003e91d83853c9356a70` | **PASS**：`876b7835ed3169f3ec14003e91d83853c9356a70` |
| R2 | 回退点树与起点树不同 | 比较 `git rev-parse HEAD^{tree}`（两侧） | 两值不相等 | **PASS**：`893b1c9dbc3c43f4ed2c67de37c169cddeabf779` ≠ `eee499f554f26d8d78f850bacdf1c7d6f5169716` |
| R3 | 回退态确实没有 P0 文档 | 隔离目录内 `ls docs/render_gpu_converge/baseline/` | 目录不存在（`No such file or directory`） | **PASS**：`没有那个文件或目录`；负向对照放入 `windows.md` 后转红（§5） |
| R4 | 回退态台账是旧状态 | 隔离目录内查 TASKS 的「下一任务」与 P0.2 行 | 「下一任务：P0.1」且 P0.2 为 ⬜ | **PASS**：`:51 下一任务：P0.1`；`:72 P0.2 … ⬜` |
| R5 | 回退态构建无新增失败（执行中修订，见下） | `go build ./...` 日志与起点逐行 diff；再排除既有坏例外构建其余包 | 失败集逐行相同；其余包全部通过 | **PASS**：与起点日志逐行相同；350 包 `go build` 通过 |
| R6 | 恢复后回到起点 | 主仓 `git rev-parse HEAD^{tree}` 对比步骤 0 记录 | 逐位相等 | **PASS**：`eee499f554f26d8d78f850bacdf1c7d6f5169716` |
| R7 | 无关未提交改动零变化 | 主仓 `git status --short` 与三个文件的 `sha1sum` 对比步骤 0 | 行数、文件名、哈希全部一致 | **PASS**：status 7 行逐行一致；`af955d3a…`/`741f4823…`/`a2e079b5…` 三个哈希不变；`.qoder`、`.atomcode` 仍在 |
| R8 | 无演练残留 | `git worktree list \| grep 'gpui-p0.3-drill'`、`git tag -l 'drill*'`、`git tag -l 'baseline*'` | 隔离目录消失；`drill*` 与 `baseline*` 均为空；4 个既有 prunable worktree 原样保留 | **PASS**：`/tmp/gpui-p0.3-drill` 已消失；两条 tag 查询为空；4 条既有 worktree 原样 |

> **R5 修订说明（首轮红保留，不删）**：模板原判据是「回退态 `go build ./...` 成功」。执行时先撞两堵墙：
> 1. `/tmp` 隔离目录下 `replace github.com/ebitengine/purego => ../purego` 解析成 `/tmp/purego`（不存在），构建在 3 秒内失败——这是隔离位置造成的环境问题，不是回退代码问题；补 `/tmp/purego` 软链（只动 `/tmp`，结束已删）后重跑。
> 2. 重跑后仍红：`examples/ui_wr_t_stress/main.go:38-39` 引用 `ui/kit/button`、`ui/kit/icon`，这两个包在 `876b7835` 与当前 `404d8e75` 都不存在。**起点本身 `go build ./...` 就红**（两份日志逐行相同）。
>
> 结论：P0 阶段的任务不是修构建（且 P1.1 本来就负责把这类失败登记为基线已知问题），原判据在红基线上无法通过、也测不出回退是否有害。因此 R5 改为「相对起点无新增失败」：失败集必须逐行相同，且除既有坏例外外的全部包必须构建通过。首轮两次真实 FAIL 与原始输出留在 §4 步骤 4，不改写。

> R3+R4 是「真退回去了」的一对证据：只有目录消失但台账仍是新状态，说明退错了地方；两项同时对，才算回到旧提交。

---

## 2 边界条件（硬，违反任何一条即 FAIL）

1. **只动本地**：禁止 `git push`、禁止改远端 tag/branch、禁止 `fetch` 覆盖本地引用。
2. **不碰主工作区**：回退在 `/tmp` 下的 `git worktree` 里做；主仓工作区全程保持 `404d8e75`（或执行时的起点 HEAD），禁止 `checkout` 旧提交到主工作区。
3. **禁止破坏性命令**：不用 `git reset --hard`、`git checkout --`、`git clean -fd`、裸 `git stash`；发现回退不干净用 `git worktree remove --force` 只清自建目录。
4. **无关改动原样保留**：本次演练前工作区已有三项无关改动——两张示例 PNG、`examples/ui_wr_c5_anim_over_static/golden/c5_final.png`（未跟踪）、`.qoder/`、`.atomcode/`（未跟踪）。演练前记录、演练后比对，不提交、不删除、不还原。
5. **只删自建对象**：演练 tag 与 `/tmp/gpui-p0.3-drill` 由本次创建，结束必须删净；**不动其他分支、tag、worktree**（删除前先 `git worktree list` 确认归属）。仓内已有 4 个 prunable worktree（`/tmp/gpui_c2_ab`、`/tmp/gpui_c2b_only`、`/tmp/gpui_c4_base`、`/tmp/opencode/gpui-head`）是其他会话/历史遗留，禁止 `git worktree prune`、禁止 `remove` 它们，即使显示 prunable 也不清。
6. **正式基线 tag 严禁在此打**：`baseline-*` 只能在 P1.8 全部通过后创建（TASKS §3）。
7. **不跑批量真窗**：本演练默认不开窗；如为 R5 补真窗证据，只允许单窗、`WM_DELETE` 正常关闭、`WaitIdle` 后再释放，遵守 `AGENTS.md` GPU 安全条，禁止并发与循环压窗。
8. **失败不掩盖**：任一 R 项红，记 FAIL + 现象 + 原始输出，状态退回 🟡/⛔，修好后整套重跑，不挑项补测。

---

## 3 演练对象（执行前确认，执行时以实际值为准）

| 项 | 值 | 说明 |
|---|---|---|
| 起点提交 | `404d8e75`（`404d8e75145a77ee2139138abc348472cda9ada2`） | P0.2 提交；执行时若 HEAD 已前移，改用执行时 HEAD 并记录 |
| 回退点 | `876b7835`（`876b7835ed3169f3ec14003e91d83853c9356a70`） | P0.1 文档写入前一个提交；该点 `baseline/` 目录不存在、TASKS「下一任务：P0.1」 |
| 隔离目录 | `/tmp/gpui-p0.3-drill` | 必须在仓库外；结束删除 |
| 演练 tag | `drill-p0.3-<YYYYMMDD>`（首次示例 `drill-p0.3-20261028`） | 轻量 tag，指起点提交；结束删除，永不推送 |
| 正式基线 tag | 不打 | 归 P1.8 |
| 已有 worktree（不动） | `/tmp/gpui_c2_ab`、`/tmp/gpui_c2b_only`、`/tmp/gpui_c4_base`、`/tmp/opencode/gpui-head` | 2026-10-28 实查存在且标 prunable；属其他会话/历史，演练期间原样保留，结束只核对 `/tmp/gpui-p0.3-drill` 已消失 |
| 外部依赖路径 | `replace github.com/ebitengine/purego => ../purego` | 相对路径；隔离目录在 `/tmp` 时需 `ln -s /home/yanghy/app/projects/gogpu/purego /tmp/purego`（或用与主仓同级的目录放 worktree），演完删除软链 |

---

## 4 执行步骤（按序；每步记录原始输出）

### 步骤 0：记录起点（主仓，只读）

```bash
git rev-parse HEAD
git rev-parse HEAD^{tree}
git status --short
sha1sum examples/ui_wr_c3_list_scroll/testdata/showcase_c3_list_scroll.png \
        examples/ui_wr_r5_picture/testdata/showcase_r5_picture.png \
        examples/ui_wr_c5_anim_over_static/golden/c5_final.png
ls -d .qoder .atomcode
```

结果：

```
HEAD  404d8e75145a77ee2139138abc348472cda9ada2
TREE  eee499f554f26d8d78f850bacdf1c7d6f5169716
status 7 行：
  M docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md
  M docs/render_gpu_converge/00_README.md
  M examples/ui_wr_c3_list_scroll/testdata/showcase_c3_list_scroll.png
  M examples/ui_wr_r5_picture/testdata/showcase_r5_picture.png
  ?? .atomcode/artifacts/
  ?? .qoder/
  ?? docs/render_gpu_converge/baseline/rollback.md
  ?? examples/ui_wr_c5_anim_over_static/golden/c5_final.png
sha1  af955d3a8719d18695cd7c217e4d340da3d52f1f  showcase_c3_list_scroll.png
      741f482373299061943a1f6477208504caa2d9b9  showcase_r5_picture.png
      a2e079b5d21b88fa9defa9f4cee8640a43087bb1  c5_final.png
dirs  .atomcode  .qoder
```

### 步骤 1：打演练 tag（主仓，只创建引用）

```bash
git tag drill-p0.3-<YYYYMMDD> <起点提交>
git rev-parse drill-p0.3-<YYYYMMDD>^{commit}
```

结果：

```
404d8e75145a77ee2139138abc348472cda9ada2   ← 与起点提交相同
```

### 步骤 2：隔离回退（仓库外 worktree，不动主工作区）

```bash
git worktree add --detach /tmp/gpui-p0.3-drill 876b7835
cd /tmp/gpui-p0.3-drill && git rev-parse HEAD && git rev-parse HEAD^{tree}
```

结果：

```
HEAD  876b7835ed3169f3ec14003e91d83853c9356a70
TREE  893b1c9dbc3c43f4ed2c67de37c169cddeabf779   ← ≠ 起点 eee499f5…
（worktree add 输出：HEAD 现在位于 876b7835 文档：校正收敛任务依赖并补每任务能力验证硬规则）
```

### 步骤 3：回退态取证（R3、R4）

```bash
cd /tmp/gpui-p0.3-drill
ls docs/render_gpu_converge/baseline/
grep -n '下一任务' docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md
grep -n '^| P0\.2' docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md
git log -1 --format='%H %s'
```

结果：

```
ls: 无法访问 'docs/render_gpu_converge/baseline/': 没有那个文件或目录
51:- 下一任务：P0.1
72:| P0.2 | 真窗与金图清单（窗口名 → 金图 → 平台 → 容差 → 单窗/批量 → 覆盖能力标签） | — | ⬜ | `baseline/windows.md` |
876b7835ed3169f3ec14003e91d83853c9356a70 文档：校正收敛任务依赖并补每任务能力验证硬规则
```

### 步骤 4：回退态构建（R5）

```bash
cd /tmp/gpui-p0.3-drill
go build ./...
```

结果：

```
尝试 1（直接跑）  exit=1，约 3 秒：
  gpu/gwgpu/ffishim/abi.go:42:2: github.com/ebitengine/purego@v0.10.1:
    replacement directory ../purego does not exist
  examples/ui_wr_t_stress/main.go:38/39: reading ../purego/go.mod:
    open /tmp/purego/go.mod: no such file or directory
  → 环境问题（隔离目录相对路径），非回退代码问题

尝试 2（ln -s /home/yanghy/app/projects/gogpu/purego /tmp/purego 后）  exit=1：
  examples/ui_wr_t_stress/main.go:38:2: no required module provides package
    github.com/energye/gpui/ui/kit/button
  examples/ui_wr_t_stress/main.go:39:2: no required module provides package
    github.com/energye/gpui/ui/kit/icon
  → 与起点 HEAD 的 go build ./... 输出逐行相同（diff /tmp/head_build.log /tmp/drill_build.log 无差异）

尝试 3（排除既有坏例外，构建其余包）
  pkgs=350 → exit=0  DRILL_EXCL_BUILD_OK
  （同时观察到 ui/scene/internal/gpupixel 只有测试文件，
    单独列出会报 no non-test Go files；`./...` 不报，登记见 §6.1）
```

> 后端选择是运行期环境变量（`GPUI_BACKEND`），构建与后端无关，故这里只构建一次；不在本演练里伪装成「双后端各构建一次」。

### 步骤 5：恢复与清理（R6、R7、R8）

```bash
cd /home/yanghy/app/projects/gogpu/gpui
git worktree remove /tmp/gpui-p0.3-drill
git worktree list
git tag -d drill-p0.3-<YYYYMMDD>
git tag -l 'drill*'
git tag -l 'baseline*'
git rev-parse HEAD^{tree}
git status --short
sha1sum examples/ui_wr_c3_list_scroll/testdata/showcase_c3_list_scroll.png \
        examples/ui_wr_r5_picture/testdata/showcase_r5_picture.png \
        examples/ui_wr_c5_anim_over_static/golden/c5_final.png
```

结果：

```
worktree list：只剩主仓 + 4 条既有 prunable（gpui_c2_ab / gpui_c2b_only /
               gpui_c4_base / opencode/gpui-head），/tmp/gpui-p0.3-drill 已消失
/tmp/purego 软链已删除
git tag -d drill-p0.3-20261028 → 已删除（曾为 404d8e75）
git tag -l 'drill*' / 'baseline*' → 均为空
TREE  eee499f554f26d8d78f850bacdf1c7d6f5169716   ← 与步骤 0 逐位相同
status 7 行与步骤 0 逐行一致（含 rollback.md 仍未跟踪）
sha1  af955d3a… / 741f4823… / a2e079b5…          ← 与步骤 0 相同
dirs  .atomcode  .qoder
```

---

## 5 负向对照（证明判据不是摆设）

在隔离目录内做一次，做完即删；**目的不是演练成功，而是确认判据会红**。

```bash
cd /tmp/gpui-p0.3-drill
mkdir -p docs/render_gpu_converge/baseline
git show 404d8e75:docs/render_gpu_converge/baseline/windows.md > docs/render_gpu_converge/baseline/windows.md
ls docs/render_gpu_converge/baseline/
```

期望：R3 由 PASS 转红（文件出现，目录出现），恢复 worktree 后再次变绿。

结果：

```
实际改法（避免在演练里用 `git checkout --`）：
  mkdir -p docs/render_gpu_converge/baseline
  git show 404d8e75:docs/render_gpu_converge/baseline/windows.md \
    > docs/render_gpu_converge/baseline/windows.md
  ls → windows.md（33897 字节）
  → R3 由 PASS 转红（目录与文件都出现），判据确实能分辨真假回退
清理：git worktree remove --force /tmp/gpui-p0.3-drill（自建目录，含未跟踪文件）
  → 路径消失，负向对照的引入物随目录一起清掉
```

> 这项单独记录「红过」，否则 R3 只能证明「命令没报错」，不能证明「能分辨真假回退」。

---

## 6 结果记录（执行后回填，通过后本文件才算证据）

| 项 | 内容 |
|---|---|
| 执行日期 | 2026-10-28 13:36–13:44 |
| 执行人/会话 | Qoder 会话（分支 `feat/框架代码整理`，起点 `404d8e75`） |
| R1–R8 | R1 PASS / R2 PASS / R3 PASS / R4 PASS / R5 PASS（修订口径，首轮两次 FAIL 见 §1 说明）/ R6 PASS / R7 PASS / R8 PASS |
| 负向对照 | 通过：放入 `windows.md` 后 R3 转红；移除 worktree 后引入物清零 |
| 失败项与处理 | 首轮 R5 FAIL 两次：① `/tmp` 下 `../purego` 不存在（环境，补软链后重跑）；② `examples/ui_wr_t_stress` 引用不存在的包（既有缺陷，登记 §6.1）。均未改代码，未用 reset/clean 掩盖 |
| 耗时 | 约 8 分钟（含 350 包构建） |
| 结论 | **PASS**：退得回去（R1–R4）、恢复逐位一致（R6–R7）、无残留（R8）；附带发现 1 个既有构建缺陷交给 P1.1 |

### 6.1 执行中发现的问题（归位，不顺手修）

| 现象 | 位置 | 影响 | 归属 / 处理 |
|---|---|---|---|
| `go build ./...` 引用不存在的包 | `examples/ui_wr_t_stress/main.go:38-39` → `ui/kit/button`、`ui/kit/icon`（`876b7835` 与 `404d8e75` 均无此目录） | 全仓 `go build ./...` 在 P0 起点即红；该示例无法构建 | 归 P1.1「六目标交叉编译」登记为基线已知问题；修复另立任务，不在 P0.3 范围 |
| 显式列出时报告「no non-test Go files」 | `ui/scene/internal/gpupixel` | 只有测试文件；`go build ./...` 不报，显式构建会报 | P1.1 一并记录，不视为回退引入 |

---

## 7 失败处理

- R1/R2 红：回退点选错或 tag 指错，删除自建对象后从步骤 1 重做。
- R3/R4 红：可能退到错误提交或 worktree 被污染，`git worktree remove --force` 后重做步骤 2。
- R5 红：记录失败包与错误，先判断是既有问题还是环境问题；不修代码，登记为基线已知问题，结论写 FAIL 并说明范围。若发现「起点即红」导致判据无法表达回退是否安全，可改为差分判据（与起点逐行比失败集），但必须同时保留首轮红与原始输出，并在 §1 写明修订理由。
- R6/R7 红：说明主工作区被动过，立即停止演练，先保护无关改动（复制到 `/tmp` 备份），查清来源再继续；禁止用 reset/clean 掩盖。
- 任何红项都要在 §6 留原始输出；不得只写「基本通过」「环境原因跳过」。

---

## 8 修订记录

| 日期 | 修订 |
|---|---|
| 2026-10-28 | 建模板：定 R1–R8 判据、8 条边界、演练对象（起点 404d8e75 / 回退点 876b7835 / `/tmp` 隔离 worktree / 演练 tag 即删）、执行步骤与负向对照；结果列留待执行回填 |
| 2026-10-28 | 执行：R1–R4、R6–R8 PASS；R5 首轮两次 FAIL（`/tmp` 下 `../purego` 不存在；`ui_wr_t_stress` 引用不存在的包，起点同错），判据修订为「相对起点无新增失败」后 PASS；负向对照 R3 按预期转红；登记 `ui_wr_t_stress` 既有缺陷交 P1.1 |
