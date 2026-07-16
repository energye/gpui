# 能力矩阵窗口验收（examples/capability_matrix）

> 版本：1.2 | 日期：2026-07-16  
> 真源：`docs/SKIA_2D_CAPABILITY_MATRIX.md`（Skia 2D 语义 ID）  
> 实现：`examples/capability_matrix/`（真实 X11 窗口 + webgpu → render 呈现链）

## 0. 「2D 画布 100%」范围声明（含 R.02）

| 声明 | 内容 |
|------|------|
| **在范围内** | Skia **2D 画布**语义：Surface/Canvas/Paint/Path/Image/Text/Layer/Filter/Vertices/Clip/Blend/Shader 等可绘制与 present |
| **明确不在范围内** | **R.02 PDF / SVG document 后端**（`SkDocument` / SVG canvas 导出） |
| **结论** | 本仓库宣称的 **「2D 画布 100%」= 矩阵画布能力完整，不含 document 导出**。R.02 永久后置或另开「文档后端」专项，**不阻塞**画布 100% 验收。 |
| **若未来要做 R.02** | 单独里程碑 `DOC.1`（PDF/SVG），与 C 系列窗口矩阵、画布 100% 解耦。 |

矩阵表内 R.02 可继续标 ⬜；验收话术使用「**2D 画布 100%（排除 document）**」，禁止在 R.02 未做时写「全 Skia 含 PDF/SVG 完整」。

---

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

### 1.1 完成度层级（本文件跟踪）

| 层级 | 含义 | 当前（2026-07-16） |
|------|------|-------------------|
| **L0** | 高频窗口基线 C01–C20 | ✅ 20/20 PASS |
| **L1** | 语义硬缺口关闭（工作项 1） | ✅ P1 B.07/B.06 关闭 |
| **L2** | 窗口 ID 覆盖扩展 C21+（工作项 3） | ⬜ 待做 |
| **L3** | 质量全管线（工作项 4） | ⬜ 待做 |
| **L4** | 像素/性能对标 Skia（工作项 5） | ⬜ 待做 |
| **L5 画布 100%** | L1+L2+L3 绿 + 范围声明排除 R.02 | ⬜ 目标态 |
| **L6 document** | R.02 PDF/SVG（**非画布 100% 必选项**） | 🚫 本阶段不做 |

---

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

L4 起追加（见 §8.5）：像素 RMSE/SSIM、与参考图差分；不得替代 L0–L3 的 GPU-first 门禁。

---

## 3. 场景表 C01–C20（对齐矩阵 · 已绿基线）

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
| C12 | 顶点网格 | V.01,V.03 | 高密度彩色网格平滑起伏 |
| C13 | EvenOdd | H.03,H.01 | 空心环 vs 实心对比 |
| C14 | 蒙版层 | L.06 | 圆形 mask 内内容 |
| C15 | Backdrop | L.05,F.01 | 半透明背景采样卡片 |
| C16 | Damage Present | S.09 | 局部 dirty 动画 |
| C17 | 高级混合 | B.03,B.04,B.07 | 多 blend 模式网格 |
| C18 | LCD 文本 | X.04,X.05,X.02 | GlyphMask/LCD/Aliased |
| C19 | 独立圆角 | G.06,G.03 | XY 半径不同的 rrect |
| C20 | 多能力合成 | S.03,T.01,P.01,G.01,C.01,D.01,L.03,I.01,X.02,V.01 | 同屏组合 |

> C01–C20 = **L0 高频窗口基线**。全表约 97 行，窗口已挂 ~53 MatrixID；其余见 §8.3 C21+。

---

## 4. 运行

```bash
cd /home/yanghy/app/projects/gogpu/gpui
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:${LD_LIBRARY_PATH}
export DISPLAY=:0

go build -o /tmp/capability_matrix ./examples/capability_matrix

# 单场景
GPUI_SCENARIO=C01 GPUI_ANIM_SECONDS=12 GPUI_RESULT_FILE=/tmp/cap_C01.json \
  /tmp/capability_matrix

# C01–C20
./scripts/run_capability_matrix.sh
# 或 for id in C01 … C20; do GPUI_SCENARIO=$id …; done
```

| 变量 | 含义 | 默认 |
|------|------|------|
| `GPUI_SCENARIO` | C01–C20（后续 C21+） | C01 |
| `GPUI_ANIM_SECONDS` | 自动退出秒数；0=手动关窗 | 0 |
| `GPUI_TARGET_FPS` | 目标帧率 | 60 |
| `GPUI_RESULT_FILE` | JSON 结果路径 | 空 |
| `GPUI_ANIM_LOG_EVERY` | 日志帧间隔 | 60 |

