# ui_wr_c11_policy_switch — C11 R0→R4 策略切换

§3 C11 组合窗：full_paint↔retained 热切换正确（W6）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_c11_policy_switch
```

可选：`C11_SNAP_DIR` 覆盖快照目录（默认 `/tmp/c11_policy_switch`）。

## Window / Close duration

- `1200×800`，关闭用 `RUN_SECONDS=15`（§2.5 层 Present 档）。
- `RUN_SECONDS<5 → FAIL`（U16）。

## Visible effect

| Region | Expectation |
|--------|-------------|
| DENSE 左区 4×4 静色格 + 8 标签 | 全程静止，跨两次切换像素不变 |
| HOT 30×30 动块 | 每帧变色，切换前后无闪烁 |
| Policy banner | `POLICY: <当前策略> switches=N`，切换时翻转 |
| Phase 芯片 | STEADY→SPIKE→RECOVER，5s/5s/5s |
| 底栏 LiveHUD | 实时 policy/switch/mode/skip + PASS 预览色 |

相位脚本：Steady full_paint（0–5s）→ Spike retained（5–10s）→ Recover
full_paint（10–15s），两次策略切换。

## Gates

| # | 门禁 | 类型 |
|---|------|------|
| 1 | `policy_switches == 2` | 硬 FAIL |
| 2 | `seen_full_mode && seen_damage_mode`（两态路径都走到） | 硬 FAIL |
| 3 | `retained_phase_damage_avg ≤ 0.35` 且 `< full_phase_damage_avg` | 硬 FAIL |
| 4 | `boundary_skip ≥ 1`，`boundary_rerecord ≥ 1` | 硬 FAIL |
| 5 | 持续 tick：`fps_interval ≥ 55`，`p95 ≤ 22ms`，`hitch ≤ 5/min` | 硬 FAIL |
| 6 | `cpu` 非双 0（15s 窗只必采不断言上限，§2.2.3） | 硬 FAIL |
| 7 | 像素断言 2/2（静格绝对色 + 静区文本密度） | 硬 FAIL |
| 8 | Golden 静态掩码次跑起逐位一致（`diff == 0`） | 硬 FAIL |
| 9 | `vsync_source` 必有；`fallback` 禁止宣称锁 60Hz | 硬 FAIL |

## Measured（同机关闭档实测，非拍脑袋阈值）

2026-09-04，15s 关闭档两连跑（第 2 跑 PASS，Golden 对第 1 跑基线 `diff=0`）：

| 指标 | 值 |
|------|-----|
| `fps_interval` | 58.6–58.9 |
| `interval_p95_ms` | 17.3–17.6 |
| `hitch_rate_per_min` | 0–4 |
| `full_phase_damage_avg` | 1.0（全树重画语义） |
| `retained_phase_damage_avg` | 0.138 |
| `cpu_pct_avg` | 25–33（`cpu_ui` ~1–2，`cpu_raster` ~24–31，路径 proxy） |
| `cpu_fallback_ops` | 0 |
| `rss_slope_kb_per_min` | ~1.2e5（15s 窗处启动爬坡段，短窗只采不断言；见 §2.2.4） |
| `vsync_source` | true |

CPU 门禁说明：本窗不断言 CPU 上限（15s 短窗按 §2.2.3 只必采），上表为回归对照基线；
若后续同机同档 `cpu_pct_avg` 持续偏离该带 2× 以上，视为回归 FAIL 再议。
