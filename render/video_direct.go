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
// P3-A planes: NV12 Y (R8) + UV (RG8) slots, one upload group per frame,
// GPU convert via Context.DrawVideoPlanes (layer 4: YUV convert pipeline,
// one WGSL source for both backends; missing variants keep the CPU
// fallback instead of dropping the frame).
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

// nv12ToRGBA converts NV12 planes to packed RGBA (BT.601 limited range,
// the decoder default): R=1.164(Y-16)+1.596(V-128),
// G=1.164(Y-16)-0.391(U-128)-0.813(V-128), B=1.164(Y-16)+2.018(U-128),
// A=255. CPU fallback and shader-parity oracle only: the fast path
// converts on the GPU (layer 4). dst must hold w*h*4.
func nv12ToRGBA(dst, y, uv []byte, w, h int) {
	for r := 0; r < h; r++ {
		uvRow := uv[(r/2)*w : (r/2)*w+w]
		yRow := y[r*w : r*w+w]
		dRow := dst[r*w*4 : r*w*4+w*4]
		for x := 0; x < w; x++ {
			yy := float64(yRow[x])
			u := float64(uvRow[(x/2)*2]) - 128
			v := float64(uvRow[(x/2)*2+1]) - 128
			c := yy - 16
			rf := 1.164*c + 1.596*v
			gf := 1.164*c - 0.391*u - 0.813*v
			bf := 1.164*c + 2.018*u
			dRow[4*x] = clampU8(rf)
			dRow[4*x+1] = clampU8(gf)
			dRow[4*x+2] = clampU8(bf)
			dRow[4*x+3] = 255
		}
	}
}

func clampU8(v float64) byte {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return byte(v + 0.5)
}

// VideoPlaneSlot is one video route's NV12 texture pair (Y + interleaved
// UV), rewritten in place. Y is R8, UV is RG8; both share W×H addressing
// (UV has H/2 rows of W bytes).
type VideoPlaneSlot struct {
	W      int
	H      int
	YTex   hal.Texture
	YView  hal.TextureView
	UVTex  hal.Texture
	UVView hal.TextureView
	device hal.Device
	// padY/padUV stage unaligned widths (row pitch to 256): allocated
	// once per size, reused every frame. Tight widths upload directly.
	padY  []byte
	padUV []byte
}

func (s *VideoPlaneSlot) destroy(device hal.Device) {
	if s == nil {
		return
	}
	for _, v := range []hal.TextureView{s.YView, s.UVView} {
		if v == nil {
			continue
		}
		if device != nil {
			device.DestroyTextureView(v)
		} else {
			v.Destroy()
		}
	}
	s.YView, s.UVView = nil, nil
	for _, t := range []hal.Texture{s.YTex, s.UVTex} {
		if t == nil {
			continue
		}
		if device != nil {
			device.DestroyTexture(t)
		} else {
			t.Destroy()
		}
	}
	s.YTex, s.UVTex = nil, nil
	s.padY, s.padUV = nil, nil
}

// packedPlaneViews boxes the plane views for Context.DrawVideoPlanes.
// Either nil yields the zero handle (fail closed).
func (s *VideoPlaneSlot) packedPlaneViews() (gpucontext.TextureView, gpucontext.TextureView) {
	if s == nil || s.YView == nil || s.UVView == nil {
		return gpucontext.TextureView{}, gpucontext.TextureView{}
	}
	return gpucontext.PackTextureView(s.YView), gpucontext.PackTextureView(s.UVView)
}

// VideoPlanePool holds NV12 plane slots apart from the image cache.
type VideoPlanePool struct {
	mu        sync.Mutex
	device    hal.Device
	idle      []*VideoPlaneSlot
	live      map[*VideoPlaneSlot]struct{}
	peak      int
	evictions uint64
	uploads   uint64
	maxSlots  int
}

