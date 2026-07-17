//go:build !nogpu

package render_test

// Phase A chi composition probes D181+ — more complex multi-axis stress.
// docs/P1_COMPOSITION_MATRIX.md

import (
	"fmt"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

func TestP1_Comp_D181_ChatThreadReactionsComposer(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D181_ChatThreadReactionsComposer"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D181_ChatThreadReactionsComposer")
		return
	}
	const w, h = 420, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for i := 0; i < 6; i++ {
		y := 16 + float64(i)*44
		mine := i%2 == 0
		x := 16.0
		if mine {
			x = 140
			dc.SetRGB(0.25, 0.55, 0.95)
		} else {
			dc.SetRGB(1, 1, 1)
		}
		dc.DrawRoundedRectangle(x, y, 240, 36, 12)
		_ = dc.Fill()
		if !mine {
			dc.SetRGB(0.15, 0.16, 0.2)
		} else {
			dc.SetRGB(1, 1, 1)
		}
		dc.DrawString(fmt.Sprintf("message body %d", i), x+12, y+22)
		if i == 2 {
			dc.SetRGB(0.95, 0.4, 0.35)
			dc.DrawRoundedRectangle(x+8, y+28, 36, 16, 8)
			_ = dc.Fill()
		}
	}
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRoundedRectangle(12, h-56, w-24, 44, 12)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.75, 0.45)
	dc.DrawRoundedRectangle(w-90, h-48, 64, 28, 8)
	_ = dc.Fill()
	compMinGPU(t, dc, 12)
	r, g, b, _ := p1Sample(dc, w-60, h-34)
	if g < 80 {
		t.Fatalf("D181 send missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 160, 120)
	if r2 < 80 {
		t.Fatalf("D181 reaction missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

func TestP1_Comp_D182_InboxFiltersBulkActions(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D182_InboxFiltersBulkActions"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D182_InboxFiltersBulkActions")
		return
	}
	const w, h = 520, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(0, 0, 140, h)
	_ = dc.Fill()
	for i, s := range []string{"Inbox", "Starred", "Sent", "Spam"} {
		y := 24 + float64(i)*36
		if i == 0 {
			dc.SetRGB(0.25, 0.5, 0.9)
			dc.DrawRoundedRectangle(8, y-8, 124, 28, 6)
			_ = dc.Fill()
		}
		dc.SetRGB(0.9, 0.92, 0.95)
		dc.DrawString(s, 20, y+8)
	}
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(140, 0, w-140, h)
	_ = dc.Fill()
	// bulk bar
	dc.SetRGB(0.2, 0.45, 0.85)
	dc.DrawRectangle(140, 0, w-140, 36)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("3 selected  Archive  Delete", 160, 24)
	for i := 0; i < 7; i++ {
		y := 48 + float64(i)*36
		if i < 3 {
			dc.SetRGB(0.85, 0.92, 1)
			dc.DrawRectangle(140, y, w-140, 36)
			_ = dc.Fill()
			dc.SetRGB(0.25, 0.55, 0.95)
			dc.DrawRectangle(148, y+10, 14, 14)
			_ = dc.Fill()
		}
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("mail subject line %d", i), 180, y+22)
	}
	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 200, 18)
	if b < 80 {
		t.Fatalf("D182 bulk bar missing rgba=%d,%d,%d", r, g, b)
	}
}

func TestP1_Comp_D183_SettingsSearchAnchoredSections(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D183_SettingsSearchAnchoredSections"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D183_SettingsSearchAnchoredSections")
		return
	}
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(20, 16, w-40, 36, 8)
	_ = dc.Fill()
	dc.SetRGB(0.5, 0.55, 0.65)
	dc.DrawString("Search settings...", 32, 38)
	for i, sec := range []string{"General", "Privacy", "Network", "Advanced"} {
		y := 70 + float64(i)*56
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(sec, 28, y)
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(28, y+8, w-56, 28, 6)
		_ = dc.Fill()
		if i == 1 {
			dc.SetRGB(0.9, 0.35, 0.3)
			dc.DrawRoundedRectangle(w-100, y+12, 50, 20, 6)
			_ = dc.Fill()
		}
	}
	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, w-80, 140)
	if r < 100 {
		t.Fatalf("D183 accent toggle missing rgba=%d,%d,%d", r, g, b)
	}
}

