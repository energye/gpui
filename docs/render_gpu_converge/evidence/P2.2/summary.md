# P2.2 删 `render/render` 旧原型 + `FloatTarget` 迁 `render/hdr` — 证据汇总

> 日期：2026-10-29；分支 `feat/p2-收敛整合`；起点 `51b9a5f6`（`baseline-p1` + 纯文档提交，代码与 tag 一致）；执行环境 linux/amd64 + go1.25.5，五目标交叉编译 `CGO_ENABLED=0`。
> 性质：**PLAN §7.7 白名单内的死代码删除 + 无消费者迁移**。全仓生产消费者为零，按台账 §4.1 唯一例外办（无真窗），给三样替代证据：全仓无引用 grep、迁移前后单测一致、P2.6 基线重放无差异（本任务先落前两样，第三样由 P2.6 执行）。

## 1. 做了什么

| 动作 | 对象 |
|---|---|
| 迁移（`git mv`） | `render/render/float_target.go` → `render/hdr/float_target.go`；`float_target_test.go` → `render/hdr/float_target_test.go`；`testdata/float_encode.json` → `render/hdr/testdata/` |
| 包声明 | `package render` → `package hdr`；包注释 `Package hdr holds CPU-side linear HDR render targets.` |
| 行为 | 签名与实现零改动：`FloatTarget`/`TextureView`/`NewFloatTarget`、3 常量、11 方法；仅 op 字符串改 `hdr.*`，注释去掉对已删 `PixmapTarget` 的引用 |
| 删除测试 | `TestFloatTargetSoftwareDeclines`——被测对象是随包删除的老软件渲染器（`NewSoftwareRenderer`/`NewScene`/`NewPixmapTarget`），无法随迁；其余 3 测（构造/编码金图/HDR 解释）全保留 |
| 删除（`git rm`，15 文件） | `device.go` `device_test.go` `doc.go` `example_test.go` `gpu_renderer.go` `gpu_renderer_test.go` `layers.go` `layers_test.go` `renderer.go` `scene.go` `scene_test.go` `software.go` `software_test.go` `target.go` `target_test.go` |
| 目录 | `render/render/`（含 `testdata/`）整体消失 |
| 工具 | `scripts/apidoc/main.go` 注释同步（`render/` 下已无同名子包，改为说明只读直属文件） |

## 2. 门禁结果

### 2.1 能力单测（迁移前后）

| 时点 | 命令 | 结果 | 原始输出 |
|---|---|---|---|
| 迁移前 | `go test ./render/render/` | 86 PASS / 0 FAIL（含 75 Test + 10 Example） | `pre-test-render_render.txt` |
| 迁移后 | `go test ./render/hdr/` | 3 PASS / 0 FAIL（`TestFloatTargetConstruct`、`TestFloatTargetEncodeGolden`、`TestFloatTargetHDRVs8Bit`） | `post-test-render_hdr.txt` |
| 迁移后 | `go vet ./render/hdr/ ./scripts/apidoc/` | rc=0 | `vet.txt` |

`TestFloatTargetEncodeGolden` 读 `render/hdr/testdata/float_encode.json`，金图值未改：2× 白编 (213,213,213)，与迁移前一致。

### 2.2 五目标交叉编译（P1.1b 同一把尺子）

每目标 8 段：原始 `./...`（已知坏包 `examples/ui_wr_t_stress` 红，`ui/scene/internal/gpupixel` 无源码）、已知坏包单独、默认主体、nogpu 原始、nogpu 主体、默认/nogpu 加载失败、nogpu 库范围（`grep -Ev '/(examples|tools)/'`）。

| 目标 | 默认主体 | nogpu 主体 | nogpu 库范围 | 加载失败（默认/nogpu） | 结论 |
|---|---|---|---|---|---|
| linux/amd64 | 351 PASS | 347 PASS | 141 PASS | 0 / 0 | ✅ 与 P1.1b 逐项一致 |
| linux/arm64 | 351 PASS | 347 PASS | 141 PASS | 0 / 0 | ✅ 同上 |
| windows/amd64 | 357 PASS | 353 PASS | 140 PASS | 0 / 0 | ✅ 同上 |
| darwin/amd64 | 349 PASS | 345 PASS | 140 PASS | 0 / 0 | ✅ 同上 |
| darwin/arm64 | 349 PASS | 345 PASS | 140 PASS | 0 / 0 | ✅ 同上 |

js/wasm 按 P1.1b 决定登记不支持，不跑。原始输出：`build/<os>-<arch>.txt`（表头含 head/dirty 与命令原文）。

