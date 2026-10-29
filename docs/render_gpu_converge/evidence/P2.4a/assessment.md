# P2.4a 影响面评估（开工前，2026-10-30）

> 性质：**只评估、不动工作区**。结论来自三处设备创建/选卡实现的逐行读码、全仓调用面 grep（含测试）、P1.3/P0.4 基线记录对账，以及一次现行复现探针（`go test ./render/internal/gpu/ -run TestBuffer`，2026-10-30 本会话实测）。
> 方案依据：PLAN §1.3（设备创建/选卡：标准 = `adapter_policy.go` 定策略 + 呈现链唯一建链；非标准 = `internal/gpu` 镜像 + 第三份独立建链）、PLAN §2.2-B1（三份 → 一份）、台账 §2.2 A3/C7。
> 结论一句话：**收敛对象是「一个标准呈现链 + 两份独立建链（探针/离屏、Vello compute）+ device.go 镜像与死代码簇」；标准链不动，两份独立建链改走 render 层唯一入口，镜像与死代码删除（零调用已证）；A3（46 个 RequestDevice SKIP）在当前会话不复现、设备创建正常，按解剖计划建档；C7（GL 第 4 窗 eglMakeCurrent 0x3002）本步只解剖定位，修复归属待拍板。**

## 0 结论摘要

| 项 | 结论 |
|---|---|
| 三份现状 | ①标准：`render/present_construct.go` 呈现链（policy 阶梯 + 重试 + 就绪门 + 共享设备发布）——保留；②`render/internal/gpu/gpu_shared.go:748 initGPU`：先借共享（好），借不到自开 Vulkan 实例 + policy 选卡 + 镜像重试；③`render/internal/gpu/vello_accelerator.go:1329 initGPU`：自开 Vulkan 实例 + **`RequestAdapter(nil)` 绕过 policy** + 镜像重试 |
| 创建入口 | `webgpu.CreateInstance` 白名单 3 处：`render/backend.go:100`（SelectBackend 漏斗，标准）、`vello_accelerator.go:1330`、`gpu_shared.go:769`（后两处在 render/internal/gpu 里各写一句，绕过 SelectBackend，硬编码 `BackendsVulkan`） |
| 镜像与死码 | `render/internal/gpu/device.go`：`renderDeviceDescriptor` ≡ `render.DeviceDescriptor`（逐行同体）、`requestDeviceWithRetry` ≡ `requestPresentDeviceWithRetry`（同逻辑）、双常量 `minRenderStorageBuffersPerShaderStage` ≡ `minStorageBuffersPerShaderStage`（同值 9）；死代码簇 `GPUInfo`/`getGPUInfo`/`logGPUInfo`/`createDevice`/`getDeviceQueue`/`releaseAdapter` **源码+测试零引用** |
| A3 现行证据 | 2026-10-30 本会话：`createNativeTestDevice` 建设备**成功**（TestBuffer 8 PASS），跳过原因是 OOM 类 "Not enough memory left"，不是 P1.3 记的 `RequestDevice: Validation Error`；P1.3 当时 `conformance_test.go` 的双后端编排用例是 SKIP（原生装箱没跑过），所以"同 descriptor 一过一挂"从未同环境对照过——A3 是未解剖状态，本步按 §4 计划解剖 |
| C7 现场 | `baseline/vram/ui_base_vram_cap/ladder/go/stderr.log:3-5`：GL 第 4 窗 `eglMakeCurrent failed: error 0x3002`（EGL_BAD_ACCESS，典型为 context 被其它线程持有）；本步只解剖定位到 file:line，修复归属待拍板（P2.4e 或本步） |
| 收敛后 | `webgpu.CreateInstance` 白名单 3→1（全在 `render/backend.go` 一个核心句）；`render` 导出唯一独立设备入口（借用方 2 处改调）；`render/internal/gpu` 删 8 个符号（2 镜像 + 6 死码，含 1 常量）；`render` 增 2 个导出符号（入口 + 重试助手），API 快照差异全部登记 |
| 行为变化面 | 本机（单 Vulkan 设备）选卡结果不变、像素门禁不受影响；**混合显卡机器上 Vello compute 独立路径的选卡语义会变**（从裸 `RequestAdapter(nil)` 变为 policy 驱动 + OOM 降级阶梯）——这是收敛的本意，但须用户拍板（§8-D1） |
| 待用户拍板 | 5 项，见 §8 |

