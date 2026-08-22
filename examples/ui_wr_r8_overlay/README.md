# ui_wr_r8_overlay — R8 Overlay 独立合成 真窗

W4 R8 单能力真窗：浮层（弹窗/菜单/Tooltip 类）开/关时**只脏浮层带**，主树不被惊动。
按方案 C（Flutter 对齐）：retained 姿态 + 脏集分带观测。

**实际应用场景测试**（v2）：Spike 相位不再单开一次浮层，而是**持续循环开关**——
每 2s 开 / 1.5s 关，三种浮层轮换（模态对话框+菜单叠层 / 下拉菜单 / Tooltip 提示条），
每次打开都构建**全新 widget**（真实应用模式：新弹窗=新 RenderObject=新纹理），
尺寸与位置逐次变化。默认 15s 关闭跑含 ~2 次循环；长跑验证用
`RUN_SECONDS=150`（~37 循环）或 `RUN_SECONDS=600`（10 分钟压测）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_r8_overlay    # 关闭用时长
RUN_SECONDS=600 go run ./examples/ui_wr_r8_overlay   # 10 分钟泄漏/衰减压测
```

## Window / Close duration

- 1200×800 逻辑像素（U15）
- 关闭用 **15s**；`RUN_SECONDS` 可覆盖但 `<5 → FAIL`（U16）

## Visible effect

| Region | Expectation |
|--------|-------------|
| MAIN 密集区（Body 左侧） | 4×4 色格（各自独立 boundary）+ 8 行静态文字，全程静止 |
| HOT 热点（Body 右下） | 每帧变色，**全程活跃**（含浮层期，用户选定） |
| SpikeCycle（5s 起，长跑占 ~90% 时长） | 循环开关：对话框+菜单叠层 → 下拉菜单 → Tooltip 提示条轮换；屏障随开合明暗切换；每次打开位置/尺寸微变 |
| Recover（收尾 10%） | 浮层消失、屏幕恢复亮度，主树内容原样 |
| LiveHUD（底部） | `ov=N cyc=K OPEN/closed main_dirty=X/base` 实时滚动；绿=门禁趋势 PASS / 红=已破线 |

## Gates

§2 主表：**开浮层后主树 `paint_count` 不涨**。逐条 FAIL 线：

| Gate | 阈值 | 来源 |
|------|------|------|
| `main_dirty_excess_frames` | ==0（浮层开着的主带脏 id 计数不得超基线；基线=稳态窗口 MAX 含 HOT+HUD 合法自脏） | §2 主表（方案 C 分带口径） |
| `overlay_entries_on_open` | ≥1（浮层自己确实被标脏并绘制） | 能力语义反面 |
| `overlay_open_observed` | true（脚本必须真的开过浮层） | 场景自检 |
| exit 时浮层已关 | Recover 必须移除全部 entry | 场景自检 |
| `overlay_cycles` | 观测字段：完成的开关循环数（15s≈2，600s≈150） | 场景化压力证据 |
| `present_policy` | retained（方案 C 姿态） | §2.1 |
| `fps_wall` | ≥55（持续 ticker 动画类；**长跑循环下不衰减**——10 分钟压测 fps 保持 59+） | §2.2.2 族 A |
| `interval_p95_ms` | ≤22 | §2.2.2 族 A |
| `present_count` | ≥1 | 通用 |
| `vsync_source` | 必须存在（fallback 如实报告） | §2.2.2 族 A |
| RSS 长跑 | 平衡后斜率受控（Go 堆 + GPU 驱动一次性爬坡后应平台化；10 分钟实测 slope≈3MB/min 且 peak≈end） | §2.2.4 族 E |

## 实现点六维（U20）

见 JSON `ability_extra`：`impl_correctness` / `impl_dirty` / `impl_cache` / `impl_edge` / `impl_fail` / `impl_visible`。

## 引擎接线（本轮 wr-implement 产物）

- `FramePacket.OverlayDirtyLayerIDs`：浮层带脏 id 镜像列（`ui/scene/packet.go`）
- `AttachToPacket` 分列记录，合并集行为不变（`ui/overlay/state.go`）
- `LastOverlayBandFrame()`：每帧主带/浮层脏计数观测口（`ui/embedder/pipeline_app.go`，挂接前采主带计数避免合并污染）
