#!/bin/bash
# P2.4d 逐文件回归的干净树对照：主仓跑的是「本次改动后」，这里在同一 HEAD（ac2e58b3，
# 不含 P2.4d 工作区）的 worktree 里跑同一批包，用来判定红是否既有。
# 本轮增量范围 = P2.4c 的 4 包之外新加的 5 包（hal 两包 + webgpu + rwgpu + wrgate）；
# 前 4 包的 27 红已由 P2.4c 的 per-file-tests-ab-clean.txt 钉过，不重复跑。
set -u
export LC_ALL=C
MAIN=/home/yanghy/app/projects/gogpu/gpui
# worktree 必须是主仓的兄弟目录：go.mod 里 `replace github.com/ebitengine/purego => ../purego`
# 是相对路径，放在 /tmp 下会让 webgpu/rwgpu/wrgate 全部 setup failed（实测踩过）。
BASE=/home/yanghy/app/projects/gogpu/gpui-p24d-base
# 父目录有 go.work 工作区，worktree 不在其模块列表里 → `go test .` 会 setup failed；
# 对照跑必须显式脱离工作区（模块内 replace 仍指向兄弟目录 ../purego，可解析）。
export GOWORK=off
OUT="$MAIN/docs/render_gpu_converge/evidence/P2.4d/per-file-tests-ab-clean.txt"
# worktree 里 lib/ 只有 ffmpeg（libwgpu_native.so 是本机构建产物，未入库），指向主仓
export WGPU_NATIVE_PATH="$MAIN/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$MAIN/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export XDG_RUNTIME_DIR="/run/user/$(id -u)"

PKGS=(
  gpu/hal
  gpu/hal/noop
  gpu/webgpu
  gpu/rwgpu
  examples/wrgate
)

{
  echo "# P2.4d per-test-file A/B on clean tree"
  echo "# date: $(date -Iseconds)"
  echo "# worktree: $BASE @ $(git -C "$BASE" rev-parse --short HEAD)（不含 P2.4d 工作区改动）"
  echo "# scope: 本轮新增的 5 个包（主仓 9 包口径减去 P2.4c 已对照过的 4 包）"
  echo "# env: WGPU_NATIVE_PATH=$WGPU_NATIVE_PATH, GOWORK=off（脱离父级 go.work）"
  echo
  pass=0; fail=0; skip=0; none=0
  for pkg in "${PKGS[@]}"; do
    cd "$BASE/$pkg" || { echo "MISSING PACKAGE $pkg"; continue; }
    echo "## $pkg"
    for f in *_test.go; do
      names=$(grep -oE '^func Test[A-Za-z0-9_]*\(' "$f" | sed 's/^func //; s/($//' | paste -sd '|')
      if [ -z "$names" ]; then
        out=$(go test -count=1 -run='XXXNONE' . 2>&1); rc=$?
        echo "$f: NO_TEST_FUNCS compile-only rc=$rc"
        [ $rc -eq 0 ] && none=$((none+1)) || { fail=$((fail+1)); echo "$out" | tail -20 | sed 's/^/    /'; }
        continue
      fi
      out=$(go test -count=1 -timeout 20m -run="^($names)\$" . 2>&1); rc=$?
      status=PASS
      [ $rc -ne 0 ] && status=FAIL
      if [ $rc -eq 0 ] && echo "$out" | grep -Eq 'no tests to run|\[no test files\]'; then status=SKIP_TAGGED; fi
      if [ $rc -eq 0 ] && ! echo "$out" | grep -q '^ok' && [ "$status" = PASS ]; then status=FAIL; fi
      echo "$f: $status rc=$rc"
      if [ "$status" = FAIL ]; then
        fail=$((fail+1)); echo "$out" | tail -30 | sed 's/^/    /'
      elif [ "$status" = SKIP_TAGGED ]; then
        skip=$((skip+1))
      else
        pass=$((pass+1))
      fi
    done
    echo
    echo "  subtotal: PASS=$pass NO_TEST=$none SKIP_TAGGED=$skip FAIL=$fail"
    pass=0; fail=0; skip=0; none=0
  done
} > "$OUT" 2>&1
grep -E '^## |subtotal|: FAIL' "$OUT" | tail -40
