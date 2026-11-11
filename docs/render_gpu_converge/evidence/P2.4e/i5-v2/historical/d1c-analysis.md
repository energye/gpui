# D1c nested Wayland 取证：部分隔离成立，4 窗判死 + CreateCommandEncoder 824GB abort（2026-11-11）

> 任务：D1 每窗独立 instance/device 后，重放 I1 nested Wayland ladder 8 窗卡死现场；验收=其余窗 fps>0、进程正常退出。
> 结论：**部分成立，整体 FAIL**。hub 不再被单窗 present 冻结（n=8 phase 仍打印；fps_min=20.5 为含判死窗的 10s 均值，不能单独证明存活窗 fps>0），watchdog 判死 4 个 surface；但随后 live 窗 flush 在 `wgpuDeviceCreateCommandEncoder` 报 `memory allocation of 824634933856 bytes failed` → SIGABRT，rc=2，进程没有正常退出。

## 1 运行

- 隔离 nested gnome-shell `gpui-i5c`；`GPUI_PRESENT_PER_WINDOW_DEVICE=1`；`RUN_SECONDS=300 -auto-only`。
- 原始输出：`ladder-per-device.stderr`（79KB，含 SIGABRT 全量栈）、`ladder-per-device.stdout`（空，未到最终 JSON）、`ladder-per-device.rc`=2。

## 2 现场

```
phase n=2 fps_min=50.8 refusals=+0 live=95.9MB
phase n=4 fps_min=28.8 refusals=+580 live=191.9MB
CRITICAL present lost (window=0x35b5aea0 640x480) after 5.004s   lost_total=1
CRITICAL present lost (window=0x70ea5c058230 640x480) after 5.005s lost_total=2
CRITICAL present lost (window=0x35b60930 640x480) after 5.013s   lost_total=3
CRITICAL present lost (window=0x35b63140 640x480) after 5.003s   lost_total=4
phase n=8 fps_min=20.5 refusals=+1367 live=383.9MB
memory allocation of 824634933856 bytes failed
SIGABRT: abort
```

## 3 判定

- **正向**：per-device 隔离生效——hub 没有被单个 present 卡死，n=8 阶段仍能完成并打印；4 个卡死 surface 被 watchdog 5s 判死并 fail-fast。**口径说明**：`fps_min=20.5` 是含 4 个判死窗前 5s presents 的 10s 均值，不能单独证明存活窗 fps>0；最终 JSON 因 abort 缺失，per-window presents 未取得。对比 2c 共享设备：同一 n=8 现场直接全窗冻结、`fps_min=0.0`，连 phase 都出不来。
- **负向**：① 不是 1 个 surface 卡，而是 **4 个**（lost_total=4），不满足「其余 7 窗」；② 进程在 live 窗 flush 的 `CreateCommandEncoder` 里 SIGABRT，rc=2，**没有正常退出**。
- 崩溃栈：`Context.flushGPUWithViewCoreImpl → GPURenderContext.Flush → GPURenderSession.encodeSubmitSurfaceGrouped (render_session.go:7103) → webgpu.Device.CreateCommandEncoder → rwgpu.Device.CreateCommandEncoder (command.go:75) → memory allocation 824634933856 bytes failed`（Rust `handle_alloc_error`）。
- `824634933856` 是**垃圾 size**（不是真实显存需求），优先怀疑：① `Device.CreateCommandEncoder` descriptor/label 的 Go FFI 指针生命周期（同 batch 1 `Proc.Call` 丢 `//go:uintptrescapes` 一类；label/StringView 被读成巨长）；② wgpu-native/Mesa 在 4 个 lost WSI surface 后的全局态损坏。两者需 A/B 分开，不能猜。

## 4 架构含义

- D1 per-window device 解决了 **Go/hub 层**的单窗冻结，但没有解决 **native 层**的多 surface lost 与进程崩溃；目标 1 的「进程可正常退出」仍未达成。
- 若 824GB 是 Go FFI descriptor 指针问题：先做 pointer-safe descriptor（heap 固定 + 保活/直调 SyscallN）A/B；修好后 D1c 可能只剩 4 lost + 存活窗，需再判是否接受「多 surface lost」。
- 若是 Mesa/ICD 全局态：D1 的设备级隔离不够，标准路线应转 **D2 进程隔离**（Chrome/Flutter GPU 进程 + watchdog 杀进程重建）；这也是设计正本原本引用的路线。

## 5 当前状态

- 2a、2b、D1a、D1b A/B 已提交；D1c 证据未提交。
- N3 登记：D1c 4 lost + CreateCommandEncoder 824GB abort；下一步先排除 Go FFI descriptor，再决定 D1 修补或 D2。
