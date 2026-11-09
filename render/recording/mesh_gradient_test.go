package recording

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
)

type meshGradientCommandCase struct {
	Rows     int `json:"rows"`
	Cols     int `json:"cols"`
	Vertices []struct {
		Pos   [2]float64 `json:"pos"`
		Color [4]int     `json:"color"`
	} `json:"vertices"`
}

func loadMeshGradientCommandCase(t *testing.T) *render.MeshGradient {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "mesh_gradient_command.json"))
	if err != nil {
		t.Fatalf("read testdata/mesh_gradient_command.json: %v", err)
	}
	var c meshGradientCommandCase
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode mesh_gradient_command.json: %v", err)
	}
	vs := make([]render.MeshGradientVertex, len(c.Vertices))
	for i, v := range c.Vertices {
		vs[i] = render.MeshGradientVertex{
			Pos: render.Pt(v.Pos[0], v.Pos[1]),
			Color: render.RGBA{
				R: float64(v.Color[0]) / 255, G: float64(v.Color[1]) / 255,
				B: float64(v.Color[2]) / 255, A: float64(v.Color[3]) / 255,
			},
		}
	}
	g, err := render.NewMeshGradient(c.Rows, c.Cols, vs)
	if err != nil {
		t.Fatalf("NewMeshGradient: %v", err)
	}
	return g
}

// P3.9a: DrawMeshGradient records the grid (not pre-tessellated triangles), so
// playback re-evaluates LOD against the playback CTM and must equal a direct
// live draw.
func TestRecorderDrawMeshGradientRoundTrip(t *testing.T) {
	g := loadMeshGradientCommandCase(t)
	rec := NewRecorder(220, 220)
	rec.DrawMeshGradient(g)

	direct := render.NewContext(220, 220)
	defer direct.Close()
	direct.ClearWithColor(render.White)
	direct.DrawMeshGradient(g)

	replayed := render.NewContext(220, 220)
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
		t.Fatalf("DrawMeshGradient round trip changed %d pixels", changed)
	}
}

func TestRecorderDrawMeshGradientBounds(t *testing.T) {
	g := loadMeshGradientCommandCase(t)
	rec := NewRecorder(220, 220)
	rec.DrawMeshGradient(g)
	bounds := rec.FinishRecording().Bounds()
	if bounds.Empty() {
		t.Fatal("DrawMeshGradient bounds empty")
	}
	if bounds.Min.X > 0 || bounds.Min.Y > 0 || bounds.Max.X < 200 || bounds.Max.Y < 200 {
		t.Fatalf("DrawMeshGradient bounds = %v, want to contain [0,0]-[200,200]", bounds)
	}
}
