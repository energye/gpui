# ui_text_edit_accept — 文本排版绘制验收主窗（M6，§5.6）

真实单行 + 多行输入框验收窗。控件只用 `ui/textinput` 引擎现货
（`NewInputBox` / `NewViewportInputBox` / `NewMultiLineInputBox`），本窗不实现输入框功能。

## 场景区

| 区 | 控件 | 预填 | 验证点 |
|---|---|---|---|
| A 单行短 | InputBox | 空 | 获焦/光标/手工键入 |
| B 单行超长 | ViewportInputBox | `testdata/b_5000.txt`（5000 字） | 横滚、末尾击键 |
| C 单行超长×10 | ViewportInputBox | `testdata/c_50000.txt`（50000 字） | 10× 量级对比 |
| D 多行短 | MultiLineInputBox | 空 | Enter 换行、上下键 |
| E 多行长文 | MultiLineInputBox | `testdata/e_1e5.txt`（1e5 字/~2000 行） | 纵滚、末行击键 |
| F 多行回绕 | MultiLineInputBox（wrap 开） | `testdata/f_wrap.txt`（2000 字长段） | 段首插入重排 |

预填一律存 `testdata/` 文件，`main.go` 无生成循环。

## 运行

```sh
go run ./examples/ui_text_edit_accept
GPUI_ACCEPT_RUN_SECONDS=30 go run ./examples/ui_text_edit_accept  # 门禁观察 30s
GPUI_ACCEPT_SELFTEST=1 go run ./examples/ui_text_edit_accept      # 无头 CPU 探针
```

## 人工步骤（§5.6.3）

1. 点 A 框，键入中英混排，看光标跟随。
2. 点 B 框末尾，连续快速击键 30 次，看是否迟滞。
3. 点 C 框重复步骤 2，对比 B 是否无感知差异。
4. 点 E 框滚到底部，在末行击键，看滚动与输入。
5. 点 F 框段落开头插入文字，看后续行重排是否错位。

## 门禁（§5.6.4）

`caret_vs_paint_max_delta_px ≤ 2`，`keystroke_p99_ms @C ≤ 16`，
`T(C)/T(B) ≤ 1.5`；A–J 全族随退出 JSON 输出。

## 基线趋势（M0-pre 取于 2026-09-03，改代码前）

| 指标 | 值 | 口径 |
|---|---|---|
| `caret_vs_paint_max_delta_px` | **821.4** | CPU 模型：布局末 caret X − 逐字 round 累加 X（B 框 5000 字） |
| `layout_ms_p99` | 6.06 | B 框 `BuildTextLayout` 31 次 p99 |
| `keystroke_p99_ms` | 4.80 | B 框 +1 字重排 31 次 p99（仅布局侧） |
| `paint_ms_p99` | 未验证 | 需 GPU 真窗 |
| `scroll_fps` | 未验证 | 需 GPU 真窗 |
| `interval_p95_ms` / `fps_interval` / `hitch_rate_per_min` / `rss_slope_kb_per_min` | 见下表真窗 30s | — |

## 真窗 30s（2026-09-03，M0 第 8–10 项之后，同一台机器）

| 指标 | 值 | 门禁 | 判定 |
|---|---|---|---|
| `fps_interval` | 59.3 | ≥ 55 | ✅ |
| `interval_p95_ms` | 17.15 | ≤ 22 | ✅ |
| `hitch_rate_per_min` | 0 | ≤ 5 | ✅ |
| `cpu_fallback_ops` | 0 | == 0 | ✅ |
| `bulk_taken`（B/C 全行字形非 0） | true | — | ✅ 批量分支已走通 |
| `caret_vs_paint_max_delta_px`（布局提交侧） | 0 | ≤ 2 | ✅（像素墨迹比对待补） |
| `keystroke_p99_ms` @C（50000 字整行重排） | 61.0 | ≤ 16 | ❌ M1 输入：整行重排太重，需行缓存 |
| `layout_ms_p99` @B（含首次冷整形） | 11.8 | — | 参考值 |
| `rss_slope_kb_per_min`（30s 含字体图集预热） | 52027 | ≤ 30000 | ❌ 口径问题：需 60s 稳态编辑复测（M3） |

