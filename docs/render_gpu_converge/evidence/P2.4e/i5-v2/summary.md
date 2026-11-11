# I5 v2 证据汇总：Wayland present 非阻塞化（2026-11-12）

> 设计正本：`design.md`。旧 worker/每窗设备设计已标废弃。
> 状态：**✅ 2026-11-12 收口**。实现、真窗 A/B、负向、X11 回归已齐；`ui_base_vram_cap` 金图 17.5273% 由 P2.6「22 窗金图漂移」专项按既有证据重种，native/go 各 300s cascade 8/8 窗 `0.0000%`、像素探针 16/16，见 `evidence/P2.6/golden-drift/i5-vram/closeout.md`。

## 1 实现落点

| 文件 | 改动 |
|---|---|
| `gpu/webgpu/swapchain_configure.go` | 新增 `(*Swapchain).SetPreferNonBlocking()`：`Mailbox → Immediate → FifoRelaxed → Fifo` |
| `render/present_swapchain.go` | `newPresentSwapchain` 增 `want Backend` / `preferNonBlocking bool`；`newWebgpuHalSwapchain` 按标志设置 prefer list；新增 `presentPreferNonBlocking`（native Wayland only）与 `PresentTarget.PresentModeName()`；Wayland 只剩 FIFO 时一次性 WARN（I5 residual） |
| `render/present_construct.go` | 创建口按平台传 preferNonBlocking；删除无条件 `SetPreferVSync`/`SetPreferFifoRelaxed`；Wayland native `fixedNoVsync=true`、`vsyncOn=false` |
| `examples/ui_base_vram_cap/main.go` | `-cascade` 测试布局开关；每窗 `present_mode` 日志 + JSON `swapchain_present_mode` / `layout` |

`hal` 接口零改；FFI 零改；共享设备/线程模型零改。

## 2 单测与构建门禁

| 门禁 | 结果 | 证据 |
|---|---|---|
| `gpu/webgpu` 逐测试文件（21 文件） | 21 PASS / 0 FAIL（`tools/per-file-tests.sh`，DISPLAY=:3，WGPU_NATIVE_PATH 已导出） | `/tmp/opencode/i5-perfile-webgpu/summary.tsv`，日志随 shell 会话 |
| `render` present_* 五文件逐文件 | `present_damage` / `present_mode_wayland` / `present_waitidle` / `present_record` / `present_resize` 全 PASS | 见 `summary.md` 第 3 节命令 |
| 新能力单测 | `TestPickPresentMode_NonBlocking` PASS；`TestPresentPreferNonBlocking` PASS | 同上 |
| `go vet ./render/... ./gpu/webgpu/...` | rc=0 | — |
| 交叉编译 linux/arm64、windows/amd64、darwin/amd64、darwin/arm64 + `-tags nogpu` | 全 rc=0 | — |

## 3 真窗 A/B（嵌套 gnome-shell Wayland，cascade 8 窗，RUN_SECONDS=300）

| 轮 | 模式（8 窗） | n=2 fps_min | n=4 fps_min | n=8 fps_min / presents | 退出 | 卡栈 |
|---|---|---|---|---|---|---|
| 正（v2） | mailbox ×8 | 45.3 | 22.2 | 11.3；每窗 113–114 | 299.65s 正常，rc=1 仅金图 | 无 `Surface.Present` |
| 负（force FIFO，`negative/negative-fifo.patch`） | fifo ×8 | 42.7 | 20.6 | **0.0**；+0 | 150s SIGQUIT，未退出 | `Surface.Present` 2 处 |

- 正轮账本 `ledger_peak_mb=375.36`（< 768×0.9）、hold 斜率 `-161.9KB/min`；`pixel_probes_ok=16/16`。
- 负轮与 I1/当前树 Fifo 现场同构：n=8 冻结、进程不可退出；只改 `presentPreferNonBlocking` 一行（见 patch）。
- 原始输出：`positive/cascade-mailbox.{stdout,stderr,rc}`、`negative/cascade-fifo.{stdout,stderr,rc}`、`positive/mode-probe.stderr`。

## 4 回归

| 平台/后端 | 结果 | 证据 |
|---|---|---|
| X11 :3 native（cascade，manual 30s） | 8 窗全 `fifo`；n=2 49.1 / n=4 23.8 / n=8 11.5；rc=0 | `x11/cascade-smoke.{stdout,stderr,rc}` |
| Wayland go/gles（70s 探针，`GPUI_BACKEND=go`） | 8 窗全 `fifo`；n=2 52.9 / n=4 28.3 / n=8 14.4；70s 未冻（未跑满 300s，C7 域） | `go-wayland/probe.{stdout,stderr,rc}` |

## 5 金图（预存漂移，未重种）

- 正轮 8 窗 `pixel_golden_per_window=[17.52734375 ×8]`，`pixel_probes_ok=16/16`。
- 与 D1b（已回滚提交）当时的 17.5273% 逐字相同；窗口 1 在当前重跑也非 0，说明与「每窗设备」无关。
- 差异分析（`golden/`）：win1 差异 51120/307200 px，最大通道差 224；差异集中在卡片条纹/边框的 1px 边缘与 HUD 文本（HUD 已被 mask 排除）；内部采样点（10,470）/（40,120）/（320,240）逐字节相同；全局平移 ±2px 均不能降低差异（最优仍是 0,0）。
- `_base.png` 为 2026-10-28 旧基线；P1.4/P2.6 的 ladder 一直是 NO_JSON，金图从未在 SDF/AA 变更后复核。判定倾向「基线过期/AA 边沿漂移」，但按 P2.6「22 窗金图漂移」纪律，**未直接重种**，待该专项逐窗判定后再重种或修渲染。
- 视觉对照：`golden/win1-current.png`、`golden/win1-base.png`、`golden/win1-diff.png`。

## 6 公开 API / 文档

- `render.PresentTarget.PresentModeName()`（新增，18→19 方法）。
- `gpu/webgpu.(*Swapchain).SetPreferNonBlocking()`（新增）。
- `docs/RENDER_API_CATALOG.md` 已同步：§0 计数、§3.7 PresentTarget 行、§8 方法速查、§12 gpu 面注记、SetVsync 语义（native Wayland no-op）。

## 7 残余风险

1. 表面能力只有 FIFO 时 WARN 并登记残余，不承诺不冻。
2. 驱动/合成器连 Mailbox 也不返回时，进程内无法打断 FFI；需 Chrome 式 GPU 进程隔离，另立远期专项。
3. go/gles Wayland 仍走 EGL 阻塞 present（C7）；本项不修、不标绿。
4. 金图 17.5273% 未处置前 I5 只能 🟡。
