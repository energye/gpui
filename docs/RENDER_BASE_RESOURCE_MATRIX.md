# 底座资源场景矩阵（RENDER_BASE_RESOURCE_MATRIX）

日期：2026-10-27 ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui` ｜ 分支：`feat/框架代码整理`
状态：**实施中——P2 已冻结；P3 已收口（2026-10-26：尾账 Golden 13→0、maskops 活窗红已收、Flutter/Godot 对照尺已立；2026-10-27：gpui 同语义对照臂三跑 + 四张 Golden 重出两连）**（E1 已豁免、机器证据在 §5.6；E2 已定案、§5.7；两处既有红已定案；对照尺实测 §5.5）；P4 未开工。范围：支撑三栋楼（任意图形渲染 / UI 组件库 / 2.5D 引擎）的**共用底座能力**的资源场景真窗覆盖，指标对齐**工业级产品同机实测**（Flutter/Godot 对照尺）。上层业务场景（组件交互全集、引擎玩法全集、视频业务）归各自域真源，本矩阵只管底座。

## 0. 一句话与做成算完口径

把底座能力封闭成 **17 行场景矩阵**（能力 × 触发模式 × 资源维度），每行做到：

> **真窗 + 资源指标 JSON（CPU/RSS/显存/fps/p95/hitch）+ 同机工业级对照数字 + 比值**——四项齐全、逐行可审计、无一行无数据或无书面豁免。

指标分两段：**健康线**（现有自定阈值，防退化）+ **对齐线**（新增，≤ 同机工业级对照 × 系数，系数已定稿：CPU 1.5× / RSS 2.0×，2026-10-28 拍板，依据与豁免见 §5.9）。"通用/对齐"的宣布权在这张矩阵的量门数字上。

## 1. 决策记录（2026-10-25，用户已定）

1. **Godot 二进制**：允许下载官方预编译或从本机源码编译（`/home/yanghy/app/projects/gogpu/godot`；GAP_PLAN 的"只读不编译"指不改其源码做参考，编译出可执行体做对照尺不违反该约束，编译产物只用于测量）；
2. **Flutter**：允许本机构建（SDK 已在 `/home/yanghy/app/projects/gogpu/flutter`，`bin/flutter` 已验存在）或取现成产物；
3. **对照系数**：**已定稿（2026-10-28 用户拍板；P4 实测分布依据见 §5.9）**——CPU **1.5×**、RSS **2.0×** + 书面豁免条款（idle 行绝对差、Flutter 臂未满 60fps 行带 fps 注、text 族 RSS）。占位初值 CPU 1.2×/RSS 1.5× 作废（1.2× 过最紧恶劣样本 Godot 0.94× 后余量仅 ~13pp，扛不住 ±2pp 中位波动）。

## 2. 场景矩阵（17 行封闭集，本表冻结；新增场景走 §7 增量登记）

| # | 底座场景 | 触发模式 | 真窗现状 | 资源指标现状 | 对照场景（尺） | 缺口动作 |
|---|---|---|---|---|---|---|
| 1 | 空闲（无脏零帧） | 开窗静止 60s | **`ui_base_idle`** | ✅ 两连 gate + 60s manual：presents=1、warmup 后增量=0、CPU 0.6–0.8%、golden 0（873600px） | Flutter 静止页 | ✅ 已对照：Flutter text 页首帧后 idle 0.02%（§5.5） |
| 2 | 常驻最小动画（心跳税地板） | 单 64px 旋转 60fps | **spin_min** | ✅ CPU 有账（现态 14.59% / 终态 3.4~3.5%） | Flutter 同尺寸旋转图标 | ✅ 已对照：Flutter 25.41% / spin_min 15.92%，1.60×（§5.5）；同语义 parity **14.16%**，**1.79×**（§5.5.3） |
| 3 | 滚动（retained 主场景） | 列表连续滚动 | C3/C9/R7/R7b | ✅ R7 60s close-run 两连：fps 59.9、p95 19.8ms、hitch 1.0–2.0/min、CPU 18.27%（ui 7.93 + raster 10.33）、RSS 斜率 3458/4435KB/min（valid）、damage 9.25%、bind 14/1200、scroll 重录 14、golden 0；C3 31.98% 已有 | Flutter ListView 同规模 | ✅ 已对照：Flutter 71.38%，对 C3 2.23× / 对 R7 3.91×（§5.5）；同语义 parity **22.99%**，**3.10×**（§5.5.3） |
| 4 | 全幅重建上限 | full_paint 全窗每帧变 | R0 | ✅ | Flutter 全幅动画页 | ✅ 已对照（§5.8.5）：R0 26.92%/59.94fps 对 Flutter 73.91%/53.2fps → **2.75×**（双侧 full 重绘策略同款；内容不同源、R0 侧静态网格+HUD 便宜，读数加内容注） |
| 5 | 异步资源→局部脏 | 解码完成逐格点亮 | R10/C3 | ✅ | Flutter 异步图片列表 | ✅ Flutter 臂 70.35%/58.4fps/184.8MB（§5.8.4，解码突发 20s 预热吸收，稳态 hitch 高 245–681/60s 为逐格热点）；gpui R10 同行资源账挂 §6 遗留 |
| 6 | 缓存命中/失效 | 边界/纹理命中与失效 | R3/R3b/R9/C9 | ✅ | Flutter RepaintBoundary 开关 | ✅ Flutter 臂 68.77%/55.0fps/182.7MB（§5.8.4）；gpui R3 族同行资源账挂 §6 遗留 |
| 7 | resize/DPR 失效波 | 改尺寸/DPR | R11/C7 | ✅ | Flutter 窗口 resize | ✅ Flutter 臂静页双波：两跳几何铁证（t3/t11 尺寸生效）、页后静止 0.15%（§5.8.4）；波级细采样挂 §6 遗留 |
| 8 | 浮层/覆盖带脏 | overlay 开关 | R8 | ✅ | Flutter Overlay/Dialog | ✅ Flutter 臂 67.47%/49.1fps/185.4MB（§5.8.4，**未满 60**）；gpui R8 同行资源账挂 §6 遗留 |
| 9 | 后台/前台节流 | 遮挡/最小化 | R17 | ✅ 两连 gate + Golden + 资源账：最小化后 fg=1308–1310、bg=1、presents=1310–1312/30.1s、pixel 3/3、golden 0；FG 双快照 live（diff 0.92% / content 13.81%） | Flutter 同内容最小化 | ✅ 已对照（§5.8.4）：Flutter 最小化后 t≥24 fps=0 帧休眠（对 R17 bg=1 同港语义）、hidden 铁证在案；30s 聚合 CPU 36.19%（FG22+BG8 混窗），FG 段 53–61fps |
| 10 | 多窗共享上限 | 1/2/4/8 窗同动画 | **`ui_base_multiwin_cap`** | ✅ 1/2/4/8 曲线（2026-10-27 XWayland 重测）：60fps 上限 **4 窗**、steady60 上限 2 窗；8 窗 39.6fps、总呈现 ≈317 帧/秒；golden 0.0000%、像素 16/16（§5.8；桌面 Xorg 期为 31fps/~250 帧/秒，饱和点随会话环境漂移） | Flutter 多实例（N 进程×1 窗，480×320） | ✅ 已对照（§5.8）：三跑中位 CPU 27.2/62.2/131.3/176.2% vs gpui 22.4/43.8/81.3/111.0%，CPU 比值 1.21–1.61×；RSS 窗均摊 163MB/进程 vs 八窗常驻 234MB（5.6×）；8 窗总吞吐两侧同为机器饱和 ~316–328fps |
| 11 | 长跑稳态（泄漏） | 300s soak | R15/C10 | ✅ 最完整 | Flutter 300s 同内容 | ✅ 已对照（§5.8.4）：Flutter list 300s×3 CPU 中位 72.23%、RSS 平稳 196.5MB（稳态斜率 535.5KB/min valid，线 30000）、fps 55.3；gpui R15/C10 同窗资源账挂 §6 遗留成比值 |
| 12 | 文本密集（整形+图集） | 密集多脚本文本页 | m5/文本族 | ✅ RSS 暴涨已修复已归档（2026-10-27，§5.5 末段/§9）：根因=缺字回退按 `(rune,size)` 缓存致 75 次整份字体加载（只 4 文件、0.8s 集中），存活堆 805MB；`render/text/source_cache` 按文件进程级共享（LRU 32 条/256MB + singleflight）后 60s：加载 **75→10**、存活堆 **144MB**、RSS **464MB 稳态**（原 1.67–1.75GB）、CPU 23.0%、fallback 0；m5 golden 独显重出两连 0.0000%（821184px）+ 像素 11/11，藏文随共享 face 合簇修正 | Flutter 密集文本页（静态 idle） | ✅ idle 对照 0.02%；同语义 parity text 0.42%/RSS 628.5MB（静态 idle，§5.5.3）；m5 CPU 23.0% 含 HUD 60fps tick 税不直比整形吞吐 |
| 13 | 大量图元/矢量密集上限（2.5D 底座，**双模式**） | ①同屏图元阶梯 1k/5k/10k（atlas 精灵）；②矢量路径密集（设计工具类，复杂路径重分格成本） | **`ui_base_batch_cap`**（engine 精灵/粒子窗、P1 矢量密度测试为前身） | ✅ 图元 1k/5k/10k 全 60fps（CPU 22.9/46.3/80.7%）；矢量 100@60fps、300→38fps、600→24fps；两连 gate + 200s manual | Godot 2d demo（bullet_shower 类）/ Flutter CustomPaint | ✅ Godot 默认 500 发 19.1%/60fps；同语义 gpui parity **20.33%/59.9fps**（0.94× CPU，§5.5.3）；1k/5k/10k 容量探针见 §5.5；Flutter 臂 cp=100 动画贝塞尔 **69.90%/58.5fps/172.4MB**（§5.8.4），对工具类语义、不做 1:1 梯对位 |
| 14 | 视频/纹理流 | 持续喂帧 | video C4 | ❌ 解码吞吐（V-5 挂视频线） | ffplay/Flutter video | 挂视频线 |
| 15 | 呈现策略对照 | 同场景 full vs retained | R0/C2/R4 | ✅ 已对照（§5.8.5）：R0 26.92% vs C2 集成壳 25.22%（retained 省 ~6.3% CPU、damage_sum 0.187≪0.35 线）、R4 局部地板 24.02%；全部 59.94–59.97fps 满拍；严格逐字同内容可切策略窗挂 §6 遗留 | Godot 视口/Flutter | ✅ Flutter 臂同触发语义见 §5.8.4 各行 |
| 16 | 首帧/恢复（t2fp、遮挡恢复） | 冷启/遮挡回来 | R16/R17 | ✅ | Flutter 首帧 | ✅ Flutter 臂 map_ms（进程启动→X 客户区 mapped，三跑中位）270–292ms（§5.8.4：min 276.6 / grid 277.4 / resize 280.6 / boundary 281.6 / cp 284.8 / full 291.8 / overlay 275.3）；遮挡恢复 Flutter 臂协议缺口挂 §6 遗留 |
| 17 | 显存压力/设备恢复（资源纪律验收） | 多窗 VRAM 预算压力、`GPUI_FAULT_OOM` 注入 | 故障注入缝在（`ui/embedder/fault_oom.go`）、预算拒增计数在（`ui/scheduler/metrics.BudgetRefusals` + `ui/embedder/pipeline_app` cacheBudgetRefusals）、`hud_hal.vram_ledger` 账本在（`gpu/hal/vram_ledger.go` + 单测） | ❌ 无专项资源场景窗（恢复正确性有注入测试、压力资源账无） | Godot 压力场景 | **立账计划已登记（2026-10-27，窗未立）**：①多窗 VRAM 预算阶梯开窗（2/4/8 窗纹理负载递增）取 `BudgetRefusals` 拒增曲线；②`GPUI_FAULT_OOM` 注入后断言 `hal.VramLiveBytes()` 恢复回落至注前账；③Godot 臂后置。窗建成前不标绿 |

**P2 冻结结论（2026-10-25）**：17 行逐行核对完毕——
- ✅ 真窗已有 + 资源指标已有（不需新窗）：行 2/4/5/6/7/8/11/16（8 行）——只需 P4 补对照数字；
- ✅ 真窗已有 + 资源指标需立账（窗在但没有专项 CPU/RSS JSON）：行 3/12/15（3 行）——P4 立账；
- ❌ 需新窗：行 1（idle）/10（multiwin_cap）/13（batch_cap）（3 行）→ **P3 已建（§5.1–5.3）**；
- ❌ 跨线依赖：行 14（视频吞吐 V-5）/17（显存压力资源账）（2 行）→ **挂视频线与资源纪律域**；
- ✅ 证据尾账：13 扇仅像素补 Golden 已全部补齐（两连 `-auto-only` 全 PASS、golden 0.0000%，§5.4）；E1（14 扇）已豁免（§5.6）；E2（2 扇）已定案（jumprope 冻结 Golden、spin_min 豁免，§5.7）；
- ✅ Flutter 对照尺：stable 3.47.8 已立（Impeller OpenGLESSDF 在 940MX 实际出画，§5.5）；Godot 4.5.1 `bullet_shower` 已立（§5.5）；gpui 同语义对照臂 2026-10-27 已立（四场景三跑 + 四张 Golden 两连，§5.5.3）——P3 收口。

尾账（不属于矩阵行，属证据域）：13 扇仅像素补 Golden（`ui_ant_verify`、`ui_render_particle`、`ui_textinput_ime`、`ui_wr_c7_resize_dpr`、`ui_wr_c8_hit_overlay_xf`、`ui_wr_ime_r2/r3/r4/r5_control`、`ui_wr_r17_bg_throttle`、`ui_wr_r21_shell`、`ui_wr_r4b_multidamage`、`ui_wr_r7_virtlist`）**已清零（13→0）**；E1（14 扇）已豁免（§5.6）；E2（2 扇）已定案（§5.7）。

## 3. 工业级对照尺（三层）

### 3.1 Flutter 尺（主尺，stable 3.47.8 已立）

- SDK：`/home/yanghy/app/projects/gogpu/flutter` stable **3.47.8**（engine `cf636b1dfb`，Dart 3.13.7）。2026-10-26 在 NVIDIA 940MX / X11 实际 release 出画，Impeller 走 `OpenGLESSDF`；截图非黑。
- 构建环境：本机没有 clang，用 `/tmp/opencode/flutter_toolchain/{clang,clang++}` 薄包装转调 gcc/g++；`flutter doctor -v` 退出 0，`flutter build linux --release` 成功。master 3.50 的黑屏结论不适用于 stable 3.47.8；未来升 SDK 必须重跑出画验证。
- 对照窗三个，对位本矩阵行 2/3/12：**旋转图标**（480×320，同 spin_min 64px/60fps 语义）、**滚动列表**（1200×800，1200 项变高行、44/92/60/76 行高、四类行型、24 图、Steady/Spike/Recover 相位、列表宽 620，对齐 C3）、**密集文本页**（1200×800，复用 m5 的 multiscript/emoji/richtext 语料，首帧后静止）。
- 代码：`tools/ruler_flutter/`（Dart UI、linux runner 客户区、`scripts/measure.py`、Makefile、README），数据在 `assets/`。
- 实测：§5.5；三跑取中位，原产物 `/tmp/opencode/ruler_runs/final_*`（可重跑）。文本臂首帧后无 ticker，只作静态 idle 对照，不能拿来宣称文本整形比 m5 快。

### 3.2 Godot 尺（第二尺，4.5.1-stable 已立）

- demo：上游 `godotengine/godot-demo-projects` 的 `2d/bullet_shower`，本机 checkout `a3b5c113112f77291d5f3d1360f33a882fdc52f7`；上游目录不改，`tools/ruler_godot/setup.sh` 复制到 `$GODOT_WORK/bullet_shower` 后打补丁。
- 二进制：官方 `Godot_v4.5.1-stable_linux.x86_64`；sha256 `db07cae7de644278a1884d4552bdf2bca3f5d30131b18faf3a0c4d730080b199`；engine `f62fdbde1`；`gl_compatibility` / OpenGL 3.3 NVIDIA 580.178.04。`project.godot` 标 `config/features=4.7`，但 4.5.1 实测能 import 并出画；升级版本需重验。
- 窗口 1200×800；默认 500 发（上游语义：每发一个 PhysicsServer2D body + 碰撞圆 + 逐帧 `draw_texture`）。`GODOT_BULLETS` 只作容量探针，含逐对象物理开销，不能当纯图元 parity 去和 gpui `ui_base_batch_cap` 1:1 比较。
- 代码：`tools/ruler_godot/`（setup/patch/fps_probe/run/measure/README）；实测 §5.5。

### 3.3 glxgears 尺（地板尺，已在用）

- 本轮 60s 实测 **2.74%**（300×300 客户区，RSS 50.1→58.1MB，59.93fps）；历史 3.30%，同机范围 2.3–3.3%。齿轮画在黑底上，不能拿黑底占比判全黑。
- 任何 60fps 呈现的 CPU 下限参照；只作地板，不与业务场景比值。

### 3.4 公开参考数字锚（第二重 sanity，非验收线）

同机两臂可能同错（系统性偏差），所以对齐线之外引入公开基准作旁证：Flutter 官方性能文档的典型区间（如简单动画页 CPU 个位数百分比、复杂滚动 10-20%）、Godot 2D demo 公开表现。**只作 sanity 参考**——验收线仍以同机对照为准，公开数字入矩阵文档备注列。

### 3.5 测量口径（三尺统一，公平性硬约束）

- 同机（NVIDIA 940MX，580.178.04；参考臂 2026-10-26 在 X11 :0，gpui parity 2026-10-27 在 X11 :1，同机同驱动，display 号不影响 `/proc` 口径）、同窗客户区尺寸（Flutter/Godot/gpui parity 1200×800，spin 480×320）、同帧率目标（60）、同动画语义（内容变化量对齐；差异在 §5.5 注明）。
- 统一采数器 `tools/ruler_flutter/scripts/measure.py`：Popen 起真实应用 → 等 `xwininfo` 窗口 → 5s/20s 预热 → 60s 从 `/proc/<pid>/stat` 采进程 CPU（utime+stime，含内核态，**不含 Xorg/合成器侧**）、`/proc/<pid>/status` VmRSS → 通过 `xwininfo` 客户区 + `ffmpeg x11grab` 截当前窗 → 汇总 fps/p95/hitch 与截图统计。
- 对照场景三跑取中位；glxgears 地板单跑 60s。Flutter spin/text 预热 5s，list 预热 20s（让 Flutter 图像解码/缓存进稳态）；Godot 5s；glxgears 5s。原图与 `report.json` 落指定 `--out`，不提交仓库。
- FPS 来源：Flutter 由页面帧回调自己打 `RULER_METRIC` 行；Godot 由 `fps_probe.gd` 每 2s 打 `GODOT_FPS=... t=...`，采数器按 `t>=warmup` 丢启动样本；gpui 的 fps/p95 取该窗自己的 60s 指标 JSON，`measure.py` 对 gpui 只做 CPU/RSS/截图交叉复核。`hitch` 定义见各自输出。

## 4. 指标规范

1. **健康线（保留现状，防退化）**：fps_interval≥55、interval_p95≤22ms、hitch≤5/min、RSS 稳态斜率 ≤30000KB/min（valid=true）、CPU≤85%。
2. **对齐线（新增）**：行指标 ≤ 同机对照 × 系数。`wrgate` 报文追加字段：`ruler_name`、`ruler_cpu_pct`、`ruler_rss_mb`、`ruler_ratio_cpu`、`ruler_ratio_rss`——**跑过尺才填，没跑如实 null（禁假绿）**。系数已定稿（§1.3：**CPU 1.5× / RSS 2.0×**，豁免条款见 §5.9；判红 = `gpui值 > 尺臂值 × 系数`，三跑中位对三跑中位）。**2026-10-27 D 步已落地**：`wrgate.RulerInput`（尺臂 id/测得 CPU%/rss_end_mb）+ `BuildInput.Ruler` + `wrgate.RulerFromEnv()`（读 `GPUI_RULER_NAME/CPU_PCT/RSS_MB`，P4 跑批流程三跑取中位后导出）；五键列入 `RequiredSchemaKeys` 硬检查，比值=窗值/尺值（分子分母同口径：`/proc` utime+stime 占单核百分比、run 末 RSS），任一侧没量到该比值保持 null，半截环境变量按「没跑」处理并在 stderr 报因。窗口 JSON 逐一嵌尺值在 P4-A 逐行落（未实测的行比值仍是 null，不得借字段装绿）。
3. 现有各域真源 §5 阈值矩阵不动；本矩阵只在资源轴加对齐线。

## 5. 新建窗规格（3 扇，模式 2：gate + manual 二合一 + 三证据）

### 5.1 `ui_base_idle`（矩阵行 1）

- 真窗：`examples/ui_base_idle`，1200×800 静态页；开窗画完一帧后不再改任何节点。
- 量法：warm-up 只允许 1 帧引导呈现；之后 60s 断 `presents` 增量、进程 CPU（`/proc` utime+stime）、RSS 稳态斜率、`vsync_source`。三证据 = 4 条逻辑探针 + 5 条像素探针 + Golden 1200×728 逐位。
- 引擎侧：warm-up 结束时只有 `sched.Tickers().MayWantFrames()` 为真才 `ScheduleFrame()`（`ui/embedder/pipeline_app.go`）；静态窗不再以 60Hz 重复呈现同一帧。
- 2026-10-25 两连 gate（最终代码）：`presents=1`、`after_warmup=0`、CPU **0.807%**，RSS 斜率 **3400.9 / 905.3 KB/min**（valid），peak **245.6 / 245.3 MB**，logic **4/4**、pixel **5/5**、golden **0.0000%**（873600px）。
- manual 60s：`presents=1`、`after_warmup=0`、CPU **0.6%**、logic **4/4**。
- 复现：`RUN_SECONDS=60 go run ./examples/ui_base_idle -auto-only`；人工档 `go run ./examples/ui_base_idle -manual-seconds 60`。

### 5.2 `ui_base_batch_cap`（矩阵行 13）

- 真窗：`examples/ui_base_batch_cap`，1200×800；两条阶梯各 6 阶、每阶 10s：
  - prims：1k / 5k / 10k atlas 四边形（`DrawAtlasEx`）；
  - vector：100 / 300 / 600 条弯曲路径（每条约 24 段三次贝塞尔，`FillPath` 重分格成本）。
- 量法：gate 先跑 **2 轮不计入测量的 prewarm 阶梯**，再跑第 3 轮测量。原因：60s 窗口的 RSS 斜率原先是 GPU/heap 高水位爬坡（预热前 ~45MB/min），一轮预热仍让 25% 稳态窗口落在爬坡段（实测 24–29MB/min，贴线易抖）；两轮后测量周期稳定在 **6–8MB/min**。gate 时长 200s（prewarm 120s + 6×10s + 收尾）。
- 两连 gate 实测（prewarm 之后的测量周期）：

| 阶 | fps | p95 | CPU | 结论 |
|---|---:|---:|---:|---|
| prims 1k | 59.9–60.0 | 17.4ms | 22.9–23.2% | 60fps |
| prims 5k | 59.8–60.0 | 18.4–18.8ms | 46.3–46.6% | 60fps |
| prims 10k | 59.8–59.9 | 19.2–19.5ms | 80.7–80.9% | 60fps |
| vector 100 | 59.7 | 18.1–18.6ms | 58.6–60.1% | 60fps |
| vector 300 | 37.7–37.8 | 29.2–29.3ms | 74.5–74.8% | 过载（如实记，不判绿） |
| vector 600 | 24.0–24.3 | 43.6–44.4ms | 82.7–83.0% | 过载（如实记，不判绿） |

- 容量结论：**60fps 最大图元数 N\*=10000**；**60fps 最大矢量路径数 = 100**。过载阶是故意的上限探针，容量判定只看 `steps[].pass_60fps`；聚合 hitch 字段跨过载阶，JSON 标 `hitch_scope`，不拿它当整窗健康值。
- 资源账（两连，post-race-fix）：RSS 斜率 **6735.0 / 8015.1 KB/min**（valid），peak **453.5 / 447.2 MB**，进程 CPU **61.3 / 62.3%**；golden **0.0000%** × 2（各 883200px，mask 1200×736），pixel **4/4**。
- manual 200s：6 阶全走完，CPU **57.7%**，`max_prims=10000`、`max_vector=100`，golden 0（人工档不重复像素断言）。
- 复现：`RUN_SECONDS=200 go run ./examples/ui_base_batch_cap -auto-only`；人工档 `go run ./examples/ui_base_batch_cap -manual-seconds 200`。

### 5.3 `ui_base_multiwin_cap`（矩阵行 10）

- 真窗：`examples/ui_base_multiwin_cap`，8 扇 480×320 窗一次性预开；每扇独立 X11 display 连接 + 独立 `Run` 循环，同一进程。
- 驱动：共享 `activeN` 原子量；每扇 ticker 实现 `scheduler.FrameWanter`，非激活时 `WantsFrame=false` → 调度器阻塞在事件等待、零帧（复用行 1 的 idle 契约）；激活时 controller 对目标窗 `ScheduleFrame()+Host().WakeUp()`。
- 量法：8 窗全激活 prewarm 10s（把 GPU pool/字体/层缓存拉到高水位），再按 **1 → 2 → 4 → 8** 分四段，每段 1s settle + 12s 测量；gate 75s。每段记各窗 presents 增量、p95、进程 CPU/RSS；8 窗末尾全部冻结到 phase 0，各写一张 Golden，逐窗像素断言。
- 两连 gate 实测（post-race-fix）：

| 激活窗 | 每窗 fps | p95max | 进程 CPU | CPU/窗 | CPU ms/帧 | RSS |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 59.8 | 17.2–17.3ms | 19.3% | 19.3% | 3.23ms | 303→304MB |
| 2 | 59.4–59.7 | 21.1–22.3ms | 33.7–34.7% | 16.8–17.4% | 2.82–2.92ms | 304→306MB |
| 4 | 59.1 | 28.4–28.7ms | 62.6–63.3% | 15.6–15.8% | 2.65–2.68ms | 306→308MB |
| 8 | 31.2–31.4 | 33.7–34.8ms | 71.5–71.7% | 8.9–9.0% | 2.86–2.87ms | 308MB |

- 容量结论：**平均 60fps 上限 = 4 窗**；**稳态 60fps（再加 p95≤22ms）在 1–2 窗之间抖动**（两连 gate 一次 2、一次 1；n=2 的 p95 实测 21.1 / 22.3ms，贴线）。8 窗时每窗 ~31fps，总计 **≈250 presents/s**（249.5 / 251.2），进程 CPU 仅 71.7%（<85%），单帧 CPU 成本稳定在 2.65–3.23ms——瓶颈是共享呈现路径的串行吞吐，不是 CPU 打满；按 §7.5 记为观察项，不判缺陷。
- 资源账（两连）：RSS 斜率 **10824.1 / 10285.5 KB/min**（valid），peak **331.6 / 332.4 MB**；CPU 全窗均值 50.4 / 50.6%（含 10s 8 窗 prewarm）；golden **8 张全 0.0000%**（合计 1075200px，单窗 mask 480×280，顶部指标带排除），pixel **16/16**，CPU fallback **0**，missed vsync **0**。
- manual 100s：四段全走完，8 窗 31.3fps、CPU 71.6%、RSS 斜率 4.5MB/min、平均上限 4、稳态 1。
- `-race`：多窗并发建层最初在 `ui/scene/layer.go` 的全局 `layerIDGen` 上暴露 **59 条同根数据竞争**（单窗从不触发）；已改为 `atomic.Uint64` 自增（保持唯一、单调、非零）。修复后 `CGO_ENABLED=1 go build -race` + `-manual-seconds 45`（覆盖 8→1、1→2 激活）**0 条竞争**；随后两连 gate 全 PASS。
- 复现：`RUN_SECONDS=75 go run ./examples/ui_base_multiwin_cap -auto-only`；人工档 `go run ./examples/ui_base_multiwin_cap -manual-seconds 100`。

### 5.4 尾账（2026-10-26：13 扇 Golden 清零）

13 扇仅像素补 Golden 的窗（清单 §2 尾账）已全部补齐「固定冻结 + 零容差 Golden + 像素/逻辑证据」，每扇 `-auto-only` **两连 PASS**；尾账 **13 → 0**。`ui_wr_ime_r1_editor` 更早已有同类证据（`9410d61f`），不在本 13 扇清单内。

| 窗 | 冻结 / 逻辑证据（两连实测） | pixel | golden |
|---|---|---|---|
| `ui_ant_verify` | 几何 AA 固定帧；presents=301（本机 MSAA=1x，msaa1/msaa4 基线均已播种） | —（逻辑+Golden） | 0.0000%（821184px） |
| `ui_render_particle` | t=1s 冻结粒子相位；599 presents、58.1–58.2fps、p95 19.2–19.4ms、CPU 25.0–25.7%（ui 10.7 + raster 14.3–15.0）、damage 79200、fallback 0 | 4/4 | 0.0000%（883200px） |
| `ui_wr_c7_resize_dpr` | 两波失效完整走完：wave1=1、wave2=19、dpr=1.0、presents=899/15.3s、skip=16731、rr=369 | 6/6 | 0.0000%（821184px） |
| `ui_wr_c8_hit_overlay_xf` | 脚本 11/11、main_dirty_base=6、excess=0、overlay 3 轮、presents=829–831/15.2s | 5/5 | 0.0000%（821184px） |
| `ui_wr_r17_bg_throttle` | 最小化后 fg=1308–1310、bg=1（presents 1310–1312/30.1s）；FG 双快照 live：diff 0.92%、content 13.81% | 3/3 | 0.0000%（821184px） |
| `ui_wr_r21_shell` | shell 滚动零重录（rr_scroll=0、skip=296、rr_total=1）、presents=300/5.3s | 6/6 | 0.0000%（883200px） |
| `ui_wr_r4b_multidamage` | damage_multi、ids_max=10、multi=891–892、dmg_avg=0.548、fps 59.9–60.0、skip 46534–46587 | 3/3 | 0.0000%（821184px） |
| `ui_wr_r7_virtlist` | 60s close-run：presents=3589–3591、bind 14/1200、scroll 重录 14、fps 59.9、p95 19.8ms、hitch 1.0–2.0/min、RSS 斜率 3458/4435KB/min（valid）、CPU 18.27%（ui 7.93 + raster 10.33）、damage 9.25%、vsync | —（逻辑+Golden） | 0.0000%（763584px） |
| `ui_textinput_ime` | 固定输入缓冲 `你好ba`、caret 固定、blink_ticks=0 | 5/5 | 0.0000%（270000px） |
| `ui_wr_ime_r2_textlayout` | 36×m 深排版/亲和/粘列/回退/省略号/Boxes/生成代数 9 探针全 true（LongBuild 1.771ms） | 4/4 | 0.0000%（821184px） |
| `ui_wr_ime_r3_channel` | Signal6/Storm/Filter/SetState/Delta/Surround4/Autofill/DeadKey/DualEngine 全 true（Storm=20、Surround=3999×4） | 5/5 | 0.0000%（821184px） |
| `ui_wr_ime_r4_selection` | t=2s 冻结 tick/选区相位与标签、caret 点亮、去闪烁；双击/三击/Shift/拖选/密码/只读/撤销/词删/锚点/原子替换全 true | 5/5 | 0.0000%（821184px） |
| `ui_wr_ime_r5_control` | t=2s 冻结控制态与标签、caret 点亮、去闪烁；5k 单行/组合滚动/粘列/省略号/占位/单行拒多行/多行放行/禁用全 true | 6/6 | 0.0000%（821184px） |

**引擎修复（本轮，通用层，不在示例绕）**：R4 补证据时首播暴露 `cpu_fallback_ops=1220`、`last_cpu_fallback: text:color-layout`——旧分类按 Unicode 区段把 `✓ ✗` 这类"文本外观的符号"判成彩色字形，送彩色图集后字体没有 COLR/CBDT 可采，走 CPU 兜底并丢字。改为按 Skia 口径"字体说算"：`render/text/color_font.go` 新增 `GlyphsHaveColor`（逐字形读 `ColorFont.GlyphType`），`ui/rendering/text_layout.go` 的 `buildShapedCarets` 用它闸住 `isColorText`（`IsColor: isColorText(seg) && GlyphsHaveColor(...)`）。回归 `TestColorRunOutlineDingbat_M5` PASS；R2–R5 fallback **1220 → 0**，✓/✗ 恢复可见。

**门语义对齐（真源 `ENGINE_TEXT_X11_IME_REQUIREMENT.md` §10.4.2/3/4/5 G）**：`measure_cache_hit` 的口径是"必采"而不是"≥1"——retained/composite-only 帧与已缓存排版不重新量，`PaintVisits`（末帧节点访问）在复合帧恒 0。六扇 IME 窗的像素/逻辑证据统一改为 `first_present_paint_count>0 && (measure_cache_hit+measure_cache_miss)>0`，同时撤掉 R9 专用的 `MinMeasureCacheHit:1`。auto-only 的 5–6s 有界档短于 15s RSS 预热、启动单次卡顿在短窗口会放大：仅 auto-only 放宽为 WARN（斜率必须 `!valid`；hitch 需 >2 次且 >2.5/min），≥10s 人工档维持硬门。

### 5.4.1 六扇 IME 真机人工验收（2026-10-26）

六扇 IME 窗逐扇开常驻人工档（X11 真窗，XTEST 真按键/真拖选，≥10s 硬门），从空窗开始走真实交互，边操作边采像素探针；全程帧率、hitch、CPU fallback、RSS 斜率同档采集。

| 窗 | 时长 / 帧率 | 交互实测（像素探针） | 资源 / 门禁 | 结论 |
|---|---|---|---|---|
| `ui_wr_ime_r1_editor` | —（修复后复验） | 光标 38px、选区 4056px（修复前两者皆 0） | 活窗像素复验 | PASS（修复后复验） |
| `ui_wr_ime_r2_textlayout` | 81.7s / 59.8fps | 深排版 9/9 探针真；光标 5px 呼吸（闪烁节奏）；选区 817px + 滚动盒 3169px | hitch 2（1.47/min）、fallback 0、RSS 斜率 7350KB/min（valid） | PASS |
| `ui_wr_ime_r3_channel` | 33.6s / 59.7fps | 渠道 9/9 探针真；光标 38px（6/12 帧亮灭）；选区 2253px | hitch 0、fallback 0、RSS 48862KB（WARN 软线，<90k 硬线） | PASS |
| `ui_wr_ime_r4_selection` | 106.4s / 59.8fps | 拖选 2621px、双击取词 1568px、跨行选区 4968px；密码框光标 48px、无选区高亮（引擎设计，见下） | 10/10 探针、hitch 0、fallback 0、RSS 5595KB | PASS |
| `ui_wr_ime_r5_control` | 51.0s / 59.8fps | 两段选区 1956px + 2780px；单行/多行/省略号控制态按预期 | 9/9 探针、hitch 0、fallback 0、RSS 46895KB（WARN 软线） | PASS |
| `ui_textinput_ime` | —（常驻人工） | 逐字输入生效；光标 24px（x=75）50 帧中亮 32 帧；该示例本身不画选区 | 交互路径正常 | PASS（仅输入/光标） |

**观察项（登记，不判缺陷）**：
- **密码框不画选区高亮**：`ui/textinput/box.go:934` 的 `syncHighlight` 对 `IsPassword()` 直接返回——选区高亮会泄露口令长度，按设计抑制；光标照常显示。
- **`ui_textinput_ime` 示例空区点击不触发 OnPointer**：自定义 inputBox 的空白区命中的是内嵌 `*rendering.RenderBox`，换行/定位点击落不到外层 handler；生产用的 `textinput.Box` 不受影响（示例层命中写法问题，不属引擎缺陷）。
- **`ui_render_base_maskops` 活窗红→绿（2026-10-26 当日收口）**：复核曾发现活窗屏幕检查 `diff=63.2419%`（`screen=false`）、四张 masked 卡片全部不显示，而快照/golden/像素全绿；临时计数 `liveOnlySkips=2872`/360 presents，回退当日 IME 修复后逐位同值，证明基线即红。根因：掩码后处理只挂 `RequireLiveContext()`，retained 全帧录制通道（Recorder）拒绝，整卡内容丢失。修复见线程计划 §18.1 #19：新增 `render.MaskCanvas` 侧接口 + 5 条录制命令 + `MaskBackend`，`CanvasBackend` 按设备尺寸门回放并计 `MaskSkips`，`PaintContext.MaskCanvas()` 统一正口。复跑两连：`screen diff=0.0000%`（627200px、±2/255）、`pixel=18/18`、`golden=0.0000%`、`live_only_skips=0`、`mask_skips=0`，presents=247/229；该窗 `fps_gate=off_correctness`，5.5s 档 fps 44.6/41.4、frame_raster_ms 132/141、hitch 0/11，性能只记账不判绿。边界：设备空间通道目前只覆盖 full_paint 全帧录制（录制尺寸=回放尺寸）；retained 图层/局部录制遇设备空间掩码按尺寸不匹配计数跳过（语义不变）。**2026-10-27 用户空间掩码另起能力补齐**：`render.DrawShaderMask` / `PaintContext.DrawShaderMask`（隔离层 + 层内 `DstIn` 矩形，Flutter ShaderMaskLayer / Skia `saveLayer(bounds)+drawRect(dstIn)`）与设备空间通道独立；真窗 `ui_wr_shadermask` 两连 `pixel=18/18`、`pairs=9/9`、`golden=0.0000%`（912000px）、`screen=0.0000%`、`cpu_fallback_ops=0`、`mask_skips=0`、`savelayer_allow=2400+`，状态=**已验证待接线**。同批登记缺口（已于同日收口，见 §9 修订行）：渐变画笔几何不随 CTM 变换（平移画布后渐变按绝对坐标取样，建窗侦察临时对拍实测 37200px 差）；修复后 `ui_wr_shadermask` 直录列改回 cell-local 录制 + `Translate(cell origin)` 回放。
- **R17 最小化闩**：生命周期闩落下的瞬间调度即停，后续 tick 永远观察不到 `App.Minimized()`；窗把控制器报告的调用点当闩沿（本轮 t=22.0 / presents 1309–1311），最终化时算 bg 增量。将来若要读闩后状态，tick 路径拿不到。
- **C8 恢复快照时序**：恢复截图按 elapsed 排队（auto +1.0s / manual +0.5s），不是按"恢复帧已呈现"确认；慢机上可能早于恢复帧。auto 档 +0.3s 冻结把窗口压窄，但根缝仍是时间而非呈现计数。

### 5.4.2 光标/选区不显示——引擎修复（2026-10-26）

**现象**：`ui_wr_ime_r1_editor`（及其余 IME 窗）焦点正常切换、字形与背景都正常，唯独**光标和选区高亮不显示**——两者都是 `rendering.NewRenderColorBox` 画出的纯色块，字形不受影响。

**根因**：祖先的 mask/difference 裁剪被提升到离屏录制通道上。mask 平面按**整面坐标**存放，而录制通道按**录制局部坐标**绘制，提升后整幅采样恒为 0，纯色块 Fill 被静默丢弃；字形走另一条路径所以看不出来。

**修复（通用层，未在示例绕）**：
- `render/context_clip_mask_gpu.go` 新增 `maskClipPromotionAllowed()`：离屏子通道（`offscreenPassSuspended()`）内一律不提升，祖先掩码改由合成时生效，与 `setGPUClipRect` / `applyClipToPaint` 的既有契约一致。
- `render/context_draw.go` 两处 `forceCPUClip`（Fill/Stroke）改用该判定。
- `applyMaskToPaint` 在离屏子通道内直接返回——录制**完整画面**，祖先掩码不在子通道里烘焙。
- 回归护栏：`render/offscreen_record_mask_test.go` 的 `TestOffscreenPassSkipsAncestorClipMask` 固定三件事——子通道外允许提升、`offscreenPassDepth=1` 时禁止、`MaskCoverage` 子通道内为 nil / 外面非 nil。PASS。

**门禁盲区（为什么 auto-only 看不到）**：`-auto-only` 的快照是**合成的一次 live-context 重绘**，走不到 retained 录制通道，这个洞只会在常驻真窗出现；所以本缺陷必须靠人工档抓，快照绿不能替代。

**既有红（2026-10-26 定案并落地，非 P3 引入）**：
- `ui/scene` 旋转/缩放纹理两例（原 `ReplayedOps=1 want 0`）：根因是**GPU 像素契约放在了无 GPU 的测试包**——headless 下 `CreateOffscreenTexture` 不可用，`recordLocal` 按设计降级为向量回放，`ReplayedOps=1` 是诚实兜底，不是引擎缺陷。已迁到 `ui/scene/internal/gpupixel/textured_transform_gpupixel_test.go`（`requireGPU` + 离屏纹理，本机 headless 走 SKIP）；无 GPU 的缓存决策改由 `ui/scene/textured_transform_internal_test.go` 用假槽固定：旋转/缩放都录**未变形内容**、bounds-sized 纹理（42×42 / 12×12）、干净帧零向量回放。
- `ui/rendering` opacity（原期望 `#ff8080`、实得 `#ff7f7f`）：`#ff8080` 是直线 alpha 四舍五入；引擎按预乘 unorm8 存 127（`render/pixmap.go` 的 `setPremul` 截断 127.5），8-bit GPU 混合同值（S3b GPU 契约允许 ±20）。断言已改为 `#ff7f7f` 并注明口径；不改像素管线（改舍入会波及所有 CPU 像素与 Golden 基线）。
- 本机验证：`go test ./ui/scene`、opacity 用例、`go test ./ui/scene/internal/gpupixel` 全绿；GPU 像素契约待真机 GPU 窗复跑（本机 `CreateOffscreenTexture unavailable` → SKIP）。

