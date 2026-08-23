# ui_wr_c8_hit_overlay_xf — C8 组合真窗（R13+R6+R8）

变换/浮层/层动画下的命中测试集成回归窗。

> **组合窗只做集成，不代替任何单 R**：R13✅、R8✅ 已各自独立关闭；**R6 单能力窗
> `ui_wr_r6_layer_anim` 尚未建（§5 W5）**，本窗不能把 R6 标 ✅。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_c8_hit_overlay_xf
```

## Window / Close duration

- `1200×800`（U15）
- 关闭用 `15s`（§2.5 C8 行）；`RUN_SECONDS<5 → FAIL`（U16）；长跑 `RUN_SECONDS=60`
  时 Spike 相位自动延长到 ~90% 时长做循环压测

## Visible effect

| Region | Expectation |
|--------|-------------|
| XF-A（左上红块） | 持续旋转；Spike 相位转速 ×2 |
| XF-B（蓝块） | 呼吸缩放 0.85×–1.15×；Spike 加速 |
| MAIN 密集区 | 4×4 色格 + lbl-0..7 静态文字，全程不变色（动画/浮层不影响） |
| HOT | 每 1/8 秒变色（live paint，全程活跃） |
| Spike 浮层 | 循环开合（开 2s/关 1.5s）：模态对话框（全屏屏障+浮层按钮+菜单）与 Tooltip（无屏障）轮换，每次全新 entry |
| HIT 横幅 | 实时 `HIT: <name> ok/total ov:… rst:… ptr: …` |
| LiveHUD | `hit=n/total dirty=x/base ov=cyc OPEN/closed excess=N rot/pulse 实时值` |

## Gates（∪(R13, R8) + §2.2 族 A 硬线）

| 门禁 | 线 | 来源 |
|------|-----|------|
| scripted_ok == scripted_total（9 探针：主树 6 + 屏障消费 + 浮层 entry 命中 + 无屏障穿透） | 全过 | R13 §2 主表 |
| 主树命中探针（含浮层按钮命中） | ≥4 | R13 §2.6 |
| main_dirty_excess_frames | =0 | R8 §2 主表 |
| overlay_entries_on_open | ≥1 | R8 |
| present_policy | retained | R8 Plan C |
| fps_interval | ≥55（持续 tick 动画窗） | §2.2 族 A |
| interval_p95_ms | ≤22 | §2.2 族 A |
| pixel_probes_ok == pixel_probes_total（§2.7/U21 像素断言：探针点合成色 = 目标基色，容差 8/255；旋转/缩放目标采变换不变中心点） | 全过 | U21 §2.7.1 F0/F2/F3 |
| 快照 ≥2 张（稳态中段 + 恢复后；`C8_SNAP_DIR` 目录留档） | 存在 | U21 §2.7.2 |

### 覆盖能力说明

- **R13 命中 ≡ 绘**：探针在**旋转/缩放动画进行中**执行（比单 R 更严——角度逐帧变化，
  HitTest 必须跟得上实时变换），另含裁剪外拒绝、空白区拒绝、重叠取上层。
- **R6 层动画**：SetRotation/SetScale paint-only、boundary 隔离；密集静态区不闪。
- **R8 浮层独立合成**：分带观测（LastOverlayBandFrame）断言开浮层不脏主带；
  屏障消费语义、无屏障穿透语义、关净后命中恢复。
- 集成交互（impl_interaction）：同一探针点 (700,400) 在四种状态下分别得到
  BandMain(lbl-5) → 屏障消费(BandOverlay, obj=nil) → 穿透(BandMain) → 恢复(BandMain)。

## U20 实现点六维

| 维度 | 说明 |
|------|------|
| 正确性 | HitTestPointer 浮层优先→主树；RenderTransform.HitTest 每次调用反映射实时角度/缩放 |
| 脏区 | 分带观测：主带基线取稳态最坏值，浮层开启帧不得超出（R8 D5） |
| 缓存 | retained 下密集网格/标签回放；动画目标各自 boundary 只重录自己 |
| 边界条件 | 裁剪外拒绝、空白拒绝、重叠取上层、无屏障穿透、关净后恢复 |
| 失败模式 | 任一探针不符 / 主带脏超基线 / 浮层未观测到打开 → FAIL exit 1 |
| 窗内如何看出 | HIT 横幅 ok/total 与 ov 三位标志；HUD dirty=x/base、excess、rot/pulse 实时值 |

## 重实现记录（v2 · 2026-08-23）

> 首版 C8 关闭后用户实测发现 XF-A/XF-B 消失、黄图不显示等问题，判定实现失败并回滚。
> v2 按 wr-engine 流程重审三个 R 能力，修引擎层三洞后重建本窗。

### 引擎洞修复（本轮）

| 洞 | 层 | 修复 | 对齐 |
|----|-----|------|------|
| A: 变换子树纹理缓存双重变换 | ui/scene/textured.go | 层树构建期 `nonTranslating()` 标记（旋转/缩放≠1 子树 noTextureCache）：phase1 跳过纹理录制，phase2 向量回放 + 四角 CTM damage rect；clip 链保持可缓存（CTM 仍纯平移，recordLocal 精确） | Flutter RasterCache::CanRasterCachePicture 同位同判 |
| B: ClipRRect 命中用 AABB | ui/rendering/clip_rrect.go | HitTest 改精确圆角包含：AABB 快判 → 角限象素归一化单位圆判定（`rrectContains`），角洞不再命中；单测含 corner hole 用例 | Flutter RRect.contains（engine geometry.dart:1731）|
| C: 浮层首帧命中 / Remove 复活 | ui/overlay/{entry,state}.go | Entry 加 `laidOut` 门控（Insert 置 false，Layout pass 置 true，HitTest 跳过未 Layout entry）；Remove 清 id=0 死亡契约（re-Insert 拒绝）；Barrier-only entry 也走门控 | Flutter _RenderDeferredLayoutBox._needsLayout |

### 像素断言（U21 §2.7 首个落地窗）

- 7 探针点全量像素断言进 EvaluateGates（`pixel_probes_ok == pixel_probes_total`）：
  合成帧颜色 = 目标基色，容差 ≤8/255（F0）；旋转/缩放目标采变换不变中心点（F2/F3）；
  must-miss 点断言不等于任何目标基色。
- 快照双张留档（稳态 2.5s + 恢复后 ~0.5s，`C8_SNAP_DIR` 目录）：
  经 `PipelineApp.SnapshotAsync` 在 raster 线程执行完整重画 + SavePNG
  （窗口 present 为零回读路径，CPU pixmap 需重画填充）。

### 洞 D（gpu 层 · v2.1 追加，用户实测后定位的真正根因）

用户报告"C/E 平时不可见、拖拽窗口大小才出现"→ 引擎级复现（GPU 像素契约测试
`TestCompositeFramePacketTextured_ClipBlitPixels` 修复前 FAIL 黑屏）。根因在
`render/internal/gpu/gpu_render_context.go`：retained 稳态帧的 layer 缓存 blits
排队时 target 为 View-nil，被 F1 的 `stashPresentPending` 机制收走后，
surface flush 的恢复条件不命中 → blits 永远滞留不渲染。resize 触发
postResizeFull 全量 vector 重画（绕过缓存）所以内容"才出现"。修复：surface
flush 且非 layer-self-flush 时一律 unstash（F1 的 defer/readback 语义不变）。

### 洞 E（v2.3 · 用户二次实测后定位）

gpu stash 修复（洞 D）只救回部分目标——live-probe 证实 **ClipRRect 子树的缓存
纹理录制在真窗环境录出全黑**（record flush 无错误但立即回读 rgb=0,0,0；录制时
队列混入他层 op，mid-frame flush 触发 gpu 层 prepareTarget 的 stash 把整批 op
连同被录层一起吞掉）。修复：`ui/scene/textured.go` `nonTranslating` 将
ClipRect/ClipRRect 子树一并纳入 noTextureCache，clip 内容每帧向量回放（正确性
优先；视口列表性能代价待 gpu pass-ownership 重构后收回）。GPU 契约测试迁至
独立包 `ui/scene/internal/gpupixel`（render/gpu init 会污染 scene 包内其他
测试的无 GPU 环境假设）。验证：live-probe 五目标全可见；ui 18 包双环境零回归。

### 洞 F（v2.4 · 旋转中心不一致）

用户实测"XF-A 默认绕左上角转圈、拖拽窗口后变绕中心转圈"。根因：两条渲染路径
的旋转 pivot 不同——`RenderTransform.Paint`（vector，resize 强制重画走此路）与
`HitTest` 都绕子树中心，而 scene 层 `TransformLayer`（retained 稳态帧走此路）
绕 layer 原点。修复：`TransformLayer.CX,CY` pivot + `SetPivot`，
`BuildLayerTree` 填入子树中心，textured/composite 两处统一
`Translate(center)→Rotate→Translate(-center)`；契约测试
`TestTransformPivot_CenterMatchesVectorPaint` 断言两路径同帧同像素。

### GPU 实测（1200×800 · 15s · 3 连跑确定性）

| 指标 | run1 | run2 | run3(final) |
|------|------|------|-------------|
| fps_wall | 57.6 | 57.8 | 55.6 |
| scripted_ok | 11/11 | 11/11 | 11/11 |
| pixel_probes_ok | 7/7 | 7/7 | 7/7 |
| main_dirty_excess | 0 | 0 | 0 |
