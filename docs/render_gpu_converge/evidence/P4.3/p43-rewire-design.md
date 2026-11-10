# P4.3 ②③ 接线设计（底层治根后，2026-11-10）

> 性质：下一步开工依据，**零代码**。底层 `TextureRef` 通用能力已落地并全门禁绿
> （`root-cause-fix.md`）；本文回答「②③ 缓存怎么消费这个能力才不重蹈覆辙」。

## 0 上一版为什么失败（复述，防重犯）

上会话 ②③ 把缓存命中放在 `*Context.DrawShadow` 内部、直接调
`c.DrawGPUTexture(...)`。窗口 live 帧是「UI 线程录制 → 光栅线程回放」：
`CmdDrawShadow` 回放时才执行 `Context.DrawShadow`，这次旁路 queue 的纹理 quad
**不属于任何录制命令**，present 重放时没有它 → 阴影丢失（golden diff ~33%）。
六种贴法都失败，因为问题不在贴法，在「绕过录制契约」。

P4.1 批 4 补正 3 的同构教训：Canvas 命令缺失 → live 回放丢内容，靠补**命令**解决。
本批底层已补 `Canvas.DrawTextureRef` 这条命令（`root-cause-fix.md`）。

## 1 接线铁律

> **缓存查询/命中必须发生在录制阶段，命中路径必须产生一条可重放命令
> （`DrawTextureRefCommand`），不能在回放期的 `Context` 里旁路 queue。**

两条 Canvas 实现都要查同一份缓存：

| 实现 | 时机 | 命中 | 未命中 |
|---|---|---|---|
| `*Recorder.DrawShadow`（UI/录制线程） | 录制期 | `r.DrawTextureRef(ref, bx, by, bw, bh)` → 录 `DrawTextureRefCommand` | 录 `CmdDrawShadow`（回放由 Context 执行原链并填充缓存） |
| `*Context.DrawShadow`（光栅/直绘） | 回放期或 live-context 直绘 | `c.DrawTextureRef(ref, ...)`（解析+queue，此路径本身就是最终执行） | 原链 `PushLayerIsolated → 剪影 fill → ApplyBlur → PopLayer`，并在 `PopLayer` 前把阴影 tile 发布成 `TextureRef` |

缓存表放 `render` 包进程级（与 `TextureRef` 注册表同域），两个实现共享；键一致
才能跨帧命中。`TextureRef` 由纹理持有方注册、释放时 `ReleaseTextureRef` + 纹理
`release()`（谁建谁放）。

## 2 键与内容

- 键 = `(path 内容哈希, elevation, color, CTM, dpr, 阴影设备紧致界)`，**禁路径
  指针键**（录制每帧克隆路径，指针必失效）。`Recorder` 已有 CTM 模拟（bounds
  走查），`Context.totalMatrix()` 提供直绘侧 CTM，两侧口径必须一致。
- 紧致界 = path 设备界 + offsetY 抬升 + 3σ 模糊边距，与 clip / surface 相交
  （`shadowDeviceBounds` 上会话已实现并验证过 tile 尺寸 280×210 << 1200×800）。
- 缓存内容 = **紧致界大小的阴影 tile**（不是整幅），这同时就是步骤②的「模糊
  区域裁剪」：静态帧只搬一小块，首帧仍付一次全画布链。
- 投影分支（`projActive`，P4.1 #4 洞族）显式不缓存，保持现状不动。

## 3 资源纪律

- `CreateOffscreenTexture(bw,bh)` 建独立纹理（Vram 由双端既有记账覆盖），
  `UploadRGBAToView` 填 tile，`RegisterTextureRef(resolve)` 发布逻辑引用。
- LRU（条数 + 字节上限）淘汰时：`ReleaseTextureRef(ref)` + `release()`。
- `Context.Close()` 清空该 Context 持有的条目；进程级表不留悬挂 ref。
- 验收：关窗显存/堆快照回落基线（AGENTS「资源谁建谁放」）。

## 4 先红后绿与验收拆解（下一阶段）

1. **红测先行**（能力单测，实现前红）：
   - 成本断言：静态二帧第二帧 < 首帧 1/4 + `GPUI_P43_BREAK=no-cache` mutation 反证；
   - 裁剪前后阴影像素逐位一致（CPU 全画布参照，金图**不重种**）；
   - 缓存命中/失效断言（改 elevation / 颜色必须重算；同参数命中）；
   - **新增录制回放 parity**：`Recorder` 录 DrawShadow → 回放画面 ≡ 直画画面
     （锁定上版失败点，逐通道容差显式声明）。
2. **实现**：按 §1 两实现接线，缓存池 + LRU + 资源账本。
3. **真窗验收**（`examples/ui_wr_p41_drawshadow`，`-auto-only` JSON）：
   - fps 恢复（`interval_p95 < 22ms` 或 `fps 55+`）；
   - **双后端** webgpu(native) + gwgpu-gles(`GPUI_BACKEND=go`) 各自金图 0.0000%；
   - **live 抓帧验收**（`evidence/P4.1/batch4/live-frames/README.md` 临时钩子，
     验收完删钩子）live vs 金图 ≈ HUD 摆动级；
   - 显存账本关窗快照无泄漏。
4. **回归**：recording/render 逐文件、双构建、`-race`、vet、apidoc rc=0；
   干净树 A/B 确认零新增失败；负向 mutation 红→还原绿零残留。
5. **收尾**：§2/§2.1/§7 当次落盘、证据落盘、PLAN 口径回写、catalog/apidoc、
   任务镜像同步、提交前 `git diff --stat` 与用户确认、逐文件 add、不推送。

## 5 边界

- 本设计**不改底层能力**（`TextureRef` 已绿），只消费它。
- 零新增公开 API 预期（缓存池为 `render` 包内未导出）；若走到要加接口先停下问。
- 金图双后端 0.0000% 锁定不重种；`fps_gate=off` 正确性窗语义不变。
