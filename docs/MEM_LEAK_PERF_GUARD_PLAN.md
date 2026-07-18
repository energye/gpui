# 内存泄漏测试计划（含性能正向护栏）

> 版本：1.1 | 日期：2026-07-18  
> 状态：**执行中（权威入口）**  
> **目标：正向优化渲染引擎** — 修泄漏/收敛冗余时，功能与 FPS/CPU/稳态内存占用只许持平或变好。  
> 时长尺度：**约 1 分钟～30 分钟**（工程稳态；非无限时长）  
> 实现细节分档：`docs/MEM_LEAK_TEST_PLAN.md`  
> 历史窗口程序：`docs/MEM_ANIM_LONGSOAK_PLAN.md`（**默认不跑**，见该文降级声明）

---

## 0. 文档入口（只跟这一份跑）

| 优先级 | 用途 | 文档 / 入口 |
|--------|------|-------------|
| **P0 日常** | 泄漏 + 短浸泡 + 性能护栏 | **本文** + `./scripts/run_mem_guard.sh` |
| P1 细节 | T0–T4 语义 / env | `docs/MEM_LEAK_TEST_PLAN.md` |
| P2 历史 | mem_anim_window 长 soak 档案 | `docs/MEM_ANIM_LONGSOAK_PLAN.md`（可选） |
| 探针说明 | PKS ID 与失败模式 | `examples/particle_kitchen_sink/README.md` · `COVERAGE.md` |

---

## 1. 硬约束（正向优化）

| ID | 约束 |
|----|------|
| C1 | **禁止**减内容、关 AA、降粒子、silent CPU fallback、放宽阈值装绿 |
| C2 | 指标只用 **本进程** `/proc/self` VmRSS + 测试/JSON；禁止整机内存/load |
| C3 | 修泄漏后：FPS / CPU / 稳态占用 **不得整体变差**（§3 护栏） |
| C4 | 优化方向：释放链、池化复用、去每帧 alloc/readback、代码收敛 |
| C5 | 全量单元未绿 → 不得宣称 mem/性能优化完成 |

**非目标：** 证明全部 API 永不泄漏；精确 VRAM 会计；替代 ASAN/Valgrind。

---

## 2. 统一指标与算法

### 2.1 稳态 RSS 斜率（TestMem 与 PKS 已对齐）

```text
1. 丢弃无效样本（≤0）
2. 丢弃前 20% 作为 Warmup
3. 对剩余样本：grow = mean(后 1/3) − mean(前 1/3)
4. grow 超过档位上限 → FAIL（软/硬门见阈值表）
```

实现：

- Go 单测：`render/mem_harness_test.go` → `memAssertSteadyRSS`
- PKS：`examples/particle_kitchen_sink/metrics.go` → `rssSteadyDelta`（JSON 字段 `rss_steady_delta_kb`）

### 2.2 阈值表（权威）

| 场景 | 稳态 Δ 上限 | 说明 |
|------|-------------|------|
| TestMem T0/T1 | **48 MiB** | `GPUI_MEM_RSS_DELTA_KB` 可覆盖 |
| TestMem T3 SizeChurn | **64 MiB** | 见测试默认 |
| TestMem T3 Escalating / T4 | **96 MiB** | 见测试默认 |
| PKS 通用 | **512 MiB** | 防失控 |
| `P_MEM_SOAK` / `P_MEM_LONG` / `P_GROW_N` | **128 MiB** | mem 专用硬顶 |
| 可选 RSS 硬顶 | `GPUI_MEM_RSS_HARD_KB`（默认关） | 防单进程吃光内存 |

### 2.3 默认时长（权威）

| 档 | 时长 | 入口 |
|----|------|------|
| **M0 快检** | TestMem 脚本 + SOAK **60s** | `run_mem_guard.sh` / `run_mem_guard.sh quick` |
| **M1 日常** | SOAK 60s + LONG **180s** + mem 矩阵 | `run_mem_guard.sh daily` |
| **M2 加深** | LONG **600s（10min）** | `run_mem_guard.sh deep` |
| **M3 加长** | LONG **900–1800s** | 手动 `GPUI_ANIM_SECONDS` |

探针默认（无 env 时）：`P_MEM_SOAK` → 60s，`P_MEM_LONG` → 180s（`MemSoakSec`）。  
覆盖：`GPUI_ANIM_SECONDS=<秒>`。

### 2.4 性能正向护栏（相对基线）

