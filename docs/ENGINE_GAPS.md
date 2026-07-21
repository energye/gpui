# 引擎必有缺口（Engine-scoped gaps）

> 版本：1.1 | 日期：2026-07-21  
> 范围：**仅渲染引擎职责**（`render` / `gpu/webgpu` / `gpu/rwgpu`）  
> 真源：现网代码；与本文冲突时以代码为准  
> 非目标：布局 / HitTest / 焦点 / IME 状态机 / 控件树（控件层）

---

## 0. 结论（给控件层与排期）

| 判断 | 说明 |
|------|------|
| **画布 API 能否支撑 antd 风格绘制** | **能**。Skia 2D 画布矩阵除 R.02 PDF/SVG 外已齐（见 `SKIA_2D_CAPABILITY_MATRIX.md`） |
| **引擎侧是否零缺口** | **否**。下面 **G1–G3 为主缺口**（G1 含 CFF/shaping 子项）；其余为次级/性能 |
| **控件层是否可开工** | **可开工**（S5.5 条件已满足）；G1–G3 影响 **深度/性能/生产稳**，不是「缺一整块绘制 API」 |

---

## 1. 主缺口（必跟）

### G1 — 文本栈深度与正确性

| 子项 | 现状（代码） | 影响 | 证据 |
|------|--------------|------|------|
| **G1.a 长文 / Input 路径** | shape + atlas + DrawString 可用；密集编辑/长列表下 reshape、cache、atlas 压力未按产品级压满 | Input、表格、虚拟列表滚动 | `render/text/*` · 能力 X.* 有门禁但非编辑器 soak |
| **G1.b CFF / CFF2 轮廓** | own parser **仅 TrueType `glyf`**；CFF 返回零框 / 不出字 | 系统 OTF、多数 Noto CJK | `render/text/font_parser_own.go`：「CFF fonts are not yet supported」· `glyf_parser.go` |
| **G1.c 复杂 OT shaping** | **GSUB** 已实现 Type **1–4、7**；**5/6/8**（contextual/chaining/reverse）未实现。**GPOS** 已实现 Type **1–2**；**3–8**（cursive/mark/context…）未实现 | 阿拉伯/印地等；mark 附着 | `render/text/gsub.go` · `gpos.go` · `Script.RequiresComplexShaping` |

**验收方向（引擎）：**

- 常用桌面/CJK **OTF(CFF)** 可测出字与度量  
- 目标 script 集合 shaping 像素/度量门禁  
- Input 形态长 soak：atlas 上传有界、无 RSS/VRAM 斜率爆炸  

---

### G2 — 矢量路径下脏区 / 合成效率

| 子项 | 现状 | 影响 | 证据 |
|------|------|------|------|
| **G2.a MSAA 矢量帧** | 含 Fill/Stroke 时走 MSAA，**LoadOpClear**；`damageRect` 不保留旧像素 | 局部脏区无法「只重画几像素」 | `render/context.go` `FlushGPUWithViewDamage` 注释 |
| **G2.b  blit-only 路径** | 仅 `DrawGPUTexture*` 时可 LoadOpLoad + scissor | 控件应用层缓存 RT 时有效 | 同上 · 对齐 Chrome/Flutter 分层实践 |
| **G2.c OS Present damage** | `Surface.PresentWithDamage` **忽略 rect**（wgpu-native 不支持） | 无 OS 多矩形 present 省电收益 | `gpu/webgpu/surface.go` |

**契约（写进产品预期）：**

- 引擎 **提供** damage API 与 scissor  
- **不承诺**「任意矢量脏区 = 仅重画脏矩形像素」  
- 稳 60fps 依赖：**轻脏 UI / 分层 RT / 少全屏滤镜**；重全帧矢量不保证 60  

---

### G3 — 重层 + 滤镜 + 多 RT 稳定性（含 lifecycle/VRAM）

