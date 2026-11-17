//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import "image"

// computeDamageScissor computes the effective scissor rect as the intersection
// of group clip and frame damage rect, clamped to surface bounds.
// Returns (x, y, w, h, valid). When valid=false, the intersection is empty
// and all fragments should be discarded.
func computeDamageScissor(groupClip *[4]uint32, surfaceW, surfaceH uint32, damage image.Rectangle) (x, y, w, h uint32, valid bool) {
	sx, sy := damage.Min.X, damage.Min.Y
	sx2, sy2 := damage.Max.X, damage.Max.Y

	if groupClip != nil {
		gx := int(groupClip[0])
		gy := int(groupClip[1])
		gx2 := gx + int(groupClip[2])
		gy2 := gy + int(groupClip[3])
		sx = max(sx, gx)
		sy = max(sy, gy)
		sx2 = min(sx2, gx2)
		sy2 = min(sy2, gy2)
	}

	sx = max(sx, 0)
	sy = max(sy, 0)
	sx2 = min(sx2, int(surfaceW))
	sy2 = min(sy2, int(surfaceH))

	if sx2 <= sx || sy2 <= sy {
		return 0, 0, 0, 0, false
	}
	return uint32(sx), uint32(sy), uint32(sx2 - sx), uint32(sy2 - sy), true //nolint:gosec // clamped above
}

// damageRectsUnion returns the bounding box of all damage rects.
// Returns empty rect if slice is empty.
func damageRectsUnion(rects []image.Rectangle) image.Rectangle {
	if len(rects) == 0 {
		return image.Rectangle{}
	}
	u := rects[0]
	for _, r := range rects[1:] {
		u = u.Union(r)
	}
	return u
}

// damageRectsRelevantToGroup returns the union of damage rects that intersect
// the group clip region (clamped to the surface). Distant multi-rect damage
// that does not touch this group is excluded, so group scissor stays tight.
//
// Returns an empty rect when rects is empty, or when no damage rect overlaps
// the group. Callers must distinguish:
//   - empty input → no damage tracking → full group scissor
//   - non-empty in, empty out → damage present but group clean → skip group
func damageRectsRelevantToGroup(groupClip *[4]uint32, surfaceW, surfaceH uint32, rects []image.Rectangle) image.Rectangle {
	if len(rects) == 0 {
		return image.Rectangle{}
	}

	gx, gy := 0, 0
	gx2, gy2 := int(surfaceW), int(surfaceH)
	if groupClip != nil {
		gx = int(groupClip[0])
		gy = int(groupClip[1])
		gx2 = gx + int(groupClip[2])
		gy2 = gy + int(groupClip[3])
	}
	gx = max(gx, 0)
	gy = max(gy, 0)
	gx2 = min(gx2, int(surfaceW))
	gy2 = min(gy2, int(surfaceH))
	if gx2 <= gx || gy2 <= gy {
		return image.Rectangle{}
	}
	group := image.Rect(gx, gy, gx2, gy2)

	var u image.Rectangle
	any := false
	for _, r := range rects {
		if r.Empty() {
			continue
		}
		inter := r.Intersect(group)
		if inter.Empty() {
			continue
		}
		if !any {
			u = inter
			any = true
			continue
		}
		u = u.Union(inter)
	}
	return u
}

// damageRectsForGroup returns the individual damage rects that intersect the
// group clip, clamped to the surface, with overlapping rects merged. Unlike
// damageRectsRelevantToGroup (a single AABB), multi-rect damage keeps its
// gaps: scissoring a group to the AABB repaints non-damaged content sitting
// between distant rects with the opaque background, erasing static pixels
// (P6 hover transition erased a settled card between two damage rects).
func damageRectsForGroup(groupClip *[4]uint32, surfaceW, surfaceH uint32, rects []image.Rectangle) []image.Rectangle {
	if len(rects) == 0 {
		return nil
	}
	out := make([]image.Rectangle, 0, len(rects))
	for _, r := range rects {
		if r.Empty() {
			continue
		}
		x, y, w, h, ok := computeDamageScissor(groupClip, surfaceW, surfaceH, r)
		if !ok {
			continue
		}
		dr := image.Rect(int(x), int(y), int(x)+int(w), int(y)+int(h))
		merged := false
		for i := range out {
			if out[i].Overlaps(dr) {
				out[i] = out[i].Union(dr)
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, dr)
		}
	}
	// A union may now overlap an earlier merged rect; converge.
	for changed := true; changed; {
		changed = false
		for i := 0; i < len(out) && !changed; i++ {
			for j := i + 1; j < len(out); j++ {
				if out[i].Overlaps(out[j]) {
					out[i] = out[i].Union(out[j])
					out = append(out[:j], out[j+1:]...)
					changed = true
					break
				}
			}
		}
	}
	return out
}
