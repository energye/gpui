package recording

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

func TestSolidBrush(t *testing.T) {
	tests := []struct {
		name  string
		color render.RGBA
	}{
		{"red", render.Red},
		{"blue", render.Blue},
		{"transparent", render.Transparent},
		{"semi-transparent", render.RGBA2(1, 0, 0, 0.5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brush := NewSolidBrush(tt.color)

			if brush.Color != tt.color {
				t.Errorf("Color = %v, want %v", brush.Color, tt.color)
			}

			// Verify it implements Brush interface
			var _ Brush = brush
		})
	}
}

func TestLinearGradientBrush(t *testing.T) {
	brush := NewLinearGradientBrush(0, 0, 100, 0).
		AddColorStop(0, render.Red).
		AddColorStop(0.5, render.Yellow).
		AddColorStop(1, render.Blue).
		SetExtend(ExtendRepeat)

	// Verify start and end points
	if brush.Start.X != 0 || brush.Start.Y != 0 {
		t.Errorf("Start = %v, want (0, 0)", brush.Start)
	}
	if brush.End.X != 100 || brush.End.Y != 0 {
		t.Errorf("End = %v, want (100, 0)", brush.End)
	}

	// Verify stops
	if len(brush.Stops) != 3 {
		t.Errorf("len(Stops) = %d, want 3", len(brush.Stops))
	}

	expectedStops := []struct {
		offset float64
		color  render.RGBA
	}{
		{0, render.Red},
		{0.5, render.Yellow},
		{1, render.Blue},
	}

	for i, expected := range expectedStops {
		if brush.Stops[i].Offset != expected.offset {
			t.Errorf("Stops[%d].Offset = %f, want %f", i, brush.Stops[i].Offset, expected.offset)
		}
		if brush.Stops[i].Color != expected.color {
			t.Errorf("Stops[%d].Color = %v, want %v", i, brush.Stops[i].Color, expected.color)
		}
	}

	// Verify extend mode
	if brush.Extend != ExtendRepeat {
		t.Errorf("Extend = %v, want ExtendRepeat", brush.Extend)
	}

	// Verify it implements Brush interface
	var _ Brush = brush
}

func TestRadialGradientBrush(t *testing.T) {
	brush := NewRadialGradientBrush(50, 50, 10, 50).
		SetFocus(30, 30).
		AddColorStop(0, render.White).
		AddColorStop(1, render.Black).
		SetExtend(ExtendReflect)

	// Verify center
	if brush.Center.X != 50 || brush.Center.Y != 50 {
		t.Errorf("Center = %v, want (50, 50)", brush.Center)
	}

	// Verify focus
	if brush.Focus.X != 30 || brush.Focus.Y != 30 {
		t.Errorf("Focus = %v, want (30, 30)", brush.Focus)
	}

	// Verify radii
	if brush.StartRadius != 10 {
		t.Errorf("StartRadius = %f, want 10", brush.StartRadius)
	}
	if brush.EndRadius != 50 {
		t.Errorf("EndRadius = %f, want 50", brush.EndRadius)
	}

	// Verify stops
	if len(brush.Stops) != 2 {
		t.Errorf("len(Stops) = %d, want 2", len(brush.Stops))
	}

	// Verify extend mode
	if brush.Extend != ExtendReflect {
		t.Errorf("Extend = %v, want ExtendReflect", brush.Extend)
	}

	// Verify it implements Brush interface
	var _ Brush = brush
}

func TestRadialGradientBrush_DefaultFocus(t *testing.T) {
	brush := NewRadialGradientBrush(50, 50, 0, 50)

	// Focus should default to center
	if brush.Focus != brush.Center {
		t.Errorf("Focus = %v, want %v (center)", brush.Focus, brush.Center)
	}
}

func TestSweepGradientBrush(t *testing.T) {
	startAngle := math.Pi / 4 // 45 degrees
	brush := NewSweepGradientBrush(50, 50, startAngle).
		SetEndAngle(startAngle+math.Pi). // 180 degrees sweep
		AddColorStop(0, render.Red).
		AddColorStop(1, render.Blue).
		SetExtend(ExtendPad)

	// Verify center
	if brush.Center.X != 50 || brush.Center.Y != 50 {
		t.Errorf("Center = %v, want (50, 50)", brush.Center)
	}

	// Verify angles
	if brush.StartAngle != startAngle {
		t.Errorf("StartAngle = %f, want %f", brush.StartAngle, startAngle)
	}

	expectedEndAngle := startAngle + math.Pi
	if math.Abs(brush.EndAngle-expectedEndAngle) > 1e-10 {
		t.Errorf("EndAngle = %f, want %f", brush.EndAngle, expectedEndAngle)
	}

	// Verify stops
	if len(brush.Stops) != 2 {
		t.Errorf("len(Stops) = %d, want 2", len(brush.Stops))
	}

	// Verify extend mode
	if brush.Extend != ExtendPad {
		t.Errorf("Extend = %v, want ExtendPad", brush.Extend)
	}

	// Verify it implements Brush interface
	var _ Brush = brush
}