## 0.1 能力替代核对（每个老实现，新路径拿什么接）

| # | 老载体 | 能力 | 新载体 | 结论 |
|---|---|---|---|---|
| 1 | `gpu_shared.initGPU` 自开分支 `webgpu.CreateInstance(BackendsVulkan)` | 探针/离屏独立实例 | render 唯一入口（严格 Vulkan 掩码，保持现语义） | 已替代（同一句核心，零行为） |
| 2 | `gpu_shared` 的 `RequestAdapterWithPolicy(instance, nil, ResolveAdapterPolicy())` | policy 选卡（无 surface） | 唯一入口内同参数调用（原样搬） | 已替代（逐行等价） |
| 3 | `gpu_shared` 的 `requestDeviceWithRetry + renderDeviceDescriptor("gg-shared")` | OOM 重试 + 平 limits descriptor | 唯一入口内 `render.DeviceDescriptor` + 导出重试（同体同值） | 已替代（label 保留） |
| 4 | `vello_accelerator.initGPU` 的 `webgpu.CreateInstance(BackendsVulkan)` + `RequestAdapter(nil)` | compute 独立实例+裸选卡 | 唯一入口（policy 驱动）——**语义变化，D1 拍板** | 替代后行为差见 §2.3 |
| 5 | `vello` 的 `requestDeviceWithRetry + renderDeviceDescriptor("gg-vello")` | 同 #3 | 同 #3 | 已替代 |
| 6 | `device.go` 死代码簇（GPUInfo 等 6 符号） | 无（零引用） | 无需替代 | 删除（白名单登记） |
| 7 | `render/gpu.SetDeviceProvider/AbandonDevice` 转发包装 | 设备注入便捷入口 | 生产零调（唯一消费者是 conformance 测试）；PLAN §2.3-B4「接进 embedder 或标实验」 | 本步只标实验（文档），接线归 P3+（D5 拍板） |
| 8 | 呈现链 `present_construct.go` 全部 | 窗口建链（policy 阶梯/重试/就绪门/共享发布） | **不动**（标准本体） | 保留 |

不收敛项（明确排除，防 scope 蔓延）：
- `DeviceDescriptorForAdapter`（水位联动/LowVRAM）与 plain `DeviceDescriptor` 的**descriptor 口径统一**：属预算语义，归 **P2.4b**；本步 standalone 保持 plain descriptor（零行为）。
- vello standalone 是否先借共享设备：本步**不借**（保持现状独立设备，零行为）；1GB 卡单设备不变式由 gpu_shared 借用路径继续承担。
- `AdapterPolicy` 的 4 个 deprecated 别名常量（PolicyNone/HighPerformance/LowPower/Auto）：别名清理归 P2.5/API 收口，本步不动。

## 1 方法与可复现命令

```bash
# 1) 三处实现定位（只读）
grep -rn "CreateInstance(\|RequestAdapter\|RequestDevice(" render/ gpu/context/ --include="*.go" | grep -v _test
#   → render/backend.go:100 / present_construct.go:37,440,452
#   → render/internal/gpu/vello_accelerator.go:1330,1338 / gpu_shared.go:769,785 / device.go:110

# 2) 镜像等价（逐行比对）
diff <(sed -n '27,36p' render/internal/gpu/device.go) <(sed -n '189,198p' render/adapter_policy.go)
#   → renderDeviceDescriptor 与 render.DeviceDescriptor 逐行同体（常量名不同，同值 9）

# 3) 死代码簇零引用（源码 + 测试）
grep -rn "createDevice\|getGPUInfo\|logGPUInfo\|getDeviceQueue\|releaseAdapter\|GPUInfo" --include="*.go" render/ examples/ ui/ | grep -v third_party
#   → 仅 device.go 自身定义处，0 消费（含 _test.go 复查 0 命中）

# 4) A3 现行探针（2026-10-30 本会话实测）
WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so go test ./render/internal/gpu/ -run 'TestBuffer' -count=1 -v
#   → 8 PASS；SKIP 项原因 "Not enough memory left"（wgpu OOM 类，非 Validation Error）

# 5) P1.3 记录对账
grep -n "RequestDevice" docs/render_gpu_converge/baseline/tests/summary.md   # :135 46 例 Validation Error
grep -n "conformance_test" docs/render_gpu_converge/baseline/tests/summary.tsv  # :1306 PASS 但 1 skipped（编排用例没跑）
```

