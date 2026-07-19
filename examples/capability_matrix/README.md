# capability_matrix 运行命令

`capability_matrix` 是 Linux/X11 真窗口能力矩阵示例，用来验证 `docs/SKIA_2D_CAPABILITY_MATRIX.md` 中的 2D 画布能力在真实 webgpu present 路径下可用。

程序没有命令行 flag，运行控制主要通过 `GPUI_*` 环境变量完成。自动化时保持一个进程只跑一个场景：`GPUI_SCENARIO=C0x GPUI_ANIM_SECONDS=N`。

## 前置条件

- Linux，且构建标签满足 `linux && !nogpu`
- 可用 X11 `DISPLAY`
- 可用 GPU/WebGPU 环境
- 仓库根目录下存在 `lib/libwgpu_native.so`，或手动设置 `WGPU_NATIVE_PATH`

程序启动时会在未设置 `WGPU_NATIVE_PATH` 时尝试使用 `lib/libwgpu_native.so`，并默认设置 `GPUI_SURFACE_SAMPLE_COUNT=1`。

批量脚本会设置常用环境：

```bash
WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
LD_LIBRARY_PATH=$PWD/lib
DISPLAY=:0                # run_capability_matrix.sh 默认
DISPLAY=:1                # run_capability_matrix_p6.sh 默认
XAUTHORITY=/run/user/1000/gdm/Xauthority
GOCACHE=/tmp/gpui-go-cache
```

## 基础运行

在仓库根目录运行。未设置 `GPUI_SCENARIO` 时默认使用 C01 SurfacePresentClear，手动关闭窗口退出。

```bash
go run ./examples/capability_matrix
```

编译后二进制运行：

```bash
go build -o /tmp/capability_matrix ./examples/capability_matrix
/tmp/capability_matrix
```

## 单场景运行

C01 跑 12 秒后自动退出：

```bash
GPUI_SCENARIO=C01 GPUI_ANIM_SECONDS=12 go run ./examples/capability_matrix
```

二进制版本：

```bash
go build -o /tmp/capability_matrix ./examples/capability_matrix
GPUI_SCENARIO=C01 GPUI_ANIM_SECONDS=12 /tmp/capability_matrix
```

写 result JSON 和单行摘要：

```bash
GPUI_SCENARIO=C20 \
GPUI_ANIM_SECONDS=12 \
GPUI_RESULT_FILE=/tmp/cap_C20.json \
go run ./examples/capability_matrix
```

输出文件：

```text
/tmp/cap_C20.json
/tmp/cap_C20.json.line
```

## 批量回归

跑 C01-C20：

```bash
scripts/run_capability_matrix.sh
```

指定时长和输出目录：

```bash
GPUI_ANIM_SECONDS=12 \
GPUI_CAP_OUT=/tmp/capability_matrix_run \
scripts/run_capability_matrix.sh
```

使用已有二进制：

```bash
go build -o /tmp/capability_matrix ./examples/capability_matrix
CAPABILITY_MATRIX_BIN=/tmp/capability_matrix \
GPUI_CAP_OUT=/tmp/capability_matrix_run \
scripts/run_capability_matrix.sh
```

## P6 全量 C01-C32

P6 脚本跑 C01-C32，并生成 `PERF.md`。

```bash
scripts/run_capability_matrix_p6.sh
```

指定输出目录和每场景秒数：

```bash
GPUI_CAP_OUT=/tmp/cap_p6_run \
GPUI_ANIM_SECONDS=8 \
scripts/run_capability_matrix_p6.sh
```

使用已有二进制：

```bash
go build -o /tmp/cap_l0_bin ./examples/capability_matrix
CAPABILITY_MATRIX_BIN=/tmp/cap_l0_bin \
GPUI_CAP_OUT=/tmp/cap_p6_run \
scripts/run_capability_matrix_p6.sh
```

## Golden 捕获与比较

捕获 C01-C32 deterministic golden：

```bash
GPUI_P6_MODE=capture-golden \
GPUI_CAP_OUT=/tmp/cap_p6_run \
GPUI_CAPTURE_DIR=/tmp/cap_p6_run/capture \
GPUI_CAPTURE_FRAME=90 \
scripts/run_capability_matrix_p6.sh
```

比较 capture 与仓库 golden：

```bash
GPUI_P6_MODE=compare \
GPUI_CAP_OUT=/tmp/cap_p6_run \
GPUI_CAPTURE_DIR=/tmp/cap_p6_run/capture \
GPUI_GOLDEN_DIR=examples/capability_matrix/golden \
scripts/run_capability_matrix_p6.sh
```

直接调用比较脚本：

```bash
python3 scripts/cap_compare_golden.py \
  --got /tmp/cap_p6_run/capture \
  --golden examples/capability_matrix/golden \
  --diff-dir /tmp/cap_p6_run/diff \
  --report /tmp/cap_p6_run/golden_report.json
```

默认比较门槛：

```text
RMSE <= 0.08
SSIM >= 0.90
```

## 单场景 PNG capture

在指定帧保存 PNG：

```bash
GPUI_SCENARIO=C06 \
GPUI_ANIM_SECONDS=3 \
GPUI_DETERMINISTIC=1 \
GPUI_GOLDEN_NO_HUD=1 \
GPUI_CAPTURE_DIR=/tmp/cap_capture \
GPUI_CAPTURE_FRAME=90 \
go run ./examples/capability_matrix
```

