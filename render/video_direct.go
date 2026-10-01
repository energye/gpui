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
	"sync"
	"sync/atomic"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// P0 video foundation: backend query + independent texture pool + fallback count.
// P2 direct upload: in-place rewrite via hal.Queue.WriteTexture + upload
// counting + zero-upload quad via Context.DrawVideoSlot (context_image.go).
//
// Video textures live outside the 64MB generic image cache and never take part
// in its eviction. One video route owns one slot, rewritten in place.
// Failures fall back to the generic DrawImage path and count as CPU fallback.

const (
	// videoRowPitchAlignment is the conservative upload row alignment.
	// WebGPU requires 256-byte BytesPerRow; GL accepts it as well.
	videoRowPitchAlignment = 256

	// videoDefaultMaxStaging caps one staging upload when the backend
	// does not report its own limit (64MB, same as hal default).
	videoDefaultMaxStaging = uint64(64 << 20)

	// videoPoolMaxSlots bounds concurrent video routes per process.
	videoPoolMaxSlots = 8
)

// VideoBackendCaps describes the current shared device in backend-neutral terms.
// BackendName is diagnostic display only; callers branch on capabilities.
type VideoBackendCaps struct {
	HasDevice             bool
	AdapterName           string
	SupportsCommandCopies bool
	MaxTexture2D          uint32
	MaxStagingBytes       uint64
	RowPitchAlignment     uint32
}

// QueryVideoBackend reports capabilities for device without owning it.
// Nil device returns HasDevice=false. Only hal/types are used.
func QueryVideoBackend(device hal.Device) VideoBackendCaps {
	caps := VideoBackendCaps{RowPitchAlignment: videoRowPitchAlignment}
	if device == nil {
		return caps
	}
	caps.HasDevice = true
	limits := device.Limits()
	caps.MaxTexture2D = limits.MaxTextureDimension2D
	if caps.MaxTexture2D == 0 {
		caps.MaxTexture2D = types.DefaultLimits().MaxTextureDimension2D
	}
	if q := device.Queue(); q != nil {
		caps.SupportsCommandCopies = q.SupportsCommandBufferCopies()
	}
	caps.MaxStagingBytes = videoDefaultMaxStaging
	if s, ok := device.(hal.MaxStagingBufferSizer); ok {
		if n := s.MaxStagingBufferSize(); n > 0 {
			caps.MaxStagingBytes = n
		}
	}
	return caps
}

// BorrowVideoBackend borrows the shared present device for video.
// Returned handles stay owned by the PresentTarget share; callers must not
// Release them. ok=false means no window device is open yet.
func BorrowVideoBackend() (device hal.Device, queue hal.Queue, adapter hal.Adapter, caps VideoBackendCaps, ok bool) {
	_, adapter, device, borrowable, _ := peekShared()
	if !borrowable || device == nil {
		return nil, nil, nil, VideoBackendCaps{RowPitchAlignment: videoRowPitchAlignment}, false
	}
	caps = QueryVideoBackend(device)
	if adapter != nil {
		caps.AdapterName = adapter.Info().Name
	}
	return device, device.Queue(), adapter, caps, true
}

// videoFallbackTotal counts video-to-generic fallbacks process-wide.
var videoFallbackTotal atomic.Uint64

// VideoFallbackTotal reports process-wide video fallback count.
func VideoFallbackTotal() uint64 {
	return videoFallbackTotal.Load()
}

// RecordVideoFallback counts one fallback to the generic DrawImage path.
// It funnels into the existing CPU fallback counter so soak tools see it.
func (c *Context) RecordVideoFallback(reason string) {
	videoFallbackTotal.Add(1)
	if reason == "" {
		reason = "video:generic"
	} else {
		reason = "video:" + reason
	}
	c.recordCPUFallbackReason(reason)
}

// VideoSlot is one video route's texture, rewritten in place.
type VideoSlot struct {
	W       int
	H       int
	Texture hal.Texture
	View    hal.TextureView
	device  hal.Device
}

