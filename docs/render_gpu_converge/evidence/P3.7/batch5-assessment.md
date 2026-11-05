# P3.7 批 5（泛光 RGBA8 近似）· 影响面评估（2026-11-05，落盘待拍板，未开工）

判据来源：`design.md` §6（做/参数真源合一/显式声明 LDR/真 float 派生为 M5 专项）、
拍板 D1（RGBA8 近似，不冒充 HDR）与 D5。前置 M6 已修（`4846add3`）。
本文件只登记实测现状与缺口，不含代码变更。

## 1 现状实测（与 design §6 的登记逐条对表）

| design §6 说的可复用件 | 实测今天 | 结论 |
|---|---|---|
| P3.4 GPU 滤镜图 `filter_gpu_graph.go:64-67,842` | 文件真身在 **`render/internal/gpu/filter_gpu_graph.go`**（不在 `render/` 根）；13 个 `ImageFilterKind`（`render/filter_ops_gpu.go:432-460`）：Blur/BlurXY/DropShadow/ColorMatrix/Grayscale/Invert/Erode/Dilate/Offset/AlphaThreshold/Lighting/Tile/Displacement | 路径与行号需按真身重定；**无 bloom/亮部提取类 kind** |
| 「blur/merge/aux_tex 双输入」 | 高斯是**可分离**且与 CPU 逐核对齐（`filter_gpu_graph.go:238-252`：`half=ceil(3σ)`，与 `CachedGaussianKernel` 同式）；`aux_tex`（binding 3）确有双输入，但那段是 **drop shadow 的「content over shadow」**（`:188-190`），**不是加法合成** | 「merge」≠ 泛光要的 additive；亮部叠加要新 pass 或新 blend |
| 「降采样模糊」 | **无任何降采样/多级 pass**（图里没有 downsample/mip 级链，模糊只有全分辨率单遍可分离高斯） | 泛光的「降采样」要么真造多级 pass，要么改用大 σ 全分辨率近似 ⇒ **待拍板** |
| `render/hdr` 的 Reinhard 曲线（CPU，零 importer） | `render/hdr/float_target.go` 只有 `FloatTarget`（CPU `[]float32`、`TextureView()`/`Pixels()` 返回 nil），**全仓无 Reinhard/Tonemap 曲线函数**（此句已作废，见文末勘误 C1：导出面确实没有，包内有未导出的 `reinhard()` 且 `Encode` 在用它） | **design 引用不存在**；调子映射的真身在 `engine/fx/lut.go`（四档 tonemap + `ApplyTonemap(exposure)`），属引擎层 |
| `engine/fx.Bloom{Threshold,Radius,Intensity}`（`bloom.go:175-179`） | 真身在 `engine/fx/bloom.go:184`（`NewBloom`）+ `Validate`（`:195`）+ `IsIdentity`（`:211`）+ `Apply`（`:216`）；语义实测为：`brightMask(luminance, threshold)` **软斜坡**（`lum<=thr→0`，向上线性升到 1，`threshold==1` 处处为 0）→ `boxBlur`（**盒式，非高斯**）→ `原色 + 模糊×intensity` 三通道各自 `clamp01` | 存在且**零消费者**（`fx.Bloom`/`Bloom(` 在 `engine/fx/` 外零命中；仅被 `NewGrade` 参数用） |

## 2 缺口清单（本批要造的东西）

1. **亮部提取**：GPU 侧无对应 pass；CPU 有 `brightMask` 软斜坡。GPU 若照抄需新增 kind 或扩 ColorMatrix 语义。
2. **模糊核不一致**：CPU 泛光用 **boxBlur**，GPU 现成高斯是**可分离 Gaussian** ⇒ 两路像素必然不等值（这是「参数合一但语义两路」的真风险，必须显式声明或统一核）。
3. **加法合成**：现有 aux 双输入是 over 合成，泛光要 additive（`src + bloom×intensity`）+ 夹到 1（LDR 的本质）。
4. **降采样**：完全缺失；且 RGBA8 上做亮部提取，>1.0 信息本就不存在（D1 已认，声明即可）。
5. **参数真源合一**：design 要求「GPU 链的参数类型为唯一真源，CPU `fx.Bloom` 改走同一参数」。今天参数在 **engine 层**（`fx.Bloom`），render 侧无任何泛光参数类型 ⇒ 先定「真源放哪一层」再谈合一。
6. **三处 LDR 声明**：文档 + 真窗探针 + catalog；**真窗在批 6 ⑤**（本批无窗可写探针）⇒ 本批只能落两处，第三处随窗收口，需在台账显式挂账。

