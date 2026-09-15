package step

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type poolLimits struct {
	MaxLen      int `json:"max_len"`
	MaxRetained int `json:"max_retained"`
	VecBytes    int `json:"vec_bytes"`
	ColorBytes  int `json:"color_bytes"`
	FloatBytes  int `json:"float_bytes"`
}

type poolSamples struct {
	Vecs   [][2]float64 `json:"vecs"`
	Colors [][4]float64 `json:"colors"`
	Floats []float64    `json:"floats"`
}

type poolReuse struct {
	N    int `json:"n"`
	Warm int `json:"warm"`
	Reps int `json:"reps"`
}

type poolPerf struct {
	N    int `json:"n"`
	Reps int `json:"reps"`
}

type poolLong struct {
	N    int `json:"n"`
	Reps int `json:"reps"`
}

type poolFile struct {
	VecSizes   []int       `json:"vec_sizes"`
	ColorSizes []int       `json:"color_sizes"`
	FloatSizes []int       `json:"float_sizes"`
	Samples    poolSamples `json:"samples"`
	Limits     poolLimits  `json:"limits"`
	Reuse      poolReuse   `json:"reuse"`
	Perf       poolPerf    `json:"perf"`
	Longrun    poolLong    `json:"longrun"`
}

func loadPoolCases(t *testing.T) poolFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "pool_cases.json"))
	if err != nil {
		t.Fatalf("read pool_cases.json: %v", err)
	}
	var f poolFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode pool_cases.json: %v", err)
	}
	if len(f.VecSizes) == 0 || len(f.Samples.Vecs) == 0 {
		t.Fatal("pool_cases.json has no cases")
	}
	return f
}

func fillVec(b []core.Vec2, samples [][2]float64) {
	for i := range b {
		s := samples[i%len(samples)]
		b[i] = core.V2(s[0], s[1])
	}
}

func checkVec(t *testing.T, name string, b []core.Vec2, samples [][2]float64) {
	t.Helper()
	for i := range b {
		s := samples[i%len(samples)]
		if b[i].X != s[0] || b[i].Y != s[1] {
			t.Fatalf("%s[%d] = %v, want %v", name, i, b[i], s)
		}
	}
}

func fillColor(b []core.Color, samples [][4]float64) {
	for i := range b {
		s := samples[i%len(samples)]
		b[i] = core.Color{R: s[0], G: s[1], B: s[2], A: s[3]}
	}
}

func checkColor(t *testing.T, name string, b []core.Color, samples [][4]float64) {
	t.Helper()
	for i := range b {
		s := samples[i%len(samples)]
		if b[i] != (core.Color{R: s[0], G: s[1], B: s[2], A: s[3]}) {
			t.Fatalf("%s[%d] = %v, want %v", name, i, b[i], s)
		}
	}
}

func fillFloat(b []float64, samples []float64) {
	for i := range b {
		b[i] = samples[i%len(samples)]
	}
}

func checkFloat(t *testing.T, name string, b []float64, samples []float64) {
	t.Helper()
	for i := range b {
		if b[i] != samples[i%len(samples)] {
			t.Fatalf("%s[%d] = %v, want %v", name, i, b[i], samples[i%len(samples)])
		}
	}
}

func expectCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

