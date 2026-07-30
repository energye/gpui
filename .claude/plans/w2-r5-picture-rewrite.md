# R5 Picture 录/回放 — 推翻重写计划

## 目标

将 `examples/ui_wr_r5_picture` 从当前「3 个矩形」升级到 §2.6.2 质量条，关闭 R5 ✅v2。

## 当前问题

现有 R5 只有 3 个 FillRect，缺少：
- Path 填充/描边
- 文字（DrawString）
- 变换（Rotate/Scale）
- wrkit Shell（TopBar/Legend/Body/HUD）
- PhaseClock 三阶段
- 质量门禁（wrgate.BuildReport + EvaluateGates）
- §2.2 全族 A–J 指标

## 改动范围

**仅改 1 个文件：** `examples/ui_wr_r5_picture/main.go`
**同步更新：** `examples/ui_wr_r5_picture/README.md`

不改引擎层（ui/render/gpu）。

## 实现方案

### 场景布局（1200×800）

```
[======== TopBar (0,0) 1200×48 ===============================]
[ Legend (12,60) 260×300 ] [ Direct (284,60) 440×480 ] [ Replay (736,60) 440×480 ]
[================ LiveHUD (0, h-72) 1200×72 ==================]
```

### 左侧 DIRECT — 每帧直绘

`drawContent(dc, ox, oy)` 函数，直接调用 DC API：
1. FillRect — 4 个色块（2×2 网格）
2. FillPath — 三角形路径
3. StrokePath — 圆角矩形描边（3px）
4. DrawString — "Picture ≡ Direct" 文字标签

### 右侧 REPLAY — 录制一次，每帧回放

`scene.RecordPicture` 录制同样 4 类 op（相同坐标），然后每帧 `pic.Replay(dc)`。

关键：PictureRecorder 使用与 `drawContent` 相同的坐标（panel-local），`OnPaint` 中 `dc.Translate(ox, oy)` 对齐到 panel 位置。

### 质量结构

| 项 | 实现 |
|----|------|
| wrkit.NewShell | ✅ TopBar + Legend(8行) + Body + LiveHUD |
| PhaseClock | ✅ Steady→Spike→Recover（Spike 加速 MarkNeedsPaint 频率） |
| EnsureUIFace | ✅ 字体加载 + 标签可见 |
| Legend | ✅ 8 行色块+文字（FillRect/FillPath/StrokePath/DrawString/Replay/Clear/Phase/HUD） |
| 多区域 Panel | ✅ 5 区（Top/Legend/Direct/Replay/HUD） |
| Extra | ✅ impl_correctness/dirty/cache/edge/fail/visible 六维 |
| wrgate.BuildReport | ✅ §2.2 全族 A–J |
| wrgate.EvaluateGates | ✅ presents + policy + fps + picture_op_count ≥ 5 + replay_frames ≥ 10 |

### 门禁

| Gate | 条件 |
|------|------|
| RUN_SECONDS ≥ 5 | U16 |
| present_count ≥ 1 | 有帧 |
| present_policy == full_paint | R5 不测 retained |
| fps_interval ≥ 55 | 60Hz-class |
| picture_op_count ≥ 5 | FillRect×2 + FillPath + StrokePath + DrawString |
| replay_frames ≥ 10 | 回放帧数 |

### README 更新

- 可见效果表（Direct vs Replay 两栏）
- 实现点六维
- 运行命令

## 不做的事

- 不改引擎层
- 不加 DrawImage（需要真图片文件，复杂度高，op 类型已在 unit test 覆盖）
- 不加变换 op（PictureRecorder 无 Rotate/Scale API；变换在 layer 级别 R6 处理）
- 不做像素级对比（unit test 已覆盖；真窗靠视觉对比 + op_count 门禁）

## 验收

```bash
RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture   # PASS
RUN_SECONDS=15 go run ./examples/ui_wr_r5_picture  # 观察 CPU/RSS
```

stderr 输出 PASS + JSON 指标；左右两栏视觉一致。
