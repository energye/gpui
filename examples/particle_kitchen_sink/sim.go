//go:build linux && !nogpu

package main

import "math"

type particle struct {
	x, y   float64
	vx, vy float64
	r      float64
	hue    float64
	life   float64
	seed   float64
}

type simWorld struct {
	ps     []particle
	trailX []float64
	trailY []float64
	trailN int
}

func newSim(n int, w, h float64) *simWorld {
	s := &simWorld{
		ps:     make([]particle, n),
		trailX: make([]float64, n*4),
		trailY: make([]float64, n*4),
		trailN: 4,
	}
	s.reset(w, h)
	return s
}

func (s *simWorld) reset(w, h float64) {
	if w < 8 {
		w = 8
	}
	if h < 8 {
		h = 8
	}
	for i := range s.ps {
		t := float64(i) * 0.6180339887
		s.ps[i] = particle{
			x:    math.Mod(t*w*1.7, w),
			y:    math.Mod(t*h*2.3, h),
			vx:   math.Sin(t*12.1)*60 + math.Cos(t*3.3)*20,
			vy:   math.Cos(t*9.7)*60 + math.Sin(t*2.1)*15,
			r:    2.5 + math.Mod(t*17, 5.5),
			hue:  math.Mod(t, 1),
			life: 0.4 + math.Mod(t*5, 0.6),
			seed: t,
		}
	}
	for i := range s.trailX {
		s.trailX[i] = 0
		s.trailY[i] = 0
	}
}

func (s *simWorld) resize(n int, w, h float64) {
	if n == len(s.ps) {
		return
	}
	s.ps = make([]particle, n)
	s.trailX = make([]float64, n*s.trailN)
	s.trailY = make([]float64, n*s.trailN)
	s.reset(w, h)
}

func (s *simWorld) step(dt, w, h float64) {
	if dt <= 0 {
		dt = 1.0 / 60.0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	for i := range s.ps {
		p := &s.ps[i]
		// soft swirl force
		p.vx += math.Sin(p.y*0.02+p.seed) * 8 * dt
		p.vy += math.Cos(p.x*0.02-p.seed) * 8 * dt
		// mild damping
		p.vx *= 0.999
		p.vy *= 0.999
		p.x += p.vx * dt
		p.y += p.vy * dt
		// bounce in stage
		if p.x < p.r {
			p.x = p.r
			p.vx = math.Abs(p.vx)
		} else if p.x > w-p.r {
			p.x = w - p.r
			p.vx = -math.Abs(p.vx)
		}
		if p.y < p.r {
			p.y = p.r
			p.vy = math.Abs(p.vy)
		} else if p.y > h-p.r {
			p.y = h - p.r
			p.vy = -math.Abs(p.vy)
		}
		// trails ring buffer (head at slot 0 each step — shift)
		base := i * s.trailN
		for k := s.trailN - 1; k > 0; k-- {
			s.trailX[base+k] = s.trailX[base+k-1]
			s.trailY[base+k] = s.trailY[base+k-1]
		}
		s.trailX[base] = p.x
		s.trailY[base] = p.y
	}
}

func hsvRGB(h, s, v float64) (r, g, b float64) {
	h = math.Mod(h, 1)
	if h < 0 {
		h += 1
	}
	i := math.Floor(h * 6)
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - f*s)
	u := v * (1 - (1-f)*s)
	switch int(i) % 6 {
	case 0:
		return v, u, p
	case 1:
		return q, v, p
	case 2:
		return p, v, u
	case 3:
		return p, q, v
	case 4:
		return u, p, v
	default:
		return v, p, q
	}
}
