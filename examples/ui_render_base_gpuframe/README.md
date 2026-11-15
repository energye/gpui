# ui_render_base_gpuframe — GPU 帧直控族直驱真窗

O12 批 1a W2（RENDER_FEATURE_COVERAGE §12 NONE → examples 能力测试，台账 O12 行）。
直驱窗（`wrkit.OpenDirect`，GL 线程纪律同 PipelineApp）；场景腿在 offscreen GPU 视图上直调。

## 覆盖面（O12 NONE 条目）

`Context.BeginGPUFrame / FlushGPUWithViewDamage / FlushGPUWithViewDamageRects /
DiscardPendingGPU / PendingGPUCount / DropGPURenderContext / MemDigCmdBufs /
LayerPoolStats / ResetLayerPoolStats`。

- 卡 A BeginGPUFrame 跨帧重绘：persistent dc 同视图跨帧——重置后清帧态（旧帧让位），
  不重置 LoadOpLoad 保留陈帧；两读回缩略图 + 新内容基准，一图看清开关语义。
- 卡 B 损伤 flush 直调：`FlushGPUWithViewDamage(Rects)` 直调 + 无操作 nil-present
  作读回标记；单矩形/双矩形落点 + 无损伤基准（语义同 W1 损伤族）。
- 卡 C 待决计数与排水：3 笔排队 → `PendingGPUCount>0` → `DiscardPendingGPU` → 0 →
  排水后帧只有新落点，被弃内容不出现。
- 卡 D 会话丢弃与统计：`MemDigCmdBufs` prev≥0 → `DropGPURenderContext` → prev=-1 →
  重建后重绘正确 + prev≥0；`LayerPoolStats`（PushLayerIsolated×3 → gets/puts/hits>0）→
  `ResetLayerPoolStats` 归零。

## 证据链（三证据）

- 逻辑探针：窗面 2 条 + 离屏腿 15 条（stderr `gfleg:` 前缀；卡 D 只画判定不画值——黄金图稳定）。
- 像素探针：快照 10 点（缩略图落点/保留点/清帧点）。
- Golden：顶栏+图例+四卡逐位 0.0000%（状态带在金区外）。
- 抓屏对照：终帧 vs 快照同区 ≤1%（0.31%）。

## 已知限界与发现

1. 层池计数只反映 pixmap 池（GPU 会话的层 RT 不经池，计数不动——`PushLayer(BlendNormal)`
   还走 F1 opacity-group 快路径跳过池；探针用 `PushLayerIsolated` + Drop 后 CPU 层路径）。
2. 读回透明区的呈现随会话状态摆动（A.reset 透出卡底 / C.drained 黑底两轮不同）——
   与 W1 的 `damage-present-findings-2026-11-15` 同族（wgpu 会话状态机），
   该点由黄金图逐位钉死，探针只钉确定性色。
3. CPU 层探针与 GPU 会话重建交错的读回异常（重建读回全黑除首 op）——按纪律不在门禁内
   交错（重排为先重建后层池），异常现象记入台账子项待查。

## 门禁

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_gpuframe -auto-only   # 门禁（JSON+exit code）
go run ./examples/ui_render_base_gpuframe                            # 常驻人工窗，关窗出汇总
```

黄金图基线按 native wgpu 播种（`GPUI_ACCEPT_GOLDEN=1` 播种后需人工过目再提交）。
`WR_GF_DBG=1` 打印重建读回采样点。
