# kit_f0_interact — F0-4 交互真窗

## 运行

```bash
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/kit_f0_interact
# 门禁自检：
go run ./examples/kit_f0_interact -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=15（焦点与浮层相位占 7s/8s/9s/11s）。
正确性窗：`slope_gate=off`（15 秒短窗不判爬升）。

## 窗内呈现

| 段 | 内容 | 证明 |
|------|----------|----------|
| click | 可点块，悬停/按压变色 | 点一次计一次，探针 hover 色 |
| disabled | 禁用块 | 点多少次都是 0 |
| loading | 加载块 | 连点两次也是 0（防重） |
| fields | 受控/非受控/拼写串 | 受控只回调不存值，拼写期不算值、提交才调一次 |
| overlay | 锚点 + 浮层（bottom） | 四向定位精确 + 顶部翻到底部 + 外点关 + Esc 关 + 焦点锁 |
| focus | Tab 走查 | 鼠标点不亮环，键盘聚焦才亮，禁用永不亮 |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 手工操作

- 鼠标移到可点块上会变 hover 色，按下变按压色，松开计一次点击（标题 HUD 的 clicks 加一）。
- 按 Tab 在可聚焦块之间走，焦点环只在键盘走时亮；鼠标点不亮环。
- 浮层开着时点外面关，按 Esc 关。所有真事件打到 stderr 日志。
- 本窗无改标题 API（平台层无 SetTitle），状态看 HUD 与日志。

## 门禁

| 门禁 | 阈值 |
|------|------|
| policy | `present_policy == full_paint` |
| presents | `present_count >= 1` |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` |
| 脚本 15 项 | 点击/禁用/防重/字段/四向+翻转/焦点锁/外点关/Esc/焦点环/命中/Tab/脏标记/三像素/墨量 15/15 |
| 像素三探针 | 可点 hover 色/禁用灰/浮层盒，容差 12/通道 |
| 墨量 | 字段区 ≥60（有字体才判，无字体直接 FAIL） |
| Golden | `testdata/showcase_interact_base.png` 零容差 |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` |
| vsync | `vsync_source` 非空 |

## 三证据

* 逻辑：单测（`interactive_test.go` 五态+焦点可见、`field_test.go` 受控/拼写、`overlay_trigger_test.go` 定位/外点关/Esc）+ 窗内 15 项进 JSON
* 像素：三色块中心采样 + 字段区墨量 + Golden 掩码逐位对比
* Golden：静态掩码逐位对比，差异超限 FAIL