## 2 三份实现逐项对比

### 2.1 标准链（保留，不动）

`render/present_construct.go`（经 `NewPresentTargetWithBackend` → `buildPresentTarget`）：

1. `ResolveBackendFor(backend)`——GPUI_BACKEND env 胜出（native|go 二选一）；
2. `SelectBackend(want)`（`render/backend.go:86`）——**全仓唯一的 webgpu.CreateInstance 句**（native）或 `hal.GetBackend(BackendGL)` 建实例（go）；
3. `presentLevels(ResolveAdapterPolicy())` 阶梯——policy 驱动 + OOM 逐级降级（discrete-first → integrated-first → software fallback）；
4. `RequestAdapterWithPolicy(inst, surf, policy)`（或 software 直取）；
5. `requestPresentDeviceWithRetry`——OOM 类错误重试 3 次、间隔 1s；
6. `DeviceDescriptorForAdapter("ui-l1-present", adapter)`——水位联动 limits；
7. `waitDeviceReady`——1x1 探针纹理就绪门；
8. 共享设备发布（`SetAcceleratorDeviceProvider(&halDeviceProvider{...})`）+ 单设备不变式（R0-2 永不覆盖已有 share）。

### 2.2 第二份：gpu_shared.initGPU（探针/离屏独立设备）

`render/internal/gpu/gpu_shared.go:748`，被 `ensureGPU`（`:640`）在无共享设备时调用：

- 先 `render.AcquireSharedPresentDevice()` 借窗口发布的共享设备（好，保留）；
- 借不到自开：`webgpu.CreateInstance(BackendsVulkan)`（`:769`，绕过 SelectBackend）→ `render.RequestAdapterWithPolicy(instance, nil, render.ResolveAdapterPolicy())`（`:785`，走 policy 但无 surface）→ CPU 适配器检测置 softwareMode → `requestDeviceWithRetry(adapter, renderDeviceDescriptor("gg-shared"))`（`:801`，镜像重试 + 镜像 descriptor）。
- 无就绪门（与标准链差异之一）。

### 2.3 第三份：vello_accelerator.initGPU（compute 独立设备）

`render/internal/gpu/vello_accelerator.go:1329`，两条调用路径：`flushLocked` 惰性（`:371`，设备为空时）与 `InitStandalone`（消费方：`scene_auto.go:85 RenderSceneAuto`、`render/examples/compute_pipeline/main.go:213`）：

- **不借共享**（与第二份差异）；
- `webgpu.CreateInstance(BackendsVulkan)`（`:1330`，绕过 SelectBackend）→ `instance.RequestAdapter(nil)`（`:1338`，**完全绕过 policy**：无 power preference、无混合显卡 iGPU 偏好、无降级阶梯）→ `requestDeviceWithRetry(adapter, renderDeviceDescriptor("gg-vello"))`（`:1343`）。

**语义变化面（D1 核心）**：改为 policy 驱动后，混合显卡（dGPU+iGPU 同时在）机器上，PolicyDefault 的「裸 None 先到 dGPU 则换 iGPU」逻辑会改变 compute 路径的选卡；OOM 时多两级降级尝试。本机只有一颗 Vulkan 设备（940MX），选卡结果不变；像素/金图门禁不受影响。该变化本机不可直接验证，跨平台标记按「语义收敛（PLAN §1.3 三份→一份），混合显卡行为变化登记」处理。

