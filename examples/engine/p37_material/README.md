# p37_material — 批 6 ③ 材质窗

P3.7 批 6 第 ③ 扇真窗（收台账 747/748/749/750：批 3a premul、3b 三入口、3c 多纹理、
3d 描边洞+哨兵；补证 3c 的「uniform 载荷参与出图」= 审计缺口 2，兑现 3d 的
「后端真触发 + fallback 语义」= 审计缺口 3 的填充腿）。满窗即内容，
指标行浮左上（金图 mask 掉 `y<32` 带）。臂：

- **A 三入口材质**：图像口 `DrawImageOptions.Material`、quad 口
  `QuadDrawOptions.Material`（仿射）、批绘口 `AtlasSprite.Material` 各挂同一枚 uv
  渐变 shader——挂材质与不挂并排；**恒等材质（TEXTURE 原样返回）≡ 无材质逐位
  diff=0**（内缩区域字节对账）。
- **B premul 半透明填充**：白底 + premul 半透明红 shader（alpha=0.5）→ 实测
  `(255,128,128)`（批 3a premul 修复、p32 d9 的窗内版）。
- **C SetImageAt 双纹理**：`ShaderEffect` 双纹理对（binding 1/2+3/4），slot1
  `SetImageAt` 绑第二张 → 左半主图红、右半第二张蓝（批 3c D4 的窗内版）。
- **D 描边吃 shader**：`SetRGB+SetShader` 的 AA 描边（轮廓展开 → fillShaderAsImage，
  批 3d 单点修复）——描边像素是 shader 渐变而非环境实心色（两端 Δ 断言）。
- **E 拒绝态底带**：七哨兵六枚前端具名（errors.Is）绿牌 + 透视+材质的
  `ErrQuadMaterialUnsupported` 现场拒绝（校验期具名返回、拒绝区保持底色）+
  图像口回退路 `MaterialSkips++` 计数。~~gles 承载尝试~~：E 臂当年的「64KB 顶格
  uniform 模块」在本机两后端都编译通过，**没兑现设计钉的「后端真触发 + fallback
  画面正确」**（如实收回，不当已收口）——真触发子现在由 **G 臂**兑现。

- **F uniform 载荷参与出图**（审计缺口 2 补证）：一枚 272B uniform 的材质——
  `split`（分屏边界）落在偏移 **256B**（旧 uniform 预算之外）、`gain`（右半亮度增益）
  落在 260B，前 256B 是 `array<vec4<f32>,16>` 占位（uniform 地址空间要求数组 stride
  是 16 的倍数，写成 `array<f32,64>` naga 直接拒整个模块）。同一几何、同一对纹理、
  同一份着色器，**只换 uniform 字节**：`split` 0.25→0.75 时同一个固定像素必须从暗蓝
  翻成红；把 `split` 写 0（增益不动）时左端也必须变暗蓝——证明值真从载荷里读，
  不是着色器写死的常数（不用全零载荷：那会连增益一起归零变纯黑，与「什么都没画」不可分辨）。

