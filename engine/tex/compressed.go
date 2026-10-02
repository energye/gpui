//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package tex

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"

	"github.com/energye/gpui/engine/core"
)

// Frozen block geometry for FormatBC1RGBAUnorm.
// S64 (2026-10-02, additive): FormatASTC_RGBA_4x4 frozen as the second
// kind (ASTC picked over ETC2, one kind only). ASTC 4x4 is the top ASTC
// tier (16 bytes per 4x4 block, 1/4 of RGBA8); Go carries and validates
// the blocks and never ships an ASTC software decoder.
const (
	blockW = 4
	blockH = 4
	// blockBytesBC1 is 8 bytes per 4x4 block (2 RGB565 ends + 4 index bytes).
	blockBytesBC1 = 8
	// blockBytesASTC is 16 bytes per 4x4 ASTC block.
	blockBytesASTC = 16
	// vkBC1RGBAUnorm is VK_FORMAT_BC1_RGBA_UNORM_BLOCK (Khronos Vulkan enum).
	vkBC1RGBAUnorm = 133
	// vkASTC4x4Unorm is VK_FORMAT_ASTC_4x4_UNORM_BLOCK (Khronos Vulkan enum).
	vkASTC4x4Unorm = 157
	// MaxDimension caps one side; MaxPixels caps the area. Beyond either
	// the file is well-formed but beyond budget: OutOfMemory, never a guess.
	MaxDimension = 8192
	MaxPixels    = 8192 * 8192
	// LargeTranscodePixels marks a big picture: at or above this pixel
	// count callers must use TranscodeAsync so transcode never runs on
	// the play thread. 256x256 sits exactly on the line.
	LargeTranscodePixels = 256 * 256
)

// KTX2 supercompressionScheme values (header offset 44). Only SchemeNone
// uploads as-is; the rest are rejected with the scheme named in the cause.
const (
	// SchemeNone means no supercompression: level bytes are blocks as-is.
	SchemeNone = 0
	// SchemeBasisLZ is Basis Universal / BasisLZ supercompression.
	SchemeBasisLZ = 1
	// SchemeZstd is Zstd supercompression.
	SchemeZstd = 2
	// SchemeZlib is Zlib supercompression.
	SchemeZlib = 3
)

// Format names the frozen block kind. BC1 plus ASTC 4x4 are built; the
// rest are reserved and report Unsupported through ParseKTX2.
type Format int

const (
	// FormatUnknown is the zero value: no kind recognized.
	FormatUnknown Format = iota
	// FormatBC1RGBAUnorm is KTX2 + BC1 RGBA (8 bytes per 4x4 block).
	FormatBC1RGBAUnorm
	// FormatASTCRGBA4x4 is KTX2 + ASTC 4x4 RGBA (16 bytes per 4x4 block).
	// S64 second kind: ETC2 stays reserved.
	FormatASTCRGBA4x4
)

// String returns the stable log name of f.
func (f Format) String() string {
	switch f {
	case FormatBC1RGBAUnorm:
		return "BC1_RGBA_UNORM"
	case FormatASTCRGBA4x4:
		return "ASTC_RGBA_4x4"
	default:
		return "unknown"
	}
}

// BlockBytes returns the GPU upload bytes per block (8 BC1, 16 ASTC,
// 0 unknown).
func (f Format) BlockBytes() int {
	switch f {
	case FormatBC1RGBAUnorm:
		return blockBytesBC1
	case FormatASTCRGBA4x4:
		return blockBytesASTC
	}
	return 0
}

// BlockExtent returns the block footprint (4x4 for both kinds, 0 unknown).
func (f Format) BlockExtent() (w, h int) {
	switch f {
	case FormatBC1RGBAUnorm, FormatASTCRGBA4x4:
		return blockW, blockH
	}
	return 0, 0
}

// VkFormat returns the Vulkan enum carried in the KTX2 header (133 BC1
// RGBA, 157 ASTC 4x4, 0 unknown).
func (f Format) VkFormat() uint32 {
	switch f {
	case FormatBC1RGBAUnorm:
		return vkBC1RGBAUnorm
	case FormatASTCRGBA4x4:
		return vkASTC4x4Unorm
	}
	return 0
}

// schemeName names a supercompressionScheme for the reject reason.
func schemeName(s uint32) string {
	switch s {
	case SchemeNone:
		return "none"
	case SchemeBasisLZ:
		return "basis-lz"
	case SchemeZstd:
		return "zstd"
	case SchemeZlib:
		return "zlib"
	default:
		return u32name(s)
	}
}

