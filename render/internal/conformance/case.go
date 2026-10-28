//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package conformance runs JSON-authored drawing scenes on a render.Context
// and evaluates semantic assertions (sample colors, region coverage, glyph
// bounding boxes, GPU path counters, VRAM ledger steadiness) against the
// result image.
//
// The package is test-only: it is imported by render/conformance_test.go and
// never by library code. Scenes and expectations live in
// render/testdata/conformance/*.json so the test bodies carry no standard
// data.
package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Op kinds understood by Draw.
const (
	OpClear          = "clear"
	OpFillRect       = "fill_rect"
	OpLinearGradient = "linear_gradient"
	OpRadialGradient = "radial_gradient"
	OpClipRect       = "clip_rect"
	OpClipRoundRect  = "clip_round_rect"
	OpPushLayer      = "push_layer"
	OpPopLayer       = "pop_layer"
	OpDrawImage      = "draw_image"
	OpText           = "text"
)

// Region kinds understood by Evaluate.
const (
	RegionCoverage = "coverage"
	RegionBBox     = "bbox"
)

// Color is straight (non-premultiplied) RGBA in 0..1.
type Color [4]float64

// Stop is one gradient color stop.
type Stop struct {
	Offset float64 `json:"offset"`
	Color  Color   `json:"color"`
}

// Op is one drawing step. Fields are populated per Kind; see Validate.
type Op struct {
	Kind    string     `json:"kind"`
	Rect    [4]float64 `json:"rect"`
	Color   *Color     `json:"color,omitempty"`
	From    [2]float64 `json:"from"`
	To      [2]float64 `json:"to"`
	Center  [2]float64 `json:"center"`
	Radius  float64    `json:"radius"`
	Stops   []Stop     `json:"stops"`
	Path    string     `json:"path"`
	X       float64    `json:"x"`
	Y       float64    `json:"y"`
	Text    string     `json:"text"`
	Font    string     `json:"font"`
	Size    float64    `json:"size"`
	Opacity float64    `json:"opacity"`
	Blend   string     `json:"blend"`
}

// Sample asserts the straight RGBA at one pixel.
type Sample struct {
	Name string   `json:"name"`
	X    int      `json:"x"`
	Y    int      `json:"y"`
	RGBA [4]uint8 `json:"rgba"`
	Tol  uint8    `json:"tol"`
}

// Region asserts an area property:
//
//   - coverage: fraction of pixels inside Rect that either alpha-match
//     (AlphaAbove) or color-match (MatchRGBA) must fall in
//     [MinRatio-TolPP/100, MaxRatio+TolPP/100].
//   - bbox: the inclusive pixel bounding box of pixels whose alpha is above
//     AlphaAbove inside Rect must match ExpectBBox within BBoxTol pixels.
type Region struct {
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Rect       [4]float64 `json:"rect"`
	AlphaAbove uint8      `json:"alpha_above"`
	MatchRGBA  *[4]uint8  `json:"match_rgba,omitempty"`
	MatchTol   uint8      `json:"match_tol"`
	MinRatio   float64    `json:"min_ratio"`
	MaxRatio   float64    `json:"max_ratio"`
	TolPP      float64    `json:"tol_pp"`
	ExpectBBox []int      `json:"expect_bbox,omitempty"`
	BBoxTol    int        `json:"bbox_tol"`
}

// Expect declares the GPU-path budget for a scene.
type Expect struct {
	MinGPUOps         int `json:"min_gpu_ops"`
	MaxCPUFallbackOps int `json:"max_cpu_fallback_ops"`
}

// Resource declares how many identical context runs feed the ledger check.
// The steady deltas are upper bounds; a non-zero bound records a known,
// measured divergence (echoed by summary.md) and must be tightened to zero
// once the owning task fixes it.
type Resource struct {
	Repeats             int   `json:"repeats"`
	MaxSteadyCountDelta int   `json:"max_steady_count_delta"`
	MaxSteadyBytesDelta int64 `json:"max_steady_bytes_delta"`
}

