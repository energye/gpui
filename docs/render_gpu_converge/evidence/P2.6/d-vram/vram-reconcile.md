# P2.6-d 显存账本重放对账（2026-11-01）

> 数据源：P2.6-c 段重放的同协议轮次（ladder/oom/multiwin 已按 P1.4/P1.6 基线证据修正编排环境后重跑：RUN_SECONDS=300 / GPUI_FAULT_OOM=180:1+60s / 75s）；基线 = `baseline/vram/summary.md`（P1.6，:3 隔离显示，串行标准关闭）。

| 窗 | 基线（P1.6） | 重放（本轮） | 判定 |
|---|---|---|---|
| vram ladder native（300s） | 账本峰值 **372.0MB**，水线 0.9×768=691.2 未破；拒增曲线 [2,0]→[4,550]→[8,747]；hold 斜率 1533KB/min | 峰值 **373.8MB**（+0.5%，落在 P1.6 资源矩阵带 362.9–381.4 内）；金图 16/16 **diff=0.0000%**；rc=0 | ✅ 不回退 |
| vram oom native（GPUI_FAULT_OOM=180:1，60s） | 注入峰值 12.06MB → 恢复 **8.5459MB（delta 0.00）**，`ledger_recovered=true` | 峰值 **14.77MB** → recovery live 11.26MB，**`ledger_recovered=true`** | ✅ 资源谁建谁放成立（recovered 语义位绿；绝对值随注入相位浮动） |
| multiwin_cap native（8 窗，75s） | crash_free、8 窗未触顶；RSS 关窗后 +206.4MB（Go 堆常态） | **crash_free**、rss_peak 265MB 同量级（Go 堆常态口径） | ✅ |
| batch_cap（合批） | FAIL（p95 55.7ms > 22，性能线） | **PASS**（P2 优化见效，c 段对账转绿 20 条之一） | ✅ 转绿登记 |
| conformance K1（solid_gradient/text_image 每轮 +2 条/+131072B） | P2.4c 已修，上界收紧 **0/0** | P2.5 收口与 P2.6-f 复跑均 0/0 不回退 | ✅ |

结论：显存账本四窗全部不回退（ladder 峰值 +0.5% 在带内、oom recovered=true、multiwin crash_free、batch 转绿），无一处触碰 0.9 水线。差异（ladder +1.8MB）为重放环境同带内浮动，非泄漏（hold 段与拒增曲线由窗内门禁自证，rc=0）。

证据：`../c-windows/ui_base_vram_cap/{ladder,oom}/native|go/{record.json,stdout.txt,stderr.log}`、`../c-windows/ui_base_multiwin_cap/default/native/*`（本轮重放，环境修正见 `../c-windows/summary-clean.tsv` 的 retry/override 注记与台账 §7）。