// ktx2Identifier is the 12-byte KTX2 magic.
var ktx2Identifier = [12]byte{0xAB, 0x4B, 0x54, 0x58, 0x20, 0x32, 0x30, 0xBB, 0x0D, 0x0A, 0x1A, 0x0A}

// Image is one decoded KTX2 picture: the block bytes that upload to the
// GPU as-is plus the CPU-decoded RGBA8 pixels for the offscreen comparison.
// BC1 pixels are decoded from the blocks. ASTC is GPU-only in Go (no ASTC
// software decoder ships here): pixels hold the tiled magenta proxy so the
// picture still measures honestly and shows pink on CPU; real colors come
// from Transcode with a driver hook.
type Image struct {
	format Format
	width  int
	height int
	blocks []byte
	pixels []byte
}

// Format returns the block kind.
func (im *Image) Format() Format {
	if im == nil {
		return FormatUnknown
	}
	return im.format
}

// Width returns the picture width in pixels.
func (im *Image) Width() int {
	if im == nil {
		return 0
	}
	return im.width
}

// Height returns the picture height in pixels.
func (im *Image) Height() int {
	if im == nil {
		return 0
	}
	return im.height
}

// BlockCount returns the number of 4x4 blocks (format-aware: BC1
// divides by 8, ASTC by 16).
func (im *Image) BlockCount() int {
	if im == nil {
		return 0
	}
	if bb := im.format.BlockBytes(); bb > 0 {
		return len(im.blocks) / bb
	}
	return 0
}

// UploadSize returns the GPU upload bytes (blocks as-is).
func (im *Image) UploadSize() int {
	if im == nil {
		return 0
	}
	return len(im.blocks)
}

// PixelSize returns the decoded RGBA8 bytes (w*h*4).
func (im *Image) PixelSize() int {
	if im == nil {
		return 0
	}
	return len(im.pixels)
}

// Ratio returns decoded bytes per upload byte (8 for BC1).
func (im *Image) Ratio() float64 {
	if im == nil || len(im.blocks) == 0 {
		return 0
	}
	return float64(len(im.pixels)) / float64(len(im.blocks))
}

// Blocks returns a copy of the GPU block bytes. The copy never aliases
// the image; mutating it cannot corrupt a replay.
func (im *Image) Blocks() []byte {
	if im == nil {
		return nil
	}
	return cloneBytes(im.blocks)
}

// Pixels returns a copy of the decoded RGBA8 bytes, row-major top-left.
func (im *Image) Pixels() []byte {
	if im == nil {
		return nil
	}
	return cloneBytes(im.pixels)
}

// At returns the pixel at (x, y) as a core.Color. Out-of-range or nil
// reports ok=false, never a panic.
func (im *Image) At(x, y int) (core.Color, bool) {
	if im == nil || x < 0 || y < 0 || x >= im.width || y >= im.height {
		return core.Color{}, false
	}
	off := (y*im.width + x) * 4
	return core.ColorFromBytes(
		im.pixels[off], im.pixels[off+1], im.pixels[off+2], im.pixels[off+3]), true
}

// Equal reports whether o holds bitwise the same picture.
func (im *Image) Equal(o *Image) bool {
	if im == nil || o == nil {
		return im == o
	}
	return im.format == o.format && im.width == o.width && im.height == o.height &&
		bytes.Equal(im.blocks, o.blocks) && bytes.Equal(im.pixels, o.pixels)
}

