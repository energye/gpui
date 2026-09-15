// Package tex freezes the CPU-side compressed-texture intake every 2.5D
// draw feeds (capability 3.1, P2 long-chain head, S02/W1).
//
// Frozen 2026-09-15: Format, FormatBC1RGBAUnorm, ParseKTX2, LoadKTX2,
// ParseBasis, LoadBasis, Image accessors. Additive changes only.
//
// What is frozen: one container plus one block format. KTX2 (Khronos
// container v2, little-endian header) carrying VK_FORMAT_BC1_RGBA_UNORM_BLOCK
// (133), supercompression NONE, single 2D level, faceCount 1, layerCount 0,
// pixelDepth 0, width and height multiples of 4. The 8 block bytes upload
// to the GPU as-is; the CPU decodes the same blocks to RGBA8 for the
// offscreen comparison. One frozen kind first; a second kind freezes
// separately later.
//
// What is reserved: every other vkFormat, every non-zero
// supercompressionScheme (BasisLZ/Zstd/Zlib), multi-level (mipmaps go to
// 3.2), arrays/cubemaps/3D, uncompressed KTX2. They parse the header far
// enough to name the kind, then report core Unsupported, never a guessed
// image. The old PNG/JPG/WebP 8-bit path stays untouched.
//
// Errors use game/core codes: empty input is InvalidArg, a missing file is
// NotFound, a torn header or mismatched level bytes is BadData, a valid
// header naming an unfrozen kind is Unsupported, a picture beyond the
// MaxPixels budget is OutOfMemory. Only core numbers are used; decoded
// pixels cross to render as core.Color at the boundary.
package tex
