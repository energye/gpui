package platform_test

import (
	"errors"
	"testing"

	"github.com/energye/gpui/ui/platform"
)

func TestDRMVBlank_APISurface(t *testing.T) {
	// Contract: Init/Wait/Has never panic; either true path or honest ErrNoDRMVBlank.
	_ = platform.HasDRMVBlank()
	err := platform.InitDRMVBlank()
	if err != nil && !errors.Is(err, platform.ErrNoDRMVBlank) {
		t.Logf("InitDRMVBlank: %v (non-sentinel; ok if driver-specific)", err)
	}
	werr := platform.WaitDRMVBlank()
	if werr == nil {
		_ = platform.WaitDRMVBlank()
		if !platform.HasDRMVBlank() {
			t.Fatal("Wait succeeded but HasDRMVBlank false")
		}
		return
	}
	if !errors.Is(werr, platform.ErrNoDRMVBlank) {
		t.Logf("WaitDRMVBlank: %v", werr)
	}
}