// ParseKTX2 validates a KTX2 file image and carries its blocks.
// BC1 blocks also decode to RGBA8; ASTC blocks ride as-is with the pink
// CPU proxy (see Image). See doc.go for the frozen subset; anything else
// is Unsupported or BadData.
func ParseKTX2(data []byte) (*Image, error) {
	const op = "tex.ParseKTX2"
	if len(data) == 0 {
		return nil, core.InvalidArg(op, "data")
	}
	if len(data) < 80 {
		return nil, core.BadData(op, "header", errors.New("truncated"))
	}
	if !bytes.Equal(data[:12], ktx2Identifier[:]) {
		return nil, core.BadData(op, "identifier", errors.New("not ktx2"))
	}
	vk := u32at(data, 12)
	typeSize := u32at(data, 16)
	w := u32at(data, 20)
	h := u32at(data, 24)
	depth := u32at(data, 28)
	layer := u32at(data, 32)
	face := u32at(data, 36)
	levels := u32at(data, 40)
	super := u32at(data, 44)
	dfdOff := u32at(data, 48)
	dfdLen := u32at(data, 52)
	kvdOff := u32at(data, 56)
	kvdLen := u32at(data, 60)
	sgdOff := u64at(data, 64)
	sgdLen := u64at(data, 72)

	if vk != vkBC1RGBAUnorm && vk != vkASTC4x4Unorm {
		return nil, core.Unsupported(op, u32name(vk), errors.New("unfrozen vkFormat"))
	}
	format := FormatBC1RGBAUnorm
	blockBytes := blockBytesBC1
	if vk == vkASTC4x4Unorm {
		format = FormatASTCRGBA4x4
		blockBytes = blockBytesASTC
	}
	if typeSize != 1 {
		return nil, core.BadData(op, "typeSize", errors.New("want 1 for block format"))
	}
	if w == 0 || h == 0 {
		return nil, core.BadData(op, "dimensions", errors.New("zero size"))
	}
	if w%blockW != 0 || h%blockH != 0 {
		return nil, core.BadData(op, "dimensions", errors.New("want multiple of 4"))
	}
	if w > MaxDimension || h > MaxDimension || uint64(w)*uint64(h) > MaxPixels {
		return nil, core.OutOfMemory(op, "dimensions", errors.New("beyond budget"))
	}
	if depth != 0 {
		return nil, core.Unsupported(op, "pixelDepth", errors.New("3d reserved"))
	}
	if layer != 0 {
		return nil, core.Unsupported(op, "layerCount", errors.New("array reserved"))
	}
	if face != 1 {
		return nil, core.Unsupported(op, "faceCount", errors.New("cubemap reserved"))
	}
	if levels != 1 {
		return nil, core.Unsupported(op, "levelCount", errors.New("mipmaps go to 3.2"))
	}
	if super != SchemeNone {
		return nil, core.Unsupported(op, "supercompression",
			errors.New("scheme "+schemeName(super)+" needs transcode, only none uploads as-is"))
	}
	if dfdLen > 0 && !rangeOK(uint64(dfdOff), uint64(dfdLen), len(data)) {
		return nil, core.BadData(op, "dfd", errors.New("out of range"))
	}
	if kvdLen > 0 && !rangeOK(uint64(kvdOff), uint64(kvdLen), len(data)) {
		return nil, core.BadData(op, "kvd", errors.New("out of range"))
	}
	if sgdLen > 0 && !rangeOK(sgdOff, sgdLen, len(data)) {
		return nil, core.BadData(op, "sgd", errors.New("out of range"))
	}
	if len(data) < 80+24 {
		return nil, core.BadData(op, "levelIndex", errors.New("truncated"))
	}
	lvlOff := u64at(data, 80)
	lvlLen := u64at(data, 88)
	lvlUnc := u64at(data, 96)
	bw := uint64(w) / blockW
	bh := uint64(h) / blockH
	want := bw * bh * uint64(blockBytes)
	if lvlLen != want {
		return nil, core.BadData(op, "levelLength", errors.New("block bytes mismatch"))
	}
	if lvlUnc != lvlLen {
		return nil, core.BadData(op, "uncompressedLength", errors.New("want equal for supercompression none"))
	}
	if lvlOff+lvlLen > uint64(len(data)) {
		return nil, core.BadData(op, "levelData", errors.New("out of range"))
	}
	blocks := cloneBytes(data[lvlOff : lvlOff+lvlLen])
	if format == FormatASTCRGBA4x4 {
		// GPU-only: carry the blocks, show pink on CPU. No ASTC
		// decoder lives in Go; colors arrive via Transcode.
		return &Image{format: format, width: int(w), height: int(h), blocks: blocks, pixels: pinkPixels(int(w), int(h))}, nil
	}
	pixels := decodeBC1(blocks, int(w), int(h))
	return &Image{format: format, width: int(w), height: int(h), blocks: blocks, pixels: pixels}, nil
}

// LoadKTX2 reads path and parses it as KTX2.
func LoadKTX2(path string) (*Image, error) {
	const op = "tex.LoadKTX2"
	if path == "" {
		return nil, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, core.NotFound(op, path, err)
	}
	return ParseKTX2(raw)
}

