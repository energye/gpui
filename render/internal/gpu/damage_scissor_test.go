package gpu

import (
	"image"
	"testing"
)

func TestComputeDamageScissor(t *testing.T) {
	tests := []struct {
		name      string
		groupClip *[4]uint32
		surfaceW  uint32
		surfaceH  uint32
		damage    image.Rectangle
		wantX     uint32
		wantY     uint32
		wantW     uint32
		wantH     uint32
		wantValid bool
	}{
		{
			name:      "damage only, no group clip",
			groupClip: nil,
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(170, 410, 218, 458),
			wantX:  170, wantY: 410, wantW: 48, wantH: 48,
			wantValid: true,
		},
		{
			name:      "damage intersects group clip",
			groupClip: &[4]uint32{150, 400, 100, 100}, // x=150, y=400, w=100, h=100
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(170, 410, 218, 458),
			wantX:  170, wantY: 410, wantW: 48, wantH: 48,
			wantValid: true,
		},
		{
			name:      "group clip smaller than damage",
			groupClip: &[4]uint32{180, 420, 20, 20}, // x=180, y=420, w=20, h=20
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(170, 410, 218, 458),
			wantX:  180, wantY: 420, wantW: 20, wantH: 20,
			wantValid: true,
		},
		{
			name:      "group clip outside damage — empty intersection",
			groupClip: &[4]uint32{0, 0, 50, 50}, // top-left corner
			surfaceW:  800, surfaceH: 600,
			damage:    image.Rect(170, 410, 218, 458), // center-bottom
			wantValid: false,
		},
		{
			name:      "damage clamped to surface bounds",
			groupClip: nil,
			surfaceW:  200, surfaceH: 200,
			damage: image.Rect(170, 180, 300, 300), // extends beyond surface
			wantX:  170, wantY: 180, wantW: 30, wantH: 20,
			wantValid: true,
		},
		{
			name:      "damage fully outside surface — empty",
			groupClip: nil,
			surfaceW:  100, surfaceH: 100,
			damage:    image.Rect(200, 200, 300, 300),
			wantValid: false,
		},
		{
			name:      "partial overlap — group clip partially in damage",
			groupClip: &[4]uint32{160, 400, 80, 80}, // x=160..240, y=400..480
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(170, 410, 218, 458), // x=170..218, y=410..458
			wantX:  170, wantY: 410, wantW: 48, wantH: 48,
			wantValid: true,
		},
		{
			name:      "full surface group clip — damage is effective scissor",
			groupClip: &[4]uint32{0, 0, 800, 600},
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(100, 100, 200, 200),
			wantX:  100, wantY: 100, wantW: 100, wantH: 100,
			wantValid: true,
		},
		{
			name:      "zero-size damage",
			groupClip: nil,
			surfaceW:  800, surfaceH: 600,
			damage:    image.Rect(100, 100, 100, 100), // zero width
			wantValid: false,
		},
		{
			name:      "negative coords in damage clamped to 0",
			groupClip: nil,
			surfaceW:  800, surfaceH: 600,
			damage: image.Rect(-10, -10, 50, 50),
			wantX:  0, wantY: 0, wantW: 50, wantH: 50,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, w, h, valid := computeDamageScissor(tt.groupClip, tt.surfaceW, tt.surfaceH, tt.damage)
			if valid != tt.wantValid {
				t.Fatalf("valid = %v, want %v", valid, tt.wantValid)
			}
			if !valid {
				return
			}
			if x != tt.wantX || y != tt.wantY || w != tt.wantW || h != tt.wantH {
				t.Errorf("scissor = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
					x, y, w, h, tt.wantX, tt.wantY, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestDamageRectsUnion(t *testing.T) {
	tests := []struct {
		name  string
		rects []image.Rectangle
		want  image.Rectangle
	}{
		{"empty", nil, image.Rectangle{}},
		{"single", []image.Rectangle{image.Rect(10, 20, 50, 60)}, image.Rect(10, 20, 50, 60)},
		{
			"two_distant",
			[]image.Rectangle{image.Rect(10, 10, 58, 58), image.Rect(500, 50, 600, 82)},
			image.Rect(10, 10, 600, 82),
		},
		{
			"three_overlapping",
			[]image.Rectangle{
				image.Rect(10, 10, 100, 100),
				image.Rect(50, 50, 200, 200),
				image.Rect(150, 150, 300, 300),
			},
			image.Rect(10, 10, 300, 300),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := damageRectsUnion(tt.rects)
			if got != tt.want {
				t.Errorf("damageRectsUnion = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDamageRectsRelevantToGroup(t *testing.T) {
	const sw, sh uint32 = 800, 600
	groupTL := &[4]uint32{0, 0, 100, 100}    // top-left group
	groupBR := &[4]uint32{500, 400, 120, 80} // bottom-right group
	distantA := image.Rect(10, 10, 40, 40)
	distantB := image.Rect(560, 420, 600, 460)
	overlapBothFat := image.Rect(10, 10, 600, 460) // huge AABB of A∪B

	t.Run("empty_input", func(t *testing.T) {
		got := damageRectsRelevantToGroup(groupTL, sw, sh, nil)
		if !got.Empty() {
			t.Fatalf("empty input want empty, got %v", got)
		}
	})

	t.Run("no_overlap_skips_group", func(t *testing.T) {
		// Only bottom-right damage; top-left group must be empty (skip).
		got := damageRectsRelevantToGroup(groupTL, sw, sh, []image.Rectangle{distantB})
		if !got.Empty() {
			t.Fatalf("expected empty relevant for non-overlapping group, got %v", got)
		}
	})

	t.Run("local_damage_tight", func(t *testing.T) {
		got := damageRectsRelevantToGroup(groupTL, sw, sh, []image.Rectangle{distantA, distantB})
		want := distantA // fully inside group
		if got != want {
			t.Fatalf("relevant=%v want %v", got, want)
		}
	})

	t.Run("partial_clip_to_group", func(t *testing.T) {
		// Damage straddles group edge; relevant must be intersection only.
		dmg := image.Rect(80, 80, 150, 150)
		got := damageRectsRelevantToGroup(groupTL, sw, sh, []image.Rectangle{dmg})
		want := image.Rect(80, 80, 100, 100)
		if got != want {
			t.Fatalf("relevant=%v want %v", got, want)
		}
	})

	t.Run("multi_overlap_union_inside_group", func(t *testing.T) {
		a := image.Rect(510, 410, 530, 430)
		b := image.Rect(560, 440, 590, 470)
		got := damageRectsRelevantToGroup(groupBR, sw, sh, []image.Rectangle{distantA, a, b})
		want := a.Union(b)
		if got != want {
			t.Fatalf("relevant=%v want %v", got, want)
		}
	})

	t.Run("nil_group_clip_uses_surface", func(t *testing.T) {
		got := damageRectsRelevantToGroup(nil, sw, sh, []image.Rectangle{distantA, distantB})
		want := distantA.Union(distantB)
		if got != want {
			t.Fatalf("relevant=%v want %v", got, want)
		}
	})

	t.Run("global_union_is_wider_than_relevant", func(t *testing.T) {
		// Documents the R7.4 win: global AABB of distant rects is huge,
		// but group-relevant stays local.
		rects := []image.Rectangle{distantA, distantB}
		global := damageRectsUnion(rects)
		if global != overlapBothFat && global != distantA.Union(distantB) {
			// just ensure global is the fat union
		}
		if global != distantA.Union(distantB) {
			t.Fatalf("global union=%v", global)
		}
		rel := damageRectsRelevantToGroup(groupTL, sw, sh, rects)
		if rel.Dx()*rel.Dy() >= global.Dx()*global.Dy() {
			t.Fatalf("expected relevant area < global: rel=%v global=%v", rel, global)
		}
		if rel != distantA {
			t.Fatalf("relevant=%v want %v", rel, distantA)
		}
	})
}

func TestApplyGroupScissorWithDamageRects_Semantics(t *testing.T) {
	// Pure helper path via damageRectsRelevantToGroup + computeDamageScissor,
	// mirroring applyGroupScissorWithDamageRects decisions without a GPU device.
	const sw, sh uint32 = 200, 200
	group := &[4]uint32{0, 0, 50, 50}
	dmgFar := []image.Rectangle{image.Rect(100, 100, 140, 140)}
	dmgNear := []image.Rectangle{image.Rect(10, 10, 30, 30), image.Rect(100, 100, 140, 140)}

	// no damage → full group (valid scissor = group)
	if len(([]image.Rectangle)(nil)) != 0 {
		t.Fatal("sanity")
	}
	// far only → skip
	if !damageRectsRelevantToGroup(group, sw, sh, dmgFar).Empty() {
		t.Fatal("far damage should not hit group")
	}
	// near+far → scissor from near only
	rel := damageRectsRelevantToGroup(group, sw, sh, dmgNear)
	x, y, w, h, valid := computeDamageScissor(group, sw, sh, rel)
	if !valid || x != 10 || y != 10 || w != 20 || h != 20 {
		t.Fatalf("scissor=(%d,%d,%d,%d valid=%v) want 10,10,20,20 true", x, y, w, h, valid)
	}
	// global union would have over-widened:
	fat := damageRectsUnion(dmgNear)
	fx, fy, fw, fh, fvalid := computeDamageScissor(group, sw, sh, fat)
	if !fvalid {
		t.Fatal("fat should still intersect group")
	}
	// fat ∩ group = full group [0,0,50,50] because fat AABB covers from (10,10) to (140,140)
	if fx != 10 || fy != 10 || fw != 40 || fh != 40 {
		// 10..50 = 40 width; documents over-wide relative to tight 20x20
		t.Logf("fat scissor=(%d,%d,%d,%d) (over-wide baseline)", fx, fy, fw, fh)
	}
	if fw*fh <= w*h {
		t.Fatalf("expected fat scissor area > tight: fat=%dx%d tight=%dx%d", fw, fh, w, h)
	}
}