### 2.4 device.go 镜像与死代码簇

`render/internal/gpu/device.go`（209 行）拆解：

| 符号 | 性质 | 处置 |
|---|---|---|
| `renderDeviceDescriptor` + `minRenderStorageBuffersPerShaderStage` | 镜像（≡ `render.DeviceDescriptor` + `minStorageBuffersPerShaderStage`，逐行同体同值） | 删，调用方（`:1343`、`:801`、`native_device_test.go:31`）改 `render.DeviceDescriptor` |
| `requestDeviceWithRetry` | 镜像（≡ `requestPresentDeviceWithRetry` 同逻辑） | 删，重试实现上收到 render 并导出，两份独立建链经唯一入口使用 |
| `GPUInfo` / `getGPUInfo` / `logGPUInfo` | 死代码（含测试 0 引用） | 删 |
| `createDevice` / `getDeviceQueue` / `releaseAdapter` | 死代码（含测试 0 引用） | 删 |
| `Backend`（最小设备持有者） | **P2.3 已拍板保留**（`gpu_texture.go` 与 3 个测试文件在用） | 不动 |

### 2.5 注入缝（不动，列清防混淆）

设备**注入**与设备**创建**是两条路：`gpucontext.DeviceProvider` 接口 + `webgpu.SimpleDeviceProvider` / `p1ProbeProvider` / `halDeviceProvider` 三个装箱类型 + `render.SetAcceleratorDeviceProvider` / `render/gpu.SetDeviceProvider` 转发。本步只收敛创建；注入缝里 PLAN §2.3-B4 点名的 `render/gpu.SetDeviceProvider/AbandonDevice`（生产零调，唯一消费者 conformance 测试）按 D5 处置。

## 3 收敛方案（目标形态，待 D1 拍板后执行）

```
render/backend.go        # 核心化：一个 webgpu.CreateInstance 句
  SelectBackend(want)            # 现状保留：env 胜出，BackendsPrimary（present 链用）
  selectBackendMask(mask)        # 内部核心：唯一 CreateInstance 句
  SelectBackendVulkan()          # 导出（或包内）：严格 Vulkan 掩码、env 不劫持（standalone 用，保持现 BackendsVulkan 语义）

render/adapter_policy.go（或新 render/device_open.go）
  RequestDeviceWithRetry(...)    # 导出：原 requestPresentDeviceWithRetry 上收，唯一重试实现
  OpenStandaloneDevice(label)    # 导出：唯一独立设备入口
    inst = SelectBackendVulkan()
    adpt, fallback = RequestAdapterWithPolicy(inst, nil, ResolveAdapterPolicy())
    dev  = RequestDeviceWithRetry(adpt, DeviceDescriptor(label), label)
    # 不带 waitDeviceReady（present 专属门，standalone 现状无门，零行为）

render/internal/gpu/gpu_shared.go
  initGPU 自开分支 → render.OpenStandaloneDevice("gg-shared")   # 借共享分支原样保留

render/internal/gpu/vello_accelerator.go
  initGPU → render.OpenStandaloneDevice("gg-vello")             # policy 生效（D1 语义变化）

render/internal/gpu/device.go
  删 8 符号（2 镜像 + 6 死码）；Backend 保留；native_device_test.go:31 改 render.DeviceDescriptor

render/present_construct.go
  requestPresentDeviceWithRetry 改调上收后的 RequestDeviceWithRetry（或原地保留薄转发，实现期定）
```

关键设计点：
- **env 不劫持 standalone**：`GPUI_BACKEND=go` 时 standalone 仍走 native（GL 无窗 surfaceless 建不了设备，conformance 是靠「不呈现的 X11 窗」做到的）；这是现状语义，严格保留。
- **BackendsVulkan 钉死**：standalone 保持现 `BackendsVulkan` 掩码（Windows 上 BackendsPrimary 会引入 DX12 可能性 = 行为变化，不做）。
- **无 import cycle**：`render/internal/gpu` 已 import render（`AcquireSharedPresentDevice` 等），方向不变。
- **错误链**：`IsGPUOutOfMemory` 走 `%w` 包装链，收敛后错误仍可被判 OOM 类（实现期用单测锁住）。

