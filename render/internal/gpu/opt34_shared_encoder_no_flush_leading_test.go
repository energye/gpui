//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

func TestOpt34_SharedEncoder_DoesNotFlushLeading(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared, s := r73Session(t)
	s.SetConvexRenderer(NewConvexRenderer(shared.device, shared.queue, 1))
	s.SetSDFPipeline(NewSDFRenderPipeline(shared.device, shared.queue, 1))
	s.SetStencilRenderer(NewStencilRenderer(shared.device, shared.queue, 1))

	const w, h uint32 = 64, 64
	mkView := func(label string) (hal.Texture, hal.TextureView) {
		tex, err := shared.device.CreateTexture(&hal.TextureDescriptor{
			Label: label, Size: hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm,
			Usage:  types.TextureUsageRenderAttachment | types.TextureUsageTextureBinding | types.TextureUsageCopySrc,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := shared.device.CreateTextureView(tex, &hal.TextureViewDescriptor{
			Format: types.TextureFormatBGRA8Unorm, Dimension: types.TextureViewDimension2D,
			Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return tex, view
	}
	mkEmptyCB := func(label string) hal.CommandBuffer {
		enc, err := shared.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: label})
		if err != nil {
			t.Fatal(err)
		}
		cmd, err := enc.Finish()
		if err != nil {
			t.Fatal(err)
		}
		return cmd
	}

	// Two deferred layer fills, already finished.
	s.EnqueueLeadingSubmit(mkEmptyCB("opt34_layer0"), nil)
	s.EnqueueLeadingSubmit(mkEmptyCB("opt34_layer1"), nil)
	if got := len(s.leadSubmitCBs); got != 2 {
		t.Fatalf("lead CBs=%d want 2 before composite", got)
	}
	submitsBefore := s.LastSubmitPathStats().Submits

	dstTex, dstView := mkView("opt34_composite_dst")
	t.Cleanup(func() {
		dstView.Destroy()
		dstTex.Destroy()
	})
	sharedEnc, err := shared.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "opt34_composite"})
	if err != nil {
		t.Fatal(err)
	}
	cmd := ConvexDrawCommand{
		Points: []render.Point{{X: 8, Y: 8}, {X: 40, Y: 8}, {X: 24, Y: 40}},
		Color:  [4]float32{0, 1, 0, 1},
	}
	target := render.GPURenderTarget{
		View:       packView(dstView),
		ViewWidth:  w,
		ViewHeight: h,
		Width:      int(w),
		Height:     int(h),
	}
	if err := s.RenderFrameGrouped(target, []ScissorGroup{{ConvexCommands: []ConvexDrawCommand{cmd}}}, nil, sharedEnc); err != nil {
		sharedEnc.DiscardEncoding()
		t.Fatalf("sharedEncoder RenderFrameGrouped: %v", err)
	}
	if got := len(s.leadSubmitCBs); got != 2 {
		t.Fatalf("after sharedEncoder encode lead CBs=%d want 2 (must not FlushLeading)", got)
	}
	if st := s.LastSubmitPathStats(); st.Submits != submitsBefore {
		t.Fatalf("Submits advanced during sharedEncoder encode: before=%d after=%d", submitsBefore, st.Submits)
	}

	// Caller Finishes composite then submitWithLeading → one Submit of layers+trailer.
	trailer, err := sharedEnc.Finish()
	if err != nil {
		t.Fatalf("Finish composite: %v", err)
	}
	if err := s.submitWithLeading(trailer); err != nil {
		t.Fatalf("submitWithLeading: %v", err)
	}
	if len(s.leadSubmitCBs) != 0 {
		t.Fatalf("lead CBs leftover=%d", len(s.leadSubmitCBs))
	}
	st := s.LastSubmitPathStats()
	if st.Submits != submitsBefore+1 {
		t.Fatalf("Submits=%d want %d (one coalesce submit)", st.Submits, submitsBefore+1)
	}
	if st.CoalescedCBs != 3 {
		t.Fatalf("CoalescedCBs=%d want 3 (2 layers + composite trailer)", st.CoalescedCBs)
	}
}

func TestOpt34_NoSharedEncoder_StillFlushesLeading(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared, s := r73Session(t)
	s.SetConvexRenderer(NewConvexRenderer(shared.device, shared.queue, 1))
	s.SetSDFPipeline(NewSDFRenderPipeline(shared.device, shared.queue, 1))
	s.SetStencilRenderer(NewStencilRenderer(shared.device, shared.queue, 1))

	const w, h uint32 = 64, 64
	mkView := func(label string) (hal.Texture, hal.TextureView) {
		tex, err := shared.device.CreateTexture(&hal.TextureDescriptor{
			Label: label, Size: hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm,
			Usage:  types.TextureUsageRenderAttachment | types.TextureUsageTextureBinding | types.TextureUsageCopySrc,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := shared.device.CreateTextureView(tex, &hal.TextureViewDescriptor{
			Format: types.TextureFormatBGRA8Unorm, Dimension: types.TextureViewDimension2D,
			Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return tex, view
	}
	mkEmptyCB := func(label string) hal.CommandBuffer {
		enc, err := shared.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: label})
		if err != nil {
			t.Fatal(err)
		}
		cmd, err := enc.Finish()
		if err != nil {
			t.Fatal(err)
		}
		return cmd
	}

	s.EnqueueLeadingSubmit(mkEmptyCB("opt34_ctl_layer0"), nil)
	s.EnqueueLeadingSubmit(mkEmptyCB("opt34_ctl_layer1"), nil)
	if got := len(s.leadSubmitCBs); got != 2 {
		t.Fatalf("lead CBs=%d want 2", got)
	}

	dstTex, dstView := mkView("opt34_present")
	t.Cleanup(func() {
		dstView.Destroy()
		dstTex.Destroy()
	})
	cmd := ConvexDrawCommand{
		Points: []render.Point{{X: 4, Y: 4}, {X: 20, Y: 4}, {X: 12, Y: 20}},
		Color:  [4]float32{1, 0, 0, 1},
	}
	target := render.GPURenderTarget{
		View:       packView(dstView),
		ViewWidth:  w,
		ViewHeight: h,
		Width:      int(w),
		Height:     int(h),
	}
	// Normal present path: sharedEncoder=nil drains leads before surface encode.
	if err := s.RenderFrameGrouped(target, []ScissorGroup{{ConvexCommands: []ConvexDrawCommand{cmd}}}, nil, nil); err != nil {
		t.Fatalf("present RenderFrameGrouped: %v", err)
	}
	if len(s.leadSubmitCBs) != 0 {
		t.Fatalf("lead CBs leftover=%d want 0 (FlushLeading on non-shared path)", len(s.leadSubmitCBs))
	}
	st := s.LastSubmitPathStats()
	// lastSubmitStats is reset after FlushLeading (RenderFrameGrouped init), so the
	// recorded surface Submit is alone: CoalescedCBs=1 proves leads were drained
	// mid-path rather than coalesced into the surface submit (which would be 3).
	if st.Submits < 1 {
		t.Fatalf("Submits=%d want >=1", st.Submits)
	}
	if st.CoalescedCBs != 1 {
		t.Fatalf("CoalescedCBs=%d want 1 (surface only after FlushLeading reset)", st.CoalescedCBs)
	}
}
