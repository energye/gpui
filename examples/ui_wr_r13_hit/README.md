# ui_wr_r13_hit — R13 Hit ≡ 绘

W2 R13 独立真窗：证明**控件命中测试 ≡ 绘制身份**——脚本化点击探针与真实指针点击都走引擎
`PipelineApp.HitTestPointer`，命中对象必须与绘制的目标身份（DebugName）一致。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r13_hit
```

## Window / Close duration

1200×800 · **5s**（§2 主表 + §2.5）。`RUN_SECONDS<5 → FAIL`（U16）。GPU 真窗必需。

## 能力与场景（U17）

| Region | Expectation（人眼） |
|--------|---------------------|
| A/B 色块目标（60×60 红/蓝，Align 布局驱动） | 脚本探针中心命中；高亮变黄（Spike 相发橙） |
| C 圆角裁剪目标（50×50 r=12，内绿块） | 探针中心命中绿块「target-round」 |
| D 旋转 30° 目标（内紫块 60×60） | 探针中心经**反变换**命中「target-xf」 |
| E 裁剪溢出目标（40×40 裁剪，子 70×70） | 裁剪内命中「target-clipped」；**裁剪外探针(50,50) 拒绝** |
| F/G 重叠目标（同位置 60×60 两层） | 重叠中心命中**最上层**「target-top」 |
| H 空白区锚点 | 空白探针无命中（HitDebugName 空） |
| 静态密集 4×4 色格 + 8 标签 | 始终静止（U17 静态密集） |
| HOT 热点 | 每帧变色（U17 动态热点；live paint 非 boundary） |
| HIT 横幅 + HUD | 运行中可见 `hit=ok/total` + 上次命中名 + 实时指针命中名 |

相位脚本：**Steady**（0–2s：探针注入+命中高亮）→ **Spike**（2–3.5s：全部命中目标发橙，
人眼见「点哪高亮哪」）→ **Recover**（≥3.5s：还原基色）。

实时指针：X11 `EventPointer+PointerDown` → 同一 `HitTestPointer` → HUD/横幅显示命中名 +
高亮目标（stderr 打日志 `ptr click @(x,y) hit="name"`）。

## Gates（§2 主表 + §2.2 全族 FAIL 线）

| Gate | 值 | 判定 |
|------|-----|------|
| scripted_ok == scripted_total | 8 == 8 | **硬**（§2 主表） |
| scripted_hit ≥ 4（§2.6 探针数） | 7 | 硬 |
| 覆盖：普通/圆角/变换/裁剪内/裁剪外拒绝/重叠上层/空白拒绝 | 7 类 | 硬（hit ≡ 绘身份） |
| presents ≥ 1 | ≥1 | 硬 |
| policy = full_paint | full_paint | 正确性窗（5s，不冒充 retained） |
| fps_interval ≥ 55（HOT 持续 tick） | ≥55 | 预算 AT_DEFAULT |
| p95 ≤ 22ms | ≤22 | 预算 AT_DEFAULT |
| vsync_source / fallback=0 | true / 0 | 诚实性 |
| slope_gate = off | — | **5s 正确性窗 §2.2.4 <15s 允许**（R4b/R5/R11 先例） |

## U20 实现点六维

| 维度 | 回答 |
|------|------|
| **正确性** | 命中对象 ≡ 绘制身份：8 探针中 7 命中目标 DebugName、1 空白拒绝，门禁 scripted_ok==total |
| **脏区** | 点击高亮只 MarkNeedsPaint 命中目标（局部）；HOT live paint 独立 |
| **缓存** | Hit 与 Paint 同源（同 size/offset/变换参数）；无边界缓存干扰 |
| **边界条件** | 空白区、裁剪外、负坐标（既有 TestHitTest 覆盖）不炸 |
| **失败模式** | 任一探针 miss → `FAIL: scripted_ok=n want total` + exit 1 |
| **窗内如何看出** | HIT 横幅 `hit=n/total`、HUD core 行、命中目标高亮色、Spike 相全高亮 |

## 诚实性声明

- 探针 = 逻辑坐标注入 `PipelineApp.HitTestPointer`（引擎公开命中入口，X11 指针走同一函数）。
- `scripted_ok/scripted_total/scripted_hit` 来自示例层逐探针计数（ability_extra），非拍脑袋。
- 空白/裁剪外探针期望**无命中**（空 DebugName）——拒绝方向也计数。
- 5s 短窗：slope_gate=off（§2.2.4 <15s 正确性窗）；fps/p95 预算未放宽。
- 能力层单测：`TestRenderClipRRect_HitTest*`（本次补）+ `TestRenderTransform_HitTest_*` +
  `TestHitTest_MatchesPaintOffset` + overlay `TestOverlay_HitTest_TopFirst` 等。
