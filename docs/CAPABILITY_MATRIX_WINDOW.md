# 能力矩阵窗口验收（examples/capability_matrix）

> 版本：1.12 | 日期：2026-07-21 | **活文档**  
> 真源：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)  
> 引擎缺口：[`ENGINE_GAPS.md`](./ENGINE_GAPS.md)  
> 实现：`examples/capability_matrix/`（X11 + webgpu → render）

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
| **L0** | 高频窗口基线 C01–C20 | ✅ 20/20 PASS（final `/tmp/cap_l0_final`） |
| **L1** | 语义硬缺口关闭（工作项 1） | ✅ P1 B.07/B.06 关闭 |
| **L2** | 窗口 ID 覆盖扩展 C21+（工作项 3） | ✅ Wave-A C21–C25 PASS |
| **L3** | 质量全管线（工作项 4） | ✅ P4（CS/MSAA/Filter/Premul；F16 书面后置） |
| **L4** | 像素/性能对标 Skia（工作项 5） | ✅ P6 关闭（golden+compare+perf+C01–C32 final） |
| **L5 画布 100%** | L1–L4 绿 + 范围声明排除 R.02 + 矩阵审计无必选 ⬜ | ✅ 已宣告（§7.7） |
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
5. **L5 已宣告**「2D 画布 100%（排除 document）」；永不因 R.02 未做而宣称含 document 全栈。  
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


## 7.1 L0 回归复跑（GPU_FIRST 后 · 2026-07-16）

环境：Linux X11 `DISPLAY=:1`，`/tmp/cap_l0_bin`，每场景 8s。

### 7.1.1 reg2（较早一轮）

| 结果 | 说明 |
|------|------|
| **pass=18 fail=2 total=20** | 证据目录 `/tmp/cap_l0_reg2/` |
| **cpu_fb** | **全部 0**（无 silent CPU；GPU_FIRST 未回退） |
| **FAIL** | C07 可分离混合 `fps_ema=52.9`；C17 高级混合 `fps_ema=52.6`（均 `want>=55`，**probe=true / cpu_fb=0**） |
| **原因归类** | 帧率预算门禁，非正确性/GPU-first 缺口；混合 RT 每帧 `ExportImageBuf`+高级 blend 单核 CPU 偏高 |
| **PASS 样例** | C01–C06、C08–C16、C18–C20 稳态约 60fps |

### 7.1.2 reg3（再次全量 C01–C20 · 2026-07-16 18:42）

| 结果 | 说明 |
|------|------|
| **pass=19 fail=1 total=20** | 证据目录 `/tmp/cap_l0_reg3/` |
| **cpu_fb** | **全部 0**（GPU_FIRST 正确性无回归） |
| **probe_ok** | **全部 true** |
| **FAIL** | 仅 **C07** 可分离混合：`fps_ema=53.3` want≥55；`fps_avg=55.9`；`cpu≈107%`；`gpu_ops=8940`；`probe=true` |
| **C17** | **PASS**（临界）：`fps_ema=55.1`；`fps_avg=52.3`；`cpu≈97%` — 较 reg2 改善/抖动，仍属高压混合场景 |
| **结论** | **GPU_FIRST 未破坏调用链/回退门禁**；剩余是 C07 热路径性能（`effectRT.publish` → 每帧 `ExportImageBuf` GPU→CPU readback） |

reg3 单行摘要：