**未收口项**：E1（14 扇）已豁免（§5.6）；E2（2 扇）已定案（§5.7）；六扇 IME 窗真机人工验收已完成（§5.4.1，光标/选区引擎缺陷已修并复验，§5.4.2）；`ui_render_base_maskops` 活窗红已收（§5.4.1，2026-10-26 两连全绿，`live_only_skips=0`/`mask_skips=0`）。**P3 已收口（2026-10-27）：上述证据 + Flutter/Godot 对照尺 + gpui 同语义 parity 臂（§5.5/§5.5.3）；P4 未开工。**

## 5.5 P3 对照尺实测（2026-10-26 / 2026-10-27 本机，stable 3.47.8 / Godot 4.5.1 / glxgears）

**纪律**：Flutter/Godot/gpui 同机（NVIDIA 940MX，580.178.04；参考臂 2026-10-26 在 X11 :0，gpui parity 2026-10-27 在 X11 :1，同机同驱动，display 号不影响 `/proc` 口径）、同客户区（Flutter/Godot/gpui parity 1200×800，spin 480×320）、60s 测量窗（Flutter/gpui list 预热 20s，其余 5s）；三臂均三跑取中位，glxgears 单跑。CPU 是整个进程 `/proc/<pid>/stat` 的 utime+stime（含内核态，不含 Xorg/合成器）。Flutter/Godot 原始 `report.json`/截图在 `/tmp/opencode/ruler_runs/final_*`；gpui parity 三跑在 `/tmp/opencode/parity_runs/p3_{spin,list,text,godot}_{1,2,3}`；可用 §8 命令重跑，不入库。gpui parity 每次结束由 `measure.py` 发 WM_DELETE_WINDOW 走标准关闭，12 次均为 `close_request:"wm_delete"`、`exit_during_run:false`，宿主 Xorg PID 4612 与 `Xorg.1.log` 大小/mtime 全程未变。

