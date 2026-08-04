package hint

// M2: cf2Blues + cf2HintMap —— FreeType 2.14.3 src/psaux/psblues.c + pshints.c 移植。
//
// 输入：M1 charstring 解释器（cffcs.go）的精确 cs 轮廓 + FD 蓝区（cff.go）。
// 输出：DS 16.16 像素轮廓（Y 轴经 hintmap 映射，X 轴 scale 直通——cf2 的
// hintmap 只映射 Y（hStem），X = MulFix(scaleX, x)，vStem 仅参与 mask 位序）。
//
// 定点全为 16.16（FT 的 CF2_Fixed）。FT_MulFix 用 2.14.3 内联 64 位版
// （ftcalc.h:90-100）：ab += 0x8000 + (ab>>63)，负数向 -∞。

import "math"

// ---------------------------------------------------------------------------
// 定点原语（FT ftcalc.h / psfixed.h）

type cf2Fixed int64

const (
	cf2FixedOne       = cf2Fixed(65536)
	cf2ICFBottom      = -120 * cf2FixedOne // psblues.h:105
	cf2ICFTop         = 880 * cf2FixedOne  // psblues.h:104
	cf2MinCounter     = cf2Fixed(32768)    // 0.5px，psblues.h:114
	cf2FixedEpsilon   = cf2Fixed(1)        // 0x.0001
	cf2MaxHints       = 96                 // pshints.h:47
	cf2MaxHintEdges   = 192                // pshints.h:122
)

func cf2IntToFixed(i int64) cf2Fixed { return cf2Fixed(i) << 16 }
func cf2DoubleToFixed(f float64) cf2Fixed {
	return cf2Fixed(f*65536.0 + 0.5)
}
func cf2FixedAbs(x cf2Fixed) cf2Fixed {
	if x < 0 {
		return -x
	}
	return x
}

// cf2MulFix：FT_MulFix_64（ftcalc.h:90-100）。
func cf2MulFix(a, b cf2Fixed) cf2Fixed {
	ab := int64(a) * int64(b)
	ab += 0x8000 + (ab >> 63)
	return cf2Fixed(ab >> 16)
}

// cf2DivFix：FT_DivFix（ftcalc.h，向零舍入）。
func cf2DivFix(a, b cf2Fixed) cf2Fixed {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	if b == 0 {
		return 0
	}
	q := (int64(a)<<16 + int64(b)/2) / int64(b)
	return cf2Fixed(q)
}

// cf2MulDiv：FT_MulDiv（ftcalc.c:161-181，向零舍入）。
func cf2MulDiv(a, b, c cf2Fixed) cf2Fixed {
	s := 1
	if a < 0 {
		a = -a
		s = -s
	}
	if b < 0 {
		b = -b
		s = -s
	}
	if c < 0 {
		c = -c
		s = -s
	}
	if c == 0 {
		if s < 0 {
			return -cf2Fixed(0x7FFFFFFF)
		}
		return cf2Fixed(0x7FFFFFFF)
	}
	d := (int64(a)*int64(b) + int64(c)/2) / int64(c)
	if s < 0 {
		return -cf2Fixed(d)
	}
	return cf2Fixed(d)
}

// cf2ComputeDarkening：cf2_computeDarkening（psfont.c:51-232）。
// stemWidth 为字体单位（16.16）；返回值也为字体单位（16.16）。
func cf2ComputeDarkening(emRatio, ppem, stemWidth cf2Fixed, stemDarkened bool, darkenParams [8]int) cf2Fixed {
	var darkenAmount cf2Fixed
	if !stemDarkened {
		return 0
	}
	if emRatio < cf2DoubleToFixed(.01) {
		return 0
	}

	x1, y1 := int64(darkenParams[0]), int64(darkenParams[1])
	x2, y2 := int64(darkenParams[2]), int64(darkenParams[3])
	x3, y3 := int64(darkenParams[4]), int64(darkenParams[5])
	x4, y4 := int64(darkenParams[6]), int64(darkenParams[7])

	stemWidthPer1000 := cf2MulFix(stemWidth, emRatio)

	// 防溢出：logBase2(MSB 和) >= 46 → 直接取 x4 段（psfont.c:142-152）
	logBase2 := ftMSB(uint32(stemWidthPer1000)) + ftMSB(uint32(ppem))
	var scaledStem cf2Fixed
	if logBase2 >= 46 {
		scaledStem = cf2IntToFixed(x4)
	} else {
		scaledStem = cf2MulFix(stemWidthPer1000, ppem)
	}

	switch {
	case scaledStem < cf2IntToFixed(x1):
		darkenAmount = cf2DivFix(cf2IntToFixed(y1), ppem)
	case scaledStem < cf2IntToFixed(x2):
		x := stemWidthPer1000 - cf2DivFix(cf2IntToFixed(x1), ppem)
		darkenAmount = cf2MulDiv(x, cf2IntToFixed(y2-y1), cf2IntToFixed(x2-x1)) + cf2DivFix(cf2IntToFixed(y1), ppem)
	case scaledStem < cf2IntToFixed(x3):
		x := stemWidthPer1000 - cf2DivFix(cf2IntToFixed(x2), ppem)
		darkenAmount = cf2MulDiv(x, cf2IntToFixed(y3-y2), cf2IntToFixed(x3-x2)) + cf2DivFix(cf2IntToFixed(y2), ppem)
	case scaledStem < cf2IntToFixed(x4):
		x := stemWidthPer1000 - cf2DivFix(cf2IntToFixed(x3), ppem)
		darkenAmount = cf2MulDiv(x, cf2IntToFixed(y4-y3), cf2IntToFixed(x4-x3)) + cf2DivFix(cf2IntToFixed(y3), ppem)
	default:
		darkenAmount = cf2DivFix(cf2IntToFixed(y4), ppem)
	}

	// 每侧一半，并换算回真实字符空间（psfont.c:226-228）
	return cf2DivFix(darkenAmount, 2*emRatio)
}

