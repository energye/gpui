//go:build !nogpu

package gpu

import (
	"fmt"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
)

// FlushAndFilterFromView encodes pending draws into srcView, then continues the
// GPU filter graph on the same command encoder (opt36), finishing once and
// submitting mesh+filter together.
//
// Pure equivalence: same mesh seed + same filter graph; fewer CommandEncoder.Finish
// calls than opt18 (which used one Finish for mesh seed CB + one for filter CB in
// a single Queue.Submit). On filter failure after mesh is encoded, the open
// encoder is Finished and submitted so srcView is populated, then FromView
// recovery re-runs the filter graph.
//
// Returns error when the combined path cannot run; caller should fall back to
// FlushGPUWithView + FromView.
func (rc *GPURenderContext) FlushAndFilterFromView(
	srcView gpucontext.TextureView, w, h int, nodes []render.ImageFilterNode,
) (gpucontext.TextureView, func(), error) {
	if rc == nil || rc.shared == nil || srcView.IsNil() || w <= 0 || h <= 0 || len(nodes) == 0 {
		return gpucontext.TextureView{}, nil, fmt.Errorf("flush+filter: invalid args")
	}
	if rc.PendingCount() == 0 {
		return gpucontext.TextureView{}, nil, fmt.Errorf("flush+filter: no pending draws")
	}

	// Ensure device + session (same as Flush) without submitting yet.
	rc.shared.mu.Lock()
	if rc.shared.device == nil {
		if err := rc.shared.ensureGPU(); err != nil {
			rc.shared.mu.Unlock()
			return gpucontext.TextureView{}, nil, err
		}
	} else {
		_ = rc.shared.ensureGPU()
	}
	rc.shared.ensurePipelines()
	device := rc.shared.device
	queue := rc.shared.queue
	cache := &rc.shared.filterGPU
	sdfPipeline := rc.shared.sdfRenderPipeline
	convexRend := rc.shared.convexRenderer
	stencilRend := rc.shared.stencilRenderer
	rc.shared.mu.Unlock()

	if device == nil || queue == nil {
		return gpucontext.TextureView{}, nil, fmt.Errorf("flush+filter: no device")
	}

	if rc.session == nil {
		sc := rc.shared.SampleCount()
		if rc.preferSampleCount1 {
			sc = 1
		}
		rc.session = NewGPURenderSession(device, queue, sc)
		if rc.preferSampleCount1 {
			rc.session.MarkOwnsShapePipelines()
		} else {
			rc.session.SetSDFPipeline(sdfPipeline)
			rc.session.SetConvexRenderer(convexRend)
			rc.session.SetStencilRenderer(stencilRend)
		}
	}

	enc, err := device.CreateCommandEncoder(filterSeedMeshEncoderDesc)
	if err != nil || enc == nil {
		return gpucontext.TextureView{}, nil, fmt.Errorf("flush+filter: create encoder: %w", err)
	}

	// Encode pending draws into srcView without submitting (ADR-017 shared encoder).
	rc.sharedEncoder = enc
	target := render.GPURenderTarget{
		Width:      w,
		Height:     h,
		View:       srcView,
		ViewWidth:  uint32(w), //nolint:gosec
		ViewHeight: uint32(h), //nolint:gosec
	}
	ferr := rc.Flush(target)
	rc.sharedEncoder = nil
	if ferr != nil {
		enc.DiscardEncoding()
		return gpucontext.TextureView{}, nil, ferr
	}

	// opt36: continue filter graph on the same encoder → one Finish for seed+filter.
	view, release, err := runGPUFilterGraphFromViewIntoEncoder(
		device, queue, cache, srcView, w, h, nodes, enc,
	)
	if err != nil {
		// Mesh seed is only applied if the CB is submitted. Finish the open encoder
		// (mesh ± partial filter) and submit, then re-run FromView for a clean graph.
		if cmd, ferr := enc.Finish(); ferr == nil && cmd != nil {
			if _, serr := queue.Submit(cmd); serr != nil {
				cmd.Release()
				return gpucontext.TextureView{}, nil, err
			}
			cmd.Release()
			view2, rel2, err2 := runGPUFilterGraphFromView(device, queue, cache, srcView, w, h, nodes)
			if err2 != nil {
				return gpucontext.TextureView{}, nil, err2
			}
			return view2, rel2, nil
		}
		// Finish failed (encoder already closed/discarded) — last resort.
		enc.DiscardEncoding()
		return gpucontext.TextureView{}, nil, err
	}
	return view, release, nil
}
