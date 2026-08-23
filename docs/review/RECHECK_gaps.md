# 复核查漏报告（RECHECK gaps）

> 日期：2026-08-23。任务：反向查漏——检查 R1–R7 / ROUND2 / ROUND3 是否漏判了重要问题。
> 结论先行：**未发现新的 P0（崩溃/内存安全级）问题**；发现 **1 条新 P1**、若干 P2，其余重点区域干净。

---

## 一、新发现

### N1（新 P1）Wayland/X11 的 DPI 缩放恒为 1.0，HiDPI 屏上整条渲染链按错误分辨率出图

**直话**：两个 Linux 后端的 `ScaleFactor()` 写死返回 1——不是「没接好」，是根本没有接缩放源的代码。HiDPI 用户（2x/125% 缩放）会得到：界面偏小、文字发虚、ggcanvas 的物理像素画布与窗口实际像素不匹配。这属于「功能在主流高分屏环境下不达标」的 P1；不是崩溃，故不算 P0。

证据链：
- Wayland：`ui/platform/wayland_linux.go:1377` 定义 `scale float64`，全文件**只有读没有写**（grep 全包仅 1489/1492 两处读取）；构造处 `wayland_linux.go:715` `host := &wlHost{win: win}` 不带 scale → 永远走 `<=0 return 1` 分支（1483-1493）。协议侧只绑了 `wl_output_interface` 当 set_fullscreen 参数用（188 行），**没有监听 wl_output.scale / wp_fractional_scale / wp_viewport**。
- X11：`ui/platform/x11_linux.go:404、45` `scale: 1` 初始化后无人改；`ScaleFactor()`（705-717）只是兜底返回。全文件无 RandR/xft.dpi 查询。
- 影响面：`ui/embedder/packet.go:20`、`pipeline_app.go:640/831`、`app.go:193` 都拿这个值建 FramePacket 和配 swapchain → 错误值贯穿全链。
- 缓解事实：`wayland_pointer_linux.go:50` 注释明说「buf.scale handling is a later refinement; GNOME Wayland scale 1 for now」——是已知取舍而非疏忽，但 FINAL_SUMMARY 只把它归进「platform 窗口层无 P0/P1」，**定级偏低**，建议升级为 P1 并入 platform 专项审计清单（FINAL_SUMMARY 第三节其实已提「还需补 platform 窗口层专项审计」，本条即其具体内容之一）。

### N2（新 P2）帧调度器 vsync 监听 goroutine 无退出路径，窗口关闭后泄漏并持续唤醒

`ui/scheduler/scheduler.go:100-116`：`ensureVsyncListener` 起的 `for {}` goroutine 只有 `WaitVSync` 成功 `continue` / 失败 `time.Sleep(d)` 两条路，**没有 done/select 退出分支**。App Close 或窗口重建后该 goroutine 仍挂在 DRM vblank 上周期性醒来。单实例进程退出无害，多窗口反复开关会累积泄漏 + 空耗 CPU 唤醒。修法：加 quit channel，`Close()` 里关掉。

### N3（新 P2）Wayland 剪贴板 Get 最长阻塞 UI 线程 3 秒

`ui/platform/wayland_clipboard_linux.go:469-505`：`readOffer` 同步读管道，EAGAIN 时 `time.Sleep(2ms)` 轮询直到 3 秒 deadline。Get 在事件线程调用，对端（粘贴源应用）卡住时整个 UI 冻结最多 3 秒。注释自认「GTK 的 Wayland paste 同样同步」，有界、可辩护，但有界冻结仍是体验洞；建议后续改异步 + 超时提前取消。X11 侧启动路径也有 `x11_linux.go:390/442` 的 50ms+最长 500ms 等几何稳定循环——仅发生在开窗一次，不算提交路径，忽略。

### N4（确认维持，非新增）lib 版本管理两条 P2 仍在

