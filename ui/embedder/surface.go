package embedder

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/platform"
)

// MapNativeSurface converts platform handles to render.PresentNativeSurface.
func MapNativeSurface(ns platform.NativeSurface) render.PresentNativeSurface {
	var p render.PresentPlatform
	switch ns.Kind {
	case platform.PlatformWayland:
		p = render.PresentPlatformWayland
	case platform.PlatformWin32:
		p = render.PresentPlatformWin32
	case platform.PlatformAppKit:
		p = render.PresentPlatformAppKit
	default:
		p = render.PresentPlatformX11
	}
	return render.PresentNativeSurface{
		Platform: p,
		Display:  ns.Display,
		Window:   ns.Window,
	}
}

// OpenPresentTarget opens a render.PresentTarget from the host surface and size.
func OpenPresentTarget(host platform.Host) (*render.PresentTarget, error) {
	if host == nil {
		return nil, fmt.Errorf("embedder: nil host")
	}
	w, h := host.Size()
	scale := host.ScaleFactor()
	if scale <= 0 {
		scale = 1
	}
	return render.NewPresentTarget(MapNativeSurface(host.NativeSurface()), w, h, scale)
}
