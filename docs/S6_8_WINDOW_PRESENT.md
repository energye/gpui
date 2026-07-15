# S6.8 — 真窗口 Present 管线

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.8 关闭**  
> 依赖：S.03 Swapchain、S5.1/S6.1 PresentFrameAuto、Linux X11  
> 架构：`X11 window → webgpu.Swapchain → render.PresentFrame* → surface.Present`

---

## 1. 目标

把 **离屏 present-only** 接到 **真窗口 Acquire/Present**，并可控 vsync / 重配：

1. Swapchain **Fifo vsync**（默认）与 **Mailbox 低延迟** 偏好  
2. **Suboptimal / outdated 自动 reconfigure**  
3. **EndFrameWithDamage**（后端可忽略；API 齐备）  
4. **Present 统计**（acquire/present/reconfig/suboptimal 计时）  
5. 多帧 **PresentFrameAuto + damage** 真窗口 e2e  
6. **Idle 跳过 present 回调**（无脏区不刷屏）  

**非目标**：Wayland 专项、控件层、改像素语义。

---

## 2. 实现摘要

| 组件 | 改动 |
|------|------|
| `webgpu.Swapchain` | Stats、MarkNeedsReconfigure、pending suboptimal、PreferVSync/LowLatency、EndFrameWithDamage、acquire 重试 |
| `examples/window_present` | PresentFrameAuto + Fifo；无额外 sleep（present 阻塞） |
| `TestS68_*` | 单元 + X11 multi-frame + idle skip |

### 诊断

```go
st := sc.Stats()
// Acquires, Presents, Reconfigures, Suboptimal, LastPresentMs
sc.PresentModeName() // "fifo" | "mailbox" | ...
sc.SetPreferVSync()
sc.SetPreferLowLatency()
sc.MarkNeedsReconfigure() // 窗口 resize 后
```

### 应用模式

```go
dc.BeginFrame()
// draw dirty only...
frame, _ := sc.BeginFrame()
out, err := dc.PresentFrameAuto(frame.Handle, frame.Width, frame.Height, func() error {
    return sc.EndFrame(frame)
})
// out.Idle == true → present 回调未执行
```

---

## 3. 验证（本机 DISPLAY=:1，真 GPU）

| 测试 | 结果 |
|------|------|
| `TestS68_Swapchain_PreferPresentModes` | ✅ |
| `TestS68_Swapchain_X11_MultiFramePresent` | ✅ 12 帧 Fifo，presents=12 |
| `TestS68_Swapchain_X11_SuboptimalReconfigureFlag` | ✅ reconfigures≥1 |
| `TestS68_WindowPresent_MultiFrameDraw` | ✅ **p50≈11.1ms**（Fifo vsync） |
| `TestS68_WindowPresent_IdleSkip` | ✅ idle 不调 present |
| `TestSwapchain_WindowPresentE2E` | ✅ |

> 沙箱内 XOpenDisplay 可能失败；真窗口测试需 **unsandboxed + DISPLAY**。

---

## 4. 复现

```bash
export DISPLAY=:1
export XAUTHORITY=...
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH

go test -count=1 ./gpu/webgpu -run 'TestS68_|TestSwapchain_Window' -timeout 120s
go test -count=1 ./render -run 'TestS68_' -timeout 180s

# 演示
go run ./examples/window_present
```

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| Acquire/Present 多帧 e2e | ✅ |
| Fifo vsync 可选 | ✅ |
| suboptimal reconfigure | ✅ |
| damage present API | ✅ |
| PresentFrameAuto + idle | ✅ |
| 与离屏基线对照（present-only 口径） | ✅ p50≈11ms @Fifo |
| 无 silent CPU | ✅ |

**S6.8 关闭。** 下一：**S6.9 重场景分级预算**。

---

## 6. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | Swapchain stats/reconfigure/damage；TestS68_*；example S6.8 |
