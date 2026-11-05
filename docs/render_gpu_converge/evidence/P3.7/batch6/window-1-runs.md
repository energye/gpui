P3.7 批 6 · 窗 ① `examples/engine/p37_persp_batch` 跑数归档（2026-11-05）
==========================================================================

本文件是「原始跑数」，与 `window-1-persp-batch.md`（结论与判据）配套。
所有数字都取自当轮 stdout/JSON 或 `pngdiff` 逐位比对，出处按行标注。
比对区口径：整窗 1200×800=960000 px，金图 mask 掉指标带 y<32（38400 px），
故比对区 = 921600 px（`wrkit.GoldenGateW` 的 rect 就是这一条带，容差 0）。

一、金图门禁相（`-auto-only`，定帧 240 / 冻结 120）
--------------------------------------------------
| 轮 | 后端 | presents | 探针 | 金图 | 窗内合批 |
|---|---|---|---|---|---|
| pb_n1（播种前，mask 前形态）| native | 240 | 14/14 | FAIL 0.0951% / 921600 px | merged=10/7 |
| pb_o1（同上第二轮）| native | 240 | 14/14 | FAIL 0.0951% / 921600 px | merged=10/7 |
| pb_seed / pb_go_seed（`GPUI_ACCEPT_GOLDEN=1` 播种）| native / go | — | — | seeded=true → 明写「这一跑不算 PASS」 | — |
| pb_q1 / pb_q2 / pb_r（播种后复验）| native | 241 | 14/14 | 0.0000% / 921600 px | merged=10/7 |
| pb_v1 / pb_v2 | native | 240 / 241 | 14/14 | 0.0000% / 921600 px | merged=10/7 |
| pb_c1 / pb_c2 | native | 240 / 241 | 14/14 | 0.0000% / 921600 px | merged=10/7 |
| pb_go1 / pb_go2 | go（纯 Go GL）| 241 | 14/14 | 0.0000% / 921600 px | merged=10/7 |

- 0.0951% = 876 px，全部落在指标带（带内文案 `presents=%d persp_draws=%d max_run=%d`
  随帧数变），mask 掉之后比对区 0 差 ⇒ 「presents=240 与 241 末帧同画面」是冻结的
  结果，不是窗静止（见第三节）。
- `presents=240` 与 `241` 两类都出现：定帧停机点差一帧（快照重绘也过一次 OnPaint），
  冻结后两者同图。

二、双后端基线互比（同一台机、同一轮播种）
------------------------------------------
`go run /tmp/pngdiff` 逐位比两张基线：

    尺寸 1200x800；指标带(y<32) 差 0 px；比对区(y>=32) 差 0 px
    比对区零差

⇒ 整窗（含指标带）零差：同一份合并腿 + 同一份投影数学，webgpu-native 与纯 Go GL
   出同一张图。基线文件：`testdata/goldens/p37_persp_batch_final_native.png` /
   `..._go.png`。

三、H15（末帧可复现）正反两面
------------------------------
- 正面：门禁相同二进制连跑 6 轮（pb_q1/q2/r、pb_v1/v2、pb_c1/c2）全 0.0000%。
- 反面（画面真在动）：常驻相不限帧，9s 后取快照与冻结基线逐位比：

      尺寸 1200x800；指标带(y<32) 差 113 px；比对区(y>=32) 差 101473 px
      比对区差异行 426 个，样本 [323 324 325 326 327 328]

  ⇒ 比对区 11% 的像素在动，冻结机制是承重的（不冻结就没有可复现的末帧）。
- 口径注（如实记）：窗内那一行 `golden=…` 打的是**快照重绘帧**的相位——
  `SnapshotPath` 是停机后在 GL 线程重绘一遍树再读回（`ui/embedder/pipeline_app.go:1838-1860`），
  不是「最后一次 present 的那一帧」。门禁相靠 `idx` 冻结让二者同图；常驻相不限帧时
  这一行的数值只随重绘那一次的 `frameNo` 走，故本窗的「画面在动」以本节的外部逐位比对为准。

四、H16（金图有牙齿）四处变异实测
---------------------------------
| 变异 | 打回的证据 | 实测 |
|---|---|---|
| H16-a 焦距 520→560 | 金图 | `golden diff=5.7130% over 921600 px seeded=false err=<nil>` |
| H16-b 合并段改走仿射腿 | 探针 `merged_run_one_draw` | 红：draws=0 / max_run=0 |
| H16-c 染色 G 0.62→0.45 | 探针 `tint_lights_projective_leg` | 红：tinted=(189,85,53) want (189,117,53) |
| H16-d 定格帧 120→90 | 金图 | `golden diff=1.8793% over 921600 px seeded=false err=<nil>` |

另两处「非变异」诊断跑（用于定位自己写的两个缺陷，见结论文件第四节）：
`pb_amp` 把 pan 振幅放大到 180 → 比对区 1.1825%（10898 px，且只有图集带在动）
⇒ 证明 `plate()` 把屏幕落点当投影中心的 bug；`pb_fix1` 修复前后同基线比 → 33.1143%
⇒ 证明修复确实改变了画面落点（不是无操作修复）。

五、窗内合批账（H12 的硬断言，取自 JSON `ability_extra`）
--------------------------------------------------------
取自 `pb_go1.json` 的 `ability_extra`（go 相整块）；native 相的同一批读数以
`pb_manual.json`（常驻相）与各轮 console 行为准，两后端的这组数字逐项一致。

    merged_persp_draws=1  merged_persp_max_run=6      （a 合并段：6 张 1 次 draw）
    split_persp_draws=6   split_persp_max_run=1       （a 反面：夹描边后 6 张 6 次）
    merge_vs_split_diff_px=0                          （合批不改画面，逐位）
    equal_weights_diff_px=0  honest_perspective_diff_px=507
    atlas_persp_draws=1     atlas_persp_max_run=5     atlas_drawn=5 degraded=false
    tint_probe_rgba=[191,118,54,255]  plain_probe_rgba=[191,191,192,255]
    projection_stats={"clip_degrades":1,"interp_downgrades":0,"path_expansions":0,"text_skips":0}
    window_frame_persp_draws=10 window_frame_max_run=7 window_frame_draws=11
    window_frame_quads=26 window_leg_merged=true      （窗内真回放帧，非离屏代答）
    golden_tolerance_pct=0  golden_seeded=false  probes_ok=true  pixel_ok=true

`-auto-only` 停机条件里有一条硬门：`lastBatch.MaxPerspRun < 2` 直接 exit 1
（「窗内没有发生任何合并——H12 的硬要求落空」），所以这扇窗不可能在合并腿没跑时变绿。

六、常驻人工相（pb_manual，native）
-----------------------------------
    presents=4467  fps_wall=58.66  fps_interval=59.56  p50=16.72ms  p95=18.60ms  p99=20.17ms
    hitch_count=1  vsync_source=true  cpu_pct_avg=25.31  rss_end=365620KB
    事件计数：frame 2198 / wake 2196 / focus 1（`OnEvent` 逐条打日志并同步标题栏）

常驻相不限帧、收真事件，跑完自动正常关闭（`-manual-seconds` 超时 → `app.Run` 返回 →
`app.Close()` → `win.Close()`），不代替门禁相，只作人工核验载体。
