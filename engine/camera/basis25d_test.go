//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package camera

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/engine/core"
)

const epsBasis25D = 1e-9

type basisAxisCase struct {
	Mode int        `json:"mode"`
	X    [2]float64 `json:"x"`
	Y    [2]float64 `json:"y"`
	Z    [2]float64 `json:"z"`
}

type basisProjectCase struct {
	Mode int        `json:"mode"`
	In   [3]float64 `json:"in"`
	Want [2]float64 `json:"want"`
}

type basisFile struct {
	Basis   []basisAxisCase    `json:"basis"`
	Project []basisProjectCase `json:"project"`
	Shadow  struct {
		Ground float64    `json:"ground"`
		In     [3]float64 `json:"in"`
		Want   [2]float64 `json:"want"`
	} `json:"shadow"`
	YSort struct {
		Keys  [][]float64 `json:"keys"`
		Order [][]float64 `json:"order"`
	} `json:"ysort"`
}

func loadBasis25DCases(t *testing.T) basisFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "basis25d_cases.json"))
	if err != nil {
		t.Fatalf("read basis25d_cases.json: %v", err)
	}
	var f basisFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode basis25d_cases.json: %v", err)
	}
	if len(f.Basis) != 6 || len(f.Project) != 6 {
		t.Fatalf("basis25d_cases.json wants 6 modes, got %d/%d", len(f.Basis), len(f.Project))
	}
	return f
}

func TestBasis25DFromCases(t *testing.T) {
	f := loadBasis25DCases(t)
	for _, c := range f.Basis {
		b, err := NewBasis25D(ViewMode(c.Mode))
		if err != nil {
			t.Fatalf("mode %d: %v", c.Mode, err)
		}
		if math.Abs(b.X.X-c.X[0]) > epsBasis25D || math.Abs(b.X.Y-c.X[1]) > epsBasis25D ||
			math.Abs(b.Y.X-c.Y[0]) > epsBasis25D || math.Abs(b.Y.Y-c.Y[1]) > epsBasis25D ||
			math.Abs(b.Z.X-c.Z[0]) > epsBasis25D || math.Abs(b.Z.Y-c.Z[1]) > epsBasis25D {
			t.Fatalf("mode %d axes = %+v, want %+v", c.Mode, b, c)
		}
	}
}

func TestBasis25DProjectFromCases(t *testing.T) {
	f := loadBasis25DCases(t)
	for _, c := range f.Project {
		b, err := NewBasis25D(ViewMode(c.Mode))
		if err != nil {
			t.Fatalf("mode %d: %v", c.Mode, err)
		}
		got, ok := b.Project(core.V3(c.In[0], c.In[1], c.In[2]))
		if !ok {
			t.Fatalf("mode %d: project rejected", c.Mode)
		}
		if math.Abs(got.X-c.Want[0]) > epsBasis25D || math.Abs(got.Y-c.Want[1]) > epsBasis25D {
			t.Fatalf("mode %d = (%.12f,%.12f), want (%.12f,%.12f)", c.Mode, got.X, got.Y, c.Want[0], c.Want[1])
		}
	}
}

func TestBasis25DEdgesNoCrash(t *testing.T) {
	b, err := NewBasis25D(View45)
	if err != nil {
		t.Fatalf("basis: %v", err)
	}
	if _, ok := b.Project(core.V3(math.NaN(), 0, 0)); ok {
		t.Fatal("NaN x must fail")
	}
	if _, ok := b.Project(core.V3(0, math.Inf(1), 0)); ok {
		t.Fatal("Inf y must fail")
	}
	if _, ok := b.ProjectPoints([]core.Vec3{core.V3(0, 0, 0), core.V3(math.NaN(), 0, 0)}); ok {
		t.Fatal("bad batch member must fail whole set")
	}
	if _, err := NewBasis25D(ViewMode(99)); err == nil {
		t.Fatal("bad mode must fail")
	}
}

func TestYSortFromCases(t *testing.T) {
	f := loadBasis25DCases(t)
	for _, k := range f.YSort.Keys {
		got := YSortKey(core.V3(k[0], k[1], k[2]))
		if math.Abs(got-k[3]) > epsBasis25D {
			t.Fatalf("key(%v) = %.12f, want %.12f", k[:3], got, k[3])
		}
	}
	items := make([]YSortItem, len(f.YSort.Order))
	for i, o := range f.YSort.Order {
		items[i] = YSortItem{Pos: core.V3(o[0], o[1], o[2]), Order: i}
	}
	sorted, err := YSort(items)
	if err != nil {
		t.Fatalf("sort: %v", err)
	}
	for i := 1; i < len(sorted); i++ {
		if YSortLess(sorted[i], sorted[i-1]) {
			t.Fatalf("not sorted at %d", i)
		}
	}
	if YSortZ(0) != -4000 || YSortZ(1) != -3998 {
		t.Fatalf("z slots = %d,%d, want -4000,-3998", YSortZ(0), YSortZ(1))
	}
	if _, err := YSort(make([]YSortItem, MaxYSort25D+1)); err == nil {
		t.Fatal("over-limit sort must fail")
	}
}

func TestShadow25DFromCases(t *testing.T) {
	f := loadBasis25DCases(t)
	b, err := NewBasis25D(View45)
	if err != nil {
		t.Fatalf("basis: %v", err)
	}
	got, ok := LandShadow(b, core.V3(f.Shadow.In[0], f.Shadow.In[1], f.Shadow.In[2]), f.Shadow.Ground)
	if !ok {
		t.Fatal("shadow must land")
	}
	if math.Abs(got.X-f.Shadow.Want[0]) > epsBasis25D || math.Abs(got.Y-f.Shadow.Want[1]) > epsBasis25D {
		t.Fatalf("shadow = (%.12f,%.12f), want (%.12f,%.12f)", got.X, got.Y, f.Shadow.Want[0], f.Shadow.Want[1])
	}
	if !ShadowVisible(core.V3(0, 1, 0), 0, true) {
		t.Fatal("above ground must show")
	}
	if _, ok := LandShadow(b, core.V3(0, -1, 0), 0); ok {
		t.Fatal("below ground must hide")
	}
	if ShadowVisible(core.V3(0, -1, 0), 0, false) {
		t.Fatal("unlanded must hide")
	}
}
