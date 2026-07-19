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
| Native | GCT 在 parent 已 lost 时 **fatal**；WriteBuffer/CreateBuffer 多为 **soft** uncaptured |
| Native | `wgpuDeviceGetLostFuture` **未实现**，禁止绑定调用 |
| Native | DeviceLostCallback 在 Destroy 等路径上 **常不进入 Go** |
| Binding | 仅靠 callback 不够；必须 sticky lost + **GCT 前 refuse** |
| Binding | Soft WriteBuffer canary 在部分 TDR 路径上仍有 **假阴性**（探测 Alive 后 GCT 仍 abort） |
| Host | 遮挡时仍 full-rate acquire 会制造 lost；**不可 present 的 surface 上不应 acquire** |
| 错误方向 | 用 SIGABRT longjmp 包 GCT：**不可靠**（abort 线程/CGO 边界），实测 **futex 假死**，已废弃 |

---

## 2. Skia / Flutter 目标模型（本仓库对齐的策略）

| 实践 | 含义 |
| --- | --- |
| Sticky per-device lost | 每逻辑 Device 独立 lost，不 mark-all |
| Refuse GPU work | lost 后 create/submit/acquire 返回结构化错误，**禁止**再调 fatal native |
| Lost-safe teardown | Release/Unconfigure 在 lost/abandoned 时跳过危险 native |
| Abandon + recreate | 下一帧 `RequestDevice` + surface reconfigure + 上层 rebind |
| **遮挡 ≠ device lost** | 最小化 / 完全挡住 = **surface 不可 present → 暂停 acquire**；真 TDR 才 sticky lost |
| **失焦 ≠ 暂停** | 窗口仍露出时，有数据更新必须继续 present（可降帧） |
| 恢复可见 | 用 **当前最新状态** 整帧画上去（dirty + force full），不是遮挡期间硬 GCT |

遮挡期间屏幕上是合成器旧帧；**最新数据在 CPU/场景状态中更新，恢复 present 后画出来**。

---

## 3. 解决方案（已实现）

### 3.1 Binding：`gpu/rwgpu` + `gpu/webgpu`

1. **Sticky lost**  
   - DeviceLostCallback + uncaptured `looksLikeDeviceLost`  
   - `Device.lost` + `lostDeviceHandles`（Release 后仍可识别）  
   - 多设备：userdata slot + handle 路由，**永不 mark-all**

2. **GCT 前 fail-closed soft probe**（`probeDeviceForSurfaceLocked` / `SyncLostState`）  
   - ProcessEvents → canary WriteBuffer（validation error scope）→ Poll/ProcessEvents → PopErrorScope  
   - Lost → sticky + `ErrSurfaceDeviceLost`  
   - 健康不可确认（Pop 失败 / 无 canary 等）→ `ErrSurfaceTimeout`，**不调 native GCT**  
   - `Surface.abandoned`：parent lost 后 Unconfigure/Release 跳过 fatal native

3. **Destroy**  
   - 先释放 canary → force sticky → `wgpuDeviceDestroy` + Release → 清 handle

4. **Facade**  
   - `FlushCallbacks` = ProcessEvents + SyncLostState  
   - `Swapchain.EnableAutoRecover`：`RequestDevice` 成功前 **不** `sc.Device = nil`  
   - `ClearRecoverCooldown()`：恢复可见时允许立刻 recreate

5. **明确不做**  
   - 不调用 `GetLostFuture`  
   - 不用 SIGABRT longjmp 包 GCT（会假死）

### 3.2 Host：`examples/mem_anim_window`（Skia surface 生命周期）

| 条件 | present / acquire |
| --- | --- |
| 最小化 / Unmap | 暂停 |
| `VisibilityFullyObscured` | 暂停 |
| **几何完全被更高 stacking 的顶层窗口盖住**（仅 unfocused 时检测，避免 shell 全屏层误报） | 暂停 |
| **仅失焦、窗口仍部分可见** | **继续画** |
| soft acquire 连续失败（约 45 帧） | 进入保护 idle，防无 FullyObscured 的 WM |

恢复可 present 时：

- `forceFull` + `MarkNeedsReconfigure`  
- `ClearRecoverCooldown` + 同步 `device = sc.Device`  
- 下一帧用 **当前最新场景状态** 整屏绘制  
- Device lost：`BeginFrame` 返回 `ErrDeviceLost` → 跳帧 + AutoRecover，**不** `os.Exit`

关键 API：

- `x11Win.IsFullyCoveredByOtherWindows()`：解析 WM reparent frame，只比较 **stacking 之上** 且非近全屏 shell 的窗口  
- FocusIn：清空 `windowGeomCovered`，保证失焦可见路径不被粘住
- **遮挡滞回**：一旦几何完全遮挡为 true，检测变 false 后仍保持 pause **3s**（防 stacking 抖动导致仍 full-rate present → TDR）
- **失焦降帧**：未完全遮挡但 unfocused 时默认 `GPUI_UNFOCUSED_FPS=15` 继续画，降低 TDR 风险

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
| `gpu/rwgpu/device.go` | sticky lost、soft probe、Destroy、SyncLostState |
| `gpu/rwgpu/surface.go` | GCT 前 probe/refuse、`abandoned` |
| `gpu/rwgpu/safety.go` / `buffer.go` / `types.go` / `wgpu.go` | gate、WriteBuffer soft-lost、canary 字段、Destroy 绑定 |
| `gpu/webgpu/device.go` / `surface.go` / `swapchain.go` | FlushCallbacks、AutoRecover、ClearRecoverCooldown |
| `examples/mem_anim_window/main.go` | EnableAutoRecover、几何完全遮挡 pause、恢复 forceFull |
| `docs/WGPU_NATIVE_DEVICE_LOST.md` | native 契约 + host 职责 |
| `gpu/rwgpu/force_native_lost_test.go` 等 | 强制 Destroy / 隔离 / 拒 GCT 单测 |

---

## 6. 残余限制

1. 本 `.so` 无法把 fatal GCT 变成可靠 Go error；**唯一安全策略是 lost/不可 present 时不调 GCT**。  
2. Soft probe 在部分 TDR 上仍可能假阴性 → host **完全遮挡 pause present** 是必要防线。  
3. Soft probe 每帧少量 WriteBuffer 成本可接受。  
4. 几何遮挡依赖 X11 stacking；极端 WM 布局需靠 FullyObscured / soft-fail idle 兜底。  
5. 更换/升级 `libwgpu_native` 使 GCT soft 化或 GetLostFuture 可用后，可减弱 probe 与 host 遮挡依赖。

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
本树：binding sticky+fail-closed 拒 GCT + swapchain AutoRecover；host 仅在不可 present 时 pause，失焦仍可见继续画，恢复时 force 最新帧。
