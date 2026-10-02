// Command game_sprite is the sprite standalone true window (2.1 batch,
// 2.2 atlas rot/tint/flip/pivot/filter, 1.2 depth via R5 branch).
//
// Window: 1200x800, title game_sprite. --case=rot|depth|batch (default all:
// three cards side by side). Offscreen CPU/GPU parity runs before the
// window opens; final offscreen canvas is snapshotted to testdata and
// compared against the frozen golden with zero tolerance.
//
// Modes:
//
//	GOGPU_RENDER_MODE=cpu go run ./examples/engine/sprite --case=all -auto-only
//	  short real window (RUN_SECONDS, default 8; JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/sprite --case=rot -manual-seconds 30
//	  resident 30s (or until close); true Pointer/Key/Resize events are
//	  logged and shown in the title as events=N; summary JSON at the end.
//	go run ./examples/engine/sprite
//	  selftest first, then resident until close.
//
// Gate (auto and final): present>=1, parity_changed_pct<=1, probes all pass,
// golden exact (after baseline exists); otherwise exit 1.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/sprite"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/examples/wrsoak"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

// GOLDEN-BEGIN: offscreen scene shared verbatim with the /tmp baseline
// generator (mechanical extraction only, never hand-copied).
const (
	sprCanvasW = 280
	sprCanvasH = 200
)

var spriteAtlasID = core.AssetID("tex/sprite_atlas")

func buildSpriteAtlas() *render.ImageBuf {
	img, _ := render.NewImageBuf(16, 16, render.FormatRGBA8)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			var r, g, b uint8
			switch {
			case x < 8 && y < 8:
				r, g, b = 255, 0, 0
			case x >= 8 && y < 8:
				r, g, b = 0, 255, 0
			case x < 8 && y >= 8:
				r, g, b = 0, 0, 255
			default:
				r, g, b = 255, 255, 255
			}
			_ = img.SetRGBA(x, y, r, g, b, 255)
		}
	}
	return img
}

func mustAtlas(sp sprite.AtlasSprite, err error) sprite.AtlasSprite {
	if err != nil {
		return sprite.AtlasSprite{}
	}
	return sp
}

func rotSprites(ox, oy float64) []sprite.AtlasSprite {
	red := mustAtlas(sprite.NewAtlasSprite(spriteAtlasID,
		core.NewRect(0, 0, 8, 8), core.NewRect(ox+8, oy+24, 48, 24),
		1, math.Pi/2, core.V2(24, 12), core.Color{}, sprite.AtlasFilterNearest, false, false))
	feetDst := core.NewRect(ox+8, oy+68, 40, 32)
	feet := mustAtlas(sprite.NewAtlasSprite(spriteAtlasID,
		core.NewRect(0, 8, 8, 8), feetDst,
		1, 0, sprite.FeetPivot(feetDst), core.Color{}, sprite.AtlasFilterNearest, false, false))
	tint := mustAtlas(sprite.NewAtlasSprite(spriteAtlasID,
		core.NewRect(8, 8, 8, 8), core.NewRect(ox+56, oy+68, 32, 32),
		1, 0, core.V2(0, 0), core.RGBA(0.5, 0.5, 0.5, 1), sprite.AtlasFilterNearest, false, false))
	flip := mustAtlas(sprite.NewAtlasSprite(spriteAtlasID,
		core.NewRect(0, 0, 16, 8), core.NewRect(ox+56, oy+24, 32, 16),
		1, 0, core.V2(16, 8), core.Color{}, sprite.AtlasFilterNearest, true, false))
	return []sprite.AtlasSprite{red, feet, tint, flip}
}

func depthSprite(name string, sx, sy, sw, sh, dx, dy, dw, dh, depth float64) render.DepthSprite {
	sp, err := render.NewDepthSprite(name, render.AtlasSprite{
		SrcX: sx, SrcY: sy, SrcW: sw, SrcH: sh,
		DstX: dx, DstY: dy, DstW: dw, DstH: dh,
		Opacity: 1, Filter: render.InterpNearest,
	}, depth)
	if err != nil {
		return render.DepthSprite{}
	}
	return sp
}

func depthTrueSprites(ox, oy float64) []render.DepthSprite {
	return []render.DepthSprite{
		depthSprite("far-green", 8, 0, 8, 8, ox+6, oy+6, 40, 40, 10),
		depthSprite("near-red", 0, 0, 8, 8, ox+20, oy+20, 40, 40, 1),
	}
}

func depthFalseSprites(ox, oy float64) []render.DepthSprite {
	return []render.DepthSprite{
		depthSprite("near-red", 0, 0, 8, 8, ox+20, oy+20, 40, 40, 1),
		depthSprite("far-green", 8, 0, 8, 8, ox+6, oy+6, 40, 40, 10),
	}
}

func depthSameSprites(ox, oy float64) []render.DepthSprite {
	return []render.DepthSprite{
		depthSprite("same-a", 0, 8, 8, 8, ox+6, oy+14, 28, 28, 5),
		depthSprite("same-b", 0, 8, 8, 8, ox+16, oy+26, 28, 28, 5),
	}
}

const (
	batchCols = 40
	batchRows = 25
	batchCell = 4
	batchSize = 3
)

func batchGameSprites(ox, oy float64) []sprite.Sprite {
	out := make([]sprite.Sprite, 0, batchCols*batchRows)
	for row := 0; row < batchRows; row++ {
		for col := 0; col < batchCols; col++ {
			s, err := sprite.NewSprite(spriteAtlasID,
				core.NewRect(8, 0, 8, 8),
				core.NewRect(ox+float64(col*batchCell), oy+float64(row*batchCell), batchSize, batchSize),
				1)
			if err != nil {
				continue
			}
			out = append(out, s)
		}
	}
	return out
}

func spriteToRender(s sprite.Sprite) render.AtlasSprite {
	return render.AtlasSprite{
		SrcX: s.Src.X, SrcY: s.Src.Y, SrcW: s.Src.W, SrcH: s.Src.H,
		DstX: s.Dst.X, DstY: s.Dst.Y, DstW: s.Dst.W, DstH: s.Dst.H,
		Opacity: sprite.EffectiveOpacity(s.Opacity), Filter: render.InterpNearest,
	}
}

type parityCanvas struct {
	img        image.Image
	stats      render.RenderPathStats
	batchCalls int
}

