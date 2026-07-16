# Problem-finding coverage matrix

目标：用真实 present 路径 **挖问题**，不是刷分。失败模式 → 探针/套件对齐。

## 如何跑

```bash
# 全面挖问题（evidence + core + combo + mem + capability C01–C20）
scripts/run_problem_suite.sh

# 更快：关键 capability
GPUI_PROBLEM_CAP=critical scripts/run_problem_suite.sh

# 更全：capability C01–C32（若示例支持）
GPUI_PROBLEM_CAP=full scripts/run_problem_suite.sh

# 分层
GPUI_PKS_FILTER=evidence scripts/run_pks_matrix.sh
GPUI_PKS_FILTER=mem scripts/run_pks_matrix.sh
GPUI_PROBE=P_FLICKER GPUI_ANIM_SECONDS=8 /tmp/pks_bin
```

产物：`/tmp/problem_suite/PROBLEMS.md`、`problems.json`。

## 失败模式 → 探针

| 失败模式 | 含义 | 主探针 / 套件 | FAIL 前缀 |
|----------|------|---------------|-----------|
| **perf_fps** | present 掉帧 | P_BLEND_GLOW, P_MULTI_LAYER, P_L2–L4, combo | `fps_low_*`, `fps_hitch_*` |
| **perf_cpu** | 主线程过重 | P_BLEND_CPU | `cpu_over_budget` |
| **gpu_fallback** | CPU 回退 | 全部 gate；capability | `cpu_fallback_ops` |
| **gpu_dead** | 未上 GPU | 全部 | `gpu_ops=0` |
| **pixel_raster** | 栅格/Export 错误 | P_PIXEL | `pixel_fail` |
| **content_dropout** | 间歇丢内容/闪 | P_FLICKER, 间歇 stage 采样 | `intermittent_content`, `stage_sig_fail` |
| **content_empty** | 空绿 / 砍密度 | P_EMPTY_TRAP, MinN | `content_fail`, `content_gutted` |
| **present_resize** | swapchain/resize | P_RESIZE | `present_errors_*`, `resize_recover_*` |
| **memory** | RSS 爬升 | P_MEM_SOAK, P_MEM_LONG, P_GROW_N | `rss_*`, `mem_*` |
| **blend_regression** | 逐圆混合 ~1fps | P_BLEND_PER_CIRCLE trap | `trap_hot_path_still_slow` |
| **hang_or_crash** | 帧过少 | 全部 | `too_few_frames` |
| **fps_jitter** | 稳态抖动过大 | P_FPS_JIT | `fps_jitter_high` |
| **filter_hitch** | 滤镜/渐变 export 尖刺 | P_GRAD_RT, P_FILTER_FLICKER | `fps_hitch_ratio` |
| **ui_combo** | 多能力合成掉帧 | P_COMBO_UI | `fps_low_*` |
| **text_glyph** | 中/英字形缺失 | P_TEXT_BI | `pixel_fail:text_bi` |

## 套件分层

| 层 | filter / 范围 | 必过？ |
|----|---------------|--------|
| **evidence** | PIXEL/STAGE_SIG/EMPTY/FLICKER/DARK/CLEAR_ALT | 是（装绿墙） |
| **core** | 全部 gate+trap + 关键 stress | gate/trap 必须绿 |
| **combo** | 交互组合 | stress FAIL = 信号 |
| **mem** | soak/grow/resize/multi-layer | 泄漏硬 FAIL |
| **capability** | C01–C20（wide） | 应绿 |

## 纪律

1. 禁止减粒子 / 关 AA / silent CPU 装绿。  
2. `P_HIGH_N` PASS 而 combo FAIL → 修高级路径。  
3. 一次只改一个 `GPUI_ENABLE_*`。  
4. CPU/RSS 仅本进程。  
5. 能 GPU 必须 GPU。

## PKS vs capability_matrix

- **PKS**：动态密度 + 隔离轴 + 内存/闪烁/CPU 预算。  
- **capability_matrix**：Cxx 能力正确性。  
- **run_problem_suite.sh** 合成 `PROBLEMS.md`。


## Dig 墙（Skia-facing）

`GPUI_PKS_FILTER=dig` — 专挖粒子轴漏掉的正确性/稳定/UI 合成问题。

| 失败模式 | 探针 | 抓什么 |
|----------|------|--------|
| clip_wrong | P_CLIP_NEST | 嵌套裁剪错位/空窗 |
| grad_empty/cpu | P_GRAD_RT | 渐变空/ColorAt 过重 |
| filter_flicker | P_FILTER_TILE, P_FILTER_FLICKER | 模糊读回闪/掉帧 |
| blend_sep | P_BLEND_SEP | 可分离混合闪/慢 |
| path_xform | P_PATH_XFORM | path submit × 变换卡顿 |
| fill_rule | P_EVENODD | EvenOdd vs NonZero |
| dash | P_DASH | 虚线描边 |
| mesh_wave | P_MESH_WAVE | 波浪网格锯齿/掉帧 |
| text_bi | P_TEXT_BI | 中英字形缺失 |
| image_px | P_IMAGE_PX | 写像素贴图 |
| fps_jitter | P_FPS_JIT | 稳态抖动 span |
| combo_ui | P_COMBO_UI | UI 多能力合成墙 |
| cpu_mesh | P_CPU_MESH | 网格主线程 CPU |
| xform_stack | P_XFORM_STACK | 矩阵栈风暴 |

跑法：

```bash
GPUI_PKS_FILTER=dig scripts/run_pks_matrix.sh
# 全面
GPUI_PROBLEM_CAP=wide scripts/run_problem_suite.sh
```
