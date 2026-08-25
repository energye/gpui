package rendering_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

func rgbAt(t *testing.T, img interface {
	At(x, y int) color.Color
}, x, y int) (r, g, b uint8) {
	t.Helper()
	c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
	return c.R, c.G, c.B
}

// TestRenderColorFilter_PaintAppliesGrayscale drives the shipped paint path:
// a red box under RenderGrayscale must come out neutral (R=G=B=luma), while
// the backdrop outside the filter box stays untouched.
func TestRenderColorFilter_PaintAppliesGrayscale(t *testing.T) {
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	box := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	cf := rendering.NewRenderGrayscale(box)
	root := rendering.NewAbsoluteBox(80, 80)
	root.Place(cf, 20, 20)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 80, Height: 80}, true)
	owner.FlushPaint(rendering.NewPaintContext(dc, 1), true)

	img := dc.Image()
	r, g, b := rgbAt(t, img, 40, 40)
	if r != g || g != b || r == 0 {
		t.Fatalf("grayscale must neutralize red to R=G=B, got #%02x%02x%02x", r, g, b)
	}
	// BT.601 luma of pure red ≈ 0.299·255 ≈ 76.
	if r < 70 || r > 82 {
		t.Fatalf("red luma want ~76, got %d", r)
	}
	wr, wg, wb := rgbAt(t, img, 5, 5)
	if wr != 255 || wg != 255 || wb != 255 {
		t.Fatalf("filter must not leak outside the subtree, got #%02x%02x%02x", wr, wg, wb)
	}
}

// TestRenderColorFilter_IdentityPaintsThrough: identity matrix paints the
// child unchanged with no layer.
func TestRenderColorFilter_IdentityPaintsThrough(t *testing.T) {
	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	var id [20]float32
	id[0], id[6], id[12], id[18] = 1, 1, 1, 1
	box := rendering.NewRenderColorBox(20, 20, 0, 0, 1, 1)
	cf := rendering.NewRenderColorFilter(id, box)
	root := rendering.NewAbsoluteBox(60, 60)
	root.Place(cf, 10, 10)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 60, Height: 60}, true)
	owner.FlushPaint(rendering.NewPaintContext(dc, 1), true)

	r, g, b := rgbAt(t, dc.Image(), 20, 20)
	if r != 0 || g != 0 || b != 255 {
		t.Fatalf("identity must paint through unchanged, got #%02x%02x%02x", r, g, b)
	}
}

// TestRenderImageFilter_PaintBlursPastBounds: a blurred box bleeds color just
// outside its layout bounds — proof the blur applied to the isolated group.
func TestRenderImageFilter_PaintBlursPastBounds(t *testing.T) {
	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	box := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	imf := rendering.NewRenderImageFilter(6, box)
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(imf, 30, 30)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	owner.FlushPaint(rendering.NewPaintContext(dc, 1), true)

	// Just outside the box edge (box spans x=30..70): blur must smear red past x=70.
	img := dc.Image()
	r, _, _ := rgbAt(t, img, 73, 50)
	if r <= 240 {
		t.Fatalf("blur should bleed red past the box edge (got R=%d at (73,50))", r)
	}
	// Far corner stays clean white.
	farR, farG, farB := rgbAt(t, img, 5, 95)
	if farR != 255 || farG != 255 || farB != 255 {
		t.Fatalf("far corner must stay untouched, got #%02x%02x%02x", farR, farG, farB)
	}
}

// TestRenderImageFilter_ZeroRadiusSkipsIsolation: radius 0 paints the child
// through with no blur bleed.
func TestRenderImageFilter_ZeroRadiusSkipsIsolation(t *testing.T) {
	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	box := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	imf := rendering.NewRenderImageFilter(0, box)
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(imf, 30, 30)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	owner.FlushPaint(rendering.NewPaintContext(dc, 1), true)

	r, g, b := rgbAt(t, dc.Image(), 73, 50)
	if r != 255 || g != 255 || b != 255 {
		t.Fatalf("radius=0 must not bleed past the box edge, got #%02x%02x%02x", r, g, b)
	}
}

