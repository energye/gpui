//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"image"
	"math"

	intImage "github.com/energye/gpui/render/internal/image"
)

// DrawAtlas draws multiple sub-rects from a single image.
// GPU path issues one QueueImageDraw per sprite from the shared ImageBuf.
// CPU path falls back to DrawImageEx per sprite.
//
// 老口只读老字段（vertices.go 惯例）：Perspective/Corners/Weights 不被本口读取，
// 带 Perspective 的精灵经本口会按 Dst 仿射画——透视精灵必须走 DrawAtlasEx
// （该口按 ErrAtlasQuad* 哨兵校验四角/权重并走透视腿）。
func (c *Context) DrawAtlas(img *ImageBuf, sprites []AtlasSprite) {
	if c == nil || img == nil || len(sprites) == 0 {
		return
	}

	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return
	}

	if rc := c.gpuCtxOps(); rc != nil {
		pixelData := img.PremultipliedData()
		if len(pixelData) == 0 {
			if c.gpuPathAvailable() {
				c.recordCPUFallbackReason("verts:DrawAtlas")
			}
			c.drawAtlasCPU(img, sprites)
			return
		}
		defer c.setGPUClipRect()()
		target := c.gpuRenderTarget()
		vpW := uint32(target.Width)  //nolint:gosec // viewport fits uint32
		vpH := uint32(target.Height) //nolint:gosec // viewport fits uint32
		ctm := c.totalMatrix()
		genID := img.GenerationID()
		stride := img.Stride()
		queued := 0
		for _, sp := range sprites {
			if sp.SrcW <= 0 || sp.SrcH <= 0 || sp.DstW == 0 || sp.DstH == 0 {
				continue
			}
			op := sp.Opacity
			if op <= 0 {
				op = 1
			}
			if op > 1 {
				op = 1
			}
			usr := [4]Point{
				Pt(sp.DstX, sp.DstY),
				Pt(sp.DstX+sp.DstW, sp.DstY),
				Pt(sp.DstX+sp.DstW, sp.DstY+sp.DstH),
				Pt(sp.DstX, sp.DstY+sp.DstH),
			}
			u0 := float32(sp.SrcX) / float32(imgW)
			v0 := float32(sp.SrcY) / float32(imgH)
			u1 := float32(sp.SrcX+sp.SrcW) / float32(imgW)
			v1 := float32(sp.SrcY+sp.SrcH) / float32(imgH)
			// P3.7 批 1d：投影层非恒等时仿射精灵升格为透视 quad（老口只读
			// 老字段的约定不涉及上下文投影层——投影层是画布状态，随链生效）。
			// 喂 projMapQuad 的必须是**用户空间**角点：它内部走 raw∘投影，
			// 传已经过 CTM 的设备角会把 CTM 再乘一遍。
			// P3.7 批 3b：材质精灵路由（老口 D-4 契约扩展一行：Material 在两口都
			// 生效——nil 恒等保证老调用方逐位不变；忽略已设 Material=静默裸画，
			// 违反非静默纪律）。材质 + 投影层/自带透视 ⇒ 跳该精灵+计数（画家序
			// behind-eye 同款，不炸整批）；材质+仿射 ⇒ QueueImageDrawMaterial。
			if sp.Material != nil {
				if c.projActive() || sp.Perspective {
					c.pathStats.MaterialSkips++
					continue
				}
				dev := [4]Point{
					ctm.TransformPoint(usr[0]),
					ctm.TransformPoint(usr[1]),
					ctm.TransformPoint(usr[2]),
					ctm.TransformPoint(usr[3]),
				}
				tr, tg, tb, ta := atlasTintFactors(sp.Tint)
				rc.QueueImageDrawMaterial(target, pixelData, genID, img, sp.Mipmap, imgW, imgH, stride,
					float32(dev[0].X), float32(dev[0].Y),
					float32(dev[1].X), float32(dev[1].Y),
					float32(dev[2].X), float32(dev[2].Y),
					float32(dev[3].X), float32(dev[3].Y),
					float32(op), vpW, vpH, u0, v0, u1, v1,
					float32(tr*ta), float32(tg*ta), float32(tb*ta), float32(ta),
					sp.Material, false)
				queued++
				continue
			}
			if c.projActive() {
				// 老口不读精灵自带透视字段（批 1c D-4），但吃画布投影层——投影层是
				// 画布状态，三口共用这一条投影腿（批 1d 去重，此前各写一遍）。
				pdev, pw, ok := c.atlasProjectSpriteCorners(sp, [4]float64{1, 1, 1, 1})
				if !ok {
					continue // behind-eye sprite: refused whole
				}
				rc.QueueImageDrawQuadW(target, pixelData, genID, img, sp.Mipmap, imgW, imgH, stride,
					float32(pdev[0].X), float32(pdev[0].Y),
					float32(pdev[1].X), float32(pdev[1].Y),
					float32(pdev[2].X), float32(pdev[2].Y),
					float32(pdev[3].X), float32(pdev[3].Y),
					float32(pw[0]), float32(pw[1]),
					float32(pw[2]), float32(pw[3]),
					float32(op), vpW, vpH, u0, v0, u1, v1,
					1, 1, 1, 1,
					false)
			} else {
				dev := [4]Point{
					ctm.TransformPoint(usr[0]),
					ctm.TransformPoint(usr[1]),
					ctm.TransformPoint(usr[2]),
					ctm.TransformPoint(usr[3]),
				}
				rc.QueueImageDraw(target, pixelData, genID, img, sp.Mipmap, imgW, imgH, stride,
					float32(dev[0].X), float32(dev[0].Y),
					float32(dev[1].X), float32(dev[1].Y),
					float32(dev[2].X), float32(dev[2].Y),
					float32(dev[3].X), float32(dev[3].Y),
					float32(op), vpW, vpH, u0, v0, u1, v1, false, false)
			}
			queued++
		}
		if queued > 0 {
			c.recordGPUOp()
			return
		}
	}

	if c.gpuPathAvailable() {
		c.recordCPUFallbackReason("verts:DrawAtlas")
	}
	c.drawAtlasCPU(img, sprites)
}

