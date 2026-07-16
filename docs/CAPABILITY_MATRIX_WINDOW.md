# 能力矩阵窗口验收（examples/capability_matrix）

> 版本：1.0 | 日期：2026-07-16  
> 真源：`docs/SKIA_2D_CAPABILITY_MATRIX.md`（Skia 2D 语义 ID）  
> 实现：`examples/capability_matrix/`（真实 X11 窗口 + webgpu → render 呈现链）

## 1. 目标

在 **真窗口 Present** 路径上，按 Skia 2D 能力矩阵 ID 分组验收：

| 验收层 | 内容 |
|--------|------|
| 调用链 | `render.Context` → `gpu/webgpu` → `gpu/rwgpu` → `libwgpu_native` → swapchain present |
| 正确性 | `cpu_fallback_ops=0`、`GPUOps>0`、场景 probe 通过 |
| 性能 | 默认目标 **60fps**（EMA ≥ 55，avg ≥ 48，除非 AllowLowFPS） |
| 内存 | 稳态 RSS 斜率门禁（不把系统总内存当指标） |
| 可视 | 中文 HUD 标注「应看到」的内容，便于人工辨认 |

**不是**：把 `mem_anim_window` 的 S01–S23 复述一遍。  
**是**：按矩阵 `S/T/P/G/C/D/B/L/I/X/F/V/H/E` 行做窗口级对标探针。

## 2. 能力测试指标（门禁）

| 指标 | 条件 | 失败含义 |
|------|------|----------|
| `frames` | ≥ 30 | 跑太短，无统计意义 |
| `cpu_fallback_ops` | **= 0** | 走了 CPU 回退，非 GPU-first |
| `gpu_ops` | **> 0** | 本帧未记 GPU 绘制 |
| `probe_ok` | true | 场景级路径统计失败 |
| `fps_ema` | ≥ target−5（默认 55） | 稳态帧率不足 |
| `fps_avg` | ≥ target−12（默认 48） | 平均帧率不足 |
| `rss_steady_delta_kb` | ≤ 512×1024 | 稳态内存异常爬升 |

结果 JSON：`GPUI_RESULT_FILE`；单行摘要：`*.line`。

判定函数：`examples/capability_matrix/metrics.go` → `judgeResult`。

## 3. 场景表 C01–C20（对齐矩阵）

| ID | 名称 | MatrixIDs（Skia 矩阵） | 应看到 |
|----|------|------------------------|--------|
| C01 | 窗口呈现/清屏 | S.03,S.04,S.05 | 清屏色相变化 + 移动圆 |
| C02 | 变换栈 | T.01,T.02,P.01 | 旋转缩放方块 + 独立圆 |
| C03 | 路径填充+描边 | H.01,G.01,G.02,G.04,P.02,P.03,P.05,P.06 | 波浪线 + Cap 三线 + 星 |
| C04 | Hairline+虚线 | P.04,E.01,P.08 | 虚线贝塞尔 + 脉动圆 |
| C05 | 裁剪 | C.01,C.02,C.05,G.03 | 条纹底 + 圆角/矩形裁剪窗 |
| C06 | 渐变+图案 | D.01,D.02,D.03,D.05 | 线性/径向/扫描 + pattern |
| C07 | 可分离混合 | B.03,B.05,B.01 | Multiply/Screen/Overlay |
| C08 | 半透明图层 | L.01,L.02,L.03 | 双圆 + 半透明层卡片 |
| C09 | 贴图+写像素 | I.01,I.02,I.03,S.07 | 棋盘图 + WritePixels |
| C10 | 中英文+装饰 | X.01,X.02,X.06,X.08 | 中英文本 + 下划线 |
| C11 | 滤镜 | F.01,F.02,F.04 | 模糊/投影/灰度瓦片 |
| C12 | 顶点网格 | V.01,V.03 | 彩色网格起伏 |
| C13 | EvenOdd | H.03,H.01 | 空心环 vs 实心对比 |
| C14 | 蒙版层 | L.06 | 圆形 mask 内内容 |
| C15 | Backdrop | L.05,F.01 | 半透明背景采样卡片 |
| C16 | Damage Present | S.09 | 局部 dirty 动画 |
| C17 | 高级混合 | B.03,B.04,B.07 | 多 blend 模式网格 |
| C18 | LCD 文本 | X.04,X.05,X.02 | GlyphMask/LCD/Aliased |
| C19 | 独立圆角 | G.06,G.03 | XY 半径不同的 rrect |
| C20 | 多能力合成 | S.03,T.01,P.01,G.01,C.01,D.01,L.03,I.01,X.02,V.01 | 同屏组合 |

> 说明：C01–C20 是 **M0–M3 高频窗口探针分组**，不是矩阵每一行单独一场景。缺口（如 E.02/E.03、X.09/X.10、I.07、R.02、真 multiplanar YUV）后续可增 C21+。

## 4. 运行

```bash
cd /home/yanghy/app/projects/gogpu/gpui
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:${LD_LIBRARY_PATH}
export DISPLAY=:0

go build -o /tmp/capability_matrix ./examples/capability_matrix

# 单场景（自动化）
GPUI_SCENARIO=C01 GPUI_ANIM_SECONDS=12 GPUI_RESULT_FILE=/tmp/cap_C01.json \
  /tmp/capability_matrix

# 顺序跑 C01–C20
for id in C01 C02 C03 C04 C05 C06 C07 C08 C09 C10 C11 C12 C13 C14 C15 C16 C17 C18 C19 C20; do
  GPUI_SCENARIO=$id GPUI_ANIM_SECONDS=12 GPUI_RESULT_FILE=/tmp/cap_${id}.json \
    /tmp/capability_matrix || echo FAIL $id
done
```

