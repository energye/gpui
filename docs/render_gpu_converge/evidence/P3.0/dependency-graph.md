# P3 缺失能力依赖图（P3.0 定稿，2026-11-01）

> 任务：P3.0 依赖图 + 文档更新（PLAN §5 + `RENDER_API_CATALOG.md` + 实现顺序）。
> 拍板（2026-11-01，用户确认）：**D1** 依赖修订 5 条全改；**D2** 实现顺序=推荐主序串行；**D3** catalog 新增 §13 规划登记节；**D4** PLAN §5 三表加「代码现状」列+修差异行。
> 现状证据：`assessment.md`（同目录，全部 file:line，HEAD `da9168a5` 实测）；本文件是图与顺序的正本，PLAN §7.4 与台账 §2 依赖列已按本文同步。
> 硬约束：P3.1（线性混合）与 P3.3（逐 run 样式）是前移项，不得挪回 P4（PLAN §7.8-4、台账 §3）；文档没写完不写代码（PLAN §7.4）。

## 1. 依赖图

```
P3.0（本图）──┬→ P3.1 色彩管理地基（线性混合接线）【前移项；HDR/滤镜/2.5D 共同地基】
              ├→ P3.5 路径与几何（conic/四角RRect/Matrix4/PathEffect/布尔保曲线）
              └→ P3.6 图片格式（GIF/APNG/动图WebP/AVIF/HEIF/EXIF）+ GPU mipmap

P3.1 ──┬→ P3.4 滤镜家族（软：色彩空间感知滤镜可选受益）
       └→ P3.7 2.5D（HDR/泛光子项）

P3.2 自定义 shader ──→ P3.7（自定义材质子项）        ← 本次新增边
P3.4 ──→ P3.8（SVG <filter> 语义子项）               ← 本次新增边
P3.3 富文本逐 run 样式 ──→ P3.8（SVG text/tspan、COLRv1 完整层）
P3.5 ──→ P3.7（Matrix4 透视子项）；──→ P3.8（维持现记）
P3.6 ──→ P3.7（软：GPU mipmap 消费关系）             ← 本次新增边
```

## 2. 节点表（范围 / 交付物 / 依赖）

| 节点 | 范围（PLAN §5/§6 对应项） | 关键现状（详证 assessment.md §1） | 依赖 |
|---|---|---|---|
| P3.1 色彩管理地基 | §5.1 色彩管理 HDR/广色域/ICC；§6 线性空间混合 | `GetBlendFuncLinear` 全仓 0 生产调用；GPU/CPU 主路都在 sRGB 空间混（`blend_gpu.go:55`、`software_blend.go:32`）；公开层零色彩 API；双后端 Float RT 格式能力已备未用；sRGB 交换链三端缺 | P3.0 |
| P3.2 自定义 shader | §5.1 自定义/运行时着色器；§4 矩阵三端缺 | render 公开层零 shader API；hal `CreateShaderModule(WGSL)` 现成（`hal/api.go:235`）；内部 25 个 WGSL go:embed 直喂 hal；翻译器 `gpu/shader` 仅 gles/metal 后端在用 | P3.0、P2.1 ✅ |
| P3.3 富文本 | §5.2 逐段样式/Justify/ellipsis/maxLines；§6 DrawParagraph 逐 run | 无 Span/RunStyle/Paragraph 类型；Justify 有常量无实现（`layout.go:30,617`）；ellipsis/maxLines 零命中；DrawParagraph 单色自述 `context_misc_draw.go:270-276` | P3.0 |
| P3.4 滤镜家族 | §5.1 MaskFilter/ColorFilter 四族/ImageFilter 八族 | 已有 blur/colormatrix(4x5+9 预设)/dropshadow 双链路；缺 8 族 ImageFilter+ColorFilter 四族+MaskFilter 概念+Paint 级字段 | P3.0；P3.1（软） |
| P3.5 路径与几何 | §5.1 conic/四角RRect/Matrix4/路径效果/布尔保曲线 | verb 无 Conic；RRect 单半径；Matrix 2x3 仿射；dash+两个 1D 子集已有；布尔 Clipper 版已有但曲线 flatten | P3.0 |
| P3.6 图片格式+mipmap | §5.1 六种格式+EXIF+mesh gradient；GPU mipmap 链 | 解码仅 PNG/JPEG/WebP 静态；动图/EXIF/mesh gradient 零命中；全部纹理 `MipLevelCount:1`，采样器 MipmapFilter 已备空转 | P3.0 |
| P3.7 2.5D | §5.3 五行 | 引擎库已有 Projector/YSort25D/Shadow25D（`engine/camera/`）；缺 render/matrix 4×4 与透视绘制口；材质/HDR/泛光/mipmap 均等上游 | P3.0、P3.1、P3.2、P3.5、P3.6（软） |
| P3.8 SVG/彩色字形/Picture | §5.1 SVG 完整语义；§5.2 COLRv1 完整/嵌套 Picture | SVG 现支持 8 元素基础属性，defs/use/渐变/clipPath/text/filter 全 skip；COLR 实际只解析 v0 表（v1 被误读）；recording 无嵌套引用命令 | P3.0、P3.3、P3.4、P3.5 |

