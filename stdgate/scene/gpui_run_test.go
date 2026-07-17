package scene_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/stdgate/compare"
	"github.com/energye/gpui/stdgate/scene"
)

func findRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "testdata", "scenes")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("module root not found")
	return ""
}

func TestRunGPUI_S01_SolidRects(t *testing.T) {
	root := findRoot(t)
	s, err := scene.Load(filepath.Join(root, "testdata", "scenes", "S01_SolidRects.json"))
	if err != nil {
		t.Fatal(err)
	}
	dc, err := scene.RunGPUI(s, root)
	if err != nil {
		t.Fatal(err)
	}
	defer dc.Close()
	out := filepath.Join(root, "tmp", "scenes_gpui")
	_ = os.MkdirAll(out, 0o755)
	path := filepath.Join(out, s.ID+".png")
	if err := dc.SavePNG(path); err != nil {
		t.Fatal(err)
	}
	// structural: not pure white
	img, err := compare.DecodePNG(path)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	r, g, bch, _ := img.At(b.Min.X+20, b.Min.Y+20).RGBA()
	if r>>8 > 250 && g>>8 > 250 && bch>>8 > 250 {
		t.Fatalf("expected non-white content at sample, got %d,%d,%d", r>>8, g>>8, bch>>8)
	}
}

func TestRunGPUI_AllScenes(t *testing.T) {
	root := findRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "testdata", "scenes", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no scenes")
	}
	out := filepath.Join(root, "tmp", "scenes_gpui")
	_ = os.MkdirAll(out, 0o755)
	for _, m := range matches {
		m := m
		t.Run(filepath.Base(m), func(t *testing.T) {
			s, err := scene.Load(m)
			if err != nil {
				t.Fatal(err)
			}
			dc, err := scene.RunGPUI(s, root)
			if err != nil {
				t.Fatal(err)
			}
			defer dc.Close()
			path := filepath.Join(out, s.ID+".png")
			if err := dc.SavePNG(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScenes_MatchStandard(t *testing.T) {
	root := findRoot(t)
	cat, err := compare.LoadCatalog(filepath.Join(root, "testdata", "refs", "standard.json"))
	if err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(root, "testdata", "scenes", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "tmp", "scenes_gpui")
	_ = os.MkdirAll(out, 0o755)
	for _, m := range matches {
		m := m
		t.Run(filepath.Base(m), func(t *testing.T) {
			s, err := scene.Load(m)
			if err != nil {
				t.Fatal(err)
			}
			dc, err := scene.RunGPUI(s, root)
			if err != nil {
				t.Fatal(err)
			}
			defer dc.Close()
			path := filepath.Join(out, s.ID+".png")
			if err := dc.SavePNG(path); err != nil {
				t.Fatal(err)
			}
			res, meta, err := cat.CompareNamed(s.ID, path)
			if err != nil {
				t.Fatal(err)
			}
			if !res.Pass {
				t.Fatalf("standard mismatch %s oracle=%s: %s mean=%.3f", s.ID, meta.Oracle, res.Reason, res.Stats.MeanAbs)
			}
			t.Logf("OK %s oracle=%s mean=%.3f", s.ID, meta.Oracle, res.Stats.MeanAbs)
		})
	}
}
