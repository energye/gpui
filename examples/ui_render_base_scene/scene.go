package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH         = 430.0, 232.0
	sceneW, sceneH       = 398.0, 194.0
	sceneImgX, sceneImgY = 16.0, 32.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colTitle  = [3]float64{0.80, 0.86, 0.94}
	colBorder = [3]float64{0.32, 0.36, 0.44}
)

var (
	sceneBG   = render.RGBA{R: 0.10, G: 0.12, B: 0.16, A: 1}
	colYellow = render.RGBA{R: 0.92, G: 0.80, B: 0.22, A: 1}
	colCyan   = render.RGBA{R: 0.18, G: 0.66, B: 0.78, A: 1}
	colBlue   = render.RGBA{R: 0.25, G: 0.45, B: 0.95, A: 1}
	colPurple = render.RGBA{R: 0.62, G: 0.35, B: 0.90, A: 1}
	colRed    = render.RGBA{R: 0.90, G: 0.25, B: 0.28, A: 1}
	colOrange = render.RGBA{R: 1.00, G: 0.58, B: 0.16, A: 1}
	colGold   = render.RGBA{R: 0.95, G: 0.78, B: 0.25, A: 1}
	colGreen  = render.RGBA{R: 0.20, G: 0.78, B: 0.35, A: 1}
	colText   = render.RGBA{R: 0.92, G: 0.95, B: 1.00, A: 1}
	colCacheR = render.RGBA{R: 0.86, G: 0.20, B: 0.22, A: 1}
	colCacheB = render.RGBA{R: 0.22, G: 0.42, B: 0.92, A: 1}
)

// A/B backend parity gate. The GPU scene renderer and the CPU tile renderer
// share the encoding; only AA coverage and blend rounding may differ at
// silhouette edges. A lost layer alpha, lost clip or a wrong blend mode moves
// large areas, far above these.
//
// The worst per-channel delta is reported but not gated: a single edge pixel
// can sit at 90% coverage on one backend and 40% on the other, a harmless
// >100 delta. The semantic gates are the share of pixels above parityTol, and
// the (much smaller) share above parityHighDelta which catches thin but real
// geometry/colour splits.
const (
	parityTol        = 10
	parityMaxPct     = 2.0
	parityHighDelta  = 96
	parityMaxHighPct = 0.1
)

type sceneLayout struct {
	fontPath string
	fontSrc  *text.FontSource
	font     text.Face

	imgPath string
	img     *scene.Image

	ab    *scene.Scene
	abPM  *render.Pixmap
	abImg *render.ImageBuf

	cache       *scene.LayerCache
	cacheHashes [2]uint64
	cacheL      *render.ImageBuf
	cacheR      *render.ImageBuf

	damageFirst image.Rectangle
	damageMove  image.Rectangle
	damageThird image.Rectangle
	damagePM    *render.Pixmap
	damageImg   *render.ImageBuf
	damageSHA   [32]byte
	fullSHA     [32]byte

	encTags    int
	encBrushes int
}

var slay sceneLayout

var sceneProbeKeys = []string{
	"builder_tags",
	"scene_bounds",
	"scene_version",
	"encoding_roundtrip",
	"flatten_clone",
	"append_translation",
	"hash_brush_sensitive",
	"hash_stable",
	"cache_brush_distinct",
	"cache_stats",
	"cpu_layer_alpha",
	"cpu_layer_multiply",
	"cpu_roundrect_clip",
	"cpu_circle_clip",
	"damage_first_full",
	"damage_move_only",
	"damage_incremental_matches_full",
	"damage_no_change",
	"text_ink_cpu",
	"image_quadrants_cpu",
	"empty_scene",
}