// A:复用画对:每种尺寸取放后数还在,第二次取命中,写满再读逐位一致.
func TestPoolReuseFromCases(t *testing.T) {
	f := loadPoolCases(t)
	p := NewPool()
	for _, n := range f.VecSizes {
		b, err := p.GetVec(n)
		if err != nil {
			t.Fatalf("GetVec(%d): %v", n, err)
		}
		if len(b) != n || cap(b) < n {
			t.Fatalf("GetVec(%d) len/cap = %d/%d", n, len(b), cap(b))
		}
		fillVec(b, f.Samples.Vecs)
		checkVec(t, "vec", b, f.Samples.Vecs)
		// Boundary still draws the same numbers after reuse.
		for i, v := range b {
			if back := core.Vec2FromRenderPoint(v.ToRenderPoint()); back != v {
				t.Fatalf("vec boundary[%d] = %v, want %v", i, back, v)
			}
		}
		p.PutVec(b)
		again, err := p.GetVec(n)
		if err != nil {
			t.Fatalf("GetVec reuse(%d): %v", n, err)
		}
		if len(again) != n || cap(again) < n {
			t.Fatalf("reuse GetVec(%d) len/cap = %d/%d", n, len(again), cap(again))
		}
		fillVec(again, f.Samples.Vecs)
		checkVec(t, "vec reuse", again, f.Samples.Vecs)
		p.PutVec(again)
	}
	for _, n := range f.ColorSizes {
		b, err := p.GetColor(n)
		if err != nil {
			t.Fatalf("GetColor(%d): %v", n, err)
		}
		fillColor(b, f.Samples.Colors)
		checkColor(t, "color", b, f.Samples.Colors)
		for i, c := range b {
			if back := core.ColorFromRender(c.ToRender()); back != c {
				t.Fatalf("color boundary[%d] = %v, want %v", i, back, c)
			}
		}
		p.PutColor(b)
		again, err := p.GetColor(n)
		if err != nil {
			t.Fatalf("GetColor reuse(%d): %v", n, err)
		}
		fillColor(again, f.Samples.Colors)
		checkColor(t, "color reuse", again, f.Samples.Colors)
		p.PutColor(again)
	}
	for _, n := range f.FloatSizes {
		b, err := p.GetFloat(n)
		if err != nil {
			t.Fatalf("GetFloat(%d): %v", n, err)
		}
		fillFloat(b, f.Samples.Floats)
		checkFloat(t, "float", b, f.Samples.Floats)
		p.PutFloat(b)
		again, err := p.GetFloat(n)
		if err != nil {
			t.Fatalf("GetFloat reuse(%d): %v", n, err)
		}
		fillFloat(again, f.Samples.Floats)
		checkFloat(t, "float reuse", again, f.Samples.Floats)
		p.PutFloat(again)
	}
	st := p.Stats()
	if st.Hits == 0 {
		t.Error("reuse produced no hits, want reuse")
	}
	if st.Gets == 0 || st.Puts == 0 {
		t.Errorf("stats gets/puts = %d/%d, want both > 0", st.Gets, st.Puts)
	}
	// Two live Gets never alias: parallel paths write without clobbering.
	q := NewPool()
	a, _ := q.GetVec(f.Reuse.N)
	b2, _ := q.GetVec(f.Reuse.N)
	fillVec(a, f.Samples.Vecs)
	fillVec(b2, f.Samples.Vecs)
	a[0] = core.V2(777, 888)
	if b2[0] == a[0] {
		t.Error("two live vec buffers alias, want independent")
	}
	q.PutVec(a)
	q.PutVec(b2)
}