// ParseBasis is the reserved Basis entry: frozen as unsupported so callers
// can probe it without guessing. Empty input stays InvalidArg.
func ParseBasis(data []byte) (*Image, error) {
	const op = "tex.ParseBasis"
	if len(data) == 0 {
		return nil, core.InvalidArg(op, "data")
	}
	return nil, core.Unsupported(op, "basis", errors.New("reserved, use ktx2+bc1"))
}

// LoadBasis reads path far enough to name the miss: missing file is
// NotFound, any present file is Unsupported (Basis decoders live later).
func LoadBasis(path string) (*Image, error) {
	const op = "tex.LoadBasis"
	if path == "" {
		return nil, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, core.NotFound(op, path, err)
	}
	return ParseBasis(raw)
}

// u32at reads one little-endian uint32. Callers already checked the
// 80-byte header, so the slice cannot run short.
func u32at(b []byte, off int) uint32 { return binary.LittleEndian.Uint32(b[off : off+4]) }

// pinkBlock is the frozen opaque-magenta BC1 block (endpoints equal take
// the transparent branch by design; index 0 still decodes to one opaque
// magenta, same bytes as the Stream placeholder).
var pinkBlock = [8]byte{0x1F, 0xF8, 0x1F, 0xF8, 0, 0, 0, 0}

// pinkPixels tiles opaque magenta over w*h. Callers already validated the
// budget, so the make cannot run wild.
func pinkPixels(w, h int) []byte {
	out := make([]byte, w*h*4)
	for i := 0; i < len(out); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = 255, 0, 255, 255
	}
	return out
}

// pinkImage builds a same-size BC1 magenta stand-in. Bad sizes fall back
// to the shared 4x4 placeholder; never nil, never a panic.
func pinkImage(w, h int) *Image {
	if w <= 0 || h <= 0 || w%blockW != 0 || h%blockH != 0 ||
		w > MaxDimension || h > MaxDimension || uint64(w)*uint64(h) > MaxPixels {
		return PlaceholderImage()
	}
	nb := (w / blockW) * (h / blockH)
	blocks := make([]byte, nb*blockBytesBC1)
	for i := range nb {
		copy(blocks[i*blockBytesBC1:], pinkBlock[:])
	}
	return &Image{format: FormatBC1RGBAUnorm, width: w, height: h, blocks: blocks, pixels: pinkPixels(w, h)}
}

// IsPink reports whether im is all-magenta (transcode fallback or ASTC
// CPU proxy). Nil is never pink.
func IsPink(im *Image) bool {
	if im == nil {
		return false
	}
	if len(im.pixels) == 0 || len(im.pixels) != im.width*im.height*4 {
		return false
	}
	for i := 0; i < len(im.pixels); i += 4 {
		if im.pixels[i] != 255 || im.pixels[i+1] != 0 || im.pixels[i+2] != 255 || im.pixels[i+3] != 255 {
			return false
		}
	}
	return true
}

// Usage names what a picture is sampled as. Normal maps need precision:
// the lowest block tier stays color-only.
type Usage int

const (
	// UsageColor is base color / diffuse: any frozen format may sample it.
	UsageColor Usage = iota
	// UsageNormal is a tangent-space normal map: BC1 is refused.
	UsageNormal
)

// String returns the stable log name of u.
func (u Usage) String() string {
	if u == UsageNormal {
		return "normal"
	}
	return "color"
}

// CheckUsage refuses a format/usage pair that would smear. BC1 is the
// lowest tier (4-color 4x4 blocks) and is color-only; ASTC 4x4 is the top
// ASTC tier and may carry normals. Anything unknown is refused loudly.
func CheckUsage(f Format, u Usage) error {
	const op = "tex.CheckUsage"
	if u != UsageColor && u != UsageNormal {
		return core.InvalidArg(op, u.String())
	}
	switch f {
	case FormatBC1RGBAUnorm:
		if u == UsageNormal {
			return core.InvalidArg(op, "normal",
				errors.New("bc1 is the lowest tier, normals need astc-4x4 or better"))
		}
		return nil
	case FormatASTCRGBA4x4:
		return nil
	default:
		return core.Unsupported(op, f.String(), errors.New("unfrozen format"))
	}
}

// GPUCaps names what the card decodes natively. Go routes on this table;
// the driver owns the pixels.
type GPUCaps struct {
	// Name is the stable log name (e.g. "desktop-bc1").
	Name string
	// BC1 decodes VK 133 natively.
	BC1 bool
	// ASTC decodes VK 157 natively.
	ASTC bool
}