func resolveSceneAsset(name string) (string, error) {
	candidates := []string{
		filepath.Join("examples", "ui_render_base_scene", "testdata", name),
		filepath.Join("testdata", name),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found (looked in %v)", name, candidates)
}

func loadQuadImage(path string) (*scene.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	b := src.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
	img := scene.NewImage(b.Dx(), b.Dy())
	img.Data = append([]byte(nil), rgba.Pix...)
	return img, nil
}

func buildSceneLayout() error {
	fontPath, err := resolveSceneAsset("SourceSans3-Regular.otf")
	if err != nil {
		return err
	}
	src, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", fontPath, err)
	}
	slay.fontPath = fontPath
	slay.fontSrc = src
	slay.font = src.Face(26)
	if slay.font == nil {
		return fmt.Errorf("%s: Face(26) returned nil", fontPath)
	}

	imgPath, err := resolveSceneAsset("scene_quad.png")
	if err != nil {
		return err
	}
	slay.imgPath = imgPath
	slay.img, err = loadQuadImage(imgPath)
	if err != nil {
		return err
	}

	slay.ab = buildABScene()
	pm, err := renderSceneCPUFull(slay.ab, int(sceneW), int(sceneH))
	if err != nil {
		return fmt.Errorf("A/B cpu render: %w", err)
	}
	slay.abPM = pm
	slay.abImg = render.ImageBufFromImage(pm)
	slay.encTags = len(slay.ab.Encoding().Tags())
	slay.encBrushes = len(slay.ab.Encoding().Brushes())

	if err := buildCacheCard(); err != nil {
		return fmt.Errorf("cache card: %w", err)
	}
	if err := buildDamageCard(); err != nil {
		return fmt.Errorf("damage card: %w", err)
	}
	return nil
}

// buildABScene builds one scene that exercises layers (alpha + multiply),
// transforms, circle clipping, a clipped round-rect fill and a stroke. Card A
// renders it with the GPU scene renderer, card B with the CPU tile renderer.
func buildABScene() *scene.Scene {
	b := scene.NewSceneBuilder()
	b.FillRect(0, 0, sceneW, sceneH, scene.SolidBrush(sceneBG))

	// Multiply layer over an opaque base.
	b.FillRect(12, 12, 90, 64, scene.SolidBrush(colYellow))
	b.Layer(scene.BlendMultiply, 1, nil, func(lb *scene.SceneBuilder) {
		lb.FillRect(36, 28, 90, 64, scene.SolidBrush(colCyan))
	})

	// 50% layer over the background (single draw, no overlap).
	b.Layer(scene.BlendNormal, 0.5, nil, func(lb *scene.SceneBuilder) {
		lb.FillRect(150, 16, 80, 56, scene.SolidBrush(colBlue))
	})

	// Rotated square around its centre.
	rot := scene.TranslateAffine(300, 60).
		Multiply(scene.RotateAffine(0.35)).
		Multiply(scene.TranslateAffine(-25, -25))
	b.WithTransform(rot, func(tb *scene.SceneBuilder) {
		tb.FillRect(0, 0, 50, 50, scene.SolidBrush(colPurple))
	})

	// Circle clip over a large red rect.
	b.Clip(scene.NewCircleShape(90, 145, 40), func(cb *scene.SceneBuilder) {
		cb.FillRect(30, 105, 120, 80, scene.SolidBrush(colRed))
	})

	// Rect clip + rounded rect wider than the clip: the rounded rect must be
	// clipped, not painted into the tile behind the clip.
	b.Clip(scene.NewRectShape(220, 105, 80, 80), func(cb *scene.SceneBuilder) {
		cb.FillRoundRect(200, 95, 120, 100, 16, 16, scene.SolidBrush(colGold))
	})

	b.StrokeCircle(330, 145, 28, scene.SolidBrush(colOrange), 6)
	return b.Build()
}

func buildCacheScene(c render.RGBA) *scene.Scene {
	b := scene.NewSceneBuilder()
	b.FillRect(0, 0, 176, 150, scene.SolidBrush(render.RGBA{R: 0.07, G: 0.08, B: 0.11, A: 1}))
	b.FillRoundRect(18, 18, 140, 114, 18, 18, scene.SolidBrush(c))
	b.StrokeCircle(88, 75, 30, scene.SolidBrush(render.RGBA{R: 0.95, G: 0.96, B: 1, A: 1}), 4)
	return b.Build()
}

func buildCacheCard() error {
	slay.cache = scene.NewLayerCache(8)
	colors := [2]render.RGBA{colCacheR, colCacheB}
	var pms [2]*render.Pixmap
	for i, c := range colors {
		sc := buildCacheScene(c)
		hash := sc.Encoding().Hash()
		slay.cacheHashes[i] = hash
		pm, err := renderSceneCPUFull(sc, 176, 150)
		if err != nil {
			return fmt.Errorf("scene %d: %w", i, err)
		}
		slay.cache.Put(hash, pm, sc.Version())
		pms[i] = pm
	}
	if slay.cacheHashes[0] == slay.cacheHashes[1] {
		return fmt.Errorf("brush-only change collided: hash=%#x", slay.cacheHashes[0])
	}

	left, ok := slay.cache.Get(slay.cacheHashes[0])
	if !ok {
		return fmt.Errorf("cache miss for red scene %#x", slay.cacheHashes[0])
	}
	right, ok := slay.cache.Get(slay.cacheHashes[1])
	if !ok {
		return fmt.Errorf("cache miss for blue scene %#x", slay.cacheHashes[1])
	}
	slay.cacheL = render.ImageBufFromImage(left)
	slay.cacheR = render.ImageBufFromImage(right)
	_ = pms
	return nil
}

