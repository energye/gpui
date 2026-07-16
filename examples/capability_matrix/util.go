//go:build linux && !nogpu

package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func rssKB() int64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			var n int64
			_, _ = fmt.Sscanf(strings.TrimSpace(line[6:]), "%d", &n)
			return n
		}
	}
	return 0
}

type cpuSample struct {
	utime, stime uint64
	wall         time.Time
}

func readCPUSample() (cpuSample, bool) {
	b, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return cpuSample{}, false
	}
	s := string(b)
	rp := strings.LastIndex(s, ")")
	if rp < 0 || rp+2 >= len(s) {
		return cpuSample{}, false
	}
	fields := strings.Fields(s[rp+2:])
	if len(fields) < 13 {
		return cpuSample{}, false
	}
	ut, err1 := strconv.ParseUint(fields[11], 10, 64)
	st, err2 := strconv.ParseUint(fields[12], 10, 64)
	if err1 != nil || err2 != nil {
		return cpuSample{}, false
	}
	return cpuSample{utime: ut, stime: st, wall: time.Now()}, true
}

func hostCPUCount() int {
	n := runtime.NumCPU()
	if n < 1 {
		return 1
	}
	return n
}

func cpuPercent(prev, cur cpuSample) (float64, bool) {
	dt := cur.wall.Sub(prev.wall).Seconds()
	if dt <= 1e-4 {
		return 0, false
	}
	const ticksPerSec = 100.0
	deltaTicks := float64((cur.utime + cur.stime) - (prev.utime + prev.stime))
	pct := (deltaTicks / ticksPerSec) / dt * 100.0
	if pct < 0 {
		pct = 0
	}
	if pct > 1000 {
		pct = 1000
	}
	return pct, true
}

func hsv(h, s, v float64) (r, g, b float64) {
	h = math.Mod(h, 1)
	if h < 0 {
		h += 1
	}
	i := math.Floor(h * 6)
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - f*s)
	u := v * (1 - (1-f)*s)
	switch int(i) % 6 {
	case 0:
		return v, u, p
	case 1:
		return q, v, p
	case 2:
		return p, v, u
	case 3:
		return p, q, v
	case 4:
		return u, p, v
	default:
		return v, p, q
	}
}

// fontPack: Latin + CJK MultiFace (X.06).
// DroidSansFallback reports HasGlyph=false for A-Z/a-z/0-9 — Latin must come
// from DejaVu (or similar), with CJK as fallback. Never use CJK-only for mixed UI.
type fontPack struct {
	sans, latin string
	srcLatin    *text.FontSource
	srcCJK      *text.FontSource
	ok          bool

	// cached MultiFace by size (pt)
	faceCache map[int]text.Face
}

func findFirstExisting(paths ...string) string {
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func loadFonts(dc *render.Context) fontPack {
	cjk := findFirstExisting(
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/truetype/droid/DroidSansFallback.ttf",
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
	)
	latin := findFirstExisting(
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	)
	fp := fontPack{
		sans: cjk, latin: latin,
		ok:        cjk != "" || latin != "",
		faceCache: make(map[int]text.Face),
	}
	if fp.sans == "" {
		fp.sans = latin
	}
	if fp.latin == "" {
		fp.latin = fp.sans
	}
	if latin != "" {
		if src, err := text.NewFontSourceFromFile(latin); err != nil {
			log.Printf("latin font: %v", err)
		} else {
			fp.srcLatin = src
		}
	}
	if cjk != "" && cjk != latin {
		if src, err := text.NewFontSourceFromFile(cjk); err != nil {
			log.Printf("cjk font: %v", err)
		} else {
			fp.srcCJK = src
		}
	}
	if dc != nil {
		ensureMixedFont(dc, fp, 16)
	}
	log.Printf("fonts latin=%s cjk=%s multi=%v", fp.latin, fp.sans, fp.srcLatin != nil || fp.srcCJK != nil)
	return fp
}

// faceAt builds MultiFace: Latin first, CJK fallback (Skia-style fallback chain).
func (fp fontPack) faceAt(size float64) text.Face {
	key := int(math.Round(size * 100))
	if fp.faceCache != nil {
		if f, ok := fp.faceCache[key]; ok {
			return f
		}
	}
	var faces []text.Face
	if fp.srcLatin != nil {
		faces = append(faces, fp.srcLatin.Face(size))
	}
	if fp.srcCJK != nil {
		faces = append(faces, fp.srcCJK.Face(size))
	}
	if len(faces) == 0 {
		return nil
	}
	if len(faces) == 1 {
		if fp.faceCache != nil {
			fp.faceCache[key] = faces[0]
		}
		return faces[0]
	}
	mf, err := text.NewMultiFace(faces...)
	if err != nil {
		log.Printf("NewMultiFace: %v", err)
		return faces[0]
	}
	if fp.faceCache != nil {
		fp.faceCache[key] = mf
	}
	return mf
}

func ensureMixedFont(dc *render.Context, fonts fontPack, size float64) {
	if dc == nil {
		return
	}
	if f := fonts.faceAt(size); f != nil {
		dc.SetFont(f)
		return
	}
	// Last resort single-face load
	path := fonts.latin
	if path == "" {
		path = fonts.sans
	}
	if path != "" {
		_ = dc.LoadFontFace(path, size)
	}
}

// ensureFont keeps the old signature used by probes; always applies mixed face.
func ensureFont(dc *render.Context, path string, size float64) {
	// path ignored when MultiFace available — kept for call-site compatibility.
	_ = path
	if dc == nil {
		return
	}
	// Callers may not have fonts pack; fall back to path load.
	if path != "" {
		_ = dc.LoadFontFace(path, size)
	}
}

// ensureFontPack is the preferred entry: Latin+CJK MultiFace at size.
func ensureFontPack(dc *render.Context, fonts fontPack, size float64) {
	ensureMixedFont(dc, fonts, size)
}