`lib/wgpu-native-meta/wgpu-native-git-tag` = v1.0.1，仅人读、加载链 `gpu/rwgpu/wgpu.go:222-250` 不校验 so 指纹/ABI（ROUND3 P2-a 成立，本次复核确认无变化）；`lib/wgpu-linux-x86_64-release (2).zip`（约16MB）下载残留仍在仓库目录（ROUND3 P2-b 成立）。加载链本身（env → ./lib → 当前目录 → 系统搜索）逻辑正确，`Init` 用 once 单次加载、失败带明确报错提示，无新洞。

---

## 二、扫过但干净的区域

| 区域 | 方法 | 结果 |
|---|---|---|
| gpu/context（22 文件：window/platform/pointer/scroll/gesture/events/registry…） | 通读核心文件 + grep panic/死循环 | 纯接口+数据结构层（W3C Pointer Events 建模），几乎无逻辑分支，Null 实现齐全；单测全绿 |
| render/integration/ggcanvas | 通读 canvas.go/render.go 全文 | 资源生命周期严谨：deferTextureDestruction/pendingTexture 升级/Close 幂等/Untrack 防双关都做了；damage 缩放取 floor/ceil 方向正确；无 P0/P1 |
| ui/platform 窗口层（wayland/x11 生命周期/resize/clipboard/cursor） | 重点通读 + grep | resize 链完整：xdg configure→resized 标志→EventResize→embedder 去重+DPR 变更清 BoundaryCache（pipeline_app.go:637-660，做得对）；X11 _NET_WM_SYNC_REQUEST 计数器格式（long[] 8字节）处理正确；cursor 有 cursor-shape-v1 + theme 双策略降级；destroying 原子门防 poll 后析构崩溃。除 N1/N3 外干净 |
| 生产代码 `panic(` 扫描 | 全仓 grep（排除 test/example/third_party） | 仅 4 处：rwgpu/errors.go:117（已标 Deprecated 且有 Async 替代）、recording/registry.go:40（注册期 fail-fast，合理）、device_browser.go:288/294（浏览器桩未实现）、rwgpu/wgpu.go:447 mustInit（生产路径无人调用，仅 checkInit）。均不在热路径 |
| 无限 `for{}` 死循环 | 全仓 grep + 逐个核对退出条件 | 生产代码 3 处均有出口：webgpu/internal/thread/thread.go（done channel）、wgsl parser×2（token 边界 break）。唯一无出口的就是 N2 的 vsync goroutine |
| 提交路径上的 `time.Sleep` | 全仓 grep | 命中点均为：开窗一次性等待、hide 时有界（2s deadline）排空、vsync 失败退避——无逐帧提交路径上的 sleep |

## 三、针对性测试（全部通过）

```
go test -count=1 -timeout 120s ./gpu/context/... ./render/integration/ggcanvas/... ./ui/platform/...
→ ok ×3（platform 6.6s 含真窗交互测试）
go test -count=1 -timeout 90s -run 'TestShaper|TestShape' ./render/text/ ./render/text/cache/
→ ok ×2
go vet ./gpu/context/... ./ui/platform/... ./render/integration/ggcanvas/...
→ 仅 unsafe.Pointer 风格告警（wayland FFI 数组遍历惯用法，非实际错误）
```

注：`render/text/shaping` 目录不存在（shaping 在 `cache/shaping.go` 与根包 shaper_*.go），此前文档若按此路径引用需更正。

## 四、方法说明

先读 FINAL_SUMMARY 定基线 → 对五个薄弱区逐一通读/grep 高危模式 → 对可疑点追全链路（如 scale 从 platform 到 embedder 到 packet 的消费者）→ 跑 4 组短超时定向测试佐证。共约 35 步工具调用，未改动任何引擎文件。

## 五、一句话总结

前三轮审查质量可信，没有漏掉崩溃级的洞；唯一值得提级的是 **Linux 双后端 HiDPI 缩放恒为 1（N1，升 P1）**，外加 vsync goroutine 泄漏和剪贴板 3 秒阻塞两条 P2。
