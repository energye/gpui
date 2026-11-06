# p37_material — 批 6 ③ 材质窗

P3.7 批 6 第 ③ 扇真窗（收台账 747/748/749/750：批 3a premul、3b 三入口、3c 多纹理、
3d 描边洞+哨兵；补证 3c 的「uniform 载荷参与出图」= 审计缺口 2）。满窗即内容，
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
  图像口回退路 `MaterialSkips++` 计数 + gles 承载尝试（64KB 顶格 uniform 模块：
  本机两后端均编译通过——真触发不可复现于此设备，如实记；哨兵链路由双 %w 包装
  钉覆盖）。

- **F uniform 载荷参与出图**（审计缺口 2 补证）：一枚 272B uniform 的材质——
  `split`（分屏边界）落在偏移 **256B**（旧 uniform 预算之外）、`gain`（右半亮度增益）
  落在 260B，前 256B 是 `array<vec4<f32>,16>` 占位（uniform 地址空间要求数组 stride
  是 16 的倍数，写成 `array<f32,64>` naga 直接拒整个模块）。同一几何、同一对纹理、
  同一份着色器，**只换 uniform 字节**：`split` 0.25→0.75 时同一个固定像素必须从暗蓝
  翻成红；把 `split` 写 0（增益不动）时左端也必须变暗蓝——证明值真从载荷里读，
  不是着色器写死的常数（不用全零载荷：那会连增益一起归零变纯黑，与「什么都没画」不可分辨）。

三证据合证：16 枚离屏逻辑/像素探针 + 终帧语义像素断言（期望色按 premul src-over
公式由同一份常数算出；臂 F 的两个锚点在边界两侧固定不动、期望色写死，不随 `fSplit`
自算，免得改参数还「自洽绿」）+ 双后端金图逐位 0（两基线逐位相同）。H15 定帧六轮恒 0
（静态内容窗，常驻相金图同 0）。H16 四变异多重报警：摘材质挂载 3.22%、
SetImageAt 摘第二张 1.25%、拒绝臂合法化 0.035%、`fSplit` 0.25→0.60 ⇒
`uniform_payload_paints_split` + `uniform_boundary_follows_payload` 双红且金图 **0.875%**。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/engine/p37_material -auto-only
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_material -auto-only  # 播种，仍不算过
go run ./examples/engine/p37_material                                                # 常驻人工
GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_material -auto-only       # 第二后端一轮
```

零新增公开 API——全部走 render 公开材质面（`ShaderEffect`/三入口 `Material` 字段/
`SetImageAt`/哨兵族/`RenderPathStats` 计数）。