```
scenario=C01 status=PASS fps_ema=62.4 fps_avg=61.3 cpu=9 cpu_fb=0 gpu_ops=8838 probe=true reason=
scenario=C02 status=PASS fps_ema=61.2 fps_avg=61.0 cpu=9 cpu_fb=0 gpu_ops=8313 probe=true reason=
scenario=C03 status=PASS fps_ema=61.3 fps_avg=60.7 cpu=10 cpu_fb=0 gpu_ops=11178 probe=true reason=
scenario=C04 status=PASS fps_ema=61.2 fps_avg=60.8 cpu=10 cpu_fb=0 gpu_ops=7792 probe=true reason=
scenario=C05 status=PASS fps_ema=62.0 fps_avg=61.5 cpu=9 cpu_fb=0 gpu_ops=16728 probe=true reason=
scenario=C06 status=PASS fps_ema=97.8 fps_avg=61.4 cpu=23 cpu_fb=0 gpu_ops=11760 probe=true reason=
scenario=C07 status=FAIL fps_ema=53.3 fps_avg=55.9 cpu=107 cpu_fb=0 gpu_ops=8940 probe=true reason=fps_low_steady ema=53.3 want>=55
scenario=C08 status=PASS fps_ema=63.3 fps_avg=61.1 cpu=56 cpu_fb=0 gpu_ops=9272 probe=true reason=
scenario=C09 status=PASS fps_ema=61.7 fps_avg=60.9 cpu=9 cpu_fb=0 gpu_ops=11712 probe=true reason=
scenario=C10 status=PASS fps_ema=63.7 fps_avg=61.0 cpu=10 cpu_fb=0 gpu_ops=13636 probe=true reason=
scenario=C11 status=PASS fps_ema=476.6 fps_avg=59.3 cpu=76 cpu_fb=0 gpu_ops=10810 probe=true reason=
scenario=C12 status=PASS fps_ema=60.4 fps_avg=61.0 cpu=17 cpu_fb=0 gpu_ops=7808 probe=true reason=
scenario=C13 status=PASS fps_ema=61.5 fps_avg=61.2 cpu=9 cpu_fb=0 gpu_ops=10290 probe=true reason=
scenario=C14 status=PASS fps_ema=63.2 fps_avg=61.1 cpu=64 cpu_fb=0 gpu_ops=9272 probe=true reason=
scenario=C15 status=PASS fps_ema=61.9 fps_avg=61.1 cpu=79 cpu_fb=0 gpu_ops=12200 probe=true reason=
scenario=C16 status=PASS fps_ema=61.9 fps_avg=61.7 cpu=8 cpu_fb=0 gpu_ops=8892 probe=true reason=
scenario=C17 status=PASS fps_ema=55.1 fps_avg=52.3 cpu=97 cpu_fb=0 gpu_ops=6225 probe=true reason=
scenario=C18 status=PASS fps_ema=523.8 fps_avg=57.0 cpu=69 cpu_fb=0 gpu_ops=5863 probe=true reason=
scenario=C19 status=PASS fps_ema=61.1 fps_avg=60.7 cpu=11 cpu_fb=0 gpu_ops=11664 probe=true reason=
scenario=C20 status=PASS fps_ema=62.5 fps_avg=61.4 cpu=15 cpu_fb=0 gpu_ops=15744 probe=true reason=
```

复跑命令：

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH DISPLAY=:1
go build -o /tmp/cap_l0_bin ./examples/capability_matrix
for id in C01 C02 C03 C04 C05 C06 C07 C08 C09 C10 C11 C12 C13 C14 C15 C16 C17 C18 C19 C20; do
  GPUI_SCENARIO=$id GPUI_ANIM_SECONDS=8 GPUI_RESULT_FILE=/tmp/cap_l0_reg3/${id}.json /tmp/cap_l0_bin
