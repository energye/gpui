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
	// S6-P0 scale notices: extension event base + root selection.
	queryExtension func(dpy uintptr, eventBase, errorBase *int32) int
	selectInput    func(dpy, win uintptr, mask int) int
}

// RandR screen-change notice (Xrandr.h): the event number is an offset from
// the extension event base; RRScreenChangeNotifyMask selects it on the root.
const (
	rrScreenChangeNotify     = 0
	rrScreenChangeNotifyMask = 1
)

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
			if _, err := purego.Dlsym(lib, "XRRQueryExtension"); err == nil {
				purego.RegisterLibFunc(&x.queryExtension, lib, "XRRQueryExtension")
				hasAny = true
			}
			if _, err := purego.Dlsym(lib, "XRRSelectInput"); err == nil {
				purego.RegisterLibFunc(&x.selectInput, lib, "XRRSelectInput")
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

// x11ScaleSource reports the logical scale for st (Xft.dpi / 96, unknown →
// 1). A package var so synthetic-event tests can pin a fixed scale without
// touching the live root resources; production never swaps it.
var x11ScaleSource = x11LiveScale

// x11LiveScale reads the scale from the X server's RESOURCE_MANAGER property
// (Xft.dpi, the value GNOME writes when the user flips the display scale —
// that flip changes no pixels, so RandR alone would miss it). Physical
// monitor DPI is deliberately NOT a source: real panels report ~166 DPI,
// which would wrongly rescale a 96-dpi desktop.
func x11LiveScale(st *x11State) float64 {
	if st == nil || st.lib == nil || st.lib.resourceManagerString == nil || st.display == 0 {
		return 1
	}
	return x11ScaleFromDPI(x11ParseDPI(x11CString(st.lib.resourceManagerString(st.display))))
}

// x11ScaleFromDPI maps an Xft.dpi value to the logical scale (96 = 1).
// Unknown (<=0) and absurd readings fall back to 1 rather than disturbing
// layout with a garbage factor; sub-unity desktops are out of scope.
func x11ScaleFromDPI(dpi float64) float64 {
	if dpi < 96 || dpi > 96*4 {
		return 1
	}
	return dpi / 96
}

// x11ParseDPI extracts the Xft.dpi value from an X RESOURCE_MANAGER string
// ("Xft.dpi:<ws>192" lines). Returns 0 when absent or unparseable.
func x11ParseDPI(res string) float64 {
	const key = "Xft.dpi:"
	for i := 0; i+len(key) <= len(res); i++ {
		if res[i:i+len(key)] != key {
			continue
		}
		if i > 0 && res[i-1] != '\n' {
			continue // not at a line start
		}
		j := i + len(key)
		for j < len(res) && (res[j] == ' ' || res[j] == '\t') {
			j++
		}
		if v, ok := x11ParseDec(res[j:]); ok {
			return v
		}
	}
	return 0
}

// x11ParseDec parses a leading decimal (digits[.digits]) from s.
func x11ParseDec(s string) (float64, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false
	}
	v := 0.0
	for k := 0; k < i; k++ {
		v = v*10 + float64(s[k]-'0')
	}
	if i < len(s) && s[i] == '.' {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		frac, div := 0.0, 1.0
		for k := i + 1; k < j; k++ {
			frac = frac*10 + float64(s[k]-'0')
			div *= 10
		}
		return v + frac/div, true
	}
	return v, true
}

// x11CString copies a NUL-terminated C string (nil → "").
func x11CString(p *byte) string {
	if p == nil {
		return ""
	}
	var b []byte
	for q := p; *q != 0; q = (*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(q)) + 1)) {
		b = append(b, *q)
	}
	return string(b)
}

// x11RandRAdjust 在 translate 之后做多显修正：目前为 no-op 钩子，
// 为后续 HiDPI per-monitor scale 预留（scale 仍由 st.scale 统一提供，
// 此处不二次缩放，避免与 XTranslateCoordinates 的 root 坐标重复计算）。
// 之前每帧 XRRGetMonitors 查询已移除以免高频开销，需要时再按需缓存。
func x11RandRAdjust(st *x11State, x, y int) (int, int) {
	return x, y
}
