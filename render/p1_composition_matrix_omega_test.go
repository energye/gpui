//go:build !nogpu

package render_test

// Phase A omega composition probes D106+ — more multi-axis stress, not widgets.
// docs/P1_COMPOSITION_MATRIX.md

import (
	"fmt"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// D106: music player — art × spectrum bars × transport × queue list.
func TestP1_Comp_D106_MusicPlayerComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// cover art
	art := compMakeImage(t, 48, 48, 180, 60, 200)
	dc.DrawImageRounded(art, 24, 24, 10)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("Track Title — Artist", 90, 48)
	dc.SetRGB(0.6, 0.65, 0.75)
	dc.DrawString("Album · 2026", 90, 70)

	// spectrum
	for i := 0; i < 32; i++ {
		bh := 10 + 40*math.Abs(math.Sin(float64(i)*0.4))
		dc.SetRGB(0.3, 0.55+float64(i%5)*0.05, 0.95)
		dc.DrawRectangle(24+float64(i)*11, 160-bh, 8, bh)
		_ = dc.Fill()
	}
	// progress
	dc.SetRGB(0.3, 0.32, 0.38)
	dc.DrawRoundedRectangle(24, 180, w-48, 6, 3)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.4, 0.55)
	dc.DrawRoundedRectangle(24, 180, 140, 6, 3)
	_ = dc.Fill()
	// queue
	dc.ClipRect(24, 200, w-48, 80)
	for i := 0; i < 4; i++ {
		y := 208 + float64(i)*18
		if i == 1 {
			dc.SetRGB(0.2, 0.25, 0.35)
			dc.DrawRectangle(24, y-4, w-48, 18)
			_ = dc.Fill()
		}
		dc.SetRGB(0.85, 0.88, 0.92)
		dc.DrawString(fmt.Sprintf("%d. Next song item %d", i+1, i), 32, y+8)
	}
	dc.ResetClip()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 40, 40)
	p1NotNearWhite(t, "D106 art", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 80, 180)
	if r2 < 100 {
		t.Fatalf("D106 progress missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D107: 3-pane resizable app shell (nav / list / detail).
func TestP1_Comp_D107_ThreePaneShellComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 560, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// nav
	dc.SetRGB(0.14, 0.15, 0.18)
	dc.DrawRectangle(0, 0, 72, h)
	_ = dc.Fill()
	for i := 0; i < 5; i++ {
		dc.SetRGB(0.3, 0.35, 0.45)
		dc.DrawRoundedRectangle(16, 20+float64(i)*48, 40, 32, 8)
		_ = dc.Fill()
	}
	// list
	dc.ClipRect(72, 0, 180, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(72, 0, 180, h)
	_ = dc.Fill()
	for i := 0; i < 10; i++ {
		y := 12 + float64(i)*30
		if i == 3 {
			dc.SetRGB(0.85, 0.91, 1)
			dc.DrawRectangle(72, y-4, 180, 28)
			_ = dc.Fill()
		}
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("Item %02d", i+1), 84, y+12)
	}
	dc.ResetClip()
	// sash
	dc.SetRGB(0.8, 0.82, 0.86)
	dc.DrawRectangle(252, 0, 4, h)
	_ = dc.Fill()
	// detail
	dc.ClipRect(256, 0, w-256, h)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(256, 0, w-256, h)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Detail title", 272, 36)
	dc.DrawStringWrapped("Three-pane composition: nav icons, selectable list with clip, and detail body with wrapped text under GPU.", 272, 60, 0, 0, 250, 1.25, render.AlignLeft)
	img := compMakeImage(t, 64, 40, 50, 120, 200)
	dc.DrawImage(img, 272, 160)
	dc.ResetClip()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 4, 6)
	if r > 70 {
		t.Fatalf("D107 nav not dark rgba=%d,%d,%d", r, g, b)
	}
	// selected list row i==3 → y=12+3*30=102
	r2, g2, b2, _ := p1Sample(dc, 100, 102)
	if b2 < 100 {
		t.Fatalf("D107 list sel missing tint rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D108: PR review — diff hunks × inline comments × approve bar.
func TestP1_Comp_D108_PRReviewComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 500, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawString("PR #42 · composition coverage", 12, 24)

	dc.ClipRect(0, 40, w, h-90)
	lines := []struct {
		k byte
		s string
	}{
		{' ', " func Render() {"},
		{'-', "  oldClip()"},
		{'+', "  clip.Preserve()"},
		{'+', "  layer.Push(0.9)"},
		{' ', "  draw()"},
		{' ', " }"},
	}
	for i, ln := range lines {
		y := 52 + float64(i)*24
		switch ln.k {
		case '+':
			dc.SetRGB(0.12, 0.28, 0.16)
		case '-':
			dc.SetRGB(0.32, 0.14, 0.14)
		default:
			dc.SetRGB(0.14, 0.15, 0.18)
		}
		dc.DrawRectangle(0, y-6, w, 24)
		_ = dc.Fill()
		if ln.k == '+' {
			dc.SetRGB(0.5, 0.9, 0.6)
		} else if ln.k == '-' {
			dc.SetRGB(0.95, 0.55, 0.55)
		} else {
			dc.SetRGB(0.75, 0.78, 0.85)
		}
		prefix := " "
		if ln.k != ' ' {
			prefix = string(ln.k)
		}
		dc.DrawString(fmt.Sprintf("%s %s", prefix, ln.s), 16, y+8)
	}
	// inline comment card
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.2, 0.22, 0.28)
	dc.DrawRoundedRectangle(80, 170, 280, 56, 8)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("Reviewer: LGTM after ClipPreserve gate", 92, 200)
	dc.PopLayer()
	dc.ResetClip()

	// action bar
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRectangle(0, h-50, w, 50)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.65, 0.4)
	dc.DrawRoundedRectangle(w-120, h-38, 100, 28, 6)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Approve", w-100, h-20)

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, w-70, h-24)
	if g < 80 {
		t.Fatalf("D108 approve missing rgba=%d,%d,%d", r, g, b)
	}
}

