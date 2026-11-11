# I1 取证：ladder 8 窗 Wayland present 卡死——H1 实锤（Mesa WSI 等合成器 release 永眠）+ 触发几何（2026-11-11）

> 任务：§2.2 待办池 I1 剩余步 = P2.4e 解剖（`evidence/P2.4e/i1-dissection.md`）下一步 1：固化复现 + 卡死时抓 hub 线程**原生栈**，判开 H1（Wayland frame/回调等待）vs H2（resize 交错）。
> 环境：嵌套 gnome-shell Wayland（`dbus-run-session -- gnome-shell --nested --wayland-display=gpui-i1 --no-x11`，mutter 42.9），隔离于会话 Wayland；树 `163e8584+`（G3 提交后）。

## 1 复现（现行树，与 P2.4b/P2.4a 同构）

```
env LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \
  WAYLAND_DISPLAY=gpui-i1 XDG_RUNTIME_DIR=/run/user/1000 RUN_SECONDS=300 \
  /tmp/i1_vram_cap -auto-only
```

```
phase n=2 shared=2 fps_min=50.5 refusals=+0 (total 4) live=92.8MB
phase n=4 shared=4 fps_min=23.8 refusals=+478 (total 622) live=180.2MB
phase n=8 shared=8 fps_min=0.0 refusals=+0 (total 642) live=194.6MB   ← 卡死点
```

stderr 静止 40s+ 确认粘死；进程存活（后因取证需要按隔离环境兜底硬杀）。
原始输出：`ladder-run.stderr`、`gdb-run.txt`（本目录）。

## 2 原生栈（gdb，SIGQUIT 触发 batch 续跑，58 线程）

hub 线程（Thread 4, LWP 2186002）自顶向下：

```
#0 __GI___poll (timeout=-1)                    ← 无限期 poll
#1 wl_display_dispatch_queue ()                ← libwayland-client.so.0
#2-#4 libvulkan_intel.so （Mesa ANV/Intel Vulkan ICD 的 WSI 内部帧）
#5 <wgpu_hal::vulkan::swapchain::NativeSwapchain>::present ()
#6 <wgpu_hal::vulkan::Queue>::present ()
#7 <DynQueue>::present ()
#8 wgpu_core::present::<impl Surface>::present ()
#9 wgpu_core::present::<impl Global>::surface_present ()
#10 wgpuSurfacePresent ()                      ← rwgpu FFI 入口
#11 syscallX (purego) / #12 runtime.asmcgocall
```

**结论：H1 实锤，且阻塞点比解剖预期更深一层**——不在 wgpu 自有代码等 frame 回调，而在 **Mesa Intel Vulkan ICD 的 Wayland WSI**：`vkQueuePresentKHR` 的 Mesa 实现把「等合成器 release 上一个 buffer」做成了 `wl_display_dispatch_queue → poll(-1)`；嵌套合成器对（至少）一个 surface 永不 release → present 永眠。H2（resize 交错）排除（栈中无 resize/重配帧）。wgpu-native 升级不改变此行为（阻塞在 Mesa ICD，不在 wgpu）。

附带事实：GPU 栈为 `libvulkan_intel`——本机 native 后端选中的是 **Intel 集显 Vulkan**（与真窗 JSON `gpu_backend=integrated` 一致），非 NVIDIA 940MX。

## 3 触发几何（候选机制，与卡死相位吻合）