### 2.3 全仓 `go vet`（默认 + nogpu）

包清单沿用 P1.2b（`packages-default.txt` 352 / `packages-nogpu.txt` 348，仅把 `render/render` 换成 `render/hdr`）：

| 模式 | 包数 | 完成分析 | 告警条数 | 与 P1.2 基线 |
|---|---|---|---|---|
| 默认标签 | 352 | 是（仅既有告警） | 61（59 unsafe.Pointer + 2 unreachable） | 逐条一致（`baseline/vet.txt` §4） |
| `-tags nogpu` | 348 | 是 | 58 | 逐条一致（§5） |

退出码 1 = 既有告警（P1.2 已逐条登记、非零告警不是门禁）；无新增包/新增告警。原始输出：`vet-full-default.txt` / `vet-full-nogpu.txt`。

### 2.4 残留 grep

命令：`grep -rn "render/render" --exclude-dir=third_party --exclude-dir=.git --exclude-dir=docs --include=*.go --include=*.sh --include=*.yml --include=*.yaml --include=Makefile .`
结果：rc=1，零代码引用（`grep-residual.txt`）。唯一命中在排除的文档/工具数据：`tools/cleanup-audit/t2_exports.json` 是 2026-10-07 T0 审计快照，记录的是当时文件路径，不参与构建，按历史数据保留。

### 2.5 API 快照 diff（vs P1.5 `baseline/api`）

包集合保持 352（`render/render` 出、`render/hdr` 进）：

| 项 | 变化 |
|---|---|
| `doc/` 文件集 | 仅 `render/render.txt` 消失、`render/hdr.txt` 新增；其余 350 份逐字节不变（`diff -rq` 只有这两条） |
| `test_counts.tsv` | 仅两行：`render/render` 8 文件 / 75 test / 4 bench / 10 example 删除；`render/hdr` 1 文件 / 3 test 新增 |
| 合计 | 测试文件 1397→1390；测试函数 14388→14302（Test 13602→13530 / Bench 753→749 / Example 18→8 / Fuzz 15→15） |
| 空 doc 白名单 | 逐字节不变 |

原始输出与命令：`api-diff.txt`；可重跑校验：`tools/api-diff.sh`（重生成到 `/tmp/p22_api_post`，与 `baseline/api` 对 diff，本次实测 `RESULT: PASS`）。变更侧小文件留在 `api-post/`（`packages.txt`、`test_counts.tsv`、`doc/render/hdr.txt`、`doc_empty_main.txt`）；未变的 350 份 doc 不入库（与 `baseline/api` 逐字节相同，避免 5.5MB 重复）。这正是 PLAN §7.7 白名单里 `render/render` 那一行的预期差异，无白名单外改动。

### 2.6 负向对照（本能力）

| 步骤 | 结果 |
|---|---|
| 干净树 `go build ./render/hdr/` | exit 0 |
| 临时加 `render/hdr/zz_p22_negative_probe.go`：`import _ "github.com/energye/gpui/render/render"` | **exit 1**：`no required module provides package github.com/energye/gpui/render/render` |
| 删除探针后重建 | exit 0 |
| 探针残留 / 代码引用 | 文件已删；grep rc=1 零命中 |

原始输出：`negative/import-render_render.txt`。

## 3. 文档同步

- `RENDER_API_CATALOG.md`：§0 新增 `render/hdr` 行（类型 2 · 函数 1 · 常量 3 · `FloatTarget` 方法 11，🔌 无生产消费者）；§7.2 新增整包一行；头部子包列表补 `render/hdr`；原 `TextureUsage` 注改写成「`render/render` 已删、唯一真源是 `gpu/types`」。`go run ./scripts/apidoc` rc=0。
- `ENGINE_UI_WIDGET_RENDER.md` §10：追加 `API目录同步` 一行（删除/迁移、计数与 API diff 结论）。
- 台账：§2.1 载体（开工前已登记）、§2 状态、§7 流水。

## 4. 结论

- 五目标编译、全仓 vet、残留 grep、API 快照 diff、负向对照全部按预期；唯一允许差异就是 PLAN §7.7 的 `render/render` → `render/hdr`。
- 测试计数减少 **7 文件 / 86 函数**，全部来自被删对象本身（8 文件 / 89 函数）与迁移（1 文件 / 3 函数）的算术结果，无其他包测试减少。
- 无真窗：零生产消费者死代码删除，按 §4.1 例外；P2.6 基线全量重放时再确认零差异。
- 零行为变化：未改任何保留代码路径；`FloatTarget` 行为由原金图测试锁死。
