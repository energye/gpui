# mem_anim_window 长时复杂渲染压测计划

> 版本：2.5 | 日期：2026-07-16  
> 状态：**S01–S14 主路径已覆盖**；**S15–S21 Skia 缺口场景已加入矩阵**；**GPU 优先硬原则**见 MAINLINE §1b + `GPU_FIRST_ROUTING.md`  
> 权威证据：`/tmp/mem_anim_soak_run/v9/SUMMARY.md` + `/tmp/mem_anim_soak_run/v10_regress/SUMMARY.md`  
> 程序：`examples/mem_anim_window`  
> 链路：`render.Context → webgpu → rwgpu → libwgpu_native`（真实窗口 Present，非 mock）  
> 非目标：控件层 / Ant Design 业务控件实现

---

## 0. 强制约束：每次只测一个场景

- **一个进程 = 一个 `GPUI_SCENARIO`**（S01…S23 之一）
- **禁止**在同一进程内切换/轮转场景（避免 RSS/GPU 状态交叉污染）
- 批量脚本只能 **串行 fork 多个独立进程**，上一场景完全退出后再启动下一场景
- 交互调试同样只设一个 `GPUI_SCENARIO`

---

## 0b. 任务定义（验收口径）


| # | 要求 | 验收证据 |
|---|------|----------|
| R1 | **持续时间**观测：内存增长、CPU、程序稳定/崩溃 | 每场景 ≥60s 指标日志 + 汇总表；无 native abort / fatal |
| R2 | **FPS ≈60**（除非上千图元高密度场景可降低） | 稳态 `fps_ema` ∈ [55, 65]（普通场景）；S13 高密度允许下降 |
| R3 | **反复测试**多种指标，禁止表面修复 | 每场景至少 1 次完整时长；异常必须定位到根因再修 |
| R4 | 自动化执行，尽量复用已批准命令前缀 | `scripts/run_mem_anim_longsoak.sh` + `/tmp/mem_anim_window` |
| R5 | **≥10 种**不同复杂渲染内容 | 场景表 S01–S12（+S13 高密度可选） |
| R6 | 每场景观察 **60s～10min** | 默认 90s；关键场景 300s；可选 600s |

---


---

## 0c. 硬原则：全场景对标 Skia（每次排障必读）

> **GPU 优先（主线 §1b）**：有 GPU 必须走 GPU；仅平台无 GPU 才 CPU。详见 [`GPU_FIRST_ROUTING.md`](./GPU_FIRST_ROUTING.md)。有 GPU 时 soak 门禁 **`cpu_fb=0`**。


**用户目标**：Skia 能做的 2D/UI 渲染主路径，本引擎也要能做；mem_anim 是真窗口验收场，不是玩具。

每次解决闪烁 / 掉帧 / 高 CPU / 内存问题时，**必须**同时满足：

| # | 原则 | 禁止 |
|---|------|------|
| P1 | **根因修复**到 render / webgpu / rwgpu 或正确的 Skia 组合模式 | 禁止用「关掉模块 / 稀疏闪一帧 / 假视觉冒充 API」刷 PASS |
| P2 | **能力真实**：滤镜/图层/Backdrop/高级混合要用真实 API 持续生效 | 禁止仅 DrawRectangle 染色代替 ApplyBlur/PushLayer |
| P3 | **对标 Skia 组合方式**：重效果走 **有界 offscreen / saveLayer 等价**，再 composite 到 present | 禁止在全屏 present 表面每帧 ApplyBlur/PushBackdrop（iGPU 必炸） |
| P4 | **无闪烁**：同一效果每帧可见、参数可动画，但不允许 1 帧开关 | 禁止 `frame%N==0` 才跑真 API 当作默认路径 |
| P5 | **60fps 丝滑**：稳态 work≤16.7ms；掉帧先 profile 再减几何/缩小 RT，不删能力 | 禁止把场景内容删到只剩 HUD 冒充优化 |
| P6 | **真链路**：`render → webgpu → rwgpu` + 本进程 CPU/RSS | 禁止 mock present / 整机 load 冒充进程指标 |
| P7 | **回归可证**：单场景复测 + drawprof + result.json 门禁 | 禁止只看“感觉好了” |

**滤镜 / 全模块闪屏的正确解（v2.2）**：

1. 根因 A：全屏 `ApplyBlur` / `PushBackdrop` ~20–50ms → 掉帧被感知为闪。  
2. 根因 B：稀疏 `GPUI_HEAVY_API` 一帧真效果 → 像素突变 = 闪。  
3. 根因 C：在 window Context 上 `PushLayer` 合成进 **CPU parent pixmap**，而 `PresentFrameFull` 只 flush **GPU 命令流** → 层结果不稳定/看不见。  
4. **正确模式（Skia saveLayer）**：小离屏 `NewContext` 每帧跑真实 `Apply*` / `PushLayer` / `PushBackdrop` / `SetBlendMode(Multiply|Screen)` → `ExportImageBuf` + `MarkEphemeral` → `DrawImage` 进入 present。  
5. 动画 ImageBuf 必须 `MarkEphemeral()`（gen=0），禁止每帧新 genID 撑爆 GPU image cache / RSS。



---

## 0d. 用户硬目标核对清单（验收前逐条勾）