- 窗 640×480，ladder 摆放 `X=(idx%4)*640, Y=(idx/4)*540`（`ui_base_vram_cap/main.go:169`）→ 8 窗网格 **2560×1080**。
- 嵌套 gnome-shell 屏 = 会话屏 1920×1080 → **第 4 列（窗 4、窗 8）超出嵌套屏右缘**。
- mutter 对不可见/离屏窗不合成 → 其 Wayland buffer 不 release → Mesa WSI 的 release 等待 `poll(-1)` 永眠。
- n=4 相位时窗 4 已离屏但仍活着（fps 23.8，重度拒增未死）；n=8 时（叠加负载与第二枚离屏窗）冻结触发。同一摆放 X11 :3 PASS（P1.6 拒增 [2,0]→[4,550]→[8,747]）——X11 路无 Mesa Wayland WSI 的 release 等待；headless Wayland（P2.4d）屏大/无遮挡策略不同，故 PASS。**环境三态与几何解释自洽。**
- mutter 冻结时无日志痕迹（`/tmp/i1_nested.log` 无告警）；「离屏窗不合成」为 mutter 已知策略方向，未做 mutter 源码级钉死（不影响引擎侧修法拍板）。

## 4 引擎侧事实（冻结放大器，承接解剖 §机制事实）

1. `rwgpu/surface.go:517` 同步 FFI 持进程级 `gpuMu`（:502-517）→ 一个 surface 的 present 永眠 = 全进程 GPU 冻结。
2. 共享 raster hub：7 窗 goroutine 全堵 `Loop.Submit`；`PipelineApp.Close → Loop.RunSync` 等 hub → 连退出都不可（不可关窗、不可测）。
3. 无超时/无弃疗：wgpu-native 29.0.4 无异步 present 口；Mesa WSI 的 poll(-1) 无 deadline 参数。

## 5 摆放修正尝试与证伪（2026-11-11，拍板方向 A 后执行）

按 §3 几何假设做了级联摆放（`X=idx*56, Y=idx*56`，8 窗全部落在嵌套屏可视区内），重跑 ladder：**n=8 相位仍冻结**（`fps_min=0.0`，phase 形态与原版逐字同构，n=2 50.0 / n=4 25.3+506 / n=8 0.0，原始输出 `ladder-run-cascade.stderr`）。**离屏窗假设证伪**——合成器不 release 的触发面不是（或不只是）可视性，真实触发（嵌套 mutter 对第 8 表面的节流/资源策略或连接数相关行为）未定。摆放置已回退（Edit 进 Edit 出，未提交），P1.6 基线可比性保持；两轮卡死 stderr 与 gdb 原生栈留档。

剩余有效结论：H1 原生栈实锤不变（Mesa WSI `poll(-1)` 等合成器 release）；**环境回避路线（摆放/可视性）已被实验排除**，引擎侧健壮化（present 异步化 + deadline + 弃疗 + gpuMu 拆锁）成为唯一根治方向；P2.6 重放的 ladder 数据可继续以隔离 X11 :3 采集（P1.6 先例，Wayland ladder 洞保持登记不标绿）。

## 6 修法方向（停下报告待拍板；取证阶段零代码）

- **方向 A（摆放修正，小，立即可做）**：ladder 摆放改 2 列×4 行（1280×2280 超高也不行→按嵌套屏算列数）或按屏宽自适应列数，保证 8 窗全部在嵌套屏可视区内。性质：**窗/测试设计修正**（把窗摆出屏本身是无意义的真窗），预期解冻 8 窗相位 → 补跑拒增曲线 + hold 斜率（P2.6 必跑项落地）。**不修引擎洞**（合成器不给 release 时引擎仍全进程冻结）。
- **方向 B（引擎健壮化：present 异步化 + 弃疗，工程正解，大）**：present 出帧路径——专职 present worker + deadline；超时 surface 走 abandon/设备恢复（复用既有恢复路）；present 不持 gpuMu（或 TimedLock）消 H3 放大。涉及 rwgpu/webgpu/render 三层，属 P2.4e「后端呈现入口收敛」主线的延续，须独立排期与分批。
- **方向 C（上游登记）**：Mesa「离屏窗 present 挂起」向上游 mesa/wgpu-native 登记问题（本仓无法改 Mesa 行为）；随 B 或独立小步。

推荐：**A 立即做（解 P2.6 数据），B 立项排期（洞不掩盖、不标绿）**，C 随 B 带。
