//go:build !nogpu

package gpu

import (
	"fmt"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// FlushAndFilterFromView encodes pending draws into srcView, then runs a GPU
// filter graph seeded from that view, submitting mesh+filter command buffers
// in a single Queue.Submit (opt18 / PKS glow multi-submit coalesce).
//
// Pure equivalence: same mesh seed + same filter graph; only submit batching
// changes. On filter failure after mesh is finished, mesh is submitted alone
// so recovery via FromView/readback remains correct.
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

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{
		Label: "filter_seed_mesh_enc",
	})
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

	cmdMesh, err := enc.Finish()
	if err != nil || cmdMesh == nil {
		return gpucontext.TextureView{}, nil, fmt.Errorf("flush+filter: finish mesh: %w", err)
	}

	// Filter graph + mesh seed in one Queue.Submit.
	view, release, err := runGPUFilterGraphFromViewWithLeading(
		device, queue, cache, srcView, w, h, nodes, []*webgpu.CommandBuffer{cmdMesh},
	)
	if err != nil {
		// Mesh CB was not released (encode failed before Submit). Submit mesh
		// alone so filterSrcRT is populated for recovery FromView/readback.
		if _, serr := queue.Submit(cmdMesh); serr != nil {
			cmdMesh.Release()
			return gpucontext.TextureView{}, nil, err
		}
		cmdMesh.Release()
		// Second submit: normal FromView (src now has mesh content).
		view2, rel2, err2 := runGPUFilterGraphFromView(device, queue, cache, srcView, w, h, nodes)
		if err2 != nil {
			return gpucontext.TextureView{}, nil, err2
		}
		return view2, rel2, nil
	}
	return view, release, nil
}
