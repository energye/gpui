# R2 审查报告：`gpu/` + `render/gpu/`（wgpu/webgpu 后端封装层）

> 审查依据：前期调查记录 `.findings_gpu_backend.txt` + 本轮对 5 条关键线索的源码逐条核实（均已给出 文件:行号）。
> 范围：`gpu/rwgpu`（FFI 层）、`gpu/webgpu`(门面层)、`gpu/types`、`gpu/shader`（uniform 布局相关）、`render/gpu`（策略薄层）、`gpu/context`。

---

## 一句直话结论

**这层能用、单窗口跑真窗没问题，但现在还不能叫工业级：有一个会读到已释放内存的真 Bug（P0），一个会让整个 GPU 线程卡死的结构性风险（P1），外加一批"声明了但没做完"的半成品（uniform 对齐）。修完 TOP5 之前，不建议对外宣称支撑工业级 GUI 组件库。**

分层架构本身是健康的：`rwgpu`（纯 FFI）→ `webgpu`（带错误恢复的门面）→ `context/render/gpu`（策略），职责清楚，设备丢失恢复、泄漏跟踪、软错误 .so 这些工程化设计都做得比一般自研项目认真。问题集中在**边界处的 unsafe 用法**和**几处只写了一半的规范实现**。

---

## 分维度发现

### 1. 正确性 / 隐藏 Bug

| # | 问题 | 位置 | 级别 |
|---|------|------|------|
| C1 | **`stringViewToString` 返回指向 C 内存、却注释说"已拷贝"**。`unsafe.String((*byte)(ptr), len)` 只是造了一个指向 C 内存的 Go 字符串头，没有任何拷贝。调用方在 `Adapter.Info()` 里用它填 `info.Vendor/Device/Description`（adapter.go:532-543），随后 ：546 就调 `procAdapterInfoFreeMembers` 把 C 内存释放了——之后读这些字段就是 **use-after-free**（内容可能是垃圾或崩溃）。对比同文件的 `cStringAt`（:574-577 用 `string(b[:n])` 真拷贝）说明作者知道正确做法，只是显式长度这条路径漏了。 | gpu/rwgpu/adapter.go:556-571（尤其 ：570）；消费点 ：532-546 | **P0** |
| C2 | **acquire 超时的"弃子协程"持有全局锁不还**。超时后 Swapchain 把卡住的 cgo 调用协程丢弃（swapchain.go:975-1000），但该协程在 `rwgpu.Surface.GetCurrentTexture` 里已经 `gpuMu.Lock()`（surface.go:274-275）。如果 native 调用真的永远不返回，锁永远不释放；而恢复路径 `reconfigureThrottled → Configure` 同样要拿这把全局锁——**恢复逻辑自己就会死锁**。注释声称"frame loop never blocks"，只在 native 最终会解开的场景成立。 | gpu/webgpu/swapchain.go:975-1020；gpu/rwgpu/surface.go:263-280 | **P1** |
| C3 | **WGSL uniform 地址空间 16 字节数组步长没实现**。lower 层自己注释承认："uniform buffer 16-byte array stride requirement is enforced by the WGSL spec … but the general type layout uses natural alignment"。后果：`var<uniform> lut: array<f32, 8>` 这类标量数组按步长 4 布局，而 WGSL/Vulkan uniform 要求步长必须是 16 的倍数——合规驱动上 CreateBindGroup/pipeline layout 校验直接失败，宽松驱动上则是静默数据错位。 | gpu/shader/lower.go（约 ：822-825 注释处，自然对齐路径） | **P1** |
| C4 | **MatrixStride 用自然对齐，uniform 下 mat2xN 错**。SPIR-V 后端给矩阵成员发 ColMajor+MatrixStride，列步长 = 行数×4（Vec2→2×4=8）。WGSL uniform 地址空间要求 f32 矩阵列步长一律 16；mat2xN 会拿到 8，Vulkan 校验不过或数据错位。（mat3x/mat4 恰好算出 16，所以平时测不出来。） | gpu/shader/spirv/internal/codegen/backend.go:1622-1635 及 ：550-551 | **P1** |
| C5 | **`uniformStructTypes` 是死代码**：声明、make、clear 三处出现（backend.go:156、:196、:236），从未写入任何条目。它注释里写着"用于套 std140 MatrixStride 规则"——正是 C3/C4 该用的钩子。这是"规范实现做了一半"的铁证：框架搭好了，线没接。 | gpu/shader/spirv/internal/codegen/backend.go:154-156,196,236 | **P1**（与 C3/C4 合并修） |
| C6 | **`Buffer.Map` 轮询协程是无睡眠忙等**：`for { select done / dev.Poll(false); runtime.Gosched() }`（map_pending.go:193-203）。映射期间烧满一个核；若设备丢失导致 map 永不完成，协程**永久空转**且无人回收。ctx 取消只让调用方返回，协程照转。 | gpu/rwgpu/map_pending.go:190-204 | **P1** |
| C7 | **`RequestDevice` 静默丢弃 `RequiredFeatures`**：桌面门面只转换 Label 和 Limits（adapter.go:47-53），应用要求的 feature（如 TimestampQuery）根本没传下去，请求照常成功、后面用到才在校验层炸。浏览器版反而实现了（adapter_browser.go:48）。 | gpu/webgpu/adapter.go:40-59 | **P2** |
| C8 | **Limits 字段丢失**：`types.Limits` 有 `MaxPushConstantSize`、`MaxNonSamplerBindings`（types/limits.go:70-73，DefaultLimits 设了 1000000），但 `convertToLimits` 只映 30 个标准字段，这两个被静默扔掉——`DeviceDescriptorLowVRAM` 把 NonSamplerBindings 调到 10000 的策略实际从未到达 native。 | gpu/webgpu/adapter.go:118-151；gpu/types/limits.go:113-114 | **P2** |
| C9 | **恢复路径无条件 `fmt.Printf` 泄漏报告**：`ForceRecoverHealthy` 里同样的打印有 `GPUI_DEBUG_GPU` 门禁（swapchain.go:585-592），`tryRecoverDeviceLocked` 里没有（:706-710），生产环境每次设备恢复都往 stdout 刷日志。复制粘贴漏改。 | gpu/webgpu/swapchain.go:706-710 | **P3** |
| C10 | **Swapchain 统计字段数据竞争**：acquires/presents 等在 BeginFrame/EndFrame 里持 frameMu 写入，但 `Stats()`/`ResetStats()` 无锁读写——race detector 必报。仅影响诊断数据，不影响渲染正确性。 | gpu/webgpu/swapchain.go（Stats/ResetStats 与 begin/end 对照） | **P2**（诊断类） |

