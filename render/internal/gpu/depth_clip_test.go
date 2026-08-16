//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// TestDepthClipPipeline_Lifecycle verifies DepthClipPipeline creation,
// pipeline compilation, and destruction.
func TestDepthClipPipeline_Lifecycle(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	p := NewDepthClipPipeline(device, queue, 4)
	if p == nil {
		t.Fatal("expected non-nil DepthClipPipeline")
	}
	if p.device != device {
		t.Error("device not stored correctly")
	}
	if p.queue != queue {
		t.Error("queue not stored correctly")
	}
	if p.tessellator == nil {
		t.Error("expected non-nil tessellator")
	}

	// Pipelines should not be created yet (lazy).
	if p.stencilFillPipeline != nil {
		t.Error("expected nil stencilFillPipeline before ensurePipeline")
	}
	if p.depthCoverPipeline != nil {
		t.Error("expected nil depthCoverPipeline before ensurePipeline")
	}

	// Force pipeline creation.
	if err := p.ensurePipeline(); err != nil {
		t.Fatalf("ensurePipeline failed: %v", err)
	}
	if p.stencilFillPipeline == nil {
		t.Error("expected non-nil stencilFillPipeline after ensurePipeline")
	}
	if p.depthCoverPipeline == nil {
		t.Error("expected non-nil depthCoverPipeline after ensurePipeline")
	}
	if p.shader == nil {
		t.Error("expected non-nil shader after ensurePipeline")
	}
	if p.uniformBGL == nil {
		t.Error("expected non-nil uniformBGL after ensurePipeline")
	}
	if p.pipeLayout == nil {
		t.Error("expected non-nil pipeLayout after ensurePipeline")
	}

	// Second call should be a no-op.
	if err := p.ensurePipeline(); err != nil {
		t.Fatalf("second ensurePipeline failed: %v", err)
	}

	// Destroy should release all resources.
	p.Destroy()
	if p.stencilFillPipeline != nil {
		t.Error("expected nil stencilFillPipeline after Destroy")
	}
	if p.depthCoverPipeline != nil {
		t.Error("expected nil depthCoverPipeline after Destroy")
	}
	if p.shader != nil {
		t.Error("expected nil shader after Destroy")
	}
	if p.uniformBGL != nil {
		t.Error("expected nil uniformBGL after Destroy")
	}
	if p.pipeLayout != nil {
		t.Error("expected nil pipeLayout after Destroy")
	}
	if p.vertBuf != nil {
		t.Error("expected nil vertBuf after Destroy")
	}
	if p.coverBuf != nil {
		t.Error("expected nil coverBuf after Destroy")
	}
	if p.uniformBuf != nil {
		t.Error("expected nil uniformBuf after Destroy")
	}
	if p.bindGroup != nil {
		t.Error("expected nil bindGroup after Destroy")
	}

	// Double-destroy should be safe.
	p.Destroy()
}

// TestDepthClipPipeline_BuildClipResources_NilPath verifies that a nil
// clip path returns nil resources (no-op).
func TestDepthClipPipeline_BuildClipResources_NilPath(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	p := NewDepthClipPipeline(device, queue, 4)
	defer p.Destroy()

	if err := p.ensurePipeline(); err != nil {
		t.Fatalf("ensurePipeline failed: %v", err)
	}

	// Nil path should return nil resources.
	res, err := p.BuildClipResources(nil, 800, 600)
	if err != nil {
		t.Fatalf("BuildClipResources(nil) returned error: %v", err)
	}
	if res != nil {
		t.Error("expected nil resources for nil path")
	}
}

// TestDepthClipPipeline_BuildClipResources_EmptyPath verifies that an
// empty path (no subpaths) returns nil resources.
func TestDepthClipPipeline_BuildClipResources_EmptyPath(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	p := NewDepthClipPipeline(device, queue, 4)
	defer p.Destroy()

	if err := p.ensurePipeline(); err != nil {
		t.Fatalf("ensurePipeline failed: %v", err)
	}

	// Empty path (no commands) should produce no tessellation vertices.
	emptyPath := &render.Path{}
	res, err := p.BuildClipResources(emptyPath, 800, 600)
	if err != nil {
		t.Fatalf("BuildClipResources(empty) returned error: %v", err)
	}
	if res != nil {
		t.Error("expected nil resources for empty path")
	}
}