func TestSweepGradientBrush_DefaultEndAngle(t *testing.T) {
	brush := NewSweepGradientBrush(50, 50, 0)

	// Default should be full rotation (2*Pi)
	expectedEndAngle := 2 * math.Pi
	if math.Abs(brush.EndAngle-expectedEndAngle) > 1e-10 {
		t.Errorf("EndAngle = %f, want %f", brush.EndAngle, expectedEndAngle)
	}
}

func TestPatternBrush(t *testing.T) {
	img := testImageBuf(t, 16, 16, render.Red)
	inverse := render.Translate(10, 20).Multiply(render.Scale(2, 2)).Invert()

	brush := NewPatternBrush(img).
		SetSourceRegion(2, 3, 8, 9).
		SetInverse(inverse).
		SetOpacity(0.4).
		SetClamp(true)

	if brush.Image != img {
		t.Errorf("Image = %p, want %p", brush.Image, img)
	}
	if brush.SrcX != 2 || brush.SrcY != 3 || brush.SrcW != 8 || brush.SrcH != 9 {
		t.Errorf("source region = %v", brush.Source())
	}
	if brush.Inverse != inverse {
		t.Errorf("Inverse = %v, want %v", brush.Inverse, inverse)
	}
	if brush.Opacity != 0.4 || !brush.Clamp {
		t.Errorf("Opacity/Clamp = %v/%v, want 0.4/true", brush.Opacity, brush.Clamp)
	}

	// A recorded pattern brush is a render.Pattern, not a render.Brush: replay
	// must route it to SetFillPattern, and ToRenderBrush must refuse to hand back
	// a colour that would repaint a textured surface black.
	if got := ToRenderBrush(brush); got != nil {
		t.Errorf("ToRenderBrush(pattern) = %v, want nil", got)
	}
	pat, ok := ToRenderPattern(brush)
	if !ok || pat == nil {
		t.Fatalf("ToRenderPattern(pattern) = %v, %v; want a pattern", pat, ok)
	}

	// The sampled result has to be bit-identical to the live pattern's, which is
	// what carrying the stored inverse (rather than re-inverting a forward) buys.
	live := render.NewImagePattern(img, 2, 3, 8, 9)
	live.SetTransform(render.Translate(10, 20).Multiply(render.Scale(2, 2)))
	live.SetOpacity(0.4)
	live.SetClamp(true)
	for _, p := range []render.Point{{X: 0, Y: 0}, {X: 12.5, Y: 33}, {X: 200, Y: 7}, {X: -40, Y: 90}} {
		if pat.ColorAt(p.X, p.Y) != live.ColorAt(p.X, p.Y) {
			t.Errorf("ColorAt(%v) = %v, want %v (live)", p, pat.ColorAt(p.X, p.Y), live.ColorAt(p.X, p.Y))
		}
	}

	var _ Brush = brush
}

func TestPatternBrush_Defaults(t *testing.T) {
	img := testImageBuf(t, 8, 8, render.Red)
	brush := NewPatternBrush(img)

	if brush.SrcW != 0 || brush.SrcH != 0 {
		t.Errorf("default source size = %d x %d, want 0 x 0 (whole image)", brush.SrcW, brush.SrcH)
	}
	if !brush.Inverse.IsIdentity() {
		t.Errorf("Inverse = %v, want identity", brush.Inverse)
	}
	if brush.Opacity != 0 || brush.Clamp {
		t.Errorf("Opacity/Clamp = %v/%v, want 0 (unspecified)/false", brush.Opacity, brush.Clamp)
	}
}

// A pattern whose image was disposed between recording and replay cannot sample
// anything. It must report "no pattern" rather than hand back a zero-sized one,
// which would divide by zero in the tiling sampler.
func TestPatternBrush_DisposedImagePaintsNothing(t *testing.T) {
	img := testImageBuf(t, 8, 8, render.Red)
	img.Dispose()

	brush := NewPatternBrush(img)
	pat, ok := ToRenderPattern(brush)
	if !ok {
		t.Fatal("disposed pattern brush reported as non-pattern")
	}
	if pat != nil {
		t.Fatalf("disposed pattern brush produced %v, want nil pattern", pat)
	}
}

