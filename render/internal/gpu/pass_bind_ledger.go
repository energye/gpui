package gpu

import (
	"github.com/energye/gpui/gpu/webgpu"
)

// PassBindLedger records the exact bound set after each Draw within ONE
// render pass. A later Draw with a bit-identical set skips the Set* calls
// and only issues Draw — identical pixels, fewer native + validation
// crossings.
//
// Correctness rules:
//   - Pass generations: BeginPassLedger runs per pass, so a recycled
//     *RenderPassEncoder address never aliases a stale entry.
//   - One ledger shared across tiers: any tier's Draw updates it, so a
//     stencil/image/text draw between two SDF draws invalidates.
//   - Any difference (nil-vs-non-nil clip/mask, new buffers after realloc,
//     different pipeline) takes the full bind path.
//   - Destroy/detach clears: stale pipeline pointers never compare equal.
type PassBindLedger struct {
	seq   uint64
	valid bool
	rp    *webgpu.RenderPassEncoder
	pipe  *webgpu.RenderPipeline
	bg0   *webgpu.BindGroup
	clip  *webgpu.BindGroup
	mask  *webgpu.BindGroup
	vert  *webgpu.Buffer
}

// BeginPassLedger starts a new pass generation: the first Draw of the pass
// always takes the full bind path.
func (l *PassBindLedger) BeginPassLedger() {
	if l == nil {
		return
	}
	l.seq++
	l.valid = false
	l.rp = nil
	l.pipe = nil
	l.bg0 = nil
	l.clip = nil
	l.mask = nil
	l.vert = nil
}

// skipBind reports whether the exact set is already bound on this pass.
// The pass encoder must match: offscreen record passes share the session
// ledger object, and a different rp means bindings were never set there.
func (l *PassBindLedger) skipBind(rp *webgpu.RenderPassEncoder, pipe *webgpu.RenderPipeline, bg0, clip, mask *webgpu.BindGroup, vert *webgpu.Buffer) bool {
	if l == nil || rp == nil || pipe == nil || bg0 == nil || vert == nil {
		return false
	}
	if !l.valid || l.rp != rp {
		return false
	}
	return l.pipe == pipe && l.bg0 == bg0 && l.clip == clip && l.mask == mask && l.vert == vert
}

// Invalidate marks the ledger empty: the next Draw takes the full bind
// path. Any tier that draws without going through the ledger (convex,
// stencil, image, text, glyph, depth-clip, base layer) must call it, or a
// later ledger Draw would skip binds while the hardware still holds the
// other tier's pipeline.
func (l *PassBindLedger) Invalidate() {
	if l == nil {
		return
	}
	l.valid = false
}

// noteBind records the bound set after a full-bind Draw.
func (l *PassBindLedger) noteBind(rp *webgpu.RenderPassEncoder, pipe *webgpu.RenderPipeline, bg0, clip, mask *webgpu.BindGroup, vert *webgpu.Buffer) {
	if l == nil || rp == nil {
		return
	}
	l.valid = true
	l.rp = rp
	l.pipe = pipe
	l.bg0 = bg0
	l.clip = clip
	l.mask = mask
	l.vert = vert
}
