# p37_persp_batch — P3.7 批 6 窗 ①：透视批绘腿（合批 / 染色 / 四角+权重 / 投影层）

一句话：把「N 张连续同透视 quad 必须合成 1 次 draw」画成同屏正反两半，再用逐帧合批账本把它钉死。

收口能力：批 1a（透视多 quad 合批放行）· 批 1b（透视腿染色 + 采样核具名拒绝）·
批 1c（批绘腿吃「四角+权重」，经 `camera.ProjectSpriteQuad` 引擎出口）· 批 1d（Canvas 级 4×4 投影层四动词）。

## 跑法

```sh
WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so go run ./examples/engine/p37_persp_batch -auto-only
RUN_SECONDS=8 go run ./examples/engine/p37_persp_batch -auto-only
GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_persp_batch -auto-only   # 第二后端一轮
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_persp_batch -auto-only  # 播种，仍不算过
go run ./examples/engine/p37_persp_batch -manual-seconds 30                          # 常驻人工窗
go run ./examples/engine/p37_persp_batch                                             # 常驻到关窗
```

- `-auto-only`：门禁相。定帧 `gateFrames=240` 停机 + 相位过 `gateFreezeAt=120` 定格
  （H15：末帧与墙钟无关）；三证据（15 枚逻辑探针 + 语义像素 + 逐位金图）不过直接 `exit 1`。
  停机没跑满 240 帧也算失败，不静默放行。
- `-manual-seconds N` / 不带旗：常驻相，不限帧，收真事件即打日志并用 `SetTitle` 显示累计数，
  结束出汇总 JSON。**常驻相的金图行只报数不判绿**（末帧未定格），它的作用是人工核验与收真事件。
- `-out <path>`：把门禁 JSON 另写一份。

## 画面（1200×800，标题 `gpui p37_persp_batch — 透视批绘腿`）

满窗即内容，指标行浮在左上角（`presents=… persp_draws=… max_run=…`，金图 mask 掉 `y<32` 那条带）：

- A 段 **合并段**：六张同透视 quad 连排，中间只有图像命令 → 必须 1 次 draw。
- B 段 **夹路径段**：同样六张，每张之前先画一条落在间隙里的分隔刻度 → 六张六次 draw。
  A/B 两半画在同一个面板落点区上，逐位必须相同（合批不改画面）。
- C 段 **染色段**：六张里前三张 `Tint=(1,0.62,0.28)`（琥珀），后三张零值不染色。
- 中列 **等权≡仿射 / 真透视≠仿射**：同一张图同一副四角，`Perspective:true + Weights 全 1` 与
  `Perspective:false` 逐位相同；梯形权重参与 1/w 有理插值后必须不同。
- 底带 **拒绝态**：透视 + bicubic 具名拒、非法权重具名拒、仿射 + bicubic 合法、
  深度压到焦点之后（眼后）整块拒绝且该格保持底色、NaN 深度引擎出口 fail-closed。
- 图集带 **批绘腿**：`camera.NewProjector` + `ProjectSpriteQuad` 产四角与权重 → `DrawAtlasEx`
  （老口按批 1c 的 D-4 契约不读精灵自带透视）。
- 右上 **投影层**：`PushProjection(Matrix4)` + `ClipRoundRect` 画远处告示牌，`Pop` 后 `Projection()` 回恒等。
  同段多两笔文字做 fail-closed 对照：投影态那一笔必须**整笔缺席**（`TextSkips++`、条带零墨差），
  `Pop` 后恒等段同一串字必须**正常在场**（条带墨差 >40）。

## 三证据

- 逻辑探针 15 枚（`merged_run_one_draw` / `interleaved_path_breaks_run` /
  `merge_keeps_picture_bitwise` / `tint_lights_projective_leg` / `equal_weights_equal_affine` /
  `honest_perspective_differs` / `perspective_bicubic_refused` / `bad_weights_refused` /
  `behind_eye_refused` / `behind_eye_painted_nothing` / `affine_bicubic_still_draws` /
  `atlas_engine_leg_merges` / `projection_stack_push_restore` / `engine_export_rejects_nonfinite` /
  `text_under_projection_failclosed`）。
- 语义像素：从画面所用同一份投影输出反推落点，容差显式写死（`tintSlackSteps=6` 档；
  合批≠改画面那条用 `mergeBitExact=0` 零容差）。
- Golden 逐位零容差，按后端分文件：`testdata/goldens/p37_persp_batch_final_native.png` /
  `..._go.png`。**fail-closed**：缺基线判红；`GPUI_ACCEPT_GOLDEN=1` 播种那一跑也不算过。

## 窗内合批账（H12 的硬断言）

`Context.ImageBatchStats` / `ResetImageBatchStats`（目录 §3.17）逐帧读-清。
`-auto-only` 里有一条硬门：`MaxPerspRun < 2` 直接 `exit 1` —— 这扇窗不可能在合并腿没跑过时变绿。
实测 `window_frame_persp_draws=10 / window_frame_max_run=7 / window_leg_merged=true`。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=p37-6-1`，`Scenario=p37_persp_batch`，
`Extra` 带 `probes_ok` / `pixel_ok` / `golden_diff_pct` / `golden_total_px` / `golden_seeded` /
`golden_backend_tag` / `merged_persp_draws` / `merged_persp_max_run` / `split_persp_draws` /
`split_persp_max_run` / `merge_vs_split_diff_px` / `tint_probe_rgba` / `atlas_*` /
`projection_stats` / `window_frame_*` / `window_leg_merged` / `gate_frames` / `gate_freeze_at`。

## 已知边界

- Metal 后端本机不可验，按跨平台标记纪律记「待验」。
- 批 3e-1/3e-2/3e-3（scene 投影口 / scene 四角+权重 / scene item 级材质）**不在本窗收口**：
  批 3e-2 把「`engine/`、`ui/` 对 `render/scene` 零引用」钉成结构约束，这三行只能由 scene 侧窗收口
  （批 6 第 ⑥ 扇，见 `docs/render_gpu_converge/evidence/P3.7/batch6-window-plan.md` §5）。
- 帧耗时/p95 只报数不判绿（性能门重定标按同日拍板不排在批 6 之前）。
