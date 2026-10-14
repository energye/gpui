//go:build !nogpu

package gpu

import (
	"bytes"
	"math"
	"testing"
)

func rigidCorners(cx, cy, w, h, angle float64) (tlx, tly, trx, try_, brx, bry, blx, bly float32) {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	mk := func(lx, ly float64) (float32, float32) {
		px := cos*(lx-cx) - sin*(ly-cy) + cx
		py := sin*(lx-cx) + cos*(ly-cy) + cy
		// Note: render.Rotate uses x'=cos*x - sin*y, y'=sin*x + cos*y,
		// identical to the uniform composer above.
		return float32(px), float32(py)
	}
	x0, y0 := cx-w/2, cy-h/2
	x1, y1 := cx+w/2, cy+h/2
	tlx, tly = mk(x0, y0)
	trx, try_ = mk(x1, y0)
	brx, bry = mk(x1, y1)
	blx, bly = mk(x0, y1)
	return
}

func TestDetectRigidQuad_AxisAligned(t *testing.T) {
	info := detectRigidQuad(208, 128, 272, 128, 272, 192, 208, 192)
	if !info.ok {
		t.Fatal("axis-aligned rect should detect as rigid")
	}
	if math.Abs(float64(info.angle)) > 1e-6 {
		t.Fatalf("angle=%v want ~0", info.angle)
	}
	if math.Abs(float64(info.w-64)) > 1e-3 || math.Abs(float64(info.h-64)) > 1e-3 {
		t.Fatalf("size got %.4f x %.4f want 64 x 64", info.w, info.h)
	}
	if math.Abs(float64(info.cx-240)) > 1e-3 || math.Abs(float64(info.cy-160)) > 1e-3 {
		t.Fatalf("center got (%.4f,%.4f) want (240,160)", info.cx, info.cy)
	}
}

func TestDetectRigidQuad_Rotated(t *testing.T) {
	const cx, cy, w, h = 240.0, 160.0, 64.0, 64.0
	for _, deg := range []float64{15, 30, 45, 90, 137} {
		th := deg * math.Pi / 180
		tlx, tly, trx, try_, brx, bry, blx, bly := rigidCorners(cx, cy, w, h, th)
		info := detectRigidQuad(tlx, tly, trx, try_, brx, bry, blx, bly)
		if !info.ok {
			t.Fatalf("deg=%.0f: rotated rect should detect as rigid", deg)
		}
		if math.Abs(float64(info.w-64)) > 0.01 || math.Abs(float64(info.h-64)) > 0.01 {
			t.Fatalf("deg=%.0f: size got %.4f x %.4f", deg, info.w, info.h)
		}
		// Angle wraps at ±pi; compare via unit vectors.
		gotX, gotY := math.Cos(float64(info.angle)), math.Sin(float64(info.angle))
		wantX, wantY := math.Cos(th), math.Sin(th)
		if math.Abs(gotX-wantX) > 1e-4 || math.Abs(gotY-wantY) > 1e-4 {
			t.Fatalf("deg=%.0f: angle got %v want %v", deg, info.angle, th)
		}
	}
}

func TestDetectRigidQuad_ShearRejected(t *testing.T) {
	// Shear: shift top edge +8px in x, bottom stays. Not a rigid rotation.
	info := detectRigidQuad(208, 128, 280, 128, 272, 192, 208, 192)
	if info.ok {
		t.Fatal("sheared quad must NOT detect as rigid (fallback to baked vertices)")
	}
}

func TestRotationUniform_AngleZeroBitIdentical(t *testing.T) {
	oldU := makeImageUniform(480, 320, 1.0)
	newU := make([]byte, imageUniformSize)
	putImageRotationUniform(newU, 480, 320, 1.0, 240, 160, 0)
	if !bytes.Equal(oldU, newU) {
		t.Fatal("angle=0 rotation uniform must be bit-identical to ortho-only uniform")
	}
}

