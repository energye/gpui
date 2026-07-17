// Package compare provides PNG/image regression comparison against frozen refs.
// It does not import the render engine; callers supply expected/actual paths or images.
package compare

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Policy is per-case (or default) acceptance thresholds for visual regression.
type Policy struct {
	// MaxMeanAbs is max mean absolute channel delta (0–255 scale, averaged over RGB×pixels).
	MaxMeanAbs float64 `json:"max_mean_abs"`
	// MaxRMSE is max root-mean-square channel delta.
	MaxRMSE float64 `json:"max_rmse"`
	// MaxDelta is max single-channel |a-b| allowed anywhere (0 = ignore this gate).
	MaxDelta uint32 `json:"max_delta"`
	// ChangedRatioMax is max fraction of pixels with any channel delta > PerChannelTol.
	ChangedRatioMax float64 `json:"changed_ratio_max"`
	// PerChannelTol: channel delta ≤ this does not count as "changed" (default 2).
	PerChannelTol uint8 `json:"per_channel_tol"`
	// IgnoreAlpha when true compares only RGB.
	IgnoreAlpha bool `json:"ignore_alpha"`
}

// CaseMeta is stored next to each ref PNG as <id>.json.
type CaseMeta struct {
	ID     string `json:"id"`
	Oracle string `json:"oracle"` // e.g. "gpui-capture", "canvaskit"
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Diff   Policy `json:"diff"`
	Note   string `json:"note,omitempty"`
	File   string `json:"file,omitempty"` // PNG basename under catalog ImagesDir
}

// Manifest describes a frozen ref set.
type Manifest struct {
	Name        string `json:"name"`
	Oracle      string `json:"oracle"`
	OracleNote  string `json:"oracle_note,omitempty"`
	CaseCount   int    `json:"case_count"`
	DefaultDiff Policy `json:"default_diff"`
	GeneratedAt string `json:"generated_at,omitempty"`
}

// Stats holds comparison metrics.
type Stats struct {
	TotalPixels   int
	ChangedPixels int
	MeanAbs       float64
	RMSE          float64
	MaxDelta      uint32
	ChangedRatio  float64
}

// Result is the outcome of one comparison.
type Result struct {
	Pass   bool
	Stats  Stats
	Reason string
}

// DefaultPolicy is conservative for GPU AA / text while still catching blanks and large shifts.
func DefaultPolicy() Policy {
	return Policy{
		MaxMeanAbs:      2.5,
		MaxRMSE:         12.0,
		MaxDelta:        96,
		ChangedRatioMax: 0.08,
		PerChannelTol:   2,
		IgnoreAlpha:     false,
	}
}

// LoadMeta reads CaseMeta JSON; missing file returns defaults with id from filename.
func LoadMeta(path string) (CaseMeta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return CaseMeta{}, err
	}
	var m CaseMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return CaseMeta{}, err
	}
	m.Diff = normalizePolicy(m.Diff)
	return m, nil
}

// WriteMeta writes CaseMeta as indented JSON.
func WriteMeta(path string, m CaseMeta) error {
	m.Diff = normalizePolicy(m.Diff)
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func normalizePolicy(p Policy) Policy {
	d := DefaultPolicy()
	if p.MaxMeanAbs <= 0 {
		p.MaxMeanAbs = d.MaxMeanAbs
	}
	if p.MaxRMSE <= 0 {
		p.MaxRMSE = d.MaxRMSE
	}
	if p.MaxDelta == 0 {
		p.MaxDelta = d.MaxDelta
	}
	if p.ChangedRatioMax <= 0 {
		p.ChangedRatioMax = d.ChangedRatioMax
	}
	if p.PerChannelTol == 0 {
		p.PerChannelTol = d.PerChannelTol
	}
	return p
}

// DecodePNG loads a PNG file.
func DecodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return png.Decode(f)
}

// Files compares expected vs actual PNG paths using policy (meta optional via CompareNamed).
func Files(expectedPath, actualPath string, pol Policy) (Result, error) {
	exp, err := DecodePNG(expectedPath)
	if err != nil {
		return Result{}, fmt.Errorf("expected: %w", err)
	}
	act, err := DecodePNG(actualPath)
	if err != nil {
		return Result{}, fmt.Errorf("actual: %w", err)
	}
	return Images(exp, act, pol)
}

// CompareNamed loads expected PNG + optional <id>.json policy from refDir.
func CompareNamed(refDir, id, actualPath string) (Result, CaseMeta, error) {
	expPath := filepath.Join(refDir, id+".png")
	metaPath := filepath.Join(refDir, id+".json")
	meta := CaseMeta{ID: id, Diff: DefaultPolicy(), Oracle: "unknown"}
	if b, err := os.ReadFile(metaPath); err == nil {
		if err := json.Unmarshal(b, &meta); err != nil {
			return Result{}, meta, fmt.Errorf("meta %s: %w", metaPath, err)
		}
		meta.Diff = normalizePolicy(meta.Diff)
	}
	res, err := Files(expPath, actualPath, meta.Diff)
	return res, meta, err
}

