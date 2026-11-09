# live 回放帧捕获（go/GL，GPUI_LIVE_DEBUG 临时钩子第 60 次 present 抓 FBO 0，诊断完钩子已删）

| 窗 | live vs 快照 diff(>8) | 结论 |
|---|---|---|
| ui_wr_p41_evenodd_msaa | 0.139% | ✓ 一致（HUD 数字摆动） |
| ui_wr_p41_fractional_clip | 0.219% | ✓ 一致（HUD 数字摆动） |
| ui_wr_p41_drawshadow | 23.515% | 色块在（live 帧含 (13,13,20) 块×3718 / (217,25,25)×2376），差异=阴影区：DrawShadow 无录制命令，live 回放显式降级无阴影（登记边界） |
| ui_wr_p41_mesh_paint_blend | 11.391% | 内容在；差异=批 1 在册 B 卡 #3（GPU 层链）双后端同败区 |

修复：四扇 p41 窗 OnPaint 绘制从 pc.LiveContext()（录制画布上断言 *render.Context 失败
静默跳过=live 全白根因）改为 pc.DC（render.Canvas 接口，live/录制两画布统一）。
