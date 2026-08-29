# ui_wr_ime_r5_control — IME R5 控件与滚动

Window: 1200×800 手动关闭（无自动关闭，RunFor=0）。最小观察时长 `RUN_SECONDS≥15`（滚动 30s，10族全硬）。Covers 7 scenarios:

1. 单行 5000 字横滚 `scrollX + SetOffset(-scrollX)`，点击 `localX+scrollX` 命中 `ByteOffsetAtPoint`
2. 组合期横滚：preedit `拼音|` + `scrollX` 仍露出候选光标，`IMERect` 实报 composing 框
3. 粘性 `caretCol` 在滚动后仍保持
4. 多行 `MaxLines/Ellipsis` 与编辑共存：`…` 像素 + `…` 不进 `editable_range`
5. 占位符 `hint`（空+未聚焦才显，获焦或有字即隐）+ 禁用态样式像素
6. 单行拒 `\n`、多行允 `\n`，`MaxLines` 裁剪不影响 `TextRange`
7. 首帧 `warmup:true` + 滚动首帧有内容（`time_to_first_present<800ms` 且有内容）

5可输框（W2）：`boxLong` 5000 Viewport 12px + `box10` 10px + `box16` 16px主 + `boxDis` 12px禁用+占位 + `multiBox` 14px多行 MaxLines=2 Ellipsis，共用 FocusManager/InputRouter/Clipboard/IME，BaseEditable 四件套 `Editor/IMERect/ContentType/DrawPreedit` ≤15行接入。

门禁（10.4.5 10族全硬，F零容忍，G/H硬）：

| 族 | 阈值 |
|---|------|
| A 帧时 | `fps_interval≥55` `p95≤22ms` `hitch≤5/min` (30s) |
| B 管线 | `build p95<4ms` `raster p95<10ms` |
| C 脏区 | `damage_ratio<0.3` (30s横滚) |
| D CPU | `cpu_pct_avg<65%` (30s) |
| E 内存 | `rss_slope<15000` (30s硬) |
| F GPU | `cpu_fallback_ops==0` |
| G 图文 | `measure_cache_hit≥1` 必采 |
| H 启动 | `time_to_first_present<800ms` 且首帧有内容 硬 |
| I 回归 | `baseline<10%` 必开 |
| J 正确性 | `BaseEditable≤15行` + `vet==0` + `单行拒\\n` 硬 |

W3手打必现：点5000框横拖→输 `你好Hello` 横滚→拼音`nihao`预编辑露出→↑↓粘滞→多行Ellipsis→空框占位隐显→禁用框灰显→单行\\n被拒/多行\\n可入。

Run: `go run ./examples/ui_wr_ime_r5_control` 然后手动点X关闭（观察至少15s，滚动建议30s）；`RUN_SECONDS`仅用于门禁校验最小观察时长，不触发自动关闭。自检：`GPUI_R5_SELFTEST=1 go run ./examples/ui_wr_ime_r5_control` 3s定式探针（CI用，不作关闭证据）。