输出：

```text
/tmp/cap_capture/C06.png
```

## 场景 ID

| ID | 名称 | 中文名 |
| --- | --- | --- |
| C01 | SurfacePresentClear | 窗口呈现/清屏 |
| C02 | TransformStack | 变换栈 |
| C03 | PathFillStroke | 路径填充+描边 |
| C04 | HairlineDash | Hairline+虚线 |
| C05 | ClipRectRRect | 矩形/圆角裁剪 |
| C06 | GradientPattern | 渐变+图案 |
| C07 | BlendModes | 可分离混合 |
| C08 | LayerOpacity | 半透明图层 |
| C09 | ImageWritePixels | 贴图+写像素 |
| C10 | TextCJKDecor | 中英文+装饰 |
| C11 | FilterBlurShadow | 模糊/阴影滤镜 |
| C12 | MeshVertices | 顶点网格 |
| C13 | EvenOddPath | EvenOdd 填充 |
| C14 | MaskLayer | 蒙版图层 |
| C15 | BackdropBlur | 背景采样层 |
| C16 | DamagePresent | 局部 Damage Present |
| C17 | AdvBlendPanel | 高级混合 |
| C18 | TextLCDShape | LCD 子像素文本 |
| C19 | RRectXYRadii | 独立圆角半径 |
| C20 | CompositeUI | 多能力 UI 合成 |
| C21 | PorterDuffBoard | PorterDuff 混合板 |
| C22 | ClipPathDiff | 路径/Difference 裁剪 |
| C23 | GradientTileLocal | 渐变 tile/局部矩阵 |
| C24 | ImageAdvanced | 图高级采样 |
| C25 | TextShapeEmoji | 文本 shaping/混排 |
| C26 | PathAdvanced | 路径进阶 |
| C27 | TransformAdvanced | 变换进阶 |
| C28 | LayerFilterGraph | 层混合+滤镜链 |
| C29 | QualityMSAAAA | 质量 MSAA/AA |
| C30 | AtlasPicture | Atlas+Picture 录制 |
| C31 | PathRasterFast | 路径光栅快径 |
| C32 | CompositeRegression | 合成压力回归 |

## 常用环境变量

| 变量 | 说明 |
| --- | --- |
| `GPUI_SCENARIO` | 场景 ID：C01-C32 |
| `GPUI_ANIM_SECONDS` | 自动退出秒数；未设置时手动关窗口退出 |
| `GPUI_RESULT_FILE` | 写 JSON 结果；程序会同时生成 `.line` 摘要 |
| `GPUI_ANIM_LOG_EVERY` | 每多少帧打印一次运行日志，默认 60 |
| `GPUI_TARGET_FPS` | 目标 FPS，默认 60，程序限制在 15-120 |
| `GPUI_DETERMINISTIC` | 使用固定 60Hz 时间线，常用于 golden/capture |
| `GPUI_FIXED_T` | 固定绘制时间 `t`，优先级高于 deterministic 时间线 |
| `GPUI_GOLDEN_NO_HUD` | capture/golden 时隐藏 HUD |
| `GPUI_CAPTURE_DIR` | 保存 PNG 的目录；设置后在指定帧保存 `${GPUI_SCENARIO}.png` |
| `GPUI_CAPTURE_FRAME` | PNG capture 帧号，默认 90 |
| `WGPU_NATIVE_PATH` | `libwgpu_native.so` 路径 |
| `LD_LIBRARY_PATH` | 需要包含仓库 `lib` 目录 |
| `DISPLAY` | X11 display |
| `XAUTHORITY` | X11 授权文件，常见为 `/run/user/1000/gdm/Xauthority` |

## 批量脚本环境变量

| 变量 | 说明 |
| --- | --- |
| `GPUI_CAP_OUT` | 输出目录；`run_capability_matrix.sh` 默认 `/tmp/capability_matrix_run`，P6 默认 `/tmp/cap_p6_run` |
| `CAPABILITY_MATRIX_BIN` | 批量脚本使用的二进制路径 |
| `GPUI_ANIM_SECONDS` | 每个场景运行秒数；普通脚本默认 12，P6 默认 8 |
| `GPUI_P6_MODE` | P6 模式：`regress`、`capture-golden`、`compare` |
| `GPUI_GOLDEN_DIR` | golden PNG 目录，默认 `examples/capability_matrix/golden` |
| `GPUI_CAPTURE_DIR` | capture PNG 目录，默认 `$GPUI_CAP_OUT/capture` |
| `GPUI_CAPTURE_FRAME` | capture 帧号，默认 90 |
| `GPUI_MAX_RMSE` | golden 比较 RMSE 上限，默认 0.08 |
| `GPUI_MIN_SSIM` | golden 比较 SSIM 下限，默认 0.90 |

## 判定规则

运行结束会写 `status=PASS` 或 `status=FAIL`。主要门禁：

- 帧数不少于 30
- `cpu_fallback_ops=0`
- `gpu_ops>0`
- capability probe 通过
- 非 `AllowLowFPS` 场景要求稳态 FPS 接近 `GPUI_TARGET_FPS`
- 稳态 RSS 增量不超过 512 MiB

