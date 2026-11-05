H20 影响面评估（2026-11-06，批 6 H19 收账后按拍板开线；停报告节点待用户拍板）
====================================================================================

一、现象与最小复现（两条测试名，同进程）
----------------------------------------

    go test -count=1 -run '^TestMem_T2_ResetAccelerator$|^TestP04_ApplyDropShadowGPU$' ./render
    → thread panicked at wgpu-core-29.0.4/src/storage.rs:137:9:
      assertion `left == right` failed: Texture[Id(1,1)] is no longer alive
      panic in a function that cannot unwind → 整进程 abort，rc=1
    栈：render_test.TestP04_ApplyDropShadowGPU → Context.materializeFilterGPU
        → materializeFilterGPUTo → GPURenderContext.ReadbackViewStraightRGBA
        → gpu/webgpu.(*CommandEncoder).CopyTextureToBuffer → wgpuCommandEncoderCopyTextureToBuffer

单测隔离（P04 单跑 / T2 单跑）都绿 ⇒ 顺序依赖，逐文件口径看不见；整包 `./render`
当前就停在这一对上（H19 修后的剩余中止点，第 ~1376 枚）。**整包 `-race ./render`
不可用的元凶就是它**。

二、探针打到的因果链（临时 H20DIAG，取完即删，工作区已核对还原）
------------------------------------------------------------------
对五个文件下探针（发布/挂载/物化/读回/装设备/abandon/accel-close，打印 shared 与
设备指针），T2+P04 最小对实测输出（tmp/h20-probe2.log，tmp 不入库）：

    T2 reset1  accel-close shared=0xc0001d2a88（abandon dev=nil，新 accel 还没建设备）
    T2 mem#1   initGPU-standalone shared=0xc0001d3508 install dev=0xc0000d80c0   ← shared#2
    T2 reset2  accel-close shared=0xc0001d3508（abandon dev=A1 release=true）      ← **shared#2 被关**
    T2 mem#2   initGPU-standalone shared=0xc00013a008 install dev=0xc0000d8480   ← shared#3
    T2 reset3  accel-close shared=0xc00013a008（abandon dev=A2 release=true）
    T2 mem#3   initGPU-standalone shared=0xc00013aa88 install dev=0xc000186000   ← shared#4（当前）
    P04        initGPU-standalone shared=0xc0001d3508 install dev=0xc000186180 gen=1
                                                                          ↑↑↑ **已关闭的 shared#2 复活了：装了台新设备**
    P04        attach view=0xc0001804b0（发布，无 publish 探针行——走的是钩子路径）
    P04        materialize view=0xc0001804b0
    P04        readback shared=0xc00013aa88 dev=0xc000186000（当前 accelerator 的 shared#4/A3）
    → assertion: Texture[Id(1,1)] is no longer alive

一句话：**发布与读回用了两台设备**——发布视图来自「被钩子闭包复活的已关闭 shared#2
的新设备」，读回走「当前 context 的 RC 绑定的 shared#4 的 A3」。两台设备各自有独立的
wgpu instance/hub，A3 的 hub 里根本没有那个句柄 ⇒ 非 unwind panic。

三、根因（定位到行）
--------------------
`render/internal/gpu/gpu_shared.go:1333-1371 registerFilterGraphIfNeeded`：

    func (s *GPUShared) registerFilterGraphIfNeeded() {
        if gpuFilterGraphRegistered {   // 包级标志，shared 关闭时**从不复位**
            return
        }
        render.RegisterGPUFilterGraph(func(...) { s.ensureGPU(); ... })        // ← 闭包捕获这份 s
        render.RegisterGPUFilterGraphTexture(func(...) { s.ensureGPU(); ... }) // ← 同
        render.RegisterGPUFilterGraphFromView(func(...) { s.ensureGPU(); ... })// ← 同
        gpuFilterGraphRegistered = true
    }