// VideoPoolStats reports independent video pool health.
// Evictions must stay zero in healthy playback.
type VideoPoolStats struct {
	Live      int
	Idle      int
	Peak      int
	Evictions uint64
	Uploads   uint64
}

// VideoTexturePool holds video textures apart from the image cache.
type VideoTexturePool struct {
	mu        sync.Mutex
	device    hal.Device
	idle      []*VideoSlot
	live      map[*VideoSlot]struct{}
	peak      int
	evictions uint64
	uploads   uint64
	maxSlots  int
}

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
func VideoUploadTotal() uint64 {
	return videoUploadTotal.Load()
}

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
func (s *VideoSlot) packedView() gpucontext.TextureView {
	if s == nil || s.View == nil {
		return gpucontext.TextureView{}
	}
	return gpucontext.PackTextureView(s.View)
}

// VideoBridge converges one video route: pool + current slot + owned
// fallback buffer. Windows call Show per new frame; Show uploads once then
// draws via the zero-upload quad, falling back to the generic path with a
// block copy when direct fails. No-new-frame means the caller skips Show,
// so upload count stays equal to shown new frames. ShowSeq takes a caller
// frame sequence: repeat calls with the same seq redraw the uploaded frame
// into another rect (same-source multi-size views) without re-uploading.
//
// Fast path pays zero CPU conversion: Upload takes raw RGBA straight into
// the texture. Video frames are opaque (ffmpeg RGBA alpha is 255), so
// straight equals premultiplied and PremultipliedData is skipped. The
// fallback path owns its buffer and copies rows in one block, never the
// old per-pixel loop.
type VideoBridge struct {
	mu        sync.Mutex
	pool      *VideoTexturePool
	slot      *VideoSlot
	fallback  *ImageBuf
	frames    uint64
	uploads   uint64
	fallbacks uint64
	redraws   uint64
	lastSeq   uint64
	hasSeq    bool
	lastW     int
	lastH     int
	fbSeq     uint64
	hasFbSeq  bool
	// shadow retains the last uploaded frame for the fallback draw
	// (UploadFrame copies synchronously; Poll recycles its Pix).
	shadow []byte
	// hasFrame reports an uploaded frame ready for DrawCurrent.
	hasFrame bool
}

// NewVideoBridge creates one route on the borrowed device.
// Nil device yields a fallback-only bridge: Show always takes the generic
// path and counts fallbacks, never crashes.
func NewVideoBridge(device hal.Device) *VideoBridge {
	if device == nil {
		return &VideoBridge{}
	}
	return &VideoBridge{pool: NewVideoTexturePool(device)}
}

// EnsureDevice attaches the pool device when the window device opens after
// the bridge was created fallback-only. No-op when already attached.
func (b *VideoBridge) EnsureDevice(device hal.Device) {
	if b == nil || device == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pool != nil {
		return
	}
	b.pool = NewVideoTexturePool(device)
}

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
// (block copy, counted) otherwise. ok=false means nothing was drawn.
func (b *VideoBridge) DrawCurrent(c *Context, opts VideoDrawOptions) (direct, ok bool) {
	if b == nil || c == nil {
		return false, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
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
func (b *VideoBridge) ensureFallbackLocked(w, h int) bool {
	if b.fallback != nil && !b.fallback.Disposed() {
		if fw, fh := b.fallback.Bounds(); fw == w && fh == h {
			return true
		}
		b.fallback.Dispose()
		b.fallback = nil
	}
	buf, err := NewImageBuf(w, h, FormatRGBA8)
	if err != nil || buf == nil {
		return false
	}
	b.fallback = buf
	return true
}

// BridgeStats snapshots one route: frames seen, direct uploads, fallbacks.
type BridgeStats struct {
	Frames    uint64
	Uploads   uint64
	Fallbacks uint64
	Redraws   uint64
	Live      int
	Idle      int
	Evictions uint64
}

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
	return st
}

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
	if b.fallback != nil {
		b.fallback.Dispose()
		b.fallback = nil
	}
	b.shadow = nil
	b.hasFrame = false
}
