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
//	GOGPU_RENDER_MODE=cpu go run ./examples/game_sprite --case=all -auto-only
//	  short real window (RUN_SECONDS, default 8; JSON on stdout, exit 1 on fail).
//	go run ./examples/game_sprite --case=rot -manual-seconds 30
//	  resident 30s (or until close); true Pointer/Key/Resize events are
//	  logged and shown in the title as events=N; summary JSON at the end.
//	go run ./examples/game_sprite
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

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/examples/wrsoak"
	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/sprite"
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

const testdataDir = "examples/game_sprite/testdata"

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
	caseName := flag.String("case", "all", "which ability to gate: rot|depth|batch|all")
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	flag.Parse()
	if *caseName != "rot" && *caseName != "depth" && *caseName != "batch" && *caseName != "all" {
		fmt.Fprintln(os.Stderr, "FAIL: --case must be rot|depth|batch|all, got", *caseName)
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
