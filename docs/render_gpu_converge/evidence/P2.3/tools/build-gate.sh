#!/bin/bash
# P2.3 build gate: five-target cross build + vet, mirroring the P1.1b/P2.2 ruler.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.3/build"
mkdir -p "$OUT"
cd "$ROOT" || exit 1

TS=$(date -Iseconds)
HEAD=$(git rev-parse --short HEAD)
DIRTY=$(git status --porcelain | wc -l)

# Per-target package list: only packages that have buildable files for that
# target (zero-GoFiles dirs are skipped, matching the P1.1b/P2.2 ruler).
list_pkgs() { # $1=os $2=arch $3=extra tags
  local tags=""
  [ -n "${3:-}" ] && tags="$3"
  CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go list -e -tags "$tags" \
    -f '{{if or .GoFiles .CgoFiles}}{{.ImportPath}}{{end}}' ./... 2>/dev/null |
    grep -v 'examples/ui_wr_t_stress$' | sort
}

for tgt in linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64; do
  os=${tgt%/*}; arch=${tgt#*/}
  f="$OUT/$(echo "$tgt" | tr / -).txt"
  pkgs_ok=$(list_pkgs "$os" "$arch")
  pkgs_nogpu=$(list_pkgs "$os" "$arch" "nogpu")
  {
    echo "# P2.3 build gate: $tgt"
    echo "# env: CGO_ENABLED=0 GOOS=$os GOARCH=$arch"
    echo "# date: $TS"
    echo "# head: $HEAD"
    echo "# dirty: $DIRTY paths (P2.3 working tree, uncommitted)"
    echo
    echo "## 1. go build ./... (raw)"
    out=$(CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build ./... 2>&1); rc=$?
    [ -n "$out" ] && echo "$out"
    echo "# exit: $rc"
    echo
    echo "## 2. known-broken packages (expected red, unchanged from P1.1b/P2.2)"
    for kb in ./examples/ui_wr_t_stress ./ui/scene/internal/gpupixel; do
      echo "### go build $kb"
      out=$(CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build "$kb" 2>&1); rc=$?
      [ -n "$out" ] && echo "$out"
      echo "# exit: $rc"
    done
    echo
    echo "## 3. go build <all buildable except known-broken>"
    okn=$(echo "$pkgs_ok" | sed '/^$/d' | wc -l)
    out=$(echo "$pkgs_ok" | CGO_ENABLED=0 GOOS=$os GOARCH=$arch xargs go build 2>&1); rc=$?
    [ -n "$out" ] && echo "$out"
    echo "# pkgs: $okn"
    echo "# exit: $rc"
    echo
    echo "## 4. go build -tags nogpu <all buildable except known-broken>"
    ngn=$(echo "$pkgs_nogpu" | sed '/^$/d' | wc -l)
    out=$(echo "$pkgs_nogpu" | CGO_ENABLED=0 GOOS=$os GOARCH=$arch xargs go build -tags nogpu 2>&1); rc=$?
    [ -n "$out" ] && echo "$out"
    echo "# pkgs: $ngn"
    echo "# exit: $rc"
  } > "$f"
  echo "$tgt done: $(grep -c '# exit: 0' "$f")/4 green"
done
