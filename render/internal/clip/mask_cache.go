//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package clip

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
	"math"
)

// Mask kinds distinguish caches that use the same path but different
// operations (plain intersection vs. difference).
const (
	maskKindPath byte = iota + 1
	maskKindPathDifference
	maskKindRectDifference
)

// maskPathKey hashes a path clip into a cache key. The mask depends on the
// path geometry, the current canvas bounds (MaskClipper allocates a
// full-canvas buffer), AA and fill rule. FNV-1a is cheap compared with
// rasterizing a canvas-sized mask.
func maskPathKey(
	verbs []PathVerb,
	coords []float64,
	bounds Rect,
	antiAlias bool,
	fillRule FillRule,
	kind byte,
) uint64 {
	h := fnv.New64a()
	h.Write([]byte{kind})
	if antiAlias {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	writeU64(h, uint64(fillRule))
	writeRect(h, bounds)
	writeU64(h, uint64(len(verbs)))
	for _, v := range verbs {
		h.Write([]byte{byte(v)})
	}
	writeU64(h, uint64(len(coords)))
	for _, c := range coords {
		writeU64(h, math.Float64bits(c))
	}
	return h.Sum64()
}

// maskRectDifferenceKey hashes the rectangle-difference mask inputs.
func maskRectDifferenceKey(bounds, hole Rect) uint64 {
	h := fnv.New64a()
	h.Write([]byte{maskKindRectDifference})
	writeRect(h, bounds)
	writeRect(h, hole)
	return h.Sum64()
}

func writeU64(h hash.Hash, v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	h.Write(b[:])
}

func writeRect(h hash.Hash, r Rect) {
	writeU64(h, math.Float64bits(r.X))
	writeU64(h, math.Float64bits(r.Y))
	writeU64(h, math.Float64bits(r.W))
	writeU64(h, math.Float64bits(r.H))
}

// Signature returns a content hash of the effective clip stack. Unlike a
// mutation counter, it stays equal across push/pop cycles that reconstruct the
// same clip, so an already-built full-surface clip plane can be reused while
// replaying many pictures under one viewport clip.
func (cs *ClipStack) Signature() uint64 {
	h := fnv.New64a()
	writeRect(h, cs.bounds)
	writeU64(h, uint64(len(cs.entries)))
	for i := range cs.entries {
		entry := &cs.entries[i]
		writeRect(h, entry.prevBounds)
		if entry.antiAlias {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
		switch {
		case entry.rrect != nil:
			h.Write([]byte{2})
			writeRect(h, entry.rrect.Rect)
			writeU64(h, math.Float64bits(entry.rrect.Radius))
		case entry.mask != nil:
			h.Write([]byte{3})
			writeU64(h, entry.maskKey)
		default:
			h.Write([]byte{1})
		}
	}
	return h.Sum64()
}

// maskCacheMaxEntries bounds retained full-canvas masks (LRU). R7 needs one;
// a handful covers layered clips without pinning unreasonable memory.
const maskCacheMaxEntries = 8

// lookupMask returns a cached mask and marks it most-recently used.
func (cs *ClipStack) lookupMask(key uint64) *MaskClipper {
	if cs == nil || cs.maskCache == nil {
		return nil
	}
	mask := cs.maskCache[key]
	if mask == nil {
		return nil
	}
	for i, k := range cs.maskCacheOrder {
		if k != key {
			continue
		}
		if i != len(cs.maskCacheOrder)-1 {
			copy(cs.maskCacheOrder[i:], cs.maskCacheOrder[i+1:])
			cs.maskCacheOrder[len(cs.maskCacheOrder)-1] = key
		}
		break
	}
	return mask
}

// storeMask adds a mask to the LRU cache, evicting the oldest entry when full.
func (cs *ClipStack) storeMask(key uint64, mask *MaskClipper) {
	if cs == nil || mask == nil {
		return
	}
	if cs.maskCache == nil {
		cs.maskCache = make(map[uint64]*MaskClipper, maskCacheMaxEntries)
	}
	if _, ok := cs.maskCache[key]; ok {
		return
	}
	if len(cs.maskCacheOrder) >= maskCacheMaxEntries {
		oldest := cs.maskCacheOrder[0]
		cs.maskCacheOrder = cs.maskCacheOrder[1:]
		delete(cs.maskCache, oldest)
	}
	cs.maskCache[key] = mask
	cs.maskCacheOrder = append(cs.maskCacheOrder, key)
}
