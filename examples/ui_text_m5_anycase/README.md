# ui_text_m5_anycase — M5 任意场景收口真窗

同窗四区：A 多脚本 · B emoji/组合字符 · C 富文本 run · D 多 DPR。
文本一律来自 `testdata/`，`main.go` 不发明语料。

## 改前基线（2026-09-04，headless CPU，`GPUI_M5_SELFTEST=1`）

| 指标 | 值 | 说明 |
|---|---|---|
| `first_screen_ms` | 1112.4 | A+B+C 三区布局一次构建（含冷字体），19 行 |
| `heap_delta_mb` | 683.6 | 单次值，含一次性字体/整形缓存，需复测细化 |
| `dpr_keys` | 16 | 1 字形×4 DPR×9 偏移去重后 16 键，未爆炸 |
| `keystroke_1k_ms` | 1.43 | 窗侧抽样（生产 RenderText 路径，5 次均值） |
| `keystroke_100k_ms` | 248.7 | 同上 |
| `g1_sample_ratio` | 174.1 | 窗侧抽样比值；**约束门以 `BenchmarkKeystroke` 为准** |
| 高棉/藏文 | 未测 | 字体缺失，待 M5-3 补 |

## 趋势（M5 收口时填改后值）

| 指标 | 改前（2026-09-04） | 改后（2026-09-04） | 说明 |
|---|---|---|---|
| `first_screen_ms` | 1112.4 | 900.9 | 含冷字体，差值在冷启动噪声内，不算收益 |
| `heap_delta_mb` | 683.6 | 683.6 | 含一次性字体/整形缓存，非稳态 |
| `dpr_keys` | 16 | 16 | 未爆炸 |
| `g1_sample_ratio`（窗侧整形抽样） | 174.1 | 128.4 | 仍超线性；约束门禁以 `TestKeystrokeRatio_M5` 为准 |
| 高棉/藏文 | 未测 | 已测 | `khmer_all`（128）+`tibetan_all`（256）整形单测绿 |

## GPU 真窗（2026-09-04，`RUN_SECONDS=30`，三跑 + 改前 A/B）

| 指标 | 改前 | 改后① | 改后② | 门禁 | 判定 |
|---|---|---|---|---|---|
| `fps_interval` | 58.76 | 58.76 | 58.94 | ≥ 55 | ✅ 三跑一致 |
| `interval_p95_ms` | 17.46 | 17.49 | 17.33 | ≤ 22 | ✅ |
| `hitch_rate_per_min` | 0 | 0 | 0 | ≤ 5 | ✅ |
| `cpu_fallback_ops` | 0 | 0 | 0 | == 0 | ✅ |
| `gpu_ops` | 463670 | 463670 | 465248 | — | 批量路稳定 |
| `time_to_first_present_ms` | 119 | 119 | 117 | — | 参考值 |
| `frame_raster_ms` | 4.42 | 8.03 | 3.13 | — | 跑间抖动，改前改后无系统差（见下） |
| `rss_end_kb`（30s） | 1554MB | 1561MB | 1548MB | — | 存量现象，改前同量级（见下） |

- 光栅说明：三跑 raster 为 4.42/8.03/3.13ms，首跑含冷图集填充；改前改后无系统差，彩色旁路无回归。
- 内存说明：三跑终点内存同为约 1.5GB，改前对照同量级——是存量一次性图集填充行为，非 M5 引入；E 族 30s 口径下超预算，记遗留（与 M3 同口径注释）。
- A/B 方法：`git stash` 引擎四文件后同窗重跑，`pop` 恢复；对照日志 `/tmp/m5_window_base.log`。

## 像素与 Golden（2026-09-04）

- 快照：`GPUI_M5_SNAP=path`（`RUN_SECONDS=30`）。基线 `golden/baseline_m5.png`。
- 复跑对照（同数据同代码，两跑，HUD 行 y≥726 遮掉后逐位比）：**差异 0px（0.0000%）**。
- F6 分区墨量（A 7.61% / B 5.98% / C 5.68% / D 2.62%，背景外像素占比）：四区均有墨 ✅。
- F0 背景点（四角与区内）：与壳背景一致 ✅。
- F7（细线/光标）：本窗无光标，不适用。
- 诚实项：B 区 emoji 字形本身无墨（wrkit 字体链无 emoji 字体，显示为空白/方框）——`IsColor` 标记与旁路由单测覆盖（DejaVu+NotoColorEmoji），端到端彩色像素待补彩色字体的链。
- CBDT 实测（2026-09-04，真机，不可用结论）：曾把 NotoColorEmoji（CBDT 位图）接进 B 区链——① 先崩：`DrawWithEmoji` 对 `face.Source()==nil`（MultiFace 复合脸）无 guard 空指针崩溃，已修（`render/text/draw_emoji.go` 加空回退，范式同 `tab.go`；回归 `render/m5_emoji_draw_test.go` 守卫摘掉即红、装回即绿）；② 修完仍不可用：CBDT 进 GPU 字形布局整段回退（`cpu_fallback_ops` 0→19 万、`last_cpu_fallback=text:glyphmask-layout`、hitch 0→4/min、CPU 42%→127%、光栅 4→33ms），且字形依旧空白。已回退（B 区用回公共链），回退跑重回全绿（fps 58.8/p95 17.4/hitch 0/零回退）。结论：端到端彩色像素需独立颜色管线，CPU 逐帧兜底此路不通。
