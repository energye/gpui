// Command game_fx is the 5.3/5.4 independent real window: warp water wobble + grade film look.
//
// Window: 1200x800, title game_fx. Body left original, middle warp, right grade.
// Warp calls game/fx Warp OffsetAt/WarpRGBA (frozen, direct use). Grade calls
// game/fx Bloom/Vignette/LUT/Grade full chain bloom->vignette->lut->tonemap.
//
// Flags:
//
//	go run ./examples/game_fx --case=all -auto-only
//	  RUN_SECONDS=8 (default 8) auto gate, JSON on stdout, exit 1 on fail.
//	go run ./examples/game_fx --case=warp -manual-seconds 30
//	  resident 30s, real events logged + SetTitle, summary JSON.
//	go run ./examples/game_fx
//	  selftest then resident until close. RUN_SECONDS also times the run.
//
// Gates (hard): present>=1 via wrgate, parity_changed_pct<=1 (CPU vs GPU old
// DrawImage), logic probes all pass, pixel assertions pass, Golden static
// mask zero tolerance (second run on). Extra carries
// parity_changed_pct/warp_moved_px/grade_dark_corner/probe_ok.
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
	"github.com/energye/gpui/game/fx"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	winTitle     = "game_fx"
	srcW, srcH   = 240, 160
	cardW, cardH = 280, 300
	imgW, imgH   = 240, 160
	goldenW      = 96
	goldenH      = 64
)

// Hardcoded tolerances (review visible, never silent).
const (
	parityBudgetPct = 1.0  // C both sides: CPU vs GPU old DrawImage
	warpStrength    = 6.0  // water wobble amplitude, pixels at noise 1
	warpProbeT      = 1.5  // probe time, seconds
	warpMovedMinPx  = 10   // warp must move at least this many pixels
	warpMagMin      = 0.5  // probe offset magnitude lower bound
	warpMagMax      = 9.0  // strength*sqrt2 upper bound
	cornerDarkMin   = 0.05 // center minus corner luminance, grade
	haloGainMin     = 0.03 // halo ring minus far background, grade
	pixelDiffThresh = 2    // per-channel byte threshold for moved
	goldenTolPct    = 0.0  // static mask zero tolerance
	bloomThreshold  = 0.8  // only the bright sun blooms, sky stays out
	bloomRadius     = 2    // halo spread, px each pass
	bloomIntensity  = 0.9  // glow strength
	vigInner        = 0.25 // full-bright radius, UV
	vigOuter        = 0.85 // fully-dimmed radius, UV
	vigStrength     = 0.45 // corner dim amount
	tonemapExposure = 1.0  // Reinhard exposure
)

const testdataDir = "examples/game_fx/testdata"

type manualSummary struct {
	Pointer  int
	Key      int
	Resize   int
	Activate int
	Timed    bool
	Note     string
}

