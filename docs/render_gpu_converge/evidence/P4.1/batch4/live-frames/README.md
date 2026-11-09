# live 回放帧捕获（go/GL，GPUI_LIVE_DEBUG 临时钩子第 60 次 present 抓 FBO 0，诊断完钩子已删）

| 窗 | live vs 快照 diff(>8) | 结论 |
|---|---|---|
| ui_wr_p41_evenodd_msaa | 0.139% | ✓ 一致（HUD 数字摆动） |
| ui_wr_p41_fractional_clip | 0.219% | ✓ 一致（HUD 数字摆动） |
| ui_wr_p41_drawshadow | 23.515% | 色块在（live 帧含 (13,13,20) 块×3718 / (217,25,25)×2376），差异=阴影区：DrawShadow 无录制命令，live 回放显式降级无阴影（登记边界） |
| ui_wr_p41_mesh_paint_blend | 11.391% | 内容在；差异=批 1 在册 B 卡 #3（GPU 层链）双后端同败区 |

修复：四扇 p41 窗 OnPaint 绘制从 pc.LiveContext()（录制画布上断言 *render.Context 失败
静默跳过=live 全白根因）改为 pc.DC（render.Canvas 接口，live/录制两画布统一）。

## 补记（2026-11-10 第二轮）：drawshadow 块错位修正

- 用户人工窗第二轮报「四块白+黑红块叠窗口左上角」→ 实锤 `dc.SetPath(rect)`（raw，
  不吃当前矩阵）在卡片 Translate(origin) 下错位到窗口原点，**快照/live 同错**，
  旧金图为错位画面自洽种子（门禁绿=与错位基线一致）。
- 修=两处 `SetPath`→`DrawPath`（变换感知）；双后端重种+verify 0.0000%；
  go/GL live 抓帧（钩子已删）：左上角块残留 0、A 卡块位 (13,13,20)×2574 命中、
  live-vs-golden 20.185%=阴影降级区（上条登记不变）。
