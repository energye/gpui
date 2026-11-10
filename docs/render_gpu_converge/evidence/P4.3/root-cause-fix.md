# P4.3 治根 · 通用修法落地与验收（2026-11-10）

> 性质：用户拍板「方案 1 治根、不打补丁、对齐标准、通用」后的**底层修复**。
> 根因定位见 `root-cause.md`；本文件记录修法、红绿证据、门禁结论与边界。
> **不含 P4.3 ②③ 缓存实现**（那是修好底层后回来做的下一步）。

## 0 修法（通用，对齐标准）

| 项 | 内容 |
|---|---|
| 根因 | `render.Canvas` 无「设备自有纹理绘制」一等能力；Canvas 命令实现内部旁路产生的 GPU 纹理绘制不进录制流 → live/present 帧静默丢内容（与 P4.1 批 4 补正 3 同构） |
| 标准 | Skia `SkImage` / `GrSurfaceProxyView`：GPU 纹理是可录制的一等绘制参数，句柄用延迟实例化的逻辑引用，不存裸设备指针；Flutter `ui.Image` + Layer 树同理 |
| 本库底座 | `render/internal/gpu/res/types.go:17` 已有「SourceKey + Ref ≈ GrSurfaceProxy」；本次在 `render` 主包补**公开、可录制、不含设备指针**的逻辑引用 |
| 落地 | 新增 `render.TextureRef`（进程内 id）+ `RegisterTextureRef` / `ReleaseTextureRef` / `ResolveTextureRef`；`render.Canvas` 加一等口 `DrawTextureRef(ref, x, y, w, h)`；recording 全链补 `CmdDrawTextureRef` / `DrawTextureRefCommand` / `TextureRefBackend` / `Recorder.DrawTextureRef` / Playback 分发 / `CanvasBackend.DrawTextureRef` / bounds |
| 通用性 | 任何 Canvas 命令实现内部的缓存/离屏纹理绘制都能穿过录制回放（DrawShadow、backdrop、RepaintBoundary 式缓存、未来嵌套 Picture 都受益），不是给某个能力开后门 |
| 资源纪律 | 谁发布谁释放：`RegisterTextureRef` 由纹理持有方调用，`ReleaseTextureRef` 在其释放时调用；释放后回放解析失败并跳过，不采样悬空纹理 |

关键设计：`Canvas` 原本**故意**不收设备句柄（`canvas.go:27-35`「只放可描述的」）。`TextureRef` 是纯 id、可描述，因此进 Canvas 不破边界；真实 `TextureView` 只在回放光栅线程由 resolver 产生。

## 1 先红后绿

| 测试（`render/recording/texture_ref_test.go`） | 实现前 | 实现后 |
|---|---|---|
| `TestTextureRefReplayParity` 直画 vs 录制→回放逐像素 | 编译红（API 不存在） | **PASS**，max channel diff = 1（容差 1 显式声明，逐像素全幅不用 mean），且两条路径中心都真是蓝（缺绘制=白底会红） |
| `TestTextureRefCommandSurvivesPlayback` 引用穿过 Playback 折叠 | 编译红 | **PASS**（内层/外层各 1 op、bounds `(4,5)-(20,17)`、无 CPU scratch、可见） |
| `TestTextureRefReleasedSkipsReplay` 先录后释放再回放 | 编译红 | **PASS**（released 判 invalid、回放安全跳过、画面保持白） |

初版红测曾出现 1 级 AA 残差与「释放后 `Valid()` 仍 true」两个实现/断言细节，已按客观判据修正：`Valid()` 改为查注册表（语义=注册且未释放）、`ResolveTextureRef` 避免嵌套锁；parity 用**逐通道容差 1 显式声明**（对齐 P4.1 批 4 补正 3 的 parity 口径 2，本批收紧到 1），连跑 3 次 max diff 恒 1 稳定。

## 2 门禁结论

| 门禁 | 结果 |
|---|---|
| 红 → 绿 | ✅ 3 测先编译红、后全绿（3 次稳定） |
| `go build ./...` / `-tags nogpu ./...` | ✅ 双 rc=0（全仓，含所有 Canvas 实现者） |
| `go run ./scripts/apidoc` | ✅ rc=0；`RENDER_API_CATALOG.md` 同步 9 个新符号（§1 类型行、§3.23 小节、§6 go doc 快照、§0 包总览） |
| `go vet ./render/...` | ✅ rc=0 |
| `go test -race ./render/recording -run '^TestTextureRef'`（CGO_ENABLED=1） | ✅ rc=0 |
| 双后端 native / `GPUI_BACKEND=go` | ✅ 两侧 rc=0 |
| 干净树 A/B（`git worktree` @ `82a4985d`，不含本会话改动） | ✅ **FAIL 列表逐条一致**（recording 3 条、render 10 条），本会话改动**零新增失败**；既有红按用户交接「别顺手修」不动 |
| 负向对照 | ✅ 临时强制 `ResolveTextureRef` 解析失败 → parity 红（max diff 19 > 1）→ 还原绿 → `git diff` 零残留 |

### A/B 既有红清单（非本会话引入，如实登记）
- recording：`TestCanvasBackendReplaysAtDeviceScale`、`TestP37E3CanvasBackendScopedShaderNoLeak`、`TestDrawShadowReplayParity`
- render：`TestR72_ApplyBlur_GPUFilterTextureNoExportPrefer`、`TestRasterizerValueMatchesAnalytic`、`TestS3c_M3_*` 六项、`TestS69_HeavyBudget_TierGates`

## 3 边界与未做

- 本次是**底层能力修复**，P4.3 的 ②③（跨帧缓存 + 模糊区域裁剪）与 ④（红测 + 双后端真窗 + live 抓帧 + 显存账本）**尚未重新落地**：上会话的实现已按纪律回退（`82a4985d`），下一步用 `DrawTextureRef` 重新落地，命中路径走可录制命令而非 `Context.DrawGPUTexture` 旁路。
- `TextureRef` 注册表是进程级；resolver 由纹理持有方提供，回放时返回当前设备有效视图。多设备/跨窗口语义由注册方保证（与 Skia `GrSurfaceProxy` 的实例化契约一致）。
- 导出后端（image/PDF/SVG）不实现 `TextureRefBackend`，跳过该命令（与既有 `GPUTextureBackend` 同语义）。
- 金图双后端 0.0000% 锁定未重种；本批零画面改动。

## 4 状态

- ① 分段归因 ✅；**底层治根 ✅（本文件）**；②③ 待重新落地；P4.3 仍 🟡。
- 工作区未提交：`render/texture_ref.go`、`render/context_texture_ref.go`、`render/canvas.go`、`render/recording/{command,command_ops,backend,recorder_canvas_ops,recorder,backend_canvas,bounds}.go`、`docs/RENDER_API_CATALOG.md`、`docs/ENGINE_UI_WIDGET_RENDER.md` §10、三个常驻钉测试、本证据与台账。
