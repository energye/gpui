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
//	go run ./examples/game_light --case=shadow -auto-only
//	  same gate for the 6.3 wall shadow (torch + occluder strip).
//	go run ./examples/game_light --case=normal -auto-only
//	  same gate for the 6.2 normal-mapped dome (flat/side/dir cards).
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
	caseFlag := flag.String("case", "torch", "light case: torch|shadow|normal")
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	flag.Parse()
	caseName := *caseFlag
	if caseName != "torch" && caseName != "shadow" && caseName != "normal" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want torch|shadow|normal\n", caseName)
		os.Exit(1)
	}
	if caseName == "shadow" {
		runShadowCase(*autoOnly, *manualSeconds)
		return
	}
	if caseName == "normal" {
		runNormalCase(*autoOnly, *manualSeconds)
		return
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

// ---- 6.3 shadow case (S48/W7, additive: torch path above untouched) ----

// shadowWallFor builds the frozen wall strip for a w-by-h picture: a
// vertical occluder over the person's right half, spanning past both
// edges (the torch only touches the person layer, so the wall must stand
// on the person for the lee to show).
func shadowWallFor(w, h int) light.Occluder {
	x0, x1 := 0.52*float64(w), 0.56*float64(w)
	y0, y1 := -0.1*float64(h), 1.1*float64(h)
	o, err := light.NewOccluder([]core.Vec2{
		core.V2(x0, y0), core.V2(x1, y0), core.V2(x1, y1), core.V2(x0, y1),
	})
	if err != nil {
		panic(fmt.Sprintf("game_light shadow: NewOccluder: %v", err))
	}
	return o
}

// mustShadowFor builds the frozen full-dark wall shadow (occlusion 1).
func mustShadowFor(w, h int) light.Shadow {
	s, err := light.NewShadow([]light.Occluder{shadowWallFor(w, h)}, 1, 1000)
	if err != nil {
		panic(fmt.Sprintf("game_light shadow: NewShadow: %v", err))
	}
	return s
}

// runShadowProbes checks wall factor/blocked/mirror rules on frozen points.
func runShadowProbes(sh light.Shadow, torch light.Light, w, h int) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	person := core.V2(0.5*float64(w), 0.6*float64(h))
	front := core.V2(0.47*float64(w), 0.6*float64(h))
	behind := core.V2(0.545*float64(w), 0.6*float64(h))
	personF := sh.FactorFor(torch, person)
	frontF := sh.FactorFor(torch, front)
	behindF := sh.FactorFor(torch, behind)
	out["person_factor"] = personF
	out["front_factor"] = frontF
	out["behind_factor"] = behindF
	out["person_blocked"] = sh.Blocked(torch, person)
	out["front_blocked"] = sh.Blocked(torch, front)
	out["behind_blocked"] = sh.Blocked(torch, behind)
	// Person and front stand before the wall: lit, unblocked.
	openOK := personF == 1 && frontF == 1 &&
		!sh.Blocked(torch, person) && !sh.Blocked(torch, front)
	out["open_ok"] = openOK
	if !openOK {
		ok = false
	}
	// Behind the wall: fully dark (occlusion 1), blocked.
	leeOK := behindF == 0 && sh.Blocked(torch, behind)
	out["lee_ok"] = leeOK
	if !leeOK {
		ok = false
	}
	// GPU mirrors agree exactly.
	mirrorOK := sh.FactorForGPU(torch, person) == personF &&
		sh.FactorForGPU(torch, behind) == behindF &&
		sh.BlockedGPU(torch, behind) == sh.Blocked(torch, behind)
	out["mirror_ok"] = mirrorOK
	if !mirrorOK {
		ok = false
	}
	// Empty shadow keeps the plain torch (no wall, no dark).
	empty, err := light.NewShadow(nil, 1, 1000)
	emptyOK := false
	if err == nil {
		emptyOK = empty.FactorFor(torch, behind) == 1 && !empty.Blocked(torch, behind)
	}
	out["empty_ok"] = emptyOK
	if !emptyOK {
		ok = false
	}
	out["probe_ok"] = ok
	return out, ok
}