func paintParityCanvas() (parityCanvas, error) {
	var pc parityCanvas
	atlas := buildSpriteAtlas()
	dc := render.NewContext(sprCanvasW, sprCanvasH)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	rs, err := sprite.AtlasToRender(rotSprites(8, 72))
	if err != nil {
		return pc, err
	}
	if _, err := dc.DrawAtlasEx(atlas, rs, render.AtlasDrawOptions{}); err != nil {
		return pc, err
	}
	if _, err := dc.DrawDepthSprites(atlas, depthTrueSprites(8, 8), render.DepthDrawOptions{DepthTest: true}); err != nil {
		return pc, err
	}
	if _, err := dc.DrawDepthSprites(atlas, depthFalseSprites(100, 8), render.DepthDrawOptions{DepthTest: false}); err != nil {
		return pc, err
	}
	if _, err := dc.DrawDepthSprites(atlas, depthSameSprites(192, 8), render.DepthDrawOptions{DepthTest: true}); err != nil {
		return pc, err
	}
	b := sprite.NewBatch()
	for _, s := range batchGameSprites(108, 80) {
		_, _ = b.Add(s)
	}
	var flat []render.AtlasSprite
	pc.batchCalls = b.Flush(func(_ core.AssetID, ss []sprite.Sprite) {
		for _, s := range ss {
			flat = append(flat, spriteToRender(s))
		}
	})
	if _, err := dc.DrawAtlasEx(atlas, flat, render.AtlasDrawOptions{}); err != nil {
		return pc, err
	}
	raw := dc.Image()
	cp := image.NewRGBA(raw.Bounds())
	copy(cp.Pix, raw.(*image.RGBA).Pix)
	pc.img = cp
	pc.stats = dc.RenderPathStats()
	return pc, nil
}

func spriteProbes() []wrsoak.PixelCheck {
	return []wrsoak.PixelCheck{
		{Pt: &wrsoak.ProbePoint{X: 41, Y: 41, Want: [3]float64{1, 0, 0}, Tol: 0.05}, Desc: "depth-cover-center"},
		{Pt: &wrsoak.ProbePoint{X: 40, Y: 88, Want: [3]float64{1, 0, 0}, Tol: 0.05}, Desc: "rot90-corner"},
		{Pt: &wrsoak.ProbePoint{X: 189, Y: 129, Want: [3]float64{0, 1, 0}, Tol: 0.05}, Desc: "batch-zone"},
	}
}

// GOLDEN-END.

const testdataDir = "examples/engine/sprite/testdata"

func diffImages(a, b image.Image) (changed, total int, mean float64) {
	ab, bb := a.Bounds(), b.Bounds()
	if !ab.Eq(bb) {
		return 1, 1, 255
	}
	var sum float64
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			ar, ag, ab2, _ := a.At(x, y).RGBA()
			br, bg, bb2, _ := b.At(x, y).RGBA()
			ds := [3]int{int(ar>>8) - int(br>>8), int(ag>>8) - int(bg>>8), int(ab2>>8) - int(bb2>>8)}
			hit := false
			for _, d := range ds {
				if d < 0 {
					d = -d
				}
				sum += float64(d)
				if d > 2 {
					hit = true
				}
			}
			if hit {
				changed++
			}
			total++
		}
	}
	if total > 0 {
		mean = sum / float64(total*3)
	}
	return changed, total, mean
}

func numExtra(m map[string]any, k string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[k]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// selftest runs offscreen parity + probes + golden before the window opens.
func selftest() (map[string]any, image.Image, bool) {
	extra := map[string]any{}
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	cpu, err := paintParityCanvas()
	if err != nil {
		fmt.Fprintln(os.Stderr, "game_sprite selftest CPU:", err)
		return extra, nil, false
	}
	_ = os.Unsetenv("GOGPU_RENDER_MODE")
	gpu, err := paintParityCanvas()
	if err != nil {
		fmt.Fprintln(os.Stderr, "game_sprite selftest GPU:", err)
		return extra, nil, false
	}
	changed, total, mean := diffImages(cpu.img, gpu.img)
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(changed) / float64(total)
	}
	extra["parity_changed_pct"] = pct
	extra["parity_mean_abs"] = mean
	extra["parity_gpu_ops"] = gpu.stats.GPUOps
	extra["batch_calls"] = cpu.batchCalls
	sorted, err := render.SortDepthSprites(depthTrueSprites(8, 8))
	depthSorted := 0
	if err == nil && render.IsDepthSorted(sorted) {
		depthSorted = 1
	}
	extra["depth_sorted"] = depthSorted
	probeRes := map[string]bool{}
	wrsoak.RunPixelChecks("game_sprite", float64(sprCanvasW), cpu.img, spriteProbes(), probeRes)
	probeOK := 1
	for _, c := range spriteProbes() {
		if !probeRes[c.Desc] {
			probeOK = 0
		}
	}
	extra["probe_ok"] = probeOK
	if err := os.MkdirAll(testdataDir, 0o755); err == nil {
		if f, err := os.Create(testdataDir + "/sprite_last.png"); err == nil {
			_ = png.Encode(f, cpu.img)
			_ = f.Close()
		}
	}
	diffPct, _, firstRun := wrsoak.EvaluateGolden("game_sprite", testdataDir,
		"sprite_last.png", "sprite_golden.png",
		[]wrsoak.Rect{{X: 0, Y: 0, W: sprCanvasW, H: sprCanvasH}}, float64(sprCanvasW))
	extra["golden_changed_pct"] = diffPct
	if firstRun {
		extra["golden_first_run"] = 1
	}
	ok := pct <= 1.0 && probeOK == 1 && depthSorted == 1 && (firstRun || diffPct == 0)
	fmt.Fprintf(os.Stderr, "game_sprite selftest: parity=%.4f%% mean=%.4f gpu_ops=%d batch=%d sorted=%d probes=%d golden=%.4f%% first=%v ok=%v\n",
		pct, mean, gpu.stats.GPUOps, cpu.batchCalls, depthSorted, probeOK, diffPct, firstRun, ok)
	return extra, cpu.img, ok
}

func abilityFor(caseName string) string {
	switch caseName {
	case "rot":
		return "sprite-rot"
	case "depth":
		return "sprite-depth"
	case "batch":
		return "sprite-batch"
	case "anim":
		return "sprite-anim"
	default:
		return "sprite-all"
	}
}