// ftMSB：FT_MSB（ftutil.c），32 位最高位位置（0 表示 0）。
func ftMSB(x uint32) int {
	if x == 0 {
		return 0
	}
	n := 0
	for x > 0 {
		x >>= 1
		n++
	}
	return n - 1
}

// defaultDarkenParams：darkening-parameters 默认值（psfont.c:79-86）。
func defaultDarkenParams() [8]int {
	return [8]int{500, 400, 1000, 275, 1667, 275, 2333, 0}
}

// cf2DarkenAmounts：cf2_font_setup 的暗化计算（psfont.c:389-472）。
// 输入 fd 的 Private 蓝区（stdHW/stdVW，字体单位）；返回 darkenX/darkenY
// （字体单位 16.16；psft.c 中 ppem = max(4, px)）。
func cf2DarkenAmounts(fd *cffBlues, ppem, upem int) (cf2Fixed, cf2Fixed) {
	p := cf2IntToFixed(int64(ppem))
	if ppem < 4 {
		p = cf2IntToFixed(4)
	}
	emRatio := cf2IntToFixed(1000) / cf2Fixed(upem)
	params := defaultDarkenParams()

	stdVW := cf2IntToFixed(int64(fd.stdVW))
	if stdVW <= 0 {
		stdVW = cf2DivFix(cf2IntToFixed(75), emRatio)
	}
	darkenX := cf2ComputeDarkening(emRatio, p, stdVW, true, params)

	stdHW := cf2IntToFixed(int64(fd.stdHW))
	if stdHW > 0 && stdVW > 2*stdHW {
		stdHW = cf2DivFix(cf2IntToFixed(75), emRatio)
	} else {
		stdHW = cf2DivFix(cf2IntToFixed(110), emRatio)
	}
	darkenY := cf2ComputeDarkening(emRatio, p, stdHW, true, params)
	return darkenX, darkenY
}

// cf2FixedRound：cf2_fixedRound（psfixed.h:64），就近 1px（0x8000 舍入）。
// FT 的 CF2_Fixed 是 int32：uint32 运算后按 int32 符号解释。
func cf2FixedRound(x cf2Fixed) cf2Fixed {
	return cf2Fixed(int32((uint32(x) + 0x8000) & 0xFFFF0000))
}

// cf2FixedFloor：cf2_fixedFloor（psfixed.h:70）。
func cf2FixedFloor(x cf2Fixed) cf2Fixed {
	return cf2Fixed(int32(uint32(x) & 0xFFFF0000))
}

// cf2FixedFrac：cf2_fixedFraction（psfixed.h:72）。
func cf2FixedFrac(x cf2Fixed) cf2Fixed { return x - cf2FixedFloor(x) }

// ---------------------------------------------------------------------------
// CF2_Hint（pshints.h:126-147 的 edge 元素；flags 见 psblues.h:79-86）

type cf2HintFlags uint32

const (
	cf2GhostBottom cf2HintFlags = 0x01
	cf2GhostTop    cf2HintFlags = 0x02
	cf2PairBottom  cf2HintFlags = 0x04
	cf2PairTop     cf2HintFlags = 0x08
	cf2Locked      cf2HintFlags = 0x10
	cf2Synthetic   cf2HintFlags = 0x20
)

func (h cf2Hint) isValid() bool   { return h.flags != 0 }
func (h cf2Hint) isPair() bool    { return h.flags&(cf2PairBottom|cf2PairTop) != 0 }
func (h cf2Hint) isPairTop() bool { return h.flags&cf2PairTop != 0 }
func (h cf2Hint) isTop() bool     { return h.flags&(cf2PairTop|cf2GhostTop) != 0 }
func (h cf2Hint) isBottom() bool  { return h.flags&(cf2PairBottom|cf2GhostBottom) != 0 }
func (h cf2Hint) isLocked() bool  { return h.flags&cf2Locked != 0 }
func (h cf2Hint) isSynthetic() bool {
	return h.flags&cf2Synthetic != 0
}