// runShadowPixelAssertions checks the shadow picture: person identical to
// torch, lee identical to night, enough pixels moved, night not black.
func runShadowPixelAssertions(night, torch, shadow *light.Image) (movedPx int, movedFrac, personGain, leeDiff, nightMin float64, ok bool) {
	totalPx := night.W * night.H
	movedPx = 0
	for i := range night.Pix {
		if shadow.Pix[i].R != torch.Pix[i].R || shadow.Pix[i].G != torch.Pix[i].G || shadow.Pix[i].B != torch.Pix[i].B {
			movedPx++
		}
	}
	movedFrac = float64(movedPx) / float64(totalPx)
	movedOK := movedPx >= litMinPx && movedPx < totalPx
	// Person center: same lamp, before the wall, identical to torch.
	px, py := night.W/2, int(0.6*float64(night.H))
	pi := py*night.W + px
	personSame := shadow.Pix[pi] == torch.Pix[pi]
	personGain = luminance(torch.Pix[pi]) - luminance(night.Pix[pi])
	personOK := personSame && personGain > personGainMin
	// Behind the wall (person right half): full occlusion, identical to night.
	bx, by := int(0.545*float64(night.W)), int(0.6*float64(night.H))
	bi := by*night.W + bx
	leeDiff = math.Abs(luminance(shadow.Pix[bi]) - luminance(night.Pix[bi]))
	leeOK := shadow.Pix[bi] == night.Pix[bi] && leeDiff == 0
	nightMin = 1
	for _, p := range night.Pix {
		if l := luminance(p); l < nightMin {
			nightMin = l
		}
	}
	nightOK := nightMin > nightBlackMin
	ok = movedOK && personOK && leeOK && nightOK
	return movedPx, movedFrac, personGain, leeDiff, nightMin, ok
}

