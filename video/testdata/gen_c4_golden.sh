#!/usr/bin/env bash
# Regenerates the C4 retained-texture Golden fixture.
#
# One NV12 frame, 904x656 = the wrkit body panel at 1200x800, so
# examples/video/video_retained_golden draws it 1:1 (no scaling, no sampler
# differences between runs). Eight colour fields plus a white cross: U and V
# differ in every primary, so a swapped or zeroed chroma plane cannot pass the
# window's colour probes. Regenerate only when the fixture itself must change;
# the committed .yuv is the reference the Golden PNG was recorded from.
set -euo pipefail
cd "$(dirname "$0")"

out=c4_golden_904x656_nv12.yuv
ffmpeg -v error -y -f lavfi -i "color=c=black:s=904x656:r=1" \
	-vf "drawbox=x=0:y=0:w=226:h=328:color=red:t=fill,drawbox=x=226:y=0:w=226:h=328:color=green:t=fill,drawbox=x=452:y=0:w=226:h=328:color=blue:t=fill,drawbox=x=678:y=0:w=226:h=328:color=white:t=fill,drawbox=x=0:y=328:w=226:h=328:color=yellow:t=fill,drawbox=x=226:y=328:w=226:h=328:color=cyan:t=fill,drawbox=x=452:y=328:w=226:h=328:color=magenta:t=fill,drawbox=x=678:y=328:w=226:h=328:color=gray:t=fill,drawbox=x=451:y=0:w=2:h=656:color=white:t=fill,drawbox=x=0:y=327:w=904:h=2:color=white:t=fill" \
	-frames:v 1 -pix_fmt nv12 -f rawvideo "$out"

expected=22926ecfe1355180b02c331d9ebec03f
got=$(md5sum "$out" | cut -d' ' -f1)
echo "$out md5=$got (expected $expected)"
if [ "$got" != "$expected" ]; then
	echo "WARN: fixture hash changed; re-seed the Golden PNG and review it" >&2
fi
