//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package light

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/engine/core"
)

type softBudgetDef struct {
	MaxShadowLights int     `json:"maxShadowLights"`
	MaxPCF13Lights  int     `json:"maxPCF13Lights"`
	LargePixelsWarn int     `json:"largePixelsWarn"`
	MinNight        float64 `json:"minNight"`
	MinLitPixels    int     `json:"minLitPixels"`
	MinLitGain      float64 `json:"minLitGain"`
}

type softModeDef struct {
	Name     string `json:"name"`
	Mode     string `json:"mode"`
	WantTaps int    `json:"wantTaps"`
}

type softLightDef struct {
	Kind      string    `json:"kind"`
	Pos       []float64 `json:"pos"`
	Dir       []float64 `json:"dir"`
	Range     float64   `json:"range"`
	Color     []float64 `json:"color"`
	Intensity float64   `json:"intensity"`
	Layers    uint32    `json:"layers"`
}

type softFactorDef struct {
	Name         string         `json:"name"`
	Kind         string         `json:"kind"`
	Pos          []float64      `json:"pos"`
	Dir          []float64      `json:"dir"`
	Range        float64        `json:"range"`
	Color        []float64      `json:"color"`
	Intensity    float64        `json:"intensity"`
	Layers       uint32         `json:"layers"`
	Occluders    [][][2]float64 `json:"occluders"`
	Occlusion    float64        `json:"occlusion"`
	Length       float64        `json:"length"`
	P            []float64      `json:"p"`
	Radius       float64        `json:"radius"`
	Mode         string         `json:"mode"`
	WantMin      float64        `json:"wantMin"`
	WantMax      float64        `json:"wantMax"`
	StrictInside bool           `json:"strictInside"`
}

type softBillDef struct {
	Name       string       `json:"name"`
	W          int          `json:"w"`
	H          int          `json:"h"`
	OX         float64      `json:"ox"`
	OY         float64      `json:"oy"`
	Step       float64      `json:"step"`
	Layers     []int        `json:"layers"`
	FillLayer  *int         `json:"fillLayer"`
	Light      softLightDef `json:"light"`
	WantPixels int          `json:"wantPixels"`
	WantWarn   bool         `json:"wantWarn"`
}

type softNightDef struct {
	Name      string    `json:"name"`
	Night     []float64 `json:"night"`
	WantErr   bool      `json:"wantErr"`
	WantFloor []float64 `json:"wantFloor"`
}

type softDownDef struct {
	Name        string `json:"name"`
	From        string `json:"from"`
	WantTo      string `json:"wantTo"`
	WantChanged bool   `json:"wantChanged"`
}

type softTorchDef struct {
	Name   string       `json:"name"`
	W      int          `json:"w"`
	H      int          `json:"h"`
	OX     float64      `json:"ox"`
	OY     float64      `json:"oy"`
	Step   float64      `json:"step"`
	Src    [][]float64  `json:"src"`
	Layers []int        `json:"layers"`
	Night  []float64    `json:"night"`
	Light  softLightDef `json:"light"`
}

type softGainDef struct {
	Name      string    `json:"name"`
	W         int       `json:"w"`
	H         int       `json:"h"`
	OX        float64   `json:"ox"`
	OY        float64   `json:"oy"`
	Step      float64   `json:"step"`
	Base      []float64 `json:"base"`
	Night     []float64 `json:"night"`
	LightPos  []float64 `json:"lightPos"`
	Range     float64   `json:"range"`
	Color     []float64 `json:"color"`
	Intensity float64   `json:"intensity"`
	Layers    uint32    `json:"layers"`
	MinPixels int       `json:"minPixels"`
	MinGain   float64   `json:"minGain"`
}

type softFile struct {
	Budgets     softBudgetDef   `json:"budgets"`
	DefaultMode string          `json:"defaultMode"`
	Modes       []softModeDef   `json:"modes"`
	SoftFactor  []softFactorDef `json:"softFactor"`
	Billing     []softBillDef   `json:"billing"`
	Night       []softNightDef  `json:"night"`
	Downgrade   []softDownDef   `json:"downgrade"`
	Torch       softTorchDef    `json:"torch"`
	LitGain     softGainDef     `json:"litGain"`
}