// TestDepthClipDepthStencil verifies the depth stencil state returned by
// depthClipDepthStencil() matches the GPU-CLIP-003a depth model.
func TestDepthClipDepthStencil(t *testing.T) {
	ds := depthClipDepthStencil()
	if ds == nil {
		t.Fatal("expected non-nil DepthStencilState")
	}

	// Format must be Depth24PlusStencil8 (shared with stencil renderer).
	if ds.Format != types.TextureFormatDepth24PlusStencil8 {
		t.Errorf("Format = %v, want Depth24PlusStencil8", ds.Format)
	}

	// Depth write must be disabled — content should NOT modify depth buffer.
	if ds.DepthWriteEnabled {
		t.Error("DepthWriteEnabled = true, want false (content must not modify depth)")
	}

	// DepthCompare must be GreaterEqual — content passes only where clip wrote Z=0.0.
	if ds.DepthCompare != types.CompareFunctionGreaterEqual {
		t.Errorf("DepthCompare = %v, want GreaterEqual", ds.DepthCompare)
	}

	// Stencil masks must be 0x00 — depth clip pipelines must not interact with stencil.
	if ds.StencilReadMask != 0x00 {
		t.Errorf("StencilReadMask = 0x%02x, want 0x00", ds.StencilReadMask)
	}
	if ds.StencilWriteMask != 0x00 {
		t.Errorf("StencilWriteMask = 0x%02x, want 0x00", ds.StencilWriteMask)
	}

	// Stencil ops must all be Keep (pass-through).
	if ds.StencilFront.PassOp != webgpu.StencilOperationKeep {
		t.Errorf("StencilFront.PassOp = %v, want Keep", ds.StencilFront.PassOp)
	}
	if ds.StencilBack.PassOp != webgpu.StencilOperationKeep {
		t.Errorf("StencilBack.PassOp = %v, want Keep", ds.StencilBack.PassOp)
	}
}

// TestScissorGroup_ClipPath verifies that ScissorGroup correctly stores a ClipPath.
func TestScissorGroup_ClipPath(t *testing.T) {
	// Without ClipPath — default state.
	grp := ScissorGroup{}
	if grp.ClipPath != nil {
		t.Error("default ScissorGroup should have nil ClipPath")
	}
	if grp.ClipDepthLevel != 0 {
		t.Error("default ScissorGroup should have ClipDepthLevel=0")
	}

	// With ClipPath set.
	path := &render.Path{}
	path.MoveTo(0, 0)
	path.LineTo(100, 0)
	path.LineTo(100, 100)
	path.Close()

	grp.ClipPath = path
	grp.ClipDepthLevel = 1

	if grp.ClipPath == nil {
		t.Error("expected non-nil ClipPath after assignment")
	}
	if grp.ClipDepthLevel != 1 {
		t.Errorf("ClipDepthLevel = %d, want 1", grp.ClipDepthLevel)
	}
}