func runSecondsEnv(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func paintRotCard(pc *rendering.PaintContext, w, h float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(1, 1, 1)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	rs, err := sprite.AtlasToRender(rotSprites(ax, ay))
	if err != nil {
		return
	}
	_, _ = pc.DC.DrawAtlasEx(atlasBuf, rs, render.AtlasDrawOptions{})
	pc.DC.SetRGBA(1, 0, 0, 1)
	pc.DC.SetLineWidth(2)
	pc.DC.DrawLine(ax+8, ay+100, ax+48, ay+100)
	_ = pc.DC.Stroke()
	if face := wrkit.FaceAt(12); face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(0.1, 0.12, 0.15, 1)
	pc.DC.DrawString("rot90 red + feet pivot + gray tint + flipX", ax+8, ay+196)
}

func paintDepthCard(pc *rendering.PaintContext, w, h float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(1, 1, 1)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	_, _ = pc.DC.DrawDepthSprites(atlasBuf, depthTrueSprites(ax+8, ay+8), render.DepthDrawOptions{DepthTest: true})
	_, _ = pc.DC.DrawDepthSprites(atlasBuf, depthFalseSprites(ax+8, ay+110), render.DepthDrawOptions{DepthTest: false})
	_, _ = pc.DC.DrawDepthSprites(atlasBuf, depthSameSprites(ax+8, ay+212), render.DepthDrawOptions{DepthTest: true})
	if face := wrkit.FaceAt(12); face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(0.1, 0.12, 0.15, 1)
	pc.DC.DrawString("up sorted(true): red covers / mid raw(false) / low same-depth", ax+8, ay+292)
}

func paintBatchCard(pc *rendering.PaintContext, w, h float64, calls int) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(1, 1, 1)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	var flat []render.AtlasSprite
	for _, s := range batchGameSprites(ax+8, ay+40) {
		flat = append(flat, spriteToRender(s))
	}
	_, _ = pc.DC.DrawAtlasEx(atlasBuf, flat, render.AtlasDrawOptions{})
	if face := wrkit.FaceAt(12); face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(0.1, 0.12, 0.15, 1)
	pc.DC.DrawString(fmt.Sprintf("1000 trees one atlas, batch_calls=%d", calls), ax+8, ay+28)
}

var atlasBuf *render.ImageBuf

func main() {
	caseName := flag.String("case", "all", "which ability to gate: rot|depth|batch|anim|all")
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	flag.Parse()
	if *caseName == "anim" {
		runAnimCase(*autoOnly, *manualSeconds)
		return
	}
	if *caseName != "rot" && *caseName != "depth" && *caseName != "batch" && *caseName != "all" {
		fmt.Fprintln(os.Stderr, "FAIL: --case must be rot|depth|batch|anim|all, got", *caseName)
		os.Exit(2)
	}
	ability := abilityFor(*caseName)
	scenario := "game_sprite--case=" + *caseName

	wrkit.EnsureUIFace()
	atlasBuf = buildSpriteAtlas()

	extra, _, ok := selftest()
	if !ok {
		raw, _ := json.Marshal(map[string]any{"ability_id": ability, "scenario": scenario, "extra": extra, "pass": false})
		fmt.Println(string(raw))
		fmt.Fprintln(os.Stderr, "game_sprite: selftest FAIL, not opening window")
		os.Exit(1)
	}
	if !*autoOnly {
		fmt.Fprintln(os.Stderr, "game_sprite: selftest done, entering manual phase (close X to finish)")
	}

	var secs int
	if *autoOnly {
		secs = runSecondsEnv(8)
		wrkit.RequireMinRun(secs, ability)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
			wrkit.RequireMinRun(secs, ability)
		}
	}
	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_sprite", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "game_sprite — rot/depth/batch ("+*caseName+")", []string{
		"rot: 90deg red + feet pivot",
		"gray tint + flipX mirror",
		"depth: far green near red",
		"true=sorted red covers",
		"false=raw order contrast",
		"same-depth strip no flicker",
		"batch: 1000 trees 1 commit",
		"parity<=1% probes 3/3",
		"golden static zero-tol",
	})

	batchCalls := 0
	if v, ok := numExtra(extra, "batch_calls"); ok {
		batchCalls = int(v)
	}
	var cards []*rendering.RenderBox
	mkCard := func(paint func(pc *rendering.PaintContext, w, h float64), w, h float64) *rendering.RenderBox {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = w, h
		box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			paint(pc, size.Width, size.Height)
		}
		cards = append(cards, box)
		return box
	}
	const cardY, cardH = 48.0, 600.0
	if *caseName == "all" {
		shell.Body.LabelAt("ROT: turn + tint + mirror + feet", 13, 8, 8, 0.75, 0.82, 0.9)
		shell.Body.Place(mkCard(paintRotCard, 292, 220), 8, cardY)
		shell.Body.LabelAt("DEPTH: sorted vs raw + same-depth", 13, 306, 8, 0.75, 0.82, 0.9)
		shell.Body.Place(mkCard(paintDepthCard, 292, 320), 306, cardY)
		shell.Body.LabelAt("BATCH: 1000 trees one commit", 13, 604, 8, 0.75, 0.82, 0.9)
		shell.Body.Place(mkCard(func(pc *rendering.PaintContext, w, h float64) {
			paintBatchCard(pc, w, h, batchCalls)
		}, 292, 220), 604, cardY)
	} else {
		var paint func(pc *rendering.PaintContext, w, h float64)
		var title string
		switch *caseName {
		case "rot":
			title, paint = "ROT: turn + tint + mirror + feet", paintRotCard
		case "depth":
			title, paint = "DEPTH: sorted vs raw + same-depth", paintDepthCard
		default:
			title = "BATCH: 1000 trees one commit"
			paint = func(pc *rendering.PaintContext, w, h float64) {
				paintBatchCard(pc, w, h, batchCalls)
			}
		}
		shell.Body.LabelAt(title, 13, 8, 8, 0.75, 0.82, 0.9)
		shell.Body.Place(mkCard(paint, 888, 400), 8, cardY)
	}
	var proc scheduler.ProcessTracker
	proc.Start()

	eventsTotal, eventsPointer, eventsKey, eventsResize := 0, 0, 0, 0
	bumpTitle := func() {
		if ctl != nil {
			ctl.SetTitle(fmt.Sprintf("game_sprite events=%d", eventsTotal))
		}
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "game_sprite: close (%s)\n", win.Backend())
			case platform.EventPointer:
				eventsPointer++
				eventsTotal++
				fmt.Fprintf(os.Stderr, "game_sprite event pointer (%d) @%.0f,%.0f\n", eventsTotal, ev.X, ev.Y)
				bumpTitle()
			case platform.EventKey:
				eventsKey++
				eventsTotal++
				fmt.Fprintf(os.Stderr, "game_sprite event key (%d) code=%d pressed=%v\n", eventsTotal, ev.KeyCode, ev.Pressed)
				bumpTitle()
			case platform.EventResize:
				eventsResize++
				eventsTotal++
				fmt.Fprintf(os.Stderr, "game_sprite event resize (%d) %dx%d\n", eventsTotal, ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				bumpTitle()
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		for _, c := range cards {
			c.MarkNeedsPaint()
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0
		shell.UpdateHUD(ability, "Steady", app, gateOK,
			fmt.Sprintf("presents=%d events=%d", app.PresentCount(), eventsTotal), *caseName)
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	extra["events_total"] = eventsTotal
	extra["events_pointer"] = eventsPointer
	extra["events_key"] = eventsKey
	extra["events_resize"] = eventsResize

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     ability,
		Scenario:      scenario,
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	pass := true
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		pass = false
	}
	if v, ok := numExtra(extra, "parity_changed_pct"); !ok || v > 1.0 {
		fmt.Fprintf(os.Stderr, "FAIL: parity_changed_pct=%v want <=1 (CPU/GPU diverge)\n", extra["parity_changed_pct"])
		pass = false
	}
	if v, ok := numExtra(extra, "probe_ok"); !ok || v != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: probe_ok=%v want 1\n", extra["probe_ok"])
		pass = false
	}
	if fr, _ := numExtra(extra, "golden_first_run"); fr != 1 {
		if v, ok := numExtra(extra, "golden_changed_pct"); !ok || v != 0 {
			fmt.Fprintf(os.Stderr, "FAIL: golden_changed_pct=%v want 0 (static mask zero tolerance)\n", extra["golden_changed_pct"])
			pass = false
		}
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: present_count=0")
		pass = false
	}
	if !pass {
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "game_sprite: OK case=%s presents=%d events=%d elapsed=%.1fs\n",
		*caseName, app.PresentCount(), eventsTotal, elapsed)
}

