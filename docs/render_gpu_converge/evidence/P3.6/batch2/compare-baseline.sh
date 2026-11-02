#!/usr/bin/env bash
# 与批 1b 基线逐条对账：状态计数 + FAIL 名单增删（新增红必须 clean HEAD A/B 归因）
set -u
R=/home/yanghy/app/projects/gogpu/gpui
B=$R/docs/render_gpu_converge/evidence/P3.6/batch1c
C=$R/docs/render_gpu_converge/evidence/P3.6/batch2
pair() { # $1=基线文件 $2=本次文件 $3=标题
  echo "### $3"
  for f in "$1" "$2"; do
    [ -f "$f" ] || { echo "  缺文件: $f"; continue; }
    printf "  %-64s %s 条 " "$(basename "$(dirname "$f")")/$(basename "$f")" "$(wc -l < "$f")"
    awk -F'\t' '{c[$1]++} END{printf "PASS=%d SKIP_TAGGED=%d SKIP_PKG=%d NO_TESTS=%d FAIL=%d\n", c["PASS"], c["SKIP_TAGGED"], c["SKIP_PKG"], c["NO_TESTS"], c["FAIL"]}' "$f"
  done
  echo "  --- FAIL 名单差异（< 只在基线 / > 只在本次）---"
  if [ -f "$1" ] && [ -f "$2" ]; then
    diff <(grep -P '^FAIL\t' "$1" | cut -f2 | sort) <(grep -P '^FAIL\t' "$2" | cut -f2 | sort) && echo "  （FAIL 名单逐条相同）"
  fi
  echo
}
pair "$B/per-file-tests/summary.tsv" "$C/per-file-tests/summary.tsv" "主 sweep（34 包）"
pair "$B/per-file-tests-extra/summary.tsv" "$C/per-file-tests-extra/summary.tsv" "补充 sweep（分包）"
