# ui_ant_verify

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_ant_verify`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/ant_verify_msaa{1,4}_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：混合 · MSAA 1x/4x

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| msaa1 | native | run | NO_JSON | 0 |  |  |  |  | 1 | 2 | no parseable JSON on stdout |
| msaa1 | go | run | NO_JSON | 0 |  |  |  |  | 1 | 2 | no parseable JSON on stdout |
| msaa4 | native | run | NO_JSON | 0 |  |  |  |  | 1 | 2 | no parseable JSON on stdout |
| msaa4 | go | run | NO_JSON | 1 |  |  |  |  | 1 | 2 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `msaa1` / `native` / `run`：no parseable JSON on stdout; ui_ant_verify: OK mode=1x 几何AA presents=301 px=821184 golden=0.0000%
- `msaa1` / `go` / `run`：no parseable JSON on stdout; ui_ant_verify: OK mode=1x 几何AA presents=300 px=821184 golden=0.0000%
- `msaa4` / `native` / `run`：no parseable JSON on stdout; ui_ant_verify: OK mode=4x MSAA presents=295 px=821184 golden=0.0000%
- `msaa4` / `go` / `run`：no parseable JSON on stdout; FAIL: golden diff=100.0000% (821184/821184 px) baseline=examples/ui_ant_verify/testdata/ant_verify_msaa4_base.png

