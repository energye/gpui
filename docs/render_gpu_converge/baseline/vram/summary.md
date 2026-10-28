# P1.6 显存账本快照

> 生成：2026-10-29；HEAD `a4d4045d`；执行 worktree `/home/yanghy/app/projects/gogpu/gpui-p14`（同 HEAD）；
> 隔离显示 `DISPLAY=:3`（rootless Xwayland，未使用当前 `:1`）；单进程串行、轮前健康检查、标准关闭（WM_DELETE→SIGTERM 兜底，未用硬杀）。

## 范围与口径

- 挑窗依据 `windows.md` §0.6 能力标签 `显存/生命周期`：`ui_base_batch_cap`、`ui_base_multiwin_cap`、`ui_base_vram_cap` 三窗全协议 × native/`GPUI_BACKEND=go` 两档（go 只是呈现层切换，`gpu_backend` 仍 integrated）。
- 切场景账本由 P0.4 一致性测试台复跑承担（3 场景逐场景采样）；全窗 RSS 关窗回落为附属盘点（P1.4 存量数据，零 GPU）。
- 账本真源：`gpu/hal/vram_ledger.go`（预算 768 MiB，`GPUI_VRAM_BUDGET_MB` 可覆盖），经 `render.VramLiveBytes()` 等转发读取；窗内由窗口自身协议采样并写进 JSON。
- 阈值真源：`baseline/thresholds.md` §4（预算 768 / 水线 0.9 / 低显存档 0.8 / 八窗带 362.9–381.4MB 上沿 +10%）。

## 1. 三 cap 窗全协议（8 轮）

| 窗/变体 | 后端 | 状态 | 账本关键数 | 判读 |
|---|---|---|---|---|
| vram_cap ladder（RUN_SECONDS=300） | native | PASS | 账本峰值 **372.0MB**，水线 0.9×768=691.2 未破（占 54%）；拒增曲线 `[2,0]→[4,550]→[8,747]` 在 4/8 窗触发；crash_free；hold 段斜率 1533KB/min ≪ 30000 泄漏线 | 峰值在资源矩阵带 362.9–381.4 内（上沿+10%=419.5）；门禁全绿 |
| vram_cap ladder | go | NO_JSON | **跑不了的档**：开到第 4 扇窗时 `eglMakeCurrent failed: error 0x3002`（stderr 有原始证据） | GL 后端 8 窗阶梯跑不了，如实登记；诊断归 P2（gles 多窗上下文），不在 P1.6 修 |
| vram_cap oom（GPUI_FAULT_OOM=180:1, RUN_SECONDS=60） | native | PASS | 注入前 8.5459MB → 峰值 12.06MB → 恢复后 **8.5459MB（delta 0.00）**，`ledger_recovered=true` | OOM 注入后账本精确回落，资源谁建谁放成立 |
| vram_cap oom | go | PASS | 同上，8.5459→12.06→8.5459MB，逐位与 native 一致 | 双后端一致 |
| multiwin_cap（8 窗共享设备, RUN_SECONDS=75） | native | PASS | crash_free、8 窗上限未触顶；RSS 关窗后 +206.4MB（Go 堆常态，见 §3） | 窗口自身门禁全绿（含像素 0 diff） |
| multiwin_cap | go | PASS | 同上 +212.6MB | 双后端一致 |
| batch_cap（合批上限, RUN_SECONDS=200） | native | FAIL | **p95 55.7ms > 22ms 门禁**（P1.4 同窗 FAIL p95≈53.1，同因同量级） | 性能 FAIL 如实登记不修；归 P2 性能线（P1.7 性能基线接手） |
| batch_cap | go | FAIL | **p95 62.1ms**（P1.4 go≈63.2） | 同上 |

> ladder 秒数说明：P1.4 用 `A/60s` 跑出 ladder NO_JSON（rc=1）是**秒数不够饿死门禁**（`gateSeconds=300` 写死在 `examples/ui_base_vram_cap/main.go:63`）；本轮按代码门禁要求用 300 秒，native 档 301 秒自关 PASS。

## 2. K1 复核（一致性测试台复跑，切场景账本）

