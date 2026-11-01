# P2.6-f 负向对照重放（2026-11-01，P1.8 口径）

> 门禁车辆：P0.4 一致性台（`go test ./render -run ^TestConformanceDualBackend$`）。绿基线 rc=0。

| §8.1 破坏 | 注入方式 | 结果 | 证据 |
|---|---|---|---|
| 1 画面偏移 1px | P1.8 补丁（`docs/render_gpu_converge/baseline/negative/patch.diff`，可干净应用）+ `GPUI_NEG_SHIFT_1PX=1` | **红**：coverage left_red_fill 0.9212 / blend_band 0.8713 ≠ 1.00（与 P1.8 同签名）；env 关=绿 | `shift1px-red.txt` / `shift1px-green.txt` |
| 2 跳过一次绘制 | 同补丁 + `GPUI_NEG_SKIP_DRAW=1` | **红**：被跳填充整块消失（solid_center got [0 0 0 255] want [255 0 0 255] 等），比 P1.8 的 gpu_ops 下限问题更直接的像素/区域证据 | `skipdraw-red.txt` |
| 3 少释放一次 | **改用 K1 原洞注入**：临时注释 `TexturedQuadPipeline.destroyPipeline` 的 bicubic 两管线销毁块（P1.8 的「上界收紧」旋钮在 P2.4c 修复后已是 0/0 无意义） | **红**：`steady count delta=2 (max 0) bytes delta=131072 (max 0)`——与 P2.4c 定位的 K1 泄漏签名逐字一致；git 还原后 conformance 复绿 | `skipforget-red.txt` / `skipforget-green.txt` |

§8.2 阈值变异对（extractor 读 thresholds.md，`docs/render_gpu_converge/baseline/perf/tools/extract_perf.py`）：

| 态 | p95 门禁 | p95_fail |
|---|---|---|
| 变异（22→30） | 30ms | **0（漏报得证：4 条超限被吞）** |
| 还原（22） | 22ms | **4（正确报警，与 P1.7 基线一致）** |

补丁/数据还原均零残留（`git status` 0 处）；thresholds.md 还原零残留。

## 事故如实记录（已恢复）

破坏 3 第一次还原时用 `grep -rln | head -1` 选还原目标，拷错文件（`glyph_mask_pipeline.go` 被 `image_pipeline.go` 原文覆盖），当场发现（还原 rc=1 + git diff 919/757）→ `git checkout -- render/internal/gpu/` 全量恢复（该目录工作区在 f 前是干净的 e67e4be7）→ build OK + conformance 复绿验证。教训：**还原动作必须用显式文件路径，禁止 grep 猜目标**。涉事文件最终态与 HEAD 逐字节一致（git status 0）。
