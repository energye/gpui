#!/bin/bash
# P3 2K/4K clips (local/CI generation; big outputs stay out of git).
# Needs system ffmpeg with libx264. Outputs loop in the new windows,
# so 5s each is enough: the window cycles them for 30s/60s.
# Usage: bash video/testdata/gen_p3_2k4k.sh [outdir]
set -e
OUT=${1:-video/testdata}
mkdir -p "$OUT"
gen() { # name size rate duration
  name=$1; size=$2; rate=$3; dur=$4
  ffmpeg -v error -y -f lavfi -i "testsrc=size=${size}:rate=${rate}:duration=${dur}" \
    -c:v libx264 -profile:v main -pix_fmt yuv420p -bf 1 -g 30 \
    "$OUT/$name.mp4"
  echo "wrote $OUT/$name.mp4"
  ls -l "$OUT/$name.mp4"
  ffprobe -v error -select_streams v:0 \
    -show_entries stream=width,height,avg_frame_rate,codec_name,duration \
    -of default=noprint_wrappers=1 "$OUT/$name.mp4"
}
gen p3_2k_2560_1440_30fps 2560x1440 30 5
gen p3_4k_3840_2160_30fps 3840x2160 30 5
