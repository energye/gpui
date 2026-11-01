# 盖章行走无界循环危害的定量测量（不跑真实盖章，只跑循环算术）

`15-walk-iteration-measure.go` 复刻 `path1DEffect.FilterPath` 的 `for d := offset; d < length; d += advance`
步进取舍，只数次数、不分配路径（避免把量级测量变成 OOM 事故）。输出：

```
1D offset=0 length=20 advance=1e-09 -> iterations until done: 5000001 (capped/stalled=true)
1D offset=0 length=1000 advance=1e-06 -> iterations until done: 5000001 (capped/stalled=true)
1D offset=1.073741824e+09 length=2.147483648e+09 advance=0.001 -> iterations until done: 5000001 (capped/stalled=true)
```

- 前两例：要在 5,000,001 次处才封顶，实际完成需要 2×10^10 / 10^9 次盖章——每次都新建一条原型副本，
  等于是「跑不完的循环 + 不停吃内存」。
- 第三例：`advance` 小于 `d` 的浮点 ULP，`d += advance` 后 `d` 不变——**永不结束**。
- 标准依据（本机拉取 `skia/chrome/m135 src/effects/Sk1DPathEffect.cpp` 原文）：
  > Since we are stepping by a float, the do/while loop might go on forever (or nearly so).
  > Put in a governor to limit crash values from looping too long (and allocating too much ram).
  > `#define MAX_REASONABLE_ITERATIONS 100000`
  每轮廓一个 governor，`while (distance < length && --governor >= 0)`，耗尽即 `return false`（整条过滤失败）。
- 本批实现按同一常量与同一「耗尽即拒绝」语义落 `maxReasonableStamps = 100000`（1D 每轮廓、2D 网格总量），
  钉 `TestStampEffectsRejectPathologicalDensity`：病态密度必须被拒 + 正常密度不得被误伤。
- 摘除反证：把常量临时降到 3 → 钉报「governor clipped a sane 1D walk」红（`13-*.log`），还原 100000 → 绿（`14-*.log`）。
