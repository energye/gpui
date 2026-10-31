# I1 解剖：多表面 present 卡死（ladder 8 窗 native）

- 日期：2026-10-31（P2.4e e4 内解剖）；归属池 §2.2 I1
- 现象三态（同代码不同环境）：
  1. **嵌套 Wayland（隔离）**：ladder 8 窗相位 hub 线程阻塞在 `rwgpu.(*Surface).Present → procSurfacePresent.Call`（FFI 不返回），`fps_min=0.0`、live 停在 ~200MB，RUN_SECONDS 到期后连关窗都退不出（主线程 `PipelineApp.Close → Loop.RunSync` 等 hub；其余 7 窗 goroutine 堵在 `Loop.Submit`）。P2.4a 干净树 `55495fdd` 同栈逐项复现 → 非回归。
  2. **隔离 X11 :3（P1.6）**：同窗 PASS（拒增 [2,0]→[4,550]→[8,747]、峰值 372MB）。
  3. **headless Wayland（P2.4d 最终环境）**：ladder 8 窗 rc=0 正常退出（n=8 `fps_min=13.3`、峰值 372.2MB），**未复现**。
- 环境三态说明卡死是**环境相关的合成器/驱动路径**，不是确定性代码缺陷；但阻塞点本身是真实的健壮性缺口。

## 机制事实（代码级）

1. `gpu/rwgpu/surface.go:517` `procSurfacePresent.Call(s.handle)` 是**同步 FFI**：wgpu-native 的 `wgpuSurfacePresent` 内部走 `surface_present`（wgpu-core），Wayland 路径会做 frame 回调等待/子图提交，合成器不消费时**不返回**。
2. 调用持有**进程级 `gpuMu`**（surface.go:502-517）：Present 一旦卡死，进程内所有其它 GPU 调用（任何窗的任何 rwgpu 入口）全部排队等锁 → 全进程 GPU 冻结，与观察到的「另 7 窗全停」一致。
3. 上层无超时/无异步化：`gpu/webgpu/surface.go:363-374` → `render/present_frame.go:343`（presentFn → EndFrame）→ hub 帧路径同步等待；hub 被卡则 `Loop.RunSync`（关窗）也进不来 → 观察到的不可退出。
4. wgpu-native 29.0.4 未暴露 present 的 future/异步口（与 discard 同类缺口，discard 已在 surface.go:456-459 注释登记）。

## 候选触发链（按证据强度排序，未定论）

- **(H1) wgpu Wayland frame 回调等待**：wgpu-core 的 Wayland present 在 `surface_present` 里等 `wl_callback`；嵌套合成器（gnome-shell nested）在多表面 + 高负载下若对某 surface 停发 frame 事件（如子表面遮挡/resize 竞态），present 永等。headless 合成器（--no-x11 虚拟屏）行为不同所以不复现。支持证据：卡死只在嵌套 Wayland 出现、X11 与 headless 均过。
- **(H2) resize 期间 acquire/present 交错**：ladder 每窗带尺寸跳变（8 窗阶梯），若 present 时 surface 尺寸与已 acquire texture 不一致，wgpu-core 可能进入重配等待；与 I2 同族但表现为阻塞而非黑帧。
- **(H3) gpuMu 上的锁序放大**：Present 持 gpuMu 期间另窗 BeginFrame 在锁上等待，属放大器不是根因（单窗也会卡，只是不扩散）。

## 下一步（修复方向在解剖闭合后才拍板）

1. 复现环境固化：嵌套 Wayland + ladder 8 窗 + GPUI_VRAM_TRACE/线程栈采样，卡死时抓 `procSurfacePresent` 原生栈（gdb attach 或 SIGQUIT 后线程 dump），确认是否停在 `wl_display_dispatch` / frame callback 等待。
2. 若 H1 证实：候选方向 a) rwgpu 侧给 present 前置 `surface 尺寸一致性` 检查（规避 H2 交错）；b) wgpu-native 升级/补丁验证 `wgpuSurfacePresent` 的非阻塞变体；c) render 层给 EndFrame 路径加看门狗（超时 → abandon 该 surface 走设备恢复），c 是兜底不是根修。
3. 任何修复都要回带三环境矩阵（嵌套 Wayland / 隔离 X11 / headless）+ ladder 8 窗拒增曲线。
4. **本步（e4）不改 present 路径代码**——按池规则解剖前不定修复方向；e4 的 C7 修复（开窗投递 hub）与本根因无交集（C7 是绑定竞争，I1 是 present FFI 阻塞）。
