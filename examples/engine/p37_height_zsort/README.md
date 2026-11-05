# p37_height_zsort — 批 6 ② 高度与 z 序窗

P3.7 批 6 第 ② 扇真窗（收台账 743：批 2 z 序与世界高度归位）。满窗即内容，
指标行浮在画面左上（金图 mask 掉 `y<32` 带）。五臂一屏：

- **A z 桶分组与 ±4096 CLAMP**：四只同脚位精灵，ZIndex 故意越界（±5000），
  桶大者盖小者；`ClampZIndex` 饱和表 + y 序→`YSortZ` 槽→再按槽排序的往返恒等。
- **B1 y 键 / B2 平票不闪**：y 差出容差由 y 定序；y 落进 `is_equal_approx`
  容差带由提交序号破平票，且窗内**逐帧重排序 240 帧恒同序**（JSON
  `window_tie_frames_checked`/`window_tie_stable`）。
- **C Height→投影→盖序 + LandShadow**：同世界 (x,y) 三档折叠高度
  （world 父 1.0 + 子 {-1,0,1} ⇒ {0,1,2}，纯求和折叠）经 `Basis45` 投影出
  脚位阶梯，按投影脚位 Y 排序——高者更近、盖住矮者；每只精灵
  `LandShadow` 落影子到地面、`ShadowVisible` 判可见。
- **D 非有限 fail-closed（拒绝态底带）**：树路 InvalidArg（NaN/Inf/坏父/
  环）、`sprite.NewItem` InvalidArg、超 4000 OutOfMemory、平铺 YSort 不校验
  （批 2 钉下的口径，如实记）、`LandShadow` 地下目标隐藏——八枚色片绿=按预期。
- **E 跨 tier 提交序=盖序不变式**：路径填充（SDF tier）与精灵（image tier）
  交错提交，交叠处=最后提交者。

三证据合证：22 枚离屏逻辑/像素探针 + 终帧语义像素断言（坐标由同一份排序/
投影输出反推）+ 双后端金图逐位（容差声明：直色探针 2 档、alpha 混合 6 档、
金图 0）。H15 定帧停机（`gateFreezeAt=120`）六轮恒 0 + 常驻相画面真在动；
H16 四变异各有三重报警（逻辑/像素/金图 0.25%·0.31%·0.96%·1.11%）。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/engine/p37_height_zsort -auto-only          # 门禁相
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_height_zsort -auto-only  # 播种，仍不算过
go run ./examples/engine/p37_height_zsort                                   # 常驻人工
GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_height_zsort -auto-only  # 第二后端一轮
```

金图：`testdata/goldens/p37_height_zsort_final_{native,go}.png`（fail-closed：
缺基线判红，播种那一跑不算 PASS）。零新增公开 API——全部走 camera/sprite/world
引擎公开口与 render Canvas。
