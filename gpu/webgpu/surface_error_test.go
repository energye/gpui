//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"fmt"
	"testing"

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
		{"timeout", rwgpu.ErrSurfaceTimeout, ErrTimeout},
		{"outdated", rwgpu.ErrSurfaceNeedsReconfigure, ErrSurfaceOutdated},
		{"surface lost", rwgpu.ErrSurfaceLost, ErrSurfaceLost},
		{"device lost surface", rwgpu.ErrSurfaceDeviceLost, ErrDeviceLost},
		{"device lost", rwgpu.ErrDeviceLost, ErrDeviceLost},
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
	if !errors.Is(got, ErrTimeout) {
		t.Fatalf("wrapped timeout -> %v, want ErrTimeout", got)
	}
}

func TestSkipFrameVsOutdatedClassification(t *testing.T) {
	if !isSkipFrameSurfaceErr(ErrSurfaceOccluded) {
		t.Fatal("occluded should skip frame")
	}
	if !isSkipFrameSurfaceErr(ErrTimeout) {
		t.Fatal("timeout should skip frame")
	}
	if isSkipFrameSurfaceErr(ErrSurfaceOutdated) {
		t.Fatal("outdated should not be skip-only")
	}
	if isSkipFrameSurfaceErr(ErrDeviceLost) {
		t.Fatal("device lost is terminal, not skip classification")
	}
	if !isOutdatedSurfaceErr(ErrSurfaceOutdated) {
		t.Fatal("outdated should reconfigure")
	}
	if !isOutdatedSurfaceErr(ErrSurfaceLost) {
		t.Fatal("surface lost should reconfigure/recreate path")
	}
	if isOutdatedSurfaceErr(ErrSurfaceOccluded) {
		t.Fatal("occluded must not reconfigure")
	}
	if isOutdatedSurfaceErr(ErrTimeout) {
		t.Fatal("timeout must not reconfigure")
	}
}

func TestIsDeviceLostErr_MessageFallback(t *testing.T) {
	if !isDeviceLostErr(errors.New("Parent device is lost")) {
		t.Fatal("parent device message should match")
	}
	if !isDeviceLostErr(ErrDeviceLost) {
		t.Fatal("ErrDeviceLost should match")
	}
	if isDeviceLostErr(ErrTimeout) {
		t.Fatal("timeout is not device lost")
	}
}