func TestP1_Comp_D184_BoardFiltersSwimLanesCollapsed(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D184_BoardFiltersSwimLanesCollapsed"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D184_BoardFiltersSwimLanesCollapsed")
		return
	}
	const w, h = 520, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// filter chips
	for i, c := range []string{"All", "Mine", "Blocked"} {
		x := 16 + float64(i)*90
		if i == 2 {
			dc.SetRGB(0.9, 0.35, 0.35)
		} else {
			dc.SetRGB(0.25, 0.5, 0.9)
		}
		dc.DrawRoundedRectangle(x, 12, 80, 26, 13)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(c, x+18, 30)
	}
	// lanes
	y := 56.0
	for li, name := range []string{"Engineering", "Design", "Ops"} {
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString("▾ "+name, 16, y+14)
		if li == 1 {
			// collapsed
			dc.SetRGB(0.85, 0.87, 0.9)
			dc.DrawRectangle(16, y+20, w-32, 2)
			_ = dc.Fill()
			y += 40
			continue
		}
		for c := 0; c < 3; c++ {
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(16+float64(c)*160, y+24, 148, 48, 8)
			_ = dc.Fill()
			dc.SetRGB(0.3+float64(c)*0.15, 0.5, 0.85)
			dc.DrawRectangle(16+float64(c)*160, y+24, 6, 48)
			_ = dc.Fill()
		}
		y += 90
	}
	compMinGPU(t, dc, 12)
	r, g, b, _ := p1Sample(dc, 200, 24)
	if r < 100 {
		t.Fatalf("D184 blocked chip missing rgba=%d,%d,%d", r, g, b)
	}
}

func TestP1_Comp_D185_PivotTableHeatCells(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D185_PivotTableHeatCells"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D185_PivotTableHeatCells")
		return
	}
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			x, y := 40+float64(col)*45, 40+float64(row)*28
			v := float64((row*3+col*5)%10) / 10
			dc.SetRGB(0.2+v*0.7, 0.3, 0.9-v*0.5)
			dc.DrawRectangle(x, y, 43, 26)
			_ = dc.Fill()
			dc.SetRGB(1, 1, 1)
			dc.DrawString(fmt.Sprintf("%.1f", v), x+8, y+17)
		}
	}
	dc.SetRGB(0.15, 0.16, 0.2)
	for i := 0; i < 8; i++ {
		dc.DrawString(fmt.Sprintf("R%d", i), 8, 55+float64(i)*28)
		dc.DrawString(fmt.Sprintf("C%d", i), 50+float64(i)*45, 28)
	}
	compMinGPU(t, dc, 40)
	// high-v cell row0 col9 would be out; use mid cell with strong blue/red
	// row=0,col=7 => v=((0+35)%10)/10=0.5 → rgb≈0.55,0.3,0.65
	r, g, b, _ := p1Sample(dc, 40+7*45+20, 40+0*28+12)
	if r > 230 && g > 230 && b > 230 {
		// scan heat field for any non-white
		hits := 0
		for y := 40; y < 250; y += 4 {
			for x := 40; x < 400; x += 4 {
				rr, gg, bb, _ := p1Sample(dc, x, y)
				if rr < 230 || gg < 230 || bb < 230 {
					hits++
				}
			}
		}
		if hits < 20 {
			t.Fatalf("D185 heat still near-white rgba=%d,%d,%d hits=%d", r, g, b, hits)
		}
	}
}