## 真窗 30s（2026-09-03，M1 之后，同一台机器，`GPUI_ACCEPT_RUN_SECONDS=30`）

| 指标 | 值 | 门禁 | 判定 |
|---|---|---|---|
| `fps_interval` | 59.2 | ≥ 55 | ✅ |
| `interval_p95_ms` | 16.91 | ≤ 22 | ✅ |
| `hitch_rate_per_min` | 2.0 | ≤ 5 | ✅ |
| `cpu_fallback_ops` | 0 | == 0 | ✅ |
| `rss_slope_kb_per_min` | 13515 | ≤ 30000 | ✅（M0 的 52027 口径问题本轮未现） |
| `bulk_taken`（B/C 全行字形非 0 且脸可源） | false | — | 非门禁：复合脸 `Source()==nil` 是 M0 主动保留语义，批量路由属 M2（M0 的 true 系回退前旧态，本轮探针与引擎语义一致） |
| `caret_vs_paint_max_delta_px`（布局提交侧） | 0 | ≤ 2 | ✅ |
| `keystroke_p99_ms` @C（50000 字整行重排） | 69.4 | ≤ 16 | ❌ 非 M1 门禁：单行整行重整形 O(行长) 系 §3.4 设计使然（M0 记 61.0，同量级）；多行档 M1 门禁全绿见单测 |
| `layout_ms_p99` @B | 9.785 | — | 参考值（M0 记 11.8，-17%） |

## 真窗 30s（2026-09-03，M2 之后，同一台机器，`GPUI_ACCEPT_RUN_SECONDS=30`）

| 指标 | 值 | 门禁 | 判定 |
|---|---|---|---|
| `fps_interval` | 59.3 | ≥ 55 | ✅ |
| `interval_p95_ms` | 16.97 | ≤ 22 | ✅ |
| `hitch_rate_per_min` | 0 | ≤ 5 | ✅ |
| `cpu_fallback_ops` | 0 | == 0 | ✅ |
| `bulk_taken`（B/C 行：单脸批量或复合分脸批量均可路由） | true | — | ✅ M1 记 false，本期复合批量打通后翻 true；探针与新 Paint 路由同条件（见 `lineBulkRoutable`） |
| `caret_vs_paint_max_delta_px`（布局提交侧） | 0 | ≤ 2 | ✅ |
| `keystroke_p99_ms` @C（50000 字整行重排） | 55.8 | ≤ 16 | ❌ 非 M2 门禁：同 M1 备注（M1 记 69.4，同量级，系 §3.4 设计） |
| `layout_ms_p99` @B | 7.828 | — | 参考值（M1 记 9.785） |
| `gpu_ops` / `frame_raster_ms` | 1355052 / 19.4 | — | 参考值（`paint_ms_p99` 文本专项探针仍未接，用整帧光栅耗时代替，见未验证清单） |

> 本轮修过一次显示问题才达标：初版复合提交把绝对坐标的分区直接交 GPU，而 GPU 批量布局是按笔起点排的，导致各语种分区全叠在行首（英文压中文），有视口滚动的 B/C 框墨迹落在视口外、看起来是空的。修法是提交前把分区变基到自原点、提交原点平移同量（`rebaseGlyphs`，位置逐字形不变，`TestCompositeBatch_RebasedPositions` 锁拉丁+CJK+阿拉伯 RTL+泰文混排）。CPU 光栅路径一直是对的，所以单测全绿也没抓住，真窗人眼发现、修完后人眼复验确认不重叠、B/C 有字。

