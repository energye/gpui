package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/emoji"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Palette. Exact pixel probes read the same constants, so a palette edit can
// never desync the drawn pixels from the asserted ones.
var (
	colCardBG    = [3]float64{0.12, 0.135, 0.165}
	colTitle     = [3]float64{0.80, 0.86, 0.94}
	colBody      = [3]float64{0.86, 0.90, 0.96}
	colRunText   = [3]float64{0.30, 0.60, 0.95}
	colRunEmoji  = [3]float64{0.95, 0.58, 0.18}
	colCOLRRed   = [3]float64{230.0 / 255, 51.0 / 255, 51.0 / 255}
	colCOLRGreen = [3]float64{51.0 / 255, 204.0 / 255, 77.0 / 255}
	colCOLRBlue  = [3]float64{51.0 / 255, 102.0 / 255, 242.0 / 255}
)

// ---- card geometry (card-local) ----

const (
	gridN    = 6
	gridCell = 28.0
	gridGap  = 2.0
	gridSpan = gridN*gridCell + (gridN-1)*gridGap // 178
	gridX0   = (cardW - gridSpan) / 2             // 126
	gridY0   = 42.0

	sampleX, sampleY = 20.0, 44.0
	sampleMax        = 150.0

	colrX, colrY = 20.0, 44.0
	colrSize     = 150

	segLineX  = 14.0
	segLineY0 = 46.0
	segLineH  = 36.0
	segBoxY   = 22.0
	segBoxH   = 6.0
)

// ---------------------------------------------------------------------------
// Emoji assets: font → CBDT/CBLC tables → RGB bitmap glyphs → render.ImageBuf
// ---------------------------------------------------------------------------

type emojiCell struct {
	gid    uint16
	img    *image.RGBA
	buf    *render.ImageBuf
	w, h   int
	pngLen int
}

type emojiFont struct {
	fontPath string
	fontSHA  string
	ppems    []uint16
	ppem     uint16
	cells    []*emojiCell
	sample   *emojiCell
	err      error
}

var (
	emojiAssets   *emojiFont
	emojiSegLines []segLine
	emojiCOLR     *colrAsset
	emojiFace12   text.Face
	emojiFace18   text.Face
)

// loadEmojiAssets finds a system color emoji font, pulls its CBDT/CBLC bitmap
// tables and bakes the first 36 decodable glyphs into CPU images. Nothing here
// touches the GPU: the render stack uploads the ImageBufs at paint time.
//
// A missing font is a skip (the C/D cards stay live), not a crash. The font
// path + SHA are reported so a golden can be traced back to the exact binary.
func loadEmojiAssets() *emojiFont {
	path, data := findColorEmojiFont()
	if path == "" {
		return &emojiFont{err: fmt.Errorf("no color emoji font found (looked for NotoColorEmoji.ttf / seguiemj.ttf / Apple Color Emoji.ttc)")}
	}
	sum := sha256.Sum256(data)
	a := &emojiFont{fontPath: path, fontSHA: hex.EncodeToString(sum[:])[:16]}

	cbdt := fontTable(data, "CBDT")
	cblc := fontTable(data, "CBLC")
	if cbdt == nil || cblc == nil {
		a.err = fmt.Errorf("font %s has no CBDT/CBLC tables (TTC and COLR-only fonts are not baked here)", filepath.Base(path))
		return a
	}

	extractor, err := emoji.NewCBDTExtractor(cbdt, cblc)
	if err != nil {
		a.err = fmt.Errorf("CBDT extractor: %w", err)
		return a
	}
	ppems := extractor.AvailablePPEMs()
	if len(ppems) == 0 {
		a.err = fmt.Errorf("font %s exposes no bitmap strikes", filepath.Base(path))
		return a
	}
	target := ppems[len(ppems)-1]
	a.ppems = ppems
	a.ppem = target

	for gid := uint16(1); gid < 10000 && len(a.cells) < gridN*gridN; gid++ {
		g, err := extractor.GetGlyph(gid, target)
		if err != nil || len(g.Data) == 0 {
			continue
		}
		std, err := g.Decode()
		if err != nil {
			continue
		}
		rgba := toRGBA(std)
		if rgba == nil {
			continue
		}
		b := rgba.Bounds()
		a.cells = append(a.cells, &emojiCell{
			gid:    gid,
			img:    rgba,
			buf:    render.ImageBufFromImage(rgba),
			w:      b.Dx(),
			h:      b.Dy(),
			pngLen: len(g.Data),
		})
	}
	if len(a.cells) == 0 {
		a.err = fmt.Errorf("font %s decoded no CBDT bitmap glyphs", filepath.Base(path))
		return a
	}
	// Pick the most chromatic cell as the B-card sample. The first CBDT
	// glyphs include monochrome symbols (✖, ❗, …); the card is meant to show
	// the color path, so choose a visibly colorful bitmap deterministically.
	best, bestChroma := 0, -1
	for i, c := range a.cells {
		if n := chromaCount(c.img); n > bestChroma {
			best, bestChroma = i, n
		}
	}
	a.sample = a.cells[best]
	return a
}

