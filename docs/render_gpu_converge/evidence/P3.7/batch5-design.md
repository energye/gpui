# P3.7 批 5（泛光 RGBA8 近似）· 实施方案（2026-11-05，四项拍板齐，待开工）

前置：`batch5-assessment.md`（现状与缺口实测）；M6 已修（`4846add3`）。
本批三条拍板已定：**P-1 渲染层新增通用 pass** / **P-2 CPU 也换可分离高斯（两路同式）** /
**P-3 不做真多级降采样，用全分辨率大 σ 近似**。本文件写落地形态，拍完再写码。

## 1 pass 结构（渲染层，一次 encoder 内多 pass——机制现成）

| # | pass | 输入 → 输出 | 要新增的东西 |
|---|---|---|---|
| 1 | 亮部提取 bright pass | src → 掩膜图 | WGSL 核：`lum=dot(rgb, luma)`，`m=clamp((lum-thr)/(1-thr),0,1)`（与 CPU `brightMask` **同式软斜坡**，`thr==1` 处处 0）；乘 rgb 得 `rgb*m`，alpha 置 0 |
| 2/3 | 可分离高斯 H、V | 掩膜图 → 模糊图 | **复用现有高斯核**（`filter_gpu_graph.go:238-252`，`half=ceil(3σ)`，与 CPU `CachedGaussianKernel` 同式），σ=半径；全分辨率、不降采样（P-3） |
| 4 | 加法合成 additive | src + 模糊图×intensity → dst | 现有 aux 双输入只有 **over**（content-over-shadow，`:188-190`），**additive 是新语义**：`dst=clamp01(src+aux*intensity)`，alpha 保留 src |

`clamp01` 是 RGBA8 近似的本质（>1.0 无表示），三处显式声明「LDR 近似，非 HDR」——本批落**文档 + catalog** 两处，
第三处（真窗探针）随批 6 ⑤ 泛光窗，**挂账不隐**。

## 2 参数真源（P-1 的必然结果：定义在渲染层，引擎吃它）

```go
// render：泛光参数唯一真源（Threshold 域 [0,1]；Sigma 为高斯标准差，全分辨率近似 ⇒ 不是 mip 级数；
// Intensity==0 恒等）。上限域值归渲染层定义，引擎可更严不可更宽（P-4 推荐）。
type BloomParams struct{ Threshold, Sigma, Intensity float64 }
func (p BloomParams) Validate() error
func (p BloomParams) IsIdentity() bool
```

- 引擎侧 `engine/fx.Bloom{Threshold, Radius int, Intensity}` 的 **`Radius int` 换成 `Sigma float64`**
  （两路同式的必然：盒半径与高斯 σ 不可互通）。属**公开 API 变更**，按接口全量切换制同批改完所有调用点，
  不留垫片；`MaxBloomRadius=16`/`MaxBloomIntensity=4` 保留为**引擎侧玩法上限**（校验委托渲染层的域 + 自身更严夹）。
- `NewGrade`（`lut.go:305`）组装点随之换型。CPU 侧 `fx.Bloom.Apply` 的 `boxBlur` 删除（见 §3）。
- 待登记进目录的公开面：`BloomParams`/`Validate`/`IsIdentity` + 新 `ImageFilterKind` 一枚（如 `ImageFilterBloom`）
  + §3 的 CPU 核公开口；catalog 同步 + `go run ./scripts/apidoc` rc=0 是硬门禁。

## 3 P-2 的落地方式（有硬约束，三选一，已拍 (a)）

事实：CPU 高斯核在 `render/internal/filter/kernel.go:138 CachedGaussianKernel`，
Go 的 internal 规则使 **engine 层引不到它**（实测 `engine/` 对 `render/internal/...` 零 import，只有 `engine/tex/doc.go:56,59` 的注释提及）；
render 的 CPU 模糊公开口只有绑画布的 `Context.ApplyBlur/ApplyBlurXY`。

