# p37_mip_exit — 批 6 ④ mip 出口窗

P3.7 批 6 第 ④ 扇真窗（收台账 756：批 4 mip 档位接线主跳）。满窗即内容，指标行浮
左上（金图 mask 掉 `y<32` 带）。臂：

- **A mip 三档 × 非整数重缩小**：`sprite.AtlasSprite{Filter: Bilinear, Mipmap:
  None/Nearest/Linear}` 经 `ToRender()` 逐字段带到渲染命令，噪声图 256→96
  （2.667x，LOD≈1.42）三档并排——None 走样 / Nearest 取整档 / Linear 跨档混合，
  两两互异实测 22036/23032/23032 字节（内缩区）。**口径如实**：档位吃有效的
  前提是 bilinear 采样路——引擎采样契约（`image_pipeline.go
  EffectiveImageSamplerKey`，`game/tex/testdata/mipmap_cases.json`
  sampler_contract 冻结）明文 nearest/bicubic 图的 mipmap 开关被屏蔽（两者共用
  历史 nearest 采样器保逐位身份），本臂因此挂 Bilinear。
- **B filter 三档**：同噪声图同几何，nearest/bilinear/bicubic × MipmapNone，
  三档互异（高频噪声保证采样核分叉——8px 规则棋盘配整数 4x 时 bilinear 采样点
  恒落同格、三档数学上必等图，首版实测抓到后换 LCG 噪声，配方承接
  `ui_wr_p36_mipmap`）。
- **C 放大段**：64px 色块 →128px（2x）。中心点纹素坐标 31.75 恰跨 32/32 格
  边界：nearest 精确取纹素绿 (60,160,90)；bilinear 精确解析混合
  0.625 绿+0.375 红 = (112,126,82)；内部点 (X+95,Y+95)（s=47.25，四邻与
  bicubic 16 邻全落同格）三档全部精确纹素色。
- **D 零值老路逐位**：引擎零值精灵（FilterUnset+MipmapNone）≡ 直构 render 零值
  精灵，内缩区 diff=0——批 4「零值即历史路」的窗内版；历史路实测为 bilinear
  （384→160 中心 (880,444) 落 (221,134,95) = 纹素 0.3 亮线+0.7 格色解析混合，
  两段同公式同色）。
- **E 映射面板**：4 档 Filter × 3 档 Mipmap 全组合经 `ToRender()` 逐字段带到
  （12 枚绿牌）+ `ParseMipmap/ParseFilter` 非法输入具名拒绝。

三证据合证：10 枚离屏逻辑/像素探针 + 终帧 7 枚语义像素断言（期望色由同一份
权重公式反推，容差 2）+ 双后端金图逐位 0.0000%/921600px 且两基线逐位相同。
H15 定帧 240 停机六轮恒 0（静态内容窗，常驻相金图同 0——如实记，无墙钟相位）。
H16 两变异（临时摘 `engine/sprite/atlas.go` ToRender 的传递行一行，跑完即还原
并 `git diff` 核对净）：
① 摘 `Mipmap: s.Mipmap` ⇒ `torender_carries_all_combinations` 红 + 三档互异红
（回退全 None 态）+ **金图 1.9829%**；
② 摘 `Filter: s.Filter` ⇒ 携带红 + filter 三档互异红（三档回退等图老路）+
**金图 1.9669%**。还原后终验复绿（presents=240 probes=10/10 golden=0.0000%）。

登记口径如实：`tex.Filter↔AtlasFilter→DrawImageOptions.Mipmap` 的 renderconv
映射（批 4 明文未做、待「档位挂引擎哪个对象」拍板）**不在本窗**，探针不冒充。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/engine/p37_mip_exit -auto-only
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_mip_exit -auto-only  # 播种，仍不算过
go run ./examples/engine/p37_mip_exit -manual-seconds 6                              # 常驻人工
GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_mip_exit -auto-only       # 第二后端一轮
```

零新增公开 API——全部走批 4 交付的引擎出口
（`sprite.AtlasSprite.Filter/Mipmap`/`ToRender`/`ParseFilter/ParseMipmap`）与
渲染层现成采样面。