func (c *Context) drawAtlasCPU(img *ImageBuf, sprites []AtlasSprite) {
	for _, sp := range sprites {
		if sp.Material != nil {
			// P3.7 批 3b：材质无 CPU 求值器——跳过 + 计数，绝不裸画。
			c.pathStats.MaterialSkips++
			continue
		}
		if sp.SrcW <= 0 || sp.SrcH <= 0 {
			continue
		}
		op := sp.Opacity
		if op <= 0 {
			op = 1
		}
		src := image.Rect(
			int(sp.SrcX), int(sp.SrcY),
			int(sp.SrcX+sp.SrcW), int(sp.SrcY+sp.SrcH),
		)
		c.DrawImageEx(img, DrawImageOptions{
			X:             sp.DstX,
			Y:             sp.DstY,
			DstWidth:      sp.DstW,
			DstHeight:     sp.DstH,
			SrcRect:       &src,
			Interpolation: InterpBilinear,
			Opacity:       op,
			BlendMode:     BlendNormal,
		})
	}
}

// atlasTintIsIdentity 报告 Tint 是否为零结构体（白不透明，不染色）。
// 只有全零才算无染色，{1,1,1,0} 这类显式零 A 仍按透明处理。
// atlasTintIsIdentity 报告 Tint 是否为零结构体（白不透明，不染色）。
// 只有全零才算无染色，{1,1,1,0} 这类显式零 A 仍按透明处理。
func atlasTintIsIdentity(t RGBA) bool {
	return t == (RGBA{})
}

// atlasTintFactors 将 Tint 折成 0..1 系数，零结构体返回全 1。
// atlasTintFactors 将 Tint 折成 0..1 系数，零结构体返回全 1。
func atlasTintFactors(t RGBA) (r, g, b, a float64) {
	if atlasTintIsIdentity(t) {
		return 1, 1, 1, 1
	}
	return t.R, t.G, t.B, t.A
}

