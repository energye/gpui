# ui_wr_r21_shell — R21 壳/内容分层

独立真窗（U5）：证明「壳与内容分层」—— 静态顶栏壳层永不因体滚动而重录。

## 实现点六维（U20）

| 维度 | 内容 |
|------|------|
| 正确性 | 顶栏是一个 `SetShellBoundary` 的 RepaintBoundary；体滚动只脏体层，壳层 Picture 回放不受影响（Flutter 壳/内容分离语义） |
| 脏区 | 滚动只更新 viewport offset → 仅体 boundary 重录；`LastShellBoundaryFrame` 逐帧采样壳分区 |
| 缓存 | 壳缓存条目在滚动中存活（skip 持续增长而 rr=0）；resize 合法失效并从门禁窗口排除 |
| 边界 | resize 帧 3 帧沉降排除，外部 WM 改窗不误判；无限滚动到内容尾回卷；HUD/相位芯片在壳 boundary **外**（每帧自脏会破坏分层） |
| 失败模式 | 滚动帧出现任何壳重录 → FAIL；壳完全没有回放 → FAIL（缓存未生效） |
| 窗内如何看出 | HUD shell_rr/shell_skip 实时数字；行滚动时顶栏像素纹丝不动 |

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_r21_shell
```

## Window / Close duration

- 1200×800 逻辑像素（§2 主表 R21）
- 关闭用时长 **15s**（§2 主表 / §2.5）
- `RUN_SECONDS<5`（U16）→ FAIL 退出
- 需 GPU 真窗（needs_gpu_window）；无环境 → exit 1，禁止 CPU stub

## Visible effect

| Region | 预期 |
|--------|------|
| 顶栏壳层（蓝灰条 @ 顶部）| 标题"标题/按钮/相位" + 按钮-1/按钮-2（蓝/橙）+ PHASE 芯片——**全程序静止不动** |
| 体滚动区（虚拟列表 60 项）| 行 chip+文字 **持续滚动**（Steady 慢 → Spike 快 → Recover 慢）|
| 静态密集区（右侧 4×4 色格+8 标签）| **静止**，不随体滚动变化 |
| HOT 热点（右下）| 每帧变色（live paint 动画，非壳层）|
| LiveHUD（底部）| fps / p95 / shell_rr / shell_skip / phase 实时 |

## Gates

按 §2 主表 + §2.6.2 R21：体内容滚动 → 顶栏 `rerecord=0`。

- `shell_rerecord_scroll == 0`（滚动全程壳层零重录）
- `shell_skip_scroll > 0`（壳层 Picture Replay 命中，缓存被复用）
- `present_count ≥ 1`；`policy=full_paint`（正确性窗，不冒充 retained）
- `fps_wall ≥ 55`（持续 tick 动画类）；`interval_p95_ms ≤ 22`
- `vsync_source` 必须存在（fallback 如实报告）
- cpu_ui/cpu_raster 非双 0；rss_* 字段齐
- 附加：`shell_rerecord_total`=1（仅首帧/暖机冷录）语义注明于 Extra
- RSS 斜率语义：15s 窗处于启动均衡爬坡内（Go 堆 + GPU 驱动一次性建立），
  数值偏大属预期；平台化证据见 §10「R8 场景化长跑」600s 实测

## U17/U18/U20 自检

- U17：多区域（顶栏壳+体滚+静态 dense+HOT+HUD）· 静态密集（4×4 色格+8 标签+按钮）· 动态热点（HOT+体滚）· 能力专属（壳/体分层）· 三相位（Steady/Spike/Recover）
- U18：LiveHUD 实时 fps/p95/policy + shell_rerecord/shell_skip 计数 + gate 绿点
- U20：见上方六维表