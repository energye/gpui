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
func NewVideoBridge(device hal.Device) *VideoBridge {
	if device == nil {
		return &VideoBridge{}
	}
	return &VideoBridge{pool: NewVideoTexturePool(device), ppool: NewVideoPlanePool(device)}
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

// UploadFrame uploads one new frame into the slot without drawing.
// Call it on the tick with the Poll pix (valid until the next Poll):
// the upload copies synchronously, so the pix may be recycled right
// after it returns. Draw with DrawCurrent in Paint. No-new-frame means
// the caller skips UploadFrame, so uploads stay equal to shown frames.
// It returns false when there is no device yet or the upload failed —
// the Paint draw then falls back and counts one fallback.
// UploadFrame uploads one new frame into the slot without drawing.
// Call it on the tick with the Poll pix (valid until the next Poll):
// the upload copies synchronously, so the pix may be recycled right
// after it returns. Draw with DrawCurrent in Paint. No-new-frame means
// the caller skips UploadFrame, so uploads stay equal to shown frames.
// It returns false when there is no device yet or the upload failed —
// the Paint draw then falls back and counts one fallback.
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
	if b.pool == nil {
		return false
	}
	slot, _, err := b.pool.AcquireForFrame(b.slot, w, h)
	if err != nil {
		return false
	}
	b.slot = slot
	b.lastW, b.lastH = w, h
	if uerr := b.pool.Upload(slot, pix[:need]); uerr != nil {
		return false
	}
	b.frames++
	b.uploads++
	b.hasFrame = true
	if len(b.shadow) != int(need) {
		b.shadow = make([]byte, need)
	}
	copy(b.shadow, pix[:need])
	return true
}

// DrawCurrent draws the last uploaded frame. Call it in Paint with the
// display rect: zero-upload quad on the fast path, generic fallback
// (block copy, counted) otherwise. NV12 shape converts on the GPU via
// DrawVideoPlanes; when the GPU entry is unavailable it converts into
// the owned fallback buffer on CPU. ok=false means nothing was drawn.
// DrawCurrent draws the last uploaded frame. Call it in Paint with the
// display rect: zero-upload quad on the fast path, generic fallback
// (block copy, counted) otherwise. NV12 shape converts on the GPU via
// DrawVideoPlanes; when the GPU entry is unavailable it converts into
// the owned fallback buffer on CPU. ok=false means nothing was drawn.
func (b *VideoBridge) DrawCurrent(c *Context, opts VideoDrawOptions) (direct, ok bool) {
	if b == nil || c == nil {
		return false, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.hasPlanes && b.pslot != nil {
		if c.DrawVideoPlanes(b.pslot, opts) {
			return true, true
		}
		w, h := b.pslot.W, b.pslot.H
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
	if !b.hasFrame || b.slot == nil {
		return false, false
	}
	if c.DrawVideoSlot(b.slot, opts) {
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

// ensureFallbackLocked keeps one owned buffer at the current size.
// Caller holds b.mu.
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

// Close releases the slot and fallback buffer. Borrowed device untouched.
// Close releases the slot and fallback buffer. Borrowed device untouched.
func (b *VideoBridge) Close() {
	if b == nil {
		return
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
	if b.fallback != nil {
		b.fallback.Dispose()
		b.fallback = nil
	}
	b.shadow = nil
	b.pshadowY, b.pshadowU = nil, nil
	b.hasFrame, b.hasPlanes = false, false
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
	if b.ppool == nil {
		return false
	}
	slot, _, err := b.ppool.AcquireForFrame(b.pslot, w, h)
	if err != nil {
		return false
	}
	b.pslot = slot
	b.lastW, b.lastH = w, h
	if uerr := b.ppool.UploadPlanes(slot, y, uv); uerr != nil {
		return false
	}
	b.frames++
	b.uploads++
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
