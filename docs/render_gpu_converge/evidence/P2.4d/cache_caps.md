# P2.4d 第三段：res.Cache 能力补齐 + 单测

日期：2026-10-30
前置：`registry_device.md`（设备级 registry + owner 分账）
范围：只补齐 Cache 能力（预算口径、每键上限、purge、按域统计），六处池仍未迁移

## 1. 改了什么

`render/internal/gpu/res/cache.go` 从「只按当前帧工作集 trim」扩成六处池的公共底座。

### 1.1 键

```go
type CacheKey struct {
    W, H     uint32
    Format   uint32
    Samples  uint32
    Usage    uint64
    Role     Role
}
```

Role 进键：同一个输出纹理在不同用途（surface / offscreen / filter / dualtex）
下不互相顶掉，也保住 registry 的 role-key 语义。

### 1.2 预算

- `CreateFn func(key CacheKey) (Native, error)`：未命中时建 native。
- `SizeFn func(key CacheKey) uint64`：真实字节口径。零值兜底
  `defaultEntryBytes = 4096`，保证只为计数的旧用例（`attack_test.go` 等）不改。
- `CacheOptions{Name, Budget, SizeFn, MaxPerKey}`：
  - `Budget` 字节，超预算按 LRU 逐出空闲条目；
  - `MaxPerKey` 每键空闲桶上限（0 = 不限），超出的直接退池 + 记账 eviction。
- 命中率、淘汰、purge、条目数、空闲数、占用、峰值、预算全走
  `CacheStats`（含 `Name`，每个域一个 Cache 一眼可分）。
- `PurgeIdle() (int, uint64)`：只收空闲条目，归还条目数 + 字节数。
- `InvalidateAll()`：设备丢失口径（不作 Release，native 归拆除流程）。

### 1.3 锁序与回调

- 固定锁序 **cache → registry**；`Release` 不在持有 cache 锁时调
  `reg.Release`（reg.Release 可能在锁外触发 idle 回调，回调会重入 cache）。
- 回池走 registry idle 回调：`SetIdleCallback(id, func(){ c.repool(id, key) })`。
  回调触发时条目必满足 retire && refs==0 && inflight==0，池不会借出在飞行纹理。
- 过期池化 id（条目已被 purge / 失效）：`repool` 走
  `reg.retireEntry + forgetLocked`，不 panic 不计错。
- `Stats()` 的 Idle 通过 `reg.idleEntry` 读（registry 锁内），读快照后 unlock，
  不会与 cache 锁嵌套成逆序。

## 2. 行为差异

无生产差异：六处池未迁移，`res.Cache` 当前只有测试消费者。

## 3. 验证

| 项 | 命令 | 结果 |
|---|---|---|
| Cache 单测 | `go test ./render/internal/gpu/res -run '^TestCache_' -v` | 12/12 PASS |
| Cache `-race` | `CGO_ENABLED=1 go test -race ./render/internal/gpu/res -run '^TestCache_'` | PASS（1.03s） |
| res 逐文件 | 8 个 `*_test.go` 按用例名分组单独跑 | 8/8 PASS |
| 构建 | `go build ./render/... ./gpu/...` | 绿 |

```
--- PASS: TestCache_InvalidateThenAcquire
--- PASS: TestCache_ReleaseUnknownRefNoPanic
--- PASS: TestCache_SizeFnDrivesBudget
--- PASS: TestCache_MaxPerKeyCapsIdleBucket
--- PASS: TestCache_PurgeIdle
--- PASS: TestCache_StatsPerDomain
--- PASS: TestCache_ConcurrentAcquireRelease
--- PASS: TestCache_AcquireReuse
--- PASS: TestCache_InflightNotLentAndRepooledAfterSubmit
--- PASS: TestCache_BudgetRetireIdleAndNotInFlight
--- PASS: TestCache_InvalidateAll
PASS
ok  github.com/energye/gpui/render/internal/gpu/res 0.004s
=== RACE ===
ok  github.com/energye/gpui/render/internal/gpu/res 1.029s
```

新增用例与覆盖面（`cache_size_test.go`）：

| 用例 | 断言点 |
|---|---|
| `TestCache_SizeFnDrivesBudget` | 400B 条目 + 600B 预算 → 释放后 eviction=1、字节=400 |
| `TestCache_MaxPerKeyCapsIdleBucket` | MaxPerKey=1 → 第二个空闲条目直接退池，registry 只留 1 |
| `TestCache_PurgeIdle` | 2 条目/2048B 全清、registry 归零、再取为 miss；借出中的条目不被 purge |
| `TestCache_StatsPerDomain` | Name/hits/misses/entries/idle/bytes/peak/budget 全对上；借出中 idle=0 |
| `TestCache_ConcurrentAcquireRelease` | 并发取放不串键、不丢条目 |

锁序证据：`TestRegistry_IdleCallbackMayReenter`（registry 段）与
`TestCache_PurgeIdle`（cache 段）交叉覆盖；`TestCache_ConcurrentAcquireRelease`
在 `-race` 下无告警。

## 4. 遗留

- 六处池迁移（stencil / offscreen / filter / dualtex / textureSet 合并）在后续段。
- 域级 `Budget` 接线（`BudgetStencilScratch` 等）随迁移一起接。