// NewVideoPlanePool creates an independent NV12 plane pool on device.
// Device is borrowed, not owned; Close destroys textures, not the device.
func NewVideoPlanePool(device hal.Device) *VideoPlanePool {
	return &VideoPlanePool{
		device:   device,
		live:     make(map[*VideoPlaneSlot]struct{}),
		maxSlots: videoPoolMaxSlots,
	}
}

// videoPlaneTexture builds one plane texture (R8 or RG8, copy-dst bound).
func videoPlaneTexture(device hal.Device, label string, w, h int, format types.TextureFormat) (hal.Texture, hal.TextureView, error) {
	tex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         label,
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec // video dims fit uint32
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        format,
		Usage:         types.TextureUsageCopyDst | types.TextureUsageTextureBinding,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("render: video plane CreateTexture %dx%d: %w", w, h, err)
	}
	view, err := device.CreateTextureView(tex, &hal.TextureViewDescriptor{
		Label:         label + "-view",
		Format:        format,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Destroy()
		return nil, nil, fmt.Errorf("render: video plane CreateTextureView: %w", err)
	}
	return tex, view, nil
}

// Acquire returns an exact-size idle plane slot or creates one.
// NV12 needs even width and height; anything else reports an error so
// the caller falls back to the RGBA path.
func (p *VideoPlanePool) Acquire(w, h int) (*VideoPlaneSlot, error) {
	if p == nil || p.device == nil {
		return nil, fmt.Errorf("render: video plane pool has no device")
	}
	if w < 2 || h < 2 || w%2 != 0 || h%2 != 0 {
		return nil, fmt.Errorf("render: video plane size %dx%d invalid (NV12 needs even)", w, h)
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
	yTex, yView, err := videoPlaneTexture(p.device, "video-plane-y", w, h, types.TextureFormatR8Unorm)
	if err != nil {
		return nil, err
	}
	uvTex, uvView, err := videoPlaneTexture(p.device, "video-plane-uv", w, h/2, types.TextureFormatRG8Unorm)
	if err != nil {
		if yView != nil {
			p.device.DestroyTextureView(yView)
		}
		if yTex != nil {
			p.device.DestroyTexture(yTex)
		}
		return nil, err
	}
	s := &VideoPlaneSlot{W: w, H: h, YTex: yTex, YView: yView, UVTex: uvTex, UVView: uvView, device: p.device}
	p.live[s] = struct{}{}
	if len(p.live) > p.peak {
		p.peak = len(p.live)
	}
	return s, nil
}

// Release returns a plane slot to idle for in-place rewrite reuse.
func (p *VideoPlanePool) Release(s *VideoPlaneSlot) {
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

// Stats snapshots plane pool health.
func (p *VideoPlanePool) Stats() VideoPoolStats {
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

// Close destroys idle and live plane textures. Borrowed device untouched.
func (p *VideoPlanePool) Close() {
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
	p.live = make(map[*VideoPlaneSlot]struct{})
}

// AcquireForFrame reuses cur when the size matches, otherwise releases it
// and acquires the new size. Nil pool or bad size errors to RGBA fallback.
func (p *VideoPlanePool) AcquireForFrame(cur *VideoPlaneSlot, w, h int) (*VideoPlaneSlot, bool, error) {
	if p == nil {
		return nil, false, fmt.Errorf("render: video plane pool has no device")
	}
	if w < 2 || h < 2 || w%2 != 0 || h%2 != 0 {
		return nil, false, fmt.Errorf("render: video plane size %dx%d invalid (NV12 needs even)", w, h)
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

// videoPlanePitch aligns one plane row to the backend row rule.
func videoPlanePitch(w int) (uint32, bool) {
	if w <= 0 {
		return 0, false
	}
	pitch := (w + videoRowPitchAlignment - 1) &^ (videoRowPitchAlignment - 1)
	if pitch <= 0 || int64(pitch) > int64(^uint32(0)) {
		return 0, false
	}
	return uint32(pitch), true
}

// padPlaneRows copies tight rows into pitch-strided scratch (reused).
func padPlaneRows(scratch []byte, pix []byte, w, rows, pitch int) []byte {
	need := pitch * rows
	if len(scratch) != need {
		scratch = make([]byte, need)
	}
	for r := 0; r < rows; r++ {
		copy(scratch[r*pitch:r*pitch+w], pix[r*w:r*w+w])
	}
	return scratch
}

// UploadPlanes rewrites the slot pair in place: Y (R8, w×h) plus UV
// (RG8 interleaved, w×h/2). One call counts one upload unit, so the
// upload discipline stays one upload per frame like the RGBA path.
// Tight widths upload directly; other widths pad rows into slot-owned
// scratch (memcpy only, no arithmetic). Any failure errors to the
// RGBA fallback path.
func (p *VideoPlanePool) UploadPlanes(s *VideoPlaneSlot, y, uv []byte) error {
	if p == nil || p.device == nil {
		return fmt.Errorf("render: video plane upload has no device")
	}
	if s == nil || s.YTex == nil || s.UVTex == nil {
		return fmt.Errorf("render: video plane upload has no slot")
	}
	w, h := s.W, s.H
	if w < 2 || h < 2 || w%2 != 0 || h%2 != 0 {
		return fmt.Errorf("render: video plane slot size %dx%d invalid", w, h)
	}
	if int64(len(y)) < int64(w)*int64(h) || int64(len(uv)) < int64(w)*int64(h)/2 {
		return fmt.Errorf("render: video plane pixels short y=%d uv=%d want %dx%d", len(y), len(uv), w, h)
	}
	if int64(w)*int64(h) > int64(videoDefaultMaxStaging) {
		return fmt.Errorf("render: video plane frame %dx%d exceeds staging", w, h)
	}
	pitch, ok := videoPlanePitch(w)
	if !ok {
		return fmt.Errorf("render: video plane width %d breaks pitch", w)
	}
	queue := p.device.Queue()
	if queue == nil {
		return fmt.Errorf("render: video plane upload has no queue")
	}
	yp, uvs := y, uv
	if int(pitch) != w {
		s.padY = padPlaneRows(s.padY, y, w, h, int(pitch))
		s.padUV = padPlaneRows(s.padUV, uv, w, h/2, int(pitch))
		yp, uvs = s.padY, s.padUV
	}
	upload := func(tex hal.Texture, pix []byte, rows int) error {
		dst := &hal.ImageCopyTexture{Texture: tex, MipLevel: 0, Aspect: types.TextureAspectAll}
		layout := &hal.ImageDataLayout{Offset: 0, BytesPerRow: pitch, RowsPerImage: uint32(rows)}
		size := &hal.Extent3D{Width: uint32(w), Height: uint32(rows), DepthOrArrayLayers: 1}
		if err := queue.WriteTexture(dst, pix[:int(pitch)*rows], layout, size); err != nil {
			return fmt.Errorf("render: video plane WriteTexture %dx%d: %w", w, rows, err)
		}
		return nil
	}
	if err := upload(s.YTex, yp, h); err != nil {
		return err
	}
	if err := upload(s.UVTex, uvs, h/2); err != nil {
		return err
	}
	p.mu.Lock()
	p.uploads++
	p.mu.Unlock()
	videoUploadTotal.Add(1)
	return nil
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
	// ppool/pslot carry the NV12 shape (layer 3); pshadow retains the
	// last planes for the fallback convert. hasPlanes selects the
	// shape DrawCurrent draws.
	ppool     *VideoPlanePool
	pslot     *VideoPlaneSlot
	pshadowY  []byte
	pshadowU  []byte
	hasPlanes bool
}

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
	if b.ppool != nil {
		ps := b.ppool.Stats()
		st.Live += ps.Live
		st.Idle += ps.Idle
		st.Evictions += ps.Evictions
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
