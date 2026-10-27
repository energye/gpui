package main

import (
	"fmt"
	"math"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

const (
	listW      = 620.0
	listBodyH  = 656.0
	listLegend = 260.0
	listRight  = 268.0
	listGap    = 16.0
	itemCount  = 1200
	freezeY    = 1234.0
)

func rowHeight(i int) float64 {
	switch i % 4 {
	case 0:
		return 44
	case 1:
		return 92
	case 2:
		return 60
	default:
		return 76
	}
}

func listTotalHeight() float64 {
	var total float64
	for i := 0; i < itemCount; i++ {
		total += rowHeight(i)
	}
	return total
}

// buildListScene mirrors tools/ruler_flutter list exactly where the public
// rendering API allows: 48 header, 12px body padding, 260 legend, 620 list,
// 16 gap, 268 static panel, 72 HUD, 1200 variable rows (44/92/60/76), 120px
// cache, 24 96x56 checkerboard images, Steady/Spike/Recover at 160/2600 px/s.
func buildListScene(gate bool) (*sceneSpec, error) {
	root := newRoot(winListW, winListH, colBg)
	addHeader(root, "gpui parity — list scroll (C3 semantic, 1200 variable rows)",
		"P3 parity ruler · RULER_SCENE=list")

	legend := newSolid(listLegend, listBodyH, colPanel)
	root.Place(legend, 12, 60)
	ly := 14.0
	lt := labelAt(legend, "C3 SCROLL", 14, 14, ly, colLabel)
	ly += textHeight(lt) + 12
	legendRows := [][2]string{
		{"C3 列表语义", "1200 项变高行"},
		{"行高", "44 / 92 / 60 / 76"},
		{"行型", "文本 / 色块 / chip / 图"},
		{"相位", "Steady / Spike / Recover"},
		{"滚动", "160 → 2600 px/s"},
		{"缓存", "120px (2 行)"},
		{"静态区", "右侧 4x4 + legend"},
		{"对照臂", "tools/ruler_flutter list"},
	}
	for _, row := range legendRows {
		l := labelAt(legend, row[0], 11, 14, ly, colMuted)
		ly += textHeight(l) + 2
		v := labelAt(legend, row[1], 13, 14, ly, colInk)
		ly += textHeight(v) + 12
	}

	// 24 committed 96x56 checkerboards, identical to the Flutter assets.
	srcs := make([]*render.ImageBuf, 24)
	for i := range srcs {
		p := testdataPath(fmt.Sprintf("c3_src_%02d.png", i))
		srcs[i] = ensureImage(p, 96, 56)
		if srcs[i] == nil {
			return nil, fmt.Errorf("list image %s missing", p)
		}
	}

	list := rendering.NewVariableVirtualList(itemCount, 60, rowHeight, func(i int) rendering.RenderObject {
		h := rowHeight(i)
		cell := rendering.NewAbsoluteBox(listW, h)
		switch i % 4 {
		case 0:
			t := wrkit.Label(fmt.Sprintf("row %04d · text line — C3 semantic", i), 15, colInk.R, colInk.G, colInk.B)
			cell.Place(t, 14, 12)
		case 1:
			base := rendering.NewRenderColorBox(listW-28, h-20,
				float64(40+(i%7)*12)/255, float64(89+(i%5)*10)/255, 140.0/255, 1)
			cell.Place(base, 14, 10)
			inner := rendering.NewRenderColorBox(140, 24, 0xF2/255.0, 0x8C/255.0, 0x26/255.0, 1)
			cell.Place(inner, 28, 10+(h-20)/2-12)
		case 2:
			t := wrkit.Label(fmt.Sprintf("row %04d · mixed text + chip", i), 14, 0xB8/255.0, 0xD9/255.0, 0xBF/255.0)
			cell.Place(t, 14, 8)
			chip := rendering.NewRenderColorBox(72, 18,
				float64(80+(i%6)*15)/255, 165.0/255, 90.0/255, 1)
			cell.Place(chip, listW-14-72, 30)
		default:
			src := i % 24
			im := rendering.NewRenderImage(96, 56)
			im.SetImageShared(srcs[src])
			cell.Place(im, 14, 10)
			t := wrkit.Label(fmt.Sprintf("row %04d · image #%d", i, src), 13, 0xCC/255.0, 0xD2/255.0, 0xE6/255.0)
			cell.Place(t, 122, 28)
		}
		return cell
	})
	list.CacheExtent = 120

	vp := rendering.NewRenderViewport(list)
	vp.FixedWidth = listW
	vp.FixedHeight = listBodyH
	root.Place(vp, 284, 60)

	maxY := listTotalHeight() - listBodyH
	if maxY < 0 {
		maxY = 0
	}
	vp.SetMaxScrollY(maxY)
	initialY := 0.0
	if gate {
		initialY = freezeY
		if initialY > maxY {
			initialY = maxY
		}
	}
	vp.SetScrollOffset(0, initialY)

	// Flutter's Scrollbar default: no track, 6px thumb, 3px radius.
	track := rendering.NewAbsoluteBox(6, listBodyH)
	root.Place(track, 284+listW-8, 60)
	thumb := rendering.NewRenderColorBox(6, 56, 0.45, 0.75, 0.95, 0.9)
	thumb.SetRepaintBoundary(true)
	thumbAlign := rendering.NewRenderAlignBox(thumb, 0.5, 0)
	track.Place(thumbAlign, 0, 0)
	thumbAlign.SetAlignment(0.5, initialY/maxY)

	right := newSolid(listRight, listBodyH, colPanel)
	root.Place(right, 920, 60)
	labelAt(right, "STATIC DENSE", 12, 12, 12, colLabel)
	for ry := 0; ry < 4; ry++ {
		for rx := 0; rx < 4; rx++ {
			c := rendering.NewRenderColorBox(48, 48,
				float64(36+(rx+ry)*8)/255,
				float64(115+(rx+ry)*6)/255,
				float64(77+rx*ry*8)/255, 1)
			right.Place(c, 12+float64(rx)*58, 34+float64(ry)*58)
		}
	}
	for k, s := range []string{
		"不随滚动重绘", "4x4 nested cells", "range / image counters",
		"scroll thumb", "same content every frame",
		"no rebuild of static panel", "C3 semantic mirror",
	} {
		labelAt(right, s, 12, 12, 260+float64(k)*20, colMuted)
	}

	phase, hy := "Steady", initialY
	first := int(hy / 68)
	last := first + 16
	if last > itemCount {
		last = itemCount
	}
	hud := addHUD(root, "C3 / Flutter",
		fmt.Sprintf("%s  y=%.0f  rows ~%d..%d / %d", phase, hy, first, last, itemCount),
		"60fps target · /proc CPU sampled externally")

	elapsed := 0.0
	direction := 1.0
	lastHud := 0.0
	if gate {
		lastHud = -1
	}
	tick := func(dt float64) bool {
		if dt <= 0 || dt > 0.25 {
			return true
		}
		elapsed += dt
		ct := math.Mod(elapsed, 9)
		speed := 160.0
		switch {
		case ct < 3:
			phase = "Steady"
			speed = 160
		case ct < 6:
			phase = "Spike"
			speed = 2600
		default:
			phase = "Recover"
			speed = math.Max(40, 2600*(1-(ct-6)/3))
		}
		hy += direction * speed * dt
		if hy >= maxY {
			hy = maxY
			direction = -1
		} else if hy <= 0 {
			hy = 0
			direction = 1
		}
		if math.Abs(vp.ScrollOffset().Y-hy) > 0.01 {
			vp.SetScrollOffset(0, hy)
		}
		if maxY > 0 {
			thumbAlign.SetAlignment(0.5, hy/maxY)
		}
		if elapsed-lastHud >= 0.5 {
			lastHud = elapsed
			first := int(hy / 68)
			last := first + 16
			if last > itemCount {
				last = itemCount
			}
			hud.set(fmt.Sprintf("%s  y=%.0f  rows ~%d..%d / %d", phase, hy, first, last, itemCount))
		}
		return true
	}

	return &sceneSpec{
		name:   sceneList,
		ref:    "tools/ruler_flutter RULER_SCENE=list",
		winW:   winListW,
		winH:   winListH,
		title:  "gpui ruler parity — Flutter list (1200x800)",
		root:   root,
		clearR: colBg.R, clearG: colBg.G, clearB: colBg.B,
		fullPaint: true,
		tick:      tick,
		goldenRects: []wrkit.GoldenRect{
			{X: 0, Y: 0, W: winListW, H: 48},
			{X: 0, Y: 48, W: winListW, H: 680},
		},
		probes: []wrkit.PixelProbe{
			{Name: "list_header", X: 600, Y: 20, R: colHeader.R, G: colHeader.G, B: colHeader.B, Tol: 4},
			{Name: "list_legend", X: 20, Y: 700, R: colPanel.R, G: colPanel.G, B: colPanel.B, Tol: 4},
			{Name: "list_right", X: 1000, Y: 700, R: colPanel.R, G: colPanel.G, B: colPanel.B, Tol: 4},
			{Name: "list_right_cell_00", X: 956, Y: 118, R: 36.0 / 255, G: 115.0 / 255, B: 77.0 / 255, Tol: 6},
			{Name: "list_right_cell_33", X: 1130, Y: 292, R: 84.0 / 255, G: 151.0 / 255, B: 149.0 / 255, Tol: 6},
			{Name: "list_gap_bg", X: 912, Y: 400, R: colBg.R, G: colBg.G, B: colBg.B, Tol: 4},
		},
		cleanup: func() {
			for _, b := range srcs {
				if b != nil {
					b.Dispose()
				}
			}
		},
	}, nil
}
