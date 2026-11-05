//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import "github.com/energye/gpui/render"

// imageBatchCensus accumulates the image layer's batching facts across frames
// until reset (P3.7 batch 6). Per-frame BatchDrawStats reports only totals, so
// it cannot show whether a run merged: a window needs "N consecutive quads of
// the same arm collapsed into 1 draw", which is a max-run fact.
type imageBatchCensus struct {
	draws, quads, maxRun          int
	perspDraws, maxPerspRun       int
	materialDraws, maxMaterialRun int
}

// note records one frame's image draws. calls is the frame's finished draw
// list (one entry per submitted draw) and quads the number of queued image
// commands those draws covered.
func (c *imageBatchCensus) note(calls []imageDrawCall, quads int) {
	c.draws += len(calls)
	c.quads += quads
	for i := range calls {
		cmds := int(calls[i].cmdCount)
		if cmds == 0 {
			cmds = 1 // 老的合并前形态：一 draw 一命令，cmdCount 未填时按一张记
		}
		if cmds > c.maxRun {
			c.maxRun = cmds
		}
		switch {
		case calls[i].perspective:
			c.perspDraws++
			if cmds > c.maxPerspRun {
				c.maxPerspRun = cmds
			}
		case calls[i].material:
			c.materialDraws++
			if cmds > c.maxMaterialRun {
				c.maxMaterialRun = cmds
			}
		}
	}
}

func (c *imageBatchCensus) snapshot() render.ImageBatchStats {
	return render.ImageBatchStats{
		Draws: c.draws, Quads: c.quads, MaxRun: c.maxRun,
		PerspDraws: c.perspDraws, MaxPerspRun: c.maxPerspRun,
		MaterialDraws: c.materialDraws, MaxMaterialRun: c.maxMaterialRun,
	}
}

func (c *imageBatchCensus) reset() render.ImageBatchStats {
	prev := c.snapshot()
	*c = imageBatchCensus{}
	return prev
}

// noteImageBatchCensus is called once per frame with the finished image draw
// list, next to the per-frame batch totals it mirrors.
func (s *GPURenderSession) noteImageBatchCensus(calls []imageDrawCall, quads int) {
	s.imageBatchCensus.note(calls, quads)
}

// ImageBatchStats / ResetImageBatchStats back render.Context.ImageBatchStats
// (optional-interface probe; the census is same-thread only, like the other
// session-side counters read straight through).
func (s *GPURenderSession) ImageBatchStats() render.ImageBatchStats {
	return s.imageBatchCensus.snapshot()
}

func (s *GPURenderSession) ResetImageBatchStats() render.ImageBatchStats {
	return s.imageBatchCensus.reset()
}
