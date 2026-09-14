package masonry_test

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/masonry"
	"github.com/energye/gpui/ui/rendering"
)

type showcaseMasonryFile struct {
	CanvasW float64 `json:"canvasW"`
	Margin  float64 `json:"margin"`
	Gap     float64 `json:"gap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
	Basic struct {
		Columns int       `json:"columns"`
		Gutter  float64   `json:"gutter"`
		Heights []float64 `json:"heights"`
	} `json:"basic"`
	Responsive struct {
		Columns  map[string]int     `json:"columns"`
		Gutter   map[string]float64 `json:"gutter"`
		Viewport float64            `json:"viewport"`
		Heights  []float64          `json:"heights"`
	} `json:"responsive"`
	Image struct {
		Columns int       `json:"columns"`
		Gutter  float64   `json:"gutter"`
		Heights []float64 `json:"heights"`
	} `json:"image"`
	Dynamic struct {
		Columns int       `json:"columns"`
		Gutter  float64   `json:"gutter"`
		Heights []float64 `json:"heights"`
	} `json:"dynamic"`
	Single struct {
		Columns int       `json:"columns"`
		Gutter  float64   `json:"gutter"`
		Heights []float64 `json:"heights"`
	} `json:"single"`
	Pair struct {
		Columns int       `json:"columns"`
		GutterH float64   `json:"gutterH"`
		GutterV float64   `json:"gutterV"`
		Heights []float64 `json:"heights"`
	} `json:"pair"`
}

func loadShowcaseMasonry(t *testing.T) showcaseMasonryFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_masonry.json"))
	if err != nil {
		t.Fatalf("read showcase_masonry.json: %v", err)
	}
	var f showcaseMasonryFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse showcase_masonry.json: %v", err)
	}
	if f.CanvasW <= 0 || f.Margin < 0 || f.Gap < 0 {
		t.Fatalf("bad canvas %+v", f)
	}
	if f.Tolerance.MaxDiff <= 0 || f.Tolerance.HardCap <= 0 {
		t.Fatalf("bad tolerance %+v", f.Tolerance)
	}
	if len(f.Basic.Heights) == 0 || len(f.Responsive.Heights) == 0 || len(f.Image.Heights) == 0 ||
		len(f.Dynamic.Heights) == 0 || len(f.Single.Heights) == 0 || len(f.Pair.Heights) == 0 {
		t.Fatalf("showcase heights missing %+v", f)
	}
	return f
}

func showcaseItems(prefix string, heights []float64, r, g, b float64) []masonry.MasonryItem {
	out := make([]masonry.MasonryItem, len(heights))
	for i, h := range heights {
		out[i] = masonry.MasonryItem{
			Key:     prefix + string(rune('a'+i)),
			Column:  -1,
			Height:  h,
			Data:    h,
			Content: rendering.NewRenderColorBox(10, h, r, g, b, 1),
		}
	}
	return out
}

func showcaseRespColumns(src map[string]int) map[masonry.Breakpoint]int {
	out := map[masonry.Breakpoint]int{}
	for k, v := range src {
		out[masonry.Breakpoint(k)] = v
	}
	return out
}

func showcaseRespGutter(src map[string]float64) map[masonry.Breakpoint]float64 {
	out := map[masonry.Breakpoint]float64{}
	for k, v := range src {
		out[masonry.Breakpoint(k)] = v
	}
	return out
}

// TestMasonry_Showcase_MainPaths lays §6.8 main paths on one big canvas:
// basic / responsive / image / dynamic / single / pair-gutter, plus a P1
// semantic hook on the last block. Three evidences: logic probe
// (columns/layout/uneven heights), pixel assertions (blue center, white gap,
// green center), golden file compare (tolerance from
// testdata/showcase_masonry.json). Regenerate with UPDATE_GOLDEN=1.
func TestMasonry_Showcase_MainPaths(t *testing.T) {
	spec := loadShowcaseMasonry(t)
	masonry.ResetGlobalConfig()
	defer masonry.ResetGlobalConfig()

	W := spec.CanvasW
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin

	// Basic: fixed columns + fixed gutter (basic.tsx subset).
	basic := masonry.NewMasonry()
	basic.SetColumns(spec.Basic.Columns)
	basic.SetGutter(spec.Basic.Gutter, spec.Basic.Gutter)
	basicItems := showcaseItems("show-b", spec.Basic.Heights, 0.09, 0.47, 1.0)
	basic.SetItems(basicItems)

	// Responsive: breakpoint columns + gutter (responsive.tsx).
	resp := masonry.NewMasonry()
	resp.SetResponsiveColumns(showcaseRespColumns(spec.Responsive.Columns))
	resp.SetResponsiveGutter(showcaseRespGutter(spec.Responsive.Gutter))
	resp.SetViewportWidth(spec.Responsive.Viewport)
	respItems := showcaseItems("show-r", spec.Responsive.Heights, 0.32, 0.77, 0.10)
	resp.SetItems(respItems)

	// Image: uneven heights (image.tsx).
	img := masonry.NewMasonry()
	img.SetColumns(spec.Image.Columns)
	img.SetGutter(spec.Image.Gutter, spec.Image.Gutter)
	imgItems := showcaseItems("show-p", spec.Image.Heights, 0.98, 0.68, 0.08)
	img.SetItems(imgItems)

	// Dynamic: explicit columns for the first row + relayout callback (dynamic.tsx).
	dyn := masonry.NewMasonry()
	dyn.SetColumns(spec.Dynamic.Columns)
	dyn.SetGutter(spec.Dynamic.Gutter, spec.Dynamic.Gutter)
	dynItems := showcaseItems("show-d", spec.Dynamic.Heights, 1.0, 0.30, 0.31)
	for i := range dynItems {
		if i < spec.Dynamic.Columns {
			dynItems[i].Column = i
		}
	}
	dynCalls := 0
	dyn.SetOnLayoutChange(func(items []masonry.MasonryColumn) { dynCalls++ })
	dyn.SetItems(dynItems)

	// Single column (MAS-S5).
	single := masonry.NewMasonry()
	single.SetColumns(spec.Single.Columns)
	single.SetGutter(spec.Single.Gutter, spec.Single.Gutter)
	singleItems := showcaseItems("show-s", spec.Single.Heights, 0.45, 0.18, 0.82)
	single.SetItems(singleItems)
	single.SetFresh(true)

	// Pair gutter + P1 semantic hook (style-class.tsx structure, strings not measured).
	pair := masonry.NewMasonry()
	pair.SetColumns(spec.Pair.Columns)
	pair.SetGutter(spec.Pair.GutterH, spec.Pair.GutterV)
	pairItems := showcaseItems("show-q", spec.Pair.Heights, 0.08, 0.76, 0.76)
	pair.SetItems(pairItems)
	pair.SetClassName(masonry.SemanticRoot, "show-root")
	pair.SetSemanticStyle(masonry.SemanticItem, masonry.Style{Props: map[string]string{"border": "1px solid #1890ff"}})

	// Logic probe before paint: columns resolve from the file.
	if basic.EffectiveColumnCount() != spec.Basic.Columns {
		t.Fatalf("basic columns=%d want %d", basic.EffectiveColumnCount(), spec.Basic.Columns)
	}
	wantRespCols := spec.Responsive.Columns["md"]
	if resp.EffectiveColumnCount() != wantRespCols {
		t.Fatalf("responsive columns=%d want md=%d", resp.EffectiveColumnCount(), wantRespCols)
	}
	wantRespGut := spec.Responsive.Gutter["md"]
	rh, rv := resp.EffectiveGutter()
	if math.Abs(rh-wantRespGut) > 0.5 || math.Abs(rv-wantRespGut) > 0.5 {
		t.Fatalf("responsive gutter=%v/%v want %v", rh, rv, wantRespGut)
	}
	if !single.Fresh() {
		t.Fatal("single fresh must stick for showcase")
	}
	if pair.ClassName(masonry.SemanticRoot) != "show-root" {
		t.Fatalf("semantic root=%q", pair.ClassName(masonry.SemanticRoot))
	}
	if _, ok := pair.SemanticStyle(masonry.SemanticItem); !ok {
		t.Fatal("semantic item style must stick")
	}
	if dynCalls != 0 {
		t.Fatalf("dynamic pre-layout calls=%d want 0", dynCalls)
	}

	// Layout every block at exact row width; each must be non-zero with
	// visible column height spread where heights are uneven.
	type placed struct {
		m *masonry.Masonry
		x float64
		y float64
		w float64
		h float64
	}
	blocks := []*masonry.Masonry{basic, resp, img, dyn, single, pair}
	var items []placed
	y := margin
	for i, m := range blocks {
		sz := m.Layout(rendering.Loose(rowW, 10000))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("block %d layout=%v want non-zero", i, sz)
		}
		if math.Abs(sz.Width-rowW) > 0.5 {
			t.Fatalf("block %d width=%v want %v", i, sz.Width, rowW)
		}
		ns := m.Node().Size()
		if ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("block %d node=%v want non-zero", i, ns)
		}
		items = append(items, placed{m: m, x: margin, y: y, w: sz.Width, h: sz.Height})
		y += sz.Height + gap
	}
	// Geometry is real: uneven blocks must spread columns, single must stack.
	for _, m := range []*masonry.Masonry{basic, img} {
		hs := m.ColumnHeights()
		mn, mx := hs[0], hs[0]
		for _, h := range hs[1:] {
			if h < mn {
				mn = h
			}
			if h > mx {
				mx = h
			}
		}
		if mx-mn <= 0 {
			t.Fatalf("uneven block heights=%v must spread", hs)
		}
	}
	// Pair vertical gutter accumulates from the file.
	qdTop := pair.ItemBox("show-qd").Min.Y
	if math.Abs(qdTop-(spec.Pair.Heights[0]+spec.Pair.GutterV)) > 0.5 {
		t.Fatalf("pair vertical=%v want %v", qdTop, spec.Pair.Heights[0]+spec.Pair.GutterV)
	}
	// Data rides along (height stored as data in this showcase).
	if got := basic.Items()[0].Data; got != spec.Basic.Heights[0] {
		t.Fatalf("data=%v want %v", got, spec.Basic.Heights[0])
	}
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.m.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel 1: basic first-item center is the section blue.
	b0 := basic.ItemBox("show-ba")
	cx := int(items[0].x + b0.Min.X + b0.Size().Width/2)
	cy := int(items[0].y + b0.Min.Y + b0.Size().Height/2)
	pr, pg, pb, _ := got.At(cx, cy).RGBA()
	r8, g8, b8 := pr/257, pg/257, pb/257
	if !(r8 < 60 && g8 > 90 && g8 < 150 && b8 > 220) {
		t.Fatalf("basic center #%02x%02x%02x want ~blue", r8, g8, b8)
	}

	// Pixel 2: gutter between basic columns 0/1 stays canvas white.
	bh, _ := basic.EffectiveGutter()
	bn := float64(basic.EffectiveColumnCount())
	colW := (rowW + bh) / bn
	itemW := colW - bh
	gx := int(items[0].x + itemW + bh/2)
	gy := int(items[0].y + 10)
	gr, gg, gb, _ := got.At(gx, gy).RGBA()
	if gr/257 < 240 || gg/257 < 240 || gb/257 < 240 {
		t.Fatalf("basic gap #%02x%02x%02x want white", gr/257, gg/257, gb/257)
	}

	// Pixel 3: responsive first-item center is the section green.
	r0 := resp.ItemBox("show-ra")
	rx := int(items[1].x + r0.Min.X + r0.Size().Width/2)
	ry := int(items[1].y + r0.Min.Y + r0.Size().Height/2)
	rr, rg, rb, _ := got.At(rx, ry).RGBA()
	r82, g82, b82 := rr/257, rg/257, rb/257
	if !(r82 < 120 && rg > 0 && g82 > 150 && b82 < 80) {
		t.Fatalf("responsive center #%02x%02x%02x want ~green", r82, g82, b82)
	}

	path := filepath.Join("testdata", "showcase_masonry.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("write showcase: %v", err)
		}
		if err := png.Encode(f, got); err != nil {
			f.Close()
			t.Fatalf("encode showcase: %v", err)
		}
		f.Close()
		t.Logf("showcase rewritten: %s (%dx%d)", path, int(W), int(H))
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open showcase %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode showcase: %v", err)
	}
	if !want.Bounds().Eq(got.Bounds()) {
		t.Fatalf("showcase bounds %v want %v (regen with UPDATE_GOLDEN=1)", got.Bounds(), want.Bounds())
	}
	maxDiff := uint32(spec.Tolerance.MaxDiff * 257)
	hardCap := uint32(spec.Tolerance.HardCap * 257)
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for yy := 0; yy < got.Bounds().Dy(); yy++ {
		for xx := 0; xx < got.Bounds().Dx(); xx++ {
			r1, g1, b1, a1 := got.At(xx, yy).RGBA()
			r2, g2, b2, a2 := want.At(xx, yy).RGBA()
			m := max4show(diffShow(r1, r2), diffShow(g1, g2), diffShow(b1, b2), diffShow(a1, a2))
			if m > hardCap {
				t.Fatalf("showcase pixel (%d,%d) diff %d exceeds hard cap", xx, yy, m/257)
			}
			if m > maxDiff {
				bad++
			}
		}
	}
	if float64(bad)/float64(total) > spec.Tolerance.BadFrac {
		t.Fatalf("showcase bad pixels %d/%d exceed %.1f%%", bad, total, spec.Tolerance.BadFrac*100)
	}
}

func diffShow(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4show(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
