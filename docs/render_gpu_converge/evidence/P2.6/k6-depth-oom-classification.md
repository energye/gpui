# K6 尾注立项：深度/解析纹理创建错误分类 fail-closed 化

> 来源：`evidence/render_full_run_reds.md` 2026-11-09 档 2 尾注（单进程全量绿需动引擎）
> + 2026-11-11 用户拍板「分类修正按标准做」。日期 2026-11-11。

## 病灶（源码钉死）

`render/internal/gpu/surface_cache.go::createSurfaceTrio` 把纹理创建的**任何错误**
无条件包成 OOM 形态：

- 全尺寸深度失败 → 一律 `*surfaceDepthOOMError`（原 :368）→ owner 走 1x1 降级并
  **锁存降级窗**（`textureSet.latchDegraded`）；
- 1x1 深度失败 → 一律 `*surfaceTerminalOOMError`（原 :362）→ `failTerminal`
  以「GPU out of memory」文案锁存终态；
- resolve 失败 → 一律同上（原 :394）。

而 `createTextureRetryOOM`（`gpu_textures.go:380-382`）内部分类**本来是对的**：
非 OOM 错误（如 `wgpu: resource already released`，validation / use-after-release）
直接裸返回、不走重试梯。错在 `createSurfaceTrio` 拿到返回值后**重新误分类**——
K6 的 88 测试连锁画空即此机制：第一次 `resource already released` 被当 depth-OOM
降级 1x1 → stencil Always/Keep 退化 → Auto/SDF 路径 ink 全空 → 后续全部连锁假绿。

## 标准对齐

- **wgpu 错误语义**：OOM（可重试/可降级）与 validation error（对该操作致命）
  是两类；后者绝不静默降级。
- **Flutter/Impeller**：资源分配失败走清缓存重试；对已释放句柄的 misuse
  绝不自愈（真 device lost 的上下文重建是另一条标准路，不与 misuse 混同）。
- **本仓 M6 先例**：未知纹理格式 fail-closed（`filter_ops_gpu.go:866`
  「格式未知一律 fail-closed」）。同一家法：**分类宁可显式红，不可静默降**。

## 修法

`surface_cache.go::createSurfaceTrio` 三处包裹点加 `render.IsGPUOutOfMemory(err)`
门控：真 OOM 才进 `surfaceDepthOOMError` / `surfaceTerminalOOMError` 恢复路；
非 OOM 显式裸错误（`create depth/stencil: %w` / `create resolve: %w`），
不降级、不锁存、下一帧重试（与 MSAA/view 错误的既有「plain error 重试」契约
一致）。契约注释同步改写。`ensure()` 的调用方分支（`gpu_textures.go:164-184`）
零改动——非 OOM 错误天然落进「else return err」显式路径。

**预期与边界（对齐拍板时的说明）**：单进程全量 ./render 不会因此变绿——
同进程几百 GPU 测试的资源累积是测试层容量问题，标准解法是隔离（分片门禁/
逐文件，已落地）。本修法的效果是：连锁画空假绿变成第一处 misuse 显式红，
**门禁恢复可信**；OOM 恢复语义（P1.6 显存 OOM 降级/终态锁存）逐位保留。

## 钉测（先红后绿，`render/internal/gpu/surface_depth_oom_classification_test.go`，
noop 设备 + 接口包装假件，无需真 GPU）

| 用例 | 修前 | 修后 |
|---|---|---|
| `TestSurfaceDepth_NonOOMFailsClosed`（released 不进 depthOOM） | 红（误分类） | 绿 |
| `TestSurfaceDepth_OOMStillDegrades`（真 OOM 仍降级） | 绿（护栏） | 绿 |
| `TestSurfaceDepth_NonOOMTerminal1x1FailsClosed`（1x1 非 OOM 不锁终态） | 红 | 绿 |
| `TestSurfaceDepth_OOM1x1StillTerminal`（1x1 真 OOM 仍锁终态） | 绿（护栏） | 绿 |
| `TestSurfaceResolve_NonOOMFailsClosed`（resolve 非 OOM 不锁终态） | 红 | 绿 |
| `TestTextureSet_EnsureDegradLatchOnlyOnOOM`（owner 级：非 OOM 不进降级窗/真 OOM 进） | 红（非 OOM 腿） | 绿 |

负向对照：摘 depth 腿 OOM 门控（`if false && …` 变异）→ 拒绝用例转红 → Edit 还原绿。

## 门禁（2026-11-11）

- `render/internal/gpu` 整包测试绿（含既有 P2.4d surface cache 全部钉）；vet rc=0；
  nogpu 构建口（钉测带 `//go:build !nogpu`）。
- K6 分片门禁 1–4 复跑绿（见 render_full_run_reds.md 11-11 复验段）。
- **过程教训（第二次同类失误，如实登记）**：负向对照还原时再次误用
  `git checkout` 把未提交实现连同变异一起抹掉——已逐字重放三处修改并复验；
  之后变异一律 Edit 进 Edit 出，不用 checkout 还原。两次 stash/checkout
  失误均已完全恢复、零内容丢失。