### 2. 驱动兼容性

- **C3/C4/C5 合起来是最大的驱动兼容雷**：shader 编译器产出的 uniform 布局不合 WGSL 规范。宽松驱动（部分桌面 GL/Vulkan）可能侥幸过，严格驱动（MoltenVK、较新 Vulkan validation、ANGLE）直接校验失败。这类 bug 的特点是"在我机器上好好的"，上线到用户硬件才爆。
- 正面项：软错误 .so（patched wgpu-native 避免 SIGABRT）、设备丢失 sticky 标记、`tryRecoverDeviceLocked` 里实测记录的 wgpuDeviceDestroy→永久 OOM 规避策略（swapchain.go:639-644 注释），说明团队已经在真机上趟过兼容性坑并沉淀了对策，这部分做得不错。
- 交换链格式选择偏向 BGRA8Unorm/Opaque，未显式处理 SRGB 变体，不同平台伽马可能不一致（P3，UI 场景影响有限）。

### 3. 性能

- **全局 `gpuMu` 一把锁串所有窗口的所有 native 调用**（surface.go/buffer.go/queue 各入口）：正确性安全，但多窗口场景 GPU 吞吐退化为串行，是扩展性天花板（P2）。
- C6 的忙等轮询是纯浪费的 CPU 烧法，改成 1ms sleep 或 condvar 即可。
- 每帧 `FlushCallbacks → ProcessEvents` 泵事件频率偏高；encoder 层 SetViewport 有缓存是好细节。
- 单窗口典型 UI 负载下性能不是瓶颈，问题都在多窗口/高负载时显现。

### 4. 资源占用

- 设备丢失时默认 skip native release，VRAM 会钉住直到进程结束（有 forceNativeReleaseOnLost 开关和恢复路径兜底，属已知取舍，P3）。
- C2 弃子协程一旦发生，除了锁还有 native SurfaceTexture 悬挂的风险（靠 `Surface.s.current` + DiscardTexture 部分兜住，窗口很窄但存在）。
- lostDeviceHandles 粘滞表只增不减，长期多次恢复会缓慢增长（P3，量级很小）。

### 5. 可读性与冗余（webgpu 与 rwgpu 两套关系）