// D109: week calendar — columns × events × now line × popup.
func TestP1_Comp_D109_WeekCalendarComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// time gutter
	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawRectangle(0, 36, 48, h-36)
	_ = dc.Fill()
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	for i, d := range days {
		x := 48 + float64(i)*66
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(d, x+18, 24)
		dc.SetRGB(0.88, 0.89, 0.92)
		dc.DrawLine(x, 36, x, h)
		_ = dc.Stroke()
	}
	// events
	ev := []struct {
		day, row int
		dur      float64
		r, g, b  float64
		title    string
	}{
		{0, 1, 2, 0.2, 0.5, 0.95, "Standup"},
		{1, 2, 3, 0.3, 0.7, 0.4, "Design"},
		{2, 1, 2, 0.9, 0.5, 0.2, "Review"},
		{3, 3, 2, 0.55, 0.35, 0.9, "Ship"},
		{4, 0, 4, 0.2, 0.65, 0.75, "Focus"},
	}
	dc.ClipRect(48, 36, w-48, h-36)
	for _, e := range ev {
		x := 52 + float64(e.day)*66
		y := 44 + float64(e.row)*40
		dc.SetRGB(e.r, e.g, e.b)
		dc.DrawRoundedRectangle(x, y, 58, 28*e.dur, 6)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(e.title, x+6, y+16)
	}
	// now line
	dc.SetRGB(0.95, 0.25, 0.3)
	dc.SetLineWidth(2)
	dc.DrawLine(48, 140, w, 140)
	_ = dc.Stroke()
	dc.ResetClip()

	// popup
	dc.PushLayer(render.BlendNormal, 0.96)
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(200, 150, 160, 70, 8)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Event details", 212, 180)
	dc.DrawString("10:00–11:00", 212, 200)
	dc.PopLayer()

	compMinGPU(t, dc, 20)
	// first event Standup at day0 row1: x≈52..110 y≈84..140
	r, g, b, _ := p1Sample(dc, 80, 100)
	if b < 80 {
		t.Fatalf("D109 event missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 250, 180)
	if r2 < 200 {
		t.Fatalf("D109 popup missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D110: network graph — nodes × edges × selected cluster × minimap.
func TestP1_Comp_D110_NetworkGraphComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 440, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.08, 0.09, 0.12)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	type n struct{ x, y float64 }
	nodes := make([]n, 12)
	for i := range nodes {
		ang := float64(i) * math.Pi / 6
		nodes[i] = n{200 + 110*math.Cos(ang), 150 + 80*math.Sin(ang)}
	}
	// edges
	dc.SetRGB(0.35, 0.4, 0.5)
	dc.SetLineWidth(1.5)
	for i := 0; i < len(nodes); i++ {
		j := (i + 3) % len(nodes)
		dc.DrawLine(nodes[i].x, nodes[i].y, nodes[j].x, nodes[j].y)
		_ = dc.Stroke()
	}
	// selected cluster layer
	dc.PushLayer(render.BlendNormal, 0.35)
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.DrawCircle(200, 150, 70)
	_ = dc.Fill()
	dc.PopLayer()
	for i, nd := range nodes {
		if i < 4 {
			dc.SetRGB(0.3, 0.65, 1)
		} else {
			dc.SetRGB(0.7, 0.75, 0.85)
		}
		dc.DrawCircle(nd.x, nd.y, 10)
		_ = dc.Fill()
	}
	// minimap
	dc.SetRGB(0.15, 0.17, 0.22)
	dc.DrawRoundedRectangle(w-90, h-80, 76, 60, 6)
	_ = dc.Fill()
	dc.SetRGB(0.4, 0.7, 1)
	dc.DrawCircle(w-52, h-50, 8)
	_ = dc.Fill()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 200, 150)
	p1NotNearWhite(t, "D110 cluster", r, g, b)
}

// D111: image editor chrome — tools × canvas × hist × layers panel.
func TestP1_Comp_D111_ImageEditorComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// toolbar
	dc.SetRGB(0.16, 0.17, 0.2)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	for i, name := range []string{"Move", "Crop", "Brush", "Text", "Filter"} {
		x := 12 + float64(i)*70
		dc.SetRGB(0.28, 0.3, 0.36)
		dc.DrawRoundedRectangle(x, 8, 60, 24, 4)
		_ = dc.Fill()
		dc.SetRGB(0.9, 0.91, 0.93)
		dc.DrawString(name, x+8, 24)
	}
	// canvas
	dc.ClipRect(0, 40, w-140, h-40)
	dc.SetRGB(0.25, 0.27, 0.32)
	dc.DrawRectangle(0, 40, w-140, h-40)
	_ = dc.Fill()
	img := compMakeImage(t, 40, 40, 0, 0, 0)
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			_ = img.SetRGBA(x, y, uint8(x*6), uint8(y*6), 180, 255)
		}
	}
	dc.DrawImageEx(img, render.DrawImageOptions{X: 80, Y: 80, DstWidth: 200, DstHeight: 160, Opacity: 1})
	// crop rect
	dc.SetRGB(0.95, 0.95, 0.2)
	dc.SetLineWidth(1)
	dc.SetDash(4, 3)
	dc.DrawRectangle(100, 100, 140, 100)
	_ = dc.Stroke()
	dc.SetDash()
	dc.ResetClip()
	// layers panel
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(w-140, 40, 140, h-40)
	_ = dc.Fill()
	for i := 0; i < 5; i++ {
		y := 56 + float64(i)*36
		if i == 1 {
			dc.SetRGB(0.85, 0.91, 1)
			dc.DrawRectangle(w-140, y-6, 140, 32)
			_ = dc.Fill()
		}
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("Layer %d", i+1), w-120, y+12)
	}

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 150, 140)
	p1NotNearWhite(t, "D111 canvas img", r, g, b)
}

