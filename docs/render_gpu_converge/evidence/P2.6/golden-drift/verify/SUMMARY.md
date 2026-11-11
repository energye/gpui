# 21 窗重种后复跑（2026-11-12）

- 采集：`run_windows.sh`，21 窗 × {native, go} = 42 轮，隔离 rootless Xwayland :3，`RUN_SECONDS=5 -auto-only`，窗口自行退出；每轮前后显示健康检查通过。
- 结果：**42/42 rc=0；42/42 `pixel_golden_diff_pct=0.0000%`；21/21 窗 native 与 go 内容区逐字节相同（`content_diff=0`，max Δ=0）。**
- webp 逻辑探针修正后：`logic_pass=true`、logic 19/19、pixel 24/24、golden 0.0000%。
- 原始输出：`run-index.tsv`、`runs/<窗>/<后端>/{stdout.json,stderr.log,rc,current.png,base.png}`。

## 负向对照（窗口级 1px 平移）

- 用 `ui_render_base_backdrop` + `SNAP_DIR` 临时目录，把新基线平移 1px：
  - 平移版：`diff=12.9901% over 627200 px` → `FAIL: pixel_golden_diff_pct=12.9901 want 0`，rc=1。
  - 还原版：`diff=0.0000% over 627200 px`，rc=0。
- 原始输出：`negative-shift1px.{stdout,stderr}`、`negative-restored.{stdout,stderr}`。

## 结论

21 窗金图重种后双后端全部归零；1px 负向能红；webp 逻辑门禁恢复全绿。P2.6「22 窗金图漂移」专项收口。