// Images compares two images.
func Images(expected, actual image.Image, pol Policy) (Result, error) {
	pol = normalizePolicy(pol)
	eb := expected.Bounds()
	ab := actual.Bounds()
	if eb.Dx() != ab.Dx() || eb.Dy() != ab.Dy() {
		return Result{
			Pass:   false,
			Reason: fmt.Sprintf("size differ expected=%dx%d actual=%dx%d", eb.Dx(), eb.Dy(), ab.Dx(), ab.Dy()),
		}, nil
	}

	tol := pol.PerChannelTol
	var sumAbs, sumSq float64
	var changed int
	var maxDelta uint32
	total := eb.Dx() * eb.Dy()
	channels := 3
	if !pol.IgnoreAlpha {
		channels = 4
	}

	for y := 0; y < eb.Dy(); y++ {
		for x := 0; x < eb.Dx(); x++ {
			er, eg, eb_, ea := rgba8at(expected, eb.Min.X+x, eb.Min.Y+y)
			ar, ag, ab_, aa := rgba8at(actual, ab.Min.X+x, ab.Min.Y+y)
			ds := []uint32{absU8(er, ar), absU8(eg, ag), absU8(eb_, ab_)}
			if !pol.IgnoreAlpha {
				ds = append(ds, absU8(ea, aa))
			}
			pixelChanged := false
			for _, d := range ds {
				if d > uint32(tol) {
					pixelChanged = true
				}
				if d > maxDelta {
					maxDelta = d
				}
				sumAbs += float64(d)
				sumSq += float64(d * d)
			}
			if pixelChanged {
				changed++
			}
		}
	}

	samples := float64(total * channels)
	st := Stats{
		TotalPixels:   total,
		ChangedPixels: changed,
		MeanAbs:       sumAbs / samples,
		RMSE:          math.Sqrt(sumSq / samples),
		MaxDelta:      maxDelta,
		ChangedRatio:  float64(changed) / float64(total),
	}

	res := Result{Pass: true, Stats: st}
	if st.MeanAbs > pol.MaxMeanAbs {
		res.Pass = false
		res.Reason = fmt.Sprintf("mean_abs=%.3f > max_mean_abs=%.3f", st.MeanAbs, pol.MaxMeanAbs)
		return res, nil
	}
	if st.RMSE > pol.MaxRMSE {
		res.Pass = false
		res.Reason = fmt.Sprintf("rmse=%.3f > max_rmse=%.3f", st.RMSE, pol.MaxRMSE)
		return res, nil
	}
	if pol.MaxDelta > 0 && st.MaxDelta > pol.MaxDelta {
		res.Pass = false
		res.Reason = fmt.Sprintf("max_delta=%d > max_delta=%d", st.MaxDelta, pol.MaxDelta)
		return res, nil
	}
	if st.ChangedRatio > pol.ChangedRatioMax {
		res.Pass = false
		res.Reason = fmt.Sprintf("changed_ratio=%.4f > changed_ratio_max=%.4f (changed=%d/%d)",
			st.ChangedRatio, pol.ChangedRatioMax, st.ChangedPixels, st.TotalPixels)
		return res, nil
	}
	return res, nil
}

// WriteDiffPNG writes a red-highlight abs-diff visualization (RGB exaggerated).
func WriteDiffPNG(expected, actual image.Image, outPath string) error {
	eb := expected.Bounds()
	ab := actual.Bounds()
	if eb.Dx() != ab.Dx() || eb.Dy() != ab.Dy() {
		return fmt.Errorf("size differ")
	}
	dst := image.NewRGBA(image.Rect(0, 0, eb.Dx(), eb.Dy()))
	for y := 0; y < eb.Dy(); y++ {
		for x := 0; x < eb.Dx(); x++ {
			er, eg, eb_, _ := rgba8at(expected, eb.Min.X+x, eb.Min.Y+y)
			ar, ag, ab_, _ := rgba8at(actual, ab.Min.X+x, ab.Min.Y+y)
			dr := absU8(er, ar)
			dg := absU8(eg, ag)
			db := absU8(eb_, ab_)
			// amplify for visibility
			v := dr + dg + db
			if v > 255 {
				v = 255
			}
			dst.SetRGBA(x, y, color.RGBA{R: uint8(v), G: uint8(v / 4), B: uint8(v / 4), A: 255})
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return png.Encode(f, dst)
}

// WritePNG encodes img to path.
func WritePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return png.Encode(f, img)
}

// CloneRGBA returns an RGBA copy of img.
func CloneRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

func rgba8at(img image.Image, x, y int) (r, g, b, a uint8) {
	R, G, B, A := img.At(x, y).RGBA()
	return uint8(R / 257), uint8(G / 257), uint8(B / 257), uint8(A / 257)
}

func absU8(a, b uint8) uint32 {
	if a > b {
		return uint32(a - b)
	}
	return uint32(b - a)
}
