# 底座资源场景矩阵（RENDER_BASE_RESOURCE_MATRIX）

日期：2026-10-25 ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui` ｜ 分支：`feat/框架代码整理`
状态：**计划文档（P1 未开工）**。范围：支撑三栋楼（任意图形渲染 / UI 组件库 / 2.5D 引擎）的**共用底座能力**的资源场景真窗覆盖，指标对齐**工业级产品同机实测**（Flutter/Godot 对照尺）。上层业务场景（组件交互全集、引擎玩法全集、视频业务）归各自域真源，本矩阵只管底座。

## 0. 一句话与做成算完口径

把底座能力封闭成 **17 行场景矩阵**（能力 × 触发模式 × 资源维度），每行做到：

> **真窗 + 资源指标 JSON（CPU/RSS/显存/fps/p95/hitch）+ 同机工业级对照数字 + 比值**——四项齐全、逐行可审计、无一行无数据或无书面豁免。

指标分两段：**健康线**（现有自定阈值，防退化）+ **对齐线**（新增，≤ 同机工业级对照 × 系数，系数数据后置定稿）。"通用/对齐"的宣布权在这张矩阵的量门数字上。

## 1. 决策记录（2026-10-25，用户已定）

1. **Godot 二进制**：允许下载官方预编译或从本机源码编译（`/home/yanghy/app/projects/gogpu/godot`；GAP_PLAN 的"只读不编译"指不改其源码做参考，编译出可执行体做对照尺不违反该约束，编译产物只用于测量）；
2. **Flutter**：允许本机构建（SDK 已在 `/home/yanghy/app/projects/gogpu/flutter`，`bin/flutter` 已验存在）或取现成产物；
3. **对照系数**：**数据后置**——先跑出基线再定稿系数（建议初值 CPU 1.2×、RSS 1.5×，仅作占位，禁直接当验收线）。

## 2. 场景矩阵（17 行封闭集，本表冻结；新增场景走 §7 增量登记）

| # | 底座场景 | 触发模式 | 真窗现状 | 资源指标现状 | 对照场景（尺） | 缺口动作 |
|---|---|---|---|---|---|---|
| 1 | 空闲（无脏零帧） | 开窗静止 60s | 按需驱动窗 | ❌ 无专项 idle 断言 | Flutter 静止页 | **改量门**（presents≤2、CPU≈0） |
| 2 | 常驻最小动画（心跳税地板） | 单 64px 旋转 60fps | **spin_min** | ✅ CPU 有账（现态 14.59% / 终态 3.4~3.5%） | Flutter 同尺寸旋转图标 | 补对照 |
| 3 | 滚动（retained 主场景） | 列表连续滚动 | C3/C9/R7/R7b | ⚠️ 字段有、未专项立账 | Flutter ListView 同规模 | 立账+对照 |
| 4 | 全幅重建上限 | full_paint 全窗每帧变 | R0 | ✅ | Flutter 全幅动画页 | 补对照 |
| 5 | 异步资源→局部脏 | 解码完成逐格点亮 | R10/C3 | ✅ | Flutter 异步图片列表 | 补对照 |
| 6 | 缓存命中/失效 | 边界/纹理命中与失效 | R3/R3b/R9/C9 | ✅ | Flutter RepaintBoundary 开关 | 补对照 |
| 7 | resize/DPR 失效波 | 改尺寸/DPR | R11/C7 | ✅ | Flutter 窗口 resize | 补对照 |
| 8 | 浮层/覆盖带脏 | overlay 开关 | R8 | ✅ | Flutter Overlay/Dialog | 补对照 |
| 9 | 后台/前台节流 | 遮挡/最小化 | R17 | ⚠️ 证据尾账（仅像素） | — | 补 Golden + 资源账 |
| 10 | 多窗共享上限 | 1/2/4/8 窗同动画 | T 压力窗 | ❌ 无 CPU-窗数曲线 | Flutter 多实例 | **新窗 `ui_base_multiwin_cap`** |
| 11 | 长跑稳态（泄漏） | 300s soak | R15/C10 | ✅ 最完整 | Flutter 300s 同内容 | 补对照 |
| 12 | 文本密集（整形+图集） | 密集多脚本文本页 | m5/文本族 | ⚠️ CPU 未专项立账 | Flutter 密集文本页 | 立账+对照 |
| 13 | 大量图元/矢量密集上限（2.5D 底座，**双模式**） | ①同屏图元阶梯 1k/5k/10k（atlas 精灵）；②矢量路径密集（设计工具类，复杂路径重分格成本） | engine 精灵/粒子窗；P1 矢量密度测试 | ❌ 无"同屏多少还 60 帧"量门、无路径密集 CPU 账 | Godot 2d demo（bullet_shower 类）/ Flutter CustomPaint | **新窗 `ui_base_batch_cap`（双模式）** |
| 14 | 视频/纹理流 | 持续喂帧 | video C4 | ❌ 解码吞吐（V-5 挂视频线） | ffplay/Flutter video | 挂视频线 |
| 15 | 呈现策略对照 | 同场景 full vs retained | 分散各窗 | ⚠️ 未成表 | — | 成表（R0 vs R4 同内容 A/B） |
| 16 | 首帧/恢复（t2fp、遮挡恢复） | 冷启/遮挡回来 | R16/R17 | ✅ | Flutter 首帧 | 补对照 |
| 17 | 显存压力/设备恢复（资源纪律验收） | 多窗 VRAM 预算压力、`GPUI_FAULT_OOM` 注入 | 故障注入缝在（`ui/embedder/fault_oom.go`）、预算拒增计数在（textured `budgetRefusals`） | ❌ 无专项资源场景窗（恢复正确性有注入测试、资源账无） | Godot 压力场景 | **立账**（预算拒增曲线 + OOM 恢复后 VramLiveBytes 回落断言） |

尾账（不属于矩阵行，属证据域）：13 扇仅像素补 Golden（`ui_ant_verify`、`ui_render_particle`、`ui_textinput_ime`、`ui_wr_c7_resize_dpr`、`ui_wr_c8_hit_overlay_xf`、`ui_wr_ime_r2/r3/r4/r5_control`、`ui_wr_r17_bg_throttle`、`ui_wr_r21_shell`、`ui_wr_r4b_multidamage`、`ui_wr_r7_virtlist`）+ E1（14 扇）/E2（2 扇）豁免书面定案。

## 3. 工业级对照尺（三层）

### 3.1 Flutter 尺（主尺，P1 先立）

- SDK：`/home/yanghy/app/projects/gogpu/flutter`（`bin/flutter` 已验存在）。允许本机构建或取现成产物（§1.2）。
- 对照窗三个，对位本矩阵行 2/3/12：**旋转图标**（480×320，同 spin_min 内容语义）、**滚动列表**（1200×800，列表项规模对齐 C3）、**密集文本页**（多脚本文本量对齐 m5）。
- 前置验证：`flutter config --enable-linux-desktop` + `flutter doctor`；linux-desktop 构建链（GTK3/ninja/clang）本机未验证，P1 第一步跑通；跑不通按 §7.1 降级。
- 测量：与 gpui 窗同口径（见 §4）。

### 3.2 Godot 尺（第二尺，用户已授权下载或编译）

- demo 现成：`/home/yanghy/app/projects/gogpu/godot-demo-projects/2d/`（bullet_shower、dynamic_tilemap_layers 等）。
- 二进制：下载官方预编译（首选，版本对齐 demo 要求）或源码编译本机 godot 检出。
- 定位：行 13（图元上限）与 2.5D 底座的主对照。

### 3.3 glxgears 尺（地板尺，已在用）

- 空转地板 2.3~3.0%（本机实测）；任何 60fps 呈现的 CPU 下限参照。

### 3.4 公开参考数字锚（第二重 sanity，非验收线）

同机两臂可能同错（系统性偏差），所以对齐线之外引入公开基准作旁证：Flutter 官方性能文档的典型区间（如简单动画页 CPU 个位数百分比、复杂滚动 10-20%）、Godot 2D demo 公开表现。**只作 sanity 参考**——验收线仍以同机对照为准，公开数字入矩阵文档备注列。

### 3.5 测量口径（三尺统一，公平性硬约束）

同机、同窗客户区尺寸、同帧率目标（60）、同动画语义（内容变化量对齐）、同测量法：进程 `/proc` CPU%（含内核态，**不含 Xorg/合成器侧**）+ RSS + `fps_interval`/`p95`/`hitch` + 60s 或 300s 档；窗内容语义差异在报告注明；每次对照三跑取中位。

## 4. 指标规范

1. **健康线（保留现状，防退化）**：fps_interval≥55、interval_p95≤22ms、hitch≤5/min、RSS 稳态斜率 ≤30000KB/min（valid=true）、CPU≤85%。
2. **对齐线（新增）**：行指标 ≤ 同机对照 × 系数。`wrgate` 报文追加字段：`ruler_name`、`ruler_cpu_pct`、`ruler_rss_mb`、`ruler_ratio_cpu`、`ruler_ratio_rss`——**跑过尺才填，没跑如实 null（禁假绿）**。系数 §1.3 数据后置。
3. 现有各域真源 §5 阈值矩阵不动；本矩阵只在资源轴加对齐线。

## 5. 新建窗规格（3 扇，模式 2：gate + manual 二合一 + 三证据）

### 5.1 `ui_base_idle`（矩阵行 1）

- 内容：静止静态页 60s；量门：warmup 后 `presents` 增量 ≤2、进程 CPU ≈0（对照 Flutter 静止页）、wake 次数；JSON 报 idle_presents/cpu。

### 5.2 `ui_base_batch_cap`（矩阵行 13）

- 内容：`DrawAtlasEx` 同屏图元阶梯 1k/5k/10k，每阶 10s；量门：每阶 fps/p95/CPU 曲线，输出"60fps 最大图元数 N*"；对照 Godot 同规模精灵场景。

### 5.3 `ui_base_multiwin_cap`（矩阵行 10）

- 内容：1/2/4/8 窗同动画（每窗同 spin 内容），单进程；量门：CPU-窗数曲线（线性度）、单窗摊薄成本、上限窗数；对照 Flutter 多实例。

### 5.4 尾账

- 13 扇 Golden（清单 §2 尾账）各两连 0.0000%；E1/E2 豁免书面裁决入审计文档。

## 5.5 P1 基线数字（2026-10-25 本机实测，对照尺已立）

**对照尺（Flutter release AOT，同机同尺寸同语义）**：
| 场景 | Flutter | gpui | 比值 |
|---|---|---|---|
| 旋转图标 480×320 | 26.28%/31.03%（两次），RSS 146→151MB | 15.46%（同口径 /proc），自报 15.77%，RSS 135MB | **gpui ≈ 0.5×**（优） |
| 滚动列表 1200×800 | **109.68%**（>1 核！），RSS 184→287MB | C3: 31.98%，RSS 387MB | **gpui ≈ 0.3×**（显著优） |
| 密集文本 1200×800 | 0.00%（首帧后 idle，Flutter 按需模型静态不渲染） | m5: 29.20%（retained+damage 9.25%，HUD 60fps tick 税） | **Flutter ≈ 0**（此项 Flutter 优——它的静态页真 idle） |
| 地板 glxgears 60fps | — | 3.30%，RSS 55MB | — |

**分析**：① 旋转/滚动两场景 gpui 已显著优于 Flutter（滚动的差距 3.4× 尤其大——Flutter GTK embedder 的 ListView 逐帧全量重排是已知开销）；② 密集文本 Flutter 0% 是其按需模型的正确行为——gpui m5 29.2% 反映的是 retained+HUD tick 的每帧成本，不是文本渲染本身的成本（文本整形/图集已缓存），降低它要走 R15/C10 的 further-tuning 方向；③ 地板 3.3% 与文档口径一致。

**P1 工程细节**（构建链验证结论）：
- Flutter linux-desktop 构建链缺口 = cmake+ninja（无 sudo）；`pip3 install --user cmake ninja` 解决；CXX 编译器注入 flutter 工具链用 `CMAKE_CXX_COMPILER=g++` 预播 build 目录缓存绕过；
- Flutter release AOT 构建 ✓（kernel + engine + runner 三阶段全通）；
- 采样器 `/tmp/ruler_flutter/sample.sh`（/proc utime+stime 差分、3s 预热排除、RSS>50MB 校验、kill 双发 SIGTERM+SIGKILL）；
- **陷阱**：`cd "$DIR" && ./$EXE &` 的 `$!` 是 bash 中转壳（wchan=do_wait、RSS 2MB、CPU 永远 0%）——exe 必须单独一行后台，`$!` 才是应用本体；
- m5 golden 漂移 0.1036% = #2 CPU 位图整形重排（同 textlow 归因），已重播种两连 0.0000%。

## 6. 实施阶段与 DoD

| 阶段 | 内容 | DoD | 量级 |
|---|---|---|---|
| **P1 尺先立** | Flutter 构建链验证 + 三对照窗 + 采数；glxgears 复测；Godot 二进制到位（下载或编译） | 三对照场景基线数字入表（§2 对照列） | 1 天 |
| **P2 矩阵冻结** | 本文档 §2 表定稿（真窗/指标/对照/状态四项逐行落实） | 每行四项齐全，缺行显式列出 | 0.5 天 |
| **P3 缺窗补齐** | idle / batch_cap / multiwin_cap 三扇 + 13 扇尾账 + E1/E2 豁免 | 每扇 `-auto-only` 两连 + manual 档；逐文件回归；触及共享数据跑 `-race` | 2-3 天 |
| **P4 全矩阵跑账** | 16 行逐行资源账（5s 档 + 长跑行 300s）、对照系数定稿、wrgate 对照字段落地、覆盖文档 §11 同步 | 16 行 JSON 齐全、对照比成立、文档标 ✅ | 1-2 天 |

## 7. 风险与边界

1. **Flutter linux-desktop 构建链**未验证（GTK3/ninja/clang）：P1 第一步验证；跑不通降级为"引用公开基准数字 + 注明"，Godot 尺升主尺；
2. **Godot 编译耗时**：源码编译大工程优先下载官方预编译；
3. **系数后置**：对齐线系数在 P4 基线出来前不得当验收线用（防新的自拍阈值）；
4. **换机门**：阈值随机器重标（照 2.5D GAP_PLAN §5 先例），本矩阵数字绑定本机；
5. **范围边界**：2D 桌面底座（不含 3D、不含移动端）；CPU 口径不含合成器侧；多窗单 GL 线程天花板是观察项不是缺陷。

## 8. 复现命令（P1）

```bash
# Flutter 构建链验证
/home/yanghy/app/projects/gogpu/flutter/bin/flutter config --enable-linux-desktop
/home/yanghy/app/projects/gogpu/flutter/bin/flutter doctor
# Flutter 对照窗（tools/ruler_flutter/ 三窗，P1 建）
cd tools/ruler_flutter && FLUTTER_SDK=/home/yanghy/app/projects/gogpu/flutter make run-spin
# gpui 侧同场景（对照臂）
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 examples/kit/spin_min   # proc CPU% / RSS / p95 / hitch
# glxgears 地板
glxgears -info 2>&1 | grep -E "FPS"
```

## 9. 修订表

| 日期 | 内容 |
|---|---|
| 2026-10-25 | **P1 阻塞登记：Flutter master 3.50 Impeller OpenGLESDF 黑屏**。Flutter 对照尺三模式（spin/list/text）窗口打开但**内容区全黑**（x11grab 截屏证实），26-31% CPU 为引擎空转——像素未到达屏幕。本机 NVIDIA GeForce 940MX（非 Intel），NVIDIA 580.178.04（OpenGL 4.6），Impeller OpenGLESDF 后端与 NVIDIA 专有 EGL 不兼容。三种绕行均失败：`FLUTTER_ENGINE_SWITCHES=--disable-impeller`（编译期决定）、`LIBGL_ALWAYS_SOFTWARE=1`（EGL 上下文独立于桌面 GL）、`CMAKE_CXX_COMPILER=g++`（Impeller 是引擎编译期选项）。`flutter channel stable` 切换卡在网络（GitHub 不可达）。**Flutter 对照数字无效，P1 Flutter 部分搁置**，等 stable SDK 到位后重测。gpui 自身数字（spin_min 15.46%、C3 31.98%、m5 29.20%、glxgears 3.30%）有效已入 §5.5。用户决策三选一：①配镜像切 stable ②git checkout 可用 tag ③先跳 Flutter 尺推进 P2/P3 |
| 2026-10-25 | 重审修正（用户复核要求）：行 13 拆双模式（图元批处理+矢量路径密集）；新增行 17 显存压力/设备恢复（注入缝+预算拒增已在源码，缺资源场景窗）；§3.4 补公开参考数字锚（sanity 非验收线）。矩阵 16→**17 行** |
| 2026-10-25 | 立项：底座资源场景矩阵 16 行封闭 + 三层对照尺（Flutter 主尺/Godot 第二尺/glxgears 地板尺）+ 指标两段制（健康线+对齐线，系数数据后置）+ 三扇新窗规格（idle/batch_cap/multiwin_cap）+ 尾账 13 扇 Golden 与 E1/E2 豁免。决策记录：Godot 允许下载/编译、Flutter 允许构建/取产物、系数后置（用户定）。状态：P1 未开工 |
