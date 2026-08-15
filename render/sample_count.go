package render

import "sync/atomic"

// MSAA sample-count constants for SetMSAASampleCount / MSAASampleCount.
//
// The rendering engine defaults to 1x MSAA: edges are anti-aliased by the
// analytic fringe coverage in the stencil cover path (Skia kCoverage
// semantics) instead of multisampling. 1x keeps GPU cost low and matches CPU
// scanline AA. Call SetMSAASampleCount(MSAASampleCount4) for 4x MSAA (window
// chrome / explicit hi-quality config); effect offscreens (SetEffectSurface)
// and OOM downgrades always run 1x.
const (
	// MSAASampleCount1 disables multisampling (1 sample per pixel).
	MSAASampleCount1 uint32 = 1
	// MSAASampleCount4 enables 4x multisampling (opt-in, via SetMSAASampleCount).
	MSAASampleCount4 uint32 = 4
)

// defaultSampleCount is the explicit global default MSAA sample count.
// 0 = not set → engine default of 1x.
var defaultSampleCount atomic.Uint32

// SetMSAASampleCount overrides the engine default MSAA sample count for every
// GPU session created afterwards (external/window devices and any context
// that does not opt into 1x via SetEffectSurface).
//
//	SetMSAASampleCount(MSAASampleCount1) // force global 1x (engine default)
//	SetMSAASampleCount(MSAASampleCount4) // opt into 4x MSAA
//	SetMSAASampleCount(0)                // reset to engine default (1x)
//
// Values other than 0/1/4 are passed through to the wgpu device; only 1 and 4
// are guaranteed supported by the surface/session pipelines. Call before the
// GPU accelerator is initialized (before the first window opens) for it to
// take effect on existing sessions.
func SetMSAASampleCount(n uint32) {
	defaultSampleCount.Store(n)
}

// MSAASampleCount returns the MSAA sample count to use: the value set by
// SetMSAASampleCount, or the engine default of 1x when unset. Callers use the
// result directly without extra defaults.
func MSAASampleCount() uint32 {
	if n := defaultSampleCount.Load(); n > 0 {
		return n
	}
	return MSAASampleCount1
}
