H19 修复账 + H20 登记（2026-11-05，批 6 ① 收尾时挖出；用户拍板「先修 H19 再开 ②」）
====================================================================================

一、现象与最小复现（两条测试名，同进程）
----------------------------------------
修复前（HEAD = `c3db7e57`，其 Go 面等同批 5 交付枚 `0e5d2549`）：

    go test -count=1 -run '^TestP1GLPresentX11OnlineSmoke$|^TestP37ProjectionCounters$' ./render
    → thread panicked at src/conv.rs:519:22:
      invalid bind group layout for pipeline layout descriptor
      panic in a function that cannot unwind → thread caused non-unwinding panic. aborting.
    整进程 SIGABRT，退出码 1，该点之后的测试不再执行。

整包 `./render`（带不带 `-race` 都一样）稳定停在第 624 枚 `TestP37ProjectionCounters`；单测隔离
（`-run '^TestP37ProjectionCounters$'`）跑绿 ⇒ **顺序依赖，逐文件门禁口径看不见它**。

二、探针打到的因果链（临时 H19DIAG，取完即删，工作区已核对还原）
--------------------------------------------------------------
    SetDeviceProvider   newdev=0xc000114000                 ← 窗/呈现设备 A
    ensureLayouts       dev=0xc000114000 clip=…180 mask=…198 ← 在 A 上建共享 layout
    ensureLayouts       dev=0xc000184000 clip=…180 mask=…198 ← 设备换成 B，layout 还是 A 的
    sdf.createPipeline  dev=0xc000184000 uniform=<B 新建> clip=…180 mask=…198
    → wgpu-native 非展开 panic（整进程）

一句话：**共享 clip/mask bind group layout 的缓存只看「是不是 nil」，不看「是谁家设备上的」**。
A 被 `pt.Close()` 释放后 `IsLost` 为真，`ensureGPU` 的丢设备分支原来只 `s.device = nil`
把 Go 引用一扔了之（不走 abandon），`initGPU` 紧接着装了 B，于是 `ensureSharedLayoutsLocked`
看见两枚非 nil 句柄就照用。整包只有 1 个 `J5TRACE Open device=` 记录也不矛盾——那行只追
webgpu 侧，A 是纯 Go GL 设备：**跨后端混用同一份缓存**。

三、标准依据（不是自造规矩）
--------------------------
- 本仓真源 `docs/ENGINE_GPU_RESOURCE_LIFECYCLE.md` §1.1 **D6**：「FFI 零防御……悬垂句柄直接穿到
  wgpu → 非 unwind panic → SIGABRT」，影响面写的是「任何一次悬垂句柄=进程级崩溃」——H19 就是
  这一类在共享 layout 路径上的具体实例。
- 同文档 §3.3 **R5（设备丢失规则）**：丢失/换绑要 `abandonDeviceOwnedLocked` 全量清场，
  native 释放保持**单出口**。`SetDeviceProvider` 一直照办，`initGPU` 两条重绑与丢设备分支没照办
  ⇒ 本修复是把 R5 兑现到漏掉的那三条路，不是新增规矩。
- 外部对照：Skia Graphite 所有 device-owned 对象归 `DirectContext`，换后端实例只有
  `Abandon/Destroy` 一条路，不存在「换个 device 还留着旧对象」；Impeller 掉 GPU 时整个 renderer
  拆了重建。

四、改动（`render/internal/gpu/gpu_shared.go`，64 增 13 删，零公开 API）
----------------------------------------------------------------------
1. 新字段 `sharedLayoutsDev hal.Device`：共享 layout 记下是哪台设备建的。
2. `ensureSharedLayoutsLocked`：由「非 nil 就复用」改成「**设备身份不对 ⇒ 先毁再建**」；
   毁走新加的单一出口 `destroySharedLayoutsLocked`，`destroyPipelinesLocked` 里原来手写的那
   两段销毁改成调它（同一份释放逻辑不留两处）。
3. `ensureGPU` 丢设备分支：`s.device = nil` 换成 `abandonDeviceOwnedLocked(false)`
   （`false`：设备已被它的主人释放过，不再替它 Release）。
4. `initGPU` 两条重绑（借窗设备 / 自建设备）都新增过 `installDeviceGuardLocked`：
   **只在设备身份真的变了时**才 abandon——同设备重绑不动缓存，否则「一设备 N 表面」的设计
   （`gpu_shared.go:115-120` 的注释）就被白拆。自建设备分支补 `s.externalDevice = false`：
   之前借过窗设备之后这个标记是脏的，会让我们自建的那台永远没人释放（漏一台设备）。

