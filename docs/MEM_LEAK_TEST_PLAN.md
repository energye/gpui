# 内存 / VRAM 泄漏测试方案

> **时长分层说明**：60s 抓快泄漏；180s+ 平台化；600–1800s 慢爬。目标斜率 ≈0。  
> 版本：1.7 | 日期：2026-07-21 | **活文档**  
> 状态：**执行中** — 日常入口 [`MEM_LEAK_PERF_GUARD_PLAN.md`](./MEM_LEAK_PERF_GUARD_PLAN.md) + `scripts/run_mem_guard.sh`  
> 范围：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 非目标：控件层、完整 ASAN/Valgrind 替代  
> 窗口压测：`examples/particle_kitchen_sink`  
> 关联：[`ENGINE_GAPS.md`](./ENGINE_GAPS.md) G3 · device_lost / surface lifecycle

---

## 1. 目标

1. **防死机**：硬门禁捕获 `Not enough memory` / CreateTexture 失败，避免无界吃光显存/系统内存  
2. **分场景观测**：简单→复杂组合，固定迭代与时间窗，看 **稳态斜率** 与 **Teardown 后能否再分配**  
3. **释放链回归**：`Context.Close` / offscreen `release` / `session.Destroy(WaitIdle)` / `ResetAccelerator`  
4. **自动反复验证**：`go test -count=N` 与脚本一键跑；可选独立窗口压测用 `examples/particle_kitchen_sink`

---

## 2. 双轨指标

| 轨 | 指标 | 硬门禁 | 软门禁 |
|----|------|--------|--------|
| **功能/GPU** | present 成功、`GPUOps>0`、`cpu_fallback_ops=0`、无 native abort | ✅ Fail | — |
| **进程 RSS** | `/proc/self/status` VmRSS（Linux） | 硬顶仅防 OOM | **平台化** rate≈0（窗口）；短测用 Δ |
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

### 4.0 窗口浸泡时长分层

| 层级 | 时长 | 入口 | 用途 |
|------|------|------|------|
| 快筛 | **60s** | `P_MEM_SOAK` / `run_mem_guard.sh quick` | 抓 ≥~100–256 KB/s 快泄漏；每次改引擎 |
| 日常 | **180s** | `P_MEM_LONG` / `run_mem_guard.sh daily` | 中等置信平台化；日常门禁 |
| 中泡 | **600s (10min)** | `GPUI_PROBE=P_MEM_LONG GPUI_ANIM_SECONDS=600` | 慢爬 / 延迟释放；改释放链后必跑 |
| 长泡 | **900–1800s** | `GPUI_ANIM_SECONDS=900\|1800` / `run_mem_guard.sh deep` | 发版前；极慢泄漏与阶梯抬升 |

**判定一律：steady 段平台化（斜率≈0）**，不用绝对 MiB 爬升门。  
`P_MEM_LONG` 使用 **固定粒子 N**（增长压力见独立探针 `P_GROW_N`）。  
分段看斜率时建议：预热丢前 20%，再比 前/中/后 各 1/3 的 RSS 均值。

## 4. 时间窗判定

每档：

1. **Warmup** 前 10% 迭代（pipeline/atlas 冷启动，允许 RSS 上升）  
2. **Steady** 中间段：采样 RSS；算法与 PKS 对齐 — **先丢前 20% warmup**，再对剩余做 后 1/3 均值 − 前 1/3 均值 ≤ `GPUI_MEM_RSS_DELTA_KB`  
   （默认 T0/T1: 48MB，T3 SizeChurn: 64MB，T3 Escalating/T4: 96MB；权威表见 PERF_GUARD §2.2）  
3. **Teardown**：`Close` / `release` / `ResetAccelerator` 后仍能完成一次中等 Present  

硬失败（任一）：

- wgpu uncaptured OOM / CreateTexture 失败 / Present error  
- `GPUOps==0` 或 `cpu_fallback_ops!=0`  
- 窗口 mem：**唯一泄漏门** = 运行期内平台化；`rate > GPUI_MEM_PLATEAU_RATE_KB_S` → FAIL
- RSS 超 `GPUI_MEM_RSS_HARD_KB`（仅防 OOM；PKS mem 默认 4GiB；TestMem 默认 0=关）
- **不用**绝对 MiB 爬升（如 128MiB/180s）判定泄漏  

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

# 独立窗口示例（可选）：particle_kitchen_sink 内存探针 / 矩阵
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export DISPLAY=:1

# 短/长内存浸泡 + resize/grow（本进程 RSS；JSON 证据）
# 默认 SOAK=60s / LONG=180s（探针 MemSoakSec）；可用 GPUI_ANIM_SECONDS 覆盖
GPUI_PROBE=P_MEM_SOAK GPUI_RESULT_FILE=/tmp/pks_mem_soak.json \
  go run ./examples/particle_kitchen_sink