// normalizeAtlasFilter 将零值映射为 Bilinear，其余仅收三种模式。
// normalizeAtlasFilter 将零值映射为 Bilinear，其余仅收三种模式。
func normalizeAtlasFilter(m InterpolationMode) (InterpolationMode, error) {
	if m == 0 {
		return InterpBilinear, nil
	}
	if m == InterpNearest || m == InterpBilinear || m == InterpBicubic {
		return m, nil
	}
	return InterpBilinear, ErrAtlasUnsupportedFilter
}

// normalizeAtlasOpacity 沿用老路惯例：<=0 按 1（默认不透明），>1 钳 1。
// normalizeAtlasOpacity 沿用老路惯例：<=0 按 1（默认不透明），>1 钳 1。
func normalizeAtlasOpacity(o float64) float64 {
	if o <= 0 {
		return 1
	}
	if o > 1 {
		return 1
	}
	return o
}

// isFiniteAtlasSprite 检查精灵全部浮点字段（含 Tint）有限。
// isFiniteAtlasSprite 检查精灵全部浮点字段（含 Tint）有限。
func isFiniteAtlasSprite(sp AtlasSprite) bool {
	for _, v := range []float64{sp.SrcX, sp.SrcY, sp.SrcW, sp.SrcH, sp.DstX, sp.DstY, sp.DstW, sp.DstH, sp.Opacity, sp.Rot, sp.PivotX, sp.PivotY} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return isFiniteRGBA(sp.Tint)
}

// atlasDrawable 报告精灵是否可画（与老 DrawAtlas 跳过规则一致）。
// 透视精灵不读 Dst（四角承载几何），可画性只看源块。
func atlasDrawable(sp AtlasSprite) bool {
	if sp.Perspective {
		return sp.SrcW > 0 && sp.SrcH > 0
	}
	return sp.SrcW > 0 && sp.SrcH > 0 && sp.DstW != 0 && sp.DstH != 0
}

// atlasPerspValidate 校验透视图集精灵的全部组合约束（P3.7 批 1c）。
// 权重非法 → ErrAtlasQuadBadWeights（先于四角，与 DrawImageQuad 校验序一致）；
// 非有限四角 → ErrAtlasNonFinite；自交 → ErrAtlasQuadBowTie；退化 → ErrAtlasQuadDegenerate；
// 透视+bicubic 组合 → ErrQuadPerspectiveBicubic（与 quad 口同一语义同一哨兵）。
// 仅 Perspective=true 时调用；false 时新字段被忽略、不参与校验（零值=老路）。
func atlasPerspValidate(sp AtlasSprite, filt InterpolationMode) error {
	for _, w := range sp.Weights {
		if math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 {
			return ErrAtlasQuadBadWeights
		}
	}
	if filt == InterpBicubic {
		return ErrQuadPerspectiveBicubic
	}
	if quadHasNonFinite(sp.Corners) {
		return ErrAtlasNonFinite
	}
	if quadBowTie(sp.Corners) {
		return ErrAtlasQuadBowTie
	}
	if quadAreaSum(sp.Corners) < 1e-9 {
		return ErrAtlasQuadDegenerate
	}
	return nil
}

// atlasSpriteUserCorners returns the sprite's four corners in **user space**
// (perspective sprites carry them directly; affine ones are decomposed with an
// identity matrix). The projection chain applies the user matrix itself, so
// feeding device corners in would multiply the CTM twice.
// atlasSpriteUserCorners returns the sprite's four user-space corners.
func atlasSpriteUserCorners(sp AtlasSprite) [4]Point {
	if sp.Perspective {
		return sp.Corners
	}
	tl, tr, br, bl := atlasSpriteCorners(sp, Identity())
	return [4]Point{tl, tr, br, bl}
}