P0.4 原命令 + `DISPLAY=:3` 复跑，报告与逐场景 frame/run 落盘 `conformance/`；`/resource/` 段逐场景（3 个采样点 = 逐场景账本状态）：

| 场景 | live_count（3 采样） | live_bytes（3 采样） | 每轮增量 | 判读 |
|---|---|---|---|---|
| clip_blend | 9, 9, 9 | 589824 ×3 | **0** | 稳态不增长，K1 口径吻合 |
| solid_gradient | 13 → 15 → 17 | 790528 → 921600 → 1052672 | **+2 条 / +131072B 每轮** | K1 精确复现 |
| text_image | 20 → 22 → 24 | 2232320 → 2363392 → 2494464 | **+2 条 / +131072B 每轮** | K1 精确复现 |

native 与 go 三场景数值**逐位一致**；`gpu_contexts_after` 全 0。K1（每轮 +2 条/+131072B，双后端一致）复核通过；释放缺口本身仍登记 P2.4c。

## 3. 全窗 RSS 关窗回落盘点（附属，P1.4 存量，零 GPU）

`tools/rss_settle.py` 扫 P1.4 全量记录（`rss_settle.tsv` 223 行，含 start+after_close 的记录）：

- **223/223 全部 after_close > start**（中位 +216MB、最大 +381MB）：RSS 含 Go 堆与预热缓存，进程不退不还 OS，**RSS 关窗不回落是运行时常态，不能当泄漏判据**；账本侧的回落证据在 §1 oom（精确 delta 0.00）与 §2 K1。
- 稳态斜率判定（与门禁同口径，只判 `rss_slope_valid=true`）：within-band 8 / over-2k-band 20（2000KB/min 是 soak 治理带，仅观测）/ **LEAK-LINE（>30000KB/min）14**。
- 14 条过泄漏线的记录里，`ui_text_m5_anycase`×2 在 P1.4 已是 FAIL（门禁抓到）；**另有 10 条 PASS 记录过线**——门禁覆盖面问题，逐条登记 P2：

| 窗/后端 | 稳态斜率 KB/min |
|---|---:|
| ui_wr_r17_bg_throttle go / native | 88801 / 62534 |
| ui_text_m4_virtual go | 49624 |
| ui_wr_c5_anim_over_static native / go | 47890 / 38694 |
| ui_wr_r6_layer_anim native / go | 42120 / 41843 |
| ui_wr_r10_async_image native / go | 41906 / 32255 |
| ui_l1_spinner native | 30180 |

## P2 移交清单（P1.6 只登记不修）

1. `ui_base_batch_cap` p95 双后端超 22ms 门禁（P1.4 已登记，本轮复核同因）→ P2 性能线。
2. `ui_base_vram_cap` ladder go 档：GL 4 窗起 `eglMakeCurrent 0x3002`，8 窗阶梯 GL 跑不了 → P2（gles 多窗上下文）。
3. 10 条 PASS 记录稳态斜率过 30000 泄漏线（上表）→ P2 判定：窗门禁集未含 RSS 线还是漏判。
4. K1 释放缺口（solid_gradient/text_image 每轮 +2 条/+131072B）→ P2.4c（P0.4 已登记，本轮复核确认仍存在）。

## 机器校验与复现

```bash
# 采集（runner 在 /tmp/p16/runner.py，串行 + 轮前健康检查 + 标准关闭）
python3 /tmp/p16/runner.py
# K1：P0.4 原命令 + 隔离显示
WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so GPUI_CONFORMANCE_OUT=$PWD/docs/render_gpu_converge/baseline/vram/conformance \
DISPLAY=:3 go test ./render -run '^TestConformanceDualBackend$' -count=1 -v
# 机器校验（非零退出 = 证据缺失/口径破坏）
python3 docs/render_gpu_converge/baseline/vram/tools/verify_vram.py
# RSS 盘点重生成
python3 docs/render_gpu_converge/baseline/vram/tools/rss_settle.py
```

校验 14 断言 2026-10-29 实测 **ALL PASS**（连跑两次幂等）：8 轮记录齐全、oom 两后端注入回落字段、ladder/native 账本三要素、ladder/go 登记理由带 stderr 证据、K1 六份报告 `/resource/`、rss_settle.tsv 224 行。