// chromaCount is a cheap, deterministic "is this bitmap colorful" score used
// only to choose the B-card sample. It samples every other pixel.
func chromaCount(img *image.RGBA) int {
	if img == nil {
		return 0
	}
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			r, g, bl, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			r8, g8, b8 := int(r>>8), int(g>>8), int(bl>>8)
			mx, mn := r8, r8
			if g8 > mx {
				mx = g8
			}
			if b8 > mx {
				mx = b8
			}
			if g8 < mn {
				mn = g8
			}
			if b8 < mn {
				mn = b8
			}
			if mx-mn > 20 {
				n++
			}
		}
	}
	return n
}

func findColorEmojiFont() (string, []byte) {
	for _, p := range []string{
		"/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/TTF/NotoColorEmoji.ttf",
		"/usr/local/share/fonts/NotoColorEmoji.ttf",
		"/System/Library/Fonts/Apple Color Emoji.ttc",
		"/Library/Fonts/Apple Color Emoji.ttc",
		"C:/Windows/Fonts/seguiemj.ttf",
	} {
		data, err := os.ReadFile(p)
		if err == nil && len(data) > 0 {
			return p, data
		}
	}
	return "", nil
}

// fontTable returns a TTF table by tag. TTC collections are rejected (the
// header is not a single sfnt face); the window reports that as a skip.
func fontTable(data []byte, tag string) []byte {
	if len(data) < 12 || string(data[0:4]) == "ttcf" {
		return nil
	}
	num := int(binary.BigEndian.Uint16(data[4:6]))
	off := 12
	for i := 0; i < num && off+16 <= len(data); i++ {
		t := string(data[off : off+4])
		to := binary.BigEndian.Uint32(data[off+8 : off+12])
		tl := binary.BigEndian.Uint32(data[off+12 : off+16])
		if t == tag && uint64(to)+uint64(tl) <= uint64(len(data)) {
			return data[to : to+tl]
		}
		off += 16
	}
	return nil
}