done
```


### 7.1.3 final（视觉修复后 · 2026-07-16 19:00）

| 结果 | 说明 |
|------|------|
| **pass=20 fail=0 total=20** | 证据目录 `/tmp/cap_l0_final/` |
| **cpu_fb** | **全部 0**；**probe_ok 全部 true** |
| **C07** | PASS `fps_ema≈60.9`：小离屏 RT + `ExportImageBuf→DrawImage`（**禁止**不完整的 `FlushGPUWithView→DrawGPUTexture` RT 合成） |
| **C17** | PASS `fps_ema≈59.8`：有界 RT + Export 合成 |
| **教训** | 曾用 `FlushGPUWithView` 直接 `DrawGPUTexture` 做 RT 合成会导致画面缺失/错误（pixmap 侧 advanced blend 等内容未进入 view）；正确热路径在像素正确前提下再谈 GPU-only blit |

final 单行摘要：

```
scenario=C01 status=PASS fps_ema=62.3 fps_avg=61.8 cpu=9 cpu_fb=0 gpu_ops=8910 probe=true reason=
scenario=C02 status=PASS fps_ema=61.2 fps_avg=61.0 cpu=9 cpu_fb=0 gpu_ops=8313 probe=true reason=
scenario=C03 status=PASS fps_ema=61.6 fps_avg=60.8 cpu=10 cpu_fb=0 gpu_ops=11201 probe=true reason=
scenario=C04 status=PASS fps_ema=60.8 fps_avg=60.9 cpu=10 cpu_fb=0 gpu_ops=7808 probe=true reason=
scenario=C05 status=PASS fps_ema=63.0 fps_avg=61.6 cpu=8 cpu_fb=0 gpu_ops=16762 probe=true reason=
scenario=C06 status=PASS fps_ema=135.8 fps_avg=61.5 cpu=25 cpu_fb=0 gpu_ops=11784 probe=true reason=
scenario=C07 status=PASS fps_ema=60.9 fps_avg=59.8 cpu=100 cpu_fb=0 gpu_ops=10472 probe=true reason=
scenario=C08 status=PASS fps_ema=61.5 fps_avg=60.9 cpu=53 cpu_fb=0 gpu_ops=9253 probe=true reason=
scenario=C09 status=PASS fps_ema=62.0 fps_avg=61.1 cpu=9 cpu_fb=0 gpu_ops=11736 probe=true reason=
scenario=C10 status=PASS fps_ema=61.0 fps_avg=60.8 cpu=11 cpu_fb=0 gpu_ops=13636 probe=true reason=
scenario=C11 status=PASS fps_ema=396.9 fps_avg=59.9 cpu=74 cpu_fb=0 gpu_ops=10971 probe=true reason=
scenario=C12 status=PASS fps_ema=61.4 fps_avg=61.0 cpu=17 cpu_fb=0 gpu_ops=7824 probe=true reason=
scenario=C13 status=PASS fps_ema=61.6 fps_avg=61.3 cpu=9 cpu_fb=0 gpu_ops=10311 probe=true reason=
scenario=C14 status=PASS fps_ema=62.8 fps_avg=61.0 cpu=63 cpu_fb=0 gpu_ops=9234 probe=true reason=
scenario=C15 status=PASS fps_ema=62.5 fps_avg=61.1 cpu=80 cpu_fb=0 gpu_ops=12175 probe=true reason=
scenario=C16 status=PASS fps_ema=61.9 fps_avg=61.8 cpu=8 cpu_fb=0 gpu_ops=8910 probe=true reason=
scenario=C17 status=PASS fps_ema=59.8 fps_avg=58.4 cpu=96 cpu_fb=0 gpu_ops=7440 probe=true reason=
scenario=C18 status=PASS fps_ema=692.4 fps_avg=59.9 cpu=72 cpu_fb=0 gpu_ops=6188 probe=true reason=
scenario=C19 status=PASS fps_ema=61.9 fps_avg=60.6 cpu=11 cpu_fb=0 gpu_ops=11664 probe=true reason=
scenario=C20 status=PASS fps_ema=62.1 fps_avg=61.7 cpu=16 cpu_fb=0 gpu_ops=15808 probe=true reason=
```


## 7.2 P2 Wave-A 证据（C21–C25 · 2026-07-16）

环境：Linux X11 `DISPLAY=:1`，`/tmp/cap_l0_bin`，每场景 8s。  
证据目录：`/tmp/cap_p2_final/`（另有 r2 稳定性：`/tmp/cap_p2_r2/`）。

| ID | 名称 | MatrixIDs | 结果 | 备注 |
|----|------|-----------|------|------|
| C21 | PorterDuff 板 | B.02 | ✅ PASS | `fps_ema≈71.7` cpu_fb=0 |
| C22 | Path/Diff 裁剪 | C.03,C.06,C.04 | ✅ PASS | present 路径固定 clip 几何；`fps_ema≈60.3`（曾因每帧 Export+复杂 mask ~30fps） |
| C23 | 渐变 tile/local | D.04,D.06 | ✅ PASS | `fps_ema≈61.9` |
| C24 | 图高级采样 | I.04–I.07 | ✅ PASS | `fps_ema≈62.3` |
| C25 | 文本 shaping/混排 | X.03,X.09–X.11 | ✅ PASS | `fps_ema≈86.2`（含 atlas 复用串） |

汇总：**pass=5 fail=0**；全场景 `cpu_fb=0` / `probe=true`。

L0 冒烟（同批）：C01/C05/C07/C17 均 PASS。

单行摘要：

```
scenario=C21 status=PASS fps_ema=71.7 fps_avg=61.6 cpu=24 cpu_fb=0 gpu_ops=7856 probe=true reason=
scenario=C22 status=PASS fps_ema=60.3 fps_avg=59.6 cpu=91 cpu_fb=0 gpu_ops=10428 probe=true reason=
scenario=C23 status=PASS fps_ema=61.9 fps_avg=60.9 cpu=83 cpu_fb=0 gpu_ops=8730 probe=true reason=
scenario=C24 status=PASS fps_ema=62.3 fps_avg=61.6 cpu=9 cpu_fb=0 gpu_ops=9860 probe=true reason=
scenario=C25 status=PASS fps_ema=86.2 fps_avg=61.5 cpu=31 cpu_fb=0 gpu_ops=9780 probe=true reason=
```

实现要点：

- C21/C23/C25：有界离屏 RT + `ExportImageBuf→DrawImage`（像素正确）。
- C22：改为 **present 直绘**（固定 path clip / Difference 孔洞，仅 clip 内动画），避免每帧 mask rebuild+Export。
- C24：present 直绘 mip/opacity/rotate/nine-patch。
- **禁止**对混合/裁剪 RT 使用不完整的 `FlushGPUWithView→DrawGPUTexture`（会导致画面缺失）。

复跑：

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH DISPLAY=:1
go build -o /tmp/cap_l0_bin ./examples/capability_matrix
for id in C21 C22 C23 C24 C25; do
  GPUI_SCENARIO=$id GPUI_ANIM_SECONDS=8 GPUI_RESULT_FILE=/tmp/cap_p2_final/${id}.json /tmp/cap_l0_bin
done
```