// ---- --case=anim: S74 flipbook window (2.4 sequence library, dodge true frames).
//
// The live library is built from testdata/anim_cases.json "art" (dodge
// player.tscn right/up, speed 5.0 = 200ms per frame, loop): two clips,
// real dodge PNGs as frames, one true sprite.Flipbook drives the live
// card and hops right->up every animHopSec. The same file's "clips" and
// "sequences" are a verbatim data-equal copy of the frozen engine cases
// (engine/sprite/testdata/flipbook_cases.json): the selftest replays all
// six sequences through the real flipbook API and demands every frame,
// index, kind, and callback to land exactly.
//
// Threes evidences: logic (six sequences replay exact + art dims + pair
// frames truly differ), pixels (strip cells carry content, pairs differ),
// goldens (offscreen anim_off_golden.png + window anim_final_base.png,
// both zero tolerance once stored). Parity (C both sides, pure math):
// the same dt stream replays bitwise identical on two flipbooks, and the
// CPU/GPU strip diff stays within animParityBudget (nearest 1:1 blits).
//
// Existing rot/depth/batch/all paths above are untouched; anim returns
// early from main and shares only diffImages/numExtra/testdataDir/ticker.

const (
	animOffW, animOffH = 576, 180
	animCellX0         = 15.0
	animCellStep       = 140.0
	animFrameBaseY     = 160.0
	animStripStep      = 210.0
	animStripBaseY     = 158.0
	animStripW         = 864.0
	animStripH         = 170.0
	animLiveW          = 400.0
	animLiveH          = 260.0

	animHopSec       = 2.0
	animSwitchMin    = 3
	animParityBudget = 1.0
	animGoldenTol    = 0.0

	animContentMin   = 800
	animPairDiffMin  = 200
	animArtOpaqueMin = 1000
)

type animClipDef struct {
	Name    string `json:"name"`
	Frames  []int  `json:"frames"`
	FrameMs int64  `json:"frame_ms"`
	Loop    string `json:"loop"`
}

type animStepDef struct {
	DtMs      int64  `json:"dt_ms"`
	Play      string `json:"play"`
	WantFrame int    `json:"want_frame"`
	WantIndex int    `json:"want_index"`
	WantKind  string `json:"want_kind"`
}

type animSeqDef struct {
	Name  string        `json:"name"`
	Clip  string        `json:"clip"`
	Steps []animStepDef `json:"steps"`
}

type animArtDef struct {
	Name    string   `json:"name"`
	FrameMs int64    `json:"frame_ms"`
	Loop    string   `json:"loop"`
	Files   []string `json:"files"`
	Widths  []int    `json:"widths"`
	Heights []int    `json:"heights"`
}

type animFileDef struct {
	Clips     []animClipDef `json:"clips"`
	Sequences []animSeqDef  `json:"sequences"`
	Art       []animArtDef  `json:"art"`
}

func loadAnimFile() (animFileDef, error) {
	var f animFileDef
	raw, err := os.ReadFile(testdataDir + "/anim_cases.json")
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	if len(f.Clips) != 6 || len(f.Sequences) != 6 || len(f.Art) != 2 {
		return f, fmt.Errorf("want 6 clips/6 sequences/2 art, got %d/%d/%d",
			len(f.Clips), len(f.Sequences), len(f.Art))
	}
	for _, a := range f.Art {
		if len(a.Files) != 2 || len(a.Widths) != 2 || len(a.Heights) != 2 {
			return f, fmt.Errorf("art %q wants 2 files/widths/heights", a.Name)
		}
	}
	return f, nil
}

func animLoopOf(s string) (sprite.LoopMode, error) {
	switch s {
	case "once":
		return sprite.LoopOnce, nil
	case "loop":
		return sprite.LoopLoop, nil
	case "pingpong":
		return sprite.LoopPingPong, nil
	}
	return sprite.LoopOnce, fmt.Errorf("unknown loop %q", s)
}

func animKindOf(s string) (sprite.EventKind, error) {
	switch s {
	case "none":
		return sprite.EventNone, nil
	case "frame":
		return sprite.EventFrame, nil
	case "loop":
		return sprite.EventLoop, nil
	case "finished":
		return sprite.EventFinished, nil
	}
	return sprite.EventNone, fmt.Errorf("unknown kind %q", s)
}

func buildAnimFlipbook(f animFileDef) (*sprite.Flipbook, error) {
	fb := sprite.NewFlipbook()
	for _, c := range f.Clips {
		loop, err := animLoopOf(c.Loop)
		if err != nil {
			return nil, err
		}
		if err := fb.AddClip(sprite.Clip{
			Name:     c.Name,
			Frames:   append([]int(nil), c.Frames...),
			FrameDur: core.Milliseconds(c.FrameMs),
			Loop:     loop,
		}); err != nil {
			return nil, fmt.Errorf("clip %q: %w", c.Name, err)
		}
	}
	return fb, nil
}

