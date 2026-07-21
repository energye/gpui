package text

import "testing"

// TestGPOS_MarkToBase_Anchor places a mark on a base using Type 4.
//
// base glyph 10 advance 100, base anchor (50, 80)
// mark glyph 20 advance 0, mark anchor (10, 0)
// Expected mark xPlacement = 50 - 10 - 100 = -60
// Expected mark yPlacement = 80
func TestGPOS_MarkToBase_Anchor(t *testing.T) {
	markCov := buildCoverageFormat1(20)
	baseCov := buildCoverageFormat1(10)

	markArray := []byte{
		0, 1, // markCount
		0, 0, // class
		0, 6, // markAnchorOffset (relative to MarkArray)
		0, 1, // anchor format 1
		0, 10, // x
		0, 0, // y
	}
	baseArray := []byte{
		0, 1, // baseCount
		0, 4, // baseAnchorOffset[0]
		0, 1, // anchor format 1
		0, 50, // x
		0, 80, // y
	}

	header := make([]byte, 12)
	header[1] = 1
	header[7] = 1 // classCount

	markCovOff := 12
	baseCovOff := markCovOff + len(markCov)
	markArrayOff := baseCovOff + len(baseCov)
	baseArrayOff := markArrayOff + len(markArray)

	header[2] = byte(markCovOff >> 8)
	header[3] = byte(markCovOff)
	header[4] = byte(baseCovOff >> 8)
	header[5] = byte(baseCovOff)
	header[8] = byte(markArrayOff >> 8)
	header[9] = byte(markArrayOff)
	header[10] = byte(baseArrayOff >> 8)
	header[11] = byte(baseArrayOff)

	data := append(header, markCov...)
	data = append(data, baseCov...)
	data = append(data, markArray...)
	data = append(data, baseArray...)

	glyphs := []shapingGlyph{{gid: 10, cluster: 0}, {gid: 20, cluster: 1}}
	adj := make([]gposAdjustment, 2)
	adv := make([]uint16, 21)
	adv[10] = 100
	metrics := gposMetrics{hmtxAdv: adv, numHMetrics: 21}

	applyMarkToBasePos(data, glyphs, adj, metrics)

	if adj[1].xPlacement != -60 {
		t.Fatalf("mark xPlacement=%d want -60", adj[1].xPlacement)
	}
	if adj[1].yPlacement != 80 {
		t.Fatalf("mark yPlacement=%d want 80", adj[1].yPlacement)
	}
}

// TestGPOS_Cursive_EntryExit links two glyphs via Type 3 anchors.
func TestGPOS_Cursive_EntryExit(t *testing.T) {
	cov := buildCoverageFormat1(10, 11)
	exitAnchor := []byte{0, 1, 0, 100, 0, 5}
	entryAnchor := []byte{0, 1, 0, 0, 0, 0}

	covOff := 14
	exitOff := covOff + len(cov)
	entryOff := exitOff + len(exitAnchor)

	header := make([]byte, 14)
	header[1] = 1
	header[2] = byte(covOff >> 8)
	header[3] = byte(covOff)
	header[5] = 2 // entryExitCount
	// rec0: entry=0, exit=exitOff
	header[8] = byte(exitOff >> 8)
	header[9] = byte(exitOff)
	// rec1: entry=entryOff, exit=0
	header[10] = byte(entryOff >> 8)
	header[11] = byte(entryOff)

	data := append(header, cov...)
	data = append(data, exitAnchor...)
	data = append(data, entryAnchor...)

	glyphs := []shapingGlyph{{gid: 10}, {gid: 11}}
	adj := make([]gposAdjustment, 2)
	applyCursivePos(data, glyphs, adj)
	if adj[1].xPlacement != 100 || adj[1].yPlacement != 5 {
		t.Fatalf("cursive adj[1]=%+v want x=100 y=5", adj[1])
	}
}

// TestGPOS_MarkToLig_Anchor attaches a mark to a ligature via Type 5.
// lig glyph 30 advance 200, component0 anchor (40,70), component1 anchor (120,70)
// mark 20 anchor (5,0) → use last component → x = 120-5-200 = -85, y=70
func TestGPOS_MarkToLig_Anchor(t *testing.T) {
	markCov := buildCoverageFormat1(20)
	ligCov := buildCoverageFormat1(30)

	// MarkArray with mark anchor (5,0)
	markArray := []byte{
		0, 1, // count
		0, 0, // class 0
		0, 6, // markAnchorOffset
		0, 1, // anchor fmt1
		0, 5, // x
		0, 0, // y
	}

	// LigatureAttach for one lig: componentCount=2, classCount=1
	// component0 anchor offset, component1 anchor offset (relative to LigatureAttach)
	// Layout of LigatureAttach:
	//   [0] componentCount=2
	//   [2] c0 class0 anchorOff
	//   [4] c1 class0 anchorOff
	//   [6] anchor0 (40,70)
	//   [12] anchor1 (120,70)
	ligAttach := []byte{
		0, 2, // componentCount
		0, 6, // c0 anchor @6
		0, 12, // c1 anchor @12
		0, 1, 0, 40, 0, 70, // anchor0
		0, 1, 0, 120, 0, 70, // anchor1
	}

	// LigatureArray: count=1, offset to ligAttach
	// We'll assemble full subtable with offsets from start.

	// Build in pieces then fix header offsets.
	// header 12 + markCov + ligCov + markArray + ligArray(2+2) + ligAttach
	header := make([]byte, 12)
	header[1] = 1
	header[7] = 1 // classCount

	markCovOff := 12
	ligCovOff := markCovOff + len(markCov)
	markArrayOff := ligCovOff + len(ligCov)
	ligArrayOff := markArrayOff + len(markArray)
	// LigatureArray: ligCount=1, attachOff relative to LigatureArray start = 4
	ligArray := []byte{0, 1, 0, 4}
	// attach follows ligArray in stream; attachOff from LigatureArray = 4 means
	// attach starts at ligArrayOff+4 = right after ligArray header+offset = good
	// Wait: ligArray is 4 bytes, attachOff=4 points to start of attach after ligArray — correct.

	header[2] = byte(markCovOff >> 8)
	header[3] = byte(markCovOff)
	header[4] = byte(ligCovOff >> 8)
	header[5] = byte(ligCovOff)
	header[8] = byte(markArrayOff >> 8)
	header[9] = byte(markArrayOff)
	header[10] = byte(ligArrayOff >> 8)
	header[11] = byte(ligArrayOff)

	data := append(header, markCov...)
	data = append(data, ligCov...)
	data = append(data, markArray...)
	data = append(data, ligArray...)
	data = append(data, ligAttach...)

	glyphs := []shapingGlyph{{gid: 30, cluster: 0}, {gid: 20, cluster: 1}}
	adj := make([]gposAdjustment, 2)
	adv := make([]uint16, 31)
	adv[30] = 200
	metrics := gposMetrics{hmtxAdv: adv, numHMetrics: 31}

	applyMarkToLigPos(data, glyphs, adj, metrics)

	// last component anchor 120,70; mark 5,0; sumAdv=200
	// x = 0+120-5-200 = -85; y = 70
	if adj[1].xPlacement != -85 {
		t.Fatalf("mark-to-lig xPlacement=%d want -85", adj[1].xPlacement)
	}
	if adj[1].yPlacement != 70 {
		t.Fatalf("mark-to-lig yPlacement=%d want 70", adj[1].yPlacement)
	}
}
