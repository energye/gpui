# P3.4 滤镜家族 影响面评估（2026-11-01）

> 任务：P3.4 滤镜家族——ImageFilter 八族（offset/erode/dilate/lighting/tile/merge/displacement/alpha-threshold）+ ColorFilter 四族（blend/lighting/compose/table）+ MaskFilter（blur 类）+ Paint 级字段（catalog §13 P3.4 规划行）。依赖 P3.0 ✅（P3.1 软：色彩空间感知滤镜可选受益，第一版不做）。
> 对齐标准：Skia `SkImageFilters`（Offset/Morphology/LightingImageFilter/Tile/Merge/DisplacementMap/AlphaThreshold）、`SkColorFilters`（Blend/Lighting/Compose/Table）、`SkMaskFilter`。
> 调查：一路深查（节点模型/GPU 图执行/CPU 链/mask 体系/分域约定/测试先例），HEAD `0feab21b`。

## 1. 关键现状

1. **节点模型**：`ImageFilterKind` 仅 6 值（blur/blurXY/dropshadow/colorMatrix/grayscale/invert，filter_ops_gpu.go:429-444）；加新 Kind 需同步四处（枚举/`imageFilterNodeRunnable`/`imageFilterGraphGPUSupported`/`applyImageFilterNode` 分发）；`coalesceImageFilterNodes` 有 colormatrix 乘法合并先例（composeColorMatrix4x5）。
2. **GPU 图**：`filterGPUGraphWGSL` mode u32 分派（0 blur/1 gray/2 invert/3 matrix/4 shadowExtract/5 shadowComposite/6 copy）；Params 128B（size/direction/mode/radius/offset/color/matrix5x4）已满，新参数族需扩 payload 或第二 binding；ping-pong RT（res.Cache）+ uniform slab（256B stride）+ **aux_tex（binding3）双输入槽已有**（shadow hold 先例 :1278-1315）。
3. **CPU 链**：`RegisterFilterOps` 固定 6 闭包签名（filter_ops.go:20-26）——扩签名或第二注册口；internal/filter 走 scene.Filter 风格 `Apply(src,dst,bounds)+ExpandBounds`；kernel 缓存/blur 分离两 pass/temp pool 已备。
4. **分域约定**：颜色类滤镜 straight 域计算（GPU 注释明示 Skia/Flutter 同款）、premul 存储；卷积类（blur/morphology）premul 域直接处理（Skia GPU blur 同）。
5. **Mask 体系**：三条消费链（SetMask→MaskCoverage、ApplyMask→DestinationIn、PushMaskLayer→Pop 调制）；`mask_canvas.go:13-25` 自述「Skia keeps SkMask at the raster stage」；现有 filter graph 是 RGBA8 RT——mask 是 A8/R8。
6. **双输入族**：merge（N 层 over）/displacement（位移贴图）需要第二输入——aux_tex 槽+hold-RT 先例可复用。
7. **测试基座**：filter_ops_contract/gpu_test 的 premul 语义/图=顺序单步/GPU==CPU 先例；internal/filter 每族 `*_test.go` 值级断言模式；真窗 filterops 四卡+逻辑 16 键可扩展。

## 2. 能力定义（草案，按族）

| 族 | Skia 对应 | 语义 | 分域 | GPU 策略 |
|---|---|---|---|---|
| Offset | SkImageFilters::Offset | 内容平移（dx,dy），ExpandBounds=offset | premul 直搬 | 单 pass mode |
| Erode/Dilate | SkImageFilters::Erode/Dilate | 半径 r 邻域 min/max（形态学） | premul（RGBA 取 min/max 含 alpha） | 单 pass |
| Lighting | Skia LightingImageFilter | point diffuse（kd·(L·N)）；height=z=半径球面近似 | straight（颜色域） | 单 pass |
| Tile | SkImageFilters::Tile | srcRect 内容平铺填 dstRect | premul 直采样 | CPU-first |
| Merge | SkImageFilters::Merge | N 层 source-over 叠加 | premul over | aux_tex（第一版 2 输入） |
| Displacement | SkImageFilters::DisplacementMap | 位移贴图重采样（x+/x- 通道） | premul 重采样 | aux_tex |
| AlphaThreshold | Skia AlphaThreshold | alpha<阈值→0（二值化掩码） | premul alpha | 单 pass |
| ColorFilter 抽象 | SkColorFilter | Apply(rgba straight) 接口 + 四族构造器 + Compose 复合 | straight | matrix/table 单 pass；blend/lighting 单 pass |
| MaskFilter | SkMaskFilter（blur） | mask 平面 A8 模糊预处理 | alpha 域 | R8 模糊 pass 或转 RGBA 图 |

