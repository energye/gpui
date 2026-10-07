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
	"fmt"
	"sync/atomic"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// NewVideoTexturePool creates an independent video pool on device.
// Device is borrowed, not owned; Close destroys textures, not the device.
func NewVideoTexturePool(device hal.Device) *VideoTexturePool {
	return &VideoTexturePool{
		device:   device,
		live:     make(map[*VideoSlot]struct{}),
		maxSlots: videoPoolMaxSlots,
	}
}

// Acquire returns an exact-size idle slot or creates one.
// Size change creates a new texture; old sizes stay idle for reuse.
// Acquire returns an exact-size idle slot or creates one.
// Size change creates a new texture; old sizes stay idle for reuse.
func (p *VideoTexturePool) Acquire(w, h int) (*VideoSlot, error) {
	if p == nil || p.device == nil {
		return nil, fmt.Errorf("render: video pool has no device")
	}
	if w < 1 || h < 1 {
		return nil, fmt.Errorf("render: video slot size %dx%d invalid", w, h)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, s := range p.idle {
		if s.W == w && s.H == h {
			p.idle = append(p.idle[:i], p.idle[i+1:]...)
			p.live[s] = struct{}{}
			if len(p.live) > p.peak {
				p.peak = len(p.live)
			}
			return s, nil
		}
	}
	if len(p.live)+len(p.idle) >= p.maxSlots && len(p.idle) > 0 {
		old := p.idle[0]
		p.idle = p.idle[1:]
		old.destroy(p.device)
		p.evictions++
	}
	tex, err := p.device.CreateTexture(&hal.TextureDescriptor{
		Label:         "video-direct",
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec // video dims fit uint32
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatRGBA8Unorm,
		Usage:         types.TextureUsageCopyDst | types.TextureUsageTextureBinding,
	})
	if err != nil {
		return nil, fmt.Errorf("render: video CreateTexture %dx%d: %w", w, h, err)
	}
	view, err := p.device.CreateTextureView(tex, &hal.TextureViewDescriptor{
		Label:         "video-direct-view",
		Format:        types.TextureFormatRGBA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Destroy()
		return nil, fmt.Errorf("render: video CreateTextureView: %w", err)
	}
	s := &VideoSlot{W: w, H: h, Texture: tex, View: view, device: p.device}
	p.live[s] = struct{}{}
	if len(p.live) > p.peak {
		p.peak = len(p.live)
	}
	return s, nil
}

// Release returns a slot to idle for in-place rewrite reuse.
// Release returns a slot to idle for in-place rewrite reuse.
func (p *VideoTexturePool) Release(s *VideoSlot) {
	if p == nil || s == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.live[s]; !ok {
		return
	}
	delete(p.live, s)
	p.idle = append(p.idle, s)
}

// Stats snapshots pool health.
// Stats snapshots pool health.
func (p *VideoTexturePool) Stats() VideoPoolStats {
	if p == nil {
		return VideoPoolStats{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return VideoPoolStats{
		Live:      len(p.live),
		Idle:      len(p.idle),
		Peak:      p.peak,
		Evictions: p.evictions,
		Uploads:   p.uploads,
	}
}

// Close destroys idle and live textures. Borrowed device is untouched.
// Close destroys idle and live textures. Borrowed device is untouched.
func (p *VideoTexturePool) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.idle {
		s.destroy(p.device)
	}
	for s := range p.live {
		s.destroy(p.device)
	}
	p.idle = nil
	p.live = make(map[*VideoSlot]struct{})
}

func (s *VideoSlot) destroy(device hal.Device) {
	if s == nil {
		return
	}
	if s.View != nil {
		if device != nil {
			device.DestroyTextureView(s.View)
		} else {
			s.View.Destroy()
		}
		s.View = nil
	}
	if s.Texture != nil {
		if device != nil {
			device.DestroyTexture(s.Texture)
		} else {
			s.Texture.Destroy()
		}
		s.Texture = nil
	}
}

// videoUploadTotal counts direct texture rewrites process-wide.
// P2 evidence uses it against shown new frames: no extra retransmit.
var videoUploadTotal atomic.Uint64

// VideoUploadTotal reports process-wide direct upload count.
// videoBytesPerRow reports the tight RGBA row stride when it satisfies
// the backend row alignment. Standard video widths already satisfy it
// (width*4 is a multiple of 256 for 1280/1920/2560/3840).
func videoBytesPerRow(w int) (uint32, bool) {
	if w <= 0 {
		return 0, false
	}
	row := int64(w) * 4
	if row <= 0 || row > int64(^uint32(0)) {
		return 0, false
	}
	if row%int64(videoRowPitchAlignment) != 0 {
		return 0, false
	}
	return uint32(row), true
}

// Upload rewrites the slot texture in place with tight RGBA pixels.
// Size must match the slot; len(pix) must hold w*h*4. Same-size reuse
// keeps the texture, so steady play pays one whole upload per new frame.
// Unaligned widths and missing queue/device report an error so the caller
// falls back to the generic DrawImage path and counts it.
// Upload rewrites the slot texture in place with tight RGBA pixels.
// Size must match the slot; len(pix) must hold w*h*4. Same-size reuse
// keeps the texture, so steady play pays one whole upload per new frame.
// Unaligned widths and missing queue/device report an error so the caller
// falls back to the generic DrawImage path and counts it.
func (p *VideoTexturePool) Upload(s *VideoSlot, pix []byte) error {
	if p == nil || p.device == nil {
		return fmt.Errorf("render: video upload has no device")
	}
	if s == nil || s.Texture == nil {
		return fmt.Errorf("render: video upload has no slot")
	}
	if s.W <= 0 || s.H <= 0 {
		return fmt.Errorf("render: video slot size %dx%d invalid", s.W, s.H)
	}
	row, ok := videoBytesPerRow(s.W)
	if !ok {
		return fmt.Errorf("render: video width %d breaks row alignment", s.W)
	}
	need := int64(s.W) * int64(s.H) * 4
	if int64(len(pix)) < need {
		return fmt.Errorf("render: video pixels %d bytes, want %d", len(pix), need)
	}
	queue := p.device.Queue()
	if queue == nil {
		return fmt.Errorf("render: video upload has no queue")
	}
	dst := &hal.ImageCopyTexture{
		Texture:  s.Texture,
		MipLevel: 0,
		Aspect:   types.TextureAspectAll,
	}
	layout := &hal.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  row,
		RowsPerImage: uint32(s.H),
	}
	size := &hal.Extent3D{
		Width:              uint32(s.W),
		Height:             uint32(s.H),
		DepthOrArrayLayers: 1,
	}
	if err := queue.WriteTexture(dst, pix[:need], layout, size); err != nil {
		return fmt.Errorf("render: video WriteTexture %dx%d: %w", s.W, s.H, err)
	}
	p.mu.Lock()
	p.uploads++
	p.mu.Unlock()
	videoUploadTotal.Add(1)
	return nil
}

// AcquireForFrame reuses cur when the size matches, otherwise releases it
// and acquires the new size. It reports rebuilt=true only on size change,
// so callers keep one texture per video route in steady play. Nil pool or
// bad size returns an error so the caller falls back to the generic path.
// AcquireForFrame reuses cur when the size matches, otherwise releases it
// and acquires the new size. It reports rebuilt=true only on size change,
// so callers keep one texture per video route in steady play. Nil pool or
// bad size returns an error so the caller falls back to the generic path.
func (p *VideoTexturePool) AcquireForFrame(cur *VideoSlot, w, h int) (*VideoSlot, bool, error) {
	if p == nil {
		return nil, false, fmt.Errorf("render: video pool has no device")
	}
	if w < 1 || h < 1 {
		return nil, false, fmt.Errorf("render: video slot size %dx%d invalid", w, h)
	}
	if cur != nil && cur.W == w && cur.H == h {
		return cur, false, nil
	}
	if cur != nil {
		p.Release(cur)
	}
	s, err := p.Acquire(w, h)
	if err != nil {
		return nil, false, err
	}
	return s, true, nil
}

// VideoDrawOptions describes where one uploaded frame lands on screen.
// Decoding resolution stays fixed; only the destination rect scales.
type VideoDrawOptions struct {
	X             float64
	Y             float64
	DstWidth      float64
	DstHeight     float64
	Opacity       float64
	Interpolation InterpolationMode
}

// DrawVideoFrame draws one frame through the generic image path after the
// caller uploaded it with Upload. It is the guaranteed-picture fallback:
// use Context.DrawVideoSlot first for the zero-upload quad, and call this
// only when DrawVideoSlot reports false. No-new-frame means the caller
// skips both Upload and drawing, so upload count stays equal to shown
// new frames.
// packedView boxes the slot texture view into the opaque GPU handle for
// Context.DrawVideoSlot. Nil slot/view yields the zero handle (fail closed).
func (s *VideoSlot) packedView() gpucontext.TextureView {
	if s == nil || s.View == nil {
		return gpucontext.TextureView{}
	}
	return gpucontext.PackTextureView(s.View)
}

// nv12ToRGBA converts NV12 planes to packed RGBA (BT.601 limited range,
// the decoder default): R=1.164(Y-16)+1.596(V-128),
// G=1.164(Y-16)-0.391(U-128)-0.813(V-128), B=1.164(Y-16)+2.018(U-128),
// A=255. CPU fallback and shader-parity oracle only: the fast path
// converts on the GPU (layer 4). dst must hold w*h*4.