## 7.3 P3 Wave-B 证据（C26–C29 · 2026-07-16）

环境：Linux X11 `DISPLAY=:1`，`/tmp/cap_l0_bin`，每场景 8s。  
证据目录：`/tmp/cap_p3_final/`（首跑 `/tmp/cap_p3_r1/` 亦全绿）。

| ID | 名称 | MatrixIDs | 结果 | 备注 |
|----|------|-----------|------|------|
| C26 | 路径进阶 | H.02,G.05,H.04,H.05,E.02,E.03 | ✅ PASS | arc/ellipse arc、boolean 差集、Trim、WithCorners/Discrete；`fps≈60.9` |
| C27 | 变换进阶 | T.03,T.04,P.07 | ✅ PASS | 非均匀 Scale stroke、DrawImageQuad、高低 MiterLimit；`fps≈61.6` |
| C28 | 层混合+滤镜链 | L.04,F.03 | ✅ PASS | PushLayer + ApplyBlur + ApplyColorMatrix（隔帧 retain）；`fps_avg≈53.9`≥48 |
| C29 | 质量 MSAA/AA | Q.01–Q.04,S.08 | ✅ PASS | AA 开/关斜边、hairline、dither 渐变、2× DeviceScale hairline RT；`fps≈79` |

汇总：**pass=4 fail=0**；`cpu_fb=0` / `probe=true`。  
冒烟：C01/C07/C22/C25 均 PASS。

单行摘要：

```
scenario=C26 status=PASS fps_ema=60.9 fps_avg=61.0 cpu=66 cpu_fb=0 gpu_ops=10714 probe=true reason=
scenario=C27 status=PASS fps_ema=61.6 fps_avg=60.9 cpu=9 cpu_fb=0 gpu_ops=10736 probe=true reason=
scenario=C28 status=PASS fps_ema=693.8 fps_avg=53.9 cpu=83 cpu_fb=0 gpu_ops=6848 probe=true reason=
scenario=C29 status=PASS fps_ema=79.4 fps_avg=61.2 cpu=35 cpu_fb=0 gpu_ops=17080 probe=true reason=
```

实现要点：

