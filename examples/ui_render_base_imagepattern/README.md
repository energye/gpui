# ui_render_base_imagepattern — B 组旧 Pattern / 画笔直调真窗

旧 `Pattern` 接口的独立直调真窗（原子验证）：`CreateImagePattern`、
`ImagePattern` 的 anchor / scale / opacity / clamp / transform、`SetFillPattern`、
`SetStrokePattern`、`NewSolidPattern`，以及 `BrushFromPattern` /
`PatternFromBrush` / `Paint.SetBrush` / `GetBrush` / `SolidColor` / `ColorAt`
的互相转换。

对应总账：

- `docs/RENDER_FEATURE_COVERAGE.md` §1 #10「画笔/图案」、§3.2 B 组点名项；
- `docs/ENGINE_UI_RENDER_BASE.md` §23 窗口表；
- 离屏旧示例 `examples/render_images` / `render_visual_images` 只出 PNG，不算真窗。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动门禁：5 秒、JSON 到 stdout、失败退出 1
RUN_SECONDS=5 go run ./examples/ui_render_base_imagepattern -auto-only

# 人工常驻：开窗直到手动关闭（事件写 stderr 并改标题）
go run ./examples/ui_render_base_imagepattern

# 人工 + 15 秒自动收尾
go run ./examples/ui_render_base_imagepattern -manual-seconds 15
```

首次建立 Golden（只在基图不存在时生效；播种后必须复跑确认 `0.0000%`）：

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_imagepattern -auto-only
RUN_SECONDS=5 go run ./examples/ui_render_base_imagepattern -auto-only
```

## 画面（1200×800，2×2 四卡）

| 卡 | 内容 | 证据 |
|---|---|---|
| A | 32×16 图集左侧 16×16 四象限裁片，按 16px 平铺；右侧预览使用同一份图集 | 逻辑 `imagepattern_kind` / `tile` / `srcregion` / `anchor` / `transform`；像素 `A_tile_*` / `A_repeat_*` |
| B | B1 anchor + scale 2 + opacity .5；B2 clamp 首块、范围外透卡底；B3 旋转 + 缩放 2 的 ImagePattern | 逻辑 `imagepattern_anchor` / `scale` / `opacity` / `clamp`；像素 `B1_*` / `B2_*` / `B3_*` |
| C | `SetStrokePattern` 画非矩形圆环（外径 72、内径 48、线宽 24）；图案相位锚在卡原点 | 像素 `C_top_red` / `C_top_blue` / `C_left_blue` / `C_left_yellow` 四象限 |
| D | 自定义 `Pattern` 棋盘、`NewSolidPattern`、`Paint` 的 brush / pattern 互转 | 像素 `D_checker_*` / `D_solid` |

源图集 `testdata/pattern_atlas.png` 为 32×16：左侧 16×16 是红/绿/蓝/黄四象限，
右侧 16×16 是洋红。像素 oracle 直接来自这份图集的象限颜色、棋盘公式和卡底色，
不调用被测 `Pattern.ColorAt`。

## 证据口径

- **逻辑 17/17**：类型/源区域、anchor、scale、opacity、clamp、transform、nil 安全、
  solid pattern、`BrushFromPattern` 的 solid/custom 分支、`PatternFromBrush` 的
  solid/custom 分支、`Paint` 的 solid/custom/pattern fallback。
- **像素 25/25**（GPU 快照，容差 8/255）：A 六点、B1 四点、B2 两点、B3 四点、
  C 四点、D 五点；C 四点按图案四象限落在环带上，证明描边按图案逐像素采样，
  而不是被实色快速路径吃掉。
- **Golden**：表头 1200×48 + 图例 + 四卡（430×232 ×4），合计 **627200 px** 逐位
  零容差；播种后复跑 `diff=0.0000%`。
- `-auto-only` 实测（2026-10-21）：`logic=17/17 pixel=25/25 golden=0.0000%`，
  `presents=300`，`fps_interval=60.06`，`interval_p95=18.37ms`，`hitch=0`，
  `gpu_ops=15003`，`cpu_fallback_ops=0`，`present_policy=full_paint`，
  `vsync_source=true`。