func loadSoftFile(t *testing.T) softFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "softshadow_cases.json"))
	if err != nil {
		t.Fatalf("read softshadow_cases.json: %v", err)
	}
	var f softFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode softshadow_cases.json: %v", err)
	}
	if len(f.Modes) == 0 || len(f.SoftFactor) == 0 || len(f.Billing) == 0 {
		t.Fatal("softshadow_cases.json misses a section")
	}
	return f
}

func buildSoftLight(t *testing.T, what string, d softLightDef) Light {
	t.Helper()
	col := core.RGBA(d.Color[0], d.Color[1], d.Color[2], d.Color[3])
	if d.Kind == "directional" {
		l, err := NewDirectionalLight(core.V2(d.Dir[0], d.Dir[1]), col, d.Intensity, d.Layers)
		if err != nil {
			t.Fatalf("%s: NewDirectionalLight: %v", what, err)
		}
		return l
	}
	l, err := NewPointLight(core.V2(d.Pos[0], d.Pos[1]), d.Range, col, d.Intensity, d.Layers, nil)
	if err != nil {
		t.Fatalf("%s: NewPointLight: %v", what, err)
	}
	return l
}

func buildSoftShadow(t *testing.T, c softFactorDef) (SoftShadow, Light) {
	t.Helper()
	l := buildSoftLight(t, c.Name, softLightDef{
		Kind: c.Kind, Pos: c.Pos, Dir: c.Dir, Range: c.Range,
		Color: c.Color, Intensity: c.Intensity, Layers: c.Layers,
	})
	var os []Occluder
	for _, loop := range c.Occluders {
		var pts []core.Vec2
		for _, q := range loop {
			pts = append(pts, core.V2(q[0], q[1]))
		}
		os = append(os, Occluder{Pts: pts})
	}
	sh := Shadow{Occluders: os, Occlusion: c.Occlusion, Length: c.Length}
	mode, err := ParseSoftMode(c.Mode)
	if err != nil {
		t.Fatalf("%s: ParseSoftMode: %v", c.Name, err)
	}
	s, err := NewSoftShadow(sh, mode, c.Radius)
	if err != nil {
		t.Fatalf("%s: NewSoftShadow: %v", c.Name, err)
	}
	return s, l
}

func buildSoftBillImage(t *testing.T, c softBillDef) *Image {
	t.Helper()
	layers := make([]Layer, c.W*c.H)
	if c.FillLayer != nil {
		for i := range layers {
			layers[i] = Layer(*c.FillLayer)
		}
	} else {
		if len(c.Layers) != c.W*c.H {
			t.Fatalf("%s: %d layers, want %d", c.Name, len(c.Layers), c.W*c.H)
		}
		for i, v := range c.Layers {
			layers[i] = Layer(v)
		}
	}
	pix := make([]core.Color, c.W*c.H)
	for i := range pix {
		pix[i] = core.White
	}
	img, err := NewImageFromColors(c.W, c.H, core.V2(c.OX, c.OY), c.Step, pix, layers)
	if err != nil {
		t.Fatalf("%s: NewImageFromColors: %v", c.Name, err)
	}
	return img
}

