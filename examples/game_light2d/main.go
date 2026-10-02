// Command game_light2d is the S82 independent real window: light/shadow
// 对照 (G21/W14, waits on S63). One case only: shadow.
//
// Scene: warm point torch plus one directional lamp over a person strip,
// a vertical wall occluder cuts the person in half; the right half falls
// back to night. All light math calls engine/light directly (Scene for
// night, Shadow for the wall, SoftShadow for the S63 None/PCF5/PCF13
// grades); the lit pictures are drawn with the existing render
// DrawImageEx, render main path untouched.
//
// Flags:
//
//	go run ./examples/game_light2d -case=shadow -auto-only
//	  three-evidence selftest (logic probes + pixels + zero-tolerance
//	  goldens, first run freezes the baselines), then a ~8s real window
//	  (RUN_SECONDS overrides, at least 5s); wrgate JSON on stdout,
//	  present>=1 else exit 1.
//	go run ./examples/game_light2d -case=shadow -manual-seconds 30
//	  resident 30s: click toggles the lamps, 1/2/3 or space switches the
//	  soft grade; every event is logged and shown in the title, summary
//	  JSON at the end.
//	go run ./examples/game_light2d -case=shadow
//	  selftest then resident until close.
//
// Dark switch copies the lights_and_shadows feel: one switch turns both
// lamps off (night only) and back on; the three soft grades stay
// switchable either way so the hand feel matches the reference demo.
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

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/light"
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

const (
	winW, winH = 1200, 800
	winTitle   = "game_light2d"
	abilityID  = "light2d-shadow"
	scenario   = "game_light2d--case=shadow"

	srcW, srcH   = 240, 160
	cardW, cardH = 280, 300
	imgW, imgH   = 240, 160
	goldenW      = 96
	goldenH      = 64
)

// Frozen look numbers (same family as game_light, re-pinned here).
const (
	nightR = 0.25
	nightG = 0.25
	nightB = 0.35

	torchIntensity = 1.2
	torchRange     = 80.0
	cookieN        = 9

	dirIntensity = 0.35

	personGainMin = 0.20
	nightBlackMin = 0.03
	litMinPx      = 100
	maxSoftFrac   = 0.30
	goldenTolPct  = 0.0
)

const testdataDir = "examples/game_light2d/testdata"

// gradeOrder is the S63 switch order: hard, default soft, finest soft.
var gradeOrder = []light.SoftMode{light.SoftNone, light.SoftPCF5, light.SoftPCF13}

func gradeFile(m light.SoftMode) string {
	switch m {
	case light.SoftPCF5:
		return "soft_pcf5_golden.png"
	case light.SoftPCF13:
		return "soft_pcf13_golden.png"
	default:
		return "soft_none_golden.png"
	}
}