### 5.5.1 Flutter 臂（stable 3.47.8，engine `cf636b1dfb`，Dart 3.13.7）

- 构建：本机无 clang，用 `/tmp/opencode/flutter_toolchain/{clang,clang++}` 包装 gcc/g++；`flutter doctor -v` 退出 0，`flutter build linux --release` 成功，`flutter analyze` 无 issue。
- 出画：Impeller 后端 `OpenGLESSDF`，NVIDIA 940MX 上三场景截图均**非黑**（black_fraction=0，色数 207/1921/2639）。这解除了 P1 的 master 3.50 黑屏阻塞；stable 3.47.8 是有效尺。
- 布局：header 48、body 656、HUD 72；text 页所有文本区含 3x 行可见（最终二进制就是下表数据来源）。

| 场景 | CPU 中位 | RSS 起→止（峰值） | RSS 斜率中位（范围） | fps 中位（min） | p95 中位 | hitch |
|---|---:|---:|---:|---:|---:|---:|
| spin 480×320 | **25.41%** | 152.4→151.2MB（152.7） | 81.9（70.6–99.1）KB/min | 61.0（60.0） | 16.67ms | 0 |
| list 1200×800 | **71.38%** | 190.0→193.8MB（197.7） | 707.1（135.6–2259.0）KB/min | 52.4（17.5） | 33.33ms | 810/60s（801–842） |
| text 1200×800 | **0.02%** | 209.7→209.8MB（209.8） | 101.8（101.0–105.0）KB/min | 0（首帧后无 ticker） | — | 0 |

- list 三跑 CPU 71.64/71.38/71.32，fps 52.5/48.7/52.4，p95 均 33.33ms；hitch 801–842 是 60s 总次数，即约 13.5 次/分钟。RSS 斜率有一次 2259KB/min 的波动，中位 707KB/min，仍远低于 30000 的健康线。**Flutter list 自身不达 60fps 健康线**（fps<55、p95>22ms、hitch>5/min）；这里只把它当对照尺，不拿它当 Flutter 验收。
- text 的 0.02% 只说明**静态 idle**：首帧后没有动画/没有新帧，与 m5 的 HUD 60fps tick 不是同一语义；不能拿它宣称 Flutter 文本整形快。

### 5.5.2 Godot 臂（4.5.1-stable，`f62fdbde1`，`gl_compatibility`）

| 场景（`GODOT_BULLETS`） | CPU | RSS | fps | 说明 |
|---|---:|---:|---:|---|
| **500（上游默认）** | **19.1%** | 160.3→160.3MB | **60（59–61）** | 3×60s 中位；1200×800；截图非黑 |
| 1000 | 28.94% | 163.6MB | 60（59–60） | 10s 容量探针（预热 5s） |
| 5000 | 102.6% | 190.2MB | 6–7 | 单线程物理饱和，过载探针 |
| 10000 | 102.66% | 225.5MB | 3（t=6.3–12.5s，之后探针停更） | 严重过载 |

- CPU 是整进程 utime+stime，可超过 100%（多核累加）；5k/10k 约等于单核跑满。
- 500 发是上游 demo 语义：每发一个 `PhysicsServer2D` body + 碰撞圆 + 逐帧 `draw_texture`；`GODOT_BULLETS` 只改对象数，物理服务器仍逐对象更新。因此 1k/5k/10k 是**含物理的容量探针**，不是纯图元 parity。
- 500 发三跑 CPU 19.1/18.95/19.6，RSS 全程 160.3MB，fps 中位 60；1200×800 截图有 290 色、非黑。

### 5.5.3 gpui 对照臂与比值

**同语义 gpui 对照臂（`examples/ui_ruler_parity`，2026-10-27，三跑中位）**

| 场景 | CPU 中位（三跑） | RSS 起→止（峰值） | RSS 斜率中位 | fps 中位（min） | p95 | hitch | Golden（两连） |
|---|---:|---:|---:|---:|---:|---:|---:|
| spin 480×320 | **14.16%**（14.04/14.16/14.17） | 94.5→94.8MB（95.0） | 179.9KB/min | 59.9（59.4） | 16.98ms | 0 | 0.0000%（153600px） |
| list 1200×800 | **22.99%**（22.37/22.99/24.82） | 301.2→309.7MB（309.7） | 8920.5KB/min | 59.9（59.9） | 17.25ms | 0 | 0.0000%（873600px） |
| text 1200×800 | **0.42%**（0.42/0.42/0.43） | 627.5→628.5MB（628.5） | 957.8KB/min | 0（无 ticker） | — | 0 | 0.0000%（873600px） |
| godot 1200×800 | **20.33%**（20.01/20.33/20.56） | 90.7→91.4MB（91.6） | 606.3KB/min | 59.9（59.9） | 17.26ms | 0 | 0.0000%（960000px） |

