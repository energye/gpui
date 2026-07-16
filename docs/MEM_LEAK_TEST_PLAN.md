# 内存 / VRAM 泄漏测试方案

> 版本：1.0 | 日期：2026-07-15  
> 状态：**执行中**  
> 范围：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`（经 render 真链路）  
> 非目标：游戏引擎、控件层、完整 ASAN/Valgrind 替代

---

## 1. 目标

1. **防死机**：硬门禁捕获 `Not enough memory` / CreateTexture 失败，避免无界吃光显存/系统内存  
2. **分场景观测**：简单→复杂组合，固定迭代与时间窗，看 **稳态斜率** 与 **Teardown 后能否再分配**  
3. **释放链回归**：`Context.Close` / offscreen `release` / `session.Destroy(WaitIdle)` / `ResetAccelerator`  
4. **自动反复验证**：`go test -count=N` 与脚本一键跑；可选独立窗口压测程序  

---

## 2. 双轨指标

| 轨 | 指标 | 硬门禁 | 软门禁 |
|----|------|--------|--------|
| **功能/GPU** | present 成功、`GPUOps>0`、`cpu_fallback_ops=0`、无 native abort | ✅ Fail | — |
| **进程 RSS** | `/proc/self/status` VmRSS（Linux） | 可选超大顶 | 稳态增量上限 |
| **生命周期** | Close/Reset 后仍能 Present 大尺寸 | ✅ Fail | — |

说明：RSS **不能**精确等于 VRAM，但与 OOM 硬失败组合足够做 CI 护栏。

---

## 3. 分档场景（简单 → 复杂）

| Tier | 名称 | 内容 | 默认迭代 | 主要覆盖 |
|------|------|------|----------|----------|
| **T0** | CreateClose | 每帧 NewContext+Present+Close，多尺寸 | 40 | session 纹理释放 |
| **T1** | RetainedMultiSize | 长寿命 Context + Resize + 新 offscreen | 30 | 尺寸切换释放 |
| **T2** | ResetAccelerator | 压力后全局 Reset 再 Present | 1 轮 | GPUShared 回收 |
| **T3** | ComplexOffscreen | path/text/image/layer/clip/blend/dash + 随机背景 + 变尺寸 | 36 | 主路径资源全集 |
| **T4** | ComplexWindow | X11 窗口 + Swapchain Resize + 动态复杂帧 | 48 帧 | Surface/Present/reconfigure |
| **T5** | Stress（可选） | `GPUI_MEM_STRESS=1` 提高迭代 | 200+ | 长跑斜率 |

**T3/T4 每帧随机（可复现种子）组合：**

- 背景色/清屏  
- 圆角卡片、圆、path stroke/dash  
- 半透明 layer + blend  
- clip rect/path  
- 多行 text  
- 小图 `DrawImage`  
- 可选 blur（filters 已注册时）  
- 逻辑尺寸与（窗口）物理尺寸随机步进  

---

## 4. 时间窗判定

每档：

1. **Warmup** 前 10% 迭代（pipeline/atlas 冷启动，允许 RSS 上升）  
2. **Steady** 中间段：采样 RSS；后 1/3 均值 − 前 1/3 均值 ≤ `GPUI_MEM_RSS_DELTA_KB`（默认 T0/T1: 48MB，T3/T4: 96MB）  
3. **Teardown**：`Close` / `release` / `ResetAccelerator` 后仍能完成一次中等 Present  

硬失败（任一）：

- wgpu uncaptured OOM / CreateTexture 失败 / Present error  
- `GPUOps==0` 或 `cpu_fallback_ops!=0`  
- RSS 超 `GPUI_MEM_RSS_HARD_KB`（默认 0=关闭；可设如 4GB）  

---

## 5. 运行方式

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export DISPLAY=:1
export XAUTHORITY=${XAUTHORITY:-/run/user/$(id -u)/gdm/Xauthority}

# 一键（推荐）
./scripts/run_mem_leak_tests.sh

# 或手动：反复 3 次验证稳定性
go test -count=3 ./render -run 'TestMem_' -timeout 600s

# 长压
GPUI_MEM_STRESS=1 GPUI_MEM_ITERS=120 go test -count=1 ./render -run 'TestMem_' -timeout 900s

# 独立窗口示例（可选）
go run ./examples/mem_window_stress
```

Env：

| 变量 | 含义 | 默认 |
|------|------|------|
| `GPUI_MEM_ITERS` | 覆盖默认迭代 | 分档默认 |
| `GPUI_MEM_STRESS` | 启用 T5 | 关 |
| `GPUI_MEM_RSS_DELTA_KB` | 稳态 RSS 软增量上限 | 分档 |
| `GPUI_MEM_RSS_HARD_KB` | RSS 硬顶（0=关） | 0 |
| `GPUI_MEM_SEED` | 随机种子 | 42 |
| `GPUI_FORCE_NO_X11` | 跳过窗口档 | 关 |

