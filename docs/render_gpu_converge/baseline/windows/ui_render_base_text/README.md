# ui_render_base_text

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_text`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_text_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：文本 · 多语言样板

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | NO_JSON | 1 |  |  |  |  | 1 | 1 | no parseable JSON on stdout |
| default | go | run | PASS | 0 | 0.0000% | 19.62 | 26.29 | 59.91 | 1 | 1 | p99=26.29; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：no parseable JSON on stdout; ui_render_base_text: avg=16.69 p50=16.65 p99=25.69 hitches=7 vsync=true; ui_render_base_text: golden showcase_text.png: diff=0.0000% over 576000 px; FAIL: interval_p95_ms=25.05 > 22.00