GPUI_PROBE=P_MEM_LONG GPUI_RESULT_FILE=/tmp/pks_mem_long.json \
  go run ./examples/particle_kitchen_sink
# 加深示例：GPUI_ANIM_SECONDS=600|900|1800

# 批量 mem 过滤（P_MEM_SOAK / P_MEM_LONG / P_GROW_N / P_RESIZE / P_MULTI_LAYER）
GPUI_PKS_FILTER=mem ./scripts/run_pks_matrix.sh

# 档位 L0–L4 一键（非 mem 专用，可作综合压力）
./scripts/run_particle_kitchen_sink.sh
```

Env（`TestMem_*`）：

| 变量 | 含义 | 默认 |
|------|------|------|
| `GPUI_MEM_ITERS` | 覆盖默认迭代 | 分档默认 |
| `GPUI_MEM_STRESS` | 启用 T5 | 关 |
| `GPUI_MEM_RSS_DELTA_KB` | 稳态 RSS 软增量上限 | 分档 |
| `GPUI_MEM_RSS_HARD_KB` | RSS 硬顶（0=关；PKS mem 默认 4GiB） | TestMem:0 / PKS mem:4GiB |
| `GPUI_MEM_PLATEAU_RATE_KB_S` | 窗口 mem 平台化噪声带（KB/s）；目标≈0 | 256 |
| `GPUI_MEM_SEED` | 随机种子 | 42 |
| `GPUI_FORCE_NO_X11` | 跳过窗口档 | 关 |

Env（`particle_kitchen_sink` 窗口压测，详见 `examples/particle_kitchen_sink/README.md`）：

| 变量 | 含义 | 默认 |
|------|------|------|
| `GPUI_PROBE` | 隔离探针（`P_MEM_SOAK` / `P_MEM_LONG` / `P_RESIZE` 等） | 空=档位模式 |
| `GPUI_TIER` | L0–L4 档位 | L0 |
| `GPUI_ANIM_SECONDS` | 运行秒数 | 探针/档位默认 |
| `GPUI_RESULT_FILE` | JSON 证据路径（含 `rss_*` / status） | 空 |
| `GPUI_PKS_FILTER` | `run_pks_matrix.sh` 过滤（`mem` / `gate` / …） | 全矩阵 |

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
| 真窗口综合 / RSS 浸泡 | `particle_kitchen_sink`：`P_MEM_SOAK` / `P_MEM_LONG` / `P_RESIZE` / `GPUI_PKS_FILTER=mem` |

---

## 7. 退出条件

- [x] 方案文档  
- [x] T0–T4 测试实现；`./scripts/run_mem_leak_tests.sh`（默认 `GPUI_MEM_COUNT=3`）  
- [x] 稳态斜率算法与 PKS 对齐；一键 `./scripts/run_mem_guard.sh`  
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
| L0 帧模型/60fps | `TestS6_L0_|TestS61_` | 帧路径未回退 |
| L1 Comp 抽样 | `TestP1_Comp_(D01\|D06\|D08\|D36\|D63\|D152)_` | 组合像素/语义 |
| L3 present 基线 | `TestS6_PresentBaseline_Scenes` | 性能基线仍可跑 |
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

- **现行窗口/内存浸泡程序**：`examples/particle_kitchen_sink`  
  - 探针：`P_MEM_SOAK` / `P_MEM_LONG` / `P_GROW_N` / `P_RESIZE`  
  - 批量：`GPUI_PKS_FILTER=mem ./scripts/run_pks_matrix.sh`  
  - 说明：`examples/particle_kitchen_sink/README.md` · `COVERAGE.md`  
- **历史（默认不跑）**：`examples/mem_anim_window`（S01–S23）。**新工作默认只用 particle_kitchen_sink**；`mem_window_stress` 非日常入口。
- **权威日常流程**：`docs/MEM_LEAK_PERF_GUARD_PLAN.md` · `./scripts/run_mem_guard.sh`

---

## 10.1 性能正向护栏与加长浸泡计划

数分钟～数十分钟尺度、**修泄漏不得拖垮 FPS/CPU/占用** 的执行清单见：

→ **`docs/MEM_LEAK_PERF_GUARD_PLAN.md`**

## 11. 底层改动后的强制门禁（F1 后现行）

任意改动 **render / webgpu / rwgpu 释放链、present-stash、LoadOp、clip/scissor、session** 后，顺序固定：

| 顺序 | 门禁 | 命令 / 证据 | 目的 |
|------|------|-------------|------|
| **1** | **全量单元测试绿** | `./scripts/run_full_unit_tests.sh` → `tmp/full_unit/summary.txt` | 语义/编译/绑定回归先过 |
| **2** | **内存泄漏观测档** | `./scripts/run_mem_leak_tests.sh`（`GPUI_MEM_COUNT=3`）→ `tmp/gpui_mem_leak_tests.log` | 进程 RSS + OOM 硬门 + 释放链 |
| **3** | 正确性抽样（若动到 present/layer） | L0 `TestS6_L0_|TestS61_` + Comp 抽样 + 相关 F1/P1 像素门 | 防「测绿但画面错」 |
| **4** | 可选长时 / 真窗口 | `examples/particle_kitchen_sink`：`P_MEM_SOAK` / `P_MEM_LONG` 或 `GPUI_PKS_FILTER=mem ./scripts/run_pks_matrix.sh` | 稳态斜率 + present 综合 |

**不得**在单元未绿时 dig 性能或「砍内容」过 mem 门。

### 11.1 全量单元范围

`scripts/run_full_unit_tests.sh` 覆盖（**每包独立进程**，包内 `-parallel 1`）：

- `./gpu/context` `./gpu/types` `./gpu/rwgpu` `./gpu/webgpu` (+ thread)
- `./render/internal/{color,blend,clip,cache,stroke,filter,parallel,raster,wide,gpu,gpu/tilecompute}`
- `./render/text` `./render/text/cache` `./render/filters` `./render/gpu`
- `./render/recording` `./render/scene` `./render/surface` `./render/raster` `./render`

默认 env：`WGPU_NATIVE_PATH`、`GPUI_SURFACE_SAMPLE_COUNT=1`、`DISPLAY=:1`、`GOCACHE=$PWD/tmp/go-cache`。

本机 iGPU / 共享内存紧时：

- **同进程串跑全包** 可能中后段 `RequestDevice: Not enough memory` — **不等于**功能回退；以 **进程隔离脚本** 为准  
- 单包仍 OOM → 查该包 Device/Close 泄漏或拆 `-run` 子进程  
- 证据目录：`tmp/full_unit/`（`summary.txt` / 每包 `.log` / `exit_code.txt`）

### 11.2 内存泄漏观测要求（process-local）

| 项 | 要求 |
|----|------|
| RSS 源 | **仅当前测试进程** `/proc/self/status` VmRSS（或脚本对子进程采样）；**禁止**把整机 CPU/内存当门禁 |
| Warmup | 前 ~10% 迭代允许冷启动上涨 |
| Steady Δ | 后 1/3 均值 − 前 1/3 均值 ≤ `GPUI_MEM_RSS_DELTA_KB`（分档默认见 §4） |
| **泄漏门（窗口）** | 仅平台化：`rate = Δ/(sec×0.8) ≤ GPUI_MEM_PLATEAU_RATE_KB_S`；目标 **≈0** |
| Hard cap | `GPUI_MEM_RSS_HARD_KB` 仅防 OOM；CreateTexture / Present error **硬 Fail** |
| 禁止 | 绝对 MiB 爬升阈值当“无泄漏” |
| GPU 有效 | `GPUOps>0`、`cpu_fallback_ops=0`（或场景声明的 GPU 路径） |
| Teardown | Close / release / Reset 后仍能中等 Present |
| 隔离 | T0–T4 **每档独立进程**；COUNT≥3 反复 |
| 内容 | **不得**为过 RSS/FPS 门砍粒子/层/滤镜内容 |

观测记录建议写入：`tmp/gpui_mem_leak_tests.log` +（可选）场景 JSON 的 `rss_kb` / `steady_delta_kb`。

### 11.3 释放链优先排查顺序（单测绿后）

1. `Context.Close` → `session.Destroy` + `WaitIdle`  
2. encoder `Finish` / pass `End` / `CommandBuffer.Release` / Queue on Device  
3. ImageCache / ephemeral / layer RT pool / glow `ExportImageBuf`  
4. Swapchain reconfigure 旧 MSAA/depth  
5. 同进程多代 Device（ResetAccelerator）  

F1 相关证据：`/tmp/f1_closeout`、`TestF1_AdvancedLayerPresentView*`（present-stash / LoadOpLoad / PrepareTarget）。

### 11.4 与当前主线顺序

```
底层改动（含 F1 present-stash 等）
  → ① 全量单元 ./scripts/run_full_unit_tests.sh 绿
  → ② 内存泄漏 ./scripts/run_mem_leak_tests.sh 绿
  → ③ 再 dig glow hitch / 性能 / 新场景
```



- Evidence 2026-07-18: P_MEM_LONG **900s PASS** rate=2.13 KB/s early/mid/late=3.69/1.36/1.38 (`tmp/mem_release_900/`).