func toRGBA(img image.Image) *image.RGBA {
	if img == nil {
		return nil
	}
	if r, ok := img.(*image.RGBA); ok {
		return r
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

func sampleDrawSize(c *emojiCell) (w, h float64) {
	w, h = float64(c.w), float64(c.h)
	if w > sampleMax {
		h *= sampleMax / w
		w = sampleMax
	}
	if h > sampleMax {
		w *= sampleMax / h
		h = sampleMax
	}
	return w, h
}

// ---------------------------------------------------------------------------
// Segmentation corpus
// ---------------------------------------------------------------------------

type segLine struct {
	text string
	runs []emoji.Run
	runX []float64
	runW []float64
}

func loadSegLines() ([]segLine, error) {
	path := filepath.Join("examples", "ui_render_base_emoji", "testdata", "emoji.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var out []segLine
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		runs := emoji.Segment(line)
		sl := segLine{text: line, runs: runs}
		x := 0.0
		for _, r := range runs {
			w := 6.0
			if emojiFace18 != nil {
				if mw, _ := text.Measure(r.Text, emojiFace18); mw > 0 {
					w = mw
				}
			}
			sl.runX = append(sl.runX, x)
			sl.runW = append(sl.runW, w)
			x += w
		}
		out = append(out, sl)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s has no segment lines", path)
	}
	return out, nil
}

func firstEmojiRun(l segLine) int {
	for i, r := range l.runs {
		if r.IsEmoji {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// Synthetic COLR glyph
// ---------------------------------------------------------------------------

type colrAsset struct {
	img    *image.RGBA
	buf    *render.ImageBuf
	layers []emoji.ColorLayer
	w, h   int
}

// buildCOLR composites three deterministic layers through the real
// emoji.RenderCOLRToImage path: red square (bottom), green circle, blue
// triangle (top). It is intentionally synthetic — Linux ships no COLR emoji
// font here — but the compositing code under test is the production one.
func buildCOLR() *colrAsset {
	const w, h = colrSize, colrSize
	layers := []emoji.ColorLayer{
		{GlyphID: 1, Color: emoji.Color{R: 230, G: 51, B: 51, A: 255}},
		{GlyphID: 2, Color: emoji.Color{R: 51, G: 204, B: 77, A: 255}},
		{GlyphID: 3, Color: emoji.Color{R: 51, G: 102, B: 242, A: 255}},
	}
	glyph := &emoji.COLRGlyph{
		GlyphID: 9001,
		Version: 0,
		Layers:  layers,
		Bounds:  emoji.Rect{MinX: 0, MinY: 0, MaxX: w, MaxY: h},
	}
	renderLayer := func(gid uint16) *image.Alpha {
		m := image.NewAlpha(image.Rect(0, 0, w, h))
		switch gid {
		case 1: // full square
			for i := range m.Pix {
				m.Pix[i] = 255
			}
		case 2: // circle, center (75,75), r=45
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					dx, dy := float64(x)-75, float64(y)-75
					if dx*dx+dy*dy <= 45*45 {
						m.SetAlpha(x, y, color.Alpha{A: 255})
					}
				}
			}
		case 3: // triangle (75,95) (45,135) (105,135)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if pointInTriangle(float64(x), float64(y), 75, 95, 45, 135, 105, 135) {
						m.SetAlpha(x, y, color.Alpha{A: 255})
					}
				}
			}
		}
		return m
	}
	img := emoji.RenderCOLRToImage(glyph, renderLayer, w, h, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if img == nil {
		return nil
	}
	return &colrAsset{img: img, buf: render.ImageBufFromImage(img), layers: layers, w: w, h: h}
}

func pointInTriangle(px, py, ax, ay, bx, by, cx, cy float64) bool {
	d1 := (px-bx)*(ay-by) - (ax-bx)*(py-by)
	d2 := (px-cx)*(by-cy) - (bx-cx)*(py-cy)
	d3 := (px-ax)*(cy-ay) - (cx-ax)*(py-ay)
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

// ---------------------------------------------------------------------------
// Logic probes
// ---------------------------------------------------------------------------

var probeKeys = []string{
	"segment_mixed",
	"segment_zwj",
	"segment_flag",
	"segment_tone",
	"segment_keycap",
	"segment_presentation",
	"segment_offsets",
	"segment_roundtrip",
	"segment_no_empty",
	"cbdt_ppems",
	"cbdt_glyph",
	"cbdt_grid",
	"colr_render",
	"colr_foreground",
}

// runLogicProbes checks the emoji package against the real corpus and the
// extracted font assets. CBDT keys are skipped (not failed) when no color
// font is installed; everything else is mandatory.
func runLogicProbes() (ran, failed, skipped []string) {
	add := func(key string, ok bool) {
		ran = append(ran, key)
		if !ok {
			failed = append(failed, key)
		}
	}
	skip := func(key string) { skipped = append(skipped, key) }

	// mixed text + emoji, byte offsets intact
	runs := emoji.Segment("Hi 😀")
	add("segment_mixed", len(runs) == 2 &&
		runs[0].Text == "Hi " && !runs[0].IsEmoji &&
		runs[1].Text == "😀" && runs[1].IsEmoji)

	// ZWJ family stays one run / one sequence
	family := "👨‍👩‍👧‍👦"
	runs = emoji.Segment(family)
	seqs := emoji.ParseString(family)
	add("segment_zwj", len(runs) == 1 && runs[0].IsEmoji && len(runs[0].Codepoints) == 7 &&
		len(seqs) == 1 && seqs[0].Type == emoji.SequenceZWJ && emoji.IsValidSequence(seqs[0]))

	// regional-indicator flag
	runs = emoji.Segment("🇨🇳")
	seqs = emoji.ParseString("🇨🇳")
	add("segment_flag", len(runs) == 1 && runs[0].IsEmoji && len(runs[0].Codepoints) == 2 &&
		len(seqs) == 1 && seqs[0].Type == emoji.SequenceFlag &&
		emoji.GetFlagCode(seqs[0]) == "CN" && emoji.IsValidSequence(seqs[0]))

	// skin-tone modifier: Segment keeps the pair together. Parse currently
	// splits it (simplified modifier-base list) — a known engine gap recorded
	// in README; this probe certifies segmentation only.
	runs = emoji.Segment("👍🏽")
	add("segment_tone", len(runs) == 1 && runs[0].IsEmoji && len(runs[0].Codepoints) == 2 &&
		runs[0].Codepoints[0] == 0x1F44D && runs[0].Codepoints[1] == 0x1F3FD)

	// keycap
	runs = emoji.Segment("1️⃣")
	seqs = emoji.ParseString("1️⃣")
	add("segment_keycap", len(runs) == 1 && runs[0].IsEmoji &&
		len(seqs) == 1 && seqs[0].Type == emoji.SequenceKeycap && emoji.IsValidSequence(seqs[0]))

	// VS16 presentation: ©️/❤️ are emoji runs, bare ❤ stays a run
	runs = emoji.Segment("©️ ❤️ ❤")
	add("segment_presentation", len(runs) == 5 &&
		runs[0].IsEmoji && len(runs[0].Codepoints) == 2 &&
		runs[2].IsEmoji && len(runs[2].Codepoints) == 2 &&
		runs[4].IsEmoji && len(runs[4].Codepoints) == 1 &&
		emoji.IsEmojiVariation(0xFE0F) && emoji.IsTextPresentation(0xFE0E))

	// byte offsets stay usable for slicing
	s := "a😀b"
	runs = emoji.Segment(s)
	add("segment_offsets", len(runs) == 3 &&
		runs[1].Start == 1 && runs[1].End == 5 &&
		s[runs[1].Start:runs[1].End] == runs[1].Text)

	// every corpus line round-trips and offsets slice back to the same text
	roundTrip := true
	for _, l := range emojiSegLines {
		var b strings.Builder
		for _, r := range l.runs {
			if r.Start < 0 || r.End > len(l.text) || r.Start > r.End || l.text[r.Start:r.End] != r.Text {
				roundTrip = false
			}
			b.WriteString(r.Text)
		}
		if b.String() != l.text {
			roundTrip = false
		}
	}
	add("segment_roundtrip", roundTrip)

	// no empty runs, at least one emoji run in the corpus
	noEmpty, emojiRuns := true, 0
	for _, l := range emojiSegLines {
		for _, r := range l.runs {
			if r.Text == "" {
				noEmpty = false
			}
			if r.IsEmoji {
				emojiRuns++
			}
		}
	}
	add("segment_no_empty", noEmpty && emojiRuns > 0)

	// CBDT font extraction
	if emojiAssets == nil || emojiAssets.err != nil {
		skip("cbdt_ppems")
		skip("cbdt_glyph")
		skip("cbdt_grid")
	} else {
		add("cbdt_ppems", len(emojiAssets.ppems) > 0 &&
			emojiAssets.ppem == emojiAssets.ppems[len(emojiAssets.ppems)-1])
		s := emojiAssets.sample
		add("cbdt_glyph", s != nil && s.w > 0 && s.h > 0 && s.pngLen > 0 &&
			len(s.img.Pix) == s.w*s.h*4)
		add("cbdt_grid", len(emojiAssets.cells) == gridN*gridN)
	}

	// COLR compositing: layer order + exact palette colors
	if emojiCOLR == nil || emojiCOLR.img == nil || len(emojiCOLR.layers) != 3 {
		add("colr_render", false)
	} else {
		add("colr_render",
			nearColorAt(emojiCOLR.img, 30, 30, colCOLRRed, 1.0/255) &&
				nearColorAt(emojiCOLR.img, 75, 75, colCOLRGreen, 1.0/255) &&
				nearColorAt(emojiCOLR.img, 75, 125, colCOLRBlue, 1.0/255))
	}

	// foreground layers (PaletteIndex 0xFFFF) take the caller color
	fgGlyph := &emoji.COLRGlyph{
		GlyphID: 1,
		Layers:  []emoji.ColorLayer{{GlyphID: 1, PaletteIndex: 0xFFFF}},
	}
	fg := emoji.RenderCOLRToImage(fgGlyph, func(uint16) *image.Alpha {
		m := image.NewAlpha(image.Rect(0, 0, 16, 16))
		for i := range m.Pix {
			m.Pix[i] = 255
		}
		return m
	}, 16, 16, color.RGBA{R: 10, G: 200, B: 30, A: 255})
	add("colr_foreground", fg != nil &&
		nearColorAt(fg, 8, 8, [3]float64{10.0 / 255, 200.0 / 255, 30.0 / 255}, 1.0/255))

	return ran, failed, skipped
}

func nearColorAt(img *image.RGBA, x, y int, want [3]float64, tol float64) bool {
	if img == nil || x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return false
	}
	r, g, b, a := img.At(x, y).RGBA()
	if a == 0 {
		return false
	}
	return nearC(float64(r>>8)/255, float64(g>>8)/255, float64(b>>8)/255, want[0], want[1], want[2], tol)
}

// ---------------------------------------------------------------------------
// Pixel probes
// ---------------------------------------------------------------------------

type pixProbe struct {
	key       string
	desc      string
	card      int
	rect      rect
	kind      string // "ink" or "exact"
	want      [3]float64
	tol       float64
	minInk    float64
	minChroma int
}

// buildPixProbes derives every probe rectangle from the same constants the
// paint functions use. A/B cards are font-dependent and only appear when the
// color font loaded; C/D are always present.
func buildPixProbes() []pixProbe {
	var out []pixProbe
	if emojiAssets != nil && emojiAssets.err == nil {
		out = append(out,
			pixProbe{
				key: "A_grid_ink", desc: "36 CBDT cells carry ink", card: 0,
				rect:   rect{x: gridX0 - 2, y: gridY0 - 2, w: gridSpan + 4, h: gridSpan + 4},
				kind:   "ink",
				minInk: 0.01, minChroma: 100,
			},
			pixProbe{
				key: "A_grid_bg", desc: "cell gap shows card BG", card: 0,
				rect: rect{x: gridX0 + gridCell + gridGap/2, y: gridY0 + 4, w: 1, h: 1},
				kind: "exact", want: colCardBG, tol: 8.0 / 255,
			},
			pixProbe{
				key: "B_sample_ink", desc: "sample glyph carries ink", card: 1,
				rect: rect{x: sampleX, y: sampleY, w: 1, h: 1}, // replaced below
				kind: "ink", minInk: 0.01, minChroma: 20,
			},
			pixProbe{
				key: "B_sample_bg", desc: "left of sample is card BG", card: 1,
				rect: rect{x: 14, y: 60, w: 1, h: 1},
				kind: "exact", want: colCardBG, tol: 8.0 / 255,
			},
		)
		if emojiAssets.sample != nil {
			sw, sh := sampleDrawSize(emojiAssets.sample)
			out[2].rect.w, out[2].rect.h = sw, sh
		}
	}

	if len(emojiSegLines) > 0 && len(emojiSegLines[0].runs) > 1 {
		addRun := func(key string, lineIdx, runIdx int, want [3]float64) {
			if lineIdx < 0 || lineIdx >= len(emojiSegLines) || runIdx < 0 {
				return
			}
			l := emojiSegLines[lineIdx]
			if runIdx >= len(l.runs) {
				return
			}
			out = append(out, pixProbe{
				key: key, desc: "run box color", card: 2,
				rect: rect{
					x: segLineX + l.runX[runIdx],
					y: segLineY0 + float64(lineIdx)*segLineH + segBoxY,
					w: l.runW[runIdx], h: segBoxH,
				},
				kind: "exact", want: want, tol: 8.0 / 255,
			})
		}
		addRun("C_text_run0", 0, 0, colRunText)
		addRun("C_emoji_run0", 0, 1, colRunEmoji)
		if len(emojiSegLines) > 2 {
			addRun("C_flag_run", 2, firstEmojiRun(emojiSegLines[2]), colRunEmoji)
		}
		if len(emojiSegLines) > 3 {
			addRun("C_tone_run", 3, firstEmojiRun(emojiSegLines[3]), colRunEmoji)
		}
	}

	if emojiCOLR != nil {
		out = append(out,
			pixProbe{key: "D_red", desc: "COLR red square", card: 3,
				rect: rect{x: colrX + 30, y: colrY + 30, w: 1, h: 1},
				kind: "exact", want: colCOLRRed, tol: 12.0 / 255},
			pixProbe{key: "D_green", desc: "COLR green circle", card: 3,
				rect: rect{x: colrX + 75, y: colrY + 75, w: 1, h: 1},
				kind: "exact", want: colCOLRGreen, tol: 12.0 / 255},
			pixProbe{key: "D_blue", desc: "COLR blue triangle", card: 3,
				rect: rect{x: colrX + 75, y: colrY + 125, w: 1, h: 1},
				kind: "exact", want: colCOLRBlue, tol: 12.0 / 255},
			pixProbe{key: "D_bg", desc: "COLR corner is card BG", card: 3,
				rect: rect{x: 5, y: 5, w: 1, h: 1},
				kind: "exact", want: colCardBG, tol: 8.0 / 255},
		)
	}
	return out
}

// ---------------------------------------------------------------------------
// Scene
// ---------------------------------------------------------------------------

func buildEmojiScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}

	aTitle := "A  CBDT 6×6 grid — font unavailable"
	bTitle := "B  single glyph — font unavailable"
	if emojiAssets != nil && emojiAssets.err == nil {
		aTitle = fmt.Sprintf("A  CBDT 6×6 grid — %d glyphs · PPEM %d", len(emojiAssets.cells), emojiAssets.ppem)
		if emojiAssets.sample != nil {
			bTitle = fmt.Sprintf("B  single glyph %d — CBDT PNG → GPU image", emojiAssets.sample.gid)
		}
	}
	card(0, aTitle, paintGridCard)
	card(1, bTitle, paintSampleCard)
	card(2, "C  Segment runs (boxes) — ZWJ / flag / tone / keycap", paintSegCard)
	card(3, "D  COLR 3 layers — synthetic red / green / blue", paintCOLRCard)
}

