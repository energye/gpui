# D1 解剖：queue_workdone 测试失败后 cleanup Release SIGSEGV

> 台账来源：§2.2 D1（P1.3 T1 登记，归属 P2.4c/P2.4e）。诊断日期 2026-11-10。
> 结论：**根因定位且已由 v29 对齐修复消除，引擎无需再改代码**；处置 = 销账 + 布局防回归钉。

## 现象（P1.3 原始登记）

- `gpu/rwgpu/queue_workdone_test.go:45` `OnSubmittedWorkDone: wgpu: Queue.OnSubmittedWorkDone: wgpu returned null future` → `t.Fatalf`；
- cleanup `Device.Release` 在 `device_misc.go:313→314`（经 `safety.go:285` `releaseNativeHandle` → purego）SIGSEGV：
  **PC=0x1 / addr=0x1 / rip=0x1**，崩溃链 `Device.Release.func1 → unixProc.Call → purego.SyscallN` → native 内跳地址 1。
- 原始栈正本：`docs/render_gpu_converge/baseline/tests/failures/queue_workdone_test.log`。

## 根因机制（复现验证，非推测）

1. **布局错位**：92ca9216（P1.3 基线）时代 `queueWorkDoneCallbackInfo` 是 pre-v29
   **四字布局（缺 `Mode` 字段）**；v29 真源头文件 `lib/include/webgpu/webgpu.h:1677-1688`
   是**五字按值传递**：`nextInChain / mode / callback / userdata1 / userdata2`（40 字节，
   头注释明说 mode "Has no default"）。
2. **字段错位一格**：wgpu 按五字读我们的四字结构 → wgpu 的 `callback` 槽拿到我们填的
   `Userdata1`（记录 id，该测试是该进程首次调用 ⇒ **恰好 = 1**），`mode` 槽拿到真回调函数指针。
3. **两症状同一根源**：`wgpuQueueOnSubmittedWorkDone` 返回 null future（登记消息来源）；
   `Device.Release` 结算 pending 回调时跳转到「callback=地址 1」→ SIGSEGV PC=0x1。

## 复现证据（2026-11-10，现行 lib + 92ca9216 布局重放）

- 复现方式：包内临时测试按老四字类型 `RegisterFunc` 调 `wgpuQueueOnSubmittedWorkDone`
  （callback=真回调、Userdata1=1），不 Fatal、直接 `device.Release()`。
- 实测输出（`d1-repro-oldlayout.txt`，同目录）：
  - `native future = 0`（null future 复现）；
  - SIGSEGV **PC=0x1 sigcode=1 addr=0x1**，调用链 `device_misc.go:313 → safety.go:285 → :314`
    与 P1.3 原始栈逐帧同构、寄存器现场逐位一致。
- 复现用临时测试文件已删，不入库（机制与输出以本文与 txt 为准）。

## 修复现状（引擎代码已对齐，无需再改）

后续 v29 对齐已消除根因（`gpu/rwgpu/queue_workdone.go`）：

- `queueWorkDoneCallbackInfo` 补 `Mode CallbackMode` 字段（:40-46，五字布局）；
- 回调签名改 v29 五参（status/message.data/message.len/userdata1/userdata2，:69-82）；
- `QueueWorkDoneStatus` 枚举改 v29 值（Success=1/CallbackCancelled=2/Error=3，:26-34）；
- `OnSubmittedWorkDone` 接受 v29 的零 native Future（v29 已知行为：回调仍触发、future 为 0），
  返回自有记录 id（:96-131）。

当前 HEAD 实测：整文件 4 用例全 PASS（带 lib，8.9s）；SIGSEGV 不再复现。

## 处置（用户拍板 2026-11-10：A 销账 + 布局钉）

- 台账 §2.2 D1 行 ⬜ → ✅（依据：根因定位 + 已修 + 复现证据）。
- 防回归钉（零行为改动）：`gpu/rwgpu/queue_workdone_test.go::TestQueueWorkDoneCallbackInfoV29Layout`
  —— 纯 Go 断言 `queueWorkDoneCallbackInfo` 尺寸 40 / 字段偏移 0/8/16/24/32 = v29 头、
  `CallbackModeAllowProcessEvents=0x2`、`Future` 尺寸 8 偏移 0（`WGPUFuture{uint64 id}` ABI 兼容）。