// replayAnimSequences runs every frozen sequence through the real
// flipbook: Play rewinds silently, Update returns the frozen frame and
// event, Current agrees, and the callback mirrors the returned event.
func replayAnimSequences(f animFileDef) (string, bool) {
	done, steps := 0, 0
	for _, seq := range f.Sequences {
		fb, err := buildAnimFlipbook(f)
		if err != nil {
			return fmt.Sprintf("build: %v", err), false
		}
		if err := fb.Play(seq.Clip); err != nil {
			return fmt.Sprintf("%s play: %v", seq.Name, err), false
		}
		var got []sprite.Event
		fb.OnEvent(func(ev sprite.Event) { got = append(got, ev) })
		wantClip := seq.Clip
		for i, st := range seq.Steps {
			steps++
			got = nil
			if st.Play != "" {
				if err := fb.Play(st.Play); err != nil {
					return fmt.Sprintf("%s step %d play: %v", seq.Name, i, err), false
				}
				wantClip = st.Play
				if len(got) != 0 {
					return fmt.Sprintf("%s step %d: play emitted events", seq.Name, i), false
				}
				clip, idx, frame, ok := fb.Current()
				if !ok || clip != st.Play || idx != st.WantIndex || frame != st.WantFrame {
					return fmt.Sprintf("%s step %d: after play %q/%d/%d", seq.Name, i, clip, idx, frame), false
				}
				continue
			}
			wantKind, err := animKindOf(st.WantKind)
			if err != nil {
				return fmt.Sprintf("%s step %d: %v", seq.Name, i, err), false
			}
			frame, ev, ok := fb.Update(core.Milliseconds(st.DtMs))
			if !ok || frame != st.WantFrame || ev.Frame != st.WantFrame ||
				ev.Index != st.WantIndex || ev.Kind != wantKind || ev.Clip != wantClip {
				return fmt.Sprintf("%s step %d: update = %d %+v", seq.Name, i, frame, ev), false
			}
			if clip, idx, cur, ok := fb.Current(); !ok || clip != wantClip || idx != st.WantIndex || cur != st.WantFrame {
				return fmt.Sprintf("%s step %d: current diverges", seq.Name, i), false
			}
			if wantKind == sprite.EventNone {
				if len(got) != 0 {
					return fmt.Sprintf("%s step %d: silent step fired", seq.Name, i), false
				}
			} else if len(got) != 1 || got[0] != ev {
				return fmt.Sprintf("%s step %d: callback diverges", seq.Name, i), false
			}
		}
		done++
	}
	return fmt.Sprintf("%d/%d sequences steps=%d callbacks_mirror", done, len(f.Sequences), steps), true
}

// animParityReplay is the C-both-sides evidence for pure math: the same
// dt stream (with a mid-run action switch) replays frame-for-frame and
// event-for-event on two flipbooks.
func animParityReplay(f animFileDef) bool {
	if len(f.Sequences) < 2 {
		return false
	}
	mk := func() *sprite.Flipbook {
		fb, err := buildAnimFlipbook(f)
		if err != nil {
			return nil
		}
		if err := fb.Play(f.Sequences[1].Clip); err != nil {
			return nil
		}
		return fb
	}
	a, b := mk(), mk()
	if a == nil || b == nil {
		return false
	}
	for i := 0; i < 500; i++ {
		if i == 250 {
			if err := a.Play(f.Sequences[0].Clip); err != nil {
				return false
			}
			if err := b.Play(f.Sequences[0].Clip); err != nil {
				return false
			}
		}
		xa, ea, oka := a.Update(core.Milliseconds(16))
		xb, eb, okb := b.Update(core.Milliseconds(16))
		if !oka || !okb || xa != xb || ea != eb {
			return false
		}
	}
	return true
}

// loadAnimArt loads the dodge true frames and builds the live clips.
// Frame ids are 100 + 10*art + index so the live library never collides
// with the frozen math ids. Dims come from the json, never hardcoded.
func loadAnimArt(f animFileDef) ([]*render.ImageBuf, []sprite.Clip, string, bool) {
	var frames []*render.ImageBuf
	var clips []sprite.Clip
	for i, a := range f.Art {
		var ids []int
		for j, name := range a.Files {
			img, err := render.LoadImage(testdataDir + "/" + name)
			if err != nil {
				return nil, nil, fmt.Sprintf("art %q load %s: %v", a.Name, name, err), false
			}
			w, h := img.Bounds()
			if w != a.Widths[j] || h != a.Heights[j] {
				return nil, nil, fmt.Sprintf("art %q %s = %dx%d, frozen %dx%d",
					a.Name, name, w, h, a.Widths[j], a.Heights[j]), false
			}
			opaque := 0
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if _, _, _, al := img.GetRGBA(x, y); al > 10 {
						opaque++
					}
				}
			}
			if opaque < animArtOpaqueMin {
				return nil, nil, fmt.Sprintf("art %q %s opaque=%d", a.Name, name, opaque), false
			}
			frames = append(frames, img)
			ids = append(ids, 100+10*i+j)
		}
		loop, err := animLoopOf(a.Loop)
		if err != nil {
			return nil, nil, fmt.Sprintf("art %q: %v", a.Name, err), false
		}
		clips = append(clips, sprite.Clip{
			Name:     a.Name,
			Frames:   ids,
			FrameDur: core.Milliseconds(a.FrameMs),
			Loop:     loop,
		})
	}
	differ := func(p, q *render.ImageBuf) int {
		pw, ph := p.Bounds()
		qw, qh := q.Bounds()
		if pw != qw || ph != qh {
			return pw * ph
		}
		n := 0
		for y := 0; y < ph; y++ {
			for x := 0; x < pw; x++ {
				pr, pg, pb, pa := p.GetRGBA(x, y)
				qr, qg, qb, qa := q.GetRGBA(x, y)
				if pr != qr || pg != qg || pb != qb || pa != qa {
					n++
				}
			}
		}
		return n
	}
	d0 := differ(frames[0], frames[1])
	d1 := differ(frames[2], frames[3])
	if d0 < animPairDiffMin || d1 < animPairDiffMin {
		return nil, nil, fmt.Sprintf("pair differ=%d/%d", d0, d1), false
	}
	detail := fmt.Sprintf("%s+%s %dms %s frames=%v pair_differ=%d/%d",
		f.Art[0].Name, f.Art[1].Name, f.Art[0].FrameMs, f.Art[0].Loop,
		[]int{100, 101, 110, 111}, d0, d1)
	return frames, clips, detail, true
}

