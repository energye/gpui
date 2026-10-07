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

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

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
