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
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

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
// VideoFallbackTotal reports process-wide video fallback count.
func VideoFallbackTotal() uint64 {
	return videoFallbackTotal.Load()
}

// RecordVideoFallback counts one fallback to the generic DrawImage path.
// It funnels into the existing CPU fallback counter so soak tools see it.
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
// VideoUploadTotal reports process-wide direct upload count.
func VideoUploadTotal() uint64 {
	return videoUploadTotal.Load()
}

// videoBytesPerRow reports the tight RGBA row stride when it satisfies
// the backend row alignment. Standard video widths already satisfy it
// (width*4 is a multiple of 256 for 1280/1920/2560/3840).
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
