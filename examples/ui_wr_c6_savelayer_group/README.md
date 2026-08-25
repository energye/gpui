# ui_wr_c6_savelayer_group — C6 组合真窗（R18+R3）

SaveLayer 离屏组 + boundary 缓存 + 超预算拒批**共存**（§3 C6 行）。retained 策略下，
每帧预算 `MaxOps=1`：组 A 每帧请求 SaveLayer 被允许（`PushLayerIsolated(0.55)` 真离屏
RT），组 B 第二个请求被拒批（红框直绘、无组 α），组 C 仅 Spike 相出现作为第三个被拒请求。
全程右侧嵌套 boundary 静态缓存照常 skip——证明离屏合成、缓存、预算三者互不破坏。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/ui_wr_c6_savelayer_group
```

快照目录默认 `/tmp/c6_savelayer_group`，可用 `C6_SNAP_DIR` 覆盖；Golden 基线
`c6_final_base.png` 首跑自动生成，次跑起逐位对比。

## Window / Close duration

- 窗口 **1200×800**（U15）；关闭用时长 **10s**（§2.5 SaveLayer/Filter 档 max(R18=10, R3=8)）。
- `RUN_SECONDS<10 → FAIL`；`<5 → RequireMinRun FAIL`（U16）。
- 无 GPU/X11 环境 → `FAIL: window open (needs_gpu_window)` exit 1，禁止 stub 关窗。

## Visible effect

| Region | Expectation |
|--------|-------------|
| GROUP-A 卡（左上板） | 两矩形经 α=0.55 半透明合成（右半叠蓝），标签 "GROUP-A allowed 合成"，内容恒定不闪 |
| GROUP-B 卡（左下板） | 绿色填充**直绘**（未经混合）+ 四边红拒批框 + "GROUP-B BUDGET REJECT" 标签 |
| GROUP-C 板（中列） | 仅 Spike 相出现：紫填充 + 红框 "GROUP-C REJECT (spike)"；Steady/Recover 相隐藏 |
| DENSE 右侧面板 | 4×4 boundary 色格 + 8 标签 + 注记全程静止（缓存 replay，肉眼零变化） |
| HOT 方块（dense 下方） | 每帧变色（Spike 相转速×2），人眼可辨静/动分界 |
| SL banner（中列下方） | `allow=N reject=N` 实时增长：稳态每帧 +1/+1，Spike 相每帧 +1/+2 |
| 底栏 HUD | 能力 ID C6 / 相位 / fps / p95 / policy=retained / allow-reject / skip-rr / PASS 预览色 |

## Gates（§3 C6 行 + §2.5 SaveLayer/Filter 档 + §2.2 全族；硬，不许放）

| 门禁 | 阈值 | 来源 |
|------|------|------|
| savelayer_allow | ≥1（实测每帧 +1） | §3 C6 |
| savelayer_reject | ≥1（稳态每帧 +1，Spike +2） | §3 C6 |
| boundary_skip | ≥3 且单调增长 | §3 C6 |
| present_policy | retained | §2.2 族 C |
| fps_wall | ≥55（持续 tick 窗） | §2.2 族 A |
| interval_p95_ms | ≤22 | §2.2 族 A |
| hitch_rate_per_min | ≤5（§2.2.3 稳态预算；关闭证据跑取 RUN_SECONDS=30——单次启动毛刺在 10s 窗口数学上即 ≥6/min，见下方「Hitch 计量说明」） | §2.2.3 默认预算 |
| cpu_pct_avg / (ui 或 raster) | 非双 0 | §2.2 族 D 防装绿 |
| paint_count drift | ≤40（min−max 带） | U20 脏区维度 |
| 像素断言 | 6/6（见下） | U21 §2.7 |
| Golden | 次跑起静态掩码逐位零差异 | U21 §2.7 |
| vsync_source | 非空（fallback 禁宣锁 60Hz） | §2.2 族 A |

## 像素断言（U21 三证据之一）

| # | 探针 | 断言 |
|---|------|------|
| P1 | groupA_offscreen_blend @ 卡内 rect1 独占点 | = 0.55·gA1 + 0.45·boardBG（容差 ±12/255） |
| P2 | groupA_two_stage_blend @ rect2∩rect1 | 组内先 0.6·gA2+0.4·gA1，再组 α 混背景（±12/255）——证明真离屏两段混合 |
| P3 | groupB_direct_fill_unblended | = gB1 **精确色**（±6/255）——若引擎错误允许则呈混合色即 FAIL |
| P4 | groupB_refusal_frame @ 顶框条 | = rejectRed（±8/255）——拒批可视化在场 |
| P5 | dense_cell_intact_under_churn @ cell(0,0) | 缓存格颜色不变（±8/255） |
| F6 | dense_note_text_density | 注记文字区 ≥120 文本像素（区域属性） |

三证据合证：逻辑探针（allow/reject/skip 计数）+ 像素断言（P1–P5/F6）+ Golden 静态掩码跨跑逐位。

## 实现点六维（U20）

| 维度 | 本窗回答 |
|------|----------|
| 正确性 | 组 A 经真离屏 RT 合成（P1/P2 两段精确公式解）；组 B 拒批后直绘（P3/P4 反证无暗混合） |
| 脏区 | retained 下稳态只重录三张卡+banner+HOT；damage 不含静区；paint_count 漂移 ≤40 |
| 缓存 | 离屏组持续搅动时 dense 边界照常 skip 且单调增；像素/Golden 证静区零变化 |
| 边界条件 | 每帧预算重置语义（allow/reject 每帧成对增长）；相位切换组 C 显隐；resize 重解析探针几何 |
| 失败模式 | 允许顺序错/B 被误允许/混合公式破/缓存风暴 → 对应计数门禁或像素断言红，exit 1 |
| 窗内如何看出 | HUD allow/reject/skip 数字、B 红框、C 相位显隐、DENSE 纹丝不动 |

## Hitch 计量说明（§2.2.3 稳态语义）

hitch 预算 5/min 是**稳态帧间隔**语义。10s 关闭档里一次启动毛刺（首帧纹理分配，
time_to_first_present ≈54ms）折合 6.01/min——数学上必然超线，不代表稳态退化。
关闭证据按 §2.5 档跑 RUN_SECONDS=30 两连：实测 2.00/min（D 轮）与 0.00/min（E 轮），
fps 60.0 / p95 17.5–17.6ms，均在线内。

## present_mode 观察留痕（2026-08-25）

policy=retained 且每帧走 `presentPacketTextured`（纹理复合路径，探针证实），
但 `PresentOutcome.Mode` 报 `full`、damage_ratio=1：`render/present_target.go`
的 `postResizeFull/inResizeStormLocked` 之外未见对 retained 复合的 partial 判定
分支（`out, err = t.dc.PresentFrameAuto(...)` 的 auto 结果恒 full）。这是 R4/C2
已关窗共用的现状（C5 同值），不影响本窗门禁（retained 证据 = boundary skip/rerecord
分账 + 纹理复合路径），如实记录不粉饰；若后续做 W6 默认 retained，此处应一并收口。
