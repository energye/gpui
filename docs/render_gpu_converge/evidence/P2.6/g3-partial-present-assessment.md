# G3 评估：TestPresentDamageX11 既有红——EGL_KHR_partial_update 在该 display 缺失，静默降级被计数断言暴露（2026-11-11）

> 任务：§2.2 待办池 G3（`gpu/gwgpu/gles/present_damage_x11_test.go::TestPresentDamageX11` 在 X11/NVIDIA FAIL：`partial present never engaged: full=12 partial=0`；干净树 `4e348fd3` 同红，与 P2.1 无因果）。
> 性质：诊断完成，零生产代码改动。停下报告待拍板修法。

## 1 复现（两形态都复现了）

```
Xwayland :3 -rootless &          # 显式隔离嵌套 X server（会话 GNOME Wayland）
env -u WAYLAND_DISPLAY DISPLAY=:3 go test ./gpu/gwgpu/gles -run '^TestPresentDamageX11$' -v -count=1
```

- 本会话首跑：`X Error BadMatch (X_GetImage, serial 36)` 进程被默认 X 错误处理器杀掉——**helper 映射窗口后无 viewable 等待，XGetImage 撞映射竞态**（G3 行登记的第二形态）。
- G3 行登记的首形态 `full=12 partial=0`：探针复现同断言（full=6 partial=0，帧数少因探针 4 档 damage）。

## 2 探针取证（临时 `TestZZG3PartialPresentProbe`，Edit 进 Edit 出已删；原始输出 `evidence/P2.6/g3/probe-run1.txt`）

| 环节 | 无上下文查询 | 上下文 current（=queue.Present 决策链真实条件） | display 扩展串 | 包级标志 |
|---|---|---|---|---|
| `BufferAge`（QuerySurface BUFFER_AGE_EXT） | ok=false | **ok=true, age=1**（可用） | `EGL_EXT_buffer_age` ✅ | — |
| `SetDamageRegionKHR`（20×20 rect） | FALSE, err=0x3009 | **FALSE, err=0x3000** | **`EGL_KHR_partial_update` ❌ 不在** | `pkg_partial=true`（符号存在） |
| `canScissorPresent` | — | true（fbo 128×128 == config） | — | — |
| `SwapBuffersWithDamage` 符号 | — | — | `EGL_KHR_swap_buffers_with_damage` ✅ | `pkg_swap=true` |

关键对照：**扩展串声明 buffer_age 但 SetDamageRegion 所需的 `EGL_KHR_partial_update` 不在**；包级标志（`egl/egl.go:775-788` GetProcAddress 符号存在性，与 display 无关）全部为真 → 测试跳过门（test:103）失效、决策链首选门（`queue_linux.go:286`）失效，最终靠 SetDamageRegion 逐帧 EGL_FALSE 兜底——而 `queue_linux.go:318-320` 把失败**静默吞掉**回落整帧（无计数、无日志），表面行为正确（像素断言全过、无鬼影）但局部路永不启用，测试的 `presentPartialCount>0` 断言把它喊了出来。

BufferAge 的上下文要求按规范核实：EGL_EXT_buffer_age 的 buffer age 查询对「未绑定当前上下文的 surface」无定义，NVIDIA 驱动要求 current context——生产决策链在 `q.ctx` 持锁 + surface current 的条件下查询（`queue_linux.go:288`），条件满足，不是问题；探针无上下文轮的 ok=false 属预期。

## 3 定性

1. **引擎回落行为正确**：无 `EGL_KHR_partial_update` 的 display 上退整帧是安全正确的（测试的像素断言/鬼影断言在整帧路上全过）。
2. **真实洞 = 能力判定层级错误 + 静默降级**：
   - 决策链与测试跳过门都用包级符号标志，未核 per-display 扩展串；符号存在 ≠ display 可用。
   - SetDamageRegion 逐帧 FALSE 无痕吞掉：每个 damage 帧白打一次 EGL 调用 + 无任何可见性（日志/计数/探针），与「降级必须显式」的项目纪律相悖。
   - 测试 BadMatch 竞态（helper 无 viewable 等待）是独立的测试基建洞。
3. **对齐标准**：Flutter engine 的 EGL 呈现对 damage 系扩展的判定就是 per-display 扩展串检查后缓存使用（`flutter/engine` shell/gpu egl 路径），不存在「只查符号」的用法；本仓应对齐。

## 4 修法方向（停下报告待拍板）

