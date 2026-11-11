# I5 设计：GPU present 健壮化——present 出帧路径异步化 + 死线 + 弃疗（I1 后续，2026-11-11）

> 来源：§2.2 I1 取证收口（`evidence/P2.4e/i1-verification/native-stack-verification.md`：Mesa Wayland WSI `poll(-1)` 等合成器 release 永眠 + 同步 FFI 持进程级 `gpuMu` + 共享 hub = 一 surface 卡 → 全进程 GPU 冻结、不可退出）。
> 性质：设计正本（动手前先对齐标准方案与分批），实现按批次另行开工。
> 对齐参考：Chrome 把 GPU 工作隔离在独立进程、watchdog 超时杀进程重建（hang=牺牲+恢复，不是修好阻塞调用）；Flutter raster 线程同为单线程同步 present，上游对合成器 stall 的路线也是 watchdog 级恢复；wgpu-native 29.0.4 与 Mesa 均不提供可超时的 present 口——**阻塞的 FFI 无法被打断，只能「牺牲 + 隔离 + 恢复」**。

## 1 目标语义

1. **一扇窗的 present 卡死，不再冻结其它窗与进程退出**。
2. 卡死 surface 在 deadline 后被标记 lost：它自己的后续帧快速失败（fail-fast），**不占 hub**；它占用的 worker 线程成为 documented 牺牲（1 线程 + 该 surface，记账可见）。
3. 卡死可观测：counters + 一次性 CRITICAL 日志（复用 G3 引入的显式降级纪律）。
4. 正常路径行为零变化：present 成功路径的时序与错误语义保持（本批硬门禁）。

## 2 架构（seam 选型）

- **seam = render 呈现层**（`render/present_target.go` / `present_frame.go` 的 `PresentFrame` → `Swapchain.EndFrameWithDamage` 边界），不在 hal 改 API（`hal.Swapchain.EndFrame` 保持同步语义，两后端零改）；也不在 ui/raster hub 改调度（hub 仍一帧一帧跑，只是 present 那一步交给 worker）。
- **per-surface present worker**（每 surface 一 goroutine，惰性创建，present 队列深度 1）：
  - hub 线程：`PresentFrame` → 把 `(surface, EndFrame 闭包)` 投给该 surface 的 worker，**不等待**；同 surface 的下一帧操作（下一次 present/acquire 前置检查）先等 in-flight 完成（自然背压）。
  - worker 线程：同步执行现有 EndFrame 链（含 rwgpu FFI）；结果（error）写回 per-surface 状态。
  - **deadlock 不扩散的代价**：worker 卡死时该 surface 的 in-flight 永不完成 → 该 surface 后续帧 fail-fast；其它 surface 的 worker/帧互不影响（worker per-surface 是关键，per-device 会被一个卡死 surface 连坐）。
- **watchdog**（单 goroutine，render 层持有 registry）：周期扫 in-flight presents，超 deadline（默认 5s，可配）→ surface 标 `presentLost` + 一次性日志 + 计数器。
- **fail-fast 路径**：`presentLost` surface 的下一次 `PresentFrame`/帧路径直接返回既有 `ErrSurfaceDeviceLost` 族错误（复用 G3/设备恢复语义），由窗口/应用层走既有的 surface 重建/放弃路。
- **gpuMu 拆锁（rwgpu 层，批次内独立子步）**：present 不再持进程级 `gpuMu`（或改 per-device 锁）——异步化后 hub 不再在 present 上阻塞，但 present worker 里的 FFI 仍持 `gpuMu`，会把其它窗的 FFI 调用连坐；必须同批拆掉或收紧到 per-device（wgpu-native 并发安全性先实证：不同 device/surface 的调用并发跑 `-race` + 真窗）。

## 3 分批与验收

| 批 | 内容 | 验收 |
|---|---|---|
| 批 1（设计+地基） | 本设计落盘；rwgpu 侧 present 计数器/时间戳字段（presentStart/in-flight，只加不改）+ 单测；wgpu-native 并发安全实证（不同 surface 并发 present 小真窗，`-race`） | 探针绿；行为零变化（正常路径 diff=0） |
| 批 2（异步化主刀） | render 层 per-surface worker + in-flight 状态机 + fail-fast 标记；hub 的 present 调用改投递；单测：mock hal 交换链阻塞 → watchdog 判死 → 其它 surface 帧率不受影响（先红后绿） | mock 红绿对 + 三证据真窗：嵌套 Wayland ladder 卡死现场 → 其余 7 窗 fps>0、进程可正常退出（I1 现场即负向对照） |
| 批 3（gpuMu 拆锁） | present 路径去全局锁（per-device）；`-race` 全量 + 三环境矩阵（嵌套 Wayland / X11 :3 / headless Wayland） | ladder 8 窗拒增曲线 + hold 斜率在至少一个 Wayland 环境取得（I1 遗留数据落地） |
| 批 4（收口） | 牺牲线程/资源的账本对账（谁建谁放：卡死 worker 的 leak 显式登记）；catalog/总账同步；P2.6 重放 | 关窗/压测显存快照 + 全量基线对账（白名单内差异） |

## 4 风险与硬约束

- 同 surface 的「上一 present 未完成，下一帧已到」的背压语义必须保真（丢帧策略=等待，不做跳帧优化，行为零变化优先）。
- watchdog 判死是**不可逆单行道**（该 surface 只能放弃重建，不能复活）——判死阈值保守（默认 5s ≫ 正常帧时 33ms），误判风险压到最低；判死后一次性 CRITICAL 日志带现场计数。
- device 级操作（app 退出时 destroy 共享设备）可能被卡死 surface 的 Rust 内部锁连坐——批 2 文档化「带 lost surface 的退出路径」，必要时批 3 处理 device destroy 的弃疗（超时后放弃 native 释放，进程退出兜底）。
- 渲染层零后端类型（分层硬规则）：worker/状态机全在 render 层，走 hal 口。
- 每批独立分支与提交点；出问题整批回退。
