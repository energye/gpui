# kit_f0_motion — F0-5 动效真窗

## 运行

```bash
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/kit_f0_motion
# 门禁自检：
go run ./examples/kit_f0_motion -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=30（转圈/波纹/审计相位占 4s–15s，28s 停转定格）。
稳态窗：`slope_gate=off`（30 秒短窗不判爬升），fps 门禁开（`fps_interval>=55` 且 `interval_p95_ms<=22`）。
动画窗用省画模式（`retained`）：只有转点、波纹块和降频 HUD 重画，静态底跳过；
全量重画每帧吃掉 13ms 光栅，只剩 3ms 余量，必掉 vsync。

## 窗内呈现

| 段 | 内容 | 证明 |
|------|----------|----------|
| spinner | 转圈圆点走节拍真转（0.9s 一圈） | 4s 启动 6s 角度差>0 |
| frozen | 省动效/总开关冻结段 | motion-off 与 reduced-motion 都不转 |
| wave | 点后散开 0→6、0.4 秒散、2 秒褪净、初始两成深 | 8s 启动 9s 半程 11s 结束 |
| semantics | 有名有角色审计 3/4 | 无名按钮判红，对比度/44 点只告警 |
| direction | RTL 镜像+箭头互换+空态文案 | mirror 1100/箭头互换 |
| gate | 中央四查自检 4/4 | Props/State/接线/Token 全过 |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 手工操作

- 鼠标点波纹块会起一次波纹（标题 HUD 的 wave 半径会动）。
- 转圈圆点一直在转，28s 停转定格等截图；省动效段永远不动。
- 所有真事件打到 stderr 日志。

## 门禁

| 门禁 | 阈值 |
|------|------|
| policy | `present_policy == retained` 且 `present_mode == damage_union` 且 `damage_ratio <= 0.5` |
| presents | `present_count >= 1` |
| 稳态 FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` |
| 脚本 17 项 | 时长/转圈/冻结/波纹三段/语义/告警/方向/四门禁/隔离/虚拟/三像素/墨量 17/17 |
| 像素三探针 | 转圈底/波纹块/冻结底，容差 12/通道 |
| 墨量 | 信息区 ≥60（有字体才判，无字体直接 FAIL） |
| Golden | `testdata/showcase_motion_base.png` 零容差 |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` |
| vsync | `vsync_source` 非空 |

## 三证据

* 逻辑：单测（`motion_test.go` 转圈真转/省动效冻结/波纹、`semantics_test.go` 有名有角色、`performance_test.go` 预算隔离、`direction_test.go` 镜像、`gate_test.go` 四查）+ 窗内 17 项进 JSON
* 像素：三色块中心采样 + 信息区墨量 + Golden 掩码逐位对比
* Golden：静态掩码逐位对比，差异超限 FAIL