func TestP1_Comp_D186_GanttDependencyArrowsToday(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D186_GanttDependencyArrowsToday"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D186_GanttDependencyArrowsToday")
		return
	}
	const w, h = 520, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// today line
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.SetLineWidth(2)
	dc.DrawLine(280, 20, 280, h-20)
	_ = dc.Stroke()
	bars := []struct{ y, x, bw float64 }{{50, 40, 120}, {100, 140, 160}, {150, 200, 100}, {200, 260, 140}}
	for i, b := range bars {
		dc.SetRGB(0.25+float64(i)*0.1, 0.5, 0.85-float64(i)*0.1)
		dc.DrawRoundedRectangle(b.x, b.y, b.bw, 28, 6)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("Task %d", i+1), 8, b.y+18)
	}
	// dependency
	dc.SetRGB(0.4, 0.45, 0.55)
	dc.SetLineWidth(1.5)
	dc.DrawLine(160, 78, 160, 100)
	_ = dc.Stroke()
	dc.DrawLine(160, 100, 140, 100)
	_ = dc.Stroke()
	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 280, 100)
	if r < 80 {
		t.Fatalf("D186 today line missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 80, 60)
	p1NotNearWhite(t, "D186 bar", r2, g2, b2)
}

func TestP1_Comp_D187_CarouselPeekDotsProgress(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D187_CarouselPeekDotsProgress"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D187_CarouselPeekDotsProgress")
		return
	}
	const w, h = 420, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// peeks
	dc.SetRGB(0.3, 0.35, 0.45)
	dc.DrawRoundedRectangle(-40, 30, 120, 140, 12)
	_ = dc.Fill()
	dc.DrawRoundedRectangle(w-80, 30, 120, 140, 12)
	_ = dc.Fill()
	// active
	dc.SetRGB(0.25, 0.55, 0.9)
	dc.DrawRoundedRectangle(80, 24, 260, 152, 14)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Slide 3 / 8", 160, 100)
	// dots
	for i := 0; i < 8; i++ {
		x := 140 + float64(i)*18
		if i == 2 {
			dc.SetRGB(0.95, 0.4, 0.35)
		} else {
			dc.SetRGB(0.5, 0.55, 0.65)
		}
		dc.DrawCircle(x, 200, 5)
		_ = dc.Fill()
	}
	// progress
	dc.SetRGB(0.3, 0.32, 0.38)
	dc.DrawRoundedRectangle(40, 220, w-80, 6, 3)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.8, 0.5)
	dc.DrawRoundedRectangle(40, 220, 160, 6, 3)
	_ = dc.Fill()
	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 150, 100)
	if b < 80 {
		t.Fatalf("D187 slide missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 176, 200)
	if r2 < 100 {
		t.Fatalf("D187 active dot missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

func TestP1_Comp_D188_OrgChartConnectorsCollapse(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D188_OrgChartConnectorsCollapse"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D188_OrgChartConnectorsCollapse")
		return
	}
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// root
	dc.SetRGB(0.25, 0.5, 0.9)
	dc.DrawRoundedRectangle(180, 20, 120, 44, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("CEO", 220, 46)
	// connectors
	dc.SetRGB(0.6, 0.65, 0.75)
	dc.SetLineWidth(2)
	dc.DrawLine(240, 64, 240, 90)
	_ = dc.Stroke()
	dc.DrawLine(100, 90, 380, 90)
	_ = dc.Stroke()
	for i, name := range []string{"Eng", "Sales", "Ops"} {
		x := 60 + float64(i)*140
		dc.DrawLine(x+50, 90, x+50, 110)
		_ = dc.Stroke()
		dc.SetRGB(0.9, 0.4, 0.35)
		if i == 1 {
			dc.SetRGB(0.3, 0.75, 0.45)
		}
		dc.DrawRoundedRectangle(x, 110, 100, 40, 8)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(name, x+30, 134)
		if i != 1 {
			// children
			dc.SetRGB(0.2, 0.45, 0.85)
			dc.DrawRoundedRectangle(x+10, 180, 80, 32, 6)
			_ = dc.Fill()
		}
	}
	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 220, 40)
	if b < 80 {
		t.Fatalf("D188 root missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 250, 130)
	if g2 < 80 {
		t.Fatalf("D188 sales node missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

func TestP1_Comp_D189_MindmapRadialBranches(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D189_MindmapRadialBranches"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D189_MindmapRadialBranches")
		return
	}
	const w, h = 420, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.12, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	cx, cy := 210.0, 160.0
	dc.SetRGB(0.95, 0.45, 0.3)
	dc.DrawCircle(cx, cy, 28)
	_ = dc.Fill()
	for i := 0; i < 8; i++ {
		a := float64(i) / 8 * 2 * math.Pi
		x := cx + 110*math.Cos(a)
		y := cy + 90*math.Sin(a)
		dc.SetRGB(0.4, 0.55, 0.85)
		dc.SetLineWidth(2)
		dc.DrawLine(cx, cy, x, y)
		_ = dc.Stroke()
		dc.SetRGB(0.3+float64(i%3)*0.15, 0.6, 0.9-float64(i%4)*0.1)
		dc.DrawRoundedRectangle(x-30, y-14, 60, 28, 8)
		_ = dc.Fill()
	}
	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 210, 160)
	if r < 100 {
		t.Fatalf("D189 center missing rgba=%d,%d,%d", r, g, b)
	}
}

func TestP1_Comp_D190_VideoEditorTimelineTracks(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D190_VideoEditorTimelineTracks"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D190_VideoEditorTimelineTracks")
		return
	}
	const w, h = 560, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// preview
	dc.SetRGB(0.2, 0.22, 0.28)
	dc.DrawRoundedRectangle(12, 12, 200, 120, 8)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawCircle(112, 72, 16)
	_ = dc.Fill()
	// tracks
	for t0 := 0; t0 < 4; t0++ {
		y := 150 + float64(t0)*34
		dc.SetRGB(0.16, 0.18, 0.22)
		dc.DrawRectangle(0, y, w, 32)
		_ = dc.Fill()
		dc.SetRGB(0.3+float64(t0)*0.1, 0.5, 0.85-float64(t0)*0.1)
		dc.DrawRoundedRectangle(40+float64(t0)*30, y+4, 160+float64(t0)*20, 24, 4)
		_ = dc.Fill()
	}
	// playhead
	dc.SetRGB(0.95, 0.85, 0.2)
	dc.SetLineWidth(2)
	dc.DrawLine(180, 140, 180, h)
	_ = dc.Stroke()
	compMinGPU(t, dc, 12)
	r, g, b, _ := p1Sample(dc, 112, 72)
	if r < 100 {
		t.Fatalf("D190 preview play missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 180, 180)
	if r2 < 100 && g2 < 100 {
		t.Fatalf("D190 playhead missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

func TestP1_Comp_D191_SchemaERDiagramCrowFoot(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D191_SchemaERDiagramCrowFoot"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D191_SchemaERDiagramCrowFoot")
		return
	}
	const w, h = 480, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// tables
	for i, name := range []string{"users", "orders", "items"} {
		x := 30 + float64(i)*150
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 60, 120, 140, 6)
		_ = dc.Fill()
		dc.SetRGB(0.25, 0.5, 0.9)
		dc.DrawRectangle(x, 60, 120, 28)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(name, x+30, 78)
		dc.SetRGB(0.2, 0.22, 0.28)
		for r := 0; r < 4; r++ {
			dc.DrawString(fmt.Sprintf("col_%d", r), x+10, 110+float64(r)*20)
		}
	}
	// relation
	dc.SetRGB(0.9, 0.4, 0.3)
	dc.SetLineWidth(2)
	dc.DrawLine(150, 130, 180, 130)
	_ = dc.Stroke()
	dc.DrawLine(300, 130, 330, 130)
	_ = dc.Stroke()
	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 60, 70)
	if b < 80 {
		t.Fatalf("D191 table header missing rgba=%d,%d,%d", r, g, b)
	}
}

func TestP1_Comp_D192_LogStreamSeverityFilters(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D192_LogStreamSeverityFilters"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D192_LogStreamSeverityFilters")
		return
	}
	const w, h = 500, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.08, 0.09, 0.12)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// severity chips
	cols := [][3]float64{{0.3, 0.8, 0.5}, {0.95, 0.8, 0.2}, {0.95, 0.35, 0.35}, {0.6, 0.4, 0.9}}
	for i, name := range []string{"INFO", "WARN", "ERROR", "FATAL"} {
		dc.SetRGB(cols[i][0], cols[i][1], cols[i][2])
		dc.DrawRoundedRectangle(12+float64(i)*70, 10, 64, 22, 6)
		_ = dc.Fill()
		_ = name
	}
	for i := 0; i < 12; i++ {
		y := 44 + float64(i)*20
		sev := i % 4
		dc.SetRGB(cols[sev][0], cols[sev][1], cols[sev][2])
		dc.DrawRectangle(8, y, 6, 16)
		_ = dc.Fill()
		dc.SetRGB(0.8, 0.85, 0.9)
		dc.DrawString(fmt.Sprintf("2026-07-15 log line entry %02d", i), 24, y+12)
	}
	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 40, 18)
	if g < 80 {
		t.Fatalf("D192 info chip missing rgba=%d,%d,%d", r, g, b)
	}
}