// A: three grades switch, tap counts match, middle grade is default.
func TestSoftModeGrades(t *testing.T) {
	f := loadSoftFile(t)
	if f.DefaultMode != DefaultSoftMode.String() {
		t.Fatalf("default = %q, want %q", f.DefaultMode, DefaultSoftMode.String())
	}
	if DefaultSoftMode != SoftPCF5 {
		t.Fatalf("DefaultSoftMode = %v, want PCF5", DefaultSoftMode)
	}
	for _, m := range f.Modes {
		got, err := ParseSoftMode(m.Mode)
		if err != nil {
			t.Fatalf("%s: ParseSoftMode: %v", m.Name, err)
		}
		if got.Taps() != m.WantTaps {
			t.Errorf("%s: taps = %d, want %d", m.Name, got.Taps(), m.WantTaps)
		}
		if back := got.String(); back != m.Mode {
			t.Errorf("%s: String = %q, want %q", m.Name, back, m.Mode)
		}
	}
	if SoftNone.Taps() != 1 || SoftPCF5.Taps() != 5 || SoftPCF13.Taps() != 13 {
		t.Fatal("taps != 1/5/13")
	}
	if _, err := ParseSoftMode("PCF9"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("ParseSoftMode PCF9 = %v, want invalid-arg", err)
	}
	if err := SoftMode(99).Validate(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("SoftMode(99) = %v, want invalid-arg", err)
	}
	wall, _ := NewOccluder([]core.Vec2{core.V2(2, -1), core.V2(3, -1), core.V2(3, 1), core.V2(2, 1)})
	sh, _ := NewShadow([]Occluder{wall}, 1, 1000)
	if _, err := NewSoftShadow(sh, SoftMode(99), 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad mode = %v, want invalid-arg", err)
	}
	if _, err := NewSoftShadow(sh, SoftPCF5, -1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad radius = %v, want invalid-arg", err)
	}
	zero, err := NewSoftShadow(sh, SoftPCF5, 0)
	if err != nil {
		t.Fatalf("radius 0: %v", err)
	}
	if zero.Radius != DefaultSoftRadius {
		t.Errorf("radius 0 = %v, want %v", zero.Radius, DefaultSoftRadius)
	}
}

// B: soft taps match the hard core on None, soften edges on PCF.
func TestSoftFactorGrades(t *testing.T) {
	f := loadSoftFile(t)
	for _, c := range f.SoftFactor {
		s, l := buildSoftShadow(t, c)
		p := core.V2(c.P[0], c.P[1])
		got := s.SoftFactor(l, p)
		if math.IsNaN(got) || got < c.WantMin-1e-9 || got > c.WantMax+1e-9 {
			t.Errorf("%s: soft = %v, want in [%v,%v]", c.Name, got, c.WantMin, c.WantMax)
		}
		if c.StrictInside && !(got > 0 && got < 1) {
			t.Errorf("%s: soft = %v, want strictly inside (0,1)", c.Name, got)
		}
		if gpu := s.SoftFactorGPU(l, p); gpu != got {
			t.Errorf("%s: CPU %v vs GPU %v", c.Name, got, gpu)
		}
		if c.Mode == "None" {
			if hard := s.Shadow.FactorFor(l, p); hard != got {
				t.Errorf("%s: None soft %v vs hard %v", c.Name, got, hard)
			}
		}
	}
	// PCF5 and PCF13 soften the same edge point to different means.
	edge5, l5 := buildSoftShadow(t, softEdgeCase(f, "dir_edge_soft5"))
	edge13, l13 := buildSoftShadow(t, softEdgeCase(f, "dir_edge_soft13"))
	p := core.V2(edge5P(f)[0], edge5P(f)[1])
	a, b := edge5.SoftFactor(l5, p), edge13.SoftFactor(l13, p)
	if !(a > 0 && a < 1 && b > 0 && b < 1) {
		t.Fatalf("edge PCF5=%v PCF13=%v, want both inside (0,1)", a, b)
	}
	if a == b {
		t.Errorf("PCF5 == PCF13 == %v, want different means", a)
	}
}

func softEdgeCase(f softFile, name string) softFactorDef {
	for _, c := range f.SoftFactor {
		if c.Name == name {
			return c
		}
	}
	return softFactorDef{}
}

func edge5P(f softFile) []float64 {
	for _, c := range f.SoftFactor {
		if c.Name == "dir_edge_soft5" {
			return c.P
		}
	}
	return []float64{5, 1}
}

