# ui_wr_r5_picture — R5 Picture 录/回放

> 状态：**✅v2**（2026-10-21 模式 2 三证据重关；`-auto-only` 5s 两连跑 0 退出）
> 旧的 `direct_replay_eq: true` 是硬编码，已删除；现在由 GPU 抓屏的同点 RGB 配对实测。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# gate：5s，JSON + 退出码（内容静态，相位只驱动重绘）
go run ./examples/ui_wr_r5_picture -auto-only
RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture -auto-only

# manual：常驻观察；-manual-seconds N 定时关闭
go run ./examples/ui_wr_r5_picture
go run ./examples/ui_wr_r5_picture -manual-seconds 2

# 首次播种 Golden（之后必须连跑两次 0.0000%）
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture -auto-only
```

## Window / Close duration

1200×800 · gate 档 5s（`-auto-only` 或 `RUN_SECONDS=5`；U16 下限 5s）；manual 档默认常驻。
GPU 真窗必需。关闭用时长按 §2.5（5s 档）。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px → 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。
能力元素（直绘区/回放区/标签）一律 **§2.6.1 Align 布局驱动**（`Body.Align`，比例定位，resize 自动跟随）；静态色格阵为装饰用 `Place`。

```
+--------------------------------------------------------------+
| TOP: ui_wr_r5_picture  R5 Picture 录/回放                     |
+----------+---------------------------------------------------+
| LEGEND   | DIRECT 直绘区           REPLAY 回放区   4x3 色格    |
| (260px)  | Align(0.10,0.10)       Align(0.55,0.10)  40x40     |
|          | 200x300 同构图          200x300 同构图              |
|          | 三角填充(红) 描边(蓝) 填充(绿) 描边(黄)             |
|          | 图 1:1+缩放(绿) 文字 PIC-REPLAY / v2               |
+----------+---------------------------------------------------+
| HUD: R5 | phase | fps | p95 | picture_ops | bounds | 门禁     |
+--------------------------------------------------------------+
```

| # | 区域 | 内容 | 预期效果 |
|---|------|------|----------|
| 1 | DIRECT 直绘区 | 每帧直绘 8 元素：红三角（FillPath）+ 蓝描边（StrokePath 3px）+ 绿填充矩形 + 黄描边矩形 + 绿图 1:1(150,20) + 绿图缩放 32×32(150,100) + 白字 `PIC-REPLAY`/`v2` | 与回放区**同点同色**（像素配对 5/5） |
| 2 | REPLAY 回放区 | 保留 Picture（`scene.RecordPicture`，**8 ops**）每帧 `Replay` | 与左区同点同色（三角/矩形/图/两条描边） |
| 3 | 区标签 | `DIRECT (直绘)` / `REPLAY (回放)` | 常驻，Align 跟随 |
| 4 | 静态 4×3 色格 | (964,80) 起，40×40×12 | 永不闪（非 picture） |
| 5 | 相位行为 | Steady 2s / Spike 2s / Recover | 相位只驱动重绘，两侧内容不变；HUD 相位行变化 |
| 6 | HUD | fps/p95/picture_ops/**bounds**/presents + 门禁 | 每 ~0.1s 刷新：picture_ops=8 常驻、bounds=(18,20 164x210) |

### 判断标准

- 左右两区同点 RGB 完全一致 = 回放等价（记录/回放无丢失、无错序）。
- 静态色格不闪 + Recover 相位无闪 = Picture 保留在层树内只回放不重录。
- HUD picture_ops=8 = op 计数诚实（2 path + 2 rect + 2 image + 2 text）。
- HUD bounds=(18,20 164x210) = 重写后 Bounds 记入 path/image 几何的证明（stroke inflate 1.5 → minX=18，maxY=FillRect 230）。

## 三证据（U21）

### 1) 逻辑 6/6

`recorded_ops=8`、`picture_op_count≥3`、`direct_replay_eq`、`presents>0`、`paint_count>0`、
静态色格像素。实测两连：**presents=299/300、paint_count=298、ops=8**。

### 2) 像素 15/15（GPU 抓屏）

| Probe | 期望 | 实测 |
|-------|------|------|
| `direct_tri` / `replay_tri` | (0.90,0.30,0.20) | 同值 |
| `direct_rect` / `replay_rect` | (0.30,0.80,0.40) | 同值 |
| `direct_img` / `replay_img` | (0.20,0.749,0.349) | 同值 |
| `direct_stroke` / `replay_stroke` | (0.20,0.60,0.90) | (0.200,0.600,0.898) |
| `direct_srect` / `replay_srect` | (0.90,0.80,0.30) | (0.898,0.800,0.298) |
| `static_grid` | (0.25,0.50,0.70) | (0.251,0.502,0.698) |
| `text_ink` | 非 Body BG | 文字像素 (0.898,0.898,0.949) |
| `body_bg` / `legend_bg` / `top_bg` | 三块面板底色 | 同值 |

### 3) direct/replay 配对 5/5 + Golden

- 同点配对：三角/矩形/图/两条描边，容差 0.02，实测 RGB 完全一致；
  这替代了旧 README 的「人眼对照」与旧 JSON 的硬编码 `true`。
- Golden：header 48 + legend 260×656 + body 904×656 = **821184 px** 逐位零容差，
  播种后两连 **0.0000%**。内容静态，所以 gate/manual 都跑同一套像素/Golden。

## Gates（`-auto-only` 必须全绿）

- `picture_op_count ≥ 3`（实测 8：2 path + 2 rect + 2 image + 2 text；face 缺失时 6）
- direct/replay 同点配对 5/5（容差 0.02）+ 像素 15/15 + Golden 0.0000%
- `present_policy = full_paint`（W6 正确性窗显式 pin）
- `vsync_source=true`、`gpu_fallbacks=0`、`cpu_fallback_ops=0`
- 观测：fps_interval **59.78**、p95 **18.92ms**、hitch **0**、gpu_ops **19139**
- `fps_gate=off_correctness`、`slope_gate=off`：5s 正确性窗，不冒充长跑性能窗；
  `rss_slope_kb_per_min=1320727` 为进程启动一次性分配在短窗的放大斜率，非泄漏指示。

## 实现点六维（U20）

| 维度 | 回答 |
|------|------|
| 正确性 | 录制 8 op → Replay 逐 op 转发；直绘 vs 回放同点 RGB 配对，不再靠肉眼/硬编码 |
| 脏区 | Picture 每帧只 Replay 不重录（层内 retained）；Recover 相位无 MarkNeedsPaint → 纯层树回放 |
| 缓存 | Picture 显示列表记录一次（`RecordPicture` 冻结 ops + detached 副本），op 不可变、paint 录制期规范化 |
| 边界条件 | face 缺失 → 文字 op 跳过（op=6 仍过门禁）；image 引用 ImageBuf 生命周期贯穿窗（defer Dispose）；空 Bounds → HUD 显示 no-bounds |
| 失败模式 | op_count<3 / 配对不等 / 像素不符 / Golden 漂移 → 红；窗开不了 → FAIL: window open (needs_gpu_window) |
| 窗内如何看出 | HUD picture_ops=8 + bounds 矩形 + 左右区同构 + 相位切换 |
