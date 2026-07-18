# particle_kitchen_sink — 非全屏粒子厨房水槽（诊断压测）

真实 X11 + webgpu present 路径。目的：**用可隔离开关挖出** CPU / 提交 / 混合 / 光晕 Export / 内存爬升 / GPU fallback 问题，而不是刷分。

**纪律**：禁止用减粒子 / 关 AA / silent CPU 装绿。内容密度不够就修底层。

## 运行

```bash
cd examples/particle_kitchen_sink
export WGPU_NATIVE_PATH=$PWD/../../lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/../../lib:$LD_LIBRARY_PATH
export DISPLAY=:1   # 按本机

go run .                              # 默认 L0
GPUI_TIER=L2 go run .
GPUI_PROBE=P_BLEND_LAYER GPUI_ANIM_SECONDS=8 GPUI_RESULT_FILE=/tmp/p.json go run .
GPUI_LIST_PROBES=1 go run .           # 打印隔离探针目录
```

批量：

```bash
# 档位 L0–L4
../../scripts/run_particle_kitchen_sink.sh

# 隔离矩阵（gate/stress/trap + 自动 SUMMARY + FAIL 二分）
../../scripts/run_pks_matrix.sh
GPUI_PKS_FILTER=gate ../../scripts/run_pks_matrix.sh
GPUI_PKS_FILTER=axes GPUI_PKS_OUT=/tmp/pks_axes ../../scripts/run_pks_matrix.sh
GPUI_PKS_FILTER=core GPUI_PKS_OUT=/tmp/pks_core ../../scripts/run_pks_matrix.sh
GPUI_PKS_FILTER=combo GPUI_PKS_OUT=/tmp/pks_combo ../../scripts/run_pks_matrix.sh

# 全面挖问题套件（PKS core+combo + capability C01/C07/C11 → PROBLEMS.md）
../../scripts/run_problem_suite.sh

# dig wall only (clip/grad/filter/text/mesh_wave/...)
GPUI_PKS_FILTER=dig scripts/run_pks_matrix.sh
# 失败模式对照：见 COVERAGE.md

```

## 档位 L0–L4

| Tier | 内容 | 默认 N | 用途 |
|------|------|--------|------|
| **L0** | 实心粒子 | 500 | 基线 60fps / cpu_fb |
| **L1** | + 半透明 Screen/Multiply（层批） | 900 | 混合路径 |
| **L2** | + 有界 glow RT | 1200 | 滤镜/读回 |
| **L3** | + mesh 批 + atlas + layer + 中文 | 1600 | UI 叠压 |
| **L4** | 全开厨房水槽 | 3000 | 综合高压 |

粒子舞台默认约占窗口 **65–72%（非全屏）**。

## 隔离探针矩阵（`GPUI_PROBE`）

