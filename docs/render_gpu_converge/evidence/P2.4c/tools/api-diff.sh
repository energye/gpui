#!/bin/bash
# P2.4c API snapshot diff (go doc -all) against the P2.4b closing state.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
EV="$ROOT/docs/render_gpu_converge/evidence/P2.4c"
OLD="$ROOT/docs/render_gpu_converge/evidence/P2.4b"
OUT="$EV/api-diff.txt"
BASE=$(mktemp -d /tmp/p24c-api.XXXXXX)
cd "$ROOT" || exit 1

git worktree add --detach "$BASE" HEAD >/dev/null 2>&1 || { echo "worktree failed" > "$OUT"; exit 1; }
ln -sfn /home/yanghy/app/projects/gogpu/purego /tmp/purego
mkdir -p "$EV/api"
( cd "$BASE" && go doc -all ./render/internal/gpu > "$EV/api/base-render-internal-gpu.txt" 2>&1 )
( cd "$BASE" && go doc -all ./render/internal/gpu/res > "$EV/api/base-render-internal-gpu-res.txt" 2>&1 )
( cd "$ROOT" && go doc -all ./render/internal/gpu > "$EV/api/render-internal-gpu.txt" 2>&1 )
( cd "$ROOT" && go doc -all ./render/internal/gpu/res > "$EV/api/render-internal-gpu-res.txt" 2>&1 )
git worktree remove --force "$BASE" >/dev/null 2>&1
rm -f /tmp/purego

{
  echo "# P2.4c API snapshot diff（对 P2.4b 收口 = HEAD 2204ad5a）"
  echo "# date: $(date -Iseconds)"
  echo
  echo "## render/internal/gpu"
  diff -u "$EV/api/base-render-internal-gpu.txt" "$EV/api/render-internal-gpu.txt" || true
  echo
  echo "## render/internal/gpu/res"
  diff -u "$EV/api/base-render-internal-gpu-res.txt" "$EV/api/render-internal-gpu-res.txt" || true
  echo
  echo "## 预期变化摘要"
  echo "- ImageCache: ReleaseEphemeral 删除；SetRetireHook 新增；ImageCacheStats.EphemeralPending 删除（retire 钩子取代裸 ephemeral/pending 队列）。"
  echo "- res.Cache: OnSubmissionFinished 删除（idle 回调自动回池）；Registry.SetIdleCallback / PendingReleaseCount 新增。"
  echo "- render / gpu/hal / gpu 其余包导出面零变化（本步只动 internal 实现 + 一条 gles 注释）。"
} > "$OUT"
cat "$OUT"