// D112: checkout density — cart lines × promo × totals × pay CTA.
func TestP1_Comp_D112_CheckoutDensityComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Checkout composition", 16, 28)

	dc.ClipRect(16, 44, 250, 220)
	for i := 0; i < 5; i++ {
		y := 52 + float64(i)*40
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(16, y, 240, 36, 6)
		_ = dc.Fill()
		thumb := compMakeImage(t, 24, 24, uint8(40+i*30), 120, 200)
		dc.DrawImageRounded(thumb, 24, y+6, 4)
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("Product %d  ×1", i+1), 56, y+22)
		dc.DrawString(fmt.Sprintf("$%d", 12+i*3), 220, y+22)
	}
	dc.ResetClip()

	// summary card
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(280, 44, 124, 200, 10)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.27, 0.32)
	dc.DrawString("Subtotal", 292, 70)
	dc.DrawString("$89.00", 292, 95)
	dc.SetRGB(0.85, 0.87, 0.9)
	dc.DrawRectangle(292, 120, 100, 28)
	_ = dc.Fill()
	dc.SetRGB(0.4, 0.42, 0.48)
	dc.DrawString("PROMO", 300, 138)
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.15, 0.55, 0.35)
	dc.DrawRoundedRectangle(292, 190, 100, 32, 6)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Pay now", 312, 210)
	dc.PopLayer()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 330, 200)
	if g < 80 {
		t.Fatalf("D112 pay CTA missing rgba=%d,%d,%d", r, g, b)
	}
}

