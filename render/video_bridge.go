//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"github.com/energye/gpui/gpu/hal"
)

// Plane staging states for UploadPlanesFrameAsync: idle → copying → ready →
// (published) idle. A ready frame waits for the draw only on backends whose
// context belongs to the raster thread.
const (
	planeStageIdle = iota
	planeStageCopying
	planeStageReady
)

// DrawVideoFrame draws one frame through the generic image path after the
// caller uploaded it with Upload. It is the guaranteed-picture fallback:
// use Context.DrawVideoSlot first for the zero-upload quad, and call this
// only when DrawVideoSlot reports false. No-new-frame means the caller
// skips both Upload and drawing, so upload count stays equal to shown
// new frames.
func (c *Context) DrawVideoFrame(img *ImageBuf, opts VideoDrawOptions) bool {
	if c == nil || img == nil || img.Disposed() {
		return false
	}
	if opts.Opacity == 0 {
		opts.Opacity = 1.0
	}
	if opts.Interpolation == 0 {
		opts.Interpolation = InterpBilinear
	}
	c.DrawImageEx(img, DrawImageOptions{
		X:             opts.X,
		Y:             opts.Y,
		DstWidth:      opts.DstWidth,
		DstHeight:     opts.DstHeight,
		Interpolation: opts.Interpolation,
		Opacity:       opts.Opacity,
		BlendMode:     BlendNormal,
	})
	return true
}

// packedView boxes the slot texture view into the opaque GPU handle for
// Context.DrawVideoSlot. Nil slot/view yields the zero handle (fail closed).
// NewVideoBridge creates one route on the borrowed device.
// Nil device yields a fallback-only bridge: Show always takes the generic
// path and counts fallbacks, never crashes.
// The bridge registers as a TextureSource so a retained window can name it by
// id (scene.TextureLayer); Close unregisters again.
func NewVideoBridge(device hal.Device) *VideoBridge {
	b := &VideoBridge{}
	if device != nil {
		b.pool = NewVideoTexturePool(device)
		b.ppool = NewVideoPlanePool(device)
		b.crossThreadWrites = videoCrossThreadWrites(device)
	}
	b.id = RegisterTextureSource(b)
	return b
}

// videoCrossThreadWrites asks the backend whether a queue write may run on the
// copy worker. Backends that bind their context to the raster thread (GLES)
// report false and take the upload when the texture layer paints.
func videoCrossThreadWrites(device hal.Device) bool {
	if device == nil {
		return false
	}
	q := device.Queue()
	if q == nil {
		return false
	}
	w, ok := q.(hal.CrossThreadTextureWriter)
	return ok && w.SupportsCrossThreadTextureWrites()
}

// EnsureDevice attaches the pool device when the window device opens after
// the bridge was created fallback-only. No-op when already attached.
// Both the RGBA pool and the NV12 plane pool share the device.
// EnsureDevice attaches the pool device when the window device opens after
// the bridge was created fallback-only. No-op when already attached.
// Both the RGBA pool and the NV12 plane pool share the device.
func (b *VideoBridge) EnsureDevice(device hal.Device) {
	if b == nil || device == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pool == nil {
		b.pool = NewVideoTexturePool(device)
	}
	if b.ppool == nil {
		b.ppool = NewVideoPlanePool(device)
		b.crossThreadWrites = videoCrossThreadWrites(device)
	}
}

// Show uploads one new frame and draws it. It returns direct=true when the
// zero-upload quad was queued, false when the generic fallback was used.
// Either way the picture is drawn when ok=true; ok=false means nothing was
// drawn (bad args or fallback buffer failure).
// Show uploads one new frame and draws it. It returns direct=true when the
// zero-upload quad was queued, false when the generic fallback was used.
// Either way the picture is drawn when ok=true; ok=false means nothing was
// drawn (bad args or fallback buffer failure).
func (b *VideoBridge) Show(c *Context, w, h int, pix []byte, opts VideoDrawOptions) (direct, ok bool) {
	return b.showImpl(0, false, c, w, h, pix, opts)
}