---

## 6. 释放链检查清单（与测试对应）

| 链 | 测试 |
|----|------|
| Context.Close → session.Destroy + WaitIdle | T0/T3 |
| offscreen release() | T0/T1/T3 |
| Resize 后旧 MSAA/depth 释放 | T1/T3/T4 |
| ResetAccelerator / GPUShared.Close | T2 |
| Swapchain.Resize + reconfigure | T4 |
| Image/text/layer 长跑有界 | T3/T4 软 RSS |

---

## 7. 退出条件

- [x] 方案文档  
- [x] T0–T4 测试实现；`./scripts/run_mem_leak_tests.sh`（`GPUI_MEM_COUNT=2`）绿  
- [x] 窗口复杂动态场景可自动跑（T4 X11）  
- [x] 脚本 `scripts/run_mem_leak_tests.sh`（进程隔离 + `GPUI_SURFACE_SAMPLE_COUNT=1`）  
- [x] 主线计划记录  

**通过标准**：反复自动测试无 OOM、GPU 路径有效、稳态 RSS 软门不破（或可解释后调参写入文档）。

---

## 8. 已修释放链（2026-07-15）

| 问题 | 修复 |
|------|------|
| `CommandEncoder.Finish` 不 `Release` 原生 encoder | `gpu/webgpu/encoder.go` Finish 后 `r.Release()` |
| `RenderPassEncoder.End` / `ComputePassEncoder.End` 不 Release | End 后 `r.Release()` |
| `FreeCommandBuffer` 为 no-op | 改为调用 `CommandBuffer.Release()` |
| `Device.Release` 不释放 Queue | 先 `queue.Release()` 再 device |
| `GPUShared` 不持有/释放 Adapter | `initGPU` 存 adapter，Close 时 Release |
| Session 误销毁 GPUShared shape pipelines | `ownsShapePipelines`；仅销毁自建/text/image/glyph |
| Vello compute 每 init 急切编译 8 阶段 | 延迟到 `CanCompute` |
| 4x `msaa_probe` uncaptured OOM | `GPUI_SURFACE_SAMPLE_COUNT=1` 跳过 probe |

**验证**：`rwgpu.SetDebugMode(true)` 下 ResetAccelerator 后 `ReportLeaks()` = clean；T0–T4 进程隔离双轮绿。

---

## 9. 与 S4–S6 正确性耦合（必守）

内存修复动到的是 **render → webgpu → rwgpu 释放链**，不是像素语义。但仍必须：

| 门禁 | 命令（建议进程隔离） | 目的 |
|------|----------------------|------|
| L0 帧模型/60fps | `TestS54_|TestS52_|TestS53_` | 帧路径未回退 |
| L1 Comp 抽样 | `TestP1_Comp_(D01\|D06\|D08\|D36\|D63\|D152)_` | 组合像素/语义 |
| L3 present 基线 | `TestS5_PresentBaseline_Scenes` | 性能基线仍可跑 |
| L4 窗口 | 各 `TestS68_*` **单独进程** | 真 present |

**主机注意**（Intel iGPU / 低共享内存）：

- 单进程串跑大量 S6x + 窗口测试可能仍 OOM abort（native uncaptured），**不等于**正确性回退  
- 门禁以 **进程隔离** 为准；全量一进程绿不是本机强制目标  
- 默认 `GPUI_SURFACE_SAMPLE_COUNT=1` 跑 mem / 窗口压力档  

---

## 10. 仍需注意 / 可继续加深

1. **VRAM 精确度**：RSS 是软指标；无 `wgpu` VRAM counter 时仍靠 OOM 硬门 + 隔离  
2. **同进程多代 Device**：iGPU 上多次 `ResetAccelerator` 连续重压可能仍紧；T2 以「压力→Reset→再 Present」一轮为主  
3. **S68 多用例同进程**：建议脚本拆分 `MultiFrameDraw` / `IdleSkip`  
4. **D01–D200 全量**：S6 关闭锁仍在；mem 切片不替代 L2 全量  
5. **控件层**：仍后置；mem 不覆盖 widget  
6. **可选加深**：cmd-buffer 池化、共享 pipeline 真正复用统计、host VRAM 硬顶 env  

---

## 相关：窗口长时压测

见 `docs/MEM_ANIM_LONGSOAK_PLAN.md`（真实 X11 窗口，**每进程单场景**，60s–10min）。

