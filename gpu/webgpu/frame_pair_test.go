//go:build !(js && wasm) && !nogpu

package webgpu

import (
	"errors"
	"sync"
	"testing"

	"github.com/energye/gpui/gpu/rwgpu"
)

func TestFramePairing_DoubleBeginRefused(t *testing.T) {
	// Ensure sticky fuse is clear so frame pairing is the path under test.
	was := rwgpu.AnyDeviceLost()
	if was {
		t.Skip("process sticky device-lost set; cannot assert frame pairing")
	}
	sc := NewSwapchain(nil, nil, 64, 64)
	// Provide a non-released surface shell so frameOpen check is reached.
	sc.Surface = &Surface{}
	sc.frameOpen = true
	_, err := sc.BeginFrame()
	if !errors.Is(err, ErrFrameInFlight) {
		t.Fatalf("double BeginFrame: %v, want ErrFrameInFlight", err)
	}
	// Discard clears open.
	sc.DiscardFrame(&Frame{})
	sc.frameMu.Lock()
	open := sc.frameOpen
	sc.frameMu.Unlock()
	if open {
		t.Fatal("DiscardFrame must clear frameOpen")
	}
	// EndFrame without open.
	if err := sc.EndFrame(&Frame{}); !errors.Is(err, ErrNoFrame) {
		t.Fatalf("EndFrame without open: %v", err)
	}
}

func TestBeginFrame_StickyDeviceLost(t *testing.T) {
	// Trip sticky fuse via public test hook, then assert real BeginFrame path.
	// Restore sticky so later tests in the package are not poisoned.
	was := rwgpu.AnyDeviceLost()
	defer func() {
		if !was {
			rwgpu.ResetDeviceLostForTest()
		}
	}()
	rwgpu.ResetDeviceLostForTest()
	rwgpu.MarkDeviceLostForTest(0xbadcafe)

	if !rwgpu.AnyDeviceLost() {
		t.Fatal("MarkDeviceLostForTest must set process sticky fuse")
	}

	sc := NewSwapchain(&Surface{}, nil, 64, 64)
	_, err := sc.BeginFrame()
	if !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("sticky lost BeginFrame: %v, want ErrDeviceLost", err)
	}
	// Frame must not open after refuse.
	sc.frameMu.Lock()
	open := sc.frameOpen
	sc.frameMu.Unlock()
	if open {
		t.Fatal("BeginFrame after sticky lost must not set frameOpen")
	}
}

func TestBeginFrame_ReleasedSurface(t *testing.T) {
	// Clear sticky so released-surface path is observable.
	was := rwgpu.AnyDeviceLost()
	if was {
		t.Skip("sticky device-lost set; cannot assert released surface")
	}
	sc := NewSwapchain(&Surface{released: true}, nil, 64, 64)
	_, err := sc.BeginFrame()
	if !errors.Is(err, ErrReleased) {
		t.Fatalf("released surface: %v, want ErrReleased", err)
	}
}

func TestConcurrentBeginFrame_NoPanic(t *testing.T) {
	// Concurrent BeginFrame/DiscardFrame on same swapchain must not race frameOpen.
	was := rwgpu.AnyDeviceLost()
	if was {
		// Sticky path still exercises concurrent lock without panic.
	}
	sc := NewSwapchain(&Surface{}, nil, 64, 64)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sc.BeginFrame()
			sc.DiscardFrame(&Frame{})
		}()
	}
	wg.Wait()
}