- 人工档 `-manual-seconds 15` 退出 0：`presents=899`，逻辑/像素/Golden 同绿，
  `fps_interval=59.96`，`interval_p95=18.35ms`，`hitch=0`，`gpu_ops=44953`，
  `cpu_fallback_ops=0`。

## 同批修的引擎缺陷

**非实色描边被快速实色路径吃掉。** `GPURenderContext.StrokePath` 在 AA、无虚线、
无 mask、相似变换时走 `fillPathUser` 快路径；该尾部只发一个实色 stencil 命令，
颜色来自 `getColorFromPaint`，对 ImagePattern 只取 `ColorAt(0,0)`。C 卡圆环因此
整环变成图集在 `(0,0)` 处抽到的黄色；修前像素实测 `C_top_red` 得到
`(0.949,0.800,0.102)`（source 黄色），而期望是红色 `(0.902,0.102,0.102)`。
修前调试记录：`fillPathUser non-solid color=(0.949,0.800,0.102,1.000)
bounds=(323,403)-(469,549) pattern=*render.ImagePattern`。

修法（对齐 Skia/Flutter：非实色描边必须按 brush 逐像素采样）：

- 快路径门槛改为 `strokeSolidFastPathEligible`，只有 `isGPUSolidPaint` 才允许进入
  `fillPathUser`；ImagePattern / 渐变 / 自定义 brush 全部走 `FillPath` 的原生
  pattern / gradient 路径。
- `fillPathUser` 加防御性 `!isGPUSolidPaint` 早退，避免后续调用方再把非实色 paint
  送进只支持实色的尾部。
- 永久回归 `render/internal/gpu/stroke_fast_path_test.go`：实色可走快路径、
  非实色与 nil paint 不可走；`fillPathUser` 对非实色返回 `ErrFallbackToCPU`。

该修复影响所有「非实色 + 快速描边」场景（ImagePattern、渐变、自定义 brush），
不改变实色描边的几何与输出。`go test ./render/internal/gpu -count=1` 与新增两例
已绿；`go test ./render` 的存量环境/性能红项（H01/H03/U05、GL present smoke、
AA probe、BlendHue）与本次改动无关，见本窗提交说明。

## 已知限界（不冒充完成）

1. **旧 Pattern API 仍无生产消费者**：组件库和 2.5D/视频模块目前没有使用
   `SetFillPattern` / `SetStrokePattern`；本窗是原子验证，不标生产接线。
2. **RSS 启动爬坡（非泄漏，2026-10-21 复核）**：5 秒档 `rss_start≈74.4MB →
   rss_end≈282.7MB`、`rss_slope≈489,753 KB/min` 是窗口/设备初始化爬坡；同批 c0
   空白窗 5 秒回放约 69.9 万，证明基线同样如此。60 秒实测 `rss_slope≈5,507
   KB/min`，稳态在 30k 预算内；`smaps` 增长只在 Go 堆，`WR_MEMDIG` live
   1.75MiB / peak 20.58MiB。原“资源指标未绿”判断收回；判漏看
   `rss_slope_steady_kb_per_min` 且 `rss_slope_valid=true`。
3. **没有抓屏 vs 快照断言**：本窗三证据为逻辑探针 + GPU 快照像素 + Golden；
   `maskops/backdrop` 那类窗口额外做了 `xwd` 抓屏对比，本窗暂未接。
4. **B3 是旋转 ImagePattern，不是任意自定义 Pattern 的 GPU 场**：D 卡的自定义
   棋盘走 CPU/场采样路径，用来验证 Pattern/Brush/Paint 互转，不等同于任意
   `Pattern` 都能在 GPU 上做带变换的场填充。

## 文件

- `main.go`：窗口 / 门禁 / 像素探针 / Golden / JSON / 手动事件。
- `scene.go`：源图集加载、逻辑探针、四卡绘制、像素 oracle。
- `testdata/pattern_atlas.png`：32×16 固定源图集（SHA-256 前缀 `a979999c3a54e5a4`）。
- `testdata/showcase_imagepattern_base.png`：Golden 基线。
- `testdata/showcase_imagepattern.png`：本轮快照，每跑重写。