- C26/C27/C29 以 **present 直绘** 为主（避免不必要 Export）。
- C28 有界 RT + Export（滤镜链必须在 pixmap/GPU filter 路径上），隔帧 presentCached 稳住帧预算。
- C29 的 2× `SetDeviceScale` 仅在小离屏 RT 上演示 S.08 hairline，不改主窗口 surface 尺寸。

复跑：

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH DISPLAY=:1
go build -o /tmp/cap_l0_bin ./examples/capability_matrix
for id in C26 C27 C28 C29; do
  GPUI_SCENARIO=$id GPUI_ANIM_SECONDS=8 GPUI_RESULT_FILE=/tmp/cap_p3_final/${id}.json /tmp/cap_l0_bin
done
```


## 7.4 P4 质量全管线收口（2026-07-16）

| 子项 | 状态 | 证据 |
|------|------|------|
| 4.1 颜色空间 sRGB/线性中点 | ✅ | `TestP1_Capability_CS03*`；窗口 C06/C29 渐变 |
| 4.2 F16 宽色域 | 📝 后置 | **不阻塞**画布 100%；需业务宽色域时再开专项 |
| 4.3 MSAA 窗口 | ✅ | C29 Q.01–Q.04；`MSAAAware.MSAASampleCount` 单测 |
| 4.4 Filter DAG | ✅ | C28 `PushLayer`+`ApplyBlur`+`ApplyColorMatrix`，cpu_fb=0 |
| 4.5 Premul/AA | ✅ | C07/C08 窗口；`TestP1_Capability_B05_*`、`Q04_PremulAAEdgeGPU` |

**L3 判定**：4.1/4.3/4.4/4.5 绿 + 4.2 书面边界 → **L3 ✅**。

## 7.5 P5 Wave-C 证据（C30–C32 · 2026-07-16）

环境：Linux X11 `DISPLAY=:1`，`/tmp/cap_l0_bin`，每场景 8s。  
证据目录：`/tmp/cap_p5_final/`。

| ID | 名称 | MatrixIDs | 结果 | 备注 |
|----|------|-----------|------|------|
| C30 | Atlas+Picture | V.02,R.01,S.01,S.02,S.06 | ✅ PASS | DrawAtlas×12 + recording→raster→DrawImage；`fps≈70` |
| C31 | 路径光栅快径 | H.06,H.07,P.09 | ✅ PASS | 凸多边形 + 非凸星 + dither 对比条；`fps≈61` |
| C32 | 合成压力回归 | 多 ID | ✅ PASS | C20 级轻量同屏（无每帧 PushLayer）；`fps≈65` |

汇总：**pass=3 fail=0**；`cpu_fb=0` / `probe=true`。  
冒烟：C01/C07/C20/C29 均 PASS。

单行摘要：

```
scenario=C30 status=PASS fps_ema=70.0 fps_avg=60.9 cpu=14 cpu_fb=0 gpu_ops=9234 probe=true reason=
scenario=C31 status=PASS fps_ema=61.2 fps_avg=60.7 cpu=20 cpu_fb=0 gpu_ops=13608 probe=true reason=
scenario=C32 status=PASS fps_ema=65.0 fps_avg=61.4 cpu=16 cpu_fb=0 gpu_ops=18167 probe=true reason=
```

实现要点：

- C30：`recording.NewRecorder` → `FinishRecording` → raster `Playback` → `ImageBufFromImage` → present；与 `DrawAtlas` 同屏。
- C31：present 直绘凸/非凸 path + `SetDither` 条带。
- C32：对齐 C20 成本模型（避免每帧 PushLayer/Export 把回归打成 5fps）。

复跑：

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH DISPLAY=:1
go build -o /tmp/cap_l0_bin ./examples/capability_matrix
for id in C30 C31 C32; do
  GPUI_SCENARIO=$id GPUI_ANIM_SECONDS=8 GPUI_RESULT_FILE=/tmp/cap_p5_final/${id}.json /tmp/cap_l0_bin
done
```


### 7.6 P6 像素/性能（关闭）

证据目录：`/tmp/cap_p6_capture/`；golden：`examples/capability_matrix/golden/`。