---

## 5. 与其它产物关系

| 产物 | 角色 |
|------|------|
| `docs/SKIA_2D_CAPABILITY_MATRIX.md` | Skia 能力真源与层状态 |
| `render/TestP1_Capability_*` | 离屏像素/语义门禁 |
| `examples/capability_matrix` | **真窗口** 分组探针（本文件） |
| `examples/mem_anim_window` | 性能/闪烁/内存 soak（S01–S23），非矩阵 ID |

---

## 6. 设计约束

1. **GPU-first**：`cpu_fallback_ops>0` 直接 FAIL。  
2. **一层一进程**：禁止进程内轮换场景。  
3. **Layer/Filter/Backdrop**：小离屏 RT → 真 API → `ExportImageBuf` → `DrawImage`。  
4. **中英混排字体**：`MultiFace(DejaVuSans + DroidSansFallback)`（X.06）。  
5. **未达 L5 前不宣称「2D 画布 100%」**；永不因 R.02 未做而卡住画布 100%。  
6. 解决问题时 **全场景对标 Skia 画布语义**（非玩具单点补丁）。

---

## 7. 验收证据（L0 · C01–C20）

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

汇总: **pass=20 fail=0**（L0 全部 PASS）

---

## 8. 迈向「2D 画布 100%」的五项工作（主计划）

下列 1–5 为 **画布 100%（排除 document）** 的必做路径；顺序即实现优先级。

### 8.1 工作项 1 — 语义硬缺口（render / 单测 + 窗口复验）

| 子项 | MatrixID | 现状 | 目标 | 验收 |
|------|----------|------|------|------|
| 1.1 Modulate 混合 | B.07 | ✅ GPU fixed-function `BlendModulate`（Src×Dst） | GPU 像素 + `cpu_fb=0` | `TestP12GPUFixedPixel_BlendModulate`；C17 探针含 Modulate |
| 1.2 paint 全局 alpha 预乘 | B.06 | ✅ solid/image/layer/text 多路径门禁 | 预乘 SO 一致 | `TestP1_Capability_B06_PaintAlpha` + `...MultiPath`；C07/C08 回归 |
| 1.3 multiplanar YUV（可选但列计划） | I.08 | external texture 子集 | 真 multiplanar YUV 采样绘制 | 离屏 + 可选 C 场景；**无视频业务可标 defer 并写进矩阵脚注** |
| 1.4 其它脚注清零 | V.03 自定义 FS、K.02 scene multi-draw | 子集 | 按需：先文档标注「画布 100% 子集边界」再实现 | 不默认阻塞 L5，除非写入「必选清单」 |

**默认 L5 必选**：1.1、1.2。  
**1.3**：需要相机/视频管线时升为必选；否则保持「后置、不阻塞画布 100%」。  
**1.4**：默认边界说明，不阻塞。

### 8.2 工作项 2 — R.02 / document 策略（已拍板）

| 决策 | 内容 |
|------|------|
| **不做进画布 100%** | R.02 PDF/SVG **不作为** L5 完成条件 |
| **矩阵处理** | `SKIA_2D_CAPABILITY_MATRIX.md` 中 R.02 保持 ⬜ 或改为 `N/A（document 专项）`，备注指向本节 |
| **禁止话术** | 未实现 R.02 时不得宣称「Skia 全栈含文档后端完整」 |
| **允许话术** | 「2D 画布能力 100%（Canvas/Paint 路径，排除 PDF/SVG document）」 |
| **若重启 R.02** | 新开 `DOC.1`：设计 API → 录制回放桥 → PDF/SVG 后端；与 C 矩阵并行、不占用 L1–L4 关键路径 |

### 8.3 工作项 3 — 窗口场景扩展 C21+（真 present 覆盖剩余 MatrixID）

目标：把 C01–C20 未挂到的 **高价值画布 ID** 做成窗口场景；一进程一场景、同一套门禁。

#### 8.3.1 规划场景表（实现时写入 `scenarios.go`）

