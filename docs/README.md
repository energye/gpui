# docs/ — 现网文档索引

> 日期：2026-07-21  
> **只保留最新活文档**；与代码冲突时以代码为准。  
> 调用链：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 范围：渲染引擎（不含控件层实现）

---

## 入口（按用途）

| 用途 | 文档 |
|------|------|
| **引擎必有缺口** | [`ENGINE_GAPS.md`](./ENGINE_GAPS.md) |
| **Skia 2D 能力表** | [`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md) |
| **Surface 生命周期** | [`SURFACE_LIFECYCLE_SKIA_FLUTTER.md`](./SURFACE_LIFECYCLE_SKIA_FLUTTER.md) |
| **Device lost / recover** | [`GPU_修复_device_lost.md`](./GPU_修复_device_lost.md) |
| **显存 / Adapter 策略** | [`VRAM_BUDGET.md`](./VRAM_BUDGET.md) |
| **GPU 优先原则** | [`GPU_FIRST_ROUTING.md`](./GPU_FIRST_ROUTING.md) |
| **内存护栏（日常）** | [`MEM_LEAK_PERF_GUARD_PLAN.md`](./MEM_LEAK_PERF_GUARD_PLAN.md) |
| **内存测试细节** | [`MEM_LEAK_TEST_PLAN.md`](./MEM_LEAK_TEST_PLAN.md) |
| **正向优化任务卡** | [`PERF_ENGINE_FORWARD.md`](./PERF_ENGINE_FORWARD.md) |
| **代码收敛卡** | [`CODE_CONVERGENCE.md`](./CODE_CONVERGENCE.md) |
| **主线阶段状态（精简）** | [`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) |
| **窗口能力验收** | [`CAPABILITY_MATRIX_WINDOW.md`](./CAPABILITY_MATRIX_WINDOW.md) |
| **组合 / 复杂 UI 门禁** | [`P1_COMPOSITION_MATRIX.md`](./P1_COMPOSITION_MATRIX.md) · [`P1_COMPLEX_UI_MATRIX.md`](./P1_COMPLEX_UI_MATRIX.md) |
| **控件入口条件** | [`S5_WIDGET_ENTRY.md`](./S5_WIDGET_ENTRY.md) · [`S5_SKIA_UI_GAP.md`](./S5_SKIA_UI_GAP.md) |
| **UI 框架总图与规划（P2）** | [`UI_FRAMEWORK_MAP.md`](./UI_FRAMEWORK_MAP.md)（v4：primitive 组合底座） |

---

## 示例入口（文档外）

| 示例 / 脚本 | 用途 |
|-------------|------|
| `examples/api_coverage_app` | 公共 Context API 全覆盖（180）+ lifecycle selftest |
| `examples/antd_desktop_app` · `flutter_app_shell` · `app_lifecycle_shell` | 产品形窗口 + SurfaceHost / 生命周期 |
| `examples/mem_anim_window` · `particle_kitchen_sink` | 压测 / 泄漏 |
| `examples/run_window_lifecycle_matrix.sh` | 多示例 lifecycle 矩阵 |
| `scripts/run_mem_guard.sh` | 内存护栏一键 |

---

## 维护规则

1. 新缺口 / 新边界 → 先改 **`ENGINE_GAPS.md`**，再改能力矩阵相关行。  
2. 生命周期 / lost / VRAM → 只改 SURFACE · device_lost · VRAM 三份。  
3. **禁止**再堆历史里程碑卡；完成的任务写进现有活文档或删掉，不另开档案目录。  
4. 改文档后至少做一轮：**文中符号/路径/env/示例名** 与仓库 `rg` / `ls` 对照。