// ShowSeq uploads frame seq once then draws it; repeat calls with the same
// seq redraw the uploaded frame into another rect without re-uploading.
// Either way the picture is drawn when ok=true.
// ShowSeq uploads frame seq once then draws it; repeat calls with the same
// seq redraw the uploaded frame into another rect without re-uploading.
// Either way the picture is drawn when ok=true.
func (b *VideoBridge) ShowSeq(seq uint64, c *Context, w, h int, pix []byte, opts VideoDrawOptions) (direct, ok bool) {
	return b.showImpl(seq, true, c, w, h, pix, opts)
}

func (b *VideoBridge) showImpl(seq uint64, useSeq bool, c *Context, w, h int, pix []byte, opts VideoDrawOptions) (direct, ok bool) {
	if b == nil || c == nil {
		return false, false
	}
	need := int64(w) * int64(h) * 4
	if w < 1 || h < 1 || need <= 0 || int64(len(pix)) < need {
		return false, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.planeStageBusyLocked() {
		return false, false
	}
	if useSeq && b.hasSeq && seq == b.lastSeq && w == b.lastW && h == b.lastH {
		// Same frame, second view: no re-upload, just redraw.
		b.redraws++
		if b.slot != nil && c.DrawVideoSlot(b.slot, opts) {
			return true, true
		}
		if b.hasFbSeq && b.fbSeq == seq && b.fallback != nil && !b.fallback.Disposed() {
			return false, c.DrawVideoFrame(b.fallback, opts)
		}
		return false, false
	}
	b.frames++
	if useSeq {
		b.hasSeq, b.lastSeq, b.lastW, b.lastH = true, seq, w, h
	}
	if b.pool != nil {
		slot, _, err := b.pool.AcquireForFrame(b.slot, w, h)
		if err == nil {
			b.slot = slot
			if uerr := b.pool.Upload(slot, pix[:need]); uerr == nil {
				b.uploads++
				if c.DrawVideoSlot(slot, opts) {
					return true, true
				}
			}
		}
	}
	if !b.ensureFallbackLocked(w, h) {
		return false, false
	}
	dst := b.fallback.Data()
	if int64(len(dst)) < need {
		return false, false
	}
	copy(dst[:need], pix[:need])
	b.fallback.MarkPixelsDirty()
	b.fallbacks++
	if useSeq {
		b.hasFbSeq, b.fbSeq = true, seq
	}
	c.RecordVideoFallback("direct-fallback")
	return false, c.DrawVideoFrame(b.fallback, opts)
}

// UploadFrame stages one new frame without drawing.
// Call it on the tick with the Poll pix (valid until the next Poll):
// the copy is synchronous, so the pix may be recycled right after it
// returns. Draw with DrawCurrent in Paint. No-new-frame means the caller
// skips UploadFrame, so frames stay equal to shown frames.
//
// A rejected GPU upload (no device yet, row-pitch alignment, allocation
// failure) must not drop the frame: the pixels are kept in the CPU shadow
// and DrawCurrent takes the counted fallback. Returns false only for bad
// arguments; true means "staged, draw me", with Stats().Uploads telling
// whether the fast path was available.
func (b *VideoBridge) UploadFrame(w, h int, pix []byte) bool {
	if b == nil {
		return false
	}
	need := int64(w) * int64(h) * 4
	if w < 1 || h < 1 || need <= 0 || int64(len(pix)) < need {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.planeStageBusyLocked() {
		return false
	}
	b.lastW, b.lastH = w, h
	// One current shape at a time: an RGBA frame replaces a plane frame, so
	// drawCurrentAt cannot keep painting the old NV12 slot.
	if b.hasPlanes {
		if b.ppool != nil && b.pslot != nil {
			b.ppool.Release(b.pslot)
		}
		if b.ppool != nil && b.pback != nil {
			b.ppool.Release(b.pback)
		}
		b.pslot, b.pback = nil, nil
		b.hasPlanes = false
	}
	if len(b.shadow) != int(need) {
		b.shadow = make([]byte, need)
	}
	copy(b.shadow, pix[:need])
	b.frames++
	b.hasFrame = true
	if b.pool == nil {
		b.slot = nil
		return true
	}
	slot, _, err := b.pool.AcquireForFrame(b.slot, w, h)
	if err != nil {
		b.slot = nil
		return true
	}
	if uerr := b.pool.Upload(slot, pix[:need]); uerr != nil {
		// The acquired slot was not uploaded; do not let Paint queue stale
		// contents. Release it and let the shadow path carry this frame.
		b.pool.Release(slot)
		b.slot = nil
		return true
	}
	b.slot = slot
	b.uploads++
	return true
}

// DrawCurrent draws the last uploaded frame. Call it in Paint with the
// display rect: zero-upload quad on the fast path, generic fallback
// (block copy, counted) otherwise. NV12 shape converts on the GPU via
// DrawVideoPlanes; when the GPU entry is unavailable it converts into
// the owned fallback buffer on CPU. ok=false means nothing was drawn.
func (b *VideoBridge) DrawCurrent(c *Context, opts VideoDrawOptions) (direct, ok bool) {
	return b.drawCurrentAt(c, opts, 0)
}

// drawCurrentAt is DrawCurrent with the view index that paint accounting is
// filed under (0 = the only view). Every caller of the bridge's draw lands
// here, so the "did it reach the screen" evidence does not depend on which
// present path ran.
func (b *VideoBridge) drawCurrentAt(c *Context, opts VideoDrawOptions, view int) (direct, ok bool) {
	if b == nil || c == nil {
		return false, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// Runs before the unlock above (defer LIFO), so the accounting needs no
	// second lock and no separate atomic set.
	defer func() { b.notePaintLocked(view, direct, ok) }()
	// Publish a staged plane frame here when the backend cannot write from the
	// copy worker (GLES: the context belongs to this thread). On a backend
	// that supports cross-thread writes the worker already published, so this
	// is a no-op. Either way the draw only ever sees a complete frame.
	b.publishPendingPlaneLocked()
	if b.hasPlanes {
		if b.pslot != nil && c.DrawVideoPlanes(b.pslot, opts) {
			return true, true
		}
		w, h := b.lastW, b.lastH
		if len(b.pshadowY) == 0 || !b.ensureFallbackLocked(w, h) {
			return false, false
		}
		dst := b.fallback.Data()
		need := int64(w) * int64(h) * 4
		if int64(len(dst)) < need {
			return false, false
		}
		nv12ToRGBA(dst[:need], b.pshadowY, b.pshadowU, w, h)
		b.fallback.MarkPixelsDirty()
		b.fallbacks++
		c.RecordVideoFallback("direct-fallback-planes")
		return false, c.DrawVideoFrame(b.fallback, opts)
	}
	if !b.hasFrame {
		return false, false
	}
	// A slot is optional: no device yet or a rejected row pitch leaves only
	// the CPU shadow, which must still reach the screen through the counted
	// fallback (V-2). A live slot that fails DrawVideoSlot (device mismatch,
	// no GPU session) also falls through to the same shadow.
	if b.slot != nil && c.DrawVideoSlot(b.slot, opts) {
		return true, true
	}
	w, h := b.lastW, b.lastH
	pix := b.shadow
	if len(pix) == 0 {
		return false, false
	}
	if !b.ensureFallbackLocked(w, h) {
		return false, false
	}
	dst := b.fallback.Data()
	need := int64(w) * int64(h) * 4
	if int64(len(dst)) < need || int64(len(pix)) < need {
		return false, false
	}
	copy(dst[:need], pix[:need])
	b.fallback.MarkPixelsDirty()
	b.fallbacks++
	c.RecordVideoFallback("direct-fallback")
	return false, c.DrawVideoFrame(b.fallback, opts)
}

// PaintTexture draws the current frame for one scene.TextureLayer. It is the
// TextureSource entry point: the retained tree names this bridge by ID and the
// raster thread calls here, so the draw runs under this bridge's own lock while
// the producer keeps uploading from the UI thread. ok=false means nothing was
// drawn (no frame yet, or the route was closed) and the caller must not treat
// the rect as covered.
func (b *VideoBridge) PaintTexture(dc *Context, opts TexturePaintOptions) (direct, ok bool) {
	if b == nil {
		return false, false
	}
	return b.drawCurrentAt(dc, VideoDrawOptions{
		X: opts.X, Y: opts.Y, DstWidth: opts.Width, DstHeight: opts.Height,
		Opacity: opts.Opacity, Interpolation: opts.Interp,
	}, opts.DrawIndex)
}

// ID returns this route's texture registry id (0 only for a zero-valued
// bridge, which was never registered). Immutable after construction, so a
// layer builder on the UI thread can read it without the route lock.
func (b *VideoBridge) ID() uint64 {
	if b == nil {
		return 0
	}
	return b.id
}

// notePaintLocked files one paint outcome. Caller holds b.mu.
func (b *VideoBridge) notePaintLocked(view int, direct, ok bool) {
	if view < 0 {
		view = 0
	}
	if view >= videoMaxViews {
		view = videoMaxViews - 1
	}
	b.paints++
	b.viewPaints[view]++
	b.lastPaintOK, b.lastPaintDirect = ok, direct
	b.viewLastOK[view] = ok
	if !ok {
		b.failedPaints++
		return
	}
	b.viewOK[view] = true
	b.viewDirect[view] = direct
	if direct {
		b.directPaints++
	} else {
		b.fallbackPaints++
	}
}

// PaintStats snapshots the draw outcomes. The retained path paints on the
// raster thread, so a render object cannot remember what reached the screen —
// probes read it from the source that was actually drawn.
type PaintStats struct {
	Paints          uint64
	DirectPaints    uint64
	FallbackPaints  uint64
	FailedPaints    uint64
	LastPaintOK     bool
	LastPaintDirect bool
	ViewPaints      []uint64
	// ViewOK[i] reports that view i of this route drew successfully at least
	// once (a picture-in-picture window proves the second view reused one
	// upload); ViewLastOK[i] reports its most recent paint outcome — a closed
	// producer turns this false without erasing the ever-drew proof.
	// ViewDirect[i] reports which path its most recent draw took (true =
	// zero-upload GPU quad, false = counted fallback). Kept per view so a PiP
	// window cannot report the small view's path as the main view's.
	ViewOK     []bool
	ViewLastOK []bool
	ViewDirect []bool
}

// PaintStats snapshots this route's paint accounting.
func (b *VideoBridge) PaintStats() PaintStats {
	if b == nil {
		return PaintStats{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	vp := make([]uint64, videoMaxViews)
	vo := make([]bool, videoMaxViews)
	vl := make([]bool, videoMaxViews)
	vd := make([]bool, videoMaxViews)
	copy(vp, b.viewPaints[:])
	copy(vo, b.viewOK[:])
	copy(vl, b.viewLastOK[:])
	copy(vd, b.viewDirect[:])
	return PaintStats{
		Paints: b.paints, DirectPaints: b.directPaints,
		FallbackPaints: b.fallbackPaints, FailedPaints: b.failedPaints,
		LastPaintOK: b.lastPaintOK, LastPaintDirect: b.lastPaintDirect,
		ViewPaints: vp, ViewOK: vo, ViewLastOK: vl, ViewDirect: vd,
	}
}

// VideoBridge is a texture source: a retained window gives the layer tree an
// id, never a pointer to this bridge.
var _ TextureSource = (*VideoBridge)(nil)

// Stats snapshots the bridge.
func (b *VideoBridge) Stats() BridgeStats {
	if b == nil {
		return BridgeStats{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	st := BridgeStats{Frames: b.frames, Uploads: b.uploads, Fallbacks: b.fallbacks, Redraws: b.redraws}
	if b.pool != nil {
		ps := b.pool.Stats()
		st.Live, st.Idle, st.Evictions = ps.Live, ps.Idle, ps.Evictions
	}
	if b.ppool != nil {
		ps := b.ppool.Stats()
		st.Live += ps.Live
		st.Idle += ps.Idle
		st.Evictions += ps.Evictions
	}
	return st
}

// Close releases the slot and fallback buffer and drops the texture-source
// registration. Borrowed device untouched. After Close, a retained layer that
// still names this id resolves to no source and paints nothing (fail closed).
func (b *VideoBridge) Close() {
	if b == nil {
		return
	}
	UnregisterTextureSource(b.ID())
	b.mu.Lock()
	b.closed = true
	var staging chan struct{}
	if b.pendState == planeStageCopying {
		staging = b.pendDone
	}
	b.mu.Unlock()
	// The worker only takes b.mu briefly; wait so teardown never races a frame
	// that is about to be installed.
	if staging != nil {
		<-staging
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pool != nil && b.slot != nil {
		b.pool.Release(b.slot)
		b.slot = nil
	}
	if b.ppool != nil && b.pslot != nil {
		b.ppool.Release(b.pslot)
		b.pslot = nil
	}
	if b.ppool != nil && b.pback != nil {
		b.ppool.Release(b.pback)
		b.pback = nil
	}
	if b.fallback != nil {
		b.fallback.Dispose()
		b.fallback = nil
	}
	b.shadow = nil
	b.pshadowY, b.pshadowU = nil, nil
	b.pendY, b.pendU = nil, nil
	b.pendState = planeStageIdle
	b.pendDone = nil
	b.hasFrame, b.hasPlanes = false, false
}

// planeStageBusyLocked reports a frame being copied or a ready frame not yet
// drawn. The synchronous routes reject frames then: publishing an older staged
// plane frame over a newer RGBA one would step the picture backwards.
func (b *VideoBridge) planeStageBusyLocked() bool {
	return b.closed || b.pendState != planeStageIdle
}

// UploadPlanesFrameAsync accepts one NV12 frame without copying it on the
// calling thread. Ownership of y/uv transfers to the route until UploadPending
// reports false; do not recycle them before then. A worker copies the planes
// and publishes them straight away when the backend supports cross-thread queue
// writes; otherwise the next draw publishes on the raster thread. Returns false
// for bad arguments, a closed route, no plane device, or while an earlier frame
// is still staged.
func (b *VideoBridge) UploadPlanesFrameAsync(w, h int, y, uv []byte) bool {
	if b == nil {
		return false
	}
	if w < 2 || h < 2 || w%2 != 0 || h%2 != 0 {
		return false
	}
	yN, uvN := w*h, w*h/2
	if int64(len(y)) < int64(yN) || int64(len(uv)) < int64(uvN) {
		return false
	}
	b.mu.Lock()
	if b.closed || b.ppool == nil || b.pendState != planeStageIdle {
		b.mu.Unlock()
		return false
	}
	b.pendState = planeStageCopying
	done := make(chan struct{})
	b.pendDone = done
	cross := b.crossThreadWrites
	b.mu.Unlock()
	// The current frame stays drawable until the plane frame is published, so a
	// shape change never blanks the view for a tick.
	go b.stagePlaneFrame(w, h, y[:yN], uv[:uvN], done, cross)
	return true
}

// UploadPending reports a staged plane frame that the route has not drawn yet.
// While true the caller must keep the planes passed to UploadPlanesFrameAsync
// alive and must not poll another frame into them.
func (b *VideoBridge) UploadPending() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.pendState != planeStageIdle
}

// stagePlaneFrame is the copy worker: it owns the staging planes until it
// installs them under the route lock. It touches the GPU only when the backend
// supports cross-thread queue writes.
func (b *VideoBridge) stagePlaneFrame(w, h int, y, uv []byte, done chan struct{}, cross bool) {
	defer close(done)
	if cross {
		// The write runs without the route lock: the raster job records its
		// draw from the published slot, so holding mu across a multi-megabyte
		// queue write would stall every upload tick's paint. The backend copies
		// the planes during the call, so the caller's slices need no second copy
		// unless the upload is rejected.
		b.publishPlaneFrame(w, h, y, uv)
		return
	}
	b.mu.Lock()
	sy, su := b.pendY, b.pendU
	b.mu.Unlock()
	if len(sy) != len(y) {
		sy = make([]byte, len(y))
	}
	if len(su) != len(uv) {
		su = make([]byte, len(uv))
	}
	copy(sy, y)
	copy(su, uv)
	b.mu.Lock()
	b.pendY, b.pendU = sy, su
	b.pendW, b.pendH = w, h
	if b.closed {
		b.pendState = planeStageIdle
		b.mu.Unlock()
		return
	}
	// Ready stays pending until the raster thread publishes it during the draw
	// (its context can only be used there).
	b.pendState = planeStageReady
	b.mu.Unlock()
}

// publishPlaneFrame installs a frame on a backend that allows queue writes off
// the raster thread. The acquire and upload happen before the lock is taken;
// the lock then only swaps the finished frame in, so a paint can never wait on
// the GPU write. Only a rejected upload keeps a CPU copy (the draw converts it,
// V-2); a successful upload leaves the texture drawable, so the planes are not
// copied a second time.
func (b *VideoBridge) publishPlaneFrame(w, h int, y, uv []byte) {
	b.mu.Lock()
	target := b.pback
	if target != nil && (target.W != w || target.H != h) {
		b.ppool.Release(target)
		b.pback = nil
		target = nil
	}
	b.mu.Unlock()
	if target == nil {
		target, _ = b.ppool.Acquire(w, h)
	}
	uploaded := target != nil && b.ppool.UploadPlanes(target, y, uv) == nil
	var sy, su []byte
	if !uploaded {
		// The caller recycles its planes once UploadPending clears, so the CPU
		// fallback copy has to be taken now.
		b.mu.Lock()
		sy, su = b.pendY, b.pendU
		b.mu.Unlock()
		if len(sy) != len(y) {
			sy = make([]byte, len(y))
		}
		if len(su) != len(uv) {
			su = make([]byte, len(uv))
		}
		copy(sy, y)
		copy(su, uv)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		if target != nil {
			b.ppool.Release(target)
		}
		b.pendState = planeStageIdle
		return
	}
	if uploaded {
		// Double buffer: the finished back slot becomes drawable and the slot
		// that was drawn until now becomes the worker's target for the next
		// frame. Both stay live, so no draw ever samples a half-written pair.
		b.pback = b.pslot
		b.pslot = target
		b.uploads++
		// A plane frame replaces any RGBA frame; drop that slot so it cannot
		// paint stale contents and Close does not double-hold.
		if b.pool != nil && b.slot != nil {
			b.pool.Release(b.slot)
		}
		b.slot = nil
	} else {
		// The upload failed: drop the plane slots so the draw converts the new
		// shadow and counts the CPU fallback (V-2) instead of showing stale
		// texture pixels.
		if target != nil {
			b.ppool.Release(target)
		}
		if b.pslot != nil {
			b.ppool.Release(b.pslot)
			b.pslot = nil
		}
		if b.pback != nil {
			b.ppool.Release(b.pback)
			b.pback = nil
		}
		b.pshadowY, b.pendY = sy, b.pshadowY
		b.pshadowU, b.pendU = su, b.pshadowU
	}
	b.pendW, b.pendH = w, h
	b.pendState = planeStageIdle
	b.lastW, b.lastH = w, h
	b.frames++
	b.hasPlanes = true
	b.hasFrame = false
}

// publishPendingPlaneLocked turns a ready staged frame into the current frame:
// the staged planes become the draw shadow, the plane slot is rewritten in
// place, and the frame is counted once. Caller holds b.mu. An upload failure
// keeps the CPU shadow current so the draw converts it and counts the fallback
// (V-2) instead of dropping the frame.
func (b *VideoBridge) publishPendingPlaneLocked() {
	if b.pendState != planeStageReady {
		return
	}
	w, h := b.pendW, b.pendH
	// Staged planes become the draw shadow; the previous shadow becomes the
	// worker's scratch for the next frame.
	b.pshadowY, b.pendY = b.pendY, b.pshadowY
	b.pshadowU, b.pendU = b.pendU, b.pshadowU
	b.pendState = planeStageIdle
	b.lastW, b.lastH = w, h
	b.frames++
	b.hasPlanes = true
	b.hasFrame = false

	uploaded := false
	if b.ppool != nil {
		if slot, _, err := b.ppool.AcquireForFrame(b.pslot, w, h); err == nil {
			if uerr := b.ppool.UploadPlanes(slot, b.pshadowY, b.pshadowU); uerr == nil {
				b.pslot = slot
				b.uploads++
				uploaded = true
			} else {
				b.ppool.Release(slot)
			}
		}
	}
	if !uploaded {
		b.pslot = nil
		return
	}
	// A plane frame replaces any RGBA frame; drop that slot so it cannot paint
	// stale contents and Close does not double-hold.
	if b.pool != nil && b.slot != nil {
		b.pool.Release(b.slot)
	}
	b.slot = nil
}

// UploadPlanesFrame uploads one new NV12 frame into the plane slot
// without drawing. Same tick/paint split and ownership as UploadFrame:
// y (w*h) plus uv (w*h/2) are copied synchronously and may be recycled
// after it returns. Even sizes only; anything else reports false so the
// caller falls back to the RGBA shape.
// UploadPlanesFrame uploads one new NV12 frame into the plane slot
// without drawing. Same tick/paint split and ownership as UploadFrame:
// y (w*h) plus uv (w*h/2) are copied synchronously and may be recycled
// after it returns. Even sizes only; anything else reports false so the
// caller falls back to the RGBA shape.
func (b *VideoBridge) UploadPlanesFrame(w, h int, y, uv []byte) bool {
	if b == nil {
		return false
	}
	if w < 2 || h < 2 || w%2 != 0 || h%2 != 0 {
		return false
	}
	if int64(len(y)) < int64(w)*int64(h) || int64(len(uv)) < int64(w)*int64(h)/2 {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.planeStageBusyLocked() {
		return false
	}
	if b.ppool == nil {
		return false
	}
	slot, _, err := b.ppool.AcquireForFrame(b.pslot, w, h)
	if err != nil {
		return false
	}
	b.pslot = slot
	if b.pback != nil && b.pback != slot {
		b.ppool.Release(b.pback)
		b.pback = nil
	}
	b.lastW, b.lastH = w, h
	if uerr := b.ppool.UploadPlanes(slot, y, uv); uerr != nil {
		return false
	}
	b.frames++
	b.uploads++
	// A plane frame replaces any RGBA frame; drop the old slot so a later
	// RGBA fallback cannot paint stale contents and Close does not double-hold.
	if b.pool != nil && b.slot != nil {
		b.pool.Release(b.slot)
	}
	b.slot = nil
	b.hasPlanes = true
	b.hasFrame = false
	yN, uvN := w*h, w*h/2
	if len(b.pshadowY) != yN {
		b.pshadowY = make([]byte, yN)
	}
	if len(b.pshadowU) != uvN {
		b.pshadowU = make([]byte, uvN)
	}
	copy(b.pshadowY, y[:yN])
	copy(b.pshadowU, uv[:uvN])
	return true
}