func TestFromRenderPattern_ImagePatternRoundTrip(t *testing.T) {
	img := testImageBuf(t, 12, 10, render.Blue)
	live := render.NewImagePattern(img, 1, 2, 5, 4)
	live.SetTransform(render.Rotate(0.3))
	live.SetOpacity(0.7)
	live.SetClamp(true)

	pool := NewResourcePool()
	brush := FromRenderPattern(live, pool)
	pb, ok := brush.(*PatternBrush)
	if !ok {
		t.Fatalf("FromRenderPattern(ImagePattern) = %T, want *PatternBrush", brush)
	}
	if pb.Image != img {
		t.Errorf("Image = %p, want %p", pb.Image, img)
	}
	if pool.ImageCount() != 1 {
		t.Errorf("pool sealed %d images, want 1 (the pattern's image must be retained)", pool.ImageCount())
	}

	back, ok := ToRenderPattern(pb)
	if !ok || back == nil {
		t.Fatalf("ToRenderPattern round trip failed: %v, %v", back, ok)
	}
	for _, p := range []render.Point{{X: 3, Y: 7}, {X: 40, Y: 12}, {X: -9, Y: 25}, {X: 100, Y: 100}} {
		if back.ColorAt(p.X, p.Y) != live.ColorAt(p.X, p.Y) {
			t.Errorf("ColorAt(%v) = %v, want %v", p, back.ColorAt(p.X, p.Y), live.ColorAt(p.X, p.Y))
		}
	}
}

func TestFromRenderPattern_SolidAndCustom(t *testing.T) {
	pool := NewResourcePool()

	solid := FromRenderPattern(render.NewSolidPattern(render.Green), pool)
	if sb, ok := solid.(SolidBrush); !ok || sb.Color != render.Green {
		t.Errorf("FromRenderPattern(SolidPattern) = %#v, want SolidBrush{Green}", solid)
	}

	custom := FromRenderPattern(patternSampler{render.Red}, pool)
	cp, ok := custom.(CustomPatternBrush)
	if !ok {
		t.Fatalf("FromRenderPattern(foreign Pattern) = %T, want CustomPatternBrush", custom)
	}
	pat, ok := ToRenderPattern(cp)
	if !ok || pat != render.Pattern(patternSampler{render.Red}) {
		t.Errorf("custom pattern replay = %v, %v; want the recorded pattern by reference", pat, ok)
	}
	if _, ok := ToRenderPattern(SolidBrush{Color: render.Red}); ok {
		t.Error("ToRenderPattern claimed a solid brush is a pattern")
	}
}

// patternSampler is a caller-supplied render.Pattern: an interface the recording
// cannot describe as data, so it carries it by reference.
type patternSampler struct{ col render.RGBA }

func (p patternSampler) ColorAt(_, _ float64) render.RGBA { return p.col }

func TestExtendMode(t *testing.T) {
	tests := []struct {
		mode ExtendMode
		want int
	}{
		{ExtendPad, 0},
		{ExtendRepeat, 1},
		{ExtendReflect, 2},
	}

	for _, tt := range tests {
		if int(tt.mode) != tt.want {
			t.Errorf("ExtendMode %v = %d, want %d", tt.mode, int(tt.mode), tt.want)
		}
	}
}

func TestFromRenderBrush_Solid(t *testing.T) {
	renderBrush := render.Solid(render.Red)
	brush := FromRenderBrush(renderBrush)

	solid, ok := brush.(SolidBrush)
	if !ok {
		t.Fatalf("FromRenderBrush(SolidBrush) returned %T, want SolidBrush", brush)
	}

	if solid.Color != render.Red {
		t.Errorf("Color = %v, want %v", solid.Color, render.Red)
	}
}