func TestP1_Comp_D193_SitemapHierarchyConnectors(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D193_SitemapHierarchyConnectors"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D193_SitemapHierarchyConnectors")
		return
	}
	const w, h = 460, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.55, 0.9)
	dc.DrawRoundedRectangle(180, 16, 100, 36, 6)
	_ = dc.Fill()
	levels := [][]string{{"Home"}, {"Products", "Blog", "About"}, {"A", "B", "C", "D", "E", "F"}}
	ys := []float64{16, 100, 200}
	// simplified second level
	for i, n := range levels[1] {
		x := 40 + float64(i)*140
		dc.SetRGB(0.9, 0.4, 0.35)
		dc.DrawRoundedRectangle(x, 100, 100, 32, 6)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(n, x+24, 120)
		dc.SetRGB(0.6, 0.65, 0.75)
		dc.DrawLine(230, 52, x+50, 100)
		_ = dc.Stroke()
	}
	for i := 0; i < 6; i++ {
		x := 20 + float64(i)*70
		dc.SetRGB(0.3, 0.7, 0.5)
		dc.DrawRoundedRectangle(x, 200, 60, 28, 4)
		_ = dc.Fill()
	}
	_ = ys
	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 200, 30)
	if b < 80 {
		t.Fatalf("D193 root missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 50, 210)
	if g2 < 80 {
		t.Fatalf("D193 leaf missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

func TestP1_Comp_D194_NotebookCellsOutputFold(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D194_NotebookCellsOutputFold"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D194_NotebookCellsOutputFold")
		return
	}
	const w, h = 480, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.97, 0.97, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	y := 16.0
	for i := 0; i < 3; i++ {
		// prompt
		dc.SetRGB(0.2, 0.55, 0.35)
		dc.DrawString(fmt.Sprintf("In [%d]:", i+1), 12, y+16)
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawRoundedRectangle(70, y, w-90, 48, 6)
		_ = dc.Fill()
		dc.SetRGB(0.4, 0.85, 0.55)
		dc.DrawString("print(result)", 84, y+28)
		y += 56
		// output
		if i != 1 {
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(70, y, w-90, 40, 6)
			_ = dc.Fill()
			dc.SetRGB(0.9, 0.35, 0.3)
			dc.DrawRectangle(70, y, 4, 40)
			_ = dc.Fill()
			y += 52
		} else {
			dc.SetRGB(0.7, 0.72, 0.78)
			dc.DrawString("▾ output folded", 70, y+14)
			y += 30
		}
	}
	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 80, 80)
	if r < 40 && g < 40 {
		// code cell dark
	}
	r2, g2, b2, _ := p1Sample(dc, 72, 120)
	if r2 < 100 {
		// may be folded text area - check first output gutter
		r2, g2, b2, _ = p1Sample(dc, 72, 72)
	}
	_ = r
	_ = g
	_ = b
	// green code text scan
	hits := 0
	for yy := 20; yy < 200; yy += 2 {
		for x := 84; x < 200; x += 4 {
			rr, gg, bb, _ := p1Sample(dc, x, yy)
			if gg > rr+10 && gg > 50 {
				hits++
			}
			_ = bb
		}
	}
	if hits < 3 {
		t.Fatalf("D194 code green ink low=%d sample=%d,%d,%d", hits, r2, g2, b2)
	}
}

func TestP1_Comp_D195_CMSBlockEditorNest(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D195_CMSBlockEditorNest"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D195_CMSBlockEditorNest")
		return
	}
	const w, h = 440, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// blocks
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(24, 24, w-48, 60, 8)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Heading block", 40, 58)
	// nested group
	dc.SetRGB(0.9, 0.93, 1)
	dc.DrawRoundedRectangle(24, 100, w-48, 160, 8)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.5, 0.9)
	dc.SetLineWidth(2)
	dc.SetDash(4, 3)
	dc.DrawRoundedRectangle(24, 100, w-48, 160, 8)
	_ = dc.Stroke()
	dc.SetDash()
	for i := 0; i < 2; i++ {
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(40, 120+float64(i)*60, w-80, 48, 6)
		_ = dc.Fill()
		dc.SetRGB(0.9, 0.4, 0.35)
		dc.DrawRectangle(40, 120+float64(i)*60, 6, 48)
		_ = dc.Fill()
	}
	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, 42, 140)
	if r < 100 {
		t.Fatalf("D195 nested accent missing rgba=%d,%d,%d", r, g, b)
	}
}

