# P2.4d conformance 双后端复核（「是否真跑」硬证据）

> 2026-10-30。复核对象：`rerun-after-pool-convergence.txt` 里 `--- PASS: TestConformanceDualBackend (0.76s)`。
> 疑点：P2.4c 同测试 16.64s，本步只有 0.76s，需排除「两端都 unavailable → `t.Skipf` 被读成 PASS」与「空通过」。

## 1 结论：真跑，6 份报告全 ok，零 SKIP

| 场景 | native | go | gpu_ops(native/go) | cpu_fallback | steady_count/bytes |
|---|---|---|---|---|---|
| solid_gradient | ok | ok | 3 / 3 | 0 / 0 | 0 / 0 |
| clip_blend | ok | ok | 3 / 3 | 0 / 0 | 0 / 0 |
| text_image | ok | ok | 2 / 2 | 0 / 0 | 0 / 0 |

- 6 份 `report.json` 的 `status` 全为 `ok`（不是 `unavailable`），`samples`/`regions` 带真实采样与期望值；`resource.max_steady_count_delta/max_steady_bytes_delta` 全 0（P2.4c 收紧后的 0/0 上界）。
- 两个后端子进程各打印三场景 `ok=true` 并 `--- PASS: TestConformanceInner`（`reports/*/run.txt`）。
- 编排测试走的是 `failed` 为空 → 不 `t.Fatalf`；`unavailable` 为空 → 不 `t.Skipf`。没有 SKIP 行，也没有空报告。

## 2 为什么 0.76s（对 16.64s 的解释）

三场景画布都是 128×96；本次两端各自 0.33s / 0.39s（`run.txt`）。P2.4c 的 16.64s 包含冷启动 shader 编译与设备初始化，本次同机热缓存下缩短，属耗时波动，不是「没跑」——报告与帧文件在 17:19:39.688 / 17:19:40.095 两组真实落盘时间，且内容逐项可核。

## 3 像素逐字节对账（对 P0.4 基线）

| 场景/后端 | 本轮 sha256（前 12） | P0.4 基线 sha256（前 12） | 结果 |
|---|---|---|---|
| solid_gradient/native | 317c13fc833f | 317c13fc833f | 一致 |
| solid_gradient/go | cb67dd9e38f2 | cb67dd9e38f2 | 一致 |
| clip_blend/native | a7fb348e8396 | a7fb348e8396 | 一致 |
| clip_blend/go | 7fd12d76c599 | 7fd12d76c599 | 一致 |
| text_image/native | 618a13ac5dac | 618a13ac5dac | 一致 |
| text_image/go | 618a13ac5dac | 618a13ac5dac | 一致 |

逐位一致 → 池收敛没有改变任何像素；三场景双后端全 PASS，`unavailable` 计数 0。

## 4 证据文件

- `rerun-after-pool-convergence.txt`：原始测试输出 + 工作区 testdata 变更检查。
- `reports/<scene>/<backend>/{report.json,run.txt,frame.png}`：本轮 6 份原始产物复制归档（与 `render/tmp/conformance/` 同内容）。
- 基线帧：`docs/render_gpu_converge/baseline/conformance/<scene>/<backend>/frame.png`。
