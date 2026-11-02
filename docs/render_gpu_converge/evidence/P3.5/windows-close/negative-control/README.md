# 负向对照：摘引擎修复→钉必须红 2026-11-02T11:43:07+08:00
## 1) Path.RRect 方角仍发退化 cubic
--- FAIL: TestRRectSquareCornersEmitNoDegenerateCubics (0.00s)
    rrect_test.go:348: corner 0: cubics=4 degenerate=3, want 1/0
--- FAIL: TestRRectSingleCornerRadiiRasterizeRounded (0.28s)
    rrect_test.go:388: gpu corner 3: probe 3 inked=true want false
FAIL
FAIL	github.com/energye/gpui/render	0.355s
FAIL
## 2) 还原后
ok  	github.com/energye/gpui/render	0.363s
## 3) 摘 clipPathCoverageMask 越界保护
--- FAIL: TestClipPathCoverageMaskOffSurfaceBBox (0.00s)
panic: runtime error: slice bounds out of range [259:256] [recovered, repanicked]
panic({0xae6a60?, 0xc0000ecbb8?})
	/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64/src/runtime/panic.go:783 +0x132
FAIL	github.com/energye/gpui/render/internal/gpu	0.010s
FAIL
## 4) 还原后
ok  	github.com/energye/gpui/render/internal/gpu	0.008s
 render/internal/gpu/depth_clip.go |  6 +++++
 render/path_build.go              | 55 +++++++++++++++++++++++++--------------
 2 files changed, 41 insertions(+), 20 deletions(-)