func buildDamageScene(x float32) *scene.Scene {
	b := scene.NewSceneBuilder()
	b.FillRect(0, 0, sceneW, sceneH, scene.SolidBrush(sceneBG))
	b.Image(slay.img, scene.Rect{MinX: 270, MinY: 16, MaxX: 334, MaxY: 80})
	b.FillRect(x, 138, 64, 40, scene.SolidBrush(colGreen))
	sc := b.Build()
	if err := sc.DrawText("scene", slay.font, 14, 56, scene.SolidBrush(colText)); err != nil {
		return nil
	}
	return sc
}

func buildDamageCard() error {
	first := buildDamageScene(20)
	if first == nil {
		return fmt.Errorf("build damage frame 1 failed")
	}
	second := buildDamageScene(150)
	if second == nil {
		return fmt.Errorf("build damage frame 2 failed")
	}

	tracker := scene.NewDamageTracker()
	r := scene.NewRenderer(int(sceneW), int(sceneH), scene.WithWorkers(1))
	if r == nil {
		return fmt.Errorf("NewRenderer returned nil")
	}
	defer r.Close()
	pm := render.NewPixmap(int(sceneW), int(sceneH))

	var err error
	slay.damageFirst, err = r.RenderWithDamage(pm, first, tracker)
	if err != nil {
		return fmt.Errorf("first render: %w", err)
	}
	slay.damageMove, err = r.RenderWithDamage(pm, second, tracker)
	if err != nil {
		return fmt.Errorf("damage render: %w", err)
	}
	slay.damageThird, err = r.RenderWithDamage(pm, second, tracker)
	if err != nil {
		return fmt.Errorf("no-change render: %w", err)
	}
	slay.damagePM = pm
	slay.damageImg = render.ImageBufFromImage(pm)
	slay.damageSHA = sha256.Sum256(pm.Data())

	full, err := renderSceneCPUFull(second, int(sceneW), int(sceneH))
	if err != nil {
		return fmt.Errorf("full render: %w", err)
	}
	slay.fullSHA = sha256.Sum256(full.Data())
	return nil
}

// renderSceneCPUFull forces the CPU tile path (Renderer.Render would take the
// GPU fast path when an accelerator is registered).
func renderSceneCPUFull(sc *scene.Scene, w, h int) (*render.Pixmap, error) {
	r := scene.NewRenderer(w, h, scene.WithWorkers(1))
	if r == nil {
		return nil, fmt.Errorf("NewRenderer returned nil")
	}
	defer r.Close()
	r.MarkAllDirty()
	pm := render.NewPixmap(w, h)
	if err := r.RenderDirty(pm, sc, nil); err != nil {
		return nil, err
	}
	return pm, nil
}

func pmRGB(pm *render.Pixmap, x, y int) (r, g, b float64) {
	if pm == nil {
		return 0, 0, 0
	}
	r32, g32, b32, _ := pm.At(x, y).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255
}

func nearRGB(pm *render.Pixmap, x, y int, want render.RGBA, tol float64) error {
	r, g, b := pmRGB(pm, x, y)
	if math.Abs(r-float64(want.R)) > tol || math.Abs(g-float64(want.G)) > tol || math.Abs(b-float64(want.B)) > tol {
		return fmt.Errorf("pixel (%d,%d) got (%.3f,%.3f,%.3f) want (%.3f,%.3f,%.3f) tol %.3f",
			x, y, r, g, b, want.R, want.G, want.B, tol)
	}
	return nil
}

func hasSceneTag(tags []scene.Tag, want scene.Tag) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

