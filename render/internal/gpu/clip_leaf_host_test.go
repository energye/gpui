// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

//go:build !nogpu

// Host-side tests for the Vello compute clip pipeline (no GPU device needed).
// These verify the clip_leaf stage wiring: enum order, workgroup count,
// bind group layout, buffer sizing, and clip-scene path metadata vs the
// CPU scene layout.

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render/internal/gpu/tilecompute"
)

// TestVelloStageLayout_ClipLeafOrder verifies clip_leaf runs after draw_leaf
// and before path_count/coarse in the stage enum.
func TestVelloStageLayout_ClipLeafOrder(t *testing.T) {
	if !(VelloStageDrawLeaf < VelloStageClipLeaf) {
		t.Error("VelloStageClipLeaf must come after VelloStageDrawLeaf (needs clip_inp from draw_leaf)")
	}
	if !(VelloStageClipLeaf < VelloStagePathCount) {
		t.Error("VelloStageClipLeaf must come before VelloStagePathCount")
	}
	if !(VelloStageClipLeaf < VelloStageCoarse) {
		t.Error("VelloStageClipLeaf must come before VelloStageCoarse (coarse reads fixed-up draw_monoids)")
	}
	if int(VelloStageCount) != 10 {
		t.Errorf("VelloStageCount = %d, want 10 (9 original + clip_leaf)", int(VelloStageCount))
	}
}

// TestVelloStageClipLeafString verifies the stage name used in diagnostics.
func TestVelloStageClipLeafString(t *testing.T) {
	if got := VelloStageClipLeaf.String(); got != "clip_leaf" {
		t.Errorf("VelloStageClipLeaf.String() = %q, want %q", got, "clip_leaf")
	}
}

// TestComputeWorkgroupCount_ClipLeaf verifies clip_leaf dispatches exactly
// one workgroup whenever there are clip operations, and zero otherwise.
func TestComputeWorkgroupCount_ClipLeaf(t *testing.T) {
	d := &VelloComputeDispatcher{wgSize: velloWGSize}
	for _, tc := range []struct {
		clips uint32
		want  uint32
	}{
		{0, 0}, {1, 1}, {2, 1}, {64, 1}, {1024, 1},
	} {
		got := d.ComputeWorkgroupCount(VelloStageClipLeaf, tc.clips)
		if got != tc.want {
			t.Errorf("ComputeWorkgroupCount(ClipLeaf, %d) = %d, want %d", tc.clips, got, tc.want)
		}
	}
}

// TestStageBindGroupLayoutEntries_ClipLeaf verifies the clip_leaf bind group:
// config uniform + clip_inp (RO) + draw_monoids (RW).
func TestStageBindGroupLayoutEntries_ClipLeaf(t *testing.T) {
	entries := stageBindGroupLayoutEntries(VelloStageClipLeaf)
	if len(entries) != 3 {
		t.Fatalf("clip_leaf bind group entries = %d, want 3", len(entries))
	}
	if entries[0].Binding != 0 || entries[0].Buffer == nil ||
		entries[0].Buffer.Type != types.BufferBindingTypeUniform {
		t.Error("binding(0) must be the config uniform buffer")
	}
	if entries[1].Binding != 1 || entries[1].Buffer == nil ||
		entries[1].Buffer.Type != types.BufferBindingTypeReadOnlyStorage {
		t.Error("binding(1) must be read-only storage (clip_inp)")
	}
	if entries[2].Binding != 2 || entries[2].Buffer == nil ||
		entries[2].Buffer.Type != types.BufferBindingTypeStorage {
		t.Error("binding(2) must be read-write storage (draw_monoids fixup)")
	}
}

// TestStageBindGroupLayoutEntries_DrawLeafHasClipInp verifies draw_leaf gained
// binding(5) = clip_inp so it can emit clip input data.
func TestStageBindGroupLayoutEntries_DrawLeafHasClipInp(t *testing.T) {
	entries := stageBindGroupLayoutEntries(VelloStageDrawLeaf)
	if len(entries) != 6 {
		t.Fatalf("draw_leaf bind group entries = %d, want 6 (incl. clip_inp)", len(entries))
	}
	if entries[5].Binding != 5 || entries[5].Buffer == nil ||
		entries[5].Buffer.Type != types.BufferBindingTypeStorage {
		t.Error("draw_leaf binding(5) must be read-write storage (clip_inp)")
	}
}

// TestComputeBufferSizes_ClipInp verifies the ClipInp buffer is sized at
// n_clip * 2 u32 (8 bytes per ClipInp: ix + path_ix), with a minimum of one
// struct: WebGPU bind group validation requires the bound buffer size >= the
// shader's min binding size (8 bytes) even for zero-clip scenes.
func TestComputeBufferSizes_ClipInp(t *testing.T) {
	d := &VelloComputeDispatcher{wgSize: velloWGSize}
	cfg := VelloComputeConfig{
		WidthInTiles:  4,
		HeightInTiles: 4,
		NumDrawObj:    5,
		NumPaths:      5,
		NumClips:      2,
		DrawTagBase:   256,
		PathTagBase:   0,
		NumLines:      20,
	}
	sz := d.computeBufferSizes(cfg, 300, 100, 25, 20, 64)
	if sz.clipInp != 2*2*4 {
		t.Errorf("clipInp buffer size = %d, want %d", sz.clipInp, 2*2*4)
	}
	// A scene without clips must still satisfy the 8-byte min binding size.
	cfg.NumClips = 0
	sz = d.computeBufferSizes(cfg, 300, 100, 25, 20, 64)
	if sz.clipInp != 8 {
		t.Errorf("clipInp buffer size with n_clip=0 = %d, want 8 (min binding size)", sz.clipInp)
	}
}