func cardBG(dc *render.Context, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func paintGridCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	if emojiAssets == nil || emojiAssets.err != nil {
		drawAssetError(dc, ox, oy)
		return
	}
	for i, c := range emojiAssets.cells {
		col, row := i%gridN, i/gridN
		x := ox + gridX0 + float64(col)*(gridCell+gridGap)
		y := oy + gridY0 + float64(row)*(gridCell+gridGap)
		dc.DrawImageEx(c.buf, render.DrawImageOptions{
			X: x, Y: y,
			DstWidth: gridCell, DstHeight: gridCell,
			Interpolation: render.InterpBilinear,
			Opacity:       1,
		})
	}
}

func paintSampleCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	if emojiAssets == nil || emojiAssets.err != nil || emojiAssets.sample == nil {
		drawAssetError(dc, ox, oy)
		return
	}
	s := emojiAssets.sample
	sw, sh := sampleDrawSize(s)
	dc.DrawImageEx(s.buf, render.DrawImageOptions{
		X: ox + sampleX, Y: oy + sampleY,
		DstWidth: sw, DstHeight: sh,
		Interpolation: render.InterpBilinear,
		Opacity:       1,
	})
	if emojiFace12 == nil {
		return
	}
	dc.SetFont(emojiFace12)
	lines := []string{
		fmt.Sprintf("glyph %d", s.gid),
		fmt.Sprintf("PPEM %d", emojiAssets.ppem),
		fmt.Sprintf("%d × %d px", s.w, s.h),
		fmt.Sprintf("PNG %d B", s.pngLen),
		"CBDT → ImageBuf → GPU",
	}
	for i, line := range lines {
		dc.SetRGBA(colBody[0], colBody[1], colBody[2], 1)
		dc.DrawString(line, ox+176, oy+64+float64(i)*18)
	}
}

func paintSegCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	if emojiFace18 != nil {
		dc.SetFont(emojiFace18)
	}
	for i, l := range emojiSegLines {
		y := oy + segLineY0 + float64(i)*segLineH
		if emojiFace18 != nil {
			dc.SetRGBA(colBody[0], colBody[1], colBody[2], 1)
			dc.DrawString(l.text, ox+segLineX, y+14)
		}
		for j, r := range l.runs {
			c := colRunText
			if r.IsEmoji {
				c = colRunEmoji
			}
			dc.SetRGBA(c[0], c[1], c[2], 1)
			dc.DrawRectangle(ox+segLineX+l.runX[j], y+segBoxY, l.runW[j], segBoxH)
			_ = dc.Fill()
		}
	}
}

func paintCOLRCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	if emojiCOLR != nil {
		dc.DrawImageEx(emojiCOLR.buf, render.DrawImageOptions{
			X: ox + colrX, Y: oy + colrY,
			DstWidth: colrSize, DstHeight: colrSize,
			Interpolation: render.InterpBilinear,
			Opacity:       1,
		})
	}
	if emojiFace12 == nil {
		return
	}
	dc.SetFont(emojiFace12)
	labels := []struct {
		text string
		col  [3]float64
		y    float64
	}{
		{"COLR layers", colTitle, 64},
		{"red square", colCOLRRed, 88},
		{"green circle", colCOLRGreen, 112},
		{"blue triangle", colCOLRBlue, 136},
	}
	for _, l := range labels {
		dc.SetRGBA(l.col[0], l.col[1], l.col[2], 1)
		dc.DrawRectangle(ox+190, oy+l.y-9, 10, 10)
		_ = dc.Fill()
		dc.DrawString(l.text, ox+206, oy+l.y)
	}
}

func drawAssetError(dc *render.Context, ox, oy float64) {
	msg := "emoji assets unavailable"
	if emojiAssets != nil && emojiAssets.err != nil {
		msg = emojiAssets.err.Error()
	}
	if len(msg) > 70 {
		msg = msg[:70] + "..."
	}
	if emojiFace12 == nil {
		return
	}
	dc.SetFont(emojiFace12)
	dc.SetRGBA(0.95, 0.55, 0.35, 1)
	dc.DrawString(msg, ox+12, oy+60)
	dc.DrawString("C/D stay live; A/B probes skipped", ox+12, oy+80)
}
