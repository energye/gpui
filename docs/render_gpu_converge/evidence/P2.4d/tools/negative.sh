#!/bin/bash
# P2.4d negative controls: the four invariants the same-size pool convergence
# claims must each have a live test — per-key cap, byte budget, OOM purge chain,
# probe outlet. Break one, prove red, restore, prove green.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4d/negative/pool-negative.txt"
mkdir -p "$(dirname "$OUT")"
cd "$ROOT" || exit 1

CACHE_F="render/internal/gpu/res/cache.go"
DUAL_F="render/internal/gpu/dual_tex_blend.go"
WRGATE_F="examples/wrgate/report.go"

bak=$(mktemp -d /tmp/p24d-neg.XXXXXX)
cp "$CACHE_F" "$bak/cache.go"
cp "$DUAL_F" "$bak/dual.go"
cp "$WRGATE_F" "$bak/wrgate.go"
restore_all() {
  cp "$bak/cache.go" "$CACHE_F"
  cp "$bak/dual.go" "$DUAL_F"
  cp "$bak/wrgate.go" "$WRGATE_F"
}
same() { cmp -s "$1" "$2" && echo "    复原一致: 是" || echo "    复原一致: 否(需人工检查)"; }
run() { # run <pkg> <regex> <期望 exit> <说明>
  go test -count=1 -timeout 20m "./$1" -run "$2" 2>&1 | tail -4
  echo "# exit: ${PIPESTATUS[0]} (期望 $3) — $4"
}
trap restore_all EXIT

