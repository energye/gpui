// Command game_light is the 6.1 independent real window: torch on a person,
// background stays night (layered light + global night).
//
// Window: 1200x800, title game_light. Body left day, middle night-only,
// right torch (night + warm point light with round cookie, world layer
// only). Light math calls game/light Scene Apply (frozen, direct use);
// the lit picture is drawn with the existing render DrawImageEx, render
// main path untouched (light covers after fx).
//
// Flags:
//
//	go run ./examples/game_light --case=torch -auto-only
//	  RUN_SECONDS=8 (default 8) auto gate, JSON on stdout, exit 1 on fail.
//	go run ./examples/game_light --case=torch -manual-seconds 30
//	  resident 30s, real events logged + SetTitle, summary JSON.
//	go run ./examples/game_light
//	  selftest then resident until close. RUN_SECONDS also times the run.
//
// Gates (hard): present>=1 via wrgate, parity_changed_pct<=1 (CPU vs GPU
// old DrawImage of the lit picture), logic probes all pass (person lit,
// background unlit, zero-light not black), pixel assertions pass
// (torch person gain, background identical to night), Golden static mask
// zero tolerance (second run on). Extra carries parity_changed_pct,
// torch_person_gain/torch_bg_same/probe_ok.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/examples/wrsoak"
	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/light"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	winTitle     = "game_light"
	srcW, srcH   = 240, 160
	cardW, cardH = 280, 300
	imgW, imgH   = 240, 160
	goldenW      = 96
	goldenH      = 64
)

// Hardcoded tolerances (review visible, never silent).
const (
	parityBudgetPct = 1.0  // C both sides: CPU vs GPU old DrawImage
	personGainMin   = 0.20 // torch person luminance minus night person
	bgSameEps       = 1e-9 // torch bg vs night bg, layer miss => identical
	nightDarkMax    = 0.45 // night person luminance below this (dimmed)
	nightBlackMin   = 0.03 // night darkest corner above this (never black)
	pixelDiffThresh = 2    // per-channel byte threshold for lit count
	litMinPx        = 100  // torch must lift at least this many pixels
	goldenTolPct    = 0.0  // static mask zero tolerance
	nightR          = 0.25 // global night color, frozen with the look
	nightG          = 0.25
	nightB          = 0.35
	torchIntensity  = 1.2  // warm bulb strength
	torchRange      = 80.0 // beam reach in world px (== screen px here)
	cookieN         = 9    // round slide samples per side
)

const testdataDir = "examples/game_light/testdata"