type cf2Hint struct {
	csCoord cf2Fixed
	dsCoord cf2Fixed
	scale   cf2Fixed
	index   int
	flags   cf2HintFlags
}

// ---------------------------------------------------------------------------
// CF2_StemHint（pshints.h:85-95）：charstring 收集的原始 stem，含跨区 DS 复用

type cf2StemHint struct {
	used  bool
	min   cf2Fixed
	max   cf2Fixed
	minDS cf2Fixed
	maxDS cf2Fixed
	index int
}

// ---------------------------------------------------------------------------
// CF2_HintMask（pshints.h:70-82）：位序 = hstem 先 vstem 后，mask[0] 最高位 = bit 0

type cf2HintMask struct {
	isValid  bool
	isNew    bool
	bitCount int
	mask     []byte
}

// setAll：全部位置 1（cf2_hintmask_setAll，psintrp.c:175+）。
func (m *cf2HintMask) setAll(bitCount int) {
	if bitCount > cf2MaxHints {
		m.isValid = false
		return
	}
	m.bitCount = bitCount
	m.mask = make([]byte, (bitCount+7)/8)
	for i := range m.mask {
		m.mask[i] = 0xFF
	}
	m.isValid = true
	m.isNew = true
}

func (m *cf2HintMask) bit(i int) bool {
	return m.mask[i/8]&(0x80>>uint(i%8)) != 0
}

func (m *cf2HintMask) clearBit(i int) {
	m.mask[i/8] &^= 0x80 >> uint(i%8)
}

// ---------------------------------------------------------------------------
// CF2_Blues（psblues.c 移植）

type cf2BlueZone struct {
	csBottomEdge cf2Fixed
	csTopEdge    cf2Fixed
	csFlatEdge   cf2Fixed
	dsFlatEdge   cf2Fixed
	bottomZone   bool
}

type cf2Blues struct {
	scale            cf2Fixed
	blueScale        cf2Fixed
	blueShift        cf2Fixed
	blueFuzz         cf2Fixed
	zone             []cf2BlueZone
	count            int
	suppressOvershoot bool
	boost            cf2Fixed
	doEmBoxHints     bool
	emBoxBottomEdge  cf2Hint
	emBoxTopEdge     cf2Hint
}

