H22 修复正本（2026-11-07，拒绝账方案；评估→停报告拍板→红测→实现→真窗证据全链）
================================================================================

被测洞（一句话）：scene 回放腿 `render/scene/gpu_renderer.go:267` 用 `_ =`
丢弃 `DrawImageQuad` 的全套具名拒——同一笔「透视 quad+材质」直调口错误有名有姓，
回放腿四可观察面全空（错误 nil、计数 0、无日志、无查询口）；呈现帧走的正是
录制→CanvasBackend 回放→这条路，静默真实发生。同文件材质门同族 discard 三处
（`:146 _ = dc.FillPath`、`:169 _ = dc.Fill()`、`:191 _ = dc.StrokePath`）一并盘。

一、复现实测（复现脚本式诊断，跑完即删，输出留痕）
----------------------------------------------------
同一枚「透视 quad+粘滞材质」：

| 路 | 结果 |
|---|---|
| 直调 `dc.DrawImageQuad(...)` | `err=render: material leg does not support this combination yet (perspective/bicubic/projection)` ✓ 可见 |
| 回放 `GPUSceneRenderer.RenderScene(同景)`（修前） | `err=<nil>  MaterialSkips=0  ShaderBackendFails=0  GPUOps=0` —— 全静默 |

腿路口径（实测定位）：离屏单测进程无 GPU 加速器注册（`_ "render/gpu"` 只在
真窗进程引入）⇒ 干净 quad 走静默 CPU 分裂腿（不计数）、材质+透视在
`m4_quadimage.go:74-77` 校验期具名拒（`MaterialSkips` 不动——校验期拒不计数是
③/⑥ 冻结语义本身）。

二、拍板（停下报告，用户）
--------------------------
「材质+透视真画出来」是批 3b 登记的后做能力（组合顶点机大活），另立项不混本修；
H22 只修「拒绝的可见性」——**拒绝账**：回放腿把 draw 动词具名拒从 `_ =` 改成
真金可查账，画面行为零变化。

三、修法（render/scene/gpu_renderer.go 单文件）
------------------------------------------------
1. `GPUSceneRenderer` +字段：`refusals int` / `lastRefusal error` /
   `loggedRefusal map[string]bool`；+导出方法 `Refusals() int`（最近一次
   RenderScene 的具名拒笔数，按场景重置——与 item material 同一份显式复位
   纪律）与 `LastRefusal() error`（最近一笔具名拒，errors.Is 对 render 口
   哨兵判因）；`loggedRefusal` 去重跨场景保留（同 reason 不因反复回放刷屏）。
2. 四处 `_ =` 全改 `r.recordRefusal(...)`：TagImageQuad（quad 口全家哨兵）+
   TagFill/TagFillRoundRect/TagStroke（材质门 fill/stroke 的 H23 链具名错）。
   `recordRefusal` 计数 + 留 last + 每 reason 一次 stderr 日志（H23 负面缓存
   风格）：`scene replay: draw refused by render leg (N this scene): <err>`。
3. `RenderScene` 返回值保持 nil——单笔拒不升级为整场景错误（recorder.go:276
   会把 RenderScene 错误沿 Playback 上传，升级=可能掀呈现帧，H23 教训反面）。

四、验收（三证据）
------------------
1. **红测先行**：`p37_h22_replay_refusal_ledger_test.go` 三钉先编译红
   （`h22-red-test.txt` 正本：Refusals/LastRefusal undefined）后全绿——
   钉 1 拒绝可查（计数=1+具名 ErrQuadMaterialUnsupported+帧不死）、
   钉 2 反假阳（干净 quad 回放≡直调**逐位** diff=0 且账干净）、
   钉 3 拒不污染后续（混合回放 ≡ 仅可画笔回放**逐位**，账只记一笔）。
2. **逻辑探针**：⑥ `ui_wr_p37_scene` 探针 14→**15**
   （`h22_replay_quad_refusal_accounted`：拒景回放后 Refusals==1 + 具名）。
3. **真窗金图**：画面语义零变化 ⇒ 金图不重播种直接双后端复验——native
   **19/19 0.0000%**/921600px、go **19/19 0.0000%**、skips=0
   （`h22-window6-runs.txt` 正本）。

五、门禁
--------
- `go vet ./render/...` rc=0；gofmt 净；`apidoc` rc=0（公开面 +2 方法已同步
  `docs/RENDER_API_CATALOG.md` §6.2 与速查区）。
- render/scene 触及 gpu_renderer.go 的测试**逐文件**回归 7/7 PASS
  （gpu_renderer / gpu_renderer_regression / p37_3e3_material_gpu /
  p37_6_scene_record / p37_3e_quad / gpu_leg_stats / p37_h22）。
- nogpu：`go vet -tags nogpu ./render/...` rc=0（坑 1 收口后基线）。