| ID | 名称 | MatrixIDs | 优先级 | 应看到（摘要） |
|----|------|-----------|--------|----------------|
| C21 | PorterDuff 板 | B.02 | P0 | Clear/Src/DstOut/Xor 等色块矩阵 |
| C22 | Path/Diff 裁剪 | C.03,C.06,C.04 | P0 | path clip + Difference 镂空 |
| C23 | 渐变 tile/local | D.04,D.06 | P0 | repeat/mirror + pattern 局部矩阵 |
| C24 | 图高级采样 | I.04,I.05,I.06,I.07 | P0 | mip/bicubic、opacity、旋转、九宫格 |
| C25 | 文本 shaping/emoji | X.03,X.09,X.10,X.11 | P0 | GSUB 混排、variable、emoji、atlas 复用可视 |
| C26 | 路径进阶 | H.02,G.05,H.04,H.05,E.02,E.03 | P1 | arc、boolean、measure、corner/discrete/trim |
| C27 | 变换进阶 | T.03,T.04,P.07 | P1 | 非均匀 stroke、图像四边形透视感、miter limit |
| C28 | 层混合 + Filter 图 | L.04,F.03 | P1 | layer blend + multi-RT filter 链 |
| C29 | 质量 MSAA/AA | Q.01,Q.02,Q.03,Q.04,S.08 | P1 | MSAA 边、coverage AA、像素对齐、HiDPI hairline |
| C30 | Atlas + Picture | V.02,R.01,S.01,S.02,S.06 | P2 | DrawAtlas 精灵、Picture 录制回放、离屏读回说明 |
| C31 | Path 光栅快径 | H.06,H.07,P.09 | P2 | 复杂 path / convex 快径 / dither |
| C32 | 合成压力+回归 | 多 ID | P2 | L0+本批 API 轻量同屏，防回归 |

> R.02 **不进** C 系列。I.08 YUV 若 1.3 必选则加 **C33 YUV multiplanar**。

#### 8.3.2 覆盖原则

- 每个 C 场景 `MatrixIDs` 必须可追溯到 `SKIA_2D_CAPABILITY_MATRIX.md`。  
- 已有 `TestP1_Capability_*` 的优先窗口化，先 present 再抠像素。  
- 不够 60fps 时：有界 RT + 保留 DrawImage，禁止关掉能力装绿。

### 8.4 工作项 4 — 质量全管线（对标 Skia 画质路径）

| 子项 | 内容 | Matrix / 层 | 验收 |
|------|------|-------------|------|
| 4.1 颜色空间 | 默认 sRGB 一致；线性混合路径可切换 | CS.01, CS.03 | 中灰/渐变带无错误编码 |
| 4.2 F16 / 宽色域 | 可选：`Context` 或离屏 RT 走 rgba16float | CS.02 | 不仅底层 RT，要有可画路径或明确「仅 RT API」边界 |
| 4.3 MSAA 窗口 e2e | 4x resolve 真 present | Q.01 | 边锯齿对比场景 C29 |
| 4.4 Filter DAG 深度 | blur/CM/shadow 多节点图稳定 | F.03 | 无闪、cpu_fb=0、可组合 |
| 4.5 Premul / AA 边 | 半透明 AA 不爆色 | Q.04, B.05 | 固定像素 + 窗口 |

### 8.5 工作项 5 — 像素与性能对标（非玩具）

| 子项 | 内容 | 验收 |
|------|------|------|
| 5.1 Golden corpus | 选定 N 张参考图（可用 Skia 离线导出或本仓库锁定 PNG） | RMSE/SSIM 门禁；diff PNG 落盘 |
| 5.2 窗口截帧 | C 场景关键帧读回 vs golden（可选自动化） | 与 L0 门禁并存 |
| 5.3 性能基线 | 全开/单场景 60fps、进程 CPU/RSS 趋势 | 对齐 mem_anim 指标哲学（只看本进程） |
| 5.4 批处理/图集 | 同屏多 draw 不无谓掉帧 | C32 + soak 抽样 |
| 5.5 回归铁律 | 改 render/webgpu/rwgpu 必须跑 C01–C20 + 已启用 C21+ | CI 或本地脚本 |

---

## 9. 实现计划（排序与里程碑）

### 9.1 阶段总览

```
L0 已完成 (C01–C20)
    │
    ▼
P1  语义硬缺口 1.1 B.07 + 1.2 B.06     → 单测绿 + C17/C07/C08 回归
    │
    ▼
P2  窗口扩展 Wave-A (C21–C25)           → 每场景独立 PASS
    │
    ▼
P3  窗口扩展 Wave-B (C26–C29)           → 路径/变换/层滤镜/质量
    │
    ▼
P4  质量全管线 4.1–4.5                  → CS/MSAA/Filter/Premul
    │
    ▼
P5  窗口扩展 Wave-C (C30–C32) + 回归   → Atlas/Picture/合成
    │
    ▼
P6  像素/性能 5.1–5.5                   → golden + 基线报告
    │
    ▼
L5  宣称「2D 画布 100%（排除 document）」
    │
    └── DOC.1 (可选) R.02 PDF/SVG       → 与 L5 解耦，默认不排期
```

