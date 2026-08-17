//go:build !nogpu

package gpu

import (
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// Transient command views (viewToResView / ResolveCommandView) are registered
// into the session resource registry at queue time and must be released AND
// retired at the next frame boundary. Before the fix, Release without Retire
// left the slot in the registry forever (refs==0 but retire==false), so the
// registry grew monotonically under HUD/texture blit load (RSS leak).
func TestRegTransientView_LifecycleClosesAtFrameBoundary(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	if device == nil {
		t.Skip("no native wgpu device (createNativeTestDevice unavailable)")
	}
	t.Cleanup(cleanup)

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })

	tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "transient_view_test",
		Size:          webgpu.Extent3D{Width: 8, Height: 8, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatR8Unorm,
		Usage:         types.TextureUsageTextureBinding,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tex.Release() })
	v, err := device.CreateTextureView(tex, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Release() })

	if got := s.Reg().Count(); got != 0 {
		t.Fatalf("registry should start empty, got %d entries", got)
	}

	// Simulate two frames of viewToResView-style transient registrations
	// (e.g. HUD + base layer blits per frame). These are BORROWED views —
	// owned by the texture cache — so retire removes the registry slot
	// without destroying the underlying view.
	for frame := 0; frame < 3; frame++ {
		ref := s.Reg().RegisterBorrowed(&texViewNative{v})
		s.notePendingViewRetire(ref)
		// Consumer release (buildGPUTextureResources defer) drops refs to 0;
		// the slot must survive until the frame boundary.
		s.Reg().Release(ref)
		if got := s.Reg().Count(); got != frame+1 {
			t.Fatalf("frame %d: slot should persist until boundary, registry=%d want %d", frame, got, frame+1)
		}
	}

	// BeginFrame is the frame boundary: all transient refs retire and the
	// registry slots are dropped (the views themselves stay alive — the
	// texture cache owns them).
	s.retirePendingViews()
	if got := s.Reg().Count(); got != 0 {
		t.Fatalf("after retirePendingViews registry should be empty, got %d entries", got)
	}
	if got := len(s.pendingViewRetires); got != 0 {
		t.Fatalf("pendingViewRetires should drain, got %d entries", got)
	}

	// Reusing the same underlying view for another frame must not re-leak:
	// retire is per-registration, each new registration is independent.
	ref := s.Reg().RegisterBorrowed(&texViewNative{v})
	s.notePendingViewRetire(ref)
	s.Reg().Release(ref)
	s.retirePendingViews()
	if got := s.Reg().Count(); got != 0 {
		t.Fatalf("repeat cycle leaked: registry=%d want 0", got)
	}
}

// countingNative counts Release calls to prove borrowed entries are dropped
// without destroying the native (owner destroys it separately).
type countingNative struct{ n int }

func (c *countingNative) Release() { c.n++ }

func TestRegBorrowed_RetireSkipsNativeRelease(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	if device == nil {
		t.Skip("no native wgpu device (createNativeTestDevice unavailable)")
	}
	t.Cleanup(cleanup)

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })

	// Borrowed (texture-cache-owned) entry: retire must drop the slot but
	// NOT release the native — the owner owns destruction.
	owned := &countingNative{}
	ref := s.Reg().RegisterBorrowed(owned)
	s.notePendingViewRetire(ref)
	s.Reg().Release(ref)
	s.retirePendingViews()
	if got := s.Reg().Count(); got != 0 {
		t.Fatalf("borrowed entry leaked: registry=%d want 0", got)
	}
	if owned.n != 0 {
		t.Fatalf("borrowed native must NOT be released by registry retire, releases=%d", owned.n)
	}

	// Owned (registry-owned) entry: retire must release the native exactly once.
	owned2 := &countingNative{}
	ref2 := s.Reg().Register(owned2)
	s.notePendingViewRetire(ref2)
	s.Reg().Release(ref2)
	s.retirePendingViews()
	if got := s.Reg().Count(); got != 0 {
		t.Fatalf("owned entry leaked: registry=%d want 0", got)
	}
	if owned2.n != 1 {
		t.Fatalf("owned native must be released exactly once by retire, releases=%d", owned2.n)
	}
}

// ResolveCommandView's Raw fallback registers a transient ref at resolution
// time; it must also be retired at the frame boundary, not leaked.
func TestRegTransientView_RawResolvePathRetired(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	if device == nil {
		t.Skip("no native wgpu device (createNativeTestDevice unavailable)")
	}
	t.Cleanup(cleanup)

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })

	// Raw fallback: viewToResView carries the *webgpu.TextureView Go pointer
	// (brush_advanced/filter_gpu_graph use gpucontext.NewTextureView(unsafe.
	// Pointer(view))), and ResolveCommandView registers it at flush time.
	tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "raw_view_test",
		Size:          webgpu.Extent3D{Width: 8, Height: 8, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatR8Unorm,
		Usage:         types.TextureUsageTextureBinding,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tex.Release() })
	v, err := device.CreateTextureView(tex, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Release() })

	view := res.ViewFromRaw(unsafe.Pointer(v))
	tv, ok := s.ResolveCommandView(&view)
	if !ok {
		t.Fatalf("ResolveCommandView raw path failed: ok=false")
	}
	if tv != v {
		t.Fatalf("raw resolve returned wrong view: %v want %v", tv, v)
	}
	if got := s.Reg().Count(); got != 1 {
		t.Fatalf("raw resolution should register one transient slot, got %d", got)
	}
	if got := len(s.pendingViewRetires); got != 1 {
		t.Fatalf("raw resolution should pend one retire, got %d", got)
	}

	// Frame boundary with the GPU barrier: retire closes the slot.
	s.retirePendingViews()
	if got := s.Reg().Count(); got != 0 {
		t.Fatalf("raw path leaked: registry=%d want 0", got)
	}
}