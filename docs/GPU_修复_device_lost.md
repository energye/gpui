# GPU device lost 修复方案（mem_anim_window / Skia·Flutter 生命周期）

> **任务**：修复 device lost 导致测试程序崩溃  
> **测试程序**：`examples/mem_anim_window`  
> **webgpu 库**：`lib/libwgpu_native.so`  
> **规范**：对标 Skia `GrDirectContext` abandon+recreate / Flutter Rasterizer surface 生命周期（策略映射，非源码拷贝）  
> **最后更新**：2026-07-20

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

5. **已删除（soft 后无用）**  
   - GCT 前 WriteBuffer canary probe  
   - SIGABRT longjmp `gct_guard`  
   - 遮挡恢复强制 MarkLost / soft 失败 streak MarkLost 启发式  

6. **明确不做**  
   - 不调用 `GetLostFuture`  

### 3.2 Host：`examples/mem_anim_window` / `device_lost_redraw`

| 条件 | present / acquire |
| --- | --- |
| 最小化 / Unmap / FullyObscured / 几何全盖住 | 暂停 |
| 仅失焦、仍可见 | **继续画** |
| `ErrDeviceLost` | skip + AutoRecover，**不** `os.Exit` |

恢复可 present：`forceFull` + `MarkNeedsReconfigure` + `ClearRecoverCooldown`，画**当前最新**状态。

### 3.3 文档与测试

- 契约：`docs/WGPU_NATIVE_DEVICE_LOST.md`  
- 本文件：任务 + 方案 + 验收  
- 门禁：

```bash
export LD_LIBRARY_PATH=$PWD/lib
go test ./gpu/rwgpu -run 'DeviceLost|Lost|Uncaptured|ForceNative|SyncLost|SurfaceParent|Registration' -count=1
go test ./gpu/webgpu -run 'Lost|Force|PrepareDevice|RecoverKeeps' -count=1
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
| `examples/mem_anim_window/main.go` | EnableAutoRecover、几何完全遮挡 pause、恢复 forceFull |
| `docs/WGPU_NATIVE_DEVICE_LOST.md` | native 契约 + host 职责 |
| `gpu/rwgpu/force_native_lost_test.go` 等 | 强制 Destroy / 隔离 / 拒 GCT 单测 |

---

## 6. 残余限制

1. 需使用 **soft 补丁** `libwgpu_native.so`（`gogpu/rwgpu`）；旧 fatal `.so` 仍会 SIGABRT。  
2. DeviceLost 回调 Destroy 路径仍可能不投递 → sticky force-mark + Uncaptured 文案仍需要。  
3. Host **完全遮挡 pause present** 仍建议保留（减 TDR，非防崩唯一手段）。  
4. 几何遮挡依赖 X11 stacking；极端 WM 靠 FullyObscured / soft-fail idle。  
5. recover 后必须清 GPU 缓存（filter bind group 等）。

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
