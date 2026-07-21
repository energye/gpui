//go:build linux && !nogpu

package exboot

import (
	"log"

	"github.com/energye/gpui/gpu/webgpu"
	rendgpu "github.com/energye/gpui/render/gpu"
)

// SurfaceHost binds swapchain + optional drop callback for adaptive lifecycle.
type SurfaceHost struct {
	SC      *webgpu.Swapchain
	Adapter *webgpu.Adapter
	Device  **webgpu.Device // optional; updated on recreate
	DropGPU func()          // DropGPURenderContext / effect RTs; may be nil
	Format  webgpu.TextureFormat
	// lastTier logged once when it changes
	loggedTier      rendgpu.SurfaceLifecycle
	abandoned       bool   // true after hide-path AbandonDevice until rebind/recreate
	lastOOMSeen     uint32 // TextureOOMCount already handled by RecoverIfOOMPressure
	surfaceDetached bool   // Unconfigure ran while hidden
}

// OnUnpresentable applies the portable hide policy (minimize / fully obscured).
//
// Skia/Flutter mapping:
//   - Always stop presenting (caller responsibility).
//   - Normal: keep device+surface configured (desktop Flutter often keeps surface).
//   - Purge: Unconfigure swapchain + purge surface GPU + DropGPU (freeGpuResources).
//   - Recreate: Purge path + AbandonDevice (abandonContext); resume recreates device.
func (h *SurfaceHost) OnUnpresentable() {
	if h == nil || h.SC == nil {
		return
	}
	tier := rendgpu.ResolveSurfaceLifecycle(h.Adapter)
	if tier != h.loggedTier {
		log.Printf("exboot: surface lifecycle tier=%s (oom_notes=%d)", tier, rendgpu.TextureOOMCount())
		h.loggedTier = tier
	}

	// Engine purge is always safe and cheap relative to full-size depth/MSAA.
	// Also runs via webgpu.AfterSurfaceUnconfigure when Unconfigure is called.
	rendgpu.PurgeSurfaceResources()

	if tier == rendgpu.LifecycleNormal {
		// Flutter-light: do not Unconfigure; host only pauses acquire/present.
		return
	}

	// Purge + Recreate: release swapchain images (platform surface buffers gone).
	if h.SC.Surface != nil && h.SC.Device != nil && !h.SC.Device.IsLost() {
		h.SC.Surface.Unconfigure()
		h.SC.MarkNeedsReconfigure()
		h.surfaceDetached = true
	}
	if h.DropGPU != nil {
		h.DropGPU()
	}
	if tier >= rendgpu.LifecycleRecreate {
		rendgpu.AbandonDevice()
		h.abandoned = true
	}
}

// OnPresentable applies the portable show/resume policy.
//
// After Unconfigure (surfaceDetached), recreate the device via ForceRecoverHealthy
// when tier is Recreate, we abandoned, or adaptive OOM has fired — otherwise
// reconfigure the existing device (Purge). Normal simply clears cooldown.
func (h *SurfaceHost) OnPresentable() {
	if h == nil || h.SC == nil {
		return
	}
	tier := rendgpu.ResolveSurfaceLifecycle(h.Adapter)
	if tier != h.loggedTier {
		log.Printf("exboot: surface lifecycle tier=%s (oom_notes=%d)", tier, rendgpu.TextureOOMCount())
		h.loggedTier = tier
	}
	h.SC.ClearRecoverCooldown()

	needRecreate := h.abandoned || tier >= rendgpu.LifecycleRecreate || rendgpu.TextureOOMCount() > 0
	// Surface was Unconfigured: Flutter mobile rebinds; on tight/OOM GPUs recreate.
	// Purge without OOM: same device + reconfigure (avoid thrashing high-VRAM desktops).
	if needRecreate && h.Adapter != nil {
		if err := h.SC.ForceRecoverHealthy(); err != nil {
			log.Printf("exboot: ForceRecoverHealthy on resume: %v — fallback reconfigure", err)
			h.SC.MarkNeedsReconfigure()
			if h.SC.Device != nil {
				if h.Device != nil {
					*h.Device = h.SC.Device
				}
				if err := BindProvider(h.SC.Device, h.Adapter, h.Format); err != nil {
					log.Printf("exboot: BindProvider fallback: %v", err)
				}
			}
		} else if h.SC.Device != nil {
			if h.Device != nil {
				*h.Device = h.SC.Device
			}
			if err := BindProvider(h.SC.Device, h.Adapter, h.Format); err != nil {
				log.Printf("exboot: BindProvider after resume recreate: %v", err)
			}
		}
		h.abandoned = false
		h.surfaceDetached = false
		return
	}

	h.SC.MarkNeedsReconfigure()
	if h.SC.Device != nil && h.Device != nil {
		*h.Device = h.SC.Device
	}
	h.surfaceDetached = false
}

// RecoverIfOOMPressure performs a one-shot healthy device recreate when the
// process has already observed CreateTexture OOM. Call after a frame that may
// have allocated GPU memory. Returns true if a recover was attempted.
//
// This closes the adaptive loop: first OOM notes pressure → next frame
// recreates on any GPU (not only 1GB cards), without forcing recreate on
// every minimize for high-VRAM machines that never OOM.
func (h *SurfaceHost) RecoverIfOOMPressure() bool {
	if h == nil || h.SC == nil || h.Adapter == nil {
		return false
	}
	n := rendgpu.TextureOOMCount()
	if n == 0 || n <= h.lastOOMSeen {
		return false
	}
	// One recreate per new OOM observation (portable adaptive loop).
	if err := h.SC.ForceRecoverHealthy(); err != nil {
		log.Printf("exboot: RecoverIfOOMPressure: %v", err)
		return false
	}
	if h.SC.Device != nil {
		if h.Device != nil {
			*h.Device = h.SC.Device
		}
		_ = BindProvider(h.SC.Device, h.Adapter, h.Format)
	}
	h.abandoned = false
	h.lastOOMSeen = n
	log.Printf("exboot: RecoverIfOOMPressure ok recoveries=%d oom_notes=%d", h.SC.Recoveries(), n)
	return true
}