// Frozen per-card profiles. Desktop GL/DX without ASTC lands on
// desktop-bc1; ASTC phones land on mobile-astc; universal keeps both.
var (
	// CapsDesktopBC1 decodes BC1 only: ASTC must transcode or go pink.
	CapsDesktopBC1 = GPUCaps{Name: "desktop-bc1", BC1: true}
	// CapsMobileASTC decodes ASTC only.
	CapsMobileASTC = GPUCaps{Name: "mobile-astc", ASTC: true}
	// CapsUniversal decodes both frozen kinds.
	CapsUniversal = GPUCaps{Name: "universal", BC1: true, ASTC: true}
	// CapsNone decodes neither: every compressed picture goes pink.
	CapsNone = GPUCaps{Name: "none"}
)

// TranscodeTarget is the routing decision for one picture on one card.
type TranscodeTarget int

const (
	// TargetNative uploads the blocks as-is; the card decodes them.
	TargetNative TranscodeTarget = iota
	// TargetBC1 re-packs to BC1 (driver hook); Go only validates.
	TargetBC1
	// TargetPink serves the magenta stand-in; no pixels are guessed.
	TargetPink
)

// String returns the stable log name of t.
func (t TranscodeTarget) String() string {
	switch t {
	case TargetNative:
		return "native"
	case TargetBC1:
		return "bc1"
	default:
		return "pink"
	}
}

// TranscodeTargetFor routes format f on caps: native where the card
// decodes, ASTC falls back to BC1 where only BC1 exists, everything else
// falls back to pink. Pure table, never touches pixels.
func TranscodeTargetFor(f Format, caps GPUCaps) TranscodeTarget {
	switch f {
	case FormatBC1RGBAUnorm:
		if caps.BC1 {
			return TargetNative
		}
		return TargetPink
	case FormatASTCRGBA4x4:
		if caps.ASTC {
			return TargetNative
		}
		if caps.BC1 {
			return TargetBC1
		}
		return TargetPink
	default:
		return TargetPink
	}
}

// Transcoder is the driver hook that re-packs ASTC blocks to BC1 blocks
// (same geometry, 8 bytes per 4x4 block). Nil means no driver here: the
// caller gets pink plus Unsupported. Engine ships no ASTC decoder, so a
// test or driver must supply this.
type Transcoder func(blocks []byte, w, h int) (bc1Blocks []byte, err error)

// Transcode routes im for caps. Native returns im as-is. BC1 routes run
// fn and decode the result with the frozen BC1 decoder; fn missing,
// failing, or short returns a same-size pink image plus the cause, never
// a panic and never nil. Nil im returns the shared placeholder.
func Transcode(im *Image, caps GPUCaps, fn Transcoder) (*Image, error) {
	const op = "tex.Transcode"
	if im == nil {
		return PlaceholderImage(), core.InvalidArg(op, "image")
	}
	switch TranscodeTargetFor(im.format, caps) {
	case TargetNative:
		return im, nil
	case TargetBC1:
		if fn == nil {
			return pinkImage(im.width, im.height),
				core.Unsupported(op, caps.Name, errors.New("astc needs a driver transcoder, have none"))
		}
		out, err := fn(im.blocks, im.width, im.height)
		if err != nil {
			return pinkImage(im.width, im.height), err
		}
		nb := (im.width / blockW) * (im.height / blockH)
		if len(out) != nb*blockBytesBC1 {
			return pinkImage(im.width, im.height), core.BadData(op, "transcoder",
				errors.New("want bc1 block bytes for geometry"))
		}
		blocks := cloneBytes(out)
		return &Image{format: FormatBC1RGBAUnorm, width: im.width, height: im.height,
			blocks: blocks, pixels: decodeBC1(blocks, im.width, im.height)}, nil
	default:
		return pinkImage(im.width, im.height),
			core.Unsupported(op, caps.Name, errors.New("no route for "+im.format.String()))
	}
}

// TranscodeResult is one background transcode outcome. Fallback is true
// exactly when Err is non-nil (pink served); Image is never nil.
type TranscodeResult struct {
	Image    *Image
	Err      error
	Fallback bool
}

// NeedsAsync reports whether im is a big picture whose transcode must go
// through TranscodeAsync instead of the play thread.
func NeedsAsync(im *Image) bool {
	if im == nil {
		return false
	}
	return uint64(im.width)*uint64(im.height) >= LargeTranscodePixels
}

