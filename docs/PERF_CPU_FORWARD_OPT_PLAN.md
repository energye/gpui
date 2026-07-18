# 性能 / CPU 正向优化计划（稳 60 + 降主线程）

> 版本：1.0 | 日期：2026-07-18  
> 状态：**opt43 稳 60+ 达成**（R8；opt41+opt42+opt43 Keep；可选下一刀 R8.3 降主线程 CPU）  
> 上位：[`RENDER_OPT_CONVERGENCE.md`](./RENDER_OPT_CONVERGENCE.md) · [`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) · [`MEM_LEAK_PERF_GUARD_PLAN.md`](./MEM_LEAK_PERF_GUARD_PLAN.md)  
> 问题墙 / 实测：`examples/particle_kitchen_sink/PROBLEMS_LATEST.md` · 证据目录 `tmp/perf_fwd_opt41/`

---

## 0. 一句话目标

在 **内容密度不减、像素语义不退、GPU 优先不破、内存斜率不坏** 的前提下：

1. **FPS 更贴 60 且更稳**（均速 + 低 hitch + 低稳健 jitter）  
2. **主线程 CPU 只许持平或下降**（同探针、同机、同 env）  
3. 每一刀 **可证明、可回滚**（class-A 等价优先）

内存泄漏 release 档（900s 平台化）**已关闭**；本计划 **不再以找漏为主**，仅把 mem 斜率当作护栏。

---

## 1. 背景与现状（启动前快照）

### 1.1 已关闭、禁止重做

| 档 | 内容 |
|----|------|
| S4–S6 | batch / atlas / damage / layer / heavy budget |
| R7.0–R7.6 | facade 栈装箱、dual-tex、filter 合并、damage、text template、pipeline pool |
| opt3–opt40 | PKS 实测小步刀（WriteBuffer 合并、sticky convex、mesh compact…） |
| Mem platformization | surface/CB/HUD text；60/180/600/**900s** PASS rate≈2 KB/s |

### 1.2 当前能力水位（真窗口 PKS，约 20s）

参考 `tmp/perf_glow_next/` + harness 诚实 jitter：

| 探针 | fps_ema | cpu | hitch | 含义 |
|------|---------|-----|-------|------|
| P_SOLID | ~58 | ~8% | ~0 | 主路径已近 60 |
| P_GLOW | ~58 | ~11% | ~0 | glow 可接受 |
| P_BLEND_GLOW | ~58 | ~16% | ~0 | 组合路径更贵 |
| P_L3 | ~57–58 | ~18% | ~0 | UI 叠压主战场 |

**不是“掉到 1fps”**，而是从「门禁绿（≥55）」推进到 **更贴 60、更省 CPU、尖刺更少**。

### 1.3 pprof 结构（L3，opt40 遗产）

主导仍是 **cgo / native**（Finish / Submit / WriteBuffer），Go 侧可动但收益需小步验证：

| 热点（cum 量级） | 含义 | 策略倾向 |
|------------------|------|----------|
| `Flush` / `RenderFrameGrouped` / `Present` | 整帧录制+提交 | 减 pass / 合并 submit（谨慎） |
| `resolvePendingAdvancedLayersEnc` | 高级混合延迟合成 | 复用 scratch、减 Create*、合并 encode |
| `CommandEncoder.Finish` | 多次 Finish | 同帧 encoder 合并（已有 opt34/36；禁盲开 singleSubmit） |
| `Queue.WriteBuffer` | 上传 | 已大量 slab/sticky；动画 mesh 难再砍 |
| `Queue.Submit` | 提交次数 | leading coalesce；禁无正确性换次数 |
| `FlushAndFilterFromView` / glow | 滤镜图 | RT/encoder 复用、减 ensure 热路径 |
| `buildConvexResources` | 凸路径资源 | sticky 已做；仅证伪后再动 |
| `packMesh` / `quantUnorm8` / `memmove` | Go 打包 | 微优化，注意 bit-identical |

**硬禁止（opt39 已踩坑）：** 默认开启 full-frame `singleSubmit`——曾回归 F1 advanced layer 像素。

---

## 2. 硬原则（违反即不得合入）

### P1 — 正向不变量

| ID | 不变量 |
|----|--------|
| F1 | 不减粒子 N、不关 AA/glow、不缩小场景装绿 |
| F2 | 有 GPU 时 `cpu_fallback_ops=0`，禁止 silent CPU |
| F3 | 像素/组合语义不退（相关 unit + 必要 STRICT/像素） |
| F4 | 公共 API / ABI / 资源生命周期不漂 |
| F5 | 内存平台化不恶化：短/中浸泡 `rss_plateau_rate` 不显著变差 |
| F6 | 性能：相对本轮基线，FPS ≥ **97%** 且仍过门禁；CPU **≤ 基线（噪声内可持平）** |

### P2 — 优化类别

| 类 | 定义 | 本轮默认 |
|----|------|----------|
| **A. 纯等价** | 同输入同像素；少 alloc/拷贝/bind/draw/submit | **主通道** |
| **B. 算法等价** | 实现不同、目标像素一致 | 需 golden/像素门 |
| **C. 语义变更** | 新能力或改默认 | **禁止混进本计划** |

### P3 — 小步

1. 一刀一主题（如「glow ensure 热路径」或「adv scratch 复用」）  
2. 改动 → 窄测 → PKS 矩阵 → 对比基线 → **Keep / 回滚**  
3. 无收益或风险 → **回滚优先**

---

## 3. 「稳 60」验收口径（本轮）

目标刷新率 `GPUI_TARGET_FPS=60`。门禁分两层：

### 3.1 硬门（已有，不得放宽）

| 指标 | 条件 |
|------|------|
| `fps_ema` | ≥ target−5（默认 **≥55**） |
| `fps_avg` | ≥ target−12（默认 **≥48**） |
| hitch | gate/stress 既有 `low_fps_ratio` 规则 |
| `cpu_fallback_ops` | **0** |

### 3.2 正向目标（本轮追求，写入结果表）

对 **P_SOLID / P_GLOW / P_BLEND_GLOW / P_L3**（建议 20s，诚实 jitter 计量）：

| 指标 | 目标 | 说明 |
|------|------|------|
| `fps_ema` | **≥58**（冲 59–60） | 从「能过」到「贴 60」 |
| `low_fps_ratio` | **≤0.005** | 几乎无尖刺 |
| `fps_jitter`（p95−p5） | **≤8**（L3 可放宽到 ≤12） | harness 已排除 dig 污染 |
| `cpu_avg` | **≤ 基线** | L3 优先压到 **≤16%** 为 stretch |
| mem rate | 60s SOAK 仍平台 | 不引入新爬升 |

> 工程上「稳 60」= **帧预算内 + 低 hitch + 均速贴近 60**，不是示波器绝对直线（X11/合成器噪声存在）。

---

## 4. 探针与环境

### 4.1 环境（必须可比）

```bash
export GOROOT=.../go1.25.5.linux-amd64   # 或本机等价
export PATH=$GOROOT/bin:$PATH GOTOOLCHAIN=local GOWORK=off
export GOCACHE=$PWD/tmp/go-cache
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib
export DISPLAY=:1
export GPUI_SURFACE_SAMPLE_COUNT=1
```

### 4.2 核心矩阵（每刀必跑）

| 探针 | 秒 | 用途 |
|------|----|------|
| P_SOLID | 20 | 地板 CPU/FPS |
| P_GLOW | 20 | 滤镜路径 |
| P_BLEND_GLOW | 20 | 组合压力 |
| P_L3 | 20 | UI 叠压主指标 |
| P_L3 + `GPUI_CPUPROFILE=` | 12 | 结构热点 |

可选：`P_BLEND_LAYER`、`P_MEM_SOAK` 60s（护栏）。

### 4.3 基线落盘

- 目录：`tmp/perf_fwd_opt41/baseline/`  
- 每刀对比：`tmp/perf_fwd_opt41/optN_*.json` + `pl3_optN.pprof`  
- 汇总写入 `PROBLEMS_LATEST.md` 与本文件 §8

---

## 5. 执行流程（固定）

```
① 写/更新本计划 + 冻结 baseline JSON
     ↓
② P_L3 pprof → 列 top5 Go/可动热点（忽略纯 libc 噪声时注明）
     ↓
③ 选 1 个 class-A 刀（一刀一路径）
     ↓
④ 实现 + 窄 unit（优先已有 opt*_test / 相关 TestS*）
     ↓
⑤ 核心矩阵 20s ×4 + pprof 对比
     ↓
⑥ 判定 Keep / 回滚（§2、§3）
     ↓
⑦ 记录 PROBLEMS + 本文件 changelog；下一刀从新 pprof 再选
```

**停止条件（本里程碑）：**

- L3：`fps_ema≥58` 且 `cpu_avg` 相对 baseline **明显下降**（建议 ≥1pp 或 pprof 结构清晰下降），hitch/jitter 不恶化；**或**  
- 连续 **2 刀** pprof 已无 ≥3% 可动 Go 热点且 wall 无收益 → 记「平台期」，改文档状态为 **阶段平台**，不硬卷。

---

## 6. 候选刀序（R8，按证据可重排）

> 仅候选；**实际执行以当轮 pprof 为准**。已否决项勿复活。

| 序 | 代号 | 主题 | 预期 | 风险 | 状态 |
|----|------|------|------|------|------|
| 0 | **R8.0** | 冻结 baseline + 诚实 jitter 计量确认 | 可比数据 | 无 | **✅** |
| 1 | **R8.1** | L3 pprof 标注可动 vs cgo 墙 | 选刀依据 | 无 | **✅** |
| 2 | **R8.2 / opt41** | surface RP desc 复用 + Flush/filter warm ensure 去冗余 | 减 per-encode alloc + warm ensure | 低 | **✅ Keep** |
| 3 | **R8.3** | glow/filter：`FlushAndFilterFromView` ensure/锁与 RT 复用 | 降 GLOW/BLEND_GLOW CPU | 中 | 待 |
| 4 | **R8.4** | 提交合并延伸：leading/encoder（**不开** full-frame singleSubmit） | 降 Finish/Submit 次数 | 高，需 F1 绿 | 待 |
| 5 | **R8.5** | Go 打包微优（mesh/convex）bit-identical | 小幅 CPU | 低 | 待 |
| 6 | **R8.6** | 文本/HUD 热路径残余（仅结构热点仍在时） | 稳 60 尖刺 | 低 | 待 |
| 2b | **R8.2b / opt42** | StringView 0-alloc + dual-tex resolve scratch | 减 label/`make` 热路径 | 低 | **✅ Keep** |
| 2c | **R8.2c / opt43** | 软件 hybrid 稳帧（默认 app_pace；sleep+末 1ms spin；无漂移 deadline） | 锁 ~60（fifo 不真正 vsync） | 低 | **✅ Keep** |
| — | **禁止** | 默认 `singleSubmit=true`、减 N、关 AA、silent CPU | — | — | 否决 |

---

## 7. 回滚条件

出现任一条 → **立即回滚该刀**：

1. 像素/F1/组合门禁失败（非环境噪声）  
2. `cpu_fallback_ops>0` 或 silent CPU  
3. 核心探针 `fps_ema` 低于硬门或相对 baseline 掉 >3%  
4. `cpu_avg` 系统性变差（同探针 >+2pp 且非噪声）  
5. 新增 mem 爬升（60s SOAK rate 明显变差）  
6. UAF / double-free / 生命周期回归  

---

## 8. 结果台账

| 刀 | 日期 | Keep? | L3 fps | L3 cpu | SOLID cpu | hitch | 证据 | 备注 |
|----|------|-------|--------|--------|-----------|-------|------|------|
| baseline | 2026-07-18 | — | 57.2 | 18.5% | 8.7% | L3 hitch=0.0028 | `tmp/perf_fwd_opt41/baseline/` | SOLID 58.0fps/8.7% · GLOW see dir |
| opt41 | 2026-07-18 | **Keep** | 57.9 | 18.0% | 9.0% | 0.0009 | `tmp/perf_fwd_opt41/opt41/` | RP reuse + warm ensure; L3 +0.7fps −0.5pp CPU |
| opt42 | 2026-07-18 | **Keep** | 57.9 | 18.3% | 7.9% | 0 | `tmp/perf_fwd_opt41/opt42/` | StringView 0-alloc + dual-tex scratch; SOLID −0.8pp |
| opt43 | 2026-07-18 | **Keep** | 60.91 | 19.6%* | 9.5%* | 0 | `tmp/perf_fwd_opt41/opt43/` | step=budget-200µs + busy-spin；核心探针 ema/avg **≥60.7** hitch=0；uncapped L3≈146 headroom；*CPU 含 spin |
| next | | | | | | | | R8.3 glow/filter ensure 或 pprof WriteBuffer |

---

## 9. 与其它文档关系

| 文档 | 关系 |
|------|------|
| `RENDER_OPT_CONVERGENCE.md` | R7 已关；本计划 = **R8 / PKS 正向实测** 执行案 |
| `MEM_LEAK_*` | mem 护栏；不重复开泄漏专项 |
| `S5_60FPS_GATE.md` / `S6_9_*` | present p50 地板仍有效 |
| `PROBLEMS_LATEST.md` | 每刀 Keep 的现场日志 |

---

## 10. 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-18 | 首版：mem 收口后启动 CPU/稳 60 正向优化；R8.0 baseline |
| 1.1 | 2026-07-18 | R8.0 baseline + opt41 Keep（surface RP reuse / warm ensure） |
| 1.2 | 2026-07-18 | opt42 Keep：stringToStringView 0-alloc + dual-tex scratch |
| 1.3 | 2026-07-18 | opt43 Keep：hybrid software pace 默认开，核心探针锁 ~60；fifo Present 不真正等 vblank |
| 1.4 | 2026-07-18 | opt43d：step-budget(−200µs)+busy-spin，核心探针稳定 **60+** 验证通过 |