// checkShadowGolden compares the small shadow image to its frozen PNG.
func checkShadowGolden(shadow *light.Image) (diffPct float64, totalPx int64, firstRun bool, ok bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, "shadow_wall_golden.png")
	cur := image.NewRGBA(image.Rect(0, 0, shadow.W, shadow.H))
	for y := 0; y < shadow.H; y++ {
		for x := 0; x < shadow.W; x++ {
			r, g, b, a := shadow.Pix[y*shadow.W+x].ToBytes()
			cur.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game_light shadow: golden store %s: %v\n", basePath, err)
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "game_light shadow: golden baseline stored: %s\n", basePath)
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_light shadow: golden open: %v\n", err)
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_light shadow: golden decode: %v\n", err)
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		fmt.Fprintf(os.Stderr, "game_light shadow: golden size %v vs %v\n", want.Bounds(), cur.Bounds())
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

func runShadowCase(autoOnly bool, manualSeconds int) {
	abilityID := "light-shadow"
	scenario := "game_light--case=shadow"
	secs, secsSet := wrkit.RunSecondsOpt()
	if autoOnly {
		if !secsSet {
			secs = 8
			secsSet = true
		}
	} else if manualSeconds > 0 {
		secs = manualSeconds
		secsSet = true
	}
	wrkit.EnsureUIFace()

	// Headless evidence before the window opens.
	baseCols := makeBaseColors(srcW, srcH)
	baseLayers := makeLayers(srcW, srcH)
	sc := mustScene(0.5*float64(srcW), 0.6*float64(srcH))
	sh := mustShadowFor(srcW, srcH)
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
	shadowImg, err := sh.Apply(baseImg, sc.Lights, sc.Night)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: shadow Apply: %v\n", err)
		os.Exit(1)
	}
	// GPU mirror agrees exactly (C both sides, headless half).
	shadowGPU, err := sh.ApplyGPU(baseImg, sc.Lights, sc.Night)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: shadow ApplyGPU: %v\n", err)
		os.Exit(1)
	}
	mirrorOK := shadowImg.ApproxEqual(shadowGPU, 0)
	probeMap, probeOK := runShadowProbes(sh, sc.Lights[0], srcW, srcH)
	movedPx, movedFrac, personGain, leeDiff, nightMin, pixelOK := runShadowPixelAssertions(nightImg, torchImg, shadowImg)
	probeOK = probeOK && mirrorOK
	probeMap["mirror_ok"] = mirrorOK
	dayBuf := bufFromColors(srcW, srcH, baseCols)
	torchBuf := bufFromColors(torchImg.W, torchImg.H, torchImg.Pix)
	shadowBuf := bufFromColors(shadowImg.W, shadowImg.H, shadowImg.Pix)
	parity := lightParityOffscreen(shadowBuf)
	// Small golden for the frozen shadow look, zero tolerance.
	smallCols := makeBaseColors(goldenW, goldenH)
	smallLayers := makeLayers(goldenW, goldenH)
	smallBase := mustImage(goldenW, goldenH, smallCols, smallLayers)
	smallSc := mustScene(0.5*float64(goldenW), 0.6*float64(goldenH))
	smallSh := mustShadowFor(goldenW, goldenH)
	smallShadow, err := smallSh.Apply(smallBase, smallSc.Lights, smallSc.Night)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: small shadow Apply: %v\n", err)
		os.Exit(1)
	}
	goldenDiff, goldenTotal, goldenFirst, goldenOK := checkShadowGolden(smallShadow)

	if !autoOnly && !secsSet && manualSeconds <= 0 {
		if !probeOK || !pixelOK || !goldenOK {
			fmt.Fprintf(os.Stderr, "game_light shadow: selftest FAIL probe=%v pixel=%v golden=%.4f%%, not opening window\n", probeOK, pixelOK, goldenDiff)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_light shadow: selftest done, entering manual phase (close X to finish)\n")
	} else if autoOnly {
		if !probeOK || !pixelOK || !goldenOK {
			fmt.Fprintf(os.Stderr, "game_light shadow: selftest FAIL probe=%v pixel=%v golden=%.4f%% first=%v\n", probeOK, pixelOK, goldenDiff, goldenFirst)
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

	shell := wrkit.NewShell(winW, winH, "game_light 墙影 — 左白天 中手电 右墙影", []string{
		"左DAY白天 中TORCH手电 右SHADOW墙影",
		"torch+竖墙压人右半, 右半=影子=夜色",
		"人左半还在墙前=和中卡一样亮",
		"零挡=中卡, 不崩",
		"JSON看parity<=1+探针全过",
		"--case=shadow, 只要这一个",
	})

	dayBox := rendering.NewRenderBox()
	dayBox.FixedWidth, dayBox.FixedHeight = cardW, cardH
	dayBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLightCard(pc, "DAY 白天原片", dayBuf, -1, "人+背景都亮")
	}
	shell.Body.Place(dayBox, 20, 20)

	torchBox := rendering.NewRenderBox()
	torchBox.FixedWidth, torchBox.FixedHeight = cardW, cardH
	torchBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLightCard(pc, "TORCH 无墙", torchBuf, torchIntensity/2.0, "零挡=手电全开")
	}
	shell.Body.Place(torchBox, 312, 20)

	shadowBox := rendering.NewRenderBox()
	shadowBox.FixedWidth, shadowBox.FixedHeight = cardW, cardH
	shadowBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLightCard(pc, "SHADOW 有墙影", shadowBuf, torchIntensity/2.0, "人右半变暗=夜色, 左半一样亮")
	}
	shell.Body.Place(shadowBox, 604, 20)

	note := wrkit.Label("左白天/中手电/右墙影, 人右半影子方向看右边", 12, 0.75, 0.82, 0.9)
	shell.Body.Place(note, 20, 340)
	chain := wrkit.Label("shadow盖在light之后: 夜色全局+点光只照人层+墙挡变暗", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(chain, 20, 362)

	var summary manualSummary
	summary.Note = "case=shadow"
	elapsed := 0.0
	setTitle := func() {
		ctl := win.Controls()
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — ptr=%d key=%d rs=%d t=%.0fs", winTitle, summary.Pointer, summary.Key, summary.Resize, elapsed))
	}

	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "shadow_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_light shadow: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				fmt.Fprintf(os.Stderr, "game_light shadow: resize %dx%d\n", ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				setTitle()
			case platform.EventPointer:
				summary.Pointer++
				fmt.Fprintf(os.Stderr, "game_light shadow: pointer kind=%v @(%.0f,%.0f)\n", ev.Pointer, ev.X, ev.Y)
				setTitle()
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "game_light shadow: key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "game_light shadow: event %s\n", ev.Type)
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		dayBox.MarkNeedsPaint()
		torchBox.MarkNeedsPaint()
		shadowBox.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && probeOK && pixelOK
		shell.UpdateHUD(abilityID, "Steady", app, gateOK,
			fmt.Sprintf("presents=%d moved=%d", app.PresentCount(), movedPx),
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
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_light", testdataDir, "shadow_final.png", "shadow_final_base.png", goldenRects, winW)

	parityPct, _ := parity["parity_changed_pct"].(float64)
	extra := map[string]any{
		"parity_changed_pct":  parityPct,
		"parity_mean_abs":     parity["parity_mean_abs"],
		"parity_gpu_ops":      parity["parity_gpu_ops"],
		"shadow_moved_px":     movedPx,
		"shadow_moved_frac":   movedFrac,
		"shadow_person_gain":  personGain,
		"shadow_lee_diff":     leeDiff,
		"shadow_night_min":    nightMin,
		"mirror_ok":           mirrorOK,
		"probe_ok":            probeOK && pixelOK && goldenOK,
		"pixel_ok":            pixelOK,
		"golden_diff_pct":     goldenDiff,
		"golden_total_px":     goldenTotal,
		"golden_first_run":    goldenFirst,
		"win_golden_diff_pct": winGoldenDiff,
		"win_golden_total_px": winGoldenTotal,
		"win_golden_first":    winGoldenFirst,
		"case":                "shadow",
		"pointer_events":      summary.Pointer,
		"key_events":          summary.Key,
		"resize_events":       summary.Resize,
		"manual_timed":        secsSet,
		"covered":             "左白天+中手电+右墙影, 人左半一样亮右半变暗",
		"impl_correctness":    "Shadow.Apply盖在Scene之后, 竖墙压人右半, 右半回夜色",
		"impl_visible":        "HUD实时presents/moved/parity, 关窗/超时出JSON",
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

	if autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if parityPct > parityBudgetPct {
			fmt.Fprintf(os.Stderr, "FAIL: parity_changed_pct=%.4f want <=%.2f (CPU/GPU diverge)\n", parityPct, parityBudgetPct)
			os.Exit(1)
		}
		if !probeOK {
			fmt.Fprintf(os.Stderr, "FAIL: probe_ok=false case=shadow (logic probes)\n")
			os.Exit(1)
		}
		if !pixelOK {
			fmt.Fprintf(os.Stderr, "FAIL: pixel assertions fail moved=%d gain=%.4f lee=%.6f night=%.4f\n", movedPx, personGain, leeDiff, nightMin)
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
		fmt.Fprintf(os.Stderr, "game_light: OK case=shadow presents=%d parity=%.4f moved=%d gain=%.4f lee=%.6f golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs\n",
			app.PresentCount(), parityPct, movedPx, personGain, leeDiff, goldenDiff, winGoldenDiff, elapsedSec)
		return
	}
	summary.Timed = secsSet
	fmt.Fprintf(os.Stderr, "game_light: case=shadow backend=%s presents=%d parity=%.4f moved=%d gain=%.4f lee=%.6f golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		win.Backend(), app.PresentCount(), parityPct, movedPx, personGain, leeDiff, goldenDiff, winGoldenDiff, elapsedSec, summary.Pointer, summary.Key, summary.Resize)
}

// ---- 6.2 normal-mapped dome (S47): flat reference, side-lit dome,
// slanted directional dome. All world layer; the立体 comes from normals. ----

const (
	normalGray      = 0.75 // uniform base, shading is all normals
	normalIntensity = 1.4  // side lamp strength
	normalRange     = 400.0
	normalHeight    = 60.0 // lamp height above the plane, world px
	normalDirHeight = 1.0  // directional tilt height (Dir ~1)
	normalDomeSlope = 1.2  // rim tilt gain
)

// makeGrayColors builds the uniform mid-gray base, row-major.
func makeGrayColors(w, h int) []core.Color {
	out := make([]core.Color, w*h)
	for i := range out {
		out[i] = core.RGBA(normalGray, normalGray, normalGray, 1)
	}
	return out
}

func makeWorldLayers(w, h int) []light.Layer {
	out := make([]light.Layer, w*h)
	for i := range out {
		out[i] = light.LayerWorld
	}
	return out
}

// domeCenter reports the bump center and radius for a w-by-h sheet.
func domeCenter(w, h int) (cx, cy, r float64) {
	return 0.5 * float64(w), 0.6 * float64(h), 0.28 * float64(w)
}

// makeDomeNormals builds the radial bump on a flat plane: inside the rim
// the slopes face away from the hub, outside the rim stays flat so the
// lit dome stands out from its lit surround. The outer 2 px ring eases
// the slope to flat (smoothstep) so the lit rim does not jump one pixel
// to the next. Row-major, 1:1 with the lit image. Engine math untouched,
// only this window sheet changes.
func makeDomeNormals(w, h int) []light.Normal {
	cx, cy, r := domeCenter(w, h)
	band := 2 / r
	if band <= 0 || band >= 1 {
		band = 0.05
	}
	edge0 := 1 - band
	out := make([]light.Normal, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := (float64(x) - cx) / r
			dy := (float64(y) - cy) / r
			d2 := dx*dx + dy*dy
			if d2 > 1 {
				out[y*w+x] = light.FlatNormal()
				continue
			}
			fade := 1.0
			if d2 > edge0*edge0 {
				d := math.Sqrt(d2)
				t := (d - edge0) / band
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
				fade = 1 - t*t*(3-2*t)
			}
			out[y*w+x] = light.N(dx*normalDomeSlope*fade, dy*normalDomeSlope*fade, 1)
		}
	}
	return out
}

func mustNormalMap(w, h int, ns []light.Normal) *light.NormalMap {
	m, err := light.NewNormalMap(w, h, ns)
	if err != nil {
		panic(fmt.Sprintf("game_light normal: NewNormalMap: %v", err))
	}
	return &m
}

func mustFlatMap(w, h int) *light.NormalMap {
	ns := make([]light.Normal, w*h)
	for i := range ns {
		ns[i] = light.FlatNormal()
	}
	return mustNormalMap(w, h, ns)
}

// mustImageStep builds the window base with an explicit world step. The
// normal case renders at 4x and shrinks back to 1x, so the dome rim gets
// real coverage anti-aliasing. Engine math untouched.
func mustImageStep(w, h int, step float64, cols []core.Color, layers []light.Layer) *light.Image {
	img, err := light.NewImageFromColors(w, h, core.V2(0, 0), step, cols, layers)
	if err != nil {
		panic(fmt.Sprintf("game_light: NewImageFromColors: %v", err))
	}
	return img
}

// downsampleSS shrinks by normalSS (power of two) with repeated box filter.
// Step doubles so the world rect stays put; layers take the block corner
// (the normal window is single-layer anyway).
func downsampleSS(src *light.Image, ss int) *light.Image {
	out := src
	for ss > 1 {
		out = downsampleHalf(out)
		ss /= 2
	}
	return out
}
func downsampleHalf(src *light.Image) *light.Image {
	if src == nil || src.W%2 != 0 || src.H%2 != 0 {
		panic(fmt.Sprintf("game_light: downsampleHalf odd size %dx%d", src.W, src.H))
	}
	w, h := src.W/2, src.H/2
	cols := make([]core.Color, w*h)
	layers := make([]light.Layer, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i0 := (2*y)*src.W + 2*x
			i1 := i0 + 1
			i2 := i0 + src.W
			i3 := i2 + 1
			a, b, c, d := src.Pix[i0], src.Pix[i1], src.Pix[i2], src.Pix[i3]
			cols[y*w+x] = core.RGBA(
				(a.R+b.R+c.R+d.R)/4,
				(a.G+b.G+c.G+d.G)/4,
				(a.B+b.B+c.B+d.B)/4,
				(a.A+b.A+c.A+d.A)/4,
			)
			layers[y*w+x] = src.Layers[i0]
		}
	}
	return mustImageStep(w, h, src.Step*2, cols, layers)
}

// mustNormalPointScene builds the frozen side lamp night for the dome.
func mustNormalPointScene(px, py float64) light.NormalScene {
	l, err := light.NewPointLight(
		core.V2(px, py), normalRange,
		core.RGBA(1, 0.95, 0.8, 1), normalIntensity,
		light.LayersFor(light.LayerWorld), nil)
	if err != nil {
		panic(fmt.Sprintf("game_light normal: NewPointLight: %v", err))
	}
	sc, err := light.NewNormalScene(core.RGBA(nightR, nightG, nightB, 1), []light.Light{l}, normalHeight)
	if err != nil {
		panic(fmt.Sprintf("game_light normal: NewNormalScene: %v", err))
	}
	return sc
}

// mustNormalDirScene builds the frozen slanted-light night for the dome.
func mustNormalDirScene() light.NormalScene {
	l, err := light.NewDirectionalLight(
		core.V2(-0.8, -0.25),
		core.RGBA(1, 0.95, 0.8, 1), 1.0,
		light.LayersFor(light.LayerWorld))
	if err != nil {
		panic(fmt.Sprintf("game_light normal: NewDirectionalLight: %v", err))
	}
	sc, err := light.NewNormalScene(core.RGBA(nightR, nightG, nightB, 1), []light.Light{l}, normalDirHeight)
	if err != nil {
		panic(fmt.Sprintf("game_light normal: NewNormalScene: %v", err))
	}
	return sc
}

// runNormalProbes checks lamp kinds, facing order, falloff, dome rows,
// the GPU mirror, and the zero-light floor.
func runNormalProbes(flat, side, dir light.NormalScene, dome *light.Image, w, h int) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	if len(flat.Lights) != 1 || flat.Lights[0].Kind != light.KindPoint {
		out["flat_kind_ok"] = false
		return out, false
	}
	out["flat_kind_ok"] = true
	if len(side.Lights) != 1 || side.Lights[0].Kind != light.KindPoint {
		out["side_kind_ok"] = false
		return out, false
	}
	out["side_kind_ok"] = true
	if len(dir.Lights) != 1 || dir.Lights[0].Kind != light.KindDirectional {
		out["dir_kind_ok"] = false
		return out, false
	}
	out["dir_kind_ok"] = true
	lamp := side.Lights[0]
	// Facing order on a frozen spot next to the lamp.
	p := core.V2(lamp.Pos.X+30, lamp.Pos.Y)
	lf, dirOK := light.LightDir(lamp, p, side.Height)
	if !dirOK {
		out["lightdir_ok"] = false
		return out, false
	}
	out["lightdir_ok"] = true
	toward := light.NormalFactor(lamp, p, light.LayerWorld, lf, side.Height)
	flatF := light.NormalFactor(lamp, p, light.LayerWorld, light.FlatNormal(), side.Height)
	awayN := light.N(-lf.X, -lf.Y, -lf.Z).Normalize()
	away := light.NormalFactor(lamp, p, light.LayerWorld, awayN, side.Height)
	out["toward_factor"] = toward
	out["flat_factor"] = flatF
	out["away_factor"] = away
	orderOK := toward > flatF && flatF > away && away == 0
	out["facing_order_ok"] = orderOK
	if !orderOK {
		ok = false
	}
	// Flat card falls off: hub beats the corner (flat lamp overhead).
	flatLamp := flat.Lights[0]
	hub := core.V2(0.5*float64(w), 0.6*float64(h))
	corner := core.V2(0, 0)
	hubF := light.NormalFactor(flatLamp, hub, light.LayerWorld, light.FlatNormal(), flat.Height)
	cornerF := light.NormalFactor(flatLamp, corner, light.LayerWorld, light.FlatNormal(), flat.Height)
	out["hub_factor"] = hubF
	out["corner_factor"] = cornerF
	fallOK := hubF > cornerF && cornerF >= 0
	out["falloff_ok"] = fallOK
	if !fallOK {
		ok = false
	}
	// Dome row under the side lamp: left slope > hub > right slope.
	cx, cy, r := domeCenter(w, h)
	rowY := int(cy)
	lumAt := func(img *light.Image, x int) float64 {
		c, _, ok := img.At(x, rowY)
		if !ok {
			return -1
		}
		return luminance(c)
	}
	l1, l2, l3 := lumAt(dome, int(cx-0.8*r)), lumAt(dome, int(cx)), lumAt(dome, int(cx+0.8*r))
	out["dome_left"] = l1
	out["dome_mid"] = l2
	out["dome_right"] = l3
	domeOK := l1 > l2 && l2 > l3
	out["dome_row_ok"] = domeOK
	if !domeOK {
		ok = false
	}
	// Zero-light floor still shows night, never black.
	empty, err := light.NewNormalScene(core.RGBA(nightR, nightG, nightB, 1), nil, normalHeight)
	emptyOK := false
	if err == nil {
		c := empty.Lit(core.RGB(1, 1, 1), hub, light.LayerWorld, light.FlatNormal())
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

// runNormalPixelAssertions checks the dome picture: enough pixels moved,
// left slope lifted, row order kept, night dimmed but not black.
func runNormalPixelAssertions(night, dome *light.Image, w, h int) (litPx int, litFrac, slopeGain, nightMin float64, ok bool) {
	totalPx := night.W * night.H
	litPx = 0
	for i := range night.Pix {
		if dome.Pix[i].R != night.Pix[i].R || dome.Pix[i].G != night.Pix[i].G || dome.Pix[i].B != night.Pix[i].B {
			litPx++
		}
	}
	litFrac = float64(litPx) / float64(totalPx)
	litOK := litPx >= litMinPx && litPx < totalPx
	cx, cy, r := domeCenter(w, h)
	slopeIdx := int(cy)*w + int(cx-0.8*r)
	midIdx := int(cy)*w + int(cx)
	rightIdx := int(cy)*w + int(cx+0.8*r)
	slopeGain = luminance(dome.Pix[slopeIdx]) - luminance(night.Pix[slopeIdx])
	gainOK := slopeGain > personGainMin
	orderOK := luminance(dome.Pix[slopeIdx]) > luminance(dome.Pix[midIdx]) &&
		luminance(dome.Pix[midIdx]) > luminance(dome.Pix[rightIdx])
	nightMin = 1
	for _, p := range night.Pix {
		if l := luminance(p); l < nightMin {
			nightMin = l
		}
	}
	nightOK := nightMin > nightBlackMin && luminance(night.Pix[midIdx]) < nightDarkMax
	ok = litOK && gainOK && orderOK && nightOK
	return litPx, litFrac, slopeGain, nightMin, ok
}

// checkNormalGolden compares the small dome image to the frozen PNG.
func checkNormalGolden(dome *light.Image) (diffPct float64, totalPx int64, firstRun bool, ok bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, "normal_dome_golden.png")
	cur := image.NewRGBA(image.Rect(0, 0, dome.W, dome.H))
	for y := 0; y < dome.H; y++ {
		for x := 0; x < dome.W; x++ {
			r, g, b, a := dome.Pix[y*dome.W+x].ToBytes()
			cur.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game_light normal: golden store %s: %v\n", basePath, err)
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "game_light normal: golden baseline stored: %s\n", basePath)
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_light normal: golden open: %v\n", err)
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_light normal: golden decode: %v\n", err)
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		fmt.Fprintf(os.Stderr, "game_light normal: golden size %v vs %v\n", want.Bounds(), cur.Bounds())
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

func runNormalCase(autoOnly bool, manualSeconds int) {
	abilityID := "light-normal"
	scenario := "game_light--case=normal"
	secs, secsSet := wrkit.RunSecondsOpt()
	if autoOnly {
		if !secsSet {
			secs = 8
			secsSet = true
		}
	} else if manualSeconds > 0 {
		secs = manualSeconds
		secsSet = true
	}
	wrkit.EnsureUIFace()

	// Headless evidence before the window opens. The normal cards render
	// at 4x (step 0.25) and shrink to 1x, so the dome rim gets coverage
	// anti-aliasing; the displayed size and world rect stay unchanged.
	const normalSS = 4
	ssStep := 1 / float64(normalSS)
	w2, h2 := srcW*normalSS, srcH*normalSS
	baseCols := makeGrayColors(w2, h2)
	baseLayers := makeWorldLayers(w2, h2)
	baseImg := mustImageStep(w2, h2, ssStep, baseCols, baseLayers)
	flatMap := mustFlatMap(w2, h2)
	domeMap := mustNormalMap(w2, h2, makeDomeNormals(w2, h2))
	flatSc := mustNormalPointScene(0.5*float64(srcW), 0.6*float64(srcH))
	sideSc := mustNormalPointScene(0.12*float64(srcW), 0.5*float64(srcH))
	dirSc := mustNormalDirScene()
	nightSc, err := light.NewScene(core.RGBA(nightR, nightG, nightB, 1), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: NewScene night: %v\n", err)
		os.Exit(1)
	}
	nightImg2, err := nightSc.Apply(baseImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: night Apply: %v\n", err)
		os.Exit(1)
	}
	flatImg2, err := flatSc.Apply(baseImg, flatMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: flat Apply: %v\n", err)
		os.Exit(1)
	}
	domeImg2, err := sideSc.Apply(baseImg, domeMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: dome Apply: %v\n", err)
		os.Exit(1)
	}
	dirImg2, err := dirSc.Apply(baseImg, domeMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: dir Apply: %v\n", err)
		os.Exit(1)
	}
	// GPU mirror agrees exactly (C both sides, headless half).
	domeGPU2, err := sideSc.ApplyGPU(baseImg, domeMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: dome ApplyGPU: %v\n", err)
		os.Exit(1)
	}
	mirrorOK := downsampleSS(domeImg2, normalSS).ApproxEqual(downsampleSS(domeGPU2, normalSS), 0)
	nightImg := downsampleSS(nightImg2, normalSS)
	flatImg := downsampleSS(flatImg2, normalSS)
	domeImg := downsampleSS(domeImg2, normalSS)
	dirImg := downsampleSS(dirImg2, normalSS)
	probeMap, probeOK := runNormalProbes(flatSc, sideSc, dirSc, domeImg, srcW, srcH)
	litPx, litFrac, slopeGain, nightMin, pixelOK := runNormalPixelAssertions(nightImg, domeImg, srcW, srcH)
	probeOK = probeOK && mirrorOK
	probeMap["mirror_ok"] = mirrorOK
	flatBuf := bufFromColors(flatImg.W, flatImg.H, flatImg.Pix)
	domeBuf := bufFromColors(domeImg.W, domeImg.H, domeImg.Pix)
	dirBuf := bufFromColors(dirImg.W, dirImg.H, dirImg.Pix)
	parity := lightParityOffscreen(domeBuf)
	// Small golden for the frozen dome look, zero tolerance (also 4x AA).
	smallBase2 := mustImageStep(goldenW*normalSS, goldenH*normalSS, ssStep, makeGrayColors(goldenW*normalSS, goldenH*normalSS), makeWorldLayers(goldenW*normalSS, goldenH*normalSS))
	smallDomeMap := mustNormalMap(goldenW*normalSS, goldenH*normalSS, makeDomeNormals(goldenW*normalSS, goldenH*normalSS))
	smallSide := mustNormalPointScene(0.12*float64(goldenW), 0.5*float64(goldenH))
	smallDome2, err := smallSide.Apply(smallBase2, smallDomeMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: small dome Apply: %v\n", err)
		os.Exit(1)
	}
	smallDome := downsampleSS(smallDome2, normalSS)
	goldenDiff, goldenTotal, goldenFirst, goldenOK := checkNormalGolden(smallDome)

	if !autoOnly && !secsSet && manualSeconds <= 0 {
		if !probeOK || !pixelOK || !goldenOK {
			fmt.Fprintf(os.Stderr, "game_light normal: selftest FAIL probe=%v pixel=%v golden=%.4f%%, not opening window\n", probeOK, pixelOK, goldenDiff)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_light normal: selftest done, entering manual phase (close X to finish)\n")
	} else if autoOnly {
		if !probeOK || !pixelOK || !goldenOK {
			fmt.Fprintf(os.Stderr, "game_light normal: selftest FAIL probe=%v pixel=%v golden=%.4f%% first=%v\n", probeOK, pixelOK, goldenDiff, goldenFirst)
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

	shell := wrkit.NewShell(winW, winH, "game_light 法线立体 — 左平光 中侧光 右斜光", []string{
		"左FLAT平光 中DOME侧光 右DIR斜光",
		"dome: 中卡左侧亮右侧暗=立体",
		"右卡斜光朝左上=左坡更亮",
		"零法线=平光, 不崩",
		"JSON看parity<=1+探针全过",
		"--case=normal, 只要这一个",
	})

	flatBox := rendering.NewRenderBox()
	flatBox.FixedWidth, flatBox.FixedHeight = cardW, cardH
	flatBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLightCard(pc, "FLAT 平光参考", flatBuf, normalIntensity/2.0, "无坡=均匀 falloff")
	}
	shell.Body.Place(flatBox, 20, 20)

	domeBox := rendering.NewRenderBox()
	domeBox.FixedWidth, domeBox.FixedHeight = cardW, cardH
	domeBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLightCard(pc, "DOME 侧光立体", domeBuf, normalIntensity/2.0, "左侧亮右侧暗=坡向对")
	}
	shell.Body.Place(domeBox, 312, 20)

	dirBox := rendering.NewRenderBox()
	dirBox.FixedWidth, dirBox.FixedHeight = cardW, cardH
	dirBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLightCard(pc, "DIR 斜光", dirBuf, 0.5, "斜光朝左上, 左坡更亮")
	}
	shell.Body.Place(dirBox, 604, 20)

	note := wrkit.Label("左平光/中侧光/右斜光, 底都是0.75灰, 立体全算法线", 12, 0.75, 0.82, 0.9)
	shell.Body.Place(note, 20, 340)
	chain := wrkit.Label("normal盖在fx之后: 夜色全局+灯高60+法线坡向调制", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(chain, 20, 362)

	var summary manualSummary
	summary.Note = "case=normal"
	elapsed := 0.0
	setTitle := func() {
		ctl := win.Controls()
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — ptr=%d key=%d rs=%d t=%.0fs", winTitle, summary.Pointer, summary.Key, summary.Resize, elapsed))
	}

	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "normal_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_light normal: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				fmt.Fprintf(os.Stderr, "game_light normal: resize %dx%d\n", ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				setTitle()
			case platform.EventPointer:
				summary.Pointer++
				fmt.Fprintf(os.Stderr, "game_light normal: pointer kind=%v @(%.0f,%.0f)\n", ev.Pointer, ev.X, ev.Y)
				setTitle()
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "game_light normal: key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "game_light normal: event %s\n", ev.Type)
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		flatBox.MarkNeedsPaint()
		domeBox.MarkNeedsPaint()
		dirBox.MarkNeedsPaint()
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
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_light", testdataDir, "normal_final.png", "normal_final_base.png", goldenRects, winW)

	parityPct, _ := parity["parity_changed_pct"].(float64)
	extra := map[string]any{
		"parity_changed_pct":  parityPct,
		"parity_mean_abs":     parity["parity_mean_abs"],
		"parity_gpu_ops":      parity["parity_gpu_ops"],
		"normal_lit_px":       litPx,
		"normal_lit_frac":     litFrac,
		"normal_slope_gain":   slopeGain,
		"normal_night_min":    nightMin,
		"mirror_ok":           mirrorOK,
		"probe_ok":            probeOK && pixelOK && goldenOK,
		"pixel_ok":            pixelOK,
		"golden_diff_pct":     goldenDiff,
		"golden_total_px":     goldenTotal,
		"golden_first_run":    goldenFirst,
		"win_golden_diff_pct": winGoldenDiff,
		"win_golden_total_px": winGoldenTotal,
		"win_golden_first":    winGoldenFirst,
		"case":                "normal",
		"pointer_events":      summary.Pointer,
		"key_events":          summary.Key,
		"resize_events":       summary.Resize,
		"manual_timed":        secsSet,
		"covered":             "左平光+中侧光+右斜光, 左坡亮右坡暗",
		"impl_correctness":    "NormalScene.Apply直调冻接口, 灯高60, 法线坡向调制",
		"impl_visible":        "HUD实时presents/lit/parity, 关窗/超时出JSON",
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

	if autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if parityPct > parityBudgetPct {
			fmt.Fprintf(os.Stderr, "FAIL: parity_changed_pct=%.4f want <=%.2f (CPU/GPU diverge)\n", parityPct, parityBudgetPct)
			os.Exit(1)
		}
		if !probeOK {
			fmt.Fprintf(os.Stderr, "FAIL: probe_ok=false case=normal (logic probes)\n")
			os.Exit(1)
		}
		if !pixelOK {
			fmt.Fprintf(os.Stderr, "FAIL: pixel assertions fail lit=%d gain=%.4f night=%.4f\n", litPx, slopeGain, nightMin)
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
		fmt.Fprintf(os.Stderr, "game_light: OK case=normal presents=%d parity=%.4f lit=%d gain=%.4f golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs\n",
			app.PresentCount(), parityPct, litPx, slopeGain, goldenDiff, winGoldenDiff, elapsedSec)
		return
	}
	summary.Timed = secsSet
	fmt.Fprintf(os.Stderr, "game_light: case=normal backend=%s presents=%d parity=%.4f lit=%d gain=%.4f golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		win.Backend(), app.PresentCount(), parityPct, litPx, slopeGain, goldenDiff, winGoldenDiff, elapsedSec, summary.Pointer, summary.Key, summary.Resize)
}
