//go:build !nogpu

package render_test

// Shared harness for memory / VRAM lifecycle gates (see docs/MEM_LEAK_TEST_PLAN.md).

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
)

func memEnvIters(def int) int {
	v := os.Getenv("GPUI_MEM_ITERS")
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n < 1 {
		return def
	}
	return n
}

func memEnvInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var n int64
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}

func memSeed() int64 {
	if v := os.Getenv("GPUI_MEM_SEED"); v != "" {
		var n int64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return 42
}

// memRSSKB reads VmRSS from /proc/self/status (Linux). Returns 0 if unavailable.
func memRSSKB() int64 {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		n, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

func memHardRSSCheck(t *testing.T) {
	t.Helper()
	hard := memEnvInt64("GPUI_MEM_RSS_HARD_KB", 0)
	if hard <= 0 {
		return
	}
	rss := memRSSKB()
	if rss > hard {
		t.Fatalf("RSS hard limit: VmRSS=%d KB > %d KB", rss, hard)
	}
}

// memAssertSteadyRSS checks steady-state RSS growth after warmup.
// Algorithm is aligned with particle_kitchen_sink rssSteadyDelta:
//  1. drop non-positive samples
//  2. drop first 20% as warmup
//  3. grow = mean(last third) - mean(first third) of the remainder
//
// Soft gate only; hard OOM/Present failures are separate.
func memAssertSteadyRSS(t *testing.T, samples []int64, deltaKB int64, label string) {
	t.Helper()
	if len(samples) < 6 {
		return
	}
	xs := make([]int64, 0, len(samples))
	for _, s := range samples {
		if s > 0 {
			xs = append(xs, s)
		}
	}
	if len(xs) < 6 {
		t.Logf("%s: RSS unavailable or sparse; skip soft growth gate", label)
		return
	}
	start := len(xs) / 5
	steady := xs[start:]
	if len(steady) < 9 {
		// short runs: fall back to full-sample thirds
		steady = xs
	}
	third := len(steady) / 3
	if third < 2 {
		return
	}
	var early, late float64
	for i := 0; i < third; i++ {
		early += float64(steady[i])
	}
	for i := len(steady) - third; i < len(steady); i++ {
		late += float64(steady[i])
	}
	early /= float64(third)
	late /= float64(third)
	grow := late - early
	t.Logf("%s RSS warmup_drop=%d early_avg=%.0fKB late_avg=%.0fKB grow=%.0fKB limit=%dKB",
		label, start, early, late, grow, deltaKB)
	if grow > float64(deltaKB) {
		t.Fatalf("%s steady RSS growth %.0f KB exceeds limit %d KB", label, grow, deltaKB)
	}
}

func memFindFont(t *testing.T) string {
	t.Helper()
	cands := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		filepath.Join("text", "testdata", "goregular.ttf"),
	}
	for _, p := range cands {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no font for mem complex scenes")
	return ""
}

func memRequireGPU(t *testing.T) {
	t.Helper()
	// Mem suite prefers 1x samples: 4x MSAA probe can abort the process on
	// VRAM-exhausted hosts instead of returning an error.
	_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Log("WGPU_NATIVE_PATH unset; relying on default lib discovery")
	}
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	// Use a real offscreen Present (not FlushGPU readback) so the probe path
	// matches production UI present and reclaims via the same session lifecycle.
	dc := render.NewContext(32, 32)
	view, rel := dc.CreateOffscreenTexture(32, 32)
	if rel == nil || view.IsNil() {
		_ = dc.Close()
		t.Skip("CreateOffscreenTexture unavailable")
	}
	dc.ResetRenderPathStats()
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()
	err := dc.PresentFrame(view, 32, 32, nil)
	st := dc.RenderPathStats()
	rel()
	_ = dc.Close()
	if err != nil {
		t.Skipf("GPU present unavailable: %v", err)
	}
	if st.GPUOps == 0 {
		t.Skipf("no GPU ops on probe: %s", st.LogLine())
	}
}

// memRNG is a tiny deterministic LCG (no math/rand global).
type memRNG struct{ s uint64 }

func newMemRNG(seed int64) *memRNG {
	if seed == 0 {
		seed = 1
	}
	return &memRNG{s: uint64(seed)}
}

func (r *memRNG) next() uint64 {
	// xorshift*
	x := r.s
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	r.s = x
	return x * 0x2545F4914F6CDD1D
}

func (r *memRNG) float01() float64 {
	return float64(r.next()%10000) / 10000.0
}

func (r *memRNG) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

// memSceneLevel selects draw complexity.
type memSceneLevel int

const (
	memSceneSimple memSceneLevel = iota
	memSceneMedium
	memSceneComplex
)

// memDrawScene paints a deterministic-random UI/game-like 2D frame into dc.
// Exercises transforms, path, stroke/dash, text, image, layer, clip, blend.
func memDrawScene(t *testing.T, dc *render.Context, w, h int, frame int, lvl memSceneLevel, rng *memRNG, img *render.ImageBuf) {
	t.Helper()
	fw, fh := float64(w), float64(h)

	// Background (random-ish but smooth)
	br := 0.10 + 0.15*rng.float01()
	bg := 0.11 + 0.12*rng.float01()
	bb := 0.14 + 0.18*rng.float01()
	dc.SetRGB(br, bg, bb)
	dc.DrawRectangle(0, 0, fw, fh)
	_ = dc.Fill()

	// Header bar
	dc.SetRGB(0.16, 0.18, 0.24)
	dc.DrawRectangle(0, 0, fw, 40)
	_ = dc.Fill()
	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawString(fmt.Sprintf("mem scene f=%d %dx%d L=%d", frame, w, h, lvl), 12, 26)

	// Cards / rrects
	nCard := 2 + int(lvl)*2 + rng.intn(3)
	for i := 0; i < nCard; i++ {
		x := 12 + float64(i%4)*(fw/4)
		y := 56 + float64(i/4)*72
		cw := fw/4 - 20
		if cw < 40 {
			cw = 40
		}
		dc.SetRGB(0.95, 0.96, 0.98)
		dc.DrawRoundedRectangle(x, y, cw, 60, 8)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.45, 0.85)
		dc.DrawRoundedRectangle(x+8, y+12, 24, 24, 4)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("card-%d", i), x+40, y+30)
	}

	if lvl >= memSceneMedium {
		// Path stroke cloud
		dc.SetLineWidth(1.5 + rng.float01())
		dc.SetDash(5, 3)
		for i := 0; i < 6+int(lvl)*4; i++ {
			p := render.NewPath()
			x0 := 20 + float64(rng.intn(max(1, w-80)))
			y0 := 120 + float64(rng.intn(max(1, h-160)))
			p.MoveTo(x0, y0)
			p.LineTo(x0+40+float64(rng.intn(40)), y0+10)
			p.LineTo(x0+20, y0+40+float64(rng.intn(20)))
			p.Close()
			dc.SetRGB(0.25+rng.float01()*0.5, 0.4, 0.75)
			dc.AppendPath(p)
			_ = dc.Stroke()
		}
		dc.SetDash()

		// Circles + blend
		dc.SetBlendMode(render.BlendMultiply)
		dc.SetRGBA(1, 0.5, 0.3, 0.7)
		dc.DrawCircle(fw*0.7, fh*0.55, 30+float64(rng.intn(40)))
		_ = dc.Fill()
		dc.SetBlendMode(render.BlendNormal)
	}

	if lvl >= memSceneComplex {
		// Nested clip + layer + text
		dc.ClipRect(16, fh*0.45, fw-32, fh*0.4)
		dc.PushLayer(render.BlendNormal, 0.88)
		dc.SetRGB(0.2, 0.55, 0.9)
		dc.DrawRoundedRectangle(24, fh*0.48, fw-48, fh*0.32, 10)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		for i := 0; i < 5; i++ {
			dc.DrawString(fmt.Sprintf("layer-text row %d frame %d", i, frame), 40, fh*0.55+float64(i)*16)
		}
		if img != nil {
			dc.DrawImage(img, 40, fh*0.72)
		}
		dc.PopLayer()
		dc.ResetClip()

		// More text density
		dc.SetRGB(0.85, 0.88, 0.92)
		for i := 0; i < 8; i++ {
			dc.DrawString(fmt.Sprintf("dense-line-%02d-%d", i, frame%97), 12, 48+float64(i)*11)
		}

		// Blur intentionally omitted in mem soak: full-surface filter allocates large
		// intermediates and can false-trigger VRAM OOM / zero-op edge cases.
	}

	// Always a moving marker (dynamic content)
	tphase := float64(frame) * 0.17
	mx := fw*0.5 + math.Cos(tphase)*fw*0.25
	my := fh*0.35 + math.Sin(tphase)*fh*0.15
	dc.SetRGB(0.95, 0.35, 0.2)
	dc.DrawCircle(mx, my, 10)
	_ = dc.Fill()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func memMakeCheckerImage(t *testing.T, s int) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(s, s, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	// fill checker via SetPixel if available, else WritePixels-like
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			on := ((x/4)+(y/4))%2 == 0
			if on {
				_ = img.SetRGBA(x, y, 40, 140, 220, 255)
			} else {
				_ = img.SetRGBA(x, y, 220, 220, 230, 255)
			}
		}
	}
	return img
}

func memPresentOffscreen(t *testing.T, dc *render.Context, w, h int) {
	t.Helper()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer rel()
	// Do not ResetRenderPathStats here: draws already incremented GPUOps.
	if err := dc.PresentFrame(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("PresentFrame %dx%d: %v", w, h, err)
	}
	st := dc.RenderPathStats()
	if st.GPUOps == 0 {
		t.Fatalf("GPUOps==0 %s", st.LogLine())
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fallback_ops=%d", st.CPUFallbackOps)
	}
	memHardRSSCheck(t)
}
