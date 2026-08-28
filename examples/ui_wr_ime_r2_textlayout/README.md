# ui_wr_ime_r2_textlayout — IME R2 TextLayout 单源

Window: 1200x800, 手动关闭（无自动关闭，RunFor=0）。最小观察时长 RUN_SECONDS≥5（5000 长串建议观察 ≥15s）。Covers 7 scenarios:

1. 36×m 深部 5 位置×8 点往返 <0.5px, X 单调
2. 换行 "你好\n世界\nFlutter" affinity 上/下游 + 空行 "\n" 盒高
3. 粘滞列 caretCol 上下保持，Home/End 清
4. Fallback 混排 中文+😀+مرحبا Carets 不劈簇
5. HiDPI 1.25/2.0 1px 对齐 (F0-F9 采样)
6. MaxLines/Ellipsis …不进 Carets
7. 长串 5000 Build <100ms 且横滚 scrollX 与 Caret.X 联动

Evidence: logic probe JSON (A-J) + pixel (PaintVisits, measure_cache_hit) + Golden snapshot.

Run: `go run ./examples/ui_wr_ime_r2_textlayout` 然后手动点 X 关闭（观察至少 5s，5000 长串观察至少 15s）；`RUN_SECONDS` 仅用于门禁校验最小观察时长，不触发自动关闭。
