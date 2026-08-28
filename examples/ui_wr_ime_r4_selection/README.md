# ui_wr_ime_r4_selection — IME R4 选区与编辑

Window: 1200×800 手动关闭（无自动关闭，RunFor=0）。最小观察时长 `RUN_SECONDS≥10`（R4 10族 A/C/D/E/J 硬，U16底线5s）。Covers 8 scenarios:

1. 双击选词/三击选段（`SelectWordAt` 词边界 `Ctrl+←→`，`cjk3000.txt` + `latin_all.txt` 抽样）
2. Shift+方向扩展 + 鼠标拖选跨行（行盒并集 `BoxesForRange` 来自 TextLayout）
3. 密码 `PurposePassword` 圆点掩码像素 + 关预测，`BeginComposing` 直接拒绝（日志无候选）
4. 只读/`input_type==NONE` 禁写但 `MoveCursor` 仍可在 `editable_range`，`hide` 即 `focus_out`
5. 撤销分组：一次组合（`Begin→Update*→Commit`）=1 步，组合中 `Backspace` 不另起步（F-C2）
6. 词删除 `Ctrl+Backspace/Delete` 限 `editable_range`
7. 锚点 `set_cursor_rectangle` 仅 `composing` 实报 1 次/变更，非 composing 0 上报，`OnChange→RefreshIMEAnchor` 闭环（含程序化 `SetText`）
8. `AddText` 原子替换 `was_composing?composing_before:selection_before`（F-S3）

5可输框（W2）：`box10` 10px + `box16` 16px主 + `box20` 20px密码 `PurposePassword` + `multiBox` 14px多行 + `roBox` 12px只读，共用 FocusManager/InputRouter/Clipboard/IME，点击获焦、打字、退格、方向键、粘贴、拼音预编辑均走`TextLayout`单源。

门禁（10.4.4 10族全采，A/C/D/E/J 硬，B/G/H 全硬，F零容忍）：

| 族 | 阈值 |
|---|------|
| A 帧时 | `fps_interval≥55` `p95≤22ms` |
| B 管线 | `build p95<4ms` |
| C 脏区 | `damage_area_px ∝ 选区并集` 硬 |
| D CPU | `cpu_pct_avg<55%` |
| E 内存 | `rss_slope<15000` |
| F GPU | `cpu_fallback_ops==0` |
| G 图文 | `measure_cache_hit`必采 ≥1 |
| H 启动 | `time_to_first_present<1000ms` |
| I 回归 | `baseline <10%` 可选 |
| J 正确性 | `composing && !collapsed → false` 硬 + `vet==0` |

W3手打必现：点不同字号框获焦→双击选词/三击选段→Shift+←→扩展→拖选跨行→Ctrl+BS删词→密码框输`ni`被拒→只读框移动→粘贴→拼音`nihao`原子替换；锚点在composing时实报（mock计数），非composing预热0上报。

Run: `go run ./examples/ui_wr_ime_r4_selection` 然后手动点X关闭（观察至少10s）；`RUN_SECONDS`仅用于门禁校验最小观察时长，不触发自动关闭。自检：`GPUI_R4_SELFTEST=1 go run ./examples/ui_wr_ime_r4_selection` 3s定式探针（CI用，不作关闭证据）。