// cf2BluesInit：cf2_blues_init（psblues.c:57-432）。
// fd 为 FD 的 Private 蓝区（FU 单位）；scale 为 hinted 16.16 scale
// （innerTransform.d = (x_scale+32)/64）；darkenY/stemDarkened 用于
// boost 抑制与幽灵顶偏移。
func cf2BluesInit(b *cf2Blues, fd *cffBlues, scale, darkenY cf2Fixed, stemDarkened bool) {
	*b = cf2Blues{scale: scale}

	// cf2_getBlueMetrics（psft.c:546-561）：blue_scale×1000 / 1000 → 16.16
	b.blueScale = cf2DivFix(cf2DoubleToFixed(fd.blueScale*1000), cf2IntToFixed(1000))
	b.blueShift = cf2IntToFixed(int64(fd.blueShift))
	b.blueFuzz = cf2IntToFixed(int64(fd.blueFuzz))

	blueValues := fd.blueValues
	otherBlues := fd.otherBlues
	familyBlues := fd.familyBlues
	familyOtherBlues := fd.familyOtherBlues

	numBlueValues := len(blueValues)
	numOtherBlues := len(otherBlues)
	numFamilyBlues := len(familyBlues)
	numFamilyOtherBlues := len(familyOtherBlues)

	emBoxBottom := cf2ICFBottom
	emBoxTop := cf2ICFTop

	// emBox 幽灵区启发式（psblues.c:133-178）
	if fd.languageGroup == 1 &&
		(numBlueValues == 0 ||
			(numBlueValues == 4 &&
				blueValues[0] < float64(emBoxBottom>>16) &&
				blueValues[1] < float64(emBoxBottom>>16) &&
				blueValues[2] > float64(emBoxTop>>16) &&
				blueValues[3] > float64(emBoxTop>>16))) {
		b.emBoxBottomEdge = cf2Hint{
			csCoord: emBoxBottom - cf2FixedEpsilon,
			dsCoord: cf2FixedRound(cf2MulFix(emBoxBottom-cf2FixedEpsilon, scale)) - cf2MinCounter,
			scale:   scale,
			flags:   cf2GhostBottom | cf2Locked | cf2Synthetic,
		}
		b.emBoxTopEdge = cf2Hint{
			csCoord: emBoxTop + cf2FixedEpsilon + 2*darkenY,
			dsCoord: cf2FixedRound(cf2MulFix(emBoxTop+cf2FixedEpsilon+2*darkenY, scale)) + cf2MinCounter,
			scale:   scale,
			flags:   cf2GhostTop | cf2Locked | cf2Synthetic,
		}
		b.doEmBoxHints = true
		return
	}

	var maxZoneHeight cf2Fixed

	// BlueValues：首对 bottom，余 top（psblues.c:182-227）
	for i := 0; i+1 < numBlueValues; i += 2 {
		z := cf2BlueZone{
			csBottomEdge: cf2IntToFixed(int64(blueValues[i])),
			csTopEdge:    cf2IntToFixed(int64(blueValues[i+1])),
		}
		zoneHeight := z.csTopEdge - z.csBottomEdge
		if zoneHeight < 0 {
			continue
		}
		if zoneHeight > maxZoneHeight {
			maxZoneHeight = zoneHeight
		}
		if i != 0 {
			// top 区上下边都上移 2×darkenY（psblues.c:203-208）
			z.csTopEdge += 2 * darkenY
			z.csBottomEdge += 2 * darkenY
		}
		if i == 0 {
			z.bottomZone = true
			z.csFlatEdge = z.csTopEdge
		} else {
			z.bottomZone = false
			z.csFlatEdge = z.csBottomEdge
		}
		b.zone = append(b.zone, z)
		b.count++
	}

	// OtherBlues：全 bottom（psblues.c:229-259）
	for i := 0; i+1 < numOtherBlues; i += 2 {
		z := cf2BlueZone{
			csBottomEdge: cf2IntToFixed(int64(otherBlues[i])),
			csTopEdge:    cf2IntToFixed(int64(otherBlues[i+1])),
		}
		zoneHeight := z.csTopEdge - z.csBottomEdge
		if zoneHeight < 0 {
			continue
		}
		if zoneHeight > maxZoneHeight {
			maxZoneHeight = zoneHeight
		}
		z.bottomZone = true
		z.csFlatEdge = z.csTopEdge
		b.zone = append(b.zone, z)
		b.count++
	}

	// FamilyBlues / FamilyOtherBlues：1px 内找最近 flat edge（psblues.c:261-345）
	csUnitsPerPixel := cf2DivFix(cf2IntToFixed(1), b.scale)
	for i := 0; i < b.count; i++ {
		flatEdge := b.zone[i].csFlatEdge
		var minDiff cf2Fixed = cf2Fixed(0x7FFFFFFF)
		if b.zone[i].bottomZone {
			for j := 0; j+1 < numFamilyOtherBlues; j += 2 {
				flatFamilyEdge := cf2IntToFixed(int64(familyOtherBlues[j+1]))
				diff := cf2FixedAbs(flatEdge - flatFamilyEdge)
				if diff < minDiff && diff < csUnitsPerPixel {
					b.zone[i].csFlatEdge = flatFamilyEdge
					minDiff = diff
					if diff == 0 {
						break
					}
				}
			}
			if numFamilyBlues >= 2 {
				flatFamilyEdge := cf2IntToFixed(int64(familyBlues[1]))
				diff := cf2FixedAbs(flatEdge - flatFamilyEdge)
				if diff < minDiff && diff < csUnitsPerPixel {
					b.zone[i].csFlatEdge = flatFamilyEdge
				}
			}
		} else {
			for j := 2; j+1 < numFamilyBlues; j += 2 {
				flatFamilyEdge := cf2IntToFixed(int64(familyBlues[j])) + 2*darkenY
				diff := cf2FixedAbs(flatEdge - flatFamilyEdge)
				if diff < minDiff && diff < csUnitsPerPixel {
					b.zone[i].csFlatEdge = flatFamilyEdge
					minDiff = diff
					if diff == 0 {
						break
					}
				}
			}
		}
	}

	// BlueScale 钳制（psblues.c:352-381）
	if maxZoneHeight > 0 {
		if b.blueScale > cf2DivFix(cf2IntToFixed(1), maxZoneHeight) {
			b.blueScale = cf2DivFix(cf2IntToFixed(1), maxZoneHeight)
		}
	}

	// suppressOvershoot + boost（psblues.c:391-412）
	if b.scale < b.blueScale {
		b.suppressOvershoot = true
		b.boost = cf2DoubleToFixed(.6) - cf2MulDiv(cf2DoubleToFixed(.6), b.scale, b.blueScale)
		if b.boost > 0x7FFF {
			b.boost = 0x7FFF
		}
	}
	if stemDarkened {
		b.boost = 0
	}

	// dsFlatEdge（psblues.c:417-431）
	for i := 0; i < b.count; i++ {
		if b.zone[i].bottomZone {
			b.zone[i].dsFlatEdge = cf2FixedRound(cf2MulFix(b.zone[i].csFlatEdge, b.scale) - b.boost)
		} else {
			b.zone[i].dsFlatEdge = cf2FixedRound(cf2MulFix(b.zone[i].csFlatEdge, b.scale) + b.boost)
		}
	}
}

