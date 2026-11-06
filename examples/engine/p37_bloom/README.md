# p37_bloom — 批 6 ⑤ 泛光窗

P3.7 批 6 第 ⑤ 扇真窗（收台账批 5 步骤 1/步骤 2 两行：CPU 竖条 + GPU 四 pass）。
满窗即内容，指标行浮左上（金图 mask 掉 `y<32` 带）。臂：

- **A 扫参五面板**：同一份亮部场景（暗底 15,15,22 + 亮黄块 L≈0.86 + 近白块
  L≈0.96 + 中灰块 L≈0.41）× `Context.ApplyBloom(BloomParams)`——①恒等（不调用）
  /②基准 T0.60 σ8 I0.60 /③阈值组 T0.15 /④半径组 σ16（域顶，σ12 实测远环只差
  +3.9 太薄后抬的，如实记）/⑤强度组 I1.60。泛光晕像素断言：基准缘外 8px 被点亮
  （vs 恒等同点 ≥+8）；阈值组把基准下 ramp=0 的中灰块点亮（≥+8）；半径组点亮
  基准≈零的 20px 远环（≥+5）；强度组近环比基准更亮（≥+8）；五面板两两互异
  （10 对全 ≥200 字节）。GPU 腿实测 `gpu_ops=5 / cpu_fallback=0`（四 pass 链真走
  GPU，报数不判绿）。
- **B 恒等参数逐位**：真调 `ApplyBloom{Intensity:0}` 后的画面 ≡ 完全不调用**逐位
  diff=0**（门口短路 `filter_ops_cpu.go:532` 语义的窗内版）。
- **C LDR 三处声明**（JSON `ldr_declarations` + 底带四行可见）：① 本窗四 pass 走
  RGBA8 LDR 近似（真 float 全链=在册 M5，不冒充 HDR）；② `render/hdr` GPU 侧未接
  ——`FloatTarget.TextureView()` 恒 nil（探针实测）；③ `engine/fx` CPU 泛光出口
  clamp01=LDR（探针实测超亮输入 R=2 出被钳 ≤1）。
- 参数域具名拒绝四枚（Threshold>1 / σ>16 / Intensity>8 / NaN）+ 零值合法 +
  `IsIdentity` 语义（I=0 真、I=0.001 假）。

**口径如实**：Canvas/Recorder 录制面没有 ApplyBloom 口（批 5 余账登记「要接另拍」，
窗内生产每帧 DC 是 Recorder）——五面板在离屏 `*render.Context` 上画场景+真调
`ApplyBloom`（生产级 GPU 四 pass 被真调用），结果以 **1:1 整数位 DrawImage** 贴回
窗（scale=1 ⇒ 采样点落纹素中心、任何采样核恒等）；恒等面板同时是语义像素的逐位锚
（四色块精确色 ±2）。泛光画面进金图；能力面缺口本身已登记，不是绕洞。
fps/p95 **只报数不判绿**（批 6 硬要求 §3-5：性能门重定标未做）。

三证据合证：11 枚离屏逻辑/像素探针 + 终帧 4 枚语义像素断言 + 双后端金图逐位
0.0000%/921600px 且两基线逐位相同（两端 panel 互异数字全同、gpu_ops=5/cpu_fb=0
同值）。H15 定帧 240 六轮恒 0（presents 240/241；静态内容窗，常驻相 354 presents
金图同 0——如实记）。H16 两变异：① `refIntensity 0.6→0`（泛光消失回恒等帧）⇒
五枚探针红（含 p0↔p1=0 直证全帧复现恒等）+ **金图 8.4939%**；② `refThreshold
0.6→0.05`（全画面泛光）⇒ 阈值组关系翻转红 + **金图 8.1050%**；还原后终验复绿。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/engine/p37_bloom -auto-only
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_bloom -auto-only  # 播种，仍不算过
go run ./examples/engine/p37_bloom -manual-seconds 6                              # 常驻人工
GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_bloom -auto-only       # 第二后端一轮
```

零新增公开 API——全部走批 5 交付面（`render.BloomParams`/`Context.ApplyBloom`/
`hdr.FloatTarget`/`fx.NewBloom`）。
