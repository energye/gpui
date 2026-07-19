# mem_anim_window 运行命令

`mem_anim_window` 是 Linux/X11 真窗口动画压测示例，主要用于复杂 2D/UI 渲染、FPS、CPU fallback 和 RSS soak 观察。

程序没有命令行 flag，运行控制主要通过 `GPUI_*` 环境变量完成。自动化时保持一个进程只跑一个场景：`GPUI_SCENARIO=S0x GPUI_ANIM_SECONDS=N`。

## 前置条件

- Linux，且构建标签满足 `linux && !nogpu`
- 可用 X11 `DISPLAY`
- 可用 GPU/WebGPU 环境
- 仓库根目录下存在 `lib/libwgpu_native.so`，或手动设置 `WGPU_NATIVE_PATH`

批量脚本会默认设置常用环境：

```bash
WGPU_NATIVE_PATH=lib/libwgpu_native.so
DISPLAY=:1
XAUTHORITY=/run/user/$(id -u)/gdm/Xauthority
GPUI_SURFACE_SAMPLE_COUNT=1
GPUI_TARGET_FPS=60
GPUI_ANIM_LOG_EVERY=60
GPUI_FIXED_SIZE=1
```

## 基础运行

在仓库根目录运行。未设置 `GPUI_SCENARIO` 时默认使用 S12 FullComposite，手动关闭窗口退出。

```bash
go run ./examples/mem_anim_window
```

编译后二进制运行：

```bash
go build -o /tmp/mem_anim_window ./examples/mem_anim_window
/tmp/mem_anim_window
```

## 单场景运行

S03 跑 90 秒后自动退出：

```bash
GPUI_SCENARIO=S03 GPUI_ANIM_SECONDS=90 go run ./examples/mem_anim_window
```

二进制版本，带超时保护：

```bash
go build -o /tmp/mem_anim_window ./examples/mem_anim_window
GPUI_SCENARIO=S03 GPUI_ANIM_SECONDS=90 timeout -s INT 120s /tmp/mem_anim_window
```

写 metrics 和 result：

```bash
GPUI_SCENARIO=S12 \
GPUI_ANIM_SECONDS=120 \
GPUI_METRICS_FILE=/tmp/mem_anim_S12.csv \
GPUI_RESULT_FILE=/tmp/mem_anim_S12.result \
go run ./examples/mem_anim_window
```

## 批量 long-soak

默认跑 S01-S12。S01-S11 使用 `GPUI_SOAK_SECONDS`，S12/S14/S21 使用 `GPUI_SOAK_HEAVY_SECONDS`。

```bash
scripts/run_mem_anim_longsoak.sh
```

只跑指定场景：

```bash
scripts/run_mem_anim_longsoak.sh S01 S05 S11 S12
```

改默认时长和输出目录：

```bash
GPUI_SOAK_OUT=/tmp/mem_anim_soak_run/manual \
GPUI_SOAK_SECONDS=90 \
GPUI_SOAK_HEAVY_SECONDS=180 \
scripts/run_mem_anim_longsoak.sh S01 S05 S11 S12
```

跑 Skia gap / stress 扩展场景：

```bash
GPUI_SOAK_OUT=/tmp/mem_anim_soak_run/gap \
GPUI_SOAK_SECONDS=90 \
GPUI_SOAK_HEAVY_SECONDS=180 \
scripts/run_mem_anim_longsoak.sh S15 S16 S17 S18 S19 S20 S21 S22 S23
```

## 场景 ID

| ID | 名称 |
| --- | --- |
| S01 | BaselineHUD |
| S02 | GlowField |
| S03 | CardUI |
| S04 | PathDash |
| S05 | ClipStack |
| S06 | LayerOpacity |
| S07 | BackdropPanel |
| S08 | ImageMaskAtlas |
| S09 | TextStyles |
| S10 | FilterFX |
| S11 | MeshBlendXform |
| S12 | FullComposite |
| S13 | HighDensity |
| S14 | StressEveryFrame |
| S15 | GradientPattern |
| S16 | AdvancedBlend |
| S17 | ClipRRectEvenOdd |
| S18 | TextLCDShape |
| S19 | DamagePartialPresent |
| S20 | ScrollModalUI |
| S21 | SkiaGapComposite |
| S22 | Mesh3DGradient |
| S23 | Mesh3DFullComposite |

## 常用排障命令

只开 glow，其他模块全关：

```bash
GPUI_FEAT_ALL=0 GPUI_FEAT_GLOW=1 GPUI_ANIM_SECONDS=60 go run ./examples/mem_anim_window
```

关闭滤镜模块跑 S12：

