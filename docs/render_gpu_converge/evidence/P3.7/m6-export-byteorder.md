# P3.7 探针副产物 · 新洞：`SetEffectSurface(true)` 下 `ExportImageBuf` 读出 R/B 互换（2026-11-03）

## 现象（复现）

同一份绘制，只切一个开关，导出面的红蓝整体对调：

| `SetEffectSurface` | 白底 | 红矩形 | 蓝图画在红矩形上 | GPUOps / CPUFallbackOps |
|---|---|---|---|---|
| `false` | (255,255,255,255) | **(255,0,0,255)** | (0,0,255,255) | 2 / 0 |
| `true` | (255,255,255,255) | **(0,0,255,255)** | (255,0,0,255) | 2 / 0 |

两档都真走 GPU（`GPUOps=2`、`CPUFallbackOps=0`），导出面两次都标称 `FormatRGBAPremul`、stride 64，**只有 R/B 两通道互换**（绿、alpha 不动；白底不变是因为对称）。原始日志 `probes/export-byteorder-ab.txt`。

复现代码（一次性探针，跑完删除）：`render.NewContext(16,16)` → `SetEffectSurface(x)` → `ClearWithColor(White)` → `SetRGBA(1,0,0,1)` + `DrawRectangle(2,2,8,8)` + `Fill()` → `DrawImage(蓝 8×8, 6,6)` → `FlushGPU()` → `ExportImageBuf(&out)` → `out.GetRGBA(4,4)`。

## 根因（读码定位到行号）

- `Context.syncViewFlushIntoPixmap`（`render/filter_ops_cpu.go:238`）把 GPU 面同步进 pixmap 时，**按「接口是否可用」而非「view 的真实纹理格式」选读回函数**：
  - `:260-261` 先试 `ReadbackViewRGBA`，注释与实现都写死「**BGRA8** offscreen → swizzle 成 tight RGBA8」（`render/internal/gpu/layer_gpu_io.go:24-26,45`）；
  - `:263-265` 只在「报错或长度不足」时才退到 `ReadbackViewStraightRGBA`（RGBA8Unorm 直取不 swizzle，`:51-53,72`）；
  - `:270` 无条件 `copy(c.pixmap.Data()[:need], rgba[:need])`，pixmap 的标称格式是 RGBA。
- `SetEffectSurface(true)`（`render/context_style.go:101-115`）会把该上下文转成 1× 采样的连续效果离屏（`SetPreferSampleCount1`），此时被 flush 的 view 走的是 **RGBA8Unorm** 一族（`internal/gpu` 里 filter/publish RT 用 `RGBA8Unorm`，见 `filter_gpu_graph.go`、`convex_renderer.go` 等），于是「BGRA 假定 + swizzle」作用在本来就是 RGBA 的面上 ⇒ **多做一次 swizzle = R/B 互换**。
- 姊妹口 `readbackFilterSrcIntoScratch`（`filter_ops_cpu.go:283-299`）注释自己点破了这件事的另一半：「filterSrcRT is BGRA offscreen — **use swizzling readback**」——说明选哪个读回函数的判据在仓里是「按调用点硬编」，不是按格式。

## 影响面（实测）

- **当前零生产消费者走这条路**：`SetEffectSurface` 全仓非测试调用者为 0（仅 `render/context.go:176`、`context_style.go:93-97`、`sample_count.go:21,36` 四处文档性提及）；`ExportImageBuf` 的唯一生产消费者 `ui/scene/textured.go:1716,1744` 是普通离屏（不设 effectSurface），落在正确档 ⇒ **latent，不改现网画面**。
- 但 `render/context_image_buf.go:20-24` 把「小离屏 Context + 每帧 `ExportImageBuf`」写成了**推荐给连续效果使用的 Skia 式用法**——照这段文档写新代码（正是 P3.7 批 5 泛光要用的形态）就会直接拿到反色输出。
- 与在册 M1 的「设计观察：合成链色空间口径无单一声明点」同族：格式/通道的真源没有一处声明，靠调用点各自假定。

## 根因的一句话形态 + 修法方向（待拍，本项未动）

**错的前提写在注释里**：`filter_ops_cpu.go:234-236` 自陈「Offscreen present targets are **BGRA8**; use the swizzling reader first」——把「离屏目标都是 BGRA8」当普适事实，而 `SetEffectSurface` 那条路发布的是 RGBA8Unorm（`ReadbackViewStraightRGBA` 的注释自己写明「Used for F.03 filter publish textures」，`layer_gpu_io.go:51-52`）。选读回函数的判据因此是**调用点硬编 + 失败再退**，而不是格式。

修法（不需要新增 hal 口）：**由产出方带格式，同步方按格式选**——`markViewFlush(view, w, h)`（`filter_ops_cpu.go:220-228`）多记一个 `lastFlushedViewFormat types.TextureFormat`（调用它的那一侧本来就知道自己 flush 的是哪张 RT 的格式），`syncViewFlushIntoPixmap` 按该格式二选一：`RGBA8Unorm` → `ReadbackViewStraightRGBA`、`BGRA8Unorm` → `ReadbackViewRGBA`；同时把「谁负责 swizzle」收敛成单一声明点（与 M1 的观察「合成链色空间口径无单一声明点」并案处理）。配套长期钉：同一绘制在 effectSurface 两档下 `ExportImageBuf` 必须逐通道等值（本探针转正）。

---

## 更正（2026-11-05，批 4 修 M6 时实测）：本文的根因定位错了

**现象登记是对的**（两档对照、只有 R/B 互换、两档都真走 GPU），**根因指错了地方**：本文把病灶定在
`Context.syncViewFlushIntoPixmap`（`render/filter_ops_cpu.go`）的"先试 BGRA 翻转读口、失败退直取"。
真修时给两个读回口与 sync 入口都打点跑了一遍，实测：

```
DBG sync entered, ahead= false          ← 两次，说明这条导出根本不进 syncViewFlushIntoPixmap
DBG readback STRAIGHT(RGBA) fmt= 27     ← 只此一处；27 = types.TextureFormatBGRA8Unorm（0x1B）
```

也就是：**病灶是 `render/filter_ops_gpu.go:806-813` 把"filter 发布纹理是 RGBA8Unorm"当普适事实、
无条件走不翻通道的那个读口**，而效果面这条路发布的是 BGRA8Unorm ⇒ 直取即 R/B 互换。方向与本文
猜测**正好相反**（本文以为"该直取的被翻了"，实为"该翻的被直取了"）。修法、钉与前后实测见
`batch4-m6-channel-order.md`；`syncViewFlushIntoPixmap` 那处同类的"按 fallback 猜"也一并改成按格式选，
但它不是 M6 的成因。本文行号与"根因"段保留原样以便对照，不再当作事实引用。