## 3. 实现批次（拍板 D1）

推荐**本步全做、四小批独立提交**（PLAN §5.1 明列全部为 P3.4 完成标准）：

- **批 1**：ImageFilter 简单单输入族 offset/erode/dilate/alpha-threshold（Kind 四处+WGSL mode 四分支+CPU 四实现+Context.ApplyOffset/Erode/Dilate/AlphaThreshold 口）
- **批 2**：lighting（point diffuse）+tile（CPU-first，GPU tile 视 Params 余量）
- **批 3**：merge/displacement（aux_tex 双输入）
- **批 4**：ColorFilter 抽象+四族+MaskFilter（A8 blur 预处理挂 SetMask/ApplyMask/PushMaskLayer）

每批：internal/filter CPU 实现+值级测试 → GPU pass（可 GPU 的族）→ Context 口 → 能力钉。

## 4. 拍板项

| # | 问题 | 推荐 |
|---|---|---|
| D1 | 范围与拆批 | 本步全做（PLAN 明列），四小批独立提交 |
| D2 | 双输入族 | merge/displacement 复用 aux_tex 槽（shadow 先例）；merge 第一版限 2 输入 |
| D3 | ColorFilter 形态 | `ColorFilter` 接口（straight 域 Apply）+四族构造器+`Compose(outer,inner)`；Context.ApplyColorFilter 口（与 ApplyColorMatrix 同构）；不挂 Paint 字段（Paint 级 ImageFilter/MaskFilter 字段随 P4.1/后续） |
| D4 | MaskFilter 落点 | mask 平面 A8 blur 预处理（SetMask/ApplyMask/PushMaskLayer 三消费链统一），对齐「Skia keeps SkMask at raster stage」自述；Paint 级字段不做 |
| D5 | CPU/GPU 分配 | 每族 CPU 必做（回退基线）；GPU 优先做 offset/erode/dilate/alpha-threshold/blend/table/lighting；tile/merge/displacement CPU-first（GPU 后续） |

## 5. 验证方案（草案）

- internal/filter 每族值级测试（offset 精确位移/erode-dilate 对偶性/lighting 球面高光/table LUT 精确/blend 语义矩阵/alpha-threshold 二值）。
- 图级钉：新 Kind 进图=顺序单步等价（现有 graph_matches_sequential 模式）+GPU==CPU。
- 真窗：`ui_render_base_filterops` 扩卡（第 5/6 卡）或新窗 `ui_wr_p34_filters`——三卡（形态学对偶/merge 双图/displacement 贴图）+金图+双后端。
- 负向：摘 morphology 的 min/max 交换（erode↔dilate 对偶破坏）→单测红→还原。
- 回归：现有六族行为零变化（contract/gpu 测试重放）。

## 6. 风险

- Params uniform 128B 已满：lighting 参数（位置/颜色/kd/ks）需扩 payload 或第二 binding——批 2 设计时定。
- displacement 的贴图坐标约定（channel→offset 映射、clamp/scale）——Skia 文档语义锁。
- mask A8 blur 的 GPU 路径：R8 采样 or 转 RGBA 图——批 4 设计时按 glyph R8 基建取舍。
- erode/dilate 的 O(n·r²) CPU 实现（Skia 有 O(n) 两 pass 优化）——第一版简单实现+性能登记。
