//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package preview records WYSIWYG preview frames (level/particle) and
// breaks one frame down to named nodes for budget triage.
//
// The package is intentionally self-contained (stdlib only): callers hand
// one frame's pixels plus its node table to Record, which drops a
// JSON (node table) and a PNG (pixels) into the target directory.
// Breakdown then splits cost/pixel shares per node so the report can
// point at the node by name; nodes over budget carry OverBudget (red).
package preview

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxNameLen caps frame and node names kept in one record.
const MaxNameLen = 64

// MaxPixels caps one frame (32MP RGBA) so a bad size fails fast.
const MaxPixels = 32 << 20

// Node is one cost/pixel contributor inside a frame, located by Name.
type Node struct {
	Name   string  `json:"name"`
	CostMs float64 `json:"cost_ms"`
	Pixels int64   `json:"pixels"`
}

// NewNode builds one node. Empty or overlong names, negative cost, and
// negative pixels are errors and store nothing.
func NewNode(name string, costMs float64, pixels int64) (Node, error) {
	if name == "" || len(name) > MaxNameLen {
		return Node{}, fmt.Errorf("preview.NewNode: bad name %q", name)
	}
	if costMs < 0 {
		return Node{}, fmt.Errorf("preview.NewNode: negative cost %v", costMs)
	}
	if pixels < 0 {
		return Node{}, fmt.Errorf("preview.NewNode: negative pixels %d", pixels)
	}
	return Node{Name: name, CostMs: costMs, Pixels: pixels}, nil
}

// Frame is one preview frame: RGBA pixels plus the node table.
// Pixels stay out of JSON (the PNG file carries them); Nodes marshal.
type Frame struct {
	Name   string
	Width  int
	Height int
	Pixels []byte // RGBA, len Width*Height*4.
	Nodes  []Node
}

// NewFrame builds one frame. Bad sizes, pixel length mismatches, and bad
// nodes are errors and store nothing.
func NewFrame(name string, width, height int, px []byte, nodes []Node) (Frame, error) {
	if name == "" || len(name) > MaxNameLen {
		return Frame{}, fmt.Errorf("preview.NewFrame: bad name %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return Frame{}, fmt.Errorf("preview.NewFrame: name %q must not carry paths", name)
	}
	if width <= 0 || height <= 0 {
		return Frame{}, fmt.Errorf("preview.NewFrame: bad size %dx%d", width, height)
	}
	if int64(width)*int64(height) > MaxPixels {
		return Frame{}, fmt.Errorf("preview.NewFrame: size %dx%d over cap", width, height)
	}
	if len(px) != width*height*4 {
		return Frame{}, fmt.Errorf("preview.NewFrame: pixels %d want %d", len(px), width*height*4)
	}
	cpNodes := append([]Node(nil), nodes...)
	if cpNodes == nil {
		cpNodes = []Node{}
	}
	for i := range cpNodes {
		if _, err := NewNode(cpNodes[i].Name, cpNodes[i].CostMs, cpNodes[i].Pixels); err != nil {
			return Frame{}, err
		}
	}
	return Frame{Name: name, Width: width, Height: height, Pixels: append([]byte(nil), px...), Nodes: cpNodes}, nil
}

// recordJSON is the JSON file shape: meta plus the node table.
type recordJSON struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Nodes  []Node `json:"nodes"`
}

// Record drops one frame as <dir>/<name>.json (node table) and
// <dir>/<name>.png (pixels), creating dir as needed. It returns both
// paths. Empty dir or frame names are errors.
func Record(dir string, f Frame) (jsonPath, pngPath string, err error) {
	if dir == "" {
		return "", "", errors.New("preview.Record: empty dir")
	}
	if _, err := NewFrame(f.Name, f.Width, f.Height, f.Pixels, f.Nodes); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("preview.Record: mkdir %s: %w", dir, err)
	}
	nodes := append([]Node(nil), f.Nodes...)
	if nodes == nil {
		nodes = []Node{}
	}
	raw, err := json.MarshalIndent(recordJSON{Name: f.Name, Width: f.Width, Height: f.Height, Nodes: nodes}, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("preview.Record: encode: %w", err)
	}
	jsonPath = filepath.Join(dir, f.Name+".json")
	pngPath = filepath.Join(dir, f.Name+".png")
	if err := os.WriteFile(jsonPath, raw, 0o600); err != nil {
		return "", "", fmt.Errorf("preview.Record: write json: %w", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, f.Width, f.Height))
	copy(img.Pix, f.Pixels)
	fh, err := os.Create(pngPath)
	if err != nil {
		return "", "", fmt.Errorf("preview.Record: create png: %w", err)
	}
	if err := png.Encode(fh, img); err != nil {
		_ = fh.Close()
		return "", "", fmt.Errorf("preview.Record: encode png: %w", err)
	}
	if err := fh.Close(); err != nil {
		return "", "", fmt.Errorf("preview.Record: close png: %w", err)
	}
	return jsonPath, pngPath, nil
}

// Row is one node's share of a frame: cost and pixel fractions plus the
// over-budget (red) flag. Exported so reports marshal directly.
type Row struct {
	Name       string  `json:"name"`
	CostMs     float64 `json:"cost_ms"`
	CostShare  float64 `json:"cost_share"`
	PixelShare float64 `json:"pixel_share"`
	OverBudget bool    `json:"over_budget"`
}

// Breakdown splits one frame by node, sorted by cost descending (name
// breaks ties) so index 0 names who eats the frame. CostShare divides by
// total node cost and PixelShare by total node pixels (0 when empty).
// Nodes above budgetMs carry OverBudget (red); budgetMs <= 0 disables it.
func Breakdown(f Frame, budgetMs float64) []Row {
	var totalCost float64
	var totalPixels int64
	for _, n := range f.Nodes {
		totalCost += n.CostMs
		totalPixels += n.Pixels
	}
	rows := make([]Row, 0, len(f.Nodes))
	for _, n := range f.Nodes {
		costShare := 0.0
		if totalCost > 0 {
			costShare = n.CostMs / totalCost
		}
		pixelShare := 0.0
		if totalPixels > 0 {
			pixelShare = float64(n.Pixels) / float64(totalPixels)
		}
		rows = append(rows, Row{
			Name:       n.Name,
			CostMs:     n.CostMs,
			CostShare:  costShare,
			PixelShare: pixelShare,
			OverBudget: budgetMs > 0 && n.CostMs > budgetMs,
		})
	}
	sort.Slice(rows, func(a, b int) bool {
		if rows[a].CostMs != rows[b].CostMs {
			return rows[a].CostMs > rows[b].CostMs
		}
		return rows[a].Name < rows[b].Name
	})
	return rows
}

const (
	redOpen  = "\x1b[31m"
	redClose = "\x1b[0m"
)

// Text renders rows for the manual preview: one line per node with cost,
// shares, and the budget verdict. Over-budget rows wrap in red and end
// with OVER so the eye (and the test) lands on the node by name.
func Text(rows []Row) string {
	var b strings.Builder
	for i, r := range rows {
		line := fmt.Sprintf("%s cost=%.3fms time=%.1f%% pixels=%.1f%%", r.Name, r.CostMs, r.CostShare*100, r.PixelShare*100)
		if r.OverBudget {
			line += " OVER"
			line = redOpen + line + redClose
		} else {
			line += " ok"
		}
		b.WriteString(line)
		if i+1 < len(rows) {
			b.WriteString("\n")
		}
	}
	if len(rows) > 0 {
		b.WriteString("\n")
	}
	return b.String()
}