// Case is one scene plus its expectations.
type Case struct {
	Name     string   `json:"name"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Notes    string   `json:"notes"`
	Ops      []Op     `json:"ops"`
	Samples  []Sample `json:"samples"`
	Regions  []Region `json:"regions"`
	Expect   Expect   `json:"expect"`
	Resource Resource `json:"resource"`
}

// LoadCase reads and validates one case file.
func LoadCase(path string) (*Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Case
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

// LoadCases loads every *.json directly under dir (subdirectories are
// fixture storage and stay untouched), sorted by scene name.
func LoadCases(dir string) ([]*Case, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no cases under %s", dir)
	}
	cases := make([]*Case, 0, len(paths))
	for _, p := range paths {
		c, err := LoadCase(p)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].Name < cases[j].Name })
	return cases, nil
}

// Validate rejects malformed cases before any GPU work happens.
func (c *Case) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("missing name")
	}
	if c.Width <= 0 || c.Height <= 0 {
		return fmt.Errorf("%s: width/height must be positive", c.Name)
	}
	if len(c.Samples) == 0 && len(c.Regions) == 0 {
		return fmt.Errorf("%s: no assertions", c.Name)
	}
	for i := range c.Ops {
		op := &c.Ops[i]
		switch op.Kind {
		case OpClear, OpFillRect:
			if op.Color == nil {
				return fmt.Errorf("%s: op %d (%s) missing color", c.Name, i, op.Kind)
			}
		case OpLinearGradient:
			if len(op.Stops) < 2 {
				return fmt.Errorf("%s: op %d (linear_gradient) needs >=2 stops", c.Name, i)
			}
		case OpRadialGradient:
			if len(op.Stops) < 2 || op.Radius <= 0 {
				return fmt.Errorf("%s: op %d (radial_gradient) needs >=2 stops and radius>0", c.Name, i)
			}
		case OpClipRect, OpClipRoundRect:
			if op.Rect[2] <= 0 || op.Rect[3] <= 0 {
				return fmt.Errorf("%s: op %d (%s) needs positive w/h", c.Name, i, op.Kind)
			}
		case OpPushLayer:
			if op.Blend != "" && op.Blend != "normal" {
				return fmt.Errorf("%s: op %d unsupported blend %q", c.Name, i, op.Blend)
			}
			if op.Opacity < 0 || op.Opacity > 1 {
				return fmt.Errorf("%s: op %d opacity %v outside [0,1]", c.Name, i, op.Opacity)
			}
		case OpPopLayer:
		case OpDrawImage:
			if op.Path == "" {
				return fmt.Errorf("%s: op %d (draw_image) missing path", c.Name, i)
			}
		case OpText:
			if op.Text == "" || op.Font == "" || op.Size <= 0 {
				return fmt.Errorf("%s: op %d (text) needs text/font/size", c.Name, i)
			}
		default:
			return fmt.Errorf("%s: op %d unknown kind %q", c.Name, i, op.Kind)
		}
	}
	if c.Expect.MinGPUOps < 0 || c.Expect.MaxCPUFallbackOps < 0 {
		return fmt.Errorf("%s: expect bounds must be >= 0", c.Name)
	}
	if c.Resource.Repeats < 0 || c.Resource.MaxSteadyCountDelta < 0 || c.Resource.MaxSteadyBytesDelta < 0 {
		return fmt.Errorf("%s: resource bounds must be >= 0", c.Name)
	}
	for i := range c.Samples {
		s := &c.Samples[i]
		if s.Name == "" {
			return fmt.Errorf("%s: sample %d missing name", c.Name, i)
		}
		if s.X < 0 || s.Y < 0 || s.X >= c.Width || s.Y >= c.Height {
			return fmt.Errorf("%s: sample %s (%d,%d) outside %dx%d", c.Name, s.Name, s.X, s.Y, c.Width, c.Height)
		}
	}
	for i := range c.Regions {
		r := &c.Regions[i]
		if r.Name == "" {
			return fmt.Errorf("%s: region %d missing name", c.Name, i)
		}
		switch r.Kind {
		case RegionCoverage:
			if r.MinRatio < 0 || r.MaxRatio > 1 || r.MinRatio > r.MaxRatio {
				return fmt.Errorf("%s: region %s ratio range [%v,%v] invalid", c.Name, r.Name, r.MinRatio, r.MaxRatio)
			}
		case RegionBBox:
			if len(r.ExpectBBox) != 4 {
				return fmt.Errorf("%s: region %s expect_bbox must have 4 values", c.Name, r.Name)
			}
		default:
			return fmt.Errorf("%s: region %s unknown kind %q", c.Name, r.Name, r.Kind)
		}
	}
	return nil
}