// D113: notification drawer — slide panel × list × unread dots × mask dim.
func TestP1_Comp_D113_NotificationDrawerComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// page
	for i := 0; i < 6; i++ {
		dc.SetRGB(0.93, 0.94, 0.96)
		dc.DrawRoundedRectangle(16, 16+float64(i)*42, w-32, 34, 6)
		_ = dc.Fill()
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("page: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	// dim
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.4)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// drawer
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(w-220, 0, 220, h)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Notifications", w-200, 28)
	for i := 0; i < 6; i++ {
		y := 50 + float64(i)*36
		dc.SetRGB(0.96, 0.97, 0.98)
		dc.DrawRectangle(w-220, y, 220, 34)
		_ = dc.Fill()
		if i%2 == 0 {
			dc.SetRGB(0.2, 0.5, 0.95)
			dc.DrawCircle(w-30, y+17, 4)
			_ = dc.Fill()
		}
		dc.SetRGB(0.25, 0.27, 0.32)
		dc.DrawString(fmt.Sprintf("Notice #%d body", i+1), w-200, y+20)
	}
	dc.PopLayer()

	compMinGPU(t, dc, base+1)
	// unread blue dot on even rows: i=0 → y=50+17=67
	r, g, b, _ := p1Sample(dc, w-30, 67)
	if b < 80 {
		t.Fatalf("D113 unread dot missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 20, 20)
	if r2 > 180 && g2 > 180 {
		t.Fatalf("D113 expected dimmed page rgba=%d,%d,%d", r2, g2, b2)
	}
	// drawer title text ink
	ink := 0
	for x := w - 200; x < w-40; x++ {
		rr, gg, bb, _ := p1Sample(dc, x, 24)
		if rr < 230 || gg < 230 || bb < 230 {
			ink++
		}
	}
	if ink < 3 {
		t.Fatalf("D113 drawer title ink low: %d", ink)
	}
}

// D114: multi-tab terminal — tabs × buffers × prompt × selection.
func TestP1_Comp_D114_MultiTabTerminalComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// tabs
	for i, name := range []string{"bash", "logs", "gpu"} {
		x := 8 + float64(i)*90
		if i == 0 {
			dc.SetRGB(0.2, 0.22, 0.28)
		} else {
			dc.SetRGB(0.16, 0.17, 0.2)
		}
		dc.DrawRoundedRectangle(x, 6, 84, 26, 6)
		_ = dc.Fill()
		dc.SetRGB(0.85, 0.88, 0.92)
		dc.DrawString(name, x+20, 24)
	}
	// buffer
	dc.ClipRect(0, 36, w, h-36)
	dc.SetRGB(0.08, 0.09, 0.1)
	dc.DrawRectangle(0, 36, w, h-36)
	_ = dc.Fill()
	// selection
	dc.SetRGB(0.15, 0.3, 0.45)
	dc.DrawRectangle(8, 80, 300, 18)
	_ = dc.Fill()
	dc.SetRGB(0.4, 0.85, 0.5)
	for i, line := range []string{
		"$ export WGPU_NATIVE_PATH=...",
		"$ go test ./render -run Comp",
		"ok  105 tests",
		"$ ▍",
	} {
		dc.DrawString(line, 12, 60+float64(i)*28)
	}
	dc.ResetClip()

	compMinGPU(t, dc, 8)
	ink := 0
	for y := 50; y < 160; y++ {
		for x := 12; x < 300; x += 2 {
			rr, gg, _, _ := p1Sample(dc, x, y)
			if gg > rr+15 && gg > 50 {
				ink++
			}
		}
	}
	if ink < 5 {
		t.Fatalf("D114 term green ink low: %d", ink)
	}
}

// D115: markdown split preview — editor × preview × code fence × quote.
func TestP1_Comp_D115_MarkdownSplitPreviewComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// editor
	dc.ClipRect(0, 0, w/2, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w/2, h)
	_ = dc.Fill()
	dc.SetRGB(0.8, 0.82, 0.88)
	for i, line := range []string{
		"# Title",
		"",
		"Paragraph with **bold**",
		"> quote block",
		"```go",
		"dc.ClipRect(...)",
		"```",
	} {
		dc.DrawString(line, 12, 28+float64(i)*24)
	}
	dc.ResetClip()
	// preview
	dc.ClipRect(w/2, 0, w/2, h)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(w/2, 0, w/2, h)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	_ = dc.LoadFontFace(font, 18)
	dc.DrawString("Title", w/2+16, 36)
	_ = dc.LoadFontFace(font, 11)
	dc.DrawStringWrapped("Paragraph with emphasis rendered in preview pane under clip.", w/2+16, 56, 0, 0, 220, 1.2, render.AlignLeft)
	// quote
	dc.SetRGB(0.9, 0.92, 0.95)
	dc.DrawRectangle(w/2+16, 120, 220, 40)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.5, 0.9)
	dc.DrawRectangle(w/2+16, 120, 4, 40)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.32, 0.38)
	dc.DrawString("quote block", w/2+28, 144)
	// code fence
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRoundedRectangle(w/2+16, 180, 220, 60, 6)
	_ = dc.Fill()
	dc.SetRGB(0.7, 0.85, 0.6)
	dc.DrawString("dc.ClipRect(...)", w/2+28, 214)
	dc.ResetClip()

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 40, 40)
	if r > 80 {
		t.Fatalf("D115 editor not dark rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w/2+40, 200)
	if r2 > 80 {
		t.Fatalf("D115 code fence not dark rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D116: data grid frozen header+col × multi-select × sort carets.
func TestP1_Comp_D116_DataGridFrozenMultiSelect(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 500, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// frozen header
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawRectangle(0, 0, w, 28)
	_ = dc.Fill()
	// frozen col
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 28, 80, h-28)
	_ = dc.Fill()
	cols := []string{"ID", "Name", "Region", "Score", "Status"}
	for i, c := range cols {
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(c, 12+float64(i)*90, 18)
	}
	dc.ClipRect(80, 28, w-80, h-28)
	for r := 0; r < 10; r++ {
		y := 32 + float64(r)*24
		if r == 2 || r == 3 {
			dc.SetRGB(0.85, 0.91, 1)
			dc.DrawRectangle(80, y-4, w-80, 24)
			_ = dc.Fill()
		} else if r%2 == 0 {
			dc.SetRGB(0.98, 0.98, 0.99)
			dc.DrawRectangle(80, y-4, w-80, 24)
			_ = dc.Fill()
		}
		dc.SetRGB(0.25, 0.27, 0.32)
		for c := 1; c < 5; c++ {
			dc.DrawString(fmt.Sprintf("v%d-%d", r, c), 12+float64(c)*90, y+10)
		}
	}
	dc.ResetClip()
	for r := 0; r < 10; r++ {
		dc.SetRGB(0.25, 0.27, 0.32)
		dc.DrawString(fmt.Sprintf("#%02d", r+1), 16, 42+float64(r)*24)
	}

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 120, 80)
	p1NotNearWhite(t, "D116 selected rows", r, g, b)
}

// D117: floating format toolbar over text selection.
func TestP1_Comp_D117_FloatingToolbarSelection(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(24, 40, 350, 160, 8)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawStringWrapped("Select this sentence to show a floating formatting toolbar above the selection range.", 36, 60, 0, 0, 320, 1.3, render.AlignLeft)
	// selection highlight
	dc.SetRGBA(0.2, 0.5, 0.95, 0.25)
	dc.DrawRectangle(36, 80, 220, 18)
	_ = dc.Fill()
	// floating toolbar
	dc.PushLayer(render.BlendNormal, 0.96)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRoundedRectangle(60, 48, 180, 32, 6)
	_ = dc.Fill()
	for i, lab := range []string{"B", "I", "U", "H", "•"} {
		dc.SetRGB(0.9, 0.91, 0.93)
		dc.DrawString(lab, 76+float64(i)*32, 68)
	}
	dc.PopLayer()

	compMinGPU(t, dc, 5)
	r, g, b, _ := p1Sample(dc, 100, 55)
	if r > 80 {
		t.Fatalf("D117 toolbar not dark rgba=%d,%d,%d", r, g, b)
	}
}

// D118: rotate+mask+blurXY+shadow card stack.
func TestP1_Comp_D118_RotateMaskBlurShadowStack(t *testing.T) {
	p1RequireGPU(t)
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	const w, h = 360, 260
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.Push()
	dc.Translate(180, 130)
	dc.Rotate(0.2)
	dc.Translate(-80, -50)
	mask := render.NewMask(w, h)
	// device-ish mask covering rotated card region approximately
	compFillMaskRect(mask, 60, 40, 300, 220, 255)
	dc.SetMask(mask)
	dc.SetRGB(0.2, 0.45, 0.9)
	dc.DrawRoundedRectangle(0, 0, 160, 100, 12)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("rotated card", 24, 55)
	dc.ClearMask()
	dc.Pop()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	dc.ApplyDropShadow(3, 4, 6, render.RGBA{R: 0, G: 0, B: 0, A: 0.35})
	dc.ApplyBlurXY(0.8, 0.3)
	p1Flush(t, dc)
	r, g, b, _ := p1Sample(dc, 180, 130)
	p1NotNearWhite(t, "D118 card", r, g, b)
}

// D119: triangle fan mesh × Overlay blend × circular images × stroke pattern.
func TestP1_Comp_D119_FanMeshOverlayCircularPattern(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 340, 240
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// triangle fan
	pos := []render.Point{{X: 160, Y: 110}}
	cols := []render.RGBA{{R: 1, G: 1, B: 1, A: 1}}
	for i := 0; i <= 8; i++ {
		ang := float64(i) * math.Pi / 4
		pos = append(pos, render.Point{X: 160 + 70*math.Cos(ang), Y: 110 + 70*math.Sin(ang)})
		cols = append(cols, render.RGBA{R: float64(i%3) * 0.4, G: 0.4, B: 0.9 - float64(i)*0.05, A: 1})
	}
	dc.DrawVertices(pos, cols, render.VertexModeTriangleFan)

	img := compMakeImage(t, 32, 32, 220, 90, 40)
	dc.SetBlendMode(render.BlendOverlay)
	dc.DrawImageCircular(img, 100, 180, 28)
	dc.DrawImageCircular(img, 220, 180, 28)
	dc.SetBlendMode(render.BlendNormal)

	tile := compMakeImage(t, 6, 6, 240, 200, 40)
	dc.SetStrokePattern(dc.CreateImagePattern(tile, 0, 0, 6, 6))
	dc.SetLineWidth(4)
	dc.DrawCircle(160, 110, 78)
	_ = dc.Stroke()

	compMinGPU(t, dc, 4)
	r, g, b, _ := p1Sample(dc, 160, 90)
	p1NotNearWhite(t, "D119 fan", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 100, 180)
	p1NotNearWhite(t, "D119 circular", r2, g2, b2)
}

// D120: stress lattice 12×16 with nested clip/layer/blend/text (correctness under load).
func TestP1_Comp_D120_StressLatticeNestedAxes(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 9)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRoundRect(8, 8, w-16, h-16, 12)
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	for row := 0; row < 12; row++ {
		for col := 0; col < 16; col++ {
			x := 16 + float64(col)*28
			y := 16 + float64(row)*28
			// base solid cell (Normal) so pixels stay visibly colored
			dc.SetBlendMode(render.BlendNormal)
			dc.SetRGB(0.2+float64(col)*0.04, 0.3+float64(row)*0.03, 0.75)
			dc.DrawRoundedRectangle(x, y, 24, 24, 4)
			_ = dc.Fill()
			// occasional overlay accent with advanced blend
			if (row+col)%5 == 0 {
				dc.SetBlendMode(render.BlendMultiply)
				dc.SetRGBA(1, 0.7, 0.5, 1)
				dc.DrawCircle(x+12, y+12, 8)
				_ = dc.Fill()
				dc.SetBlendMode(render.BlendNormal)
			}
			if (row+col)%4 == 0 {
				dc.PushLayer(render.BlendNormal, 0.5)
				dc.SetRGB(1, 1, 1)
				dc.DrawRectangle(x+2, y+2, 10, 10)
				_ = dc.Fill()
				dc.PopLayer()
			}
			if col%4 == 0 {
				dc.SetRGB(0.05, 0.05, 0.08)
				dc.DrawString(fmt.Sprintf("%d", (row*16+col)%100), x+3, y+15)
			}
		}
	}
	dc.ResetClip()

	// overlay badge
	dc.PushLayer(render.BlendNormal, 0.9)
	dc.SetRGB(0.9, 0.25, 0.3)
	dc.DrawRoundedRectangle(w-90, 16, 70, 24, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("STRESS", w-80, 32)
	dc.PopLayer()

	compMinGPU(t, dc, 80)
	r, g, b, _ := p1Sample(dc, 28, 28)
	if b < 80 {
		t.Fatalf("D120 cell not blue-ish rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-50, 24)
	if r2 < 100 {
		t.Fatalf("D120 badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}
