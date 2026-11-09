# GLES 后端深度门控失效定位：glClearDepth/glDepthRange 桌面 GL double 签名按 float32 传参（P4.1 批 4，2026-11-10）

> 性质：只读诊断 + 临时插桩（待还原）。发现于 even-odd MSAA（批 4 子项①）红测实施中：
> INVERT 管线已就位但画面完全不受 Phase 1 stencil 影响 → 逐层实验锁定深度门控整条死。
> **与 go4x 黑洞（resolve READ 绑定，已修已验）是同一「GLES 4x 链」上的第二、第三个独立洞。**

## 0 直话

- **一行直话：桌面 GL 上下文里 `glClearDepth`/`glDepthRange`（double 签名）被按 float32
  的 FFI 调用约定传参——float32(1.0) 的位型被当 double 读 ≈ 1.4e-312 ≈ 0，深度清屏
  清成 0、深度映射范围压成 [≈0,≈0]，深度门控（GPU-CLIP-003a 的 GreaterEqual）从上线
  起在桌面 GL 路径上就没生效过。**
- 1x 不暴露：sampleCount==1 走 R8 coverage mask 门控（BuildClipMask），深度门控冗余。
- 4x 才暴露：MSAA>1 时 mask 被跳过（depth_clip.go:625 既有登记），深度是唯一逐像素
  门控——死了就只剩 clip 包围盒 scissor 粗裁。**这正是 even-odd 4x 验收被卡的第二层原因**
  （第一层=孔洞被 winding stencil 保留，已修）。

## 1 复现与实测（三样齐）

### 1.1 复现

`render/clip_evenodd_msaa_test.go :: TestClipEvenOddMSAACasesGPU/evenodd_ring_clip_msaa4`
（先红后绿红测，4x 真生效前提下）：INVERT 管线分支确认执行（临时插桩
`evenOdd=true eoPipe!=nil=true sampleCount=4`），但孔洞仍红。

### 1.2 判别实验链（临时插桩 GPUI_EO_DEBUG/GPUI_EO_OP，全部待还原）

| 实验 | 预期（若深度门控活着） | 实得 | 结论 |
|---|---|---|---|
| eo 管线 PassOp=Zero（stencil 恒 0→cover 全跳→全绿） | 画面全变 | **与 winding 图完全相同** | Phase 1 stencil 对最终画面零影响 |
| skipcover（Phase 2 整个跳过，深度永不写） | 内容全被 1.0 深度拒掉→全绿 | 同上 | 内容门控不走 cover 深度 |
| skipall（整个 depth-clip 跳过） | 应失去逐像素裁剪 | 同上 | 门控完全在 depth_clip 之外=深度门控已死 |
| noconvex / nosdf（内容层二分） | 红矩形消失 | nosdf 时消失 | 内容=SDF 层（hasDepthClip→GreaterEqual） |

### 1.3 缓冲区直读（决定性）

- `ClearDepth depth=1.000000` 执行后**立即** ReadPixels(GL_DEPTH_COMPONENT)：
  `depth@(64,64)=0.000000 depth@(64,116)=0.000000`，glErr=0x0，FBO 绑定正确（=1），
  DepthMask 已强制 true——**clear 执行了、没报错、缓冲没变**。
- 内容 draw 时刻四探针点（孔/环/外/角）深度全部 0.000000（应为 1.0 清屏值/环内 0.0）。
- Draw 时刻 GL 状态读取：eo fill `frontPassOp=0x150a(GL_INVERT)` ✓、cover
  `frontFunc=0x205(GL_NOTEQUAL)` ✓——管线状态本身已正确到达 GL。
- `GL_VERSION=4.6 (Core Profile) Mesa 23.2.1` → **桌面 GL 上下文**（X11 路径
  `api_linux.go:71 config.GLES=false` 优先桌面 GL，本机成功）。

## 2 根因（file:line）与标准方案

### 2.1 根因

`gpu/gwgpu/gles/gl/context_linux.go`：

- `:857-864 ClearDepth`：无条件按 `cifVoid1Float`（float32）调用 `c.glClearDepth`。
  加载端 `:604-608`：GLES 上下文加载 `glClearDepthf`（float，匹配）；**桌面 GL 上下文
  加载 `glClearDepth`（double 签名）→ float32 位型被当 double 读 → 1.0 变 ≈1.4e-312**。
  注释「Linux GLES contexts always load glClearDepthf, so we pass float32
  unconditionally」的假设对桌面 GL 路径不成立。
- `:866-871 DepthRange`：同类——桌面 `glDepthRange(near, far double)` 被按
  `cifVoid2Float` 传两个 float32 → far≈0 → 深度范围 [≈0,≈0] → 所有深度写入/比较 ≈0。
  该函数每次 pass begin 经 SetViewportCommand（command_execute.go:828）调用。
- 加载端 `:721-724`：GLES 加载 `glDepthRangef`（float，匹配）；桌面加载 double 版。

### 2.2 标准方案

GL 规范签名：桌面 `void glClearDepth(GLdouble)` / `void glDepthRange(GLdouble, GLdouble)`；
GLES `void glClearDepthf(GLfloat)` / `void glDepthRangef(GLfloat, GLfloat)`。
FFI 调用约定必须按上下文 API 分派：isGLES→float 路径（现状保留），桌面→double 路径
（新增 `cifVoid1Double`/`cifVoid2Double`，`types.DoubleTypeDescriptor`——现文件不存在
任何 double cif，全文件审计仅 glClearDepth/glDepthRange 两处 double 签名函数）。

### 2.3 修法草案

1. `initCommonCallInterfaces` 增加 `cifVoid1Double`、`cifVoid2Double`。
2. `ClearDepth`/`DepthRange` 按 `c.isGLES` 分派 float/double cif。
3. 同类审计：加载表中其余符号无第三处 double 签名（glDepthBoundsEXT/glGetDoublev
   未加载），修完即闭合。

### 2.4 影响面

- 仅 GLES 后端的桌面 GL 上下文（X11/Wayland 默认路径）；GLES 上下文行为不变；
  native 零变化。
- 修复后桌面 GL 的深度门控**首次真正生效**：sampleCount==1 有 R8 mask 兜底，画面
  预期不变（金图回归验证）；MSAA>1 深度门控激活——这正是 even-odd/winding 4x 裁剪
  正确性的前提。
- 1x 金图回归（desktop GL 1x 窗）必须跑：深度门控激活后与 mask 门控叠加，
  预期逐位一致（winding stencil 与 mask 同语义），红了按差异定位。

## 3 与已修 go4x 洞（resolve READ）的关系

同一「GLES 4x MSAA 链」上三个独立洞，全部要修完 even-odd 4x 才能收口：

| # | 洞 | 状态 |
|---|---|---|
| 1 | `ensureResolveFBO` 首用裸 FRAMEBUFFER bind 毁 READ → 离屏读回全零（快照黑） | ✅ 已修已验（用户拍板 DRAW-only） |
| 2 | `glClearDepth` 桌面 double 签名按 float32 传参 → 深度清屏≈0 | 本报告，待拍板 |
| 3 | `glDepthRange` 同类 → 深度范围 [≈0,≈0] | 本报告，待拍板 |

#3（PopLayer 层链族）依旧不同根：那四个洞 1x 双后端可复现，本报告两洞只在
桌面 GL 上下文生效（GLES 上下文签名匹配）。