func runSceneLogic() ([]string, []string) {
	var ran, failed []string
	p := func(key string, fn func() error) {
		ran = append(ran, key)
		if err := fn(); err != nil {
			failed = append(failed, key)
			fmt.Fprintf(os.Stderr, "scene logic %s: %v\n", key, err)
		}
	}

	p("builder_tags", func() error {
		tags := slay.ab.Encoding().Tags()
		want := []scene.Tag{
			scene.TagFill, scene.TagStroke, scene.TagFillRoundRect,
			scene.TagPushLayer, scene.TagPopLayer,
			scene.TagBeginClip, scene.TagEndClip, scene.TagTransform,
		}
		for _, w := range want {
			if !hasSceneTag(tags, w) {
				return fmt.Errorf("missing tag %v", w)
			}
		}
		return nil
	})

	p("scene_bounds", func() error {
		b := slay.ab.Bounds()
		if b.MinX < -1 || b.MinY < -1 || b.MaxX < sceneW-1 || b.MaxY < sceneH-1 {
			return fmt.Errorf("bounds %+v does not cover canvas", b)
		}
		if b.MaxX > sceneW+64 || b.MaxY > sceneH+64 {
			return fmt.Errorf("bounds %+v overshoots canvas", b)
		}
		return nil
	})

	p("scene_version", func() error {
		sc := scene.NewScene()
		v0 := sc.Version()
		sc.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(colRed), scene.NewRectShape(0, 0, 10, 10))
		if sc.Version() <= v0 {
			return fmt.Errorf("version did not advance: %d -> %d", v0, sc.Version())
		}
		return nil
	})

	p("encoding_roundtrip", func() error {
		enc := slay.ab.Encoding()
		dec := scene.NewDecoder(enc)
		if dec == nil {
			return fmt.Errorf("nil decoder")
		}
		n := 0
		for dec.Next() {
			n++
		}
		if n != len(enc.Tags()) {
			return fmt.Errorf("decoded %d tags, encoding has %d", n, len(enc.Tags()))
		}
		if len(enc.Brushes()) == 0 {
			return fmt.Errorf("no brushes encoded")
		}
		return nil
	})

	p("flatten_clone", func() error {
		flat := slay.ab.Flatten()
		if flat == nil {
			return fmt.Errorf("nil flatten")
		}
		if flat.Hash() != slay.ab.Encoding().Hash() {
			return fmt.Errorf("flatten changed hash: %#x vs %#x", flat.Hash(), slay.ab.Encoding().Hash())
		}
		return nil
	})

	p("append_translation", func() error {
		a := scene.NewScene()
		a.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(colRed), scene.NewRectShape(0, 0, 10, 10))
		other := scene.NewScene()
		other.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(colBlue), scene.NewRectShape(0, 0, 10, 10))
		before := a.Bounds()
		a.AppendWithTranslation(other, 50, 30)
		after := a.Bounds()
		if after.MaxX < before.MaxX+50 || after.MaxY < before.MaxY+30 {
			return fmt.Errorf("translated append bounds %+v (before %+v)", after, before)
		}
		return nil
	})

	p("hash_brush_sensitive", func() error {
		build := func(c render.RGBA) uint64 {
			b := scene.NewSceneBuilder()
			b.FillRect(8, 8, 48, 48, scene.SolidBrush(c))
			return b.Build().Encoding().Hash()
		}
		if build(colRed) == build(colBlue) {
			return fmt.Errorf("brush-only change kept hash")
		}
		return nil
	})

	p("hash_stable", func() error {
		build := func() uint64 {
			b := scene.NewSceneBuilder()
			b.FillRect(8, 8, 48, 48, scene.SolidBrush(colGold))
			return b.Build().Encoding().Hash()
		}
		if build() != build() {
			return fmt.Errorf("same input hashed differently")
		}
		return nil
	})

	p("cache_brush_distinct", func() error {
		left, ok := slay.cache.Get(slay.cacheHashes[0])
		if !ok {
			return fmt.Errorf("missing red entry")
		}
		right, ok := slay.cache.Get(slay.cacheHashes[1])
		if !ok {
			return fmt.Errorf("missing blue entry")
		}
		lr, lg, lb := pmRGB(left, 88, 75)
		rr, rg, rb := pmRGB(right, 88, 75)
		if math.Abs(lr-rr) < 0.3 || math.Abs(lb-rb) < 0.3 {
			return fmt.Errorf("cache entries not distinct: left=(%.2f,%.2f,%.2f) right=(%.2f,%.2f,%.2f)", lr, lg, lb, rr, rg, rb)
		}
		return nil
	})

	p("cache_stats", func() error {
		st := slay.cache.Stats()
		if st.Entries != 2 {
			return fmt.Errorf("entries=%d want 2", st.Entries)
		}
		if st.Hits < 2 {
			return fmt.Errorf("hits=%d want >=2", st.Hits)
		}
		return nil
	})

	p("cpu_layer_alpha", func() error {
		want := render.RGBA{
			R: 0.5*float64(colBlue.R) + 0.5*float64(sceneBG.R),
			G: 0.5*float64(colBlue.G) + 0.5*float64(sceneBG.G),
			B: 0.5*float64(colBlue.B) + 0.5*float64(sceneBG.B),
			A: 1,
		}
		return nearRGB(slay.abPM, 190, 44, want, 0.03)
	})

	p("cpu_layer_multiply", func() error {
		want := render.RGBA{
			R: float64(colYellow.R) * float64(colCyan.R),
			G: float64(colYellow.G) * float64(colCyan.G),
			B: float64(colYellow.B) * float64(colCyan.B),
			A: 1,
		}
		return nearRGB(slay.abPM, 60, 50, want, 0.04)
	})

	p("cpu_roundrect_clip", func() error {
		if err := nearRGB(slay.abPM, 240, 140, colGold, 0.03); err != nil {
			return fmt.Errorf("inside: %w", err)
		}
		return nearRGB(slay.abPM, 310, 140, sceneBG, 0.02)
	})

	p("cpu_circle_clip", func() error {
		if err := nearRGB(slay.abPM, 90, 145, colRed, 0.03); err != nil {
			return fmt.Errorf("inside: %w", err)
		}
		return nearRGB(slay.abPM, 150, 145, sceneBG, 0.02)
	})

	p("damage_first_full", func() error {
		if slay.damageFirst != image.Rect(0, 0, int(sceneW), int(sceneH)) {
			return fmt.Errorf("first render damage=%v want full canvas", slay.damageFirst)
		}
		return nil
	})

	p("damage_move_only", func() error {
		// Old rect (20,138)-(84,178), new rect (150,138)-(214,178).
		want := image.Rect(20, 138, 214, 178)
		if slay.damageMove != want {
			return fmt.Errorf("damage=%v want %v", slay.damageMove, want)
		}
		fullArea := int(sceneW) * int(sceneH)
		area := slay.damageMove.Dx() * slay.damageMove.Dy()
		if area*4 > fullArea {
			return fmt.Errorf("damage area %d too large for full %d", area, fullArea)
		}
		return nil
	})

	p("damage_incremental_matches_full", func() error {
		if slay.damageSHA != slay.fullSHA {
			return fmt.Errorf("incremental sha %x != full sha %x", slay.damageSHA[:8], slay.fullSHA[:8])
		}
		return nil
	})

	p("damage_no_change", func() error {
		if !slay.damageThird.Empty() {
			return fmt.Errorf("unchanged frame reported damage %v", slay.damageThird)
		}
		return nil
	})

	p("text_ink_cpu", func() error {
		n := 0
		for y := 20; y < 72; y++ {
			for x := 12; x < 200; x++ {
				r, g, b := pmRGB(slay.damagePM, x, y)
				if r > 0.5 && g > 0.5 && b > 0.5 {
					n++
				}
			}
		}
		if n < 150 {
			return fmt.Errorf("text ink pixels=%d want >=150", n)
		}
		return nil
	})

	p("image_quadrants_cpu", func() error {
		checks := []struct {
			x, y int
			want [3]float64
		}{
			{286, 32, [3]float64{1.0, 0.125, 0.125}},
			{318, 32, [3]float64{32.0 / 255, 220.0 / 255, 64.0 / 255}},
			{286, 64, [3]float64{40.0 / 255, 96.0 / 255, 1.0}},
			{318, 64, [3]float64{250.0 / 255, 220.0 / 255, 40.0 / 255}},
		}
		for _, c := range checks {
			r, g, b := pmRGB(slay.damagePM, c.x, c.y)
			if math.Abs(r-c.want[0]) > 0.04 || math.Abs(g-c.want[1]) > 0.04 || math.Abs(b-c.want[2]) > 0.04 {
				return fmt.Errorf("quadrant (%d,%d) got (%.3f,%.3f,%.3f) want (%.3f,%.3f,%.3f)",
					c.x, c.y, r, g, b, c.want[0], c.want[1], c.want[2])
			}
		}
		return nil
	})

	p("empty_scene", func() error {
		pm, err := renderSceneCPUFull(scene.NewScene(), 64, 64)
		if err != nil {
			return fmt.Errorf("render: %w", err)
		}
		if pm == nil {
			return fmt.Errorf("nil pixmap")
		}
		return nil
	})

	return ran, failed
}