- 12 次 60s 全部走 `measure.py` 标准关闭：`close_request:"wm_delete"`、`exit_during_run:false`，应用 stderr 为 `ui_ruler_parity: close (x11)` / `OK`；宿主 Xorg PID 4612 与 `Xorg.1.log` 大小/mtime 全程未变。四张 Golden 于 2026-10-27 重出（旧标题/ HUD 基线作废），seed 后各连续两次零差异；旧基线留档 `/tmp/opencode/old_parity_goldens/`，不入库。
- list 的 RSS 斜率 8920.5KB/min 仍远低于 30000KB/min 健康线；text 的 0.42% 是首帧后静止的 idle 成本，不代表文本整形吞吐。
- Godot 臂的 `report.json` 里 `scene` 字段是 app basename `ui_ruler_parity_sig`（未传 `--scene`，采数器按 app basename 取），窗口标题/参数确认场景是 Godot；不要按 `scene` 字段误判。2026-10-27 三跑时窗口标题还是 `gpui ruler parity`；随后 Godot 臂已把外壳对齐上游（标题 `Bullet Shower`、可缩放且内容跟随、客户区隐藏鼠标），复跑时 `--window-match` 用 `'Bullet Shower'`。外壳改动不触及内容渲染与进程 CPU 口径，上表三跑数字保留。
- **Golden 是独显口径**：四张 Golden 基线在 940MX 上生成。默认适配器策略在混合机优先核显，2026-10-27 用同一 HEAD 代码在核显复跑得到 godot **1.6710%** 差异（全部为 ±1 灰度、最大通道差 2、16042px，精灵边缘纹理过滤舍入差）；`GPUI_POWER=high` 下当前代码两连 **0.0000%**、快照与基线逐字节一致。门禁/重出必须带 `GPUI_POWER=high`；核显差异不是内容回归，不要用重新 seed 的方式“修绿”。
- 三跑原始值、截图与 `report.json`：`/tmp/opencode/parity_runs/p3_{spin,list,text,godot}_{1,2,3}`；复跑命令见 §8。

**同语义比值（对照 / gpui；CPU >1 表示 gpui 更低，RSS >1 表示 gpui 更低）**

| 场景 | Flutter/Godot 对照 | gpui parity | 比值 / 结论 |
|---|---|---|---|
| spin 480×320 | Flutter **25.41%**；RSS 152.4→151.2MB（峰值 152.7）；61.0fps（60.0） | **14.16%**；94.5→94.8MB（峰值 95.0）；59.9fps | CPU **1.79×**、RSS **1.61×**：gpui 低 CPU、低 RSS；两侧动画语义相同，轮廓实现不同 |
| list 1200×800 | Flutter **71.38%**；RSS 190.0→193.8MB（峰值 197.7）；52.4fps（17.5）、p95 33.33ms、hitch 810/60s | **22.99%**；301.2→309.7MB（峰值 309.7）；59.9fps、p95 17.25ms、hitch 0 | CPU **3.10×**；RSS **0.64×**（gpui 高 1.57×）。**Flutter 自身未达 60fps**，CPU 比值不是同帧率等价；gpui 用 retained/virtual 列表，Flutter 用 lazy `ListView` |
| text 1200×800 | Flutter **0.02%**（静态 idle）；RSS 209.7→209.8MB | **0.42%**（静态 idle）；RSS 627.5→628.5MB | 两侧首帧后都零帧；gpui CPU 0.42% 对 Flutter 0.02%，RSS **0.33×**（gpui 高 3.0×）。只作 idle 对照，不宣称文本整形快慢；gpui 字体/图集常驻偏高登记为 P4 观察项 |
| Godot 500 发 | **19.1%/60fps**；RSS 160.3MB | **20.33%/59.9fps**；RSS 90.7→91.4MB（峰值 91.6） | CPU **0.94×**（gpui 高 6.4%）；RSS **1.75×**（gpui 低）。Godot 每发有 PhysicsServer2D body + 碰撞圆，gpui 只做绘制/回绕/玩家碰撞，**不是 1:1 物理等价** |

**不同实现的能力窗参照（保留，不与同语义表混用）**

| 场景 | 对照数字 | gpui 数字 | 比值 / 结论 |
|---|---|---|---|
| spin 480×320 | Flutter **25.41%** | spin_min **15.92%**（§5.7）；本轮同采数器复核 15.02% | Flutter / gpui = **1.60×**（用 15.02 则 1.69×）；RSS 152.4 vs 137.7MB = 1.11×；同语义臂见上表 14.16% / 1.79× |
| list 1200×800 | Flutter **71.38%** | C3 **31.98%**（本轮复核 30.19%）；R7 18.27% | 对 C3 **2.23×**（用 30.19 则 2.36×）；对 R7 **3.91×**。RSS：Flutter 190→194MB，C3 355→365MB——**Flutter RSS 反而更低**，CPU 才是 gpui 的优势；同语义臂见上表 22.99% / 3.10× |
| text 1200×800 | Flutter **0.02%**（静态 idle） | m5 **29.20%**（HUD 60fps tick；本轮复核 CPU 22.64%） | 只作静态 idle 对照；m5 的 CPU 含 tick 税，不是文本整形成本。同语义臂见上表 0.42% 静态 idle |
| Godot 2D demo 500 发 | **19.1%/60fps** | 同语义 parity **20.33%/59.9fps**（上表） | **0.94×** CPU（gpui 高 6.4%）；RSS 160.3 vs 91.4MB（gpui 低）；Godot 有逐对象物理，不做 1:1 图元比值 |
| Godot 1k vs gpui 1k | Godot 28.94% | batch_cap 1k 22.9–23.2% | 1.26×，两者都 60fps；仅作量级参照 |
| Godot 5k/10k | 102.6% / 6–7fps；102.66% / 3fps | batch_cap 10k 80.7–80.9% / 60fps | **不可直接比**：Godot 有逐对象物理，gpui 是 atlas 图元；只说明 Godot 这档被单线程物理压垮 |
| 地板 glxgears | **2.74%**（300×300，60s） | — | 60fps 呈现地板；历史 3.30% 仅作参考 |

**语义差异（必须随数字一起读）**：
- list：同 1200 项变高行、同 44/92/60/76 行高、四类行型、24 图、Steady/Spike/Recover 相位、列表宽 620；Flutter 用 lazy `ListView`，gpui C3 与 parity 臂都是 retained/virtual 列表，不是同一实现。CPU 3.10× 是真实差距，但不能归因到单一 API；Flutter 本机只有 52.4fps，不能把两侧 CPU 当同帧率等价。
- idle/text：Flutter text 页是密集静态页，首帧后零帧；gpui parity text 同样首帧后静止，行 1 只比“零脏 idle”属性，不比页面内容。
- spin：同 64px/60fps 旋转；Flutter 用 Material `sync` 图标，gpui 用 kit 图标，字形不同。
- text：同 m5 的 multiscript/emoji/richtext 语料；Flutter 首帧后静止，m5 有 HUD tick，故只比 idle，不比文本整形吞吐；parity text 的 0.42% 是 gpui 静态 idle 成本。
- Godot：500 发是官方 demo 默认；gpui parity 不建逐对象 PhysicsServer2D body，只对齐绘制/回绕/玩家碰撞语义；5k/10k 的 6–7/3fps 是物理过载，不是 Godot 渲染器对 10k 图元的上限。

**旧 §5.5 数字作废**：旧表（Flutter 26.28/31.03%、list 109.68%、text 0.00%）来自 P1 的黑屏/未验证阶段，和旧 §3.1 的“Impeller 黑屏”直接矛盾，且本轮 stable 3.47.8 无法复现 109.68% 的 list 值（真实为 71.38%，截图为可见列表）。旧表整节不再作为基线；以本节三跑中位为唯一依据。

**本轮新发现（登记，不并入对照基线）**：m5 60s 复核 CPU 22.64%、RSS **1.09GB→1.63GB**（约 0.5GB/min）；独立 35s 复核 1.09→1.73GB，增长更快、未收敛。旧基线只记 CPU 29.20%、未记 RSS。该异常归 row 12 / P4，在立账前先定位，不拿 m5 RSS 作文本对照。gpui parity text 的 RSS 627.5→628.5MB 也只作静态页常驻记录，不当作文本整形基线；P4 一并定位字体/图集常驻。

**m5 RSS 暴涨定位与修复（2026-10-27，row 12）**：不是泄漏，是字体文件按字符重复整份加载。① `gctrace` 存活堆 13s 后平台在 **805MB**（`802@13s / 805@33s / 804@53s`），RSS ≈ 2× 存活（GOGC=100 堆目标 1278→1610MB），与 1.09→1.63/1.73GB 吻合；VRAM 排除（1024MiB 独立显存，窗口 1200×800）。② 加载日志：**75 次 `NewFontSourceFromFile` 只覆盖 4 个文件**（Padauk 487K×37、KhmerOS 260K×15、TibetanMachineUni 4.4M×13、NotoColorEmoji 11M×10 ≈ 183MB 原始字节），全部集中在开窗后 0.8s——根因是 `fontScanFallback.resolveFace` 只按 `(rune, size)` 缓存 face，每个未命中 rune 都重新读盘 + 解析整份文件（每次两份字节：`os.ReadFile` + 防御拷贝，再加解析表与位图表）。强制 GC 后存活 769MB，约 460MB 出自这里。③ 修复（引擎层通用方案，对齐 Skia `SkTypefaceCache` / Flutter font manager）：`NewFontSourceFromFile` 统一走进程级 `fontSourceCache`，键 `(绝对路径, TTC index, 解析器)`，LRU 32 条/256MB + singleflight，一个文件只读盘解析一次；缓存源 `Close` 为 no-op，`PurgeFontSourceCache()` 刷新；共享源支持多家族（不可变家族集合 + CAS）；缺字回退 face 改键 `(路径, 字号)`。④ 改后 60s 实测（`GODEBUG=gctrace=1`，`GPUI_POWER=high`，X11 :1）：加载 **75→10**（`Loads=10/Entries=10/Hits=4/Waits=0`，10 个不同文件各一次，54.5MB 文件字节）、存活堆 **144MB** 平台（`285→285→144MB`，29–58s）、RSS **464MB** 稳态（53–60s；退出瞬间单采样 549MB 为进程内 golden 比对分配，不属窗口稳态）、CPU 23.0%、`cpu_fallback_ops=0`、pixel 11/11。⑤ 像素副产物：回退 face 由逐 rune 改为按文件共享后，`MultiFace.Runs` 能把同文件连续字符合成一个整形段——藏文由“逐字符 + 虚线圈占位”变为正确合簇渲染（干净 HEAD 与改后快照差异集中在该行，其余内容逐位一致）。⑥ golden：`showcase_text_m5_base.png` 旧基线（10-25）在本机已陈旧，**干净 HEAD 基线跑同样 3.1822%**（面板边缘整行位移，非本次引入），改后 3.4650% 含上述藏文合簇修正；2026-10-27 经确认用当前代码在独显（`GPUI_POWER=high`）重出基线，随后 `GPUI_POWER=high -auto-only` 两连 **golden 0.0000%（821184px）**、pixel 11/11、`cpu_fallback_ops=0`。不带 `GPUI_POWER=high` 会落核显，与独显基线是适配器舍入差（核显 vs 独显快照 116557px 不同，其中 116367px ≤2 灰度，>8 的 174px 集中在面板边缘整行与 HUD 带），与 §5.5.3 的“Golden 是独显口径”一致，不是内容回归。

**嵌套 Wayland 呈现观察（2026-10-27，待定位，不并入对照）**：在嵌套 GNOME 42.9（软件渲染，`WAYLAND_DISPLAY=gpui-nested`）下复验 godot 臂：`backend=wayland` 确认；隐藏光标与默认光标对照成立（同会话 spin 截图有箭头、godot 无箭头）。但 godot 内容区在 x11grab 截屏里呈**黑色**，sprites 正常；app 侧 `RULER_SNAP` 是正确灰底 `#4D4D4D`（窗口被合成器约束为 748×541），同会话 list（同为 fullPaint）与 spin 背景均正常，HEAD 二进制也复现黑底。绘制侧无异常，疑点在 Wayland 呈现/合成路径；需真 Wayland 会话复验后再定性，暂不修、不标绿。

**P3 收口结论**：Flutter/Godot 基准各三跑取中位，gpui 同语义对照臂四场景各三跑取中位，glxgears 单跑地板，三臂均真实出画；四张 parity Golden 重出后两连零差异；旧 P1 黑屏阻塞已由 stable 3.47.8 解除；Flutter/Godot/gpui 的语义边界已写明。P3 只收口“对照尺 + 三扇新窗 + 尾账”，不把 row 12 的 m5 CPU/RSS 立账算绿；P4 接全矩阵对照与 m5/parity text RSS 定位。

## 5.6 E1 机器证据（14 扇，2026-10-26 本机实测）

E1 的 14 扇断言目标是**控制回路、平台能力、性能、行为**，不是画面形态：有的画面是整屏清色/系统窗框/指标看板，有的全程在动，有的要真人敲键盘。这类窗补 Golden 只能冻结"演示壳自己长什么样"，证不了被测能力，所以按用户定案走"**先补机器证据、再书面豁免 Golden**"。本节是补出来的机器证据；豁免已于 2026-10-26 经用户书面签收。

- 环境：Linux X11（`DISPLAY=:0`，Xorg/GNOME），核显（`gpu_backend=integrated`），`LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`。
- 原始产物落在 `/tmp/e1/`（JSON / stderr / 退出码），命令均可用 §8 的 `RUN_SECONDS` 方式复跑；表中数字为当次实测，不作长期基线。
- 判定口径：各窗自带门禁（缺 JSON 键、presents=0、虚拟化绑全量等直接 FAIL/非零退出）；本节只记**机器证据与残留**，不把 E1 标成矩阵行绿。

| # | 窗 | 断言目标 | 机器证据（2026-10-26 实测） | 通道 | 为何不补 Golden |
|---|---|---|---|---|---|
| 1 | `ui_l1_blank` | 呈现回路地板 | 3s：presents=180、hitch=0、avg 16.46ms、p95 16.97ms、missed_vsync=0、gpu_fallbacks=0 | stdout JSON + stderr | 画面=整屏清色，无形体语义；清色像素由带 Golden 的窗覆盖 |
| 2 | `ui_l1_scroll` | 虚拟滚动 | 5s：presents=301、hitch=0、bind=17/1000、layout=10；40s：presents=2399、hitch=0、RSS 稳态斜率 575.6KB/min（valid）、peak 131.2MB、关窗后 130.4MB | stdout JSON | 画面=列表演示；列表骨架/滚动由 C3/R7 的 Golden 覆盖 |
| 3 | `ui_l1_spinner` | retained 局部动画 | 5s：presents=301、hitch=0、layout=1、raster_layers=2、damage=144px、boundary_skip=5920 | stdout JSON | 画面=旋转指示器演示；冻结相位形态由 `ui_render_particle`（pixel 4/4 + Golden）覆盖 |
| 4 | `ui_l2_shell` | 交互外壳 | 静态窗 3.1s：presents=2（present-on-change 正常）；tap/pan/focus/overlay 由 `examples/ui_l2_shell/scene_test.go` 断言（`TestScene_Integration_TapPanFocusOverlay` 等） | stdout JSON（零输入计数）+ 单测 | 真窗计数要真人点击；机器档只证窗与事件泵在跑，交互语义落在单测 |
| 5 | `ui_render_base_perfsoak` | 连续压测 | 5s：presents=302、hitch=0、p95 17.31ms、gpu_ops=655、cpu_fallback=0、layout=1 | stdout JSON | 画面刻意全程变化（压测语义），没有可冻结的稳定形态 |
| 6 | `ui_text_m4_virtual` | 1M 行虚拟化 | 自检：首屏 5.71ms、物化 42 行、堆增量 17.0MB；真窗 3s：presents=180、hitch=0、bind=45/1e6、measured=69、window_lag_max=0、总高 20M px | stdout JSON（自检 + 窗） | 画面=日志文本；文本形态由 m5/IME 带 Golden 的窗覆盖，此窗证的是绑定规模 |
| 7 | `ui_wr_r12_metrics` | 指标 schema 契约 | 6s：presents=360、hitch=0、family_keys=42、gate=schema_only、hot_repaints=359 | stdout JSON（缺键即 FAIL）+ 退出码 | 画面=指标看板，被测本体是 JSON 契约不是画面 |
| 8 | `ui_pf_x11` | X11 窗口能力 | 19/19 PASS；EventPump 29 events | stdout JSON `{backend,rows,pass}` + 退出码 | 画面=系统窗框，无引擎内容 |
| 9 | `ui_pf_wayland` | Wayland 窗口能力 | 19/19 PASS、backend=wayland（嵌套合成器）；协议禁止项诚实返回 unsupported：Decorations/Position/Focus/AlwaysOnTop/RequestMove/RequestResize | stdout JSON + 退出码 | 同上；嵌套软件栈只作能力证据，数字不作性能结论（见 `ENGINE_WAYLAND_NESTED_TEST.md`） |
| 10 | `ui_pf_appkit` | macOS 窗口能力 | 本机 SKIP：mode=skip、pass=true、row=`SKIP: AppKit real-window verification runs on macOS (this is linux)` | stdout JSON + 退出码 | 本机无 macOS；**状态=未验，不标绿**，待 macOS 机器复跑 |
| 11 | `ui_pf_win32` | Windows 窗口能力 | 本机 SKIP：mode=skip、pass=true、row=`SKIP: Win32 real-window verification runs on Windows (this is linux)` | stdout JSON + 退出码 | 本机无 Windows；**状态=未验，不标绿**，待 Windows 机器复跑 |
| 12 | `video_player` | 视频业务样例 | 5s：presents=290、出帧 27、uploads=27、fallbacks=0、retained 证据 ok=true、墨 1.00/1.00、动 0.15、退出 0 | stderr 结构化行（设计上无 JSON） | 视频业务不属底座矩阵（§0 范围）；此窗只证样例链路可跑 |
| 13 | `zz_soak_control` | 多窗压测对照组 | 3 窗 15s：presents 882–894（每窗 ~56.3–57.0fps）、p95 20.9–22.8ms、hitch 1–6、rerecord=22、skip≈7000 | stderr CTRL 行（对照组，无 JSON） | 画面=移动方块，是对照组不是能力窗；性能判定归 `ui_base_multiwin_cap` |
| 14 | `ui_ime_probe` | IME 活体探针 | x11：IME capability AVAILABLE、EnableIME sent、`XGetInputFocus=OUR-WINDOW`、合成按键 KEY `a`×2 / `b` / `space` 全到（code 97/98/32） | stderr 事件行（设计为活体手测） | 画面=空探针窗；真实合成提交要真人+输入法，六扇 IME 窗已有 Golden |

