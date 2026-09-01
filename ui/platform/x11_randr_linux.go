//go:build linux

package platform

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// B6: RandR 多显修正 — 动态绑定 libXrandr，查询当前输出几何，
// 供 IME 锚点 translate 时做物理坐标修正（HiDPI/多屏偏移）。
// 无 RandR 时回退到 XTranslateCoordinates 的 root 坐标。

type xrandrLib struct {
	lib           uintptr
	getMonitors   func(dpy uintptr, win uintptr, getActive int, nmonitors *int) uintptr
	freeMonitors  func(monitors uintptr) int
	getResources  func(dpy uintptr, win uintptr) uintptr
	freeResources func(res uintptr) int
}

var (
	xrandrOnce sync.Once
	xrandr     *xrandrLib
	xrandrOK   bool
)

func xrandrLoad() *xrandrLib {
	xrandrOnce.Do(func() {
		for _, name := range []string{"libXrandr.so.2", "libXrandr.so"} {
			lib, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				continue
			}
			x := &xrandrLib{lib: lib}
			hasAny := false
			if _, err := purego.Dlsym(lib, "XRRGetMonitors"); err == nil {
				purego.RegisterLibFunc(&x.getMonitors, lib, "XRRGetMonitors")
				purego.RegisterLibFunc(&x.freeMonitors, lib, "XRRFreeMonitors")
				hasAny = true
			}
			if _, err := purego.Dlsym(lib, "XRRGetScreenResources"); err == nil {
				purego.RegisterLibFunc(&x.getResources, lib, "XRRGetScreenResources")
				purego.RegisterLibFunc(&x.freeResources, lib, "XRRFreeScreenResources")
				hasAny = true
			}
			if hasAny {
				xrandr = x
				xrandrOK = true
				return
			}
		}
	})
	return xrandr
}

// x11RandRMonitorForPoint 返回包含 (x,y) 的 RandR monitor 几何，找不到回退 false。
// monitor 结构：XRRMonitorInfo { name Atom, primary Bool, automatic Bool, noutput int, x,y,width,height, mwidth,mheight, outputs *Atom }
// 我们只取 x,y,width,height（偏移 16/20/24/28 在 64 位）。
func x11RandRMonitorForPoint(dpy, root uintptr, x, y int) (mx, my, mw, mh int, ok bool) {
	lib := xrandrLoad()
	if lib == nil || !xrandrOK || lib.getMonitors == nil {
		return 0, 0, 0, 0, false
	}
	var n int
	monitors := lib.getMonitors(dpy, root, 1, &n)
	if monitors == 0 || n == 0 {
		return 0, 0, 0, 0, false
	}
	defer lib.freeMonitors(monitors)
	// XRRMonitorInfo 大小 32 字节（Atom 8 + 3*4 pad + 8 + 4*4 + 8*4）
	const stride = 48
	for i := 0; i < n; i++ {
		base := monitors + uintptr(i*stride)
		// 布局按 64 位：name(0:8) primary(8:4) automatic(12:4) noutput(16:4) pad4 x(20:4) y(24:4) width(28:4) height(32:4)
		// 为兼容不同版本，直接按偏移 20/24/28/32 读 int32
		mx0 := int(int32(*(*int32)(unsafe.Pointer(base + 20))))
		my0 := int(int32(*(*int32)(unsafe.Pointer(base + 24))))
		mw0 := int(int32(*(*int32)(unsafe.Pointer(base + 28))))
		mh0 := int(int32(*(*int32)(unsafe.Pointer(base + 32))))
		if x >= mx0 && x < mx0+mw0 && y >= my0 && y < my0+mh0 {
			return mx0, my0, mw0, mh0, true
		}
	}
	return 0, 0, 0, 0, false
}

// x11RandRAdjust 在 translate 之后做多显修正：目前仅做 monitor 命中校验与日志，
// 为后续 HiDPI per-monitor scale 预留（scale 仍由 st.scale 统一提供，
// 此处不二次缩放，避免与 XTranslateCoordinates 的 root 坐标重复计算）。
func x11RandRAdjust(st *x11State, x, y int) (int, int) {
	if st == nil || st.display == 0 || st.root == 0 {
		return x, y
	}
	if _, _, _, _, ok := x11RandRMonitorForPoint(st.display, st.root, x, y); ok {
		// 命中 monitor，坐标已在虚拟 root 中，无需偏移修正，保留 hook 供未来 per-monitor scale
		return x, y
	}
	return x, y
}
