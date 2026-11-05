批 5 步骤 1 · game_fx 真窗金图复现性与接线实证（2026-11-05，当前桌面 Wayland+Xwayland，一轮一窗、`-auto-only` 自退）
================================================================================================

一、动手前实测：这扇窗的窗口金图本来就不可能逐位复现
------------------------------------------------
跑法：`-auto-only`（默认 RUN_SECONDS=8），窗口金图比对区 = y∈[32,800)×x∈[0,1200) 共 921600 px
（顶部指标带按 2.5D 摆法排除）。零容差 `goldenTolPct = 0.0`。

| 对照 | 结果 |
|---|---|
| HEAD 的基线 `fx_final_base.png` vs HEAD 自己的末帧 `fx_final.png`（同区） | **diff 2.9640%**（27316/921600） |
| 同一份代码连跑两轮的末帧互比（变异前） | **run2 vs run3 = 3.0270%**，**run3 vs run4 = 0.6745%** |
| HEAD 基线 vs 新代码末帧 | 100%（画面整体变了：泛光换核 + 场景在动） |

根因（实测到行）：`examples/engine/fx/main.go` 的 ticker 用墙钟累积量 `elapsed += dt` 推画面
（`shakeX = 6*sin(elapsed*22)`、`monX = mod(elapsed*60, …)`、每 0.12s 换一张预置水晃帧），
而停机是 `RunFor` 到点——**末帧画的是「这一刻」的相位**，与机器负载有关。所以
「static mask 零容差」这行注释在这扇窗里从来没成立过；本批之前没人二连过它（批 3 的 fail-closed
只改了 p31/p32/p33/p34 四窗，不含这扇）。

二、修法（拍板 A：收尾定格一帧再拍，改动只在窗自身的拍摄口径）
--------------------------------------------------------
1. **定帧停机**：门禁模式给 `PipelineOptions.MaxFrames = goldenFrames(240)`，常驻人工模式不限帧。
   harness 在 `presents >= MaxFrames` 处停（`ui/embedder/pipeline_app.go:1268`），末帧不再由墙钟决定。
2. **相位改吃「已完成帧数」**：`presentIdx = app.PresentCount()`，OnPaint 用
   `phase = presentIdx / goldenHz(60)` 折震屏/巡逻怪/水晃帧号——与 dt 无关。
3. **过 settling 帧定格**：`gateMode && presentIdx > goldenFreezeAt(120)` 时相位钉死（Flutter golden
   的冻帧做法）。这一步是实测逼出来的：只上 1+2 时仍出现两种末帧（10.8963% 与 0.0000% 并存），
   因为最后一笔的绘制时机存在收尾抖动；定格后 6 连跑，比对区**全部 0.0000%**。
4. **跑不满定帧 = 显式失败**：`app.PresentCount() < gateMaxFrames` 时报
   `FAIL 定帧停机没达成 presents=… want>=…`，把 `win_golden_diff_pct` 置 100 并非零退出——
   禁止拿墙钟随机帧当金图。多出一帧（presents=241）是 harness 收尾在飞的那一笔，画面早已定格，
   所以判据用 `<` 不用 `!=`；240/241 两类的 md5 差只落在被排除的 HUD 行（`presents=` 文案）。

三、第二个实测洞：窗口像素里根本没有 grade（用户拍板 a：本批接回）
-------------------------------------------------------------
变异实证（σ 2→3，其余不动）：

| 证据 | 接线前 | 接线后 |
|---|---|---|
| 离屏金图（96×64=6144 px） | **10.1562%** 报警 | 10.1562% 报警 |
| 窗口金图（921600 px） | **0.0000%（零敏感，假绿）** | **3.9265% 报警** |
| halo 探针 | 0.3438 → 0.3679 | 0.3438 → 0.3679 |

根因（实测到行）：`warpBufs[i]` 是水晃后的**未胶片**帧（`main.go:696`），窗口满幅取的就是它；
含 bloom 的 `gradeBuf` 只在 `cur == nil` 时当兜底（永不为 nil）；且 `caseName == "grade"` 反而
`cur = srcBuf`（原图）。 ⇒ 这扇窗对外写着「GRADE 胶片 Bloom暗角LUT映射」，窗口像素却不含胶片。
改法：预置 `warpGraded[i] = grd.Apply(warp 帧)`，满窗 case=all 取它；case=grade 取 `gradeForCase`
（静帧胶片），case=warp 仍取纯水晃帧（那一路本来就不该吃胶片）。custom 路不动。

四、接线后复现性（同一二进制，四连跑）
--------------------------------
```
run1 rc=0 presents=240 golden=0.0000% win_golden=0.0000%  md5(末帧)=d51320ccf0
run2 rc=0 presents=240 golden=0.0000% win_golden=0.0000%  md5=d51320ccf0
run3 rc=0 presents=241 golden=0.0000% win_golden=0.0000%  md5=87ee4b6fa9
run4 rc=0 presents=241 golden=0.0000% win_golden=0.0000%  md5=87ee4b6fa9
```
`parity=0.4514 moved=32499 corner=0.0717 halo=0.3438` 四轮逐字一致；比对区零差（md5 的两类差在 HUD 文案）。
Xwayland（PID 2938）跑前跑后都在，未出现崩溃；每轮单窗、`-auto-only` 自退，无 kill/pkill。

五、状态与后继
--------------
- game_fx 这扇窗的门禁从「不可复现的假零容差」变成「可复现且对泛光参数有鉴别力」；
  重基线只动了 `fx_grade_golden.png`（离屏，CPU 核换高斯所致）与 `fx_final_base.png`/`fx_final.png`（窗）。
  `fx_custom_*` 两张零改动（实测未参与本轮 case=all 的比对，mtime/大小未变）。
- 同形风险登记 **H15**（末帧随墙钟漂）与 **H16**（窗内像素不含被测能力），批 6 逐窗扫；
  批 6 ⑤ 的独立泛光窗照旧另开——本批状态仍按 §4.1 记 ☑ 已交付待金图（收口批=批 6 ⑤）。