// cf2BluesCapture：cf2_blues_capture（psblues.c:452-568）。
// 返回是否捕获；捕获则移动两边缘并置 Locked。
func (b *cf2Blues) capture(bottomHintEdge, topHintEdge *cf2Hint) bool {
	csFuzz := b.blueFuzz
	var dsNew cf2Fixed
	var dsMove cf2Fixed
	captured := false

	for i := 0; i < b.count; i++ {
		z := &b.zone[i]
		if z.bottomZone && bottomHintEdge.isBottom() {
			if z.csBottomEdge-csFuzz <= bottomHintEdge.csCoord &&
				bottomHintEdge.csCoord <= z.csTopEdge+csFuzz {
				if b.suppressOvershoot {
					dsNew = z.dsFlatEdge
				} else if z.csTopEdge-bottomHintEdge.csCoord >= b.blueShift {
					a := cf2FixedRound(bottomHintEdge.dsCoord)
					c := z.dsFlatEdge - cf2IntToFixed(1)
					if a < c {
						dsNew = a
					} else {
						dsNew = c
					}
				} else {
					dsNew = cf2FixedRound(bottomHintEdge.dsCoord)
				}
				dsMove = dsNew - bottomHintEdge.dsCoord
				captured = true
				break
			}
		}
		if !z.bottomZone && topHintEdge.isTop() {
			if z.csBottomEdge-csFuzz <= topHintEdge.csCoord &&
				topHintEdge.csCoord <= z.csTopEdge+csFuzz {
				if b.suppressOvershoot {
					dsNew = z.dsFlatEdge
				} else if topHintEdge.csCoord-z.csBottomEdge >= b.blueShift {
					a := cf2FixedRound(topHintEdge.dsCoord)
					c := z.dsFlatEdge + cf2IntToFixed(1)
					if a > c {
						dsNew = a
					} else {
						dsNew = c
					}
				} else {
					dsNew = cf2FixedRound(topHintEdge.dsCoord)
				}
				dsMove = dsNew - topHintEdge.dsCoord
				captured = true
				break
			}
		}
	}

	if captured {
		if bottomHintEdge.isValid() {
			bottomHintEdge.dsCoord += dsMove
			bottomHintEdge.flags |= cf2Locked
		}
		if topHintEdge.isValid() {
			topHintEdge.dsCoord += dsMove
			topHintEdge.flags |= cf2Locked
		}
	}
	return captured
}

// ---------------------------------------------------------------------------
// CF2_HintMap（pshints.c 移植）

type cf2HintMap struct {
	edges   []cf2Hint
	scale   cf2Fixed
	hinted  bool
	lastIdx int
	initial *cf2HintMap // 初始图（cf2_hintmap_init 的 initialHintMap）
}

// hintInit：cf2_hint_init（pshints.c:89-206）。从 stem 构造单边 hint。
func hintInit(stems []cf2StemHint, idx int, scale, darkenY cf2Fixed, bottom bool) cf2Hint {
	var h cf2Hint
	stem := &stems[idx]
	width := stem.max - stem.min

	switch {
	case width == -21*cf2FixedOne:
		// ghost bottom
		if bottom {
			h.csCoord = stem.max
			h.flags = cf2GhostBottom
		}
	case width == -20*cf2FixedOne:
		// ghost top
		if !bottom {
			h.csCoord = stem.min
			h.flags = cf2GhostTop
		}
	case width < 0:
		// inverted pair
		if bottom {
			h.csCoord = stem.max
			h.flags = cf2PairBottom
		} else {
			h.csCoord = stem.min
			h.flags = cf2PairTop
		}
	default:
		if bottom {
			h.csCoord = stem.min
			h.flags = cf2PairBottom
		} else {
			h.csCoord = stem.max
			h.flags = cf2PairTop
		}
	}

	if h.isTop() {
		h.csCoord += 2 * darkenY
	}
	h.scale = scale
	h.index = idx

	if h.flags != 0 && stem.used {
		if h.isTop() {
			h.dsCoord = stem.maxDS
		} else {
			h.dsCoord = stem.minDS
		}
		h.flags |= cf2Locked
	} else {
		h.dsCoord = cf2MulFix(h.csCoord, scale)
	}
	return h
}

// mapCS：cf2_hintmap_map（pshints.c:330-378）。
func (m *cf2HintMap) mapCS(cs cf2Fixed) cf2Fixed {
	if len(m.edges) == 0 || !m.hinted {
		return cf2MulFix(cs, m.scale)
	}
	i := m.lastIdx
	for i < len(m.edges)-1 && cs >= m.edges[i+1].csCoord {
		i++
	}
	for i > 0 && cs < m.edges[i].csCoord {
		i--
	}
	m.lastIdx = i
	if i == 0 && cs < m.edges[0].csCoord {
		return cf2MulFix(cs-m.edges[0].csCoord, m.scale) + m.edges[0].dsCoord
	}
	return cf2MulFix(cs-m.edges[i].csCoord, m.edges[i].scale) + m.edges[i].dsCoord
}

