# ui_wr_ime_r2_textlayout — IME R2 TextLayout 单源

Window: 1200x800, 手动关闭（无自动关闭，RunFor=0）。最小观察时长 RUN_SECONDS≥5（5000 长串建议观察 ≥15s）。Covers 7 scenarios:

1. 36×m 深部 5 位置×8 点往返 <0.5px, X 单调
2. 换行 "你好\n世界\nFlutter" affinity 上/下游 + 空行 "\n" 盒高
3. 粘滞列 caretCol 上下保持，Home/End 清
4. Fallback 混排 中文+😀+مرحبا Carets 不劈簇
5. HiDPI 1.25/2.0 1px 对齐 (F0-F9 采样)
6. MaxLines/Ellipsis …不进 Carets
7. 长串 5000 Build <100ms 且横滚 scrollX 与 Caret.X 联动

3 可输框（W2）：`boxSticky` 多行 280×72 14px + `boxLong` Viewport 560×36 12px（5000字）+ `boxThird` 单行 320×20 20px 混排 `Aa@10 你好@16 Hello@12`，共用 FocusManager/InputRouter/Clipboard/IME，点击获焦、打字、退格、方向键、粘贴均走 TextLayout 单源。

门禁（10.4.2 全族硬，A-J 全采）：

| 族 | 阈值 |
|---|------|
| A 帧时 | `fps_interval≥55`（持续 tick），`p95≤22ms`，`hitch≤2/min` |
| B 管线 | `build p95<3ms`，`raster p95<8ms` |
| C 脏区 | `paint_count` 可解释，`damage_ratio` 如实（full_paint 允许≈1） |
| D CPU | `cpu_pct_avg<50%`（5s），`<65%`（15s 长串）；`cpu_ui/raster` 必采 |
| E 内存 | 短窗 `<15s` 时 `slope_gate=off` 仅告警；长串 `≥14s` 时 `rss_slope<20000` 硬 |
| F GPU | `cpu_fallback_ops==0` |
| G 图文 | `measure_cache_hit` 必采且 `≥1` |
| H 启动 | `time_to_first_present<800ms` |
| J 正确性 | `grep MeasureWidth text_layout.go` 为空，`go vet 0` |

环境豁免：llvmpipe 软光栅（`frame_raster 20-50ms`）会使 `cpu_pct_avg 78-92%`、`rss_slope 50-70k` 超 D/E 阈值，属软渲染环境而非泄露，审计可设 `GPUI_R2_SOFTRAST=1` 临时豁免 CPU/RSS（真显卡仍硬卡 `cpu<65% / rss<20k`）。

Evidence: logic probe JSON (A-J) + pixel (PaintVisits, measure_cache_hit) + Golden snapshot.

Run: `go run ./examples/ui_wr_ime_r2_textlayout` 然后手动点 X 关闭（观察至少 5s，5000 长串观察至少 15s）；`RUN_SECONDS` 仅用于门禁校验最小观察时长，不触发自动关闭。软光栅环境：`GPUI_R2_SOFTRAST=1 go run ./examples/ui_wr_ime_r2_textlayout`
