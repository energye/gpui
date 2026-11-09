# P3.9b 新发现问题落位：go/GL + 4x MSAA 快照整窗全黑（2026-11-09）

> 性质：**开工测量时抓到的真实引擎洞，稳定复现 2 次**。本文件只做只读诊断与归位，不改代码、不标绿。
> 归属：GLES 后端 4x MSAA 呈现/readback 路径，**不在 P3.9b 契约内**（P3.9b = 测量补充，用户明确「零引擎改动、零新增公开 API」）。

## 0 直话

- 满屏 1080p 混合动画场景，1x/4x × native/go 四组合门禁跑到第 7 轮时，**go/GL + 4x 的快照整张全黑**（0% 非黑像素），稳定复现 2 次。
- **native 1x / native 4x / go 1x 三组合全绿**（金图 seed+verify 逐位 0.0000%、锚点探针 ok）；只有 `GPUI_BACKEND=go` + `MSAA=4` 这一条坏。
- 不是「4x 撑不住显存」：native 4x 与 go 4x 的显存压力**完全相同**（surface=76,289,120B、peak=147,766,752B > budget=134,217,728、evict=5），native 画面正常。
- 这是本库**第一次认真在 GLES 后端测 4x**（此前 P3.9/P3.9a 双后端金图都只跑 1x；P2.6 e-perf 的 msaa4 记录全是 backdrop 重负载 FAIL、显存两端相同，不能作画面证据）。所以这是**既有存量洞**，不是 P3.9b 引入。

## 1 完整问题项（按 AGENTS「新问题归位」字段）

| 字段 | 内容 |
|---|---|
| 现象 | `GPUI_BACKEND=go GPUI_P39B_MSAA=4 RUN_SECONDS=6 /tmp/p39b_bin -auto-only`：快照 PNG 1868×1021 **全部像素 (0,0,0,0)**，nonblack=0/38982 (0.0%)；锚点探针 `got=(0,0,0)` FAIL、exit 1 |
| 复现步骤 | 见 `gate/go_msaa4_seed.txt` 与 `/tmp/go4x_repro.txt`；两次独立运行均全黑 |
| 实测数据 | gpu_ops=16333~16475（>0，命令在提交）、present_count=230、cpu_fallback_ops=0、gpu_fallbacks=0、无 `MSAA downgraded`/OOM/GL error 日志；pool `device-surface` bytes=76,289,120 / peak=147,766,752 / evict=5 |
| 与真值差异 | 应与 native 4x 同场景画面一致（彩色 mesh 渐变 + 曲线 + 卡片 + 图片 + 粒子 + 锚点纯色卡）；实得整窗透明黑 |
| 定位线索（未定论） | 快照走 `ui/embedder/pipeline_app.go:1854` `dc.SavePNG`（GPU readback）；GLES MSAA resolve 在 `gpu/gwgpu/gles/command_encoder.go:524-650`（`setupColorAttachment` 记录 `msaaTexture`/`ResolveTarget`）、`glBlitFramebuffer` 到 FBO 0 在 `:557`。疑点集中在「4x 多采样缓冲 → resolve/readback」这段；**尚未定位到确切行、未对齐标准方案，故不动手** |
| 归属阶段/能力 | GLES 后端 4x MSAA 呈现与读回；**非 P3.9b**（测量补充线） |
| 状态 | ⬜ 待办：需切到对应线，补齐「定位到文件行号 + 对齐标准方案」两样后按 skill 流程修；修前停在「停下报告」等用户确认方向 |
| 待确认 | 窗口肉眼是否也黑、还是仅 SnapshotPath readback 黑（本轮未开 go4x 常驻人工窗）；两者都是洞，定位不同 |

## 2 已做的防假绿处置

- 删除被污染的 `goldens/p39b_anim_go_msaa4.png`（9670 字节黑图——gate 模式 pixel 探针失败未拦住 `GoldenGateW` 的 seed，把黑图写成了基线）。**该组合当前无金图、不标绿**。
- 门禁批次在第 7 轮 `STOP-BATCH`（脚本 `run_measure.sh` 每轮前健康检查 + 失败即停），negative / measure 阶段**未继续**，避免把坏数据往外倒。
- 三组合已绿证据保留：`gate/native_msaa{1,4}_{seed,verify}`、`gate/go_msaa1_{seed,verify}`（金图逐位 0.0000%）。

## 3 顺带拿到的有效结论（4x 占内存，native 口径）

| 组合 | device-surface bytes | peak | evict | 画面 |
|---|---|---|---|---|
| native 1x | 62,840,480 (59.9 MiB) | 同 | 0 | ✅ |
| native 4x | 76,289,120 (72.7 MiB) | 147,766,752 (140.9 MiB) > 128 MiB budget | 5 | ✅ |
| go 1x | 62,840,480 | 同 | 0 | ✅ |
| go 4x | 76,289,120 | 147,766,752 | 5 | ❌ 全黑 |

- 4x 让 `peak` 从 ~60 MiB 抬到 ~141 MiB（**2.35×** 峰值、超出 128 MiB 池预算触发淘汰），这是「4x 很占显存」在满屏 1080p 复杂场景下的真实数字；比上一轮静态 1200×800 小窗的 5.67× 更能代表压力场景（池预算淘汰削平了稳态值）。
- 帧时 / 首帧 / CPU 分解的 1x↔4x 对照，待 measure 阶段四轮跑完才出；go4x 那轮即使跑也是黑屏路径数据，**不能当有效性能结论**。

## 4 为什么停在这里（不自决）

- AGENTS「技能执行规则 / 修复流程纪律」：影响面评估 + 标准方案对齐后必须停在「停下报告」用 question 工具确认方向；「动手修门槛」要求先同时拿出「复现+实测+定位到行号」与「对齐标准方案」，目前**行号与标准方案未齐**。
- AGENTS「禁止在示例层绕引擎洞」「门禁绿 ≠ 底层洞已修」「下层不满足交下层 skill」：不能用改示例/跳过该组合/放宽断言来假绿。
- P3.9b 契约是「零引擎改动」；修 GLES 属于扩范围，需用户点头。