## 3. 边修订对照（D1 拍板 5 条，台账 §2 依赖列已同步）

| # | 台账原记 | 修订后 | 依据 |
|---|---|---|---|
| 1 | P3.3 依赖 P3.0、P3.1 | **P3.0** | 逐 run 颜色/装饰样式不经过混合空间；linear.go 与富文本无技术交集（assessment §1.1/§1.3） |
| 2 | P3.4 依赖 P3.0、P3.1 | **P3.0 + P3.1（软）** | offset/erode/tile/merge 等几何滤镜不依赖色彩空间；blend/lighting 族在线性空间可选受益 |
| 3 | P3.7 依赖 P3.0、P3.1、P3.5 | **P3.0、P3.1、P3.2、P3.5、P3.6（软）** | 自定义材质依赖自定义 shader（PLAN §5.3 明写，原漏记）；GPU mipmap 由 P3.6 交付、2.5D 消费 |
| 4 | P3.8 依赖 P3.0、P3.3、P3.5 | **P3.0、P3.3、P3.4、P3.5** | SVG 完整语义含 `<filter>`（PLAN §5.1 明列，原漏记） |
| 5 | （P3.2 依赖不变） | P3.0、P2.1（✅） | 维持；hal 口现成，主工作量在 render 新 API 面 |

## 4. 实现顺序（D2 拍板：推荐主序，串行）

```
P3.1 → P3.2 → P3.3 → P3.4 → P3.5 → P3.6 → P3.7 → P3.8
```

| 位次 | 节点 | 理由 |
|---|---|---|
| 1 | P3.1 | 前移项；P3.4（软）/P3.7（HDR/泛光）的共同地基；PLAN §7.8-4 明文「线性混合是 HDR 地基」 |
| 2 | P3.2 | 只依赖 P2.1（✅）；hal 口现成，早期见效 |
| 3 | P3.3 | 前移项（不得挪回 P4）；P3.8 的 text/tspan/COLRv1 依赖它 |
| 4 | P3.4 | P3.8 `<filter>` 依赖它；软受益于 P3.1 已就位 |
| 5 | P3.5 | 不被 P3.1–P3.4 依赖、不阻塞任何人；Matrix4 是 P3.7 前置 |
| 6 | P3.6 | 同上，最独立；mipmap 供 P3.7 消费 |
| 7 | P3.7 | 消费方最多（P3.1+P3.2+P3.5+P3.6） |
| 8 | P3.8 | 消费 P3.3+P3.4+P3.5，且 SVG `<filter>` 需要滤镜家族齐 |

并行说明：台账 §2「P3.2–P3.8 在 P3.0（及各自前置）完成后可分批并行」的技术可行性保留；拍板采用串行（GPU 真窗隔离 :3 串行排期是瓶颈，每项独立分支+全门禁）。前移项位置核对：P3.1=第 1、P3.3=第 3，均满足「不得挪回 P4」。

## 5. 各项交付时的文档义务（台账 §3 P3.1–P3.8 口径）

每项开工登记时补四样（P3.0 不越位代办）：能力定义（对齐 Skia/Flutter 对应物）、公开 API 面（先把 catalog §13 对应规划行转正式登记）、双后端实现方式（不支持的端显式标注）、验证方案（逻辑探针+像素断言+金图三证据+负向对照）。

## 6. 自校验清单（机器可复现）

```bash
# 现状抽查（全部应零命中/如注，逐条对照 assessment.md §1）
grep -rn "GetBlendFuncLinear\|BlendLinear\|BlendModeLinear" --include=*.go . | grep -v _test | grep -v "blend/linear.go"   # → 0 行
grep -rn "MaskFilter" render/ --include=*.go | grep -v _test                                                              # → 0 行
grep -in "conic" render/path_verb.go render/path.go | grep -v _test                                                       # → 0 行
grep -rin "meshgrad\|mesh gradient" render/ --include=*.go | grep -v _test                                                # → 0 行
grep -rin "exif" render/ --include=*.go | grep -v _test                                                                   # → 0 行
grep -rn "DrawRecording" render/recording/ --include=*.go | grep -v _test                                                 # → 0 行
grep -rn "MipLevelCount:" render/internal/gpu/*.go render/*.go | grep -v _test | grep -vE "MipLevelCount:\s*(1|0)" | grep -v "internal/gpu/texture.go"  # → 0 行（创建点全为 1，共 79 处；texture.go 是描述符搬运层与 0=全层哨兵——首轮宽模式误报 18 行已逐行定性）
# 依赖列一致性：台账 §2 表 P3.3/P3.4/P3.7/P3.8 四行 与本文 §3 修订后一致
# apidoc：go run ./scripts/apidoc → rc=0（留档 apidoc-p30.txt）
```
