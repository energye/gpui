# GPU device lost 修复方案（Skia·Flutter 生命周期）

> **状态：主路径已落地（活文档）** — 2026-07-21  
> **任务**：修复 device lost 导致测试程序崩溃 / 恢复后 OOM  
> **主测**：`examples/mem_anim_window` · `api_coverage_app` · `device_lost_redraw` · lifecycle matrix  
> **webgpu 库**：`lib/libwgpu_native.so`（soft 补丁：lost 不 panic）  
> **规范**：对标 Skia `GrDirectContext` abandon+recreate / Flutter Rasterizer surface 生命周期（策略映射，非源码拷贝）  
> **策略活文档**：[`SURFACE_LIFECYCLE_SKIA_FLUTTER.md`](./SURFACE_LIFECYCLE_SKIA_FLUTTER.md)  
> **引擎缺口**：[`ENGINE_GAPS.md`](./ENGINE_GAPS.md) G3（持续 soak，非「未实现」）

---

## 1. 问题与根因

### 症状

- `PUI_SCENARIO` 误写已更正：正确命令为  
  `GPUI_SCENARIO=S23 LD_LIBRARY_PATH=$PWD/lib go run ./examples/mem_anim_window`  
  （或已编译的 `/tmp/mem_anim_window`）
- 窗口被**其他窗口完全挡住**后继续 full-rate present → 驱动 TDR → parent device lost  
- 随后调用 `wgpuSurfaceGetCurrentTexture` → 本机 `.so` **Rust panic**  
  `Parent device is lost` → **SIGABRT**（进程崩溃，不是 Go error）

### 根因（分层）

| 层 | 事实 |
| --- | --- |
| Native（旧 `.so`） | GCT 在 parent 已 lost 时 **fatal panic** → SIGABRT |
| Native（soft `gogpu/rwgpu` 补丁） | GCT/Configure/Submit 等 **ErrorSink + status/null**，不 panic |
| Native | `wgpuDeviceGetLostFuture` **未实现**；DeviceLost 回调 Destroy 路径上常不进入 Go |
| Binding | sticky per-device lost + 错误映射 + AutoRecover（Skia abandon+recreate） |
| Host | 不可 present 的 surface **不 acquire**；失焦仍可见继续画 |

---

## 2. Skia / Flutter 目标模型（本仓库对齐的策略）

| 实践 | 含义 |
| --- | --- |
| Sticky per-device lost | 每逻辑 Device 独立 lost，不 mark-all |
| 底层只报错 | soft ABI 返回 error/status；上层记状态 |
| Refuse GPU work | lost 后 create/submit/acquire 返回 `ErrDeviceLost` |
| Lost-safe teardown | abandoned surface 跳过危险 native |
| Abandon + recreate | `EnableAutoRecover` → RequestDevice + reconfigure + rebind + 清缓存 |
| **遮挡 ≠ device lost** | 不可 present → 暂停 acquire |
| **失焦 ≠ 暂停** | 仍露出则继续 present（可降帧） |
| 恢复可见 | force 最新整帧 |

---

## 3. 解决方案（已实现）

### 3.1 Binding：`gpu/rwgpu` + `gpu/webgpu`（soft native 假设）

1. **Sticky lost**（Skia abandon）  
   - DeviceLostCallback + uncaptured `looksLikeDeviceLost`  
   - `Device.lost` + `lostDeviceHandles`  
   - 多设备 userdata 路由，**永不 mark-all**

2. **Acquire 路径（无 WriteBuffer canary）**  
   - `FlushCallbacks` / `SyncLostState` = ProcessEvents + 吸收 pending uncaptured lost  
   - sticky lost → `ErrSurfaceDeviceLost` / `ErrDeviceLost`  
   - 否则调 soft GCT → 按 **status + Uncaptured** 映射错误  
   - `Surface.abandoned`：parent lost 后 Unconfigure/Release 跳过 native

3. **Destroy**  
   - force sticky → native Destroy+Release → 清 handle  

4. **Facade**  
   - `EnableAutoRecover` + `ClearRecoverCooldown`  
   - recover 成功后 `SetDeviceProvider` **必须**清 filter/glyph 等缓存  

4b. **引擎级 abandon（标准解，不靠示例枚举 Context）** — 2026-07-21
   - `render` 维护 **GPU Context 注册表**（`NewContext`/`ensureGPUCtx` 自动登记）
   - `AbandonAcceleratorDevice` → **先 `abandonAllContextGPU()`**（所有窗口/离屏 Context 的 session + filter publish）→ 再 `AbandonDeviceProvider`
   - `SetAcceleratorDeviceProvider` 换绑前同样 `abandonAllContextGPU`
   - `GPUShared.AbandonExternalDevice` 对 `liveCtxs` **完整 Close**（非仅 Destroy session）
   - 示例 `closeAllEffectRTs` 变为双保险；**根治点在引擎**

