//go:build linux && !(js && wasm)

package gles

import (
	"image"
	"testing"
)

// TestSpinBlitDamageScissorGL locks the top-left → bottom-left scissor math
// for the damage-scissored present blit (B-part2). Pure logic, no GL/EGL.
func TestSpinBlitDamageScissorGL(t *testing.T) {
	const surfH = int32(320)
	cases := []struct {
		name       string
		in         image.Rectangle
		wantX      int32
		wantY      int32
		wantW      int32
		wantH      int32
	}{
		// 480x320 surface, damage 66x66-ish box near center.
		{name: "center", in: image.Rect(207, 127, 273, 193), wantX: 207, wantY: 320 - 193, wantW: 66, wantH: 66},
		// top-left corner stays top-left after flip.
		{name: "topleft", in: image.Rect(0, 0, 10, 10), wantX: 0, wantY: 310, wantW: 10, wantH: 10},
		// bottom edge.
		{name: "bottom", in: image.Rect(0, 310, 480, 320), wantX: 0, wantY: 0, wantW: 480, wantH: 10},
		// fully outside clamps to empty.
		{name: "outside", in: image.Rect(-50, -50, -10, -10), wantX: 0, wantY: 320, wantW: 0, wantH: 0},
		// partially outside clamps origin, keeps size math sane.
		{name: "clip-left", in: image.Rect(-5, 100, 20, 120), wantX: 0, wantY: 200, wantW: 20, wantH: 20},
	}
	for _, c := range cases {
		x, y, w, h := damageScissorGL(c.in, surfH)
		if x != c.wantX || y != c.wantY || w != c.wantW || h != c.wantH {
			t.Errorf("%s: got (%d,%d,%d,%d) want (%d,%d,%d,%d)",
				c.name, x, y, w, h, c.wantX, c.wantY, c.wantW, c.wantH)
		}
	}
}
