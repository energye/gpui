# P2.4d 前置：hal 完成 token（D3）

日期：2026-10-30
范围：`gpu/hal`、`gpu/rwgpu`、`gpu/webgpu`、`gpu/gwgpu/gles`、`gpu/gwgpu/metal`、`gpu/hal/noop`
状态：实现完成，单测/构建/双向交叉编译全绿；尚未进入六池迁移。

## 1 目标

给 `hal.Queue` 一个后端无关的「提交完成」原语，让资源层（`res`）能在 GPU 真正用完某个原生资源之后才回收/销毁它。这是 P2.4d 六池收敛的前置（D3 决策）：filter / dualtex 的池是进程级对象、且 `queue.Submit` 是函数内直提，不进 session 的 submission 记账，只能靠完成 token 给闸门。

## 2 新增接口

```go
// gpu/hal/api.go
type CompletionToken interface {
    Done() bool  // 非阻塞；后端可顺带泵自己的回调
    Wait() error // 只给关停/拆除路径用
}

type Queue interface {
    ...
    OnSubmittedWorkDone() (CompletionToken, error)
}
```

语义：token 覆盖「调用时刻之前已提交的全部工作」，之后的提交不延长它。调用方必须消费（Done/Wait），后端靠消费释放完成记录。

各端实现：

| 后端 | 完成判据 | 文件 |
|---|---|---|
| webgpu-native | rwgpu 回调记录（`Device.FlushCallbacks` 泵回调）+ `rwgpu.FutureDone` 非阻塞查 | `gpu/webgpu/queue.go` |
| gles（windows/linux） | 捕获提交序号，`Poll()`（GL sync `Fence.GetLatest`）追上即完成 | `gpu/gwgpu/gles/queue_token.go` |
| metal | 捕获提交序号，`completedIndex` 追上即完成 | `gpu/gwgpu/metal/queue.go` |
| noop | 同步后端，立即完成 | `gpu/hal/noop/queue.go` |

## 3 顺带查实的三个既存缺陷（T1 根因）

`TestQueue_OnSubmittedWorkDone` 自 P1.1b 起就失败并伴随清理阶段 SIGSEGV，当时按「既有问题 T1」冻结。P2.4d 需要 native token，必须把根因查清。用「头文件 + C 垫片实测」两证据定位：

### 3.1 回调信息结构体缺 `mode` 字段（v29 ABI）

`lib/include/webgpu/webgpu.h:1677-1688`：

```c
typedef struct WGPUQueueWorkDoneCallbackInfo {
    WGPUChainedStruct * nextInChain;
    WGPUCallbackMode mode;          // ← Go 侧缺这一格
    WGPUQueueWorkDoneCallback callback;
    void* userdata1;
    void* userdata2;
} WGPUQueueWorkDoneCallbackInfo;
```

Go 结构体少了 `mode`，后续字段整体前移，native 读到的 callback 指针无效。已按头文件补齐（`gpu/rwgpu/queue_workdone.go`），并选 `AllowProcessEvents`，使 `Device.Poll / FlushCallbacks`（`wgpuInstanceProcessEvents`）能推进完成——token 的非阻塞 `Done` 依赖这一点。

### 3.2 回调函数签名少了 `WGPUStringView message`

头文件 `:1468`：

```c
void (*WGPUQueueWorkDoneCallback)(WGPUQueueWorkDoneStatus status,
                                  WGPUStringView message,
                                  void* userdata1, void* userdata2);
```

SysV 下 `WGPUStringView` 占两个整数寄存器。旧 handler 按 `(status, userdata1, userdata2)` 读，`userdata1` 实际落在第 4 个参数上，完成记录永远匹配不到。已改为 5 参（两个 message word 丢弃），与同仓 `adapterCallbackHandler` / `mapCallbackHandler` 的既有写法一致。

### 3.3 状态枚举还是 v29 之前的数值

头文件 `:842-854`：`Success=1, CallbackCancelled=2, Error=3`。Go 侧还是 `iota`（`Success=0`），成功回调会被判成错误。已按头文件改值、同步 `QueueWorkDoneStatusName` 与单测。

### 3.4 wgpu-native v29 的 `wgpuQueueOnSubmittedWorkDone` 返回空 future（实测）

C 垫片（直接用头文件编译、绕开 purego）调用结果：

```
C-SHIM future=0 after ProcessEvents fired=1
C-SHIM future=0 after device.Poll(false) fired=1
```

即：回调会正常触发（status=1 Success），但返回的 `WGPUFuture.id` 恒为 0。这不是 purego 的锅，是库自身行为。因此 token **不能**依赖 native future id：

- `rwgpu.Queue.OnSubmittedWorkDone` 返回的 `Future.ID` 改为本模块自己的完成记录 id；
- 完成判定走 `FutureDone`（查记录），`WaitForFuture` 自己泵 `Instance.ProcessEvents`；
- 记录里保留 `nativeID` 仅作诊断。

附带结果：`TestQueue_OnSubmittedWorkDone` 由「失败 + 清理段 SIGSEGV」变为 PASS，清理路径不再崩溃。

## 4 验证

命令与结果（Linux，`WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`）：

| 项 | 命令 | 结果 |
|---|---|---|
| rwgpu 回调/非阻塞 token | `go test ./gpu/rwgpu -run 'TestQueue_OnSubmittedWorkDone\|TestQueueWorkDoneStatusName\|TestQueue_FutureDoneNonBlocking'` | 3 PASS |
| webgpu token | `go test ./gpu/webgpu -run TestQueueCompletionToken` | PASS |
| noop token | `go test ./gpu/hal/noop -run TestQueueCompletionToken` | PASS |
| 构建 | `go build ./gpu/... ./render/...` | 绿 |
| vet | `go vet ./gpu/hal/... ./gpu/rwgpu/... ./gpu/webgpu/... ./gpu/gwgpu/gles/...` | 净 |
| gles windows 交叉编译 | `GOOS=windows GOARCH=amd64 go build ./gpu/gwgpu/gles/...` | 绿 |
| metal darwin 交叉编译 | `GOOS=darwin GOARCH=arm64 go build ./gpu/gwgpu/metal/...` | 绿 |

gles token 的完成判据复用现有 `Poll()` 契约（有 fence 时读 GL sync，无 fence 时等于提交序号），真机行为随 P2.4d 全量门禁的 GL 窗一起验。

## 5 临时产物

`/tmp/probe_workdone.c`（C 垫片）与 `/tmp/libprobe_workdone.so` 仅用于本次根因定位，未入库；仓内临时探针 `gpu/rwgpu/zz_tmp_probe_test.go` 已删除。