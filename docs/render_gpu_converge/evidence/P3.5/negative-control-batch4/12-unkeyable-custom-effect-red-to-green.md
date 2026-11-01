# 批 4 补充：自定义效果缓存投毒洞——先红后绿（非摘除式负向）

发现时机：批 4 自审（门禁跑完后逐文件复查实现）。原实现把**值形态自定义效果**（value receiver 实现
`PathEffect` 的结构体）统一哈希成 `1`，注释理由是「等值必等输出」——该推理不成立：
**不同**参数值的两个效果（`myEff{3}` vs `myEff{9}`）会落进同一个缓存条目，第二次直接取回第一次的几何，
GPU 描边画错形状（缓存投毒），不是性能问题而是正确性问题。

## 红（修复前，实测）

```
$ go test ./render/internal/gpu -run '^TestDashGeomCacheUnkeyableEffectNeverSharesAnEntry$' -count=1 -v
=== RUN   TestDashGeomCacheUnkeyableEffectNeverSharesAnEntry
    path_effect_cache_test.go:95: two different value-type effects shared one cached path
--- FAIL: TestDashGeomCacheUnkeyableEffectNeverSharesAnEntry (0.00s)
FAIL	github.com/energye/gpui/render/internal/gpu	0.008s
```

钉法：值 receiver 自定义效果 `valShiftEffect{shift}`（平移源路径），两次分别用 shift=0/40 进
`DashGeometryCache.GetOrApplyEffect` → 断言 (1) 两次几何确实差 40、(2) 返回的不是同一共享指针、
(3) 缓存条目数不增长。

## 绿（修复后）

修复口径（对齐 Skia：`SkPathEffect` 是引用计数对象、缓存按对象身份键，值语义效果在 Skia 里不存在）：
- `effectHashOf` 对不可取身份（非引用形态）的自定义效果返回 **0 = 无键**（原返回 1）；
  `PathEffectHash` 文档写明 0 = nil 或不可键。
- `GetOrApplyEffect`：效果哈希为 0 时**直接应用、不入库**（每帧现算，正确性优先，不入 LRU 污染）。
- `render/path_effect_test.go` 的 `TestPathEffectHashIsParameterStable` 加钉：引用形态实例按地址分开、
  值形态必须返回 0。

```
$ go test ./render/internal/gpu -run '^TestDashGeomCache' -count=1 -v
--- PASS: TestDashGeomCacheGetOrApplyEffect (0.00s)
--- PASS: TestDashGeomCacheUnkeyableEffectNeverSharesAnEntry (0.00s)
ok  	github.com/energye/gpui/render/internal/gpu
$ go test ./render -run '^TestPathEffectHashIsParameterStable$' -count=1
ok  	github.com/energye/gpui/render
```

批 4 定向集合复跑（修复后）：render 15 条 + internal/gpu 2 条 + recording 1 条全绿；
整包主逐文件门禁以修复后的树重跑（见 `../per-file-tests-batch4/`）。
