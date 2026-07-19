# S5.1 — Present-only 基线

> 版本：1.1 | 日期：2026-07-15（文档归档 2026-07-19）  
> 状态：**S5.1 关闭**；**harness 已移除**（并入 S6）  
> 依赖：S5.0 `docs/S5_SKIA_UI_GAP.md`  
> 架构：`render.PresentFrame*` → `FlushGPUWithView*` → webgpu → rwgpu → libwgpu_native

---

## 1. 目标

建立 **无 CPU 读回** 的 present 路径计时基线，与 S4 读回基线分列。

- 计量：`draw_ms + present_ms`（`PresentFrame` / `PresentFrameDamageRects`）  
- **不含** `ReadPixels` / `FlushGPU` 读回  
- 路径标记：`present-only-offscreen`（离屏 texture view；窗口 X11 为可选加深）

---

## 2. 测试（已归档）

原 `render/s5_present_baseline_test.go` 中的 `TestS5_*` / `TestS52_*` / `TestS53_*` / `TestS54_*` 已删除。  
共享测量 helpers（`s5Scenes` / `s5MeasurePresent` / `s5Percentile` 等）保留在 `render/s5_present_helpers_test.go`，供 S6 使用。

| 替代 | 作用 |
|------|------|
| `TestS6_PresentBaseline_Scenes` | present-only 场景测量 + JSON |
| `TestS6_RegressionLock_Contract` | 主路径回归锁 |
| `TestS61_*` | 帧模型 / damage / idle present |

```bash
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH
S6_PERF_WARMUP=3 S6_PERF_ITERS=10 go test -count=1 ./render -run 'TestS6_PresentBaseline_Scenes|TestS6_RegressionLock' -timeout 300s
```

---

## 3. 场景表

| ID | 意图 |
|----|------|
| P01 | 地板 solid present |
| U01 | 静态壳 bootstrap + 状态 chip 脏区 |
| U02 | 列表滚动 3 行脏带 |
| U03 | 表单字段脏区 |
| U04 | 模态静止全量 redraw（轻） |
| U05 | kitchen-sink 应力（不设 60fps 硬门） |
| B15like | 双面板 multi-damage |

---

## 4. 基线数字（本机；p50 为门禁指标）

Note: [Errno 2] No such file or directory: 'tmp/s5_present_baseline.json'

| 场景 | 尺寸 | p50_ms | avg_ms | fps_p50 | gpu | cpu_fb | retained | damage |
|------|------|--------|--------|---------|-----|--------|----------|--------|
| B15like_MultiDamage | 320×200 | 2.30 | 2.41 | 434.0 | 4 | 0 | True | True |
| P01_SolidPresent | 640×400 | 2.58 | 2.69 | 388.2 | 1 | 0 | False | False |
| U01_StaticShell | 800×480 | 6.47 | 6.79 | 154.4 | 3 | 0 | True | True |
| U02_ListScrollMorph | 400×560 | 4.71 | 4.79 | 212.3 | 9 | 0 | True | True |
| U03_FormFieldDamage | 400×300 | 3.99 | 4.01 | 250.5 | 3 | 0 | True | True |
| U04_ModalStatic | 480×320 | 6.62 | 7.20 | 151.2 | 6 | 0 | True | False |
| U05_KitchenSinkStress | 480×320 | 170.08 | 170.90 | 5.9 | 18 | 0 | True | False |

全部 **`cpu_fallback_ops=0`**，**`gpu_ops>0`**。

---

## 5. 观察

1. **脏区稳态**（U01/U02/U03/B15like）易进 60fps 预算。  
2. **全量 redraw 重场景**（U05 layers）仍慢 — 不设硬门，作回归观察。  
3. 离屏 present ≠ 窗口 vsync present；X11 真窗口为可选 e2e（`gpui_x11_present`）。  

---

## 6. 退出条件

| 条件 | 状态 |
|------|------|
| present-only harness | ✅ |
| JSON 产物 | ✅ `tmp/s5_present_baseline.json` |
| GPUOps>0 / 无 silent CPU | ✅ |
| 与 S4 读回基线分列 | ✅ |

**S5.1 关闭。** 下一：**S5.2 帧模型**。