type manualSummary struct {
	Pointer      int
	Key          int
	Resize       int
	Toggles      int
	GradeChanges int
	Timed        bool
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

// personRect marks the lit person strip (world layer); the rest is
// background (fx layer, lamps must not touch it).
func personRect(x, y, w, h int) bool {
	px0, px1 := int(0.44*float64(w)), int(0.56*float64(w))
	py0, py1 := int(0.38*float64(h)), int(0.82*float64(h))
	return x >= px0 && x < px1 && y >= py0 && y < py1
}

func makeBaseColors(w, h int) []core.Color {
	out := make([]core.Color, w*h)
	for y := 0; y < h; y++ {
		v := float64(y) / float64(max1(h-1))
		for x := 0; x < w; x++ {
			var r, g, b float64
			if v < 0.45 {
				t := v / 0.45
				r = 0.45 + (0.70-0.45)*t
				g = 0.62 + (0.78-0.62)*t
				b = 0.85 + (0.90-0.85)*t
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

// mustCookie builds the frozen round flashlight slide.
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
		panic(fmt.Sprintf("game_light2d: NewCookie: %v", err))
	}
	return &ck
}

// mustLamps builds the two frozen lamps: warm point torch on the person
// plus a horizontal directional lamp (travels +X, same open/lee pattern
// as the torch against the vertical wall).
func mustLamps(personCX, personCY float64) []light.Light {
	pt, err := light.NewPointLight(
		core.V2(personCX, personCY), torchRange,
		core.RGBA(1, 0.95, 0.8, 1), torchIntensity,
		light.LayersFor(light.LayerWorld), mustCookie())
	if err != nil {
		panic(fmt.Sprintf("game_light2d: NewPointLight: %v", err))
	}
	dl, err := light.NewDirectionalLight(
		core.V2(1, 0), core.RGBA(0.9, 0.95, 1, 1), dirIntensity,
		light.LayersFor(light.LayerWorld))
	if err != nil {
		panic(fmt.Sprintf("game_light2d: NewDirectionalLight: %v", err))
	}
	return []light.Light{pt, dl}
}

func mustNightScene() light.Scene {
	sc, err := light.NewScene(core.RGBA(nightR, nightG, nightB, 1), nil)
	if err != nil {
		panic(fmt.Sprintf("game_light2d: NewScene night: %v", err))
	}
	return sc
}

func mustLampScene(personCX, personCY float64) light.Scene {
	sc, err := light.NewScene(core.RGBA(nightR, nightG, nightB, 1), mustLamps(personCX, personCY))
	if err != nil {
		panic(fmt.Sprintf("game_light2d: NewScene lamps: %v", err))
	}
	return sc
}

func mustImage(w, h int, cols []core.Color, layers []light.Layer) *light.Image {
	img, err := light.NewImageFromColors(w, h, core.V2(0, 0), 1, cols, layers)
	if err != nil {
		panic(fmt.Sprintf("game_light2d: NewImageFromColors: %v", err))
	}
	return img
}

// mustWall builds the frozen vertical wall over the person's right half,
// spanning past both edges so every grade sees the same lee.
func mustWall(w, h int) light.Shadow {
	x0, x1 := 0.52*float64(w), 0.56*float64(w)
	y0, y1 := -0.1*float64(h), 1.1*float64(h)
	o, err := light.NewOccluder([]core.Vec2{
		core.V2(x0, y0), core.V2(x1, y0), core.V2(x1, y1), core.V2(x0, y1),
	})
	if err != nil {
		panic(fmt.Sprintf("game_light2d: NewOccluder: %v", err))
	}
	s, err := light.NewShadow([]light.Occluder{o}, 1, 1000)
	if err != nil {
		panic(fmt.Sprintf("game_light2d: NewShadow: %v", err))
	}
	return s
}

func mustSoft(w, h int, mode light.SoftMode) light.SoftShadow {
	s, err := light.NewSoftShadow(mustWall(w, h), mode, 0)
	if err != nil {
		panic(fmt.Sprintf("game_light2d: NewSoftShadow %s: %v", mode, err))
	}
	return s
}

func luminance(c core.Color) float64 {
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

func bufFromColors(w, h int, cols []core.Color) *render.ImageBuf {
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		panic(fmt.Sprintf("game_light2d: NewImageBuf: %v", err))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := cols[y*w+x].ToBytes()
			_ = buf.SetRGBA(x, y, r, g, b, a)
		}
	}
	return buf
}

func imgToRGBA(img *light.Image) *image.RGBA {
	cur := image.NewRGBA(image.Rect(0, 0, img.W, img.H))
	for y := 0; y < img.H; y++ {
		for x := 0; x < img.W; x++ {
			r, g, b, a := img.Pix[y*img.W+x].ToBytes()
			cur.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	return cur
}

// diffExactCount counts bitwise-different pixels between two same-size
// light images.
func diffExactCount(a, b *light.Image) int {
	if a.W != b.W || a.H != b.H {
		return a.W * a.H
	}
	n := 0
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			n++
		}
	}
	return n
}

type gradeEvidence struct {
	Mode        light.SoftMode
	Img         *light.Image
	Bill        light.FrameBill
	MirrorOK    bool
	MovedPx     int
	MovedFrac   float64
	PersonGain  float64
	LeeDiff     float64
	PixelOK     bool
	GoldenDiff  float64
	GoldenTotal int64
	GoldenFirst bool
	GoldenOK    bool
}

type probeReport struct {
	LogicOK   bool
	LogicMap  map[string]any
	Grades    []gradeEvidence
	PixelOK   bool
	GoldenOK  bool
	OffIsDark bool
	OK        bool
}

// runLogicProbes checks the S63 switch rules on frozen points: parse
// round-trip, tap counts, open/lee factors per grade, CPU/GPU mirror,
// billing, OOM downgrade chain, budgets, night floor, both lamp kinds,
// empty shadow, and the dark-switch identity (lamps off == night).
func runLogicProbes(base, nightImg *light.Image, lamps []light.Light, w, h int) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	fail := func(k string, v any) {
		out[k] = v
		ok = false
	}

	person := core.V2(0.5*float64(w), 0.6*float64(h))
	lee := core.V2(0.545*float64(w), 0.6*float64(h))
	if len(lamps) != 2 {
		fail("lamp_count", len(lamps))
		return out, false
	}
	out["lamp_count"] = 2
	torch, dir := lamps[0], lamps[1]
	if torch.Kind != light.KindPoint || dir.Kind != light.KindDirectional {
		fail("lamp_kinds", torch.Kind.String()+"/"+dir.Kind.String())
	} else {
		out["lamp_kinds"] = "point/directional"
	}

	wantTaps := map[light.SoftMode]int{light.SoftNone: 1, light.SoftPCF5: 5, light.SoftPCF13: 13}
	for _, m := range gradeOrder {
		pm, err := light.ParseSoftMode(m.String())
		if err != nil || pm != m {
			fail("parse_"+m.String(), m.String())
			continue
		}
		if m.Taps() != wantTaps[m] {
			fail("taps_"+m.String(), m.Taps())
		}
		soft := mustSoft(w, h, m)
		if soft.Taps() != wantTaps[m] {
			fail("soft_taps_"+m.String(), soft.Taps())
		}
		// Open point stays fully lit on every grade, CPU and GPU agree.
		for _, l := range lamps {
			f := soft.SoftFactor(l, person)
			g := soft.SoftFactorGPU(l, person)
			if f != 1 || g != 1 {
				fail("open_"+m.String(), fmt.Sprintf("%s f=%v g=%v", l.Kind, f, g))
			}
		}
		// Lee point: hard grade fully dark, soft grades stay within [0,1].
		lf := soft.SoftFactor(torch, lee)
		lg := soft.SoftFactorGPU(torch, lee)
		out["lee_"+m.String()] = lf
		if lg != lf {
			fail("lee_mirror_"+m.String(), fmt.Sprintf("%v vs %v", lf, lg))
		}
		if m == light.SoftNone && lf != 0 {
			fail("lee_none", lf)
		}
		if lf < 0 || lf > 1 {
			fail("lee_range_"+m.String(), lf)
		}
		// Mirror: full Apply agrees bitwise between CPU and GPU paths.
		a, bill, err := soft.Apply(base, lamps, mustNightScene().Night)
		if err != nil {
			fail("apply_"+m.String(), err.Error())
			continue
		}
		ag, _, err := soft.ApplyGPU(base, lamps, mustNightScene().Night)
		if err != nil || !a.ApproxEqual(ag, 0) {
			fail("apply_mirror_"+m.String(), "diverge")
		}
		if bill.Taps != wantTaps[m] || bill.Cost != bill.Total*bill.Taps || len(bill.PerLight) != 2 {
			fail("bill_"+m.String(), bill)
		}
		if bill.Total <= 0 {
			fail("bill_cover_"+m.String(), bill.Total)
		}
		if over := soft.OverPCF13Count(3); m == light.SoftPCF13 && !over {
			fail("over_pcf13", over)
		}
		if light.OverShadowLightCount(light.MaxShadowLights+1) != true {
			fail("over_count", false)
		}
		out["bill_"+m.String()] = bill.Total
	}

	// OOM downgrade chain steps exactly one grade per call with a log line.
	var log []string
	d := mustSoft(w, h, light.SoftPCF13)
	if !d.DowngradeOOM(&log, "probe") || d.Mode != light.SoftPCF5 {
		fail("downgrade_13_5", d.Mode.String())
	}
	if !d.DowngradeOOM(&log, "probe") || d.Mode != light.SoftNone {
		fail("downgrade_5_0", d.Mode.String())
	}
	if d.DowngradeOOM(&log, "probe") || len(log) != 3 {
		fail("downgrade_bottom", len(log))
	}
	var auto light.SoftShadow = mustSoft(w, h, light.SoftPCF13)
	var alog []string
	if !auto.AutoDowngradeForLights(&alog, light.MaxPCF13Lights+1) || auto.Mode != light.SoftPCF5 {
		fail("auto_downgrade", auto.Mode.String())
	}

	// Night floor keeps the dark readable, never black.
	fl := light.ClampNightFloor(core.RGBA(nightR, nightG, nightB, 1))
	if fl.R != nightR || fl.G != nightG || fl.B != nightB {
		fail("night_floor", fl)
	}
	if err := light.CheckNightFloor(core.RGBA(nightR, nightG, nightB, 1)); err != nil {
		fail("night_floor_check", err.Error())
	}

	// Directional lamp follows the same open/lee pattern as the torch.
	sh := mustWall(w, h)
	if sh.Blocked(dir, person) || !sh.Blocked(dir, lee) {
		fail("dir_blocked", fmt.Sprintf("open=%v lee=%v", sh.Blocked(dir, person), sh.Blocked(dir, lee)))
	}
	if dir.FactorAt(person, light.LayerWorld) != 1 {
		fail("dir_factor", dir.FactorAt(person, light.LayerWorld))
	}

	// Empty shadow keeps the plain lamps: factor 1, never blocked.
	empty, err := light.NewShadow(nil, 1, 1000)
	if err != nil || empty.FactorFor(torch, lee) != 1 || empty.Blocked(torch, lee) {
		fail("empty_shadow", "must pass through")
	}

	// Dark switch identity: lamps off renders exactly the night picture.
	off, err := mustNightScene().Apply(base)
	if err != nil || !off.ApproxEqual(nightImg, 0) {
		fail("off_is_night", "lamps-off must equal night")
	} else {
		out["off_is_night"] = true
	}

	// Lit stats see the torch: some pixels beat the gain gate.
	lit, _, err := mustSoft(w, h, light.SoftNone).Apply(base, lamps, mustNightScene().Night)
	if err != nil {
		fail("lit_apply", err.Error())
	} else if n, gain := light.LitStats(base, lit, mustNightScene().Night); n <= 0 || gain <= 0 {
		fail("lit_stats", fmt.Sprintf("n=%d gain=%v", n, gain))
	} else {
		out["lit_stats_n"] = n
	}

	out["probe_ok"] = ok
	return out, ok
}

// runPixelAssertions checks one grade picture: enough pixels moved vs
// night, person lifted, lee back at night, night never black.
func runPixelAssertions(night, img *light.Image) (movedPx int, movedFrac, personGain, leeDiff, nightMin float64, ok bool) {
	totalPx := night.W * night.H
	for i := range night.Pix {
		if img.Pix[i] != night.Pix[i] {
			movedPx++
		}
	}
	movedFrac = float64(movedPx) / float64(totalPx)
	movedOK := movedPx >= litMinPx && movedPx < totalPx
	px, py := night.W/2, int(0.6*float64(night.H))
	pi := py*night.W + px
	personGain = luminance(img.Pix[pi]) - luminance(night.Pix[pi])
	personOK := personGain > personGainMin
	bx, by := int(0.545*float64(night.W)), int(0.6*float64(night.H))
	bi := by*night.W + bx
	leeDiff = math.Abs(luminance(img.Pix[bi]) - luminance(night.Pix[bi]))
	leeOK := img.Pix[bi] == night.Pix[bi] && leeDiff == 0
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

// checkGolden compares one grade picture against its frozen PNG with zero
// tolerance; the first run stores the baseline and reports first=true.
func checkGolden(mode light.SoftMode, img *light.Image) (diffPct float64, totalPx int64, firstRun bool, ok bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, gradeFile(mode))
	cur := imgToRGBA(img)
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game_light2d %s: golden store %s: %v\n", mode, basePath, err)
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "game_light2d %s: golden baseline stored: %s\n", mode, basePath)
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_light2d %s: golden open: %v\n", mode, err)
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_light2d %s: golden decode: %v\n", mode, err)
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		fmt.Fprintf(os.Stderr, "game_light2d %s: golden size %v vs %v\n", mode, want.Bounds(), cur.Bounds())
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
	return diffPct, total, false, diff == 0
}