// TestRotationUniform_PixelEquivalence drives the real shipped path:
// baked corners through putImageUniform vs axis-aligned base through
// putImageRotationUniform must land on the same NDC within tolerance.
func TestRotationUniform_PixelEquivalence(t *testing.T) {
	const vpW, vpH = 480, 320
	const cx, cy, w, h = 240.0, 160.0, 64.0, 64.0
	for _, deg := range []float64{10, 30, 45, 120, 200} {
		th := deg * math.Pi / 180
		tlx, tly, trx, try_, brx, bry, blx, bly := rigidCorners(cx, cy, w, h, th)
		info := detectRigidQuad(tlx, tly, trx, try_, brx, bry, blx, bly)
		if !info.ok {
			t.Fatalf("deg=%.0f: rigid detect failed", deg)
		}
		oldU := makeImageUniform(vpW, vpH, 1.0)
		newU := make([]byte, imageUniformSize)
		putImageRotationUniform(newU, vpW, vpH, 1.0, info.cx, info.cy, info.angle)
		x0, y0, x1, y1 := info.axisAlignedBase()
		base := [4][2]float32{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
		baked := [4][2]float32{{tlx, tly}, {trx, try_}, {brx, bry}, {blx, bly}}
		for i := 0; i < 4; i++ {
			nx, ny := applyRotationUniform(newU, base[i][0], base[i][1])
			ox, oy := applyOrthoUniform(oldU, baked[i][0], baked[i][1])
			if math.Abs(float64(nx-ox)) > 2e-5 || math.Abs(float64(ny-oy)) > 2e-5 {
				t.Fatalf("deg=%.0f corner %d: new NDC (%.6f,%.6f) vs old (%.6f,%.6f)",
					deg, i, nx, ny, ox, oy)
			}
		}
	}
}

// TestRigidBase_JitterKillsHash documents the honest net-zero finding:
// deriving the base box from baked corners jitters ±1ulp across frames,
// so exact-byte vertex-hash skip never engages without upstream-stable
// center/size. This test locks the observation, not a fix.
func TestRigidBase_JitterKillsHash(t *testing.T) {
	const cx, cy, w, h = 240.0, 160.0, 64.0, 64.0
	th1 := 30.0 * math.Pi / 180
	// Simulate next-frame float32 rounding drift: 1ulp nudge on one corner.
	tlx, tly, trx, try_, brx, bry, blx, bly := rigidCorners(cx, cy, w, h, th1)
	info1 := detectRigidQuad(tlx, tly, trx, try_, brx, bry, blx, bly)
	if !info1.ok {
		t.Fatal("setup: frame1 must detect rigid")
	}
	x0a, y0a, x1a, y1a := info1.axisAlignedBase()
	// Nudge one baked corner by 1 float32 ulp and re-derive.
	trx2 := math.Float32frombits(math.Float32bits(trx) + 1)
	info2 := detectRigidQuad(tlx, tly, trx2, try_, brx, bry, blx, bly)
	if !info2.ok {
		t.Skip("1ulp nudge broke rigid detection; observation N/A on this platform")
	}
	x0b, y0b, x1b, y1b := info2.axisAlignedBase()
	same := x0a == x0b && y0a == y0b && x1a == x1b && y1a == y1b
	if same {
		t.Log("1ulp corner nudge did not move derived base on this platform")
		return
	}
	// Emit both bases as real shipped vertex buffers and fingerprint them.
	mkVerts := func(x0, y0, x1, y1 float32) []byte {
		cmd := &ImageDrawCommand{
			TLX: x0, TLY: y0, TRX: x1, TRY: y0,
			BRX: x1, BRY: y1, BLX: x0, BLY: y1,
			U0: 0, V0: 0, U1: 1, V1: 1,
		}
		return buildImageVertices(cmd)
	}
	va := mkVerts(x0a, y0a, x1a, y1a)
	vb := mkVerts(x0b, y0b, x1b, y1b)
	if bytes.Equal(va, vb) {
		t.Fatal("derived bases differ but vertex bytes identical: fingerprint would wrongly hit")
	}
	t.Logf("jitter moves base by dx0=%.7g dy0=%.7g; vertex bytes differ (%dB), hash skip misses",
		float64(x0b-x0a), float64(y0b-y0a), len(va))
}

// TestMergeKey_CurrentIgnoresCorners locks current shipped behavior:
// same-texture different-angle draws still merge today (corners live in
// vertices, not in the key). A future uniform-path wiring MUST fold the
// angle into the merge decision, or angles would freeze.
func TestMergeKey_CurrentIgnoresCorners(t *testing.T) {
	base := ImageDrawCommand{
		GenerationID: 7, Opacity: 1.0,
		ViewportWidth: 480, ViewportHeight: 320,
		ImgWidth: 64, ImgHeight: 64,
		U0: 0, V0: 0, U1: 1, V1: 1,
	}
	tlx1, tly1, trx1, try1, brx1, bry1, blx1, bly1 := rigidCorners(240, 160, 64, 64, 0.3)
	tlx2, tly2, trx2, try2, brx2, bry2, blx2, bly2 := rigidCorners(240, 160, 64, 64, 0.9)
	a := base
	a.TLX, a.TLY, a.TRX, a.TRY, a.BRX, a.BRY, a.BLX, a.BLY = tlx1, tly1, trx1, try1, brx1, bry1, blx1, bly1
	b := base
	b.TLX, b.TLY, b.TRX, b.TRY, b.BRX, b.BRY, b.BLX, b.BLY = tlx2, tly2, trx2, try2, brx2, bry2, blx2, bly2
	if !canMergeImageDraw(&a, &b) {
		t.Fatal("current canMergeImageDraw ignores corners: different-angle same-texture draws merge today")
	}
	ia := detectRigidQuad(a.TLX, a.TLY, a.TRX, a.TRY, a.BRX, a.BRY, a.BLX, a.BLY)
	ib := detectRigidQuad(b.TLX, b.TLY, b.TRX, b.TRY, b.BRX, b.BRY, b.BLX, b.BLY)
	if !ia.ok || !ib.ok {
		t.Fatal("setup: both quads must detect rigid")
	}
	if math.Abs(float64(ia.angle-ib.angle)) < 0.1 {
		t.Fatal("setup: test angles must differ")
	}
	t.Logf("current merge=TRUE across angles %.3f vs %.3f; uniform path must key the angle",
		ia.angle, ib.angle)
}

func BenchmarkRigidDetectAndCompose(b *testing.B) {
	tlx, tly, trx, try_, brx, bry, blx, bly := rigidCorners(240, 160, 64, 64, 0.5)
	u := make([]byte, imageUniformSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		info := detectRigidQuad(tlx, tly, trx, try_, brx, bry, blx, bly)
		if !info.ok {
			b.Fatal("must detect")
		}
		putImageRotationUniform(u, 480, 320, 1.0, info.cx, info.cy, info.angle)
	}
}

func BenchmarkBakedVertexEmit(b *testing.B) {
	tlx, tly, trx, try_, brx, bry, blx, bly := rigidCorners(240, 160, 64, 64, 0.5)
	cmd := &ImageDrawCommand{
		TLX: tlx, TLY: tly, TRX: trx, TRY: try_,
		BRX: brx, BRY: bry, BLX: blx, BLY: bly,
		U0: 0, V0: 0, U1: 1, V1: 1,
	}
	dst := make([]byte, 6*imageVertexStride)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildImageVerticesInto(dst, cmd)
	}
}