// adjustHints：cf2_hintmap_adjustHints（pshints.c:395-601）。
func (m *cf2HintMap) adjustHints() {
	type hintMove struct {
		j      int
		moveUp cf2Fixed
	}
	var hintMoves []hintMove

	for i := 0; i < len(m.edges); i++ {
		isPair := m.edges[i].isPair()
		j := i
		if isPair {
			j = i + 1
		}
		if j >= len(m.edges) {
			break
		}
		dsCoordI := m.edges[i].dsCoord
		dsCoordJ := m.edges[j].dsCoord

		if !m.edges[i].isLocked() {
			fracDown := cf2FixedFrac(dsCoordI)
			fracUp := cf2FixedFrac(dsCoordJ)

			downMoveDown := 0 - fracDown
			upMoveDown := 0 - fracUp
			var downMoveUp, upMoveUp cf2Fixed
			if fracDown == 0 {
				downMoveUp = 0
			} else {
				downMoveUp = cf2IntToFixed(1) - fracDown
			}
			if fracUp == 0 {
				upMoveUp = 0
			} else {
				upMoveUp = cf2IntToFixed(1) - fracUp
			}

			moveUp := downMoveUp
			if upMoveUp < moveUp {
				moveUp = upMoveUp
			}
			moveDown := downMoveDown
			if upMoveDown > moveDown {
				moveDown = upMoveDown
			}

			downMinCounter := cf2MinCounter
			upMinCounter := cf2MinCounter
			saveEdge := false
			var move cf2Fixed

			if j >= len(m.edges)-1 ||
				m.edges[j+1].dsCoord >= dsCoordJ+moveUp+upMinCounter {
				if i == 0 ||
					m.edges[i-1].dsCoord <= dsCoordI+moveDown-downMinCounter {
					if -moveDown < moveUp {
						move = moveDown
					} else {
						move = moveUp
					}
				} else {
					move = moveUp
				}
			} else {
				if i == 0 ||
					m.edges[i-1].dsCoord <= dsCoordI+moveDown-downMinCounter {
					move = moveDown
					if moveUp < -moveDown {
						saveEdge = true
					}
				} else {
					move = 0
					saveEdge = true
				}
			}

			if saveEdge &&
				j < len(m.edges)-1 &&
				!m.edges[j+1].isLocked() {
				hintMoves = append(hintMoves, hintMove{j: j, moveUp: moveUp - move})
			}

			m.edges[i].dsCoord = dsCoordI + move
			if isPair {
				m.edges[j].dsCoord = dsCoordJ + move
			}
		}

		if i > 0 {
			if m.edges[i].csCoord != m.edges[i-1].csCoord {
				m.edges[i-1].scale = cf2DivFix(
					m.edges[i].dsCoord-m.edges[i-1].dsCoord,
					m.edges[i].csCoord-m.edges[i-1].csCoord)
			}
		}
		if isPair {
			if m.edges[j].csCoord != m.edges[j-1].csCoord {
				m.edges[j-1].scale = cf2DivFix(
					m.edges[j].dsCoord-m.edges[j-1].dsCoord,
					m.edges[j].csCoord-m.edges[j-1].csCoord)
			}
			i++
		}
	}

	// 二遍上移（pshints.c:573-600）
	for n := len(hintMoves) - 1; n >= 0; n-- {
		hm := hintMoves[n]
		j := hm.j
		if m.edges[j+1].dsCoord >= m.edges[j].dsCoord+hm.moveUp+cf2MinCounter {
			m.edges[j].dsCoord += hm.moveUp
			if m.edges[j].isPair() && j > 0 {
				m.edges[j-1].dsCoord += hm.moveUp
			}
		}
	}
}

