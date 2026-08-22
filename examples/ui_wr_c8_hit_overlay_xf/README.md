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
