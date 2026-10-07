//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"image"
	"sync"
	"sync/atomic"
	"time"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
)

// Swapchain manages Configure → GetCurrentTexture → Present for a platform Surface.
// It is the production path for window presentation; offscreen textures remain a headless stand-in.
//

// acquireTimeout bounds a single surface acquire. Skia (Ganesh Vulkan) and
// Flutter (Impeller) do NOT bound the acquire — they assume present/acquire
// stay paired and recover only via swapchain recreate on VK_ERROR_OUT_OF_DATE
// (the DiscardTexture+Configure path below is that alignment). This timeout
// is an environment guard for software Vulkan (llvmpipe): once swapchain
// images stop being returned (a lost/dropped present) wgpu-native's
// GetCurrentTexture can block forever, and without a bound the frame loop
// freezes. On timeout the hung latch routes later frames through the
// recreate instead of re-entering the dead acquire.
const acquireTimeout = 250 * time.Millisecond

// ErrAcquireTimeout reports a surface acquire that did not return in time.
// The swapchain is recreated (Configure) before the next acquire attempt.
var ErrAcquireTimeout = errors.New("wgpu: surface acquire timed out")

type Swapchain struct {
	Surface     *Surface
	Device      hal.Device
	Width       uint32
	Height      uint32
	Format      TextureFormat
	Usage       TextureUsage
	PresentMode PresentMode
	AlphaMode   CompositeAlphaMode

	// PreferPresentModes, when non-empty, is tried in order during
	// ConfigureFromCapabilities. Empty → prefer Fifo then first available.
	PreferPresentModes []PresentMode

	// supportedPresentModes caches the adapter's surface capability list at
	// ConfigureFromCapabilities time, so a runtime present-mode switch
	// (SetPresentModeForce / PresentModeForVsync) can pick a supported mode
	// without re-querying the adapter (the PresentTarget releases the adapter
	// after init).
	supportedPresentModes []PresentMode

	configured         bool
	pendingReconfigure bool
	// suboptHandledW/H: last extent for which we already acted on a suboptimal
	// signal. Prevents Configure thrash (and visible flicker) when the
	// compositor keeps reporting suboptimal for an unchanged size.
	suboptHandledW uint32
	suboptHandledH uint32

	// stats
	acquires        uint64
	presents        uint64
	discards        uint64
	reconfigures    uint64
	suboptimal      uint64
	acquireRetries  uint64
	acquireTimeouts uint64
	lastAcquireNs   int64
	lastPresentNs   int64

	// acquireHung latches after a timed-out surface acquire (software Vulkan
	// can block forever once swapchain images are exhausted). While set,
	// BeginFrame skips the cgo acquire and routes to the throttled
	// DiscardTexture+Configure recreate; a successful Configure clears it.
	acquireHung  atomic.Bool
	hungMarkedAt time.Time

	// lastReconfig rate-limits native Surface.Configure. Continuous reconfigure
	// under long stress can abort wgpu-native ("failed to initiate panic").
	lastReconfig time.Time

	// frameOpen is true between a successful BeginFrame and EndFrame/DiscardFrame.
	// Enforces one-in-flight pairing: BeginFrame while open is an error.
	// Protected by frameMu for concurrent BeginFrame/EndFrame/DiscardFrame.
	frameMu   sync.Mutex
	frameOpen bool

	// --- Device-lost auto recovery (library-level, optional) ---
	// When RecoveryAdapter is set, BeginFrame attempts RequestDevice + reconfigure
	// instead of permanently failing.
	RecoveryAdapter hal.Adapter
	// OnDeviceAbandon is called on the sticky-lost device BEFORE it is Destroy/Release'd
	// and before RequestDevice. Host must drop all GPU objects (pipelines, pools,
	// sessions via deviceGen) so VRAM is free for the new device — otherwise the
	// next CreateTexture hits "Not enough memory left".
	OnDeviceAbandon func(oldDevice hal.Device)
	// OnDeviceRecreated is called after a successful recovery with the new device.
	// Apps rebind accelerators / device providers here.
	OnDeviceRecreated func(newDevice hal.Device)
	// DeviceLabel is passed to RequestDevice during recovery.
	DeviceLabel     string
	recoverAttempts uint64
	lastRecoverAt   time.Time
	recoverCooldown time.Duration // min interval between recover tries (default 1s)
	recoverGrace    int           // BeginFrames to skip after recover (ErrRecovered)
}

// Frame is one acquired swapchain image ready for rendering.
type Frame struct {
	SurfaceTexture *SurfaceTexture
	View           *TextureView
	// Handle is the gpucontext-facing view for render.FlushGPUWithView.
	Handle     gpucontext.TextureView
	Suboptimal bool
	Width      uint32
	Height     uint32
	// DamageRects optionally records dirty regions for EndFrameWithDamage.
	// wgpu-native currently ignores them at present; still used for diagnostics
	// and future partial-present backends.
	DamageRects []image.Rectangle
}

type SwapchainStats struct {
	Acquires       uint64
	Presents       uint64
	Discards       uint64
	Reconfigures   uint64
	Suboptimal     uint64
	AcquireRetries uint64
	LastAcquireNs  int64
	LastPresentNs  int64
	// Derived: last present wall time in milliseconds.
	LastPresentMs float64
	LastAcquireMs float64
}