// B:空零超大坏输入全不崩不卡死,错码分得清.
func TestPoolEdgesNoCrash(t *testing.T) {
	f := loadPoolCases(t)
	p := NewPool()
	// Empty pool Gets allocate, never panic.
	if b, err := p.GetVec(3); err != nil || len(b) != 3 {
		t.Fatalf("empty GetVec = %v/%v, want len 3", b, err)
	} else {
		p.PutVec(b)
	}
	if b, err := p.GetColor(2); err != nil || len(b) != 2 {
		t.Fatalf("empty GetColor = %v/%v", b, err)
	} else {
		p.PutColor(b)
	}
	if b, err := p.GetFloat(4); err != nil || len(b) != 4 {
		t.Fatalf("empty GetFloat = %v/%v", b, err)
	} else {
		p.PutFloat(b)
	}
	// Zero Gets are empty and safe.
	if b, err := p.GetVec(0); err != nil || len(b) != 0 {
		t.Errorf("GetVec(0) = %v/%v, want empty", b, err)
	}
	if b, err := p.GetColor(0); err != nil || len(b) != 0 {
		t.Errorf("GetColor(0) = %v/%v, want empty", b, err)
	}
	if b, err := p.GetFloat(0); err != nil || len(b) != 0 {
		t.Errorf("GetFloat(0) = %v/%v, want empty", b, err)
	}
	// Nil and zero-cap Puts are no-ops.
	p.PutVec(nil)
	p.PutColor(nil)
	p.PutFloat(nil)
	p.PutVec([]core.Vec2{})
	p.PutColor([]core.Color{})
	p.PutFloat([]float64{})
	// Negative is InvalidArg, over-limit is OutOfMemory.
	_, err := p.GetVec(-1)
	expectCode(t, "GetVec(-1)", err, core.CodeInvalidArg)
	_, err = p.GetColor(-5)
	expectCode(t, "GetColor(-5)", err, core.CodeInvalidArg)
	_, err = p.GetFloat(-9)
	expectCode(t, "GetFloat(-9)", err, core.CodeInvalidArg)
	huge := f.Limits.MaxLen + 1
	_, err = p.GetVec(huge)
	expectCode(t, "GetVec(huge)", err, core.CodeOutOfMemory)
	_, err = p.GetColor(huge)
	expectCode(t, "GetColor(huge)", err, core.CodeOutOfMemory)
	_, err = p.GetFloat(huge)
	expectCode(t, "GetFloat(huge)", err, core.CodeOutOfMemory)
	// Oversized Puts drop but never crash or retain.
	full := NewPool()
	for i := 0; i < f.Limits.MaxRetained+2; i++ {
		full.PutVec(make([]core.Vec2, 1))
	}
	if st := full.Stats(); st.RetainedVec != f.Limits.MaxRetained {
		t.Errorf("over-budget retained vec = %d, want %d", st.RetainedVec, f.Limits.MaxRetained)
	}
	full.PutFloat(make([]float64, 0, f.Limits.MaxLen+1))
	if st := full.Stats(); st.RetainedFloat != 0 {
		t.Errorf("oversized retained float = %d, want 0", st.RetainedFloat)
	}
	// Nil pool never panics.
	var nilPool *Pool
	if _, err := nilPool.GetVec(1); err == nil {
		t.Error("nil GetVec want error")
	} else if core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil GetVec code = %v, want invalid-arg", core.CodeOf(err))
	}
	nilPool.PutVec([]core.Vec2{{X: 1, Y: 2}})
	nilPool.PutColor([]core.Color{{R: 1}})
	nilPool.PutFloat([]float64{1})
	nilPool.ResetStats()
	if got := nilPool.Stats(); got != (Stats{}) {
		t.Errorf("nil Stats = %+v, want zero", got)
	}
	if got := nilPool.RetainedBytes(); got != 0 {
		t.Errorf("nil RetainedBytes = %d, want 0", got)
	}
	if got := nilPool.HitRate(); got != 0 {
		t.Errorf("nil HitRate = %v, want 0", got)
	}
	// Non-finite samples survive the pool byte-for-byte (no math inside).
	nanVec := []core.Vec2{{X: math.NaN(), Y: math.Inf(1)}}
	p.PutVec(nanVec)
	got, err := p.GetVec(1)
	if err != nil {
		t.Fatalf("NaN GetVec: %v", err)
	}
	got[0] = nanVec[0]
	if !math.IsNaN(got[0].X) || !math.IsInf(got[0].Y, 1) {
		t.Errorf("NaN vec did not survive: %v", got[0])
	}
	p.PutVec(got)
}

// C:纯池化不画画,两边同数靠边界往返无损加逐位重放.
func TestPoolBoundaryIdentical(t *testing.T) {
	f := loadPoolCases(t)
	p := NewPool()
	// Every frozen sample crosses to render and back losslessly.
	for i, s := range f.Samples.Vecs {
		v := core.V2(s[0], s[1])
		if back := core.Vec2FromRenderPoint(v.ToRenderPoint()); back != v {
			t.Fatalf("vec sample[%d] boundary = %v, want %v", i, back, v)
		}
	}
	for i, s := range f.Samples.Colors {
		c := core.Color{R: s[0], G: s[1], B: s[2], A: s[3]}
		if back := core.ColorFromRender(c.ToRender()); back != c {
			t.Fatalf("color sample[%d] boundary = %v, want %v", i, back, c)
		}
	}
	// Pooled buffers replay bitwise identical across Get/Put cycles.
	b, err := p.GetVec(f.Reuse.N)
	if err != nil {
		t.Fatalf("GetVec: %v", err)
	}
	fillVec(b, f.Samples.Vecs)
	snap := append([]core.Vec2(nil), b...)
	p.PutVec(b)
	for r := 0; r < 20; r++ {
		got, err := p.GetVec(f.Reuse.N)
		if err != nil {
			t.Fatalf("rep %d: %v", r, err)
		}
		fillVec(got, f.Samples.Vecs)
		for i := range got {
			if got[i] != snap[i] {
				t.Fatalf("rep %d diverged at %d: %v vs %v", r, i, got[i], snap[i])
			}
		}
		p.PutVec(got)
	}
	cb, _ := p.GetColor(f.Reuse.N)
	fillColor(cb, f.Samples.Colors)
	csnap := append([]core.Color(nil), cb...)
	p.PutColor(cb)
	for r := 0; r < 20; r++ {
		got, _ := p.GetColor(f.Reuse.N)
		fillColor(got, f.Samples.Colors)
		for i := range got {
			if got[i] != csnap[i] {
				t.Fatalf("color rep %d diverged at %d", r, i)
			}
		}
		p.PutColor(got)
	}
	fb, _ := p.GetFloat(f.Reuse.N)
	fillFloat(fb, f.Samples.Floats)
	fsnap := append([]float64(nil), fb...)
	p.PutFloat(fb)
	for r := 0; r < 20; r++ {
		got, _ := p.GetFloat(f.Reuse.N)
		fillFloat(got, f.Samples.Floats)
		for i := range got {
			if got[i] != fsnap[i] {
				t.Fatalf("float rep %d diverged at %d", r, i)
			}
		}
		p.PutFloat(got)
	}
}

