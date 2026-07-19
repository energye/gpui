//go:build !(js && wasm) && !nogpu

package webgpu

import (
	"errors"
	"sync"
	"testing"
)

func TestFramePairing_DoubleBeginRefused(t *testing.T) {
	sc := NewSwapchain(nil, nil, 64, 64)
	sc.Surface = &Surface{}
	sc.frameOpen = true
	_, err := sc.BeginFrame()
	if !errors.Is(err, ErrFrameInFlight) {
		t.Fatalf("double BeginFrame: %v, want ErrFrameInFlight", err)
	}
	sc.DiscardFrame(&Frame{})
	sc.frameMu.Lock()
	open := sc.frameOpen
	sc.frameMu.Unlock()
	if open {
		t.Fatal("DiscardFrame must clear frameOpen")
	}
	if err := sc.EndFrame(&Frame{}); !errors.Is(err, ErrNoFrame) {
		t.Fatalf("EndFrame without open: %v", err)
	}
}

func TestBeginFrame_ReleasedSurface(t *testing.T) {
	sc := NewSwapchain(&Surface{released: true}, nil, 64, 64)
	_, err := sc.BeginFrame()
	if !errors.Is(err, ErrReleased) {
		t.Fatalf("released surface: %v, want ErrReleased", err)
	}
}

func TestConcurrentBeginFrame_NoPanic(t *testing.T) {
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
