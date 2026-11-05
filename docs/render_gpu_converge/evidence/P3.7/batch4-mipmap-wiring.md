# P3.7 批 4（mip 档位接线）· 主跳交付记录（2026-11-05）

拍板链：用户定「用 render 两枚现成枚举」+「存档词汇直接换格式」+「先解决主要问题」；
分层原则由用户重申：**渲染层只做渲染，2.5D 的东西写在引擎层**（本批全部改动都在引擎层，
`render/` 一行未动，符合依赖方向 engine→render）。

## 1 复现红（改造前的三条实测事实，不是推测）

目标态写的测试今天编译不过，报出的正是三处断点：

```
engine/sprite/p37_4_mipmap_test.go:43:6:  s.Mipmap undefined (type AtlasSprite has no field or method Mipmap)
engine/sprite/p37_4_mipmap_test.go:24:38: cannot use render.InterpBilinear … as AtlasFilter value in argument to NewAtlasSprite
engine/sprite/p37_4_mipmap_test.go:116:15: undefined: ParseFilter
```

配套 grep 实测：`tex.Filter.SamplerParams()` 生产消费者 0（只有定义 + `examples/engine/tex` 两处自检）；
`AtlasSprite.ToRender()` 逐字段转 render 时从不填 `Mipmap`；`engine/renderconv/conv.go` 只有
Vec2/Color/Mat2D 六组转换，没有任何 Filter/Sampler 转换（而 `engine/camera/project.go:20-21`
自陈"render 类型都在 renderconv 转一次"）。

## 2 做了什么（主跳：精灵能带档位 → 渲染命令收到档位）

- `AtlasSprite.Filter`：`AtlasFilter` → **`render.InterpolationMode`**（引擎不再自造第三套过滤枚举）。
  三档 1:1 保留：nearest/bilinear/bicubic —— **bicubic 是活能力**，有专档逐位断言
  （`testdata/atlas_cases.json` 的 `filter_bicubic_identity`），不许随枚举合一被裁。
- `AtlasSprite.Mipmap`：**新增** `render.MipmapMode` 承载（P3.6 批 2 在渲染侧加的那枚字段，引擎侧此前无对应）。
- `ToRender()` 逐字段带上 `Mipmap`；`Validate()` 对非法过滤与非法档位一律具名拒绝
  （旧 `AtlasFilter(99)` 的纪律不变）。
- 词汇随迁不裁剪：`ParseAtlasFilter`→`ParseFilter`（`""/default/nearest/bilinear/bicubic` 原样可用，
  稳定小写名走 `FilterName`），新增 `ParseMipmap`/`MipmapName`（`none/nearest/linear`）。
  旧钉里 `gotF.String()` 那处自动改到了渲染层枚举的大写名上（"Bicubic"），已改回 `FilterName` ——
  这是随迁时容易悄悄丢掉的能力，红过一次才发现。
- 接口全量切换制（不留垫片）：`NewAtlasSprite` 形参换型，调用方同笔改完 ——
  `engine/sprite/atlas_test.go`、`examples/engine/sprite/main.go`、`examples/engine/particle/trail.go`、
  `engine/sprite/doc.go` 冻结登记同步。

## 3 零值语义（这批最容易踩错的地方，写死）

`mipFixture` 第一次写成显式 `render.InterpBilinear` 却断言"零值路"，第二次又把零值断成
"引擎归一为 Bilinear(2)"——两次都是钉写歪，实测把两种写法都跑过才定准：

- 改造前 `AtlasFilterDefault(0)` 走 `ToRender()` 返回的也是 **`render.InterpolationMode(0)`**，
  由渲染层 `normalizeAtlasFilter` 归一为 Bilinear；所以**引擎必须原样透传 0**，提前改判成 2
  等于把归一判据搬到引擎侧，动了老路。钉 2 现断言「零值透传」+「显式 Bilinear(2) 各自原样过去」。
- `Mipmap` 零值 = `MipmapNone`（渲染层注释明写：单档历史路逐位不变）；**不能**经
  `tex.Filter.Normalized()` 那条路拿档位——它会把未指定映射成 `MipNearest`，那样一接线
  所有存量精灵的采样键都会漂。这也是本批不引入 `renderconv.FilterToRender(tex.Filter)` 的原因之一。

## 4 负向（摘掉必红，全部实测）

| 号 | 打断点 | 红象 |
|---|---|---|
| N13 | 摘 `ToRender()` 里 `Mipmap: s.Mipmap` 一行 | 钉 1：`档位没传到渲染命令：0 want 1` / `0 want 2`（nearest/linear 两子档各红一次） |
| N14 | 摘 `Validate()` 的 `validMipmap` 块 | 新钉 4「非法档位值被放过了」+ 旧边测 `atlas_test.go:410`「bad mipmap ToRender err = <nil>」双红 |

