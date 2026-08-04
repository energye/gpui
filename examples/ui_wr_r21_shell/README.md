# ui_wr_r21_shell — R21 壳/内容分层

独立真窗（U5）：证明「壳与内容分层」—— 静态顶栏壳层永不因体滚动而重录。

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
- 全族 §2.2.4（fps_interval 持续 tick ≥55、vsync_source=true、cpu 分轨、rss_* 字段齐）
- 附加：`shell_rerecord_total`=1（仅首帧/暖机冷录）语义注明于 Extra。

## U17/U18/U20 自检

- U17：多区域（顶栏壳+体滚+静态 dense+HOT+HUD）· 静态密集（4×4 色格+8 标签+按钮）· 动态热点（HOT+体滚）· 能力专属（壳/体分层）· 三相位（Steady/Spike/Recover）
- U18：LiveHUD 实时 fps/p95/policy + shell_rerecord/shell_skip 计数 + gate 绿点
- U20：正确性——SetShellBoundary 只随体滚动脏；脏区——体滚只脏体层；缓存——壳 Replay skip 累积；边界——首帧冷录后焊底、滚动反弹 clamp；失败模——壳被体拖脏→`shell_rerecord_scroll` 上跳 FAIL；窗内——HUD shell_rr/shell_skip 数字。