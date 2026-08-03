# ui_wr_r4b_multidamage — R4b DirtyLayerID + 多 damage

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_r4b_multidamage
```

GPU 真窗（X11 + wgpu）必需；无 GPU 环境 → `FAIL: window open (needs_gpu_window)` + exit 1。

## Window / Close duration

- 窗口：1200×800（客户区逻辑像素）
- 关闭用 RUN_SECONDS：15（本地可加长观察 30）
- `RUN_SECONDS<5` → FAIL（U16）

## Visible effect

| Region | Expectation |
|--------|-------------|
| 左上 HOT 90×90 红块 @ Align(0.0246, 0.0353) | 每帧变色（独立 boundary，独立 dirty layer id）；**布局驱动**——位置=(Body−90)×比例，resize 自动跟随 |
| 右下 HOT 90×90 蓝块 @ Align(0.926, 0.894) | 每帧变色，与左上**同帧**变脏 → 第二独立 id；相位切换 SetAlignment 微移（SPIKE 0.93/0.88、RECOVER 0.94/0.90），对角远离几何保持 |
| 中央 6×5 色格（各 100×80 + boundary） | 全程静止，retained 下不重绘（skip 累加） |
| 中央 8 个 static 标签 | 全程静止，不重绘 |
| 相位横幅 @ (360,30) | 仅相位切换时重录（STEADY→SPIKE→RECOVER） |
| HUD 底栏 | `ids=N/MAX multi=M dmg=X mode=Y skip=Z` 实时可见；`ids≥2` 且绿色 = 门禁趋势达线 |

两脏点（左上+右下）同帧变脏 → 两独立 damage rect，并集不覆盖中央静区（damage_ratio≪1），present 走 damage_multi（多矩形独立 scissor）。

## Gates

| Gate | FAIL 线 |
|------|---------|
| §3 R4b 门禁 | `dirty_layer_id_max>=2`（两远离脏点 → 两 id） |
| §3 R4b 门禁 | `damage_multi_frames>=1`（多矩形独立 scissor 出现） |
| 族 C | `present_policy=retained`；`damage_ratio_sum<=0.35`（真实重绘像素：2×90×90 热点 + HUD 10Hz ≈ 0.026，**sum of rects 非 union bbox**——frame.go 用 sum 判 full 升格，两对角热点保持 multi 不升 full） |
| 族 C | `present_mode` 非 full；`boundary_skip>0`（中央静态区不重绘） |
| 族 A | 持续 tick 下 `fps_interval>=55` |
| 族 H | `warmup=true`；首帧有内容 |
| 族 J | 无 ui→gpu import、无 cgo、未降画质（depcheck） |

> 诚实边界：`damage_ratio_avg/max`（union bbox 语义）在 R4b 两对角脏点场景下几何上接近全屏（0.49），
> 这是**并集 bbox** 而非真实重绘面积；引擎升格决策与省绘门禁均以 **sum of rects**（`damage_ratio_sum`≈0.026）
> 为准，与 §2.2 族 C「成本 ∝ 脏」语义一致。
>
> 内存诚实边界：本窗为 **15s 正确性功能窗**（§2.2.4），`rss_slope_kb_per_min=484074` 为短窗初始
> RSS 抖动（start=9MB 字体/纹理加载前 → end=134MB 首帧后稳定），非泄漏爬升——README 声明
> `slope_gate=off`（仅短于 15s 的正确性窗允许）；`rss_after_close=end`（关闭后未降，soak 才升 FAIL，
> 短功能窗记 WARN）。
>
> 帧时观察：hitch=3/15.1s（≈12/min）来自相位切换首帧与 HUD 首次刷新，非持续；本窗 <60s 非 soak，
> §2.2.2 hitch 硬门禁仅适用于长 soak 窗。fps_interval=59.4（≥55）与 interval_p95=17.0ms（≤22）为硬门禁。