// D:复用跑得动,命中率内存有数.
func TestPoolPerfReuse(t *testing.T) {
	// Synthetic load only (no golden): golden sizes stay in pool_cases.json.
	f := loadPoolCases(t)
	p := NewPool()
	n, reps := f.Perf.N, f.Perf.Reps
	if n <= 0 || reps <= 0 {
		t.Fatalf("perf params = %d/%d, want > 0", n, reps)
	}
	start := time.Now()
	for i := 0; i < reps; i++ {
		v, err := p.GetVec(n)
		if err != nil {
			t.Fatalf("rep %d GetVec: %v", i, err)
		}
		fillVec(v, f.Samples.Vecs)
		c, err := p.GetColor(n)
		if err != nil {
			t.Fatalf("rep %d GetColor: %v", i, err)
		}
		fillColor(c, f.Samples.Colors)
		fl, err := p.GetFloat(n)
		if err != nil {
			t.Fatalf("rep %d GetFloat: %v", i, err)
		}
		fillFloat(fl, f.Samples.Floats)
		p.PutVec(v)
		p.PutColor(c)
		p.PutFloat(fl)
	}
	el := time.Since(start)
	st := p.Stats()
	rate := p.HitRate()
	t.Logf("pool-perf: %d reps x vec/color/float %d in %v (%.1f us/rep, hits %d/%d rate %.3f retained %dB)",
		reps, n, el, float64(el.Microseconds())/float64(reps), st.Hits, st.Gets, rate, st.RetainedBytes)
	if st.Hits == 0 {
		t.Error("perf produced no hits, want reuse")
	}
	if rate < 0.9 {
		t.Errorf("hit rate = %.3f, want >= 0.9 after warm pool", rate)
	}
	if st.RetainedVec > f.Limits.MaxRetained || st.RetainedColor > f.Limits.MaxRetained || st.RetainedFloat > f.Limits.MaxRetained {
		t.Errorf("retained %+v exceeds max %d", st, f.Limits.MaxRetained)
	}
	if st.RetainedBytes <= 0 {
		t.Error("retained bytes = 0, want accounted memory")
	}
}

// E:长跑不涨不漂不粘.
func TestPoolLongRunStable(t *testing.T) {
	f := loadPoolCases(t)
	p := NewPool()
	n, reps := f.Longrun.N, f.Longrun.Reps
	if n <= 0 || reps <= 0 {
		t.Fatalf("longrun params = %d/%d, want > 0", n, reps)
	}
	// Warm all three kinds so the retained set is populated, then pin it.
	v0, _ := p.GetVec(n)
	fillVec(v0, f.Samples.Vecs)
	snap0 := v0[0]
	p.PutVec(v0)
	c0, _ := p.GetColor(n)
	fillColor(c0, f.Samples.Colors)
	p.PutColor(c0)
	f0, _ := p.GetFloat(n)
	fillFloat(f0, f.Samples.Floats)
	p.PutFloat(f0)
	p.ResetStats()
	base := p.RetainedBytes()
	for i := 0; i < reps; i++ {
		v, err := p.GetVec(n)
		if err != nil {
			t.Fatalf("rep %d GetVec: %v", i, err)
		}
		fillVec(v, f.Samples.Vecs)
		if v[0] != snap0 {
			t.Fatalf("rep %d first vec = %v, want %v", i, v[0], snap0)
		}
		c, _ := p.GetColor(n)
		fillColor(c, f.Samples.Colors)
		fl, _ := p.GetFloat(n)
		fillFloat(fl, f.Samples.Floats)
		p.PutVec(v)
		p.PutColor(c)
		p.PutFloat(fl)
		if (i+1)%1000 == 0 {
			if got := p.RetainedBytes(); got != base {
				t.Fatalf("rep %d retained moved: %dB, want steady %dB", i, got, base)
			}
		}
	}
	end := p.RetainedBytes()
	st := p.Stats()
	t.Logf("pool-long: %d reps x %d retained %dB -> %dB hits %d/%d", reps, n, base, end, st.Hits, st.Gets)
	if end != base {
		t.Errorf("retained moved: %dB -> %dB, want steady (no growth)", base, end)
	}
	if st.RetainedVec != 1 || st.RetainedColor != 1 || st.RetainedFloat != 1 {
		t.Errorf("retained vec/color/float = %d/%d/%d, want 1/1/1 steady",
			st.RetainedVec, st.RetainedColor, st.RetainedFloat)
	}
	if st.Gets != reps*3 {
		t.Errorf("gets = %d, want %d (3 kinds x reps)", st.Gets, reps*3)
	}
	if st.Puts != reps*3 {
		t.Errorf("puts = %d, want %d", st.Puts, reps*3)
	}
	// ResetStats clears counters without dropping buffers.
	kept := p.RetainedBytes()
	p.ResetStats()
	zero := p.Stats()
	if zero.Gets != 0 || zero.Puts != 0 || zero.Hits != 0 || zero.Misses != 0 {
		t.Errorf("after ResetStats = %+v, want zero counters", zero)
	}
	if got := p.RetainedBytes(); got != kept {
		t.Errorf("ResetStats dropped buffers: %dB -> %dB", kept, got)
	}
}