func TestP1_Comp_D196_ShopProductGalleryVariant(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D196_ShopProductGalleryVariant"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D196_ShopProductGalleryVariant")
		return
	}
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// main image
	img := compMakeImage(t, 180, 180, 80, 140, 200)
	dc.DrawImage(img, 24, 24)
	// thumbs
	for i := 0; i < 4; i++ {
		timg := compMakeImage(t, 40, 40, uint8(60+i*40), 100, uint8(180-i*20))
		dc.DrawImage(timg, 24+float64(i)*48, 220)
	}
	// details
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Product Title", 230, 50)
	dc.SetRGB(0.9, 0.35, 0.3)
	dc.DrawString("$129.00", 230, 80)
	for i, v := range []string{"S", "M", "L", "XL"} {
		x := 230 + float64(i)*48
		if i == 1 {
			dc.SetRGB(0.25, 0.55, 0.95)
		} else {
			dc.SetRGB(0.9, 0.91, 0.93)
		}
		dc.DrawRoundedRectangle(x, 110, 40, 32, 6)
		_ = dc.Fill()
		dc.SetRGB(0.1, 0.1, 0.12)
		if i == 1 {
			dc.SetRGB(1, 1, 1)
		}
		dc.DrawString(v, x+12, 130)
	}
	dc.SetRGB(0.25, 0.7, 0.45)
	dc.DrawRoundedRectangle(230, 170, 160, 40, 8)
	_ = dc.Fill()
	compMinGPU(t, dc, 8)
	r, g, b, _ := p1Sample(dc, 50, 50)
	p1NotNearWhite(t, "D196 image", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 250, 120)
	if b2 < 80 {
		t.Fatalf("D196 variant missing rgba=%d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, 280, 185)
	if g3 < 80 {
		t.Fatalf("D196 CTA missing rgba=%d,%d,%d", r3, g3, b3)
	}
}

func TestP1_Comp_D197_AdminCRUDTableRowActions(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D197_AdminCRUDTableRowActions"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D197_AdminCRUDTableRowActions")
		return
	}
	const w, h = 520, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.2, 0.45, 0.85)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Users Admin", 16, 26)
	dc.SetRGB(0.3, 0.8, 0.5)
	dc.DrawRoundedRectangle(w-100, 8, 80, 24, 6)
	_ = dc.Fill()
	// table header
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawRectangle(0, 40, w, 28)
	_ = dc.Fill()
	for i := 0; i < 6; i++ {
		y := 68 + float64(i)*34
		if i%2 == 0 {
			dc.SetRGB(1, 1, 1)
		} else {
			dc.SetRGB(0.97, 0.98, 0.99)
		}
		dc.DrawRectangle(0, y, w, 34)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawString(fmt.Sprintf("user_%d@mail", i), 16, y+20)
		// actions
		dc.SetRGB(0.25, 0.55, 0.9)
		dc.DrawRoundedRectangle(w-140, y+6, 50, 22, 4)
		_ = dc.Fill()
		dc.SetRGB(0.9, 0.35, 0.35)
		dc.DrawRoundedRectangle(w-80, y+6, 50, 22, 4)
		_ = dc.Fill()
	}
	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, w-50, 20)
	if g < 80 {
		t.Fatalf("D197 create missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-60, 85)
	if r2 < 100 {
		t.Fatalf("D197 delete action missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

func TestP1_Comp_D198_OnboardingCoachmarksOverlay(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D198_OnboardingCoachmarksOverlay"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D198_OnboardingCoachmarksOverlay")
		return
	}
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// app chrome
	for i := 0; i < 4; i++ {
		dc.SetRGB(0.9, 0.91, 0.93)
		dc.DrawRoundedRectangle(20, 20+float64(i)*50, w-40, 40, 6)
		_ = dc.Fill()
	}
	// dim
	dc.SetRGBA(0.05, 0.06, 0.08, 0.55)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// spotlight hole simulation: redraw bright target
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRoundedRectangle(40, 70, 180, 40, 6)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.55, 0.95)
	dc.SetLineWidth(3)
	dc.DrawRoundedRectangle(38, 68, 184, 44, 8)
	_ = dc.Stroke()
	// coachmark
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRoundedRectangle(240, 60, 150, 80, 10)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("Click here next", 252, 95)
	dc.SetRGB(0.3, 0.75, 0.45)
	dc.DrawRoundedRectangle(252, 110, 70, 22, 6)
	_ = dc.Fill()
	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 270, 120)
	if g < 80 {
		t.Fatalf("D198 coach CTA missing rgba=%d,%d,%d", r, g, b)
	}
	// dimmed corner should be darker than original light chrome (~230+)
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 > 200 && g2 > 200 && b2 > 200 {
		t.Fatalf("D198 dim missing rgba=%d,%d,%d", r2, g2, b2)
	}
	// spotlight ring blue border
	r3, g3, b3, _ := p1Sample(dc, 38, 70)
	if b3 < 80 {
		t.Fatalf("D198 spotlight ring missing rgba=%d,%d,%d", r3, g3, b3)
	}
}