| 子项 | 现状 | 影响 | 证据 |
|------|------|------|------|
| **G3.a 多离屏 / filter 图** | API 齐；重场景预算与 soak 需持续 | Modal/Drawer/毛玻璃/多路由 RT | `api_coverage_app` · particle · mem 护栏 |
| **G3.b Device lost / recover** | sticky lost · AutoRecover · `ForceRecoverHealthy` · Context 注册表 abandon · CB/pool/pipeline 回收 | 遮挡/TDR 后 OOM、双 Device 堆 | `GPU_修复_device_lost.md` |
| **G3.c Surface lifecycle** | tier Normal/Purge/Recreate 已实现；**auto 默认 Purge**（非离散=Normal）；OOM→Recreate；Unconfigure 钩子 purge | 最小化/恢复跨硬件 | `SURFACE_LIFECYCLE_SKIA_FLUTTER.md` · `lifecycle_policy.go` · `exboot/lifecycle.go` |
| **G3.d VRAM 基线** | 独显 Vulkan 设备基线高（本机约 300MiB 级）；策略可 low-power | 弱显存机 | `VRAM_BUDGET.md` |

**验收方向：** lifecycle matrix + force-lost + 重场景 mem guard 持续绿；OOM 自适应升档。

---

## 2. 次级 / 性能 polish（不挡控件层开工）

| ID | 项 | 说明 |
|----|-----|------|
| P1 | 嵌套 path-clip 恢复限制 | `depth_clip` 单 path/group；antd 多为 rect/rrect |
| P2 | Path boolean 质量 | UI soft path，非 CAD 级 |
| P3 | F16 / 宽色域全 Context 链 | RT 有；Context 仍 8-bit（矩阵 CS.02） |
| P4 | N3–N5 冷路径 GPU 化 | CustomBrush fragment / Bicubic / 极冷 path effect（`GPU_FIRST_ROUTING` 后置） |
| P5 | Backdrop 无 readback 拷贝 | 可选性能 |
| P6 | COLR/SVG emoji 深度 | 部分 emoji 路径有 TODO |

---

## 3. 明确非引擎缺口

| 项 | 归属 |
|----|------|
| Flex/Grid 布局、组件状态 | 控件层 |
| HitTest 树、焦点路由 | 控件层 |
| IME 组合态、按键语义 | OS + 控件层 |
| 滚动 offset / 虚拟列表策略 | 控件层 |
| 光标闪烁/选区**状态** | 控件层（引擎只画几何） |
| 无障碍树 | 非 GPU 画布 |
| PDF/SVG 文档后端 R.02 | 旁路（`DOC.1` 专项，不挡画布 100%） |

---

## 4. 与文档 / 代码映射

| 缺口 | 活文档 | 关键代码 |
|------|--------|----------|
| G1 | 本文 · 矩阵 X.* | `render/text/` |
| G2 | 本文 · 矩阵 S.09（API ✅，效率有界） | `render/context.go` · `gpu/webgpu/surface.go` |
| G3 | `SURFACE_LIFECYCLE_*` · `GPU_修复_device_lost` · `VRAM_BUDGET` · `MEM_LEAK_*` | `render/gpu/lifecycle_policy.go` · `context_gpu_registry.go` · `gpu/webgpu/swapchain.go` |
| 原则 | `GPU_FIRST_ROUTING` | ensureGPU / fallback 观测 |

---

## 5. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-21 | 1.0 | 首版：从源码与 antd 引擎职责评估收敛；归档旧方案中的重复缺口叙述 |
| 2026-07-21 | 1.1 | 多轮源码对照：GSUB/GPOS 类型边界；lifecycle auto=Purge；去掉错误「G4」表述 |


---

## 6. 源码对照检查（多轮）

| 轮次 | 检查项 | 结果 |
|------|--------|------|
| R1 | G1.b `font_parser_own.go` CFF 注释；`glyf_parser.go` CFF 无 glyf | ✅ |
| R1 | G1.c `gsub.go` Type 5/6/8；`gpos.go` Type 3–8 | ✅ |
| R1 | G2.a `context.go` FlushGPUWithViewDamage MSAA LoadOpClear | ✅ |
| R1 | G2.c `surface.go` PresentWithDamage 忽略 rect | ✅ |
| R2 | G3 `NoteTextureOOM` / `ForceRecoverHealthy` / `requestDeviceWithVRAMProbe` / registry abandon | ✅ |
| R2 | lifecycle `auto`→**Purge**（含 dGPU）；仅 env `normal`→Normal | ✅ |
| R3 | 活文档引用的路径均存在（无悬空契约文档链） | ✅ |
| R4 | SurfaceHost：Normal **不** Unconfigure；Purge/Recreate 才 Unconfigure+DropGPU | ✅ |
| R5 | VRAM `ResolveAdapterPolicy` 默认 HighPerformance；`RequestAdapterWithPolicy` 签名 | ✅ |
