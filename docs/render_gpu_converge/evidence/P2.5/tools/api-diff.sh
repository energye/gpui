#!/bin/bash
# P2.5 API snapshot diff: working tree vs HEAD（基线 api-diff.sh 口径，路径 P2.5 化）
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
EV="$ROOT/docs/render_gpu_converge/evidence/P2.5"
OUT="$EV/api-diff.txt"
BASE=$(mktemp -d /tmp/p25-api.XXXXXX)
cd "$ROOT" || exit 1
PKGS=(render gpu/hal gpu/webgpu render/internal/gpu render/internal/gpu/res ui/scheduler ui/scene)
git worktree add --detach "$BASE" HEAD >/dev/null 2>&1 || { echo "worktree failed" > "$OUT"; exit 1; }
ln -sfn /home/yanghy/app/projects/gogpu/purego /tmp/purego
for pkg in "${PKGS[@]}"; do
  slug=$(echo "$pkg" | tr '/' '-')
  ( cd "$BASE" && go doc -all "./$pkg" > "$EV/api/base-$slug.txt" 2>&1 )
  ( cd "$ROOT" && go doc -all "./$pkg" > "$EV/api/$slug.txt" 2>&1 )
done
git worktree remove --force "$BASE" >/dev/null 2>&1
rm -f /tmp/purego
{
  echo "# P2.5 API snapshot diff（working tree vs HEAD 9bb2e99a）"
  echo "# date: $(date -Iseconds)"
  echo
  for pkg in "${PKGS[@]}"; do
    slug=$(echo "$pkg" | tr '/' '-')
    echo "## $pkg"
    diff -u "$EV/api/base-$slug.txt" "$EV/api/$slug.txt" || true
    echo
  done
  echo "## 预期变化（本步白名单）"
  echo "- gpu/hal：+WithForceNativeReleaseOnLost / +SetForceNativeReleaseOnLostHook（P2.5 D2 rwgpu 直引收敛）。"
  echo "- ui/scheduler FrameMetrics：+FPSWall / +TargetHz 字段（A2 A-family 补齐）；json 标签 interval_* 一次切换。"
  echo "- 其余包零变化。"
} > "$OUT"
echo "api-diff done"