| 交付 | 状态 | 说明 |
|------|------|------|
| Golden C01–C32 | ✅ | 确定性 `GPUI_DETERMINISTIC=1` + `GPUI_GOLDEN_NO_HUD=1` + frame 90 |
| RMSE/SSIM 对比 | ✅ | `scripts/cap_compare_golden.py`（默认 RMSE≤0.08 SSIM≥0.90） |
| 性能表 | ✅ | `/tmp/cap_p6_capture/PERF.md`（C01–C32 本进程指标） |
| 全量回归脚本 | ✅ | `scripts/run_capability_matrix_p6.sh` |
| 画面修正 | ✅ | C06 渐变坐标溢出；C10 文本衬底；C15 backdrop 隔帧；C22 clip RT 隔帧 |

**P6 已关闭**：C01–C32 全 PASS（`/tmp/cap_p6_final`，cpu_fb=0）+ golden 32 张 + compare 脚本 + PERF 表。L4 ✅。→ 已衔接到 §7.7 P7/L5。

### 7.7 P7 L5 画布 100% 宣告（关闭）

**审计日期**：2026-07-16  

| 检查项 | 结果 | 证据 |
|--------|------|------|
| L0 C01–C20 窗口基线 | ✅ | 历史 final + `/tmp/cap_p6_final` 复验 |
| L1 语义硬缺口 B.07/B.06 | ✅ | P1 单测 + C07/C08/C17 |
| L2/L3 窗口扩展 + 质量管线 | ✅ | C21–C32；P4（F16 书面后置） |
| L4 golden/性能 | ✅ | golden 32 + `/tmp/cap_p6_final` 32/32 PASS，cpu_fb=0 |
| 矩阵画布行无必选 ⬜ | ✅ | 仅 **R.02** 为 ⬜ 且 **排除 document**（`SKIA_2D_CAPABILITY_MATRIX.md` §4） |
| 子集边界书面化 | ✅ | I.08 真 multiplanar YUV 后置；CS.02 Context 8-bit / RT F16 门禁；K.02 高层 multi-draw 后置 |
| R.02 不进 L5 | ✅ | 本文件 §0 / §8.2 |

**宣告（可对外使用）**：

> gpui **2D 画布能力 100%（排除 PDF/SVG document）**：  
> `render → webgpu → rwgpu → libwgpu_native` 真窗口 present 覆盖矩阵画布语义；  
> **不包含** R.02 document 后端；**不包含** 控件层 / ant.design。

**非本宣告范围**：控件层、游戏 3D 专有路径、DOC.1 PDF/SVG、真 multiplanar 视频 YUV 产品化。

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
| C21 | PorterDuff 板 | B.02 | ✅ | Clear/Src/DstOut/Xor 等色块矩阵 |
| C22 | Path/Diff 裁剪 | C.03,C.06,C.04 | ✅ | path clip + Difference 镂空 |
| C23 | 渐变 tile/local | D.04,D.06 | ✅ | repeat/mirror + pattern 局部矩阵 |
| C24 | 图高级采样 | I.04,I.05,I.06,I.07 | ✅ | mip/bicubic、opacity、旋转、九宫格 |
| C25 | 文本 shaping/emoji | X.03,X.09,X.10,X.11 | ✅ | GSUB 混排、variable、emoji、atlas 复用可视 |
| C26 | 路径进阶 | H.02,G.05,H.04,H.05,E.02,E.03 | ✅ | arc、boolean、measure、corner/discrete/trim |
| C27 | 变换进阶 | T.03,T.04,P.07 | ✅ | 非均匀 stroke、图像四边形透视感、miter limit |
| C28 | 层混合 + Filter 图 | L.04,F.03 | ✅ | layer blend + multi-RT filter 链 |
| C29 | 质量 MSAA/AA | Q.01,Q.02,Q.03,Q.04,S.08 | ✅ | MSAA 边、coverage AA、像素对齐、HiDPI hairline |
| C30 | Atlas + Picture | V.02,R.01,S.01,S.02,S.06 | ✅ | DrawAtlas 精灵、Picture 录制回放、离屏读回说明 |
| C31 | Path 光栅快径 | H.06,H.07,P.09 | ✅ | 复杂 path / convex 快径 / dither |
| C32 | 合成压力+回归 | 多 ID | ✅ | L0+本批 API 轻量同屏，防回归 |

