//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// svgconformance runs the render/svg engine against the resvg test suite
// (github.com/RazrFalcon/resvg, MIT — corpus NOT committed; clone it and
// point -root at crates/resvg/tests/tests) and reports a per-group pass
// rate with graded pixel comparison.
//
// Grading (perceptual, not bit-exact — cross-renderer AA/色彩管理 differs):
//
//	identical: mean grid diff ≤ 3
//	close:     ≤ 24
//	far:       > 24
//	blank:     render is fully transparent while the expectation is not
//	error:     parse/render failure
//
// Usage:
//
//	go run ./scripts/svgconformance -root /tmp/resvg/crates/resvg/tests/tests \
//	    [-group text] [-font render/text/testdata/wqy-microhei.ttf] [-out report.md]
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/render/svg"
	"github.com/energye/gpui/render/text"

	// CPU filter implementations MUST be registered in this process: the
	// SVG filter family runs on the fn-ptr registration seam, and without
	// the side-effect import every Apply* silently no-ops (bias=9999 graded
	// far 204 on a working engine — the filter never ran).
	_ "github.com/energye/gpui/render/filters"
)

// gridN is the comparison grid: both images are downsampled to this many
// cells per side before diffing (absorbs AA and sub-pixel placement noise).
const gridN = 48

func main() {
	root := flag.String("root", "", "resvg tests/tests directory")
	group := flag.String("group", "", "optional single group to run")
	font := flag.String("font", "", "optional TTF to map generic font families to")
	out := flag.String("out", "", "optional markdown report path")
	perCasePath := flag.String("per-case", "", "optional file for per-case `<file> <grade> <delta>` lines")
	only := flag.String("only", "", "run only the case whose basename matches (diagnostics)")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "need -root")
		os.Exit(1)
	}
	if *font != "" {
		if src, err := text.NewFontSourceFromFile(*font); err == nil {
			svg.SetFontResolver(func(family string, weight text.FontWeight, italic bool) *text.FontSource {
				_, _ = weight, italic
				return src
			})
		}
	}

	groups, err := os.ReadDir(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	type job struct{ svgPath, pngPath, group string }
	var jobs []job
	for _, g := range groups {
		if !g.IsDir() {
			continue
		}
		if *group != "" && g.Name() != *group {
			continue
		}
		// Case depth is variable (<group>/<kind>/<case>/ or deeper for the
		// filter families): walk recursively and take every <name>.svg that
		// has a sibling <name>.png.
		_ = filepath.WalkDir(filepath.Join(*root, g.Name()), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".svg") {
				return nil
			}
			pngPath := strings.TrimSuffix(path, ".svg") + ".png"
			if _, err := os.Stat(pngPath); err == nil {
				if only == nil || *only == "" || filepath.Base(path) == *only {
					jobs = append(jobs, job{path, pngPath, g.Name()})
				}
			}
			return nil
		})
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].svgPath < jobs[j].svgPath })

	var mu sync.Mutex
	results := map[string]map[string]int{}
	worst := map[string][]string{}
	var perCase []string
	var done atomic.Int64
	start := time.Now()

	sem := make(chan struct{}, 1) // GPU accelerator is a process-wide singleton: concurrent offscreen contexts deadlock on it
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem; recover() }()
			grade, delta := runOne(j.svgPath, j.pngPath)
			if *perCasePath != "" {
				mu.Lock()
				perCase = append(perCase, fmt.Sprintf("%s %s %.2f\n", filepath.Base(j.svgPath), grade, delta))
				mu.Unlock()
			}
			done.Add(1)
			mu.Lock()
			if results[j.group] == nil {
				results[j.group] = map[string]int{}
			}
			results[j.group][grade]++
			if grade == "blank" || grade == "error" || grade == "far" {
				list := worst[j.group]
				if len(list) < 8 {
					worst[j.group] = append(list, filepath.Base(j.svgPath)+" ("+grade+")")
				}
			}
			mu.Unlock()
			if n := done.Load(); n%200 == 0 {
				fmt.Fprintf(os.Stderr, "\r%d/%d (%s)", n, len(jobs), time.Since(start).Round(time.Second))
			}
		}(j)
	}
	wg.Wait()
	fmt.Fprintln(os.Stderr)

	if *perCasePath != "" {
		sort.Strings(perCase)
		_ = os.WriteFile(*perCasePath, []byte(strings.Join(perCase, "")), 0o644)
	}

	// Report
	total := map[string]int{}
	order := []string{"identical", "close", "partial", "far", "blank", "error"}
	var b strings.Builder
	fmt.Fprintf(&b, "# SVG conformance vs resvg v0.42 suite\n\n- corpus: %s (%d cases)\n- grading: perceptual %dx%d grid, identical ≤3 / close ≤24 / partial ≤128 / far\n\n", *root, len(jobs), gridN, gridN)
	fmt.Fprintf(&b, "| group | total | identical | close | partial | far | blank | error | pass%%(id+close) |\n|---|---|---|---|---|---|---|---|---|\n")
	groupNames := make([]string, 0, len(results))
	for g := range results {
		groupNames = append(groupNames, g)
	}
	sort.Strings(groupNames)
	for _, g := range groupNames {
		m := results[g]
		n := 0
		for _, v := range m {
			n += v
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d | %d | %.1f%% |\n",
			g, n, m["identical"], m["close"], m["partial"], m["far"], m["blank"], m["error"],
			100*float64(m["identical"]+m["close"])/float64(n))
		for _, k := range order {
			total[k] += m[k]
		}
	}
	tn := 0
	for _, v := range total {
		tn += v
	}
	fmt.Fprintf(&b, "| **TOTAL** | %d | %d | %d | %d | %d | %d | %d | %.1f%% |\n\n",
		tn, total["identical"], total["close"], total["partial"], total["far"], total["blank"], total["error"],
		100*float64(total["identical"]+total["close"])/float64(tn))
	fmt.Fprintf(&b, "## worst cases (far/blank/error, up to 8 per group)\n\n")
	for _, g := range groupNames {
		if len(worst[g]) > 0 {
			fmt.Fprintf(&b, "### %s\n%s\n\n", g, strings.Join(worst[g], "\n"))
		}
	}
	report := b.String()
	fmt.Print(report)
	if *out != "" {
		_ = os.WriteFile(*out, []byte(report), 0o644)
	}
}