// clipMetadataElements builds the clip scene (draw -> begin clip -> draw ->
// end clip -> even-odd star) plus the allLines/metaPaths arrays built exactly
// like dispatchComputeSceneDef does on the accelerator side.
func clipMetadataElements(size float32) ([]tilecompute.SceneElement, []tilecompute.LineSoup, []tilecompute.PathDef) {
	elements := []tilecompute.SceneElement{
		// Green full-canvas background.
		{
			Type:     tilecompute.ElementDraw,
			Lines:    computeSquareLines(0, 0, size, size),
			Color:    [4]uint8{60, 180, 60, 255},
			FillRule: tilecompute.FillRuleNonZero,
		},
		// BeginClip: center rectangle.
		{
			Type:      tilecompute.ElementBeginClip,
			Lines:     computeSquareLines(size/4, size/4, 3*size/4, 3*size/4),
			BlendMode: 0x8003,
			Alpha:     1.0,
		},
		// Red full-canvas rect (visually clipped to the center).
		{
			Type:     tilecompute.ElementDraw,
			Lines:    computeSquareLines(0, 0, size, size),
			Color:    [4]uint8{220, 40, 40, 220},
			FillRule: tilecompute.FillRuleNonZero,
		},
		// EndClip.
		{Type: tilecompute.ElementEndClip},
		// Even-odd star-like polygon outside the clip (fully visible).
		{
			Type:     tilecompute.ElementDraw,
			Lines:    computeStarLines(),
			Color:    [4]uint8{230, 200, 0, 255},
			FillRule: tilecompute.FillRuleEvenOdd,
		},
	}

	var allLines []tilecompute.LineSoup
	var metaPaths []tilecompute.PathDef
	for pathIdx, el := range elements {
		switch el.Type {
		case tilecompute.ElementDraw, tilecompute.ElementBeginClip:
			for _, line := range el.Lines {
				allLines = append(allLines, tilecompute.LineSoup{
					PathIx: uint32(pathIdx),
					P0:     line.P0,
					P1:     line.P1,
				})
			}
			metaPaths = append(metaPaths, tilecompute.PathDef{
				Lines:    el.Lines,
				Color:    el.Color,
				FillRule: el.FillRule,
			})
		case tilecompute.ElementEndClip:
			metaPaths = append(metaPaths, tilecompute.PathDef{})
		}
	}
	return elements, allLines, metaPaths
}

// TestBuildPathMetadata_ClipScene verifies the clip-aware path metadata:
// - path count matches the encoded scene (EndClip dummy path included)
// - EndClip dummy path has a zero bbox and consumes no tiles
// - fill rule style bits are preserved per real path
func TestBuildPathMetadata_ClipScene(t *testing.T) {
	const sizePx = 64
	elements, allLines, metaPaths := clipMetadataElements(sizePx)

	enc := tilecompute.EncodeSceneDef(elements)
	scene := tilecompute.PackScene(enc)

	if scene.Layout.NumPaths != uint32(len(metaPaths)) {
		t.Fatalf("NumPaths = %d, metaPaths = %d", scene.Layout.NumPaths, len(metaPaths))
	}
	if scene.Layout.NumDrawObjects != 5 {
		t.Errorf("NumDrawObjects = %d, want 5", scene.Layout.NumDrawObjects)
	}
	if scene.Layout.NumClips != 2 {
		t.Errorf("NumClips = %d, want 2", scene.Layout.NumClips)
	}

	widthInTiles := uint32((sizePx + tilecompute.TileWidth - 1) / tilecompute.TileWidth)
	heightInTiles := uint32((sizePx + tilecompute.TileHeight - 1) / tilecompute.TileHeight)
	pathsU32, stylesU32, _ := buildPathMetadata(
		metaPaths, allLines, sizePx, sizePx, widthInTiles, heightInTiles,
	)

	if len(pathsU32) != len(metaPaths)*5 {
		t.Fatalf("pathsU32 len = %d, want %d", len(pathsU32), len(metaPaths)*5)
	}
	if len(stylesU32) != len(metaPaths) {
		t.Fatalf("stylesU32 len = %d, want %d", len(stylesU32), len(metaPaths))
	}

	// EndClip dummy path (index 3) must have a zero bbox.
	endOff := 3 * 5
	for i := 0; i < 4; i++ {
		if pathsU32[endOff+i] != 0 {
			t.Errorf("EndClip dummy path bbox[%d] = %d, want 0", i, pathsU32[endOff+i])
		}
	}

	// Fill rule bits: elements 0-3 are non-zero (bit 1 clear), star (index 4) is even-odd.
	for i := 0; i < 4; i++ {
		if stylesU32[i]&0x02 != 0 {
			t.Errorf("path %d style has even-odd bit set, want non-zero", i)
		}
	}
	if stylesU32[4]&0x02 == 0 {
		t.Error("star path style must have even-odd bit set (bit 1)")
	}

	// The EndClip dummy path must not consume tile slots: its tile offset
	// must equal the star's offset (the running offset does not advance
	// between the dummy path and the next real path).
	if pathsU32[3*5+4] != pathsU32[4*5+4] {
		t.Errorf("EndClip dummy path advanced the tile offset: dummy=%d star=%d (must be equal)",
			pathsU32[3*5+4], pathsU32[4*5+4])
	}
}