五、三枚钉与负向
----------------
| 钉 | 位置 | 断什么 | 负向 |
|---|---|---|---|
| `TestP376SharedLayoutsFollowDevice` | `render/internal/gpu/h19_shared_layouts_test.go` | 假设备计数：换设备必须重建（`created 2→4`）+ 旧的两枚必须毁（`destroyed 0→2`）+ 归属字段跟手 + **同设备重复调用必须幂等**（不动句柄） | 摘掉身份守卫 ⇒ **实测红**：`换设备后没重建共享 layout：created=2 want 4`；还原复绿 |
| `TestP376InstallGuardOnlyAbandonsOnDeviceChange` | 同上 | 漏斗契约：空绑定放行不动状态；同设备重绑不拆；换设备先拆（`s.device` 归 nil）；**借来的设备不许我们 Release**（`released=0`）；**自建设备换绑必须 Release**（`released=1`） | — |
| `TestP376H19DeviceSwapRunsToCompletion` | `render/p37_6_h19_test.go` | **子进程**跑最小复现序列（被钉的失败模式是 SIGABRT，同进程一 occurrence 就把整个测试二进制带走，所以只能外部断）：输出里出现 `invalid bind group layout`/`non-unwinding panic` 即红，且要求两枚真 `--- PASS`、出现 `--- SKIP:` 判空钉、要求 `ok` 行 | — |

**如实记「摘掉不红」**：单独摘掉第 3 处（丢设备走 abandon）后，整包与子进程钉仍绿——因为第 2 处
的身份守卫自己就能挡住那次崩溃。两条是**纵深**不是重复：守卫管「句柄是谁家的」，abandon 管
「旧设备的管线/池/注册表记账不留」，后者这一半当前没有会红的钉（其价值是代码路径直读 + R5 的
单出口要求），不许把它算作已被像素/行为证据收口。

六、验收（逐项实测，取状态位一律「先落文件再 `$?`」）
-------------------------------------------------
- `go build ./...` rc=0；`go vet ./render ./render/internal/gpu` rc=0；`gofmt -l` 触及 3 文件 = 0 行。
- `CGO_ENABLED=1 go test -race ./render/internal/gpu` rc=0（ok 32.907s，含两枚新钉）。
- **整包 `go test ./render` 从「第 624 枚整进程中止」变成「跑到第 1376 枚才中止」**——多跑 750 枚，
  H19 那一条彻底没了（`invalid bind group layout` 命中 0）；剩下的中止是下面 H20，存量洞。
- 逐文件 sweep（触及 `render/internal/gpu` + `render`）在册比对：见本目录 `h19-sweep.txt`。

七、H20 登记（存量洞，非本修引入；批 6 后续门禁仍受它压着）
----------------------------------------------------------
现象（两枚测试即可复现）：

    go test -count=1 -run '^TestMem_T2_ResetAccelerator$|^TestP04_ApplyDropShadowGPU$' ./render
    → thread panicked at wgpu-core-29.0.4/src/storage.rs:137:9:
      assertion `left == right` failed: Texture[Id(25,1)] is no longer alive
      panic in a function that cannot unwind → aborting
    栈：render.(*Context).Image → materializeFilterGPU → ReadbackViewStraightRGBA
        → gpu/webgpu.(*CommandEncoder).CopyTextureToBuffer

归因（三条实测）：① **HEAD 对照**——把本次四处修复整个回退（`gpu_shared.go` 回 HEAD、新钉移开），
同一对测试同样 `no longer alive=1` 且 `invalid bind group layout=0` ⇒ **不是 H19 修复引入**，
是原先被 H19 提前炸掉而 masked 住的存量洞；② 单独摘掉 abandon-on-loss（保留守卫）后整包仍在同
一点中止 ⇒ 与本修的第三处无关；③ 缩到最小窗口：`[lo=1374..1376]` 不红、`[lo=1352..1376]` 红，
毒手在 `TestMem_*ResetAccelerator` 这一族（重置 accelerator＝换设备）。

归属：真源 `ENGINE_GPU_RESOURCE_LIFECYCLE.md` §1.1 **D1**（命令持有裸句柄，入队到消费全程无存活
校验）与 **D3**（paint 入队 → flush 重建 → 用已释放视图）的又一个实例，落在滤镜 GPU 的读回腿：
加速设备被重置后，滤镜图/发布槽里的纹理视图仍指向旧设备，读回时 `CopyTextureToBuffer` 把它递给
新设备 ⇒ 又一次「悬垂句柄 = 进程级崩溃」（D6 的表现形式）。

门禁影响（写清别只挂着）：**整包 `-race ./render` 仍不可用**，卡的是 H20 不是 H19；批 6 剩余窗
按逐文件（进程隔离）+ 真窗双后端拿证据，H20 修完才能重新拿到整包竞态证据。
