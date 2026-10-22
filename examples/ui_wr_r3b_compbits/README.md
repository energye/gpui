# ui_wr_r3b_compbits — R3b 合成位 / 边界发现（三证据 ✅v2）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 首次播种 Golden（输出 SEEDED，不算 PASS）
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits -auto-only

# 关闭证据：必须再连跑两次，均 0.0000%
RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits -auto-only
RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits -auto-only

# manual：默认无限运行；-manual-seconds 15 跑完 Steady→Spike→Recover 自动关
go run ./examples/ui_wr_r3b_compbits
go run ./examples/ui_wr_r3b_compbits -manual-seconds 15
```

## Window / Close duration

- 窗口 **1200×800** 逻辑像素，X11/GUI 真窗，GPU 必需。
- R3b 关闭时长 **8s**（§2.5）；gate 档 `RUN_SECONDS < 8` 直接 FAIL。
  `RUN_SECONDS>=5` 只是通用最低线，不能拿 5s 关 R3b。
- manual 档默认无限运行，手动关窗退出；`-manual-seconds 15` 用于三相位人工观察。
- gate 档冻结在 **Steady**；manual 档跑完整 Steady（0–2s）→ Spike（2–4s）→ Recover（4s+）。

## 布局

```text
+----------------------------------------------------------------------+
| TopBar 48px: R3b 合成位 — 4 层嵌套 + NeedsCompositing 可视化          |
+----------+-----------------------------------------------------------+
| Legend   | Body (284,60) 904×656                                     |
| (12,60)  |  主链 a (308,84) 260² → b 200² → c 140² → d 60²           |
| 260×656  |    c 内 hot 红 50² (378,154)；d 内 dHot 橙 20² (448,224)  |
| 10 行    |  兄弟 s (604,84) 160² + hotS 蓝 50² (659,139)              |
|          |  兄弟 s2 (804,84) 140² + hotS2 橙 44² (852,132)            |
|          |  非 boundary 祖先 wrap (984,84) 160² → boundary pw        |
|          |    pw (1014,114) 100²，wrap 合成位=true                    |
|          |  无 boundary 负控 neg (984,260) 160×120，合成位=false      |
|          |  状态面板 (308,380) 856×150：12 个合成位色块 + 名字        |
|          |  4×4 静态色格 (308,546) 30²/格，间距 38 + 8 个 grid 标签   |
+----------+-----------------------------------------------------------+
| HUD (0,728) 72px: phase/fps/p95/skip/rr/presents/comp/门禁            |
+----------------------------------------------------------------------+
```

- **7 个内容 RepaintBoundary**：a/b/c/d、s、s2、pw；HUD 自身另算 1 个，`boundary_count=8`。
- **嵌套深度 4**：a→b→c→d；`boundary_max_depth=4`（结构实测，不是硬编码）。
- **兄弟旁路 2 套**：s、s2 不在主链上，各自独立 skip/rerecord。
- **传播正控**：`wrap` 自身不是 boundary，但子树里有 `pw` → 合成位必须为 true。
- **负控**：`neg`（160×120，无任何 boundary 后代）→ 合成位必须为 false，即祖先 Body 为 true 也不能向下污染。
- **17 个静态标签 + 16 个静态色格**：边界脏时静内容不闪的像素证据。
- **12 个合成位色块**：绿色=NeedsCompositing true，品红=false；色块 + `name=T/F` 文字都在 Body 状态面板内，并被像素探针逐块覆盖。
- Legend 10 行，每行有色块 + 文字（`ColorAt` + `LabelAt`）。

## 可见效果

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | 主链 a→b→c→d | (308,84) a 260²；(338,114) b 200²；(368,144) c 140²；(438,214) d 60² | 4 层 RepaintBoundary，每层背景 + 静态块/标签 | Steady 只有 c 内 hot 变；Spike 只 a 背景变；Recover 只 d 内 dHot 变；其余层像素不动 |
| 2 | c 内 hot 红块 | (378,154) 50² | 每 tick 改色（gate 前 60 tick 后冻结） | Steady 相位肉眼可见红块变化；只驱动 c 层重录 |
| 3 | d 内 dHot + “d” 标签 | (448,224) 20²；(448,246) 标签 | 深链第 4 层叶子 | Recover 相位变化；证明深度 4 的边界仍能独立重录 |
| 4 | 兄弟 s | (604,84) 160²；hotS (659,139) 50² | 独立旁路 nest + 静态块/标签 | Steady 与 hot 同步变；a/b/c/d 与 s2 不闪 |
| 5 | 兄弟 s2 | (804,84) 140²；hotS2 (852,132) 44² | 第二旁路 nest + 静态块/标签 | 仅 Spike 变；其他 nest 与主链不受污染 |
| 6 | wrap → pw | (984,84) wrap 160²；(1014,114) pw 100² | 非 boundary 祖先包 boundary 子节点 | wrap 自身不重录，但合成位必须为 true；pw 内容全程静 |
| 7 | neg 负控 | (984,260) 160×120 | 无 boundary 的纯色面板 + 棕色块 + “no boundary” 标签 | 合成位固定 false，且不因 Body=true 被误置 true |
| 8 | 合成位状态面板 | (308,380) 856×150 | 12 个 14×14 色块 + 名字；左列 root/body/a/b/c/d，右列 hot/s/s2/wrap/pw/neg | 绿=true、品红=false；tick 10 写入后不再变，进阶像素探针逐块比对 |
| 9 | 4×4 静态色格 | (308,546) 起，30²/格，间距 38 | 16 个非 boundary 色格，色值随 i/j 变化 | 任一相位都不闪；Golden 覆盖整块区域 |
| 10 | 相位脚本 | — | gate：Steady 冻结；manual：Steady 2s → Spike 2s → Recover 11s | gate 前 60 tick 做确定性颜色变化产生真实内容脏，随后保持最终色；manual 三相位各驱动不同层 |
| 11 | HUD | (0,728) 72px | phase / fps / p95 / policy / mode / paint / presents / skip / rr / comp=T/F / 门禁 | 每 ~0.1s 刷新；干净层 `skip` 持续涨，内容变脏时 `rr` 涨；Golden 排除 HUD |

### 判断标准

- 12 个被测节点合成位与场景结构一致：root/body/a/b/c/d/s/s2/wrap/pw=true，hot/neg=false。
- 负控 `neg` 必须为 false，传播正控 `wrap` 必须为 true——两者同时成立才证明是自底向上并集，不是“见 boundary 就全树置 true”。
- `boundary_count=8`（7 内容 + HUD）与 `boundary_max_depth=4` 必须同时等于场景结构实测值。
- 合成位色块与真实 bit 一致：每个色块的像素探针就是 `NeedsCompositingOf` 的结果；bit 错则像素必错。
- 静态层/色格/标签全程不闪；Spike 只影响 a 与 s2，Steady 只影响 c 与 s 的内容。
- gate 冻结后画面逐位稳定；manual 三相位都产生对应的真实重录。
- 像素/Golden 必须与逻辑指标同时绿；只有 JSON 绿不算关窗。

## Gates

### 逻辑与能力门禁

- `presents >= 1`、`paint_count > 0`、`boundary_skip >= 3`、`boundary_rerecord >= 2`。
- `boundary_count >= 3`，且必须等于场景实测（7 内容 boundary + HUD = 8）。
- `boundary_max_depth >= 2`（文档下限），本窗按结构实测钉 `>= 4`（更严），且必须等于结构深度。
- `NeedsCompositing`：`root=true`、`body=true`、12 个节点逐个与期望一致、true 计数 = 场景期望 10、色块已写入且状态稳定。
- `static_labels >= 8`（实际 17）。
- `present_policy = full_paint`（W1 FullPaint 主路径，显式钉住，不继承 W6 retained 默认）。
- `cpu_pct_avg > 0` 且 `cpu_ui_pct`/`cpu_raster_pct` 至少一路 > 0；`cpu_fallback_ops = 0`；`vsync_source` 非空。
- gate 档额外要求 `phase = Steady`、`rr_spike = 0`、`rr_recover = 0`；manual 档进入 Spike/Recover 后要求对应 `rr_* > 0`。
- `rr_steady + rr_spike + rr_recover == boundary_rerecord`（相位归属不丢账）。

### 帧时（§2.2.2 正确性 + 持续 tick）

- 运行 8s（等于 R3b 关闭用时长），有效 FPS `fps_interval >= 55`。
- `interval_p95_ms <= 22ms`；`hitch_count` 只观测，R3b 不设 hitch 硬门禁。
- `target_hz = 60`，`vsync_source=true`。

### 像素与 Golden（U21）

- **像素 54/54**：29 个颜色探针（主链/兄弟/wrap/neg/网格/面板底色）+ 12 个合成位色块探针 + 13 个文字墨迹区域。
  - 颜色容差 0.06；墨迹区域按“与背景色不同的像素数”判定，最小 4–20 px。
  - 探针坐标按 Body 原点锚定，gate 档不 resize，坐标固定。
- **Golden 逐位 0 容差**：mask = header `1200×48` + legend `260×656` + body `904×656` = **821184 px**。
  - HUD 不计入（计数器每 ~0.1s 刷新）。
  - 两次非 seed 连跑均 `0.0000%`。
- manual 档动画中不做像素/Golden 断言：JSON 输出 `pixel_evidence=manual_not_asserted`、`pixel_golden_diff_pct=null`、`pixel_probes_ok/total=0/0`，避免 0/0 冒充通过。

### CPU / 内存 / GPU / 依赖

- CPU：短窗（8s）只必采并记录，不设 >85% 硬门禁；`cpu_pct_avg`/`cpu_ui_pct`/`cpu_raster_pct` 均非零。
- 内存：8s < `rss_warmup_sec=15`，`rss_slope_valid=false`；**不能据此下泄漏结论**。
  - 原始 `rss_slope_kb_per_min` 照常输出（含启动期，数值大是正常现象）。
  - `rss_slope_steady_kb_per_min` 在 `rss_slope_valid=false` 时按 `wrgate` 既有测试不输出（无效稳态斜率不得声称数值），README 与 `rss_slope_steady_note` 说明原因。
- GPU：`gpu_ops` 累计观测；`cpu_fallback_ops=0`，无回退。
- 图/文缓存：R3b 不是 R9/R10，`g_metrics=skipped` 并写原因；`measure_cache_hit/miss` 只观测，不设门禁。
- 依赖：`ui` 无 cgo（仅 doc 注释提到 `import "C"`）；`ui` 非测试代码无直引 `gpu/`；`go vet ./examples/ui_wr_r3b_compbits` 通过。
- 不降画质：场景密度、分辨率、full_paint、阈值均未为过门禁而降低。

## U20 六维实现点

| 维度 | 本窗实现 |
|------|----------|
| `impl_correctness` | 主链 4 层 a→b→c→d、兄弟 s/s2、非 boundary 祖先 wrap→pw、无 boundary 负控 neg；合成位自底向上并集：root/body/a/b/c/d/s/s2/wrap/pw=true，hot/neg=false |
| `impl_dirty` | gate：前 60 tick hot/hotS 确定性变色后保持最终色；manual Steady hot/hotS 每 tick 变，Spike `a.Background` + hotS2，Recover dHot；HUD 是独立 RepaintBoundary 带 |
| `impl_cache` | 干净 boundary 走 Replay 累加 `boundary_skip`；内容变化的脏层累加 `boundary_rerecord`；gate 冻结后内容指纹命中 Replay（rr 持平、skip 继续涨），manual 持续内容脏使三相位 rr 都增长 |
| `impl_edge` | 深度 4 主链、2 套兄弟、非 boundary 祖先 + boundary 子节点、无 boundary 负控、12 个合成位色块、16 色格、17 标签、10 行图例；resize 重新摆 shell |
| `impl_fail` | skip=0 / rerecord=0 / count<3 或 ≠ 场景 / depth<4 或 ≠ 结构 / 任一合成位错 / 色块未写或不稳 / 像素不符 / Golden≠0 / fps<55 / p95>22 / policy≠full_paint → FAIL |
| `impl_visible` | 12 个绿/品红色块直接显示合成位；HUD 显示 skip/rr/phase/comp 计数；manual 三相位热点肉眼可见；Legend 色块；gate 像素/Golden 证据 |

## 实测证据

### gate 两连（2026-10-22，最终代码，非 seed）

| 指标 | 第 1 次 | 第 2 次 |
|------|---------|---------|
| `presents` / `paint_count` | 480 / 479 | 479 / 478 |
| `boundary_skip` | 3240 | 3235 |
| `boundary_rerecord` | 127 | 125 |
| `boundary_count` / `max_depth` | 8 / 4 | 8 / 4 |
| `fps_interval` | 60.08 | 59.95 |
| `interval_p95_ms` | 17.50 | 17.41 |
| `hitch_count` | 0 | 0 |
| `cpu_pct_avg` / `ui` / `raster` | 30.99 / 2.63 / 28.35 | 29.63 / 2.65 / 26.97 |
| `gpu_ops` / `fallback` | 57123 / 0 | 57004 / 0 |
| `time_to_first_present_ms` | 96.3 | 77.8 |
| 像素 | 54/54 | 54/54 |
| Golden | **0.0000% / 821184 px** | **0.0000% / 821184 px** |
| 逻辑 | 26/26 | 26/26 |

- `comp_states` 两跑一致：`root/body/a/b/c/d/s/s2/wrap/pw=true`（10），`hot/neg=false`（2）。
- 冻结观测：`rr_at_freeze` 125 / 123，`rr_after_freeze` **2 / 2**，`skip_after_freeze` 2938 / 2931，`freeze_observed=true`。
  即 rr 来自前 60 tick 的真实内容变化；冻结后同样的内容按指纹 Replay，7 秒里只再录 2 次，skip 继续涨近 3000。
- `rr_steady` 分别为 127 / 125，`rr_spike=0`、`rr_recover=0`，相位和 = `boundary_rerecord`。
- 两跑前后 `showcase_r3b_compbits_base.png` 的 sha256 未变（958fa257…），基线没有被本窗写动。

### manual 15s（-manual-seconds 15）

- `presents=899`、`paint_count=898`、`skip=6035`、`rr=258`、`count=8`、`depth=4`。
- `rr_steady=243`、`rr_spike=5`、`rr_recover=10`；`phases_seen=["Steady","Spike","Recover"]`、`phase_mode=animated`。
- `fps_interval=59.92`、`p95=17.39ms`、`hitch=0`、`vsync=true`、`fallback=0`。
- `rr_at_freeze` / `skip_at_freeze` / `rr_after_freeze` / `skip_after_freeze` 均为 `null`、`freeze_observed=false`：manual 没有冻结相位，不把总计数伪装成“冻结后”数字。
- `pixel_evidence=manual_not_asserted`、`pixel_golden_diff_pct=null`、`0/0`；逻辑 24/24（manual 无 `gate_phase_frozen`/`gate_rr_spike_zero`，改为 `manual_rr_spike_positive`）。
- manual 档 `SnapshotPath` 为空，不写 gate 快照；`snapshot_png=""` + `snapshot_note` 说明，跑完 manual 后 `testdata/showcase_r3b_compbits.png` 哈希不变。

### 基线文件

- 当前 gate 快照：`testdata/showcase_r3b_compbits.png`（GPU readback，不是窗口截图）。
- Golden 基线：`testdata/showcase_r3b_compbits_base.png`（1200×800 RGBA）。
- 首次 `GPUI_ACCEPT_GOLDEN=1` 只播种基线，输出 `SEEDED`，不算 PASS；必须再跑两次 0.0000%。

## 已知边界

- 8s 短窗只证明功能/帧时/像素，不证明无内存泄漏；泄漏结论需 ≥60s soak + `rss_slope_valid=true`。
- gate 前 60 tick 的 hot/hotS 颜色变化是**故意**制造真实内容脏；冻结后颜色相同，boundary cache 按内容指纹 Replay，因此 `boundary_rerecord` 基本持平（实测冻结后 7 秒只再录 2 次），rr 来自真实内容变化，不是每帧假计数。manual 相位持续改内容，三相位 rr 均有增长。
- `frame_raster_ms` 是本窗异步管线观测值（约 118–126ms 量级），R3b 不设 B 族硬门禁；A 族 fps/p95 已绿。
- `damage_ratio=1` 是 FullPaint 正确性窗的正常结果，**不冒充** retained 脏区。
- 本轮只在 X11 后端跑（JSON `backend=x11`）；Wayland 未跑，不能声称 Wayland 绿。
- 本窗是引擎能力的**原子验证**，没有组件/2.5D/视频生产消费者；不按生产接线标绿。