### 9.2 阶段明细

| 阶段 | 名称 | 交付物 | 完成定义 |
|------|------|--------|----------|
| **P0** | 文档与范围 | 本文件 §0/§8/§9；矩阵 R.02 备注「画布 100% 排除」 | 本文落地 |
| **P1** | 语义硬缺口 | B.07 Modulate；B.06 premul 精修；更新矩阵脚注 | 单测绿 + C07/C08/C17 回归 PASS |
| **P2** | C21–C25 | `scenarios`/`probes`/`main` 扩展；证据 JSON | 五场景均 PASS，cpu_fb=0 |
| **P3** | C26–C29 | 同上 | 四场景均 PASS |
| **P4** | 质量管线 | CS/MSAA/Filter/Premul 门禁与窗口挂钩 | §8.4 子项全勾或书面边界 |
| **P5** | C30–C32 | Atlas/Picture/合成回归 | 三场景 PASS + 全量脚本 |
| **P6** | 对标 | golden 目录、对比工具、性能表 | §8.5 可重复跑出报告 |
| **P7** | L5 宣告 | 更新矩阵 §4 差距表 + 本文件 L5=✅ | 清单审计无必选项残留 |
| **Px** | DOC.1 | PDF/SVG（可选） | **不阻塞 P7** |

### 9.3 每阶段工程纪律

1. 先单测/离屏，再窗口场景；窗口失败不得靠关能力装绿。  
2. 一次只开一个 `GPUI_SCENARIO`。  
3. 修复闪烁/掉帧时考虑全场景，对标 Skia 画布语义。  
4. 证据写入 `/tmp/capability_matrix_evidence/` 或更新本节。  
5. 同步改 `SKIA_2D_CAPABILITY_MATRIX.md` 脚注状态，避免表与实现漂移。

### 9.4 建议排期（人力 1 时的粗序，可并行标 *）

| 序 | 工作 | 依赖 |
|----|------|------|
| 1 | P0 文档（本文件） | — |
| 2 | P1.1 Modulate | — |
| 3 | P1.2 alpha premul | — |
| 4 | P2 C21 blend PD | P1 后更稳 |
| 5 | P2 C22 clip | — * |
| 6 | P2 C23 gradient tile | — * |
| 7 | P2 C24 image advanced | — * |
| 8 | P2 C25 text advanced | MultiFace 已有 |
| 9 | P3 C26–C29 | P2 稳定后 |
| 10 | P4 质量 | 可与 P3 尾部重叠 * |
| 11 | P5 C30–C32 | P3+P4 |
| 12 | P6 golden/性能 | P5 |
| 13 | P7 L5 宣告 | P1–P6 |
| — | Px R.02 | 单独产品决策，默认不做 |

### 9.5 不做清单（防范围膨胀）

- R.02 PDF/SVG 不进 L5。  
- 控件层 / ant.design 组件库（更后阶段）。  
- 游戏专有 3D（已有 mesh3d 示例，不进画布 100% 必选）。  
- Ray tracing / 视频解码器本体（矩阵 2.6 后置）。

---

## 10. 状态看板（维护用）

| 项 | 状态 | 更新日期 |
|----|------|----------|
| L0 C01–C20 | ✅ | 2026-07-16 |
| P0 范围声明 R.02 排除 | ✅（本文 §0） | 2026-07-16 |
| P1 B.07 / B.06 | ✅ | 2026-07-16 |
| P2 C21–C25 | ⬜ | — |
| P3 C26–C29 | ⬜ | — |
| P4 质量管线 | ⬜ | — |
| P5 C30–C32 | ⬜ | — |
| P6 golden/性能 | ⬜ | — |
| L5 画布 100% 宣告 | ⬜ | — |
| Px R.02 document | 🚫 默认不做 | 2026-07-16 |

---

## 11. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-16 | 1.0 | C01–C20 基线、门禁、证据 |
| 2026-07-16 | 1.1 | 写入工作项 1–5、R.02 排除 document 声明、P0–P7 实现计划与 C21+ 表 |
| 2026-07-16 | 1.2 | P1 关闭：B.07 GPU `BlendModulate` + B.06 multi-path premul 门禁；C17 纳入 Modulate |