R5 对照窗（`GPUI_R5_SELFTEST=1`，3s）：9 探针全 true，`gpu_ops` 36034，`cpu_fallback_ops` 0；新增 `submitted_glyphs` 1061 / `vertex_count` 4244（CPU 侧提交观测，GPU 端精确顶点数未量）。

## 真窗 30s（M3 之后，同一台机器，`GPUI_ACCEPT_RUN_SECONDS=30`）

| 指标 | 值 | 门禁 | 判定 |
|---|---|---|---|
| `fps_interval` | 59.37 | ≥ 55 | ✅ |
| `interval_p95_ms` | 16.98 | ≤ 22 | ✅ |
| `hitch_rate_per_min` | 0 | ≤ 5 | ✅ |
| `cpu_fallback_ops` | 0 | == 0 | ✅ |
| `bulk_taken` | true | — | ✅ 与 M2 一致 |
| `caret_vs_paint_max_delta_px`（布局提交侧） | 0 | ≤ 2 | ✅ |
| `keystroke_p99_ms` @C（50000 字整行重排） | 57.9 | ≤ 16 | ❌ 非 M3 门禁：同 M2 备注（M2 记 55.8，同量级，系 §3.4 设计） |
| `layout_ms_p99` @B | 7.724 | — | 参考值（M2 记 7.828，同量级） |
| `rss_slope_kb_per_min`（30s 含图集预热 133→317MB） | 39184 | ≤ 30000 | ❌ 口径问题：同 M0/M2 备注，需 60s 稳态编辑复测；M3 单元证据见 `TestUndoDelta_MemoryCeiling_M3`（1e5 字 + 100 次 1 字节编辑，历史增长 100 字节） |

M3 单元证据（`ui/textinput`，本机）：`TestUndoDelta_*` 3 个全绿；`BenchmarkPushHistory_M3` 比值 `T(1e5)/T(1e3) ≈ 0.34 ≤ 2` ✅；
M3 补丁（2026-09-03，`ui/textinput/editor.go`）：`pushDelta` 对存入历史的 deleted/inserted 做 `strings.Clone`——此前子串与原文档共享底数组，每组名义 O(编辑量)、实际钉住整篇旧串（`TestUndoDelta_NoDocRetention_M3` 红灯复现：未修时 group 0 即命中原文数组；修后绿 + 堆增长 < 2MB + 全量撤销回初态）；`BenchmarkPushHistory_M3` 比值约 0.62 仍 ≤ 2，无性能回退。
M3 稳态复测（2026-09-04，同一台真窗机，`GPUI_ACCEPT_RUN_SECONDS=60` 实测非估算）：`fps_interval` 57.7 / `interval_p95_ms` 19.9 / `hitch_rate_per_min` 0 / `cpu_fallback_ops` 0 / `caret_vs_paint_max_delta_px` 0 全达标；`rss_slope_kb_per_min` 8895 ≤ 30000 ✅（rss 130→319MB，仍含一次性图集填充，斜率口径已排除首段预热尖峰）；`keystroke_p99_ms` 117.8 同 M2 备注非 M3 门禁。
M3.5 触发数据（同尺寸对照，`face==nil` 估算路径）：1e3 下拼接 11.4µs / 排版 98.6µs（拷贝约占 10%），1e5 下拼接 711µs / 排版 28.2ms（拷贝约占 2.5%），均远低于 30% 触发线 → M3.5 条件一不触发。

## M5 收口（2026-09-04，CPU headless，`GPUI_ACCEPT_SELFTEST=1`）

- `caret_vs_paint_max_delta_px` 821.5：headless 旧逐字模型复述值（M0-pre 同口径），不代表回退；真实一致性见 `TestPaintUsesShapedX_MultiFaceBulk` 与 `TestCaretMatchesPaint_M1`。
- `layout_ms_p99` 7.24 / `keystroke_p99_ms` 5.47（B 框 5000 字整形全重建口径；增量口径见 `TestKeystrokeRatio_M5`）。
- `paint_ms_p99` / `scroll_fps` / GPU 全族：见下 M5-GPU 行（已补跑）。