| 方案 | 做法 | 代价 |
|---|---|---|
| **(a) 提升核为渲染层公开口**（推荐） | render 新增 `func GaussianKernel(sigma float64) []float32`（转发 internal 的 `CachedGaussianKernel`，同一处实现），engine/fx 用它把 `boxBlur` 换掉 | 多一枚公开口（属通用图形工具，合规）；engine/fx CPU 泛光画面会变（需重出相关金图/测试期望） |
| (b) engine/fx 自带高斯核 | 引擎内再写一份核 | **第二套实现**，AGENTS 明禁 |
| (c) CPU 泛光整链交给 render | `fx.Bloom` 不再自己算像素，只做参数与组装；CPU 回退走 render 滤镜链 | 最"合一"，但动 fx 包能力面与 Grade 链，范围显著大于批 5 |

⇒ **已拍 (a)**（2026-11-05 用户拍板）：render 新增公开 `GaussianKernel(sigma)` 转发 `CachedGaussianKernel`，
engine/fx 用它替换 `boxBlur`，同一处实现、引擎只写薄对接；(c)（CPU 泛光整链交回 render）另案——
「fx 的 CPU 后处理与 render 的 CPU 滤镜链是否双实现」单列为架构观察项（M 族），本批不处理，登记不隐。

## 4 恒等与边界语义（必须可钉）

- `Intensity == 0` ⇒ 恒等，输出与输入**逐位不变**（早退，不进 pass 链）。
- `Threshold == 1` ⇒ 掩膜处处 0 ⇒ 加法项为 0 ⇒ 等价恒等（CPU 现状如此，GPU 必须一致）。
- 加法后 `clamp01` 逐通道；alpha 保留 src（不改透明度）。
- 零值 `BloomParams{}` 必须是一个合法且不改变画面的默认（与批 4 的 `MipmapNone` 同理，防"默认值漂"）。
- 非法参数（σ≤0、Threshold 越界、Intensity 负）一律具名错误，不猜。

## 5 验证与状态

- 钉（GPU+CPU 两路）：阈值斜坡逐值、加法与夹色、恒等逐位（Intensity=0 与 threshold=1 两条）、
  两路同式对齐（同参数 CPU 与 GPU 输出在声明容差内，容差显式写进测试）、新 kind 路由到四 pass 结构。
- 负向（摘掉必红，至少四条）：摘亮部掩膜、摘加法合成、摘 clamp01、摘 CPU 核替换（退回 boxBlur）。
- conformance 双后端一轮；`-race` 触及包；逐文件回归（`render`、`engine/fx`、`engine/renderconv`）。
- 性能：本批净增 3~4 个全屏 pass（bright + H + V + additive），**只加计数与探针、不做性能门**；
  按 design §批5 与红线，帧耗时实测挂批 6 ⑤ 泛光窗（连同 fps/p95 与第三处 LDR 声明）。
- **状态**：按 §4.1 两态口径，本批交付后只能是 **☑ 已交付待金图（收口批=批 6 ⑤）**——不造窗就不标 ✅。

## 6 影响面

- 渲染层：`render/internal/gpu/filter_gpu_graph.go`（新 pass + WGSL 核）、`render/filter_ops_gpu.go`
  （kind/参数/CPU 回退分派）、`render/filter_ops_cpu.go`（CPU 链语义对齐）、CPU 核公开口所在文件。
- 引擎层：`engine/fx/bloom.go`（换核 + 参数换型 + 校验委托）、`engine/fx/lut.go`（Grade 组装）、
  调用 `fx.Bloom`/`NewBloom` 的示例与测试同笔切换（实测 `fx.Bloom` 在 `engine/fx/` 外零消费者，改动半径小）。
- 不动：`render/hdr`（真 float 属 M5 专项）、呈现链、`engine/` 其它包。
- 文档：目录（新公开面 + LDR 声明一处）、台账 §2.1/§7、在册 M5 行保持 ⬜ 不动。