```bash
GPUI_SCENARIO=S12 GPUI_FEAT_FILTER=0 GPUI_ANIM_SECONDS=90 go run ./examples/mem_anim_window
```

打开 draw 分段耗时日志：

```bash
GPUI_SCENARIO=S12 GPUI_PROFILE_DRAW=1 GPUI_ANIM_SECONDS=90 go run ./examples/mem_anim_window
```

调整目标 FPS 和日志频率：

```bash
GPUI_SCENARIO=S12 GPUI_TARGET_FPS=60 GPUI_ANIM_LOG_EVERY=30 GPUI_ANIM_SECONDS=90 go run ./examples/mem_anim_window
```

## 常用环境变量

| 变量 | 说明 |
| --- | --- |
| `GPUI_SCENARIO` | 场景 ID：S01-S23 |
| `GPUI_ANIM_SECONDS` | 自动退出秒数；未设置时手动关窗口退出 |
| `GPUI_METRICS_FILE` | 写 CSV metrics |
| `GPUI_RESULT_FILE` | 写结果；程序会同时生成 `.json.line` 摘要 |
| `GPUI_ANIM_LOG_EVERY` | 每多少帧打印一次运行日志，默认 60 |
| `GPUI_TARGET_FPS` | 目标 FPS，默认 60，程序限制在 15-120 |
| `GPUI_FIXED_SIZE` | 固定窗口尺寸，timed soak 默认固定 |
| `GPUI_RSS_HARD_KB` | RSS 硬上限，默认 3670016 KB |
| `GPUI_PERF_LITE` | 轻量绘制模式 |
| `GPUI_STRESS` | 压力模式 |
| `GPUI_DENSITY` | 额外高密度绘制数量，主要用于 S13 |
| `GPUI_CADENCE_SPARSE` | 恢复旧的稀疏轮转调度，仅用于 leak-hunt |
| `GPUI_PROFILE_DRAW` | 每 60 帧打印 draw 分段耗时 |
| `GPUI_FEAT_ALL` | `0` 表示先关闭全部 feature，再按单项开关启用 |
| `GPUI_FEAT_BG` | 背景开关 |
| `GPUI_FEAT_GLOW` | glow 模块开关 |
| `GPUI_FEAT_CARDS` | cards 模块开关 |
| `GPUI_FEAT_PATHS` | paths 模块开关 |
| `GPUI_FEAT_DASH` | dash stroke 模块开关 |
| `GPUI_FEAT_CLIP` | clip 模块开关 |
| `GPUI_FEAT_LAYER` | layer 模块开关 |
| `GPUI_FEAT_BACKDROP` | backdrop 模块开关 |
| `GPUI_FEAT_MASK` | mask 模块开关 |
| `GPUI_FEAT_IMAGE` | image 模块开关 |
| `GPUI_FEAT_TEXT` | text 模块开关 |
| `GPUI_FEAT_FILTER` | filter 模块开关 |
| `GPUI_FEAT_TRANSFORM` | transform 模块开关 |
| `GPUI_FEAT_BLEND` | blend 模块开关 |
| `GPUI_FEAT_VERTICES` | vertices 模块开关 |
| `GPUI_FEAT_PIXELS` | pixels 模块开关 |
| `GPUI_FEAT_POLYGON` | polygon 模块开关 |
| `GPUI_FEAT_GRADIENT` | gradient 模块开关 |
| `GPUI_FEAT_PATTERN` | pattern 模块开关 |
| `GPUI_FEAT_ADVBLEND` | advanced blend 模块开关 |
| `GPUI_FEAT_RRECTCLIP` | rounded-rect clip / even-odd 模块开关 |
| `GPUI_FEAT_TEXTLCD` | LCD/text shaping 模块开关 |
| `GPUI_FEAT_DAMAGE` | damage partial present 模块开关 |
| `GPUI_FEAT_SCROLL` | scroll/modal UI 模块开关 |
| `GPUI_FEAT_MESH3D` | 3D mesh 模块开关 |
| `GPUI_FEAT_HUD` | HUD 开关 |

## 批量脚本环境变量

| 变量 | 说明 |
| --- | --- |
| `GPUI_MEM_ANIM_BIN` | 批量脚本使用的二进制路径，默认 `/tmp/mem_anim_window` |
| `GPUI_SOAK_SECONDS` | 普通场景秒数，默认 90 |
| `GPUI_SOAK_HEAVY_SECONDS` | heavy 场景秒数，默认 300 |
| `GPUI_SOAK_OUT` | 输出目录，默认 `/tmp/mem_anim_soak_YYYYMMDD_HHMMSS` |