**证据通道说明（豁免 JSON 的两扇）**：`zz_soak_control` 与 `ui_ime_probe` 设计上就不是 JSON 门禁窗——前者是压测对照组（`CTRL win%d presents=… hitches=…` 结构化 stderr），后者是活体探针（逐事件 stderr 行）。两者的退出码同样是门禁信号（打开失败/没出画即非零）。

**未能机器补齐的残留（如实登记，不混进绿）**：
- `ui_pf_appkit` / `ui_pf_win32` 真窗能力：本机只有 Linux，SKIP 不是 PASS；需 macOS/Windows 机器各复跑一次 `go run ./examples/ui_pf_appkit|ui_pf_win32`。
- `ui_ime_probe` 真实输入法合成提交（IME kind=1 commit）：需真人 + 输入法；
- `ui_l2_shell` 真窗 tap/pan/focus/overlay 计数：需真人点击（语义已由单测固定）；
- `ui_pf_wayland` 性能类结论：嵌套软件合成器数字不作数，仅作能力证据。

**豁免决定（2026-10-26 用户签收）**：上述 14 扇按"不补 Golden、接受机器证据 + 既有单测/JSON 契约"豁免像素三证据；appkit/win32 真机与真人项仍作为残留登记，不影响豁免成立。

## 5.7 E2 定案（2 扇：jumprope 补 Golden、spin_min 豁免）

E2 是两扇常驻动画窗：画面一直在动，原先都没有 Golden。按用户 2026-10-26 选定方向分开处理——能冻结相位的补 Golden，冻结会改变被测目标的写书面豁免。

### 5.7.1 `ui_render_jumprope`：冻结相位 + 零容差 Golden（两连 PASS）

- 门禁档：`-auto-only` 固定 10s，构造后立即 `sim.freezeAt(0.25)`（绳在手线前层，`simT=0` 保证绳横摆与计数文本都为零相位），底部 64px LiveHUD 实时 fps 带排除在 Golden 外。
- 证据（`testdata/jumprope_base.png`，Golden 区 1200×736 = 883200px，零容差）：
  - 第 1 跑：presents=600、~58.4fps、hitch=0、pixel 5/5、golden 0.0000%；
  - 第 2 跑：presents=599、~58.2fps、hitch=0、pixel 5/5、golden 0.0000%。
- 像素探针（语义锚点，tol=3）：地板 `(201,180,142)`、左摇绳人头 `(242,201,154)`、跳绳人头 `(242,201,154)`、跳绳人躯干 `(74,144,217)`、手线绳段 `(122,74,33)`。
- 播种档 `GPUI_ACCEPT_GOLDEN=1` 只生成基线，明确打印 SEEDED 且不计 PASS。
- 复现：`GPUI_ACCEPT_GOLDEN=1 go run ./examples/ui_render_jumprope -auto-only`（首次播种）→ `go run ./examples/ui_render_jumprope -auto-only`（两连验收）。

### 5.7.2 `kit/spin_min`：书面豁免 Golden（矩阵行 2 心跳税地板）

- 豁免理由：这扇窗存在的意义是量"常驻最小动画的每帧税"（CPU/RSS/presents）；画面按设计一直在旋转，冻结相位会把它变成静态窗，测的就不是心跳税了。静态图标形态属组件（`ui/kit`）覆盖率，不归底座资源矩阵。
- 机器证据（60s 档，2026-10-26）：presents=3592、hitch=0、avg 16.71ms、p95 18.20ms、CPU **15.92%**（ui 1.10 + raster 14.82）、RSS peak/after_close 137.7/137.7MB、稳态斜率 **511.9KB/min（valid）**、gpu_ops=7187、fallback=0、damage=7225px（damage_union）、退出 0。
- 证据链修复（本轮，示例层只补取证不动行为）：ticker 原先不调 `proc.Sample()`，60s 只有首尾两点，稳态斜率永远 `valid=false`；已按其他真窗同款每帧采样。同时按 README 原有承诺补 stdout 指标 JSON，并把 README 的"JSON/门禁"描述改成与代码一致。
- 复现：`RUN_SECONDS=60 go run ./examples/kit/spin_min`。

**E2 残留**：`spin_min` 无 Golden 属书面豁免；若将来组件域要求图标形态回归，应在 `ui/kit` 的组件测试里冻结，不在本矩阵补。

## 5.8 P4 对照实测——行 10 多实例（2026-10-27，隔离 Wayland 会话 / XWayland `:1`）

按安全口径批量多窗在隔离显示跑：真 GNOME Wayland 会话，Flutter 臂强制
`GDK_BACKEND=x11`（XWayland），gpui 臂 `GPUI_RULER_BACKEND=x11`。机器抓合成
像素在该环境三条路都不通（Shell 截图 API 白名单拒绝、`gnome-screenshot` 未装、
GL 窗口 `XGetImage` BadMatch，详见 `tools/ruler_flutter/README.md`），行 10
像素证据走「自测 + 人工二合一」：机器证据 = 进程存活 + X11 窗映射（标题/几何）
+ 应用侧逐窗 `RULER_METRIC` 帧统计，**用户目击已确认**（2026-10-27，窗数递增、
全程旋转在动）。gpui 臂自身门禁在该环境 golden 两连 0.0000%（基线来自桌面
Xorg）+ 像素 16/16，等价于「同一代码在 XWayland 与 Xorg 出画逐位一致」。

### 5.8.1 Flutter 多实例臂（N 进程 × 1 窗，60s × 3 跑取中位）

窗语义：`RULER_SPIN_<i>`，480×320，摆位 col=i%4 / row=i/4 / X=col*480 /
Y=row*380（与 gpui multiwin 同款）；窗均满帧语义 = 每进程独立 Flutter
`FrameTiming` 计帧。

| N（窗/进程数） | 三跑 CPU 合计（%） | 中位 CPU | 合计 fps 中位（逐窗） | RSS 末合计（MB） | 单窗 fps 最低 |
|---|---|---:|---:|---:|---:|
| 1 | 26.75 / 27.19 / 27.24 | **27.19%** | 61.0 | 162.5 | 61.0 |
| 2 | 62.13 / 62.16 / 62.52 | **62.16%** | 60.92 | 326.5 | 60.5 |
| 4 | 131.62 / 131.27 / 130.96 | **131.27%** | 59.15 | 653.0 | 58.9 |
| 8 | 177.09 / 176.16 / 176.09 | **176.16%** | 41.0 | 1305.3 | 38.6 |

- 1→4 窗线性堆叠：约 27→33% 每进程，合计 4 窗 ≈ 4×33%。
- **8 窗进入机器饱和**：逐窗 fps 降到 38.6–48.8（合计 ≈328 帧/秒），每窗 CPU
  回落到 ~22%（帧率低了、每窗工作量变少），合计 176% ≈ 6.5×N=1 基线。
- RSS 严格线性：≈163MB/进程，8 窗 ≈1.3GB。
- N=1 走单窗路径标准关闭（`close_request=wm_delete`、`exit_during` 无）；N≥2
  走多实例路径，采样结束对每窗发 WM_DELETE（`close_requests_sent=N`）后按
  `RULER_SECONDS` 定时自退，无硬杀。
- 单窗 spin 参照：桌面 Xorg 三跑中位 25.41%（§5.5.3）；本会话 XWayland 27.19%，
  环境漂移 +1.8 个百分点，随行 2 语义一起读。

### 5.8.2 gpui 臂（一进程八窗，`-auto-only` 门禁，RUN_SECONDS=90）

`ui_base_multiwin_cap` 门禁先常驻全部 8 窗 + 预热（10s，行 1 契约：未激活窗
零帧休眠），再按 1→2→4→8 各 12s 相位（激活前 1s settle）。

| 相位（激活窗数） | fps avg（min） | p95 上限（ms） | CPU 合计 | 单窗摊销 | 边际 CPU | RSS（MB） |
|---|---|---|---:|---:|---:|---|
| 1 | 59.83（59.83） | 18.14 | 22.42% | 22.42% | 0 | 232.05→232.11 |
| 2 | 59.91（59.83） | 18.56 | 43.83% | 21.92% | 21.41% | 232.17→232.62 |
| 4 | 59.39（59.16） | 28.77 | 81.33% | 20.33% | 18.75% | 232.68→233.33 |
| 8 | 39.57（39.50） | 33.11 | 111.0% | 13.87% | 7.42% | 233.51→234.45 |

- 门禁判：`max_windows_60fps=4`（相位 4 已不稳 60）、`max_windows_steady60fps=2`、
  全相位 CPU 可用、8/8 窗 presents 逐窗有效（总 12746）、`crash_free=true`、
  scale(8/1)=4.95、golden 逐窗 0.0000%（掩码 480×280 内容区，15ms 内指标带
  排除）、pixel 良全部 16/16、MRSS 斜率 6118KB/min < 30000 线（valid）。
- 一进程八窗的 RSS 常驻 ≈234MB：从 1 窗到 8 窗全开，进程级 RSS 只增 2.3MB
  （每窗 incremental 表面/缓冲不进进程堆主账）。

### 5.8.3 比值与结论（对照 / gpui）

| N（窗数） | Flutter CPU | gpui CPU | CPU 比值 | Flutter RSS（MB） | gpui RSS（MB） | RSS 比值 | 两侧合计 fps |
|---|---:|---:|---:|---:|---:|---:|---|
| 1 | 27.19% | 22.42% | **1.21×** | 162.5 | 232.1 | 0.70× | 61.0 / 59.8 |
| 2 | 62.16% | 43.83% | **1.42×** | 326.5 | 232.6 | 1.40× | 60.9 / 59.9 |
| 4 | 131.27% | 81.33% | **1.61×** | 653.0 | 233.3 | 2.80× | 59.2 / 59.4 |
| 8 | 176.16% | 111.0% | **1.59×** | 1305.3 | 234.4 | **5.57×** | ≈328 / ≈317 |

**结论（三句）**：
1. **多窗共摊是 gpui 的主场**：一进程八窗共享调度器/渲染器，单窗摊销从
   22.4% 降到 13.9%，RSS 八窗常驻 234MB；Flutter 每窗一个进程，RSS 5.57×、
   CPU 偏高 1.21→1.61×（窗数越多差距越大）。
2. **8 窗机器饱和点两侧对齐**：Flutter ≈328 帧/秒（8×41）与 gpui ≈317 帧/秒
   （8×39.6）在 ~316–328 区间贴住同一机器上限（480×320×8 全重绘），说明该
   上限是环境瓶颈不是任一实现的调度 bug。
3. **1 窗时 gpui RSS 反高**（232 vs 163MB）：gpui 进程带着 8 预开窗的表面
   与引擎常驻（row 10 语义就是先全开再激活），不能拿这格宣称「gpui 单窗
   RSS 高」——单窗常态照 §5.5.3（spin 94.5MB）。

**语义差异（必须随数字一起读）**：
- 臂结构：Flutter 稳定版无一进程多窗公开 API（多实例即多进程），gpui 真窗
  先全开后阶梯激活；CPU/RSS 的「每窗」分母不同（见 RSS 1 窗格）。
- 8 窗降帧机制不同：Flutter 是各进程事件循环被终端饱和拖慢、拦截在自家帧
  调度里；gpui 是调度器主动把 60fps 目标降到机器可承载的总量。两者合计
  吞吐同点饱和，逐窗 fps 分布不同（Flutter 38.6–48.8 抖动，gpui 39.5 均拍）。
- N=1 与 N≥2 的关闭口径不同（单窗 WM_DELETE 后退出；多窗发完 WM_DELETE
  按 self-退出定时收尾），都无硬杀。

数据与复现：`/tmp/opencode/ruler_runs/P4A_multi/`（12 轮 + N1 三跑 +
gpui 门禁日志/JSON）；Flutter 臂逐轮命令见 `tools/ruler_flutter/README.md`
（多实例 `--instances N` + `GDK_BACKEND=x11`；N=1 单窗路径 `--window-match
RULER_SPIN`）；gpui 臂 `RUN_SECONDS=90 GPUI_RULER_BACKEND=x11 go run
./examples/ui_base_multiwin_cap -auto-only`（门禁下限 75s）。

### 5.8.4 行 4/5/6/7/8/9/13/16 Flutter 单窗臂（桌面 Xorg `:1`，60s/30s × 3 跑取中位，2026-10-27）

口径与 §5.5 同款：桌面 Xorg 单窗、标准关闭（十二跑全数 `close_request=wm_delete`、
`exit_during_run=false`，宿主 Xorg PID/log 轮间检查未变）、整进程 CPU、
`x11grab` 客户区截屏通过（画面非黑）。

| 场景（对行） | 三跑 CPU（%） | 中位 CPU | fps 中位 | RSS 末（MB） | 备注 |
|---|---|---:|---:|---:|---|
| full（行 4） | 72.75 / 73.91 / 73.98 | **73.91%** | 53.2 | 167.5 | 全窗重绘未满 60，比值非同帧率等价 |
| cp（行 13 矢量，100 动画贝塞尔） | 74.63 / 69.80 / 69.90 | **69.90%** | 58.5 | 172.4 | 工具类矢量语义，不与 batch_cap 梯 1:1 对位 |
| boundary（行 6） | 68.66 / 68.77 / 70.36 | **68.77%** | 55.0 | 182.7 | damage 传播/边界开关族 |
| overlay（行 8） | 69.77 / 67.16 / 67.47 | **67.47%** | 49.1 | 185.4 | 全屏调光层开关，未满 60 |
| grid（行 5，异步解码逐格点亮） | 70.35 / 75.75 / 70.15 | **70.35%** | 58.4 | 184.8 | 20s 预热吸收解码突发；稳态 hitch 245–681/60s = 逐格热点 |
| resize（行 7） | 0.15 / 0.15 / 0.13 | **0.15%** | 0（静页） | 177.6 | 双波几何铁证（测量窗 t=3/11 两跳 1200×800→1000×700→1200×800）；页后静止；波级细采样挂 §6 遗留 |
| min（行 9，30s 窗） | 37.45 / 35.95 / 36.19 | **36.19%** | FG 段 53–61 | 183.6 | `minimize@22` 生效：t≥24 fps=0 帧休眠（对 R17 bg 零帧同港）；hidden 铁证在案；30s 聚合 CPU 为 FG22+BG8 混窗 |
| map_ms（行 16 首帧代理，进程启动→X 客户区 mapped） | — | **270–292ms** 七场中位区间 | — | — | overlay 275.3 / min 276.6 / grid 277.4 / resize 280.6 / boundary 281.6 / cp 284.8 / full 291.8 |
| list 300s soak（行 11 长跑稳态） | 72.23 / 72.38 / 72.19 | **72.23%** | 55.3 | 196.5 | 稳态斜率（t≥75s，225s 样本）348.8/535.5/541.1 → 中位 **535.5KB/min（valid，线 30000）**；RSS 平稳 194.6→196.5MB（peak 201.8）；hitch ≈12/min 为 list 周期降幅语义；首批采数（P4B）因单窗路径缺 valid 标记作废，同口径重采（P4B2）入账 |

