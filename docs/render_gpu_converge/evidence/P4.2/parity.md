# P4.2 GPU/CPU 一致性：滤镜内核与 AA 带（2026-11-10）

> 任务定义（台账 §3 P4）：一致性：GPU/CPU 模糊对齐。本批用同一场景双腿对拍，
> 把「内核是否一致」和「抗锯齿带差多少」分开量，先量后判，未改引擎行为。

## 1 方法

- 载体：`render/p42_gpu_cpu_parity_test.go`——20 个场景，每个场景画两遍：
  GPU 腿（默认 Context）与 CPU 腿（`WithCPUOnly`），逐像素取最大通道差。
- **分区判定**：把「距尖锐过渡（中心差分 >12 级）2px 内」的像素记为 AA 带，
  其余记为**内部**。模糊后的斜坡逐像素变化很小，因此斜坡本身仍算内部——
  这样内核一致性判在斜坡上，AA 带只作为单独的、已登记的缺陷放行。
- 两档容差：内部 ≤2（内核契约）；AA 带 ≤24（SDF 覆盖差的上界，超出即回归）。
- 负向对照：把内部容差临时改成 0 ⇒ blur4/blur8 立刻红；改回 2 ⇒ 全绿
  （证明断言不是空转）。

## 2 实测

| 场景 | 内部 max | AA 带 max | AA 带像素 |
|---|---|---|---|
| blur2 | 0 | 2 | 2264 |
| blur4 / blur8 | 1 / 1 | 1 / 1 | 3224 / 3672 |
| blur16 / blur24 | 1 / 1 | 0 / 0 | 0 / 0 |
| blur3.5 / blur12.7（分数半径） | 1 / 1 | 1 / 1 | 2948 / 1916 |
| circle_blur(6) | 1 | 1 | 3038 |
| blurXY(0,8) / blurXY(6,2) | 1 / 1 | 17 / 2 | 2668 / 2360 |
| DropShadow 黑 / 彩色 | 0 / 0 | 18 / 18 | 1268 |
| 线性渐变 / 径向渐变 | 1 / 1 | 1 / 1 | 1532 / 4704 |
| 半透明方块 / 反色 / 灰度 | 0 / 0 / 0 | 9 / 9 / 5 | 1340 |
| Paint.MaskFilter(4) | 1 | 1 | 3804 |
| Bloom(σ5) | 2 | 15 | 1568 |

结论：

1. **滤镜内核已对齐**——所有场景内部最大差 ≤2（多数 0/1），含大半径、分数半径、
   异向模糊、圆形、渐变、mask 滤镜、泛光。
2. **剩余差异全部落在 1px 级 AA 带**：最大 18（阴影压在 AA 边缘下），
   其次 15（泛光边缘）、9（半透明方块边缘）。这是 GPU SDF 覆盖公式
   （单点采样 + 0.75px smoothstep）与 CPU 解析覆盖的差，属 **SDF 线**已登记洞
   （`evidence/P3.8/batch4f`），不在 P4.2 修；本批只把它钉成 ≤24 的上界，
   防止在无人察觉时继续恶化。
3. 台账里早先登记的「渐变 stop 中段 GPU ramp 插值域 vs CPU 线性域」担忧，
   实测两种渐变内部差 ≤1 ⇒ 该疑虑由 P3.8 4c 的 premultiplied sRGB 修正后已闭合。

## 3 相关边界（本批登记，不修）

- **`ClearWithColor` 只写 CPU pixmap**：无窗口 headless 下，读 dst 的 GPU 合成
  （Porter-Duff 非 Normal 层、plus 等）看不到这层底色；窗口 present 另有
  compositor base seeding，生产窗不受影响。`SeedFromData` 仅在 target 有 View 时
  生效（`gpu_render_context.go:3085/3555`），主 flush 是无 View 路径 ⇒ 结构性 seam。
  证据见 `evidence/P4.1/batch1/hole3-gpu-fix.md`（该批已把测试背景改为非 AA 全幅 Fill）。
- **破坏性 Porter-Duff 层的复合范围**：CPU 复合按层内容界、GPU 曾整幅；
  P4.1 #3 已把 GPU 复合裁到层内容界（clip），两侧对齐。

## 4 门禁

- `go test ./render -run TestP42GPUCPUFilterParity` 连跑两次全绿（0.5s 级）。
- 本批零引擎改动：只有新增测试文件；模糊/渐变/mask/泛光的生产路径未改。
