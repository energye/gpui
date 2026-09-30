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

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// P0 video foundation: backend query + independent texture pool + fallback count.
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
}

// VideoTexturePool holds video textures apart from the image cache.
type VideoTexturePool struct {
	mu        sync.Mutex
	device    hal.Device
	idle      []*VideoSlot
	live      map[*VideoSlot]struct{}
	peak      int
	evictions uint64
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
	return VideoPoolStats{Live: len(p.live), Idle: len(p.idle), Peak: p.peak, Evictions: p.evictions}
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