type pointProbe struct {
	key  string
	desc string
	card int
	rect rect
	want [3]float64
	tol  float64
}

type inkProbe struct {
	key  string
	desc string
	card int
	rect rect
	min  int
}

func scenePointProbes() []pointProbe {
	ab := func(card int, prefix string) []pointProbe {
		// Coordinates are card-local; the shared scene is painted at
		// (sceneImgX, sceneImgY) inside each card.
		return []pointProbe{
			{prefix + "_yellow", "base fill", card, rect{sceneImgX + 20, sceneImgY + 20, 4, 4}, [3]float64{0.92, 0.80, 0.22}, 0.05},
			{prefix + "_alpha", "50% layer over bg", card, rect{sceneImgX + 188, sceneImgY + 42, 4, 4}, [3]float64{0.175, 0.285, 0.555}, 0.05},
			{prefix + "_multiply", "multiply layer", card, rect{sceneImgX + 58, sceneImgY + 48, 4, 4}, [3]float64{0.166, 0.528, 0.172}, 0.06},
			{prefix + "_rr_clip", "round-rect clipped out", card, rect{sceneImgX + 308, sceneImgY + 138, 4, 4}, [3]float64{0.10, 0.12, 0.16}, 0.04},
			{prefix + "_circle", "circle clip inside", card, rect{sceneImgX + 88, sceneImgY + 143, 4, 4}, [3]float64{0.90, 0.25, 0.28}, 0.05},
		}
	}
	probes := ab(0, "A")
	probes = append(probes, ab(1, "B")...)
	probes = append(probes,
		pointProbe{"C_left", "cached red scene", 2, rect{98, 97, 4, 4}, [3]float64{0.86, 0.20, 0.22}, 0.04},
		pointProbe{"C_right", "cached blue scene", 2, rect{296, 97, 4, 4}, [3]float64{0.22, 0.42, 0.92}, 0.04},
		// Card D image region origin is (sceneImgX, sceneImgY); image quadrants
		// live at scene-local (270..334, 16..80).
		pointProbe{"D_q_red", "image red quadrant", 3, rect{300, 62, 4, 4}, [3]float64{1.0, 0.125, 0.125}, 0.05},
		pointProbe{"D_q_green", "image green quadrant", 3, rect{332, 62, 4, 4}, [3]float64{0.125, 0.863, 0.251}, 0.05},
		pointProbe{"D_q_blue", "image blue quadrant", 3, rect{300, 94, 4, 4}, [3]float64{0.157, 0.376, 1.0}, 0.05},
		pointProbe{"D_moved", "damage frame moved rect", 3, rect{16 + 182, 32 + 158, 4, 4}, [3]float64{0.20, 0.78, 0.35}, 0.04},
		pointProbe{"D_old_clear", "old rect area cleared", 3, rect{16 + 52, 32 + 158, 4, 4}, [3]float64{0.10, 0.12, 0.16}, 0.03},
	)
	return probes
}

