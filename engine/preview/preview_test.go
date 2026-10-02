//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package preview

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type nodeCase struct {
	Name   string  `json:"name"`
	CostMs float64 `json:"cost_ms"`
	Pixels int64   `json:"pixels"`
}

type frameCase struct {
	Name     string     `json:"name"`
	Width    int        `json:"width"`
	Height   int        `json:"height"`
	Pixels   []byte     `json:"pixels"`
	Nodes    []nodeCase `json:"nodes"`
	BudgetMs float64    `json:"budget_ms"`
	WantTop  string     `json:"want_top"`
	WantOver []string   `json:"want_over"`
}

type casesFile struct {
	Version  int         `json:"version"`
	BudgetMs float64     `json:"budget_ms"`
	Frames   []frameCase `json:"frames"`
}

func loadPreviewCases(t *testing.T) casesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "preview_cases.json"))
	if err != nil {
		t.Fatalf("read preview_cases.json: %v", err)
	}
	var c casesFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode preview_cases.json: %v", err)
	}
	if len(c.Frames) == 0 {
		t.Fatal("preview_cases.json has no frames")
	}
	return c
}

// buildFrame replays one frozen case through the frozen constructors
// without re-spelling its numbers: the file owns the data.
func buildFrame(t *testing.T, c frameCase) Frame {
	t.Helper()
	nodes := make([]Node, len(c.Nodes))
	for i, n := range c.Nodes {
		one, err := NewNode(n.Name, n.CostMs, n.Pixels)
		if err != nil {
			t.Fatalf("%s: NewNode %q: %v", c.Name, n.Name, err)
		}
		nodes[i] = one
	}
	f, err := NewFrame(c.Name, c.Width, c.Height, c.Pixels, nodes)
	if err != nil {
		t.Fatalf("%s: NewFrame: %v", c.Name, err)
	}
	return f
}

func findRow(rows []Row, name string) (Row, bool) {
	for _, r := range rows {
		if r.Name == name {
			return r, true
		}
	}
	return Row{}, false
}

func closeFloat(t *testing.T, what string, got, want float64) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > 1e-9 {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

// A:录制落盘,JSON 节点表加 PNG 像素都写到指定目录.
func TestRecordWritesFiles(t *testing.T) {
	c := loadPreviewCases(t)
	for _, fc := range c.Frames {
		f := buildFrame(t, fc)
		dir := t.TempDir()
		jpath, ppath, err := Record(dir, f)
		if err != nil {
			t.Fatalf("%s: Record: %v", fc.Name, err)
		}
		if jpath != filepath.Join(dir, fc.Name+".json") {
			t.Errorf("%s: json path = %q", fc.Name, jpath)
		}
		if ppath != filepath.Join(dir, fc.Name+".png") {
			t.Errorf("%s: png path = %q", fc.Name, ppath)
		}
		raw, err := os.ReadFile(jpath)
		if err != nil {
			t.Fatalf("%s: read json: %v", fc.Name, err)
		}
		var back recordJSON
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("%s: decode json: %v", fc.Name, err)
		}
		if back.Name != fc.Name || back.Width != fc.Width || back.Height != fc.Height {
			t.Errorf("%s: json meta = %v/%dx%d, want %v/%dx%d",
				fc.Name, back.Name, back.Width, back.Height, fc.Name, fc.Width, fc.Height)
		}
		if len(back.Nodes) != len(fc.Nodes) {
			t.Fatalf("%s: json nodes = %d, want %d", fc.Name, len(back.Nodes), len(fc.Nodes))
		}
		for i, w := range fc.Nodes {
			if back.Nodes[i].Name != w.Name || back.Nodes[i].CostMs != w.CostMs || back.Nodes[i].Pixels != w.Pixels {
				t.Errorf("%s: json node[%d] = %+v, want %+v", fc.Name, i, back.Nodes[i], w)
			}
		}
		fh, err := os.Open(ppath)
		if err != nil {
			t.Fatalf("%s: open png: %v", fc.Name, err)
		}
		img, err := png.Decode(fh)
		_ = fh.Close()
		if err != nil {
			t.Fatalf("%s: decode png: %v", fc.Name, err)
		}
		if img.Bounds().Dx() != fc.Width || img.Bounds().Dy() != fc.Height {
			t.Fatalf("%s: png size = %v, want %dx%d", fc.Name, img.Bounds(), fc.Width, fc.Height)
		}
		for y := 0; y < fc.Height; y++ {
			for x := 0; x < fc.Width; x++ {
				r, g, b, a := img.At(x, y).RGBA()
				off := (y*fc.Width + x) * 4
				want := fc.Pixels[off : off+4]
				if byte(r>>8) != want[0] || byte(g>>8) != want[1] || byte(b>>8) != want[2] || byte(a>>8) != want[3] {
					t.Fatalf("%s: png pixel (%d,%d) = %d/%d/%d/%d, want %d/%d/%d/%d",
						fc.Name, x, y, byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8),
						want[0], want[1], want[2], want[3])
				}
			}
		}
	}
	if _, _, err := Record("", buildFrame(t, c.Frames[0])); err == nil {
		t.Error("empty dir want error")
	}
}