4c. **Recover VRAM（CommandBuffer / offscreen pool / blend pipeline）** — 2026-07-21
   - 根因：旧 Device 子资源未 Release → 进程显存 ~322→515MiB（双 RequestDevice 堆）→ 新 Device 上 `session_depth_stencil` 连 1x1 都 OOM
   - **离屏 readback** `encodeSubmitReadback`：`Finish`+`Submit` 后必须 `CommandBuffer.Release`（否则每帧/每个 effect RT 漏 CB）
   - 同类：`sdf_render` / `vello_*` / `stencil` readback 路径
   - **`GPURenderContext.Close`** 必须 `drainOffscreenPool`（`offscreen_cache`）+ 释放 `frameScratch`
   - **`StencilRenderer.destroyPipelines`** 必须 Release `coverBlendPipelines`（如 `cover_pipeline_blend_Plus`）
   - 注入/自测：优先 `Swapchain.ForceRecoverHealthy`（健康 abandon+Release），避免 sticky `MarkLost` 钉堆
   - 验收：`GPUI_SCENARIO=S12 GPUI_FORCE_LOST_AFTER=45` 后 nvidia-smi 进程显存保持 ~322MiB、`CreateTexture OOM`=0、帧继续；lifecycle selftest `exit=selftest_ok recoveries>=1`
5. **已删除（soft 后无用）**  
   - GCT 前 WriteBuffer canary probe  
   - SIGABRT longjmp `gct_guard`  
   - 遮挡恢复强制 MarkLost / soft 失败 streak MarkLost 启发式  

6. **明确不做**  
   - 不调用 `GetLostFuture`  

### 3.2 Host（`examples/exboot` + 各示例）

| 条件 | present / acquire（与 `SurfaceHost` / 手写 pause 对齐） |
| --- | --- |
| 最小化 / Unmap / FullyObscured / 几何全盖住 | **暂停 present**；**auto/Purge/Recreate**：`Unconfigure` + DropGPU（见 lifecycle）；**Normal 档不 Unconfigure** |
| 仅失焦、仍可见 | **继续画**（FocusOut ≠ hidden） |
| `ErrDeviceLost` | skip + AutoRecover，**不** `os.Exit` |
| 恢复可 present | force full / reconfigure 或 `ForceRecoverHealthy`；recover 回调 **`DropGPURenderContext`** |

共享启动：`examples/exboot`（`InitEnv` / `NewInstanceX11` / `OpenDevice` / `WireAutoRecover`）。  
自适应策略：`exboot.SurfaceHost`（`lifecycle.go`）。

| 示例 | 接入方式（2026-07-21 源码） |
| --- | --- |
| `api_coverage_app` · `mem_anim_window` · `antd_desktop_app` · `flutter_app_shell` | **SurfaceHost** + WireAutoRecover |
| `app_lifecycle_shell` · `capability_matrix` · `window_present` · `particle_kitchen_sink` | WireAutoRecover；pause/Unconfigure **手写**（未统一 SurfaceHost） |
| `device_lost_redraw` | 手写 Unconfigure + DropGPU（参考路径） |
| `mem_window_stress` | 离屏；`InitEnv` only |
| `vram_stages` | 诊断 |

恢复可 present：画**当前最新**状态（非暂停前缓存帧）。

### 3.3 文档与测试

- 本文件：问题 / 方案 / 验收（**已实现**）  
- 表面策略：[`SURFACE_LIFECYCLE_SKIA_FLUTTER.md`](./SURFACE_LIFECYCLE_SKIA_FLUTTER.md)  
- 持续缺口：[`ENGINE_GAPS.md`](./ENGINE_GAPS.md) G3  
- 门禁：

```bash
export LD_LIBRARY_PATH=$PWD/lib
go test ./gpu/rwgpu -run 'DeviceLost|Lost|Uncaptured|ForceNative|SyncLost|SurfaceParent|Registration' -count=1
go test ./gpu/webgpu -run 'Lost|Force|PrepareDevice|RecoverKeeps' -count=1
go test ./render/gpu -run 'Lifecycle|TextureOOM' -count=1
```
---

## 4. 验收步骤（人工）

环境：Ubuntu 22 + X11；`export LD_LIBRARY_PATH=$PWD/lib`。

