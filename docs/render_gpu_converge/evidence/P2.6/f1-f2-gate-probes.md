# F1/F2：门禁探针两处不敏感补强

> 台账来源：§2.2 F1/F2（P1.8 负向对照移交，`baseline/negative/summary.md` §结论）。
> 诊断/修复日期 2026-11-11。用户拍板「都做」。

## F1：一致性台 gpu_ops 计数下限收紧为用例精确值

**病灶**：三份用例 `render/testdata/conformance/*.json` 全挂 `"min_gpu_ops": 1`，
比较逻辑 `render/internal/conformance/report.go:200` 纯下限——P1.8 skipdraw 实测
跳一次绘制 gpu_ops 3→2/2→1 仍 ok（当时只靠像素断言兜底）。

**修法（零代码，纯数据收紧）**：`min_gpu_ops` 1→精确值。依据 = 当前树实测
（`render/tmp/conformance/` 双后端 6 份 report）与 P0.4 归档逐值一致：
**solid_gradient=3、clip_blend=3、text_image=2**（native/go 各自相同）。

**验证**：
- 收紧后正常跑绿：`TestConformanceDualBackend` ok，6 路全部 `gpu_ops == min`
  （3/3/2）——精确值即实测值，无假绿空间。
- 负向（`docs/render_gpu_converge/baseline/negative/patch.diff` 原补丁重放，
  `git apply --check` 干净）：`GPUI_NEG_SKIP_DRAW=1` → **六路全红**，每路首条
  即 `gpu_ops=2 < min=3`（text_image `1 < min=2`）——计数门禁独立抓住单次跳绘，
  不再依赖像素断言；还原补丁 → 绿。
- 口径登记：将来引擎 op 数合法变化会红——这是门禁意义（变化必须被看见并
  有意重录，同金图重种流程）。

## F2：oom probe 加早期基线锚（抓「整体抬底」型泄漏）

**病灶**：`examples/ui_base_vram_cap/main.go::runOOMProbe` 的
`Recovered = abs(post-pre) <= tol`（原 :923）只比注入窗两侧相对差——P1.8
skipforget 第一方案实测：全局少释放把 pre/post 一起抬高，`pre==post` 恒真，
探针失效。

**修法（example 层测量窗探针，非引擎）**：
- 注入点之前加**早期基线窗**（presents ∈ [after-preLo-32, after-preLo-16]，
  50ms 采样 ≥5 样本）；`oomVerdict` 增 `EarlyMB/EarlyN/BaselineStable` 字段
  （JSON 只增不改，`verify_vram.py` 只查 `pre_inject_live_mb` 存在性，兼容）。
- gate 新增判定：`preMB - earlyMB > tol` → FAIL
  `ledger baseline lifted before the injection: … 整体抬底型泄漏（F2，资源谁建谁放）`；
  早期窗样本 <5 → FAIL 提示抬高注入点。

**验证（含真泄漏红证据，用户拍板「造真泄漏跑出红」）**：
- 正常跑绿：`early=11.26MB pre=11.26MB post=11.26MB recovered=true stable=true`
  （6 样本，单窗标准关闭）。
- **P1.8 原补丁（GPUI_NEG_SKIP_FORGET）重放：不红，且如实定性**——现行引擎
  pooling 化后该稳态窗零逐帧分配，账本全程平（early=pre=post=11.26），原泄漏
  形态在此窗已不显形；P1.8 当年能显形系当时窗有逐帧 churn。
- **真泄漏注入（临时 env 门控补丁，跑完即还原）**：`GPUI_NEG_LEAK_FRAME=1`
  在栅格线程 `endSubmission` 每第 6 帧真分配 512×512 BGRA 纹理并永不释放
  （「引擎逐帧分配不释放」回归形态，真账本抬底）。全窗泄漏变体 →
  `pre=46.26 post=64.26 delta=18.00` 被 `Recovered` 抓红（附带证明注入后抬升
  也可抓）；**收窄到 early/pre 窗之间的泄漏变体 → 正中锚红**：
  `FAIL: ledger baseline lifted before the injection: early=11.26MB pre=17.26MB
  tol=4.00MB`（6 次 ×1MB 抬升，Recovered 保持绿 ⇒ 红专属于基线锚）。
  还原补丁 → 工作区零残留。

## 附带产出：render/text 逐文件门禁首验证（K6 子项）

`tools/per-file-tests.pkgs` 加 `render/text`（125 测试文件，整包 1006 测试
20m/40m 超时=体量，逐文件为 K6 口径）。首验证（本轮后台跑）：**119 PASS /
5 SKIP_TAGGED / 1 TIMEOUT**——唯一红 = `zz_b2_bitmap_scan_test.go` 20 分钟
超时（韩文位图扫描重测试，体量非断言红；AGENTS「长跑文件单独给足 -timeout」
口径，后续按需单独给时或拆分）。
