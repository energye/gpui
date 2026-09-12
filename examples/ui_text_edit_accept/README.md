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

## 剧本像素验收（2026-09-12，F 框尾部空白 + 拖选跳跃问题线）

- 跑法：`GPUI_ACCEPT_SCRIPT=1 GPUI_ACCEPT_RUN_SECONDS=22 go run
  ./examples/ui_text_edit_accept`（需先 `export LD_LIBRARY_PATH=$PWD/lib
  WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`）。窗内只做驱动加逻辑断言，
  像素全部交给窗外 `xwd -id` 实拍（窗内 `Image()` 回读按设计是空的，只能看逻辑；
  转码用 `/tmp/xwd2png.py`，本机无需 `--roll-left`，+769 循环偏移在此合成器路径下未现）。
- 三个稳态：s1 尾巴（F 框 25 个回车 + TAIL，滚到最大，散选）→ s2 全选高亮 →
  s3 B 框尾巴（横滚见证）。逻辑门禁（`SCRIPT_VERIFY pass`）：s1 滚到最大（±1px）、
  s2 选中全篇且三点击中配对 3/3、B 横滚 >0。
- 像素阈值（`golden/f_script/ref.json` 同值，同机同字体；边框走 F7 细线口径，
  高亮走 F5 混合口径，其余走 F6 文字区域级）：

| 稳态 | 断言 | 实测 |
|---|---|---|
| s1 | F 墨芯 `= (15,216,153)` ≥50px；底部 25px 墨 ≥100px（尾巴在底可见）；高亮 ≤20px（无渗选）；顶边框蓝线全行 | 120 / 209 / 4 / 88 of 88 ✅ |
| s2 | 高亮芯 `= (41,65,106)` ≥300px；墨芯 ≥50px；底部墨 ≥100px | 684 / 120 / 222 ✅ |
| s3 | B 墨芯 `= (12,191,242)` ≥500px，同族 ≥1500px，非底色 ≥10% | 1106 / 3290 / 20.8% ✅ |

- Golden：`golden/f_script/{s1,s2,s3}.png` 为 xwd 实拍裁片（旧片是窗内空回读，
  全黑作废已替换），`ref.json` 记数；复跑三片逐位 0.0% ✅（可复现门禁）。
- 六框全窗：B/C 浓墨 ~20.8%，F 3.0% 含尾巴，A/D/E 空框只有边框无杂字 ✅。
- 闪（问题一）：获焦 F 框 `xwd` 120×2 连抓 240 帧，平均亮度中位 35.4 上下只差 0.1，
  掉半帧 0 ✅；补丁 grep 门（`FRMDBG|PRSDBG|NOCLR|TMPDIAG|markTransparent|VisibleContent`，
  非测试代码）零命中 ✅。
- 查数脚本：`/tmp/accept_pixel_check.py`（阈值 + Golden 零容差），
  `/tmp/accept_flash.py`（亮度骤降扫描），`/tmp/accept_watch.py`（按 `SCRIPT_MARK` 跟拍）。
- 拍摄握手（`GPUI_ACCEPT_HANDSHAKE=1`，跟拍必开）：剧本原来按墙钟放行，
  跟拍晚到几秒就会拍到下一个状态（s1 曾拍到 s2 的高亮，查成高亮渗漏假阳性），
  泵卡住还曾把 s5 排到 s4 前面。现在每步多三道门——前一步已放行、
  前一步的帧已上屏（present 计数涨了，4s 兜底）、前一步的 mark 已被跟拍
  （`/tmp/accept_ack_<tag>`，8s 兜底），跟拍每拍一张就落一个 ack 文件。
  迟到 10s 再跟拍也能张张拍对：s1→s2 曾拉开到 8s 等 ack。
  握手只影响剧本节拍，逻辑断言口径不变；`SCRIPT_VERIFY` 末尾新增 `presents`
  数组（每步放行时的 present 计数，方便事后对时间线）。

## 中段空白修复（2026-09-12，A–F 全框问题线）

- 症状：框里字不管粘贴的还是原来就有的，从头翻到尾看一遍，中间一段是空白。
  真窗复现：B 框 5000 字滚到正中（`scrollX=28304`），整框墨点是 0（原来 1106）。
- 根：一坑两层。①场景层纹理拒存（超长段宽出屏幕）时没清旧条目，后面一直把
  尾巴那张旧图贴在中段位置上（贴出去 2 万多像素，框里全空）。②恢复帧直接画整窗
  却不经过留存纹理，脏标记被吃掉，后面几十帧都以为不用重录。
  修法（只动引擎层）：拒存即清旧条目（`ui/scene/textured.go`，清掉才回退矢量重画，
  跟刷帧失败的路子一致）；干净层几何对不上就重录（只比尺寸不比位置，滚动不误伤，
  静稳帧照样跳过）。
- 证据：修完 B 中段左中右墨数 1172/1176/1189（原来 0/0/0）；单测
  `TestRecordLocal_RefusalEvictsStaleEntry` + 真像素走查
  `TestScrollWalk_*`（头/四分之一/中段/四分之三/尾，段段有墨且相邻两站画面不一样）全绿。
- 剧本新加 s4（B 中段）、s5（F 中段）：逻辑只要求滚到中间（`>0`），像素看
  「每段有墨」（B 看左中右三段，F 看每行都有墨）。