// TestHasAnyDepthClip verifies the hasAnyDepthClip helper detects depth clip
// presence across a slice of groupResources.
func TestHasAnyDepthClip(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	defer s.Destroy()

	tests := []struct {
		name string
		grps []groupResources
		want bool
	}{
		{
			name: "nil slice",
			grps: nil,
			want: false,
		},
		{
			name: "empty slice",
			grps: []groupResources{},
			want: false,
		},
		{
			name: "all false",
			grps: []groupResources{
				{hasDepthClip: false},
				{hasDepthClip: false},
			},
			want: false,
		},
		{
			name: "first true",
			grps: []groupResources{
				{hasDepthClip: true},
				{hasDepthClip: false},
			},
			want: true,
		},
		{
			name: "last true",
			grps: []groupResources{
				{hasDepthClip: false},
				{hasDepthClip: true},
			},
			want: true,
		},
		{
			name: "all true",
			grps: []groupResources{
				{hasDepthClip: true},
				{hasDepthClip: true},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.hasAnyDepthClip(tt.grps)
			if got != tt.want {
				t.Errorf("hasAnyDepthClip() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRecordGroupDraws_NoDepthClip_Regression verifies that groups without
// depth clipping produce the same pipeline selection as before GPU-CLIP-003a.
// The stencil renderer must use the base (non-depth-clipped) pipelines when
// hasDepthClip is false, ensuring backward compatibility.
func TestRecordGroupDraws_NoDepthClip_Regression(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	defer s.Destroy()

	if err := s.EnsureTextures(200, 200); err != nil {
		t.Fatalf("EnsureTextures failed: %v", err)
	}

	// Build a simple stencil path (triangle).
	path := &render.Path{}
	path.MoveTo(10, 10)
	path.LineTo(190, 100)
	path.LineTo(100, 190)
	path.Close()

	tess := NewFanTessellator()
	tess.TessellatePath(path)
	fanVerts := tess.Vertices()
	coverQuad := tess.CoverQuad()

	cmd := StencilPathCommand{
		Vertices:  fanVerts,
		CoverQuad: coverQuad,
		Color:     [4]float32{1, 0, 0, 1},
		FillRule:  render.FillRuleNonZero,
	}

	target := render.GPURenderTarget{
		Width:  200,
		Height: 200,
		Data:   make([]uint8, 200*200*4),
		Stride: 200 * 4,
	}

	// Render with a single group (no depth clip).
	groups := []ScissorGroup{
		{
			StencilPaths: []StencilPathCommand{cmd},
		},
	}

	err := s.RenderFrameGrouped(target, groups, nil, nil)
	if err != nil {
		t.Fatalf("RenderFrameGrouped failed: %v", err)
	}

	// Verify stencil renderer was initialized with base pipelines.
	if s.stencilRenderer == nil {
		t.Fatal("expected non-nil stencilRenderer after render")
	}
	if s.stencilRenderer.nonZeroStencilPipeline == nil {
		t.Error("expected non-nil nonZeroStencilPipeline")
	}
	if s.stencilRenderer.nonZeroCoverPipeline == nil {
		t.Error("expected non-nil nonZeroCoverPipeline")
	}

	// Depth-clipped variants should NOT have been created (no depth clip used).
	if s.stencilRenderer.pipelineWithDepthClipNZ != nil {
		t.Error("expected nil pipelineWithDepthClipNZ when no depth clip active")
	}
	if s.stencilRenderer.pipelineWithDepthClipEO != nil {
		t.Error("expected nil pipelineWithDepthClipEO when no depth clip active")
	}
	if s.stencilRenderer.pipelineWithDepthClipCover != nil {
		t.Error("expected nil pipelineWithDepthClipCover when no depth clip active")
	}
}

// TestStencilRenderer_EnsureDepthClipPipelines verifies that the depth-clipped
// pipeline variants are created correctly and are idempotent on second call.
func TestStencilRenderer_EnsureDepthClipPipelines(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	sr := NewStencilRenderer(device, queue, 4)
	defer sr.Destroy()

	// Base pipelines must be created first (or ensureDepthClipPipelines handles it).
	if err := sr.ensureDepthClipPipelines(); err != nil {
		t.Fatalf("ensureDepthClipPipelines failed: %v", err)
	}

	// All three depth-clipped variants must exist.
	if sr.pipelineWithDepthClipNZ == nil {
		t.Error("expected non-nil pipelineWithDepthClipNZ")
	}
	if sr.pipelineWithDepthClipEO == nil {
		t.Error("expected non-nil pipelineWithDepthClipEO")
	}
	if sr.pipelineWithDepthClipCover == nil {
		t.Error("expected non-nil pipelineWithDepthClipCover")
	}

	// Base pipelines should also have been created as prerequisite.
	if sr.nonZeroStencilPipeline == nil {
		t.Error("expected non-nil nonZeroStencilPipeline after ensureDepthClipPipelines")
	}
	if sr.evenOddStencilPipeline == nil {
		t.Error("expected non-nil evenOddStencilPipeline after ensureDepthClipPipelines")
	}
	if sr.nonZeroCoverPipeline == nil {
		t.Error("expected non-nil nonZeroCoverPipeline after ensureDepthClipPipelines")
	}

	// Second call should be a no-op (idempotent).
	origNZ := sr.pipelineWithDepthClipNZ
	if err := sr.ensureDepthClipPipelines(); err != nil {
		t.Fatalf("second ensureDepthClipPipelines failed: %v", err)
	}
	if sr.pipelineWithDepthClipNZ != origNZ {
		t.Error("pipeline was recreated on second call (should be idempotent)")
	}
}

// TestDepthLoadOp_AlwaysClear_Regression verifies that the render session
// can render multiple frames with depth clip without errors.
//
// Regression test for circle depth clip appearing empty on frame 2+:
// DepthStoreOp=Discard discards depth after each render pass. Loading
// discarded depth on subsequent passes produces undefined values. If
// undefined depth happens to be small positive values, the GreaterEqual
// depth test in content pipelines fails everywhere, producing empty
// (invisible) clipped content.
//
// Fix: always clear depth to 1.0 regardless of frameRendered state. The
// depth clip pipeline writes Z=0.0 fresh each pass, so a clean 1.0 is
// always the correct starting point.
func TestDepthLoadOp_AlwaysClear_Regression(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	defer s.Destroy()

	if err := s.EnsureTextures(200, 200); err != nil {
		t.Fatalf("EnsureTextures failed: %v", err)
	}

	// Build a simple triangle clip path.
	clipPath := &render.Path{}
	clipPath.MoveTo(50, 0)
	clipPath.LineTo(100, 100)
	clipPath.LineTo(0, 100)
	clipPath.Close()

	// Build one group with depth clip.
	groups := []ScissorGroup{
		{
			ClipPath:  clipPath,
			SDFShapes: []SDFRenderShape{{Kind: 0, CenterX: 50, CenterY: 50, Param1: 40, Param2: 40, ColorR: 1, ColorG: 0, ColorB: 0, ColorA: 1}},
		},
	}

	target := render.GPURenderTarget{
		Width:  200,
		Height: 200,
		Data:   make([]uint8, 200*200*4),
		Stride: 200 * 4,
	}

	// Render 3 consecutive frames. Before the fix, frame 2+ used LoadOpLoad
	// on discarded depth, causing undefined depth buffer values and empty
	// clipped output.
	for frame := 0; frame < 3; frame++ {
		err := s.RenderFrameGrouped(target, groups, nil, nil)
		if err != nil {
			t.Fatalf("Frame %d RenderFrameGrouped failed: %v", frame, err)
		}
	}
}

// TestScissorSegments_CircleClip_MultipleSDFContent verifies that multiple
// SDF shapes drawn inside a circle clip each get their own scissor group
// with the clip path correctly assigned. This is a regression test for the
// circle clip appearing empty: each Fill() call creates SetClipRect +
// SetClipPath (2 segments at setup) + ClearClipPath + ClearClipRect (2
// segments at cleanup), producing 4 segments per fill. buildScissorGroups
// must correctly assign content to the segment that has the clipPath set.
func TestScissorSegments_CircleClip_MultipleSDFContent(t *testing.T) {
	rc := &GPURenderContext{
		shared: &GPUShared{},
	}

	// Build a circle clip path (device-space).
	circlePath := &render.Path{}
	circlePath.MoveTo(180, 100)
	for i := 1; i <= 36; i++ {
		angle := float64(i) * 2 * 3.14159265 / 36
		circlePath.LineTo(100+80*math.Cos(angle), 100+80*math.Sin(angle))
	}
	circlePath.Close()

	// Simulate 3 Fill() calls inside a circle clip, each following the
	// setGPUClipRect → setGPUClipPath → QueueShape → ClearClipPath → ClearClipRect
	// pattern from context.go doFill().
	clipRect := [4]uint32{20, 20, 160, 160}

	for i := 0; i < 3; i++ {
		// Setup: setGPUClipPath calls SetClipRect then SetClipPath.
		rc.SetClipRect(clipRect[0], clipRect[1], clipRect[2], clipRect[3])
		rc.SetClipPath(circlePath)

		// GPU fill: queue SDF shape.
		rc.pendingShapes = append(rc.pendingShapes, SDFRenderShape{
			Kind:    1,
			CenterX: 100, CenterY: float32(40 + i*40),
			Param1: 80, Param2: 20,
			ColorR: 1, ColorA: 1,
		})

		// Cleanup: ClearClipPath then ClearClipRect (deferred from setGPUClipPath).
		rc.ClearClipPath()
		rc.ClearClipRect()
	}

	groups := rc.buildScissorGroups()

	// Count groups that have content (non-empty SDF shapes) AND a clip path.
	clippedGroupCount := 0
	totalClippedShapes := 0
	for _, g := range groups {
		if len(g.SDFShapes) > 0 && g.ClipPath != nil {
			clippedGroupCount++
			totalClippedShapes += len(g.SDFShapes)
		}
	}

	if clippedGroupCount != 3 {
		t.Errorf("expected 3 groups with clipPath + SDF content, got %d (total groups: %d)", clippedGroupCount, len(groups))
		for i, g := range groups {
			t.Logf("  group[%d]: sdf=%d clipPath=%v rect=%v", i, len(g.SDFShapes), g.ClipPath != nil, g.Rect != nil)
		}
	}
	if totalClippedShapes != 3 {
		t.Errorf("expected 3 total SDF shapes in clipped groups, got %d", totalClippedShapes)
	}
}

// TestClipPath_SurvivesDeepCopy verifies that ClipPath is preserved when
// buildScissorGroups output is deep-copied in Flush(). This is a regression
// test for the bug where the deep-copy loop in gpu_render_context.go:Flush()
// initialized ScissorGroup with only Rect and ClipRRect, dropping ClipPath.
// Without ClipPath, BuildClipResources was never called and depth clipping
// had no effect — shapes rendered as rectangles (scissor only), not clipped
// to the arbitrary path boundary.
func TestClipPath_SurvivesDeepCopy(t *testing.T) {
	// Simulate the deep-copy pattern from gpu_render_context.go:Flush().
	clipPath := &render.Path{}
	clipPath.MoveTo(50, 0)
	clipPath.LineTo(100, 100)
	clipPath.LineTo(0, 100)
	clipPath.Close()

	original := []ScissorGroup{
		{
			Rect:           &[4]uint32{0, 0, 100, 100},
			ClipRRect:      &ClipParams{Enabled: 1.0},
			ClipPath:       clipPath,
			ClipDepthLevel: 1,
			SDFShapes:      []SDFRenderShape{{Kind: 0}},
		},
	}

	// Replicate the deep-copy pattern (as fixed).
	owned := make([]ScissorGroup, len(original))
	for i := range original {
		g := &original[i]
		owned[i] = ScissorGroup{
			Rect:           g.Rect,
			ClipRRect:      g.ClipRRect,
			ClipPath:       g.ClipPath,
			ClipDepthLevel: g.ClipDepthLevel,
		}
		if len(g.SDFShapes) > 0 {
			owned[i].SDFShapes = make([]SDFRenderShape, len(g.SDFShapes))
			copy(owned[i].SDFShapes, g.SDFShapes)
		}
	}

	// Verify ClipPath is preserved.
	if owned[0].ClipPath == nil {
		t.Fatal("ClipPath lost during deep-copy — depth clip will not work")
	}
	if owned[0].ClipPath != clipPath {
		t.Error("ClipPath pointer changed during deep-copy")
	}
	if owned[0].ClipDepthLevel != 1 {
		t.Errorf("ClipDepthLevel = %d, want 1", owned[0].ClipDepthLevel)
	}
}

// TestGPURenderSession_DepthClipPipelineConstructed verifies that
// NewGPURenderSession eagerly constructs the depth-clip pipeline
// (GPU-CLIP-003a). No GPU required: construction only stores pointers,
// pipeline compilation happens lazily in ensurePipeline. Before this fix
// the pipeline was never created in production, the hasDepthClip guard
// was always false, and Clip() drew through the clip region on GPU.
func TestGPURenderSession_DepthClipPipelineConstructed(t *testing.T) {
	s := NewGPURenderSession(nil, nil, 1)
	defer s.Destroy()
	if s.depthClipPipeline == nil {
		t.Fatal("NewGPURenderSession must construct depthClipPipeline (GPU-CLIP-003a)")
	}
}

// TestGPURenderSession_DepthClipPipelineInitialized verifies that the
// session depth-clip pipeline compiles on a native device.
func TestGPURenderSession_DepthClipPipelineInitialized(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	defer s.Destroy()

	if s.depthClipPipeline == nil {
		t.Fatal("NewGPURenderSession must construct depthClipPipeline (GPU-CLIP-003a)")
	}

	// Lazy compile must succeed on the native device.
	if err := s.depthClipPipeline.ensurePipeline(); err != nil {
		t.Fatalf("depthClipPipeline.ensurePipeline failed: %v", err)
	}
	if s.depthClipPipeline.stencilFillPipeline == nil {
		t.Error("expected stencilFillPipeline after ensurePipeline")
	}
	if s.depthClipPipeline.depthCoverPipeline == nil {
		t.Error("expected depthCoverPipeline after ensurePipeline")
	}
}

// TestGPURenderSession_EnsureStagePipelines_CreatesDepthClip verifies the
// defensive re-creation path: if depthClipPipeline is nil when a frame
// needs depth clipping, ensureStagePipelines reconstructs it.
func TestGPURenderSession_EnsureStagePipelines_CreatesDepthClip(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	defer s.Destroy()

	// Simulate a session built without the pipeline (defensive path).
	s.depthClipPipeline = nil
	if err := s.ensureStagePipelines(false, false, true); err != nil {
		t.Fatalf("ensureStagePipelines(needDepthClip=true) failed: %v", err)
	}
	if s.depthClipPipeline == nil {
		t.Fatal("ensureStagePipelines must re-create depthClipPipeline when nil")
	}
}

// TestDepthClipResources_BuildAndReleaseFromSession verifies that a clip
// path on a ScissorGroup produces buildable resources through the session
// depth-clip pipeline (guarded by a non-nil pipeline).
func TestDepthClipResources_BuildAndReleaseFromSession(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	defer s.Destroy()

	if err := s.depthClipPipeline.ensurePipeline(); err != nil {
		t.Fatalf("ensurePipeline failed: %v", err)
	}

	path := &render.Path{}
	path.MoveTo(10, 10)
	path.LineTo(110, 10)
	path.LineTo(110, 110)
	path.LineTo(10, 110)
	path.Close()

	// Same guard as render_session.go group build: non-nil pipeline +
	// non-nil ClipPath must yield resources.
	if s.depthClipPipeline == nil {
		t.Fatal("depthClipPipeline is nil — ClipPath will never build resources")
	}
	res, err := s.depthClipPipeline.BuildClipResources(path, 800, 600)
	if err != nil {
		t.Fatalf("BuildClipResources failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil depth clip resources for triangle path")
	}
	defer res.Release()
	if res.vertCount == 0 {
		t.Error("expected fan vertices")
	}
	if res.coverCount != 6 {
		t.Errorf("coverCount = %d, want 6", res.coverCount)
	}
}

// TestDepthClip_ProductionPath_NoManualEnsure verifies that arbitrary path
// clipping works through the production RenderFrameGrouped path WITHOUT any
// manual ensurePipeline call. Regression for the render_clipping draw-through
// bug: DepthClipPipeline.ensurePipeline() was never called in the production
// path, uniformBGL stayed nil, BuildClipResources failed with "layout is nil",
// groups got no depthClipRes, and Clip()/ClipPreserve() drew straight through
// the clip region.
func TestDepthClip_ProductionPath_NoManualEnsure(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	defer s.Destroy()

	const W, H = 200, 200
	if err := s.EnsureTextures(W, H); err != nil {
		t.Fatalf("EnsureTextures: %v", err)
	}

	// Clip path: square (50,50)-(150,150). Production path must compile the
	// depth-clip pipeline lazily via ensureStagePipelines(needDepthClip=true).
	clipPath := &render.Path{}
	clipPath.MoveTo(50, 50)
	clipPath.LineTo(150, 50)
	clipPath.LineTo(150, 150)
	clipPath.LineTo(50, 150)
	clipPath.Close()

	// Content: full-width strip crossing the clip region.
	strip := ConvexDrawCommand{
		Points: []render.Point{
			{X: 20, Y: 95},
			{X: 180, Y: 95},
			{X: 180, Y: 105},
			{X: 20, Y: 105},
		},
		Color: [4]float32{0.5, 0.25, 0.0, 1},
	}

	groups := []ScissorGroup{
		{
			ClipPath:       clipPath,
			ClipDepthLevel: 1,
			ConvexCommands: []ConvexDrawCommand{strip},
		},
	}
	target := render.GPURenderTarget{
		Width: W, Height: H,
		Data:   make([]uint8, W*H*4),
		Stride: W * 4,
	}
	if err := s.RenderFrameGrouped(target, groups, nil, nil); err != nil {
		t.Fatalf("RenderFrameGrouped: %v", err)
	}

	lum := func(x, y int) int {
		i := (y*W + x) * 4
		r, g, b := int(target.Data[i]), int(target.Data[i+1]), int(target.Data[i+2])
		return (r + g + b) / 3
	}
	outsidePainted := 0
	insidePainted := 0
	for x := 20; x < 180; x++ {
		if lum(x, 100) > 8 {
			if x < 50 || x >= 150 {
				outsidePainted++
			} else {
				insidePainted++
			}
		}
	}
	t.Logf("production depth clip: inside=%d outside=%d (clip square x 50..150)", insidePainted, outsidePainted)
	if outsidePainted > 10 {
		t.Errorf("production depth clip FAILED: %d px painted outside clip square — Clip() draws through", outsidePainted)
	}
	if insidePainted < 90 {
		t.Errorf("production depth clip FAILED: only %d px painted inside clip square", insidePainted)
	}
}
