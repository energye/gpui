//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package svg

import (
	"encoding/json"
	"os"
	"testing"
)

// P3.8 G14 marker: SVG 1.1 marker-start/mid/end must render the referenced
// <marker> at path vertices. Current gap (red): render/svg only carries the
// feature string; <marker> is ignored and the marker-end line shows no marker.
func TestP38MarkerEndRenders(t *testing.T) {
	data, err := os.ReadFile("testdata/p38/marker/line_end.svg")
	if err != nil {
		t.Skipf("marker corpus missing: %v", err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(120, 120)
	red := 0
	for y := 45; y < 75; y++ {
		for x := 90; x < 115; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if r>>8 > 200 && g>>8 < 80 && b>>8 < 80 && a>>8 > 200 {
				red++
			}
		}
	}
	if red == 0 {
		t.Fatal("marker-end not rendered: no red marker pixels at the line end")
	}
}

type markerProbe struct {
	X, Y    int
	R, G, B uint8
	Tol     uint8
	A       string // "" / "opaque" / "clear"
	Label   string
}

type markerCase struct {
	Name   string        `json:"name"`
	SVG    string        `json:"svg"`
	Probes []markerProbe `json:"probes"`
}

// TestP38MarkerSemantics is the table-driven marker contract: host shapes,
// start/mid/end, closed paths, orient, markerUnits, ref, viewBox, overflow,
// context paint, shorthand, invalid refs and multiple subpaths. The corpus is
// testdata/p38/marker/cases.json (data file, not generated in the test).
func TestP38MarkerSemantics(t *testing.T) {
	data, err := os.ReadFile("testdata/p38/marker/cases.json")
	if err != nil {
		t.Skipf("marker cases missing: %v", err)
	}
	var cfg struct {
		Cases []markerCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("marker cases: %v", err)
	}
	for _, c := range cfg.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			doc, err := Parse([]byte(c.SVG))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			img := doc.Render(160, 120)
			for _, pr := range c.Probes {
				r, g, b, a := img.At(pr.X, pr.Y).RGBA()
				a8 := int(a >> 8)
				if pr.A == "clear" {
					if a8 > 40 {
						t.Errorf("%s (%d,%d): alpha=%d, want clear", pr.Label, pr.X, pr.Y, a8)
					}
					continue
				}
				if a8 < 200 {
					t.Errorf("%s (%d,%d): alpha=%d, want opaque", pr.Label, pr.X, pr.Y, a8)
					continue
				}
				if absInt(int(r>>8)-int(pr.R)) > int(pr.Tol) ||
					absInt(int(g>>8)-int(pr.G)) > int(pr.Tol) ||
					absInt(int(b>>8)-int(pr.B)) > int(pr.Tol) {
					t.Errorf("%s (%d,%d): got=(%d,%d,%d), want=(%d,%d,%d)±%d",
						pr.Label, pr.X, pr.Y, r>>8, g>>8, b>>8, pr.R, pr.G, pr.B, pr.Tol)
				}
			}
		})
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
