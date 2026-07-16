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
| P_MULTI_LAYER / P_LAYER_BLEND | fps≈30–38 |
| P_BLEND_GLOW / P_L2 | fps≈51 |
| P_L3 / P_L4 | fps≈25–31 |
| P_BLEND_CPU | cpu≈110 > 80 |

### Bisect 示例

- `P_ALPHA_MESH` FAIL → `ENABLE_BLEND=0` 或 `ENABLE_MESH=0` 可恢复 → 组合路径问题

详见 `COVERAGE.md` 失败模式表。