## M5-GPU（2026-09-04，真机，30s 与 60s）

| 指标 | 30s | 60s | 门禁 | 判定 |
|---|---|---|---|---|
| `fps_interval` | 59.37 | 59.19 | ≥ 55 | ✅ |
| `interval_p95_ms` | 16.99 | 17.01 | ≤ 22 | ✅ |
| `hitch_rate_per_min` | 0 | 0 | ≤ 5 | ✅ |
| `cpu_fallback_ops` | 0 | 0 | == 0 | ✅ |
| `bulk_taken` | true | true | — | ✅ |
| `caret_vs_paint_max_delta_px`（布局提交侧） | 0 | 0 | ≤ 2 | ✅ |
| `rss_slope_kb_per_min` | 41199 | 5175 | ≤ 30000 | 30s 含预热超，60s 稳态 ✅（与 M3 同口径注释） |
| `keystroke_p99_ms` @C（50000 字整行重排） | 64.5 | 77.1 | ≤ 16 | ❌ 存量设计（§3.4 整行整形），增量口径由 `TestKeystrokeRatio_M5` 守 |

像素：快照 `/tmp/accept_m5_rerun.png` 目检六区正常出字；与 M0 基线逐位差约 1.4%（>8 通道差 1.48%，3840 余字形行散布 + 底部动态条约 0.3%），差异为散布单字形抗锯齿级、无移位无缺字（差异图 `/tmp/accept_diffmap_rerun.png`，游程最长 20px，字形行每行 25–31 个短段）；M0 基线早于 M1–M4 提交路变更且从未刷新，已按 `LEGACY_03` 刷新归档为 `golden/baseline_m5.png`（独立评审通过），本窗 Golden 以新基线为准。

## 未验证清单

- 像素墨迹与布局的对照：B 框截图已量出 3242 墨点、左缘在框内 +3px（内边距），与布局一致；全框逐字对照待补。
- `paint_ms_p99` / `scroll_fps`（探针未接，M2 状态：仍用整帧 `frame_raster_ms` 代替记录，未冒充文本专项 p99；滚动帧率需 E/F 纵滚脚本，待 M4 虚拟化时补）。

## Golden 基线（2026-09-04 按 LEGACY_03 刷新为 M5）

- 文件：`golden/baseline_m5.png`（默认预填 B/C/F，E 为空，1200×800，M5 代码后；M0 时期旧基线已归档替换删除）。
- 复现：`GPUI_ACCEPT_RUN_SECONDS=8 GPUI_ACCEPT_SNAP=/tmp/x.png go run
  ./examples/ui_text_edit_accept`（需先 export 两个 lib 路径），逐像素对比。
- 容差：零容差（同机同字体）；换机器/换字体只做目检，不判失败。
- 稳定性：同条件两跑（`/tmp/accept_m5.png` vs `/tmp/accept_m5_rerun.png`）全幅差 0.35%，其中底部动态条（tick/HUD/光标闪烁相位）约 0.3%，其余静态区约 0.07%（光标闪烁相位差，非字形渲染差）。

## 已知问题（M2 范围）

- E 框（1e5 字/2000 行）整窗预填时首帧提交 10 万逐字字形，GPU 图集同步转圈（B/C/F 单框均正常，布局 328ms、数值干净）。
  M2 的裁剪覆盖多行 + M4 虚拟化就是治这个的，本窗不绕。
  默认不预填 E（M0 只看 B/C/F 真实）；分段跑：`GPUI_ACCEPT_ONLY=B|C|E|F`
  只预填一框，`GPUI_ACCEPT_FULL=1` 全预填（含 E，会卡），
  `GPUI_ACCEPT_NOPREFILL=1` 全空跑。