// C: billing matches footprint, big lights warn, cost is pixels times taps.
func TestSoftBilling(t *testing.T) {
	f := loadSoftFile(t)
	if MaxShadowLights != f.Budgets.MaxShadowLights ||
		MaxPCF13Lights != f.Budgets.MaxPCF13Lights ||
		LargeLightPixelsWarn != f.Budgets.LargePixelsWarn {
		t.Fatalf("budgets differ from testdata")
	}
	for _, c := range f.Billing {
		img := buildSoftBillImage(t, c)
		l := buildSoftLight(t, c.Name, c.Light)
		if got := CoverPixels(l, img); got != c.WantPixels {
			t.Errorf("%s: cover = %d, want %d", c.Name, got, c.WantPixels)
		}
		for _, mode := range []SoftMode{SoftNone, SoftPCF5, SoftPCF13} {
			wall, _ := NewOccluder([]core.Vec2{core.V2(200, 200), core.V2(201, 200), core.V2(201, 201)})
			sh, _ := NewShadow([]Occluder{wall}, 1, 0)
			s, err := NewSoftShadow(sh, mode, 1)
			if err != nil {
				t.Fatalf("%s: NewSoftShadow: %v", c.Name, err)
			}
			bill := s.Bill(img, []Light{l})
			if len(bill.PerLight) != 1 || bill.PerLight[0] != c.WantPixels {
				t.Errorf("%s %v: bill = %v, want [%d]", c.Name, mode, bill.PerLight, c.WantPixels)
			}
			if bill.Total != c.WantPixels {
				t.Errorf("%s %v: total = %d, want %d", c.Name, mode, bill.Total, c.WantPixels)
			}
			if bill.Taps != mode.Taps() {
				t.Errorf("%s %v: taps = %d, want %d", c.Name, mode, bill.Taps, mode.Taps())
			}
			if bill.Cost != bill.Total*bill.Taps {
				t.Errorf("%s %v: cost = %d, want total*taps", c.Name, mode, bill.Cost)
			}
			warn, idx := bill.WarnLarge()
			if warn != c.WantWarn {
				t.Errorf("%s %v: warn = %v, want %v", c.Name, mode, warn, c.WantWarn)
			}
			if warn && idx != 0 {
				t.Errorf("%s: warn idx = %d, want 0", c.Name, idx)
			}
			if !warn && idx != -1 {
				t.Errorf("%s: no-warn idx = %d, want -1", c.Name, idx)
			}
		}
	}
	// Two-light ledger adds up per light.
	img := buildSoftBillImage(t, f.Billing[1])
	l := buildSoftLight(t, "pair", f.Billing[1].Light)
	wall, _ := NewOccluder([]core.Vec2{core.V2(200, 200), core.V2(201, 200), core.V2(201, 201)})
	sh, _ := NewShadow([]Occluder{wall}, 1, 0)
	s, _ := NewSoftShadow(sh, SoftPCF5, 1)
	bill := s.Bill(img, []Light{l, l})
	if len(bill.PerLight) != 2 || bill.Total != bill.PerLight[0]+bill.PerLight[1] {
		t.Fatalf("pair bill = %+v, want per-light sum", bill)
	}
	if bill.Cost != bill.Total*5 {
		t.Fatalf("pair cost = %d, want total*5", bill.Cost)
	}
	// Counts gate the frame: 10 pass, 11 warn; PCF13 only a few.
	if OverShadowLightCount(10) || !OverShadowLightCount(11) {
		t.Error("shadow count gate wrong at 10/11")
	}
	pcf13, _ := NewSoftShadow(sh, SoftPCF13, 1)
	if pcf13.OverPCF13Count(2) || !pcf13.OverPCF13Count(3) {
		t.Error("PCF13 count gate wrong at 2/3")
	}
	plain, _ := NewSoftShadow(sh, SoftNone, 1)
	if plain.OverPCF13Count(100) {
		t.Error("None must never trip the PCF13 count")
	}
	if CoverPixels(l, nil) != 0 {
		t.Error("nil image cover != 0")
	}
}

