# I2 收口证据：同尺寸 Resize 语义（2026-11-12）

> 任务：§2.2 I2 —— `TestS68_Swapchain_X11_MultiFramePresent` 第 6 帧同尺寸 `Resize(160,120)` 期望 `reconfigures` 抬升，实测 `reconfig=0`；产品侧 `swapchain_configure.go` 自 `34d9785e` 起把同尺寸 Resize 定为 no-op（避 `Surface.Configure` thrash/黑闪）。
> 结论（2026-11-12 用户拍板：按现语义销账）：采用「**同尺寸 Resize = no-op + `MarkNeedsReconfigure` 显式重配口**」。当前树代码与用例已经如此（e4 批 `03a630be` 改绑），本次复跑 PASS；台账此前滞后为 ⬜，现销账。**不改 `Resize` 签名、不加 force 参数、不删断言。**

## 1 当前树复现（PASS）

```
DISPLAY=:3 WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib \
  go test -count=1 -v -timeout 300s -run '^TestS68_Swapchain_X11_MultiFramePresent$' ./gpu/webgpu
```

```
=== RUN   TestS68_Swapchain_X11_MultiFramePresent
    swapchain_x11_linux_test.go:217: S6.8 swapchain multi-frame mode=fifo acquires=12 presents=12 reconfig=0 suboptimal=0 lastPresentMs=17.213
--- PASS: TestS68_Swapchain_X11_MultiFramePresent (0.42s)
PASS
```

原始日志：`repro.txt`。

## 2 代码与历史

| 位置 | 事实 |
|---|---|
| `gpu/webgpu/swapchain_configure.go:236-238` | `if sc.Width == width && sc.Height == height && sc.configured && !sc.pendingReconfigure { return nil }` —— 同尺寸 no-op |
| `gpu/webgpu/swapchain_configure.go:276` | `MarkNeedsReconfigure()` 置 `pendingReconfigure`，下一次 `BeginFrame` 真正重配（显式 escape） |
| `gpu/gwgpu/gles/swapchain.go:194-196` | gles 同尺寸 `return nil`，双后端语义一致 |
| `gpu/webgpu/swapchain_x11_linux_test.go:206-237` | e4 `03a630be` 改绑：断言同尺寸 no-op，再断言 `MarkNeedsReconfigure` 必须重配 |
| git blame / `git log -S` | 用例由 `8e4c0e99`（2026-07-15）引入旧期望；同尺寸 no-op 自 `34d9785e`；e4 `03a630be`（2026-10-31）改绑并桌面实跑 PASS。台账未同步，故仍显示 ⬜ |

## 3 标准对照（Skia/Flutter 一致）

Flutter/Impeller Vulkan 交换链：

- `flutter/engine/src/flutter/impeller/renderer/backend/vulkan/swapchain/khr/khr_swapchain_vk.cc:58-61`：
  `if (!result.out_of_date && size_ == impl_->GetSize()) { return std::move(result.surface); }` —— 同尺寸且非 out-of-date 不重建。
- `flutter/engine/src/flutter/impeller/renderer/backend/vulkan/swapchain/ahb/ahb_swapchain_vk.cc:64-67`：
  `if (impl_ && impl_->GetSize() == size) { return; }` —— 同尺寸直接返回。
- Impeller 的显式重建走 `RecreateSwapchain`/`out_of_date` 路径，不是给 `Resize` 加 force 参数。

本库对应：`Resize` 同尺寸 no-op（避黑闪），显式重配走 `MarkNeedsReconfigure`；与 Impeller 的「同尺寸不重建 + 单独重建入口」同构。

## 4 其他验证

- `render/present_resize_test.go`、`render/present_record_test.go`：真实 X11 窗口 resize 路径 + 同尺寸 no-op 不清 pending 的 headless 断言，均 PASS（见 `../i5-v2/tests/render-present-files.txt`）。
- `render/s6_8_window_present_x11_linux_test.go`：真实 X11 窗口多帧 present / idle skip，PASS（同上）。
- `ui_wr_c7_resize_dpr -auto-only`（X11 :3，真实 resize/DPR 窗）：功能门禁 fps 55.4、p95 18.93ms、hitch 0；金图 1.4439%（预存漂移，非 I2 引入，未用作 I2 像素判定）。原始输出 `window-c7.*`。

## 5 双后端格

| 后端 | 结果 |
|---|---|
| native / webgpu（X11） | ✅ 用例 PASS，`reconfig=0`；`MarkNeedsReconfigure` 后 `reconfig>=1` |
| go / gles | ✅ 代码 `gles/swapchain.go:194` 同尺寸 no-op；无独立 X11 gles 用例，语义逐行对齐 |
| Wayland | 同尺寸 Resize 语义与后端无关；I5 v2 已跑 Wayland 8 窗 present（见 `../i5-v2/`），本项不另跑 |
| metal / Win32 | 本机不可验；`Resize` 同尺寸 no-op 为后端无关代码路径 |

## 6 负向/边界

- 用例内成对断言：同尺寸 `Resize` 不得重配（红则说明 no-op 被破坏）；`MarkNeedsReconfigure` 必须重配（红则说明显式 escape 失效）。
- 跨尺寸 Resize 仍走 `Configure`；`Resize` 零宽/零高返回错误；`pendingReconfigure` 期间同尺寸调用不吞掉待重配（`render/present_record_test.go:41`）。
