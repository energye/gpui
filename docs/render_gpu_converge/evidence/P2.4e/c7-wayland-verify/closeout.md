# C7 收口：gles 多窗 Wayland 第二机制——EGL display 生命周期（2026-11-12）

> 任务：§2.2 C7。X11 主修复随 P2.4e/e4 + J5 已闭合；本批核实并修掉 Wayland「context-per-surface 第二机制」。
> 结论：**✅ 闭合**。根因不是 0x3002，而是关窗时多个 per-surface EGL context 反复 `eglTerminate` 同一个 `wl_display` 的 EGL display；修复=EGL display 按 native display 引用计数，Wayland 借用 display 在最后一笔也不 terminate（宿主/进程退出回收）。

## 1 修复前（当前树可复现）

命令（嵌套 gnome-shell Wayland，cascade 8 窗，go/gles）：

```
GPUI_BACKEND=go RUN_SECONDS=300 examples/ui_base_vram_cap -auto-only -cascade
```

- 8 窗全部打开，n=8 phase 正常（fps_min 16.2、每窗 presents>0），但 **300s 收尾关窗 SIGSEGV**，rc=2。
- 45s `-manual-seconds` 同样在关窗 SIGSEGV，rc=2。
- 卡栈：`PresentTarget.Close → releaseShared → gles.Device.Release → AdapterContext.release → AdapterContext.Destroy → egl.Context.Destroy → eglTerminate → SIGSEGV`。
- 原始：`red-before-full.*`、`diagnosis-handles.stderr`（诊断打印显示 8 个 EGLDisplay 句柄不同：`0x…4a0/…e10/…a20/…410/…6e0/…5d0/…630/…620`，但 native display 同一个 `wl_display*`）。
- 不含 `eglMakeCurrent 0x3002`：X11 那条开窗竞态已由 e4/J5 修掉，Wayland 剩下的只是关闭生命周期。

## 2 根因

- Wayland Path B：每扇窗 `Instance.CreateSurface` 各建一个 EGL context（`gpu/gwgpu/gles/api_linux.go:170-190`），`egl.NewContext` 用同一个 `wl_display*` 调 `eglGetPlatformDisplay`。
- Mesa 对同一个 native display 的多次 `eglGetPlatformDisplay` 返回 **不同的 EGLDisplay 句柄**；每个 `egl.Context.Destroy` 都调 `Terminate(display)`（`gpu/gwgpu/gles/egl/context.go`）。
- 第一个/前几个 terminate 让共享 native display 失效；最后一个 terminate 在宿主已经关闭 `wl_display` 之后执行，Mesa 走到已释放状态 → SIGSEGV。
- J5 只给 `AdapterContext`（首窗 context 存活）加了 refs，没有覆盖 **EGLDisplay 连接级生命周期**。

## 3 修复

| 文件 | 改动 |
|---|---|
| `gpu/gwgpu/gles/egl/display_ref.go`（新增） | `displayKey{kind, native}` + 进程级 refs：按 native display 归组，保存首个 EGLDisplay 句柄；最后一笔只对非 Wayland terminate 一次。Wayland 借用 display 不 terminate（宿主拥有 `wl_display`，可能已先关闭）。 |
| `gpu/gwgpu/gles/egl/context.go` | `Context` 增 `displayKey`；`Initialize` 成功后 `retainDisplay`，错误路径 `releaseDisplay`；`Destroy` 释放 key，不再逐 context `Terminate`。 |
| `gpu/gwgpu/gles/egl/display_ref_test.go`（新增） | `TestDisplayRef_Lifecycle`（X11/headless：首句柄只 terminate 一次）、`TestDisplayRef_ContextDestroyReleasesOnce`、`TestDisplayRef_WaylandBorrowedDisplayIsNotTerminated`。 |

标准依据：EGL display 的生命周期是 **native display connection 级**，不是 context 级；`eglTerminate` 释放并失效整个 display。Wayland 的 `wl_display` 归宿主（`ui/platform`）所有，库不应在宿主关闭连接后再 terminate 它；X11/headless 由本库创建 display 时才 terminate。引用计数保证 X11/headless 仍只释放一次。

## 4 验收

| 轮 | 结果 | 证据 |
|---|---|---|
| 修复前 300s go/gles Wayland | n=8 phase 正常，关窗 SIGSEGV，rc=2 | `red-before-full.*` |
| 修复后 45s 关窗 | 2/4/8 三相位，rc=0，无 SIGSEGV/0x3002 | `fixed-45s-close.*`、`fixed-45s-final.*`（最终树复跑） |
| 修复后 300s 满轮 | 8 窗全开/全 present（每窗 136–137），elapsed 299.65s，rc=1 仅金图；无 SIGSEGV/0x3002；ledger 357.4MB < 水线，hold 斜率 -1917KB/min | `fixed-full-300s.*` |
| X11 go 30s 回归 | 2/4/8 三相位，rc=0，无 SIGSEGV/0x3002 | `x11-go-smoke.*` |
| 单测 | `TestDisplayRef_*` PASS；`TestAdapterContext_Destroy_SerializedWithLock` / `TestSurface_Destroy_OwnedCallsAdapterContextDestroy` / `TestSurface_Destroy_SharedDoesNotDestroyAdapterContext` PASS；`go vet ./gpu/gwgpu/gles/... ./render/...` rc=0 | 见 §5 |

## 5 门禁摘要

```
go test -run '^TestDisplayRef_' ./gpu/gwgpu/gles/egl        # ok
go test -run '^TestAdapterContext_Destroy_SerializedWithLock$|^TestSurface_Destroy_' ./gpu/gwgpu/gles  # ok
go vet ./gpu/gwgpu/gles/... ./render/...                    # rc=0
```

## 6 双后端 / 平台

| 端 | 结果 |
|---|---|
| go/gles Wayland | ✅ 8 窗 300s 全开/全 present/正常退出（本项目标） |
| go/gles X11 | ✅ 30s smoke rc=0；X11 原 0x3002 已由 e4/J5 闭合 |
| native Wayland | 不受本改动影响；I5 v2 8 窗 mailbox 已验证（`../i5-v2/`） |
| Windows gles / metal / Win32 | 本机不可验；改动仅在 linux EGL 路径（`//go:build linux`），windows twin 不触及 |

## 7 残余

1. Wayland 借用的 EGL display 在最后一窗关闭后不 `eglTerminate`，由宿主连接/进程退出回收；这是有意的所有权选择，不再走会崩的路径。
2. 金图 17.5273%（窗 2–8；窗 1 为 0）与 I5 v2/P2.6 记录的同一预存漂移一致，非 C7 引入；C7 判定用 8 窗开/呈现/无 EGL 错误，不用金图重种。金图债仍在 P2.6「22 窗金图漂移」名下。
3. 若宿主在同一 `wl_display` 上关掉全部窗口后又重新开窗，EGL display 仍处于 initialized 状态；本轮未覆盖该重开路径，登记为后续观察项（不阻塞 C7）。