- 与 gpui 同行窗口的比值**不在本节成账**（行 4 已携 R0 同策略对位成账于
  §5.8.5；行 5/6/8/11 的 gpui 同窗资源账挂 §6 遗留清单）——避免单边比值假绿。
- Flutter 满帧与否如实带注（full 53.2 / overlay 49.1 / boundary 55.0 未满 60），
  这些行 CPU 比值读数时必须并排写明两侧 fps。
- min 的 36.19% 是 30s 混窗聚合（FG 22.1s + BG 12.9s），不可直接对 R17 的 FG
  段 CPU；FG 段拆分挂 §6 遗留。
- grid 的 decode 段（20s 预热内）不进测量窗；稳态指示 245–681 hitch/60s 属
  逐格点亮热点的语义成本，gpui R10 对照读数时同语义逐格对齐。

### 5.8.5 行 15 呈现策略对照：R0（full_paint）/ C2（retained 集成壳）/ R4（retained 局部热点）（桌面 Xorg `:1`，60s × 3 跑取中位，2026-10-27）

三窗同为 `-auto-only` 门禁自退、wrgate JSON 出账；C2 先遇 08:34 陈旧基线红
（golden 0.0235%，row 12 字体共享涟漪），按已批准口径重播种收口（备份 + 量化 +
两连全绿）后取数；R0/R4 基线现行全绿直取。

| 窗（策略） | 三跑 CPU（%） | 中位 | fps 中位 | p95 中位 | RSS 50→（MB） | 稳态斜率中位（KB/min） | golden | 像素 | 备注 |
|---|---|---:|---:|---:|---:|---:|---|---|---|
| R0（full_paint 全树 paint） | 27.95 / 26.92 / 26.57 | **26.92%** | 59.94 | 17.17 | 50.0→289.8 | 2243.8（valid） | 0×3 | 14/14×3 | 每帧全树 paint，U16 线 ≥5s |
| C2（retained 集成壳 R3+R4+R4b+R5） | 25.22 / 24.84 / 25.22 | **25.22%** | 59.94 | 18.55 | 50.0→300.8 | 8449.8（valid） | 0 两连+复位 1 跑 | 14/14×3 | damage_sum=0.187（<0.35 线）；重播种后三连 rc=0 |
| R4（retained 局部热点层） | 23.99 / 24.28 / 24.02 | **24.02%** | 59.97 | 18.42 | 49.9→295.5 | 5166.2（valid） | 0×3 | 15/15×3 | 纯局部 HOT 层对照 |

**读数（三句）**：
1. **同壳族策略 A/B**：R0（full_paint）26.92% vs C2（retained 集成）25.22%——
   retained 在本负载省约 6.3% CPU（相对值 0.94×），且 damage_sum 0.187 ≪ 0.35
   线（真实重绘像素受控）；RSS 差 ~11MB 属内容差（C2 多 R5 回放区两块位图）。
2. **三窗全部 60fps 满拍**（59.94–59.97），呈现策略不降帧；斜率全 valid 且
   1871–9745KB/min 远低于 30000 线（R0 全树 paint 每帧触碰全资产，斜率波动
   4993/2243/1871 大属采样承载，如实带注）。
3. **R4 的 24.02% 是「局部热点 retained」地板参考**：它只重绘 5x3 静态格中的
   HOT 层，负载本来就轻，不能拿它与 R0 差值宣称「retained 省 X%」——同壳同内容
   的严格 A/B 需可切策略对照窗（C2 是集成本、非逐字同内容），挂 §6 遗留。

复现：`RUN_SECONDS=60 <bin> -auto-only`（bin = `go build -o /tmp/p4c_{r0,r4,c2}
./examples/{ui_wr_r0_fullpaint,ui_wr_r4_composite,ui_wr_c2_retained_scene}`）；
原三跑 JSON：`/tmp/opencode/ruler_runs/P4C/{r0,c2,r4}_run{1..3}.stdout`、
C2 复位三连 `P4C/c2v_run{1..3}.stdout`；旧 C2 基线备份
`/tmp/opencode/old_c2_golden_backup/`。

### 5.9 E 步：`GPUI_RULER_*` 窗口接线 + 系数定稿（2026-10-27 实测 / 2026-10-28 拍板）

**接线（D 步字段的消费端落地）**：五窗 `BuildReport` 挂 `Ruler: wrgate.RulerFromEnv()`——
`ui_ruler_parity`（live 模式 JSON）、`ui_wr_r0_fullpaint` / `ui_wr_c2_retained_scene` /
`ui_wr_r4_composite` / `ui_base_multiwin_cap`（门禁 JSON）。env 三件套
`GPUI_RULER_NAME` / `GPUI_RULER_CPU_PCT` / `GPUI_RULER_RSS_MB` 齐才生效，缺任一字段
JSON 五对齐线键保持 null（禁假绿语义，`report.go` L268–283）。

**接线实证（wire-probe）**：臂侧数值用假值注名 `*-wire-probe`，只证管道不为账：
- parity live（`RUN_SECONDS=8` 自关）：`ruler_name/ruler_cpu_pct/ruler_rss_mb` 全种、
  `ruler_ratio_cpu=0.65` / `ruler_ratio_rss=0.82` 自动成算（JSON：
  `/tmp/opencode/ruler_runs/parity_wire_live.stdout`）。注意 parity 的
  `-auto-only` 门禁路径走自家 `RULER_GATE` JSON（`runGate` 后即退），不走
  wrgate 报告——尺比值在 live 模式 JSON，两模态分工如实注明。
- 门禁窗三连 rc=0 + 比值成算 + 门禁零失败（`/tmp/opencode/ruler_runs/wire_{r0,c2,r4}.stdout`）。
- multiwin 接线实证挂 §6 遗留（隔离会话窗活，桌面不跑 8 窗）。

**系数定稿（§1.3 占位数字 → 数据定稿；P4 依实测分布，2026-10-28 用户拍板）**：

| 指标 | 定稿系数 | 当前最紧 adverse 样本（gpui 高于尺臂） | 豁免/注 |
|---|---:|---|---|
| CPU | **1.5×**（占位 1.2 紧：现账最紧恶劣样本后余量仅 ~13pp，扛不住两跑中位的 ±2pp 波动） | Godot 0.94×（gpui 高 6.4%，行 13） | ①双侧 CPU < 1% 的 idle 级行不走系数，走绝对差 Δ≤0.5pp 判红（text idle 0.02 vs 0.42 的 21× 是假信号）；②Flutter 臂未满 60fps 的行（full 53.2 / overlay 49.1 / boundary 55.0 / soak 55.3）CPU 比值必须双侧带 fps 注，不单独作对齐判据 |
| RSS | **2.0×** | 行 4 对 R0 **1.73×**（289.8 vs 167.5MB）；multiwin N1 1.43× | text 族（m5/parity text）豁免：字体位图图集进程级共享粒度与臂结构不同源，row 12 已把真实泄漏修掉（1.67GB→464MB），剩余差值是常驻图集粒度语义，写豁免注不抬系数；多窗 N≥2 的 gpui 长期大幅更省（2.80–5.57×）不改条款 |

规则落点：系数入 §1.3 与 §4 对齐线公式；判红 = `gpui值 > 尺臂值 × 系数`
（三跑中位对三跑中位）；任何豁免行必须书面挂号（本节 + §2 该行）。

## 6. 实施阶段与 DoD

| 阶段 | 内容 | DoD | 量级 |
|---|---|---|---|
| **P1 尺先立（✅ 2026-10-26）** | Flutter stable 3.47.8 构建链 + 三对照窗 + 采数；glxgears 复测；Godot 4.5.1 二进制到位 | ✅ 三对照场景基线数字入表（§2 对照列、§5.5） | 1 天 |
| **P2 矩阵冻结（✅ 2026-10-25）** | 本文档 §2 表定稿（真窗/指标/对照/状态四项逐行落实） | 每行四项齐全，缺行显式列出 | 0.5 天 |
| **P3 缺窗补齐（✅ 2026-10-27）** | idle / batch_cap / multiwin_cap 三扇 + 13 扇尾账 + E1/E2 豁免 + 对照尺（Flutter/Godot 基准 + gpui 同语义 parity 三跑） | 每扇 `-auto-only` 两连 + manual 档；逐文件回归；触及共享数据跑 `-race`；三臂真实出画；parity 四张 Golden 重出两连零差异 | 2-3 天 |
| **P4 全矩阵跑账** | 17 行逐行资源账（5s 档 + 长跑行 300s）、对照系数定稿、wrgate 对照字段落地、覆盖文档 §11 同步 | 17 行 JSON 齐全、对照比成立、文档标 ✅ | 1-2 天 |

**P3 进度（2026-10-27 更新）**：三扇新窗（idle/batch_cap/multiwin_cap）两连 gate + manual 已完成（§5.1–5.3）；多窗 `-race` 清零；13 扇尾账 Golden 两连全 PASS（§5.4，13→0）；IME 颜色字形分类引擎修复落地；两处既有红已定案（用例迁移 + 断言口径，§5.4）；E1 14 扇机器证据已补（§5.6，其中 appkit/win32 本机诚实 SKIP、待真机）。E2 已定案（jumprope 冻结 Golden 两连 PASS、spin_min 豁免）；六扇 IME 真机人工验收已完成并修掉光标/选区引擎缺陷（§5.4.1/§5.4.2）；maskops 可录制掩码后处理已收，两连 `screen=0.0000%`/`pixel=18/18`/`golden=0.0000%`/`live_only_skips=0`/`mask_skips=0`（§5.4.1、线程计划 §18.1 #19）；**Flutter stable 3.47.8 与 Godot 4.5.1 对照尺已立，三臂真实出画；Flutter/Godot 基准各三跑取中位；gpui 同语义 parity 四场景各三跑中位（spin 14.16% / list 22.99% / text 0.42% / godot 20.33%），四张 Golden 重出两连零差异（§5.5.3）**；旧 P1 黑屏阻塞解除。故 **P3 收口（2026-10-27），P4 未开工**；P4 接手全矩阵对照、系数定稿、m5/parity text RSS 异常定位。

**P4 进度（2026-10-27 更新）**：**D 步完成（wrgate 对照字段落地，§4）**——`Report` 五对齐线字段（未接尺显式 `null`）+ `RulerInput`/`RulerFromEnv` + `RequiredSchemaKeys` 纳检，单测五测绿、全仓编译过；行 1/2/3 对照沿用 P3 成果。**A 步完成**：行 10 多实例（隔离 Wayland 会话 12 轮，§5.8/§5.8.3）+ 行 4/5/6/7/8/9/13/16 七场单窗臂（桌面 Xorg 12 轮，§5.8.4）。**B 步完成**：行 11 长跑 300s×3（稳态斜率 535.5KB/min valid，§5.8.4）。**C 步完成**：行 12 修复归档、行 15 R0/C2/R4 策略对照成表（§5.8.5）、行 17 立账计划登记（§2 行 17）。**E 步完成**：五窗 `GPUI_RULER_*` 接线（multiwin 接线实证挂 §6 遗留 7）、C2/parity 陈旧金基线收口（row 12 字体共享 + `dc5248de` 壳对齐涟漪，§5.8.5/§9）、系数定稿（2026-10-28 拍板：CPU 1.5×/RSS 2.0× + 双豁免 + text 族 RSS 豁免，§5.9）。行 14 挂视频线不进本轮验收；行 17 尺臂（Godot 压力场景）后置作观察项。

**P4 遗留清单（§2 各行指向，2026-10-27 登记）**：
1. 行 5/6/8/11 的 gpui 同窗资源账带出成比值写回 §2（R15/C10 等门禁 JSON 自带 cpu/rss 的可复用既有两连记录；缺字段的窗跑 60s 对照口径补采数）；
2. 行 7 resize/DPR 双臂波级成本细采样（60s 整段聚合分不出单波成本，两臂同口径）；
3. 行 9 最小化的 FG/BG 段 CPU 拆分（Flutter 臂 36.19% 是 FG22+BG8 混窗聚合，不可直比 R17 FG 段）；
4. 行 16 遮挡恢复：Flutter 臂 WM 协议无「遮挡→恢复」事件路径，恢复臂暂缺（人工恢复路径可选）；
5. 行 15 严格同内容 full vs retained「可切策略」对照窗（现 R0/C2 为同壳族策略对照、非逐字同内容）；
6. 行 17 专项资源场景窗：预算拒增曲线（`BudgetRefusals`）+ `GPUI_FAULT_OOM` 注入后 `hal.VramLiveBytes()` 回落断言——窗未立不标绿；
7. multiwin 接线实证 + 行 10 臂 wrgate JSON 复跑（`GPUI_RULER_*` 带 env）在隔离 Wayland 会话补（桌面不跑 8 窗）。

## 7. 风险与边界

1. **Flutter 构建链**：stable 3.47.8 已在 940MX 验证（无 clang，wrapper 转 gcc/g++；Impeller OpenGLESSDF 出画，§5.5）。master 3.50 的 Impeller 黑屏仍存在，不得拿 master 当尺；升 SDK 必须重跑三窗出画与三跑采数。
2. **Godot 版本/预编译包**：固定官方 4.5.1-stable + sha256（§3.2）；demo 的 `config/features=4.7` 在 4.5.1 实测可 import，但换版本需重验；不源码编译大工程。
3. **系数防自拍条款（已按期解除）**：占位期（P4 基线出来前）系数不得当验收线用的约束已随 2026-10-28 定稿解除；此后改系数必须带新实测依据并走 §9 修订表，防新的自拍阈值；
4. **换机门**：阈值随机器重标（照 2.5D GAP_PLAN §5 先例），本矩阵数字绑定本机；
5. **范围边界**：2D 桌面底座（不含 3D、不含移动端）；CPU 口径不含合成器侧；多窗单 GL 线程天花板是观察项不是缺陷。实测：4 窗平均 60fps，8 窗 31fps，总呈现 ≈250 帧/秒；单帧 CPU 成本 2.65–3.23ms 稳定，CPU 未饱和（71.7%）——瓶颈是共享呈现串行吞吐。

## 8. 复现命令（P1 尺 + P3 实测）