```bash
GPUI_SCENARIO=S23 go run ./examples/mem_anim_window
# 或: GPUI_SCENARIO=S23 /tmp/mem_anim_window
```

| 步骤 | 操作 | 期望 |
| --- | --- | --- |
| 4.1 | 启动 | 动画正常；`frame=` 持续增长 |
| 4.2 | **仅失焦**（点旁窗，测试窗仍露出） | **继续绘制**；无 `geom_covered=true` 误暂停 |
| 4.3 | **完全挡在其他窗口后** | 日志可有 `geom cover changed: covered=true` / `pause present`；**不 SIGABRT** |
| 4.4 | 挡着等 ≥5–10 min | 进程存活 |
| 4.5 | 可选：最小化 → 还原 → 再完全挡住 | 同 4.3–4.4 |
| 4.6 | **最后一次完全遮挡后 ≥20 min 不崩** | 验收通过 |
| 4.7 | 重新露出窗口 | `window visible again`；继续 `frame=`；画面为**当前最新状态** |

**禁止**用 `GPUI_FORCE_RENDER_WHEN_HIDDEN=1` 作为正式验收（仅调试库错误路径）。

被动验收时：**不要**对测试窗做 restack / 周期性改 WM 属性 / 高频 xwd；自动化会改变复现条件。

---

## 5. 关键文件

| 路径 | 角色 |
| --- | --- |
| `gpu/rwgpu/device.go` | sticky lost、SyncLostState（pump+uncaptured）、Destroy |
| `gpu/rwgpu/surface.go` | GCT status/Uncaptured → error、`abandoned` |
| `gpu/rwgpu/safety.go` / `buffer.go` / `types.go` / `wgpu.go` | gate、WriteBuffer soft-lost、Destroy 绑定 |
| `gpu/webgpu/device.go` / `surface.go` / `swapchain.go` | FlushCallbacks、AutoRecover、ClearRecoverCooldown |
| `examples/exboot/boot.go` · `lifecycle.go` | InitEnv / OpenDevice / WireAutoRecover · **SurfaceHost** 三档策略 |
| `examples/mem_anim_window` · `device_lost_redraw` · `api_coverage_app` · `antd_desktop_app` · `flutter_app_shell` | Unconfigure + DropGPU + lifecycle selftest |
| `render/context_gpu_registry.go` · `render/accelerator.go` | 全 Context abandon / purge |
| `render/gpu/lifecycle_policy.go` · `gpu.go` | NoteTextureOOM · Purge hooks |
| `gpu/webgpu/lifecycle_hooks.go` · `swapchain.go` | AfterSurfaceUnconfigure · ForceRecoverHealthy · VRAM probe |
| `gpu/rwgpu/force_native_lost_test.go` 等 | 强制 Destroy / 隔离 / 拒 GCT 单测 |

---

## 6. 残余限制（与 ENGINE_GAPS G3 对齐）

1. 需使用 **soft 补丁** `libwgpu_native.so`；旧 fatal `.so` 仍会 SIGABRT。  
2. DeviceLost 回调 Destroy 路径仍可能不投递 → sticky + Uncaptured 仍需要。  
3. Host **完全遮挡 pause present** 仍建议保留（减 TDR）。  
4. 几何遮挡依赖 X11 stacking；极端 WM 靠 FullyObscured。  
5. recover 后必须 abandon Context GPU 缓存（引擎注册表已做；host 仍应 DropGPU）。  
6. **持续**：重层 + 多 RT + force-lost soak（非未实现 API，见 G3）。

---

## 7. 原任务步骤对照（已校正）

| 原描述 | 校正后策略 |
| --- | --- |
| 完全遮挡仍“正常绘制动画不是暂停” | **逻辑/数据可更新**；**GPU present 在不可 present 时暂停**；恢复后画**最新**状态（Skia 模型）。全遮挡仍 full-rate GCT 会制造 TDR 崩溃。 |
| 命令 `PUI_SCENARIO=23` | `GPUI_SCENARIO=S23` + `LD_LIBRARY_PATH=lib` |
| 崩溃即失败 | SIGABRT / 进程退出 = 失败；pause present + 恢复后继续 frame = 成功路径 |

---

## 8. 一句话

**Skia 式：Context 可 abandon+recreate；Surface 不可 present 则不 acquire；Device lost 是状态不是崩溃。**  
本树：soft native 报错 → sticky abandon + AutoRecover；host 不可 present 时 pause，失焦仍可见继续画，恢复 force 最新帧。