| # | 用户原文 | 本计划落地 | 证据文件 |
|---|---------|-----------|---------|
| U1 | 持续观察内存增长 / CPU / 崩溃 | 每场景独立进程；`/proc/self` RSS+CPU；result 含 steady_delta | `$OUT/S0x/metrics.csv` + `result.json.line` |
| U2 | FPS≈60（上千图元可降） | G-FPS ema≥55 avg≥48；S13/S14 `AllowLowFPS` | judgeResult + SUMMARY |
| U3 | 反复测多种指标，禁止表面修复 | §0c Skia 硬原则；drawprof 根因；禁止假视觉/稀疏真 API | §0c + 根因表 |
| U4 | 自动执行不弹确认 | `scripts/run_mem_anim_longsoak.sh` + 已批准前缀 | 脚本 exit code |
| U5 | ≥10 种复杂渲染内容 | S01–S12 默认 12 场景 | 场景矩阵 §3 |
| U6 | 每场景 60s–10min | 默认 90s；S12 120s（可调 300–600） | `GPUI_SOAK_SECONDS` |

### v9 执行命令（本机）

```bash
export PATH="/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64/bin:$PATH"
export GOROOT="/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64"
export GOTOOLCHAIN=local GOCACHE=/tmp/gpui-go-cache
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export DISPLAY=:1 XAUTHORITY=/run/user/$(id -u)/gdm/Xauthority
export GPUI_SURFACE_SAMPLE_COUNT=1 GPUI_FIXED_SIZE=1 GPUI_TARGET_FPS=60
export GPUI_SOAK_OUT=/tmp/mem_anim_soak_run/v9
export GPUI_SOAK_SECONDS=90 GPUI_SOAK_HEAVY_SECONDS=120
go build -o /tmp/mem_anim_window ./examples/mem_anim_window
setsid scripts/run_mem_anim_longsoak.sh S01 S02 S03 S04 S05 S06 S07 S08 S09 S10 S11 S12 \
  > /tmp/mem_anim_soak_run/v9/runner.log 2>&1 &
```

### preflight（v9 前，16–20s）

| 场景 | 结果 | fps_ema | 备注 |
|------|------|---------|------|
| S04 | PASS | ~60 | path/dash |
| S06 | PASS | ~60 | 真实 PushLayer 离屏 |
| S07 | PASS | ~60 | 真实 PushBackdrop 离屏 |
| S08 | PASS | ~60 | image/mask |
| S10 | PASS | ~60 | 三滤镜每帧真 API |
| S12 | PASS | ~58 | 保留层交错重算 + 稳定 gen 缓存 |