环境变量：

| 变量 | 含义 | 默认 |
|------|------|------|
| `GPUI_SCENARIO` | C01–C20 | C01 |
| `GPUI_ANIM_SECONDS` | 自动退出秒数；0=手动关窗 | 0 |
| `GPUI_TARGET_FPS` | 目标帧率 | 60 |
| `GPUI_RESULT_FILE` | JSON 结果路径 | 空 |
| `GPUI_ANIM_LOG_EVERY` | 日志帧间隔 | 60 |

## 5. 与其它产物关系

| 产物 | 角色 |
|------|------|
| `docs/SKIA_2D_CAPABILITY_MATRIX.md` | Skia 能力真源与层状态 |
| `render/TestP1_Capability_*` | 离屏像素/语义门禁 |
| `examples/capability_matrix` | **真窗口** 分组探针（本文件） |
| `examples/mem_anim_window` | 性能/闪烁/内存 soak（S01–S23），非矩阵 ID |

## 6. 设计约束

1. **GPU-first**：`cpu_fallback_ops>0` 直接 FAIL。  
2. **一层一进程**：禁止进程内轮换场景。  
3. **Layer/Filter/Backdrop**：小离屏 RT → 真 API → `ExportImageBuf` → `DrawImage`（与 Skia saveLayer 有界层一致；避免整窗 Apply 卡顿/闪）。  
4. **中英混排字体**：`MultiFace(DejaVuSans + DroidSansFallback)`。  
   DroidSansFallback 对 `A-Z/a-z/0-9` 的 `HasGlyph=false`，仅用 CJK 面会只出中文、英文空白（已复现 ink=0）。  
   必须 Latin 主脸 + CJK fallback（对齐矩阵 X.06）。避免 CFF Noto CJK 作主路径。  
5. **未全绿前不宣称 Skia 完整**。

## 7. 验收证据（自动）

> 运行目录: `/tmp/capability_matrix_evidence` | 二进制: `/tmp/capability_matrix`  
> 门禁: `cpu_fallback_ops=0` + `gpu_ops>0` + `probe_ok` + FPS≈60

```
scenario=C01 status=PASS fps_ema=62.1 fps_avg=61.7 cpu=9 cpu_fb=0 gpu_ops=8892 probe=true reason=
scenario=C02 status=PASS fps_ema=61.2 fps_avg=61.0 cpu=9 cpu_fb=0 gpu_ops=8296 probe=true reason=
scenario=C03 status=PASS fps_ema=60.9 fps_avg=60.8 cpu=11 cpu_fb=0 gpu_ops=11201 probe=true reason=
scenario=C04 status=PASS fps_ema=61.1 fps_avg=60.8 cpu=10 cpu_fb=0 gpu_ops=7792 probe=true reason=
scenario=C05 status=PASS fps_ema=63.7 fps_avg=61.4 cpu=8 cpu_fb=0 gpu_ops=16728 probe=true reason=
scenario=C06 status=PASS fps_ema=530.9 fps_avg=58.1 cpu=51 cpu_fb=0 gpu_ops=11160 probe=true reason=
scenario=C07 status=PASS fps_ema=58.5 fps_avg=56.1 cpu=96 cpu_fb=0 gpu_ops=6735 probe=true reason=
scenario=C08 status=PASS fps_ema=63.1 fps_avg=60.8 cpu=32 cpu_fb=0 gpu_ops=9253 probe=true reason=
scenario=C09 status=PASS fps_ema=62.2 fps_avg=60.9 cpu=10 cpu_fb=0 gpu_ops=11712 probe=true reason=
scenario=C10 status=PASS fps_ema=62.5 fps_avg=60.9 cpu=10 cpu_fb=0 gpu_ops=13664 probe=true reason=
scenario=C11 status=PASS fps_ema=71.8 fps_avg=60.9 cpu=25 cpu_fb=0 gpu_ops=11224 probe=true reason=
scenario=C12 status=PASS fps_ema=61.4 fps_avg=60.9 cpu=17 cpu_fb=0 gpu_ops=7808 probe=true reason=
scenario=C13 status=PASS fps_ema=61.7 fps_avg=61.2 cpu=9 cpu_fb=0 gpu_ops=10290 probe=true reason=
scenario=C14 status=PASS fps_ema=61.8 fps_avg=61.0 cpu=42 cpu_fb=0 gpu_ops=9272 probe=true reason=
scenario=C15 status=PASS fps_ema=61.2 fps_avg=60.9 cpu=52 cpu_fb=0 gpu_ops=12200 probe=true reason=
scenario=C16 status=PASS fps_ema=62.2 fps_avg=61.7 cpu=9 cpu_fb=0 gpu_ops=8892 probe=true reason=
scenario=C17 status=PASS fps_ema=62.6 fps_avg=60.2 cpu=86 cpu_fb=0 gpu_ops=7230 probe=true reason=
scenario=C18 status=PASS fps_ema=410.1 fps_avg=60.5 cpu=51 cpu_fb=0 gpu_ops=6292 probe=true reason=
scenario=C19 status=PASS fps_ema=63.0 fps_avg=60.6 cpu=11 cpu_fb=0 gpu_ops=11664 probe=true reason=
scenario=C20 status=PASS fps_ema=64.2 fps_avg=61.5 cpu=16 cpu_fb=0 gpu_ops=15776 probe=true reason=
```

汇总: **pass=20 fail=0**（全部场景 PASS）

