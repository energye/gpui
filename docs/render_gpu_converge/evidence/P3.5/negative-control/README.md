# P3.5 批 1 负向对照：摘掉光栅 conic 分支

**日期**：2026-11-01

## 目的

证明新增的 conic 城界展开不是「写了但没鉴别力」：把 `render/software_fill.go`
`convertGGPathToCorePath` 里的 `case ConicTo` 分支摘掉，钉必须转红；还原后必须复绿。

> 为什么钉 `TestConvertGGPathToCorePathConic` 而不是 `TestContextFillConicQuarterDisk`：
> `SoftwareRenderer.Fill` 入口也有一道 `ExpandConicsToQuads` 守卫，直接从 `Fill`
> 走会把 conic 提前降成 quad，摘掉 `convertGGPathToCorePath` 分支仍然全绿。
> `convertGGPathToCorePath` 是 raster 边界的最后一道转换，单测直接调它才有鉴别力。

## 变异

```
-		case ConicTo:
-			ctrl := Pt(coords[0], coords[1])
-			end := Pt(coords[2], coords[3])
-			NewConicBez(cur, ctrl, end, coords[4]).ToQuads(0.1, func(q QuadBez) {
-				verbs = append(verbs, raster.QuadTo)
-				points = append(points,
-					float32(q.P1.X), float32(q.P1.Y),
-					float32(q.P2.X), float32(q.P2.Y),
-				)
-			})
-			cur = end
```

## 结果

| 步骤 | 命令 | 结果 | 日志 |
|---|---|---|---|
| 摘分支 | `go test ./render -run '^TestConvertGGPathToCorePathConic$' -count=1 -v` | **FAIL** `conic converted to 2 verbs, want MoveTo+Quad(s)+Close` | `removed-conic-case.txt` |
| 还原 | 同上 | **PASS** | `restored-conic-case.txt` |

结论：该钉对 raster 边界的 conic 转换有真实鉴别力，还原后零残留。