// TestRenderFilters_SettersArePaintOnly.
func TestRenderFilters_SettersArePaintOnly(t *testing.T) {
	cf := rendering.NewRenderColorFilter(identityMatrixForTest(), rendering.NewRenderColorBox(10, 10, 1, 1, 1, 1))
	root := rendering.NewAbsoluteBox(50, 50)
	root.Place(cf, 5, 5)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 50, Height: 50}, true)

	var gray [20]float32
	for i := range gray {
		gray[i] = float32(i)
	}
	cf.SetMatrix(gray)
	if !cf.NeedsPaint() {
		t.Fatal("SetMatrix must mark needs-paint")
	}
	if cf.NeedsLayout() {
		t.Fatal("SetMatrix must NOT mark needs-layout")
	}
	for i := range gray {
		if got := cf.ColorMatrix()[i]; got != gray[i] {
			t.Fatalf("ColorMatrix[%d]=%v want %v", i, got, gray[i])
		}
	}

	imf := rendering.NewRenderImageFilter(2)
	root2 := rendering.NewAbsoluteBox(50, 50)
	root2.Place(imf, 5, 5)
	owner2 := rendering.NewPipelineOwner(root2)
	owner2.FlushLayout(rendering.Size{Width: 50, Height: 50}, true)
	imf.SetBlurRadius(-3)
	if imf.BlurParams() != 0 {
		t.Fatal("negative radius must clamp to 0")
	}
	imf.SetBlurRadius(8)
	if !imf.NeedsPaint() || imf.NeedsLayout() {
		t.Fatal("SetBlurRadius must be paint-only")
	}
}

func identityMatrixForTest() [20]float32 {
	var id [20]float32
	id[0], id[6], id[12], id[18] = 1, 1, 1, 1
	return id
}

// TestBuildLayerTree_FilterLayers: RO filters must emit real scene filter
// layers with their params and nested children; identity/zero omit the layer;
// boundary composition nests the filter inside the boundary.
func TestBuildLayerTree_FilterLayers(t *testing.T) {
	scene.ResetLayerIDGen()

	child := rendering.NewRenderColorBox(30, 24, 1, 0, 0, 1)
	cf := rendering.NewRenderGrayscale(child)
	cf.SetRepaintBoundary(true)

	blurChild := rendering.NewRenderColorBox(30, 24, 0, 0, 1, 1)
	imf := rendering.NewRenderImageFilter(4, blurChild)

	root := rendering.NewAbsoluteBox(120, 160)
	root.Place(cf, 7, 9)
	root.Place(imf, 7, 60)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 120, Height: 160}, true)

	b := rendering.BuildLayerTree(root)
	var (
		foundCF    *scene.ColorFilterLayer
		foundIMF   *scene.ImageFilterLayer
		cfInBound  bool
		boundNamed bool
	)
	scene.Walk(b.Root(), func(l scene.Layer) {
		switch t := l.(type) {
		case *scene.ColorFilterLayer:
			if foundCF == nil {
				foundCF = t
			}
		case *scene.ImageFilterLayer:
			if foundIMF == nil {
				foundIMF = t
			}
		}
	})
	scene.Walk(b.Root(), func(l scene.Layer) {
		if bl, ok := l.(*scene.BoundaryLayer); ok && bl.Source == "color_filter" {
			boundNamed = true
			scene.Walk(bl, func(inner scene.Layer) {
				if inner == foundCF {
					cfInBound = true
				}
			})
		}
	})

	if foundCF == nil {
		t.Fatal("expected ColorFilterLayer from RenderColorFilter")
	}
	want := cf.ColorMatrix()
	for i := range want {
		if foundCF.Matrix[i] != want[i] {
			t.Fatalf("layer matrix[%d]=%v want %v", i, foundCF.Matrix[i], want[i])
		}
	}
	if len(foundCF.Children()) == 0 {
		t.Fatal("color filter layer must nest children")
	}
	if !cfInBound {
		t.Fatal("boundary-marked RenderColorFilter must nest its filter layer inside the boundary")
	}
	if !boundNamed {
		t.Fatal("expected boundary sourced color_filter")
	}
	if foundIMF == nil {
		t.Fatal("expected ImageFilterLayer from RenderImageFilter")
	}
	if foundIMF.BlurRadius != 4 {
		t.Fatalf("blur radius=%v want 4", foundIMF.BlurRadius)
	}
}

// TestBuildLayerTree_NoopFiltersOmitted: identity matrix and zero radius must
// not push filter layers.
func TestBuildLayerTree_NoopFiltersOmitted(t *testing.T) {
	scene.ResetLayerIDGen()

	idCf := rendering.NewRenderColorFilter(identityMatrixForTest(), rendering.NewRenderColorBox(10, 10, 1, 0, 0, 1))
	zeroImf := rendering.NewRenderImageFilter(0, rendering.NewRenderColorBox(10, 10, 0, 0, 1, 1))
	root := rendering.NewAbsoluteBox(60, 60)
	root.Place(idCf, 4, 4)
	root.Place(zeroImf, 4, 30)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 60, Height: 60}, true)

	b := rendering.BuildLayerTree(root)
	scene.Walk(b.Root(), func(l scene.Layer) {
		switch l.(type) {
		case *scene.ColorFilterLayer:
			t.Fatal("identity matrix must omit the color filter layer")
		case *scene.ImageFilterLayer:
			t.Fatal("zero radius must omit the image filter layer")
		}
	})
}
