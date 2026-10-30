#!/bin/bash
# P2.4d residual grep: the six same-size texture managers must be gone;
# res.Cache must be the only size-bucketed allocation outlet.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4d/grep-residual.txt"
cd "$ROOT" || exit 1

{
  echo "# P2.4d residual grep"
  echo "# date: $(date -Iseconds)"
  echo
  echo "## 1) 六处旧管理器符号（期望 0 代码命中；docs/ 与注释里的历史名不算）"
  echo "### #1 TexturePool / #3 stencilPool / #4 offscreenPool / #5 filter 乒乓 / #6 dualtex outPool"
  echo "### 匹配按整词（(?<!ident)pat(?!ident)），所以 VideoTexturePool / poolHitPct 不算命中"
  for pat in 'TexturePool' 'textureKey' 'NewTexturePool' 'TexturePoolStats' 'BudgetTexturePool' \
             'stencilPool' 'pooledStencil' 'stencilPoolKey' 'takePooledStencil' 'pruneStencilPoolExcept' \
             'offscreenPool' 'offscreenPooled' 'makePoolRelease' 'pooledBytesLocked' 'drainOffscreenPool' \
             'poolW' 'poolH' 'publishFree' 'ensurePool' 'promotePoolResultToPublish' 'dualTexPooledTex' \
             'putOutBGRA' 'enforceOutPoolBudgetLocked'; do
    hits=$(grep -rnP --include='*.go' "(?<![A-Za-z0-9_])$pat(?![A-Za-z0-9_])" render/ gpu/ ui/ examples/ 2>/dev/null)
    lines=$(printf '%s\n' "$hits" | grep -c .)
    code=$(printf '%s\n' "$hits" | grep -v '_test.go' | grep -vE ':[0-9]+:\s*(//|/\*)' | grep -c .)
    echo "$pat: 整词命中 $lines，其中非测试非整行注释 $code"
    printf '%s\n' "$hits" | grep -vE '^$' | sed 's/^/    /'
  done
  echo "(注释/测试里的历史名保留是预期：budget.go 的 P2.4d 改名说明、budget_evict_test 的旧域引用)"
  echo
  echo "## 2) 按尺寸分桶的裸 map（期望 0 命中：同尺寸桶只剩 res.Cache）"
  n=$(grep -rnE --include='*.go' 'map\[\[2\]int\][[:space:]]*\[\]|map\[3\]uint32\][[:space:]]*\[\]|map\[\[2\]uint32\][[:space:]]*\[\]' render/internal/gpu 2>/dev/null | grep -v '_test.go' | wc -l)
  echo "size-bucketed slice maps: $n 命中"
  grep -rnE --include='*.go' 'map\[\[2\]int\][[:space:]]*\[\]|map\[3\]uint32\][[:space:]]*\[\]|map\[\[2\]uint32\][[:space:]]*\[\]' render/internal/gpu 2>/dev/null | grep -v '_test.go' | sed 's/^/    /'
  echo
  echo "## 3) 收敛后的唯一分配口：res.NewCache 调用点 + 池域名"
  grep -rn --include='*.go' 'res\.NewCache(' render/internal/gpu | grep -v '_test.go' | sed 's/^/    /'
  echo "池域名（探针按域名上报）:"
  grep -rn --include='*.go' -oE 'Name:[[:space:]]*"[a-z0-9-]+"|"[a-z]+-surface"|"[a-z]+-snap"|"[a-z]+-graph"|"[a-z]+-offscreen"' render/internal/gpu 2>/dev/null |
    grep -v '_test.go' | sed 's/^[^:]*:[0-9]*:/    /' | sort -u
  echo
  echo "## 4) 保留的包装函数（新语义 = Cache 的薄壳，非旧池）"
  for pat in 'getOutBGRA' 'releasePooledVRAM' 'releasePoolUnlocked' 'filterPublishSlot'; do
    echo "$pat:"
    grep -rn --include='*.go' -- "$pat" render/internal/gpu | grep -v '_test.go' | sed 's/^/    /'
  done
  echo
  echo "## 5) 分层：render 非测试代码中的后端特有类型（期望 0；创建入口除外）"
  grep -rnE --include='*.go' '\bwebgpu\.|\bgwgpu/|\bgles\.' render/*.go 2>/dev/null |
    grep -v '_test.go' | grep -v 'present_swapchain' | grep -v 'CreateInstance' | grep -v '_ "' | sed 's/^/    /'
  echo "(空 = 合规)"
  echo
  echo "## 6) render → internal 反向 import（期望 0：探针与 purge 都由 internal 注册进来）"
  n=$(grep -rn --include='*.go' '"github.com/energye/gpui/render/internal/gpu' render/*.go 2>/dev/null | grep -v '_test.go' | wc -l)
  echo "render 主包 import internal/gpu: $n 命中"
  grep -rn --include='*.go' '"github.com/energye/gpui/render/internal/gpu' render/*.go 2>/dev/null | grep -v '_test.go' | sed 's/^/    /'
  echo "(注释里提到 internal/gpu 不算 import：本项只匹配引号包住的包路径)"
  echo
  echo "## 7) ui 层直引 gpu（期望 0，测试缝除外）"
  n=$(grep -rn --include='*.go' '"github.com/energye/gpui/gpu/' ui/ 2>/dev/null | grep -v '_test.go' | wc -l)
  echo "ui/ import gpu/ 非测试: $n 命中"
  grep -rn --include='*.go' '"github.com/energye/gpui/gpu/' ui/ 2>/dev/null | grep -v '_test.go' | sed 's/^/    /'
} > "$OUT"
cat "$OUT"