三个滤镜钩子闭包**捕获了第一次注册时的那份 GPUShared**，而标志永不复位：
1. T2 第一次 `memDrawPresentSimple` 时 shared#2 ensureGPU 注册钩子（闭包捕获 shared#2）；
2. T2 `ResetAccelerator` → `CloseAccelerator` → `SDFAccelerator.Close` → shared#2 关闭
   （abandon，设备 A1 释放）——**钩子还攥着 shared#2**；
3. P04 `ApplyDropShadow` → `filter_ops_gpu.go:201/230 gpuFilterGraphApplyFromView(...)`
   → 钩子闭包 → `shared#2.ensureGPU()`：s.device 为 nil（已关）→ `initGPU` 在**已关闭
   的 shared 上装了台全新设备**并发布滤镜视图（`filter_src` 的 srcView 本身还来自当前
   context 的 shared#4——发布链自身就跨了两次设备）；
4. `sampleRGBA` → `materializeFilterGPUTo` → `c.GPURenderContext()`＝当前 context 的 RC
   （绑 shared#4/A3）→ `ReadbackViewStraightRGBA` 把 shared#2 新设备的视图喂给 A3 的
   `CopyTextureToBuffer` → abort。

次要条件（同根）：`ensureGPU/initGPU` 对**已 Close 的 shared** 照常复活装设备
（`gpu_shared.go:782 ensureGPU` 只看 device nil/IsLost，不知道 shared 已 Close）。
没有这条，钩子就算攥着死 shared 也拿不到新设备。

四、为什么此前没被发现（负向证据自洽）
--------------------------------------
- **单测隔离绿**：钩子和读回都在同一份 shared 上（进程内只有一个 shared），发布/读回
  同设备——洞只在「shared 被换过」时暴露。
- **HEAD（H19 修复前）同样红**：这是存量洞。原先整包在第 624 枚就被 H19 炸掉，
  T2/P04 在 ~1376 枚——H19 修完才轮到它暴露（masked → uncovered）。
- **毒手是 TestMem_*ResetAccelerator 一族**：全仓只有这一族会在测试中 CloseAccelerator
  （重置 accelerator＝换 shared），恰好触发「钩子指死 shared」。

五、标准依据（不是自造规矩）
----------------------------
- 本仓真源 `docs/ENGINE_GPU_RESOURCE_LIFECYCLE.md` §1.1 **D1/D3/D6**（悬垂句柄/跨设备
  句柄=进程级崩溃）+ §3.3 **R5**（丢失/换绑 abandon 全量清场）——本例是 R5 的一个**漏网
  面**：abandon 清的是 GPUShared 自己名下的资源，而 render 包的全局钩子闭包不在其中。
- **同文件既有先例**：`GPUShared.Close()` 本来就注销三件套 render 级登记
  （`UnregisterPurgeEvictable` / `UnregisterGPUPoolStatsCollector` /
  `UnregisterShaderEffectEvictor`，`gpu_shared.go:698-701`）——**唯独滤镜图钩子没注销**。
  修复就是把滤镜钩子补进这张清单，不是发明新规矩。
- 外部对照：Skia `GrDirectContext::abandonContext` 之后，任何持有旧 context 引用的
  全局缓存都属于未定义行为（Skia 的做法是引用方统一经 GrContextFactory 重新取）；
  Flutter Impeller 没有「全局滤镜钩子」这种形态，pipeline 全随 context 生命周期。
  全局单例闭包捕获会死的具体对象=本例形态，标准做法是钩子随对象生死或经当前
  活对象转发。

六、影响面（改什么、谁受影响）
------------------------------
改动面（两文件，预计零新增公开 API 之外加一个注销口）：
1. `render/filter_ops_gpu.go`：新增 `UnregisterGPUFilterGraphs()`（清三个钩子）。
   **公开 API +1**，需同步目录 §3 + apidoc（机器校验必跑）。