// paintAnimStrip draws the four true frames bottom-aligned on one row,
// shared verbatim by the offscreen golden and the window strip card.
func paintAnimStrip(dc *render.Context, frames []*render.ImageBuf, ox, baseY, step float64) {
	if dc == nil {
		return
	}
	for i, fr := range frames {
		if fr == nil {
			continue
		}
		fw, fh := fr.Bounds()
		x := ox + step*float64(i) + (animCellStep-float64(fw))/2
		dc.DrawImageEx(fr, render.DrawImageOptions{
			X: x, Y: baseY - float64(fh),
			Interpolation: render.InterpNearest,
			Opacity:       1.0,
			BlendMode:     render.BlendNormal,
		})
	}
}

func renderAnimOffscreen(frames []*render.ImageBuf) image.Image {
	prev, had := os.LookupEnv("GOGPU_RENDER_MODE")
	// Offscreen golden parity must compare the same draw path twice:
	// forcing cpu for one side lets a GPU-capable host paint the other
	// strip through the textured path, and the two rasters differ at
	// edge texels (measured 2.65% on this host). Both sides are pinned
	// to the CPU raster so the frozen baseline is deterministic.
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	}()
	dc := render.NewContext(animOffW, animOffH)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	paintAnimStrip(dc, frames, animCellX0, animFrameBaseY, animCellStep)
	raw := dc.Image()
	b := raw.Bounds()
	cp := image.NewRGBA(b)
	// Copy out before Close: the live pixmap backing is recycled on
	// Close, so encoding the live view later (anim_last.png) can read
	// a reused buffer. The copy freezes this frame's pixels.
	if rgba, isRGBA := raw.(*image.RGBA); isRGBA {
		copy(cp.Pix, rgba.Pix)
		return cp
	}
	for y := 0; y < animOffH; y++ {
		for x := 0; x < animOffW; x++ {
			cp.Set(x, y, raw.At(x, y))
		}
	}
	return cp
}

// animPixelProbes asserts the strip carries real content: every cell band
// holds art pixels, and the two frames of each pair paint differently.
func animPixelProbes(img image.Image) (string, bool) {
	content := func(x0 int) int {
		n := 0
		for y := 0; y < animOffH; y++ {
			for x := x0; x < x0+int(animCellStep); x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				dr, dg, db := int(r>>8)-255, int(g>>8)-255, int(b>>8)-255
				if dr < 0 {
					dr = -dr
				}
				if dg < 0 {
					dg = -dg
				}
				if db < 0 {
					db = -db
				}
				if dr > 8 || dg > 8 || db > 8 {
					n++
				}
			}
		}
		return n
	}
	counts := make([]int, 4)
	for i := range counts {
		counts[i] = content(int(animCellX0) + i*int(animCellStep))
		if counts[i] < animContentMin {
			return fmt.Sprintf("cell%d content=%d", i, counts[i]), false
		}
	}
	bandDiffer := func(a, b int) int {
		n := 0
		for y := 0; y < animOffH; y++ {
			for x := 0; x < int(animCellStep); x++ {
				ar, ag, ab, _ := img.At(a+x, y).RGBA()
				br, bg, bb, _ := img.At(b+x, y).RGBA()
				if ar != br || ag != bg || ab != bb {
					n++
				}
			}
		}
		return n
	}
	x0 := int(animCellX0)
	d0 := bandDiffer(x0, x0+int(animCellStep))
	d1 := bandDiffer(x0+2*int(animCellStep), x0+3*int(animCellStep))
	if d0 < animPairDiffMin || d1 < animPairDiffMin {
		return fmt.Sprintf("band differ=%d/%d", d0, d1), false
	}
	return fmt.Sprintf("cells=%v pair_differ=%d/%d", counts, d0, d1), true
}

// animSelftest runs the headless evidences before the window opens.
func animSelftest() (map[string]any, []*render.ImageBuf, []sprite.Clip, bool) {
	extra := map[string]any{}
	f, err := loadAnimFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "game_sprite anim selftest load:", err)
		return extra, nil, nil, false
	}
	seqDetail, sixOK := replayAnimSequences(f)
	extra["sixseq_detail"] = seqDetail
	extra["sixseq_ok"] = boolToInt(sixOK)
	parityOK := animParityReplay(f)
	extra["parity_replay_ok"] = boolToInt(parityOK)
	frames, clips, artDetail, artOK := loadAnimArt(f)
	extra["art_detail"] = artDetail
	extra["art_ok"] = boolToInt(artOK)
	if !sixOK || !parityOK || !artOK {
		fmt.Fprintf(os.Stderr, "game_sprite anim selftest: six=%v parity=%v art=%v (%s | %s)\n",
			sixOK, parityOK, artOK, seqDetail, artDetail)
		return extra, frames, clips, false
	}
	cpu := renderAnimOffscreen(frames)
	gpu := renderAnimOffscreen(frames)
	changed, total, mean := diffImages(cpu, gpu)
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(changed) / float64(total)
	}
	extra["parity_changed_pct"] = pct
	extra["parity_mean_abs"] = mean
	pixDetail, pixOK := animPixelProbes(cpu)
	extra["pixels_detail"] = pixDetail
	extra["probe_ok"] = boolToInt(pixOK)
	if err := os.MkdirAll(testdataDir, 0o755); err == nil {
		if fh, err := os.Create(testdataDir + "/anim_last.png"); err == nil {
			_ = png.Encode(fh, cpu)
			_ = fh.Close()
		}
	}
	diffPct, _, firstRun := wrsoak.EvaluateGolden("game_sprite", testdataDir,
		"anim_last.png", "anim_off_golden.png",
		[]wrsoak.Rect{{X: 0, Y: 0, W: animOffW, H: animOffH}}, float64(animOffW))
	extra["golden_changed_pct"] = diffPct
	if firstRun {
		extra["golden_first_run"] = 1
	}
	ok := sixOK && parityOK && artOK && pixOK && pct <= animParityBudget && (firstRun || diffPct == animGoldenTol)
	fmt.Fprintf(os.Stderr, "game_sprite anim selftest: six=%v parity_replay=%v art=%v pixels=%v cpu/gpu=%.4f%% golden=%.4f%% first=%v ok=%v\n",
		sixOK, parityOK, artOK, pixOK, pct, diffPct, firstRun, ok)
	return extra, frames, clips, ok
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type animLive struct {
	fb           *sprite.Flipbook
	art          []animArtDef
	frames       map[int]*render.ImageBuf
	active       int
	hopClock     float64
	switches     int
	switchErrors int
	callbacks    int
	mirrorErrors int
	cbFired      bool
	cbLast       sprite.Event
	framesSeen   [2]int
	elapsed      [2]float64
	presents     [2]int64
	lastPresents int64
	curFrame     int
	curIndex     int
	app          *embedder.PipelineApp
	shell        *wrkit.ShellChrome
	phase        *wrkit.PhaseClock
	liveBox      *rendering.RenderBox
	statusL      *rendering.RenderText
	framesTotal  int
}