## 可调窗口（2026-09-12）

- 窗口可拉（`Resizable: true`，标准 `OnEvent/EventResize → shell.Resize` 接线，
  跟 `ui_wr_r0/r21` 同款）。单行框定高 40 不变；D/E/F 三个多行框吃掉多出来的
  高度（默认 1200×800 时跟原来一模一样，基线图不用动）。
- 闲时收敛：合成器拉窗动画会经过一串中间尺寸，最后一下事件要是丢了就跟真相差半拍，
  2Hz 小巡检发现实际尺寸跟用的不一样就重摆一次。尺寸一变必约一帧（闲窗不定帧，
  不约就刷不出来）。
- 剧本新加 s6：自己调到 1400×950（跟手拉走同一条路），断言框变宽（B≥1000）、
  F 变高（>90），再外拍一张。
- 查数开关（默认全关，跟本窗以前的诊断开关同规矩）：`GPUI_ACCEPT_LAYOUTDBG=1`
  看每次摆布尺寸，`GPUI_ACCEPT_EVDBG=1` 看收到的事件，`GPUI_ACCEPT_DUMPTEX=dir`
  存留存纹理。

## 没修完、如实记下（大 resize 偶发画面定在中间尺寸）

- 现象（低频，看运气）：一次拉很大（1200×800 → 1400×950）后，布局、条目、逻辑
  全是 1080（对的），但画面上框只有 902 宽，右边一截没画（黑/灰垃圾）。
  小步拉（+80/+60）稳，多跑几次大步拉有时全对有时定住。
- 已排除的：布局没跑（跑了，尺寸 1080 有日志）、条目没录（录了，1082 条目导得出）、
  事件没收到（收到了，1400 回调有日志）、脏标记（恢复帧 28 个脏层全在）。
- 还在查的：合成器换链子那一下（X11 靠拿缓冲过时信号重配，信号丢了链子就还是
  1200×800，画面自然被砍在 1200）。这是 `render/`  swapchain 层的事，按规矩要
  单独立项、协议级取证，不在这轮里硬动。
- 2026-09-12 S6 黑边已修（`render/present_target.go`，wr-engine 立项）。
  根因（协议级钉死）：X11 下 resize 只记标记、等拿缓冲过时再重配，但程序点
  resize 后拿缓冲一次都不失败（整轮零 `acquire-fail`/`retry-cfg`），链子永远
  是旧尺寸，1400 的逻辑画进 1200 的缓冲，超的全黑。
  对齐 Flutter/Impeller（`khr_swapchain_vk.cc`：拿成功了还要比尺寸，
  `!out_of_date && size_ == GetSize()` 不成立就重建）：每帧拿完比一次，
  对不上就按记录尺寸重配+重拿（每帧最多一次，跨帧收敛；Impeller 无风暴暂缓
  概念，首版的风暴窗暂缓已删除——暂缓让手拖大 resize 步步留白）。
  稳态无 resize 时只是两次整数比较。证据：`force reconfig` 一次 →
  `sc.frame 1400x950 (forced)` → 后续全 1400x950；s6 边框 108/108，
  F 框 7 行零空行；确认轮 Golden 三张 0.0% 全绿。
  拖拽风暴轮：s6 前改成 5 步风暴（1240x830 → 1400x950，每 tick 一步），
  日志显示每步 `force reconfig` 一次、画出的帧尺寸永远等于逻辑尺寸
  （1240→1320→1400，步步跟上，零裁剪帧）；s6 边框 108/108，确认轮全绿。
  回归：render 包与修前基线失败集完全一致（AA/描边/混合等项，环境敏感+
  存量问题，非引入）；ui/embedder、ui/textinput、gpupixel、ui/scene 定向全绿。
  公开 API 零增删，目录文档不用动。
- Vulkan 现状定档（2026-09-12，用户决策：不切 GL）：风暴跟手做到「步步对版、
  零空白」即达标，帧率看硬件——本机（Intel Mesa Vulkan）每步 = 换链 15–30ms
  + 整幅重画 25–50ms，风暴期约 12–25 帧/秒；真显卡上换链 1–5ms。
  Skia 系软件（Chrome/VSCode）在 Linux 走 GL/EGL、resize 无重建开销，所以更滑；
  Impeller（Vulkan）与我们同成本 profile。计时器现状：200ms（回 Fifo）、
  300ms（换链后全幅）、500ms（gpu 重配限流）都不卡收敛，只保正确性。
- 跑 `git log` 认准：本轮只动 `ui/scene/textured.go`（拒存清旧 + 几何对不上重录）、
  `ui/textinput/box.go`（尺寸跟进裁剪/视口 + 变尺寸重画），不动渲染后端。

## 已知问题（M2 范围）

- E 框（1e5 字/2000 行）整窗预填时首帧提交 10 万逐字字形，GPU 图集同步转圈（B/C/F 单框均正常，布局 328ms、数值干净）。
  M2 的裁剪覆盖多行 + M4 虚拟化就是治这个的，本窗不绕。
  默认不预填 E（M0 只看 B/C/F 真实）；分段跑：`GPUI_ACCEPT_ONLY=B|C|E|F`
  只预填一框，`GPUI_ACCEPT_FULL=1` 全预填（含 E，会卡），
  `GPUI_ACCEPT_NOPREFILL=1` 全空跑。
