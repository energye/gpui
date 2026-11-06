# p37_material — 批 6 ③ 材质窗

P3.7 批 6 第 ③ 扇真窗（收台账 747/748/749/750：批 3a premul、3b 三入口、3c 多纹理、
3d 描边洞+哨兵）。满窗即内容，指标行浮左上（金图 mask 掉 `y<32` 带）。臂：

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

三证据合证：13 枚离屏逻辑/像素探针 + 终帧语义像素断言（期望色按 premul src-over
公式由同一份常数算出）+ 双后端金图逐位 0（两基线逐位相同）。H15 定帧六轮恒 0
（静态内容窗，常驻相金图同 0）。H16 三变异三重报警：摘材质挂载 3.22%、
SetImageAt 摘第二张 1.25%、拒绝臂合法化 0.035%。

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