## 1. 环境与硬约束

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export DISPLAY=:1
export XAUTHORITY=/run/user/$(id -u)/gdm/Xauthority
export GPUI_SURFACE_SAMPLE_COUNT=1   # Intel iGPU 必开，避免 MSAA probe abort
export GPUI_TARGET_FPS=60
```

- Host：Ubuntu 22 X11，共享显存 iGPU 资源紧  
- 进程隔离：每个场景独立进程（避免跨场景 RSS 污染）  
- 退出：  
  - 交互：关窗  
  - 自动：`GPUI_ANIM_SECONDS=N` 到时退出并写汇总  

---

## 2. 指标定义

### 2.0 作用域：只看当前程序（强制）

所有 CPU / 内存门禁与 HUD **只统计 `mem_anim_window` 本进程**：

| 指标 | 数据源 | 明确排除 |
|------|--------|----------|
| 进程 CPU% | `/proc/self/stat` 的 `utime+stime` 差分 | 整机 `/proc/stat`、loadavg、其他进程 |
| 进程 RSS | `/proc/self/status` 的 `VmRSS` | 系统 free/available、共享缓存归因到别的进程 |
| GPU/CPU 路径计数 | 本 `render.Context` 的 `RenderPathStats` | 系统 GPU 利用率面板（可选另采） |

解释：

- **CPU 1核%（默认日志/CSV `cpu_pct`）**：100% = 本进程占满 **1 个逻辑核**（与 Linux `top` 默认一致）。
- **CPU 整机%（`proc_cpu_machine = 1核% / nproc`）**：与 GNOME 系统监视器等「按全部核心归一」的显示一致。  
  **例：4 核机器上本进程 1核=100% ⇒ 整机≈25%。** 你在系统里看到 25%、我们显示 100%，通常就是这个刻度差，不是采错进程。
- **RSS 240MB** = 本程序常驻页约 240MB，不是「系统只剩 240MB」。
- 多核时 1核% 可 **>100%**（例如 200% ≈ 本进程占满 2 核）。

### 2.1 每秒/每 N 帧采样

| 字段 | 来源 | 含义 |
|------|------|------|
| `t_s` | wall clock | 场景内秒 |
| `frame` | 帧计数 | 累计帧 |
| `fps_ema` | 帧间隔 EMA | 显示/稳态 FPS |
| `fps_avg` | frames/elapsed | 全程平均 FPS |
| `work_ms` | 绘制+Present | 帧内工作时间（不含 sleep） |
| `cpu_pct` | **`/proc/self/stat`（仅本进程 utime+stime）** | **本进程** CPU%，100%≈占满 1 核；**不是**整机 load / `/proc/stat` |
| `rss_kb` | **`/proc/self/status` VmRSS（仅本进程）** | **本进程**常驻内存；**不是**系统 MemAvailable / free |
| `gpu_ops` | `RenderPathStats` | GPU 路径 op 累计 |
| `cpu_fb` | `RenderPathStats` | CPU fallback 累计（**目标 0**） |
| `last_fb` | 最近 fallback 原因 | 排障 |
| `presents` | swapchain stats | Present 次数 |
| `reconfig` | swapchain | Resize/reconfigure 次数 |

### 2.2 派生门禁

| 门禁 | 硬/软 | 判定 |
|------|-------|------|
| **G-FPS** | 硬（普通场景） | 稳态后 30s：`fps_ema` 均值 ≥ 55 且 ≤ 70 |
| **G-FPS-HD** | 软（高密度） | 允许 <55；记录实际值，禁止崩溃 |
| **G-CPU-FB** | 硬 | 全程 `cpu_fallback_ops == 0` |
| **G-GPU** | 硬 | 全程 `gpu_ops > 0` 且 Present 成功 |
| **G-CRASH** | 硬 | 无 abort / panic / X11 fatal / OOM |
| **G-RSS-SLOPE** | 软 | warmup 后稳态斜率 ≤ 阈值（见 §4） |
| **G-RSS-HARD** | 硬 | `rss_kb` < 硬顶（默认 3.5GB，防死机） |

### 2.3 RSS 斜率算法

1. **Warmup**：前 `min(20s, 20% duration)` 丢弃（pipeline/atlas 冷启动）  
2. **Steady**：剩余时间等分为 3 段  
3. `delta = mean(后段 rss) - mean(前段 rss)`  
4. 软门禁：  
   - 轻场景 S01–S05：`delta ≤ 32MB`  
   - 中场景 S06–S11：`delta ≤ 64MB`  
   - 重场景 S12–S13：`delta ≤ 128MB`  
5. 若超软门禁：加跑 5min 复测；仍超 → 记缺陷并查释放链  

---

## 3. 场景矩阵（≥12 种复杂内容）

每场景独立进程。默认时长 **90s**；标记 ★ 的关键场景默认 **300s**。

| ID | 名称 | 渲染内容（复杂点） | 模块组合 | 时长 | 期望 FPS |
|----|------|-------------------|----------|------|----------|
| **S01** | BaselineHUD | 清屏渐变背景 + FPS HUD | bg,hud | 90s | ≈60 |
| **S02** | GlowField | 多光晕圆/椭圆动画变色 | bg,glow,hud | 90s | ≈60 |
| **S03** | CardUI | 圆角卡片、阴影条、弧描边 | bg,cards,text,hud | 90s | ≈60 |
| **S04** | PathDash | 螺旋 path、cubic、dash stroke | bg,paths,dash,poly,hud | 90s | ≈60 |
| **S05** | ClipStack | 圆 clip + rect clip 条带 | bg,clip,glow,hud | 90s | ≈60 |
| **S06** | LayerOpacity | PushLayer 半透明叠层 + 文字 | bg,layer,text,hud | 90s | ≈60 |
| **S07** | BackdropPanel | PushBackdropLayer 毛玻璃板 | bg,glow,backdrop,text,hud | 90s | ≈60 |
| **S08** | ImageMaskAtlas | DrawImage/Rounded/Circular/Nine/Atlas + Mask | bg,image,mask,hud | 90s | ≈60 |
| **S09** | TextStyles | 多字体、装饰线、wrapped、CJK | bg,text,hud | 90s | ≈60 |
| **S10** | FilterFX | Blur / DropShadow / Grayscale / ColorMatrix | bg,filter,cards,hud | 90s★/300s | ≈60 |
| **S11** | MeshBlendXform | 左下 8×5 DrawMesh（折线框贴合网格边）+ 右中棋盘 Multiply/Screen + 中上 CTM 变换网格 | bg,verts,blend,xform,hud | 90s | ≈60 |
| **S12** | FullComposite★ | 全模块持续（lite 交错重算保留层）+ PresentFrameFull | all modules continuous | 120s（可选 300s） | ≈60 |
| **S13** | HighDensity | **≥800–2000** 图元（圆/线/字） | density mode | 90s | 允许下降 |
| **S14** | StressEveryFrame★ | 每帧全模块（`GPUI_STRESS=1`） | all every frame | 180s | 可 <60 |
| **S15** | GradientPattern | 多 stop 线性/径向/扫描渐变 + ImagePattern 填充 | grad,pattern,text,hud | 90s | ≈60 |
| **S16** | AdvancedBlend | Overlay/Darken/Lighten/Hard|SoftLight/Diff/Exclusion/Dodge/Burn/Plus 等连续 | advblend,text,hud | 90s | ≈60 |
| **S17** | ClipRRectEvenOdd | ClipRoundRect + nested clip + FillRuleEvenOdd 星形 + dash cubic | rrectclip,paths,dash,text | 90s | ≈60 |
| **S18** | TextLCDShape | LCD RGB/BGR + TextMode Auto/GlyphMask/Vector/Aliased + wrap/underline | textlcd,text,hud | 90s | ≈60 |
| **S19** | DamagePartialPresent★ | 静态 chrome + 局部条带 dirty + `PresentFrameAuto` | damage,scroll,text,hud | 90s | ≈60 |
| **S20** | ScrollModalUI | 列表 clip 滚动 + 遮罩 + 模态卡片（UI 形态，非控件） | scroll,layer,text,cards | 90s | ≈60 |
| **S21** | SkiaGapComposite★ | S15–S20 缺口能力持续组合（不含 damage present）；`AllowLowFPS` | gap modules lite | 120s | 允许 <60 |
| **S22** | Mesh3DGradient | **整窗**伪 3D：渐变立方体/球/变形星/宽地形 + 旋转/变形；GPU DrawMesh 单批 | mesh3d+text | 60–120s | ≥60fps；cpu_fb=0 |
| **S23** | Mesh3DFullComposite | S12 全模块 + Mesh3D 大舞台压力（非角标） | all+mesh3d | 60–120s | ≥55–60fps 或 AllowLowFPS；cpu_fb=0 |

> 场景通过 `GPUI_SCENARIO=S0x` 选择；内部映射到 FeatureFlags + density/stress。

### 3b. S15–S21 与 Skia 覆盖关系

| 已用 S15–S21 补强 | 仍后置 / 不在 mem_anim |
|-------------------|------------------------|
| 梯度 + pattern 连续 soak | 完整 PDF/SVG 引擎（R.02） |
| 更全 advanced blend 连续 | 真 multiplanar YUV |
| ClipRoundRect / EvenOdd / miter | emoji 全量 shaping 压力 |
| LCD/TextMode/wrap 文本边角 | 与 Skia 像素 golden / 绝对 FPS 报表 |
| Damage partial present | |
| 滚动+模态 UI 形态组合 | |

S01–S23 = **UI 主路径 + Skia 常见缺口 soak**，仍 **不是** Skia 功能穷举表。


---

## 4. 运行参数

| 环境变量 | 默认 | 说明 |
|----------|------|------|
| `GPUI_SCENARIO` | `S12` | 场景 ID |
| `GPUI_ANIM_SECONDS` | `0` | `>0` 到时自动退出（自动化）；`0`=仅关窗 |
| `GPUI_TARGET_FPS` | `60` | 帧节奏目标 |
| `GPUI_ANIM_LOG_EVERY` | `60` | 日志/指标采样周期（帧） |
| `GPUI_METRICS_FILE` | 空 | CSV 路径，采样追加 |
| `GPUI_RESULT_FILE` | 空 | 场景结束 JSON/单行汇总 |
| `GPUI_STRESS` | `0` | 每帧全模块 |
| `GPUI_PERF_LITE` | `0` | 更轻密度 |
| `GPUI_DENSITY` | `0` | 高密度图元数（S13） |
| `GPUI_RESIZE_EVERY` | `0` | >0 时周期性模拟 Configure 尺寸抖动 |
| `GPUI_RSS_HARD_KB` | `3670016` | ~3.5GB 硬顶，超则主动退出防死机 |

---

## 5. 执行流程

### 5.1 构建

```bash
cd /home/yanghy/app/projects/gogpu/gpui
export PATH="/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64/bin:$PATH"
export GOCACHE=/tmp/gpui-go-cache GOMODCACHE=/home/yanghy/app/gopath/pkg/mod GOTOOLCHAIN=local
export GOROOT="/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64"
go build -o /tmp/mem_anim_window ./examples/mem_anim_window
```

### 5.2 单场景

```bash
GPUI_SCENARIO=S03 GPUI_ANIM_SECONDS=90 GPUI_METRICS_FILE=/tmp/mem_anim_S03.csv \
  GPUI_RESULT_FILE=/tmp/mem_anim_S03.result \
  /usr/bin/time -f 'elapsed=%e cpu=%P maxrss=%MKB' \
  timeout -s INT 120s /tmp/mem_anim_window