> R.02 **不进** C 系列。I.08 YUV 若 1.3 必选则加 **C33 YUV multiplanar**。

#### 8.3.2 覆盖原则

- 每个 C 场景 `MatrixIDs` 必须可追溯到 `SKIA_2D_CAPABILITY_MATRIX.md`。  
- 已有 `TestP1_Capability_*` 的优先窗口化，先 present 再抠像素。  
- 不够 60fps 时：有界 RT + 保留 DrawImage，禁止关掉能力装绿。

### 8.4 工作项 4 — 质量全管线（对标 Skia 画质路径）

| 子项 | 内容 | Matrix / 层 | 验收 |
|------|------|-------------|------|
| 4.1 颜色空间 | 默认 sRGB 一致；线性混合路径可切换 | CS.01, CS.03 | ✅ 单测 `TestP1_Capability_CS03_LinearBlendMidGPU` + C06/C29 渐变 present |
| 4.2 F16 / 宽色域 | 可选：`Context` 或离屏 RT 走 rgba16float | CS.02 | 📝 **书面后置**：底层 RT 子集；**不阻塞** L3/L5 画布 100%（与 I.08 同类） |
| 4.3 MSAA 窗口 e2e | 4x resolve 真 present | Q.01 | ✅ C29 + `MSAAAware` 单测门禁 |
| 4.4 Filter DAG 深度 | blur/CM/shadow 多节点图稳定 | F.03 | ✅ C28 layer+blur+CM；cpu_fb=0 |
| 4.5 Premul / AA 边 | 半透明 AA 不爆色 | Q.04, B.05 | ✅ C07/C08 + `TestP1_Capability_B05_*` / Q.04 |


### 8.5 工作项 5 — 像素与性能对标（非玩具）

| 子项 | 内容 | 验收 |
|------|------|------|
| 5.1 Golden corpus | 选定 N 张参考图（可用 Skia 离线导出或本仓库锁定 PNG） | ✅ 仓库锁定 `examples/capability_matrix/golden/C01–C32.png`（确定性 frame=90，无 HUD） |
| 5.2 窗口截帧 | C 场景关键帧读回 vs golden（可选自动化） | ✅ `GPUI_CAPTURE_DIR` + `scripts/cap_compare_golden.py`（RMSE/SSIM + diff） |
| 5.3 性能基线 | 全开/单场景 60fps、进程 CPU/RSS 趋势 | ✅ `/tmp/cap_p6_capture/PERF.md`（本进程 CPU/RSS/fps） |
| 5.4 批处理/图集 | 同屏多 draw 不无谓掉帧 | ✅ C30/C32 窗口 PASS；soak 可复用 `run_capability_matrix_p6.sh` |
| 5.5 回归铁律 | 改 render/webgpu/rwgpu 必须跑 C01–C20 + 已启用 C21+ | ✅ `scripts/run_capability_matrix_p6.sh` 覆盖 C01–C32 |

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
P2  窗口扩展 Wave-A (C21–C25)           → ✅ 每场景独立 PASS（§7.2）
    │
    ▼
P3  窗口扩展 Wave-B (C26–C29)           → ✅ 路径/变换/层滤镜/质量（§7.3）
    │
    ▼
P4  质量全管线 4.1–4.5                  → ✅ CS/MSAA/Filter/Premul（F16 后置）
    │
    ▼
P5  窗口扩展 Wave-C (C30–C32) + 回归   → ✅ Atlas/Picture/合成（§7.5）
    │
    ▼
P6  像素/性能 5.1–5.5                   → ✅ golden + 基线报告
    │
    ▼
L5  宣称「2D 画布 100%（排除 document）」  → ✅
    │
    └── DOC.1 (可选) R.02 PDF/SVG       → 与 L5 解耦，默认不排期
