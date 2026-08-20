# ui_wr_c2_retained_scene — C2 组合真窗（R3+R4+R4b+R5 集成）

W2 组合窗：retained 整场景——多 boundary + Picture 回放 + 远离脏点 + damage present。
只做集成回归，**不替代** R3/R4/R4b/R5 单能力窗（U6）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_c2_retained_scene
```

`RUN_SECONDS` 可覆盖；`<5` → `FAIL:` + `exit 1`（U16）。

## Window / Close duration

- 客户区 **1200×800**（U15），标题 `gpui ui_wr_c2_retained_scene — R3+R4+R4b+R5 集成`
- 关闭用 **15s**（§2.5 C2 行；本地观察可加长）
- GPU 真窗；无 GPU/X11 → `FAIL: window open (needs_gpu_window)` + exit 1

## Visible effect

| Region | Expectation |
|--------|-------------|
| 左上 区 A（R3 嵌套） | 3 层嵌套 boundary（outer 220 / mid 160 / inner 100，白描边视觉靠色阶区分）；inner 内 50×50 红块 Steady/Recover 每帧（或 0.5s）变色，外层 mid/outer 静（**仅 inner 重录**，外层只重合成）；Spike 相 outer 背景翻转（整巢重录，巢外不变） |
| 中上 区 B（R4 静态阵） | 5×3 色格阵（70×70 全 boundary）+ 3 行静态标签，全程静止不重绘；右下 hotA 60×60 红→橙→青随相位变色（Align 布局驱动，resize 自动跟随） |
| 右上/左下 区 C（R4b 双热点） | hot1（右上角）蓝/紫、hot2（左下角）绿/黄，每帧变脏 → 两独立 dirty layer id；中间静态区不重绘 |
| 中下 区 D（R5 Picture） | 左 DIRCT（直绘）vs 右 REPLAY（回放）同构图并排（路径填充/描边/矩形/图/文 8 ops），肉眼两侧一致；**0.5s 低频刷新**（retained 下回放缓存静态走 blit skip，Spike 相每帧刷新），下方标签标区 |
| 顶部 TopBar | 能力 ID + 当前相位 |
| 底部 HUD | 实时 fps/p95/policy/present_mode + `skip=/rr=`（boundary 缓存）+ `sum=`（真实重绘像素比）+ `ids=/multi=`（dirty layer）+ `ops=`（picture op 数）+ `union=`（观测）；绿=达门禁趋势，红=破线预警 |

## Gates

门禁 = ∪(各 R 门禁)（§3.1.1）：

| 来源 | Gate | 硬 FAIL 线 |
|------|------|-----------|
| R3 | `boundary_skip` | `>=3`（干净 boundary Replay；区 A mid/outer + 区 B 色格） |
| R3 | `boundary_rerecord` | `>=2`（脏 boundary 重录：inner 每帧 + Spike outer 整巢） |
| R4 | `present_policy` | `== retained`（`RequireRetainedPolicy`） |
| R4 | `damage_ratio_sum` | `<=0.35`（**sum of rects 真实重绘像素**；峰值帧 ≈0.19，R4b 先例） |
| R4 | `present_mode` | 非 `full`/`idle`（damage_union/damage_multi） |
| R4b | `dirty_layer_id_max` | `>=2`（hot1+hot2 对角同帧脏 → 两独立 id） |
| R4b | `damage_multi_frames` | `>=1`（多矩形独立 scissor 发生） |
| R5 | `picture_op_count` | `>=3`（8 ops：FillPath/StrokePath/FillRect/StrokeRect/2×DrawImage/2×DrawString） |
| 持续 tick | `fps_interval` | `>=55`（`RequirePersistentFPS`，60Hz 档 §2.2.2） |
| 持续 tick | `interval_p95_ms` | `<=22` |
| §2.2 | 全族 A–J | wrgate.BuildReport 必采；缺失字段 FAIL（schema 检查） |

诚实性说明（README 契约）：

- **门禁用 `damage_ratio_sum`（sum of rects 真实重绘像素，R4b 先例）**：峰值帧 = inner 100² + 3×hot 60² + R5 2×190² + HUD 1200×72 ≈ 0.19 ≪ 0.35。`damage_ratio_avg`（union bbox）≈0.59 为脏层分散的**几何必然**（R4b 文档同一结论：对角脏点 union≈0.49 而真实 sum≈0.026），仅作观测字段、不设门禁。
- `vsync_source` 如实输出；fallback 禁止宣称锁 60Hz。
- `cpu_ui_pct`/`cpu_raster_pct` 必采（动画窗禁双 0）；本窗 cpu_raster≈40%（每帧多区重录真实开销）。
- 15s 正确性/集成窗：`rss_slope` 仅必采、默认不 FAIL（§2.2.4 短窗允许，`slope_gate=off` 语义同 R4b/R5/R11 先例）。RSS 峰值 ≈270MB 为每帧 ~9 个 picture 层重录的 offscreen 纹理分配（rss_after_close 回落证明非泄漏）。

## 集成验证（§3.1.2 C2 行）

| 跨能力边界 | 本窗如何证明 |
|-----------|-------------|
| damage ratio ≪ 1 | retained 稳态 sumRatio≤0.35；静态色格阵/嵌套层不参与损伤 |
| dirty_layer_id ≥ 2 | hot1+hot2 对角同帧变脏 → 两独立 boundary id；R4 动画区再贡献第 3 个 |
| Picture 回放正确 | 直绘 vs 回放同构图并排（8 ops），两侧肉眼一致；`picture_op_count=8` |
| boundary skip > 0 | 区 A mid/outer + 区 B 色格阵每帧 skip（Replay 不重录） |

**引擎洞记录（本窗暴露，已修 ui/rendering）**：`SubtreeNeedsPaint` 原穿透子 boundary，内层脏会误判所有嵌套祖先进 DirtyLayerIDs → 被迫 re-record（每帧 14 ids、RSS 暴涨）。修复 = `layerSubtreeNeedsPaint`（遇 boundary 停，BuildLayerTree 专用）；单测 `TestBuildLayerTree_NestedBoundaryInnerDirtyOnly` + ui 全树回归 + R3/R4b/R4 GPU 真窗回归全绿。详见 `docs/ENGINE_UI_WIDGET_RENDER.md` §10。