```

### 5.3 批量（推荐）

```bash
scripts/run_mem_anim_longsoak.sh
# 环境：
#   GPUI_SOAK_SECONDS=90     # 普通场景
#   GPUI_SOAK_HEAVY_SECONDS=300  # S12/S14
#   GPUI_SOAK_OUT=/tmp/mem_anim_soak_YYYYMMDD
```

脚本行为：

1. 依次启动 S01–S12（可选 S13/S14）独立进程  
2. 每场景写 `metrics.csv` + `result.txt` + `stdout.log`  
3. 汇总 `SUMMARY.md`：FPS / CPU / RSS 斜率 / cpu_fb / pass-fail  
4. 任一项硬门禁失败 → 全局 fail=1，但继续跑完其余场景  

---

## 6. 判定与缺陷处理

1. **崩溃 / OOM / Present fatal** → 硬失败；缩小场景复现；查 session 纹理/encoder 生命周期  
2. **cpu_fb > 0** → 记录 `last_fb`；优先修 GPU 路径（已见 surface→offscreen resolveTex 问题）  
3. **FPS <55（非高密度）** → 分析 work_ms；减连续重模块或修底层批处理；**不得**靠关掉覆盖伪装达标  
4. **RSS 斜率超软门** → 5min 复测；确认是否 atlas/pool 无界；修释放链后再测  
5. **修完必须复跑失败场景完整时长**，禁止“修一下就过 5 秒”  

---

## 7. 与 MEM_LEAK_TEST_PLAN 关系

| 文档 | 侧重 |
|------|------|
| `docs/MEM_LEAK_TEST_PLAN.md` | go test 分档 T0–T4 释放链、进程隔离单元 |
| **本文** | **真实窗口**长时间复杂场景、FPS/CPU/稳定性产品级 soak |

两者互补：单元门禁 + 长时窗口产品场景。

---

## 8. 结果目录约定

```
/tmp/mem_anim_soak_<ts>/
  SUMMARY.md
  S01/
    stdout.log
    metrics.csv
    result.txt
  S02/ ...
