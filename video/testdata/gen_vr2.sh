#!/bin/bash
# VR2 ground-truth clips (local/CI generation; big outputs stay out of git).
# Needs system ffmpeg with libx264. Raw YUV is the decode oracle.
set -e
OUT=${1:-video/testdata}
mkdir -p "$OUT"
gen() { # name filter profile extra...
  name=$1; filter=$2; profile=$3; shift 3
  ffmpeg -v error -y -f lavfi -i "$filter" -c:v libx264 -profile:v "$profile" \
    -pix_fmt yuv420p "$@" "$OUT/$name.mp4"
  ffmpeg -v error -y -i "$OUT/$name.mp4" -f rawvideo -pix_fmt yuv420p "$OUT/$name.yuv"
}
SRC=testsrc=size=96x96:rate=5:duration=1
# VR2a: Baseline I-only (intra + CAVLC only).
gen vr2_b_intra "$SRC" baseline -bf 0 -g 1
# VR2b: Baseline mixed IP (adds P frames + motion).
gen vr2_b_mixed "$SRC" baseline -bf 0 -g 5
# VR2b F18: static grey (P frames dominated by skips).
gen vr2_b_skip color=size=96x96:rate=5:duration=1:color=0x808080 baseline -bf 0 -g 5
# VR2c: Main mixed (Main profile proof).
gen vr2_m_main "$SRC" main -bf 0 -g 5
# VR2c F6/F9: High mixed with 8x8 intra/transform (mandelbrot detail
# forces 8x8 blocks; I + 4P, no B to isolate from reorder).
gen vr2_h_8x8 "mandelbrot=size=96x96:rate=5:end_pts=5" high -frames:v 5 -bf 0 -g 5
# VR2c F9: High mixed with JVT default scaling lists (matrix present,
# all implicit: dequant must use the default tables, not flat).
gen vr2_h_scale "mandelbrot=size=96x96:rate=5:end_pts=5" high -frames:v 5 -bf 0 -g 5 -x264-params cqm=jvt
# VR2c F5/F9: High mixed with CAVLC entropy + 8x8 (dual-entropy 8x8 proof).
gen vr2_h_cavlc "mandelbrot=size=96x96:rate=5:end_pts=5" high -frames:v 5 -bf 0 -g 5 -x264-params no-cabac=1
# VR2d: Main with 2 consecutive B frames (reorder proof).
gen vr2_m_bframes "$SRC" main -bf 2 -g 8
# VR2d matrix (short 5-frame clips per档; 1440p/4K local-only, not committed).
gen vr2_480p testsrc=size=854x480:rate=5:duration=1 main -bf 1 -g 5
gen vr2_720p testsrc=size=1280x720:rate=5:duration=1 main -bf 1 -g 5
ls -l "$OUT"/vr2_*