```bash
# 本机无 clang：用包装脚本转 gcc/g++（新机器可直接装 clang）
export PATH=/tmp/opencode/flutter_toolchain:$PATH
export FLUTTER_SDK=/home/yanghy/app/projects/gogpu/flutter

# Flutter stable 3.47.8：构建链 + 三窗（使用 tools/ruler_flutter）
$FLUTTER_SDK/bin/flutter doctor -v
cd tools/ruler_flutter
make analyze && make build
make measure-spin DURATION=60
make measure-list DURATION=60
make measure-text DURATION=60
# 三跑取中位（原产物落 out/，已 gitignore）
for s in spin list text; do for i in 1 2 3; do
  python3 scripts/measure.py --scene "$s" --window-match "RULER_$(echo $s | tr a-z A-Z)" \
    --duration 60 --warmup $([ "$s" = list ] && echo 20 || echo 5) --out "out/${s}_$i"
done; done

# Godot 4.5.1-stable：复制 bullet_shower 副本，不改上游
export GODOT_DEMO_ROOT=/home/yanghy/app/projects/gogpu/godot-demo-projects
tools/ruler_godot/setup.sh
for i in 1 2 3; do
  GODOT_BULLETS=500 DURATION=60 WARMUP=5 \
    tools/ruler_godot/measure.sh /tmp/opencode/ruler_runs/godot_500_$i
done
# 容量探针（含逐对象物理，不能当纯图元 parity）
GODOT_BULLETS=1000 DURATION=10 WARMUP=5 tools/ruler_godot/measure.sh /tmp/opencode/ruler_runs/godot_1000
GODOT_BULLETS=5000 DURATION=10 WARMUP=5 tools/ruler_godot/measure.sh /tmp/opencode/ruler_runs/godot_5000
GODOT_BULLETS=10000 DURATION=20 WARMUP=15 tools/ruler_godot/measure.sh /tmp/opencode/ruler_runs/godot_10000

# gpui 同采数器复核 CPU/RSS/截图（示例：spin；C3/m5 换 --app 和 --window-match）
# fps/p95 用该窗自身 60s 指标 JSON，不从进程 stdout 猜
go build -o /tmp/gpui_spin ./examples/kit/spin_min
python3 tools/ruler_flutter/scripts/measure.py --app /tmp/gpui_spin \
  --env LD_LIBRARY_PATH=$PWD/lib --env WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \
  --env RUN_SECONDS=90 --window-match spin_min --duration 60 --warmup 5 \
  --out /tmp/opencode/ruler_runs/gpui_spin

# glxgears 地板（300×300）
python3 tools/ruler_flutter/scripts/measure.py --app /usr/bin/glxgears \
  --window-match glxgears --duration 60 --warmup 5 --out /tmp/opencode/ruler_runs/glxgears

# gpui 同语义对照臂（§5.5.3；三跑；单窗 WM_DELETE 标准关闭）
# 批量/长循环先切隔离显示；单窗测量必须一轮一窗并在每轮后查 Xorg。
go build -o /tmp/opencode/ui_ruler_parity ./examples/ui_ruler_parity
for s in spin list text; do
  warm=5; [ "$s" = list ] && warm=20
  for i in 1 2 3; do
    python3 tools/ruler_flutter/scripts/measure.py \
      --app /tmp/opencode/ui_ruler_parity --scene "$s" \
      --window-match 'gpui ruler parity' --duration 60 --warmup "$warm" \
      --out "/tmp/opencode/parity_runs/p3_${s}_$i"
  done
done
for i in 1 2 3; do
  python3 tools/ruler_flutter/scripts/measure.py \
    --app /tmp/opencode/ui_ruler_parity \
    --window-match 'Bullet Shower' --duration 60 --warmup 5 \
    --arg=-scene --arg=godot --env RULER_SCENE=godot --env RUN_SECONDS=80 \
    --out "/tmp/opencode/parity_runs/p3_godot_$i"
done
# Golden 重出：先把 testdata/golden_*.png 备份到仓库外；有旧文件时只比较不覆盖。
# 基线在独显（940MX）上生成；混合机默认策略优先核显，会出 ±1 LSB 差异，必须带 GPUI_POWER=high。
# seed 运行 rc=1 且 golden_seeded=true 是预期，不算 PASS；随后两连必须 pass=true。
for s in spin list text godot; do
  GPUI_POWER=high GPUI_ACCEPT_GOLDEN=1 RULER_SCENE=$s RUN_SECONDS=5 \
    /tmp/opencode/ui_ruler_parity -auto-only || true
  for i in 1 2; do
    GPUI_POWER=high RULER_SCENE=$s RUN_SECONDS=5 /tmp/opencode/ui_ruler_parity -auto-only || exit 1
  done
done

# P3 三扇新窗（gate 两连命令重复执行两次）
RUN_SECONDS=60 go run ./examples/ui_base_idle -auto-only
RUN_SECONDS=200 go run ./examples/ui_base_batch_cap -auto-only
RUN_SECONDS=75 go run ./examples/ui_base_multiwin_cap -auto-only
# manual 观察档
go run ./examples/ui_base_idle -manual-seconds 60
go run ./examples/ui_base_batch_cap -manual-seconds 200
go run ./examples/ui_base_multiwin_cap -manual-seconds 100
# 多窗竞态回归（-race 需要 CGO_ENABLED=1）
CGO_ENABLED=1 go build -race -o /tmp/multiwin_race ./examples/ui_base_multiwin_cap
/tmp/multiwin_race -manual-seconds 45
# P3 尾账 13 扇（-auto-only 每条连跑两次；单包构建，勿一次 build 多个包）
for w in ui_ant_verify ui_render_particle ui_wr_c7_resize_dpr ui_wr_c8_hit_overlay_xf \
         ui_wr_r17_bg_throttle ui_wr_r21_shell ui_wr_r4b_multidamage ui_wr_r7_virtlist \
         ui_textinput_ime ui_wr_ime_r2_textlayout ui_wr_ime_r3_channel \
         ui_wr_ime_r4_selection ui_wr_ime_r5_control; do
  go build -o /tmp/$w ./examples/$w || break
  /tmp/$w -auto-only || break
done
# IME 交互档（R2–R5：不传 RUN_SECONDS 为无限交互、点 X 关闭；15s 有界冒烟如下）
RUN_SECONDS=15 go run ./examples/ui_wr_ime_r2_textlayout
RUN_SECONDS=15 go run ./examples/ui_wr_ime_r3_channel
RUN_SECONDS=15 go run ./examples/ui_wr_ime_r4_selection
RUN_SECONDS=15 go run ./examples/ui_wr_ime_r5_control
```

# P4-A 行 10 Flutter 多实例臂（隔离 Wayland 会话里跑；桌面禁止批量多窗）
# N=1 单实例基线（单窗路径）
GDK_BACKEND=x11 python3 tools/ruler_flutter/scripts/measure.py \
  --scene spin --window-match RULER_SPIN --duration 60 --warmup 5 \
  --out /tmp/ruler/P4A_multi/multi_N1_run1
# N>=2 多实例路径（GDK/XWayland 由会话决定；RULER_SECONDS 自退，无硬杀）
GDK_BACKEND=x11 python3 tools/ruler_flutter/scripts/measure.py \
  --scene spin --window-match RULER_SPIN_ --instances 4 --duration 60 --warmup 5 \
  --out /tmp/ruler/P4A_multi/multi_N4_run1
