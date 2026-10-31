//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// layoutPinner is implemented by the renderers whose pipelines embed the
// shared clip/mask bind group layouts (SDFRenderPipeline, StencilRenderer,
// ConvexRenderer).
type layoutPinner interface {
	SetClipBindLayout(layout hal.BindGroupLayout)
	SetMaskBindLayout(layout hal.BindGroupLayout)
}

// testSharedLayouts creates the production shared clip/mask bind group
// layouts and pins them on the given renderers. The GPUShared frame path
// pins the same pair (ensureSharedLayoutsLocked + gpu_shared.go:869-880);
// tests building renderers standalone must do the same, or ensurePipeline
// fails with "no mask layout (shared layouts must be pinned by GPUShared
// first)" — P2.4e red-fix campaign.
func testSharedLayouts(t *testing.T, device hal.Device, pinners ...layoutPinner) (clip, mask hal.BindGroupLayout) {
	t.Helper()
	var err error
	if clip, err = createClipBindGroupLayout(device, "test_shared_clip_layout"); err != nil {
		t.Fatalf("test shared clip layout: %v", err)
	}
	if mask, err = createMaskBindGroupLayout(device, "test_shared_mask_layout"); err != nil {
		t.Fatalf("test shared mask layout: %v", err)
	}
	t.Cleanup(func() {
		clip.Destroy()
		mask.Destroy()
	})
	for _, p := range pinners {
		if p == nil {
			continue
		}
		p.SetClipBindLayout(clip)
		p.SetMaskBindLayout(mask)
	}
	return clip, mask
}