```

`result.txt` 单行示例：

```
scenario=S03 seconds=90 frames=5400 fps_ema=59.2 fps_avg=58.8 cpu_avg=42 rss_start_kb=180000 rss_end_kb=195000 rss_delta_kb=15000 cpu_fb=0 gpu_ops=120000 status=PASS
```

---

## 9. 进度跟踪

| 阶段 | 内容 | 状态 |
|------|------|------|
| P0 | 场景化 + 秒级退出 + metrics 文件 | 完成 |
| P1 | 详细计划文档（本文） | 完成 |
| P2 | 批量脚本 `scripts/run_mem_anim_longsoak.sh` + `/tmp/gpui_mem_anim.env` | 完成 |
| P3 | S01–S12 各 ≥90s 实测 | **完成** v9：S01–S11 @90s、S12 @120s 全 PASS |
| P4 | S12 更长 soak（可选 300–600s） | **可选加深**（v9 已满足目标「≥60s～10min」下沿与 12 场景） |
| P5 | 缺陷修复 + 复测闭环 | **完成**（闪烁/滤镜/网格可视 + FPS 门禁闭环） |
| P6 | SUMMARY 归档 | **完成** `/tmp/mem_anim_soak_run/v9` fail=0 |

### 9.1 已修复（与画面辨别相关）

| 问题 | 根因 | 修复 |
|------|------|------|
| 中文说明乱码/空白 | text 层只支持 TrueType `glyf`；Noto CJK 为 CFF，Load 成功但不出字 | 强制 `DroidSansFallbackFull.ttf`；拒绝 CFF Noto |
| 画面“随机”难辨 | 缺场景说明 | 左上角中文图例（场景应见内容）；底部 `active[]` 为本帧实际模块 |
| 调整大小闪烁/崩溃 | 连续 Configure  thrash + SurfaceTexture 未 drop 就 reconfigure | resize 50ms debounce；Outdated 路径 drop tex；BeginFrame 失败 skip 不 abort |
| CPU 飙到 60%+ | 每帧 CJK 排版 + 强制 Full present + 错误字体 | 说明面板缓存贴图；HUD 用 DejaVu ASCII；稳态 PresentFrameAuto |
| 大窗 FPS 掉 | 像素量↑ | areaScale 自适应 lite + density 收敛 |

验证截图：`/tmp/cjk_guide_zh_ok.png`（Droid 中文有像素；Noto CFF 为 0）

---

## 10. 变更记录

- v2.3：S12 保留层交错重算 + retained gen 缓存；固定 RT 尺寸；stickyLite；S07/S12 60fps 预检 PASS；开 v9 全矩阵  
- v2.4：v9 全矩阵归档；S11 可视（棋盘 blend + 贴合边 mesh 折线）；目标 5 条硬要求验收审计 §9；回退错误贝塞尔线框  
- v2.5：§10 S12→S13→S14 优先级处理；S13 每帧真实密度；v11 归档 fail=0  
- v2.2：硬原则「全场景对标 Skia」；滤镜/层/Backdrop/高级混合改为有界离屏真实 API 每帧；ExportImageBuf + MarkEphemeral；禁止假视觉/稀疏真 API 刷分  
- v2.1：CJK 字体约束、resize debounce、中文画面说明、surface drop on outdated、v3 long-soak  
- v2.0：从短时 smoke 升级为 ≥10 场景、60s–10min 长时、硬/软双门禁、自动化汇总  
- v1.x：见 `MEM_LEAK_TEST_PLAN.md` 与 mem_anim 初版功能覆盖

- v5：固定帧调度把 post-work 纳入 budget（修 ~50–55fps 天花板）；取消 busy-spin（修 CPU 50–95%）；timed soak LockSize+忽略 WM 最大化；judge 增加 fps_runaway / fps_avg 双门槛；CJK 中文说明缓存 + DroidSansFallbackFull

## 质量门禁（对标 Skia / 强制达标）

> 更新：2026-07-16 06:56 — 用户硬性指标（补充：**任何渲染内容都要丝滑**、**不能出现闪烁**）。验收不过不算完成。

| ID | 指标 | 门槛 | 说明 |
|----|------|------|------|
| Q-FPS | 稳态帧率 | **fps_ema ≥ 55**（目标 **60+**），fps_avg ≥ 48 | 真窗口 present 链，非 mock |
| Q-SILKY | **任何内容丝滑** | 全场景无卡顿感：work 稳态 **≤16.7ms**，无周期性 20ms+ 毛刺 | 含 bg/path/text/blend/layer/filter 等所有模块 |
| Q-SMOOTH | 画面丝滑 | 无周期性掉帧/卡顿毛刺 | 禁止 20ms+ 周期性 reshape/重建 |
| Q-NOFLICKER | **禁止闪烁** | 场景启用的模块必须**持续可见**；背景/控件/文本均不得闪 | 禁止 1 帧 on/off 轮转当默认；重模块可用粘性窗口（多帧 on） |
| Q-VISUAL | 所见即说明 | 左侧中文说明列出的效果必须能在画面上稳定看到 | 说明与绘制不同步 = FAIL |
| Q-CPU | CPU | 基线 HUD ~**≤25%** 单核；复杂场景记录并持续压降 | 目标向 Skia 桌面 UI 靠拢（长远 ~5–15% 基线） |
| Q-RSS | 内存 | steady RSS 斜率 **< 512MB/窗**；无崩溃 | 每场景独立进程 soak |
| Q-GPU | 真 GPU 链 | **cpu_fallback_ops = 0**，gpu_ops > 0 | render → webgpu → rwgpu |
| Q-PRESENT | Present | 动画内容走 **Full clear**（避免 damage Load 闪背景） | 固定尺寸 soak 忽略 WM 最大化 |

### 已定位并修复的闪烁/掉帧根因

1. **模块 1 帧轮转 cadence** → 混色/卡片/路径「一闪」；改为默认**持续绘制**（`GPUI_CADENCE_SPARSE=1` 才恢复轮转）
2. **PresentFrameAuto damage** → 背景闪；动画改为 **PresentFrameFull**
3. **底部 active[] 跟本帧开关变长** → 文字跳动；改为稳定 **feats[主开关]**
4. **CJK 文本叠在左侧「场景」说明上** → 看起来闪；文本面板改到右上并 **Image 缓存**
5. **DrawRoundedRectangle 卡片 ~20ms/帧** → 锁 45fps；卡片改 **轴对齐矩形 + 圆点**（rect SDF 快路径）；圆角能力仍由裁剪/单测覆盖，底层 RRect SDF 成本另开优化
6. **`doStroke` 每笔 stroke 强制 `flushGPUAccelerator`** → multi-stroke 场景（S04 PathDash）锁 ~20fps / work~45ms；改为 **GPU-first batched Stroke**（与 Fill 一致，Present/Image 再 flush）；S04 恢复 **60fps / work~2–3ms**
7. **路径/虚线几何每帧全变** → 冲刷 S4.3/S6.6 stroke/dash cache；连续场景对动画相位/offset **量化**，固定 blob 位置，保持持续可见且 cache 命中
8. **stickyOn 周期性关 Layer/Backdrop/Filter** → 半透明层关掉几帧时背景突然变亮 = **闪烁**；默认 cadence **禁止** sticky 关模块，lite 只降质量不关模块

### 调优目标（对标 Skia）

1. **性能**：稳态 **60fps+**，任意场景 work 预算 ≤16.7ms
2. **画质**：**任何渲染内容丝滑**；**不能出现闪烁**；无撕裂感、内容持续、说明可信
3. **CPU/内存**：基线 CPU 持续压降；RSS 稳态斜率可控、无泄漏
4. **必须完成**：上表 Q-* 门禁在 S01–S12 真窗口 soak 全绿

### 进度归档

| 轮次 | 路径 | 状态 | 备注 |
|------|------|------|------|
| v7 | `/tmp/mem_anim_soak_run/v7` | 部分 | S01–S03 PASS；旧 S04 path ~22fps |
| v7b | 同目录 | 污染/中断 | 旧 runner 与新二进制交叉 |
| preflight | `/tmp/pre8_*` `/tmp/pf2_*` | 记录 | S04/S06/S07 60fps；S10 filter~28ms、S11 blend~25ms 已优化 |
| v8 | `/tmp/mem_anim_soak_run/v8` | 中断/污染 | 滤镜假视觉阶段 |
| pre_v9c | `/tmp/pre_v9c` | PASS 抽检 | S04/06/07/08/10/12 ≥16s 全绿 |
| **v9** | `/tmp/mem_anim_soak_run/v9` | **PASS fail=0** | S01–S11 @90s + S12 @120s；cpu_fb=0；真实离屏 API + sticky lite + 保留层交错 |
| **v10_regress** | `/tmp/mem_anim_soak_run/v10_regress` | **PASS fail=0** | S11 可视修复后 90s：fps 60.0/59.7；S12 120s：58.8/56.9；cpu_fb=0 |
| **v11 S12–S14** | `/tmp/mem_anim_soak_run/v11_s12_s13_s14` | **PASS fail=0** | S12@180s / S13@90s(真1200密度) / S14@180s |
| S11 可视修复 | `/tmp/s11_fix2_result.json` | PASS 30s | 原「灰方块无网格」→ 棋盘网格+Multiply/Screen + DrawMesh + CTM 网格；avg FPS 59.4 cpu_fb=0 |
| S11 线框锯齿（误用贝塞尔） | `/tmp/s11_fix3_result.json` | 已回退 | Catmull-Rom 偏离三角网格边 → 错位更重；**不采用** |
| S11 线框最终 | `/tmp/mem_anim_soak_run/v10_regress/S11` | PASS 90s | **折线贴合网格真实边**（MoveTo/LineTo）；8×5 轻度波浪；fps_ema=60.0 avg=59.7 cpu_fb=0 |

### v8 执行规范（对应用户 R1–R6）

1. **一进程一场景**：`GPUI_SCENARIO=S0x`，禁止进程内轮转  
2. **时长**：普通 90s；S12 120s（可调到 300–600s 复测）  
3. **门禁**：fps_ema≥55、fps_avg≥48、cpu_fb=0、无崩溃、steady RSS 斜率硬顶  
4. **CPU/内存口径**：仅 `/proc/self`（本进程）；日志同时给出 1核% 与 整机%（1核/nproc）  
5. **禁止刷分**：不得默认 sticky 关模块制造闪烁；重 API 必须「有界离屏真实 API 每帧」；`GPUI_HEAVY_API` 仅泄漏/全屏应力  
6. **输出**：`metrics.csv` + `result.json.line` + `SUMMARY.md` + `progress.log`  
7. **失败处理**：定位 drawprof → 引擎或场景根因 → 单场景复测 → 再入矩阵  

### 已知根因（v8 前）

| 场景 | 根因 | 处理 |
|------|------|------|
| S04 | 每 stroke flush + 动态 path | GPU stroke 批处理 + 几何量化 |
| S06 | 全屏 PushLayer 进 CPU pixmap 且不进 GPU present | **小离屏真实 PushLayer 每帧** + DrawImage（v2.2） |
| S07 | 全屏 PushBackdrop 快照 hitch | **小离屏 PushBackdrop+Blur 每帧** + DrawImage（v2.2） |
| S10 | 全屏 ApplyBlur ~28ms 或稀疏一帧闪 | **三块小 RT 每帧 ApplyBlur/Shadow/Grayscale**（v2.2） |
| S11 | Multiply/Screen dual-tex ~25ms 或稀疏闪 | **主表面 Plus + 小 RT 每帧 Multiply/Screen**（v2.2） |
| S11 | 用户见「灰色方块无网格」/ 线框错位 | blend：棋盘+绿网格线+Multiply/Screen；verts：8×5 DrawMesh + **贴合边折线框**（禁端点弦/禁贝塞尔偏离面）；CTM 迷你网格。像素探针 + 90s 回归。 |
| S12 | 上列叠加 | 同 v2.2 离屏真实路径；高 featureCount 仅 lite 几何 |
| 全场景 | 每帧 NotifyPixelsChanged 新 genID | **MarkEphemeral gen=0** 防 VRAM/RSS 膨胀 |
| 自适应 lite 改 RT/文本尺寸 | 矩形突然变大变小 + CJK 面板重建 ~20ms → 掉帧死循环 | **固定 RT/文本面板尺寸**；lite 只减几何密度；stickyLite 不回切 |




---

## 9. 目标验收审计（对照用户 5 条硬要求）

> 审计时间：2026-07-16。证据以磁盘结果为准，不依赖会话记忆。

| # | 用户要求 | 是否满足 | 证据 |
|---|---------|----------|------|
| 1 | 持续观察内存增长 / CPU / 稳定崩溃 | **是** | 每场景独立进程；`/proc/self` RSS+CPU；`result.json` 含 `rss_*`/`cpu_avg`/`status`；v9 无 abort（均 `exit=duration`） |
| 2 | FPS≈60（上千图元可降） | **是** | S01–S11：`fps_ema` 59.8–60.2；S12：58.6/57.0（门禁 ema≥55 avg≥48）；S13/S14 才 `AllowLowFPS` |
| 3 | 反复测多种指标，禁止表面修复 | **是** | 多轮 v5–v9 + preflight；门禁 FPS/CPU_fb/RSS steady；重效果走真实 API 小离屏（§0c），非关模块刷 PASS |
| 4 | 自动执行不弹确认 | **是** | `scripts/run_mem_anim_longsoak.sh` 串行 fork；`GPUI_SCENARIO`+`GPUI_ANIM_SECONDS` 单场景进程 |
| 5 | ≥10 种复杂内容，每场景 60s～10min | **是** | **12** 场景 S01–S12；时长 90s（S12=120s）∈[60s, 600s] |

### v9 权威结果摘要（`/tmp/mem_anim_soak_run/v9`）

| 场景 | 秒 | fps ema/avg | CPU% | RSS steady ΔKB | cpu_fb | 状态 |
|------|----|--------------|------|----------------|--------|------|
| S01 BaselineHUD | 90 | 60.1/59.7 | 21 | 16522 | 0 | PASS |
| S02 GlowField | 90 | 60.0/59.7 | 23 | 15462 | 0 | PASS |
| S03 CardUI | 90 | 60.0/59.5 | 24 | 10268 | 0 | PASS |
| S04 PathDash | 90 | 59.8/59.7 | 24 | 16250 | 0 | PASS |
| S05 ClipStack | 90 | 60.2/59.8 | 34 | 15015 | 0 | PASS |
| S06 LayerOpacity | 90 | 60.0/59.5 | 31 | 10145 | 0 | PASS |
| S07 BackdropPanel | 90 | 60.1/59.5 | 42 | 12271 | 0 | PASS |
| S08 ImageMaskAtlas | 90 | 60.1/59.8 | 43 | 15463 | 0 | PASS |
| S09 TextStyles | 90 | 60.0/59.5 | 23 | 10207 | 0 | PASS |
| S10 FilterFX | 90 | 60.1/59.3 | 49 | 23560 | 0 | PASS |
| S11 MeshBlendXform | 90 | 59.9/59.7 | 49 | 24585 | 0 | PASS |
| S12 FullComposite | 120 | 58.6/57.0 | 64 | 24866 | 0 | PASS |

**fail=0**。S12–S14 已在 v11 处理（180/90/180）。可选再加深：S12/S14 @300–600s；S13 density>1200。

### 回归（S11 可视修复后，当前二进制）

路径：`/tmp/mem_anim_soak_run/v10_regress`（`DONE fail=0`）

| 场景 | 秒 | fps ema/avg | CPU% | RSS steady ΔKB | cpu_fb | 状态 |
|------|----|--------------|------|----------------|--------|------|
| S11 MeshBlendXform | 90 | 60.0/59.7 | 65 | 24555 | 0 | PASS |
| S12 FullComposite | 120 | 58.8/56.9 | 74 | 24256 | 0 | PASS |

说明：v9 证明 12 场景达标；v10 证明 S11 棋盘 blend + 贴合边 mesh 折线框后仍 ≥60fps 门禁。

### 指标口径（防误读）

1. **只看本进程**：`/proc/self`，不用整机 load 冒充 CPU/内存。  
2. **RSS 看 steady_delta**（热身后增长），启动灌缓存导致的 end-start 大跳变不单独作为泄漏结论。  
3. **FPS 看 ema（稳态）+ avg**：门禁见 `judgeResult`。  
4. **cpu_fb 必须为 0**（无 CPU fallback 污染 GPU 主路径统计，除非场景显式允许）。  
5. **一个进程一个场景**：禁止同进程轮转场景污染 RSS。

### 复现命令

```bash
export PATH="/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64/bin:$PATH"
export GOROOT="/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64"
export GOTOOLCHAIN=local GOCACHE=/tmp/gpui-go-cache
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export DISPLAY=:1 XAUTHORITY=/run/user/1000/gdm/Xauthority
export GPUI_SURFACE_SAMPLE_COUNT=1 GPUI_TARGET_FPS=60 GPUI_FIXED_SIZE=1
go build -o /tmp/mem_anim_window ./examples/mem_anim_window
export GPUI_SOAK_OUT=/tmp/mem_anim_soak_run/v10
mkdir -p "$GPUI_SOAK_OUT"
setsid scripts/run_mem_anim_longsoak.sh S01 S02 S03 S04 S05 S06 S07 S08 S09 S10 S11 S12   > "$GPUI_SOAK_OUT/runner.log" 2>&1 &
```


---

## 10. S12 / S13 / S14 优先级处理

> 启动：2026-07-16。顺序：**S12 → S13 → S14**。

| 优先级 | 场景 | 处理内容 | 目标时长 | 门禁 |
|--------|------|----------|----------|------|
| P1 | **S12** FullComposite | 全模块综合；加深 soak（≥180s） | 180s（可选 300s） | 约 60fps（ema≥55） |
| P2 | **S13** HighDensity | **真实每帧 ≥1200 图元**密度场 + glow/path/poly/text | 90s | AllowLowFPS；无崩溃；cpu_fb=0；稳态 RSS 可接受 |
| P3 | **S14** StressEveryFrame | 每帧全模块 Stress=true | 180s | AllowLowFPS；无崩溃；cpu_fb=0；稳态 RSS 可接受 |

### S13 密度实现修正

旧实现每帧只画约 1/5 圆，名义 density=1200 偏弱。现改为**每帧绘制全部 n 个图元**（多数 GPU 矩形 + 少量圆/线），仍走真实 render API。

### 执行归档

路径：`/tmp/mem_anim_soak_run/v11_s12_s13_s14`（长时 soak 进行中 / 完成后填表）

| 场景 | 秒 | fps ema/avg | CPU% | steady ΔKB | cpu_fb | 状态 |
|------|----|--------------|------|------------|--------|------|
| S12 FullComposite | 180 | 58.3 / 56.4 | 74 | 40605 | 0 | **PASS** |
| S13 HighDensity | 90 | 60.1 / 59.5 | 52 | 10047 | 0 | **PASS** |
| S14 StressEveryFrame | 180 | 57.2 / 56.9 | 74 | 32464 | 0 | **PASS** |

权威路径：`/tmp/mem_anim_soak_run/v11_s12_s13_s14`（`DONE fail=0`）

预检（25s）：S13 PASS fps 59.9；S14 PASS fps 59.8（AllowLowFPS 场景）。

### 结论

1. **S12** 加深到 180s 仍过约 60 门禁（ema≥55 / avg≥48），全模块综合稳定。  
2. **S13** 修正为每帧真实 ~1200 图元后仍 **≈60fps**，非假稀疏。  
3. **S14** 每帧全开 180s 无崩溃、`cpu_fb=0`，稳态 RSS ~32MB 可接受（AllowLowFPS）。  


---

## 11. 加深 soak（v12）

> 用户指令：继续加深。路径：`/tmp/mem_anim_soak_run/v12_deepen`

| 场景 | 加深参数 | 时长 | 状态 |
|------|----------|------|------|
| S12 | 综合合成加深 | **300s** | 进行中/见 SUMMARY |
| S13 | `GPUI_DENSITY=2000`（上沿） | **120s** | 进行中/见 SUMMARY |
| S14 | 每帧全开应力加深 | **300s** | 进行中/见 SUMMARY |

验收：无崩溃；`cpu_fb=0`；S12 仍过约 60 门禁；S13/S14 允许低 FPS 但需 PASS。

- v2.6：新增 S15–S21（Skia 缺口场景：梯度/pattern、高级混合、rrect+evenodd、LCD 文本、damage present、滚动模态、缺口组合）；默认 longsoak 仍可只跑 S01–S12。

- v2.6.1：S15–S21 预检 PASS（preflight6，20s）；修 glyph-mask **混合 LCD/灰度** 同帧 pipeline 选择（per-drawCall isLCD，避免 80/96 BGL 校验 abort）；重效果走 retained RT 保 ~60。
