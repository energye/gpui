//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package conformance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// Report is the per-scene, per-backend evidence document.
type Report struct {
	Backend     string          `json:"backend"`
	Scene       string          `json:"scene"`
	Status      string          `json:"status"` // ok | unavailable
	Reason      string          `json:"reason,omitempty"`
	Width       int             `json:"width,omitempty"`
	Height      int             `json:"height,omitempty"`
	ImageSHA256 string          `json:"image_sha256,omitempty"`
	Samples     []SampleResult  `json:"samples,omitempty"`
	Regions     []RegionResult  `json:"regions,omitempty"`
	Path        *PathCheck      `json:"path,omitempty"`
	Resource    *ResourceResult `json:"resource,omitempty"`
	OK          bool            `json:"ok"`
}

// SampleResult records one pixel assertion.
type SampleResult struct {
	Name     string   `json:"name"`
	X        int      `json:"x"`
	Y        int      `json:"y"`
	WantRGBA [4]uint8 `json:"want_rgba"`
	GotRGBA  [4]uint8 `json:"got_rgba"`
	Tol      uint8    `json:"tol"`
	OK       bool     `json:"ok"`
}

// RegionResult records one area assertion.
type RegionResult struct {
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	GotRatio   float64 `json:"got_ratio,omitempty"`
	GotBBox    []int   `json:"got_bbox,omitempty"`
	MinRatio   float64 `json:"min_ratio,omitempty"`
	MaxRatio   float64 `json:"max_ratio,omitempty"`
	TolPP      float64 `json:"tol_pp,omitempty"`
	ExpectBBox []int   `json:"expect_bbox,omitempty"`
	BBoxTol    int     `json:"bbox_tol,omitempty"`
	OK         bool    `json:"ok"`
}

// PathStats mirrors render.RenderPathStats for the counters this harness
// asserts on.
type PathStats struct {
	GPUOps                int
	CPUFallbackOps        int
	FrameFlushes          int
	LastCPUFallbackReason string
}

// PathCheck records whether the scene actually ran on the GPU path.
type PathCheck struct {
	GPUOps             int    `json:"gpu_ops"`
	CPUFallbackOps     int    `json:"cpu_fallback_ops"`
	FrameFlushes       int    `json:"frame_flushes"`
	LastFallbackReason string `json:"last_cpu_fallback_reason,omitempty"`
	MinGPUOps          int    `json:"min_gpu_ops"`
	MaxCPUFallbackOps  int    `json:"max_cpu_fallback_ops"`
	GPUOpsOK           bool   `json:"gpu_ops_ok"`
	FallbackOK         bool   `json:"fallback_ok"`
	OK                 bool   `json:"ok"`
}

// ResourceResult records the VRAM ledger across repeated context runs.
type ResourceResult struct {
	Repeats             int      `json:"repeats"`
	LiveCount           []int    `json:"live_count"`
	LiveBytes           []uint64 `json:"live_bytes"`
	SteadyCountDelta    int      `json:"steady_count_delta"`
	SteadyBytesDelta    int64    `json:"steady_bytes_delta"`
	MaxSteadyCountDelta int      `json:"max_steady_count_delta"`
	MaxSteadyBytesDelta int64    `json:"max_steady_bytes_delta"`
	GPUContextsAfter    []int    `json:"gpu_contexts_after"`
	OK                  bool     `json:"ok"`
}

// Unavailable builds a report for a backend that cannot run in this
// environment. The reason is mandatory — a silent skip is not allowed.
func Unavailable(backend, scene, reason string) *Report {
	return &Report{Backend: backend, Scene: scene, Status: "unavailable", Reason: reason}
}

// Evaluate compares img against the case assertions for one backend.
// resource may be nil (comparator unit tests).
func Evaluate(backend string, c *Case, img image.Image, path PathStats, resource *ResourceResult) *Report {
	rep := &Report{
		Backend:     backend,
		Scene:       c.Name,
		Status:      "ok",
		Width:       c.Width,
		Height:      c.Height,
		ImageSHA256: imageSHA256(img),
		Resource:    resource,
	}

	for _, s := range c.Samples {
		got := straightPixel(img, s.X, s.Y)
		rep.Samples = append(rep.Samples, SampleResult{
			Name:     s.Name,
			X:        s.X,
			Y:        s.Y,
			WantRGBA: s.RGBA,
			GotRGBA:  got,
			Tol:      s.Tol,
			OK:       within(got, s.RGBA, s.Tol),
		})
	}

	for _, r := range c.Regions {
		rep.Regions = append(rep.Regions, evalRegion(img, &r))
	}

	rep.Path = evalPath(c.Expect, path)

	ok := rep.Path.OK
	if resource != nil {
		ok = ok && resource.OK
	}
	for i := range rep.Samples {
		ok = ok && rep.Samples[i].OK
	}
	for i := range rep.Regions {
		ok = ok && rep.Regions[i].OK
	}
	rep.OK = ok
	return rep
}