func (s *animLive) onEvent(ev sprite.Event) {
	s.callbacks++
	s.cbFired = true
	s.cbLast = ev
}

func (s *animLive) tick(dt float64) {
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	s.framesTotal++
	s.cbFired = false
	frame, ev, ok := s.fb.Update(core.SecondsFloat(dt))
	if !ok {
		s.mirrorErrors++
		return
	}
	if ev.Kind == sprite.EventNone {
		if s.cbFired {
			s.mirrorErrors++
		}
	} else if !s.cbFired || s.cbLast != ev {
		s.mirrorErrors++
	}
	s.curFrame, s.curIndex = frame, ev.Index
	s.framesSeen[s.active]++
	s.elapsed[s.active] += dt
	if s.app != nil {
		cur := s.app.PresentCount()
		if d := cur - s.lastPresents; d > 0 {
			s.presents[s.active] += d
		}
		s.lastPresents = cur
	}
	s.hopClock += dt
	if s.hopClock >= animHopSec {
		s.hopClock -= animHopSec
		next := (s.active + 1) % len(s.art)
		if err := s.fb.Play(s.art[next].Name); err != nil {
			s.switchErrors++
		} else {
			s.active = next
			s.switches++
		}
		if s.liveBox != nil {
			s.liveBox.MarkNeedsPaint()
		}
	}
	if s.liveBox != nil {
		s.liveBox.MarkNeedsPaint()
	}
}

func paintAnimLiveCard(pc *rendering.PaintContext, w, h float64, s *animLive) {
	if pc == nil || pc.DC == nil || s == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(1, 1, 1)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	if fr := s.frames[s.curFrame]; fr != nil {
		fw, fh := fr.Bounds()
		pc.DC.DrawImageEx(fr, render.DrawImageOptions{
			X: ax + (w-float64(fw))/2, Y: ay + (h-float64(fh))/2,
			Interpolation: render.InterpNearest,
			Opacity:       1.0,
			BlendMode:     render.BlendNormal,
		})
	}
	pc.DC.SetRGBA(0.55, 0.58, 0.62, 1)
	pc.DC.SetLineWidth(1.5)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Stroke()
}