| 指标 | 门禁 |
|------|------|
| 相关单测 / F1 像素 | 不得新红 |
| `cpu_fallback_ops` | **= 0** |
| `fps_ema` | ≥ 基线 × **97%**（且满足探针 MinFPS） |
| `cpu_avg` | ≤ 基线 × **105%** |
| 稳态平台 RSS（末段） | ≤ 基线 × **110%** |
| `rss_steady_delta` | ≤ 门禁，且 **≤ 基线**（斜率不恶化） |

---

## 3. 流程图（一览）

```text
环境准备（WGPU_NATIVE_PATH / LD_LIBRARY_PATH / DISPLAY / GOCACHE）
   ↓
① 全量单元  ./scripts/run_full_unit_tests.sh     （quick 模式可跳过）
   ↓ 绿
② 泄漏档    ./scripts/run_mem_leak_tests.sh       （T0→T4 × COUNT，默认 3）
   ↓ 绿
③ 窗口浸泡   P_MEM_SOAK / P_MEM_LONG 或 FILTER=mem
   ↓
③b 性能护栏  P_SOLID / P_BLEND_LAYER / P_L1 等 vs 基线
   ↓
④ FAIL → GPUI_ENABLE_*=0 对分 → 修释放/缓存/复用（禁止砍内容）
   ↓
⑤ 再跑 ②③③b：斜率回阈值 + 性能不回退
   ↓
⑥ 证据目录 + DELTA.md → 合并
```

---

## 4. 一键脚本

```bash
cd /home/yanghy/app/projects/gogpu/gpui
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export DISPLAY=:1

# 最短日常（推荐提交前）
./scripts/run_mem_guard.sh quick

# 日常完整（含 180s LONG + mem 矩阵 + 性能抽样）
./scripts/run_mem_guard.sh daily

# 加深 10min LONG（释放大改后）
./scripts/run_mem_guard.sh deep

# 指定输出目录
GPUI_MEM_GUARD_OUT=tmp/mem_guard_run_fix ./scripts/run_mem_guard.sh daily
```

证据默认：`tmp/mem_guard_<mode>_<timestamp>/`  
含：`mem_leak.log`、PKS JSON、`SUMMARY.md`（摘录 status / fps / cpu / rss_steady_delta）。

---

## 5. 分轨说明（摘要）

### 轨 A — TestMem

```bash
./scripts/run_mem_leak_tests.sh   # GPUI_MEM_COUNT 默认 3
```

T0 CreateClose · T1 RetainedResize · T2 ResetAccelerator · T3 ComplexOffscreen · T4 Window。

### 轨 B — PKS 窗口

```bash
GPUI_PKS_FILTER=mem GPUI_PKS_OUT=/tmp/pks_mem ./scripts/run_pks_matrix.sh
# 或
GPUI_PROBE=P_MEM_LONG GPUI_ANIM_SECONDS=600 GPUI_RESULT_FILE=/tmp/long.json \
  go run ./examples/particle_kitchen_sink
```

### 轨 C — 功能与性能

```bash
./scripts/run_full_unit_tests.sh
# + gate 探针 JSON 与基线对比（脚本 daily/deep 会抽 P_SOLID / P_BLEND_LAYER）
```

---

## 6. 优化闭环（发现问题后）

```text
对分定位 → 最小修复（释放/池/去冗余）
  → 轨 A 绿 → 轨 B 绿 → 轨 C 不回退
  → SUMMARY/DELTA 记录 → 合并
```

允许：Close 配对、RT/buffer 池、有界 atlas、去掉每帧错误 gen/upload、死代码收敛。  
禁止：降负载装绿、扩阈值掩盖爬升。

---

## 7. 成功标准

1. `run_mem_leak_tests.sh`（COUNT=3）全绿  
2. mem 探针 / 矩阵：`rss_steady_delta` 在 §2.2 内  
3. 至少按模式达到对应时长（quick/daily/deep）  
4. 性能护栏相对基线无超标回退  
5. 结论表述限定为：  
   > 在 T0–T4 + PKS mem 场景、约 1～30 分钟尺度上，稳态内存可接受且性能未回退。

---

## 8. 变更记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-18 | 1.0 | 初版：时长档 + 性能护栏 |
| 2026-07-18 | 1.1 | 审查后：统一斜率算法与阈值表；SOAK/LONG 默认 60s/180s；`run_mem_guard.sh`；标明权威入口 |
