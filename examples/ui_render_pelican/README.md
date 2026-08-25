# ui_render_pelican — 鹈鹕骑行 · Pelican Rider

按 `pelican-bike.html` 参考实现 1:1 移植的渲染层 2D 场景动画真窗：
一只鹈鹕骑着自行车在乡间公路上飞驰。

## 画面内容（与参考实现逐元素对应）

- 天空线性渐变 + 旋转光芒太阳（60s/圈）
- 三朵漂移云（55s/75s/95s 循环，含负延时相位）+ 两只扇翅飞鸟（38s/52s 横穿）
- 四层视差滚动：远山(×0.22, 周期1200) / 树木(×0.50, 周期800) / 车道虚线(×1.0, 周期130) / 前景草丛(×1.35, 周期600)
- 自行车：辐条车轮随车速旋转、红色车架、车筐 + 鱼、链条牙盘
- 双腿两段式逆运动学踩踏（髋-膝-踝，取偏前解）、曲柄踏频 = 轮转速 / 2.6
- 鹈鹕：身体随踩踏起伏（sin(2θ)·2.6）、翅膀扇动、围巾双片飘动、后轮扬尘三相位
- 右下 HUD 药丸：暂停/播放按钮（悬停变红）+ 速度滑条（×0.3..×2.5，步进 0.1）
- 左下提示文字；标题「鹈鹕骑行记 / PELICAN RIDER」逐字排版带 letter-spacing

## 双时钟模型

与浏览器行为一致：

- **环境时钟**驱动 CSS/SMIL 类动画（云、鸟、太阳、围巾、翅膀、扬尘），永不暂停；
- **骑行时钟**驱动视差滚动、车轮、曲柄、双腿 IK、身体起伏，暂停时冻结
 （对应 JS `frame()` 的 `if (paused) return;` 早退——页面里 CSS 动画不受 JS 暂停影响）。

## 操作

| 输入 | 动作 |
| --- | --- |
| 空格 | 暂停 / 继续 |
| ↑ / ↓ | 调速 ±0.1（×0.3..×2.5） |
| 点击圆钮 | 暂停 / 继续 |
| 点拖滑条 | 连续调速 |

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_render_pelican                  # 不限时长，关窗退出
RUN_SECONDS=15 go run ./examples/ui_render_pelican   # 定时退出（>=10s 门禁 fps>=30）
SNAPSHOT=tmp/pelican_shot.png RUN_SECONDS=5 go run ./examples/ui_render_pelican
```

### 诊断环境变量（引擎 AA 缺陷复现工具）

| 变量 | 作用 |
| --- | --- |
| `PELICAN_T=秒` | 把仿真冻结到确定时刻（云/鸟/视差/曲柄全部确定），供逐像素对比 |
| `GOLDEN_PNG=path` | 不开窗，用软件光栅渲染 `PELICAN_T` 单帧存 PNG（配合 `GOGPU_RENDER_MODE=cpu` 得纯 CPU 基准） |
| `GPUI_BACKEND=x11\|wayland` | 强制显示后端（x11 后端可用 `xwd -id <窗口>` 抓真实上屏内容） |

标准复现流程（同帧三方对比）：

```bash
GOGPU_RENDER_MODE=cpu PELICAN_T=5 GOLDEN_PNG=/tmp/gt.png go run ./examples/ui_render_pelican
GPUI_BACKEND=x11 PELICAN_T=5 RUN_SECONDS=40 go run ./examples/ui_render_pelican &  # xwd 抓窗口
SNAPSHOT=/tmp/snap.png PELICAN_T=5 RUN_SECONDS=3 go run ./examples/ui_render_pelican
```

## 已知引擎层缺陷（✅ 已修复，2026-08-25）

**曲线/斜线描边沿线黑色锯齿暗斑**——根因为 `render/internal/gpu/stencil_renderer.go` Tier-2b
（stencil-then-cover，sampleCount==1）的**内侧边缘带在二值 cover 之后以 Replace 混合写预乘
部分覆盖色**，背景项丢失导致 f<1 处偏暗。已重排为「内外带都在 cover 前 SrcOver 混合于完整底色，
内带画完即清 stencil，cover 自动跳过」，真机验证暗斑 617→94（软渲基线 49，剩余为边缘 AA 相位差）。
详见 `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表「曲线暗斑修复」行。

**附带发现（未修，独立问题）**：`GOGPU_RENDER_MODE=cpu` 且注册了 GPU 会话时，多子路径开线描边
（如车架 `M..L..M..L..` 五段）会被闭合填充成实心多边形（纯 CPU 无 GPU 注册时正常）——软渲回退模式质量缺陷。


## 实现说明

- 舞台坐标系 = SVG viewBox 1200×700；每帧按窗口等比缩放居中（默认窗口 1:1）。
- 帧首推 `T(ox,oy)·S(s)` 基矩阵后全部用 SVG 原始坐标绘制，描边宽度随 CTM 缩放
 （Skia/Cairo 惯例）；组旋转用 Save/RotateAbout/RestoreCanvas 对应 SVG transform 组语义。
- 静态路径（山体瓦片、车架、鹈鹕各部位等）只在初始化构建一次；动态路径
 （飞鸟翅形、双腿折线）每帧重建。