// B:帧分解定位到节点名,耗时像素占比拼满 1,头名即最耗时节点.
func TestBreakdownLocatesNode(t *testing.T) {
	c := loadPreviewCases(t)
	for _, fc := range c.Frames {
		f := buildFrame(t, fc)
		rows := Breakdown(f, fc.BudgetMs)
		if len(rows) != len(fc.Nodes) {
			t.Fatalf("%s: rows = %d, want %d", fc.Name, len(rows), len(fc.Nodes))
		}
		if rows[0].Name != fc.WantTop {
			t.Errorf("%s: top = %q, want %q", fc.Name, rows[0].Name, fc.WantTop)
		}
		var costSum, pixelSum float64
		for _, n := range fc.Nodes {
			r, ok := findRow(rows, n.Name)
			if !ok {
				t.Fatalf("%s: breakdown has no node %q", fc.Name, n.Name)
			}
			if r.CostMs != n.CostMs {
				t.Errorf("%s/%s: cost = %v, want %v", fc.Name, n.Name, r.CostMs, n.CostMs)
			}
			costSum += r.CostShare
			pixelSum += r.PixelShare
		}
		closeFloat(t, fc.Name+" cost shares", costSum, 1)
		closeFloat(t, fc.Name+" pixel shares", pixelSum, 1)
		for i := 1; i < len(rows); i++ {
			if rows[i].CostMs > rows[i-1].CostMs {
				t.Errorf("%s: rows not cost-descending at %d", fc.Name, i)
			}
		}
	}
}

// C:超预算标红, OVER 行变红, 预算内行保持原色.
func TestBreakdownOverBudgetRed(t *testing.T) {
	c := loadPreviewCases(t)
	for _, fc := range c.Frames {
		f := buildFrame(t, fc)
		rows := Breakdown(f, fc.BudgetMs)
		over := map[string]bool{}
		for _, n := range fc.WantOver {
			over[n] = true
		}
		for _, r := range rows {
			if r.OverBudget != over[r.Name] {
				t.Errorf("%s/%s: over = %v, want %v", fc.Name, r.Name, r.OverBudget, over[r.Name])
			}
		}
		text := Text(rows)
		for _, n := range fc.WantOver {
			r, _ := findRow(rows, n)
			_ = r
			if !strings.Contains(text, redOpen+n) && !strings.Contains(text, n+" ") {
				t.Errorf("%s: text misses node %q", fc.Name, n)
			}
		}
		if !strings.Contains(text, "OVER") {
			t.Errorf("%s: text has no OVER marker", fc.Name)
		}
		if !strings.Contains(text, redOpen) || !strings.Contains(text, redClose) {
			t.Errorf("%s: text has no red wrap", fc.Name)
		}
		for _, r := range rows {
			if over[r.Name] {
				continue
			}
			for _, line := range strings.Split(text, "\n") {
				if strings.HasPrefix(line, r.Name+" ") && strings.Contains(line, "OVER") {
					t.Errorf("%s/%s: in-budget row marked OVER", fc.Name, r.Name)
				}
			}
		}
		// 预算放宽后全员不过线, 文本里不再见红.
		calm := Breakdown(f, 1<<30)
		for _, r := range calm {
			if r.OverBudget {
				t.Errorf("%s/%s: huge budget still over", fc.Name, r.Name)
			}
		}
		if strings.Contains(Text(calm), redOpen) {
			t.Errorf("%s: calm text still red", fc.Name)
		}
	}
}
