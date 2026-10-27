# ui_wr_shadermask — 用户空间 ShaderMask 真窗

验证 `render.DrawShaderMask` / `ui/rendering.PaintContext.DrawShaderMask`：
把一段内容画进隔离层，再用 mask 画笔的 alpha 调制整层，最后裁到 bounds
（对齐 Flutter `ShaderMaskLayer` / Skia `saveLayer(bounds)` + `drawRect(mask, dstIn)`）。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_shadermask -auto-only   # 门禁
go run ./examples/ui_wr_shadermask                            # 人工：直到关窗
go run ./examples/ui_wr_shadermask -manual-seconds 15         # 人工：限时
```

窗口 1200x800，满窗即内容；指标行浮在左上角（Golden 时整条 `y<40` 排除）。

## 画面

两列 × 四行，左列与右列是同一份绘制代码的两条路径：

- 左列 `RETAINED`：`PaintContext.DrawShaderMask`，在 retained UI 走查里录制。
- 右列 `DIRECT`：`recording.Recorder` + `render.DrawShaderMask`，按窗口绝对坐标
  录成 Picture，每帧原样回放（不再 Translate 到格位）。

四种掩码（每行一格）：

| 格 | mask | 内容 | 要看出的现象 |
| --- | --- | --- | --- |
| FADE | 垂直线性 white 1→0 | 图片 + 文字 + 底部蓝条 | 上浓下淡；蓝条落在零 alpha 尾部，必须被擦掉 |
| VIGNETTE | 径向 white 1→0（0.52·min(w,h)） | 橙色面板 | 中心保留、四角擦除、中间按半径过渡 |
| SOLID 0.5 | 实心白 alpha=0.5 | 不透明三色条 | 每色都以 50% 叠在格子底色上 |
| OVERLAP | 实心白 alpha=0.5 | 不透明红块 + 半透明蓝块 | 先层内合成再整体掩码（不是各画各乘） |

重叠格的判据：蓝块压红块（SourceOver）得到
`0.5·B + 0.5·A`，再乘掩码 0.5 叠底 `0.5·合成 + 0.5·底色`。
如果实现是“每条命令各自乘掩码”，重叠区数值会明显不同。

## 门禁（-auto-only 或 RUN_SECONDS）

- 18 个像素探针（9 个解析 oracle × 左右两列）：直方图取点全部避开抗锯齿边与文字。
- 9 个 retained/direct 成对探针：同格同点 RGB 差 ≤0.02。
- 位级 Golden：`testdata/showcase_shadermask.png` 对 `_base.png`，
  比较区 `{0,40,1200,760}`，要求 0.0000%。
- 屏幕抓取 vs 离屏快照：同区、±2/255、差异 ≤1%。
- 逻辑探针：`gpu_ops>0`、`cpu_fallback_ops==0`、`mask_skips==0`、
  `live_only_skips==0`、`savelayer_allow>0`、`savelayer_reject==0`、
  presents/paint 为正。
- `wrgate.EvaluateGates{MinPresents:1, RequireFullPaintPolicy:true}` 与
  `vsync_source` 非空。

Golden 播种（只做一次，之后常规跑不带该变量）：

```bash
GPUI_POWER=high GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_wr_shadermask -auto-only
```

播种后人工看一遍 `testdata/showcase_shadermask.png`，再连跑两次确认 0.0000%
才可提交 baseline。GPU 相关用例无设备时如实跳过/失败，不静默假绿。

## 已知边界

- 掩码不要嵌在 `PushLayer(Normal, opacity<1)` 的 F1 透明组里：组透明会二次
  缩放源 alpha。需要外层组透明度时用 `PushLayerIsolated`。
- 设备空间掩码通道（`SetMask`/`ApplyMask`/`PushMaskLayer`）不变，由
  `examples/ui_render_base_maskops` 覆盖。
- 直录列按绝对坐标录制，因为引擎存量缺陷：渐变画刷忽略 CTM（平移后仍按
  绝对坐标出画）。归属渲染渐变域，待单独修；本窗不绕过引擎，只用绝对坐标
  避免该缺陷干扰掩码本身的对比。