## 3 需要拍板（每项给推荐）

- **P-1 泛光链放哪一层**（AGENTS「渲染层只做通用难事；玩法与业务归模块库」）：
  A 渲染层新增通用 pass（亮部提取→模糊→加法合成，任意图形可复用，参数类型落 render，engine/fx 改吃它）；
  B 引擎层用现成件组合（ColorMatrix 近似阈值 + Blur + 自绘加法 blend），render 不动。
  **推荐 A**：三段都是任意图形可复用的图形算法，B 会把阈值/夹色语义散进玩法层，且 CPU/GPU 两路更难以一源对齐。
- **P-2 CPU/GPU 语义怎么对齐**：**推荐** GPU 链采用「可分离高斯 + 软阈值斜坡」（与 CPU 同形状），
  **并显式登记 CPU 侧 `boxBlur` 与 GPU 高斯不等值**（要么把 CPU 路也换成高斯以同式，要么保留差异并标注为 LDR 近似两路）。
  推荐**把 CPU `fx.Bloom` 的 boxBlur 也换成同一可分离高斯**（消掉第二套算法），代价是 CPU 路画面变化需重出金图。
- **P-3 降采样做不做**：**推荐本批不做真多级降采样**，用「大 σ 全分辨率高斯 + 加法」近似（成本可控、无新 pass 机制），
  并在文档/catalog 写明「半径=σ，不是 mip 级数」；多级降采样留给真 HDR（M5）专项一并做。
- **P-4 半径/强度上限怎么合一**：CPU 现有 `MaxBloomRadius=16`/`MaxBloomIntensity=4` 是引擎数字；参数真源上移到渲染层后，
  **推荐渲染层定义域（含 0=恒等）、引擎层保留各自的玩法上限校验**（引擎可以比渲染更严，不可更宽）。
- **P-5 验证到哪一步**：**推荐**本批 = 单测逐通道断言（阈值斜坡/夹色/恒等/intensity=0 逐位不变）+ 负向（摘加法、摘夹色、摘阈值必红）
  + conformance 双后端；**状态只能 ☑ 已交付待金图（收口=批 6 ⑤ 泛光窗）**，且 LDR 声明的第三处（真窗探针）随窗补，挂账不隐。

## 4 影响面（改动会落在哪）

- 渲染层：`render/internal/gpu/filter_gpu_graph.go`（新 pass + WGSL 核）、`render/filter_ops_gpu.go`（kind 与参数）、
  `render/filter_ops_cpu.go`（CPU 回退路语义）、新增参数类型所在文件。
- 引擎层：`engine/fx/bloom.go`（改吃渲染层参数类型；若 P-2 选统一高斯则换核）、`engine/fx/lut.go`（Grade 组装点）。
- 不动：`render/hdr`（M5 专项）、呈现链（无 float 输出）。
- 风险：新 pass 数量上升影响帧耗时（泛光是 2~3 遍全屏/半屏）⇒ 需按性能红线在批 6 窗上量；本批先加计数与探针不做门。
- 回归面：`render`、`engine/fx`、`engine/renderconv` 三包逐文件 + conformance + race。

## 勘误（2026-11-05 步骤 1 落地时实测，原文保留不删）

- **C1 本文件第 14 行「`render/hdr` 全仓无 Reinhard/Tonemap 曲线函数」写错了。** 实情：`render/hdr/float_target.go:153`
  就有未导出的 `reinhard(x) = x/(1+x)`，注释还明写「same math as fx Reinhard」，`:129` 的 `Encode(exposure)` 正在用它
  （Reinhard → sRGB → 8-bit）。要收回的只有「**导出面**没有调子映射口」这一半。这条错判的直接后果是我先拍错了方向：
  因为它，我把「泛光核只能落 8-bit `Pixmap`」当成前提，进而判断「fx 薄转调必然把 float64 链打到 1/255」；
  步骤 1 动手前实测到 `FloatTarget` 本身就是 CPU 线性 float32、>1 合法（`:39-44`），前提塌了，
  拍板随即从「(a) 提升核为公开口 + 并存」改为 **A：核落 `render/hdr` float 域、fx 薄转调**（`batch5-design.md` §7）。
- **C2 「`fx.Bloom` 在 `engine/fx/` 外零消费者」按字面也不准。** 实测：`examples/engine/fx/main.go:175` 就在调
  `fx.NewBloom`（真窗示例），`ui/kit` 与 2.5D 模块库确实零调用。准确说法是**生产零消费者、示例一处在调**——
  这条差别在本次动手时直接影响改动面（示例调用点与 README 必须同笔切 `Radius`→`Sigma`）。
