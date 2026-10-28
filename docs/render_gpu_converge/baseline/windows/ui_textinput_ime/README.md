# ui_textinput_ime

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_textinput_ime`
- 运行：`A`（mode=A，seconds=None，batch=False）
- 金图：`testdata/textinput_ime_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：X11/native

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  |  |  |  | 1 | 1 | missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| default | go | run | PASS | 0 |  |  |  |  | 1 | 1 | missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |

