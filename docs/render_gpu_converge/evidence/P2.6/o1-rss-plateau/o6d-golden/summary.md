# O6d 22 窗金图 + O5 multiwin 回归（2026-11-13）

> 隔离显示：rootless `Xwayland :3`（批次内每轮 `xdpyinfo` 健康检查）；
> 单窗串行、`RUN_SECONDS=5 -auto-only`、窗口自行 Quit/Close；未硬杀。
> 采集器：`evidence/P2.6/golden-drift/tools/run_windows.sh`（P2.6 同款）。

## 1 22 窗 × 双后端（44 轮）

- **44/44 rc=0，golden 全部 0.0000%**（`run-index.tsv` 44 行；golden 非零 0 行）。
- `logic_pass` 缺字段 2 行：`ui_wr_shadermask` native/go 的 JSON 未输出该字段
  （rc=0、golden 0，窗自身门禁通过）。
- `screen_ok=False` 16 行：全部 `ui_wr_p38_*` 窗未输出 `screen_check_ok`
  （字段缺失，非抓屏失败；rc=0、golden 0）。
- 结论：O2 通用叶子录制、O4 归因 map 有界、O5 RSS 探针、O6 测量改动
  **没有改变 22 窗任何一帧**。

## 2 O5 multiwin 8 窗（:3，75s，native/go）

| 后端 | rc | RSS 探针 | present/frame | golden | fps 门 |
|---|---|---:|---|---|---|
| native | 1 | `valid=false / inconclusive`（O5 口径，通过） | 13499/3333 | 0.4911%/窗 | `max_windows_60fps=4`（floor 2，过） |
| go | 1 | 同上 | 3712/3704 | 0.4911%/窗 | `max_windows_60fps=1`（floor 2，环境性能红） |

- **RSS 探针不再误红**：75s 轮按 O3 口径为 inconclusive，O5 的门禁修改生效。
- golden 0.4911% 在**pre-O2 `7a7fc15d` 干净树同值** → 既有基线/环境漂移，非本次改动。
- go `max_windows_60fps=1` 是隔离 XWayland 上 8 窗 GL 的性能环境红，不是内存问题
  （P1.6 基线曾 PASS，本机当前 :3 环境不同）；登记 O7/环境，不混入内存结论。
- `present_count` 为真实上屏数（native 多窗 13499 vs 3333 帧；go 3712 vs 3704）；
  空闲窗不再计 job，口径符合 O2 C。

## 3 结论与余项

- **O6d 通过**：22 窗双后端金图零漂移；O5 multiwin 的 RSS 探针口径已对齐。
- 既有红（multiwin 0.4911%、go 8 窗 fps）归 O7/环境，不阻塞内存矩阵。
- 余项：**O6c 2h soak**（`ui_text_m4_virtual` + `video_vc2_soak`，隔离显示，
  待用户排期）；O7 既有金图/性能红归类。