还原后 `engine/sprite` 全包绿。

## 5 门禁

- 逐文件（触及四包，带 `WGPU_NATIVE_PATH`）：`engine/sprite` 7 文件、`engine/tex` 3、`engine/renderconv` 1、
  `engine/camera` 6 —— **16 个测试文件全 PASS**；`go test ./engine/...` 24 包全 ok。
- `go build ./...` rc=0；`go vet ./engine/sprite ./engine/tex ./engine/renderconv ./examples/engine/sprite
  ./examples/engine/particle` rc=0；`gofmt` 净。
- `CGO_ENABLED=1 go test -race ./engine/sprite ./engine/renderconv` ok。
- 渲染侧未受影响（依赖方向 engine→render，`render/` 零改动）：`render/scene`、`render/recording` 全 ok。
  整包 `go test ./render` 仍红在 `s3c_m3_residual_gate_test.go`（"FlushGPU: gg: falling back to CPU"）——
  该文件**逐文件跑 PASS、批 3 sweep 记录也是 PASS**，属已登记的"整包与逐文件口径差异"（M4 同族），非新增红。
- `go run ./scripts/apidoc` rc=0（引擎公开面不在 render 目录范围内，同步落在 `engine/sprite/doc.go` 冻结登记）。

## 6 本批未做（如实记，不是漏）

1. **`renderconv.FilterToRender(tex.Filter)` 不加**：`engine/tex` 里没有任何持档位的对象
   （包里只有 `Image/Stream/Table/State/Format/Filter`，**无 `Texture` 结构体**；`SetFilter/SetMipmap`
   是 `*Filter` 自己的方法，全仓调用者只有 `examples/engine/tex`），先造转换函数=造无人调用的公开口，
   且 `tex.Filter.Normalized()` 的零值语义与渲染层的单档历史路不兼容（见 §3）。
   **待拍板**：档位归属（挂 `tex.Image`/`Stream`？挂精灵作者配置？）——用户已定「归引擎层」，
   但引擎内挂在哪个对象未定。
2. **aniso 不进 sprite**：渲染层 `AtlasSprite` 没有各向异性字段（只有单图路 `SetPerImageFilter(mip, maxAniso)`
   有），sprite 上放 `MaxAniso` 就是承诺不存在的能力。
3. **`testdata/atlas_cases.json` 的 `mip` 字段本轮不加**：词汇与解析已由钉 5 覆盖；数据子集随批 6
   ④ mip 出口窗一并落盘（避免造一份"被解析但从不参与画面"的数据）。
4. **端到端像素收口**：本批状态按 §4.1 两态口径为 **☑ 已交付待金图（收口批=批 6 ④ mip 出口窗）**——
   引擎侧仍无生产调用者设档位（与批 1–3 各步同属"已验证待接线"），且没有窗画过"档位差异画面"。

## 7 与标准的对照与一处真实差异（本批新发现，登记）

**分层归属按仓内既有取证表执行**（`docs/RENDER_2_5D_GAP_PLAN.md`）：G01 `CanvasItem`
（`scene/main/canvas_item.h`）把 `texture_filter`/`texture_repeat`/`set_material`/`z_index` 都算
**场景 item 的属性**，归属列写的就是 `engine/sprite` + `engine/world`；G18
`RendererCanvasRender` 的命令口只带 `texture_filter/repeat 默认值`，归属「render 新分支口径」；
G21 采样器（`rendering_server_enums.h`/`canvas.glsl`）的 `TextureFilter` 六档含各向异性，归属
`engine/tex/mipmap.go` + render 采样开关 ⇒ 本批「档位记在精灵上、渲染层只收参数、aniso 不上图集 item」
三条正是这张表的分工。外部同构：Flutter 的 `filterQuality` 挂在 `widgets/image.dart`（默认
`FilterQuality.medium`）由 `dart:ui` 绘制参数接收，画质不由引擎后端自行决定。

**差异（如实记，不当已解决）**：Godot 的 `TextureFilter` 是**一枚六档枚举**把「最近/线性 × 是否
mipmap × 各向异性」打包，并含 `INHERIT`/`ancestor_with_mipmaps` 一类**继承父节点**的取值；
本引擎拆成两枚正交枚举（`InterpolationMode` + `MipmapMode`，形状近 Skia `SkSamplingOptions` 的
filter/mipmap/aniso 分解），因此**没有"父级默认档位"这层语义**——`AtlasSprite` 是绘制项、
不是场景节点，没有可继承的父链。将来要补，按分层规矩应落在 **`engine/world` 的节点层**（节点持默认档、
精灵字段表示"显式覆盖"），不得下沉到 `render/`。**归属与继承一起待拍板**（用户 2026-11-05 定：
先登记不阻塞，批 5 继续）。

