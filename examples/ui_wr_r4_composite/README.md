# ui_wr_r4_composite — R4 层 Composite Present 真窗

## Run

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_r4_composite
```

## Window / Close duration

- 窗口 **1200×800**（U15），关闭用 **RUN_SECONDS=15**（§2.5；本地建议 30）。
- `RUN_SECONDS<5` → `FAIL: RUN_SECONDS must be >= 5 to close R4 (U16)` + exit 1。
- GPU 真窗（X11 + wgpu）；无 GPU 环境 → `FAIL: window open (needs_gpu_window)`。

## Visible effect

| Region | Expectation |
|--------|-------------|
| TopBar 标题/相位 | 标题 "R4 retained composite"，相位文字随脚本切换（STEADY/SPIKE/RECOVER） |
| 左图例 legend | 静态，不随帧重绘 |
| Body 网格 16 格（4×4） | 静态色块，含嵌套 boundary；网格外右下角 PhaseLabel（`PHASE: STEADY` 等）仅相位切换时变化 |
| Body 动态热点 hot | 红块 稳态（align 0.78,0.86）/ 黄块 SPIKE（0.84,0.86）/ 蓝块 RECOVER（0.78,0.97）；**Align 布局驱动**：位置 = (body−hot)×比例，相位切换移动/变色；resize 时 Layout 自动重算 offset（无 clamp、无固定坐标） |
| 底栏 HUD | 实时 `dmg=.. mode=.. skip=..` + PASS/FAIL 预览色；`WR_HUD=0` 可关 |

## Gates（§2 主表 R4 + §2.2 全族 FAIL 线）

- `present_policy` = `retained`（JSON）
- `damage_ratio_avg ≤ 0.35`（远小于全屏；稳态实测 ≈0.011，仅首帧 warmup 全屏 + 相位切换帧小矩形）
- `present_mode` 非 `full`（稳态 `damage_union`/`damage_multi`）
- `boundary_skip > 0`（静态区走 boundary 缓存）
- `fps_interval ≥ 55`（持续 tick 窗，§2.2.2 60 档）
- 族 A–J 全字段必采（§2.2.1）；缺失 → FAIL
- `vsync_source` 必输出；`fallback` 不宣称锁 60Hz
- 族 E：短正确性窗 `rss_slope_kb_per_min` 必采、默认告警（`slope_gate=off`，<15s 窗允许）
- 族 J：ui 无 import gpu / 无 cgo / 不降画质

## 实现点六维（U20）

| 维度 | 落点 |
|------|------|
| 正确性 | dirty PictureLayer 重录进 offscreen RT（FlushGPUWithView LoadOpClear），clean 层纯纹理 blit；无 GPU 时 record 返回 false → 矢量重放兜底，内容仍正确 |
| 脏区 | damage = 仅重录层几何 rect（TrackDamageRect 逻辑→物理）；blit 与 record 回放不污染 FrameDamage（SetDamageTracking 抑制）；稳态 fd 仅 hot 一格 |
| 缓存 | PictureTextureCache：纯平移子树 bounds 尺寸纹理（recordLocal），clip/rotate/scale 走全窗纹理；LRU 64；BeginFrame/recordFrame 区分 re-record 与 blit；Entry 释放延迟 2 帧（避免 bind-group slot cache 悬空 → conv.rs panic） |
| 边界条件 | 首帧 53 层全重录（warmup full）；resize/evict 后一波重录再回稳；空 bounds 文本层用字体度量（Face.Metrics/Advance）得文本矩形；负偏移（baseline 以上）blit off 处理 |
| 失败模式 | DiagCreateNil/DiagFlushErr/DiagBlitFail 计数（JSON 外 WR_DIAG 可查）；blit 失败回退矢量；无 GPU 全路径降级 |
| 窗内如何看出 | HUD `dmg%`（≈0.01）+ `mode`（非 full）+ `skip`（boundary_skip 累加）+ 相位文字切换；网格静区颜色稳定不闪 |

## 引擎实现（本次修复）

- `ui/rendering/align.go`（新）：`RenderAlignBox`（Flutter Align 语义）——填满父约束、child 宽松布局、offset=(父−子)×比例；`SetAlignment` 脏 layout+child paint；`node.go` baseOf 注册。示例经 `wrkit.Panel.Align` 使用（布局驱动，resize 自动跟随）。
- `ui/scene/textured.go`（新）：`PictureTextureCache`（record/recordLocal/evictForNew/BeginFrame/recordFrame/releaseDeferred/drainDeferred/restrictive/transformBounds/measureTextBounds）+ `CompositeFramePacketTextured`。
- `ui/embedder/pipeline_app.go`：`presentPacketTextured`（逐脏矩形 TrackDamageRect、boundaryFrameSnap、ConsumeNeedsPaint）。
- `render/context_image.go`：`TextureView` 别名（ui→render 边界，禁止 ui→gpu）。
- 崩溃修复：render_session view→bind-group slot cache 引用在飞 command buffer → 纹理延迟 2 帧 release（conv.rs:1643 panic 根因）。
