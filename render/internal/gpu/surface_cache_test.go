package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// TestSurfaceCache_LoadOpCrossFrame verifies the A2 retained compositing core:
// cacheHasContent survives BeginFrame (unlike frameRendered), so steady
// frames LoadOpLoad the persistent cache texture while the first frame (and
// any texture rebuild) LoadOpClears.
func TestSurfaceCache_LoadOpCrossFrame(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, 1)
	defer s.Destroy()
	s.SetSurfaceCacheMode(true)
	defer s.SetSurfaceCacheMode(false)

	// Simulate the first render pass creating the persistent cache texture.
	if err := s.textures.ensureSurfaceCache(device, 64, 64, "test"); err != nil {
		t.Fatalf("ensureSurfaceCache: %v", err)
	}

	if s.SurfaceCacheView() == nil {
		t.Fatal("cache view should exist after SetSurfaceCacheMode in 1x session")
	}

	// Frame 1 (steady): cacheHasContent=false → first render must LoadOpClear.
	op1 := s.cacheLoadOp(s.textures.cacheView)
	if op1 != types.LoadOpClear {
		t.Fatalf("frame 1 should LoadOpClear, got %v", op1)
	}
	// Mark content rendered → subsequent steady frames LoadOpLoad.
	s.markCacheContentRendered()
	op2 := s.cacheLoadOp(s.textures.cacheView)
	if op2 != types.LoadOpLoad {
		t.Fatalf("steady frame should LoadOpLoad, got %v", op2)
	}

	// BeginFrame resets frameRendered but must NOT wipe cacheHasContent.
	s.BeginFrame()
	if !s.cacheHasContent {
		t.Fatal("cacheHasContent must survive BeginFrame (retained compositing)")
	}
	op3 := s.cacheLoadOp(s.textures.cacheView)
	if op3 != types.LoadOpLoad {
		t.Fatalf("steady frame after BeginFrame should LoadOpLoad, got %v", op3)
	}
}

// TestSurfaceCache_TextureRebuildInvalidates verifies that a cache texture
// rebuild (resize / recreate) resets cacheHasContent so the new texture
// LoadOpClears on its first frame instead of loading garbage.
func TestSurfaceCache_TextureRebuildInvalidates(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, 1)
	defer s.Destroy()
	s.SetSurfaceCacheMode(true)
	defer s.SetSurfaceCacheMode(false)

	// Simulate the first render pass creating the persistent cache texture.
	if err := s.textures.ensureSurfaceCache(device, 64, 64, "test"); err != nil {
		t.Fatalf("ensureSurfaceCache: %v", err)
	}

	// Simulate a resolved steady frame.
	s.cacheHasContent = true
	s.cacheGenSeen = s.textures.cacheGen
	if op := s.cacheLoadOp(s.textures.cacheView); op != types.LoadOpLoad {
		t.Fatalf("steady frame should LoadOpLoad before rebuild, got %v", op)
	}

	// Rebuild the cache texture (e.g. window resize) → cacheGen bumps.
	s.textures.cacheGen++
	op := s.cacheLoadOp(s.textures.cacheView)
	if op != types.LoadOpClear {
		t.Fatalf("after rebuild first frame should LoadOpClear, got %v", op)
	}
	if s.cacheGenSeen != s.textures.cacheGen {
		t.Fatal("cacheGenSeen should track the new generation")
	}
}

// TestSurfaceCache_NonCachePathUnaffected verifies cache mode does not change
// load-op behavior for non-cache views (layer RTs, scratch, other surfaces).
func TestSurfaceCache_NonCachePathUnaffected(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, 1)
	defer s.Destroy()
	s.SetSurfaceCacheMode(true)
	defer s.SetSurfaceCacheMode(false)

	// A non-cache view follows legacy frameRendered logic.
	s.frameRendered = true
	if op := s.cacheLoadOp(s.textures.stencilView); op != types.LoadOpLoad {
		t.Fatalf("non-cache view with frameRendered should LoadOpLoad, got %v", op)
	}
	s.frameRendered = false
	if op := s.cacheLoadOp(s.textures.stencilView); op != types.LoadOpClear {
		t.Fatalf("non-cache view without frameRendered should LoadOpClear, got %v", op)
	}

	// Disabling cache mode resets retained state.
	s.SetSurfaceCacheMode(false)
	s.cacheHasContent = true
	if op := s.cacheLoadOp(s.textures.cacheView); op != types.LoadOpClear {
		t.Fatalf("cache mode off should LoadOpClear the cache view, got %v", op)
	}
}
