package core

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	epsFloat = 1e-12
	epsGeom  = 1e-9
)

func loadCases(t *testing.T, name string, v any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode testdata/%s: %v", name, err)
	}
}

func close2(got Vec2, want [2]float64, eps float64) bool {
	return math.Abs(got.X-want[0]) < eps && math.Abs(got.Y-want[1]) < eps
}

func close4(got Color, want [4]float64, eps float64) bool {
	return math.Abs(got.R-want[0]) < eps && math.Abs(got.G-want[1]) < eps &&
		math.Abs(got.B-want[2]) < eps && math.Abs(got.A-want[3]) < eps
}

// A: vec arithmetic matches the frozen numbers.
func TestVecFromCases(t *testing.T) {
	var c struct {
		Add []struct{ A, B, Want [2]float64 } `json:"add"`
		Sub []struct{ A, B, Want [2]float64 } `json:"sub"`
		Mul []struct {
			A, Want [2]float64
			S       float64
		} `json:"mul"`
		Dot []struct {
			A, B [2]float64
			Want float64
		} `json:"dot"`
		Cross []struct {
			A, B [2]float64
			Want float64
		} `json:"cross"`
		Length []struct {
			V    [2]float64
			Want float64
		} `json:"length"`
		LengthSq []struct {
			V    [2]float64
			Want float64
		} `json:"lengthsq"`
		Normalize []struct{ V, Want [2]float64 } `json:"normalize"`
		Lerp      []struct {
			A, B, Want [2]float64
			T          float64
		} `json:"lerp"`
		Rotate []struct {
			V, Want [2]float64
			Rad     float64
		} `json:"rotate"`
		Angle []struct {
			A, B [2]float64
			Want float64
		} `json:"angle"`
	}
	loadCases(t, "vec_cases.json", &c)

	for i, k := range c.Add {
		if got := V2(k.A[0], k.A[1]).Add(V2(k.B[0], k.B[1])); !close2(got, k.Want, epsGeom) {
			t.Errorf("add[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Sub {
		if got := V2(k.A[0], k.A[1]).Sub(V2(k.B[0], k.B[1])); !close2(got, k.Want, epsGeom) {
			t.Errorf("sub[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Mul {
		if got := V2(k.A[0], k.A[1]).Mul(k.S); !close2(got, k.Want, epsGeom) {
			t.Errorf("mul[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Dot {
		if got := V2(k.A[0], k.A[1]).Dot(V2(k.B[0], k.B[1])); math.Abs(got-k.Want) >= epsGeom {
			t.Errorf("dot[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Cross {
		if got := V2(k.A[0], k.A[1]).Cross(V2(k.B[0], k.B[1])); math.Abs(got-k.Want) >= epsGeom {
			t.Errorf("cross[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Length {
		if got := V2(k.V[0], k.V[1]).Length(); math.Abs(got-k.Want) >= epsGeom {
			t.Errorf("length[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.LengthSq {
		if got := V2(k.V[0], k.V[1]).LengthSq(); math.Abs(got-k.Want) >= epsGeom {
			t.Errorf("lengthsq[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Normalize {
		if got := V2(k.V[0], k.V[1]).Normalize(); !close2(got, k.Want, epsGeom) {
			t.Errorf("normalize[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Lerp {
		got := V2(k.A[0], k.A[1]).Lerp(V2(k.B[0], k.B[1]), k.T)
		if !close2(got, k.Want, epsGeom) {
			t.Errorf("lerp[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Rotate {
		got := V2(k.V[0], k.V[1]).Rotate(k.Rad)
		if !close2(got, k.Want, epsGeom) {
			t.Errorf("rotate[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Angle {
		got := V2(k.A[0], k.A[1]).Angle(V2(k.B[0], k.B[1]))
		if math.Abs(got-k.Want) >= epsGeom {
			t.Errorf("angle[%d] = %v, want %v", i, got, k.Want)
		}
	}
}

// A: rect ops match the frozen numbers.
func TestRectFromCases(t *testing.T) {
	var c struct {
		Contains []struct {
			R    [4]float64
			P    [2]float64
			Want bool
		} `json:"contains"`
		Intersection []struct {
			A, B [4]float64
			Want [4]float64
			OK   bool
		} `json:"intersection"`
		Union []struct {
			A, B, Want [4]float64
		} `json:"union"`
		Area []struct {
			R    [4]float64
			Want float64
		} `json:"area"`
		Empty []struct {
			R    [4]float64
			Want bool
		} `json:"empty"`
		Inset []struct {
			R, Want [4]float64
			Dx, Dy  float64
		} `json:"inset"`
	}
	// center/offset decode separately: they share [4]float64 shapes.
	var raw map[string]json.RawMessage
	loadCases(t, "rect_cases.json", &raw)
	if err := json.Unmarshal(raw["contains"], &c.Contains); err != nil {
		t.Fatalf("contains: %v", err)
	}
	if err := json.Unmarshal(raw["intersection"], &c.Intersection); err != nil {
		t.Fatalf("intersection: %v", err)
	}
	if err := json.Unmarshal(raw["union"], &c.Union); err != nil {
		t.Fatalf("union: %v", err)
	}
	if err := json.Unmarshal(raw["area"], &c.Area); err != nil {
		t.Fatalf("area: %v", err)
	}
	if err := json.Unmarshal(raw["empty"], &c.Empty); err != nil {
		t.Fatalf("empty: %v", err)
	}
	var centers []struct {
		R    [4]float64
		Want [2]float64
	}
	if err := json.Unmarshal(raw["center"], &centers); err != nil {
		t.Fatalf("center: %v", err)
	}
	var offsets []struct {
		R, Want [4]float64
		D       [2]float64
	}
	if err := json.Unmarshal(raw["offset"], &offsets); err != nil {
		t.Fatalf("offset: %v", err)
	}
	if err := json.Unmarshal(raw["inset"], &c.Inset); err != nil {
		t.Fatalf("inset: %v", err)
	}

	mk := func(a [4]float64) Rect { return NewRect(a[0], a[1], a[2], a[3]) }
	for i, k := range c.Contains {
		if got := mk(k.R).Contains(V2(k.P[0], k.P[1])); got != k.Want {
			t.Errorf("contains[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Intersection {
		got, ok := mk(k.A).Intersection(mk(k.B))
		if ok != k.OK {
			t.Errorf("intersection[%d] ok = %v, want %v", i, ok, k.OK)
			continue
		}
		if ok && got != mk(k.Want) {
			t.Errorf("intersection[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Union {
		if got := mk(k.A).Union(mk(k.B)); got != mk(k.Want) {
			t.Errorf("union[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Area {
		if got := mk(k.R).Area(); got != k.Want {
			t.Errorf("area[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Empty {
		if got := mk(k.R).IsEmpty(); got != k.Want {
			t.Errorf("empty[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range centers {
		if got := mk(k.R).Center(); !close2(got, k.Want, epsGeom) {
			t.Errorf("center[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range offsets {
		if got := mk(k.R).Offset(V2(k.D[0], k.D[1])); got != mk(k.Want) {
			t.Errorf("offset[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Inset {
		if got := mk(k.R).Inset(k.Dx, k.Dy); got != mk(k.Want) {
			t.Errorf("inset[%d] = %v, want %v", i, got, k.Want)
		}
	}
}

// A: matrix ops match the frozen numbers.
func TestMatFromCases(t *testing.T) {
	var c struct {
		Mul []struct {
			A, B, Want [6]float64
		} `json:"mul"`
		Transform []struct {
			M    [6]float64
			P    [2]float64
			Want [2]float64
		} `json:"transform"`
		Invert []struct {
			M    [6]float64
			Want [6]float64
			OK   bool
		} `json:"invert"`
	}
	loadCases(t, "mat_cases.json", &c)
	mk := func(a [6]float64) Mat2D { return Mat2D{a[0], a[1], a[2], a[3], a[4], a[5]} }
	for i, k := range c.Mul {
		if got := mk(k.A).Mul(mk(k.B)); !got.ApproxEqual(mk(k.Want), epsFloat) {
			t.Errorf("mul[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Transform {
		if got := mk(k.M).TransformPoint(V2(k.P[0], k.P[1])); !close2(got, k.Want, epsGeom) {
			t.Errorf("transform[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Invert {
		got, ok := mk(k.M).Invert()
		if ok != k.OK {
			t.Errorf("invert[%d] ok = %v, want %v", i, ok, k.OK)
			continue
		}
		if ok && !got.ApproxEqual(mk(k.Want), epsFloat) {
			t.Errorf("invert[%d] = %v, want %v", i, got, k.Want)
		}
	}
	if !Identity2D().IsIdentity() {
		t.Error("Identity2D is not identity")
	}
}

// A: color ops match the frozen numbers.
func TestColorFromCases(t *testing.T) {
	var c struct {
		Bytes []struct {
			RGBA [4]float64
			Want [4]uint8
		} `json:"bytes"`
		FromBytes []struct {
			ABGR [4]uint8
			Want [4]float64
		} `json:"from_bytes"`
		Lerp []struct {
			A, B, Want [4]float64
			T          float64
		} `json:"lerp"`
		Clamp []struct {
			In, Want [4]float64
		} `json:"clamp"`
		Premul []struct {
			In, Want [4]float64
		} `json:"premul"`
		Unpremul []struct {
			In, Want [4]float64
		} `json:"unpremul"`
	}
	loadCases(t, "color_cases.json", &c)
	mk := func(a [4]float64) Color { return Color{a[0], a[1], a[2], a[3]} }
	for i, k := range c.Bytes {
		r, g, b, a := mk(k.RGBA).ToBytes()
		if got := [4]uint8{r, g, b, a}; got != k.Want {
			t.Errorf("bytes[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.FromBytes {
		got := ColorFromBytes(k.ABGR[0], k.ABGR[1], k.ABGR[2], k.ABGR[3])
		if !close4(got, k.Want, epsFloat) {
			t.Errorf("from_bytes[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Lerp {
		if got := mk(k.A).Lerp(mk(k.B), k.T); !close4(got, k.Want, epsFloat) {
			t.Errorf("lerp[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Clamp {
		if got := mk(k.In).Clamped(); !close4(got, k.Want, epsFloat) {
			t.Errorf("clamp[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Premul {
		if got := mk(k.In).Premultiplied(); !close4(got, k.Want, epsFloat) {
			t.Errorf("premul[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Unpremul {
		if got := mk(k.In).Unpremultiplied(); !close4(got, k.Want, epsFloat) {
			t.Errorf("unpremul[%d] = %v, want %v", i, got, k.Want)
		}
	}
}

// A+B: hex parsing accepts the frozen valid set, rejects the bad set.
func TestHexFromCases(t *testing.T) {
	var c struct {
		Valid []struct {
			In   string
			Want [4]float64
		} `json:"valid"`
		Invalid []string `json:"invalid"`
	}
	loadCases(t, "hex_cases.json", &c)
	for i, k := range c.Valid {
		got, err := ParseHex(k.In)
		if err != nil {
			t.Errorf("valid[%d] %q: unexpected error %v", i, k.In, err)
			continue
		}
		if !close4(got, k.Want, 1e-9) {
			t.Errorf("valid[%d] %q = %v, want %v", i, k.In, got, k.Want)
		}
	}
	for i, s := range c.Invalid {
		if got, err := ParseHex(s); err == nil {
			t.Errorf("invalid[%d] %q: got %v, want error", i, s, got)
		} else if CodeOf(err) != CodeBadData {
			t.Errorf("invalid[%d] %q: code = %v, want bad-data", i, s, CodeOf(err))
		}
	}
}

// A: duration/step arithmetic matches the frozen numbers.
func TestTimeFromCases(t *testing.T) {
	var c struct {
		Millis []struct {
			Ms  int64
			Sec float64
		} `json:"millis"`
		SecondsFloat []struct {
			S  float64
			Ms int64
		} `json:"seconds_float"`
		Scale []struct {
			Ms   int64
			F    float64
			Want int64
		} `json:"scale"`
		Clamp []struct {
			V, Min, Max, Want int64
		} `json:"clamp"`
		Step []struct {
			Index uint64
			Dt    int64
			Total int64
		} `json:"step"`
	}
	loadCases(t, "time_cases.json", &c)
	for i, k := range c.Millis {
		if got := Milliseconds(k.Ms).Seconds(); got != k.Sec {
			t.Errorf("millis[%d] = %v, want %v", i, got, k.Sec)
		}
	}
	for i, k := range c.SecondsFloat {
		if got := SecondsFloat(k.S).Milliseconds(); got != k.Ms {
			t.Errorf("seconds_float[%d] = %v, want %v", i, got, k.Ms)
		}
	}
	for i, k := range c.Scale {
		if got := Milliseconds(k.Ms).Scale(k.F).Milliseconds(); got != k.Want {
			t.Errorf("scale[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Clamp {
		got := Milliseconds(k.V).Clamp(Milliseconds(k.Min), Milliseconds(k.Max))
		if got.Milliseconds() != k.Want {
			t.Errorf("clamp[%d] = %v, want %v", i, got, k.Want)
		}
	}
	for i, k := range c.Step {
		s := Step{Index: k.Index, Dt: Milliseconds(k.Dt)}
		if got := s.Total().Milliseconds(); got != k.Total {
			t.Errorf("step[%d] total = %v, want %v", i, got, k.Total)
		}
	}
	if s := FirstStep(Milliseconds(16)); s.Index != 0 || s.Next().Index != 1 {
		t.Errorf("FirstStep/Next = %+v, want index 0 then 1", s)
	}
}

// A: seeded stream replays the frozen golden sequence exactly.
func TestRandGolden(t *testing.T) {
	var c struct {
		Uint64 []struct {
			Seed uint64
			Want []uint64
		} `json:"uint64_golden"`
		Float []struct {
			Seed uint64
			Want []float64
		} `json:"float_golden"`
	}
	loadCases(t, "rand_cases.json", &c)
	for _, k := range c.Uint64 {
		r := NewRand(k.Seed)
		for i, want := range k.Want {
			if got := r.Uint64(); got != want {
				t.Fatalf("seed %d uint64[%d] = %d, want %d", k.Seed, i, got, want)
			}
		}
	}
	for _, k := range c.Float {
		r := NewRand(k.Seed)
		for i, want := range k.Want {
			if got := r.Float64(); got != want {
				t.Fatalf("seed %d float[%d] = %.17g, want %.17g", k.Seed, i, got, want)
			}
		}
	}
}

// Same seed replays, different seeds diverge; bounds hold.
func TestRandProperties(t *testing.T) {
	a, b := NewRand(99), NewRand(99)
	for i := 0; i < 64; i++ {
		if a.Uint64() != b.Uint64() {
			t.Fatalf("same seed diverged at draw %d", i)
		}
	}
	if NewRand(1).Uint64() == NewRand(2).Uint64() {
		t.Error("different seeds produced the same first value")
	}
	r := NewRand(5)
	for i := 0; i < 1000; i++ {
		if f := r.Float64(); f < 0 || f >= 1 {
			t.Fatalf("Float64 out of [0,1): %v", f)
		}
		if f := r.RangeFloat(2, -2); f < -2 || f > 2 {
			t.Fatalf("RangeFloat swapped bounds out of range: %v", f)
		}
		if n := r.RangeInt(10, 3); n != 10 {
			t.Fatalf("RangeInt bad bounds = %d, want 10", n)
		}
	}
	if got := r.Int63n(0); got != 0 {
		t.Errorf("Int63n(0) = %d, want 0", got)
	}
	if got := r.Int63n(-7); got != 0 {
		t.Errorf("Int63n(-7) = %d, want 0", got)
	}
	// Shuffle replays with the same seed and never drops elements.
	perm := func(seed uint64) []int {
		rr := NewRand(seed)
		xs := []int{0, 1, 2, 3, 4, 5, 6, 7}
		rr.Shuffle(len(xs), func(i, j int) { xs[i], xs[j] = xs[j], xs[i] })
		return xs
	}
	p1, p2 := perm(11), perm(11)
	for i := range p1 {
		if p1[i] != p2[i] {
			t.Fatalf("shuffle not replayable: %v vs %v", p1, p2)
		}
	}
	seen := map[int]int{}
	for _, v := range p1 {
		seen[v]++
	}
	if len(seen) != 8 {
		t.Errorf("shuffle lost elements: %v", p1)
	}
	r.Shuffle(0, nil)
	r.Shuffle(-3, nil)
	r.Shuffle(4, nil)
}

// A: version parse/compat matches the frozen table.
func TestVersionFromCases(t *testing.T) {
	var c struct {
		Parse []struct {
			In   string
			Want [2]int
		} `json:"parse"`
		ParseBad []string `json:"parse_bad"`
		Compat   []struct {
			Data, Engine string
			Want         bool
		} `json:"compat"`
	}
	loadCases(t, "version_cases.json", &c)
	for i, k := range c.Parse {
		got, err := ParseVersion(k.In)
		if err != nil {
			t.Errorf("parse[%d] %q: unexpected error %v", i, k.In, err)
			continue
		}
		if got.Major != k.Want[0] || got.Minor != k.Want[1] {
			t.Errorf("parse[%d] %q = %v, want %v", i, k.In, got, k.Want)
		}
	}
	for i, s := range c.ParseBad {
		if got, err := ParseVersion(s); err == nil {
			t.Errorf("parse_bad[%d] %q: got %v, want error", i, s, got)
		} else if CodeOf(err) != CodeBadData {
			t.Errorf("parse_bad[%d] %q: code = %v, want bad-data", i, s, CodeOf(err))
		}
	}
	for i, k := range c.Compat {
		data, err := ParseVersion(k.Data)
		if err != nil {
			t.Fatalf("compat[%d] data: %v", i, err)
		}
		engine, err := ParseVersion(k.Engine)
		if err != nil {
			t.Fatalf("compat[%d] engine: %v", i, err)
		}
		if got := data.CompatibleWith(engine); got != k.Want {
			t.Errorf("compat[%d] %v vs %v = %v, want %v", i, data, engine, got, k.Want)
		}
		if err := data.CheckCompatibility(engine); (err == nil) == !k.Want {
			t.Errorf("compat[%d] check err = %v, want compatible=%v", i, err, k.Want)
		} else if err != nil && CodeOf(err) != CodeVersionMismatch {
			t.Errorf("compat[%d] check code = %v, want version-mismatch", i, CodeOf(err))
		}
	}
	if SchemaVersion.Major < 1 {
		t.Errorf("SchemaVersion = %v, want major >= 1", SchemaVersion)
	}
}

// Asset ledger behavior driven by the frozen id list.
func TestAssetManagerFromCases(t *testing.T) {
	var c struct {
		IDs    []string `json:"ids"`
		BadIDs []string `json:"bad_ids"`
	}
	loadCases(t, "asset_cases.json", &c)
	m := NewManager()
	for _, id := range c.IDs {
		h, err := m.Acquire(AssetID(id))
		if err != nil {
			t.Fatalf("acquire %q: %v", id, err)
		}
		if !m.Loaded(AssetID(id)) || m.LiveCount(AssetID(id)) != 1 {
			t.Fatalf("after acquire %q: loaded=%v count=%d", id, m.Loaded(AssetID(id)), m.LiveCount(AssetID(id)))
		}
		if !h.Ref() || h.Refs() != 2 {
			t.Fatalf("ref %q: refs=%d, want 2", id, h.Refs())
		}
		if err := h.Release(); err != nil {
			t.Fatalf("release %q: %v", id, err)
		}
		if err := h.Release(); err != nil {
			t.Fatalf("release2 %q: %v", id, err)
		}
		if m.Loaded(AssetID(id)) {
			t.Errorf("after full release %q still loaded", id)
		}
		if err := h.Release(); err == nil {
			t.Errorf("over-release %q: want error", id)
		} else if CodeOf(err) != CodeInvalidArg {
			t.Errorf("over-release %q code = %v, want invalid-arg", id, CodeOf(err))
		}
	}
	for _, id := range c.BadIDs {
		if _, err := m.Acquire(AssetID(id)); err == nil {
			t.Errorf("acquire empty id: want error")
		} else if CodeOf(err) != CodeInvalidArg {
			t.Errorf("acquire empty id code = %v, want invalid-arg", CodeOf(err))
		}
	}
	var nilH *Handle
	if nilH.Ref() {
		t.Error("nil handle Ref = true, want false")
	}
	if err := nilH.Release(); err == nil {
		t.Error("nil handle Release = nil, want error")
	}
	if got := nilH.Refs(); got != 0 {
		t.Errorf("nil handle Refs = %d, want 0", got)
	}
}

// Error taxonomy: codes survive wrapping, foreign errors stay unknown.
func TestResultCodes(t *testing.T) {
	cases := []struct {
		err  error
		want Code
	}{
		{nil, CodeUnknown},
		{fmt.Errorf("boom"), CodeUnknown},
		{NotFound("op", "a"), CodeNotFound},
		{BadData("op", "b"), CodeBadData},
		{OutOfMemory("op", "c"), CodeOutOfMemory},
		{Unsupported("op", "d"), CodeUnsupported},
		{InvalidArg("op", "e"), CodeInvalidArg},
		{fmt.Errorf("wrap: %w", NotFound("op", "f")), CodeNotFound},
	}
	for i, k := range cases {
		if got := CodeOf(k.err); got != k.want {
			t.Errorf("case[%d] code = %v, want %v", i, got, k.want)
		}
	}
	e := BadData("core.ParseHex", "#zz")
	if e.Error() == "" {
		t.Error("Error() is empty")
	}
	for _, code := range []Code{CodeNotFound, CodeBadData, CodeOutOfMemory, CodeUnsupported, CodeInvalidArg, CodeVersionMismatch, CodeUnknown} {
		if code.String() == "" {
			t.Errorf("code %d has no name", int(code))
		}
	}
}

// Step 3 (wiring): core<->render boundary converts losslessly.
// C (GPU vs CPU pixels) does not apply: core draws nothing.
func TestBoundaryRenderRoundTrip(t *testing.T) {
	vs := []Vec2{{1.5, -2.25}, {0, 0}, {-1e6, 1e6}}
	for i, v := range vs {
		if got := Vec2FromRenderPoint(v.ToRenderPoint()); got != v {
			t.Errorf("vec[%d] round trip = %v, want %v", i, got, v)
		}
	}
	cs := []Color{{0.1, 0.2, 0.3, 0.4}, Black, White, Transparent}
	for i, col := range cs {
		if got := ColorFromRender(col.ToRender()); got != col {
			t.Errorf("color[%d] round trip = %v, want %v", i, got, col)
		}
	}
	ms := []Mat2D{Identity2D(), Translate2D(3, -4), Scale2D(2, 0.5), Rotate2D(0.7)}
	for i, m := range ms {
		if got := Mat2DFromRenderMatrix(m.ToRenderMatrix()); got != m {
			t.Errorf("mat[%d] round trip = %v, want %v", i, got, m)
		}
	}
}

// B: empty/zero/huge/singular inputs never panic, NaN, or hang.
func TestEdgeNoCrash(t *testing.T) {
	if got := (Vec2{}).Normalize(); got != (Vec2{}) {
		t.Errorf("zero normalize = %v, want zero", got)
	}
	if got := V2(1, -1).Div(0); math.IsNaN(got.X) || math.IsNaN(got.Y) {
		t.Errorf("div by zero produced NaN: %v", got)
	}
	if got := V2(1e308, 1e308).Add(V2(1e308, 1e308)); math.IsNaN(got.X) {
		t.Errorf("huge add produced NaN: %v", got)
	}
	if _, ok := (Mat2D{}).Invert(); ok {
		t.Error("singular matrix inverted, want ok=false")
	}
	if NewRect(0, 0, 0, 0).Contains(V2(0, 0)) {
		t.Error("empty rect contains origin, want false")
	}
	if _, ok := NewRect(0, 0, 0, 0).Intersection(NewRect(0, 0, 0, 0)); ok {
		t.Error("empty rect intersection ok=true, want false")
	}
	if _, err := ParseVersion(""); err == nil {
		t.Error("ParseVersion(\"\") = nil, want error")
	}
	if _, err := ParseHex(""); err == nil {
		t.Error("ParseHex(\"\") = nil, want error")
	}
	if got := Milliseconds(5).Clamp(Milliseconds(9), Milliseconds(2)); got != Milliseconds(9) {
		t.Errorf("inverted clamp = %v, want 9ms", got)
	}
}

// D: 10k mixed ops complete with a measured cost (numbers in -v output).
func TestCorePerf10k(t *testing.T) {
	const n = 10000
	start := time.Now()
	var acc Vec2
	m := Rotate2D(0.01)
	col := RGB(0.2, 0.4, 0.6)
	for i := 0; i < n; i++ {
		acc = acc.Add(V2(float64(i), float64(-i))).Mul(0.999)
		acc = m.TransformPoint(acc)
		col = col.Lerp(White, 0.0001)
	}
	el := time.Since(start)
	t.Logf("core-10k: %d mixed vec/mat/color ops in %v (%.1f ns/op)", n, el, float64(el.Nanoseconds())/n)
	if acc.IsZero() && col == (Color{}) {
		t.Error("perf loop folded to zero, benchmark invalid")
	}
}

// E: long runs do not drift (integer ms ledger + exact step counts).
func TestCoreLongRunNoDrift(t *testing.T) {
	const ticks = 3600000 // 1h of 1ms ticks
	var total Duration
	for i := 0; i < ticks; i++ {
		total = total.Add(Millisecond)
	}
	if total != 3600000*Millisecond {
		t.Errorf("1h ms accumulation = %d, want %d", total.Milliseconds(), 3600000)
	}
	s := FirstStep(Milliseconds(16))
	const steps = 1000000
	for i := 0; i < steps; i++ {
		s = s.Next()
	}
	if s.Index != steps {
		t.Errorf("step index = %d, want %d", s.Index, steps)
	}
	if want := Duration(steps) * 16; s.Total() != want {
		t.Errorf("step total = %d, want %d", s.Total().Milliseconds(), want.Milliseconds())
	}
	r1, r2 := NewRand(2026), NewRand(2026)
	for i := 0; i < 10000; i++ {
		if r1.Uint64() != r2.Uint64() {
			t.Fatalf("replay diverged at draw %d", i)
		}
	}
}