func TestP1_Comp_D199_StatusPageIncidentsTimeline(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D199_StatusPageIncidentsTimeline"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D199_StatusPageIncidentsTimeline")
		return
	}
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// status banner
	dc.SetRGB(0.3, 0.75, 0.45)
	dc.DrawRoundedRectangle(16, 16, w-32, 48, 10)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("All Systems Operational", 40, 44)
	// components
	for i, s := range []string{"API", "Web", "DB", "CDN"} {
		y := 84 + float64(i)*36
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(16, y, w-32, 32, 6)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(s, 28, y+20)
		if i == 2 {
			dc.SetRGB(0.95, 0.7, 0.2)
		} else {
			dc.SetRGB(0.3, 0.8, 0.45)
		}
		dc.DrawCircle(w-40, y+16, 8)
		_ = dc.Fill()
	}
	// incident timeline
	dc.SetRGB(0.9, 0.35, 0.35)
	dc.DrawRoundedRectangle(16, 240, w-32, 60, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Incident: DB elevated latency", 32, 272)
	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 40, 40)
	if g < 80 {
		t.Fatalf("D199 banner missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 40, 260)
	if r2 < 100 {
		t.Fatalf("D199 incident missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

func TestP1_Comp_D200_KitchenSinkV7CompositionBlast(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D200_KitchenSinkV7CompositionBlast"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D200_KitchenSinkV7CompositionBlast")
		return
	}
	const w, h = 640, 440
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)
	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRoundRect(4, 4, w-8, h-8, 12)
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("KitchenSink v7 blast — dense arbitrary composition", 12, 24)
	// multi panels
	for i := 0; i < 4; i++ {
		x := 12 + float64(i)*155
		dc.SetRGB(0.16, 0.18, 0.22)
		dc.DrawRoundedRectangle(x, 48, 148, 200, 10)
		_ = dc.Fill()
		dc.SetRGB(0.25+float64(i)*0.1, 0.5, 0.85-float64(i)*0.1)
		dc.DrawRoundedRectangle(x+12, 64, 124, 36, 6)
		_ = dc.Fill()
		for r := 0; r < 5; r++ {
			dc.SetRGB(0.25, 0.28, 0.34)
			dc.DrawRoundedRectangle(x+12, 112+float64(r)*24, 124, 18, 4)
			_ = dc.Fill()
		}
	}
	// blend overlay
	dc.SetBlendMode(render.BlendMultiply)
	dc.SetRGBA(1, 0.6, 0.4, 1)
	dc.DrawCircle(320, 280, 50)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)
	// mesh
	dc.DrawVertices([]render.Point{{X: 480, Y: 260}, {X: 560, Y: 280}, {X: 500, Y: 340}}, []render.RGBA{
		{R: 1, G: 0.3, B: 0.3, A: 1}, {R: 0.3, G: 1, B: 0.3, A: 1}, {R: 0.3, G: 0.4, B: 1, A: 1},
	}, render.VertexModeTriangles)
	// toast
	dc.SetRGB(0.3, 0.75, 0.45)
	dc.DrawRoundedRectangle(w-160, h-48, 140, 30, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("v7 blast ok", w-140, h-28)
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.1})
	dc.ResetClip()
	compMinGPU(t, dc, 30)
	r, g, b, _ := p1Sample(dc, 40, 80)
	p1NotNearWhite(t, "D200 panel", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, w-90, h-35)
	if g2 < 80 {
		t.Fatalf("D200 toast missing rgba=%d,%d,%d", r2, g2, b2)
	}
}
