package tex

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"

	"github.com/energye/gpui/game/core"
)

// Frozen block geometry for FormatBC1RGBAUnorm.
const (
	blockW = 4
	blockH = 4
	// blockBytesBC1 is 8 bytes per 4x4 block (2 RGB565 ends + 4 index bytes).
	blockBytesBC1 = 8
	// vkBC1RGBAUnorm is VK_FORMAT_BC1_RGBA_UNORM_BLOCK (Khronos Vulkan enum).
	vkBC1RGBAUnorm = 133
	// MaxDimension caps one side; MaxPixels caps the area. Beyond either
	// the file is well-formed but beyond budget: OutOfMemory, never a guess.
	MaxDimension = 8192
	MaxPixels    = 8192 * 8192
)

// Format names the frozen block kind. Only BC1 is built; the rest are
// reserved and report Unsupported through ParseKTX2.
type Format int

const (
	// FormatUnknown is the zero value: no kind recognized.
	FormatUnknown Format = iota
	// FormatBC1RGBAUnorm is KTX2 + BC1 RGBA (8 bytes per 4x4 block).
	FormatBC1RGBAUnorm
)

// String returns the stable log name of f.
func (f Format) String() string {
	switch f {
	case FormatBC1RGBAUnorm:
		return "BC1_RGBA_UNORM"
	default:
		return "unknown"
	}
}

// BlockBytes returns the GPU upload bytes per block (8 for BC1, 0 unknown).
func (f Format) BlockBytes() int {
	if f == FormatBC1RGBAUnorm {
		return blockBytesBC1
	}
	return 0
}

// BlockExtent returns the block footprint (4x4 for BC1, 0 unknown).
func (f Format) BlockExtent() (w, h int) {
	if f == FormatBC1RGBAUnorm {
		return blockW, blockH
	}
	return 0, 0
}

// VkFormat returns the Vulkan enum carried in the KTX2 header (133 for
// BC1 RGBA, 0 unknown).
func (f Format) VkFormat() uint32 {
	if f == FormatBC1RGBAUnorm {
		return vkBC1RGBAUnorm
	}
	return 0
}

// ktx2Identifier is the 12-byte KTX2 magic.
var ktx2Identifier = [12]byte{0xAB, 0x4B, 0x54, 0x58, 0x20, 0x32, 0x30, 0xBB, 0x0D, 0x0A, 0x1A, 0x0A}

// Image is one decoded KTX2 picture: the block bytes that upload to the
// GPU as-is plus the CPU-decoded RGBA8 pixels for the offscreen comparison.
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

// BlockCount returns the number of 4x4 blocks.
func (im *Image) BlockCount() int {
	if im == nil {
		return 0
	}
	return len(im.blocks) / blockBytesBC1
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

// ParseKTX2 validates a KTX2 file image and decodes its BC1 blocks.
// See doc.go for the frozen subset; anything else is Unsupported or BadData.
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

	if vk != vkBC1RGBAUnorm {
		return nil, core.Unsupported(op, u32name(vk), errors.New("unfrozen vkFormat"))
	}
	if typeSize != 1 {
		return nil, core.BadData(op, "typeSize", errors.New("want 1 for bc1"))
	}
	if w == 0 || h == 0 {
		return nil, core.BadData(op, "dimensions", errors.New("zero size"))
	}
	if w%blockW != 0 || h%blockH != 0 {
		return nil, core.BadData(op, "dimensions", errors.New("want multiple of 4 for bc1"))
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
	if super != 0 {
		return nil, core.Unsupported(op, "supercompression", errors.New("basis/zstd reserved for 3.3"))
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
	want := bw * bh * blockBytesBC1
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
	pixels := decodeBC1(blocks, int(w), int(h))
	return &Image{format: FormatBC1RGBAUnorm, width: int(w), height: int(h), blocks: blocks, pixels: pixels}, nil
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