| ID | class | 轴 | 说明 |
|----|-------|----|------|
| **P_SOLID** | gate | 实心 | 基线密度 |
| **P_MESH** | gate | DrawMesh | 大批三角 |
| **P_BLEND_LAYER** | gate | 层批高级混合 | 修复后路径，应 ≈60fps |
| **P_BLEND_PER_CIRCLE** | **trap** | 逐圆 SetBlendMode | 历史 ~1fps；用于抓回退回归 |
| **P_GLOW** | stress | glow RT | blur export 成本 |
| **P_ATLAS** | gate | DrawAtlas | 精灵 |
| **P_TEXT** | gate | 中文 | 字体/排版 |
| **P_LAYER** | gate | PushLayer | 半透明层 + RT 池 |
| **P_TRAILS** | gate | stroke 拖尾 | 描边成本 |
| **P_ALPHA_MESH** | gate | alpha mesh+混合 | 密度叠压 |
| **P_RESIZE** | stress | 尺寸振荡 | swapchain/context 重建泄漏 |
| **P_DARK_STAGE** | gate | 暗场少粒子 | 非空内容 + GPU 路径 |
| **P_MEM_SOAK** | stress | 短浸泡（默认 **60s**） | RSS 稳态斜率 |
| **P_BLEND_GLOW** | stress | 混合×光晕 | L2 组合分解 |
| **P_LAYER_BLEND** | stress | 层×混合 | 嵌套 composite |
| **P_MULTI_LAYER** | stress | 多层嵌套 | RT 池/闪烁 |
| **P_SUBMIT_PATH** | stress | 路径提交风暴 | 无 mesh 批 |
| **P_HIGH_N** | stress | N=5000 | 密度 vs 高级路径 |
| **P_FULL_STAGE** | stress | region=100% | 全窗 present |
| **P_CLEAR_ALT** | gate | 交替清屏 | full redraw |
| **P_ATLAS_TEXT** | gate | 图集×文本 | UI 标注 |
| **P_GROW_N** | stress | N 阶梯增长 | realloc/RSS |
| **P_BLEND_CPU** | stress | 混合+CPU预算 | 主线程过重 |
| **P_PIXEL** | gate | 像素指纹 | 小 RT Export RGB 采样 |
| **P_STAGE_SIG** | gate | 舞台签名 | 角标 vs 背景可区分 |
| **P_EMPTY_TRAP** | trap | 空内容陷阱 | 装空绿必 FAIL |
| **P_FLICKER** | gate | 交替清屏闪烁 | intermittent_content |
| **P_FLICKER_BLEND** | stress | 混合+交替清屏 | hitch+闪烁 |
| **P_MEM_LONG** | stress | 加长浸泡（默认 **180s**，固定 N） | 稳态斜率≈0 |
| 时长分层 | — | `P_MEM_SOAK` **60s** 快筛 · `P_MEM_LONG` **180s** 日常 · `GPUI_ANIM_SECONDS=600` **10min** 中泡 · `900/1800` 发版长泡 | 抓慢爬/延迟释放 |
| FPS jitter | — | present→present；dig 后一帧不计；`fps_jitter`=p95−p5 | 避免 harness 假 hitch |
| **P_L0…P_L4** | gate/stress | 档位别名 | 与 L0–L4 对齐 |

`scripts/run_pks_matrix.sh`：

- 写出 `/tmp/pks_matrix/{PROBE}.json` + `SUMMARY.md` + `CATALOG.md`
- FAIL 时按特征自动二分 `GPUI_ENABLE_*=0` / `GPUI_BLEND_CIRCLES`
- 默认：**gate/trap FAIL → 脚本退出非 0**；stress FAIL 记证据但不拦（`GPUI_PKS_STRICT=1` 全严）

## 二分开关（一次只改一个）

| 环境变量 | 含义 |
|----------|------|
| `GPUI_PROBE` | 隔离探针 ID（优先于 TIER） |
| `GPUI_TIER` / `GPUI_SCENARIO` | L0–L4 |
| `GPUI_PARTICLE_N` | 粒子数（仍受 MinN 内容底线约束） |
| `GPUI_REGION` | 舞台占比 0.2–1.0 |
| `GPUI_ENABLE_SOLID/BLEND/GLOW/MESH/ATLAS/TEXT/LAYER/TRAILS` | 功能开关 |
| `GPUI_ENABLE_PER_CIRCLE_BLEND` | 强制/关闭逐圆混合陷阱路径 |
| `GPUI_ENABLE_RESIZE` | 尺寸振荡 |
| `GPUI_BLEND_CIRCLES` | 高级混合圆数量 |
| `GPUI_ALLOW_LOW_FPS` | 放宽帧率门禁（仅记录用） |
| `GPUI_ANIM_SECONDS` | 自动退出秒数 |
| `GPUI_RESULT_FILE` | JSON 结果 |
| `GPUI_LIST_PROBES` | 打印探针目录 |
| `GPUI_TARGET_FPS` | 默认 60 |
| `GPUI_APP_PACE` | 软件稳帧；默认 **开**（本机 fifo Present 不真正等 vblank；`0`=不限速 dig） |
| `GPUI_LOCK_SIZE` | 默认锁 800×600（resize 探针自动解锁） |

## 门禁（JSON `status`）

- `cpu_fallback_ops == 0`
- `gpu_ops > 0` 且 content 标记足够（非空绿）
- `present_errors == 0`
- 默认 gate：`fps_ema ≥ 55`、`fps_avg ≥ 48`；过大 jitter + 低 min 记 hitch
- **trap** `P_BLEND_PER_CIRCLE`：若仍 `fps_ema < 10` → `trap_hot_path_still_slow`
- mem 探针：泄漏判定**只看运行期内平台化** rate≈0（`GPUI_MEM_PLATEAU_RATE_KB_S`）；`GPUI_MEM_RSS_HARD_KB` 仅防 OOM，**无绝对 MiB 爬升门**
- `particle_n >= min_particle_n`（禁止砍内容过门）
- CPU / RSS 仅统计 **本进程**