type manualSummary struct {
	Pointer  int
	Key      int
	Resize   int
	Activate int
	Timed    bool
	Note     string
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// personRect reports whether pixel (x,y) belongs to the person (world
// layer, torch lights it). Everything else is background (fx layer,
// torch must not touch it).
func personRect(x, y, w, h int) bool {
	px0, px1 := int(0.44*float64(w)), int(0.56*float64(w))
	py0, py1 := int(0.38*float64(h)), int(0.82*float64(h))
	return x >= px0 && x < px1 && y >= py0 && y < py1
}

// makeBaseColors builds the deterministic day picture, row-major: sky
// gradient up top, ground below, warm person block in the middle.
func makeBaseColors(w, h int) []core.Color {
	out := make([]core.Color, w*h)
	for y := 0; y < h; y++ {
		v := float64(y) / float64(max1(h-1))
		for x := 0; x < w; x++ {
			u := float64(x) / float64(max1(w-1))
			var r, g, b float64
			if v < 0.45 {
				t := v / 0.45
				r = 0.45 + (0.70-0.45)*t
				g = 0.62 + (0.78-0.62)*t
				b = 0.85 + (0.90-0.85)*t
				_ = u
			} else {
				d := (v - 0.45) / 0.55
				r = 0.30 + (0.18-0.30)*d
				g = 0.46 + (0.30-0.46)*d
				b = 0.34 + (0.24-0.34)*d
			}
			if personRect(x, y, w, h) {
				r, g, b = 0.92, 0.68, 0.46
			}
			out[y*w+x] = core.RGBA(clamp01(r), clamp01(g), clamp01(b), 1)
		}
	}
	return out
}

func makeLayers(w, h int) []light.Layer {
	out := make([]light.Layer, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if personRect(x, y, w, h) {
				out[y*w+x] = light.LayerWorld
			} else {
				out[y*w+x] = light.LayerFX
			}
		}
	}
	return out
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// mustCookie builds the frozen round flashlight slide: 1 at the center,
// linear falloff to 0 at the rim.
func mustCookie() *light.Cookie {
	mask := make([]float64, cookieN*cookieN)
	c := float64(cookieN-1) / 2
	for y := 0; y < cookieN; y++ {
		for x := 0; x < cookieN; x++ {
			d := math.Sqrt((float64(x)-c)*(float64(x)-c) + (float64(y)-c)*(float64(y)-c))
			mask[y*cookieN+x] = clamp01(1 - d/(c+0.5))
		}
	}
	ck, err := light.NewCookie(cookieN, cookieN, mask)
	if err != nil {
		panic(fmt.Sprintf("game_light: NewCookie: %v", err))
	}
	return &ck
}

// mustScene builds the frozen torch night: global night plus one warm
// point light with the round cookie, world layer only.
func mustScene(personCX, personCY float64) light.Scene {
	ck := mustCookie()
	l, err := light.NewPointLight(
		core.V2(personCX, personCY), torchRange,
		core.RGBA(1, 0.95, 0.8, 1), torchIntensity,
		light.LayersFor(light.LayerWorld), ck)
	if err != nil {
		panic(fmt.Sprintf("game_light: NewPointLight: %v", err))
	}
	sc, err := light.NewScene(core.RGBA(nightR, nightG, nightB, 1), []light.Light{l})
	if err != nil {
		panic(fmt.Sprintf("game_light: NewScene: %v", err))
	}
	return sc
}

func mustImage(w, h int, cols []core.Color, layers []light.Layer) *light.Image {
	img, err := light.NewImageFromColors(w, h, core.V2(0, 0), 1, cols, layers)
	if err != nil {
		panic(fmt.Sprintf("game_light: NewImageFromColors: %v", err))
	}
	return img
}

func luminance(c core.Color) float64 {
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

// runLogicProbes checks torch factor/layer/night rules on frozen points.
func runLogicProbes(sc light.Scene, w, h int) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	person := core.V2(0.5*float64(w), 0.6*float64(h))
	bg := core.V2(0.1*float64(w), 0.1*float64(h))
	var torch light.Light
	if len(sc.Lights) != 1 {
		out["torch_count_ok"] = false
		return out, false
	}
	torch = sc.Lights[0]
	out["torch_kind"] = torch.Kind.String()
	kindOK := torch.Kind == light.KindPoint
	out["torch_kind_ok"] = kindOK
	if !kindOK {
		ok = false
	}
	personF := torch.FactorAt(person, light.LayerWorld)
	bgF := torch.FactorAt(bg, light.LayerFX)
	out["person_factor"] = personF
	out["bg_factor_layer_miss"] = bgF
	personOK := personF > 0.5
	out["person_factor_ok"] = personOK
	if !personOK {
		ok = false
	}
	bgOK := bgF == 0
	out["bg_factor_ok"] = bgOK
	if !bgOK {
		ok = false
	}
	// GPU mirror agrees exactly.
	personG := torch.FactorGPU(person, light.LayerWorld)
	mirrorOK := personG == personF
	out["factor_mirror_ok"] = mirrorOK
	if !mirrorOK {
		ok = false
	}
	// Cookie slide: center brightest, rim darkest.
	ckOK := false
	ckCenter, ckRim := 0.0, 0.0
	if torch.Cookie != nil {
		ckCenter = torch.Cookie.Sample(core.V2(0.5, 0.5))
		ckRim = torch.Cookie.Sample(core.V2(0, 0))
		ckOK = ckCenter == 1 && ckRim == 0 && ckCenter > ckRim
	}
	out["cookie_center"] = ckCenter
	out["cookie_rim"] = ckRim
	out["cookie_ok"] = ckOK
	if !ckOK {
		ok = false
	}
	// Zero-light scene still shows night, never black.
	empty, err := light.NewScene(core.RGBA(nightR, nightG, nightB, 1), nil)
	emptyOK := false
	if err == nil {
		c := empty.Lit(core.RGB(1, 1, 1), person, light.LayerWorld)
		emptyOK = luminance(c) > nightBlackMin && luminance(c) < 1
		out["zero_light_lum"] = luminance(c)
	}
	out["zero_light_ok"] = emptyOK
	if !emptyOK {
		ok = false
	}
	out["probe_ok"] = ok
	return out, ok
}

// runPixelAssertions checks the lit picture: person lifted, background
// identical to night, night dimmed but not black, enough pixels moved.
func runPixelAssertions(night, torch *light.Image) (litPx int, litFrac, personGain, bgDiff, nightMin, nightPerson float64, ok bool) {
	totalPx := night.W * night.H
	litPx = 0
	for i := range night.Pix {
		if torch.Pix[i].R != night.Pix[i].R || torch.Pix[i].G != night.Pix[i].G || torch.Pix[i].B != night.Pix[i].B {
			litPx++
		}
	}
	litFrac = float64(litPx) / float64(totalPx)
	litOK := litPx >= litMinPx && litPx < totalPx
	// Person center gain vs background identity.
	personGain = 0
	bgDiff = 0
	nightMin = 1
	nightPerson = 1
	for y := 0; y < night.H; y++ {
		for x := 0; x < night.W; x++ {
			i := y*night.W + x
			nl := luminance(night.Pix[i])
			if nl < nightMin {
				nightMin = nl
			}
			if personRect(x, y, night.W, night.H) && x == night.W/2 && y == int(0.6*float64(night.H)) {
				personGain = luminance(torch.Pix[i]) - nl
				nightPerson = nl
			}
			if x == 2 && y == 2 {
				bgDiff = math.Abs(luminance(torch.Pix[i]) - nl)
			}
		}
	}
	personOK := personGain > personGainMin
	bgOK := bgDiff <= bgSameEps
	nightOK := nightMin > nightBlackMin && nightPerson < nightDarkMax
	ok = litOK && personOK && bgOK && nightOK
	return litPx, litFrac, personGain, bgDiff, nightMin, nightPerson, ok
}

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
				if d > pixelDiffThresh {
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

// lightParityOffscreen renders the lit picture via old DrawImage on CPU vs GPU.
func lightParityOffscreen(lit *render.ImageBuf) map[string]any {
	out := map[string]any{}
	const pw, ph = 96, 60
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	dcCPU := render.NewContext(pw, ph)
	dcCPU.ClearWithColor(render.White)
	dcCPU.DrawImageEx(lit, render.DrawImageOptions{
		X: 0, Y: 0, DstWidth: float64(pw), DstHeight: float64(ph),
		Interpolation: render.InterpBilinear, Opacity: 1, BlendMode: render.BlendNormal,
	})
	cpuImg := dcCPU.Image()
	cpuPix := image.NewRGBA(cpuImg.Bounds())
	if rgba, ok := cpuImg.(*image.RGBA); ok {
		copy(cpuPix.Pix, rgba.Pix)
	} else {
		for y := 0; y < ph; y++ {
			for x := 0; x < pw; x++ {
				r, g, b, a := cpuImg.At(x, y).RGBA()
				cpuPix.Set(x, y, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
			}
		}
	}
	_ = dcCPU.Close()
	_ = os.Unsetenv("GOGPU_RENDER_MODE")
	dcGPU := render.NewContext(pw, ph)
	dcGPU.ClearWithColor(render.White)
	dcGPU.DrawImageEx(lit, render.DrawImageOptions{
		X: 0, Y: 0, DstWidth: float64(pw), DstHeight: float64(ph),
		Interpolation: render.InterpBilinear, Opacity: 1, BlendMode: render.BlendNormal,
	})
	_ = dcGPU.FlushGPU()
	gpuOps := dcGPU.RenderPathStats().GPUOps
	gpuImg := dcGPU.Image()
	gpuPix := image.NewRGBA(gpuImg.Bounds())
	if rgba, ok := gpuImg.(*image.RGBA); ok {
		copy(gpuPix.Pix, rgba.Pix)
	} else {
		for y := 0; y < ph; y++ {
			for x := 0; x < pw; x++ {
				r, g, b, a := gpuImg.At(x, y).RGBA()
				gpuPix.Set(x, y, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
			}
		}
	}
	_ = dcGPU.Close()
	changed, total, mean := diffImages(cpuPix, gpuPix)
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(changed) / float64(total)
	}
	out["parity_changed_pct"] = pct
	out["parity_mean_abs"] = mean
	out["parity_gpu_ops"] = gpuOps
	return out
}

// checkOffscreenGolden compares the small torch image to the frozen PNG, zero tolerance.
func checkOffscreenGolden(torch *light.Image) (diffPct float64, totalPx int64, firstRun bool, ok bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, "light_torch_golden.png")
	cur := image.NewRGBA(image.Rect(0, 0, torch.W, torch.H))
	for y := 0; y < torch.H; y++ {
		for x := 0; x < torch.W; x++ {
			r, g, b, a := torch.Pix[y*torch.W+x].ToBytes()
			cur.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game_light: golden store %s: %v\n", basePath, err)
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "game_light: golden baseline stored: %s\n", basePath)
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_light: golden open: %v\n", err)
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_light: golden decode: %v\n", err)
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		fmt.Fprintf(os.Stderr, "game_light: golden size %v vs %v\n", want.Bounds(), cur.Bounds())
		return 100, int64(cur.Bounds().Dx() * cur.Bounds().Dy()), false, false
	}
	var diff int64
	total := int64(cur.Bounds().Dx() * cur.Bounds().Dy())
	for y := 0; y < cur.Bounds().Dy(); y++ {
		for x := 0; x < cur.Bounds().Dx(); x++ {
			ar, ag, ab, aa := cur.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				diff++
			}
		}
	}
	if total > 0 {
		diffPct = 100 * float64(diff) / float64(total)
	}
	ok = diff == 0
	return diffPct, total, false, ok
}

func bufFromColors(w, h int, cols []core.Color) *render.ImageBuf {
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		panic(fmt.Sprintf("game_light: NewImageBuf: %v", err))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := cols[y*w+x].ToBytes()
			_ = buf.SetRGBA(x, y, r, g, b, a)
		}
	}
	return buf
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func main() {
	caseFlag := flag.String("case", "torch", "light case: torch")
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	flag.Parse()
	caseName := *caseFlag
	if caseName != "torch" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want torch\n", caseName)
		os.Exit(1)
	}
	abilityID := "light-torch"
	scenario := "game_light--case=" + caseName

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		if !secsSet {
			secs = 8
			secsSet = true
		}
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
		secsSet = true
	}
	wrkit.EnsureUIFace()

	// Headless evidence before the window opens.
	baseCols := makeBaseColors(srcW, srcH)
	baseLayers := makeLayers(srcW, srcH)
	sc := mustScene(0.5*float64(srcW), 0.6*float64(srcH))
	baseImg := mustImage(srcW, srcH, baseCols, baseLayers)
	nightSc, err := light.NewScene(core.RGBA(nightR, nightG, nightB, 1), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: NewScene night: %v\n", err)
		os.Exit(1)
	}
	nightImg, err := nightSc.Apply(baseImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: night Apply: %v\n", err)
		os.Exit(1)
	}
	torchImg, err := sc.Apply(baseImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: torch Apply: %v\n", err)
		os.Exit(1)
	}
	// GPU mirror agrees exactly (C both sides, headless half).
	torchGPU, err := sc.ApplyGPU(baseImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: torch ApplyGPU: %v\n", err)
		os.Exit(1)
	}
	mirrorOK := torchImg.ApproxEqual(torchGPU, 0)
	probeMap, probeOK := runLogicProbes(sc, srcW, srcH)
	litPx, litFrac, personGain, bgDiff, nightMin, nightPerson, pixelOK := runPixelAssertions(nightImg, torchImg)
	probeOK = probeOK && mirrorOK
	probeMap["mirror_ok"] = mirrorOK
	srcBuf := bufFromColors(srcW, srcH, baseCols)
	nightBuf := bufFromColors(nightImg.W, nightImg.H, nightImg.Pix)
	torchBuf := bufFromColors(torchImg.W, torchImg.H, torchImg.Pix)
	parity := lightParityOffscreen(torchBuf)
	// Small golden for the frozen torch look, zero tolerance.
	smallCols := makeBaseColors(goldenW, goldenH)
	smallLayers := makeLayers(goldenW, goldenH)
	smallBase := mustImage(goldenW, goldenH, smallCols, smallLayers)
	smallSc := mustScene(0.5*float64(goldenW), 0.6*float64(goldenH))
	smallTorch, err := smallSc.Apply(smallBase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: small torch Apply: %v\n", err)
		os.Exit(1)
	}
	goldenDiff, goldenTotal, goldenFirst, goldenOK := checkOffscreenGolden(smallTorch)

	if !*autoOnly && !secsSet && *manualSeconds <= 0 {
		if !probeOK || !pixelOK || !goldenOK {
			fmt.Fprintf(os.Stderr, "game_light: selftest FAIL probe=%v pixel=%v golden=%.4f%%, not opening window\n", probeOK, pixelOK, goldenDiff)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_light: selftest done, entering manual phase (close X to finish)\n")
	} else if *autoOnly {
		if !probeOK || !pixelOK || !goldenOK {
			fmt.Fprintf(os.Stderr, "game_light: selftest FAIL probe=%v pixel=%v golden=%.4f%% first=%v\n", probeOK, pixelOK, goldenDiff, goldenFirst)
		}
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "game_light 手电只照人 — 左白天 中夜色 右手电", []string{
		"左DAY白天 中NIGHT夜色 右TORCH手电",
		"torch: 点光+圆灯片, 只照人层",
		"背景层不照=还是夜色",
		"零灯不黑屏=夜色保底",
		"JSON看parity<=1+探针全过",
		"--case=torch, 只要这一个",
	})

	dayBox := rendering.NewRenderBox()
	dayBox.FixedWidth, dayBox.FixedHeight = cardW, cardH
	dayBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLightCard(pc, "DAY 白天原片", srcBuf, -1, "人+背景都亮")
	}
	shell.Body.Place(dayBox, 20, 20)

	nightBox := rendering.NewRenderBox()
	nightBox.FixedWidth, nightBox.FixedHeight = cardW, cardH
	nightBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLightCard(pc, "NIGHT 夜色保底", nightBuf, -1, "零灯不黑屏")
	}
	shell.Body.Place(nightBox, 312, 20)

	torchBox := rendering.NewRenderBox()
	torchBox.FixedWidth, torchBox.FixedHeight = cardW, cardH
	torchBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLightCard(pc, "TORCH 手电只照人", torchBuf, torchIntensity/2.0, "人亮背景还是夜色")
	}
	shell.Body.Place(torchBox, 604, 20)

	note := wrkit.Label("左白天/中夜色/右手电, 强度条=灯强度", 12, 0.75, 0.82, 0.9)
	shell.Body.Place(note, 20, 340)
	chain := wrkit.Label("light盖在fx之后: 夜色全局+点光只照人层+灯片圆光", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(chain, 20, 362)

	var summary manualSummary
	summary.Note = "case=" + caseName
	elapsed := 0.0
	setTitle := func() {
		ctl := win.Controls()
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — ptr=%d key=%d rs=%d t=%.0fs", winTitle, summary.Pointer, summary.Key, summary.Resize, elapsed))
	}

	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "light_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_light: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				fmt.Fprintf(os.Stderr, "game_light: resize %dx%d\n", ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				setTitle()
			case platform.EventPointer:
				summary.Pointer++
				fmt.Fprintf(os.Stderr, "game_light: pointer kind=%v @(%.0f,%.0f)\n", ev.Pointer, ev.X, ev.Y)
				setTitle()
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "game_light: key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "game_light: event %s\n", ev.Type)
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		dayBox.MarkNeedsPaint()
		nightBox.MarkNeedsPaint()
		torchBox.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && probeOK && pixelOK
		shell.UpdateHUD(abilityID, "Steady", app, gateOK,
			fmt.Sprintf("presents=%d lit=%d", app.PresentCount(), litPx),
			fmt.Sprintf("parity=%.2f probe=%v", parity["parity_changed_pct"], probeOK))
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

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)

	// Window Golden over the static mask (HUD excluded by design).
	goldenRects := []wrsoak.Rect{
		{X: 0, Y: 0, W: winW, H: 48},
		{X: 12, Y: 60, W: 260, H: 656},
		{X: 284 + 20 + 20, Y: 60 + 20 + 40, W: imgW, H: imgH},
		{X: 284 + 312 + 20, Y: 60 + 20 + 40, W: imgW, H: imgH},
		{X: 284 + 604 + 20, Y: 60 + 20 + 40, W: imgW, H: imgH},
	}
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_light", testdataDir, "light_final.png", "light_final_base.png", goldenRects, winW)

	parityPct, _ := parity["parity_changed_pct"].(float64)
	extra := map[string]any{
		"parity_changed_pct":  parityPct,
		"parity_mean_abs":     parity["parity_mean_abs"],
		"parity_gpu_ops":      parity["parity_gpu_ops"],
		"torch_lit_px":        litPx,
		"torch_lit_frac":      litFrac,
		"torch_person_gain":   personGain,
		"torch_bg_same":       bgDiff,
		"torch_night_min":     nightMin,
		"torch_night_person":  nightPerson,
		"mirror_ok":           mirrorOK,
		"probe_ok":            probeOK && pixelOK && goldenOK,
		"torch_probe_ok":      probeMap["probe_ok"],
		"pixel_ok":            pixelOK,
		"golden_diff_pct":     goldenDiff,
		"golden_total_px":     goldenTotal,
		"golden_first_run":    goldenFirst,
		"win_golden_diff_pct": winGoldenDiff,
		"win_golden_total_px": winGoldenTotal,
		"win_golden_first":    winGoldenFirst,
		"case":                caseName,
		"pointer_events":      summary.Pointer,
		"key_events":          summary.Key,
		"resize_events":       summary.Resize,
		"manual_timed":        secsSet,
		"covered":             "左白天+中夜色+右手电, 强度条, 人亮背景夜色",
		"impl_correctness":    "Scene.Apply直调冻接口, 灯片圆光, 只照人层, 夜色全局",
		"impl_visible":        "HUD实时presents/lit/parity, 强度条=灯强, 关窗/超时出JSON",
	}
	for k, v := range probeMap {
		if _, dup := extra[k]; !dup {
			extra[k] = v
		}
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     abilityID,
		Scenario:      scenario,
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if *autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if parityPct > parityBudgetPct {
			fmt.Fprintf(os.Stderr, "FAIL: parity_changed_pct=%.4f want <=%.2f (CPU/GPU diverge)\n", parityPct, parityBudgetPct)
			os.Exit(1)
		}
		if !probeOK {
			fmt.Fprintf(os.Stderr, "FAIL: probe_ok=false case=%s (logic probes)\n", caseName)
			os.Exit(1)
		}
		if !pixelOK {
			fmt.Fprintf(os.Stderr, "FAIL: pixel assertions fail lit=%d gain=%.4f bg=%.6f night=%.4f\n", litPx, personGain, bgDiff, nightMin)
			os.Exit(1)
		}
		if !goldenOK && !goldenFirst {
			fmt.Fprintf(os.Stderr, "FAIL: golden_diff_pct=%.4f want %.1f over %d px\n", goldenDiff, goldenTolPct, goldenTotal)
			os.Exit(1)
		}
		if !winGoldenFirst && winGoldenDiff != goldenTolPct {
			fmt.Fprintf(os.Stderr, "FAIL: win_golden_diff_pct=%.4f want %.1f over %d px\n", winGoldenDiff, goldenTolPct, winGoldenTotal)
			os.Exit(1)
		}
		summary.Timed = secsSet
		fmt.Fprintf(os.Stderr, "game_light: OK case=%s presents=%d parity=%.4f lit=%d gain=%.4f bg=%.6f golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs\n",
			caseName, app.PresentCount(), parityPct, litPx, personGain, bgDiff, goldenDiff, winGoldenDiff, elapsedSec)
		return
	}
	summary.Timed = secsSet
	fmt.Fprintf(os.Stderr, "game_light: case=%s backend=%s presents=%d parity=%.4f lit=%d gain=%.4f bg=%.6f golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		caseName, win.Backend(), app.PresentCount(), parityPct, litPx, personGain, bgDiff, goldenDiff, winGoldenDiff, elapsedSec, summary.Pointer, summary.Key, summary.Resize)
}

