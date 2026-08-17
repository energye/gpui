package hint

import (
	"fmt"
	"sync"
	"unsafe"
)

// CFF/CFF2 table-parse cache. LightHintVar reparses the entire CFF table for
// every glyph rasterization (readIndex hotspot — CJK masks, and empty glyphs
// like spaces that the glyph mask atlas never caches, re-trigger it on every
// draw). Parsing once per font makes repeated glyph rasterizations cheap.
//
// The parsed data holds slices into the font's raw bytes, so the cache keeps
// the raw data alive; the LRU cap bounds that (a UI session typically holds
// 2–4 font files).
const cffCacheMax = 8

// cffKey identifies a font's CFF/CFF2 table: the font raw-data backing array
// (same FontSource passes the same raw slice for every glyph) plus the
// collection index selecting the sub-font.
type cffKey struct {
	ptr     uintptr
	ln      int
	faceIdx int
}

var (
	cffCacheMu sync.Mutex
	cffParsed  = map[cffKey]*cffFontData{}
	cffOrder   []cffKey
	cff2Parsed = map[cffKey]*cff2FontData{}
	cff2Order  []cffKey
)

func cffCacheKey(raw []byte, faceIdx int) cffKey {
	if len(raw) == 0 {
		return cffKey{}
	}
	return cffKey{ptr: uintptr(unsafe.Pointer(&raw[0])), ln: len(raw), faceIdx: faceIdx}
}

// cffParseCached parses a font's CFF table once and reuses the result across
// all glyph rasterizations of that font.
func cffParseCached(raw []byte, faceIdx int) (*cffFontData, error) {
	k := cffCacheKey(raw, faceIdx)
	cffCacheMu.Lock()
	if cd, ok := cffParsed[k]; ok {
		cffCacheMu.Unlock()
		return cd, nil
	}
	cffCacheMu.Unlock()

	start, ln, err := cffTableData(raw, faceIdx)
	if err != nil {
		return nil, err
	}
	if ln <= 0 || start+ln > len(raw) {
		return nil, fmt.Errorf("hint: cff table out of range")
	}
	upem := hintFontUpem(raw, faceIdx)
	if upem <= 0 {
		upem = 1000
	}
	cd, err := cffParseAll(raw[start:start+ln], upem)
	if err != nil {
		return nil, err
	}
	cffCacheMu.Lock()
	if cd2, ok := cffParsed[k]; ok {
		cffCacheMu.Unlock()
		return cd2, nil
	}
	cffParsed[k] = cd
	cffOrder = append(cffOrder, k)
	for len(cffOrder) > cffCacheMax {
		old := cffOrder[0]
		cffOrder = cffOrder[1:]
		delete(cffParsed, old)
	}
	cffCacheMu.Unlock()
	return cd, nil
}

// cff2ParseCached is the CFF2 variant of cffParseCached.
func cff2ParseCached(raw []byte, faceIdx int) (*cff2FontData, error) {
	k := cffCacheKey(raw, faceIdx)
	cffCacheMu.Lock()
	if cd, ok := cff2Parsed[k]; ok {
		cffCacheMu.Unlock()
		return cd, nil
	}
	cffCacheMu.Unlock()

	start, ln, err := cff2TableData(raw, faceIdx)
	if err != nil {
		return nil, err
	}
	if ln <= 0 || start+ln > len(raw) {
		return nil, fmt.Errorf("hint: cff2 table out of range")
	}
	upem := hintFontUpem(raw, faceIdx)
	if upem <= 0 {
		upem = 1000
	}
	cd, err := cff2ParseAll(raw[start:start+ln], upem)
	if err != nil {
		return nil, err
	}
	cffCacheMu.Lock()
	if cd2, ok := cff2Parsed[k]; ok {
		cffCacheMu.Unlock()
		return cd2, nil
	}
	cff2Parsed[k] = cd
	cff2Order = append(cff2Order, k)
	for len(cff2Order) > cffCacheMax {
		old := cff2Order[0]
		cff2Order = cff2Order[1:]
		delete(cff2Parsed, old)
	}
	cffCacheMu.Unlock()
	return cd, nil
}