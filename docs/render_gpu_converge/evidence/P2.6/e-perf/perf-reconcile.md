# P2.6-e 性能对账（2026-11-01）

> 方法：c 段重放的 360 份 record.json 转换为 P1.4 形态（`emit` 步骤见 `windows/`），用 P1.7 同款 extractor 副本（`extract_perf_p26.py`，WIN/OUT 指向本目录、windows.md/thresholds.md 仍读基线真源）重出 `perf_p26.tsv`，与 `baseline/perf/perf_baseline.tsv` 逐轮对照。

## 门禁机检（A 族硬门禁，读自 thresholds.md）

| 项 | P1.7 基线 | P2.6 重放 | 判定 |
|---|---|---|---|
| p95_fail（≤22ms） | 4（c3_list_scroll/r7_virtlist/r7b 等超限） | **0** | ✅ 全绿（原超限窗转绿，见 c 段对账 20 条之一） |
| fps_fail（≥55fps） | 8 处集中 4 轮（A 族） | **0** | ✅ |
| first_frame_fail | 0 | **0** | ✅ |
| gate_mismatch | 0 | **0** | ✅ |

## 逐轮 p95 对照（296 个有数值对）

- 恶化 >10% 且 >1ms：**0 轮**。
- 改善 >10% 且 >1ms：10 轮——backdrop msaa1/msaa4 双后端 137→108/118→88ms（P1.4 的 x11grab 黑图拉高因素消除+滤镜链优化）、clipop 22.6→17.4、maskops 23.9→17.5 等。
- 结论：**性能不回退，且 P1.7 的 A 族超限点全部消除**。

## 别名退役注记（P2.5 登记的换代点）

新代证据（本轮 360 record）全部原生发 `interval_*`+fps_wall+target_hz，别名映射在重放数据上是死代码；**正本 `extract_perf.py` 的双代读取保留**——它服务的冻结 P1.4/P1.7 证据仍是旧代标签，`verify_perf.py` 对冻结证据的机检（alias 行=12、vr7 例外）保持成立。别名彻底退役挂「基线证据被新基线替换」时点（P2.6 之后下一轮基线冻结），本轮不推翻冻结证据。