func paintLightCard(pc *rendering.PaintContext, title string, img *render.ImageBuf, barFrac float64, foot string) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGBA(0.13, 0.14, 0.17, 1)
	pc.DC.DrawRectangle(ax, ay, cardW, cardH)
	_ = pc.DC.Fill()
	pc.DC.SetRGBA(0.35, 0.55, 0.75, 1)
	pc.DC.SetLineWidth(1)
	pc.DC.DrawRectangle(ax, ay, cardW, cardH)
	_ = pc.DC.Stroke()
	if face := wrkit.FaceAt(12); face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(0.88, 0.92, 0.98, 1)
	pc.DC.DrawString(title, ax+12, ay+20)
	if img != nil {
		pc.DC.DrawImageEx(img, render.DrawImageOptions{
			X: ax + 20, Y: ay + 40, DstWidth: float64(imgW), DstHeight: float64(imgH),
			Interpolation: render.InterpBilinear, Opacity: 1, BlendMode: render.BlendNormal,
		})
	}
	if barFrac >= 0 {
		bx, by, bw, bh := ax+20, ay+216, float64(imgW), 14.0
		pc.DC.SetRGBA(0.25, 0.27, 0.32, 1)
		pc.DC.DrawRectangle(bx, by, bw, bh)
		_ = pc.DC.Fill()
		fw := bw * clamp01(barFrac)
		pc.DC.SetRGBA(1.0, 0.80, 0.40, 1)
		pc.DC.DrawRectangle(bx, by, fw, bh)
		_ = pc.DC.Fill()
		if face := wrkit.FaceAt(11); face != nil {
			pc.DC.SetFont(face)
		}
		pc.DC.SetRGBA(0.75, 0.82, 0.90, 1)
		pc.DC.DrawString(fmt.Sprintf("强度 %.1f", barFrac*2.0), bx+6, by+11)
	}
	if face := wrkit.FaceAt(11); face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(0.70, 0.78, 0.88, 1)
	pc.DC.DrawString(foot, ax+12, ay+262)
}
