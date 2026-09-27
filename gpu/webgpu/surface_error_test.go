//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"fmt"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

func TestMapSurfaceAcquireErr_Sentinels(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"occluded", rwgpu.ErrSurfaceOccluded, ErrSurfaceOccluded},
		{"timeout", rwgpu.ErrSurfaceTimeout, hal.ErrTimeout},
		{"outdated", rwgpu.ErrSurfaceNeedsReconfigure, hal.ErrSurfaceOutdated},
		{"surface lost", rwgpu.ErrSurfaceLost, hal.ErrSurfaceLost},
		{"device lost surface", rwgpu.ErrSurfaceDeviceLost, hal.ErrDeviceLost},
		{"device lost", rwgpu.ErrDeviceLost, hal.ErrDeviceLost},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapSurfaceAcquireErr(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapSurfaceAcquireErr(%v)=%v, want errors.Is %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMapSurfaceAcquireErr_Wrapped(t *testing.T) {
	err := fmt.Errorf("outer: %w", rwgpu.ErrSurfaceTimeout)
	got := mapSurfaceAcquireErr(err)
	if !errors.Is(got, hal.ErrTimeout) {
		t.Fatalf("wrapped timeout -> %v, want hal.ErrTimeout", got)
	}
}

func TestSkipFrameVsOutdatedClassification(t *testing.T) {
	if !isSkipFrameSurfaceErr(ErrSurfaceOccluded) {
		t.Fatal("occluded should skip frame")
	}
	if !isSkipFrameSurfaceErr(hal.ErrTimeout) {
		t.Fatal("timeout should skip frame")
	}
	if isSkipFrameSurfaceErr(hal.ErrSurfaceOutdated) {
		t.Fatal("outdated should not be skip-only")
	}
	if isSkipFrameSurfaceErr(hal.ErrDeviceLost) {
		t.Fatal("device lost is terminal, not skip classification")
	}
	if !isOutdatedSurfaceErr(hal.ErrSurfaceOutdated) {
		t.Fatal("outdated should reconfigure")
	}
	if !isOutdatedSurfaceErr(hal.ErrSurfaceLost) {
		t.Fatal("surface lost should reconfigure/recreate path")
	}
	if isOutdatedSurfaceErr(ErrSurfaceOccluded) {
		t.Fatal("occluded must not reconfigure")
	}
	if isOutdatedSurfaceErr(hal.ErrTimeout) {
		t.Fatal("timeout must not reconfigure")
	}
}

func TestIsDeviceLostErr_MessageFallback(t *testing.T) {
	if !isDeviceLostErr(errors.New("Parent device is lost")) {
		t.Fatal("parent device message should match")
	}
	if !isDeviceLostErr(hal.ErrDeviceLost) {
		t.Fatal("hal.ErrDeviceLost should match")
	}
	if isDeviceLostErr(hal.ErrTimeout) {
		t.Fatal("timeout is not device lost")
	}
}