func sceneInkProbes() []inkProbe {
	return []inkProbe{
		{"D_text", "scene DrawText ink", 3, rect{sceneImgX + 12, sceneImgY + 20, 180, 60}, 120},
	}
}

// sceneParity compares the same scene drawn by the GPU renderer (card A) and
// the CPU tile renderer (card B), pixel for pixel within the image area.
func sceneParity(img image.Image, bx, by, dpr float64) (mismatchPct, highPct float64, worst int, valid bool) {
	if img == nil || dpr <= 0 {
		return 100, 100, 255, false
	}
	ax := int((bx + cardAt[0][0] + sceneImgX) * dpr)
	ay := int((by + cardAt[0][1] + sceneImgY) * dpr)
	bxx := int((bx + cardAt[1][0] + sceneImgX) * dpr)
	byy := int((by + cardAt[1][1] + sceneImgY) * dpr)
	w, h := int(sceneW*dpr), int(sceneH*dpr)
	if ax < 0 || bxx < 0 || ay < 0 || byy < 0 ||
		ax+w > img.Bounds().Dx() || bxx+w > img.Bounds().Dx() ||
		ay+h > img.Bounds().Dy() || byy+h > img.Bounds().Dy() {
		return 100, 100, 255, false
	}
	var mismatch, high, total int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, ab, _ := img.At(ax+x, ay+y).RGBA()
			br, bg, bb, _ := img.At(bxx+x, byy+y).RGBA()
			d := max3(absInt(int(ar>>8)-int(br>>8)), absInt(int(ag>>8)-int(bg>>8)), absInt(int(ab>>8)-int(bb>>8)))
			if d > worst {
				worst = d
			}
			if d > parityTol {
				mismatch++
			}
			if d > parityHighDelta {
				high++
			}
			total++
		}
	}
	if total == 0 {
		return 100, 100, 255, false
	}
	return float64(mismatch) * 100 / float64(total),
		float64(high) * 100 / float64(total),
		worst, true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func max3(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

func buildSceneScene(shell *wrkit.ShellChrome) {
	cardWithBoundary := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	cardWithBoundary(0, "A  GPU scene · layer α · multiply · clip", paintSceneA)
	cardWithBoundary(1, "B  CPU tile · same scene (A/B parity)", paintSceneB)
	cardWithBoundary(2, "C  Encoding.Hash · LayerCache", paintSceneC)
	cardWithBoundary(3, "D  text + image · damage render", paintSceneD)
}

func paintSceneA(pc *rendering.PaintContext, _ rendering.Size) {
	if pc == nil || pc.DC == nil || slay.ab == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	paintSceneCardBG(pc)
	dc.Push()
	dc.Translate(ox+sceneImgX, oy+sceneImgY)
	// The GPU scene renderer talks to the device directly, so it can only run on
	// the live context; a recording walk refuses and counts the refusal.
	if live, ok := pc.RequireLiveContext(); ok {
		_ = scene.NewGPUSceneRenderer(live).RenderScene(slay.ab)
	}
	dc.Pop()
	paintSceneFrame(pc, sceneImgX, sceneImgY, sceneW, sceneH)
}

func paintSceneB(pc *rendering.PaintContext, _ rendering.Size) {
	if pc == nil || pc.DC == nil || slay.abImg == nil {
		return
	}
	paintSceneCardBG(pc)
	pc.DC.DrawImageEx(slay.abImg, render.DrawImageOptions{
		X: pc.OriginX + sceneImgX, Y: pc.OriginY + sceneImgY,
		DstWidth: sceneW, DstHeight: sceneH,
		Interpolation: render.InterpNearest, Opacity: 1,
	})
	paintSceneFrame(pc, sceneImgX, sceneImgY, sceneW, sceneH)
}

func paintSceneC(pc *rendering.PaintContext, _ rendering.Size) {
	if pc == nil || pc.DC == nil || slay.cacheL == nil || slay.cacheR == nil {
		return
	}
	paintSceneCardBG(pc)
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	dc.DrawImageEx(slay.cacheL, render.DrawImageOptions{
		X: ox + 12, Y: oy + 24, DstWidth: 176, DstHeight: 150,
		Interpolation: render.InterpNearest, Opacity: 1,
	})
	dc.DrawImageEx(slay.cacheR, render.DrawImageOptions{
		X: ox + 210, Y: oy + 24, DstWidth: 176, DstHeight: 150,
		Interpolation: render.InterpNearest, Opacity: 1,
	})
	paintSceneFrame(pc, 12, 24, 176, 150)
	paintSceneFrame(pc, 210, 24, 176, 150)
}

func paintSceneD(pc *rendering.PaintContext, _ rendering.Size) {
	if pc == nil || pc.DC == nil || slay.damageImg == nil {
		return
	}
	paintSceneCardBG(pc)
	pc.DC.DrawImageEx(slay.damageImg, render.DrawImageOptions{
		X: pc.OriginX + sceneImgX, Y: pc.OriginY + sceneImgY,
		DstWidth: sceneW, DstHeight: sceneH,
		Interpolation: render.InterpNearest, Opacity: 1,
	})
	paintSceneFrame(pc, sceneImgX, sceneImgY, sceneW, sceneH)
}

func paintSceneCardBG(pc *rendering.PaintContext) {
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, cardW, cardH)
	_ = dc.Fill()
}

func paintSceneFrame(pc *rendering.PaintContext, x, y, w, h float64) {
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	dc.SetRGBA(colBorder[0], colBorder[1], colBorder[2], 1)
	dc.SetLineWidth(1)
	dc.DrawRectangle(ox+x, oy+y, w, h)
	_ = dc.Stroke()
}