// makeSrcColors builds the deterministic lake+sun picture, row-major.
func makeSrcColors(w, h int) []core.Color {
	out := make([]core.Color, w*h)
	cx := 0.72 * float64(w)
	cy := 0.28 * float64(h)
	sunR := 0.13 * math.Min(float64(w), float64(h))
	if sunR < 4 {
		sunR = 4
	}
	for y := 0; y < h; y++ {
		v := float64(y) / float64(max1(h-1))
		for x := 0; x < w; x++ {
			u := float64(x) / float64(max1(w-1))
			var r, g, b float64
			if v < 0.45 {
				t := v / 0.45
				r = 0.35 + (0.65-0.35)*t
				g = 0.55 + (0.75-0.55)*t
				b = 0.80 + (0.88-0.80)*t
				_ = u
			} else {
				d := (v - 0.45) / 0.55
				r = 0.18 + (0.05-0.18)*d
				g = 0.38 + (0.16-0.38)*d
				b = 0.66 + (0.36-0.66)*d
				wave := 0.07 * (0.5 + 0.5*math.Sin(float64(y)*0.55+float64(x)*0.02))
				r += wave * 0.4
				g += wave * 0.6
				b += wave * 0.5
			}
			dx := float64(x) - cx
			dy := float64(y) - cy
			if math.Sqrt(dx*dx+dy*dy) < sunR {
				r, g, b = 1.0, 0.97, 0.85
			}
			out[y*w+x] = core.RGBA(clamp01(r), clamp01(g), clamp01(b), 1)
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

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// srcRGBA converts colors to RGBA8 bytes.
func srcRGBA(cols []core.Color) []uint8 {
	out := make([]uint8, len(cols)*4)
	for i, c := range cols {
		r, g, b, a := c.ToBytes()
		out[i*4] = r
		out[i*4+1] = g
		out[i*4+2] = b
		out[i*4+3] = a
	}
	return out
}

// mustWarp builds the frozen water wobble.
func mustWarp() fx.Warp {
	w, err := fx.NewWarp(warpStrength, fx.DefaultNoise())
	if err != nil {
		panic(fmt.Sprintf("game_fx: NewWarp: %v", err))
	}
	return w
}

// mustGrade builds the frozen film chain bloom->vignette->lut->tonemap.
func mustGrade() fx.Grade {
	b, err := fx.NewBloom(bloomThreshold, bloomRadius, bloomIntensity)
	if err != nil {
		panic(fmt.Sprintf("game_fx: NewBloom: %v", err))
	}
	v, err := fx.NewVignette(core.V2(0.5, 0.5), vigInner, vigOuter, vigStrength)
	if err != nil {
		panic(fmt.Sprintf("game_fx: NewVignette: %v", err))
	}
	lut, err := fx.NewLUT(
		[]float64{0, 0.18, 0.58, 1},
		[]float64{0, 0.12, 0.48, 0.96},
		[]float64{0, 0.08, 0.38, 0.88},
	)
	if err != nil {
		panic(fmt.Sprintf("game_fx: NewLUT: %v", err))
	}
	tm, err := fx.ParseTonemap("reinhard")
	if err != nil {
		panic(fmt.Sprintf("game_fx: ParseTonemap: %v", err))
	}
	g, err := fx.NewGrade(b, v, &lut, tm, tonemapExposure)
	if err != nil {
		panic(fmt.Sprintf("game_fx: NewGrade: %v", err))
	}
	return g
}

// runLogicProbes checks warp offset + grade stage identities/thresholds.
func runLogicProbes(w fx.Warp, g fx.Grade) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	off := w.OffsetAt(core.V2(100, 50), warpProbeT)
	mag := math.Sqrt(off.X*off.X + off.Y*off.Y)
	out["warp_offset_x"] = off.X
	out["warp_offset_y"] = off.Y
	out["warp_offset_mag"] = mag
	warpOK := mag > warpMagMin && mag <= warpMagMax && math.Abs(off.X) <= warpStrength && math.Abs(off.Y) <= warpStrength
	out["warp_offset_ok"] = warpOK
	if !warpOK {
		ok = false
	}
	zero := fx.Warp{}
	zeroOff := zero.OffsetAt(core.V2(100, 50), warpProbeT)
	zeroOK := zeroOff == (core.Vec2{})
	out["warp_zero_identity"] = zeroOK
	if !zeroOK {
		ok = false
	}
	out["bloom_is_identity"] = g.Bloom.IsIdentity()
	out["bloom_threshold"] = g.Bloom.Threshold
	bloomOK := !g.Bloom.IsIdentity() && g.Bloom.Threshold == bloomThreshold
	out["bloom_stage_ok"] = bloomOK
	if !bloomOK {
		ok = false
	}
	centerF := g.Vignette.FactorAt(core.V2(0.5, 0.5))
	cornerF := g.Vignette.FactorAt(core.V2(0, 0))
	out["vignette_center_factor"] = centerF
	out["vignette_corner_factor"] = cornerF
	vigOK := centerF == 1 && cornerF < 0.9 && cornerF > 0
	out["vignette_stage_ok"] = vigOK
	if !vigOK {
		ok = false
	}
	lutOK := false
	lutMid := 0.0
	if g.LUT != nil {
		if err := g.LUT.Validate(); err == nil {
			lutMid = g.LUT.Sample(0, 0.5)
			lutOK = lutMid > 0 && lutMid < 1
		}
	}
	out["lut_sample_r05"] = lutMid
	out["lut_stage_ok"] = lutOK
	if !lutOK {
		ok = false
	}
	mapped := g.Tonemap.Map(0.5, tonemapExposure)
	out["tonemap_map_05"] = mapped
	out["tonemap_name"] = g.Tonemap.String()
	tmOK := mapped < 0.5 && mapped > 0
	out["tonemap_stage_ok"] = tmOK
	if !tmOK {
		ok = false
	}
	stages := g.Stages()
	stagesOK := len(stages) == 4 && stages[0] == "bloom" && stages[1] == "vignette" && stages[2] == "lut" && stages[3] == "tonemap"
	out["grade_stages"] = stages
	out["grade_stages_ok"] = stagesOK
	if !stagesOK {
		ok = false
	}
	out["probe_ok"] = ok
	return out, ok
}

func luminance(c core.Color) float64 {
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

// runPixelAssertions checks warp moved-not-flooded, grade corner darker, halo spread.
func runPixelAssertions(srcRGBABytes []uint8, w, h int, warped []uint8, graded *fx.Image) (movedPx int, movedFrac float64, cornerDiff, haloGain float64, ok bool) {
	totalPx := w * h
	movedBytes := 0
	movedPx = 0
	for i := 0; i < totalPx; i++ {
		diffPix := false
		for k := 0; k < 4; k++ {
			d := int(srcRGBABytes[i*4+k]) - int(warped[i*4+k])
			if d < 0 {
				d = -d
			}
			if d > pixelDiffThresh {
				movedBytes++
				diffPix = true
			} else if k < 3 && srcRGBABytes[i*4+k] != warped[i*4+k] {
				// Count exact byte diffs too for the flood guard.
				movedBytes++
				diffPix = true
			}
		}
		_ = diffPix
	}
	// Pixel-level moved count: any RGB channel differs at all.
	for i := 0; i < totalPx; i++ {
		if srcRGBABytes[i*4] != warped[i*4] || srcRGBABytes[i*4+1] != warped[i*4+1] || srcRGBABytes[i*4+2] != warped[i*4+2] {
			movedPx++
		}
	}
	movedFrac = float64(movedPx) / float64(totalPx)
	warpOK := movedPx >= warpMovedMinPx && movedBytes < len(srcRGBABytes)
	// Grade corner vs center on the graded float image.
	cornerSum, cornerN := 0.0, 0
	for _, pt := range [][2]int{{1, 1}, {w - 2, 1}, {1, h - 2}, {w - 2, h - 2}} {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				x, y := pt[0]+dx, pt[1]+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				cornerSum += luminance(graded.Pix[y*w+x])
				cornerN++
			}
		}
	}
	cornerLum := cornerSum / float64(max1(cornerN))
	centerSum, centerN := 0.0, 0
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			x, y := w/2+dx, h/2+dy
			centerSum += luminance(graded.Pix[y*w+x])
			centerN++
		}
	}
	centerLum := centerSum / float64(max1(centerN))
	cornerDiff = centerLum - cornerLum
	cornerOK := cornerDiff > cornerDarkMin
	// Halo: ring just outside the sun disc vs far water.
	sunCX := int(0.72 * float64(w))
	sunCY := int(0.28 * float64(h))
	sunR := int(0.13 * math.Min(float64(w), float64(h)))
	haloX := sunCX + sunR + 3
	if haloX >= w-1 {
		haloX = w - 2
	}
	haloLum := luminance(graded.Pix[sunCY*w+haloX])
	farLum := luminance(graded.Pix[(h-h/10)*w+w/10])
	haloGain = haloLum - farLum
	haloOK := haloGain > haloGainMin
	ok = warpOK && cornerOK && haloOK
	return movedPx, movedFrac, cornerDiff, haloGain, ok
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

