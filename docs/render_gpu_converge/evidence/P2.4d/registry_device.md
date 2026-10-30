# P2.4d 第二段：registry 设备级化 + owner 分账

日期：2026-10-30
前置：`hal_token.md`（hal 完成 token，五端全绿）
范围：只做归属与释放口径，不迁移六处纹理池（后续段）

## 1. 改了什么

`res.Registry` 从「每 session 一份」升为「每 device 一份」（Skia GrContext：一个
resource cache，N 个 surface）：

- `OwnerID` / `Owner`：owner 作用域句柄。`OwnerDevice=0` 给设备共享资源；
  每个 session 用 `Registry.NewOwner()` 领一个 id。
- `slot.owner`、`roleKey.owner`：同一 `SourceKey` 在不同窗口互不串台（这是
  合并 registry 后必须补的隔离点，否则两个窗口的 resolve 会互相顶掉）。
- `Registry` 全方法加 `sync.Mutex`：设备级后被多窗 raster 线程共享。
  idle 回调（`onIdle`／Cache 回池）一律在解锁后执行，回调可安全重入 registry。
- 按 owner 的两个出口：
  - `Owner.ReleaseOwned()`：session `Destroy` 用，只释放本窗 native（device 仍健康，前提是已 drain）。
  - `Owner.InvalidateOwned()`：设备丢失用，丢簿记不释放 native（native 拆除归 abandon 单一出口）。
  - 保留 `ReleaseAll()` / `InvalidateAll()` 给设备整体拆除/设备级丢失。
- Camera-ready 诊断：`Owner.Count()` / `Owner.PendingReleaseCount()` 按窗统计；
  全局 `Count()` / `PendingReleaseCount()` 仍给设备级对账。
- `res.Cache` 构造改 `NewCache(owner Owner, create, budget)`；Acquire 的池命中
  与 trim 改走 registry 原子助手（`acquireIfIdle` / `idleEntry` / `retireEntry`），
  不再直读 `slots`（多窗并发下 TOCTOU）。
- `GPUShared.resReg`：设备级 registry 唯一持有者，`NewGPUShared` 创建；
  `abandonDeviceOwnedLocked` 只清 `OwnerDevice` 簿记（共享缓存 native 由拆除
  流程自己销毁，避免二次释放），session 条目留给各自 `Destroy`。
- `GPURenderSession`：
  - `NewGPURenderSession` 仍隔离（测试/独立 session）；
  - 新增 `NewGPURenderSessionWithRegistry`，生产两处建 session
    （`gpu_render_context.go` ensureGPU、`filter_flush_coalesce.go`）传 `shared.resReg`；
  - session 持 `resOwner`；`Destroy` 由 `ReleaseAll` 改 `ReleaseOwned`；
    `InvalidateForDeviceLoss` 改 `InvalidateOwned`；
  - `Res()` 暴露 owner 句柄，`Reg()` 保留给诊断（NativeOf / 计数）。
- 构造器对 nil registry 兜底：用裸 `GPUShared{}` 字面量的测试上下文仍拿到
  独立 registry，不会退化成「立即释放」。

## 2. 行为差异（有意）

- 关一个窗不再清空全设备 registry：只释放该 owner 的条目，其余窗口的
  resolve/role 不受影响（此前每窗独立 registry，不存在此交叉）。
- 每窗诊断计数不再包含其他窗口的条目。
- 六处纹理池本身的分配策略未变（下一步迁移）。

## 3. 验证

| 项 | 命令 | 结果 |
|---|---|---|
| res 单测（逐文件） | 6 组 `go test ./render/internal/gpu/res -run ...` | 全 PASS |
| owner 新单测 | `TestRegistry_OwnerIsolation / InvalidateOwnedSkipsRelease / IdleCallbackMayReenter / ConcurrentOwners` | PASS |
| res `-race` | `CGO_ENABLED=1 go test ./render/internal/gpu/res -race` | PASS |
| 共享 registry 会话 | `TestSessionRegistry_SharedDeviceOwnerScope / InvalidateForDeviceLossScoped` | PASS |
| 原有寄存器单测 | `TestRegTransientView_* / TestRegBorrowed_*`（native 设备） | PASS |
| 无设备路径 | `TestP6_* / TestViewToResView_* / TestResolveCommandView_*` | PASS |
| 构建 | `go build ./render/... ./gpu/...` | 绿 |
| vet | `go vet ./render/internal/gpu/res ./render/internal/gpu` | 仅既有告警（unsafe.Pointer / unreachable） |

锁序证据：`TestRegistry_IdleCallbackMayReenter` 在 idle 回调里调用
`reg.Count()`；若回调在锁内执行必死锁，该测试 2s 超时即红。

## 4. 遗留（后续段）

- 六处同尺寸池迁移到 `Cache`（surface trio / stencil / offscreen / filter / dualtex）。
- `Cache` 能力补齐（SizeFn / MaxPerKey / PurgeIdle / 按域统计）。
- filter/dualtex 的提交完成 token 闸门接入。