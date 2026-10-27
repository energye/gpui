package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

const godotGray = 77.0 / 255.0

type bullet struct {
	x, y, speed float64
}

type godotSim struct {
	winW, winH float64
	bullets    []bullet
	playerX    float64
	playerY    float64
	touching   bool
}

func newGodotSim(n int, winW, winH float64) *godotSim {
	s := &godotSim{winW: winW, winH: winH}
	r := rand.New(rand.NewSource(0x5EEDBEEF))
	for i := 0; i < n; i++ {
		s.bullets = append(s.bullets, bullet{
			// Same spawn rule as bullets.gd: x = rand(0,w)+w, y = rand(0,h).
			x:     r.Float64()*winW + winW,
			y:     r.Float64() * winH,
			speed: 20 + r.Float64()*60,
		})
	}
	return s
}

// step advances one Godot physics tick (1/60 s): move left, wrap at x<-16,
// then test the player's 27px circle against each bullet's 8px circle.
func (s *godotSim) step(dt float64) {
	if s == nil || dt <= 0 {
		return
	}
	for i := range s.bullets {
		b := &s.bullets[i]
		b.x -= b.speed * dt
		if b.x < -16 {
			b.x = s.winW + 16
		}
	}
	s.touching = false
	const rr = (27.0 + 8.0) * (27.0 + 8.0)
	for i := range s.bullets {
		dx, dy := s.bullets[i].x-s.playerX, s.bullets[i].y-s.playerY
		if dx*dx+dy*dy < rr {
			s.touching = true
			break
		}
	}
}

func (s *godotSim) draw(pc *rendering.PaintContext, _ rendering.Size, bulletImg, happy, sad *render.ImageBuf) {
	rendering.FillRect(pc, 0, 0, s.winW, s.winH, godotGray, godotGray, godotGray, 1)
	for i := range s.bullets {
		b := &s.bullets[i]
		rendering.DrawImageBuf(pc, bulletImg, b.x-16, b.y-16, 32, 32)
	}
	// AnimatedSprite2D is centered: at player (0,0) only the lower-right
	// quadrant of the 64x64 face is visible, exactly like the Godot window.
	face := happy
	if s.touching {
		face = sad
	}
	rendering.DrawImageBuf(pc, face, s.playerX-32, s.playerY-32, 64, 64)
}

func bulletCount() (int, error) {
	n := 500
	if v := os.Getenv("RULER_BULLETS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("RULER_BULLETS=%q is not a positive integer", v)
		}
		n = parsed
	}
	return n, nil
}

// buildGodotScene mirrors the official Godot 4.5.1 bullet_shower demo at
// 1200x800: #4D4D4D clear, 500 32x32 bullets spawned in [w,2w], 20..80 px/s,
// wrap at x<-16, player follows the pointer and swaps happy/sad on circle
// collision. Godot's PhysicsServer2D is replaced by the equivalent 27px/8px
// circle test here; there is no bullet-bullet physics in either arm.
func buildGodotScene(gate bool) (*sceneSpec, error) {
	n, err := bulletCount()
	if err != nil {
		return nil, err
	}
	bulletImg := ensureImage(testdataPath("bullet.png"), 32, 32)
	happy := ensureImage(testdataPath("face_happy.png"), 64, 64)
	sad := ensureImage(testdataPath("face_sad.png"), 64, 64)
	if bulletImg == nil || happy == nil || sad == nil {
		return nil, fmt.Errorf("godot bullet_shower assets missing under testdata/")
	}

	sim := newGodotSim(n, winListW, winListH)
	if gate {
		// Freeze at t=30s: after 60s of wrapping the cloud is fully mixed;
		// 30s is still deterministic and visually representative.
		for i := 0; i < 30*60; i++ {
			sim.step(1.0 / 60.0)
		}
	}

	root := newRoot(winListW, winListH, rgbColor{R: godotGray, G: godotGray, B: godotGray})
	box := rendering.NewRenderBox()
	box.FixedWidth = winListW
	box.FixedHeight = winListH
	box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		sim.draw(pc, size, bulletImg, happy, sad)
	}
	root.Place(box, 0, 0)

	acc := 0.0
	return &sceneSpec{
		name: sceneGodot,
		ref:  "godot-demo-projects 2d/bullet_shower @ a3b5c11 (Godot 4.5.1)",
		winW: winListW,
		winH: winListH,
		// Upstream project.godot config/name is "Bullet Shower"; keep the
		// title identical instead of advertising the local parity harness.
		title:  "Bullet Shower",
		root:   root,
		clearR: godotGray, clearG: godotGray, clearB: godotGray,
		fullPaint: true,
		cursor:    platform.CursorHidden,
		resizable: true,
		onResize: func(w, h int) {
			sim.winW, sim.winH = float64(w), float64(h)
			root.FixedWidth, root.FixedHeight = float64(w), float64(h)
			box.FixedWidth, box.FixedHeight = float64(w), float64(h)
			root.MarkNeedsLayout()
			box.MarkNeedsLayout()
			root.MarkNeedsPaint()
			box.MarkNeedsPaint()
		},
		tick: func(dt float64) bool {
			if dt > 0 && dt <= 0.25 {
				acc += dt
				// PhysicsServer2D runs at 60Hz. Cap catch-up so a long
				// compositor stall cannot run thousands of steps in one frame.
				for steps := 0; acc >= 1.0/60.0 && steps < 8; steps++ {
					sim.step(1.0 / 60.0)
					acc -= 1.0 / 60.0
				}
				if acc > 8.0/60.0 {
					acc = 0
				}
			}
			box.MarkNeedsPaint()
			return true
		},
		onEvent: func(ev platform.Event) {
			if ev.Type == platform.EventPointer && ev.Pointer == platform.PointerMove {
				sim.playerX, sim.playerY = ev.X, ev.Y-16
				box.MarkNeedsPaint()
			}
		},
		goldenRects: []wrkit.GoldenRect{{X: 0, Y: 0, W: winListW, H: winListH}},
		probes: []wrkit.PixelProbe{
			{Name: "godot_bg", X: 1100, Y: 700, R: godotGray, G: godotGray, B: godotGray, Tol: 4},
			{Name: "godot_player_green", X: 10, Y: 10, R: 118.0 / 255, G: 237.0 / 255, B: 140.0 / 255, Tol: 6},
		},
		extraCheck: func(snapshot string) error {
			img, err := loadSnapshot(snapshot)
			if err != nil {
				return fmt.Errorf("decode godot snapshot: %w", err)
			}
			// Count bullet/face ink; the gray field alone must not pass.
			n := inkCount(img, 0, 0, winListW, winListH, rgbColor{R: godotGray, G: godotGray, B: godotGray}, 6)
			if n < 1000 {
				return fmt.Errorf("godot bullet cloud ink too small: %d px (want >=1000)", n)
			}
			return nil
		},
		cleanup: func() {
			for _, b := range []*render.ImageBuf{bulletImg, happy, sad} {
				if b != nil {
					b.Dispose()
				}
			}
		},
	}, nil
}