// D: night floor never blacks, downgrade always logs, silence is red.
func TestSoftNightAndDowngrade(t *testing.T) {
	f := loadSoftFile(t)
	if MinNightChannel != f.Budgets.MinNight {
		t.Fatalf("MinNightChannel = %v, want %v", MinNightChannel, f.Budgets.MinNight)
	}
	for _, c := range f.Night {
		n := core.RGBA(c.Night[0], c.Night[1], c.Night[2], c.Night[3])
		err := CheckNightFloor(n)
		if (err != nil) != c.WantErr {
			t.Errorf("%s: CheckNightFloor err = %v, wantErr %v", c.Name, err, c.WantErr)
		}
		if c.WantErr && core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("%s: code = %v, want invalid-arg", c.Name, err)
		}
		got := ClampNightFloor(n)
		want := core.RGBA(c.WantFloor[0], c.WantFloor[1], c.WantFloor[2], c.WantFloor[3])
		if got != want {
			t.Errorf("%s: floor = %v, want %v", c.Name, got, want)
		}
		if got.R < MinNightChannel || got.G < MinNightChannel || got.B < MinNightChannel {
			t.Errorf("%s: floor %v below 0.03", c.Name, got)
		}
	}
	if got := ClampNightFloor(core.Color{R: math.NaN()}); got != core.White {
		t.Errorf("invalid night floor = %v, want white (never black)", got)
	}
	wall, _ := NewOccluder([]core.Vec2{core.V2(2, -1), core.V2(3, -1), core.V2(3, 1), core.V2(2, 1)})
	sh, _ := NewShadow([]Occluder{wall}, 1, 1000)
	for _, c := range f.Downgrade {
		from, err := ParseSoftMode(c.From)
		if err != nil {
			t.Fatalf("%s: ParseSoftMode: %v", c.Name, err)
		}
		s, err := NewSoftShadow(sh, from, 1)
		if err != nil {
			t.Fatalf("%s: NewSoftShadow: %v", c.Name, err)
		}
		var log []string
		changed := s.DowngradeOOM(&log, c.Name)
		if changed != c.WantChanged {
			t.Errorf("%s: changed = %v, want %v", c.Name, changed, c.WantChanged)
		}
		if s.Mode.String() != c.WantTo {
			t.Errorf("%s: mode = %v, want %v", c.Name, s.Mode, c.WantTo)
		}
		if len(log) == 0 {
			t.Errorf("%s: no log line (silent downgrade)", c.Name)
		} else if !strings.Contains(log[0], "softshadow downgrade") {
			t.Errorf("%s: log %q misses downgrade note", c.Name, log[0])
		}
	}
	// OOM error downgrades with a log; other codes never downgrade.
	s, _ := NewSoftShadow(sh, SoftPCF13, 1)
	var log []string
	if !s.DowngradeOnAllocFail(&log, core.OutOfMemory("alloc", "shadow")) {
		t.Fatal("OOM did not downgrade")
	}
	if s.Mode != SoftPCF5 || len(log) == 0 {
		t.Fatalf("OOM downgrade = %v log %v, want PCF5 + log", s.Mode, log)
	}
	before := s.Mode
	if s.DowngradeOnAllocFail(&log, core.InvalidArg("alloc", "shadow")) {
		t.Error("non-OOM must not downgrade")
	}
	if s.Mode != before {
		t.Error("non-OOM changed the grade")
	}
	if s.DowngradeOnAllocFail(&log, nil) {
		t.Error("nil error must not downgrade")
	}
	// Nil receiver and nil log refuse instead of silently dropping.
	var nilSoft *SoftShadow
	if nilSoft.DowngradeOOM(&log, "x") {
		t.Error("nil soft downgrade = true, want false")
	}
	if s.DowngradeOOM(nil, "x") {
		t.Error("nil log downgrade = true, want false (no silent drop)")
	}
	// Count-driven auto downgrade logs too.
	auto, _ := NewSoftShadow(sh, SoftPCF13, 1)
	var alog []string
	if !auto.AutoDowngradeForLights(&alog, 3) || len(alog) == 0 {
		t.Error("PCF13 x3 must auto downgrade with a log")
	}
	keep, _ := NewSoftShadow(sh, SoftPCF13, 1)
	var klog []string
	if keep.AutoDowngradeForLights(&klog, 2) {
		t.Error("PCF13 x2 must stay")
	}
	many, _ := NewSoftShadow(sh, SoftPCF5, 1)
	var mlog []string
	if !many.AutoDowngradeForLights(&mlog, 11) || len(mlog) == 0 {
		t.Error("11 lights must auto downgrade with a log")
	}
	none, _ := NewSoftShadow(sh, SoftNone, 1)
	var nlog []string
	if none.AutoDowngradeForLights(&nlog, 11) {
		t.Error("None must not downgrade further")
	}
}

