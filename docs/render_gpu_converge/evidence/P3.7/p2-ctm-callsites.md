# P3.7 批 1d 前置探针 P2 · CTM 调用点分类（2026-11-03 实测）

来源命令：`grep -rn "totalMatrix()" --include=*.go render/ | grep -v _test` ⇒ **39 个调用点 + 1 个定义**（`render/context_transform.go:174`）。
CTM 现形态：`c.matrix`（用户）×`c.deviceMatrix`（设备缩放）拼成的 2×3 `render.Matrix`（`context_transform.go:66,128,138,148`）。

分类口径：**透视真消费者**=该点拿到矩阵后做的是「图像/贴图的几何映射」，改成 4×4 后必须走投影除法才有正确画面；**只要仿射子块**=该点做的是位置/尺寸/逆变换/顶点上传，透视进来无处表达或不该表达。

| 组 | 调用点 | 现用法 | 分类 | 4×4 化处置 |
|---|---|---|---|---|
| A 图像与 quad | `context_image_gpu.go:76,141,312,394`、`context_image_draw.go:141`、`m4_quadimage.go:52` | 取 ctm 变换四角/包围盒 | **透视真消费者**（6 处） | 走 `Matrix4.ProjectPoint/ProjectQuad` + 权重出到透视腿（批 1c/1a 已备） |
| B1 图集批绘 | `vertices_atlas.go:46,263,339` | 逐精灵四角 + 包围盒 | **透视真消费者**（3 处） | 批绘腿吃四角+权重（批 1c），栈有透视时由 render 侧投影 |
| B2 顶点网格 | `vertices_mesh.go:31,475`、`:439`（`Invert`） | 顶点变换 + 屏幕→用户逆变换 | 半真消费者（3 处） | 顶点是任意几何，透视有意义；`Invert` 用 `Matrix4.Invert`（已有） |
| C 裁剪 | `clip_op.go:40`、`context_clip.go:98,180,195,281` | 用户路径→设备路径、设备矩形 | **只要仿射**（5 处） | 走 `Matrix4.Affine()`；非仿射⇒显式降级为路径裁剪并计数（Skia 的 clip 同样不接受透视 CTM） |
| D 路径填充/描边 | `context_gpu.go:503,526`、`context_path.go:66` | SDF/convex 快路判定 + 路径展开 | **只要仿射**（3 处） | 同上；非仿射⇒落通用展开，不进 SDF 快路 |
| E 文字 | `text_gpu.go:77,146,167,208,226,268,280,304,323`（9）、`text_dispatch.go:172,182,216,233,371,402,439`（7） | 2×3 上传给字形管线 + `TransformPoint` 定位 | **只要仿射**（16 处） | 取仿射子块；非仿射⇒拒绝或按仿射画 + 具名计数（透视文字需 mesh warp，不属本轮，登记） |
| F 刷与杂项 | `brush_device_space.go:228`、`context_misc_draw.go:153`（`Invert`）、`:218` | 设备空间刷相位、阴影/渐变逆变换 | **只要仿射**（3 处） | 仿射子块 + `Matrix4.Invert` 备选 |

**计数**：透视真消费者 = A 6 + B1 3 = **9 处**；半真（mesh）3 处；**只要仿射 = 30 处**（C5+D3+E16+F3+…含 mesh 的 `Invert` 计入 B2）。

### 结论（给批 1d 的形态依据）

1. **并存式而非整体替换**：CTM 升级若走「全链吃 4×4」，要动 39 个点里 27 个根本表达不了透视的地方（裁剪走 SDF 距离场、文字走字形图集顶点上传，两者的 GPU 契约都是 2×3）；而只有 9–12 个点（A/B1 加网格）才是透视的真受益方。⇒ 采「栈内持 `Matrix4` + 各链按需取 `Affine()` 派生 + 仅 A/B(±B2) 链走投影」。
2. **非仿射必须显式表态**：`Matrix4.Affine()`（`matrix4.go:218`）已保证「不是纯提升仿射就 ok=false」，30 个仿射点遇到透视 CTM 时的行为必须逐个定（降级画 / 拒绝 / 计数），禁止 `ok=false` 时静默用 identity 或压扁——这是 P3.5 批 3 的「非仿射不静默压扁」纪律的延续。
3. **批 1d 的真实工作量在 27 个点的表态，不在矩阵数学**：数学（`Matrix4`/`ProjectPoint`/`ProjectQuad`/`Invert`）P3.5 批 3 已交付且有钉。
4. 与 D2 拍板自洽：批 1c（引擎算好四角+权重喂批绘腿）不依赖 1d；1d 是把「整场景共用一个投影」这条形态补上，服务地面/平面类内容，二者不互为前置。

原始 grep 输出见同目录 `ctm-callsites.txt`。