// atlasProjectSpriteCorners is the shared batch-1d projection leg for all three
// atlas entries (each used to inline the same mapping): user corners -> the
// canvas projection chain -> device corners plus homogeneous weights
// (w = w_proj x w_own). ok=false means a corner is behind the eye and the
// caller skips that whole sprite (painter's order; same rule as the quad
// entry's ErrQuadBehindEye).
func (c *Context) atlasProjectSpriteCorners(sp AtlasSprite, own [4]float64) (dev [4]Point, w [4]float64, ok bool) {
	pdev, pw, okP := c.projMapQuad(atlasSpriteUserCorners(sp))
	if !okP {
		return [4]Point{}, [4]float64{}, false
	}
	w = own
	for k := 0; k < 4; k++ {
		w[k] *= pw[k]
	}
	return pdev, w, true
}

// atlasSpriteCorners 算出设备四角（TL/TR/BR/BL），先绕轴心翻转再旋转。
// 老路（Rot==0 且无翻转）走原直接式，保证与老 DrawAtlas 逐位一致。
// atlasSpriteCorners 算出设备四角（TL/TR/BR/BL），先绕轴心翻转再旋转。
// 老路（Rot==0 且无翻转）走原直接式，保证与老 DrawAtlas 逐位一致。
func atlasSpriteCorners(sp AtlasSprite, ctm Matrix) (tl, tr, br, bl Point) {
	if sp.Rot == 0 && !sp.FlipX && !sp.FlipY {
		tl = ctm.TransformPoint(Pt(sp.DstX, sp.DstY))
		tr = ctm.TransformPoint(Pt(sp.DstX+sp.DstW, sp.DstY))
		br = ctm.TransformPoint(Pt(sp.DstX+sp.DstW, sp.DstY+sp.DstH))
		bl = ctm.TransformPoint(Pt(sp.DstX, sp.DstY+sp.DstH))
		return tl, tr, br, bl
	}
	px := sp.DstX + sp.PivotX
	py := sp.DstY + sp.PivotY
	cos := math.Cos(sp.Rot)
	sin := math.Sin(sp.Rot)
	xform := func(x, y float64) Point {
		if sp.FlipX {
			x = 2*px - x
		}
		if sp.FlipY {
			y = 2*py - y
		}
		dx := x - px
		dy := y - py
		ux := px + dx*cos - dy*sin
		uy := py + dx*sin + dy*cos
		return ctm.TransformPoint(Pt(ux, uy))
	}
	tl = xform(sp.DstX, sp.DstY)
	tr = xform(sp.DstX+sp.DstW, sp.DstY)
	br = xform(sp.DstX+sp.DstW, sp.DstY+sp.DstH)
	bl = xform(sp.DstX, sp.DstY+sp.DstH)
	return tl, tr, br, bl
}