// E: torch lights the person, background stays on night, floor holds,
// thousand-pixel scenes gain above 0.6.
func TestSoftTorchAndGain(t *testing.T) {
	f := loadSoftFile(t)
	tc := f.Torch
	pix := make([]core.Color, len(tc.Src))
	for i, v := range tc.Src {
		pix[i] = core.RGBA(v[0], v[1], v[2], v[3])
	}
	layers := make([]Layer, len(tc.Layers))
	for i, v := range tc.Layers {
		layers[i] = Layer(v)
	}
	img, err := NewImageFromColors(tc.W, tc.H, core.V2(tc.OX, tc.OY), tc.Step, pix, layers)
	if err != nil {
		t.Fatalf("torch image: %v", err)
	}
	l := buildSoftLight(t, tc.Name, tc.Light)
	night := core.RGBA(tc.Night[0], tc.Night[1], tc.Night[2], tc.Night[3])
	open, err := NewShadow(nil, 1, 1000)
	if err != nil {
		t.Fatalf("NewShadow: %v", err)
	}
	s, err := NewSoftShadow(open, DefaultSoftMode, 1)
	if err != nil {
		t.Fatalf("NewSoftShadow: %v", err)
	}
	got, bill, err := s.Apply(img, []Light{l}, night)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if bill.Total == 0 {
		t.Fatal("torch bill is zero, want lit footprint")
	}
	// Background pixels sit exactly on night (torch misses bg).
	for i, want := range [][]int{{0, 0}, {2, 0}} {
		_ = want
		_ = i
	}
	person := []int{0, 2}
	bg := []int{1, 3}
	for _, i := range person {
		if !(got.Pix[i].R > got.Pix[1].R && got.Pix[i].R > night.R) {
			t.Errorf("person pixel %d = %v, want above bg %v", i, got.Pix[i], got.Pix[1])
		}
	}
	for _, i := range bg {
		base, _, _ := img.At(i%tc.W, i/tc.W)
		if want := NightOnlyPixel(base, night); got.Pix[i] != want {
			t.Errorf("bg pixel %d = %v, want night-only %v", i, got.Pix[i], want)
		}
	}
	// Night floor holds on every output channel triple.
	for i, p := range got.Pix {
		if p.R < MinNightChannel || p.G < MinNightChannel || p.B < MinNightChannel {
			t.Fatalf("pixel %d = %v below 0.03 floor", i, p)
		}
	}

	// Thousand-pixel gain scene.
	g := f.LitGain
	if MinLitPixels != g.MinPixels || MinLitGain != g.MinGain {
		t.Fatalf("gain gates differ from testdata")
	}
	spix := make([]core.Color, g.W*g.H)
	slay := make([]Layer, g.W*g.H)
	for i := range spix {
		spix[i] = core.RGBA(g.Base[0], g.Base[1], g.Base[2], g.Base[3])
		slay[i] = LayerWorld
	}
	src, err := NewImageFromColors(g.W, g.H, core.V2(g.OX, g.OY), g.Step, spix, slay)
	if err != nil {
		t.Fatalf("gain image: %v", err)
	}
	gl, err := NewPointLight(core.V2(g.LightPos[0], g.LightPos[1]), g.Range,
		core.RGBA(g.Color[0], g.Color[1], g.Color[2], g.Color[3]), g.Intensity, g.Layers, nil)
	if err != nil {
		t.Fatalf("gain light: %v", err)
	}
	gnight := core.RGBA(g.Night[0], g.Night[1], g.Night[2], g.Night[3])
	gs, err := NewSoftShadow(open, DefaultSoftMode, 1)
	if err != nil {
		t.Fatalf("gain soft: %v", err)
	}
	lit, _, err := gs.Apply(src, []Light{gl}, gnight)
	if err != nil {
		t.Fatalf("gain Apply: %v", err)
	}
	count, mean := LitStats(src, lit, gnight)
	if count < g.MinPixels {
		t.Fatalf("lit pixels = %d, want >= %d", count, g.MinPixels)
	}
	if mean < g.MinGain {
		t.Fatalf("mean gain = %v, want >= %v", mean, g.MinGain)
	}
	if n, m := LitStats(nil, lit, gnight); n != 0 || m != 0 {
		t.Errorf("nil LitStats = (%d,%v), want (0,0)", n, m)
	}
}

