package render

import "sync/atomic"

// MSAA sample-count constants for SetDefaultSampleCount / DefaultSampleCount.
//
// The rendering engine defaults to 4x MSAA (see docs/RENDER_API_CATALOG.md):
// soft edges for circles / 1px borders / icons. 1x produces hard binary
// coverage and is only the default on effect offscreens (SetEffectSurface),
// software paths, and OOM downgrades. These constants let callers configure
// the global default in code.
const (
	// MSAASampleCount1 disables multisampling (1 sample per pixel).
	MSAASampleCount1 uint32 = 1
	// MSAASampleCount4 is the engine default (4x multisampling).
	MSAASampleCount4 uint32 = 4
)

// defaultSampleCount is the explicit global default MSAA sample count.
// 0 = not set → auto (device probe, then 4).
var defaultSampleCount atomic.Uint32

// SetDefaultSampleCount overrides the engine's default MSAA sample count for
// every GPU session created afterwards (external/window devices and any
// context that does not opt into 1x via SetEffectSurface).
//
//	SetDefaultSampleCount(MSAASampleCount1) // force global 1x
//	SetDefaultSampleCount(MSAASampleCount4) // restore 4x
//	SetDefaultSampleCount(0)                // reset to auto (probe → 4x)
//
// Values other than 0/1/4 are passed through to the wgpu device; only 1 and 4
// are guaranteed supported by the surface/session pipelines. Call before the
// GPU accelerator is initialized (before the first window opens) for it to
// take effect on existing sessions.
func SetDefaultSampleCount(n uint32) {
	defaultSampleCount.Store(n)
}

// DefaultSampleCount returns the explicit default MSAA sample count set by
// SetDefaultSampleCount, or 0 when auto (the engine then device-probes 4x
// support and falls back to 4x).
func DefaultSampleCount() uint32 {
	return defaultSampleCount.Load()
}