// insertHint：cf2_hintmap_insertHint（pshints.c:604-796）。
// m.initial 有效时未锁定边经初始图定位（midpoint∓halfWidth）。
func (m *cf2HintMap) insertHint(bottomHintEdge, topHintEdge *cf2Hint) {
	isPair := true
	first := bottomHintEdge
	second := topHintEdge

	if !bottomHintEdge.isValid() {
		first = topHintEdge
		isPair = false
	} else if !topHintEdge.isValid() {
		isPair = false
	}

	if isPair && topHintEdge.csCoord < bottomHintEdge.csCoord {
		return
	}

	indexInsert := 0
	for indexInsert < len(m.edges) && m.edges[indexInsert].csCoord < first.csCoord {
		indexInsert++
	}

	if indexInsert < len(m.edges) {
		if m.edges[indexInsert].csCoord == first.csCoord {
			return
		}
		if isPair && m.edges[indexInsert].csCoord <= second.csCoord {
			return
		}
		if m.edges[indexInsert].isPairTop() {
			return
		}
	}

	if m.initial != nil && len(m.initial.edges) > 0 && !first.isLocked() {
		if isPair {
			midpoint := m.initial.mapCS(first.csCoord + (second.csCoord-first.csCoord)/2)
			halfWidth := cf2MulFix((second.csCoord-first.csCoord)/2, m.scale)
			first.dsCoord = midpoint - halfWidth
			second.dsCoord = midpoint + halfWidth
		} else {
			first.dsCoord = m.initial.mapCS(first.csCoord)
		}
	}

	if indexInsert > 0 {
		if first.dsCoord < m.edges[indexInsert-1].dsCoord {
			return
		}
	}
	if indexInsert < len(m.edges) {
		if isPair {
			if second.dsCoord > m.edges[indexInsert].dsCoord {
				return
			}
		} else {
			if first.dsCoord > m.edges[indexInsert].dsCoord {
				return
			}
		}
	}

	// 插入（copy struct）
	if isPair {
		m.edges = append(m.edges, cf2Hint{}, cf2Hint{})
		copy(m.edges[indexInsert+2:], m.edges[indexInsert:len(m.edges)-2])
		m.edges[indexInsert] = *first
		m.edges[indexInsert+1] = *second
	} else {
		m.edges = append(m.edges, cf2Hint{})
		copy(m.edges[indexInsert+1:], m.edges[indexInsert:len(m.edges)-1])
		m.edges[indexInsert] = *first
	}
}

