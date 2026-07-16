package render

import (
	"math"
)

// installGPUClipMask promotes a mask/difference clip (HasMaskClip without
// gpuClipPath) onto the MaskAware R8 plane so Fill/Stroke can stay on GPU
// instead of forceCPUClip (P1-2 / C.02 / C.03).
//
// Returns (cleanup, true) when the GPU mask is installed and forceCPUClip
// may be cleared. Caller must defer cleanup.
func (c *Context) installGPUClipMask() (func(), bool) {
	if c == nil || c.pixmap == nil || c.clipStack == nil {
		return func() {}, false
	}
	if !c.clipStack.HasMaskClip() || c.gpuClipPath != nil {
		return func() {}, false
	}
	if !c.gpuPathAvailable() {
		return func() {}, false
	}
	a := Accelerator()
	if a == nil {
		return func() {}, false
	}
	ma, ok := a.(MaskAware)
	if !ok {
		return func() {}, false
	}

	mask := c.ensureClipMaskGPU()
	if mask == nil {
		return func() {}, false
	}

	// Upload combined plane (clip × user mask).
	ma.SetMaskTexture(mask.Data(), mask.Width(), mask.Height())
	// GPU masked fill requires MaskCoverage non-nil.
	c.paint.MaskCoverage = func(x, y int) uint8 {
		return mask.At(x, y)
	}

	return func() {
		// Restore user SetMask plane (or clear).
		c.syncGPUMaskTexture()
	}, true
}

// ensureClipMaskGPU returns a full-surface R8 mask = clip coverage × user mask.
// Cached while clip stack depth/bounds and user mask pointer are unchanged.
func (c *Context) ensureClipMaskGPU() *Mask {
	if c.clipStack == nil || c.pixmap == nil {
		return nil
	}
	w, h := c.pixmap.Width(), c.pixmap.Height()
	if w <= 0 || h <= 0 {
		return nil
	}
	b := c.clipStack.Bounds()
	if c.clipMaskGPU != nil &&
		c.clipMaskGPU.Width() == w && c.clipMaskGPU.Height() == h &&
		c.clipMaskGPUAt == c.clipMaskGPUGen &&
		c.clipMaskGPUUser == c.mask {
		return c.clipMaskGPU
	}

	m := NewMask(w, h)
	x0 := int(math.Floor(b.X))
	y0 := int(math.Floor(b.Y))
	x1 := int(math.Ceil(b.X + b.W))
	y1 := int(math.Ceil(b.Y + b.H))
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > w {
		x1 = w
	}
	if y1 > h {
		y1 = h
	}
	cs := c.clipStack
	user := c.mask
	for y := y0; y < y1; y++ {
		py := float64(y) + 0.5
		row := y * w
		for x := x0; x < x1; x++ {
			px := float64(x) + 0.5
			cov := cs.Coverage(px, py)
			if cov == 0 {
				continue
			}
			if user != nil {
				um := user.At(x, y)
				if um == 0 {
					continue
				}
				cov = uint8((uint16(cov) * uint16(um)) / 255)
			}
			m.data[row+x] = cov
		}
	}

	c.clipMaskGPU = m
	c.clipMaskGPUAt = c.clipMaskGPUGen
	c.clipMaskGPUUser = user
	return m
}

// bumpClipMaskGPUGen invalidates the cached clip→R8 plane.
func (c *Context) bumpClipMaskGPUGen() {
	c.clipMaskGPUGen++
	c.clipMaskGPU = nil
	c.clipMaskGPUUser = nil
}
