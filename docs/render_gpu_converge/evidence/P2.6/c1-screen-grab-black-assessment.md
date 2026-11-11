# C1：14 窗 x11grab screen check 抓黑图 —— 排查评估（2026-11-11）

> 台账 §2.2 C1：`baseline/windows/README.md:33-37` 登记「rootless Xwayland 的 x11grab 抓图是黑图，
> 14 个窗自身带 screen check 必然红，属采集环境口径，P2 需在隔离 Xorg 重跑」。
> 本文件只做排查与方向设计，**未动生产代码**。

## 0 结论（两句话直话）

抓黑图**不只是环境限制，还有一个代码洞**：`xwininfo` 的窗口 id 行带 `xwininfo: ` 前缀，
窗口按第一个冒号切分，永远取不到 `Window id`，于是「优先 xwd -id 抓窗口自身像素」的分支
从未生效，全部退回 ffmpeg x11grab 抓 X root —— 而 rootless Xwayland 的 X root 根本不绘制
（Wayland 合成器持有窗口表面），所以必然全黑。实测 `xwd -id <winID>` 能取到真实像素。

## 1 复现与实测数据（同机同显示，单窗自退出）

| 档位 | 跑法 | screen check 实测 |
|---|---|---|
| native Wayland（默认，WAYLAND_DISPLAY=wayland-0） | `RUN_SECONDS=5 go run ./examples/ui_render_base_backdrop -auto-only` | `xwininfo -name ...: exit status 1`（窗口是 Wayland surface，无 X11 窗）→ 抓图失败 |
| X11/Xwayland（`env -u WAYLAND_DISPLAY`，窗口成 X11 客户端） | 同上 | `diff=100.0000% over 627200 px`（backdrop）/ `99.3265%`（filterops）/ `100.0000%`（dither）→ FAIL |

X11/Xwayland 档同一窗口、同一时刻的三路抓图对照（后台常驻窗 + xwininfo 取 id）：

| 手段 | 产物 | 逐像素统计 |
|---|---|---|
| `xwd -silent -id 0xa00001`（窗口自身 backing store） | 3843219 B XWD → 63403 B PNG | **minR=0 maxR=255 nonbg=263999**（真实画面） |
| `xwd -id 0xa00001`（不带 -silent） | 同样 | minR=0 maxR=255 nonbg=263983（真实画面） |
| `ffmpeg -f x11grab -i :0+102,114`（X root 区域） | 2891 B PNG | **minR=0 maxR=0 nonbg=0**（全黑） |

xwd 抓图 vs 窗内最终快照：1200×800 全幅 diff=1808 px（0.19%，HUD/末帧差级别），
说明 xwd -id 抓到的就是呈现帧，可作 screen check 的真源。

**shim 轨迹（决定性）**：给 filterops 挂 `xwd`/`ffmpeg` 诊断壳再跑 gate，
日志只有一行 `FFMPEG ARGS: ... -f x11grab ...`，**xwd 从未被调用** —— 它的 xwd 优先分支是死代码。

## 2 根因（定位到行）

1. 解析洞（12 个窗同款，`examples/ui_render_base_filterops/main.go:530-545` 为样板）：
   `xwininfo` 输出首行是 `xwininfo: Window id: 0xa00001 "..."`，代码对每行
   `strings.Cut(line, ":")` 取首个冒号，`k` 得到 `xwininfo` 而不是 `Window id`；
   于是 `winID` 恒为空，`if winID != "" { xwd ... }` 分支永不执行，直接落到 x11grab。
   其余几何行（`Absolute upper-left X: 102` 等）无前缀，所以坐标解析正常 —— 这个洞很隐蔽。
2. 环境根因：rootless Xwayland 下 X11 窗口由 Wayland 合成器合成，X root 区域没有绘制内容；
   `x11grab` 抓 root 必然黑。xwd 按 window id 读窗口自身 backing store 则有真实像素
   （仓库自己 `examples/ui_render_base_filterops/README.md:50` 也是这个口径：优先 xwd，
   x11grab 仅作无 xwd 时的回退）。
3. native Wayland：窗口根本不注册 X11 窗口，`xwininfo -name` 失败，screen check 无法执行。

## 3 影响面（22 个含 x11grab 的窗）

- C1 登记的 14 窗：backdrop / cliprrect / dither / filterops / maskops / miscdraw /
  pathbool / svg / textlow / textspacing / textstroke / verticesmesh / webp / ui_wr_shadermask。
