# ui_wr_r9_text_cache — R9 文本 measure 缓存（三证据 ✅v2）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 首次播种 Golden（只建基线，seed 不算 PASS；seed 快照不覆盖任何已提交证据）
GPUI_ACCEPT_GOLDEN=1 go run ./examples/ui_wr_r9_text_cache -auto-only

# 正式门禁：必须两连 0.0000% / 821184px，pixel 29/29，logic 28/28
go run ./examples/ui_wr_r9_text_cache -auto-only
go run ./examples/ui_wr_r9_text_cache -auto-only

# manual：默认无限运行，手动关窗；-manual-seconds 15 自动跑完三相位
go run ./examples/ui_wr_r9_text_cache
go run ./examples/ui_wr_r9_text_cache -manual-seconds 15
```

`-auto-only` 默认 gate 5s；`RUN_SECONDS` 可加长，但小于 5s 直接 FAIL（§2.5）。
manual 档 `SnapshotPath` 为空，**绝不覆盖** gate 提交的 `showcase_r9_text_cache.png` 和 Golden 基线。

## Window / Close duration

- 窗口 **1200×800** 逻辑像素，X11/GUI 真窗，GPU 必需。
- R9 关闭时长 **5s**；gate 冻结在 **Steady**，从 warmup 后重置 measure 计数再判。
- manual 跑完整 **Steady（0–2s）→ Spike（2–5s）→ Recover（5s+）**；15s 档覆盖完整三相位。
- 能力：`RenderText.measureLine` 的宽度缓存。每 tick 对全树文字强制 `MarkNeedsLayout`，文字/style 不变时必须命中；只有 manual Spike 改动的两条热点文字允许 miss。

## 场景与布局

完整 wrkit Shell：TopBar 48 + Legend 260×656 + Body 904×656 + LiveHUD 72。Body 内同时摆多字号/多颜色/多字重、两种回绕宽度、密集小字和热点文字，避免「两色块」式简陋场景：

```text
+----------------------------------------------------------------------+
| TopBar 48: R9 text measure cache — forced relayout, hits>=miss       |
+----------+-----------------------------------------------------------+
| LEGEND   | STYLE MATRIX 520x210      | DENSE LABELS 324x210           |
| 260x656  | 8 styles / 2 cols          | 12 small labels / 2 cols       |
|          |-----------------------------------------------------------|
|          | PARAGRAPHS 520x220         | MEASURE CONTRACT 324x220       |
|          | 2 wrap widths              | 6 contract lines               |
|          |-----------------------------------------------------------|
|          | CACHE KEY / CHIPS 520x150  | HOT TEXT 324x150               |
|          | 4 exact colour chips       | hotA/hotB + manual-only note   |
+----------+-----------------------------------------------------------+
| LiveHUD 72: R9 | phase | fps | p95 | hit | miss | layout | presents |
+----------------------------------------------------------------------+
```

| # | 区域（Body 局部坐标） | 内容 | 预期效果 |
|---|----------------------|------|----------|
| 1 | STYLE MATRIX (20,20) 520×210 | 12/13/14/15/16/18/20/22px，4 种字重，多种颜色 | 每 tick 强制重布局；同一文字/字号/字重宽度稳定，measure 命中 |
| 2 | WRAPPED PARAGRAPHS (20,245) 520×220 | 240px 与 232px 两种回绕宽度、多行长文本 | 多次 `measureLine` 走同一批 key；换行不漂移 |
| 3 | DENSE LABELS (560,20) 324×210 | 12 个小字号标签，9–11px、不同颜色 | 静态密集文字，覆盖小字号 AA 边界 |
| 4 | MEASURE CONTRACT (560,245) 324×220 | 6 行实现契约说明 | 操作者可读；不是纯指标窗 |
| 5 | CACHE KEY / CHIPS (20,480) 520×150 | 4 个精确颜色条 + 两行说明 | 像素探针的稳定色块锚点 |
| 6 | HOT TEXT (560,480) 324×150 | hotA/hotB；gate 固定，manual Steady 仅颜色脉冲，Spike 每 tick 改内容，Recover 装一次最终文字 | 只有热点文字产生 cold miss；静态文字 miss 必须为 0 |

## Gate 断言

gate 在 `tick >= 20` 时对全树 `RenderText` 调 `ResetMeasureCacheStats()`；此前首轮布局已把每个宽度 key 暖起来。之后每 tick 强制重布局，因此命中是**复用**，不是首轮 miss 被冲掉。

| 断言 | 门禁 |
|------|------|
| 文字节点 / 内容文字 | 全树 51 / 内容 24 |
| 样式覆盖 | 11 种字号、4 种字重、22 种颜色、4 个带 `MaxWidth` 的节点 |
| measure 命中 | `measure_cache_hit >= 1000`（实测 19884 / 19884） |
| measure 未命中 | `measure_cache_miss == 0`（gate 冻结 Steady + 文字不变） |
| 静态文字 miss | `static_text_miss == 0` |
| 热点文字 miss | `hot_text_miss == 0`（gate 不改内容） |
| 真字重 | 14px semibold 与 15px bold 的解析 face 均 `Embolden=false`，`checked=2`；必须走真字重切或 wght 变量轮廓，禁止回到栅格后 mask 膨胀 |
| 尺寸稳定 | 每个文字节点 `Size()` 与 reset 时逐节点完全相等，`size_bad=[]` |
| 帧时 | `fps_interval >= 55`（实测 60.11 / 60.09），`interval_p95_ms > 0 且 <= 22`（实测 17.40 / 17.53） |
| Present | `present_policy=full_paint`；R9 是正确性窗，不拿 FullPaint 冒充 Retained |
| CPU/GPU | `cpu_pct_avg≈36.9%`，`cpu_ui_pct≈2.7%`，`cpu_raster_pct≈34.1%`；`cpu_fallback_ops=0`，`gpu_ops=23103`（累计口径） |
| 像素 | **29/29**：9 个面板/区域底色 + 4 个精确色条 + 16 个文字墨迹区域；其中 14 个墨迹区域同时断言主色像素 |
| Golden | **821184 px**（header 1200×48 + legend 260×656 + body 904×656），**零容差**逐位比较 |
| 回归 | 非 seed 两连均 `0.0000% / 821184 px`；`pixel_golden_first_run=false` |

### 像素/Golden 口径

- 快照是 `SnapshotPath` 的 GPU readback，不是窗口抓屏。
- 文字按 §2.7 **F6**：区域级非背景墨迹像素数 ≥ 阈值；14 个区域另查接近声明文字色的主色像素数。小字号（9–11px）纯色内部像素很少，`dense_label_ink` 只做墨迹计数，主色份额由同面板 13px 标题承担。
- Golden 覆盖静态 shell + 全部 body；HUD 因计数器刷新被排除。manual 不做像素/Golden 断言，也不写快照。
- `frame_raster_ms≈135–142ms` 包含结束时的 SnapshotPath GPU readback/PNG；`live_frame_raster_ms` 最后一帧活帧实测 2.5–2.7ms。README 不把 readback 成本当稳态性能。

## 14/15px 模糊回归（本次修复）

旧实现在静态字体请求 600/700 时，对已经栅格化的字形 mask 做 1px 3×3 膨胀；14/15px 的字形主体只有一两个像素宽，膨胀后边缘发糊。这条路径已删除，按 FreeType/Skia 的标准做法分两层解决：

1. **真字重优先**：`render/text/font_family` 按 CSS 权重匹配，把 Regular/SemiBold/Bold 文件组成一个字族；请求 600/700 时优先选真实字重切，真切的轮廓本身清晰，且不会被二次加粗。
2. **轮廓兜底**：没有粗体兄弟姐妹的独立静态字体，仍走 `EmboldenOutline` 在矢量轮廓上按 `ppem/24` 每侧扩张，再进解析 AA 光栅器；不再对 bitmap mask 做 max-filter。合成本身不改变 advance，只有真实字重切会按字体自身前进变宽。

本机 R9 的字体链是 CJK 真 Regular+Bold 家族 + Latin DejaVu Regular+Bold；DejaVu 没有 Semibold，因此 14px semibold 按 CSS 规则落到真实 DejaVu Bold（700），不是合成半粗；15px bold 也是真实 Bold。两者在 gate/manual 的 `real_weight_14_15_faces` 中都记录 `embolden=false`。引擎回归覆盖真字重选择、缺粗体时轮廓兜底、CFF 轮廓扩张、AA/aliased/LCD 三路加粗且 advance 不变（`render/text/family_test.go`、`embolden_test.go`）。

## Manual 断言

manual 15s，三相位都由 `wrkit.PhaseClock` 驱动：

| 相位 | 行为 | 实测 |
|------|------|------|
| Steady 0–2s | 每 tick 强制重布局；hotA 只改颜色（paint dirty，不失效 measure），hotB 不动 | `hits_steady=7029`，`miss_steady=0` |
| Spike 2–5s | hotA/hotB 每 tick `SetText`，制造真实 cold miss；静态文字仍只命中 | `hits_spike=13136`，`miss_spike=360`，`static_text_miss=0` |
| Recover 5s+ | 两热点装一次最终文字，之后不再改；miss 停止 | `hits_recover=42598`，`miss_recover=2`，`recover_settled=true`，`recover_miss_after_settle=0` |

manual 总：`measure_cache_hit=62763`、`measure_cache_miss=362`、`hot_text_miss=362`、`static_text_miss=0`、`text_sizes_stable=true`、`real_weight_14_15_ok=true`、logic 28/28；`fps_interval=59.98`、`p95=17.20ms`。gate 两张 PNG 的 sha256 在 manual 前后一致。

## 指标诚实点

- `measure_cache_hit/miss` 来自每个 `RenderText` 的累计计数器；`FrameBuildStats` 每帧读同一组数。`Run()` 结束时最后一次强制布局可能晚于最后一次 present，所以 pre-sync 快照 gate 少 8 个 hit、manual 少 4 个。报告在 `app.Close()` 后从 `TreeMeasureCacheStats` 同源同步，`measure_metrics_match_tree=true`；`measure_metrics_pre_sync_lag_hit` 如实保留差值，不是把指标改绿。
- `rss_slope_kb_per_min` 是 5s/15s 启动爬坡原始值，`rss_slope_valid=false`，不能当泄漏结论；稳态斜率字段按既有单测 `TestBuildReport_ShortRunSlopeInvalidIsHonest` 在 invalid 时省略。`rss_after_close` 短窗等于 peak 也只记观察，不做 soak 判断。
- `gpu_ops` 是 Present Context 累计路由计数，不是每帧 submit 次数。
- `fps` 门禁用 `fps_interval`，不用含开关窗开销的 `fps_wall`。
- 本窗没有为了过门禁删场景、降画质或放宽阈值；`present_policy=full_paint` 是能力选择，不冒充 retained。

## U20 六维

| 维 | 本窗落点 |
|----|----------|
| **正确性** | 同文字/字号/字重宽度稳定；`measure_cache_hit` 增长且 gate miss=0；每个文字 `Size()` reset 前后逐节点相等 |
| **脏路径** | 每 tick 全树 `MarkNeedsLayout`；静态文字命中、热点文字才 miss；Steady 颜色脉冲只脏 paint，不失效 measure |
| **缓存** | `measureLine` key 覆盖文字+字号+字间距+字重；`SetText/SetFace/SetFontSize/SetFontWeight` 失效；manual 362 次 miss 全部来自 hotA/hotB |
| **边界** | 11 种字号、4 种字重、22 种颜色、两种回绕宽度、9–11px 小字、完整 shell；gate 冻结 vs manual 动态两档分离 |
| **失败条件** | hit<1000、gate miss!=0、静态 miss>0、尺寸漂移、Spike miss==0、Recover miss 继续增长、像素/ Golden 失败、policy 非 full_paint、fps<55、p95>22 任一即 FAIL |
| **可见性** | HUD 实时 hit/miss/layout/phase；manual Spike 可见两条热点文字改写，Recover 冻结；静态矩阵/段落/密集标签/色条全程可见 |

## 证据留档（2026-10-22，X11）

| 跑次 | presents/layouts | hit/miss | pixel | Golden | logic | fps_interval / p95 |
|------|------------------|----------|-------|--------|-------|--------------------|
| seed `GPUI_ACCEPT_GOLDEN=1 -auto-only` | 300/300 | 19884/0 | 29/29 | seed（不算 PASS） | 28/28 | 60.03 / 17.46ms |
| gate #1 `-auto-only` | 300/300 | 19884/0 | 29/29 | **0.0000% / 821184px** | 28/28 | 60.11 / 17.40ms |
| gate #2 `-auto-only` | 300/300 | 19884/0 | 29/29 | **0.0000% / 821184px** | 28/28 | 60.09 / 17.53ms |
| manual `-manual-seconds 15` | 899/899 | 62763/362 | 不适用（manual 不写快照） | 不适用 | 28/28 | 59.98 / 17.20ms |

Gate 证据文件：

- `testdata/showcase_r9_text_cache.png`（gate 快照）
- `testdata/showcase_r9_text_cache_base.png`（Golden 基线，零容差）
