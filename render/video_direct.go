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

import ()

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
