# ui_base_vram_cap

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_base_vram_cap`
- 运行：`批量(8 窗+OOM) A/60s`（mode=A，seconds=60，batch=True）
- 金图：`testdata/vram_win{1..8}_base.png`、`testdata/vram_oom_probe_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：X11/native

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| ladder | native | run | NO_JSON | 1 |  |  |  |  | 0 | 9 | no parseable JSON on stdout |
| ladder | go | run | NO_JSON | 1 |  |  |  |  | 0 | 9 | no parseable JSON on stdout |
| oom | native | run | PASS | 0 | 0.0000% | 19.25 | 21.07 | 57.41 | 1 | 9 | p99=21.07; vsync_source=true (not a source name) |
| oom | go | run | PASS | 0 | 0.0000% | 18.59 | 20.02 | 57.76 | 1 | 9 | p99=20.02; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `ladder` / `native` / `run`：no parseable JSON on stdout; FAIL: BASE_VRAM_CAP gate needs RUN_SECONDS>=300 (3 open batches + 3 phases)
- `ladder` / `go` / `run`：no parseable JSON on stdout; FAIL: BASE_VRAM_CAP gate needs RUN_SECONDS>=300 (3 open batches + 3 phases)