## 4 A3 解剖计划（46 个 RequestDevice SKIP，先解剖后定修复方向）

已掌握事实：
- P1.3（2026-10-29 环境）：46 例 `wgpu: failed to request device: wgpu: RequestDevice: Validation Error`（`baseline/tests/summary.md:135`），同轮 `conformance_test.go` 编排用例 SKIP（`:1306`，1 skipped）——原生装箱在 P1.3 从未跑过，无同环境对照。
- P0.4/P1.1b（独立跑）：`render.DeviceDescriptor("p0.4-conformance")` 在 native（Vulkan）与 GL 两档 RequestDevice 都 PASS。
- 2026-10-30 本会话探针：`createNativeTestDevice`（同序列：BackendsPrimary 建实例 → HighPerformance 选卡 → plain descriptor 建设备）**成功**；同文件其余 SKIP 是 wgpu OOM 类（"Not enough memory left"），与 Validation Error 无关。

解剖步骤（P2.4a 实施首日执行，产出 `evidence/P2.4a/a3-dissection.md`）：
1. **复现矩阵**：按 P1.3 同命令（`WGPU_NATIVE_PATH=… go test -run <该文件用例>`）逐个跑 46 例所在代表文件（buffer/texture/s6_7/render_session/stencil 等 ≥5 个文件），记录 PASS/SKIP 与原因；同机当天两种结果本身就是数据。
2. **limits 对账探针**：一次性小程序打印 `adapter.GetLimits()` 与 `types.DefaultLimits()` 逐字段对比，找「请求值 > 适配器上限」的字段（wgpu RequestDevice Validation Error 的典型根因）；探针源码固化进 evidence（一次性，跑完删）。
3. **错误全文捕获**：若 1 复现，用 hal 错误面/uncaptured error handler 拿 wgpu 完整验证消息（当前 t.Skipf 只透传一层）。
4. **结论归档**：三种可能——(a) 环境依赖（驱动/显存状态），登记「不可复现 + 证据」关闭 A3；(b) DefaultLimits 超限，修复方向（收紧 DefaultLimits 或请求前对账适配器 limits）经用户拍板后另立行动；(c) 代码路径差异，定位 file:line。**解剖前不定修复方向（台账 A3 原文口径）。**

## 5 C7 现场（GL 第 4 窗 eglMakeCurrent 0x3002）与归属

- 证据：`baseline/vram/ui_base_vram_cap/ladder/go/stderr.log:3-5`——第 4 扇窗打开时 `gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"`，随后 `FAIL: app 4 open: render: ConfigureFromCapabilities: ... MakeCurrent failed`。
- 初判（仅诊断假设，不作为动手依据）：0x3002 = EGL_BAD_ACCESS；`gpu/gwgpu/gles/adapter_context_linux.go` 的 AdapterContext 是带互斥的 MakeCurrent 切换封装，多窗阶梯在第 4 窗触发，指向「同一 EGLContext 被其它线程持有 / 多窗上下文数量与绑定策略」类问题，与设备/上下文创建路径相关。
- 归属建议：**P2.4a 本步只解剖**（定位到 file:line + EGL 错误码证据链，产出 `evidence/P2.4a/c7-dissection.md`）；修复动作随 D4 拍板——若根因在建设备/上下文创建（本步对象内）则本步修（P2 零行为口径下属于「修不可用路径」，真窗门禁不受影响），若在多窗呈现/绑定策略则归 P2.4e（与 StubHost 假句柄、窗口关闭协议同批）。

## 6 门禁设计