# gpui 臂（一进程八窗门禁；下限 75s；隔离 Wayland 会话加 GPUI_RULER_BACKEND=x11）
RUN_SECONDS=90 GPUI_RULER_BACKEND=x11 go run ./examples/ui_base_multiwin_cap -auto-only
```

## 9. 修订表

| 日期 | 内容 |
|---|---|
| 2026-10-28 | **P4-E 对齐线系数定稿（2026-10-28 拍板）+ 五窗 `GPUI_RULER_*` 接线落地 + 行 15 策略对照成表（§5.8.5/§5.9）**：**系数定稿 CPU 1.5× / RSS 2.0× 入 §1.3/§4**（判红 = gpui 值 > 尺臂值 × 系数，三跑中位对三跑中位；占位 1.2×/1.5× 作废——1.2× 过最紧恶劣样本 Godot 0.94× 后余量仅 ~13pp 扛不住 ±2pp 中位波动；RSS 2.0× 罩最紧行 4 对 R0 1.73× 与 multiwin N1 1.43×）。豁免两条 + 一注：①双侧 CPU < 1% 的 idle 级行不走系数走绝对差 Δ≤0.5pp（text idle 0.02 vs 0.42 的 21× 是假信号）；②Flutter 臂未满 60fps 的行（full 53.2 / overlay 49.1 / boundary 55.0 / soak 55.3）CPU 比值必须双侧带 fps 注、不单独作对齐判据；③text 族 RSS 豁免（字体位图图集进程级共享粒度与臂结构不同源，row 12 已把真实泄漏修掉 1.67GB→464MB，剩余差值是常驻图集粒度语义）。**接线**：五窗 `BuildReport` 挂 `Ruler: wrgate.RulerFromEnv()`——parity（live 模式 JSON；其 `-auto-only` 走自家 RULER_GATE 不走 wrgate 报告，两模态分工注 §5.9）+ r0/c2/r4/multiwin（门禁 JSON）；env 三件套不齐五键保持 null 禁假绿。wire-probe 实证：parity live 五 ruler 字段全非 null、比值自动成算 0.65×/0.82×（臂值假值注名 `*-wire-probe` 只证管道）；r0/c2/r4 门禁三连 rc=0+比值成算+零失败；multiwin 接线实证挂 §6 遗留 7（隔离 Wayland 会话补，桌面不跑 8 窗）。**行 15 成表（§5.8.5）**：R0 full_paint 26.92%/59.94fps/289.8MB vs C2 retained 集成壳 25.22%/300.8MB（damage_sum 0.187≪0.35 线，省 ~6.3% CPU）、R4 局部地板 24.02%；三窗满拍、斜率全 valid 低于 30000 线；严格逐字同内容可切策略对照窗挂 §6 遗留 5。§0/§1.3/§2 行 4/15/§4/§6/§7 同步。 |
| 2026-10-28 | **P4-E C2/parity 陈旧金基线收口（六窗九文件重播种：备份+量化+两连全绿）**：row 12 字体源共享（`32a68b64`，10-27 11:22）与壳对齐（`dc5248de`，10-27 10:07）两笔合法引擎提交把 08:34 前的旧基线甩陈旧。铁证链：`evidence.go` L117 `evaluateGolden` 以 `_base.png` 对拍、旧基线 mtime 10-27 08:19/08:34（提交 `7c4885c6`）、两笔引擎提交 10:07/11:22，时间全对上。量化与处置：C2 `showcase_c2_retained_scene{,_base}.png` 旧基线 diff 内容区 0.0235%（821184px 门禁口径）/整幅 0.2595%，重播种后三连 rc=0、golden=0（`P4C/c2v_run1..3`）；parity 四张 golden + 四张 showcase 量化 spin 154px/0.1003%（图标边 AA ±1）、text 30806px/3.2090%、list 24266px/2.5277%、godot 16042px/1.6710%，每场景先播种（rc=1 为设计——seed 不算 PASS）再两连验收 rc=0、golden=0.0000%、pass=true（12 份门禁 JSON `/tmp/opencode/ruler_runs/parity_reseed.*`）。旧基线备份 `/tmp/opencode/old_c2_golden_backup/`、`/tmp/opencode/old_parity_goldens_p4e/`（/tmp 重启即失，量化数字已在此存档）。 |
| 2026-10-27 | **P4-B 行 11 长跑（300s×3）收官（§5.8.4 补 soak 行）**：Flutter list 300s 三跑 CPU 中位 72.23%、RSS 194.6→196.5MB 平稳（peak 201.8）、稳态窗口斜率中位 535.5KB/min（**valid**，稳态窗 t≥75s/225s 样本，健康线 30000）、fps 55.3、hitch≈12/min（list 周期降幅）。工程：`measure.py` 单窗路径补 `rss_slope_steady_kb_per_min` + `rss_slope_valid`/`rss_slope_scope`（与 wrgate 同语义：剔 max(25% 时长,15s) 预热爬坡、样本窗 <30s 判 invalid）——首批三跑因缺 valid 标记作废，同口径重采入账。§2 行 11 对照列更新（gpui R15/C10 资源账比值留 C 步）。 |
| 2026-10-27 | **P4-A 桌面单窗 12 轮收官（cp/resize/min/grid 各 3）+ 七场 Flutter 臂成表（§5.8.4）**：cp 69.90%/58.5fps、resize 静页双波（0.15%，两跳几何铁证）、min 36.19%（FG 53–61fps→t≥24 零帧休眠，对 R17 bg 同港）、grid 70.35%（解码突发预热吸收、稳态 hitch 逐格热点）、+ 前批 full/boundary/overlay；行 16 map_ms 七场 270–292ms。十二条标准关闭全绿、Xorg 轮间检查未变。§2 行 4/5/6/7/8/9/13/16 对照列更新（比值留 C 步带出 gpui 资源账后成账）；发现并修复 grid 素材未进 bundle（pubspec `assets/` 不递归子目录，补 `assets/grid/` 重 build，md5 同公式不动）。 |
| 2026-10-27 | **P4-A 行 10 对照成表（Flutter 多实例臂 + gpui 臂，隔离 Wayland 会话/XWayland）**：Flutter N=1/2/4/8 各 3×60s 全效（CPU 中位 27.19/62.16/131.27/176.16%，fps 61.0/60.9/59.2/41.0，RSS 162.5/326.5/653.0/1305.3MB）；gpui `ui_base_multiwin_cap -auto-only`（RUN_SECONDS=90，XWayland）门禁全绿：阶梯 CPU 22.42/43.83/81.33/111.0%、单窗摊销 13.87%（8 窗）、max60=4/maxsteady60=2、golden 逐窗 0.0000%（XWayland 上与桌面基线逐位一致）+ 像素 16/16 + RSS 斜率 valid。比值 CPU 1.21→1.61×、8 窗 RSS 5.57×、同时 8 窗总吞吐贴同一机器饱和 ~316–328fps（Flutter 8×41 / gpui 8×39.6）。环境限制如实记账：Wayland 会话机器取像素三条路不通（Shell 截图 API 白名单/gnome-screenshot 缺装/GL 窗 XGetImage BadMatch），`measure.py` 截图记 unavailable 不炸轮，行 10 像素证据转人工目击（用户确认全程在动）；桌面单窗轮口径不变。N=1 走单窗路径发现并修复单窗截图块缺回退链（此前 XWayland 三跑全废，重跑三效）。工具改动：`measure.py` 回退链、`/tmp` 批跑脚本；证据目录 `/tmp/opencode/ruler_runs/P4A_multi/`。详见 §5.8。 |
| 2026-10-27 | **P4 开工 · D 步：wrgate 对照字段落地**：`Report` 追加对齐线五字段 `ruler_name`/`ruler_cpu_pct`/`ruler_rss_mb`/`ruler_ratio_cpu`/`ruler_ratio_rss`（指针字段，未接尺时 JSON 序列化显式 `null`，「跑过尺才填、没跑如实 null」禁假绿）；新增 `RulerInput`（尺臂 id + `/proc` 口径 CPU% + `rss_end_mb`）与 `BuildInput.Ruler` 入口、`RulerFromEnv()` 读 `GPUI_RULER_*`（三跑中位由跑批脚本导出；部分设置/解析失败按「没跑」处理报 stderr，不造假比值）；五键进 `RequiredSchemaKeys` 硬检查。比值=窗值/尺值，同口径配对（窗 `cpu_pct_avg`↔尺 `cpu_pct`；窗 `rss_end_kb`↔尺 `rss_end_mb`），任一侧未量到该比值保持 null。证据：`examples/wrgate/report_test.go` 五个新单测（未跑 null+schema 过 / 填充+比值算术 / 零侧不造假 / env 三态）、包 `-count=1` 绿、全仓编译仅 `ui_wr_t_stress` 已知存量红（§线程计划 §18.2 #12）。矩阵 §4 同步。下一步 P4-A：逐行按窗实跑接数。 |
| 2026-10-27 | **画笔几何跟随 CTM 收口（上一条登记缺口修复，已验证待接线）**：路径先烘进设备空间、画笔却按用户坐标取样，平移/缩放/旋转画布后渐变与图像图案钉在原处（建窗期实测 37200px 差）。修法对齐 Skia 把 CTM 组进 shader local matrix：新增 `render/brush_device_space.go`，填充与 GPU 描边入口把画笔重述到同一设备空间——相似变换（`Matrix.SimilarityScale` 判定）直接折算渐变几何（线性端点；径向中心/焦点+半径乘比例；扫掠中心+角度加旋转，镜像因方向反转保留逆取样），非相似变换（非等比缩放/错切）用逆矩阵逐点取样（`deviceSpaceBrush`，精确但走通用 `ColorAt` 场）；`ImagePattern` 统一为绘制空间逆变换并移除 `DrawImageEx` 里的 `totalMatrix` 重复组合；GPU 字段缓存新增 `render.BrushFieldSeed`，修掉 `opacityMulBrush`/`deviceSpaceBrush` 共用常量种子串缓存。证据：CPU `render/gradient_ctm_test.go`（平移/旋转/等比/非等比 oracle/设备缩放/状态还原/描边/图案/判定/种子）、GPU 离屏 `render/gradient_ctm_gpu_test.go`（`GPUOps>0`、`CPUFallbackOps==0`）、录制回放 `render/recording/gradient_ctm_test.go`（环境平移回放 ≡ 活画布、录制内平移、折叠保 CTM），逐文件回归全绿。`examples/ui_wr_shadermask` 直录列改回 cell-local 录制 + `Translate(cell origin)` 回放并编译通过；真窗门禁待隔离显示复跑（当日两次 Xorg/NVIDIA `nvidiaUnlock` 崩溃，未在当前桌面执行）。状态=**已验证待接线**。 |
| 2026-10-27 | **用户空间 ShaderMask 补齐 + 真窗 `ui_wr_shadermask`（已验证待接线）**：新增 `render.DrawShaderMask`（`PushLayerIsolated` → bounds 裁剪 → 内容 → `ClearPath` → 掩码 brush + `BlendDestinationIn` 矩形 → 还原）与 `PaintContext.DrawShaderMask`（`SaveLayerBudget/Stats` 记账，`saveLayerDepth` 不参与）；`Canvas` 补 `SetBlendMode/BlendMode`，四条填充/描边命令与 `Backend` 逐命令携带 `Blend`（`CanvasBackend` 逐调用设/还原），GPU 只对「固定混合 + 轴对齐 rect」直出（渐变 span 走 `QueueImageDrawBlend`，无混合变体的 native stage 补 `BlendNormal` 守卫），层内 CPU 回退前 `materializeLayerForCPU` 先材料化层内 GPU 内容，F1 组对非 Normal/Copy 跳过组 alpha 乘法。证据：`ui_wr_shadermask` 满窗四类掩码 × 左 retained / 右直录两列，两连 `pixel=18/18`、`pairs=9/9`、`golden=0.0000%`（912000px）、`screen=0.0000%`、`cpu_fallback_ops=0`、`mask_skips=0`、`live_only_skips=0`、`savelayer_allow=2400+`，presents=300/301，GPUI_POWER=high（独显）重播种；回归 `ui_render_base_maskops`（设备空间通道不变，默认集显 golden 0.0000%）与 `ui_render_base_recording`（Backend 签名原子切换，golden 0.0000%/逻辑 19/19/像素 17/17）。**登记缺口（同日晚间已修，见本表下一行）**：渐变画笔几何不随 CTM 变换（平移画布后渐变按绝对坐标取样；临时活路径/回放对拍实测 37200px 差，直录列曾改录绝对坐标规避）。状态=**已验证待接线**（ui/kit、2.5D 未调用），非生产接线。 |
| 2026-10-27 | **row 12 m5 RSS 暴涨定位 + 引擎级修复（字体源按文件共享）**：根因=缺字回退只按 `(rune,size)` 缓存 face，75 次整份字体加载只覆盖 4 个文件（0.8s 内），存活堆平台 805MB、RSS 1.09→1.63/1.73GB（GOGC=100 两倍堆目标）。修复：新增 `render/text/source_cache.go`（LRU 32 条/256MB + singleflight，键=(绝对路径,TTC index,解析器)），`NewFontSourceFromFile` 统一走缓存（调用方零改动），缓存源 `Close` no-op + `PurgeFontSourceCache()`，共享源支持多家族（CAS 家族集合），回退 face 改键 `(路径,字号)`。改后 60s：加载 75→10（各文件一次、54.5MB）、存活堆 144MB、RSS 464MB 稳态、CPU 23.0%、fallback 0、pixel 11/11；藏文随 run 合簇修正。golden：旧基线（10-25）已陈旧（干净 HEAD 亦 3.1822%），2026-10-27 用当前代码在独显重出基线，`GPUI_POWER=high -auto-only` 两连 **0.0000%（821184px）**。单测：`source_cache_test.go`/`fontscan_fallback_share_test.go`/`family_multi_test.go` + `source_test.go` Close 用例，`-race` 绿；`zz_m7_fcmatch/system_font/family/multi/shaper_hb_race/shaper_own/shape_result_cache/g1a_soak` 逐文件 `-race` 绿；`BenchmarkKeystroke` 改前改后同量级。详见 row 12 与 §5.5。 |
| 2026-10-27 | **P3 gpui 同语义臂三跑 + 四张 Golden 重出**：`examples/ui_ruler_parity` 四场景各 3×60s（spin 14.16% / list 22.99% / text 0.42% / godot 20.33% CPU 中位；RSS 94.5→94.8 / 301.2→309.7 / 627.5→628.5 / 90.7→91.4MB），全部 `close_request="wm_delete"`、`exit_during_run=false`，宿主 Xorg PID 4612 与日志未变；四张 parity Golden 重出后各两连 `golden_diff_pct=0`（spin 153600px、list/text 873600px、godot 960000px）。`measure.py` 改为 WM_DELETE 主关闭、SIGTERM 兜底、硬杀仅隔离显示 opt-in；`PipelineApp.Close/parkNativeSurface` 在销毁 surface/swapchain 前 `WaitIdle`；AGENTS 规则细化“批量/长循环隔离、单窗标准关闭 + 每轮健康检查”。同语义比值见 §5.5.3，P3 收口日期更新为 2026-10-27。 |
| 2026-10-26 | **P3 对照尺落地 + P3 收口**：Flutter stable 3.47.8（engine `cf636b1dfb` / Dart 3.13.7）在 940MX 实际出画（Impeller `OpenGLESSDF`，三窗截图非黑）；三窗 3×60s 中位 spin **25.41%**、list **71.38%**、text **0.02%**（首帧后静态 idle）；Godot 4.5.1-stable `bullet_shower` 500 发 **19.1%/60fps**，1k 28.94%，5k/10k **102.6%/6–7fps**、**102.66%/3fps**（含逐对象物理，容量探针）；glxgears **2.74%**。旧 §5.5（26.28/31.03%、109.68%、0.00%）与旧 §3.1 黑屏记录矛盾，整节作废。m5 复核 RSS **1.09→1.63GB/60s**（独立 35s 1.09→1.73GB），异常增长归 row 12/P4。P3 收口，P4 未开工。详见 §5.5。 |
| 2026-10-26 | **maskops 可录制掩码后处理收口（活窗红→绿）**：`ui_render_base_maskops` 常驻真窗复核曾 `screen diff=63.2419%`、四卡不显示（`liveOnlySkips=2872`/360 presents），快照/golden/像素全绿属假绿；根因=掩码后处理只挂 `RequireLiveContext()`，retained 全帧录制通道（Recorder）拒绝后整卡丢失。修法：新增 `render.MaskCanvas` 侧接口（不塞进 `Canvas` 用户空间契约）+ `render/recording` 5 条设备空间命令/`MaskBackend`；录制时 `Mask.Clone()` 快照、回放每次再克隆；`CanvasBackend` 仅在同尺寸目标上执行，尺寸/能力不符计 `MaskSkips`，`PushMaskLayer` 退化普通层保 `PopLayer` 平衡，`End` 清理并给活 Context 重装外层掩码；`PaintContext.MaskCanvas()` 为正口，不支持计 `LiveOnlySkips`。证据：`render/recording/mask_test.go` 六测 + `-race` 绿；maskops `-auto-only` 两连 `screen=0.0000%`（627200px，±2/255）、`pixel=18/18`、`golden=0.0000%`、`live_only_skips=0`、`mask_skips=0`、presents=247/229；5.5s 档 fps 44.6/41.4、frame_raster_ms 132/141、hitch 0/11，`fps_gate=off_correctness` 只记账不判绿。边界：只打通 full_paint 全帧录制；retained 图层/局部录制遇掩码按尺寸不匹配计数跳过，完整用户空间掩码留后续。详见线程计划 §18.1 #19、`ENGINE_UI_RENDER_BASE.md` A 组#6/§24 1.63、`RENDER_API_CATALOG.md` 掩码族/§6.3/§8。P3 只剩对照尺阻塞 |
| 2026-10-26 | **六扇 IME 真机人工验收 + 光标/选区引擎修复**：六窗常驻人工档逐扇走真实输入——r2 81.7s/59.8fps/hitch 2/fallback 0/RSS 7350KB/min；r3 33.6s/59.7fps/hitch 0/RSS 48862KB；r4 106.4s/59.8fps/10探针/拖选 2621px/双击 1568px/跨行 4968px；r5 51.0s/59.8fps/两段选区 1956+2780px；`ui_textinput_ime` 逐字输入 + 光标 24px 呼吸。发现并修复通用层缺陷：祖先 mask/difference 裁剪被提升进离屏录制通道（整面 mask 对录制局部坐标采样恒 0），光标/选区纯色块被静默丢弃——`maskClipPromotionAllowed()` 闸住 `forceCPUClip` + `applyMaskToPaint` 子通道内返回，新增 `TestOffscreenPassSkipsAncestorClipMask` 护栏；r1 复验光标 38px、选区 4056px。门禁盲区：auto-only 快照是 live-context 合成重绘，走不到录制通道，此类洞只能靠人工档抓。登记跨线红：maskops 活窗 `diff=63.2419%`、四卡不显示（liveOnlySkips=2872，基线同值），归属 S2b/C3 可录制掩码后处理。详见 §5.4.1/§5.4.2。P3 仍因对照尺与跨线红未收口 |
| 2026-10-26 | **E2 定案**：`ui_render_jumprope` 补冻结相位门禁——`-auto-only` 冻结 `sim.freezeAt(0.25)`（绳在手线前层、`simT=0`），LiveHUD 底带排除，零容差 Golden 883200px + 5 条像素探针；两连 PASS（presents 600/599、58.4/58.2fps、hitch 0、pixel 5/5、golden 0.0000%）。`kit/spin_min` 书面豁免 Golden（持续旋转是心跳税语义，冻结即改变被测目标）；60s 档 presents=3592、CPU 15.92%、稳态 RSS 511.9KB/min（valid）、fallback 0。顺带修证据链：spin_min 每帧 `proc.Sample()`（原先稳态斜率永远 invalid）+ 补 JSON 输出 + README 对齐。详见 §5.7 |
| 2026-10-26 | **E1 14 扇机器证据补齐（豁免已签收）**：按"先补机器证据再豁免"逐扇实跑——l1_blank presents=180/hitch 0；l1_scroll 5s presents=301/bind 17/1000，40s 稳态 RSS 575.6KB/min（valid）；l1_spinner presents=301/layout=1/damage=144px；l2_shell 静态 presents=2 + 交互单测；perfsoak presents=302/fallback 0；m4_virtual 首屏 5.71ms/物化 42 行/真窗 bind 45/1e6；r12 schema 42 键全；pf_x11 19/19、pf_wayland 19/19（backend=wayland 嵌套，协议禁止项诚实 unsupported）；pf_appkit/pf_win32 本机诚实 SKIP（待 macOS/Windows 真机，不标绿）；video_player 出帧 27/fallback 0；zz_soak_control 3 窗 ~56–57fps；ime_probe IME 可用 + 合成按键全到。残留：真机跨平台、真人 IME 合成、真人交互。详见 §5.6。用户当日签收豁免：14 扇不补 Golden。P3 仍因 E2/人工目视/对照尺未收口 |
| 2026-10-26 | **既有红定案落地**：`ui/scene` 两例根因是 GPU 像素契约放在无 GPU 测试包（`CreateOffscreenTexture` 不可用时录制按设计降级向量回放），已迁到 `ui/scene/internal/gpupixel/textured_transform_gpupixel_test.go`，并在 `ui/scene/textured_transform_internal_test.go` 补无 GPU 缓存决策断言（未变形 bounds 42×42 / 12×12、干净帧零回放）；opacity 断言改为预乘 unorm8 的 `#ff7f7f`（`setPremul` 截断 127.5，8-bit GPU 同值），不动像素管线。本机 `ui/scene` 全包 + opacity 用例 + gpupixel 包全绿，GPU 像素档待真机复跑。P3 仍因 E1/E2、人工目视与对照尺未收口 |
| 2026-10-26 | **P3 尾账 Golden 清零（13 → 0）**：13 扇逐扇补「固定冻结 + 零容差 Golden + 像素/逻辑证据」，`-auto-only` 两连全 PASS——ant presents=301/golden 0；particle 58.1–58.2fps/p95 19.2–19.4ms/CPU 25.0–25.7%/pixel 4/4；c7 wave1=1/wave2=19/dpr=1.0/pixel 6/6；c8 scripted 11/11/excess=0/pixel 5/5；r17 fg=1308–1310/bg=1/pixel 3/3；r21 rr_scroll=0/skip=296/pixel 6/6；r4b damage_multi/fps 59.9–60.0/pixel 3/3；r7 presents=3589–3591/fps 59.9/p95 19.8ms/hitch 1.0–2.0/min/RSS 3458/4435KB/min/CPU 18.27%；textinput 5/5（270000px）；r2 4/4；r3 5/5；r4 5/5；r5 6/6。全部 golden 0.0000%。行 3（R7）标绿、行 9（R17）补齐。详见 §5.4 |
| 2026-10-26 | **R7 裁剪/掩码复用 + idle 调度修复**：滚动帧原先每个 scissor 组重建全幅 mask，改为共享 mask path 缓存（FNV-1a 键，`render/internal/clip/mask_cache.go`）+ 锁外缓存 Gray8 视图 + 跨组复用已细分裁剪几何（`render/internal/gpu/depth_clip.go`、`brush_fill.go`）；R7 60s close-run 达标（fps 59.9、CPU 18.27%，行 3）。idle：warm-up 后仅 `sched.Tickers().MayWantFrames()` 为真才排帧，静态窗呈现数归 1（行 1）。 |
| 2026-10-26 | **IME 引擎修复 + 门语义对齐**：颜色字形分类从「Unicode 区段」改为「字体说算」（`text.GlyphsHaveColor` 逐字形读 `GlyphType`，Skia 口径），`buildShapedCarets` 闸住 `isColorText`；R2–R5 `cpu_fallback_ops` 1220 → 0，✓/✗ 恢复可见，回归 `TestColorRunOutlineDingbat_M5` 绿。六扇 IME 窗证据改为真源 §10.4.x G「必采」口径（首帧绘制 + hit/miss>0），撤掉 R9 专用 `MinMeasureCacheHit:1`；auto-only 短档斜率/启动 hitch 仅 WARN，≥10s 人工档硬门不变。观察项登记：R17 最小化闩后 tick 不可见、C8 恢复快照按 elapsed 而非呈现确认。详见 §5.4 |
| 2026-10-26 | **batch_cap 两段预热屏障**：60s RSS 窗口原先落在 GPU/heap 爬坡段（预热前 ~45MB/min、一轮预热后 24–29MB/min 贴线），改为 gate 先跑 2 轮不计入测量的 prewarm 再测，稳态落到 6–8MB/min；两连 gate + 200s manual PASS（§5.2） |
| 2026-10-25 | **P3 三扇新窗落地**：idle 两连 gate + 60s manual（presents=1 / after_warmup=0 / CPU 0.807% / golden 0）；batch_cap 图元 10k@60fps、矢量 100@60fps、RSS 稳态 6.7/8.0MB/min、golden 0；multiwin 1/2/4 窗 60fps、8 窗 31fps/≈250 presents/s、平均上限 4 窗、8 张 golden 全 0、pixel 16/16。详见 §5.1–5.3 |
| 2026-10-25 | **多窗 -race 修复**：`ui/scene/layer.go` 全局 `layerIDGen` 非原子，8 窗并发建层触发 59 条同根数据竞争；改 `atomic.Uint64`（唯一/单调/非零语义不变）。修复后 45s `-race` 0 条，两连 gate 复跑 PASS |
| 2026-10-25 | **P3 回归既有红登记**：`ui/scene` textured transform 两例（`ReplayedOps=1 want 0`）、`ui/rendering` opacity 1 级舍入（`#ff7f7f` vs `#ff8080`）；已在 HEAD 复现，非 P3 引入，归引擎线。P3 尾账 13 扇 Golden + E1/E2 未完成，**P3 未收口** |
| 2026-10-25 | **P1 阻塞登记：Flutter master 3.50 Impeller OpenGLESDF 黑屏**。Flutter 对照尺三模式（spin/list/text）窗口打开但**内容区全黑**（x11grab 截屏证实），26-31% CPU 为引擎空转——像素未到达屏幕。本机 NVIDIA GeForce 940MX（非 Intel），NVIDIA 580.178.04（OpenGL 4.6），Impeller OpenGLESDF 后端与 NVIDIA 专有 EGL 不兼容。三种绕行均失败：`FLUTTER_ENGINE_SWITCHES=--disable-impeller`（编译期决定）、`LIBGL_ALWAYS_SOFTWARE=1`（EGL 上下文独立于桌面 GL）、`CMAKE_CXX_COMPILER=g++`（Impeller 是引擎编译期选项）。`flutter channel stable` 切换卡在网络（GitHub 不可达）。**Flutter 对照数字无效，P1 Flutter 部分搁置**，等 stable SDK 到位后重测。gpui 自身数字（spin_min 15.46%、C3 31.98%、m5 29.20%、glxgears 3.30%）有效已入 §5.5。用户决策三选一：①配镜像切 stable ②git checkout 可用 tag ③先跳 Flutter 尺推进 P2/P3 |
| 2026-10-25 | 重审修正（用户复核要求）：行 13 拆双模式（图元批处理+矢量路径密集）；新增行 17 显存压力/设备恢复（注入缝+预算拒增已在源码，缺资源场景窗）；§3.4 补公开参考数字锚（sanity 非验收线）。矩阵 16→**17 行** |
| 2026-10-25 | 立项：底座资源场景矩阵 16 行封闭 + 三层对照尺（Flutter 主尺/Godot 第二尺/glxgears 地板尺）+ 指标两段制（健康线+对齐线，系数数据后置）+ 三扇新窗规格（idle/batch_cap/multiwin_cap）+ 尾账 13 扇 Golden 与 E1/E2 豁免。决策记录：Godot 允许下载/编译、Flutter 允许构建/取产物、系数后置（用户定）。状态：P1 未开工 |