// build：cf2_hintmap_build（pshints.c:813-1092）。
// hStems/vStems 为 charstring stem 数组；mask 为当前 hintmask（未提供时
// 全激活）；initial 表示构建初始图（含 0 合成点）。
func (m *cf2HintMap) build(blues *cf2Blues, hStems, vStems []cf2StemHint, mask *cf2HintMask, scale, darkenY cf2Fixed, initial bool) {
	m.scale = scale
	m.hinted = true
	m.edges = nil
	m.lastIdx = 0

	// 初始图未建 → 递归建（pshints.c:820-838）。注意：FT 只在首次
	// build 时构建 initialHintMap，后续区复用（isValid 检查）；因此
	// hintCFFLight 须先构建共享 initial 图赋给各区，否则后续区重建时
	// 会把已 used（被前区锁定 DS）的 stem 以 Locked 状态插入初始图。
	if !initial && m.initial == nil {
		m.initial = &cf2HintMap{}
		m.initial.build(blues, hStems, vStems, nil, scale, darkenY, true)
	}

	bitCount := len(hStems)
	maskPtr := mask
	if mask == nil || !mask.isValid {
		nm := &cf2HintMask{}
		nm.setAll(bitCount + len(vStems))
		maskPtr = nm
	}
	if maskPtr.bitCount < bitCount {
		return
	}
	if !initial && !m.hinted {
		return
	}

	// 幽灵区最高优先级（pshints.c:878-893）
	if blues.doEmBoxHints {
		dummy := cf2Hint{}
		be := blues.emBoxBottomEdge
		m.insertHint(&be, &dummy)
		te := blues.emBoxTopEdge
		m.insertHint(&dummy, &te)
	}

	// 被捕获/已锁定 stem 高优先级（pshints.c:897-941）
	for i := 0; i < bitCount; i++ {
		if maskPtr.bit(i) {
			bottomHintEdge := hintInit(hStems, i, scale, darkenY, true)
			topHintEdge := hintInit(hStems, i, scale, darkenY, false)
			if bottomHintEdge.isLocked() || topHintEdge.isLocked() ||
				blues.capture(&bottomHintEdge, &topHintEdge) {
				m.insertHint(&bottomHintEdge, &topHintEdge)
				maskPtr.clearBit(i)
			}
		}
	}

	if initial {
		// 0 合成点（pshints.c:965-991）
		if len(m.edges) == 0 ||
			m.edges[0].csCoord > 0 ||
			m.edges[len(m.edges)-1].csCoord < 0 {
			edge := cf2Hint{
				flags: cf2GhostBottom | cf2Locked | cf2Synthetic,
				scale: m.scale,
			}
			m.insertHint(&edge, &cf2Hint{})
		}
	} else {
		// 其余激活 stem（pshints.c:996-1031）
		for i := 0; i < bitCount; i++ {
			if maskPtr.bit(i) {
				bottomHintEdge := hintInit(hStems, i, scale, darkenY, true)
				topHintEdge := hintInit(hStems, i, scale, darkenY, false)
				m.insertHint(&bottomHintEdge, &topHintEdge)
			}
		}
	}

	m.adjustHints()

	// 保存 DS 位置供跨区复用（pshints.c:1062-1085）
	if !initial {
		for i := 0; i < len(m.edges); i++ {
			if !m.edges[i].isSynthetic() {
				idx := m.edges[i].index
				if idx >= 0 && idx < len(hStems) {
					if m.edges[i].isTop() {
						hStems[idx].maxDS = m.edges[i].dsCoord
					} else {
						hStems[idx].minDS = m.edges[i].dsCoord
					}
					hStems[idx].used = true
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 主入口：cs 轮廓 → DS 16.16 轮廓（Y 轴 hintmap + X 轴直通）

// cf2StemSlice 把 csOutline 的 stem 数组转成 16.16 的 cf2StemHint 数组。
func cf2StemSlice(stems []csStem) []cf2StemHint {
	out := make([]cf2StemHint, len(stems))
	for i, s := range stems {
		out[i] = cf2StemHint{
			min:   cf2IntToFixed(int64(math.Round(s.lo))),
			max:   cf2IntToFixed(int64(math.Round(s.hi))),
			index: i,
		}
	}
	return out
}

// cf2HintResult 是 M2 的输出：每轮廓的 DS 点（16.16 px）。
type cf2HintResult struct {
	pts      [][2]cf2Fixed // 与 cs 轮廓同序（不含重复首点）
	contours []int
}

// hintCFFLight：M2/M3 主入口。对 cs 轮廓施加 cf2 语义的 Y 轴 hintmap
// （hStem 集 + 蓝区捕获），X 轴按 scale 直通（cf2 build 只用 hStem 位，
// vStem 仅贡献 mask 位宽，pshints.c:847-850）。
// 单区（无 hintmask）= 全 1 mask 一张图；多区 = 每个 hintmask 事件一张图，
// 路径点按「产生时最新 mask」归属（pshints.c:1705-1720 moveTo/lineTo
// 的 cf2_hintmask_isNew 触发 rebuild 语义）。
// 返回 DS 16.16 坐标。
func hintCFFLight(cs *csOutline, fd *cffFD, scale cf2Fixed, darkenX, darkenY cf2Fixed) *cf2HintResult {
	hStems := cf2StemSlice(cs.hstems)
	vStems := cf2StemSlice(cs.vstems)

	var blues cf2Blues
	cf2BluesInit(&blues, fd.blues, scale, darkenY, true)

	// 建 zone 序列：zone0 = 全 1 初始图；其后每 mask 事件一区。
	// 各区共享 hStems/vStems（build 末尾保存 DS 供跨区锁定，pshints.c:1062-1085）。
	type hintZone struct {
		atPt int
	}
	var zones []hintZone
	// FT 语义（pshints.c:1705-1712 + 840-850）：hintMask 无效（无任何
	// hintmask 事件）→ setAll 全 1 建图；若首个事件发生在首点之前
	// （atPt==0），moveTo 时 mask 已有效 → 直接建事件图，无全 1 图
	// （全 1 图会错误锁定全部 stem 的 DS，污染后续区）。
	startZones := 0
	if len(cs.maskEvents) == 0 || cs.maskEvents[0].atPt > 0 {
		zones = append(zones, hintZone{atPt: 0})
		startZones = 1
	}
	for _, ev := range cs.maskEvents {
		zones = append(zones, hintZone{atPt: ev.atPt})
	}
	var masks []cf2HintMask
	if startZones == 1 {
		all := cf2HintMask{}
		all.setAll(len(hStems) + len(vStems))
		masks = append(masks, all)
	}
	for _, ev := range cs.maskEvents {
		masks = append(masks, cf2HintMask{isValid: true, isNew: true, bitCount: ev.numHints, mask: ev.mask})
	}
	maps := make([]*cf2HintMap, len(zones))
	// initial 图只建一次（首次全 1 状态，stems 未 used），各区共享
	// （pshints.c:820-838：isValid 后复用，不重建）。
	initMap := &cf2HintMap{}
	initMap.build(&blues, hStems, vStems, nil, scale, darkenY, true)
	for i := range zones {
		hm := &cf2HintMap{initial: initMap}
		hm.build(&blues, hStems, vStems, &masks[i], scale, darkenY, false)
		maps[i] = hm
	}

	zoneOf := func(k int) *cf2HintMap {
		z := maps[0]
		for i := 1; i < len(zones); i++ {
			if zones[i].atPt <= k {
				z = maps[i]
			}
		}
		return z
	}

	out := &cf2HintResult{contours: cs.contours}
	off := 0
	for _, n := range cs.contours {
		for k := 0; k < n; k++ {
			p := cs.pts[off+k]
			x := cf2MulFix(cf2IntToFixed(int64(math.Round(p.x))), scale)
			y := zoneOf(off + k).mapCS(cf2IntToFixed(int64(math.Round(p.y))))
			out.pts = append(out.pts, [2]cf2Fixed{x, y})
		}
		off += n
	}
	return out
}