{
  echo "# P2.4d negative controls（四处红→绿）"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git rev-parse --short HEAD)"
  echo

  echo "## N1 每键空闲上限（MaxPerKey 是不是真在挡回桶）"
  echo "### 断言前移：确认测试名存在（防拼错名 = 假红）"
  grep -rn 'func TestCache_MaxPerKeyCapsIdleBucket\|func TestDualTexOutPool_PerBucketCap' render/internal/gpu | sed 's/^/    /'
  echo "### 干净态"
  run render/internal/gpu/res '^TestCache_MaxPerKeyCapsIdleBucket$' 0 "res 层上限"
  run render/internal/gpu '^TestDualTexOutPool_PerBucketCap$' 0 "dualtex 域上限"
  echo "### 断开：回桶不再查 MaxPerKey"
  perl -0pi -e 's/if c\.opt\.MaxPerKey > 0 && len\(c\.pool\[key\]\) >= c\.opt\.MaxPerKey \{/if false \&\& len(c.pool[key]) >= c.opt.MaxPerKey {/' "$CACHE_F"
  grep -n 'if false && len(c.pool\[key\])' "$CACHE_F" | sed 's/^/    /'
  run render/internal/gpu/res '^TestCache_MaxPerKeyCapsIdleBucket$' 1 "无上限则桶无限增长"
  run render/internal/gpu '^TestDualTexOutPool_PerBucketCap$' 1 "同一断言在真域上"
  restore_all; same "$bak/cache.go" "$CACHE_F"
  echo "### 复原"
  run render/internal/gpu/res '^TestCache_MaxPerKeyCapsIdleBucket$' 0 "复原"
  echo

  echo "## N2 字节预算逐出：trimLocked 是否真在管字节上限"
  echo "# 第一轮注错位置：只摘 repool 里的 trimLocked() 调用 → 两处仍绿（Release 也调 trim）。"
  echo "# 这说明预算有两个执行点，负向必须关掉执行点本身（按实测保留本段说明）。"
  grep -rn 'func TestCache_BudgetRetireIdleAndNotInFlight\|func TestDualTexOutPool_ByteBudgetEviction' render/internal/gpu | sed 's/^/    /'
  echo "### 干净态"
  run render/internal/gpu/res '^TestCache_BudgetRetireIdleAndNotInFlight$' 0 "res 层预算"
  echo "### 断开一：只摘 repool 的 trim 调用（预期仍绿：Release 也 trim）"
  perl -0pi -e 's/\tc\.pool\[key\] = append\(c\.pool\[key\], id\)\n\tc\.lru = append\(c\.lru, id\)\n\tc\.trimLocked\(\)/\tc.pool[key] = append(c.pool[key], id)\n\tc.lru = append(c.lru, id)/' "$CACHE_F"
  run render/internal/gpu/res '^TestCache_BudgetRetireIdleAndNotInFlight$' 0 "两个执行点之一失效，另一个仍在"
  restore_all; same "$bak/cache.go" "$CACHE_F"
  echo "### 断开二：关掉 trimLocked 的逐出循环本身（预算声明仍在、无人执行）"
  perl -0pi -e 's/func \(c \*Cache\) trimLocked\(\) \{\n\tif c\.opt\.Budget == 0 \{/func (c *Cache) trimLocked() {\n\tif true {/' "$CACHE_F"
  grep -n 'func (c \*Cache) trimLocked' -A 2 "$CACHE_F" | sed 's/^/    /'
  run render/internal/gpu/res '^TestCache_BudgetRetireIdleAndNotInFlight$' 1 "超预算条目不再被逐出"
  run render/internal/gpu '^TestDualTexOutPool_ByteBudgetEviction$' 1 "dualtex 域同一断言"
  run render/internal/gpu '^TestSurfaceCache_ByteBudgetEvictsLRU$' 1 "surface 域同一断言"
  restore_all; same "$bak/cache.go" "$CACHE_F"
  echo "### 复原"
  run render/internal/gpu/res '^TestCache_BudgetRetireIdleAndNotInFlight$' 0 "复原"
  echo

  echo "## N3 purge/OOM 链：GPUShared.PurgeEvictable 是否真的清到 dualtex 域"
  echo "### 干净态"
  run render/internal/gpu '^TestGPUShared_PoolsAreOnTheOOMChain$' 0 "两域都在链上"
  echo "### 断开：dualtex 的 PurgeIdle 调用摘掉（域仍在，但 OOM 清不动）"
  perl -0pi -e 's/\tn, _ := cache\.PurgeIdle\(\)\n\treturn n/\t_ = cache\n\treturn 0/' "$DUAL_F"
  echo "    补丁后的 releasePooledVRAM:"
  grep -n 'func (c \*dualTexBlendCache) releasePooledVRAM' -A 9 "$DUAL_F" | sed 's/^/    /'
  run render/internal/gpu '^TestGPUShared_PoolsAreOnTheOOMChain$' 1 "gpu-shared-pools 只报出 filter 一条"
  restore_all; same "$bak/dual.go" "$DUAL_F"
  echo "### 复原"
  run render/internal/gpu '^TestGPUShared_PoolsAreOnTheOOMChain$' 0 "复原"
  echo

  echo "## N4 命中率探针出口：Report.Pool 必须由探针填（漏填不能读成沉默通过）"
  echo "### 干净态"
  run examples/wrgate '^(TestBuildReport_PoolSectionFromProbe|TestEvaluatePoolGates)$' 0 "探针段与门禁"
  echo "### 断开：BuildReport 不再问探针（Pool 置 nil）"
  perl -0pi -e 's/Pool:                   render\.GPUPoolStats\(\),/Pool:                   nil,/' "$WRGATE_F"
  echo "    补丁后的 Pool 赋值:"
  grep -n '^\s*Pool:' "$WRGATE_F" | sed 's/^/    /'
  run examples/wrgate '^TestBuildReport_PoolSectionFromProbe$' 1 "pool 段的域名与数字消失"
  restore_all; same "$bak/wrgate.go" "$WRGATE_F"
  echo "### 复原"
  run examples/wrgate '^(TestBuildReport_PoolSectionFromProbe|TestEvaluatePoolGates)$' 0 "复原"
  echo

  echo "## 残留检查（四处补丁必须全部还原）"
  echo -n "  cache.go 差异行数: "; diff "$bak/cache.go" "$CACHE_F" | grep -c . || true
  echo -n "  dual.go 差异行数: "; diff "$bak/dual.go" "$DUAL_F" | grep -c . || true
  echo -n "  report.go 差异行数: "; diff "$bak/wrgate.go" "$WRGATE_F" | grep -c . || true
  rm -rf "$bak"
} > "$OUT" 2>&1
restore_all
cat "$OUT"