| 门禁 | 口径 | 对象 |
|---|---|---|
| 五目标构建 | linux×2 / windows / darwin×2，默认 + nogpu 主体 + nogpu 库范围 | 全绿（raw `./...` 仅 `ui_wr_t_stress` 既有红） |
| vet | 默认 61 / nogpu 58 逐条不差 | 收敛不触 `ui/platform`、`gpu_render_context.go` unreachable 两处 |
| 逐测试文件 | render/ + render/internal/gpu/ + render/gpu/ 全部 `*_test.go` 逐文件（禁全量），36 个用 createNativeTestDevice 的文件重点跑 | PASS 集合与 P2.3 后一致；SKIP 带原因 |
| API 快照 diff | `render` +2 导出符号；`render/internal/gpu` −8 符号；其余 350 包逐字节不变 | 差异逐条对 §7.7 白名单（本评估 §0 收敛后行） |
| 残留 grep | `grep -rn "webgpu.CreateInstance\|RequestAdapter(" render/internal/gpu/` 仅剩经唯一入口的调用；`renderDeviceDescriptor\|requestDeviceWithRetry` 全仓 0 命中 | rc=1 |
| 生命周期测试台（P2.4 附加） | `ui_base_multiwin_cap`（或单窗标准关闭）：开窗/关窗 → 显存账本归零 | 隔离显示环境，标准 WM_DELETE 关闭 |
| 真窗三证据 | `ui_l1_blank`（native + go 两档 `-auto-only`）：JSON 探针 + PNG + 金图 diff=0 | 金图容差 0（P0.1） |
| 负向对照 | 探针恢复第二份 retry / standalone 绕 policy → 编译红或 policy 单测红；撤销复绿零残留 | 输出落盘 |

## 7 风险与对策

| # | 风险 | 对策 |
|---|---|---|
| 1 | 混合显卡机器上 vello compute 选卡语义变化，本机不可验 | D1 明示拍板；跨平台格登记「语义收敛 + 混合显卡行为变化待验证」；metal/darwin 交叉编译保构建面 |
| 2 | `GPUI_BACKEND=go` 误劫持 standalone → GL surfaceless 失败 | 严格掩码入口 env 不劫持（现语义），实现期加单测锁住 |
| 3 | Windows 上 BackendsVulkan vs Primary 差异被无意引入 | 核心句显式传掩码，不重载语义；交叉编译 + 快照对账 |
| 4 | 错误链包装变化破坏 `IsGPUOutOfMemory` 判定（OOM 降级阶梯失效） | 单测锁错误链；present 构造行为由既有窗口测试回归 |
| 5 | A3/C7 解剖发现真缺陷，超出零行为阶段 | 按 AGENTS「新问题归位」：登记台账子项，修复方向经用户拍板；不动本步收敛范围 |
| 6 | import cycle（render ↔ internal/gpu） | 方向不变（internal → render 单向），实现期 `go build` 五目标即证 |

## 8 待用户拍板

| # | 决策点 | 选项 | 建议 |
|---|---|---|---|
| D1 | 收敛方向 | A：两份独立建链全走唯一入口，vello standalone 获得 policy + 降级阶梯（混合显卡机器选卡语义变，本机门禁不变）；B：只删镜像与死码，vello 裸选卡保留（创建入口仍不唯一） | **A**（PLAN §1.3「三份→一份」的本意） |
| D2 | standalone 后端掩码 | 保持 `BackendsVulkan` 钉死（零行为）vs 改 `BackendsPrimary`（macOS standalone 可能变可用=行为变化） | **保持 Vulkan 钉死** |
| D3 | descriptor 口径 | standalone 保持 plain `DeviceDescriptor`（零行为，水位联动归 P2.4b）vs 本步就统一 `DeviceDescriptorForAdapter` | **保持 plain，归 P2.4b** |
| D4 | A3/C7 处置 | 本步解剖后：A3 修复方向另拍板；C7 根因若在建设备路径本步修、否则归 P2.4e | **先解剖，按 §4/§5 计划走** |
| D5 | `render/gpu.SetDeviceProvider/AbandonDevice` | 本步标实验（文档注明实验性 + 生产零调）vs 留 P2.5 统一处置 | **本步标实验，两行文档** |
