# ui_wr_ime_r3_channel — IME R3 通道与平台

Window: 1200×800 手动关闭（无自动关闭，RunFor=0）。最小观察时长 `RUN_SECONDS≥10`（R3 10族全硬，U16底线5s）。Covers 9 scenarios:

1. 6信号 `preedit-start→change(+get_preedit_string)→commit→end` + `retrieve-surrounding` + `delete-surrounding` 仿真全通
2. 风暴锁 单键≤2 preedit（10ms内连击20键去抖）
3. `filter_keypress` 命中即 `return TRUE` 拦截，Home/End/PageUp/Down/Return分流（Return仅MULTILINE+换行才落）
4. `set_editing_state`二次覆盖：`SetText→{-1,-1}→0,0→SetText二次→SetSelection→composing`往返，`NonTextUpdate(deltaStart==-1)`仅`oldText==text`触发
5. `enableDeltaModel`定值不可变 + `apply` last-write-wins
6. surrounding 4000居中4锚点 + `-1` NUL结尾（Wayland/GTK共用）
7. `autofillHints`透传 + `inputAction/done/go/search/send`映射 `ContentType`
8. 死键 `´+e=é` 重音正确（`Xutf8LookupString`后`AddText`）
9. 双引擎 ibus/fcitx均过P1-P9 + 守护重启恢复/降级（仿真双路径）

4可输框（W2）：`box10` 10px `enableDelta=true autofill=username` + `box16` 16px主 `action=go` + `box20` 20px对照 `enableDelta=false action=search` + `multiBox` 14px多行 `action=send`，共用 FocusManager/InputRouter/Clipboard/IME，点击获焦、打字、退格、方向键、粘贴、拼音预编辑均走`TextLayout`单源。

门禁（10.4.3 10族全硬，零容忍）：

| 族 | 阈值 |
|---|------|
| A 帧时 | `fps_interval≥55` `p95≤22ms` `hitch≤5/min` |
| B 管线 | `build p95<5ms` `pipeline_depth`不超限 |
| C 脏区 | `damage_ratio`可≈1（full_paint）但`paint_count`可解释 |
| D CPU | `cpu_pct_avg<60%` |
| E 内存 | `rss_slope<15000` `rss_after_close`不高于peak |
| F GPU | `cpu_fallback_ops==0` `gpu_ops`稳定 |
| G 图文 | `measure_cache_hit`必采 ≥1 |
| H 启动 | `time_to_first_present<1000ms` |
| I 回归 | `baseline delta<10%` 必开（可选基线时） |
| J 正确性 | `go vet==0` + `filter_keypress`日志命中即拦截 |

W3手打必现：点不同字号框获焦→输`nihao`→选词上屏→Backspace缩拼音→Esc取消→Home/End→长文5000粘贴→4锚点拖动；死键`´`+`e`手打`é`验证；`Ctrl+C/V`在3框间切换验证多字段无残留。

Run: `go run ./examples/ui_wr_ime_r3_channel` 然后手动点X关闭（观察至少10s）；`RUN_SECONDS`仅用于门禁校验最小观察时长，不触发自动关闭。自检：`GPUI_R3_SELFTEST=1 go run ./examples/ui_wr_ime_r3_channel` 3s定式探针（CI用，不作关闭证据）。