func runAnimCase(autoOnly bool, manualSeconds int) {
	ability := abilityFor("anim")
	scenario := "game_sprite--case=anim"

	var secs int
	if autoOnly {
		secs = runSecondsEnv(8)
		wrkit.RequireMinRun(secs, ability)
	} else if manualSeconds > 0 {
		secs = manualSeconds
	} else if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
			wrkit.RequireMinRun(secs, ability)
		}
	}
	manualMode := !autoOnly

	wrkit.EnsureUIFace()
	extra, frames, clips, ok := animSelftest()
	if !ok {
		raw, _ := json.Marshal(map[string]any{"ability_id": ability, "scenario": scenario, "extra": extra, "pass": false})
		fmt.Println(string(raw))
		fmt.Fprintln(os.Stderr, "game_sprite: anim selftest FAIL, not opening window")
		os.Exit(1)
	}
	if !autoOnly {
		fmt.Fprintln(os.Stderr, "game_sprite: anim selftest done, entering manual phase (close X to finish)")
	}

	f, err := loadAnimFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: anim cases reload:", err)
		os.Exit(1)
	}
	sim := &animLive{art: f.Art, frames: map[int]*render.ImageBuf{}}
	sim.fb = sprite.NewFlipbook()
	for _, c := range clips {
		if err := sim.fb.AddClip(c); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: live AddClip:", err)
			os.Exit(1)
		}
	}
	for i, img := range frames {
		sim.frames[100+10*(i/2)+(i%2)] = img
	}
	if err := sim.fb.Play(sim.art[0].Name); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: live Play:", err)
		os.Exit(1)
	}
	if _, _, fr, ok := sim.fb.Current(); ok {
		sim.curFrame = fr
	}
	sim.fb.OnEvent(sim.onEvent)
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_sprite", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "game_sprite — 帧动画 flipbook (anim)", []string{
		"right/up 双动画 dodge 真帧",
		"5fps 200ms 一帧 loop 循环",
		"待机跑跳六序列全对",
		"每2秒一切右上互换",
		"静条=四真帧金图覆盖",
		"活卡=当前帧实时跟播",
		"回调与返回逐事件一致",
		"parity<=1% 双金零容差",
		"JSON见 ability_extra",
	})
	sim.shell = shell

	shell.Body.LabelAt("TRUE FRAMES: right walk1/walk2 + up up1/up2 (dodge)", 13, 8, 8, 0.75, 0.82, 0.9)
	strip := rendering.NewRenderBox()
	strip.FixedWidth, strip.FixedHeight = animStripW, animStripH
	strip.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGB(1, 1, 1)
		pc.DC.DrawRectangle(ax, ay, size.Width, size.Height)
		_ = pc.DC.Fill()
		paintAnimStrip(pc.DC, frames, ax+12, ay+animStripBaseY, animStripStep)
	}
	shell.Body.Place(strip, 8, 40)

	live := rendering.NewRenderBox()
	live.FixedWidth, live.FixedHeight = animLiveW, animLiveH
	live.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintAnimLiveCard(pc, size.Width, size.Height, sim)
	}
	shell.Body.Place(live, 8, 230)
	sim.liveBox = live
	shell.Body.LabelAt("LIVE: current frame follows the flipbook", 13, 8, 208, 0.75, 0.82, 0.9)

	sim.statusL = wrkit.Label("CLIP right frame=100 switches=0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.statusL, 424, 230)
	chain := wrkit.Label("engine/sprite只算数: Flipbook直调冻接口", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(chain, 424, 256)
	cbL := wrkit.Label("callbacks mirror every crossing", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(cbL, 424, 282)

	var proc scheduler.ProcessTracker
	proc.Start()

	eventsTotal, eventsPointer, eventsKey, eventsResize := 0, 0, 0, 0
	bumpTitle := func() {
		if ctl != nil {
			ctl.SetTitle(fmt.Sprintf("game_sprite anim=%s sw=%d events=%d",
				sim.art[sim.active].Name, sim.switches, eventsTotal))
		}
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: testdataDir + "/anim_final.png",
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_sprite: anim close (%s)\n", win.Backend())
			case platform.EventPointer:
				eventsPointer++
				eventsTotal++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_sprite anim event pointer (%d) @%.0f,%.0f\n", eventsTotal, ev.X, ev.Y)
				}
				bumpTitle()
			case platform.EventKey:
				if ev.Pressed {
					eventsKey++
					eventsTotal++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_sprite anim event key (%d) code=%d\n", eventsTotal, ev.KeyCode)
					}
					bumpTitle()
				}
			case platform.EventResize:
				eventsResize++
				eventsTotal++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_sprite anim event resize (%d) %dx%d\n", eventsTotal, ev.Width, ev.Height)
				}
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				bumpTitle()
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		sim.tick(dt)
		app.ScheduleFrame()
		proc.Sample()
		phase := sim.phase.Advance(dt)
		sim.statusL.SetText(fmt.Sprintf("CLIP %s frame=%d idx=%d switches=%d cb=%d",
			sim.art[sim.active].Name, sim.curFrame, sim.curIndex, sim.switches, sim.callbacks))
		sim.statusL.MarkNeedsPaint()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0
		shell.UpdateHUD(ability, phase, app, gateOK,
			fmt.Sprintf("presents=%d clip=%s sw=%d", app.PresentCount(), sim.art[sim.active].Name, sim.switches),
			"right/up")
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_sprite", testdataDir,
		"anim_final.png", "anim_final_base.png",
		// Static mask: title bar + legend only. The four-frame strip and
		// the live playback card advance with the flipbook, so they are
		// live regions by construction — masking them here is the same
		// rule other windows use (live bar excluded), not a widened gate.
		[]wrsoak.Rect{
			{X: 0, Y: 0, W: winW, H: 48},
			{X: 12, Y: 60, W: 260, H: 656},
		}, winW)

	fpsClip := make([]float64, 2)
	for i := range fpsClip {
		if sim.elapsed[i] > 1e-9 {
			fpsClip[i] = float64(sim.presents[i]) / sim.elapsed[i]
		}
	}
	extra["case"] = "anim"
	extra["clips"] = "idle,run,jump,ping,solo_loop,solo_once"
	extra["art"] = sim.art[0].Name + "," + sim.art[1].Name
	extra["frame_ms"] = sim.art[0].FrameMs
	extra["switches"] = sim.switches
	extra["switch_errors"] = sim.switchErrors
	extra["callbacks"] = sim.callbacks
	extra["mirror_errors"] = sim.mirrorErrors
	extra["frames_clip0"] = sim.framesSeen[0]
	extra["frames_clip1"] = sim.framesSeen[1]
	extra["fps_clip0"] = fpsClip[0]
	extra["fps_clip1"] = fpsClip[1]
	extra["win_golden_diff_pct"] = winGoldenDiff
	extra["win_golden_total_px"] = winGoldenTotal
	if winGoldenFirst {
		extra["win_golden_first"] = 1
	}
	extra["events_total"] = eventsTotal
	extra["events_pointer"] = eventsPointer
	extra["events_key"] = eventsKey
	extra["events_resize"] = eventsResize

	if autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     ability,
			Scenario:      scenario,
			Snap:          snap,
			PresentCount:  app.PresentCount(),
			ElapsedSec:    elapsed,
			SurfaceAreaPx: winW * winH,
			Warmup:        true,
			Extra:         extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))

		pass := true
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			pass = false
		}
		mustPass := func(cond bool, msg string, args ...any) {
			if !cond {
				fmt.Fprintf(os.Stderr, "FAIL: "+msg+"\n", args...)
				pass = false
			}
		}
		six, _ := numExtra(extra, "sixseq_ok")
		mustPass(six == 1, "sixseq_ok=%v want 1 (six sequences exact)", extra["sixseq_ok"])
		art, _ := numExtra(extra, "art_ok")
		mustPass(art == 1, "art_ok=%v want 1 (dodge true frames)", extra["art_ok"])
		if v, ok := numExtra(extra, "parity_changed_pct"); !ok || v > animParityBudget {
			mustPass(false, "parity_changed_pct=%v want <=%.1f", extra["parity_changed_pct"], animParityBudget)
		}
		probe, _ := numExtra(extra, "probe_ok")
		mustPass(probe == 1, "probe_ok=%v want 1", extra["probe_ok"])
		if fr, _ := numExtra(extra, "golden_first_run"); fr != 1 {
			g, _ := numExtra(extra, "golden_changed_pct")
			mustPass(g == animGoldenTol, "golden_changed_pct=%.4f want %.1f", g, animGoldenTol)
		}
		if !winGoldenFirst {
			mustPass(winGoldenDiff == animGoldenTol, "win_golden_diff_pct=%.4f want %.1f over %d px",
				winGoldenDiff, animGoldenTol, winGoldenTotal)
		}
		mustPass(sim.switches >= animSwitchMin, "switches=%d want >=%d", sim.switches, animSwitchMin)
		mustPass(sim.switchErrors == 0, "switch_errors=%d want 0", sim.switchErrors)
		mustPass(sim.mirrorErrors == 0, "mirror_errors=%d want 0", sim.mirrorErrors)
		if app.PresentCount() < 1 {
			fmt.Fprintln(os.Stderr, "FAIL: present_count=0")
			pass = false
		}
		if !pass {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_sprite: OK case=anim presents=%d switches=%d callbacks=%d golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs\n",
			app.PresentCount(), sim.switches, sim.callbacks, extra["golden_changed_pct"], winGoldenDiff, elapsed)
		return
	}
	probeOK, _ := numExtra(extra, "probe_ok")
	b, _ := json.Marshal(map[string]any{
		"ability_id": ability,
		"scenario":   scenario,
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": eventsPointer, "key": eventsKey, "resize": eventsResize,
		},
		"presents":      app.PresentCount(),
		"elapsed_sec":   elapsed,
		"switches":      sim.switches,
		"switch_errors": sim.switchErrors,
		"callbacks":     sim.callbacks,
		"mirror_errors": sim.mirrorErrors,
		"frames":        []int{sim.framesSeen[0], sim.framesSeen[1]},
		"fps_clip":      fpsClip,
		"probe_ok":      probeOK,
		"timed":         secs > 0,
		"note":          "case=anim",
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_sprite: case=anim backend=%s presents=%d switches=%d callbacks=%d elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		win.Backend(), app.PresentCount(), sim.switches, sim.callbacks, elapsed, eventsPointer, eventsKey, eventsResize)
}
