# P3.9b 1x/4x MSAA 真实窗口动画性能基准：开工前评估（2026-11-09）

> 性质：开工前只读评估，零代码。真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2/§3 P3/§4.1/§4.2、`docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md` §5.1/§7.4、`AGENTS.md`（GPU 真窗与压测安全 / 2.5D 真窗摆法 / 像素级断言 / 测试数据）。
> 触发：用户问「开 4x 很占内存和性能吧？mesh 和 4x 的使用场景是什么」→ 需要一份**真实窗口、通用复杂动画**下的 1x/4x 对照数据，而不是静态小窗的推断。

## 0 直话

- **字面 4K 在这台机器上做不了 1x/4x 对照**：物理屏 1920×1080、GPU 940MX **1024 MiB** 显存、NVIDIA 580。按 P3.9a 实测推算，1200×800 开 4x 的 device-surface 池是 65.3 MB；3840×2160 是 8.64 倍像素 → 约 **564 MB** 单池，加字形图集/滤镜图/管线必爆；引擎 `GPUShared.RequestMSAADowngrade()`（`gpu_shared.go:751`）会在 OOM 时把 4x 拆回 1x 重建，**最终记录到的还是 1x**。
- 所以按用户拍板：**本机跑满屏 1920×1080**（真 GPU、有效数据），代码里把分辨率做成可配、留 3840×2160 档给以后大显存机器，本机不跑该档并如实登记原因。
- **仓库里其实已有 1x/4x 证据，但不可用作性能结论**：`evidence/P2.6/e-perf/windows/ui_render_base_backdrop/{msaa1,msaa4}/{native,go}/record.json` 四份全是 `status=FAIL`、`fps_interval ≈ 9~11`、`hitch ≈ 47~58`，且 1x 与 4x 的 device-surface `bytes` 完全相同（15,360,000）——那是 backdrop 重滤镜负载/软件栈下跑的，显存没随 4x 变。文件在、数据不能当 1x/4x 性能结论。上一轮说「没有成体系证据」据此修正：**有成体系文件，但没有成体系的有效数据**。
- 唯一干净的 1x/4x 对照来自上一轮临时诊断（`/tmp/p39a_msaa4.txt`，静态 1200×800 窗）：显存 **11.5 MB → 65.3 MB（5.67×）**、首帧 **113 ms → 135 ms**、帧间隔/p95/fps 基本不变（该窗 CPU 侧瓶颈 `cpu_raster_pct ≈ 90%`）。这组数说明不了「动画 + 满屏 + 复杂场景」下 4x 的真实代价，正是本项要补的。

## 1 环境硬约束（实测，非推断）

| 项 | 值 | 来源 |
|---|---|---|
| 会话 | `XDG_SESSION_TYPE=wayland`，GNOME，`WAYLAND_DISPLAY=wayland-0` | `env` |
| 物理输出 | XWAYLAND0 1920×1080 @59.88Hz，最大 32767×32767 | `xrandr --current` |
| GPU | NVIDIA GeForce 940MX，**1024 MiB**，驱动 580.178.04 | `nvidia-smi` |
| 进程 | 1× gnome-shell、1× Xwayland（无独立 Xorg） | `ps` |
| 已知事故 | 2026-10-27 Xorg `nvidia_drv.so (nvidiaUnlock+...)` 段错误，触发点是 `measure.py` 的 **SIGTERM 强杀** parity 窗；940MX + NVIDIA 580 + Xorg Present/DRI3 同类 | `AGENTS.md` |
| 引擎默认 | 1x（`render/sample_count.go:32,57`），`SetMSAASampleCount(4)` 是进程级 global，开窗/GPU 初始化前调，作用于之后所有会话 | `sample_count.go:34-47`、`gpu_shared.go:802` |
| OOM 降级 | `RequestMSAADowngrade` 把 `sampleCount` 闩到 1、`deviceGen++`、销毁 4x 管线重建 | `gpu_shared.go:745-772` |
| 性能门禁 | `fps_interval ≥ 55`、`interval_p95_ms ≤ 22`、`hitch_rate_per_min ≤ 5`；p99 只观测 +8ms 回归带 | `baseline/thresholds.md:86-97` |
| 既有 1x/4x 记录 | 4 份全 FAIL、显存两端相同 | `P2.6/e-perf/.../ui_render_base_backdrop/` |

## 2 缺口与目标边界

- **缺口**：缺一份「真实 GPU 窗口 / 通用复杂动画场景 / 1x 与 4x 各自有效 / 双后端」的对照数据，用来回答 4x 到底占多少显存、掉多少帧、首帧多慢。
- **本项做**：
  - 新建 `examples/ui_wr_p39b_msaa_anim/`：满窗即内容的混合通用动画场景（文字/圆角/曲线路径/线性径向渐变/图片/半透明层/旋转缩放/粒子点阵 + **mesh gradient 网格**一起每帧动），指标行浮在画面左上角。
  - 分辨率可配 `GPUI_P39B_W/H`（默认 1920×1080，留 3840×2160 档）；MSAA 可配 `GPUI_P39B_MSAA=1|4`（开窗前 `SetMSAASampleCount`）。
  - 三种运行模式：`-auto-only` 冻结相位的门禁（三证据）、`RUN_SECONDS` 相位推进的测量（性能 JSON）、无参数常驻人工窗（收真事件打日志改标题、关窗正常退出）。
  - 当前 Wayland 桌面 **一轮一窗、串行**跑 1x/4x × native/go 共 4 轮，每轮 18s；每轮前查显示连接/合成器存活、发现异常立即停批；程序自己 `RunFor` 到点正常退出（`app.Close()`→`win.Close()`），不并发、不加速循环、不硬杀。
  - 落 `evidence/P3.9b/`：四轮 JSON、对照表、显存/首帧/帧时/CPU 分解、4K 档不跑的原因、结论。
