// Package scene defines a shared drawing script format used by both gpui and
// external oracles (e.g. CanvasKit). Scenes describe intent only — not engine API.
package scene

import (
	"encoding/json"
	"fmt"
	"os"
)

// Scene is one deterministic offscreen drawing script.
type Scene struct {
	ID    string  `json:"id"`
	Size  [2]int  `json:"size"`            // width, height in device pixels
	Scale float64 `json:"scale,omitempty"` // device scale; 0 or 1 = 1
	Font  *Font   `json:"font,omitempty"`
	Ops   []Op    `json:"ops"`
	// Note is free-form documentation.
	Note string `json:"note,omitempty"`
}

// Font selects a face for fill_text ops (path relative to repo or absolute).
type Font struct {
	Path string  `json:"path"`
	Size float64 `json:"size"`
}

// Op is one drawing command. Unknown fields are ignored by strict runners that
// only read known keys.
type Op struct {
	Op string `json:"op"`

	// Geometry / color
	RGBA   []float64 `json:"rgba,omitempty"`   // 3 or 4 floats 0..1
	Rect   []float64 `json:"rect,omitempty"`   // x,y,w,h
	Radius float64   `json:"radius,omitempty"` // rrect / circle
	X      float64   `json:"x,omitempty"`
	Y      float64   `json:"y,omitempty"`
	X2     float64   `json:"x2,omitempty"`
	Y2     float64   `json:"y2,omitempty"`
	CX     float64   `json:"cx,omitempty"`
	CY     float64   `json:"cy,omitempty"`
	W      float64   `json:"w,omitempty"`
	H      float64   `json:"h,omitempty"`

	// Text
	Text     string  `json:"text,omitempty"`
	FontSize float64 `json:"font_size,omitempty"`

	// Path (clip_path / fill_path): [[x,y], ...] closed if close=true
	Points [][]float64 `json:"points,omitempty"`
	Close  bool        `json:"close,omitempty"`

	// Layer / blend
	Opacity float64 `json:"opacity,omitempty"`
	Blend   string  `json:"blend,omitempty"` // normal|plus|multiply|screen

	// Transform
	Angle float64 `json:"angle,omitempty"` // radians for rotate

	// Procedural solid image (draw_image_solid)
	ImgW int `json:"img_w,omitempty"`
	ImgH int `json:"img_h,omitempty"`

	// Line
	Width float64 `json:"width,omitempty"` // stroke width; 0 = hairline where supported

	// Dash pattern for stroke_* ops
	Dash []float64 `json:"dash,omitempty"`

	// set_mask_alpha
	B64 string `json:"b64,omitempty"`
}

// Load reads a scene JSON file.
func Load(path string) (*Scene, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Scene
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if s.ID == "" {
		return nil, fmt.Errorf("%s: missing id", path)
	}
	if s.Size[0] <= 0 || s.Size[1] <= 0 {
		return nil, fmt.Errorf("%s: invalid size", path)
	}
	if s.Scale <= 0 {
		s.Scale = 1
	}
	return &s, nil
}

// Validate performs light structural checks.
func (s *Scene) Validate() error {
	if s == nil {
		return fmt.Errorf("nil scene")
	}
	for i, op := range s.Ops {
		if op.Op == "" {
			return fmt.Errorf("op[%d]: empty op", i)
		}
	}
	return nil
}
