# zz_b2_bitmap_scan_test.go 长跑复跑（P1.3 纪律 3600s 上限）
# date: 2026-10-30T08:15:07+08:00
# 命令: cd render/text && go test -count=1 -timeout 3600s -run '^(TestB2BitmapCJK|TestB2BitmapKorean|TestB2BitmapThai|TestB2BitmapTTF)$' -v .
# 结果: ok github.com/energye/gpui/render/text 1264.275s (rc=0)，4/4 PASS
# 对照: P1.3 同文件 PASS 1256.2s（baseline/tests/summary.tsv）——逐文件轮的 FAIL 是 per-file 脚本 20m timeout 所致（脚本限制非回归），P1.3 对长跑文件单给 3600s 口径，P2.4b 沿用

# 逐文件最终定性（32 FAIL）：
# - 31 个与 P1.3 基线逐文件同红（internal/gpu 18 = P2.4a 同集；render 9（P2.4a 为 10，mem_window 竞态本轮未复现）；ui/scene 4（caret/shrink/textured/textured_budget，P1.3 §4.2 已登记）；render/text 1 = zz_b2 超时工件，本轮复跑 PASS）
# - 零新增红