- 其中 4 窗（cliprrect / filterops / miscdraw / svg）**有** xwd 优先分支但被解析洞废掉；
  另 10 窗只有 x11grab，无 xwd 路径。
- 同款解析洞还存在于 8 个后加的窗（ui_wr_p38_color_font / picture / svg_doc / svg_filter /
  svg_paint / svg_showcase / svg_structure / svg_text）——同源同因，按「新问题归位」一并登记。

## 4 标准修法（对齐仓库自有口径 + X11/Wayland 语义）

- 解析：`strings.Contains(ln, "Window id:")` 后对该子串再切，或按最后一个冒号切，取出
  `0x...` 形式的窗口 id；`xwininfo -id <id>` 可复核。
- 抓图顺序：`xwd -id <winID>`（窗口自身像素，抗遮挡）→ ffmpeg 转 PNG；
  仅当没有 xwd 或 xwd 失败时才回退 `x11grab`（真 Xorg 会话）。
- native Wayland 无 X11 窗：不是「产品回归」，但 screen check 也拿不到像素 —— 需明确口径
  （显式 SKIP + 机器可读原因，或另接 Wayland 截屏协议），不能把全黑当绿、也不能静默跳过。

## 5 顺带发现（非 C1，单独立项）

三扇窗在本树上有**既有金图漂移**（与抓黑图无关，两档显示环境同值）：
backdrop golden 6.3519% / filterops 5.2101% / dither 31.7164%。按「新问题归位」登记，
不在 C1 里改金图。

## 6 待拍板（停下报告）

- D1 修复范围：14 窗（C1 登记面）/ 22 窗（含 8 个同源解析洞）/ 只先修 4 窗验证。
- D2 native Wayland 无 X11 窗时的口径：显式 SKIP+原因 / Wayland 原生截屏 / 保持 FAIL 记为环境。
- D3 验收方式：X11/Xwayland 档逐窗跑 screen check 转绿 + 负向（强制 x11grab 回退应红）/ 隔离 Xorg 重放。

证据目录：`evidence/P2.6/c1-repro/`（wayland.log / xwayland.log / filterops_xwayland.log /
dither_xwayland.log / shim 轨迹 / 抓图 PNG 统计）。

## 7 D2 追加排查：Wayland 原生截屏在本机 GNOME 会话的可行性（2026-11-11）

用户拍板 D2=「接 Wayland 原生截屏」后，按「动手修门槛」先验证标准方案在本机能否自动化：

| 路径 | 探测 | 结果 |
|---|---|---|
| XDG portal `org.freedesktop.portal.Screenshot`（version 2，godbus/dbus/v5 已在 go.mod） | Go 探针调用 `Screenshot("", {interactive:false, handle_token})` 并订阅 `Request.Response` | 返回 handle `/org/freedesktop/portal/desktop/request/1_782/c1probe1`，**20s 内无 Response 信号** —— GNOME 弹出交互授权对话框，无人值守拿不到图；请求超时后自动取消（对象已不存在，桌面无残留） |
| GNOME Shell 内部 `org.gnome.Shell.Screenshot`（Screenshot / ScreenshotArea / ScreenshotWindow 均在） | gdbus 直接调用 | `GDBus.Error:org.freedesktop.DBus.Error.AccessDenied: Screenshot is not allowed`（GNOME 43+ 收紧，仅放行 portal 路径） |
| `org.freedesktop.portal.ScreenCast` + PipeWire | 能力面 | 同样需交互选源，且要 PipeWire 客户端读帧（纯 Go 无现成实现），比 Screenshot 更重 |

**结论（如实）**：本机 GNOME Wayland 会话的两条原生截屏路径，一条需人工交互授权、一条被内核接口策略拒绝；
「接 Wayland 原生截屏」在当前桌面环境下无法做成无人值守门禁。这是桌面环境的授权策略，不是实现能绕过的，
不应绕过。要真正落地需：一次性人工授权拿 `restore_token` + PipeWire 帧读取客户端，属独立大批次。

因此 D1（22 窗解析洞修复）与 D3（X11/Xwayland 逐窗+负向验收）可以照拍板立即开工；
只有 **native Wayland 档的 screen check 口径需要重新拍板**（见停下报告）。

## 8 实现与验收（2026-11-11，用户拍板 D1=22 窗 / D2=显式 SKIP / D3=X11 逐窗+负向）

### 实现（收敛到单一真源）

