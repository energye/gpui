#!/bin/bash
# P2.4d API snapshot diff (go doc -all): working tree vs the P2.4c closing state (HEAD).
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
EV="$ROOT/docs/render_gpu_converge/evidence/P2.4d"
OUT="$EV/api-diff.txt"
BASE=$(mktemp -d /tmp/p24d-api.XXXXXX)
cd "$ROOT" || exit 1

PKGS=(render gpu/hal render/internal/gpu render/internal/gpu/res)

git worktree add --detach "$BASE" HEAD >/dev/null 2>&1 || { echo "worktree failed" > "$OUT"; exit 1; }
ln -sfn /home/yanghy/app/projects/gogpu/purego /tmp/purego
mkdir -p "$EV/api"
for pkg in "${PKGS[@]}"; do
  slug=$(echo "$pkg" | tr '/' '-')
  ( cd "$BASE" && go doc -all "./$pkg" > "$EV/api/base-$slug.txt" 2>&1 )
  ( cd "$ROOT" && go doc -all "./$pkg" > "$EV/api/$slug.txt" 2>&1 )
done
git worktree remove --force "$BASE" >/dev/null 2>&1
rm -f /tmp/purego

{
  echo "# P2.4d API snapshot diff（对 P2.4c 收口 = HEAD ac2e58b3）"
  echo "# date: $(date -Iseconds)"
  echo
  for pkg in "${PKGS[@]}"; do
    slug=$(echo "$pkg" | tr '/' '-')
    echo "## $pkg"
    diff -u "$EV/api/base-$slug.txt" "$EV/api/$slug.txt" || true
    echo
  done
  echo "## 预期变化摘要"
  echo "- render/budget.go：BudgetTexturePool 更名 BudgetSurfacePool（语义=共享 surface trio 池）；新增 BudgetFilterGraph（8 条 / 128MiB，等价收敛前『3 乒乓 + publish 自由表 ≤4』）。常量净增 0（删 1 增 1，预算域 10→11）。"
  echo "- render 新增 GPU 池命中率探针出口：PoolDomainStats（+HitRate）、GPUPoolStatsFunc、Register/UnregisterGPUPoolStatsCollector、GPUPoolStats。"
  echo "- gpu/hal 新增 CompletionToken（Done/Wait）与 Queue.OnSubmittedWorkDone，供 res 闸门等直提完成。"
  echo "- res 侧：Cache 升为 GrResourceCache 形态（CacheOptions{Budget,SizeFn,MaxPerKey}、CacheStats、PurgeIdle、按域统计、SetIdleCallback 回池），新增 RoleFilterRT 与 InflightGate/GateHandle；Registry 设备级化（DeviceOwner/NewOwner、owner 分账、retireEntry/idleEntry、acquireIfIdle、releaseIDs）。"
  echo "- render/internal/gpu：TexturePool（手写同尺寸池）删除，五域（surface/stencil/offscreen/filter/dualtex）统一到 res.Cache；新增 directSubmit、surfaceCache 等导出面仅内部可见（go doc 不列未导出符号，预期为空变化）。"
} > "$OUT"
cat "$OUT"
