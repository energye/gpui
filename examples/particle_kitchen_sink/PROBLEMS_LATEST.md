# Latest problem-suite snapshot (machine-local)

> 生成于本机 suite 跑分，**失败即信号**，不是刷分目标。

## 如何复现

```bash
# 全面挖问题（含 dig 墙）
GPUI_PROBLEM_CAP=critical scripts/run_problem_suite.sh
# 仅 dig
GPUI_PKS_FILTER=dig GPUI_ANIM_SECONDS=5 scripts/run_pks_matrix.sh
```

产物默认：`/tmp/problem_suite/PROBLEMS.md`（本快照对应 `/tmp/problem_suite3/`）。

## 本快照摘要（suite3, critical cap）

- evidence 6/6 PASS（pixel/stage/empty/flicker 装绿墙）
- **total_failures ≈ 19**（core+dig+combo+mem）
- capability critical C01/C07/C11 PASS
- `P_HIGH_N` PASS + 多 combo FAIL → 高级路径瓶颈，非粒子密度

### 新 dig 墙抓到的真问题

| probe | signal |
|-------|--------|
| P_GRAD_RT | `fps_hitch_ratio≈0.32`（渐变 RT export 尖刺） |
| P_FPS_JIT | `fps_jitter_high span≈65`（稳态抖动） |
| P_COMBO_UI | `fps≈25`（clip×grad×filter×blend 合成） |
| P_FILTER_FLICKER | `fps_hitch_ratio≈0.49`（滤镜+交替清屏） |

### 持续暴露的压力问题

| probe | signal |
|-------|--------|
| P_BLEND_GLOW / P_L2 | fps≈51（未在 F1 批次重跑） |
| P_L3 / P_L4 | fps≈25–31（未在 F1 批次重跑） |

### F1 收口（`/tmp/f1_batch5b`, 2026-07-16）

**内容修复（同日续）**：`P_BLEND_LAYER`/`P_BLEND_CPU` 画面无高级混合内容 — `dualTexBlendLayersIntoDest` 未写入 dest。  
已改为 `dualTexAdvancedBlendViewsRegionSized` → out RT → blit；View-nil `FlushGPU` 也可 resolve。  
证据：`TestF1_AdvancedLayerPresentView*`、`TestP03_AdvancedLayerDualTexGPU` PASS；`/tmp/f1_blend_fix`。


Present-deferred parent stash（`prepareTarget` + scissor re-base）落地后：

| probe | status | fps_avg | cpu_avg | gate |
|-------|--------|---------|---------|------|
| P_SOLID | PASS | 57.4 | 21.5 | — |
| P_MULTI_LAYER | PASS | 57.2 | 20.5 | ≥55 fps |
| P_LAYER_BLEND | PASS | 58.1 | 31.2 | ≥55 fps |
| P_BLEND_LAYER | PASS | 58.1 | 31.5 | ≥55 fps |
| P_L1 | PASS | 58.1 | 30.1 | ≥55 fps |
| **P_BLEND_CPU** | **PASS** | **58.1** | **31.8** | **cpu &lt; 80** |

此前：`P_MULTI_LAYER`/`P_LAYER_BLEND` ≈30–38fps；`P_BLEND_CPU` cpu≈102–110。  
改动要点：`render/internal/gpu/gpu_render_context.go` — 全部 Queue\* 走 `prepareTarget`；View-nil parent→layer 时 stash 而非 mid-frame full-scene Flush；Present 路径 unstash；stash merge 重基 scissor 计数。

### Bisect 示例

- `P_ALPHA_MESH` FAIL → `ENABLE_BLEND=0` 或 `ENABLE_MESH=0` 可恢复 → 组合路径问题

详见 `COVERAGE.md` 失败模式表。


### F1 HUD text (2026-07-16 续)

**问题**：`ClipRect` + `PushLayer(advanced)` 后 HUD/FPS `DrawString` 不可见。  
**根因**：`setGPUClipRect` 在 `Queue*→prepareTarget` 之前记录 `SetClip`；首笔 layer 绘制会把**未配对的 SetClip** stash 进 parent present-stash，present 时 HUD 文本落入 stage scissor。  
**修复**：`PrepareTarget` 先于 scissor 记录；base 编码后 drop pending 再 resolve；blit-only `LoadOpLoad` 保留 scratch。  
**证据**：`TestF1_AdvancedLayerPresentView_HUDText` PASS；`P_BLEND_LAYER`/`P_BLEND_CPU` PASS。


### F1 closeout (`/tmp/f1_closeout`, 2026-07-16 夜)

**F1 核心（层批 / 高级混合 present / HUD）已收口** — 证据目录 `/tmp/f1_closeout` + `TestF1_AdvancedLayerPresentView*`。

| probe | status | fps_ema | fps_avg | cpu | note |
|-------|--------|---------|---------|-----|------|
| P_SOLID | PASS | 59.5 | 57.2 | 21 | 基线 |
| P_MULTI_LAYER | PASS | 59.2 | 57.4 | 23 | present-stash + opacity-group |
| P_LAYER_BLEND | PASS | 58.8 | 57.5 | 33 | ≥55 |
| P_BLEND_LAYER | PASS | 59.6 | 57.6 | 32 | 高级混合内容+HUD |
| P_BLEND_CPU | PASS | 58.7 | 57.6 | 32 | cpu&lt;80 |
| P_L1 | PASS | 57.5 | 57.3 | 36 | gate |
| P_BLEND_GLOW | FAIL | 70.2 | 57.2 | 49 | hitch_ratio≈0.32（glow RT ApplyBlur+export） |
| P_L2 | FAIL | 72.6 | 57.2 | 51 | 同上 |
| P_L3 | FAIL | 76.2 | 56.1 | 61 | glow+mesh+text 组合 hitch |
| P_L4 | FAIL | 59.0 | 50.5 | 95 | 高密度+trails hitch/cpu |

**F1 交付项**
1. present-stash：View-nil parent 跨 layer RT 不 mid-frame full Flush  
2. advanced resolve：`viewsRegionSized → out → blit`（非 dead `dualTexBlendLayersIntoDest`）  
3. HUD/FPS 文本：`PrepareTarget` 先于 scissor；scratch blit `LoadOpLoad`  
4. 像素门：`TestF1_AdvancedLayerPresentView_HUDText`  

**非 F1 阻塞（下一挖）**：glow 滤镜 export 尖刺（`ApplyBlur`+`ExportImageBuf` 隔帧）— 修 GPU 常驻 glow RT / 减 readback，**不得**靠降内容刷 PASS。