// DrawAtlasEx 校验后按 R4 新分支绘制并返回降级标记（新函数，老路不动）。
// 空批或全跳过返回 Skipped；非有限数与未知过滤返回哨兵错且什么都不画。
// tint 染色走显卡逐顶点 premul 颜色（与 CPU 真采样同真值，只差采样舍入）。
// DrawAtlasEx 校验后按 R4 新分支绘制并返回降级标记（新函数，老路不动）。
// 空批或全跳过返回 Skipped；非有限数与未知过滤返回哨兵错且什么都不画。
// tint 染色走显卡逐顶点 premul 颜色（与 CPU 真采样同真值，只差采样舍入）。
func (c *Context) DrawAtlasEx(img *ImageBuf, sprites []AtlasSprite, _ AtlasDrawOptions) (AtlasDrawResult, error) {
	if c == nil || img == nil || len(sprites) == 0 {
		return AtlasDrawResult{Skipped: true}, nil
	}
	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return AtlasDrawResult{Skipped: true}, nil
	}
	for i := range sprites {
		if !isFiniteAtlasSprite(sprites[i]) {
			return AtlasDrawResult{}, ErrAtlasNonFinite
		}
		filt, err := normalizeAtlasFilter(sprites[i].Filter)
		if err != nil {
			return AtlasDrawResult{}, err
		}
		if sprites[i].Perspective {
			if err := atlasPerspValidate(sprites[i], filt); err != nil {
				return AtlasDrawResult{}, err
			}
		}
	}
	drawIdx := make([]int, 0, len(sprites))
	for i := range sprites {
		if atlasDrawable(sprites[i]) {
			drawIdx = append(drawIdx, i)
		}
	}
	if len(drawIdx) == 0 {
		return AtlasDrawResult{Skipped: true}, nil
	}
	useGPU := !CPUOnlyMode() && c.gpuCtxOps() != nil
	var pixelData []byte
	var genID uint64
	var stride int
	tintIface := false
	if useGPU {
		pixelData = img.PremultipliedData()
		if len(pixelData) == 0 {
			useGPU = false
		} else {
			genID = img.GenerationID()
			stride = img.Stride()
		}
		// 染色走逐顶点颜色：QueueImageDrawTint 已是 gpuContextOps 的
		// 必备方法（所有后端必须实现，旧桩随接口一起补），不断言；
		// 仍保留含染色整批回 CPU 的注释历史见 git log。
		tintIface = useGPU
	}
	if useGPU {
		defer c.setGPUClipRect()()
		target := c.gpuRenderTarget()
		vpW := uint32(target.Width)  //nolint:gosec // viewport fits uint32
		vpH := uint32(target.Height) //nolint:gosec // viewport fits uint32
		ctm := c.totalMatrix()
		queued := 0
		for _, i := range drawIdx {
			sp := sprites[i]
			filt, _ := normalizeAtlasFilter(sp.Filter)
			op := normalizeAtlasOpacity(sp.Opacity)
			// 透视精灵四角用户空间经 CTM 进设备空间（与 DrawImageQuad 同语义）；
			// 仿射精灵沿用 atlasSpriteCorners 拆法逐位不变。
			var dev [4]Point
			wFinal := [4]float64{1, 1, 1, 1}
			// P3.7 批 1d：投影层非恒等 ⇒ 四角走 `raw ∘ projection` 全链映射——
			// 仿射精灵升格为透视 quad，透视精灵权重齐次合成（w = w_proj × w_own）；
			// 任一角 behind-eye ⇒ 该精灵跳过（画家序：眼后不可画，与 quad 口整块
			// 拒绝同源）。喂 projMapQuad 的必须是**用户空间**角点：它内部一趟
			// 应用 raw CTM，传设备角会把 CTM 乘两遍。
			if c.projActive() {
				if sp.Perspective {
					copy(wFinal[:], sp.Weights[:])
				}
				pdev, wProj, ok := c.atlasProjectSpriteCorners(sp, wFinal)
				if !ok {
					continue // behind-eye sprite: refused whole
				}
				dev, wFinal = pdev, wProj
				if quadHasNonFinite(dev) || quadAreaSum(dev) < 1e-9 {
					return AtlasDrawResult{}, ErrAtlasQuadDegenerate
				}
			} else if sp.Perspective {
				copy(wFinal[:], sp.Weights[:])
				for k := 0; k < 4; k++ {
					dev[k] = ctm.TransformPoint(sp.Corners[k])
				}
				if quadHasNonFinite(dev) || quadAreaSum(dev) < 1e-9 {
					return AtlasDrawResult{}, ErrAtlasQuadDegenerate
				}
			} else {
				tl, trPt, br, bl := atlasSpriteCorners(sp, ctm)
				dev = [4]Point{tl, trPt, br, bl}
			}
			u0 := float32(sp.SrcX) / float32(imgW)
			v0 := float32(sp.SrcY) / float32(imgH)
			u1 := float32(sp.SrcX+sp.SrcW) / float32(imgW)
			v1 := float32(sp.SrcY+sp.SrcH) / float32(imgH)
			nearest := filt == InterpNearest
			bicubic := filt == InterpBicubic
			// 投影层生效 ⇒ 恒走透视槽（bicubic 在透视槽无变体 ⇒ 降 bilinear，
			// 与 quad 口同理由）；恒等时保持原分流逐字节不变。
			perspSlot := sp.Perspective || c.projActive()
			if perspSlot && bicubic {
				bicubic = false
				nearest = false
				if c.projActive() {
					// 透视槽无 bicubic 变体 ⇒ 采样核降级，具名计数（不静默换核）。
					c.projInterpDegrades++
				}
			}
			tintR, tintG, tintB, tintA := atlasTintFactors(sp.Tint)
			// Premultiply on the CPU side (texel premul * tint premul):
			// CPU does srcRGB = tex*tr*srcA with srcA = baseA*ta*op,
			// so the vertex tint must carry tr*ta (not straight tr).
			tintR, tintG, tintB = tintR*tintA, tintG*tintA, tintB*tintA
			// P3.7 批 3b：材质精灵（图集子矩形 UV 即 Godot TEXTURE/UV）。材质 +
			// 透视槽（自带透视或投影层）⇒ 跳该精灵+计数；材质+仿射 ⇒
			// QueueImageDrawMaterial（tint 走 36B 顶点通道）。
			if sp.Material != nil {
				if perspSlot {
					c.pathStats.MaterialSkips++
					continue
				}
				c.gpuCtxOps().QueueImageDrawMaterial(target, pixelData, genID, img, sp.Mipmap, imgW, imgH, stride,
					float32(dev[0].X), float32(dev[0].Y),
					float32(dev[1].X), float32(dev[1].Y),
					float32(dev[2].X), float32(dev[2].Y),
					float32(dev[3].X), float32(dev[3].Y),
					float32(op), vpW, vpH, u0, v0, u1, v1,
					float32(tintR), float32(tintG), float32(tintB), float32(tintA),
					sp.Material, nearest)
				queued++
				continue
			}
			if perspSlot {
				// 透视腿走 36B 透视槽（批 1a 合批谓词自动覆盖图集透视精灵；
				// 批 1b 扩的 tint 形参在这里同式下发）。bicubic 已在校验拒绝。
				c.gpuCtxOps().QueueImageDrawQuadW(target, pixelData, genID, img, sp.Mipmap, imgW, imgH, stride,
					float32(dev[0].X), float32(dev[0].Y),
					float32(dev[1].X), float32(dev[1].Y),
					float32(dev[2].X), float32(dev[2].Y),
					float32(dev[3].X), float32(dev[3].Y),
					float32(wFinal[0]), float32(wFinal[1]),
					float32(wFinal[2]), float32(wFinal[3]),
					float32(op), vpW, vpH, u0, v0, u1, v1,
					float32(tintR), float32(tintG), float32(tintB), float32(tintA),
					nearest)
			} else if tintIface {
				c.gpuCtxOps().QueueImageDrawTint(target, pixelData, genID, img, sp.Mipmap, imgW, imgH, stride,
					float32(dev[0].X), float32(dev[0].Y),
					float32(dev[1].X), float32(dev[1].Y),
					float32(dev[2].X), float32(dev[2].Y),
					float32(dev[3].X), float32(dev[3].Y),
					float32(op), vpW, vpH, u0, v0, u1, v1,
					float32(tintR), float32(tintG), float32(tintB), float32(tintA),
					nearest, bicubic)
			} else {
				c.gpuCtxOps().QueueImageDraw(target, pixelData, genID, img, sp.Mipmap, imgW, imgH, stride,
					float32(dev[0].X), float32(dev[0].Y),
					float32(dev[1].X), float32(dev[1].Y),
					float32(dev[2].X), float32(dev[2].Y),
					float32(dev[3].X), float32(dev[3].Y),
					float32(op), vpW, vpH, u0, v0, u1, v1, nearest, bicubic)
			}
			queued++
		}
		if queued > 0 {
			c.recordGPUOp()
			return AtlasDrawResult{Drawn: queued}, nil
		}
		return AtlasDrawResult{Skipped: true}, nil
	}
	if c.gpuPathAvailable() {
		c.recordCPUFallbackReason(AtlasCPUFallbackReason)
	}
	n := c.drawAtlasExCPU(img, sprites, drawIdx)
	if n == 0 {
		return AtlasDrawResult{Skipped: true}, nil
	}
	return AtlasDrawResult{Degraded: true, Reason: AtlasCPUFallbackReason, Drawn: n}, nil
}

