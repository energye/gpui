# I5 批 2c 取证：I1 ladder 真窗 FAIL——wgpu-native 内部锁连坐 hub（2026-11-11）

> 任务：在 2a（present 去 gpuMu）+ 2b（per-surface worker + watchdog + fail-fast）之后，重放 I1 的 nested Wayland ladder 8 窗卡死现场，验收「其余 7 窗 fps>0、进程可正常退出」。
> 结论：**FAIL（否决）**。worker 确实不再持 gpuMu，watchdog 也在 5.012s 正确判死；但卡死的 `wgpuSurfacePresent` 持有 **wgpu-native 内部锁**，hub 的 `Device.FlushCallbacks → Instance.ProcessEvents` 被连坐 7 分钟，其它窗与关窗路径全部堵死，进程无法正常退出。**per-surface worker + watchdog 单独不足以实现 I5 目标 1**；必须改架构（每窗独立 instance/device 或进程隔离）。

## 1 运行

- 环境：隔离 nested gnome-shell Wayland `gpui-i5b`（`dbus-run-session -- gnome-shell --nested --wayland-display=gpui-i5b --no-x11`），Intel Vulkan，wgpu-native 29.0.4。
- 命令：
  ```
  env LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \
    WAYLAND_DISPLAY=gpui-i5b XDG_RUNTIME_DIR=/run/user/1000 RUN_SECONDS=300 \
    /tmp/i5b_vram_cap -auto-only
  ```
- 原始输出：`ladder.stderr`（含 SIGQUIT 全量栈）、`ladder.stdout`（空：SIGQUIT 前未到最终 JSON）、`ladder.rc`=2（SIGQUIT，不是正常退出）。

## 2 现场

```
phase n=2 fps_min=44.5 refusals=+0
phase n=4 fps_min=23.6 refusals=+472
CRITICAL render: present lost (platform=1 window=0x7384280073f0 640x480) after 5.012s; surface abandoned, subsequent frames fail fast (lost_total=1)
phase n=8 fps_min=0.0 refusals=+0
```

watchdog/日志/fail-fast 都按设计工作；但运行超过 `RUN_SECONDS=300` 约 8 分钟仍未退出，SIGQUIT 抓栈。

## 3 关键栈（`ladder.stderr`）

| goroutine | 状态 | 栈 |
|---|---|---|
| 5（卡死 surface 的 worker） | syscall 7 分钟 | `rwgpu.(*Surface).Present` → `wgpuSurfacePresent` → Mesa WSI（**没有 gpuMu**，2a 生效） |
| 34（raster hub，locked to thread） | syscall 7 分钟 | `PipelineApp.Run → presentPacketTextured → PresentTarget.present:233 → webgpu.Device.FlushCallbacks → webgpu.Instance.ProcessEvents → rwgpu.Instance.ProcessEvents`（FFI 不返回） |
| 51（某窗 UI） | sync.Mutex.Lock 7 分钟 | `PipelineApp.Run:1573 → inFullRecovery → PresentTarget.InFullRecovery → t.mu.Lock`（t.mu 被 hub 的 present 持有） |
| 1（main） | chan send 3 分钟 | `PipelineApp.Close → runOnGL → Loop.RunSync → Loop.Submit`（hub 队列满/不消费） |

因果链：卡死 present 在 wgpu-native 内部持锁 → hub 的 `ProcessEvents` 等同一把锁 → hub 卡在 `PresentTarget.present` 并持 `t.mu` → 该窗 UI 卡 `t.mu`、其它窗的 RunSync/Close 卡 hub 队列 → 进程无法退出。**所有共享同一个 instance/device 的窗都被连坐**，与 Go 侧 gpuMu 无关。

## 4 判定

- 2a（present 去 gpuMu）是必要条件：worker 栈里确实没有 gpuMu 了。
- 2b（worker/watchdog/fail-fast）在**单元/集成层**成立：mock 阻塞不堵 peer；watchdog 5s 判死；fail-fast 日志在案。
- 2c 真窗否决：共享 wgpu instance/device 下，卡死 present 的**内部锁**会堵住 hub 的 `ProcessEvents`/GPU 调用；worker 隔离不了这把锁。设计 §2 的「worker per-surface => 其它窗不受影响」假设不成立。
- 不满足验收：其余 7 窗未能持续 fps>0；进程不能正常退出（SIGQUIT rc=2）。

## 5 架构选项（停下报告）

| 选项 | 内容 | 代价/风险 |
|---|---|---|
| **D1：每窗独立 instance/adapter/device/queue** | 不再共享 `shareDevice`；每个 PresentTarget 自己的 wgpu 实例/设备。卡死 present 只锁自己的设备，hub 的 ProcessEvents 走别的设备 | 代码改动中等偏大（设备共享是 P2.4a 收敛成果；render Context/纹理缓存/账本要按设备分账）；1GB 卡上 8 个设备的显存/创建开销需实测，可能 OOM |
| **D2：进程隔离（GPU 进程/窗或窗组）** | Chrome/Flutter watchdog 路线：present 在独立进程，卡死杀进程重建 | 最稳，但工程量大（IPC、纹理共享、窗口重建、账本） |
| **D3：保留 2a，回退 2b，重新设计 I5** | 承认共享设备架构下 present 阻塞无法隔离，I5 目标 1 不成立；只保留可观测/快速失败，不做「其它窗不受影响」承诺 | 不满足用户目标 1；但避免半吊子 |
| D4：接受卡死，只保证进程可被外部杀掉 | 不推荐 | 违反「正常退出」与「不冻结」 |

推荐：先做 **D1 的可行性实测**（8 窗 ladder 的显存/创建开销 + 卡死现场），若 1GB 卡放不下再做 **D2**。D1 与 D2 都需要用户拍板后才能继续；本轮不硬写。

## 6 当前工作区状态

- 2a 已提交 `a84ceb66`。
- 2b 实现（worker/watchdog/fail-fast/GL 同步能力/UI 门控）与 2c 负向证据在**工作区未提交**；2b 单测/集成绿，但 2c 真窗 FAIL，故**不标 ✅**。
- 设计 `i5-robustify-design.md` 的批 2/3 计划需按本次否决重写（§6 回填）。