// TranscodeAsync runs Transcode off the caller thread and delivers one
// TranscodeResult on the returned buffered channel. The call itself never
// blocks; a failing transcode still delivers pink, never a panic.
func TranscodeAsync(im *Image, caps GPUCaps, fn Transcoder) <-chan TranscodeResult {
	ch := make(chan TranscodeResult, 1)
	go func() {
		out, err := Transcode(im, caps, fn)
		ch <- TranscodeResult{Image: out, Err: err, Fallback: err != nil}
	}()
	return ch
}

// u64at reads one little-endian uint64 (level index, sgd range).
func u64at(b []byte, off int) uint64 { return binary.LittleEndian.Uint64(b[off : off+8]) }

// rangeOK reports whether off+ln stays inside a file of size total.
func rangeOK(off, ln uint64, total int) bool { return off+ln <= uint64(total) }

// cloneBytes copies b so callers can never alias image internals.
func cloneBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// blend mixes endpoints with integer weights, truncating like the frozen
// decoder always did: blend(a,b,1,1) is the midpoint, blend(a,b,2,1) and
// blend(a,b,1,2) are the BC1 4-color steps.
func blend(a, b uint8, wa, wb uint32) uint8 {
	return uint8((uint32(a)*wa + uint32(b)*wb) / (wa + wb))
}

func u32name(v uint32) string {
	// Numeric name on purpose; DFD text names arrive with 3.2 mipmaps.
	const digits = "0123456789"
	var out [10]byte
	n := v
	i := len(out)
	if n == 0 {
		i--
		out[i] = '0'
	} else {
		for n > 0 {
			i--
			out[i] = digits[n%10]
			n /= 10
		}
	}
	return "vk" + string(out[i:])
}

func rgb565To888(c uint16) (r, g, b uint8) {
	// Rounded expansion on purpose: (v*255+half)/max keeps solid
	// primaries bit-exact and may sit 1 above a truncating renderer.
	// Package tolerance 0 only pins our own golden, not that renderer.
	r5 := (c >> 11) & 31
	g6 := (c >> 5) & 63
	b5 := c & 31
	r = uint8((uint32(r5)*255 + 15) / 31)
	g = uint8((uint32(g6)*255 + 31) / 63)
	b = uint8((uint32(b5)*255 + 15) / 31)
	return r, g, b
}

// decodeBC1 expands BC1 blocks to RGBA8, row-major top-left.
// Blocks are 8 bytes: c0 LE, c1 LE, 32-bit 2-bit indices (LSB = top-left).
// Equal endpoints take the transparent branch by design; our frozen blocks
// use index 0 there, so they still decode to one opaque color.
func decodeBC1(blocks []byte, w, h int) []byte {
	out := make([]byte, w*h*4)
	bw := w / blockW
	for by := 0; by < h/blockH; by++ {
		for bx := 0; bx < bw; bx++ {
			blk := blocks[(by*bw+bx)*blockBytesBC1:]
			c0 := binary.LittleEndian.Uint16(blk[0:2])
			c1 := binary.LittleEndian.Uint16(blk[2:4])
			bits := binary.LittleEndian.Uint32(blk[4:8])
			r0, g0, b0 := rgb565To888(c0)
			r1, g1, b1 := rgb565To888(c1)
			var pal [4][4]uint8
			pal[0] = [4]uint8{r0, g0, b0, 255}
			pal[1] = [4]uint8{r1, g1, b1, 255}
			if c0 > c1 {
				pal[2] = [4]uint8{
					blend(r0, r1, 2, 1),
					blend(g0, g1, 2, 1),
					blend(b0, b1, 2, 1), 255,
				}
				pal[3] = [4]uint8{
					blend(r0, r1, 1, 2),
					blend(g0, g1, 1, 2),
					blend(b0, b1, 1, 2), 255,
				}
			} else {
				pal[2] = [4]uint8{
					blend(r0, r1, 1, 1),
					blend(g0, g1, 1, 1),
					blend(b0, b1, 1, 1), 255,
				}
				pal[3] = [4]uint8{0, 0, 0, 0}
			}
			for py := 0; py < blockH; py++ {
				for px := 0; px < blockW; px++ {
					idx := (bits >> (2 * uint(py*blockW+px))) & 3
					x := bx*blockW + px
					y := by*blockH + py
					off := (y*w + x) * 4
					out[off] = pal[idx][0]
					out[off+1] = pal[idx][1]
					out[off+2] = pal[idx][2]
					out[off+3] = pal[idx][3]
				}
			}
		}
	}
	return out
}