// drawAtlasExCPU 用显卡同拆法逐像素真采样。
// 与 GPU 同为 TL-TR-BL + TR-BR-BL，对角线归首三角；采样钳边与显卡一致。
// drawAtlasExCPU 用显卡同拆法逐像素真采样。
// 与 GPU 同为 TL-TR-BL + TR-BR-BL，对角线归首三角；采样钳边与显卡一致。
func (c *Context) drawAtlasExCPU(img *ImageBuf, sprites []AtlasSprite, drawIdx []int) int {
	if c == nil || c.pixmap == nil || img == nil || len(drawIdx) == 0 {
		return 0
	}
	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return 0
	}
	if c.layerStack == nil || len(c.layerStack.layers) == 0 {
		c.flushGPUAccelerator()
	}
	c.noteLayerCPUDraw()
	pw, ph := c.pixmap.Width(), c.pixmap.Height()
	if pw <= 0 || ph <= 0 {
		return 0
	}
	hasClip := c.clipStack != nil && c.clipStack.Depth() > 0
	mask := c.mask
	premulSrc := img.Format().IsPremultiplied()
	ctm := c.totalMatrix()
	drawn := 0
	for _, si := range drawIdx {
		sp := sprites[si]
		if sp.Material != nil {
			// P3.7 批 3b：材质无 CPU 求值器——跳过 + 计数，绝不裸画。
			c.pathStats.MaterialSkips++
			continue
		}
		filt, _ := normalizeAtlasFilter(sp.Filter)
		op := normalizeAtlasOpacity(sp.Opacity)
		if op <= 0 {
			continue
		}
		tr, tg, tb, ta := atlasTintFactors(sp.Tint)
		// 透视精灵四角用户空间经 CTM 进设备空间（与 GPU 腿同拆法）；
		// 仿射精灵沿用 atlasSpriteCorners 拆法逐位不变。
		var dev [4]Point
		wFinal := [4]float64{1, 1, 1, 1}
		perspSampler := sp.Perspective
		// P3.7 批 1d：投影层非恒等 ⇒ 四角走 `raw ∘ projection` 全链（与 GPU 腿
		// 同式；喂**用户空间**角点，设备角会被 raw CTM 乘两遍；behind-eye 精灵
		// 跳过），权重齐次合成；透视采样核无 bicubic 变体 ⇒ 降 bilinear 并计数。
		if c.projActive() {
			if sp.Perspective {
				copy(wFinal[:], sp.Weights[:])
			}
			pdev, wProj, ok := c.atlasProjectSpriteCorners(sp, wFinal)
			if !ok {
				continue // behind-eye sprite: refused whole
			}
			dev, wFinal = pdev, wProj
			perspSampler = true
			if filt == InterpBicubic {
				filt = InterpBilinear
				c.projInterpDegrades++
			}
		} else if sp.Perspective {
			copy(wFinal[:], sp.Weights[:])
			for k := 0; k < 4; k++ {
				dev[k] = ctm.TransformPoint(sp.Corners[k])
			}
		} else {
			tl, trPt, br, bl := atlasSpriteCorners(sp, ctm)
			dev = [4]Point{tl, trPt, br, bl}
		}
		minX, minY, maxX, maxY := quadBounds(dev)
		c.trackDamage(image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1))
		u0 := sp.SrcX / float64(imgW)
		v0 := sp.SrcY / float64(imgH)
		u1 := (sp.SrcX + sp.SrcW) / float64(imgW)
		v1 := (sp.SrcY + sp.SrcH) / float64(imgH)
		// GPU twin UVs: TL(u0,v0) TR(u1,v0) BR(u1,v1) BL(u0,v1)；透视腿 1/w
		// 有理插值（与 DrawImageQuad CPU 腿同款 newTriSamplerPerspective）。
		var t1, t2 triSampler
		if perspSampler {
			t1 = newTriSamplerPerspective(dev[0], dev[1], dev[3], [3][2]float64{{u0, v0}, {u1, v0}, {u0, v1}},
				wFinal[0], wFinal[1], wFinal[3])
			t2 = newTriSamplerPerspective(dev[1], dev[2], dev[3], [3][2]float64{{u1, v0}, {u1, v1}, {u0, v1}},
				wFinal[1], wFinal[2], wFinal[3])
		} else {
			t1 = newTriSampler(dev[0], dev[1], dev[3], [3][2]float64{{u0, v0}, {u1, v0}, {u0, v1}})
			t2 = newTriSampler(dev[1], dev[2], dev[3], [3][2]float64{{u1, v0}, {u1, v1}, {u0, v1}})
		}
		x0 := int(math.Floor(minX))
		y0 := int(math.Floor(minY))
		x1 := int(math.Ceil(maxX))
		y1 := int(math.Ceil(maxY))
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		if x1 > pw {
			x1 = pw
		}
		if y1 > ph {
			y1 = ph
		}
		if x0 >= x1 || y0 >= y1 {
			continue
		}
		mode := intImage.InterpolationMode(filt)
		touched := false
		for y := y0; y < y1; y++ {
			py := float64(y) + 0.5
			for x := x0; x < x1; x++ {
				px := float64(x) + 0.5
				u, v, ok := t1.uv(px, py)
				if !ok {
					u, v, ok = t2.uv(px, py)
					if !ok {
						continue
					}
				}
				sr, sg, sb, sa := intImage.Sample(img, u, v, mode)
				if sa == 0 {
					continue
				}
				k := op
				if hasClip {
					cover := float64(c.clipStack.Coverage(px, py)) / 255.0
					if cover <= 0 {
						continue
					}
					k *= cover
				}
				if mask != nil {
					mc := float64(mask.At(x, y)) / 255.0
					if mc <= 0 {
						continue
					}
					k *= mc
				}
				if k <= 0 {
					continue
				}
				var srcR, srcG, srcB, srcA float64
				if premulSrc {
					srcA = float64(sa) / 255.0 * ta * k
					srcR = float64(sr) / 255.0 * tr * ta * k
					srcG = float64(sg) / 255.0 * tg * ta * k
					srcB = float64(sb) / 255.0 * tb * ta * k
				} else {
					baseA := float64(sa) / 255.0
					srcA = baseA * ta * k
					if srcA <= 0 {
						continue
					}
					srcR = float64(sr) / 255.0 * tr * srcA
					srcG = float64(sg) / 255.0 * tg * srcA
					srcB = float64(sb) / 255.0 * tb * srcA
				}
				if srcA <= 0 {
					continue
				}
				touched = true
				if srcA >= 1 {
					if srcR < 0 {
						srcR = 0
					}
					if srcG < 0 {
						srcG = 0
					}
					if srcB < 0 {
						srcB = 0
					}
					if srcR > 1 {
						srcR = 1
					}
					if srcG > 1 {
						srcG = 1
					}
					if srcB > 1 {
						srcB = 1
					}
					c.pixmap.SetPixelPremul(x, y, uint8(srcR*255+0.5), uint8(srcG*255+0.5), uint8(srcB*255+0.5), 255)
					continue
				}
				dstR, dstG, dstB, dstA := c.pixmap.getPremul(x, y)
				inv := 1 - srcA
				c.pixmap.setPremul(x, y,
					srcR+dstR*inv,
					srcG+dstG*inv,
					srcB+dstB*inv,
					srcA+dstA*inv,
				)
			}
		}
		_ = touched
		drawn++
	}
	return drawn
}

// Mesh describes an indexed triangle mesh for DrawMesh.
// Positions are user-space points (CTM applied). When Indices is non-empty, triangles
// are formed as (i0,i1,i2) groups of 3 indices; otherwise positions are a triangle list.
// When len(Colors)==len(Positions), Gouraud shading is used.
type Mesh struct {
	Positions []Point
	Colors    []RGBA
	Indices   []uint16
}

// DrawMesh draws an indexed (or triangle-list) colored mesh on the GPU path when available.
// This is the V.03 subset: positions + optional vertex colors + optional indices.
// Full custom fragment shaders / cubics are deferred.
//
// when Indices is set, the GPU path keeps unique verts + DrawIndexed
// (no CPU expand). CPU fallback still expands to triangle lists.
