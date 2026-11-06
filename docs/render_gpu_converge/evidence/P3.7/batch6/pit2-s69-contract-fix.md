坑 2 修复正本（2026-11-07，结构修落地、冻结正本待另线后补；h23-fix.md §七.2 登记）
==================================================================================

被测坑（一句话）：`render/s6_9_heavy_budget_test.go` 同一文件既当测量者
（`TestS69_HeavyBudget_TierGates` 写 `tmp/s6_9_heavy_budget.json`）又当契约门
（`TestS69_Contract_FromJSON` 读同一份）⇒ 整包跑时 Contract 判的永远是
「本次机器状态」——自造假红。

一、实测账（四轮，全部可复跑；落盘 tmp/s6_9_heavy_budget.json）
--------------------------------------------------------------
| 轮 | 参数 | non-PASS 行 |
|---|---|---|
| 2026-11-06（整包负载） | warmup3/iters6 | 7 行（H01/H02/H03/H04/H05/H06/U05） |
| 2026-11-07 独立轮 1 | 默认（S6.0 基线 6 天前） | 3 行（H04/H06/U05，全是亚毫秒相对门） |
| 2026-11-07 轮 2（S6.0 基线刚刷新） | 默认 | 1 行（U05 p50=7.57 vs 基线 6.80+floor 7.10——相邻两轮抖 11%，门只容 +0.3ms） |
| 2026-11-07 轮 3-5（连试 3+大样本） | warmup8/iters16 | 1~3 行轮换红（U05/H04/H06 轮着来） |

**判定**：本机相邻两轮抖动稳定超出相对门带宽（亚 10ms 场景 ±10%+0.3ms floor）
⇒ 带宽本身过紧属**性能门重定标另线**，本坑不越权改门限；all-PASS 冻结对
在现有门限下取不到（5 次尝试全数落空）——用户拍板「落结构修，正本后补」。

二、修法（结构两半全拆，门限零改动）
------------------------------------
1. **writer→TempDir**：`s69JSONPath()` 删除，TierGates 落盘改
   `s69DumpPath(t)`——默认 `t.TempDir()`（整包跑不再改写仓内任何文件），
   `S6_9_JSON` env 保留给人工查看跑。
2. **reader→冻结正本**：Contract 改读 `s69ContractPath()` =
   `render/testdata/s6_9_heavy_budget_contract.json`（env
   `S6_9_CONTRACT_JSON` 可覆写）。正本语义=**门限语义冻结**——对一份受版本
   控的 all-PASS 测量重验 `s69ScenePass` 判定一致性（改门逻辑必翻车被钉），
   **不是**本机当前性能声明；文件自带 machine/date/warmup/iters 字段全透明。
3. **正本暂缺 → 具名 Skip**：`t.Skipf("no frozen S6.9 contract fixture …;
   capture after the perf-gate recalibration line")`——不静默假绿、不再
   自造假红（测试数据纪律：缺数据 Skipf 具名）。
4. **正本后补条件**：性能门重定标另线落定稳定带宽后，独立 unloaded 跑一轮
   all-PASS，`cp` 进 testdata 即转真；无需再改代码。

三、验收
--------
- TierGates 跑前后仓内 `tmp/s6_9_heavy_budget.json` mtime 不变
  （1793986034 → 1793986034）——writer 不再碰仓内文件。
- Contract：`--- SKIP: TestS69_Contract_FromJSON` 带具名原因——假红机制消失。
- TierGates 自身的相对门噪声红保持原样（那属「浮动计时门一律不作收口证据」
  在册族 + 门限重定标另线，本坑不背）。
- `go vet ./render` rc=0、gofmt 净。
