# P1.8 负向对照

> 生成：2026-10-29；HEAD `91a94109`；隔离显示 `DISPLAY=:3`（rootless Xwayland，跑完即关）；
> 执行 worktree `/home/yanghy/app/projects/gogpu/gpui-p14`（同 HEAD 代码），全部破坏用**环境变量门控的临时补丁或用例数据收紧**注入，红/绿对跑完后当场还原，主仓与 worktree 代码零残留（`git diff` 仅剩 worktree 既有的运行期 testdata PNG 产物，0 行代码变更）。

## 口径（thresholds.md §8，逐条对题）

| §8.1 破坏 | 应当触发 | 本轮车辆 | 结果 |
|---|---|---|---|
| 画面偏移 1px | 像素断言红 | P0.4 一致性台：`GPUI_NEG_SHIFT_1PX=1` 使每个 fill_rect 偏移 1px | **双后端红**：coverage 区域断言 `left_red_fill 0.9212 / blend_band 0.8713 ≠ 1.00±0`（`shift1px/red-run.txt`）；env 关 = 绿（`green-baseline.txt`） |
| 跳过一次绘制 | 逻辑探针（draw/paint 计数）红 | 一致性台：`GPUI_NEG_SKIP_DRAW=1` 跳过第一个非 clear 绘制 | **红**（采样/区域断言抓到：被跳绘的填充整块消失）；**诚实发现**：`gpu_ops` 计数门禁下限是 1，单次跳绘抓不住（`skipdraw/out/*/report.json`：gpu_ops 2/2/1 仍 ok）——计数收紧登记 P2 |
| 少释放一次 | 显存账本/生命周期红 | 一致性台 resource 断言：把 K1 释放缺口（solid_gradient 每轮 **+2 条/+131072B**，真实存在的少释放）的放宽上界 `max_steady_bytes_delta` 临时 131072→0 | **双后端红**：`resource: steady count delta=2 bytes delta=131072 (max 0)`（`skipforget/red-run-k1tighten.txt`）；还原 JSON 即绿（`green-after-revert.txt`） |

每项还原后复绿均有留档；`patch.diff` 是破坏 1/2 的临时补丁全文（已 `git checkout` 还原）。

### skipforget 目录的尝试记录（诚实留档）

第一方案（`GPUI_NEG_SKIP_FORGET=1` 全局跳过 `VramForget`，`patch.diff`）**没有转红**：全局跳过会把 probe 的 pre 与 post 采样一起抬高，`pre==post` 恒成立 → `ledger_recovered` 恒真。这是一个真实的门禁覆盖面发现：**oom probe 的「注入前后相等」语义对「整体抬底」型泄漏不敏感**（只对「注入窗内单笔退款失败」敏感），登记 P2。第二方案（K1 上界收紧）转红，采信。

## §8.2 阈值文件自身的变异自校验（成对）

`threshold-mutation.txt` 全文留档。extractor 的 p95 门禁改为**从 thresholds.md 读**（阈值文件承重），然后：

1. 正常态（p95=22）：p95_fail = **4** 条。
2. 变异态（p95 临时改 30）：p95_fail = **0** —— 改错后判绿，**漏报得证**（4 条超限全部被放宽的门禁吞掉）。
3. 还原态（p95=22）：p95_fail = **4** 回归 —— 正确报警。

`thresholds.md` 还原后 `git diff` 为空，零残留。

## 结论与 P2 移交

- 三种破坏全部被门禁抓红（破坏 2 由像素断言兜住，计数门禁偏松）、还原后全部复绿、§8.2 漏报/报警成对成立——**P1 基线的门禁不是摆设**。
- P2 移交两项门禁覆盖面：① `gpu_ops` 计数下限 1 → 应收紧为用例精确 op 数；② oom probe 的 `pre==post` 语义对整体抬底型泄漏不敏感。

## 复现

```bash
# 破坏 1/2（补丁已还原；重放需按 patch.diff 临时打入后跑，跑完还原）
GPUI_NEG_SHIFT_1PX=1 go test ./render -run '^TestConformanceDualBackend$' -count=1   # 必红
GPUI_NEG_SKIP_DRAW=1  go test ./render -run '^TestConformanceDualBackend$' -count=1   # 必红
# 破坏 3（数据旋钮，可随时重放）
sed -i 's/"max_steady_bytes_delta": 131072/"max_steady_bytes_delta": 0/' render/testdata/conformance/solid_gradient.json
go test ./render -run '^TestConformanceDualBackend$' -count=1   # 必红；跑完 git checkout 还原
# §8.2（工具读 thresholds.md，改 22→30 跑一遍再还原）
python3 docs/render_gpu_converge/baseline/perf/tools/extract_perf.py
```