2. `render/internal/gpu/gpu_shared.go`：
   - `registerFilterGraphIfNeeded` 记 `s.filterGraphHooksRegistered`；
   - `GPUShared.Close()`：若本 shared 注册过钩子 → `render.UnregisterGPUFilterGraphs()`
     + 复位 `gpuFilterGraphRegistered=false`（与既有三件套注销同位同风格）；
   - 新增 `closed` 守卫：`Close()` 置位；`ensureGPU`（及 `initGPU` 入口）遇 closed
     直接返回错误——**已关闭的 shared 禁止复活装设备**（纵深：钩子即使没清干净、
     stale RC 即使还在调，也拿不到设备，上层滤镜 op 收 err 优雅退 CPU）。

行为变化（如实声明）：
- `CloseAccelerator/ResetAccelerator` 之后，滤镜图的 GPU 钩子被清空；下一个活 shared
  在它下一次 `ensureGPU` 时重新注册（现所有 ensureGPU 路径都调
  `registerFilterGraphIfNeeded`，天然重挂）。窗口期内滤镜 op 走 CPU fallback（有
  `GPUFilterGraphRegistered()==false` 的既有判定挡着，不会崩）——正常单 accelerator
  应用无感（钩子本来就由当前 shared 注册）。
- 测试侧 `DisableGPUFilterGraphForTest` 的 save/restore 语义不变。
- 对 P04 的最终效果：T2 之后 P04 的发布与读回都走当前 shared ⇒ 同设备 ⇒ 不崩，
  滤镜照常走 GPU。

消费者影响盘点：三个钩子的消费点是 `ApplyBlur/ApplyDropShadow/ApplyImageFilterGraph/
ImageFilterNode/Bloom` 全族（`filter_ops_gpu.go:201/230` 等）——全部经
`GPUFilterGraphRegistered()` 判定后才调钩子，清钩子=自动退 CPU，无悬空调用。

七、修复与钉（拟，待拍板）
--------------------------
| 项 | 内容 |
|---|---|
| 根修 | Close 时注销滤镜钩子 + 复位标志；活 shared 在 ensureGPU 路径重注册（现状已有调用点，无需新接线） |
| 纵深① | closed 守卫：已 Close 的 GPUShared 禁止 ensureGPU/initGPU 复活（返回错误，上层退 CPU） |
| 钉A（根修，进程级） | `render/p37_6_h20_test.go`：**子进程**跑 T2+P04 最小对（失败模式是 SIGABRT，同进程一 occurrence 带走整个测试二进制，只能外部断），断两枚真 PASS、无 `no longer alive`/`invalid bind group layout`、有 SKIP 判空钉 |
| 钉B（钩子随 shared 生死） | internal/gpu 单测：`registerFilterGraphIfNeeded()` 后 `GPUFilterGraphRegistered()==true` → `Close()` 后钩子三枚全 nil + 标志复位 → 新 shared ensureGPU 后重新注册（纯逻辑可跑，不需要真设备） |
| 钉C（closed 不复活） | `Close()` 后 `ensureGPU()` 必须返回错误且 `s.device` 仍 nil |
| 负向 | 摘掉「Close 注销钩子」⇒ 钉A 实测红（复现 abort）；摘掉 closed 守卫 ⇒ 钉C 红、钉A 预期仍绿（两条是纵深，如实记「摘掉不红」的那条） |

门禁（修复后）：build/vet/gofmt 触及三文件、apidoc rc=0（新增公开口入目录 §3）、
`-race ./render/internal/gpu`、**整包 `go test ./render` 应当首次跑到底**（H20 是最后
一个已知中止点——这是本修的标志性验收）+ `CGO_ENABLED=1 go test -race ./render` 整包
竞态证据恢复、触及包逐文件 sweep 对基线只减不增。

八、待拍板
----------
1. 修复方案按「根修 + closed 守卫」两条一起做（推荐），还是只做根修（钩子随 shared
   生死）？只做根修的话钉C 没着落，且 stale RC/闭包仍有复活窗口（纵深少一层）。
2. 新公开口命名 `UnregisterGPUFilterGraphs`（与既有 RegisterGPUFilterGraph{,Texture,
   FromView} 同族命名）是否可以。
