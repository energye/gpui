# video_retained_golden

- 来源：`windows.md` §3.5（video）
- 目录：`examples/video/video_retained_golden`
- 运行：`A`（mode=A，seconds=None，batch=False）
- 金图：`testdata/showcase_retained_golden_dpr1_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：视频/录制 · 固定帧纹理层零容差

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% |  |  |  | 2 | 1 |  |
| default | go | run | FAIL | 1 | 78.9469% |  |  |  | 2 | 1 | exit code 1 |

## 失败原因（原始 stderr 摘录）

- `default` / `go` / `run`：exit code 1; golden diff 78.9469%; video_retained_golden: pixel blue                   fg  @( 849, 224) got=(0.000,0.718,0.000) want=(0.001,0.000,1.000) ok=false; video_retained_golden: pixel magenta                fg  @( 849, 552) got=(0.412,0.412,0.412) want=(0.999,0.000,0.996) ok=false; video_retained_golden: golden video_retained_golden_a.png: diff=78.9469% over 589908 px; FAIL: 颜色探针 0/4 达标