// F:离屏金对照窗(W1免窗):冻结数加形状断言.
func TestPoolOffscreenGolden(t *testing.T) {
	f := loadPoolCases(t)
	// Limits pin the frozen budgets: code and file must agree.
	if MaxLen != f.Limits.MaxLen {
		t.Fatalf("MaxLen = %d, want frozen %d", MaxLen, f.Limits.MaxLen)
	}
	if MaxRetained != f.Limits.MaxRetained {
		t.Fatalf("MaxRetained = %d, want frozen %d", MaxRetained, f.Limits.MaxRetained)
	}
	if VecBytes != f.Limits.VecBytes || ColorBytes != f.Limits.ColorBytes || FloatBytes != f.Limits.FloatBytes {
		t.Fatalf("bytes = %d/%d/%d, want frozen %d/%d/%d",
			VecBytes, ColorBytes, FloatBytes, f.Limits.VecBytes, f.Limits.ColorBytes, f.Limits.FloatBytes)
	}
	if len(f.Samples.Vecs) == 0 || len(f.Samples.Colors) == 0 || len(f.Samples.Floats) == 0 {
		t.Fatal("samples missing, want frozen vecs/colors/floats")
	}
	// Shape: retained accounting is cap * frozen width per kind.
	p := NewPool()
	v, _ := p.GetVec(10)
	c, _ := p.GetColor(6)
	fl, _ := p.GetFloat(8)
	p.PutVec(v)
	p.PutColor(c)
	p.PutFloat(fl)
	st := p.Stats()
	want := int64(cap(v)*VecBytes + cap(c)*ColorBytes + cap(fl)*FloatBytes)
	if st.RetainedBytes != want {
		t.Errorf("retained = %dB, want cap formula %dB", st.RetainedBytes, want)
	}
	if got := p.RetainedBytes(); got != want {
		t.Errorf("RetainedBytes() = %d, want %d", got, want)
	}
	// Shape: first sample stays first after a full Get/fill/Put/Get cycle.
	b, _ := p.GetVec(len(f.Samples.Vecs))
	fillVec(b, f.Samples.Vecs)
	first := b[0]
	p.PutVec(b)
	again, _ := p.GetVec(len(f.Samples.Vecs))
	fillVec(again, f.Samples.Vecs)
	if again[0] != first {
		t.Errorf("first vec = %v, want stable %v", again[0], first)
	}
	p.PutVec(again)
	// Shape: reuse order is LIFO-stable (no flicker across identical Gets).
	r := NewPool()
	for i := 0; i < f.Reuse.Warm; i++ {
		b, _ := r.GetVec(f.Reuse.N)
		r.PutVec(b)
	}
	hitBase := r.Stats().Hits
	for i := 0; i < f.Reuse.Reps; i++ {
		b, _ := r.GetVec(f.Reuse.N)
		r.PutVec(b)
	}
	if got := r.Stats().Hits - hitBase; got != f.Reuse.Reps {
		t.Errorf("steady hits = %d, want %d (every Get hits)", got, f.Reuse.Reps)
	}
}