- **本项不做**：新增 render 公开 API（无）、改引擎 MSAA 默认值、做 mesh 专属 4x/per-target MSAA（非标准扩展）、UI 层超采样（非标准方案，用户已排除）、跑 3840×2160（本机显存不允许）、PDF/SVG/kit/2.5D 引擎改动。

## 3 影响面评估

| 层 | 文件 | 改动 | 风险 |
|---|---|---|---|
| 示例 | 新 `examples/ui_wr_p39b_msaa_anim/main.go` | 满窗混合动画场景 + 三模式 + 可配分辨率/MSAA | 只调 `render`/`ui/rendering`/`wrkit`/`wrgate` 公开 API；不在示例里实现引擎行为 |
| 示例 | 新 `examples/ui_wr_p39b_msaa_anim/testdata/` + `goldens/` | 冻结相位快照与 4 份基线（backend×msaa） | 金图仅门禁模式用 |
| 证据 | 新 `evidence/P3.9b/` | assessment/design/四轮日志/JSON/PNG/对照表/summary | 无代码 |
| 文档 | `TASKS.md` §2/§2.1/§7、`PLAN.md` §7.4 | 状态/载体/流水 | `RENDER_API_CATALOG.md` **不改**（无新公开 API），`apidoc` 应仍 rc=0 |
| 引擎/GPU/WGSL/ui/kit | — | **零改动** | 本项是测量，不是修洞 |

**不做**：不改 `gpu/hal`、WGSL、shader 翻译器、MSAA 默认、`render.Mesh*`、`ui/kit`、engine。

## 4 验证载体（§4.2 四项，开工前填齐）

- **能力/逻辑单测（本项为逻辑探针，随真窗）**：`runLogicProbes` 断言场景图元计数、mesh 三角数 > 0、相位确实推进、请求的 MSAA 值、`render.MSAASampleCount()` 实际生效值、无 CPU 回退、device-surface 池字节数 > 0；像素探针（冻结相位）断言背景/卡片/curve/mesh 中心/图片/半透明叠加的不变量点。
- **真窗**：`examples/ui_wr_p39b_msaa_anim`（`-auto-only` 冻结相位判 PASS/FAIL + 常驻人工窗二合一；`RUN_SECONDS=18` 测量模式出性能 JSON）。
- **金图与容差**：`examples/ui_wr_p39b_msaa_anim/testdata/goldens/p39b_anim_<backend>_msaa<1|4>.png`，**容差 0**（P0.1 金图逐位口径），按 backend×msaa 各一份（4x 边缘与 1x 不同，不能共用）；比对区排除左上角指标行。像素探针通道容差显式声明。
- **双后端**：webgpu（native）跑 1x/4x；gwgpu-gles（`GPUI_BACKEND=go`）跑 1x/4x；gwgpu-**metal 不可验**（无 macOS 真机，交叉编译只算编译通过，本项连交叉编译都不涉及新代码）。
- **负向对照**：`GPUI_P39B_MSAA=4` 请求 4x 时，device-surface 池字节数必须显著大于 1x（≥4×），否则判「4x 未生效/被 OOM 降级」并记录；日志 grep `MSAA downgraded` 必须为空。另以 `GPUI_P39B_BREAK=freeze` 人为冻结动画相位，断言「相位推进」逻辑探针必须红。

## 5 安全流程（AGENTS 硬规则）

- 当前桌面是 GNOME **Wayland 会话**（非裸 Xorg），前序 P3.9/P3.9a 真窗均在此单窗直接跑且正常退出（exit 0），有同路径先例；用户拍板「当前桌面单窗串行」。
- 一轮一窗、**串行**、每轮 18s，程序 `RunFor` 到点自己 `app.Close()`→`win.Close()` 正常退出；**不并发、不加速循环、不 `kill -9`/`pkill`/`killall`/`loginctl`**。
- 每轮开始前检查：gnome-shell / Xwayland 存活、`wayland-0` 套接字可连、`nvidia_drv.so` 段错误 / `Invalid MIT-MAGIC-COOKIE-1` / 会话消失 任一出现立即**停批并报告**。
- 关 GPU 资源前由引擎 `WaitIdle`；顺序「停帧 → WaitIdle → 释放 surface/swapchain → 关窗口」由 `app.Close()` 内部保证，示例不改此顺序。
- 4K 档本机不跑，避免在 1 GiB 卡上触发 OOM 降级/驱动崩溃。

## 6 完成判定（§4.1 两态 + 本项特殊性）

- 本项是**性能基准测量**，不是新能力收口；「✅」判定为：四轮真窗都正常退出、各自拿到有效性能 JSON、逻辑探针全绿、1x/4x 显存差异可观测（或如实登记降级）、**对照表与结论落盘**。
- 三证据：① 逻辑探针（含 MSAA 生效证据）② 冻结相位像素探针 + 金图逐位 ③ 四轮真窗 JSON/PNG。缺一不标绿。
- 若 1080p 4x 也触发 OOM 降级：如实登记「1 GiB 卡 1080p 4x 也撑不住」，不粉饰、不改成「绿」。

## 7 结论

- 阶段 1（P3.9a）已收口；本项是其**后续测量补充**，不是新能力线，不新增公开 API、不改引擎。
- 本机唯一能出有效 1x/4x 结论的尺寸是满屏 1920×1080；4K 档留可配但本机不跑，原因写清。
- 开工条件满足；按 §4.1 先落文档（本评估 + `design.md`）再实现。