func runProbes() probeReport {
	var rep probeReport
	rep.LogicMap = map[string]any{}

	smallBase := mustImage(goldenW, goldenH, makeBaseColors(goldenW, goldenH), makeLayers(goldenW, goldenH))
	nightSc := mustNightScene()
	nightImg, err := nightSc.Apply(smallBase)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: night Apply:", err)
		os.Exit(1)
	}
	lamps := mustLamps(0.5*float64(goldenW), 0.6*float64(goldenH))

	logicMap, logicOK := runLogicProbes(smallBase, nightImg, lamps, goldenW, goldenH)
	rep.LogicMap = logicMap
	rep.LogicOK = logicOK

	night := nightSc.Night
	for _, m := range gradeOrder {
		soft := mustSoft(goldenW, goldenH, m)
		img, bill, err := soft.Apply(smallBase, lamps, night)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: soft %s Apply: %v\n", m, err)
			os.Exit(1)
		}
		ag, _, err := soft.ApplyGPU(smallBase, lamps, night)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: soft %s ApplyGPU: %v\n", m, err)
			os.Exit(1)
		}
		ev := gradeEvidence{Mode: m, Img: img, Bill: bill, MirrorOK: img.ApproxEqual(ag, 0)}
		ev.MovedPx, ev.MovedFrac, ev.PersonGain, ev.LeeDiff, _, ev.PixelOK = runPixelAssertions(nightImg, img)
		ev.GoldenDiff, ev.GoldenTotal, ev.GoldenFirst, ev.GoldenOK = checkGolden(m, img)
		rep.Grades = append(rep.Grades, ev)
	}

	rep.PixelOK = true
	for _, g := range rep.Grades {
		if !g.PixelOK || !g.MirrorOK {
			rep.PixelOK = false
		}
	}
	// The three grades must be visibly distinct (softening stays local).
	total := float64(goldenW * goldenH)
	if len(rep.Grades) == 3 {
		d01 := diffExactCount(rep.Grades[0].Img, rep.Grades[1].Img)
		d02 := diffExactCount(rep.Grades[0].Img, rep.Grades[2].Img)
		d12 := diffExactCount(rep.Grades[1].Img, rep.Grades[2].Img)
		rep.LogicMap["grade_diff_none_pcf5"] = d01
		rep.LogicMap["grade_diff_none_pcf13"] = d02
		rep.LogicMap["grade_diff_pcf5_pcf13"] = d12
		if d01 <= 0 || d02 <= 0 || d12 <= 0 ||
			float64(d01)/total > maxSoftFrac || float64(d02)/total > maxSoftFrac {
			rep.PixelOK = false
		}
	}
	rep.GoldenOK = true
	for _, g := range rep.Grades {
		if !g.GoldenOK {
			rep.GoldenOK = false
		}
	}
	rep.OffIsDark, _ = rep.LogicMap["off_is_night"].(bool)
	rep.OK = rep.LogicOK && rep.PixelOK && rep.GoldenOK && rep.OffIsDark
	return rep
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