- **方向 A（能力如实化 + 测试口径修正，推荐，小代码）**：
  1. `Surface.Configure/CreateSurface` 时按 display 扩展串解析并缓存能力位：`partialPresentOK = 符号存在 && 串含 EGL_KHR_partial_update`（bufferAge 同理）；
  2. 决策链改用缓存能力位（有能力 display 行为零变化；无能力 display 省去逐帧 FALSE 调用）；
  3. 测试跳过门改用同一 per-display 能力（缺扩展 → skip，与 test:103 现有 skip 设计同型；有能力 → 计数断言照旧硬卡）；
  4. 兜底回落从「静默吞」改「一次性日志/计数」（hal.Logger 或 surf 计数器）。
  5. 顺带修 helper viewable 等待（BadMatch 竞态）。
- **方向 B（A + SwapBuffersWithDamage 独立局部路探索）**：该 display 声明了 `EGL_KHR_swap_buffers_with_damage`，理论上可用 buffer_age 历史 + swap_with_damage 走无 SetDamageRegion 的局部路；但缺 SetDamageRegion 时驱动不保证保留 back buffer 其余部分，需逐像素证据链验证正确性，成本与风险显著高于 A，不建议现在做。
- **方向 C（只修测试跳过，引擎不动）**：无能力 display 上跳过测试，保留「逐帧白调 EGL + 静默吞」的降级痕迹；不推荐。

## 5 拍板与实现批（2026-11-11，用户拍板方向 A，已实现）

**代码改动（4 文件 + 1 单测）**：

1. `gpu/gwgpu/gles/surface_damage_linux.go`：新增 `presentDamageCapabilities` 类型 + `resolvePresentDamageCapabilities(hasSwap, hasPartial, displayExtensions)` 纯函数（符号存在 ∩ display 扩展串；未导出）。
2. `gpu/gwgpu/gles/resource_linux.go`：`Surface` 增 `presentDamageCaps`（创建 EGL 窗面时解析一次、只读）与 `presentDamageFallbacks/presentDamageFallbackLogged`（降级可见性计数）；`createEGLWindowSurface` 解析能力位。
3. `gpu/gwgpu/gles/queue_linux.go`：决策链改用缓存能力位（`swapWithDamage && partialUpdate`）；SetDamageRegion 失败回落从静默吞改为计数 + 首次 `hal.Logger().Warn`（带 egl_error 与能力位快照）。
4. `gpu/gwgpu/gles/present_damage_x11_test.go`：能力跳过门改用 per-display 能力位并**后移到全帧/Y-flip 门之后**（保留无能力 display 上的全帧像素覆盖）；`EGL_EXT_buffer_age` 字符串检查并入能力位；新增 XSetErrorHandler（计数）+ XGetImage nil 重试（3s）——BadMatch 从进程击杀变为带诊断的干净失败/成功。

**验证数据（Xwayland :0 会话 mutter 托管 + 显式 :3）**：

| 项 | 结果 |
|---|---|
| `TestPresentDamageX11`（:0） | **SKIP** `swap=true partial=false`（此前 FAIL `full=12 partial=0`；全帧 1/2/3 像素断言在 skip 前实际跑过：red/green/blue 0xff0000/0xff00/0xff） |
| `TestPresentDamageX11`（裸 :3，无 WM） | FAIL→XGetImage 持续 nil（x_errors=147，重试 3s 后干净 Fatal，不再杀进程）——**环境属性**（无 WM 合成读不回窗口内容），登记为该窗运行要求：需 WM 托管 X display |
| 能力解析单测 `TestResolvePresentDamageCapabilities` | PASS（4 例含 G3 形态「符号有/串无」、串空、符号缺失） |
| `TestPresentDamageUnionForAge` / 全包 | PASS；`go test ./gpu/gwgpu/gles -count=1` ok（2.387s） |
| `-race`（damage 四测试，CGO_ENABLED=1） | ok |
| 行为零变化核对 | 有能力 display 的决策序列不变（能力位=旧条件∩扩展串；旧条件下两者同值时行为一致）；本机（无 partial_update）从「逐帧 FALSE+静默吞」变为「不进分支」，呈现结果同为整帧（像素断言同值） |

**负向对照**：旧树 `TestPresentDamageX11` FAIL（G3 登记红 + 本会话探针 full=6 partial=0，`evidence/P2.6/g3/probe-run1.txt`）→ 新树同 display SKIP 且全帧断言绿；能力解析单测的「串说有/符号无」「串说无/符号有」两腿反向钉死。

**跨平台格（硬）**：本修复属 Linux EGL 呈现线——Linux EGL（X11/Wayland）已修并本机验证；Windows 走 WGL 独立呈现路（`queue.go` wgl 分支，无 EGL damage 语义，不涉及）；macOS Metal 不涉及；Web 路径不涉及。

**公开 API**：零新增/修改公开符号（类型与函数均未导出），render 域 catalog/apidoc 不涉及。