- **G 后端拒收「看得见拒绝」三格**（审计缺口 3 的设计子项，填充腿）：故意一枚
  **前端放行、后端 naga 必拒**的模块（uniform 地址空间数组 stride 非 16 倍数：
  `array<f32,64>`），挂填充腿画中间那格 ⇒ 该格**必须保持底色**（环境实心红不许冒充
  回退）、`ShaderBackendFails++`、`CPUFallbackOps++`；左右两格是「这块地方本来就
  画得出东西」的对照（无材质红 / 合法材质绿 (51,229,102)）。
  **两端行为不同，两边各自钉死**（写死任一端都会把另一端判假）：
  | 后端 | 这枚模块 | 格 2 实测 | 呈现帧计数 |
  |---|---|---|---|
  | native（Vulkan / wgpu-native） | naga 拒 | 底色 (14,14,19) | `ShaderBackendFails=239`（每帧一次） |
  | go（纯 Go GL） | 其 WGSL 前端不查 uniform stride 规则 ⇒ **接受并出图** | shader 解析色 (128,130,128) | 0 |
  终帧硬门 `arm_g_frame_check` 就钉这个二选一：**「底色 + 计数 ≥1」或「shader 解析色
  + 计数 0」**，两者都不许是环境实心红，也不许「拒而不计数」（=静默消失）。
  ⚠ 为什么必须看终帧而不是离屏探针：离屏 `render.NewContext` 走独立设备，而独立设备
  按设计**不吃 `GPUI_BACKEND`**（`render/device_open.go` 的 `OpenStandaloneDevice` 注记：
  纯 Go GL 没有 surfaceless 独立设备）⇒ 离屏那三枚 `standalone_leg_*` 探针两后端都在
  Vulkan 上跑，只量独立腿。
  ⚠ 同一枚模块挂在**图像口/quad 口**上不走具名链——直接把 flush 打断、计数为 0，
  那是已登记的 **H23**（`docs/render_gpu_converge/evidence/P3.7/batch6/gap3-backend-reject-probe.txt`），
  本窗只声明填充腿，不替它遮。
  ⚠ 每帧挂一枚必拒模块 ⇒ 后端每帧重新尝试编译（无负面缓存）+ 每帧一整段 stderr，
  实测 p95 帧间隔 23.4ms → 51.1ms；**本窗门禁相墙钟要 `RUN_SECONDS=20`** 才跑得满
  定帧 240（8 秒只能到 ~193 帧，会被 H15 停机门判红——这是被测行为的代价，不是门禁放水）。

三证据合证：20 枚离屏逻辑/像素探针 + 终帧语义像素断言 12 枚（期望色按 premul src-over
公式由同一份常数算出；臂 F 的两个锚点在边界两侧固定不动、期望色写死，不随 `fSplit`
自算，免得改参数还「自洽绿」；臂 G 只放两枚**两端都为真**的对照锚，格 2 交给上面的
二选一终帧门）+ 双后端金图逐位 0。**两基线自臂 G 起不再逐位相同**（这就是两端分歧的
画面证据）：格 2 整格 7800 px（底色 vs shader 解析色）+ 底带 8 枚色片边缘 992 px
（每行 32 px×18 行，1 级 AA 取整差，色片中心色两端逐一相同 ⇒ 状态无差）；
指标带 y<32 不进比对。H15 定帧六轮恒 0（静态内容窗，常驻相金图同 0）。
H16 变异五处、各有重报警：摘材质挂载 3.22%、SetImageAt 摘第二张 1.25%、
拒绝臂合法化 0.035%、`fSplit` 0.25→0.60（双探针红 + 金图 **0.875%**）、
**臂 G 的拒收模块换成合法模块**（`standalone_leg_reject_counted` +
`standalone_leg_reject_region_keeps_backdrop` + 终帧 `arm_g_frame_check` 三处红；
金图 native **0.9540%** / go **0.8464%**）。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=20 go run ./examples/engine/p37_material -auto-only
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=20 go run ./examples/engine/p37_material -auto-only  # 播种，仍不算过
go run ./examples/engine/p37_material                                                # 常驻人工
GPUI_BACKEND=go RUN_SECONDS=20 go run ./examples/engine/p37_material -auto-only       # 第二后端一轮
```

墙钟为什么是 20 秒：臂 G 每帧挂一枚后端必拒的模块，每帧重新尝试编译（无负面缓存），
p95 帧间隔 23.4→51.1ms；`RUN_SECONDS=8` 只能跑到 ~193 presents，会被 H15 的定帧停机门判红。

零新增公开 API——全部走 render 公开材质面（`ShaderEffect`/三入口 `Material` 字段/
`SetImageAt`/哨兵族/`RenderPathStats` 计数）。