// Failed lists every failed assertion with its name, for evidence and for
// the orchestrator's failure message.
func (r *Report) Failed() []string {
	if r.Status != "ok" {
		return []string{"status=" + r.Status + " reason=" + r.Reason}
	}
	var out []string
	if r.Path != nil && !r.Path.OK {
		if !r.Path.GPUOpsOK {
			out = append(out, fmt.Sprintf("path: gpu_ops=%d < min=%d", r.Path.GPUOps, r.Path.MinGPUOps))
		}
		if !r.Path.FallbackOK {
			out = append(out, fmt.Sprintf("path: cpu_fallback_ops=%d > max=%d reason=%q", r.Path.CPUFallbackOps, r.Path.MaxCPUFallbackOps, r.Path.LastFallbackReason))
		}
	}
	for _, s := range r.Samples {
		if !s.OK {
			out = append(out, fmt.Sprintf("sample %s @(%d,%d): got %v want %v ±%d", s.Name, s.X, s.Y, s.GotRGBA, s.WantRGBA, s.Tol))
		}
	}
	for _, g := range r.Regions {
		if !g.OK {
			if g.Kind == RegionCoverage {
				out = append(out, fmt.Sprintf("region %s: coverage=%.4f want [%.2f,%.2f] ±%.1fpp", g.Name, g.GotRatio, g.MinRatio, g.MaxRatio, g.TolPP))
			} else {
				out = append(out, fmt.Sprintf("region %s: bbox=%v want %v ±%d", g.Name, g.GotBBox, g.ExpectBBox, g.BBoxTol))
			}
		}
	}
	if r.Resource != nil && !r.Resource.OK {
		out = append(out, fmt.Sprintf("resource: steady count delta=%d (max %d) bytes delta=%d (max %d) contexts=%v",
			r.Resource.SteadyCountDelta, r.Resource.MaxSteadyCountDelta,
			r.Resource.SteadyBytesDelta, r.Resource.MaxSteadyBytesDelta,
			r.Resource.GPUContextsAfter))
	}
	return out
}

func evalPath(expect Expect, p PathStats) *PathCheck {
	pc := &PathCheck{
		GPUOps:             p.GPUOps,
		CPUFallbackOps:     p.CPUFallbackOps,
		FrameFlushes:       p.FrameFlushes,
		LastFallbackReason: p.LastCPUFallbackReason,
		MinGPUOps:          expect.MinGPUOps,
		MaxCPUFallbackOps:  expect.MaxCPUFallbackOps,
	}
	pc.GPUOpsOK = p.GPUOps >= expect.MinGPUOps
	pc.FallbackOK = p.CPUFallbackOps <= expect.MaxCPUFallbackOps
	pc.OK = pc.GPUOpsOK && pc.FallbackOK
	return pc
}

func evalRegion(img image.Image, r *Region) RegionResult {
	b := img.Bounds()
	x0 := clampInt(int(r.Rect[0]), b.Min.X, b.Max.X)
	y0c := clampInt(int(r.Rect[1]), b.Min.Y, b.Max.Y)
	x1 := clampInt(int(r.Rect[0]+r.Rect[2]), b.Min.X, b.Max.X)
	y1 := clampInt(int(r.Rect[1]+r.Rect[3]), b.Min.Y, b.Max.Y)
	if x1 < x0 || y1 < y0c {
		return RegionResult{Name: r.Name, Kind: r.Kind, OK: false}
	}

	total := 0
	matched := 0
	minX, minY, maxX, maxY := -1, -1, -1, -1
	for y := y0c; y < y1; y++ {
		for x := x0; x < x1; x++ {
			px := straightPixel(img, x, y)
			hi := px[3] > r.AlphaAbove
			if r.MatchRGBA != nil {
				hi = within(px, *r.MatchRGBA, r.MatchTol)
			}
			if hi {
				matched++
				if minX < 0 || x < minX {
					minX = x
				}
				if y < minY || minY < 0 {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
			total++
		}
	}

	switch r.Kind {
	case RegionCoverage:
		ratio := 0.0
		if total > 0 {
			ratio = float64(matched) / float64(total)
		}
		lo := r.MinRatio - r.TolPP/100
		hi := r.MaxRatio + r.TolPP/100
		if lo < 0 {
			lo = 0
		}
		if hi > 1 {
			hi = 1
		}
		return RegionResult{
			Name: r.Name, Kind: r.Kind,
			GotRatio: ratio,
			MinRatio: r.MinRatio, MaxRatio: r.MaxRatio, TolPP: r.TolPP,
			OK: total > 0 && ratio >= lo && ratio <= hi,
		}

	case RegionBBox:
		got := []int(nil)
		if matched > 0 {
			got = []int{minX, minY, maxX, maxY}
		}
		ok := matched > 0
		if ok {
			for i := 0; i < 4; i++ {
				if absInt(got[i]-r.ExpectBBox[i]) > r.BBoxTol {
					ok = false
				}
			}
		}
		return RegionResult{
			Name: r.Name, Kind: r.Kind,
			GotBBox: got, ExpectBBox: r.ExpectBBox, BBoxTol: r.BBoxTol,
			OK: ok,
		}
	}
	return RegionResult{Name: r.Name, Kind: r.Kind, OK: false}
}

// straightPixel reads one pixel and unpremultiplies it to straight 8-bit
// RGBA (image.Image.RGBA() is alpha-premultiplied by contract).
func straightPixel(img image.Image, x, y int) [4]uint8 {
	r, g, b, a := img.At(x, y).RGBA()
	if a == 0 {
		return [4]uint8{}
	}
	a8 := a >> 8
	if a8 == 0 {
		return [4]uint8{0, 0, 0, 0}
	}
	den := a8 * 257
	un := func(v uint32) uint8 {
		if v == 0 {
			return 0
		}
		return uint8(v * 255 / den)
	}
	return [4]uint8{un(r), un(g), un(b), uint8(a8)}
}

func within(got [4]uint8, want [4]uint8, tol uint8) bool {
	for i := 0; i < 4; i++ {
		d := int(got[i]) - int(want[i])
		if d < 0 {
			d = -d
		}
		if d > int(tol) {
			return false
		}
	}
	return true
}

func imageSHA256(img image.Image) string {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])
}

// WriteReport writes rep as indented JSON, creating parent directories.
func WriteReport(path string, rep *Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// WriteFramePNG saves the rendered frame next to the report for review.
func WriteFramePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
