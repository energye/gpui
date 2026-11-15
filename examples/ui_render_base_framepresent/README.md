# ui_render_base_framepresent — 手动呈现族直驱真窗

O12 批 1a W1（RENDER_FEATURE_COVERAGE §12 NONE → examples 能力测试，台账 O12 行）。
**直驱窗**：不走 embedder.PipelineApp——帧循环由 `wrkit.OpenDirect` 骨架自己跑
（platform.Open → embedder.OpenPresentTarget → `Target.PresentWith*`，GL 线程纪律同 PipelineApp：
建链/每次 present/快照/Close 全走 `raster.RunGLSync`）。

## 覆盖面（O12 NONE 条目）

`Context.PresentFrame / PresentFrameAuto / PresentFrameFull / PresentFrameDamage /
PresentFrameDamageRects / PresentFramePlan / PlanPresent / FrameDamage` +
包级 `PlanFramePresent` + `PresentMode*` 常量 + `FramePresentPlan/PresentOutcome` 类型。

- 卡 A 全量呈现：`Target.PresentWith`（内部 PresentFrameFull 路径）→ outcome=full + os_full 计数。
- 卡 B 空闲帧：3×`PresentAuto(nil)` → Idle、swapchain 不 acquire、OS 计数 Δ=0、抓屏逐像素不变。
- 卡 C 损伤呈现：补丁纹理 blit + `Invalidate`×2（保留 UI 同形：blit-only 帧 + 层界损伤记账）→
  `PlanFramePresent` 判 multi → 独立剪刀直派。
- 卡 D 离屏直驱：每个 `Context.PresentFrame*` 以 nil present 直调 offscreen GPU 视图
  （视图保持可读 → `Image()` 读回像素断言）+ `PlanFramePresent` 纯函数五态探针。

## 证据链（三证据）

- 逻辑探针：窗面 9 条（outcome 模式/OS 计数/live PlanPresent）+ 离屏腿 15 条（stderr `offleg:` 前缀）。
- 像素探针：快照 12 点（四卡内容 + 卡 D 四个读回缩略图的落点/保留点）。
- Golden：顶栏+图例+四卡 802608px 逐位 0.0000%（状态带在金区外）。
- 抓屏对照：终帧 vs 快照同区 ≤1%；空闲段前后抓屏 diff=0；损伤段前后抓屏四点语义
  （`damage_grab_evidence`）。

## 已知限界与发现（见 `docs/render_gpu_converge/evidence/damage-present-findings-2026-11-15/FINDINGS.md`）

1. **wgpu-native 表面路径损伤呈现=清屏+只画损伤内内容**（#6）：既有「wgpu-native 仍全量
   （平台限界）」的精确定位；GL 后端保留完美生效（gap/legend kept=true 实测）。
   当前生产流（FullPaint / packet 全内容合成帧）不触此路径。
2. **wgpu 会话状态相关的损伤裁剪不一致**（#3 UV 漏剪 / #4 未触点变暗）：新发现，
   归位台账子项，修复面=render/internal/gpu 会话状态机，另批处理。
3. 矢量帧损伤=MSAA 全清（文档明载限界，已钉成行为探针 vector_msaa_clear）。
4. OS 计数：wgpu-native 恒 0（无 OS damage 协议，诚实上报）；GL full 计数递增实测 ✓；
   GL partial 计数受 buffer-age 升格影响（语义归 P1 指标链），只记录不判死。

## 门禁

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/ui_render_base_framepresent -auto-only   # 门禁（JSON+exit code）
GPUI_BACKEND=go RUN_SECONDS=10 go run ./examples/ui_render_base_framepresent -auto-only  # GL 冒烟证据
go run ./examples/ui_render_base_framepresent                            # 常驻人工窗，关窗出汇总
```

黄金图基线按 native wgpu 播种（`GPUI_ACCEPT_GOLDEN=1` 播种后需人工过目再提交）。
改卡 D 探针行/发现行会改卡内文字 → 黄金图需重播种并过目。