func TestFromRenderBrush_LinearGradient(t *testing.T) {
	renderBrush := render.NewLinearGradientBrush(0, 0, 100, 0).
		AddColorStop(0, render.Red).
		AddColorStop(1, render.Blue).
		SetExtend(render.ExtendRepeat)

	brush := FromRenderBrush(renderBrush)

	linear, ok := brush.(*LinearGradientBrush)
	if !ok {
		t.Fatalf("FromRenderBrush(LinearGradientBrush) returned %T, want *LinearGradientBrush", brush)
	}

	// Verify conversion
	if linear.Start.X != 0 || linear.Start.Y != 0 {
		t.Errorf("Start = %v, want (0, 0)", linear.Start)
	}
	if linear.End.X != 100 || linear.End.Y != 0 {
		t.Errorf("End = %v, want (100, 0)", linear.End)
	}
	if len(linear.Stops) != 2 {
		t.Errorf("len(Stops) = %d, want 2", len(linear.Stops))
	}
	if linear.Extend != ExtendRepeat {
		t.Errorf("Extend = %v, want ExtendRepeat", linear.Extend)
	}
}

func TestFromRenderBrush_RadialGradient(t *testing.T) {
	renderBrush := render.NewRadialGradientBrush(50, 50, 10, 50).
		SetFocus(30, 30).
		AddColorStop(0, render.White).
		AddColorStop(1, render.Black)

	brush := FromRenderBrush(renderBrush)

	radial, ok := brush.(*RadialGradientBrush)
	if !ok {
		t.Fatalf("FromRenderBrush(RadialGradientBrush) returned %T, want *RadialGradientBrush", brush)
	}

	// Verify conversion
	if radial.Center.X != 50 || radial.Center.Y != 50 {
		t.Errorf("Center = %v, want (50, 50)", radial.Center)
	}
	if radial.Focus.X != 30 || radial.Focus.Y != 30 {
		t.Errorf("Focus = %v, want (30, 30)", radial.Focus)
	}
	if radial.StartRadius != 10 {
		t.Errorf("StartRadius = %f, want 10", radial.StartRadius)
	}
	if radial.EndRadius != 50 {
		t.Errorf("EndRadius = %f, want 50", radial.EndRadius)
	}
	if len(radial.Stops) != 2 {
		t.Errorf("len(Stops) = %d, want 2", len(radial.Stops))
	}
}

func TestFromRenderBrush_SweepGradient(t *testing.T) {
	renderBrush := render.NewSweepGradientBrush(50, 50, math.Pi/4).
		AddColorStop(0, render.Red).
		AddColorStop(1, render.Blue)

	brush := FromRenderBrush(renderBrush)

	sweep, ok := brush.(*SweepGradientBrush)
	if !ok {
		t.Fatalf("FromRenderBrush(SweepGradientBrush) returned %T, want *SweepGradientBrush", brush)
	}

	// Verify conversion
	if sweep.Center.X != 50 || sweep.Center.Y != 50 {
		t.Errorf("Center = %v, want (50, 50)", sweep.Center)
	}
	if sweep.StartAngle != math.Pi/4 {
		t.Errorf("StartAngle = %f, want %f", sweep.StartAngle, math.Pi/4)
	}
	if len(sweep.Stops) != 2 {
		t.Errorf("len(Stops) = %d, want 2", len(sweep.Stops))
	}
}

func TestFromRenderBrush_CustomBrush(t *testing.T) {
	// Create a custom brush via render.CustomBrush
	// Note: render.CustomBrush implements render.Brush interface
	custom := render.CustomBrush{
		Func: func(x, y float64) render.RGBA {
			return render.Blue
		},
		Name: "test-custom",
	}

	brush := FromRenderBrush(custom)

	// Should fallback to SolidBrush with color sampled at origin
	solid, ok := brush.(SolidBrush)
	if !ok {
		t.Fatalf("FromRenderBrush(CustomBrush) returned %T, want SolidBrush", brush)
	}

	if solid.Color != render.Blue {
		t.Errorf("Color = %v, want %v (sampled at origin)", solid.Color, render.Blue)
	}
}

func TestGradientStop(t *testing.T) {
	stop := GradientStop{
		Offset: 0.5,
		Color:  render.Red,
	}

	if stop.Offset != 0.5 {
		t.Errorf("Offset = %f, want 0.5", stop.Offset)
	}
	if stop.Color != render.Red {
		t.Errorf("Color = %v, want %v", stop.Color, render.Red)
	}
}

// BenchmarkFromRenderBrush benchmarks brush conversion.
func BenchmarkFromRenderBrush_Solid(b *testing.B) {
	brush := render.Solid(render.Red)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FromRenderBrush(brush)
	}
}

func BenchmarkFromRenderBrush_LinearGradient(b *testing.B) {
	brush := render.NewLinearGradientBrush(0, 0, 100, 0).
		AddColorStop(0, render.Red).
		AddColorStop(0.5, render.Yellow).
		AddColorStop(1, render.Blue)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FromRenderBrush(brush)
	}
}