// F: CPU/GPU agree, inputs frozen, bad inputs keep their codes.
func softBillEqual(a, b FrameBill) bool {
	if a.Total != b.Total || a.Taps != b.Taps || a.Cost != b.Cost {
		return false
	}
	if len(a.PerLight) != len(b.PerLight) {
		return false
	}
	for i := range a.PerLight {
		if a.PerLight[i] != b.PerLight[i] {
			return false
		}
	}
	return true
}

func TestSoftApplyMirror(t *testing.T) {
	f := loadSoftFile(t)
	tc := f.Torch
	pix := make([]core.Color, len(tc.Src))
	for i, v := range tc.Src {
		pix[i] = core.RGBA(v[0], v[1], v[2], v[3])
	}
	layers := make([]Layer, len(tc.Layers))
	for i, v := range tc.Layers {
		layers[i] = Layer(v)
	}
	mk := func() (*Image, []Light, core.Color, SoftShadow) {
		img, err := NewImageFromColors(tc.W, tc.H, core.V2(tc.OX, tc.OY), tc.Step, pix, layers)
		if err != nil {
			t.Fatalf("image: %v", err)
		}
		l := buildSoftLight(t, tc.Name, tc.Light)
		night := core.RGBA(tc.Night[0], tc.Night[1], tc.Night[2], tc.Night[3])
		open, _ := NewShadow(nil, 1, 1000)
		s, _ := NewSoftShadow(open, SoftPCF5, 1)
		return img, []Light{l}, night, s
	}
	imgA, lsA, nA, sA := mk()
	snapPix := append([]core.Color(nil), imgA.Pix...)
	snapLay := append([]Layer(nil), imgA.Layers...)
	a, ba, err := sA.ApplyCPU(imgA, lsA, nA)
	if err != nil {
		t.Fatalf("ApplyCPU: %v", err)
	}
	imgB, lsB, nB, sB := mk()
	b, bb, err := sB.ApplyGPU(imgB, lsB, nB)
	if err != nil {
		t.Fatalf("ApplyGPU: %v", err)
	}
	if !a.ApproxEqual(b, 0) {
		t.Fatal("CPU/GPU diverged")
	}
	if !softBillEqual(ba, bb) {
		t.Fatalf("bills differ: %+v vs %+v", ba, bb)
	}
	for i := range snapPix {
		if imgA.Pix[i] != snapPix[i] || imgA.Layers[i] != snapLay[i] {
			t.Fatalf("input mutated at pixel %d", i)
		}
	}
	a.Pix[0] = core.Magenta
	if imgA.Pix[0] == core.Magenta {
		t.Fatal("output aliases input")
	}
	if b.Pix[0] == core.Magenta {
		t.Fatal("outputs share storage")
	}

	// Bad inputs keep frozen codes.
	pl, _ := NewPointLight(core.V2(0, 0), 10, core.White, 1, LayersFor(LayerWorld), nil)
	var nilImg *Image
	if _, _, err := sA.Apply(nilImg, []Light{pl}, nA); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Apply = %v, want invalid-arg", err)
	}
	bad := &Image{W: 2, H: 2, Origin: core.V2(0, 0), Step: 1, Pix: make([]core.Color, 3), Layers: make([]Layer, 3)}
	if _, _, err := sA.Apply(bad, []Light{pl}, nA); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("corrupt Apply = %v, want bad-data", err)
	}
	many := make([]Light, MaxLights+1)
	for i := range many {
		many[i] = pl
	}
	if _, _, err := sA.Apply(imgB, many, nA); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("too many lights = %v, want invalid-arg", err)
	}
	raw := SoftShadow{Mode: SoftMode(99)}
	if _, _, err := raw.Apply(imgB, []Light{pl}, nA); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad mode Apply = %v, want invalid-arg", err)
	}
	// Soft pixel mirrors agree too.
	if x, y := sA.SoftLit(core.White, core.V2(10, 10), LayerWorld, []Light{pl}, nA),
		sA.SoftLitGPU(core.White, core.V2(10, 10), LayerWorld, []Light{pl}, nA); x != y {
		t.Fatalf("SoftLit CPU %v vs GPU %v", x, y)
	}
}