- 新增 `examples/wrkit/screencheck.go`：`GrabClientArea(title, outPath)` 一份实现，
  22 窗各自重复的 grabClientArea 全删，调用点全量切到 `wrkit.GrabClientArea`（接口全量切换制）。
  - 解析修复：用 `strings.Index(s, "Window id:")` 定位窗口 id（不再整行按首个冒号切），
    `xwininfo: Window id: 0x...` 正确取出 winID。
  - 抓图顺序：`xwd -silent -id <winID>` → ffmpeg 转 PNG；仅当无 xwd/失败或
    `GPUI_C1_FORCE_X11GRAB=1`（负向用）才回退 ffmpeg x11grab。
  - `ErrScreenUnsupported` / `ScreenUnsupported(err)`：native Wayland 无 X11 窗
    （DISPLAY 空或 xwininfo 失败）时返回，调用方记显式 SKIP。
- 22 窗：`screenCheckResult` 加 `Skipped`，JSON 加 `screen_check_skipped`；
  unsupported 时 `Attempted=false`（不触发 `if Attempted && !OK` 的 fail）、
  `Detail=skipped:no_x11_window (...)`，不记绿、不静默。
- goimports/gofmt 全量；`go build ./...` 与 `go vet ./examples/...` rc=0。

### 验收（证据 evidence/P2.6/c1-verify/）

| 项 | 结果 |
|---|---|
| X11/Xwayland 档 14 窗（gate 模式必跑） | `screen_check_ok=true`，diff=0.0000%（627200/737760/912000 px 各窗），逐窗日志在 c1-verify/*.log + per-window.tsv |
| X11/Xwayland 档 8 个 p38 窗（screen check 为手动 `-screencheck` opt-in） | 带 `-screencheck` 全部 diff=0.0000%（647620–1076640 px），*_screencheck.log 在案 |
| native Wayland 档（backdrop 代表） | `screen_check_skipped=true`、`screen_check_ok=false`、Detail=`skipped:no_x11_window`；screen check 不再拉红 |
| 负向 `GPUI_C1_FORCE_X11GRAB=1`（强制回退 x11grab） | `diff=100.0000% over 627200 px` 仍红 —— 证明不是放水 |
| 既有金图漂移（与 C1 无关） | backdrop 6.3519% / filterops 5.2101% / dither 31.7164% 等窗口 FAIL 只来自 golden，screen check 已绿；按「新问题归位」登记，不改金图 |

### 口径说明

- C1 登记的「根因=环境，需在隔离 Xorg 重放」修正为：**主因是解析洞（代码）**，
  修好后 rootless Xwayland 用 xwd -id 即可真抓；隔离 Xorg 不再是必要条件。
- native Wayland 档按用户重拍板走显式 SKIP（机器可读原因）；Wayland 原生截屏
  （portal/PipeWire，需人工授权 token）登记为后续独立大批次，不假装已解决。

## 9 登记勘误：既有金图漂移的真实范围（2026-11-11 复核）

§5 只登记了 backdrop / filterops / dither 三扇，复核 `evidence/P2.6/c1-verify/` 全部
22 窗日志后更正：**21/22 窗的 pixel_golden_diff_pct 非零**（唯一 0 是 ui_render_base_svg），
幅度 0.1880%–31.7164%：

| 代表窗 | golden diff |
|---|---|
| ui_render_base_dither | 31.7164% |
| ui_render_base_miscdraw | 14.2041% |
| ui_render_base_backdrop | 6.3519% |
| ui_wr_shadermask | 5.6476% |
| ui_render_base_filterops | 5.2101% |
| ui_render_base_textlow | 3.4169% |
| ui_render_base_maskops | 2.2262% |
| ui_render_base_webp | 2.1279% |
| ui_render_base_pathbool | 1.8532% |
| 其余 12 窗 | 0.1880%–1.6722% |

- 修复前（c1-repro 两档显示）与修复后（c1-verify）同值（backdrop 恒 6.35188137755102），
  两档显示环境同值 ⇒ **与 C1 的抓图修复无关、与显示后端无关**，是历史金图（2026-10 月基线）
  相对当前渲染结果的普遍漂移。
- C1 验收口径只看 screen_check 字段：22 窗 screen check 全绿/显式 SKIP，golden FAIL 是既有欠账。
- 按「新问题归位」登记为独立余留项：22 窗金图漂移专项（需逐窗判定是「渲染有洞」还是「基线过期」，
  禁止直接重种掩盖）。本项不处理。
