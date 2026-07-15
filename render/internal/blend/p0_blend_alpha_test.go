package blend

import "testing"

// P0.3: fixed-pixel calibration for UI-critical blend modes.
// All blend operators in this package take premultiplied alpha inputs.

func TestP03FixedPixels_UICriticalModes(t *testing.T) {
	type px struct{ r, g, b, a byte }
	type caseT struct {
		name string
		mode BlendMode
		src  px
		dst  px
		want px
	}

	// Premul fixtures:
	//  - opaque red / blue / white / black
	//  - 50% gray premul: straight (128,128,128,128) already is premul of (255,255,255,128)?
	//    For straight gray 255 with a=128: premul rgb = 128. So (128,128,128,128) is premul white@50%.
	cases := []caseT{
		// SourceOver / Normal equivalence target
		{
			name: "SourceOver opaque red over opaque blue",
			mode: BlendSourceOver,
			src:  px{255, 0, 0, 255},
			dst:  px{0, 0, 255, 255},
			want: px{255, 0, 0, 255},
		},
		{
			name: "SourceOver transparent over opaque white",
			mode: BlendSourceOver,
			src:  px{0, 0, 0, 0},
			dst:  px{255, 255, 255, 255},
			want: px{255, 255, 255, 255},
		},
		{
			name: "SourceOver premul white@50 over opaque white",
			mode: BlendSourceOver,
			src:  px{128, 128, 128, 128},
			dst:  px{255, 255, 255, 255},
			// S + D*(1-Sa) = 128 + 255*(127/255) ≈ 128+127 = 255
			want: px{255, 255, 255, 255},
		},
		// Copy ≡ Source (replace)
		{
			name: "Source(Copy) replaces destination",
			mode: BlendSource,
			src:  px{10, 20, 30, 40},
			dst:  px{200, 210, 220, 230},
			want: px{10, 20, 30, 40},
		},
		// Plus
		{
			name: "Plus clamps channel sum",
			mode: BlendPlus,
			src:  px{200, 10, 0, 200},
			dst:  px{200, 20, 5, 200},
			want: px{255, 30, 5, 255},
		},
		{
			name: "Plus transparent is identity on dest",
			mode: BlendPlus,
			src:  px{0, 0, 0, 0},
			dst:  px{50, 60, 70, 80},
			want: px{50, 60, 70, 80},
		},
		// Multiply (advanced separable, opaque)
		{
			name: "Multiply white * gray stays gray",
			mode: BlendMultiply,
			src:  px{255, 255, 255, 255},
			dst:  px{128, 128, 128, 255},
			want: px{128, 128, 128, 255},
		},
		{
			name: "Multiply black * white = black",
			mode: BlendMultiply,
			src:  px{0, 0, 0, 255},
			dst:  px{255, 255, 255, 255},
			want: px{0, 0, 0, 255},
		},
		{
			name: "Multiply gray * gray",
			mode: BlendMultiply,
			src:  px{128, 128, 128, 255},
			dst:  px{128, 128, 128, 255},
			// 128*128/255 ≈ 64 with div255 approximation
			want: px{64, 64, 64, 255},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := GetBlendFunc(tc.mode)
			r, g, b, a := fn(tc.src.r, tc.src.g, tc.src.b, tc.src.a, tc.dst.r, tc.dst.g, tc.dst.b, tc.dst.a)
			if r != tc.want.r || g != tc.want.g || b != tc.want.b || a != tc.want.a {
				t.Fatalf("%v = rgba(%d,%d,%d,%d), want rgba(%d,%d,%d,%d)",
					tc.mode, r, g, b, a, tc.want.r, tc.want.g, tc.want.b, tc.want.a)
			}
		})
	}
}

func TestP03PremulVsStraightAlphaBoundary(t *testing.T) {
	// Boundary contract used by render/GPU:
	// 1) blend package operators assume premultiplied inputs.
	// 2) straight alpha must be converted before blending:
	//      premul.rgb = straight.rgb * a / 255; premul.a = a
	// 3) feeding straight colors into SourceOver under-composites RGB.

	// Straight red @ 50% alpha: (255,0,0,128)
	// Correct premul: (128,0,0,128) approximately (255*128/255 = 128)
	straightR, straightG, straightB, straightA := byte(255), byte(0), byte(0), byte(128)
	premulR := mulDiv255(straightR, straightA)
	premulG := mulDiv255(straightG, straightA)
	premulB := mulDiv255(straightB, straightA)
	premulA := straightA

	if premulR != 128 || premulG != 0 || premulB != 0 || premulA != 128 {
		t.Fatalf("premul conversion = (%d,%d,%d,%d), want (128,0,0,128)", premulR, premulG, premulB, premulA)
	}

	dstR, dstG, dstB, dstA := byte(0), byte(0), byte(255), byte(255) // opaque blue

	// Correct path: premul source over blue
	cr, cg, cb, ca := blendSourceOver(premulR, premulG, premulB, premulA, dstR, dstG, dstB, dstA)
	// Wrong path: feed straight source
	wr, wg, wb, wa := blendSourceOver(straightR, straightG, straightB, straightA, dstR, dstG, dstB, dstA)

	// Correct result should be closer to half-red over blue than the wrong path.
	// Premul SO: (128,0,0,128) over (0,0,255,255)
	//   invSa=127
	//   r = 128 + 0*127/255 = 128
	//   g = 0
	//   b = 0 + 255*127/255 ≈ 127
	//   a = 128 + 255*127/255 ≈ 255
	if cr != 128 || cg != 0 {
		t.Fatalf("correct premul SO rgb = (%d,%d,%d), want r=128 g=0", cr, cg, cb)
	}
	if cb < 120 || cb > 130 {
		t.Fatalf("correct premul SO blue channel = %d, want ~127", cb)
	}
	if ca < 250 {
		t.Fatalf("correct premul SO alpha = %d, want ~255", ca)
	}

	// Wrong straight path inflates red (uses 255 as premul red)
	if wr <= cr {
		t.Fatalf("straight-fed SO red=%d should exceed premul-fed red=%d (documents the bug class)", wr, cr)
	}
	_ = wg
	_ = wb
	_ = wa
}

func TestP03NormalIsSourceOverContract(t *testing.T) {
	// Document that "Normal" at the scene/UI layer is SourceOver here.
	// Internal package uses BlendSourceOver; scene.BlendNormal maps to it.
	src := [4]byte{100, 50, 25, 200}
	dst := [4]byte{10, 20, 30, 40}
	r1, g1, b1, a1 := blendSourceOver(src[0], src[1], src[2], src[3], dst[0], dst[1], dst[2], dst[3])
	r2, g2, b2, a2 := GetBlendFunc(BlendSourceOver)(src[0], src[1], src[2], src[3], dst[0], dst[1], dst[2], dst[3])
	if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
		t.Fatalf("GetBlendFunc(SourceOver) mismatch")
	}
}
