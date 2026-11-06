#!/bin/bash
# P3.7 batch 6 build gate: five-target cross build (same ruler as batch 1c).
# 尺子沿用批 1d：五目标×{默认,nogpu}，排除项只有已知的 ui/scene/internal/gpupixel
#（无非测试 Go 文件，见 P2.6）与本证据目录自身；批 2 未新增排除项。
# 批 2 变更面：engine/camera（z 序真源两级键）、engine/sprite（改委托）、render/depth_r5.go
# （排序口下线）、render/recording（命令与 Backend 契约签名）、engine/world（Transform.Height
# ＋prefab/scene 1.1）。
set -u
export LC_ALL=C
ROOT=/home/yanghy/app/projects/gogpu/gpui
OUT="$ROOT/docs/render_gpu_converge/evidence/P3.7/batch6/gates/build"
mkdir -p "$OUT"
cd "$ROOT" || exit 1

TS=$(date -Iseconds)
HEAD=$(git rev-parse --short HEAD)
DIRTY=$(git status --porcelain | wc -l)

# Known-broken packages excluded from the per-package build lists:
#   ui/scene/internal/gpupixel — pre-existing: no non-test Go files (see P2.6).
list_pkgs() { # $1=os $2=arch $3=extra tags
  local tags=""
  [ -n "${3:-}" ] && tags="$3"
  CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go list -e -tags "$tags" \
    -f '{{if or .GoFiles .CgoFiles}}{{.ImportPath}}{{end}}' ./... 2>/dev/null |
    grep -v 'ui/scene/internal/gpupixel$' |
    grep -v '/docs/render_gpu_converge/evidence/' | sort
}

for tgt in linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64; do
  os=${tgt%/*}; arch=${tgt#*/}
  f="$OUT/$(echo "$tgt" | tr / -).txt"
  pkgs_ok=$(list_pkgs "$os" "$arch")
  pkgs_nogpu=$(list_pkgs "$os" "$arch" "nogpu")
  {
    echo "# P3.7 batch 6 build gate: $tgt"
    echo "# env: CGO_ENABLED=0 GOOS=$os GOARCH=$arch"
    echo "# date: $TS"
    echo "# head: $HEAD"
    echo "# dirty: $DIRTY paths (P3.7 批 6 收官 working tree, uncommitted)"
    echo
    echo "## 1. go build ./... (raw)"
    out=$(CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build ./... 2>&1); rc=$?
    [ -n "$out" ] && echo "$out"
    echo "# exit: $rc (expected 1: known-broken pkgs excluded below, pre-existing)"
    echo
    echo "## 2. go build <all buildable except known-broken>"
    okn=$(echo "$pkgs_ok" | sed '/^$/d' | wc -l)
    out=$(echo "$pkgs_ok" | CGO_ENABLED=0 GOOS=$os GOARCH=$arch xargs go build 2>&1); rc=$?
    [ -n "$out" ] && echo "$out"
    echo "# pkgs: $okn"
    echo "# exit: $rc"
    echo
    echo "## 3. go build -tags nogpu <all buildable except known-broken>"
    ngn=$(echo "$pkgs_nogpu" | sed '/^$/d' | wc -l)
    out=$(echo "$pkgs_nogpu" | CGO_ENABLED=0 GOOS=$os GOARCH=$arch xargs go build -tags nogpu 2>&1); rc=$?
    [ -n "$out" ] && echo "$out"
    echo "# pkgs: $ngn"
    echo "# exit: $rc"
  } > "$f"
  echo "$tgt done: $(grep -c '# exit: 0' "$f")/2 green (per-package + nogpu; raw known-broken)"
done
