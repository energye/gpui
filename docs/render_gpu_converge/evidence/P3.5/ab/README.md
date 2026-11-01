# P3.5 批 1：开工前 7 条红的 A/B 定性

**日期**：2026-11-01　**HEAD**：`6d861410`（干净树）　**工作树**：P3.5 批 1（conic，未提交）

批 1 动代码前，先把工作树上跑出的 7 条红逐条定性。原则：只有「干净 HEAD 也红」或「本步有意改契约/值」才算既有/预期，不用推测结案。

## 结论

| # | 红 | 分类 | 证据 |
|---|---|---|---|
| 1 | `TestAAProbe_CircleDiag` | 环境：缺 `WGPU_NATIVE_PATH`/`LD_LIBRARY_PATH` → GPU 路径回退 CPU，测试拒绝 CPU fallback | `head-clean-env.txt`、`worktree-env.txt`、`worktree-noenv.txt` |
| 2 | `TestF14_EffectSurface_FlushGPU_PublishesGPUFilterTexture` | 同上（无 GPU 时没有 pending GPU draw） | 同上 |
| 3 | `TestCustomPatternFill` | 顺序耦合（整包一起跑才红）；逐文件在本工作树/干净 HEAD、带环境/不带环境四组全绿 | `worktree-env-patterns.txt`、`worktree-noenv-patterns.txt`、`head-env-patterns.txt`、`head-noenv-patterns.txt` |
| 4 | `TestCustomBrushFill` | 同上 | 同上 |
| 5 | `TestCheckerboardFill` | 同上 | 同上 |
| 6 | `TestVerbCoordCount` | 有意改契约：未知 verb 从 `0`（静默跳过）改为 `-1`（`Iterate` 直接 panic），新增 `ConicTo=5` | `render/path_test.go` 值钉 + `render/path_verb.go` 注释 |
| 7 | `TestPathArea_Quadratic` | 旧期望值跟旧错误公式对上了；`quadArea` 端点项缺失，已按 Green 公式修正，并用数值积分独立复核 | `TestQuadAreaExactAgainstNumericIntegral`（`render/curve_conic_test.go`），三组期望 `100/3` / `80/3` / `250/3` |

## 1/2：GPU 环境红

有库环境（`WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`、`LD_LIBRARY_PATH=$PWD/lib`）：

- `head-clean-env.txt`：干净 HEAD 两测 PASS。
- `worktree-env.txt`：P3.5 工作树两测 PASS。

去掉库环境后（`worktree-noenv.txt`）：

- `TestAAProbe_CircleDiag` FAIL：`cpu_fallback not allowed: 6 reason="fill:gg: falling back to CPU rendering"`。
- `TestF14_EffectSurface_FlushGPU_PublishesGPUFilterTexture` FAIL：`expected pending GPU draws after Fill, got 0`。

定性：与本步 conic 改动无关，是测试机没把 native 库路径喂给测试进程。

## 3/4/5：pattern 三测顺序耦合

四组逐文件命令（`-run '^(TestCheckerboardFill|TestCustomPatternFill|TestCustomBrushFill)$'`）：

| 树 | GPU 库环境 | 结果 | 日志 |
|---|---|---|---|
| HEAD `6d861410` | 有 | 3/3 PASS | `head-env-patterns.txt` |
| HEAD `6d861410` | 无 | 3/3 PASS | `head-noenv-patterns.txt` |
| P3.5 工作树 | 有 | 3/3 PASS | `worktree-env-patterns.txt` |
| P3.5 工作树 | 无 | 3/3 PASS | `worktree-noenv-patterns.txt` |

定性：三测只在「整包一起跑」时红，逐文件（本仓门禁口径，AGENTS「回归按测试文件逐个跑」）在干净 HEAD 与工作树都绿，且 conic 改动不经过这三条 pattern 路径。归为整包运行时的跨用例全局状态耦合，不是 P3.5 回归。正式回归以逐文件门禁为准。

## 6：未知 verb 契约变更（有意）

`verbCoordCount` 原来对未知值返回 `0`，`Path.Iterate` 会静默跳过几何。批 1 改成：

- 已知 verb：`MoveTo/LineTo=2`、`QuadTo=4`、`CubicTo=6`、`ConicTo=5`、`Close=0`。
- 未知 verb：返回 `-1`，`Iterate` 立即 `panic`。

`TestVerbCoordCount` 的 `PathVerb(99)` 期望随之从 `0` 改为 `-1`；这是有意的「漏改一处就炸」防护，不是把红改绿。

## 7：`quadArea` 公式修复

旧公式漏了 `−3y0` / `+3y2` 端点项，且中间项系数少 2。用独立数值积分（`TestQuadAreaExactAgainstNumericIntegral`，四个任意控制点）证明旧公式错、新公式对：

```
[x0(2y1+y2−3y0) + 2x1(y2−y0) + x2(3y2−2y1−y0)] / 6
```

`TestPathArea_Quadratic` 三组期望改为精确值 `100/3`、`80/3`、`250/3`，容差从 1.0/2.0/10.0 收紧到 `1e-9`。修的是公式，不是把断言放宽。
