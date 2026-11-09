package recording

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
)

type meshPatchCommandCase struct {
	Style  string       `json:"style"`
	Points [][2]float64 `json:"points"`
	Colors [][4]float64 `json:"colors"`
}

func loadMeshPatchCommandCase(t *testing.T) render.MeshPatch {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "mesh_patch_command.json"))
	if err != nil {
		t.Fatalf("read testdata/mesh_patch_command.json: %v", err)
	}
	var c meshPatchCommandCase
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode mesh_patch_command.json: %v", err)
	}
	patch := render.MeshPatch{Style: render.MeshPatchCoons}
	for _, p := range c.Points {
		patch.Points = append(patch.Points, render.Pt(p[0], p[1]))
	}
	for i := 0; i < 4 && i < len(c.Colors); i++ {
		patch.Colors[i] = render.RGBA{R: c.Colors[i][0], G: c.Colors[i][1], B: c.Colors[i][2], A: c.Colors[i][3]}
	}
	return patch
}

// P3.9: DrawMeshPatch must survive the display-list round trip exactly like a
// live Context call. The recording stores the patch primitive (not triangles),
// so playback re-tessellates against the playback CTM.
func TestRecorderDrawMeshPatchRoundTrip(t *testing.T) {
	patch := loadMeshPatchCommandCase(t)
	rec := NewRecorder(128, 128)
	rec.DrawMeshPatch(patch)

	direct := render.NewContext(128, 128)
	defer direct.Close()
	direct.ClearWithColor(render.White)
	direct.DrawMeshPatch(patch)

	replayed := render.NewContext(128, 128)
	defer replayed.Close()
	replayed.ClearWithColor(render.White)
	if err := rec.FinishRecording().Playback(NewCanvasBackend(replayed)); err != nil {
		t.Fatalf("playback: %v", err)
	}

	a := direct.Image()
	b := replayed.Image()
	if a.Bounds() != b.Bounds() {
		t.Fatalf("bounds direct=%v replay=%v", a.Bounds(), b.Bounds())
	}
	changed := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
		}
	}
	if changed != 0 {
		t.Fatalf("DrawMeshPatch round trip changed %d pixels", changed)
	}
}

func TestRecorderDrawMeshPatchBounds(t *testing.T) {
	patch := loadMeshPatchCommandCase(t)
	rec := NewRecorder(128, 128)
	rec.DrawMeshPatch(patch)
	bounds := rec.FinishRecording().Bounds()
	if bounds.Empty() {
		t.Fatal("DrawMeshPatch bounds empty")
	}
	if bounds.Min.X > 0 || bounds.Min.Y > 0 || bounds.Max.X < 100 || bounds.Max.Y < 100 {
		t.Fatalf("DrawMeshPatch bounds = %v, want to contain [0,0]-[100,100]", bounds)
	}
}