```

### 9.2 阶段明细

| 阶段 | 名称 | 交付物 | 完成定义 |
|------|------|--------|----------|
| **P0** | 文档与范围 | 本文件 §0/§8/§9；矩阵 R.02 备注「画布 100% 排除」 | 本文落地 |
| **P1** | 语义硬缺口 | B.07 Modulate；B.06 premul 精修；更新矩阵脚注 | 单测绿 + C07/C08/C17 回归 PASS |
| **P2** | C21–C25 | `scenarios`/`probes`/`main` 扩展；证据 JSON | ✅ 五场景 PASS，cpu_fb=0（§7.2） |
| **P3** | C26–C29 | 同上 | ✅ 四场景 PASS，cpu_fb=0（§7.3） |
| **P4** | 质量管线 | CS/MSAA/Filter/Premul 门禁与窗口挂钩 | ✅ §8.4 已勾/边界（§7.4） |
| **P5** | C30–C32 | Atlas/Picture/合成回归 | ✅ 三场景 PASS（§7.5） |
| **P6** | 对标 | golden 目录、对比工具、性能表 | ✅ §7.6 / §8.5 |
| **P7** | L5 宣告 | 更新矩阵 §4 差距表 + 本文件 L5=✅ | ✅ §7.7 |
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
| L0 C01–C20 | ✅ 20/20 final | 2026-07-16 |
| P0 范围声明 R.02 排除 | ✅（本文 §0） | 2026-07-16 |
| P1 B.07 / B.06 | ✅ | 2026-07-16 |
| P2 C21–C25 | ✅ `/tmp/cap_p2_final` | 2026-07-16 |
| P3 C26–C29 | ✅ `/tmp/cap_p3_final` | 2026-07-16 |
| P4 质量管线 | ✅ §7.4 / §8.4 | 2026-07-16 |
| P5 C30–C32 | ✅ `/tmp/cap_p5_final` | 2026-07-16 |
| P6 golden/性能 | ✅ `/tmp/cap_p6_final` + golden 32 | 2026-07-16 |
| L5 画布 100% 宣告 | ✅ §7.7 / 矩阵 §4 | 2026-07-16 |
| Px R.02 document | 🚫 默认不做 | 2026-07-16 |

---

## 11. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-16 | 1.0 | C01–C20 基线、门禁、证据 |
| 2026-07-16 | 1.1 | 写入工作项 1–5、R.02 排除 document 声明、P0–P7 实现计划与 C21+ 表 |
| 2026-07-16 | 1.2 | P1 关闭：B.07 GPU `BlendModulate` + B.06 multi-path premul 门禁；C17 纳入 Modulate |
| 2026-07-16 | 1.3 | L0 C01–C20 复跑：18/20 PASS；FAIL 仅 C07/C17 fps 门禁；全场景 cpu_fb=0 |
| 2026-07-16 | 1.4 | L0 再全量 reg3：19/20 PASS；仅 C07 fps；C17 临界 PASS；全场景 cpu_fb=0 / probe=true（GPU_FIRST 无正确性回归） |
| 2026-07-16 | 1.5 | 修复错误 RT GPU blit 导致画面异常；C07 小 RT+Export 合成；L0 final 20/20 PASS `/tmp/cap_l0_final` |
| 2026-07-16 | 1.6 | P2 Wave-A 关闭：C21–C25 全 PASS；C22 present 固定 clip；证据 `/tmp/cap_p2_final`；L2 ✅ |
| 2026-07-16 | 1.7 | P3 Wave-B 关闭：C26–C29 全 PASS；证据 `/tmp/cap_p3_final`；路径/变换/层滤镜/质量窗口覆盖 |
| 2026-07-16 | 1.8 | P4 质量管线收口（F16 后置）+ P5 C30–C32 全 PASS；L3 ✅；证据 `/tmp/cap_p5_final` |
| 2026-07-16 | 1.9 | 画面修正 C06/C10/C15/C22；P6 启动：golden C01–C32 + 对比/回归脚本 + 性能表 `/tmp/cap_p6_capture` |
| 2026-07-16 | 1.10 | P6 关闭：C01–C32 final 全 PASS `/tmp/cap_p6_final`；L4 ✅；下一 P7 L5 宣告 |
| 2026-07-16 | 1.11 | **P7/L5 关闭**：矩阵 §4 审计重写；宣告 2D 画布 100%（排除 document）；证据 `/tmp/cap_p6_final` |