## FAIL 原因怎么读

| reason | 含义 | 下一步 |
|--------|------|--------|
| `fps_low_*` | present 太慢 | 二分 ENABLE_* / 引擎路径 |
| `trap_hot_path_still_slow` | 逐圆混合仍 ~1fps | dual_tex / advanced blend |
| `cpu_fallback_ops` | CPU 回退 | GPU_FIRST |
| `gpu_ops=0` | 未上 GPU | device/provider |
| `content_fail` / `content_gutted` | 空画或砍密度 | **禁止**降 N 装绿 |
| `present_errors_steady` | 非 resize 宽限内 present 失败 | swapchain |
| `present_errors_resize` / `resize_recover_fails` | resize 不可恢复 | Resize/BeginFrame |
| `cpu_over_budget` | 本进程 CPU 超探针预算 | 批处理/减主线程 |
| `pixel_fail` | 小 RT 纯色采样错误/空栅格 | ExportImageBuf/pixmap |
| `stage_sig_fail` | 角标签名与背景不可分 | 绘制 API 空内容 |
| `rss_*` / `mem_soak_*` | 内存爬升 | layer RT 池 / 泄漏 |

## 修 bug 纪律

1. 一次只开一个开关定位。  
2. 禁止用减粒子/关 AA/ silent CPU 装绿。  
3. 修完本示例后回归 `capability_matrix` 关键场景（C07/C11/C01）。  
4. 全场景对标 Skia：能 GPU 必须 GPU，平台不能才退 CPU。  

## 已用本示例挖到的问题点

1. **逐粒子切换 BlendMode + 大量 DrawCircle**：present 可掉到 &lt;1fps → 层批 + Present 延迟 resolve。  
2. **swapchain 无 COPY_SRC**：不能把 surface 当 dual-tex dest 采样。  
3. **每帧 CreateOffscreenTexture 不回收** → L3/L4 VRAM OOM → size pool。  
4. **全量拖尾**：N 很大时 stroke 爆炸 → 拖尾只画前 N。  

### 根因摘要（引擎，2026-07-16）

- `fillAdvancedBlendTiled`：紧 bounds + dual-tex 无 CPU readback。  
- `PopLayer` 高级混合：延迟到 `FlushGPUWithView`（Present）解析。  
- offscreen layer RT 按尺寸池化；BeginFrame 清理 pending advanced。

## 最新诊断批次结论（`/tmp/pks_diag`）

用隔离矩阵挖到的问题（**不是**装绿）：

| 信号 | 探针 | 含义 |
|------|------|------|
| blend×glow ~48fps | `P_BLEND_GLOW` / `P_L2` | 组合路径掉帧；单独 `P_GLOW`/`P_BLEND_LAYER` 可过 |
| 嵌套层（F1 已修） | `P_MULTI_LAYER` / `P_LAYER_BLEND` ≈58fps | present-stash 避免 mid-frame parent Flush |
| CPU 预算（F1 已修） | `P_BLEND_CPU` ≈32% &lt; 80 | 同上 + dual-tex into-dest |
| 高密度实心仍 60 | `P_HIGH_N` n=5000 PASS | **禁止**用减粒子解释 L2 失败 |
| resize 真路径 | `P_RESIZE` | 必须 `XResizeWindow`+ConfigureNotify，禁止只改 swapchain 尺寸 |
| 1fps 回归陷阱 | `P_BLEND_PER_CIRCLE` | 当前 PASS≈58；若掉到 <10 记 `trap_hot_path_still_slow` |

过滤器：`all` / `gate` / `stress` / `trap` / `axes` / `combo` / `core`。

## 内存门禁（引擎正向优化）

权威流程：`docs/MEM_LEAK_PERF_GUARD_PLAN.md`

```bash
../../scripts/run_mem_guard.sh quick   # TestMem + SOAK 60s + P_SOLID
../../scripts/run_mem_guard.sh daily   # + unit + mem 矩阵 + LONG 180s + 性能抽样
../../scripts/run_mem_guard.sh deep    # LONG 600s
```