- 三层结构（rwgui FFI → webgpu 门面 → context 策略）+ 浏览器侧 `*_browser.go` 平行文件，层次本身合理，但**门面层的枚举/结构转换代码与 rwgpu 的 wire 转换存在双份维护**（如 bindgroup entry 两边各转一遍），改 ABI 时容易漏一边——C8 就是这种"两边不对齐"的直接产物。
- 死代码/半成品不止 C5：`convertRenderPassDescriptorRust` 等疑似无消费者的重复转换函数；no-op 的 Fence 实现（CreateFence 永远 signaled）属于"API 说谎但有注释"。
- 注释质量整体很高（大量 Skia/Flutter/Chrome 对标引用、实测记录），这在同类项目里少见，值得肯定；主要扣分点是**注释与行为不符**（C1 的"已拷贝"是最危险的一例——错误注释比没注释更害人）。

### render/gpu 说明

`render/gpu` 是纯策略薄层（低配描述符、生命周期钩子如 AfterSurfaceUnconfigure），自身无大问题；它的 LowVRAM 策略被 C8 架空属于上游 bug。

---

## 能否支撑工业级 GUI 组件库？

**现在不能，差距是明确的、可收敛的。** 架构方向（分层、软错误、丢失恢复、对标 Skia/Flutter 的交换链重建模型）是对的，真窗验证也跑得动。挡路的不是设计，而是：

1. 一个必然踩的内存安全 Bug（C1）；
2. 一个把"恢复"变成"死锁"的结构缺口（C2）;
3. shader uniform 布局不合规范（C3-C5），决定了它只能在自己编译的 shader 上闭环，不敢对接任何外部/第三方 WGSL；
4. 工程卫生问题（C6-C10）拉低可信度。

把 TOP5 修掉并补上对应回归测试后，可以升级为"可支撑"；再解决全局锁和多窗口吞吐，才算工业级。

---

## 评分表

| 维度 | 得分（/10） | 一句话理由 |
|------|:---:|-----------|
| 正确性 | **5** | 架构与恢复模型扎实，但 C0 内存安全 Bug + uniform 布局三连错 + 多个静默丢参数，硬伤压分 |
| 性能 | **6** | 单窗口够用、局部优化用心；忙等轮询烧核 + 全局一把锁封死多窗口扩展性 |
| 资源占用 | **7** | 泄漏跟踪、VRAM 兜底意识到位；丢失钉内存/悬挂纹理窗口/粘滞表是小尾巴 |
| 可读性 | **7** | 注释密度和对标质量罕见地好；扣在注释与行为不符、半成品死代码、双层转换易漂移 |
| **总分** | **25/40** | 底子好于平均自研封装层，硬伤集中且可修，不属于推倒重来型 |

## TOP5 必修清单

1. **【P0】修 `stringViewToString`**（gpu/rwgpu/adapter.go:570）：显式长度路径改为真拷贝（`unsafe.Slice` + `string(...)`，或统一走 `cStringAt`），同步修正 :552 的错误注释；补一条 Info() 后反复读取字段的回归测试。
2. **【P1】堵 acquire 超时的锁黑洞**（gpu/webgpu/swapchain.go:975-1020 + gpu/rwgpu/surface.go:274）：要么给 cgo 阻塞调用加可中断/看门狗机制，要么恢复路径绕开全局锁（独立 surface 锁），确保"native 卡死 ≠ 进程 GPU 全卡死"；至少先文档化这个前提。
3. **【P1】补齐 WGSL uniform 地址空间布局**（gpu/shader/lower.go 自然对齐路径 + codegen/backend.go:1622-1635）：uniform 下数组步长向上取整到 16、f32 矩阵 MatrixStride=16；把 `uniformStructTypes` 真正接上或删掉；用 MoltenVK/严格 Vulkan validation 跑对照。
4. **【P1】修 Buffer.Map 忙等**（gpu/rwgpu/map_pending.go:190-204）：轮询循环加 sleep（≥1ms）或改条件变量；设备丢失时保证协程能退出。
5. **【P2】门面参数透传补全**：`RequestDevice` 传递 RequiredFeatures（gpu/webgpu/adapter.go:47-53）；`convertToLimits` 补 MaxPushConstantSize/MaxNonSamplerBindings 或显式注明不支持并去掉 types 里的假字段（adapter.go:118-151）；顺带清理 tryRecoverDeviceLocked 无门禁打印（swapchain.go:706-710）和 Stats 数据竞争。

---
*审查方式：调查记录采信 + 5 条关键线索源码逐一核实（C1/C2/C3-C5/C6/C7-C8 均已亲验 文件:行号）。*
