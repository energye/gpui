package text

import (
	"github.com/energye/gpui/render/text/hint"
)

// cff_light_bridge.go —— CFF/CFF2 轮廓字体的 light 拟合接线。
//
// 生产路径（glyph_outline.go ExtractOutlineHinted）对 CFF 字体走 auto→gridFit
// 兜底（glyf 启发式网格偏陆界粗）。本文件把它替换为自研 cf2 引擎：
// hint.LightHint（pshinter light 移植，M2/M3 已逐点对齐 FT-light 26.6）。
//
// 重建规则（FT-light outline 的二次曲线分解）：
//   - CFF charstring 曲线是二次贝塞尔，FT 输出点为 (off,off,on) 组或
//     (off,on) 组；双 off 时隐式 on 中点 = 两 off 控制点的算术中点，
//     拆两段 QuadTo（FT cffgload 的 quadratic segment 拼装语义）。
//   - LightPt 坐标为 26.6 定点 Y-up（baseline=0 向上为正）；GlyphOutline
//     为 Y-down 像素：X = v/64，Y = -v/64。
func (e *OutlineExtractor) cffLightHintOutline(f *ownParsedFont, gid GlyphID, size float64) (*GlyphOutline, bool) {
	return e.cffLightHintOutlineVar(f, gid, size, nil)
}

// cffLightHintOutlineVar 同 cffLightHintOutline，支持 CFF2 变体坐标。
func (e *OutlineExtractor) cffLightHintOutlineVar(f *ownParsedFont, gid GlyphID, size float64, variations []FontVariation) (*GlyphOutline, bool) {
	if f == nil {
		return nil, false
	}
	raw := f.RawFontData()
	if len(raw) == 0 {
		return nil, false
	}
	isCFF2 := f.hasCFF2Table() && !f.hasCFFTable()

	var coords []float32
	if isCFF2 && len(variations) > 0 {
		for _, c := range f.cff2VariationCoords(variations) {
			coords = append(coords, float32(c)/16384.0)
		}
	}
	pts, contours, _, err := hint.LightHintVar(raw, f.collectionIndex, isCFF2, uint16(gid), size, coords)
	if err != nil {
		return nil, false
	}
	if len(pts) == 0 || len(contours) == 0 {
		return nil, false
	}

	segments := rebuildSegmentsFromLightPts(pts, contours)
	if len(segments) == 0 {
		return nil, false
	}

	outline := &GlyphOutline{
		Segments: segments,
		GID:      gid,
		Type:     GlyphTypeOutline,
		Advance:  float32(f.GlyphAdvance(uint16(gid), size)),
	}
	refreshOutlineBounds(outline)
	return outline, true
}

// rebuildSegmentsFromLightPts 把 LightPt（26.6 定点 Y-up）重建为
// OutlineSegments（Y-down 像素）。
//
// 分解规则（FT outline 语义）：
//   - 每个轮廓以 on 点开始（MoveTo）。
//   - off 点总是在两个 on 之间：单个 off → QuadTo；两个 off → 中间
//     隐含 on 点（两控制点中点）拆两段 QuadTo。
func rebuildSegmentsFromLightPts(pts []hint.LightPt, contours []int) []OutlineSegment {
	if len(pts) == 0 || len(contours) == 0 {
		return nil
	}
	xPx := func(v int64) float32 { return float32(float64(v) / 64.0) }
	yPx := func(v int64) float32 { return -float32(float64(v) / 64.0) }

	var segs []OutlineSegment
	first := 0
	for _, n := range contours {
		if n <= 0 || first+n > len(pts) {
			return nil
		}
		end := first + n
		start := pts[first]
		segs = append(segs, OutlineSegment{
			Op:     OutlineOpMoveTo,
			Points: [3]OutlinePoint{{X: xPx(start.X), Y: yPx(start.Y)}},
		})

		i := first + 1
		for i < end {
			p := pts[i]
			if p.On {
				segs = append(segs, OutlineSegment{
					Op:     OutlineOpLineTo,
					Points: [3]OutlinePoint{{X: xPx(p.X), Y: yPx(p.Y)}},
				})
				i++
				continue
			}
			if i+2 < end && !pts[i+1].On && pts[i+2].On {
				midX := (p.X + pts[i+1].X) / 2
				midY := (p.Y + pts[i+1].Y) / 2
				segs = append(segs, OutlineSegment{
					Op:     OutlineOpQuadTo,
					Points: [3]OutlinePoint{{X: xPx(p.X), Y: yPx(p.Y)}, {X: xPx(midX), Y: yPx(midY)}},
				})
				segs = append(segs, OutlineSegment{
					Op:     OutlineOpQuadTo,
					Points: [3]OutlinePoint{{X: xPx(pts[i+1].X), Y: yPx(pts[i+1].Y)}, {X: xPx(pts[i+2].X), Y: yPx(pts[i+2].Y)}},
				})
				i += 3
				continue
			}
			if i+1 < end && pts[i+1].On {
				segs = append(segs, OutlineSegment{
					Op:     OutlineOpQuadTo,
					Points: [3]OutlinePoint{{X: xPx(p.X), Y: yPx(p.Y)}, {X: xPx(pts[i+1].X), Y: yPx(pts[i+1].Y)}},
				})
				i += 2
				continue
			}
			// 轮廓尾部闭合语义（FT CFF charstring 允许轮廓以 off 结束，
			// 闭合段以轮廓起点 on 为隐式终点）：
			//   - 单 off 结尾 → QuadTo(off, start)
			//   - 双 off 结尾 → 中点拆两段，第二段终点 = start
			if i+1 >= end || !pts[i+1].On {
				startPt := start
				if i+1 < end {
					midX := (p.X + pts[i+1].X) / 2
					midY := (p.Y + pts[i+1].Y) / 2
					segs = append(segs, OutlineSegment{
						Op:     OutlineOpQuadTo,
						Points: [3]OutlinePoint{{X: xPx(p.X), Y: yPx(p.Y)}, {X: xPx(midX), Y: yPx(midY)}},
					})
					segs = append(segs, OutlineSegment{
						Op:     OutlineOpQuadTo,
						Points: [3]OutlinePoint{{X: xPx(pts[i+1].X), Y: yPx(pts[i+1].Y)}, {X: xPx(startPt.X), Y: yPx(startPt.Y)}},
					})
				} else {
					segs = append(segs, OutlineSegment{
						Op:     OutlineOpQuadTo,
						Points: [3]OutlinePoint{{X: xPx(p.X), Y: yPx(p.Y)}, {X: xPx(startPt.X), Y: yPx(startPt.Y)}},
					})
				}
				i = end
				continue
			}
			return nil
		}
		first = end
	}
	return segs
}