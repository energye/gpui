# P2.4c 延迟释放统一（三队列 → `res/registry`）收口

- 完成日期：2026-10-30
- 依据：台账 §2.1 P2.4c 载体、`assessment.md`（三队列盘点 + 收敛设计 + D1–D5）
- 一句话结论：**生产里跑的三条延迟释放队列（A `pendingTex/Buf`、B `ImageCache.ephemeral+pending`、C `res.Cache.pending`）全部删除，所有延后释放统一走 `res.Registry` 槽位状态机（Register → Track → Release → Retire），唯一销毁出口 = `SubmitDone` 清 inflight 后的 `removeIfReady`；K1（conformance 每轮 +2 条 / +131072B）按根因跨文件修掉并把上界收紧到 0/0；全门禁绿，真窗 native 两项 PASS，ladder 撞既有 I1、go 真窗既有 C7 均沿用干净树 A/B 判定，修复归 P2.4e。**

## 1 三队列 → 单状态机映射

| 队列 | 原实现 | 现状 |
|---|---|---|
| A：纹理/缓冲裸切片 | `GPURenderSession.pendingTexRetire/pendingBufRetire` + `Drain()`（多同步点手动排空） | 删除；`RetireTexture/RetireBuffer` → `retireNative`（`render_session.go:729`）：`Register → sub.Track → Release → Retire`，只在 `SubmitDone` 后由 `removeIfReady` 释放 |
| B：图片缓存 | `ImageCache.ephemeral` + `pending`，**异步 submit 返回后立即 Destroy** | 删除两个切片与 `ReleaseEphemeral`；新增 `SetRetireHook`（`image_cache.go:136`）/`retireEntry`（`:145`），session 注入 `RetireTexture`（受 D2 批准：释放点后移到提交完成同步点）；无钩子时保持立即 Destroy（单测语义不变） |
| C：资源池 | `res.Cache.pending` + `OnSubmissionFinished`（生产零实例） | 删除；`Acquire` 注册 registry idle 回调（`cache.go:117` `repool`），`Release` 只递减引用，回池由 `notifyIdle`（`registry.go:237`）在 refs==0 且 inflight==0 时触发 |

- 同步点收敛为 5 处 `s.sub.SubmitDone()`：`SetSurfaceTarget` 切回离屏、`BeginFrame`、`PurgeSurfaceTextures`、`Destroy`、`copySubmitAndReadback`（Map 之后）。
- 兜底：nil receiver / nil session / nil registry → 立即释放；无 `sub` 时 `Retire` 当场由 `removeIfReady` 释放。standalone/零值会话路径不变。

## 2 等价性与安全性证明

- **时序单测**：`res/deferred_release_test.go` 钉住 Register→Track→Release→Retire 顺序、`SubmitDone` 前绝不 Release、重复 `SubmitDone` 只释放一次、`PendingReleaseCount` 计数；`image_cache_retire_test.go` 钉住「有钩子=延后、无钩子=立即」与 Destroy 路径。
- **负向对照**：去掉 `removeIfReady` 的 inflight 闸门 → 2 个用例红（`released before SubmitDone` / `released while submission in flight`）→ 还原绿，残留 grep 0（`negative/registry-gate-negative.txt`）。
- **conformance 级负向**：上界收紧到 0 后在 K1 未修状态整台红（`negative/conformance-negative.txt`，四后端逐条列出 +2/131072B）；K1 修后复绿——即「少释放一次 → 资源断言红」在真链路上成立。
- **D2 风险（B 队列多活一帧）实测**：见 §5 峰值对账，无可测抬升，不需要退回「同帧 submit 后」方案。
- **临时追踪工具已删**：K1 定位用的 `GPUI_VRAM_TRACE` / `VramLiveEntries` 全仓 0 命中（D4）。

## 3 K1 定位与修复（D5：根因在 res 包外，跨文件就地修）

1. **追踪**：临时 env 账本记录 `VramAdd` 调用点 PC，`k1-trace-prefix/` 逐 handle 差分显示每轮新增且上轮仍在的条目 = `textured_quad_pipeline_bicubic` + `textured_quad_pipeline_bicubic_depth_clip`（各 65536B，共 +2 条/+131072B，native/go 一致）。
2. **根因**：`TexturedQuadPipeline.destroyPipeline()` 漏销毁两条 bicubic 管线与 `bicubicShader`（创建在 `image_pipeline.go` 的 `ensureBicubic*`，销毁清单没有对应项）。
3. **修复**：`render/internal/gpu/image_pipeline.go` 销毁清单补 `pipelineWithBicubicDepthClip`、`pipelineWithBicubic`、`bicubicShader`（置 nil），与其它管线同口径。
4. **收紧**：`render/testdata/conformance/{solid_gradient,text_image}.json` 的 `max_steady_count_delta` / `max_steady_bytes_delta` 由 `2 / 131072` 收紧到 `0 / 0` 并写明原因；`TestConformanceDualBackend` native+go 三场景全 PASS（`conformance/rerun-after-queue-convergence.txt`）。
5. 能力级红线单测：`TestTexturedQuadPipeline_DestroyReleasesAllOwnedResources`（noop 设备）钉住全部自有资源销毁。

## 4 门禁结果