// paintCard draws one grade card: frozen lit picture, title names the
// grade and taps, the live grade gets the bright frame.
func paintCard(pc *rendering.PaintContext, title string, img *render.ImageBuf, current bool, foot string) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGBA(0.13, 0.14, 0.17, 1)
	pc.DC.DrawRectangle(ax, ay, cardW, cardH)
	_ = pc.DC.Fill()
	if current {
		pc.DC.SetRGBA(1.0, 0.80, 0.40, 1)
	} else {
		pc.DC.SetRGBA(0.35, 0.55, 0.75, 1)
	}
	pc.DC.SetLineWidth(2)
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
	if face := wrkit.FaceAt(11); face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(0.70, 0.78, 0.88, 1)
	pc.DC.DrawString(foot, ax+12, ay+262)
}

func failJSON(rep probeReport) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   0,
		"pass":       false,
		"logic_ok":   rep.LogicOK,
		"pixel_ok":   rep.PixelOK,
		"golden_ok":  rep.GoldenOK,
		"case":       "shadow",
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func main() {
	caseFlag := flag.String("case", "shadow", "scenario case (only shadow)")
	autoOnly := flag.Bool("auto-only", false, "selftest + short real window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()
	if *caseFlag != "shadow" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want shadow (only shadow gate)\n", *caseFlag)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	rep := runProbes()
	for _, g := range rep.Grades {
		fmt.Fprintf(os.Stderr, "game_light2d: grade=%s taps=%d mirror=%v moved=%d gain=%.4f lee=%.6f golden=%.4f%%(first=%v) pixel=%v\n",
			g.Mode, g.Mode.Taps(), g.MirrorOK, g.MovedPx, g.PersonGain, g.LeeDiff, g.GoldenDiff, g.GoldenFirst, g.PixelOK)
	}
	fmt.Fprintf(os.Stderr, "game_light2d: probes ok=%v logic=%v pixel=%v golden=%v off_is_night=%v\n",
		rep.OK, rep.LogicOK, rep.PixelOK, rep.GoldenOK, rep.OffIsDark)
	if !rep.OK {
		if *autoOnly {
			failJSON(rep)
		} else {
			fmt.Fprintln(os.Stderr, "game_light2d: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		if !secsSet {
			secs = 8
			secsSet = true
		}
		wrkit.RequireMinRun(secs, abilityID)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
		secsSet = true
	}
	manualMode := !*autoOnly
	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	// Frozen window pictures: full-size grades plus the night reference.
	// The dark switch only flips which buffer each card shows.
	fullBase := mustImage(srcW, srcH, makeBaseColors(srcW, srcH), makeLayers(srcW, srcH))
	nightSc := mustNightScene()
	nightFull, err := nightSc.Apply(fullBase)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: night Apply:", err)
		os.Exit(1)
	}
	fullLamps := mustLamps(0.5*float64(srcW), 0.6*float64(srcH))
	onBufs := make([]*render.ImageBuf, len(gradeOrder))
	for i, m := range gradeOrder {
		img, _, err := mustSoft(srcW, srcH, m).Apply(fullBase, fullLamps, nightSc.Night)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: window soft %s Apply: %v\n", m, err)
			os.Exit(1)
		}
		onBufs[i] = bufFromColors(img.W, img.H, img.Pix)
	}
	offBuf := bufFromColors(nightFull.W, nightFull.H, nightFull.Pix)

	live := &struct {
		lightOn  bool
		gradeIdx int
	}{lightOn: true, gradeIdx: 1} // PCF5 default, mirrors DefaultSoftMode.

	shell := wrkit.NewShell(winW, winH, "game_light2d 灯影对照 — 开关＋三档柔边 (light2d-shadow)", []string{
		"点光＋方向光 双灯",
		"竖墙压人右半＝影子",
		"点击/按L 开关灯",
		"1/2/3或空格 切三档",
		"None硬边 PCF柔边",
		"黄框＝当前档",
		"JSON见 ability_extra",
	})

	gradeTitles := []string{"None·1tap 硬边", "PCF5·5tap 默认", "PCF13·13tap 最柔"}
	gradeFeet := []string{"硬边无过渡", "十字五点平均", "十三点环形平均"}
	boxes := make([]*rendering.RenderBox, len(gradeOrder))
	for i := range gradeOrder {
		i := i
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			img := onBufs[i]
			state := "ON"
			if !live.lightOn {
				img = offBuf
				state = "OFF"
			}
			paintCard(pc, fmt.Sprintf("%s %s", state, gradeTitles[i]), img, i == live.gradeIdx, gradeFeet[i])
		}
		boxes[i] = box
		shell.Body.Place(box, 20+float64(i)*292, 20)
	}
	note := wrkit.Label("左None/中PCF5/右PCF13, 人右半影子=夜色, 点击开关灯, 数字键切档", 12, 0.75, 0.82, 0.9)
	shell.Body.Place(note, 20, 340)
	chain := wrkit.Label("engine/light 真包: Scene夜色＋双灯, Shadow竖墙, SoftShadow三档盖在其上", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(chain, 20, 362)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()

	var summary manualSummary
	elapsed := 0.0
	repaint := func() {
		for _, b := range boxes {
			b.MarkNeedsPaint()
		}
	}
	setTitle := func() {
		ctl := win.Controls()
		if ctl == nil {
			return
		}
		on := "ON"
		if !live.lightOn {
			on = "OFF"
		}
		ctl.SetTitle(fmt.Sprintf("%s 灯=%s 档=%s 事件=%d", winTitle, on,
			gradeOrder[live.gradeIdx], summary.Pointer+summary.Key+summary.Resize))
	}
	switchGrade := func(idx int, why string) {
		if idx < 0 || idx >= len(gradeOrder) || idx == live.gradeIdx {
			return
		}
		live.gradeIdx = idx
		summary.GradeChanges++
		fmt.Fprintf(os.Stderr, "game_light2d: grade -> %s (%s)\n", gradeOrder[idx], why)
		repaint()
		setTitle()
	}
	toggleLight := func(why string) {
		live.lightOn = !live.lightOn
		summary.Toggles++
		fmt.Fprintf(os.Stderr, "game_light2d: light -> %v (%s)\n", live.lightOn, why)
		repaint()
		setTitle()
	}

	_ = os.MkdirAll(testdataDir, 0o755)
	snapPath := filepath.Join(testdataDir, "light2d_final.png")

	app := embedder.NewPipelineApp(win.Host(), shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_light2d: close (%s)\n", win.Backend())
				return
			case platform.EventResize:
				summary.Resize++
				fmt.Fprintf(os.Stderr, "game_light2d: resize %dx%d\n", ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				if manualMode {
					setTitle()
				}
				return
			case platform.EventPointer:
				if ev.Pointer != platform.PointerDown {
					return
				}
				summary.Pointer++
				toggleLight(fmt.Sprintf("pointer @(%.0f,%.0f)", ev.X, ev.Y))
				return
			case platform.EventKey:
				if !ev.Pressed {
					return
				}
				summary.Key++
				switch ev.Rune {
				case '1':
					switchGrade(0, "key 1")
				case '2':
					switchGrade(1, "key 2")
				case '3':
					switchGrade(2, "key 3")
				case ' ':
					switchGrade((live.gradeIdx+1)%len(gradeOrder), "key space")
				case 'l', 'L', 'd', 'D':
					toggleLight(fmt.Sprintf("key %q", string(ev.Rune)))
				default:
					fmt.Fprintf(os.Stderr, "game_light2d: key code=%d rune=%q\n", ev.KeyCode, string(ev.Rune))
					if manualMode {
						setTitle()
					}
				}
				return
			default:
				return
			}
		},
	})

	var proc scheduler.ProcessTracker
	proc.Start()
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		repaint()
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && rep.OK
		shell.UpdateHUD(abilityID, "Steady", app, gateOK,
			fmt.Sprintf("presents=%d grade=%s", app.PresentCount(), gradeOrder[live.gradeIdx]),
			fmt.Sprintf("light=%v toggles=%d grades=%d", live.lightOn, summary.Toggles, summary.GradeChanges))
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
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	presents := app.PresentCount()

	goldenRects := []wrsoak.Rect{
		{X: 0, Y: 0, W: winW, H: 48},
		{X: 12, Y: 60, W: 260, H: 656},
		{X: 284 + 20 + 20, Y: 60 + 20 + 40, W: imgW, H: imgH},
		{X: 284 + 312 + 20, Y: 60 + 20 + 40, W: imgW, H: imgH},
		{X: 284 + 604 + 20, Y: 60 + 20 + 40, W: imgW, H: imgH},
	}
	winGoldenDiff, winGoldenTotal, winGoldenFirst := wrsoak.EvaluateGolden("game_light2d", testdataDir, "light2d_final.png", "light2d_final_base.png", goldenRects, winW)

	probeOK := 0
	if rep.OK {
		probeOK = 1
	}
	extra := map[string]any{
		"probe_ok":         probeOK,
		"logic_ok":         rep.LogicOK,
		"pixel_ok":         rep.PixelOK,
		"golden_ok":        rep.GoldenOK,
		"off_is_night":     rep.OffIsDark,
		"win_golden_diff":  winGoldenDiff,
		"win_golden_total": winGoldenTotal,
		"win_golden_first": winGoldenFirst,
		"final_light_on":   live.lightOn,
		"final_grade":      gradeOrder[live.gradeIdx].String(),
		"toggles":          summary.Toggles,
		"grade_changes":    summary.GradeChanges,
		"pointer_events":   summary.Pointer,
		"key_events":       summary.Key,
		"resize_events":    summary.Resize,
		"manual_timed":     secsSet,
		"case":             "shadow",
		"grade_diff_pcf5":  rep.LogicMap["grade_diff_none_pcf5"],
		"grade_diff_pcf13": rep.LogicMap["grade_diff_none_pcf13"],
		"grade_diff_5_13":  rep.LogicMap["grade_diff_pcf5_pcf13"],
		"lit_stats_n":      rep.LogicMap["lit_stats_n"],
		"covered":          "左None/中PCF5/右PCF13, 人右半影子=夜色, 开关灯+切三档",
		"impl_correctness": "Scene夜色+双灯之后盖Shadow竖墙, SoftShadow三档只改柔边",
		"impl_visible":     "HUD实时presents/grade/light, 关窗/超时出JSON",
	}
	for i, g := range rep.Grades {
		p := "grade" + string(rune('0'+i)) + "_"
		extra[p+"mode"] = g.Mode.String()
		extra[p+"taps"] = g.Mode.Taps()
		extra[p+"moved_px"] = g.MovedPx
		extra[p+"person_gain"] = g.PersonGain
		extra[p+"lee_diff"] = g.LeeDiff
		extra[p+"mirror_ok"] = g.MirrorOK
		extra[p+"golden_diff"] = g.GoldenDiff
		extra[p+"golden_total"] = g.GoldenTotal
		extra[p+"golden_first"] = g.GoldenFirst
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     abilityID,
		Scenario:      scenario,
		Snap:          snap,
		PresentCount:  presents,
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
		if presents < 1 || !rep.OK {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v (want >=1, true)\n", presents, rep.OK)
			os.Exit(1)
		}
		if !winGoldenFirst && winGoldenDiff != goldenTolPct {
			fmt.Fprintf(os.Stderr, "FAIL: win_golden_diff=%.4f want %.1f over %d px\n", winGoldenDiff, goldenTolPct, winGoldenTotal)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_light2d: OK case=shadow presents=%d grades=3 win_golden=%.4f%% elapsed=%.1fs\n",
			presents, winGoldenDiff, elapsedSec)
		return
	}
	summary.Timed = secsSet
	fmt.Fprintf(os.Stderr, "game_light2d: case=shadow backend=%s presents=%d light=%v grade=%s toggles=%d grades=%d elapsed=%.1fs ptr=%d key=%d rs=%d\n",
		win.Backend(), presents, live.lightOn, gradeOrder[live.gradeIdx], summary.Toggles, summary.GradeChanges, elapsedSec, summary.Pointer, summary.Key, summary.Resize)
}
