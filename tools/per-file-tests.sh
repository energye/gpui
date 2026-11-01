#!/usr/bin/env bash
#
# P2.5 正本：逐测试文件门禁（合入前必跑，AGENTS「回归按测试文件逐个跑」）。
# 各阶段 evidence 目录里的副本出自这里；跑法：
#
#   tools/per-file-tests.sh [清单文件] [输出目录]
#
#   清单文件   默认 tools/per-file-tests.pkgs（每行一个包路径，# 注释）
#   输出目录   默认 tmp/per-file-tests（**必须按阶段传参**——历史教训：
#              P2.4d 副本把输出路径硬编码进上阶段目录，覆盖了证据）
#
# 每包逐 *_test.go 提取 Test/Benchmark 函数名后整文件一次 `go test -run`：
#   PASS / FAIL / SKIP_TAGGED（函数名提取不到或编译标签排除，仅编译）/ NO_TESTS（无测试文件）。
# GPU 包必须带 WGPU_NATIVE_PATH，否则 native 用例全 SKIP 文件级 rc=0=假绿（池 H1）。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"

PKGS_FILE="${1:-$ROOT/tools/per-file-tests.pkgs}"
OUT="${2:-$ROOT/tmp/per-file-tests}"
# 归一为绝对路径：进包目录后日志/汇总仍可写（相对路径会在 cd 后失效）。
mkdir -p "$OUT" && OUT="$(cd "$OUT" && pwd)"

mapfile -t PKGS < <(grep -v '^[[:space:]]*#' "$PKGS_FILE" | grep -v '^[[:space:]]*$' | sed 's|^github.com/energye/gpui/||')
echo "pkgs=${#PKGS[@]} list=$PKGS_FILE out=$OUT WGPU_NATIVE_PATH=$WGPU_NATIVE_PATH"

: > "$OUT/summary.tsv"
FAIL_TOTAL=0
for pkg in "${PKGS[@]}"; do
  if [ ! -d "$ROOT/$pkg" ]; then
    echo -e "SKIP_PKG\t$pkg (no dir)" >> "$OUT/summary.tsv"
    continue
  fi
  cd "$ROOT/$pkg"
  files=(*_test.go)
  if [ ! -e "${files[0]}" ]; then
    echo -e "NO_TESTS\t$pkg" >> "$OUT/summary.tsv"
    cd "$ROOT"
    continue
  fi
  for f in "${files[@]}"; do
    log="$OUT/$(echo "$pkg" | tr '/' '_')__${f%.go}.log"
    tests=$(grep -E '^func (Test|Benchmark)[A-Z]' "$f" | sed -E 's/^func ([A-Za-z0-9_]+)\(.*/\1/' | paste -sd'|' -)
    if [ -z "$tests" ]; then
      echo -e "SKIP_TAGGED\t$pkg/$f" >> "$OUT/summary.tsv"
      continue
    fi
    if go test -count=1 -timeout "${PER_FILE_TIMEOUT:-20m}" -run "^(${tests})\$" . > "$log" 2>&1; then
      if grep -q "no tests to run" "$log"; then
        echo -e "SKIP_TAGGED\t$pkg/$f" >> "$OUT/summary.tsv"
      else
        echo -e "PASS\t$pkg/$f" >> "$OUT/summary.tsv"
      fi
    else
      echo -e "FAIL\t$pkg/$f" >> "$OUT/summary.tsv"
      FAIL_TOTAL=$((FAIL_TOTAL+1))
    fi
  done
  cd "$ROOT"
done

echo "=== summary ==="
cut -f1 "$OUT/summary.tsv" | sort | uniq -c
echo "FAIL_TOTAL=$FAIL_TOTAL (对照当阶段基线红名单：只减不增)"
[ "$FAIL_TOTAL" -eq 0 ] || exit 1
