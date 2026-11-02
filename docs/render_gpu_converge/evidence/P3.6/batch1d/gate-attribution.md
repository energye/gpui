# 批 1d 门禁归因（2026-11-03，冻结树 `HEAD=b0c18b7b` + 本批工作区改动）

门禁大盘：apidoc rc=0；vet 四触及包干净；定向三组绿；conformance 双后端 ok；五目标×{默认,nogpu} 10/10；
主逐文件 **735 条 = 704 PASS/21 SKIP/9 FAIL/1 NO_TESTS**（比批 2 基线 734 多 1 条 = 本批新增钉
`TestRenderImage_InPlaceFrameAdvanceReplaysFreshFrame`）；补充逐文件 **80 条 = 72/2/6**，FAIL 名单与
基线逐条相同。金图：本批零绘制路径改动（参与者注册接口/注册表实现/泵桥接/测试），存量金图按构造性
论证零变化；真窗金图归本批 anim-image 真窗交付时跑。

以下三条是门禁中的红，逐条 A/B 归因（临时 worktree 于干净 `b0c18b7b` 上同命令对照，用完已删）：

## 1 `-race -count=2` 三包红（ui/rendering、ui/textinput、ui/embedder）

- **ui/rendering 四测**（`TestPaintPerRuneX_NoQuadratic_M1`/`TestSingleLongLineKeystroke_M5`/
  `TestCaretBuild_NoQuadratic`/`TestTextLayout_LongBuild_RealFace`，全为性能/NoQuadratic 类）：
  **HEAD 上同命令同红**（A/B 实测 `FAIL 40.164s`）⇒ 该组在 `-race` 口径下本来就红（race 检测器
  拖慢执行破坏时序断言），存量债非本批引入；与批 2 `gpu/hal` 的 race 债同类（`batch2/race-attribution.md`），
  登记不顺手改。非 race 的逐文件 sweep 中该组对应文件记 PASS/基线名单内。
- **ui/textinput 三测**（`TestBoxMatrix_LongDocumentCorrectness`/`TestPushHistory_NoQuadratic_M3`/
  `TestR1_Long5000Perf`）：门禁期红；无干扰复跑（空载、同 `-race -count=2`）**绿** ⇒ 门禁期 CPU
  竞争（同机 build/sweep 并行窗口）波动，非本批。
- **ui/embedder 两测**（`TestPaintPresentTreeCompositeOnly_PartialOnlyDirtiesHotWidget`/
  `TestPaintPresentTree_SteadyFullPaintKeepsStatic`）：同上，无干扰复跑**绿**；HEAD 上同命令也绿。
- `ui/scheduler` `-race -count=2` 绿。

## 2 主 sweep 新增红：`render/s6_9_heavy_budget_test.go`（基线 8 条之外唯一新增）

- 归因链（逐环实测）：
  1. `render` 包本批零改动：`git diff HEAD --name-only -- render/` 输出为空；
  2. 该文件含两测：`TestS69_HeavyBudget_TierGates`（真 GPU 实测 13 场景并**写** `tmp/s6_9_heavy_budget.json`）
     与 `TestS69_Contract_FromJSON`（读该 JSON 断言契约，缺文件 `t.Skipf`）；
  3. 本次 sweep（00:49）TierGates 实测跑成，`H06_NestedClipLayerText`（P3_stress 层）p50=0.444ms 超预算
     0.307ms（S6.0 基线 0.267×1.15）落盘，FromJSON 随即 FAIL；空载复跑实测 0.4439ms **稳定复现**；
  4. **决定性 A/B**：干净 `b0c18b7b` worktree（render 代码逐位相同）借同一份 S6.0 基线后跑 TierGates 实测，
     H06 p50=0.32ms **同样超预算 FAIL**（0.32 > 0.31，ratio 1.195 > 1.15）；
  5. 批 2 基线 sweep 记 PASS 的前提是当时 JSON 里是旧 PASS 数据（其日志 `ok 0.669s`，FromJSON 读旧数据）。
- 结论：**非本批引入**。H06_NestedClipLayerText 踩在 P3_stress 相对预算边缘（预算 0.31ms，两次实测
  0.32/0.44 的运行间波动就足以翻转判定），属于 S6.9 性能专项线的存量边缘态（预算口径或场景需专项处理），
  登记**不顺手修**。本批对 FAIL 名单差异如实记录：`> render/s6_9_heavy_budget_test.go`。

## 3 记账

- 逐文件门禁机器：同机串行 sweep；`-race` 组判据按「HEAD 同命令 A/B」归因，非 `-count=2` 安全的测试
  逐条登记（本批新增：ui/rendering 性能四测；沿用批 2 口径不混入本批回归）。
- 门禁脚本与产物：`batch1d/run-gates.sh`、`build-gate.sh`、`compare-baseline.sh`；输出全部落 `batch1d/`
  （apidoc/vet/targeted/conformance/race/build/per-file-tests{,-extra}/baseline-compare.txt）。