// runOne renders one case and grades it against the expected PNG.
func runOne(svgPath, pngPath string) (string, float64) {
	if os.Getenv("CONF_VERBOSE") != "" {
		fmt.Fprintln(os.Stderr, "CASE", svgPath)
	}
	data, err := os.ReadFile(svgPath)
	if err != nil {
		return "error", 0
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "PANIC %s: %v\n", svgPath, r)
		}
	}()
	doc, err := svg.ParseWithConfig(data, svg.ParseConfig{
		ResourcesDir: filepath.Dir(svgPath),
	})
	if err != nil {
		return "error", 0
	}
	want, err := loadPNG(pngPath)
	if err != nil {
		return "error", 0
	}
	bw := want.Bounds().Dx()
	bh := want.Bounds().Dy()
	if bw <= 0 || bh <= 0 || bw > 4096 || bh > 4096 {
		return "error", 0
	}
	img := doc.Render(bw, bh)
	if img == nil {
		return "error", 0
	}
	if isBlank(img) && !isBlank(want) {
		return "blank", 0
	}
	return grade(img, want)
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// isBlank reports a fully transparent image.
func isBlank(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0 {
				return false
			}
		}
	}
	return true
}

// grade downsamples both images to a gridN grid of premultiplied-agnostic
// RGBA means and returns the mean channel delta grade.
func grade(got, want image.Image) (string, float64) {
	g1 := gridMeans(got)
	g2 := gridMeans(want)
	sum, n := 0.0, 0
	for i := range g1 {
		for c := 0; c < 4; c++ {
			d := g1[i][c] - g2[i][c]
			if d < 0 {
				d = -d
			}
			sum += d
			n++
		}
	}
	mean := sum / float64(n)
	switch {
	case mean <= 3:
		return "identical", mean
	case mean <= 24:
		return "close", mean
	case mean <= 128:
		return "partial", mean
	default:
		return "far", mean
	}
}

// gridMeans averages each grid cell's RGBA (0..255 scale).
func gridMeans(img image.Image) [][4]float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([][4]float64, gridN*gridN)
	cnt := make([]int, gridN*gridN)
	for y := 0; y < h; y++ {
		gy := y * gridN / h
		for x := 0; x < w; x++ {
			gx := x * gridN / w
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := gy*gridN + gx
			out[i][0] += float64(r >> 8)
			out[i][1] += float64(g >> 8)
			out[i][2] += float64(bl >> 8)
			out[i][3] += float64(a >> 8)
			cnt[i]++
		}
	}
	for i := range out {
		if cnt[i] > 0 {
			for c := 0; c < 4; c++ {
				out[i][c] /= float64(cnt[i])
			}
		}
	}
	return out
}