// fxParityOffscreen renders the fx source via old DrawImage on CPU vs GPU.
func fxParityOffscreen(src *render.ImageBuf) map[string]any {
	out := map[string]any{}
	const pw, ph = 96, 60
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	dcCPU := render.NewContext(pw, ph)
	dcCPU.ClearWithColor(render.White)
	dcCPU.DrawImageEx(src, render.DrawImageOptions{
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
	dcGPU.DrawImageEx(src, render.DrawImageOptions{
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

// checkOffscreenGolden compares the small graded image to the frozen PNG, zero tolerance.
func checkOffscreenGolden(graded *fx.Image) (diffPct float64, totalPx int64, firstRun bool, ok bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, "fx_grade_golden.png")
	cur := image.NewRGBA(image.Rect(0, 0, graded.W, graded.H))
	for y := 0; y < graded.H; y++ {
		for x := 0; x < graded.W; x++ {
			r, g, b, a := graded.Pix[y*graded.W+x].ToBytes()
			cur.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game_fx: golden store %s: %v\n", basePath, err)
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "game_fx: golden baseline stored: %s\n", basePath)
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_fx: golden open: %v\n", err)
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_fx: golden decode: %v\n", err)
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		fmt.Fprintf(os.Stderr, "game_fx: golden size %v vs %v\n", want.Bounds(), cur.Bounds())
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
		panic(fmt.Sprintf("game_fx: NewImageBuf: %v", err))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := cols[y*w+x].ToBytes()
			_ = buf.SetRGBA(x, y, r, g, b, a)
		}
	}
	return buf
}

func bufFromRGBABytes(w, h int, px []uint8) *render.ImageBuf {
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		panic(fmt.Sprintf("game_fx: NewImageBuf: %v", err))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			_ = buf.SetRGBA(x, y, px[i], px[i+1], px[i+2], px[i+3])
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

func abilityForCase(c string) string {
	switch c {
	case "warp":
		return "fx-warp"
	case "grade":
		return "fx-grade"
	case "custom":
		return "fx-custom"
	default:
		return "fx-all"
	}
}

func main() {
	caseFlag := flag.String("case", "all", "fx case: warp|grade|custom|all")
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	flag.Parse()
	caseName := *caseFlag
	if caseName != "warp" && caseName != "grade" && caseName != "custom" && caseName != "all" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want warp|grade|custom|all\n", caseName)
		os.Exit(1)
	}
	abilityID := abilityForCase(caseName)
	scenario := "game_fx--case=" + caseName

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

	// Headless evidence before the window opens (post-close GPU draws lose depth).
	wvb := mustWarp()
	grd := mustGrade()
	srcCols := makeSrcColors(srcW, srcH)
	srcBytes := srcRGBA(srcCols)
	srcImg, err := fx.NewImageFromColors(srcW, srcH, srcCols)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: NewImageFromColors: %v\n", err)
		os.Exit(1)
	}
	warpedBytes, err := fx.WarpRGBA(srcBytes, srcW, srcH, warpProbeT, wvb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: WarpRGBA: %v\n", err)
		os.Exit(1)
	}
	gradedImg, err := grd.Apply(srcImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: Grade.Apply: %v\n", err)
		os.Exit(1)
	}
	probeMap, probeOK := runLogicProbes(wvb, grd)
	movedPx, movedFrac, cornerDiff, haloGain, pixelOK := runPixelAssertions(srcBytes, srcW, srcH, warpedBytes, gradedImg)
	// Case filtering: single cases only gate their own pixels/probes.
	casePixelOK := pixelOK
	caseProbeOK := probeOK
	if caseName == "warp" {
		cornerOK := cornerDiff > cornerDarkMin
		haloOK := haloGain > haloGainMin
		_ = cornerOK
		_ = haloOK
		// Warp-only still reports grade numbers, but gate only needs warp moved.
		warpOnlyOK := movedPx >= warpMovedMinPx
		casePixelOK = warpOnlyOK && probeMap["warp_offset_ok"] == true
		caseProbeOK = probeMap["warp_offset_ok"] == true && probeMap["warp_zero_identity"] == true
	} else if caseName == "grade" {
		warpOnlyOK := movedPx >= warpMovedMinPx
		_ = warpOnlyOK
		casePixelOK = cornerDiff > cornerDarkMin && haloGain > haloGainMin
		caseProbeOK = probeMap["bloom_stage_ok"] == true && probeMap["vignette_stage_ok"] == true && probeMap["lut_stage_ok"] == true && probeMap["tonemap_stage_ok"] == true && probeMap["grade_stages_ok"] == true
	}
	srcBuf := bufFromColors(srcW, srcH, srcCols)
	parity := fxParityOffscreen(srcBuf)
	// Small golden for the frozen grade look, zero tolerance.
	smallCols := makeSrcColors(goldenW, goldenH)
	smallImg, err := fx.NewImageFromColors(goldenW, goldenH, smallCols)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: small NewImageFromColors: %v\n", err)
		os.Exit(1)
	}
	smallGraded, err := grd.Apply(smallImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: small Grade.Apply: %v\n", err)
		os.Exit(1)
	}
	goldenDiff, goldenTotal, goldenFirst, goldenOK := checkOffscreenGolden(smallGraded)

	// Custom case evidence (5.5): same probe+pixel+golden rule as grade.
	customOutline := mustOutline()
	customDissolve := mustDissolve()
	customSrcCols := makeCustomSrcColors(srcW, srcH)
	customSrcImg, err := fx.NewImageFromColors(srcW, srcH, customSrcCols)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: custom NewImageFromColors: %v\n", err)
		os.Exit(1)
	}
	customOutlined, _, err := customOutline.Apply(customSrcImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: custom outline Apply: %v\n", err)
		os.Exit(1)
	}
	customDissolved, _, err := customDissolve.Apply(customSrcImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: custom dissolve Apply: %v\n", err)
		os.Exit(1)
	}
	customProbeMap, customProbeOK := runCustomLogicProbes(customOutline, customDissolve)
	customOutlinePx, customKept, customEdged, customGone, customPixelOK := runCustomPixelAssertions(customSrcImg, customOutlined, customDissolved)
	smallCustomCols := makeCustomSrcColors(goldenW, goldenH)
	smallCustomImg, err := fx.NewImageFromColors(goldenW, goldenH, smallCustomCols)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: small custom NewImageFromColors: %v\n", err)
		os.Exit(1)
	}
	smallOutlined, _, err := customOutline.Apply(smallCustomImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: small custom outline Apply: %v\n", err)
		os.Exit(1)
	}
	smallDissolved, _, err := customDissolve.Apply(smallCustomImg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: small custom dissolve Apply: %v\n", err)
		os.Exit(1)
	}
	customGoldenDiff, customGoldenTotal, customGoldenFirst, customGoldenOK := checkCustomOffscreenGolden(smallOutlined, smallDissolved)
	if caseName == "custom" {
		casePixelOK = customPixelOK
		caseProbeOK = customProbeOK
		goldenDiff, goldenTotal, goldenFirst, goldenOK = customGoldenDiff, customGoldenTotal, customGoldenFirst, customGoldenOK
	}

	if !*autoOnly && !secsSet && *manualSeconds <= 0 {
		if !caseProbeOK || !casePixelOK || !goldenOK {
			fmt.Fprintf(os.Stderr, "game_fx: selftest FAIL probe=%v pixel=%v golden=%.4f%%, not opening window\n", caseProbeOK, casePixelOK, goldenDiff)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_fx: selftest done, entering manual phase (close X to finish)\n")
	} else if *autoOnly {
		if !caseProbeOK || !casePixelOK || !goldenOK {
			fmt.Fprintf(os.Stderr, "game_fx: selftest FAIL probe=%v pixel=%v golden=%.4f%% first=%v\n", caseProbeOK, casePixelOK, goldenDiff, goldenFirst)
		}
	}

	// Precompute warp animation frames (frozen times, fresh intermediates).
	const warpFrames = 8
	warpBufs := make([]*render.ImageBuf, warpFrames)
	for i := 0; i < warpFrames; i++ {
		t := float64(i) * 0.2
		px, err := fx.WarpRGBA(srcBytes, srcW, srcH, t, wvb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: warp frame %d: %v\n", i, err)
			os.Exit(1)
		}
		warpBufs[i] = bufFromRGBABytes(srcW, srcH, px)
	}
	gradeBuf := bufFromColors(gradedImg.W, gradedImg.H, gradedImg.Pix)
	gradeForCase := gradeBuf
	if caseName == "warp" {
		// Warp-only: right card shows the original so layout stays, gate ignores grade.
		gradeForCase = srcBuf
	}
	customSrcBuf := bufFromColors(srcW, srcH, customSrcCols)
	customOutlineBuf := bufFromColors(customOutlined.W, customOutlined.H, customOutlined.Pix)
	// Precompute dissolve animation frames (frozen cycle amounts).
	customDissolveBufs := make([]*render.ImageBuf, len(customDissolveCycle))
	for i, amount := range customDissolveCycle {
		frame, _, err := mustDissolveAt(amount).Apply(customSrcImg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: custom dissolve frame %d: %v\n", i, err)
			os.Exit(1)
		}
		customDissolveBufs[i] = bufFromColors(frame.W, frame.H, frame.Pix)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shellTitle := "game_fx 水晃+胶片 — 左原图 中warp水晃 右grade胶片"
	shellLegend := []string{
		"左ORIG原图 中WARP水晃 右GRADE胶片",
		"warp: OffsetAt/WarpRGBA, 强度条=振幅",
		"grade: 发光→暗角→查表→映射",
		"四角压暗+亮日带晕=气氛对",
		"JSON看parity<=1+探针全过",
		"--case=warp|grade|all, 默认all",
	}
	if caseName == "custom" {
		shellTitle = "game_fx 材质钩子 — 左原片 中outline描边 右dissolve溶解"
		shellLegend = []string{
			"左CUSTOM原片 中OUTLINE描边 右DISSOLVE溶解",
			"outline: 红环width=1, 中心原色不动",
			"dissolve: 橙边+透明, 量走0.2→0.8",
			"坏钩子占位主路不断=气氛对",
			"JSON看outline/dissolve/金图全过",
			"--case=warp|grade|custom|all, 默认all",
		}
	}
	shell := wrkit.NewShell(winW, winH, shellTitle, shellLegend)

	origBox := rendering.NewRenderBox()
	origBox.FixedWidth, origBox.FixedHeight = cardW, cardH
	origBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		origCard := srcBuf
		origTitle, origFoot := "ORIG 原图", "湖+日原片"
		if caseName == "custom" {
			origCard, origTitle, origFoot = customSrcBuf, "CUSTOM 原片", "蓝块+白芯透明底"
		}
		paintFxCard(pc, origTitle, origCard, -1, origFoot)
	}
	shell.Body.Place(origBox, 20, 20)

	warpIdx := 0
	customIdx := 0
	warpBox := rendering.NewRenderBox()
	warpBox.FixedWidth, warpBox.FixedHeight = cardW, cardH
	warpBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if caseName == "custom" {
			paintFxCard(pc, "OUTLINE 描边 width=1", customOutlineBuf, customOutlineBar, "红环+中心原色")
			return
		}
		cur := warpBufs[warpIdx%len(warpBufs)]
		if caseName == "grade" {
			cur = srcBuf
		}
		paintFxCard(pc, "WARP 水晃 strength=6.0", cur, warpStrength/10.0, "OffsetAt噪声偏移采样")
	}
	shell.Body.Place(warpBox, 312, 20)

	gradeBox := rendering.NewRenderBox()
	gradeBox.FixedWidth, gradeBox.FixedHeight = cardW, cardH
	gradeBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if caseName == "custom" {
			cur := customDissolveBufs[customIdx%len(customDissolveBufs)]
			amount := customDissolveCycle[customIdx%len(customDissolveCycle)]
			paintFxCard(pc, fmt.Sprintf("DISSOLVE 溶解 amount=%.2f", amount), cur, amount, "橙边+透明混合")
			return
		}
		paintFxCard(pc, "GRADE 胶片 Bloom暗角LUT映射", gradeForCase, -1, "四角压暗+亮日带晕")
	}
	shell.Body.Place(gradeBox, 604, 20)

	noteText, chainText := "左原图/中水晃/右胶片, 强度条=warp振幅", "grade链: 发光Bloom→暗角Vignette→查表LUT→映射Tonemap"
	if caseName == "custom" {
		noteText = "左原片/中描边/右溶解, 强度条=dissolve量"
		chainText = "custom钩子: outline描边→dissolve溶解, 坏钩子占位主路不断"
	}
	note := wrkit.Label(noteText, 12, 0.75, 0.82, 0.9)
	shell.Body.Place(note, 20, 340)
	chain := wrkit.Label(chainText, 12, 0.70, 0.78, 0.88)
	shell.Body.Place(chain, 20, 362)

	var summary manualSummary
	summary.Note = "case=" + caseName
	elapsed := 0.0
	warpAccum := 0.0
	setTitle := func() {
		ctl := win.Controls()
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — ptr=%d key=%d rs=%d t=%.0fs", winTitle, summary.Pointer, summary.Key, summary.Resize, elapsed))
	}

	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "fx_final.png")
	if caseName == "custom" {
		snapPath = filepath.Join(testdataDir, "fx_custom_last.png")
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_fx: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				fmt.Fprintf(os.Stderr, "game_fx: resize %dx%d\n", ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				setTitle()
			case platform.EventPointer:
				summary.Pointer++
				fmt.Fprintf(os.Stderr, "game_fx: pointer kind=%v @(%.0f,%.0f)\n", ev.Pointer, ev.X, ev.Y)
				setTitle()
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "game_fx: key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "game_fx: event %s\n", ev.Type)
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		warpAccum += dt
		if warpAccum >= 0.12 {
			warpAccum = 0
			warpIdx++
			customIdx++
			warpBox.MarkNeedsPaint()
		}
		origBox.MarkNeedsPaint()
		gradeBox.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && caseProbeOK && casePixelOK
		hudLeft, hudRight := fmt.Sprintf("presents=%d moved=%d", app.PresentCount(), movedPx), fmt.Sprintf("parity=%.2f probe=%v", parity["parity_changed_pct"], caseProbeOK)
		if caseName == "custom" {
			hudLeft = fmt.Sprintf("presents=%d outline=%d", app.PresentCount(), customOutlinePx)
			hudRight = fmt.Sprintf("kept=%d edge=%d gone=%d", customKept, customEdged, customGone)
		}
		shell.UpdateHUD(abilityID, "Steady", app, gateOK, hudLeft, hudRight)
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

	// Window Golden over the static mask (warp water + HUD excluded by design).
	// Custom case reuses the same shell but snapshots its own file and masks
	// only static rects: the right dissolve card animates, so it stays out.
	snapName, customWinBase := "fx_final.png", "fx_final_base.png"
	goldenRects := []wrsoak.Rect{
		{X: 0, Y: 0, W: winW, H: 48},
		{X: 12, Y: 60, W: 260, H: 656},
		{X: 284 + 20 + 20, Y: 60 + 20 + 40, W: imgW, H: imgH},
		{X: 284 + 604 + 20, Y: 60 + 20 + 40, W: imgW, H: imgH},
	}
	if caseName == "custom" {
		snapName, customWinBase = "fx_custom_last.png", "fx_custom_base.png"
		goldenRects = []wrsoak.Rect{
			{X: 0, Y: 0, W: winW, H: 48},
			{X: 12, Y: 60, W: 260, H: 656},
			{X: 284 + 20 + 20, Y: 60 + 20 + 40, W: imgW, H: imgH},
			{X: 284 + 20 + 20 + 292, Y: 60 + 20 + 40, W: imgW, H: imgH},
		}
	}
	_ = os.MkdirAll(testdataDir, 0o755)
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_fx", testdataDir, snapName, customWinBase, goldenRects, winW)

	parityPct, _ := parity["parity_changed_pct"].(float64)
	extra := map[string]any{
		"parity_changed_pct":  parityPct,
		"parity_mean_abs":     parity["parity_mean_abs"],
		"parity_gpu_ops":      parity["parity_gpu_ops"],
		"warp_moved_px":       movedPx,
		"warp_moved_frac":     movedFrac,
		"warp_offset_mag":     probeMap["warp_offset_mag"],
		"grade_dark_corner":   cornerDiff,
		"grade_halo_gain":     haloGain,
		"grade_center_factor": probeMap["vignette_center_factor"],
		"probe_ok":            caseProbeOK && casePixelOK && goldenOK,
		"warp_probe_ok":       probeMap["warp_offset_ok"],
		"pixel_ok":            casePixelOK,
		"golden_diff_pct":     goldenDiff,
		"golden_total_px":     goldenTotal,
		"golden_first_run":    goldenFirst,
		"win_golden_diff_pct": winGoldenDiff,
		"win_golden_total_px": winGoldenTotal,
		"win_golden_first":    winGoldenFirst,
		"case":                caseName,
		"custom_outline_px":   customOutlinePx,
		"custom_kept_px":      customKept,
		"custom_edge_px":      customEdged,
		"custom_gone_px":      customGone,
		"pointer_events":      summary.Pointer,
		"key_events":          summary.Key,
		"resize_events":       summary.Resize,
		"manual_timed":        secsSet,
		"covered":             "左原图+中水晃warp+右胶片grade, 强度条, 四角压暗+亮日带晕",
		"impl_correctness":    "warp OffsetAt/WarpRGBA直调冻接口, grade Bloom/Vignette/LUT/Grade整链直调",
		"impl_visible":        "HUD实时presents/moved/parity, 强度条=振幅, 关窗/超时出JSON",
	}
	for k, v := range customProbeMap {
		if _, dup := extra[k]; !dup {
			extra[k] = v
		}
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
		if !caseProbeOK {
			fmt.Fprintf(os.Stderr, "FAIL: probe_ok=false case=%s (logic probes)\n", caseName)
			os.Exit(1)
		}
		if !casePixelOK {
			if caseName == "custom" {
				fmt.Fprintf(os.Stderr, "FAIL: pixel assertions fail outline=%d kept=%d edge=%d gone=%d\n", customOutlinePx, customKept, customEdged, customGone)
			} else {
				fmt.Fprintf(os.Stderr, "FAIL: pixel assertions fail moved=%d corner=%.4f halo=%.4f\n", movedPx, cornerDiff, haloGain)
			}
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
		fmt.Fprintf(os.Stderr, "game_fx: OK case=%s presents=%d parity=%.4f moved=%d corner=%.4f halo=%.4f golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs\n",
			caseName, app.PresentCount(), parityPct, movedPx, cornerDiff, haloGain, goldenDiff, winGoldenDiff, elapsedSec)
		return
	}
	summary.Timed = secsSet
	fmt.Fprintf(os.Stderr, "game_fx: case=%s backend=%s presents=%d parity=%.4f moved=%d corner=%.4f halo=%.4f golden=%.4f%% win_golden=%.4f%% elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		caseName, win.Backend(), app.PresentCount(), parityPct, movedPx, cornerDiff, haloGain, goldenDiff, winGoldenDiff, elapsedSec, summary.Pointer, summary.Key, summary.Resize)
}

func paintFxCard(pc *rendering.PaintContext, title string, img *render.ImageBuf, barFrac float64, foot string) {
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
		pc.DC.SetRGBA(0.30, 0.65, 0.95, 1)
		pc.DC.DrawRectangle(bx, by, fw, bh)
		_ = pc.DC.Fill()
		if face := wrkit.FaceAt(11); face != nil {
			pc.DC.SetFont(face)
		}
		pc.DC.SetRGBA(0.75, 0.82, 0.90, 1)
		pc.DC.DrawString(fmt.Sprintf("强度 %.1f", barFrac*10.0), bx+6, by+11)
	}
	if face := wrkit.FaceAt(11); face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(0.70, 0.78, 0.88, 1)
	pc.DC.DrawString(foot, ax+12, ay+262)
}