| 门禁 | 结果 | 证据 |
|---|---|---|
| 五目标构建 | ✅ linux×2 321/317、windows 327/323、darwin×2 319/315，段 3/4 全 exit 0；raw 红仅 2 个已知破损包（`ui_wr_t_stress`、`ui/scene/internal/gpupixel`），与 P2.4b 逐字一致 | `build/*.txt` |
| vet | ✅ 119 条诊断与 P2.4b **逐行 diff 为空**（默认 61 / nogpu 58） | `vet-full.txt` |
| 逐测试文件 | ✅ 341 文件：**301 PASS / 12 仅编译 / 1 标签排除 / 27 FAIL**；27 红在干净 HEAD `2204ad5a` 同测试名同因 27/27 复现（零新增红），且是 P2.4b 32 红的子集 | `per-file-tests.txt`、`per-file-tests-ab-clean.txt`、`per-file-tests-notes.md` |
| `-race` | ✅ res 状态机（registry/submission/cache/idle 回调）+ gpu 退休面（P6/ImageCache/TextureSet/TransientView/帧缓冲收缩/bicubic 销毁）全过 | `race.txt` |
| conformance 双后端 | ✅ native + go 三场景 PASS，K1 上界 **0/0** | `conformance/summary.md`、`rerun-after-queue-convergence.txt` |
| 负向对照 | ✅ inflight 闸门去掉 → 红 → 还原绿、零残留；conformance 级 +2/131072 红→修后绿 | `negative/` |
| API 快照 | ✅ 仅 internal 面变化：删 `ReleaseEphemeral`/`OnSubmissionFinished`/`EphemeralPending`；增 `SetRetireHook`/`SetIdleCallback`/`PendingReleaseCount`；`render` 公开面零变化；apidoc rc=0 | `api-diff.txt`、`apidoc-check.txt` |
| 残留 grep | ✅ 旧符号 0 命中、`.Drain()` 0、`retireNative` 3 调用点、`SubmitDone` 5 同步点、render 后端特有类型 0 | `grep-residual.txt` |

## 5 真窗与生命周期（隔离嵌套 Wayland，标准关闭）

隔离环境：`dbus-run-session -- gnome-shell --nested --wayland-display=gpui-p24c --no-x11`；当前桌面 X :1 只做健康检查，未在其上开窗；被测窗先构建成二进制再直接跑（信号打到被测进程）。

| 轮 | rc | 关键结果 | 判定 |
|---|---|---|---|
| `ui_l1_blank` native（60s） | 0 | 3588 帧 / 59.8fps / missed_vsync 0 / hitch 4（P2.4b 3571 帧、hitch 9） | ✅ |
| `ui_base_multiwin_cap` native `-auto-only` | 0 | 8 窗四相位 59.8/57.5/59.6/37.6fps；账本 315→316MB；pixel 16/16、golden 0.0000%；峰值 RSS 348064KB | ✅ |
| `ui_base_vram_cap` ladder native（RUN_SECONDS=300） | 124 | n=2 91.5MB / n=4 181.5MB / n=8 fps 0.0、live 205.9MB 后卡死；栈 `rwgpu.(*Surface).Present → wgpuSurfacePresent` 不返回 + 主线程 `RunSync` 等 hub | ⛔ 既有 I1 复现 |
| `ui_l1_blank` go | — | 未重跑：既有 C7（`eglMakeCurrent 0x3002`）已在 P2.4b 两次复现并干净树 A/B，与释放时序无关 | ⛔ 既有 C7（归 P2.4e） |

**B 队列释放点后移的峰值对账（D2 风险）**：multiwin 峰值 348064KB vs P2.4b 347736KB（**+328KB / +0.09%**，steady 斜率 6391.7 vs 6630.0KB/min 略降）；ladder n=2/n=4 live 与 P2.4b 逐位一致（91.5/181.5MB）；n=8 卡死态 205.9 vs 201.0（干净树 203.4）落在 I1 自身抖动带内。结论：多活一帧无可辨识的峰值抬升。

## 6 已知阻塞归属（非本步回归）

- **I1**（ladder 8 窗 native present 卡死）：P2.4b 已用干净树 `55495fdd` 在同一合成器 A/B 判定既有（Wayland/驱动面），本轮同栈复现；修复归 P2.4e。
- **C7**（gles `eglMakeCurrent 0x3002` / 关窗等锁死）：既有，P2.4b 双形态证据在案；修复归 P2.4e。
- **27 个逐文件红**：stencil/mask layout 未 pin、surface 模式 stencil 纹理断言、wgpu 校验 panic 等环境/设备面既有失败，干净树逐项同红。

## 7 改动清单（仅本线）

- 引擎：`render/internal/gpu/{render_session,image_cache,image_pipeline,gpu_types,texture_pool}.go`、`render/internal/gpu/res/{registry,cache}.go`、`render/internal/image/buf.go`、`gpu/gwgpu/gles/resource.go`（仅注释同步）。
- 测试：新增 `res/deferred_release_test.go`、`image_cache_retire_test.go`（另有 K1 修复前建立的 `image_pipeline_destroy_test.go`）；更新 `submission_gate_test.go`、`retire_queue_test.go`、`tex_shrink_r6_test.go`、`s6_7_resources_test.go`、`res/cache_test.go`。
- 用例数据：`render/testdata/conformance/{solid_gradient,text_image}.json` 上界收紧 0/0。
- 证据：`evidence/P2.4c/`（assessment、k1-trace、k1-trace-prefix、build、vet-full、per-file-tests(+ab-clean+notes)、race、conformance、negative、window-gates、api(+diff)、apidoc-check、grep-residual、tools）。

## 8 下一步

- P2.4d：同尺寸纹理分配六处 → `res/cache`（依赖本步的 registry/inflight 语义与 idle 回池）。
- P2.4e：I1（多表面 present）与 C7（gles 多窗上下文）解剖修复。