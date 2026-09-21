# kit_f0_theme — F0 主题真窗（默认/换肤/紧凑）

## 运行

```bash
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
THEME=default go run ./examples/kit_f0_theme -auto-only
THEME=skinned go run ./examples/kit_f0_theme -auto-only
THEME=compact go run ./examples/kit_f0_theme -auto-only
# 常驻人工：
THEME=default RUN_SECONDS=10 go run ./examples/kit_f0_theme
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=10。
正确性窗：`slope_gate=off`（10 秒短窗不判爬升），fps 门禁开。

`THEME` 只认 `default|skinned|compact`，不认回落 `default`。

## 窗内呈现

同一按钮横排三个，一次 `theme.Default.SetBase` 整树跟随：

| 段 | 内容 | 证明 |
|------|----------|----------|
| 三按钮 | 同色同高，全走 `theme.Default.Current()` | 4s 切主题后三块色=主色 |
| 换肤 | skinned 主色紫 `#722ed2` | 探针紫、金图独立一张 |
| 紧凑 | compact 字 12、控件高 24 | 高=24、金图独立一张 |
| 整树禁用 | 6s 全禁用点不动 | 三块点击全 0 |
| 尺寸切换 | 8s 切小、9s 恢复 | 高先小后回 |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 门禁

| 门禁 | 阈值 |
|------|------|
| policy | `present_policy == full_paint` |
| presents | `present_count >= 1` |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` |
| 脚本 10 项 | 整树跟随/尺寸跟主题/全禁用盖住/重启用计数/切小/恢复/三探针/墨量 10/10 |
| 像素三探针 | 三按钮中心=主色，容差 12/通道 |
| 墨量 | 信息区 ≥60（有字体才判，无字体直接 FAIL） |
| Golden | `testdata/showcase_theme_{default,skinned,compact}_base.png` 各零容差 |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` |
| vsync | `vsync_source` 非空 |

## 三证据

* 逻辑：窗内 10 项进 JSON（整树跟随+禁用盖住+尺寸切换）
* 像素：三按钮中心采样 + 信息区墨量 + Golden 掩码逐位对比
* Golden：三套主题各一张，逐位对比，差异超限 FAIL
