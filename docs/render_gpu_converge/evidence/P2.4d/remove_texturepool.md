# P2.4d 第四段：删除废弃 TexturePool + 预算域改名

日期：2026-10-30
前置：`cache_caps.md`（res.Cache 能力补齐）
范围：只删死代码与改名声明域，六处池仍未迁移

## 1. 为什么可以删

`render/internal/gpu/texture_pool.go` 的 `Acquire/Release/EndFrame` 在
P2.4c 后已无生产调用（`grep -rn "texturePool\." render/` 只剩
`DestroyAll/SetBudget/Stats` 三个遥测口，池自始至终为空）。会话纹理走
`textureSet` + registry retire，长期池化归 `res.Cache`，该结构已无存在意义。

## 2. 改了什么

| 位置 | 变化 |
|---|---|
| `render/internal/gpu/texture_pool.go` | 整文件删除（`TexturePool`/`NewTexturePool`/`TexturePoolStats`/`defaultTexturePoolBudgetMB`/`textureSetEstimatedBytes`） |
| `render/internal/gpu/gpu_shared.go` | 删 `texturePool` 字段与初始化、`SetTexturePoolBudget`、三处 `DestroyAll`（unpresentable purge / abandon / MSAA 降级）、`GPUMemoryStats.TexturePoolPooled/MB/Hits/Misses` 字段与填充 |
| `render/internal/gpu/sdf_gpu.go` | `ResourceUploadStats.Texture` 字段与填充删除 |
| `render/internal/gpu/doc.go` | 预算分域注释改为 res.Cache 四域 |
| `render/budget.go` | `BudgetTexturePool` → `BudgetSurfacePool`；`String()` 改 `"surface-pool"`；128 MiB 原值保留（域语义=设备级 surface trio 池） |
| `render/budget_default_test.go` | pin 表与名字表同步改名 |
| `docs/RENDER_API_CATALOG.md` | `BudgetDomain` 行改名 + 状态说明 |

测试同步删除（都只测已删结构）：

- `pipeline_wiring_test.go`：`TestTexturePool_AcquireRelease`、`TestTexturePool_EndFrame`
- `s6_7_resources_test.go`：`TestS67_TexturePool_HitMissStats`、`TestS67_TexturePool_EndFrameBudget`
- `alloc_bench_test.go`：`BenchmarkTexturePool_AcquireRelease`

## 3. 行为差异

零。被删的三条路径（unpresentable purge / abandon / MSAA 降级）此前都是
`if s.texturePool != nil { s.texturePool.DestroyAll() }`，池恒空 → 空操作；
遥测字段恒为 0。`BudgetSurfacePool` 数字与 P2.4b pin 一致（128 MiB），
pin 测试照红/绿逻辑保护。

## 4. 验证

| 项 | 命令 | 结果 |
|---|---|---|
| 全仓残留 | `grep -rn "BudgetTexturePool\|TexturePoolStats\|NewTexturePool\|TexturePoolPooled\|SetTexturePoolBudget\|texture-pool" --include=*.go` | 0 命中（业务/引擎代码；仅 docs 旧证据与 `.zcode` 计划留档） |
| 构建 | `go build ./render/... ./gpu/...` | 绿 |
| `pipeline_wiring_test.go` | 41 个用例分组单跑 | PASS |
| `s6_7_resources_test.go` | 5 个用例分组单跑 | PASS |
| `budget_default_test.go` | pin + 名字 + FairSplit 三用例 | PASS |
| bench 编译 | `go test ./render/internal/gpu -run '^$' -bench '^$'` | PASS |
| gofmt | 改动 8 个 Go 文件 | 无输出 |
| vet | `go vet ./render/internal/gpu ./render` | 仅既有 3 条（unreachable ×2 / unsafe.Pointer ×1） |

## 5. 遗留

- `BudgetSurfacePool` 的消费点随 textureSet 合并（后续段）接到 `res.Cache`
  预算上；此刻表中声明与 P2.4b 等价。`BudgetFilterGraph` 随 filter 池下沉一起加。